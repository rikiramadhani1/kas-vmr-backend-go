package domain

import "time"

// ChatLog mirrors the Prisma `ChatLog` model - used by the WhatsApp bot
// (not part of this REST API phase) to record inbound messages and
// enforce the daily rate limit.
type ChatLog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	MemberID  *uint     `gorm:"index" json:"member_id,omitempty"`
	Member    *Member   `gorm:"foreignKey:MemberID" json:"-"`
	Phone     string    `gorm:"not null;index" json:"phone"`
	Message   *string   `json:"message,omitempty"`
	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (ChatLog) TableName() string { return "chat_logs" }
