package usecase

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/gorm"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/internal/repository"
	"github.com/vmr/kas-vmr-backend/pkg/imagevalidator"
	"github.com/vmr/kas-vmr-backend/pkg/ocr"
	"github.com/vmr/kas-vmr-backend/pkg/receiptparser"
	"github.com/vmr/kas-vmr-backend/pkg/response"
)

type CountPaymentDTO struct {
	Unpaid    int      `json:"unpaid"`
	MonthsDue []string `json:"monthsDue"`
	PaidUntil *string  `json:"paidUntil,omitempty"`
}

type UnpaidMemberDTO struct {
	MemberID    uint     `json:"memberId"`
	Name        string   `json:"name"`
	HouseNumber *string  `json:"house_number,omitempty"`
	Unpaid      int      `json:"unpaid"`
	MonthsDue   []string `json:"monthsDue"`
}

type PaymentStatusDTO struct {
	MemberID    uint    `json:"memberId"`
	Name        string  `json:"name"`
	HouseNumber *string `json:"house_number,omitempty"`
	PaidUntil   *string `json:"paidUntil"`
}

type RecordTransactionResult struct {
	Transaksi *domain.Transaksi `json:"transaksi"`
	Months    int               `json:"months"`
	PaidUntil string            `json:"paidUntil"`
}

// PaymentUsecase holds a raw *gorm.DB (in addition to the "read" repos) so
// it can open explicit transactions for RecordTransaction, which
// atomically touches `transaksis`, `payments` (the cursor), and
// `cash_flows` together.
type PaymentUsecase struct {
	db                  *gorm.DB
	paymentRepo         repository.PaymentRepository
	transaksiRepo       repository.TransaksiRepository
	cashFlowRepo        repository.CashFlowRepository
	memberRepo          repository.MemberRepository
	notificationUsecase *NotificationUsecase

	amountPerMonth       float64
	defaultStartMonth    int
	defaultStartYear     int
	bendaharaNameKeyword string
}

func NewPaymentUsecase(
	db *gorm.DB,
	paymentRepo repository.PaymentRepository,
	transaksiRepo repository.TransaksiRepository,
	cashFlowRepo repository.CashFlowRepository,
	memberRepo repository.MemberRepository,
	notificationUsecase *NotificationUsecase,
	amountPerMonth float64,
	defaultStartMonth, defaultStartYear int,
	bendaharaNameKeyword string,
) *PaymentUsecase {
	return &PaymentUsecase{
		db:                   db,
		paymentRepo:          paymentRepo,
		transaksiRepo:        transaksiRepo,
		cashFlowRepo:         cashFlowRepo,
		memberRepo:           memberRepo,
		notificationUsecase:  notificationUsecase,
		amountPerMonth:       amountPerMonth,
		defaultStartMonth:    defaultStartMonth,
		defaultStartYear:     defaultStartYear,
		bendaharaNameKeyword: bendaharaNameKeyword,
	}
}

func (u *PaymentUsecase) description(month, year int) string {
	return fmt.Sprintf("Iuran bulan %s %d", MonthNameID(month), year)
}

// RecordTransaction is THE single entry point for booking a member's
// dues payment, regardless of how the money was detected: OCR upload
// (source=upload), the email auto-confirm worker (source=email,
// referenceNumber set), or a manual admin entry (source=admin). It:
//
//  1. Rejects the transaction as a duplicate if a transaction for the
//     same member, same amount, on the same calendar day already exists
//     (see TransaksiRepository.FindDuplicate) - this is what prevents a
//     transfer from being counted twice when it's picked up by BOTH the
//     email watcher and a member's manual proof upload.
//  2. Validates the amount is a whole multiple of the configured dues
//     amount.
//  3. Atomically (single DB transaction): records the Transaksi row,
//     advances the member's Payment cursor by however many months the
//     amount covers, and books each of those months into cash flow
//     (upserting by month description, with future months' CreatedAt set
//     to the 1st of that month rather than "now" - see
//     CashFlowRepository.UpsertByDescription).
//  4. Sends a push notification to the member once everything has
//     committed successfully.
func (u *PaymentUsecase) RecordTransaction(ctx context.Context, memberID uint, amount float64, source string, referenceNumber *string, transactionDate time.Time) (*RecordTransactionResult, error) {
	if u.amountPerMonth <= 0 {
		return nil, response.NewAPIError(500, "IURAN_AMOUNT tidak valid")
	}
	if int64(amount)%int64(u.amountPerMonth) != 0 {
		return nil, response.NewAPIError(400, fmt.Sprintf("nominal harus kelipatan %.0f", u.amountPerMonth))
	}
	months := int(amount / u.amountPerMonth)
	if months <= 0 {
		return nil, response.NewAPIError(400, "nominal tidak valid")
	}

	// Duplicate check happens BEFORE opening the booking transaction -
	// it's a read-only guard, not something that needs the same
	// atomicity/locking as the booking itself.
	dup, err := u.transaksiRepo.FindDuplicate(ctx, memberID, amount, transactionDate)
	if err != nil {
		log.Printf("[RecordTransaction] FindDuplicate ERROR: %v", err)
		return nil, err
	}
	if dup != nil {
		return nil, response.NewAPIError(409, fmt.Sprintf(
			"Transaksi dengan nominal Rp%.0f pada tanggal %s sepertinya sudah tercatat sebelumnya (sumber: %s). Kalau ini keliru, silakan hubungi admin.",
			amount, transactionDate.Format("02-01-2006"), dup.Source))
	}

	var result *RecordTransactionResult

	err = u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		paymentRepo := repository.NewPaymentRepository(tx)
		cashFlowRepo := repository.NewCashFlowRepository(tx)
		transaksiRepo := repository.NewTransaksiRepository(tx)

		cursor, err := paymentRepo.FindByMemberIDForUpdate(ctx, memberID)
		if err != nil {
			log.Printf("[RecordTransaction] FindByMemberIDForUpdate ERROR: %v", err)
			return err
		}

		hasPaid := cursor != nil && cursor.HasPaidAnything()
		fromMonth, fromYear := 0, 0
		if hasPaid {
			fromMonth, fromYear = cursor.PaidUntilMonth, cursor.PaidUntilYear
		}

		newMonth, newYear, paidMonths := AdvanceMonths(hasPaid, fromMonth, fromYear, u.defaultStartMonth, u.defaultStartYear, months)

		if err := paymentRepo.AdvanceCursor(ctx, memberID, newMonth, newYear); err != nil {
			log.Printf("[RecordTransaction] AdvanceCursor ERROR: %v", err)
			return err
		}

		for _, my := range paidMonths {
			desc := u.description(my.Month, my.Year)
			forDate := time.Date(my.Year, time.Month(my.Month), 1, 0, 0, 0, 0, time.UTC)
			if _, err := cashFlowRepo.UpsertByDescription(ctx, domain.CashFlowTypeIn, domain.CashFlowSourceDues, desc, u.amountPerMonth, forDate); err != nil {
				return err
			}
		}

		t := &domain.Transaksi{
			MemberID:        memberID,
			Amount:          amount,
			Months:          months,
			Source:          source,
			ReferenceNumber: referenceNumber,
			TransactionDate: transactionDate,
		}
		if err := transaksiRepo.Create(ctx, t); err != nil {
			return err
		}

		result = &RecordTransactionResult{
			Transaksi: t,
			Months:    months,
			PaidUntil: fmt.Sprintf("%s %d", MonthNameID(newMonth), newYear),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Push notification happens AFTER the transaction commits - sending
	// it from inside the closure above would risk telling the member
	// "sudah tercatat" for a booking that then gets rolled back by a
	// later error in the same transaction.
	if u.notificationUsecase != nil {
		go u.notificationUsecase.SendToMember(context.Background(), memberID, PushPayload{
			Title: "Pembayaran Kas Berhasil",
			Body: fmt.Sprintf(
				"Terima kasih! Pembayaran kas untuk %d bulan sudah tercatat, lunas sampai %s.",
				result.Months, result.PaidUntil,
			),
		})
	}

	return result, nil
}

// CreateByAdmin lets an admin directly record a payment (e.g. cash
// handed in person) - a thin wrapper over RecordTransaction with
// source=admin and today as the transaction date.
func (u *PaymentUsecase) CreateByAdmin(ctx context.Context, memberID uint, nominal float64) (*RecordTransactionResult, error) {
	return u.RecordTransaction(ctx, memberID, nominal, domain.TransaksiSourceAdmin, nil, time.Now().UTC())
}

// GetRecentByMemberID returns a member's most recent transactions
// (default 5) - equivalent to the original "riwayat pembayaran" list,
// now backed by the Transaksi table instead of per-month Payment rows.
func (u *PaymentUsecase) GetRecentByMemberID(ctx context.Context, memberID uint, limit int) ([]domain.Transaksi, error) {
	if limit <= 0 {
		limit = 5
	}
	return u.transaksiRepo.FindLatestByMemberID(ctx, memberID, limit)
}

// GetRecentByPhone resolves phone -> member (via the shared
// phoneutil-normalized lookup) then returns their recent transactions.
func (u *PaymentUsecase) GetRecentByPhone(ctx context.Context, phone string, limit int) ([]domain.Transaksi, error) {
	member, err := u.memberRepo.FindByPhoneOrSpouse(ctx, phone)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return []domain.Transaksi{}, nil
	}
	return u.GetRecentByMemberID(ctx, member.ID, limit)
}

// CountPayment reports how many months a member currently owes.
func (u *PaymentUsecase) CountPayment(ctx context.Context, memberID uint) (*CountPaymentDTO, error) {
	cursor, err := u.paymentRepo.FindByMemberID(ctx, memberID)
	if err != nil {
		return nil, err
	}

	hasPaid := cursor != nil && cursor.HasPaidAnything()
	cursorMonth, cursorYear := 0, 0
	if hasPaid {
		cursorMonth, cursorYear = cursor.PaidUntilMonth, cursor.PaidUntilYear
	}

	now := jakartaNow()
	unpaid := CalculateUnpaidMonths(hasPaid, cursorMonth, cursorYear, u.defaultStartMonth, u.defaultStartYear, now)

	monthsDue := make([]string, 0, len(unpaid))
	for _, my := range unpaid {
		monthsDue = append(monthsDue, fmt.Sprintf("%s %d", MonthNameID(my.Month), my.Year))
	}

	dto := &CountPaymentDTO{Unpaid: len(unpaid), MonthsDue: monthsDue}
	if hasPaid {
		paidUntil := fmt.Sprintf("%s %d", MonthNameID(cursorMonth), cursorYear)
		dto.PaidUntil = &paidUntil
	}
	return dto, nil
}

// FindUnpaidMembers lists every active member with at least one unpaid
// month.
func (u *PaymentUsecase) FindUnpaidMembers(ctx context.Context) ([]UnpaidMemberDTO, error) {
	members, err := u.memberRepo.FindAllActive(ctx)
	if err != nil {
		return nil, err
	}

	cursors, err := u.paymentRepo.FindAllActive(ctx)
	if err != nil {
		return nil, err
	}
	cursorByMember := make(map[uint]domain.Payment, len(cursors))
	for _, c := range cursors {
		cursorByMember[c.MemberID] = c
	}

	now := jakartaNow()
	result := make([]UnpaidMemberDTO, 0)

	for _, m := range members {
		cursor, hasPaid := cursorByMember[m.ID]
		hasPaid = hasPaid && cursor.HasPaidAnything()

		cursorMonth, cursorYear := 0, 0
		if hasPaid {
			cursorMonth, cursorYear = cursor.PaidUntilMonth, cursor.PaidUntilYear
		}

		unpaid := CalculateUnpaidMonths(hasPaid, cursorMonth, cursorYear, u.defaultStartMonth, u.defaultStartYear, now)
		if len(unpaid) == 0 {
			continue
		}

		monthsDue := make([]string, 0, len(unpaid))
		for _, my := range unpaid {
			monthsDue = append(monthsDue, fmt.Sprintf("%s %d", MonthNameID(my.Month), my.Year))
		}

		result = append(result, UnpaidMemberDTO{
			MemberID:    m.ID,
			Name:        m.Name,
			HouseNumber: m.HouseNumber,
			Unpaid:      len(unpaid),
			MonthsDue:   monthsDue,
		})
	}

	return result, nil
}

// FindPaymentStatus lists every active member who HAS paid up to the
// current month (the counterpart to FindUnpaidMembers) - members who
// have never paid, or who are behind, are excluded here (they show up
// in FindUnpaidMembers instead).
func (u *PaymentUsecase) FindPaymentStatus(ctx context.Context) ([]PaymentStatusDTO, error) {
	members, err := u.memberRepo.FindAllActive(ctx)
	if err != nil {
		return nil, err
	}

	cursors, err := u.paymentRepo.FindAllActive(ctx)
	if err != nil {
		return nil, err
	}
	cursorByMember := make(map[uint]domain.Payment, len(cursors))
	for _, c := range cursors {
		cursorByMember[c.MemberID] = c
	}

	now := jakartaNow()
	result := make([]PaymentStatusDTO, 0)

	for _, m := range members {
		cursor, ok := cursorByMember[m.ID]
		hasPaid := ok && cursor.HasPaidAnything()
		if !hasPaid {
			continue
		}

		unpaid := CalculateUnpaidMonths(true, cursor.PaidUntilMonth, cursor.PaidUntilYear, u.defaultStartMonth, u.defaultStartYear, now)
		if len(unpaid) > 0 {
			continue // masih nunggak, biar muncul di FindUnpaidMembers aja
		}

		paidUntil := fmt.Sprintf("%s %d", MonthNameID(cursor.PaidUntilMonth), cursor.PaidUntilYear)
		result = append(result, PaymentStatusDTO{
			MemberID:    m.ID,
			Name:        m.Name,
			HouseNumber: m.HouseNumber,
			PaidUntil:   &paidUntil,
		})
	}

	return result, nil
}

// CreateByProof runs OCR on an uploaded payment-proof screenshot,
// validates it, extracts the transferred amount and date, and records
// the transaction via RecordTransaction (which handles duplicate
// detection, cursor advancement, cash flow booking, and the push
// notification).
func (u *PaymentUsecase) CreateByProof(ctx context.Context, memberID uint, imagePath string) (*RecordTransactionResult, error) {
	if err := imagevalidator.RejectIfCameraPhoto(imagePath); err != nil {
		return nil, response.NewAPIError(400, err.Error())
	}

	cleanPath := imagePath + "_clean.jpg"
	if err := ocr.Preprocess(imagePath, cleanPath); err != nil {
		return nil, response.NewAPIError(400, "Gagal memproses gambar")
	}
	defer os.Remove(cleanPath)

	rawText, err := ocr.Recognize(cleanPath, "ind+eng")
	if err != nil {
		return nil, response.NewAPIError(500, "Gagal membaca teks dari gambar")
	}

	cleanedText := receiptparser.CleanText(rawText)

	if !receiptparser.ContainsName(cleanedText, u.bendaharaNameKeyword) {
		return nil, response.NewAPIError(400, "Bukti transfer tidak valid, silahkan hubungi bendahara")
	}

	nominal, err := receiptparser.ExtractNominal(cleanedText)
	if err != nil {
		return nil, response.NewAPIError(400, "Nominal pembayaran tidak terbaca, silahkan hubungi bendahara")
	}

	transactionDate := jakartaNow()
	if dateStr := receiptparser.ExtractDate(cleanedText); dateStr != "" {
		if parsed, err := receiptparser.ParseReceiptDate(dateStr, transactionDate.Location()); err == nil {
			transactionDate = parsed
		}
	}

	return u.RecordTransaction(ctx, memberID, nominal, domain.TransaksiSourceUpload, nil, transactionDate)
}

// jakartaNow returns the current time in the Asia/Jakarta timezone.
// Falls back to plain UTC if the tzdata database isn't available on the
// host.
func jakartaNow() time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.Now().UTC()
	}
	return time.Now().In(loc)
}
