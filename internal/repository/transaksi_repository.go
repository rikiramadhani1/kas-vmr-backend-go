package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/vmr/kas-vmr-backend/internal/domain"
)

type TransaksiRepository interface {
	Create(ctx context.Context, t *domain.Transaksi) error
	// FindDuplicate looks for an existing transaction for the same
	// member, same amount, on the same calendar day as
	// transactionDate - the heuristic used to catch the same real-world
	// transfer being recorded twice (once via the email watcher, once
	// via a member's manual proof upload, or vice versa). Returns nil,
	// nil if no match is found.
	FindDuplicate(ctx context.Context, memberID uint, amount float64, transactionDate time.Time) (*domain.Transaksi, error)
	FindLatestByMemberID(ctx context.Context, memberID uint, limit int) ([]domain.Transaksi, error)
}

type transaksiRepository struct {
	db *gorm.DB
}

func NewTransaksiRepository(db *gorm.DB) TransaksiRepository {
	return &transaksiRepository{db: db}
}

func (r *transaksiRepository) Create(ctx context.Context, t *domain.Transaksi) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *transaksiRepository) FindDuplicate(ctx context.Context, memberID uint, amount float64, transactionDate time.Time) (*domain.Transaksi, error) {
	dayStart := time.Date(transactionDate.Year(), transactionDate.Month(), transactionDate.Day(), 0, 0, 0, 0, transactionDate.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	var t domain.Transaksi
	err := r.db.WithContext(ctx).
		Where("member_id = ? AND amount = ? AND transaction_date >= ? AND transaction_date < ?", memberID, amount, dayStart, dayEnd).
		First(&t).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *transaksiRepository) FindLatestByMemberID(ctx context.Context, memberID uint, limit int) ([]domain.Transaksi, error) {
	var list []domain.Transaksi
	err := r.db.WithContext(ctx).
		Where("member_id = ?", memberID).
		Order("transaction_date desc").
		Limit(limit).
		Find(&list).Error
	return list, err
}
