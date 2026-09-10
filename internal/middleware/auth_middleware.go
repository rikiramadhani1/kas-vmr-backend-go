package middleware

import (
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/vmr/kas-vmr-backend/pkg/jwtutil"
	"github.com/vmr/kas-vmr-backend/pkg/response"
)

// Context keys used to stash the authenticated user's claims on the Echo
// context, mirroring the original `req.user` shape ({ id, email, role }).
const (
	ContextKeyUserID = "user_id"
	ContextKeyEmail  = "user_email"
	ContextKeyRole   = "user_role"
)

// Auth returns middleware that requires a valid Bearer access token,
// equivalent to the original authMiddleware.ts. On success it stashes the
// decoded payload on the Echo context for downstream handlers/middleware
// (see GetUserID/GetRole helpers below).
func Auth(signer *jwtutil.Signer) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				return response.Error(c, "Unauthorized", 401)
			}
			token := strings.TrimPrefix(header, "Bearer ")

			payload := signer.VerifyAccessToken(token)
			if payload == nil {
				return response.Error(c, "Unauthorized", 401)
			}

			c.Set(ContextKeyUserID, payload.ID)
			c.Set(ContextKeyEmail, payload.Email)
			c.Set(ContextKeyRole, payload.Role)

			return next(c)
		}
	}
}

// GetUserID reads the authenticated user's ID from the Echo context.
// Returns (0, false) if Auth middleware hasn't run.
func GetUserID(c echo.Context) (uint, bool) {
	v, ok := c.Get(ContextKeyUserID).(uint)
	return v, ok
}

func GetEmail(c echo.Context) (string, bool) {
	v, ok := c.Get(ContextKeyEmail).(string)
	return v, ok
}

func GetRole(c echo.Context) (string, bool) {
	v, ok := c.Get(ContextKeyRole).(string)
	return v, ok
}
