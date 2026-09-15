package usecase

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/internal/repository"
	"github.com/vmr/kas-vmr-backend/pkg/mailwatcher"
)

// AutoConfirmUsecase turns a parsed SeaBank transfer-notification email
// into a recorded payment.
//
// Members are identified by house number written in the transfer's
// "Catatan" (note) field - NOT by sender bank account, because a member
// might transfer using a spouse's or friend's account. Every member is
// asked to always fill in their house number in the note, regardless of
// whose account they transfer from.
//
// Transfers whose note doesn't match any active member's house number
// (wrong/missing note, a donation, etc.) are logged as "unmatched" for
// the bendahara to review manually (e.g. via /payments/admin-create).
type AutoConfirmUsecase struct {
	emailTxRepo    repository.EmailTransactionRepository
	memberRepo     repository.MemberRepository
	paymentUsecase *PaymentUsecase
}

func NewAutoConfirmUsecase(
	emailTxRepo repository.EmailTransactionRepository,
	memberRepo repository.MemberRepository,
	paymentUsecase *PaymentUsecase,
) *AutoConfirmUsecase {
	return &AutoConfirmUsecase{
		emailTxRepo:    emailTxRepo,
		memberRepo:     memberRepo,
		paymentUsecase: paymentUsecase,
	}
}

// HandleTransfer is the pkg/mailwatcher.Handler implementation: called
// once per recognized SeaBank transfer-masuk email.
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

	// RecordTransaction handles its own duplicate check (by member+amount
	// +day, via TransaksiRepository.FindDuplicate) - this is what catches
	// the exact scenario the email watcher and a member's manual proof
	// upload could otherwise both record: the SAME real transfer.
	result, err := u.paymentUsecase.RecordTransaction(ctx, member.ID, tx.Amount, domain.TransaksiSourceEmail, &tx.ReferenceNumber, transactionTimeOrNow(tx))
	if err != nil {
		return u.logUnmatched(ctx, tx, fmt.Sprintf("member %s cocok tapi gagal mencatat transaksi: %v", member.Name, err))
	}

	log.Printf("autoconfirm: %s bayar %d bulan (Rp%.0f) via transfer email, ref=%s, lunas sampai %s",
		member.Name, result.Months, tx.Amount, tx.ReferenceNumber, result.PaidUntil)

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

// findMemberByNote tokenizes the free-text "Catatan" field and tries
// each token as a house number. Tokenizing (rather than a substring
// match) matters: a note like "Rumah 7" or "Blok A No 7" should match
// house_number "7" - but house_number "1" must NOT wrongly match a note
// that says "12".
// func (u *AutoConfirmUsecase) findMemberByNote(ctx context.Context, note string) (*domain.Member, error) {
// 	tokens := strings.FieldsFunc(note, func(r rune) bool {
// 		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
// 	})
// 	for _, token := range tokens {
// 		member, err := u.memberRepo.FindByHouseNumber(ctx, token)
// 		if err != nil {
// 			return nil, err
// 		}
// 		if member != nil && member.Status == domain.MemberStatusActive {
// 			return member, nil
// 		}
// 	}
// 	return nil, nil
// }

var houseNumberPattern = regexp.MustCompile(`^\d+[A-Za-z]?$`)
// findMemberByNote requires the note to be EXACTLY a house number per
// the house numbering standard (digits + optional single letter, e.g.
// "12A"). This is deliberately strict: house numbers like "12" and
// "12A" can be different, unrelated members, so any ambiguity (extra
// words, a stray space splitting the letter off, wrong format) must
// fall through to unmatched for manual bendahara review - silently
// guessing risks crediting the wrong member's payment.
func (u *AutoConfirmUsecase) findMemberByNote(ctx context.Context, note string) (*domain.Member, error) {
	trimmed := strings.TrimSpace(note)

	if !houseNumberPattern.MatchString(trimmed) {
		return nil, nil
	}

	member, err := u.memberRepo.FindByHouseNumber(ctx, trimmed)
	if err != nil {
		return nil, err
	}
	if member != nil && member.Status == domain.MemberStatusActive {
		return member, nil
	}
	return nil, nil
}

// ListUnmatched returns transfers that didn't decode to any active
// member's house number, for the bendahara to review and confirm
// manually.
func (u *AutoConfirmUsecase) ListUnmatched(ctx context.Context) ([]domain.EmailTransactionLog, error) {
	return u.emailTxRepo.FindUnmatched(ctx, 100)
}

// transactionTimeOrNow parses SeaBank's "Waktu Transaksi" format (e.g.
// "08 Sep 2026 18:01") into an Asia/Jakarta time.Time, falling back to
// the current time if the email's format ever changes and parsing
// fails - the transaction still gets recorded either way, just with a
// less precise date for duplicate-detection purposes.
func transactionTimeOrNow(tx *mailwatcher.Transaction) time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.UTC
	}
	if t, err := time.ParseInLocation("02 Jan 2006 15:04", tx.Time, loc); err == nil {
		return t
	}
	return time.Now().In(loc)
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
