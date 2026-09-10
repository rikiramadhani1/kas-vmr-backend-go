package domain

import "time"

// EmailTransactionLog records every SeaBank transfer-notification email
// reference number the auto-confirm worker has already acted on, so a
// reprocessed email (e.g. after a restart, or an IMAP \Seen flag that
// didn't stick) never creates a duplicate payment - this table, not IMAP
// flag state, is the actual idempotency guarantee (see
// pkg/mailwatcher.Watcher.processUnseen's doc comment).
type EmailTransactionLog struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ReferenceNumber string    `gorm:"not null;uniqueIndex" json:"reference_number"`
	MemberID        *uint     `json:"member_id,omitempty"`
	Amount          float64   `gorm:"not null" json:"amount"`
	Status          string    `gorm:"not null" json:"status"` // "processed" | "unmatched"
	Note            *string   `json:"note,omitempty"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (EmailTransactionLog) TableName() string { return "email_transaction_logs" }

const (
	EmailTxStatusProcessed = "processed"
	EmailTxStatusUnmatched = "unmatched"
)
