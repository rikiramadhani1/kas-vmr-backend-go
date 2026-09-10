package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/vmr/kas-vmr-backend/internal/domain"
)

type LogSignTfRepository interface {
	FindBySignatureHash(ctx context.Context, hash string) (*domain.LogSignTf, error)
	Create(ctx context.Context, memberID uint, amount float64, hash string) error
}

type logSignTfRepository struct {
	db *gorm.DB
}

func NewLogSignTfRepository(db *gorm.DB) LogSignTfRepository {
	return &logSignTfRepository{db: db}
}

func (r *logSignTfRepository) FindBySignatureHash(ctx context.Context, hash string) (*domain.LogSignTf, error) {
	var log domain.LogSignTf
	err := r.db.WithContext(ctx).Where("signature_hash = ?", hash).First(&log).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *logSignTfRepository) Create(ctx context.Context, memberID uint, amount float64, hash string) error {
	return r.db.WithContext(ctx).Create(&domain.LogSignTf{
		MemberID:      memberID,
		Amount:        amount,
		SignatureHash: hash,
	}).Error
}
