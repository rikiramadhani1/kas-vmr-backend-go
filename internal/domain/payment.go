package domain

import "time"

// Payment is now a per-member CURSOR, not a per-month record.
//
// This replaced the original design (one Payment row per month, with a
// pending/approved/rejected workflow) after removing manual
// approve/reject entirely - every incoming transfer is now auto-recorded
// via Transaksi (either OCR upload or the email auto-confirm worker), and
// immediately advances this cursor. There's no more "pending" state to
// track, so a single row per member showing "paid up through this
// month/year" is all that's needed - no amount field, since the amount
// actually paid lives on the Transaksi row that advanced the cursor.
type Payment struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	MemberID       uint      `gorm:"not null;uniqueIndex" json:"member_id"`
	Member         *Member   `gorm:"foreignKey:MemberID" json:"member,omitempty"`
	PaidUntilMonth int       `gorm:"not null;default:0" json:"paid_until_month"`
	PaidUntilYear  int       `gorm:"not null;default:0" json:"paid_until_year"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Payment) TableName() string { return "payments" }

// HasPaidAnything reports whether this cursor has ever been advanced
// (PaidUntilMonth/Year default to 0 for a member who's never paid).
func (p *Payment) HasPaidAnything() bool {
	return p.PaidUntilMonth > 0 && p.PaidUntilYear > 0
}

// MonthYear is a small value type used throughout the dues-calculation
// logic (see internal/usecase/dues.go).
type MonthYear struct {
	Month int `json:"month"`
	Year  int `json:"year"`
}
