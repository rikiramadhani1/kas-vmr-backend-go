package dto

// CreatePaymentByAdminRequest is what an admin submits to manually
// record a payment (e.g. cash handed in person).
type CreatePaymentByAdminRequest struct {
	MemberID uint    `json:"member_id" validate:"required"`
	Nominal  float64 `json:"nominal" validate:"required,gt=0"`
}
