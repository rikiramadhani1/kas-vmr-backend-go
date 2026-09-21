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
	log.Printf(
		"notification: mulai kirim push ke member_id=%d, title=%q",
		memberID,
		payload.Title,
	)

	if u.vapidPrivate == "" {
		log.Printf(
			"notification: VAPID private key belum dikonfigurasi, skip kirim ke member_id=%d",
			memberID,
		)
		return
	}

	if u.vapidPublic == "" {
		log.Printf(
			"notification: VAPID public key belum dikonfigurasi, skip kirim ke member_id=%d",
			memberID,
		)
		return
	}

	if u.vapidSubject == "" {
		log.Printf(
			"notification: VAPID subject belum dikonfigurasi, skip kirim ke member_id=%d",
			memberID,
		)
		return
	}

	log.Printf(
		"notification: mengambil subscription untuk member_id=%d",
		memberID,
	)

	subs, err := u.pushRepo.FindByMemberID(ctx, memberID)
	if err != nil {
		log.Printf(
			"notification: gagal ambil subscription member_id=%d: %v",
			memberID,
			err,
		)
		return
	}

	log.Printf(
		"notification: ditemukan %d subscription untuk member_id=%d",
		len(subs),
		memberID,
	)

	if len(subs) == 0 {
		log.Printf(
			"notification: tidak ada subscription aktif untuk member_id=%d, push tidak dikirim",
			memberID,
		)
		return
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf(
			"notification: gagal marshal payload member_id=%d: %v",
			memberID,
			err,
		)
		return
	}

	log.Printf(
		"notification: payload berhasil di-marshal untuk member_id=%d, size=%d bytes",
		memberID,
		len(body),
	)

	for i, sub := range subs {
		log.Printf(
			"notification: mengirim subscription #%d untuk member_id=%d, endpoint=%s",
			i+1,
			memberID,
			sub.Endpoint,
		)

		resp, err := webpush.SendNotification(
			body,
			&webpush.Subscription{
				Endpoint: sub.Endpoint,
				Keys: webpush.Keys{
					P256dh: sub.P256dh,
					Auth:   sub.Auth,
				},
			},
			&webpush.Options{
				VAPIDPublicKey:  u.vapidPublic,
				VAPIDPrivateKey: u.vapidPrivate,
				Subscriber:      u.vapidSubject,
				TTL:             86400,
			},
		)

		if err != nil {
			log.Printf(
				"notification: GAGAL kirim subscription #%d member_id=%d endpoint=%s error=%v",
				i+1,
				memberID,
				sub.Endpoint,
				err,
			)
			continue
		}

		log.Printf(
			"notification: push response member_id=%d subscription #%d status=%d",
			memberID,
			i+1,
			resp.StatusCode,
		)

		resp.Body.Close()

		if resp.StatusCode == 404 || resp.StatusCode == 410 {
			log.Printf(
				"notification: subscription sudah tidak valid, menghapus endpoint member_id=%d endpoint=%s",
				memberID,
				sub.Endpoint,
			)

			if err := u.pushRepo.DeleteByEndpoint(ctx, sub.Endpoint); err != nil {
				log.Printf(
					"notification: gagal menghapus subscription member_id=%d endpoint=%s error=%v",
					memberID,
					sub.Endpoint,
					err,
				)
			} else {
				log.Printf(
					"notification: subscription berhasil dihapus member_id=%d",
					memberID,
				)
			}
		}
	}

	log.Printf(
		"notification: selesai kirim push ke member_id=%d",
		memberID,
	)
}
