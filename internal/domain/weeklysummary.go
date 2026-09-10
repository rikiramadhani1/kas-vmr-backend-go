package domain

import "time"

// WeeklySummary mirrors the Prisma `WeeklySummary` model. It exists in the
// original schema but is never written to or read from by any of the
// Node.js code we were given, so no repository/usecase wires it up here
// either - it's included purely so the Go schema stays 1:1 with Prisma's,
// in case a future feature needs it.
type WeeklySummary struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	WeekStart time.Time `gorm:"not null" json:"week_start"`
	WAU       int       `gorm:"not null" json:"wau"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (WeeklySummary) TableName() string { return "weekly_summaries" }
