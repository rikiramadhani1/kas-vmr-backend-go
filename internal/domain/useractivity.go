package domain

import "time"

// UserActivity mirrors the Prisma `UserActivity` model, used for the
// analytics module (WAU + per-member activity breakdown).
type UserActivity struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	MemberID  uint      `gorm:"not null;index" json:"member_id"`
	Action    string    `gorm:"not null;index" json:"action"`
	Feature   string    `gorm:"not null" json:"feature"`
	Metadata  JSONMap   `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (UserActivity) TableName() string { return "user_activities" }
