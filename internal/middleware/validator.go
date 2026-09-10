package middleware

import (
	"github.com/go-playground/validator/v10"
)

// RequestValidator implements echo.Validator using go-playground/validator,
// the closest Go equivalent to the Zod schemas used throughout the
// original Node.js code (createPaymentByAdminSchema, loginMemberSchema,
// setPinSchema, etc). DTO structs declare their rules with `validate:"..."`
// struct tags; handlers call c.Bind(&dto) followed by c.Validate(&dto).
type RequestValidator struct {
	validator *validator.Validate
}

func NewRequestValidator() *RequestValidator {
	return &RequestValidator{validator: validator.New()}
}

func (v *RequestValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}
