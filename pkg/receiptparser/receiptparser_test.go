package receiptparser

import (
	"testing"
	"time"
)

func TestParseReceiptDate(t *testing.T) {
	loc := time.UTC

	cases := []struct {
		raw  string
		want time.Time
	}{
		{"05/07/2025 14:32", time.Date(2025, 7, 5, 14, 32, 0, 0, loc)},
		{"05-07-2025 14:32:10", time.Date(2025, 7, 5, 14, 32, 10, 0, loc)},
		{"05/07/25 09:00", time.Date(2025, 7, 5, 9, 0, 0, 0, loc)},
	}

	for _, c := range cases {
		got, err := ParseReceiptDate(c.raw, loc)
		if err != nil {
			t.Errorf("ParseReceiptDate(%q) error: %v", c.raw, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("ParseReceiptDate(%q) = %v, want %v", c.raw, got, c.want)
		}
	}
}

func TestParseReceiptDate_Unrecognized(t *testing.T) {
	if _, err := ParseReceiptDate("not a date", time.UTC); err == nil {
		t.Error("expected error for unrecognized date format")
	}
}
