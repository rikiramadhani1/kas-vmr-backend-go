// Package mailwatcher watches a Gmail inbox (via IMAP IDLE) for SeaBank
// "transfer masuk" notification emails and turns them into structured
// transaction data the payment usecase can auto-confirm against.
package mailwatcher

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Transaction is the structured data extracted from one SeaBank
// "transfer masuk" notification email.
type Transaction struct {
	Time            string  // as printed, e.g. "08 Sep 2026 18:01"
	Type            string  // e.g. "Real Time - Sesama Rekening SeaBank"
	SenderName      string  // e.g. "SI PENGIRIM"
	SenderAccount   string  // masked, e.g. "XXXXXX1011"
	Amount          float64 // parsed from "Rp10.000" -> 10000
	ReferenceNumber string  // e.g. "2026090nomorref9546085" - used for dedup
	Note            string  // "Catatan" field, often empty
}

// labelOrder mirrors the exact field labels SeaBank uses in the
// notification email body (confirmed against a real sample). The parser
// is line-based rather than one big regex: each label is expected to sit
// alone on its own line, with the value as the next non-blank line - this
// is far more robust to minor whitespace/HTML-to-text differences than
// trying to match the whole body with a single pattern.
var labelOrder = []string{
	"Waktu Transaksi",
	"Jenis Transaksi",
	"Nama Pengirim",
	"Nomor Rekening Pengirim",
	"Jumlah",
	"No. Referensi",
	"Catatan",
}

var currencyDigitsRe = regexp.MustCompile(`[^\d]`)

// ParseSeaBankTransferEmail extracts a Transaction from the plain-text
// body of a SeaBank "transfer masuk" notification email.
//
// Returns an error if the email doesn't look like a transfer notification
// at all (e.g. missing "Jumlah"/"Nomor Rekening Pengirim" - a different
// SeaBank email type, or the format changed) so the caller can safely
// skip/log it instead of silently mis-processing an unrelated email.
func ParseSeaBankTransferEmail(body string) (*Transaction, error) {
	rawLines := strings.Split(body, "\n")

	values := make(map[string]string, len(labelOrder))
	for i, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if !isLabel(trimmed) {
			continue
		}
		if v, ok := valueAfterLabel(rawLines, i); ok {
			values[trimmed] = v
		}
	}

	// "Jumlah" and "Nomor Rekening Pengirim" are the two fields that
	// reliably distinguish this email type from other SeaBank emails
	// (statements, promos, OTP, etc.) - if either is missing, this isn't
	// a transfer-masuk notification we know how to handle.
	amountRaw, hasAmount := values["Jumlah"]
	_, hasAccount := values["Nomor Rekening Pengirim"]
	if !hasAmount || !hasAccount {
		return nil, fmt.Errorf("email tidak dikenali sebagai notifikasi transfer masuk SeaBank")
	}

	amount, err := parseRupiah(amountRaw)
	if err != nil {
		return nil, fmt.Errorf("gagal parse nominal %q: %w", amountRaw, err)
	}

	return &Transaction{
		Time:            values["Waktu Transaksi"],
		Type:            values["Jenis Transaksi"],
		SenderName:      values["Nama Pengirim"],
		SenderAccount:   values["Nomor Rekening Pengirim"],
		Amount:          amount,
		ReferenceNumber: values["No. Referensi"],
		Note:            values["Catatan"],
	}, nil
}

// valueAfterLabel looks at the raw (unfiltered) lines right after a label
// at index i and returns its value.
//
// SeaBank's HTML template renders a variable number of blank lines
// between "Label" and "Value" after HTML-tag stripping (each closing
// tag becomes a newline) - so the separator is "one or more blank
// lines", not a fixed count. A field is considered genuinely empty only
// when the next non-blank line is itself a known label (i.e. there was
// no value line at all before the next field started).
func valueAfterLabel(lines []string, labelIdx int) (string, bool) {
	j := labelIdx + 1
	for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
		j++
	}
	if j >= len(lines) {
		return "", false
	}

	next := strings.TrimSpace(lines[j])
	if isLabel(next) {
		return "", false
	}
	return next, true
}

// parseRupiah converts "Rp10.000" -> 10000. SeaBank uses "." as the
// thousands separator and no decimal part for whole-rupiah amounts (which
// all IDR transfers are).
func parseRupiah(raw string) (float64, error) {
	digitsOnly := currencyDigitsRe.ReplaceAllString(raw, "")
	if digitsOnly == "" {
		return 0, fmt.Errorf("tidak ada digit ditemukan")
	}
	n, err := strconv.ParseInt(digitsOnly, 10, 64)
	if err != nil {
		return 0, err
	}
	return float64(n), nil
}

func isLabel(line string) bool {
	for _, l := range labelOrder {
		if line == l {
			return true
		}
	}
	return false
}
