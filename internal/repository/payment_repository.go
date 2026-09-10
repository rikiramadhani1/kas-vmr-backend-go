package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmr/kas-vmr-backend/internal/domain"
)

type PaymentRepository interface {
	Create(ctx context.Context, p *domain.Payment) error
	CreateMany(ctx context.Context, payments []domain.Payment) error
	FindByID(ctx context.Context, id uint) (*domain.Payment, error)
	// FindByIDForUpdate locks the row (SELECT ... FOR UPDATE) so two
	// concurrent approve/reject calls for the same payment can't both
	// succeed - must be called within a transaction.
	FindByIDForUpdate(ctx context.Context, id uint) (*domain.Payment, error)
	UpdateStatus(ctx context.Context, id uint, status string) error
	FindLastByMemberID(ctx context.Context, memberID uint) (*domain.Payment, error)
	FindPendingByMemberID(ctx context.Context, memberID uint) ([]domain.Payment, error)
	FindApprovedByMemberID(ctx context.Context, memberID uint) ([]domain.Payment, error)
	FindByMemberID(ctx context.Context, memberID uint) ([]domain.Payment, error)
	FindPending(ctx context.Context) ([]domain.Payment, error)
	FindLatestByMemberID(ctx context.Context, memberID uint, limit int) ([]domain.Payment, error)
	FindAll(ctx context.Context) ([]domain.Payment, error)
}

type paymentRepository struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) PaymentRepository {
	return &paymentRepository{db: db}
}

func (r *paymentRepository) Create(ctx context.Context, p *domain.Payment) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *paymentRepository) CreateMany(ctx context.Context, payments []domain.Payment) error {
	if len(payments) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&payments).Error
}

func (r *paymentRepository) FindByID(ctx context.Context, id uint) (*domain.Payment, error) {
	var p domain.Payment
	err := r.db.WithContext(ctx).Preload("Member").First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *paymentRepository) FindByIDForUpdate(ctx context.Context, id uint) (*domain.Payment, error) {
	var p domain.Payment
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Member").
		First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *paymentRepository) UpdateStatus(ctx context.Context, id uint, status string) error {
	return r.db.WithContext(ctx).Model(&domain.Payment{}).Where("id = ?", id).Update("status", status).Error
}

func (r *paymentRepository) FindLastByMemberID(ctx context.Context, memberID uint) (*domain.Payment, error) {
	var p domain.Payment
	err := r.db.WithContext(ctx).
		Where("member_id = ?", memberID).
		Order("year desc, month desc").
		First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *paymentRepository) FindPendingByMemberID(ctx context.Context, memberID uint) ([]domain.Payment, error) {
	var list []domain.Payment
	err := r.db.WithContext(ctx).
		Where("member_id = ? AND status = ?", memberID, domain.PaymentStatusPending).
		Find(&list).Error
	return list, err
}

func (r *paymentRepository) FindApprovedByMemberID(ctx context.Context, memberID uint) ([]domain.Payment, error) {
	var list []domain.Payment
	err := r.db.WithContext(ctx).
		Where("member_id = ? AND status = ?", memberID, domain.PaymentStatusApproved).
		Order("year asc, month asc").
		Find(&list).Error
	return list, err
}

func (r *paymentRepository) FindByMemberID(ctx context.Context, memberID uint) ([]domain.Payment, error) {
	var list []domain.Payment
	err := r.db.WithContext(ctx).
		Where("member_id = ?", memberID).
		Order("year asc, month asc").
		Find(&list).Error
	return list, err
}

func (r *paymentRepository) FindPending(ctx context.Context) ([]domain.Payment, error) {
	var list []domain.Payment
	err := r.db.WithContext(ctx).
		Where("status = ?", domain.PaymentStatusPending).
		Order("created_at asc").
		Preload("Member").
		Find(&list).Error
	return list, err
}

func (r *paymentRepository) FindLatestByMemberID(ctx context.Context, memberID uint, limit int) ([]domain.Payment, error) {
	var list []domain.Payment
	err := r.db.WithContext(ctx).
		Where("member_id = ?", memberID).
		Order("created_at desc").
		Limit(limit).
		Find(&list).Error
	return list, err
}

func (r *paymentRepository) FindAll(ctx context.Context) ([]domain.Payment, error) {
	var list []domain.Payment
	err := r.db.WithContext(ctx).Order("created_at desc").Find(&list).Error
	return list, err
}
