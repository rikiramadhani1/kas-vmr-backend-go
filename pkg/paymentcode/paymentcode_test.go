package paymentcode

import "testing"

func TestEncodeDecodeRoundTrip(t *testing.T) {
	iuranAmount := 20000.0
	base := 1000

	cases := []struct {
		memberID uint
		months   int
	}{
		{memberID: 7, months: 1},
		{memberID: 42, months: 3},
		{memberID: 999, months: 1},
		{memberID: 1, months: 12},
	}

	for _, tc := range cases {
		amount := Encode(iuranAmount, tc.months, tc.memberID)
		gotMemberID, gotMonths, ok := Decode(amount, iuranAmount, base)
		if !ok {
			t.Fatalf("Decode(%v) returned ok=false, want true", amount)
		}
		if gotMemberID != tc.memberID {
			t.Errorf("Decode(%v).memberID = %d, want %d", amount, gotMemberID, tc.memberID)
		}
		if gotMonths != tc.months {
			t.Errorf("Decode(%v).months = %d, want %d", amount, gotMonths, tc.months)
		}
	}
}

func TestDecodeRejectsUnrelatedAmounts(t *testing.T) {
	iuranAmount := 20000.0
	base := 1000

	// A donation or unrelated transfer that doesn't decompose into a whole
	// number of months should be rejected, not silently misattributed.
	unrelatedAmounts := []float64{1, 500, 15000, 99999999}

	for _, amount := range unrelatedAmounts {
		if _, _, ok := Decode(amount, iuranAmount, base); ok {
			t.Errorf("Decode(%v) = ok, want rejected as unrelated", amount)
		}
	}
}

func TestDecodeRejectsNonIntegerAmount(t *testing.T) {
	if _, _, ok := Decode(20007.5, 20000, 1000); ok {
		t.Error("Decode should reject non-integer rupiah amounts")
	}
}
