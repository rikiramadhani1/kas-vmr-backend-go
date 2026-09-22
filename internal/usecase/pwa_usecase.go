package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/internal/repository"
)

type RegisterPWAInstallationRequest struct {
	DeviceID string
	Platform string
	Browser  string
}

type PWAUsecase struct {
	repo repository.PWARepository
}

func NewPWAUsecase(repo repository.PWARepository) *PWAUsecase {
	return &PWAUsecase{
		repo: repo,
	}
}

func (u *PWAUsecase) RegisterInstallation(
	ctx context.Context,
	memberID uint,
	req RegisterPWAInstallationRequest,
) error {
	now := time.Now()

	installation := &domain.PWAInstallation{
		MemberID:    memberID,
		DeviceID:    strings.TrimSpace(req.DeviceID),
		Platform:    strings.TrimSpace(req.Platform),
		Browser:     strings.TrimSpace(req.Browser),
		InstalledAt: now,
		LastSeenAt:  now,
	}

	return u.repo.Upsert(ctx, installation)
}

func (u *PWAUsecase) GetMemberInstallations(
	ctx context.Context,
	memberID uint,
) ([]domain.PWAInstallation, error) {
	return u.repo.FindByMemberID(ctx, memberID)
}