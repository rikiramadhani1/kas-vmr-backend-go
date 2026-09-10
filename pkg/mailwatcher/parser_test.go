package mailwatcher

import "testing"

// sampleEmailBody is the exact sample provided, with sender name/account
// as given (already illustrative placeholders in the original sample).
const sampleEmailBody = `Hai NAMA PENERIMA,


Terima kasih telah menggunakan aplikasi SeaBank. Kamu menerima transfer masuk ke rekening SeaBank kamu.
Berikut ini adalah detail transaksi kamu:


Waktu Transaksi

08 Sep 2026 18:01

Jenis Transaksi

Real Time - Sesama Rekening SeaBank

Nama Pengirim

SI PENGIRIM

Nomor Rekening Pengirim

XXXXXX1011

Jumlah

Rp10.000

No. Referensi

2026090nomorref9546085

Catatan



Mohon simpan email ini sebagai referensi atas transaksi kamu.

Salam Hangat,
SeaBank
`

func TestParseSeaBankTransferEmail(t *testing.T) {
	tx, err := ParseSeaBankTransferEmail(sampleEmailBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tx.Time != "08 Sep 2026 18:01" {
		t.Errorf("Time = %q, want %q", tx.Time, "08 Sep 2026 18:01")
	}
	if tx.Type != "Real Time - Sesama Rekening SeaBank" {
		t.Errorf("Type = %q", tx.Type)
	}
	if tx.SenderName != "SI PENGIRIM" {
		t.Errorf("SenderName = %q", tx.SenderName)
	}
	if tx.SenderAccount != "XXXXXX1011" {
		t.Errorf("SenderAccount = %q", tx.SenderAccount)
	}
	if tx.Amount != 10000 {
		t.Errorf("Amount = %v, want 10000", tx.Amount)
	}
	if tx.ReferenceNumber != "2026090nomorref9546085" {
		t.Errorf("ReferenceNumber = %q", tx.ReferenceNumber)
	}
	if tx.Note != "" {
		t.Errorf("Note = %q, want empty", tx.Note)
	}
}

func TestParseSeaBankTransferEmail_WithNote(t *testing.T) {
	body := `Waktu Transaksi

08 Sep 2026 18:01

Jenis Transaksi

Real Time - Sesama Rekening SeaBank

Nama Pengirim

BUDI SANTOSA

Nomor Rekening Pengirim

XXXXXX2022

Jumlah

Rp20.007

No. Referensi

2026090ref123

Catatan

Iuran kas bulan September
`
	tx, err := ParseSeaBankTransferEmail(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Amount != 20007 {
		t.Errorf("Amount = %v, want 20007", tx.Amount)
	}
	if tx.Note != "Iuran kas bulan September" {
		t.Errorf("Note = %q", tx.Note)
	}
}

func TestParseSeaBankTransferEmail_RejectsUnrelatedEmail(t *testing.T) {
	body := `Halo,

Ini adalah promo spesial dari SeaBank buat kamu!
Dapatkan cashback hingga Rp50.000.
`
	if _, err := ParseSeaBankTransferEmail(body); err == nil {
		t.Error("expected error for unrelated (non-transfer) email, got nil")
	}
}
