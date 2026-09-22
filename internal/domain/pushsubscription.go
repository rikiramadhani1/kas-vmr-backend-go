package domain

import "time"

// PushSubscription represents one browser/device's Web Push
// subscription. A member can have several (phone, laptop, etc) - all are
// notified when NotificationUsecase.SendToMember is called.
type PushSubscription struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	MemberID       uint      `gorm:"not null;index;uniqueIndex:idx_push_member_installation" json:"member_id"`
	InstallationID string    `gorm:"not null;size:100;uniqueIndex:idx_push_member_installation" json:"installation_id"`
	Endpoint       string    `gorm:"not null" json:"endpoint"`
	P256dh         string    `gorm:"not null" json:"p256dh"`
	Auth           string    `gorm:"not null" json:"auth"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (PushSubscription) TableName() string { return "push_subscriptions" }
