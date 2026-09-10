package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmr/kas-vmr-backend/internal/domain"
)

type SaldoResult struct {
	Saldo    float64
	TotalIn  float64
	TotalOut float64
}

type GroupedCashFlow struct {
	Year         string
	Month        string
	Transactions []domain.CashFlow
}

type CashFlowRepository interface {
	Create(ctx context.Context, cf *domain.CashFlow) error
	GetSaldo(ctx context.Context, all bool) (*SaldoResult, error)
	GetByYear(ctx context.Context, year *int) ([]domain.CashFlow, error)
	GetLatest(ctx context.Context, limit int) ([]domain.CashFlow, error)
	// UpsertByDescription atomically adds amountToAdd to an existing
	// CashFlow row matching `description`, or creates a new one if none
	// exists.
	//
	// IMPORTANT: to actually be atomic under concurrency, this must be
	// called with a repository constructed on a *gorm.DB that is itself a
	// transaction (see db.Transaction(...) in the usecase layer) - the row
	// lock (`FOR UPDATE`) it takes is only meaningful inside a
	// transaction. This replaces the original Node.js
	// find-then-conditionally-update-or-create logic, which raced under
	// concurrent approvals for the same month.
	UpsertByDescription(ctx context.Context, cfType, source, description string, amountToAdd float64) (*domain.CashFlow, error)
}

type cashFlowRepository struct {
	db *gorm.DB
}

func NewCashFlowRepository(db *gorm.DB) CashFlowRepository {
	return &cashFlowRepository{db: db}
}

func (r *cashFlowRepository) Create(ctx context.Context, cf *domain.CashFlow) error {
	return r.db.WithContext(ctx).Create(cf).Error
}

func (r *cashFlowRepository) GetSaldo(ctx context.Context, all bool) (*SaldoResult, error) {
	db := r.db.WithContext(ctx)

	if !all {
		now := time.Now().UTC()
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0).Add(-time.Nanosecond)
		db = db.Where("created_at BETWEEN ? AND ?", start, end)
	}

	var totalIn, totalOut float64
	if err := db.Session(&gorm.Session{}).Model(&domain.CashFlow{}).
		Where("type = ?", domain.CashFlowTypeIn).
		Select("COALESCE(SUM(amount), 0)").Scan(&totalIn).Error; err != nil {
		return nil, err
	}
	if err := db.Session(&gorm.Session{}).Model(&domain.CashFlow{}).
		Where("type = ?", domain.CashFlowTypeOut).
		Select("COALESCE(SUM(amount), 0)").Scan(&totalOut).Error; err != nil {
		return nil, err
	}

	return &SaldoResult{
		Saldo:    totalIn - totalOut,
		TotalIn:  totalIn,
		TotalOut: totalOut,
	}, nil
}

func (r *cashFlowRepository) GetByYear(ctx context.Context, year *int) ([]domain.CashFlow, error) {
	var list []domain.CashFlow
	db := r.db.WithContext(ctx).Order("created_at desc")
	if year != nil {
		start := time.Date(*year, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(*year, 12, 31, 23, 59, 59, 999999999, time.UTC)
		db = db.Where("created_at BETWEEN ? AND ?", start, end)
	}
	err := db.Find(&list).Error
	return list, err
}

func (r *cashFlowRepository) GetLatest(ctx context.Context, limit int) ([]domain.CashFlow, error) {
	var list []domain.CashFlow
	err := r.db.WithContext(ctx).Order("created_at desc").Limit(limit).Find(&list).Error
	return list, err
}

func (r *cashFlowRepository) UpsertByDescription(ctx context.Context, cfType, source, description string, amountToAdd float64) (*domain.CashFlow, error) {
	var existing domain.CashFlow
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("description = ?", description).
		First(&existing).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		newCF := &domain.CashFlow{
			Type:        cfType,
			Source:      source,
			Amount:      amountToAdd,
			Description: &description,
		}
		if err := r.db.WithContext(ctx).Create(newCF).Error; err != nil {
			return nil, err
		}
		return newCF, nil
	}
	if err != nil {
		return nil, err
	}

	existing.Amount += amountToAdd
	if err := r.db.WithContext(ctx).Model(&existing).Update("amount", existing.Amount).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}
