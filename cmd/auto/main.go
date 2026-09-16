package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/vmr/kas-vmr-backend/config"
	"github.com/vmr/kas-vmr-backend/internal/repository"
	"github.com/vmr/kas-vmr-backend/internal/usecase"
	"github.com/vmr/kas-vmr-backend/pkg/mailwatcher"
)

func main() {
	note := flag.String("note", "", "isi catatan/note transfer, misal '12A' atau 'Rumah 12 A'")
	months := flag.Int("months", 2, "jumlah bulan iuran yang mau ditest")
	flag.Parse()

	if *note == "" {
		log.Fatal("wajib isi --note, contoh: go run ./cmd/test-transfer --note=\"12A\"")
	}

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

	// ---- dummy transfer data ----
	amount := float64(cfg.IuranAmount) * float64(*months)

	dummyTx := &mailwatcher.Transaction{
		Time:            time.Now().Format("02 Jan 2006 15:04"),
		Type:            "Real Time - Sesama Rekening SeaBank",
		SenderName:      "TEST PENGIRIM",
		SenderAccount:   "XXXXXX0000",
		Amount:          amount,
		ReferenceNumber: fmt.Sprintf("TEST-%d", time.Now().UnixNano()), // unik tiap run
		Note:            *note,
	}

	log.Printf("test-transfer: note=%q, amount=Rp%.0f (%d bulan)...", *note, amount, *months)

	if err := autoConfirmUsecase.HandleTransfer(dummyTx); err != nil {
		log.Fatalf("HandleTransfer error: %v", err)
	}

	log.Println("test-transfer: transaksi tercatat, menunggu push notif terkirim...")
	time.Sleep(5 * time.Second) // beri waktu goroutine SendToMember selesai sebelum exit

	log.Println("test-transfer: selesai. Cek notif di HP.")

	log.Println("test-transfer: selesai. Cek log di atas: 'unmatched' (note gak match) atau berhasil tercatat + notif.")
}