package repository

import (
	"context"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PWARepository interface {
	Upsert(ctx context.Context, installation *domain.PWAInstallation) error
	FindByMemberID(ctx context.Context, memberID uint) ([]domain.PWAInstallation, error)
	FindByMemberAndInstallationID(ctx context.Context, memberID uint, installationID string) (*domain.PWAInstallation, error)
}

type pwaRepository struct {
	db *gorm.DB
}

func NewPWARepository(db *gorm.DB) PWARepository {
	return &pwaRepository{db: db}
}

// Upsert menggunakan kombinasi member_id + installation_id.
// Installation yang sama tidak akan membuat row baru.
func (r *pwaRepository) Upsert(
	ctx context.Context,
	installation *domain.PWAInstallation,
) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "member_id"},
				{Name: "installation_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"platform",
				"browser",
				"last_seen_at",
				"updated_at",
			}),
		}).
		Create(installation).Error
}

func (r *pwaRepository) FindByMemberID(
	ctx context.Context,
	memberID uint,
) ([]domain.PWAInstallation, error) {
	var list []domain.PWAInstallation

	err := r.db.WithContext(ctx).
		Where("member_id = ?", memberID).
		Order("last_seen_at DESC").
		Find(&list).Error

	return list, err
}

func (r *pwaRepository) FindByMemberAndInstallationID(
	ctx context.Context,
	memberID uint,
	installationID string,
) (*domain.PWAInstallation, error) {
	var installation domain.PWAInstallation

	err := r.db.WithContext(ctx).
		Where(
			"member_id = ? AND installation_id = ?",
			memberID,
			installationID,
		).
		First(&installation).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}

		return nil, err
	}

	return &installation, nil
}