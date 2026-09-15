package usecase

import (
	"testing"
	"time"
)

func TestAdvanceMonths_FirstTimePayer(t *testing.T) {
	// Never paid before, config start = June 2025, pays 3 months.
	newMonth, newYear, paid := AdvanceMonths(false, 0, 0, 6, 2025, 3)

	if newMonth != 8 || newYear != 2025 {
		t.Fatalf("cursor = %d/%d, want 8/2025", newMonth, newYear)
	}
	want := []struct{ m, y int }{{6, 2025}, {7, 2025}, {8, 2025}}
	if len(paid) != len(want) {
		t.Fatalf("paid = %v, want %d entries", paid, len(want))
	}
	for i, w := range want {
		if paid[i].Month != w.m || paid[i].Year != w.y {
			t.Errorf("paid[%d] = %d/%d, want %d/%d", i, paid[i].Month, paid[i].Year, w.m, w.y)
		}
	}
}

func TestAdvanceMonths_YearRollover(t *testing.T) {
	// Already paid until November 2025, pays 3 more months - should
	// cross into 2026 and update the year correctly.
	newMonth, newYear, paid := AdvanceMonths(true, 11, 2025, 6, 2025, 3)

	if newMonth != 2 || newYear != 2026 {
		t.Fatalf("cursor = %d/%d, want 2/2026", newMonth, newYear)
	}
	want := []struct{ m, y int }{{12, 2025}, {1, 2026}, {2, 2026}}
	for i, w := range want {
		if paid[i].Month != w.m || paid[i].Year != w.y {
			t.Errorf("paid[%d] = %d/%d, want %d/%d", i, paid[i].Month, paid[i].Year, w.m, w.y)
		}
	}
}

func TestCalculateUnpaidMonths_NeverPaid(t *testing.T) {
	now := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	unpaid := CalculateUnpaidMonths(false, 0, 0, 6, 2025, now)

	// June 2025 through March 2026 inclusive = 10 months.
	if len(unpaid) != 10 {
		t.Fatalf("len(unpaid) = %d, want 10: %v", len(unpaid), unpaid)
	}
	if unpaid[0].Month != 6 || unpaid[0].Year != 2025 {
		t.Errorf("first unpaid = %d/%d, want 6/2025", unpaid[0].Month, unpaid[0].Year)
	}
	last := unpaid[len(unpaid)-1]
	if last.Month != 3 || last.Year != 2026 {
		t.Errorf("last unpaid = %d/%d, want 3/2026", last.Month, last.Year)
	}
}

func TestCalculateUnpaidMonths_PaidUpToDate(t *testing.T) {
	now := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	unpaid := CalculateUnpaidMonths(true, 3, 2026, 6, 2025, now)

	if len(unpaid) != 0 {
		t.Fatalf("unpaid = %v, want empty (already paid up to current month)", unpaid)
	}
}

func TestCalculateUnpaidMonths_PaidAhead(t *testing.T) {
	// Paid 2 months ahead of the current month - still nothing owed.
	now := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	unpaid := CalculateUnpaidMonths(true, 5, 2026, 6, 2025, now)

	if len(unpaid) != 0 {
		t.Fatalf("unpaid = %v, want empty (paid ahead)", unpaid)
	}
}

func TestCalculateUnpaidMonths_BehindAcrossYearBoundary(t *testing.T) {
	// Paid until November 2025, now it's February 2026 - owes Dec, Jan, Feb.
	now := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	unpaid := CalculateUnpaidMonths(true, 11, 2025, 6, 2025, now)

	want := []struct{ m, y int }{{12, 2025}, {1, 2026}, {2, 2026}}
	if len(unpaid) != len(want) {
		t.Fatalf("unpaid = %v, want %d entries", unpaid, len(want))
	}
	for i, w := range want {
		if unpaid[i].Month != w.m || unpaid[i].Year != w.y {
			t.Errorf("unpaid[%d] = %d/%d, want %d/%d", i, unpaid[i].Month, unpaid[i].Year, w.m, w.y)
		}
	}
}

// TestAdvanceThenCalculateRoundTrip checks the two functions agree with
// each other: after AdvanceMonths pays N months, CalculateUnpaidMonths
// on the resulting cursor at the same point in time should report zero
// unpaid months.
func TestAdvanceThenCalculateRoundTrip(t *testing.T) {
	newMonth, newYear, _ := AdvanceMonths(false, 0, 0, 6, 2025, 5)
	// AdvanceMonths(never paid, start 6/2025, 5 months) -> paid through Oct 2025.
	now := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)

	unpaid := CalculateUnpaidMonths(true, newMonth, newYear, 6, 2025, now)
	if len(unpaid) != 0 {
		t.Fatalf("unpaid = %v, want empty right after paying through the current month", unpaid)
	}
}
