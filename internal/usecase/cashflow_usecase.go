package usecase

import (
	"context"
	"sort"
	"strconv"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/internal/repository"
	"github.com/vmr/kas-vmr-backend/pkg/response"
)

type CreateCashFlowInput struct {
	// Source is a free-text category (e.g. "dues", "donation",
	// "consumables") - Type (below) is what actually distinguishes
	// money in vs out.
	Source      string  `json:"source" validate:"required,min=1"`
	Amount      float64 `json:"amount" validate:"required,gt=0"`
	Description string  `json:"description" validate:"required,min=1"`
	Type        string  `json:"type" validate:"required,oneof=in out"`
}

type SaldoDTO struct {
	Saldo    float64 `json:"saldo"`
	TotalIn  float64 `json:"total_in"`
	TotalOut float64 `json:"total_out"`
}

type CashFlowPeriodDTO struct {
	Year         string            `json:"year"`
	Month        string            `json:"month"`
	Transactions []domain.CashFlow `json:"transactions"`
}

type CashFlowUsecase struct {
	cashFlowRepo repository.CashFlowRepository
}

func NewCashFlowUsecase(cashFlowRepo repository.CashFlowRepository) *CashFlowUsecase {
	return &CashFlowUsecase{cashFlowRepo: cashFlowRepo}
}

func (u *CashFlowUsecase) Create(ctx context.Context, in CreateCashFlowInput) (*domain.CashFlow, error) {
	cf := &domain.CashFlow{
		Type:        in.Type,
		Source:      in.Source,
		Amount:      in.Amount,
		Description: &in.Description,
	}
	if err := u.cashFlowRepo.Create(ctx, cf); err != nil {
		return nil, response.NewAPIError(500, "Terjadi kesalahan saat membuat cashflow")
	}
	return cf, nil
}

func (u *CashFlowUsecase) GetSaldo(ctx context.Context, all bool) (*SaldoDTO, error) {
	result, err := u.cashFlowRepo.GetSaldo(ctx, all)
	if err != nil {
		return nil, err
	}
	return &SaldoDTO{Saldo: result.Saldo, TotalIn: result.TotalIn, TotalOut: result.TotalOut}, nil
}

// GetGroupedByYear returns cash flow transactions grouped by "year-month",
// sorted newest year first, matching the original getCashFlowTerakhirService.
func (u *CashFlowUsecase) GetGroupedByYear(ctx context.Context, year *int) ([]CashFlowPeriodDTO, error) {
	transactions, err := u.cashFlowRepo.GetByYear(ctx, year)
	if err != nil {
		return nil, err
	}

	type group struct {
		year, month string
		items       []domain.CashFlow
	}
	groups := map[string]*group{}
	var order []string

	for _, tx := range transactions {
		y := strconv.Itoa(tx.CreatedAt.Year())
		m := MonthNameID(int(tx.CreatedAt.Month()))
		key := y + "-" + m
		g, ok := groups[key]
		if !ok {
			g = &group{year: y, month: m}
			groups[key] = g
			order = append(order, key)
		}
		g.items = append(g.items, tx)
	}

	result := make([]CashFlowPeriodDTO, 0, len(order))
	for _, key := range order {
		g := groups[key]
		result = append(result, CashFlowPeriodDTO{Year: g.year, Month: g.month, Transactions: g.items})
	}

	sort.Slice(result, func(i, j int) bool {
		yi, _ := strconv.Atoi(result[i].Year)
		yj, _ := strconv.Atoi(result[j].Year)
		return yi > yj
	})

	return result, nil
}
