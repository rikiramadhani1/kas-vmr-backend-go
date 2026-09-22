package dto

type SubscribePushRequest struct {
	InstallationID string `json:"installation_id" validate:"required,max=100"`
	Endpoint       string `json:"endpoint" validate:"required,url"`
	Keys           struct {
		P256dh string `json:"p256dh" validate:"required"`
		Auth   string `json:"auth" validate:"required"`
	} `json:"keys" validate:"required"`
}

type UnsubscribePushRequest struct {
	Endpoint string `json:"endpoint" validate:"required,url"`
}
