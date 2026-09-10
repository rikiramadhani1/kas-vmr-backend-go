package usecase

import (
	"context"
	"fmt"
	"log"
	"strings"
	"unicode"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/internal/repository"
	"github.com/vmr/kas-vmr-backend/pkg/mailwatcher"
)

// AutoConfirmUsecase turns a parsed SeaBank transfer-notification email
// into an approved payment, using the "nominal unik" scheme (see
// pkg/paymentcode) to figure out which member paid and for how many
// months - no OCR, no manual approval needed for transfers that match a
// known member's code.
//
// Transfers that DON'T decode to a known, active member (wrong amount, a
// donation, a typo, etc.) are logged as "unmatched" for the bendahara to
// review manually rather than silently dropped or guessed at.
type AutoConfirmUsecase struct {
	emailTxRepo    repository.EmailTransactionRepository
	memberRepo     repository.MemberRepository
	paymentUsecase *PaymentUsecase

	iuranAmount    float64
	uniqueCodeBase int
}

func NewAutoConfirmUsecase(
	emailTxRepo repository.EmailTransactionRepository,
	memberRepo repository.MemberRepository,
	paymentUsecase *PaymentUsecase,
	iuranAmount float64,
	uniqueCodeBase int,
) *AutoConfirmUsecase {
	return &AutoConfirmUsecase{
		emailTxRepo:    emailTxRepo,
		memberRepo:     memberRepo,
		paymentUsecase: paymentUsecase,
		iuranAmount:    iuranAmount,
		uniqueCodeBase: uniqueCodeBase,
	}
}

// HandleTransfer is the pkg/mailwatcher.Handler implementation: called
// once per recognized SeaBank transfer-masuk email.
// func (u *AutoConfirmUsecase) HandleTransfer(tx *mailwatcher.Transaction) error {
// 	ctx := context.Background()

// 	if tx.ReferenceNumber != "" {
// 		existing, err := u.emailTxRepo.FindByReference(ctx, tx.ReferenceNumber)
// 		if err != nil {
// 			return fmt.Errorf("cek duplikat referensi: %w", err)
// 		}
// 		if existing != nil {
// 			log.Printf("autoconfirm: referensi %q sudah pernah diproses (status=%s), dilewati", tx.ReferenceNumber, existing.Status)
// 			return nil
// 		}
// 	}

// 	memberID, months, ok := paymentcode.Decode(tx.Amount, u.iuranAmount, u.uniqueCodeBase)
// 	if !ok {
// 		return u.logUnmatched(ctx, tx, "nominal tidak cocok dengan skema kode unik manapun")
// 	}

// 	member, err := u.memberRepo.FindByID(ctx, memberID)
// 	if err != nil {
// 		return fmt.Errorf("cek member: %w", err)
// 	}
// 	if member == nil || member.Status != domain.MemberStatusActive {
// 		return u.logUnmatched(ctx, tx, fmt.Sprintf("kode unik %d tidak cocok member aktif manapun", memberID))
// 	}

// 	nominal := float64(months) * u.iuranAmount
// 	result, err := u.paymentUsecase.CreatePaymentByAdmin(ctx, memberID, nominal, "")
// 	if err != nil {
// 		return u.logUnmatched(ctx, tx, fmt.Sprintf("member #%d cocok tapi gagal membuat payment: %v", memberID, err))
// 	}

// 	log.Printf("autoconfirm: %s bayar %d bulan (Rp%.0f) via transfer email, ref=%s",
// 		member.Name, len(result.Payments), nominal, tx.ReferenceNumber)

// 	if tx.ReferenceNumber == "" {
// 		// Nothing to dedupe against next time, but the payment itself is
// 		// already booked - just skip the log entry.
// 		return nil
// 	}

// 	return u.emailTxRepo.Create(ctx, &domain.EmailTransactionLog{
// 		ReferenceNumber: tx.ReferenceNumber,
// 		MemberID:        &memberID,
// 		Amount:          tx.Amount,
// 		Status:          domain.EmailTxStatusProcessed,
// 	})
// }


func (u *AutoConfirmUsecase) HandleTransfer(tx *mailwatcher.Transaction) error {
	ctx := context.Background()

	if tx.ReferenceNumber != "" {
		existing, err := u.emailTxRepo.FindByReference(ctx, tx.ReferenceNumber)
		if err != nil {
			return fmt.Errorf("cek duplikat referensi: %w", err)
		}
		if existing != nil {
			log.Printf("autoconfirm: referensi %q sudah pernah diproses (status=%s), dilewati", tx.ReferenceNumber, existing.Status)
			return nil
		}
	}

	member, err := u.findMemberByNote(ctx, tx.Note)
	if err != nil {
		return fmt.Errorf("cek member dari catatan: %w", err)
	}
	if member == nil {
		return u.logUnmatched(ctx, tx, fmt.Sprintf("catatan %q tidak cocok nomor rumah member manapun", tx.Note))
	}

	if u.iuranAmount <= 0 || int64(tx.Amount)%int64(u.iuranAmount) != 0 {
		return u.logUnmatched(ctx, tx, fmt.Sprintf("member %s cocok tapi nominal Rp%.0f bukan kelipatan iuran", member.Name, tx.Amount))
	}

	result, err := u.paymentUsecase.CreatePaymentByAdmin(ctx, member.ID, tx.Amount, "")
	if err != nil {
		return u.logUnmatched(ctx, tx, fmt.Sprintf("member %s cocok tapi gagal membuat payment: %v", member.Name, err))
	}

	log.Printf("autoconfirm: %s bayar %d bulan (Rp%.0f) via transfer email, ref=%s",
		member.Name, len(result.Payments), tx.Amount, tx.ReferenceNumber)

	if tx.ReferenceNumber == "" {
		return nil
	}

	return u.emailTxRepo.Create(ctx, &domain.EmailTransactionLog{
		ReferenceNumber: tx.ReferenceNumber,
		MemberID:        &member.ID,
		Amount:          tx.Amount,
		Status:          domain.EmailTxStatusProcessed,
	})
}

// findMemberByNote memecah teks bebas "Catatan" jadi token, lalu cocokin
// tiap token sebagai nomor rumah. Ini penting: dengan tokenisasi (bukan
// substring match), catatan "Rumah 7", "No 7", atau "Blok A No 7" semua
// tetap kecocok ke member dengan house_number "7" - dan house_number "1"
// TIDAK bakal salah kecocok ke catatan yang isinya "12".
func (u *AutoConfirmUsecase) findMemberByNote(ctx context.Context, note string) (*domain.Member, error) {
	tokens := strings.FieldsFunc(note, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, token := range tokens {
		member, err := u.memberRepo.FindByHouseNumber(ctx, token)
		if err != nil {
			return nil, err
		}
		if member != nil && member.Status == domain.MemberStatusActive {
			return member, nil
		}
	}
	return nil, nil
}


// ListUnmatched returns transfers that didn't decode to any active
// member's unique code, for the bendahara to review and confirm
// manually (e.g. via /payments/admin-create).
func (u *AutoConfirmUsecase) ListUnmatched(ctx context.Context) ([]domain.EmailTransactionLog, error) {
	return u.emailTxRepo.FindUnmatched(ctx, 100)
}

func (u *AutoConfirmUsecase) logUnmatched(ctx context.Context, tx *mailwatcher.Transaction, reason string) error {
	log.Printf("autoconfirm: transaksi tidak cocok (ref=%s, amount=%.0f, pengirim=%s): %s",
		tx.ReferenceNumber, tx.Amount, tx.SenderName, reason)

	if tx.ReferenceNumber == "" {
		// Can't dedupe an entry with no reference number (the unique
		// index would collide on a second blank one) - the log line
		// above is the record for this case.
		return nil
	}

	return u.emailTxRepo.Create(ctx, &domain.EmailTransactionLog{
		ReferenceNumber: tx.ReferenceNumber,
		Amount:          tx.Amount,
		Status:          domain.EmailTxStatusUnmatched,
		Note:            &reason,
	})
}
