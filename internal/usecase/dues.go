package usecase

import (
	"fmt"
	"time"

	"github.com/vmr/kas-vmr-backend/internal/domain"
)

// CalculateUnpaidMonths is THE single source of truth for figuring out
// which months a member still owes dues for.
//
// The original Node.js codebase had three separate implementations of
// this same idea, and they did not agree with each other:
//   - kasRepository.getUnpaidMonthsForMember: builds a set of
//     "year-month" strings from *approved* payments and walks
//     month-by-month from the configured start (or last paid + 1) up to
//     the current month, collecting any month missing from the set. This
//     correctly handles gaps (e.g. paid Aug + Oct but not Sep).
//   - payment.service.countPaymentService /
//     findUnpaidMembersService: computed unpaid count via plain
//     arithmetic `(currentYear-startYear)*12 + (currentMonth-startMonth+1)`
//     based only on the *last* approved payment, then subtracted the
//     number of pending payments. This silently assumes any gap in
//     payment history doesn't exist and that pending payments always
//     correspond to the earliest unpaid months - both assumptions break
//     under real-world usage (a member paying out of order, or a pending
//     payment for a future month).
//
// We keep the first (set-based, gap-aware) approach since it's the
// correct one, and use it everywhere dues need to be calculated.
func CalculateUnpaidMonths(approvedPayments []domain.Payment, startMonth, startYear int, now time.Time) (unpaid []domain.MonthYear, lastPaid *domain.MonthYear) {
	approvedSet := make(map[string]struct{}, len(approvedPayments))
	for _, p := range approvedPayments {
		approvedSet[monthKey(p.Year, p.Month)] = struct{}{}
	}

	// Determine lastPaid = most recent approved payment (by year, month).
	for _, p := range approvedPayments {
		if lastPaid == nil || p.Year > lastPaid.Year || (p.Year == lastPaid.Year && p.Month > lastPaid.Month) {
			lastPaid = &domain.MonthYear{Month: p.Month, Year: p.Year}
		}
	}

	currentYear, currentMonthNum := now.Year(), int(now.Month())

	// Already paid up to or beyond the current month - nothing owed.
	if lastPaid != nil && (lastPaid.Year > currentYear || (lastPaid.Year == currentYear && lastPaid.Month >= currentMonthNum)) {
		return []domain.MonthYear{}, lastPaid
	}

	month, year := startMonth, startYear
	if lastPaid != nil {
		month = lastPaid.Month + 1
		year = lastPaid.Year
		if month > 12 {
			month = 1
			year++
		}
	}

	unpaid = []domain.MonthYear{}
	for year < currentYear || (year == currentYear && month <= currentMonthNum) {
		if _, ok := approvedSet[monthKey(year, month)]; !ok {
			unpaid = append(unpaid, domain.MonthYear{Month: month, Year: year})
		}
		month++
		if month > 12 {
			month = 1
			year++
		}
	}

	return unpaid, lastPaid
}

// NextMonthsAfter returns the next n consecutive (month, year) pairs
// following the given starting point - used to extend a payment beyond
// current arrears into "pay ahead" months.
func NextMonthsAfter(month, year, n int) []domain.MonthYear {
	result := make([]domain.MonthYear, 0, n)
	m, y := month, year
	for i := 0; i < n; i++ {
		m++
		if m > 12 {
			m = 1
			y++
		}
		result = append(result, domain.MonthYear{Month: m, Year: y})
	}
	return result
}

func monthKey(year, month int) string {
	return fmt.Sprintf("%d-%02d", year, month)
}

// MonthNameID returns the Indonesian month name for display purposes
// (e.g. "Juli"), matching the original `toLocaleString('id-ID', { month: 'long' })`.
func MonthNameID(month int) string {
	names := []string{
		"Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}
	if month < 1 || month > 12 {
		return ""
	}
	return names[month-1]
}
