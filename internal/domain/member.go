package domain

import "time"

const (
	MemberStatusActive   = "active"
	MemberStatusInactive = "inactive"
)

// Member mirrors the Prisma `Member` model.
type Member struct {
	ID                uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name              string    `gorm:"not null" json:"name"`
	PhoneNumber       string    `gorm:"uniqueIndex;not null" json:"phone_number"`
	SpousePhoneNumber *string   `json:"spouse_phone_number,omitempty"`
	BankAccountSuffix *string 	`json:"-"`
	Pin               *string   `json:"-"` // never serialize the PIN hash
	HouseNumber       *string   `json:"house_number,omitempty"`
	Status            string    `gorm:"not null;default:active" json:"status"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`

	Payments       []Payment      `gorm:"foreignKey:MemberID" json:"-"`
	ChatLogs       []ChatLog      `gorm:"foreignKey:MemberID" json:"-"`
	UserActivities []UserActivity `gorm:"foreignKey:MemberID" json:"-"`
	LogSignTfs     []LogSignTf    `gorm:"foreignKey:MemberID" json:"-"`
}

func (Member) TableName() string { return "Member" }
