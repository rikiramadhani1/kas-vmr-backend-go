package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/vmr/kas-vmr-backend/internal/domain"
)

type EmailTransactionRepository interface {
	FindByReference(ctx context.Context, ref string) (*domain.EmailTransactionLog, error)
	Create(ctx context.Context, log *domain.EmailTransactionLog) error
	FindUnmatched(ctx context.Context, limit int) ([]domain.EmailTransactionLog, error)
}

type emailTransactionRepository struct {
	db *gorm.DB
}

func NewEmailTransactionRepository(db *gorm.DB) EmailTransactionRepository {
	return &emailTransactionRepository{db: db}
}

func (r *emailTransactionRepository) FindByReference(ctx context.Context, ref string) (*domain.EmailTransactionLog, error) {
	var log domain.EmailTransactionLog
	err := r.db.WithContext(ctx).Where("reference_number = ?", ref).First(&log).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *emailTransactionRepository) Create(ctx context.Context, log *domain.EmailTransactionLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *emailTransactionRepository) FindUnmatched(ctx context.Context, limit int) ([]domain.EmailTransactionLog, error) {
	var list []domain.EmailTransactionLog
	err := r.db.WithContext(ctx).
		Where("status = ?", domain.EmailTxStatusUnmatched).
		Order("created_at desc").
		Limit(limit).
		Find(&list).Error
	return list, err
}
