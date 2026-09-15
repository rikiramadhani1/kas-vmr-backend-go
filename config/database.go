package config

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/vmr/kas-vmr-backend/internal/domain"
)

// NewDatabase opens a GORM/postgres connection and configures the
// underlying connection pool. It does NOT run AutoMigrate by default in
// production - see RunMigrations below, which is called explicitly from
// main.go so migrations are an intentional, visible step.
func NewDatabase(cfg *Config) (*gorm.DB, error) {
	gormLogLevel := logger.Warn
	if cfg.AppEnv == "development" {
		gormLogLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
		NowFunc: func() time.Time {
			// Standardize all DB timestamps on UTC to avoid the timezone
			// drift issues this project has run into before.
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}

// RunMigrations applies GORM AutoMigrate for all domain models. Mirrors the
// tables defined in the original Prisma schema.prisma.
func RunMigrations(db *gorm.DB) error {
	log.Println("Running database migrations...")
	err := db.AutoMigrate(
		&domain.Member{},
		&domain.Payment{},
		&domain.CashFlow{},
		&domain.Admin{},
		&domain.ChatLog{},
		&domain.UserActivity{},
		&domain.LogSignTf{},
		&domain.WeeklySummary{},
		&domain.EmailTransactionLog{},
		&domain.Transaksi{},
		&domain.PushSubscription{},
	)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	log.Println("Migrations complete.")
	return nil
}
