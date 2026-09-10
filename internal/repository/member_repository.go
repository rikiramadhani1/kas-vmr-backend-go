package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/pkg/phoneutil"
)

type MemberRepository interface {
	FindByPhoneOrSpouse(ctx context.Context, phone string) (*domain.Member, error)
	FindAllActive(ctx context.Context) ([]domain.Member, error)
	FindByID(ctx context.Context, id uint) (*domain.Member, error)
	UpdatePin(ctx context.Context, memberID uint, hashedPin string) (*domain.Member, error)
	FindByBankAccountSuffix(ctx context.Context, suffix string) (*domain.Member, error)
	UpdateBankAccountSuffix(ctx context.Context, memberID uint, suffix string) (*domain.Member, error)
	FindByHouseNumber(ctx context.Context, houseNumber string) (*domain.Member, error)
}

type memberRepository struct {
	db *gorm.DB
}

func NewMemberRepository(db *gorm.DB) MemberRepository {
	return &memberRepository{db: db}
}

// FindByPhoneOrSpouse normalizes phone via pkg/phoneutil (single shared
// rule - see that package's doc comment for why this matters) then looks
// the member up by either their own phone number or their spouse's.
func (r *memberRepository) FindByPhoneOrSpouse(ctx context.Context, phone string) (*domain.Member, error) {
	p := phoneutil.Normalize(phone)

	var member domain.Member
	err := r.db.WithContext(ctx).
		Where("phone_number = ? OR spouse_phone_number = ?", p, p).
		First(&member).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *memberRepository) FindAllActive(ctx context.Context) ([]domain.Member, error) {
	var members []domain.Member
	err := r.db.WithContext(ctx).
		Where("status = ?", domain.MemberStatusActive).
		Order("name asc").
		Find(&members).Error
	return members, err
}

func (r *memberRepository) FindByID(ctx context.Context, id uint) (*domain.Member, error) {
	var member domain.Member
	err := r.db.WithContext(ctx).First(&member, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *memberRepository) UpdatePin(ctx context.Context, memberID uint, hashedPin string) (*domain.Member, error) {
	var member domain.Member
	if err := r.db.WithContext(ctx).First(&member, memberID).Error; err != nil {
		return nil, err
	}
	member.Pin = &hashedPin
	if err := r.db.WithContext(ctx).Model(&member).Update("pin", hashedPin).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *memberRepository) UpdateBankAccountSuffix(ctx context.Context, memberID uint, suffix string) (*domain.Member, error) {
	var member domain.Member
	if err := r.db.WithContext(ctx).First(&member, memberID).Error; err != nil {
		return nil, err
	}
	member.BankAccountSuffix = &suffix
	if err := r.db.WithContext(ctx).Model(&member).Update("bank_account_suffix", suffix).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *memberRepository) FindByBankAccountSuffix(ctx context.Context, suffix string) (*domain.Member, error) {
	var member domain.Member
	err := r.db.WithContext(ctx).
		Where("bank_account_suffix = ?", suffix).
		First(&member).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *memberRepository) FindByHouseNumber(ctx context.Context, houseNumber string) (*domain.Member, error) {
	var member domain.Member
	err := r.db.WithContext(ctx).
		Where("UPPER(house_number) = UPPER(?)", houseNumber).
		First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &member, nil
}