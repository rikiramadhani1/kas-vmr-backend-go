package usecase

import (
	"context"
	"log"
	"time"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/internal/repository"
)

type ActivityByMemberDTO struct {
	MemberID uint     `json:"id_member"`
	Name     string   `json:"nama_member"`
	Action   string   `json:"action"`
	Count    int      `json:"count"`
	Feature  []string `json:"feature"`
}

type ActivityLogEntryDTO struct {
	ID         uint           `json:"id"`
	MemberName string         `json:"member_name"`
	Action     string         `json:"action"`
	Feature    string         `json:"feature"`
	Metadata   domain.JSONMap `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

type PaginationDTO struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
}

type ActivityLogResult struct {
	Data       []ActivityLogEntryDTO `json:"data"`
	Pagination PaginationDTO         `json:"pagination"`
}

type ActivityUsecase struct {
	activityRepo repository.ActivityRepository
	memberRepo   repository.MemberRepository
}

func NewActivityUsecase(activityRepo repository.ActivityRepository, memberRepo repository.MemberRepository) *ActivityUsecase {
	return &ActivityUsecase{activityRepo: activityRepo, memberRepo: memberRepo}
}

// Log records a user activity event. Errors are logged but not
// propagated - matching the original activityLogger's
// "never break the request just because analytics logging failed" intent.
func (u *ActivityUsecase) Log(ctx context.Context, memberID uint, action, feature string, metadata domain.JSONMap) {
	if err := u.activityRepo.Log(ctx, memberID, action, feature, metadata); err != nil {
		log.Printf("failed to log user activity: %v", err)
	}
}

func (u *ActivityUsecase) GetWAU(ctx context.Context) (int64, error) {
	now := time.Now().UTC()
	sevenDaysAgo := now.AddDate(0, 0, -7)
	return u.activityRepo.CountDistinctActiveUsers(ctx, sevenDaysAgo, now)
}

func (u *ActivityUsecase) GetActivityByMember(ctx context.Context, start, end *time.Time, action string) ([]ActivityByMemberDTO, error) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	tomorrow := today.AddDate(0, 0, 1)

	rangeStart, rangeEnd := today, tomorrow
	if start != nil {
		rangeStart = *start
	}
	if end != nil {
		rangeEnd = *end
	}

	activities, err := u.activityRepo.FindByFilter(ctx, rangeStart, rangeEnd, action)
	if err != nil {
		return nil, err
	}

	type key struct {
		memberID uint
		action   string
	}
	grouped := map[key]*ActivityByMemberDTO{}
	var order []key

	for _, a := range activities {
		k := key{memberID: a.MemberID, action: a.Action}
		g, ok := grouped[k]
		if !ok {
			g = &ActivityByMemberDTO{MemberID: a.MemberID, Action: a.Action}
			grouped[k] = g
			order = append(order, k)
		}
		g.Feature = append(g.Feature, a.Feature)
		g.Count++
	}

	// Resolve member names in one pass.
	nameCache := map[uint]string{}
	result := make([]ActivityByMemberDTO, 0, len(order))
	for _, k := range order {
		g := grouped[k]
		name, ok := nameCache[g.MemberID]
		if !ok {
			member, err := u.memberRepo.FindByID(ctx, g.MemberID)
			if err == nil && member != nil {
				name = member.Name
			} else {
				name = "Unknown"
			}
			nameCache[g.MemberID] = name
		}
		g.Name = name
		result = append(result, *g)
	}

	return result, nil
}

// GetActivityLog returns a paginated, filterable raw activity log
// (newest first, member names resolved) - equivalent to the original
// NestJS/Express getActivityLog controller. Unlike GetActivityByMember,
// this returns one row per log entry rather than grouping by
// member+action.
func (u *ActivityUsecase) GetActivityLog(ctx context.Context, page, limit int, filter repository.ActivityFilter) (*ActivityLogResult, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	activities, total, err := u.activityRepo.FindPaginated(ctx, filter, page, limit)
	if err != nil {
		return nil, err
	}

	names := make(map[uint]string)
	data := make([]ActivityLogEntryDTO, 0, len(activities))
	for _, a := range activities {
		name, cached := names[a.MemberID]
		if !cached {
			if member, err := u.memberRepo.FindByID(ctx, a.MemberID); err == nil && member != nil {
				name = member.Name
			} else {
				name = "Unknown"
			}
			names[a.MemberID] = name
		}

		data = append(data, ActivityLogEntryDTO{
			ID:         a.ID,
			MemberName: name,
			Action:     a.Action,
			Feature:    a.Feature,
			Metadata:   a.Metadata,
			CreatedAt:  a.CreatedAt,
		})
	}

	return &ActivityLogResult{
		Data:       data,
		Pagination: PaginationDTO{Page: page, Limit: limit, Total: total},
	}, nil
}
