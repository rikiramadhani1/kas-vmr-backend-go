package usecase

import (
	"time"

	"github.com/vmr/kas-vmr-backend/internal/domain"
)

// CalculateUnpaidMonths lists every month a member owes dues for, given
// their current "paid until" cursor (see domain.Payment).
//
// This used to operate on a whole list of per-month Payment rows and
// build a set to find gaps (see the git history / old README notes on
// why THAT existed - it was working around a pending/approved workflow
// where months could theoretically be approved out of order). Now that
// every transaction advances a single cursor by N consecutive months
// starting right after wherever it currently sits, gaps can't happen by
// construction - so this is now a plain walk from (cursor+1, or the
// configured start if the member has never paid) up to the current
// month.
func CalculateUnpaidMonths(hasPaid bool, cursorMonth, cursorYear int, defaultStartMonth, defaultStartYear int, now time.Time) []domain.MonthYear {
	currentYear, currentMonthNum := now.Year(), int(now.Month())

	month, year := defaultStartMonth, defaultStartYear
	if hasPaid {
		// Already paid up to or beyond the current month - nothing owed.
		if cursorYear > currentYear || (cursorYear == currentYear && cursorMonth >= currentMonthNum) {
			return []domain.MonthYear{}
		}
		month, year = cursorMonth+1, cursorYear
		if month > 12 {
			month = 1
			year++
		}
	}

	unpaid := []domain.MonthYear{}
	for year < currentYear || (year == currentYear && month <= currentMonthNum) {
		unpaid = append(unpaid, domain.MonthYear{Month: month, Year: year})
		month++
		if month > 12 {
			month = 1
			year++
		}
	}
	return unpaid
}

// AdvanceMonths returns the cursor position after paying `months`
// consecutive months starting right after (fromMonth, fromYear) - or
// starting at (defaultStartMonth, defaultStartYear) if the member has
// never paid before (hasPaid == false).
//
// Also returns the full list of (month, year) pairs being paid, in
// order - needed by the caller to book each one into cash flow with the
// correct month bucket.
func AdvanceMonths(hasPaid bool, fromMonth, fromYear int, defaultStartMonth, defaultStartYear, months int) (newMonth, newYear int, paidMonths []domain.MonthYear) {
	month, year := defaultStartMonth, defaultStartYear
	if hasPaid {
		month, year = fromMonth+1, fromYear
		if month > 12 {
			month = 1
			year++
		}
	}

	paidMonths = make([]domain.MonthYear, 0, months)
	for i := 0; i < months; i++ {
		paidMonths = append(paidMonths, domain.MonthYear{Month: month, Year: year})
		if i < months-1 {
			month++
			if month > 12 {
				month = 1
				year++
			}
		}
	}

	return month, year, paidMonths
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
