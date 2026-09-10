package dto

// CreatePaymentRequestBody is what a member submits when requesting to
// pay N upcoming months (creates pending payments awaiting approval).
type CreatePaymentRequestBody struct {
	Months int `json:"months" validate:"required,gt=0,lte=12"`
}

// CreatePaymentByAdminRequest - equivalent to createPaymentByAdminSchema (Zod).
type CreatePaymentByAdminRequest struct {
	MemberID uint    `json:"member_id" validate:"required"`
	Nominal  float64 `json:"nominal" validate:"required,gt=0"`
	Sign     string  `json:"sign"`
}
