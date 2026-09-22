package dto

type RegisterPWAInstallationRequest struct {
	InstallationID string `json:"installation_id" validate:"required,max=100"`
	Platform       string `json:"platform" validate:"required,max=30"`
	Browser        string `json:"browser" validate:"required,max=50"`
}