package domain

import "time"

// LogSignTf mirrors the Prisma `LogSignTf` model - stores a SHA-256
// signature hash of every OCR'd payment proof to detect and reject
// duplicate/replayed transfer screenshots.
type LogSignTf struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	MemberID      uint      `gorm:"not null;index" json:"member_id"`
	Amount        float64   `gorm:"not null" json:"amount"`
	SignatureHash string    `gorm:"not null;uniqueIndex" json:"signature_hash"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (LogSignTf) TableName() string { return "LogSignTf" }
