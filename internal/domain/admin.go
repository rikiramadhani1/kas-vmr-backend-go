package domain

import "time"

// Role constants shared by Admin and the JWT payload's `role` claim.
const (
	RoleMember    = "member"
	RoleAdmin     = "admin"
	RoleBendahara = "bendahara"
)

// Admin mirrors the Prisma `Admin` model.
type Admin struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"`
	Role      string    `gorm:"not null;default:admin" json:"role"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Admin) TableName() string { return "Admin" }
