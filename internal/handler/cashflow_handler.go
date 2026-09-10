package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/vmr/kas-vmr-backend/internal/usecase"
	"github.com/vmr/kas-vmr-backend/pkg/response"
)

type CashFlowHandler struct {
	cashFlowUsecase *usecase.CashFlowUsecase
}

func NewCashFlowHandler(cashFlowUsecase *usecase.CashFlowUsecase) *CashFlowHandler {
	return &CashFlowHandler{cashFlowUsecase: cashFlowUsecase}
}

// Create handles POST /api/cashflow (admin only).
func (h *CashFlowHandler) Create(c echo.Context) error {
	var req usecase.CreateCashFlowInput
	if err := c.Bind(&req); err != nil {
		return response.Error(c, "Payload tidak valid", 400)
	}
	if err := c.Validate(&req); err != nil {
		return response.Error(c, err.Error(), 400)
	}

	cf, err := h.cashFlowUsecase.Create(c.Request().Context(), req)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Cash flow berhasil dibuat", cf, 201)
}

// GetSaldo handles GET /api/cashflow/saldo?all=true|false.
func (h *CashFlowHandler) GetSaldo(c echo.Context) error {
	all, _ := strconv.ParseBool(c.QueryParam("all"))

	saldo, err := h.cashFlowUsecase.GetSaldo(c.Request().Context(), all)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Saldo berhasil diambil", saldo)
}

// GetAll handles GET /api/cashflow?year=2026.
func (h *CashFlowHandler) GetAll(c echo.Context) error {
	var year *int
	if y := c.QueryParam("year"); y != "" {
		if parsed, err := strconv.Atoi(y); err == nil {
			year = &parsed
		}
	}

	grouped, err := h.cashFlowUsecase.GetGroupedByYear(c.Request().Context(), year)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Riwayat cash flow berhasil diambil", grouped)
}
