// Package paymentcode implements the "nominal unik" (unique-amount) trick
// commonly used by small Indonesian communities/RT-RW kas systems to
// identify a payer without any bank API integration: each member adds a
// small, member-specific code on top of the normal dues amount.
//
// Example: dues = Rp20.000, member #7 transfers Rp20.007 instead of a
// plain Rp20.000. The system can then recover *both* how many months were
// paid *and* which member paid, purely from the transferred amount - no
// OCR, no notification-content guessing, no manual lookup needed.
package paymentcode

import "fmt"

// Encode returns the amount a member should be told to transfer for
// `months` months of dues: months*iuranAmount + memberID.
//
// Requires iuranAmount to be an exact multiple of `base` (validated at
// config-load time - see config.Config.validate) so that Decode can
// always unambiguously split the amount back into (months, memberID).
func Encode(iuranAmount float64, months int, memberID uint) float64 {
	return float64(months)*iuranAmount + float64(memberID)
}

// Decode splits a transferred `amount` back into the member's unique code
// and the number of months paid, given the configured `iuranAmount` and
// code `base` (modulus).
//
// Returns ok=false if the amount doesn't decompose into a whole number of
// months (e.g. it's an unrelated transfer - a donation, a refund, etc.) -
// callers should leave such transactions for manual review rather than
// guessing.
func Decode(amount float64, iuranAmount float64, base int) (memberID uint, months int, ok bool) {
	if iuranAmount <= 0 || base <= 0 {
		return 0, 0, false
	}

	total := int64(amount)
	// Reject non-integer rupiah amounts (shouldn't happen for IDR transfers,
	// but guards against float precision surprises).
	if float64(total) != amount {
		return 0, 0, false
	}

	code := total % int64(base)
	remaining := total - code

	if remaining <= 0 || remaining%int64(iuranAmount) != 0 {
		return 0, 0, false
	}

	return uint(code), int(remaining / int64(iuranAmount)), true
}

// Describe returns a human-readable instruction for a member, e.g.
// "Transfer Rp20.007 (3 bulan, kode unik 007)" - handy for member-facing
// UI/bot messages telling them exactly what to transfer.
func Describe(iuranAmount float64, months int, memberID uint, base int) string {
	amount := Encode(iuranAmount, months, memberID)
	digits := len(fmt.Sprintf("%d", base-1))
	return fmt.Sprintf("Transfer Rp%.0f (%d bulan, kode unik %0*d)", amount, months, digits, memberID)
}
