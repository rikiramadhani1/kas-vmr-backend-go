package domain

import "time"

const (
	CashFlowTypeIn  = "in"
	CashFlowTypeOut = "out"

	CashFlowSourceDues = "dues"
)

// CashFlow mirrors the Prisma `CashFlow` model.
//
// NOTE on the `id` field: the original Node.js code manually computed the
// next ID via `aggregate(max(id)) + 1` before inserting, to keep IDs
// "nice" for a couple of upsert-by-description lookups. That is a classic
// race condition under concurrent requests (two requests can compute the
// same "next" ID and collide, or skip one). Here we let Postgres'
// auto-increment sequence handle ID generation, which is safe under
// concurrency; upsert-by-description flows use a row lock instead (see
// internal/usecase/cashflow.go and internal/usecase/payment.go).
type CashFlow struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Type        string    `gorm:"not null" json:"type"`
	Source      string    `gorm:"not null" json:"source"`
	Amount      float64   `gorm:"not null" json:"amount"`
	Description *string   `json:"description,omitempty"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (CashFlow) TableName() string { return "cash_flows" }
