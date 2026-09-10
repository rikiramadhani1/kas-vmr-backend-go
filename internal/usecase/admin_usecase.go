package usecase

import (
	"context"

	"golang.org/x/crypto/bcrypt"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/internal/repository"
	"github.com/vmr/kas-vmr-backend/pkg/jwtutil"
	"github.com/vmr/kas-vmr-backend/pkg/response"
	"github.com/vmr/kas-vmr-backend/pkg/tokenstore"
)

type AuthTokens struct {
	AccessToken  string      `json:"accessToken"`
	RefreshToken string      `json:"refreshToken,omitempty"`
	User         interface{} `json:"user,omitempty"`
}

type AdminProfile struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

type AdminUsecase struct {
	adminRepo repository.AdminRepository
	signer    *jwtutil.Signer
	tokens    *tokenstore.Store
}

func NewAdminUsecase(adminRepo repository.AdminRepository, signer *jwtutil.Signer, tokens *tokenstore.Store) *AdminUsecase {
	return &AdminUsecase{adminRepo: adminRepo, signer: signer, tokens: tokens}
}

func (u *AdminUsecase) Register(ctx context.Context, name, email, password string) (*domain.Admin, error) {
	existing, err := u.adminRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, response.NewAPIError(400, "Email sudah terdaftar")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	admin := &domain.Admin{
		Name:     name,
		Email:    email,
		Password: string(hashed),
		Role:     domain.RoleAdmin,
	}
	if err := u.adminRepo.Create(ctx, admin); err != nil {
		return nil, err
	}
	return admin, nil
}

func (u *AdminUsecase) Login(ctx context.Context, email, password string) (*AuthTokens, error) {
	admin, err := u.adminRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if admin == nil {
		return nil, response.NewAPIError(401, "Email atau password salah")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(password)); err != nil {
		return nil, response.NewAPIError(401, "Email atau password salah")
	}

	payload := jwtutil.Payload{ID: admin.ID, Email: admin.Email, Role: admin.Role}
	accessToken, err := u.signer.SignAccessToken(payload)
	if err != nil {
		return nil, err
	}
	refreshToken, err := u.signer.SignRefreshToken(payload)
	if err != nil {
		return nil, err
	}
	if err := u.tokens.Add(ctx, idToString(admin.ID), refreshToken); err != nil {
		return nil, err
	}

	return &AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: map[string]interface{}{
			"name":  admin.Name,
			"role":  admin.Role,
			"email": admin.Email,
		},
	}, nil
}

func (u *AdminUsecase) Profile(ctx context.Context, email string) (*AdminProfile, error) {
	admin, err := u.adminRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if admin == nil {
		return nil, nil
	}
	return &AdminProfile{
		Name:      admin.Name,
		Email:     admin.Email,
		Role:      admin.Role,
		CreatedAt: admin.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func (u *AdminUsecase) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	decoded := u.signer.VerifyRefreshToken(refreshToken)
	if decoded == nil {
		return "", response.NewAPIError(403, "Invalid refresh token")
	}

	valid, err := u.tokens.Has(ctx, idToString(decoded.ID), refreshToken)
	if err != nil {
		return "", err
	}
	if !valid {
		return "", response.NewAPIError(403, "Expired or revoked refresh token")
	}

	newAccessToken, err := u.signer.SignAccessToken(jwtutil.Payload{ID: decoded.ID, Email: decoded.Email, Role: decoded.Role})
	if err != nil {
		return "", err
	}
	return newAccessToken, nil
}

func (u *AdminUsecase) Logout(ctx context.Context, refreshToken string) error {
	decoded := u.signer.VerifyRefreshToken(refreshToken)
	if decoded == nil {
		return response.NewAPIError(403, "Invalid refresh token")
	}
	return u.tokens.Remove(ctx, idToString(decoded.ID), refreshToken)
}

func (u *AdminUsecase) LogoutAll(ctx context.Context, refreshToken string) error {
	decoded := u.signer.VerifyRefreshToken(refreshToken)
	if decoded == nil {
		return response.NewAPIError(403, "Invalid refresh token")
	}
	return u.tokens.RevokeAll(ctx, idToString(decoded.ID))
}
