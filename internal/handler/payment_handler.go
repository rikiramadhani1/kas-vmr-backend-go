package handler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

// CreateRequest handles POST /api/payments/request (member requests to pay N months).
func (h *PaymentHandler) CreateRequest(c echo.Context) error {
	var req dto.CreatePaymentRequestBody
	if err := c.Bind(&req); err != nil {
		return response.Error(c, "Payload tidak valid", 400)
	}
	if err := c.Validate(&req); err != nil {
		return response.Error(c, err.Error(), 400)
	}

	memberID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Error(c, "Unauthorized", 401)
	}

	payments, err := h.paymentUsecase.CreatePaymentRequest(c.Request().Context(), memberID, req.Months)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Permintaan pembayaran berhasil dibuat", payments, 201)
}

// GetAll handles GET /api/payments?phone=... - matches the original
// getAllPayments, which took a phone query param directly rather than the
// authenticated user (kept as-is since it's used by admin/bendahara
// lookups, not by the member themselves).
func (h *PaymentHandler) GetAll(c echo.Context) error {
	phone := c.QueryParam("phone")
	if phone == "" {
		return response.Error(c, `Query param "phone" wajib diisi`, 400)
	}

	list, err := h.paymentUsecase.GetAllByPhone(c.Request().Context(), phone)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Berhasil mengambil transaksi", list)
}

// GetPending handles GET /api/payments/pending (admin only).
func (h *PaymentHandler) GetPending(c echo.Context) error {
	list, err := h.paymentUsecase.GetPending(c.Request().Context())
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Berhasil mengambil pembayaran pending", list)
}

// Approve handles POST /api/payments/:id/approve (admin only).
func (h *PaymentHandler) Approve(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, "ID tidak valid", 400)
	}

	result, err := h.paymentUsecase.ApprovePayment(c.Request().Context(), uint(id))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, result.Message, result.Payment)
}

// Reject handles POST /api/payments/:id/reject (admin only).
func (h *PaymentHandler) Reject(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, "ID tidak valid", 400)
	}

	result, err := h.paymentUsecase.RejectPayment(c.Request().Context(), uint(id))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, result.Message, result.Payment)
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

// GetStatus handles GET /api/payments/status (admin only).
func (h *PaymentHandler) GetStatus(c echo.Context) error {
	list, err := h.paymentUsecase.FindPaymentStatus(c.Request().Context())
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Berhasil mengambil status pembayaran member", list)
}

// CreateByAdmin handles POST /api/payments/admin-create (admin only).
func (h *PaymentHandler) CreateByAdmin(c echo.Context) error {
	var req dto.CreatePaymentByAdminRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, "Payload tidak valid", 400)
	}
	if err := c.Validate(&req); err != nil {
		return response.Error(c, err.Error(), 400)
	}

	result, err := h.paymentUsecase.CreatePaymentByAdmin(c.Request().Context(), req.MemberID, req.Nominal, req.Sign)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Pembayaran berhasil dibuat", result, 201)
}

// ListUnmatched handles GET /api/payments/unmatched-transfers (admin
// only) - transfers received via the email auto-confirm worker that
// didn't match any active member's unique code, needing manual review.
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
//
// The original Node.js handler trusted `req.file.path` (from multer)
// as-is with no size/extension/mimetype check before feeding it to Jimp
// and Tesseract. We add basic guardrails here: enforce a max size and a
// whitelist of image extensions before writing anything to disk.
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
	// Always clean up the uploaded file once we're done processing it,
	// regardless of outcome - the original code left this commented out,
	// which would slowly fill up disk with old proof images.
	defer os.Remove(destPath)

	result, err := h.paymentUsecase.CreatePaymentByProof(c.Request().Context(), memberID, destPath)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Konfirmasi pembayaran berhasil", result, 201)
}
