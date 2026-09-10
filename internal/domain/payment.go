package domain

import "time"

const (
	PaymentStatusPending  = "pending"
	PaymentStatusApproved = "approved"
	PaymentStatusRejected = "rejected"
)

// Payment mirrors the Prisma `Payment` model - one row represents a single
// month's dues (iuran) for a member.
type Payment struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	MemberID  uint      `gorm:"not null;index" json:"member_id"`
	Member    *Member   `gorm:"foreignKey:MemberID" json:"member,omitempty"`
	Month     int       `gorm:"not null" json:"month"`
	Year      int       `gorm:"not null" json:"year"`
	Amount    float64   `gorm:"not null" json:"amount"`
	Status    string    `gorm:"not null;default:pending;index" json:"status"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Payment) TableName() string { return "Payment" }

// MonthYear is a small value type used throughout the dues-calculation
// logic (see internal/usecase/dues.go), unifying what used to be three
// separate, slightly-inconsistent implementations in the Node.js codebase.
type MonthYear struct {
	Month int `json:"month"`
	Year  int `json:"year"`
}
