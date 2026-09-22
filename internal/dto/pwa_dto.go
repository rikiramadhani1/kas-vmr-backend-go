package dto

type RegisterPWAInstallationRequest struct {
	DeviceID string `json:"device_id" validate:"required,max=100"`
	Platform string `json:"platform" validate:"required,max=30"`
	Browser  string `json:"browser" validate:"required,max=50"`
}