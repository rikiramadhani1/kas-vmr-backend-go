package domain

import "time"

const (
	TransaksiSourceUpload = "upload" // member uploaded a proof screenshot (OCR)
	TransaksiSourceEmail  = "email"  // auto-confirmed via SeaBank email notification
	TransaksiSourceAdmin  = "admin"  // admin manually recorded (e.g. cash handed in person)
)

// Transaksi records one incoming money transfer that was matched to a
// member's dues, regardless of how it was detected (OCR upload, email
// auto-confirm, or manual admin entry). This is the single source of
// truth for "recent payments" and is what advances a member's Payment
// cursor.
//
// TransactionDate is the date the money actually arrived (parsed from the
// OCR'd receipt or the email notification) - NOT necessarily when this
// row was inserted (CreatedAt). It's used both for display and, crucially,
// for duplicate detection: the same real-world transfer can otherwise
// slip through both the email watcher AND a member's manual upload,
// double-counting the payment - see FindDuplicate in
// TransaksiRepository, which checks (member_id, amount, same calendar
// day as TransactionDate).
type Transaksi struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	MemberID        uint      `gorm:"not null;index" json:"member_id"`
	Member          *Member   `gorm:"foreignKey:MemberID" json:"member,omitempty"`
	Amount          float64   `gorm:"not null" json:"amount"`
	Months          int       `gorm:"not null" json:"months"`
	Source          string    `gorm:"not null" json:"source"`
	ReferenceNumber *string   `gorm:"index" json:"reference_number,omitempty"`
	TransactionDate time.Time `gorm:"not null;index" json:"transaction_date"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Transaksi) TableName() string { return "transaksis" }
