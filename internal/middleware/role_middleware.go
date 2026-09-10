package middleware

import (
	"github.com/labstack/echo/v4"

	"github.com/vmr/kas-vmr-backend/pkg/response"
)

// RequireRole returns middleware that only allows requests through if the
// authenticated user's role (set by Auth middleware) is one of the
// allowed roles. Must be chained after Auth.
func RequireRole(allowed ...string) echo.MiddlewareFunc {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = struct{}{}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role, ok := GetRole(c)
			if !ok {
				return response.Error(c, "Unauthorized", 401)
			}
			if _, allowed := allowedSet[role]; !allowed {
				return response.Error(c, "Forbidden: insufficient role", 403)
			}
			return next(c)
		}
	}
}
