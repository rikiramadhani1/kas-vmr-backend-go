package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/internal/dto"
	"github.com/vmr/kas-vmr-backend/internal/middleware"
	"github.com/vmr/kas-vmr-backend/internal/usecase"
	"github.com/vmr/kas-vmr-backend/pkg/response"
)

type MemberHandler struct {
	memberUsecase *usecase.MemberUsecase
	adminUsecase  *usecase.AdminUsecase
	notificationUsecase *usecase.NotificationUsecase
	vapidPublicKey string
}

func NewMemberHandler(memberUsecase *usecase.MemberUsecase, adminUsecase *usecase.AdminUsecase, notificationUsecase *usecase.NotificationUsecase, vapidPublicKey string) *MemberHandler {
	return &MemberHandler{memberUsecase: memberUsecase, adminUsecase: adminUsecase, notificationUsecase: notificationUsecase, vapidPublicKey: vapidPublicKey}
}

// Login handles POST /api/auth (member login by phone + PIN).
func (h *MemberHandler) Login(c echo.Context) error {
	var req dto.LoginMemberRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, "Payload tidak valid", 400)
	}
	if err := c.Validate(&req); err != nil {
		return response.Error(c, err.Error(), 400)
	}

	tokens, err := h.memberUsecase.Login(c.Request().Context(), req.Phone, req.Pin)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Login member successful", tokens)
}

// SetPin handles POST /api/members/pin (member sets their own PIN).
func (h *MemberHandler) SetPin(c echo.Context) error {
	var req dto.SetPinRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, "Payload tidak valid", 400)
	}
	if err := c.Validate(&req); err != nil {
		return response.Error(c, err.Error(), 400)
	}

	memberID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Error(c, "Unauthorized", 401)
	}

	name, err := h.memberUsecase.SetPin(c.Request().Context(), memberID, req.Pin)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Set PIN Berhasil", map[string]string{"name": name})
}

// SetPinByAdmin handles POST /api/members/:member_id/reset-pin (admin
// resets a member's PIN to the configured default).
func (h *MemberHandler) SetPinByAdmin(c echo.Context) error {
	memberID, err := strconv.ParseUint(c.Param("member_id"), 10, 64)
	if err != nil || memberID == 0 {
		return response.Error(c, "Member ID wajib", 400)
	}

	name, err := h.memberUsecase.SetPinByAdmin(c.Request().Context(), uint(memberID))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Set PIN berhasil oleh admin", map[string]string{"name": name})
}

// GetAll handles GET /api/members.
func (h *MemberHandler) GetAll(c echo.Context) error {
	members, err := h.memberUsecase.GetAllActive(c.Request().Context())
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Daftar member berhasil diambil", members)
}

// GetByID handles GET /api/members/:id (admin only).
func (h *MemberHandler) GetByID(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, "ID harus berupa angka", 400)
	}

	member, err := h.memberUsecase.GetByID(c.Request().Context(), uint(id))
	if err != nil {
		return response.FromError(c, err)
	}
	if member == nil {
		return response.Error(c, "Member tidak ditemukan", 404)
	}
	return response.Success(c, "Member berhasil diambil", member)
}

// GetProfile handles GET /api/auth/profile, dispatching to the admin or
// member profile depending on the authenticated role (matching the
// original getProfileHandler's combined admin/member logic).
func (h *MemberHandler) GetProfile(c echo.Context) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Error(c, "Unauthorized", 401)
	}
	role, _ := middleware.GetRole(c)

	if role == domain.RoleAdmin {
		email, _ := middleware.GetEmail(c)
		profile, err := h.adminUsecase.Profile(c.Request().Context(), email)
		if err != nil {
			return response.FromError(c, err)
		}
		if profile == nil {
			return response.Error(c, "Admin tidak ditemukan", 404)
		}
		return response.Success(c, "Profile admin berhasil diambil", profile)
	}

	if role == domain.RoleMember {
		member, err := h.memberUsecase.GetByID(c.Request().Context(), userID)
		if err != nil {
			return response.FromError(c, err)
		}
		if member == nil {
			return response.Error(c, "Member tidak ditemukan", 404)
		}
		return response.Success(c, "Profile member berhasil diambil", member)
	}

	return response.Error(c, "Unauthorized", 401)
}

func (h *MemberHandler) SubscribePush(c echo.Context) error {
	var req dto.SubscribePushRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, "Payload tidak valid", 400)
	}
	if err := c.Validate(&req); err != nil {
		return response.Error(c, err.Error(), 400)
	}
	memberID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Error(c, "Unauthorized", 401)
	}
	if err := h.notificationUsecase.Subscribe(c.Request().Context(), memberID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Berhasil subscribe notifikasi", nil)
}

func (h *MemberHandler) GetPushPublicKey(c echo.Context) error {
	return response.Success(c, "OK", map[string]string{"publicKey": h.vapidPublicKey})
}
