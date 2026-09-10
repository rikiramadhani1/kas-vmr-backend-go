package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmr/kas-vmr-backend/internal/domain"
)

type PushRepository interface {
	Save(ctx context.Context, sub *domain.PushSubscription) error
	FindByMemberID(ctx context.Context, memberID uint) ([]domain.PushSubscription, error)
	DeleteByEndpoint(ctx context.Context, endpoint string) error
}

type pushRepository struct {
	db *gorm.DB
}

func NewPushRepository(db *gorm.DB) PushRepository {
	return &pushRepository{db: db}
}

// Save upserts by endpoint - kalau member subscribe ulang dari device yang
// sama (misal token refresh), gak dobel row.
func (r *pushRepository) Save(ctx context.Context, sub *domain.PushSubscription) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "endpoint"}},
			DoUpdates: clause.AssignmentColumns([]string{"p256dh", "auth", "member_id"}),
		}).
		Create(sub).Error
}

func (r *pushRepository) FindByMemberID(ctx context.Context, memberID uint) ([]domain.PushSubscription, error) {
	var list []domain.PushSubscription
	err := r.db.WithContext(ctx).Where("member_id = ?", memberID).Find(&list).Error
	return list, err
}

func (r *pushRepository) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	return r.db.WithContext(ctx).Where("endpoint = ?", endpoint).Delete(&domain.PushSubscription{}).Error
}