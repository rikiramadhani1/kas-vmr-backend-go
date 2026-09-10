package response

import "github.com/labstack/echo/v4"

// Meta mirrors the `{ success, message, code }` envelope used throughout
// the original Node.js API.
type Meta struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Envelope is the `{ meta, data }` response shape.
type Envelope struct {
	Meta Meta        `json:"meta"`
	Data interface{} `json:"data"`
}

// APIError is a typed error carrying an HTTP status code, equivalent to
// the original `ApiError` class. Handlers can type-assert on this to
// decide the right status code instead of always defaulting to 400/500.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string { return e.Message }

// NewAPIError constructs an *APIError.
func NewAPIError(statusCode int, message string) *APIError {
	return &APIError{StatusCode: statusCode, Message: message}
}

// Success writes a 200-family success envelope.
func Success(c echo.Context, message string, data interface{}, code ...int) error {
	statusCode := 200
	if len(code) > 0 {
		statusCode = code[0]
	}
	return c.JSON(statusCode, Envelope{
		Meta: Meta{Success: true, Message: message, Code: statusCode},
		Data: data,
	})
}

// Error writes an error envelope. If err is an *APIError its StatusCode is
// used; otherwise the provided code (default 400) is used and the error's
// message is NOT echoed verbatim to the client to avoid leaking internal
// details - callers should pass a safe, user-facing message.
func Error(c echo.Context, message string, code ...int) error {
	statusCode := 400
	if len(code) > 0 {
		statusCode = code[0]
	}
	return c.JSON(statusCode, Envelope{
		Meta: Meta{Success: false, Message: message, Code: statusCode},
		Data: nil,
	})
}

// FromError inspects err and writes the most appropriate error envelope:
//   - *APIError: uses its StatusCode + Message as-is (these are always
//     intentionally user-facing messages set by usecase code).
//   - anything else (unexpected/internal errors): logs are expected to have
//     already happened at the call site; the client only gets a generic
//     500 message so internal details never leak over the wire.
func FromError(c echo.Context, err error) error {
	var apiErr *APIError
	if e, ok := err.(*APIError); ok {
		apiErr = e
		return Error(c, apiErr.Message, apiErr.StatusCode)
	}
	return Error(c, "Terjadi kesalahan internal, silakan coba lagi", 500)
}
