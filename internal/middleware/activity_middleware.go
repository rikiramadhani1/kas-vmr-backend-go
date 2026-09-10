package middleware

import (
	"context"

	"github.com/labstack/echo/v4"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/internal/usecase"
)

// LogActivity returns middleware that fires an async, best-effort
// analytics log entry (action/feature) for the authenticated member after
// the wrapped handler completes successfully. It never blocks or fails
// the request - matching the original activityLogger's
// "logging is not allowed to break the user-facing request" behavior.
//
// Must be chained after Auth.
func LogActivity(activityUsecase *usecase.ActivityUsecase, action, feature string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			// Only log for genuinely successful requests, and only for
			// authenticated members (admins don't have "activity" in the
			// analytics sense).
			if err == nil && c.Response().Status < 400 {
				if userID, ok := GetUserID(c); ok {
					role, _ := GetRole(c)
					if role == domain.RoleMember {
						// Fire-and-forget: run in the background so
						// analytics writes never add latency to the
						// response the member already received. We
						// deliberately use a fresh context.Background()
						// rather than c.Request().Context(), since the
						// request context may be cancelled by the server
						// immediately after the handler returns.
						go activityUsecase.Log(context.Background(), userID, action, feature, nil)
					}
				}
			}

			return err
		}
	}
}
