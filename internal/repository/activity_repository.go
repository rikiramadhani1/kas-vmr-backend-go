package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/vmr/kas-vmr-backend/internal/domain"
)

type ActivityFilter struct {
	StartDate *time.Time
	EndDate   *time.Time
	Action    string
	MemberID  *uint
}

type ActivityRepository interface {
	Log(ctx context.Context, memberID uint, action, feature string, metadata domain.JSONMap) error
	CountDistinctActiveUsers(ctx context.Context, since, until time.Time) (int64, error)
	FindByFilter(ctx context.Context, start, end time.Time, action string) ([]domain.UserActivity, error)
	FindPaginated(ctx context.Context, filter ActivityFilter, page, limit int) ([]domain.UserActivity, int64, error)
}

type activityRepository struct {
	db *gorm.DB
}

func NewActivityRepository(db *gorm.DB) ActivityRepository {
	return &activityRepository{db: db}
}

// Log writes a UserActivity row. Errors are returned to the caller (which,
// matching the original helper's intent of "never break the request just
// because analytics logging failed", the middleware layer chooses to log
// and swallow - see middleware/activity_middleware.go).
func (r *activityRepository) Log(ctx context.Context, memberID uint, action, feature string, metadata domain.JSONMap) error {
	return r.db.WithContext(ctx).Create(&domain.UserActivity{
		MemberID: memberID,
		Action:   action,
		Feature:  feature,
		Metadata: metadata,
	}).Error
}

func (r *activityRepository) CountDistinctActiveUsers(ctx context.Context, since, until time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.UserActivity{}).
		Where("created_at BETWEEN ? AND ?", since, until).
		Distinct("member_id").
		Count(&count).Error
	return count, err
}

func (r *activityRepository) FindByFilter(ctx context.Context, start, end time.Time, action string) ([]domain.UserActivity, error) {
	var list []domain.UserActivity
	db := r.db.WithContext(ctx).Where("created_at BETWEEN ? AND ?", start, end)
	if action != "" {
		db = db.Where("action = ?", action)
	}
	err := db.Find(&list).Error
	return list, err
}

func (r *activityRepository) applyFilter(filter ActivityFilter) *gorm.DB {
	db := r.db.Model(&domain.UserActivity{})
	if filter.StartDate != nil {
		db = db.Where("created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		db = db.Where("created_at <= ?", *filter.EndDate)
	}
	if filter.Action != "" {
		db = db.Where("action = ?", filter.Action)
	}
	if filter.MemberID != nil {
		db = db.Where("member_id = ?", *filter.MemberID)
	}
	return db
}

func (r *activityRepository) FindPaginated(ctx context.Context, filter ActivityFilter, page, limit int) ([]domain.UserActivity, int64, error) {
	var total int64
	if err := r.applyFilter(filter).WithContext(ctx).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []domain.UserActivity
	err := r.applyFilter(filter).WithContext(ctx).
		Order("created_at desc").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&list).Error
	return list, total, err
}
