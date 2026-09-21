package usecase

import (
	"context"
	"encoding/json"
	"log"

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
func (u *NotificationUsecase) SendToMember(ctx context.Context, memberID uint, payload PushPayload) {
	if u.vapidPrivate == "" {
		log.Printf("notification: VAPID belum dikonfigurasi, skip kirim ke member %d", memberID)
		return
	}

	subs, err := u.pushRepo.FindByMemberID(ctx, memberID)
	if err != nil {
		log.Printf("notification: gagal ambil subscription member %d: %v", memberID, err)
		return
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("notification: gagal marshal payload: %v", err)
		return
	}

	for _, sub := range subs {
		log.Printf("notification: sebelum SendNotification member_id=%d", memberID)

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
			log.Printf("notification: gagal kirim ke endpoint %s: %v", sub.Endpoint, err)
			continue
		}

		log.Printf(
			"notification: SendNotification berhasil member_id=%d status=%d",
			memberID,
			resp.StatusCode,
		)

		resp.Body.Close()

		if resp.StatusCode == 404 || resp.StatusCode == 410 {
			_ = u.pushRepo.DeleteByEndpoint(ctx, sub.Endpoint)
		}
	}
}
