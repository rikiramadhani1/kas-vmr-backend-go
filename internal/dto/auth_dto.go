package dto

// LoginMemberRequest - equivalent to loginMemberSchema (Zod).
type LoginMemberRequest struct {
	Phone string `json:"phone" validate:"required,min=8"`
	Pin   string `json:"pin" validate:"required,len=6,numeric"`
}

// SetPinRequest - equivalent to setPinSchema (Zod).
type SetPinRequest struct {
	Pin string `json:"pin" validate:"required,len=6,numeric"`
}

// AdminRegisterRequest - equivalent to the body admin.controller expects
// for registerAdmin (name/email/password), no separate Zod schema existed
// for it in the original code, so this documents the same implicit shape.
type AdminRegisterRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// AdminLoginRequest mirrors the body loginAdmin expects.
type AdminLoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshTokenRequest is shared by the refresh/logout/logout-all routes.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}
