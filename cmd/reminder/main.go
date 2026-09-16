package main

import (
	"fmt"
	"log"
	"time"

	"github.com/vmr/kas-vmr-backend/config"
	"github.com/vmr/kas-vmr-backend/internal/repository"
	"github.com/vmr/kas-vmr-backend/internal/usecase"
	"github.com/vmr/kas-vmr-backend/pkg/mailwatcher"
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

	// ---- repositories ----
	memberRepo := repository.NewMemberRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	transaksiRepo := repository.NewTransaksiRepository(db)
	cashFlowRepo := repository.NewCashFlowRepository(db)
	emailTxRepo := repository.NewEmailTransactionRepository(db)
	pushRepo := repository.NewPushRepository(db)

	// ---- usecases ----
	notificationUsecase := usecase.NewNotificationUsecase(pushRepo, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject)
	paymentUsecase := usecase.NewPaymentUsecase(
		db, paymentRepo, transaksiRepo, cashFlowRepo, memberRepo, notificationUsecase,
		float64(cfg.IuranAmount), cfg.StartMonth, cfg.StartYear, cfg.BendaharaNameKeyword,
	)
	autoConfirmUsecase := usecase.NewAutoConfirmUsecase(emailTxRepo, memberRepo, paymentUsecase)

	// ---- skenario yang mau ditest ----
	// Sesuaikan note ini dengan nomor rumah yang beneran ada di DB kamu,
	// biar bisa lihat mana yang match dan mana yang sengaja ditolak.
	scenarios := []struct {
		label string
		note  string
	}{
		{"format benar (nempel)", "12A"},
		{"format benar, lowercase", "12a"},
		{"salah: ada spasi sebelum huruf", "12 A"},
		{"salah: ada kata tambahan", "Rumah 12A"},
		{"salah: kalimat bebas", "bayar kas 12a trims"},
	}

	amountPerMonth := float64(cfg.IuranAmount)

	for i, sc := range scenarios {
		amount := amountPerMonth * float64(i+1) // beda nominal tiap skenario, hindari FindDuplicate

		tx := &mailwatcher.Transaction{
			Time:            time.Now().Format("02 Jan 2006 15:04"),
			Type:            "Real Time - Sesama Rekening SeaBank",
			SenderName:      "TEST PENGIRIM",
			SenderAccount:   "XXXXXX0000",
			Amount:          amount,
			ReferenceNumber: fmt.Sprintf("TEST-%d-%d", time.Now().UnixNano(), i),
			Note:            sc.note,
		}

		log.Printf("=== [%s] note=%q, amount=Rp%.0f ===", sc.label, sc.note, amount)

		if err := autoConfirmUsecase.HandleTransfer(tx); err != nil {
			log.Printf("    -> HandleTransfer error: %v", err)
		}
		log.Println()
	}

	log.Println("semua skenario selesai dijalankan. cek log di atas: 'unmatched' vs 'bayar N bulan'.")
	log.Println("menunggu goroutine push notif selesai...")
	time.Sleep(5 * time.Second)
	log.Println("selesai.")
}
