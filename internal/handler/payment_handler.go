package handler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/vmr/kas-vmr-backend/internal/dto"
	"github.com/vmr/kas-vmr-backend/internal/middleware"
	"github.com/vmr/kas-vmr-backend/internal/usecase"
	"github.com/vmr/kas-vmr-backend/pkg/response"
)

type PaymentHandler struct {
	paymentUsecase     *usecase.PaymentUsecase
	autoConfirmUsecase *usecase.AutoConfirmUsecase
	uploadDir          string
}

func NewPaymentHandler(paymentUsecase *usecase.PaymentUsecase, autoConfirmUsecase *usecase.AutoConfirmUsecase, uploadDir string) *PaymentHandler {
	return &PaymentHandler{paymentUsecase: paymentUsecase, autoConfirmUsecase: autoConfirmUsecase, uploadDir: uploadDir}
}

// GetRecent handles GET /api/payments/recent (member's own last 5
// transactions) - equivalent to the original "riwayat pembayaran", now
// backed by the Transaksi table.
func (h *PaymentHandler) GetRecent(c echo.Context) error {
	memberID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Error(c, "Unauthorized", 401)
	}

	list, err := h.paymentUsecase.GetRecentByMemberID(c.Request().Context(), memberID, 5)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Berhasil mengambil riwayat pembayaran", list)
}

// GetRecentByPhone handles GET /api/payments?phone=... (admin/bendahara
// lookup by phone).
func (h *PaymentHandler) GetRecentByPhone(c echo.Context) error {
	phone := c.QueryParam("phone")
	if phone == "" {
		return response.Error(c, `Query param "phone" wajib diisi`, 400)
	}

	list, err := h.paymentUsecase.GetRecentByPhone(c.Request().Context(), phone, 5)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Berhasil mengambil riwayat pembayaran", list)
}

// Count handles GET /api/payments/count (member's own tunggakan).
func (h *PaymentHandler) Count(c echo.Context) error {
	memberID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Error(c, "Unauthorized", 401)
	}

	result, err := h.paymentUsecase.CountPayment(c.Request().Context(), memberID)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Berhasil menghitung tanggungan", result)
}

// ListUnpaid handles GET /api/payments/unpaid (admin only).
func (h *PaymentHandler) ListUnpaid(c echo.Context) error {
	list, err := h.paymentUsecase.FindUnpaidMembers(c.Request().Context())
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Berhasil mengambil member yang menunggak", list)
}

// GetStatus handles GET /api/payments/status (admin only) - members who
// are currently paid up, the counterpart to ListUnpaid.
func (h *PaymentHandler) GetStatus(c echo.Context) error {
	list, err := h.paymentUsecase.FindPaymentStatus(c.Request().Context())
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Berhasil mengambil status pembayaran member", list)
}

// CreateByAdmin handles POST /api/payments/admin-create (admin only) -
// manually record a payment, e.g. cash handed in person.
func (h *PaymentHandler) CreateByAdmin(c echo.Context) error {
	var req dto.CreatePaymentByAdminRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, "Payload tidak valid", 400)
	}
	if err := c.Validate(&req); err != nil {
		return response.Error(c, err.Error(), 400)
	}

	result, err := h.paymentUsecase.CreateByAdmin(c.Request().Context(), req.MemberID, req.Nominal)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Pembayaran berhasil dicatat", result, 201)
}

// ListUnmatched handles GET /api/payments/unmatched-transfers (admin
// only) - transfers received via the email auto-confirm worker that
// didn't match any active member's house number, needing manual review.
func (h *PaymentHandler) ListUnmatched(c echo.Context) error {
	if h.autoConfirmUsecase == nil {
		return response.Success(c, "Mail watcher tidak diaktifkan", []interface{}{})
	}
	list, err := h.autoConfirmUsecase.ListUnmatched(c.Request().Context())
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Berhasil mengambil transfer yang belum cocok", list)
}

const maxUploadSize = 10 << 20 // 10 MB

var allowedUploadExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

// CreateByProof handles POST /api/payments/proof (member uploads a
// transfer-proof screenshot, multipart/form-data, field name "file").
func (h *PaymentHandler) CreateByProof(c echo.Context) error {
	memberID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Error(c, "Unauthorized", 401)
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return response.Error(c, "File bukti transfer wajib diupload", 400)
	}

	if fileHeader.Size > maxUploadSize {
		return response.Error(c, "Ukuran file terlalu besar (maksimal 10MB)", 400)
	}

	ext := filepath.Ext(fileHeader.Filename)
	if !allowedUploadExt[ext] {
		return response.Error(c, "Format file tidak didukung, gunakan JPG atau PNG", 400)
	}

	src, err := fileHeader.Open()
	if err != nil {
		return response.Error(c, "Gagal membaca file", 400)
	}
	defer src.Close()

	if err := os.MkdirAll(h.uploadDir, 0o755); err != nil {
		return response.FromError(c, err)
	}

	filename := fmt.Sprintf("%d_%d%s", memberID, time.Now().UnixNano(), ext)
	destPath := filepath.Join(h.uploadDir, filename)

	dest, err := os.Create(destPath)
	if err != nil {
		return response.FromError(c, err)
	}
	if _, err := io.Copy(dest, src); err != nil {
		dest.Close()
		return response.FromError(c, err)
	}
	dest.Close()
	defer os.Remove(destPath)

	result, err := h.paymentUsecase.CreateByProof(c.Request().Context(), memberID, destPath)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Konfirmasi pembayaran berhasil", result, 201)
}
