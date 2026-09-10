package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"github.com/vmr/kas-vmr-backend/config"
	"github.com/vmr/kas-vmr-backend/internal/handler"
	appmw "github.com/vmr/kas-vmr-backend/internal/middleware"
	"github.com/vmr/kas-vmr-backend/internal/repository"
	"github.com/vmr/kas-vmr-backend/internal/routes"
	"github.com/vmr/kas-vmr-backend/internal/usecase"
	"github.com/vmr/kas-vmr-backend/pkg/jwtutil"
	"github.com/vmr/kas-vmr-backend/pkg/mailwatcher"
	"github.com/vmr/kas-vmr-backend/pkg/tokenstore"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	db, err := config.NewDatabase(cfg)
	if err != nil {
		log.Fatalf("database error: %v", err)
	}
	if cfg.AutoMigrate {
		if err := config.RunMigrations(db); err != nil {
			log.Fatalf("migration error: %v", err)
		}
	}

	redisClient, err := config.NewRedisClient(cfg)
	if err != nil {
		log.Fatalf("redis error: %v", err)
	}
	defer redisClient.Close()

	// ---- pkg-level dependencies ----
	signer := jwtutil.NewSigner(cfg.JWTAccessSecret, cfg.JWTRefreshSecret)
	tokens := tokenstore.New(redisClient)

	// ---- repositories ----
	memberRepo := repository.NewMemberRepository(db)
	adminRepo := repository.NewAdminRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	cashFlowRepo := repository.NewCashFlowRepository(db)
	logSignTfRepo := repository.NewLogSignTfRepository(db)
	activityRepo := repository.NewActivityRepository(db)
	emailTxRepo := repository.NewEmailTransactionRepository(db)
	pushRepo := repository.NewPushRepository(db)

	// ---- usecases ----
	adminUsecase := usecase.NewAdminUsecase(adminRepo, signer, tokens)
	memberUsecase := usecase.NewMemberUsecase(memberRepo, signer, tokens, cfg.SetDefaultPin)
	cashFlowUsecase := usecase.NewCashFlowUsecase(cashFlowRepo)
	notificationUsecase := usecase.NewNotificationUsecase(
		pushRepo, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject,
	)
	paymentUsecase := usecase.NewPaymentUsecase(
		db, paymentRepo, cashFlowRepo, memberRepo, logSignTfRepo, notificationUsecase,
		float64(cfg.IuranAmount), cfg.StartMonth, cfg.StartYear, cfg.BendaharaNameKeyword,
	)
	activityUsecase := usecase.NewActivityUsecase(activityRepo, memberRepo)

	// AutoConfirmUsecase is only meaningful when the mail watcher is
	// enabled, but we still construct it either way and let
	// PaymentHandler accept a nil *AutoConfirmUsecase gracefully - simpler
	// than threading an extra "enabled" flag through the handler layer.
	var autoConfirmUsecase *usecase.AutoConfirmUsecase
	if cfg.MailEnabled {
		autoConfirmUsecase = usecase.NewAutoConfirmUsecase(
			emailTxRepo, memberRepo, paymentUsecase,
			float64(cfg.IuranAmount), cfg.UniqueCodeBase,
		)
	}

	// ---- handlers ----
	h := routes.Handlers{
		Admin:    handler.NewAdminHandler(adminUsecase),
		Member:   handler.NewMemberHandler(memberUsecase, adminUsecase, notificationUsecase, cfg.VAPIDPublicKey),
		Payment:  handler.NewPaymentHandler(paymentUsecase, autoConfirmUsecase, cfg.UploadDir),
		CashFlow: handler.NewCashFlowHandler(cashFlowUsecase),
		Activity: handler.NewActivityHandler(activityUsecase),
	}

	// ---- echo setup ----
	e := echo.New()
	e.HideBanner = true
	e.Validator = appmw.NewRequestValidator()

	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	e.Use(echomw.CORS())
	e.Use(echomw.BodyLimit("10M"))

	routes.Register(e, h, signer, activityUsecase)

	// ---- background workers ----
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()

	if cfg.MailEnabled && autoConfirmUsecase != nil {
		watcher := mailwatcher.New(mailwatcher.Config{
			Host:         cfg.MailIMAPHost,
			Username:     cfg.MailUsername,
			AppPassword:  cfg.MailAppPassword,
			SenderFilter: cfg.MailSenderFilter,
			PollInterval: time.Duration(cfg.MailPollInterval) * time.Second,
		}, autoConfirmUsecase.HandleTransfer)

		go func() {
			log.Println("mailwatcher: starting (IMAP IDLE with polling fallback)")
			if err := watcher.Run(workerCtx); err != nil && err != context.Canceled {
				log.Printf("mailwatcher: stopped: %v", err)
			}
		}()
	} else {
		log.Println("mailwatcher: disabled (set MAIL_WATCHER_ENABLED=true to enable auto-confirm via email)")
	}

	// ---- graceful shutdown ----
	go func() {
		addr := ":" + cfg.Port
		log.Printf("kas-vmr-backend listening on %s (env=%s)", addr, cfg.AppEnv)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("shutting down gracefully...")
	cancelWorkers()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
}
