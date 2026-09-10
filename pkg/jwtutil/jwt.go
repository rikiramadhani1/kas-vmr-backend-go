package jwtutil

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 31 * 24 * time.Hour
)

// Payload mirrors the original `JwtPayload` interface: { id, email, role }.
type Payload struct {
	ID    uint   `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type claims struct {
	Payload
	jwt.RegisteredClaims
}

// Signer signs and verifies access/refresh tokens using two distinct
// secrets, matching the original design (JWT_ACCESS_SECRET /
// JWT_REFRESH_SECRET). Unlike the Node.js version, the secrets are
// required at startup (see config.Load) rather than silently falling
// back to a hardcoded default.
type Signer struct {
	accessSecret  []byte
	refreshSecret []byte
}

func NewSigner(accessSecret, refreshSecret string) *Signer {
	return &Signer{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
	}
}

func (s *Signer) SignAccessToken(p Payload) (string, error) {
	return s.sign(p, s.accessSecret, AccessTokenTTL)
}

func (s *Signer) SignRefreshToken(p Payload) (string, error) {
	return s.sign(p, s.refreshSecret, RefreshTokenTTL)
}

func (s *Signer) sign(p Payload, secret []byte, ttl time.Duration) (string, error) {
	now := time.Now()
	c := claims{
		Payload: p,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString(secret)
}

// VerifyAccessToken parses and validates an access token, returning nil,
// nil on any failure (invalid signature, expired, malformed) - mirroring
// the original `verifyAccessToken` which swallowed errors and returned
// undefined.
func (s *Signer) VerifyAccessToken(tokenStr string) *Payload {
	return s.verify(tokenStr, s.accessSecret)
}

// VerifyRefreshToken parses and validates a refresh token.
func (s *Signer) VerifyRefreshToken(tokenStr string) *Payload {
	return s.verify(tokenStr, s.refreshSecret)
}

func (s *Signer) verify(tokenStr string, secret []byte) *Payload {
	var c claims
	token, err := jwt.ParseWithClaims(tokenStr, &c, func(t *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil
	}
	return &c.Payload
}
