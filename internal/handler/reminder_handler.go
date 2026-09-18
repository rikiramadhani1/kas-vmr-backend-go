package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/vmr/kas-vmr-backend/internal/usecase"
	"github.com/vmr/kas-vmr-backend/pkg/response"
)

type ReminderHandler struct {
	reminderUsecase *usecase.ReminderUsecase
	broadcastSecret string
}

func NewReminderHandler(reminderUsecase *usecase.ReminderUsecase, broadcastSecret string) *ReminderHandler {
	return &ReminderHandler{reminderUsecase: reminderUsecase, broadcastSecret: broadcastSecret}
}

// BroadcastReminder handles POST /api/broadcast/reminder (secret-protected
// via X-Broadcast-Secret header, meant to be called directly via
// curl/cron rather than through the app's normal member/admin login flow).
func (h *ReminderHandler) BroadcastReminder(c echo.Context) error {
	secret := c.Request().Header.Get("X-Broadcast-Secret")
	if h.broadcastSecret == "" || secret != h.broadcastSecret {
		return response.Error(c, "unauthorized", 401)
	}

	h.reminderUsecase.RunOnce(c.Request().Context())
	return response.Success(c, "Reminder berhasil dikirim ke member yang menunggak", nil)
}
