package domain

import "time"

// PWAInstallation represents one installed PWA instance/device.
// A member can have multiple installations across different devices.
type PWAInstallation struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	MemberID       uint      `gorm:"not null;index;uniqueIndex:idx_pwa_member_installation" json:"member_id"`
	InstallationID string    `gorm:"not null;size:100;uniqueIndex:idx_pwa_member_installation" json:"installation_id"`
	Platform       string    `gorm:"size:30" json:"platform"`
	Browser        string    `gorm:"size:50" json:"browser"`
	InstalledAt    time.Time `gorm:"not null" json:"installed_at"`
	LastSeenAt     time.Time `gorm:"not null" json:"last_seen_at"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (PWAInstallation) TableName() string {
	return "pwa_installations"
}