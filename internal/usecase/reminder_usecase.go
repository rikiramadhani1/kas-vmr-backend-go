package usecase

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// ReminderUsecase periodically reminds members who still owe dues, via
// push notification (see NotificationUsecase) - the direct successor to
// the original Node.js bot's node-cron based reminder broadcast, minus
// the WhatsApp bot dependency.
type ReminderUsecase struct {
	paymentUsecase      *PaymentUsecase
	notificationUsecase *NotificationUsecase
}

func NewReminderUsecase(paymentUsecase *PaymentUsecase, notificationUsecase *NotificationUsecase) *ReminderUsecase {
	return &ReminderUsecase{paymentUsecase: paymentUsecase, notificationUsecase: notificationUsecase}
}

// RunOnce sends one reminder push to every active member who currently
// has unpaid months.
func (u *ReminderUsecase) RunOnce(ctx context.Context) {
	log.Println("reminder: mulai proses RunOnce")

	unpaid, err := u.paymentUsecase.FindUnpaidMembers(ctx)
	if err != nil {
		log.Printf("reminder: gagal ambil daftar member menunggak: %v", err)
		return
	}

	log.Printf("reminder: ditemukan %d member yang menunggak", len(unpaid))

	if len(unpaid) == 0 {
		log.Println("reminder: tidak ada member yang menunggak, tidak ada reminder dikirim")
		return
	}

	for _, m := range unpaid {
		monthsList := strings.Join(m.MonthsDue, ", ")
		body := fmt.Sprintf(
			"Kamu belum bayar iuran untuk %d bulan (%s). Yuk segera dibayar! Jangan sampai menunggak lebih lama lagi ya.",
			m.Unpaid,
			monthsList,
		)

		log.Printf(
			"reminder: menyiapkan push untuk member_id=%d, unpaid=%d, months=%s",
			m.MemberID,
			m.Unpaid,
			monthsList,
		)

		u.notificationUsecase.SendToMember(ctx, m.MemberID, PushPayload{
			Title: "Pengingat Iuran Kas",
			Body:  body,
		})

		log.Printf(
			"reminder: selesai proses push untuk member_id=%d",
			m.MemberID,
		)
	}

	log.Printf(
		"reminder: RunOnce selesai, diproses %d member yang menunggak",
		len(unpaid),
	)
}

// StartScheduler blocks (run it in a goroutine) checking once a minute
// whether it's time to send the monthly reminder - i.e. today's date
// matches reminderDay and the current time matches reminderHour:reminderMinute
// (both in Asia/Jakarta time). Guards against sending more than once on
// the same day even if the per-minute check fires more than once within
// that same minute.
func (u *ReminderUsecase) StartScheduler(ctx context.Context, reminderDay, reminderHour, reminderMinute int) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.UTC
	}

	lastSentDate := ""
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Printf("reminder: scheduler aktif, jalan tiap tanggal %d jam %02d:%02d WIB", reminderDay, reminderHour, reminderMinute)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().In(loc)
			today := now.Format("2006-01-02")

			if now.Day() == reminderDay && now.Hour() == reminderHour && now.Minute() == reminderMinute && today != lastSentDate {
				u.RunOnce(ctx)
				lastSentDate = today
			}
		}
	}
}
