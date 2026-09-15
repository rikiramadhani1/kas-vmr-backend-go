package handler

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/vmr/kas-vmr-backend/internal/repository"
	"github.com/vmr/kas-vmr-backend/internal/usecase"
	"github.com/vmr/kas-vmr-backend/pkg/response"
)

type ActivityHandler struct {
	activityUsecase *usecase.ActivityUsecase
}

func NewActivityHandler(activityUsecase *usecase.ActivityUsecase) *ActivityHandler {
	return &ActivityHandler{activityUsecase: activityUsecase}
}

// GetWAU handles GET /api/analytics/wau.
func (h *ActivityHandler) GetWAU(c echo.Context) error {
	wau, err := h.activityUsecase.GetWAU(c.Request().Context())
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "WAU berhasil diambil", map[string]int64{"wau": wau})
}

// GetActivityByMember handles GET /api/analytics/action?start=...&end=...&action=...
// (admin only).
func (h *ActivityHandler) GetActivityByMember(c echo.Context) error {
	var start, end *time.Time
	if s := c.QueryParam("start"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			start = &t
		}
	}
	if e := c.QueryParam("end"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			end = &t
		}
	}
	action := c.QueryParam("action")

	result, err := h.activityUsecase.GetActivityByMember(c.Request().Context(), start, end, action)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Aktivitas berhasil diambil", result)
}

// GetActivityLog handles GET /api/analytics/logs?page=&limit=&startDate=&endDate=&action=&memberId=
// (admin only) - raw paginated log, one row per entry.
func (h *ActivityHandler) GetActivityLog(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	var filter repository.ActivityFilter
	if s := c.QueryParam("startDate"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			filter.StartDate = &t
		}
	}
	if e := c.QueryParam("endDate"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			filter.EndDate = &t
		}
	}
	filter.Action = c.QueryParam("action")
	if mid := c.QueryParam("memberId"); mid != "" {
		if n, err := strconv.ParseUint(mid, 10, 64); err == nil {
			id := uint(n)
			filter.MemberID = &id
		}
	}

	result, err := h.activityUsecase.GetActivityLog(c.Request().Context(), page, limit, filter)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Berhasil mengambil log aktivitas", result)
}
