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
}

// Register wires up every route, mirroring the original Express route
// files (auth.route.js, admin.route.js, member.route.js, payment.route.js,
// cashflow.route.js, analytics.route.js) under the same `/api` prefix.
func Register(e *echo.Echo, h Handlers, signer *jwtutil.Signer, activityUsecase *usecase.ActivityUsecase) {
	auth := middleware.Auth(signer)

	api := e.Group("/api")

	// ---- Auth routes (auth.route.js) ----
	authGroup := api.Group("/auth")
	authGroup.POST("/admin", h.Admin.Login) // admin login
	authGroup.POST("", h.Member.Login)      // member login
	authGroup.POST("/token", h.Admin.RefreshToken, auth, middleware.LogActivity(activityUsecase, "refresh_token", "auth"))
	authGroup.POST("/logout", h.Admin.Logout)
	authGroup.POST("/logout-all", h.Admin.LogoutAll)

	// ---- Admin routes (admin.route.js) ----
	adminGroup := api.Group("/admin")
	adminGroup.POST("/register", h.Admin.Register, auth, middleware.RequireRole(domain.RoleAdmin))

	// ---- Member routes (member.route.js) ----
	memberGroup := api.Group("/members")
	memberGroup.GET("", h.Member.GetAll, auth)
	memberGroup.GET("/profile", h.Member.GetProfile, auth, middleware.LogActivity(activityUsecase, "view_profile", "auth"))
	memberGroup.GET("/:id", h.Member.GetByID, auth, middleware.RequireRole(domain.RoleAdmin))
	memberGroup.POST("/pin", h.Member.SetPin, auth, middleware.LogActivity(activityUsecase, "set_pin", "member"))
	memberGroup.POST("/:member_id/reset-pin", h.Member.SetPinByAdmin, auth, middleware.RequireRole(domain.RoleAdmin))
	memberGroup.POST("/push-subscribe", h.Member.SubscribePush, auth)
	memberGroup.GET("/push-public-key", h.Member.GetPushPublicKey) // publik, gak perlu auth

	// ---- Payment routes (payment.route.js) ----
	paymentGroup := api.Group("/payments")
	paymentGroup.POST("/request", h.Payment.CreateRequest, auth, middleware.LogActivity(activityUsecase, "request_payment", "payment"))
	paymentGroup.GET("/count", h.Payment.Count, auth, middleware.LogActivity(activityUsecase, "count_payment", "payment"))
	paymentGroup.GET("/pending", h.Payment.GetPending, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))
	paymentGroup.GET("/unpaid", h.Payment.ListUnpaid, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))
	paymentGroup.GET("/paid-status", h.Payment.GetStatus, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))
	paymentGroup.POST("/:id/approve", h.Payment.Approve, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))
	paymentGroup.POST("/:id/reject", h.Payment.Reject, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))
	paymentGroup.POST("/admin-create", h.Payment.CreateByAdmin, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))
	paymentGroup.POST("/proof", h.Payment.CreateByProof, auth, middleware.LogActivity(activityUsecase, "upload_proof", "payment"))
	paymentGroup.GET("/unmatched-transfers", h.Payment.ListUnmatched, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))
	paymentGroup.GET("", h.Payment.GetAll, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))

	// ---- CashFlow routes (cashflow.route.js) ----
	cashFlowGroup := api.Group("/cashflow")
	cashFlowGroup.GET("", h.CashFlow.GetAll, auth, middleware.LogActivity(activityUsecase, "view_cashflow", "cashflow"))
	cashFlowGroup.GET("/saldo", h.CashFlow.GetSaldo, auth, middleware.LogActivity(activityUsecase, "view_saldo", "cashflow"))
	cashFlowGroup.POST("", h.CashFlow.Create, auth, middleware.RequireRole(domain.RoleAdmin, domain.RoleBendahara))

	// ---- Analytics routes (analytics.route.js) ----
	analyticsGroup := api.Group("/analytics")
	analyticsGroup.GET("/wau", h.Activity.GetWAU, auth, middleware.RequireRole(domain.RoleAdmin))
	analyticsGroup.GET("/action", h.Activity.GetActivityByMember, auth, middleware.RequireRole(domain.RoleAdmin))
	analyticsGroup.GET("/logs", h.Activity.GetActivityLog, auth, middleware.RequireRole(domain.RoleAdmin))

	// ---- Health check ----
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})
}
