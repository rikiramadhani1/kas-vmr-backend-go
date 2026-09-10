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
	URL   string `json:"url,omitempty"` // dibuka kalau notif diklik
}

type NotificationUsecase struct {
	pushRepo    repository.PushRepository
	vapidPublic string
	vapidPrivate string
	vapidSubject string
}

func NewNotificationUsecase(pushRepo repository.PushRepository, vapidPublic, vapidPrivate, vapidSubject string) *NotificationUsecase {
	return &NotificationUsecase{
		pushRepo: pushRepo, vapidPublic: vapidPublic,
		vapidPrivate: vapidPrivate, vapidSubject: vapidSubject,
	}
}

func (u *NotificationUsecase) Subscribe(ctx context.Context, memberID uint, endpoint, p256dh, auth string) error {
	return u.pushRepo.Save(ctx, &domain.PushSubscription{
		MemberID: memberID, Endpoint: endpoint, P256dh: p256dh, Auth: auth,
	})
}

// SendToMember mengirim push ke semua device milik member. Dijalankan
// sebagai best-effort (dipanggil via goroutine oleh caller) - gagal kirim
// push TIDAK BOLEH menggagalkan proses approve payment yang memicunya.
func (u *NotificationUsecase) SendToMember(ctx context.Context, memberID uint, payload PushPayload) {
	subs, err := u.pushRepo.FindByMemberID(ctx, memberID)
	if err != nil {
		log.Printf("notification: gagal ambil subscription member %d: %v", memberID, err)
		return
	}

	body, _ := json.Marshal(payload)

	for _, sub := range subs {
		resp, err := webpush.SendNotification(body, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
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
		resp.Body.Close()

		// 404/410 artinya subscription udah gak valid (uninstall app,
		// clear browser data, dll) - bersihin biar gak dicoba lagi.
		if resp.StatusCode == 404 || resp.StatusCode == 410 {
			_ = u.pushRepo.DeleteByEndpoint(ctx, sub.Endpoint)
		}
	}
}