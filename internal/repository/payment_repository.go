package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmr/kas-vmr-backend/internal/domain"
)

// PaymentRepository manages the per-member "paid until" cursor. See
// domain.Payment's doc comment for why this is a single row per member
// rather than one row per month.
type PaymentRepository interface {
	FindByMemberID(ctx context.Context, memberID uint) (*domain.Payment, error)
	// FindByMemberIDForUpdate locks the member's cursor row (SELECT ...
	// FOR UPDATE) so two concurrent transactions for the same member
	// can't both read the same starting point and advance from it
	// independently (which would silently drop one of the payments'
	// worth of months). Must be called within a transaction.
	FindByMemberIDForUpdate(ctx context.Context, memberID uint) (*domain.Payment, error)
	// AdvanceCursor sets the member's cursor to (month, year), creating
	// the row if it doesn't exist yet.
	AdvanceCursor(ctx context.Context, memberID uint, month, year int) error
	FindAllActive(ctx context.Context) ([]domain.Payment, error)
}

type paymentRepository struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) PaymentRepository {
	return &paymentRepository{db: db}
}

func (r *paymentRepository) FindByMemberID(ctx context.Context, memberID uint) (*domain.Payment, error) {
	var p domain.Payment
	err := r.db.WithContext(ctx).Where("member_id = ?", memberID).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *paymentRepository) FindByMemberIDForUpdate(ctx context.Context, memberID uint) (*domain.Payment, error) {
	var p domain.Payment
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("member_id = ?", memberID).
		First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *paymentRepository) AdvanceCursor(ctx context.Context, memberID uint, month, year int) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "member_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"paid_until_month", "paid_until_year", "updated_at"}),
		}).
		Create(&domain.Payment{
			MemberID:       memberID,
			PaidUntilMonth: month,
			PaidUntilYear:  year,
		}).Error
}

// FindAllActive returns every member's cursor row - used by
// FindUnpaidMembers to compute who's behind. Members who have never paid
// at all won't have a row here; callers should treat "no row" the same
// as "cursor at zero".
func (r *paymentRepository) FindAllActive(ctx context.Context) ([]domain.Payment, error) {
	var list []domain.Payment
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}
