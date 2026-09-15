// Package receiptparser extracts structured data (nominal amount, a
// signature for duplicate detection) out of raw OCR text from a payment
// proof screenshot. Ported line-for-line in spirit from the regex-based
// logic in the original `createPaymentByProofService`.
package receiptparser

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	whitespaceRe   = regexp.MustCompile(`\s+`)
	nonAlphaRe     = regexp.MustCompile(`[^a-z]`)
	nominalLabelRe = regexp.MustCompile(`(?i)nominal\s+(?:rp|idr)?\s*([\d.,]+)`)
	currencyRe     = regexp.MustCompile(`(?i)(?:rp|idr)\s*([\d.,]+)`)
	dateRe         = regexp.MustCompile(`(\d{1,2}[/\-]\d{1,2}(?:[/\-]\d{2,4})?\s+\d{1,2}:\d{2}(?::\d{2})?)`)
)

// CleanText collapses whitespace, matching the original `cleanedText`.
func CleanText(raw string) string {
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(raw, " "))
}

// ContainsName checks whether the OCR'd text mentions the given name,
// ignoring case, spacing, and punctuation (e.g. "Riki Ramadhani" matches
// "RIKI RAMADHANI" or "riki  ramadhani"). The name to check against is
// configuration (BENDAHARA_NAME_KEYWORD), not hardcoded, unlike the
// original implementation.
func ContainsName(cleanedText, name string) bool {
	if name == "" {
		return true // treasurer-name check disabled if not configured
	}
	normalizedText := nonAlphaRe.ReplaceAllString(strings.ToLower(cleanedText), "")
	normalizedName := nonAlphaRe.ReplaceAllString(strings.ToLower(name), "")
	return strings.Contains(normalizedText, normalizedName)
}

// normalizeCurrency converts a raw matched number string (which may use
// either Indonesian "20.000,00" or US "20,000.00" thousand/decimal
// separators) into a float64.
func normalizeCurrency(raw string) (float64, error) {
	value := strings.TrimSpace(raw)

	switch {
	case strings.Contains(value, ",") && strings.Contains(value, "."):
		// US format: 20,000.00
		value = strings.ReplaceAll(value, ",", "")
	case strings.Contains(value, ","):
		// Indonesian decimal comma: 20000,00
		value = strings.ReplaceAll(value, ".", "")
		value = strings.Replace(value, ",", ".", 1)
	default:
		// Indonesian thousands dot: 20.000
		value = strings.ReplaceAll(value, ".", "")
	}

	return strconv.ParseFloat(value, 64)
}

// ExtractNominal finds the transferred amount in cleaned OCR text. It
// first looks for an explicit "Nominal: Rp ..." label; if that's absent,
// it falls back to the largest Rp/IDR-prefixed number found anywhere in
// the text (since receipts often show the amount more than once, e.g.
// subtotal + admin fee + total).
func ExtractNominal(cleanedText string) (float64, error) {
	if m := nominalLabelRe.FindStringSubmatch(cleanedText); m != nil {
		v, err := normalizeCurrency(m[1])
		if err == nil && v > 0 {
			return v, nil
		}
	}

	matches := currencyRe.FindAllStringSubmatch(cleanedText, -1)
	var maxValue float64
	for _, m := range matches {
		v, err := normalizeCurrency(m[1])
		if err == nil && v > maxValue {
			maxValue = v
		}
	}

	if maxValue <= 0 {
		return 0, fmt.Errorf("nominal pembayaran tidak terbaca")
	}
	return maxValue, nil
}

// ExtractDate returns the first date/time-like substring found in the
// text (e.g. "05/07/2025 14:32"), or "" if none found.
func ExtractDate(cleanedText string) string {
	if m := dateRe.FindString(cleanedText); m != "" {
		return strings.TrimSpace(m)
	}
	return ""
}

// receiptDateLayouts covers the date/time formats Indonesian banking
// apps commonly print on a transfer receipt screenshot. Tried in order;
// the first one that parses wins.
var receiptDateLayouts = []string{
	"02/01/2006 15:04:05",
	"02/01/2006 15:04",
	"02-01-2006 15:04:05",
	"02-01-2006 15:04",
	"02/01/06 15:04:05",
	"02/01/06 15:04",
	"02-01-06 15:04:05",
	"02-01-06 15:04",
}

// yearlessDateLayouts is tried when the receipt only shows day/month
// with no year - the current year is assumed (see ParseReceiptDate).
var yearlessDateLayouts = []string{
	"02/01 15:04:05",
	"02/01 15:04",
	"02-01 15:04:05",
	"02-01 15:04",
}

// ParseReceiptDate converts the raw string returned by ExtractDate into
// a time.Time, in the given location (pass the same location used
// elsewhere for "now", e.g. Asia/Jakarta) - used so a payment-proof
// screenshot's actual transfer date/time can be compared for duplicate
// detection (see TransaksiRepository.FindDuplicate) instead of relying
// on whenever the upload happened to be processed.
func ParseReceiptDate(raw string, loc *time.Location) (time.Time, error) {
	for _, layout := range receiptDateLayouts {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return t, nil
		}
	}
	for _, layout := range yearlessDateLayouts {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			now := time.Now().In(loc)
			return time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc), nil
		}
	}
	return time.Time{}, fmt.Errorf("format tanggal %q tidak dikenali", raw)
}

// Signature builds a stable dedup fingerprint from the nominal, the
// extracted date, and the first 120 characters of the cleaned OCR text -
// the same composition the original code hashed with SHA-256.
func Signature(nominal float64, dateStr, cleanedText string) string {
	snippet := cleanedText
	if len(snippet) > 120 {
		snippet = snippet[:120]
	}
	key := fmt.Sprintf("%v|%s|%s", nominal, dateStr, snippet)
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
