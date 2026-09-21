package usecase

import (
	"context"
	"encoding/json"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/internal/repository"
)

type PushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url,omitempty"`
}

type NotificationUsecase struct {
	pushRepo     repository.PushRepository
	vapidPublic  string
	vapidPrivate string
	vapidSubject string
}

func NewNotificationUsecase(pushRepo repository.PushRepository, vapidPublic, vapidPrivate, vapidSubject string) *NotificationUsecase {
	return &NotificationUsecase{
		pushRepo:     pushRepo,
		vapidPublic:  vapidPublic,
		vapidPrivate: vapidPrivate,
		vapidSubject: vapidSubject,
	}
}

func (u *NotificationUsecase) PublicKey() string {
	return u.vapidPublic
}

func (u *NotificationUsecase) Subscribe(ctx context.Context, memberID uint, endpoint, p256dh, auth string) error {
	return u.pushRepo.Save(ctx, &domain.PushSubscription{
		MemberID: memberID, Endpoint: endpoint, P256dh: p256dh, Auth: auth,
	})
}

func (u *NotificationUsecase) Unsubscribe(ctx context.Context, endpoint string) error {
	return u.pushRepo.DeleteByEndpoint(ctx, endpoint)
}

// SendToMember sends a push notification to every device a member has
// subscribed from. Best-effort: errors are logged, never returned - a
// failed push must never fail whatever business operation triggered it
// (payment recording, reminders, etc). Callers should invoke this via
// `go usecase.SendToMember(...)` so it doesn't add latency either.
func (u *NotificationUsecase) SendToMember(ctx context.Context, memberID uint, payload PushPayload) (bool, bool) {
	if u.vapidPrivate == "" {
		return false, false
	}

	subs, err := u.pushRepo.FindByMemberID(ctx, memberID)
	if err != nil {
		return false, false
	}

	if len(subs) == 0 {
		return false, false
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return false, true
	}

	sent := false

	for _, sub := range subs {
		resp, err := webpush.SendNotification(body, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256dh,
				Auth:   sub.Auth,
			},
		}, &webpush.Options{
			VAPIDPublicKey:  u.vapidPublic,
			VAPIDPrivateKey: u.vapidPrivate,
			Subscriber:      u.vapidSubject,
			TTL:             86400,
		})

		if err != nil {
			continue
		}

		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			sent = true
		}

		if resp.StatusCode == 404 || resp.StatusCode == 410 {
			_ = u.pushRepo.DeleteByEndpoint(ctx, sub.Endpoint)
		}
	}

	return sent, true
}
