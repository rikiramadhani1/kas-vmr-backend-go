package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/vmr/kas-vmr-backend/internal/dto"
	"github.com/vmr/kas-vmr-backend/internal/middleware"
	"github.com/vmr/kas-vmr-backend/internal/usecase"
	"github.com/vmr/kas-vmr-backend/pkg/response"
)

type AdminHandler struct {
	adminUsecase *usecase.AdminUsecase
}

func NewAdminHandler(adminUsecase *usecase.AdminUsecase) *AdminHandler {
	return &AdminHandler{adminUsecase: adminUsecase}
}

// Register handles POST /api/admin/register (requires an existing admin).
func (h *AdminHandler) Register(c echo.Context) error {
	var req dto.AdminRegisterRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, "Payload tidak valid", 400)
	}
	if err := c.Validate(&req); err != nil {
		return response.Error(c, err.Error(), 400)
	}

	admin, err := h.adminUsecase.Register(c.Request().Context(), req.Name, req.Email, req.Password)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Admin berhasil didaftarkan", admin, 201)
}

// Login handles POST /api/auth/admin.
func (h *AdminHandler) Login(c echo.Context) error {
	var req dto.AdminLoginRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, "Payload tidak valid", 400)
	}
	if err := c.Validate(&req); err != nil {
		return response.Error(c, err.Error(), 400)
	}

	tokens, err := h.adminUsecase.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Login admin berhasil", tokens)
}

// Profile handles GET /api/auth/profile when the caller is an admin
// (member profile is handled separately - see MemberHandler.GetProfile).
func (h *AdminHandler) Profile(c echo.Context) error {
	email, _ := middleware.GetEmail(c)
	if email == "" {
		return response.Error(c, "Unauthorized", 401)
	}

	profile, err := h.adminUsecase.Profile(c.Request().Context(), email)
	if err != nil {
		return response.FromError(c, err)
	}
	if profile == nil {
		return response.Error(c, "Admin tidak ditemukan", 404)
	}
	return response.Success(c, "Profile admin berhasil diambil", profile)
}

// RefreshToken handles POST /api/auth/token.
func (h *AdminHandler) RefreshToken(c echo.Context) error {
	var req dto.RefreshTokenRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, "Payload tidak valid", 400)
	}
	if err := c.Validate(&req); err != nil {
		return response.Error(c, err.Error(), 400)
	}

	accessToken, err := h.adminUsecase.RefreshToken(c.Request().Context(), req.RefreshToken)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Token berhasil diperbarui", map[string]string{"accessToken": accessToken})
}

// Logout handles POST /api/auth/logout.
func (h *AdminHandler) Logout(c echo.Context) error {
	var req dto.RefreshTokenRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, "Payload tidak valid", 400)
	}
	if err := c.Validate(&req); err != nil {
		return response.Error(c, err.Error(), 400)
	}

	if err := h.adminUsecase.Logout(c.Request().Context(), req.RefreshToken); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Logout berhasil", nil)
}

// LogoutAll handles POST /api/auth/logout-all.
func (h *AdminHandler) LogoutAll(c echo.Context) error {
	var req dto.RefreshTokenRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, "Payload tidak valid", 400)
	}
	if err := c.Validate(&req); err != nil {
		return response.Error(c, err.Error(), 400)
	}

	if err := h.adminUsecase.LogoutAll(c.Request().Context(), req.RefreshToken); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Berhasil logout dari semua perangkat", nil)
}
