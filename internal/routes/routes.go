package routes

import (
	"github.com/labstack/echo/v4"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/internal/handler"
	"github.com/vmr/kas-vmr-backend/internal/middleware"
	"github.com/vmr/kas-vmr-backend/internal/usecase"
	"github.com/vmr/kas-vmr-backend/pkg/jwtutil"
)

// Handlers bundles every handler the router needs to wire up.
type Handlers struct {
	Admin    *handler.AdminHandler
	Member   *handler.MemberHandler
	Payment  *handler.PaymentHandler
	CashFlow *handler.CashFlowHandler
	Activity *handler.ActivityHandler
	Reminder  *handler.ReminderHandler
}

// Register wires up every route under the `/api` prefix.
func Register(e *echo.Echo, h Handlers, signer *jwtutil.Signer, activityUsecase *usecase.ActivityUsecase) {
	auth := middleware.Auth(signer)

	api := e.Group("/api")

	// ---- Auth routes ----
	authGroup := api.Group("/auth")
	authGroup.POST("/admin", h.Admin.Login) // admin login
	authGroup.POST("", h.Member.Login)      // member login
	authGroup.POST("/pin", h.Member.SetPin, auth, middleware.LogActivity(activityUsecase, "set_pin", "member"))
	authGroup.POST("/members/:member_id/reset-pin", h.Member.SetPinByAdmin, auth, middleware.RequireRole(domain.RoleAdmin))
	authGroup.POST("/token", h.Admin.RefreshToken)
	authGroup.POST("/logout", h.Admin.Logout)
	authGroup.POST("/logout-all", h.Admin.LogoutAll)
	authGroup.GET("/profile", h.Member.GetProfile, auth, middleware.LogActivity(activityUsecase, "view_profile", "auth"))

	// ---- Admin routes ----
	adminGroup := api.Group("/admin")
	adminGroup.POST("/register", h.Admin.Register, auth, middleware.RequireRole(domain.RoleAdmin))

	// ---- Member routes ----
	memberGroup := api.Group("/members")
	memberGroup.GET("", h.Member.GetAll, auth)
	memberGroup.GET("/:id", h.Member.GetByID, auth, middleware.RequireRole(domain.RoleAdmin))
	memberGroup.GET("/push-public-key", h.Member.GetPushPublicKey) // public, no auth - FE needs this before subscribing
	memberGroup.POST("/push-subscribe", h.Member.SubscribePush, auth)
	memberGroup.POST("/push-unsubscribe", h.Member.UnsubscribePush, auth)
	memberGroup.POST("/pwa-install", h.Member.RegisterPWAInstallation, auth)

	// ---- Payment routes ----
	// NOTE: the pending/approve/reject workflow is GONE - every payment is
	// now auto-recorded the moment it's detected (OCR upload or the email
	// auto-confirm worker), so there's nothing left for an admin to
	// manually approve.
	paymentGroup := api.Group("/payments")
	paymentGroup.GET("/count", h.Payment.Count, auth)
	paymentGroup.GET("/recent", h.Payment.GetRecent, auth, middleware.LogActivity(activityUsecase, "view_recent_payments", "payment"))
	paymentGroup.GET("/unpaid", h.Payment.ListUnpaid, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))
	paymentGroup.GET("/paid-status", h.Payment.GetStatus, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))
	paymentGroup.POST("/admin-create", h.Payment.CreateByAdmin, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))
	paymentGroup.POST("/proof", h.Payment.CreateByProof, auth, middleware.LogActivity(activityUsecase, "upload_proof", "payment"))
	paymentGroup.GET("/unmatched-transfers", h.Payment.ListUnmatched, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))
	paymentGroup.GET("", h.Payment.GetRecentByPhone, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))

	// ---- CashFlow routes ----
	cashFlowGroup := api.Group("/cashflow")
	cashFlowGroup.GET("", h.CashFlow.GetAll, auth, middleware.LogActivity(activityUsecase, "view_cashflow", "cashflow"))
	cashFlowGroup.GET("/saldo", h.CashFlow.GetSaldo, auth)
	cashFlowGroup.POST("", h.CashFlow.Create, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))

	// ---- Analytics routes ----
	analyticsGroup := api.Group("/analytics")
	analyticsGroup.GET("/wau", h.Activity.GetWAU, auth, middleware.RequireRole(domain.RoleAdmin))
	analyticsGroup.GET("/action", h.Activity.GetActivityByMember, auth, middleware.RequireRole(domain.RoleAdmin))
	analyticsGroup.GET("/logs", h.Activity.GetActivityLog, auth, middleware.RequireRole(domain.RoleAdmin))

	// ---- Health check ----
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// ---- Manual broadcast trigger (curl/cron, secret-protected) ----
	e.POST("/api/broadcast/reminder", h.Reminder.BroadcastReminder)
}
