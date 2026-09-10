package usecase

import (
	"context"
	"fmt"
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
	Unpaid      int      `json:"unpaid"`
	Pending     int      `json:"pending"`
	MonthsDue   []string `json:"monthsDue"`
	Overpayment int      `json:"overpayment"`
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
	PaidUntil   string  `json:"paidUntil"`
}

type CreatePaymentResult struct {
	Nominal  float64          `json:"nominal"`
	Months   int              `json:"months"`
	Payments []domain.Payment `json:"payments"`
}

type ApprovePaymentResult struct {
	Payment *domain.Payment `json:"payment"`
	Message string          `json:"message"`
}

// PaymentUsecase holds a raw *gorm.DB (in addition to the "read" repos) so
// it can open explicit transactions for the operations that need to
// atomically touch both `payments` and `cash_flows` - fixing the
// non-atomic create/approve flows in the original Node.js code, where a
// crash partway through could leave a payment recorded without its
// corresponding cash flow entry (or vice versa).
type PaymentUsecase struct {
	db            *gorm.DB
	paymentRepo   repository.PaymentRepository
	cashFlowRepo  repository.CashFlowRepository
	memberRepo    repository.MemberRepository
	logSignTfRepo repository.LogSignTfRepository
	notificationUsecase *NotificationUsecase

	amountPerMonth       float64
	defaultStartMonth    int
	defaultStartYear     int
	bendaharaNameKeyword string
}

func NewPaymentUsecase(
	db *gorm.DB,
	paymentRepo repository.PaymentRepository,
	cashFlowRepo repository.CashFlowRepository,
	memberRepo repository.MemberRepository,
	logSignTfRepo repository.LogSignTfRepository,
	notificationUsecase *NotificationUsecase,
	amountPerMonth float64,
	defaultStartMonth, defaultStartYear int,
	bendaharaNameKeyword string,
) *PaymentUsecase {
	return &PaymentUsecase{
		db:                   db,
		paymentRepo:          paymentRepo,
		cashFlowRepo:         cashFlowRepo,
		memberRepo:           memberRepo,
		logSignTfRepo:        logSignTfRepo,
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

// CreatePaymentRequest lets a member request to pay `n` upcoming months
// (as *pending* payments, awaiting admin approval).
//
// The original Node.js version had a hardcoded `if (member_id > 15) throw`
// guard here - almost certainly a leftover sanity check from early
// development/seed data that would break as soon as membership grew past
// 15 people. We replace it with a real check: the member must actually
// exist and be active.
func (u *PaymentUsecase) CreatePaymentRequest(ctx context.Context, memberID uint, n int) ([]domain.Payment, error) {
	member, err := u.memberRepo.FindByID(ctx, memberID)
	if err != nil {
		return nil, err
	}
	if member == nil || member.Status != domain.MemberStatusActive {
		return nil, response.NewAPIError(400, "Member tidak ditemukan atau tidak aktif")
	}

	pending, err := u.paymentRepo.FindPendingByMemberID(ctx, memberID)
	if err != nil {
		return nil, err
	}
	if len(pending) > 0 {
		months := make([]string, 0, len(pending))
		for _, p := range pending {
			months = append(months, fmt.Sprintf("%d/%d", p.Month, p.Year))
		}
		return nil, response.NewAPIError(400, fmt.Sprintf(
			"Kamu masih memiliki pembayaran pending (Bulan: %v). Silakan tunggu hingga disetujui atau dibatalkan.", months))
	}

	last, err := u.paymentRepo.FindLastByMemberID(ctx, memberID)
	if err != nil {
		return nil, err
	}

	startMonth, startYear := u.defaultStartMonth, u.defaultStartYear
	if last != nil {
		startMonth = last.Month + 1
		startYear = last.Year
		if startMonth > 12 {
			startMonth = 1
			startYear++
		}
	}

	monthsToPay := []domain.MonthYear{{Month: startMonth, Year: startYear}}
	monthsToPay = append(monthsToPay, NextMonthsAfter(startMonth, startYear, n-1)...)

	payments := make([]domain.Payment, 0, n)
	for _, my := range monthsToPay {
		payments = append(payments, domain.Payment{
			MemberID: memberID,
			Month:    my.Month,
			Year:     my.Year,
			Amount:   u.amountPerMonth,
			Status:   domain.PaymentStatusPending,
		})
	}

	if err := u.paymentRepo.CreateMany(ctx, payments); err != nil {
		return nil, err
	}
	return payments, nil
}

// GetAllByPhone resolves phone -> member (via the shared, single
// normalization rule in pkg/phoneutil - see FindByPhoneOrSpouse) and
// returns their 5 most recent payments. Equivalent to the original
// getAllPaymentsService -> repo.getTransaksiTerakhirByPhone(phone).
func (u *PaymentUsecase) GetAllByPhone(ctx context.Context, phone string) ([]domain.Payment, error) {
	member, err := u.memberRepo.FindByPhoneOrSpouse(ctx, phone)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return []domain.Payment{}, nil
	}
	return u.paymentRepo.FindLatestByMemberID(ctx, member.ID, 5)
}

func (u *PaymentUsecase) GetPending(ctx context.Context) ([]domain.Payment, error) {
	return u.paymentRepo.FindPending(ctx)
}

func (u *PaymentUsecase) GetAll(ctx context.Context) ([]domain.Payment, error) {
	return u.paymentRepo.FindAll(ctx)
}

// ApprovePayment approves a pending payment and books it into the cash
// flow, atomically.
//
// The original Node.js implementation did this as two independent
// operations (update payment status, then separately read-modify-write
// the matching CashFlow row) with no transaction and no row locking.
// Under concurrent approvals of two payments for the same month, both
// could read the same "existing amount" before either write lands,
// causing one update to silently overwrite the other (a lost update).
// Wrapping both steps in a single serializable-ish transaction with a
// `SELECT ... FOR UPDATE` lock (see CashFlowRepository.UpsertByDescription)
// closes that gap.
func (u *PaymentUsecase) ApprovePayment(ctx context.Context, id uint) (*ApprovePaymentResult, error) {
	var result *ApprovePaymentResult

	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		paymentRepo := repository.NewPaymentRepository(tx)
		cashFlowRepo := repository.NewCashFlowRepository(tx)

		payment, err := paymentRepo.FindByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if payment == nil {
			return response.NewAPIError(404, "Payment not found")
		}
		if payment.Status != domain.PaymentStatusPending {
			return response.NewAPIError(400, "Payment already processed")
		}

		if err := paymentRepo.UpdateStatus(ctx, id, domain.PaymentStatusApproved); err != nil {
			return err
		}

		desc := u.description(payment.Month, payment.Year)
		if _, err := cashFlowRepo.UpsertByDescription(ctx, domain.CashFlowTypeIn, domain.CashFlowSourceDues, desc, payment.Amount); err != nil {
			return err
		}

		payment.Status = domain.PaymentStatusApproved
		result = &ApprovePaymentResult{Payment: payment, Message: "Payment approved and cash flow recorded/updated"}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (u *PaymentUsecase) RejectPayment(ctx context.Context, id uint) (*ApprovePaymentResult, error) {
	var result *ApprovePaymentResult

	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		paymentRepo := repository.NewPaymentRepository(tx)

		payment, err := paymentRepo.FindByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if payment == nil {
			return response.NewAPIError(404, "Payment not found")
		}
		if payment.Status != domain.PaymentStatusPending {
			return response.NewAPIError(400, "Payment already processed")
		}

		if err := paymentRepo.UpdateStatus(ctx, id, domain.PaymentStatusRejected); err != nil {
			return err
		}

		payment.Status = domain.PaymentStatusRejected
		result = &ApprovePaymentResult{Payment: payment, Message: "Payment rejected"}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CountPayment reports how many months a member owes, using the single
// shared CalculateUnpaidMonths function (see dues.go) instead of the
// separate, less-accurate arithmetic the original countPaymentService used.
func (u *PaymentUsecase) CountPayment(ctx context.Context, memberID uint) (*CountPaymentDTO, error) {
	approved, err := u.paymentRepo.FindApprovedByMemberID(ctx, memberID)
	if err != nil {
		return nil, err
	}
	pending, err := u.paymentRepo.FindPendingByMemberID(ctx, memberID)
	if err != nil {
		return nil, err
	}

	now := jakartaNow()
	unpaid, lastPaid := CalculateUnpaidMonths(approved, u.defaultStartMonth, u.defaultStartYear, now)

	monthsDue := make([]string, 0, len(unpaid))
	for _, my := range unpaid {
		monthsDue = append(monthsDue, fmt.Sprintf("%s %d", MonthNameID(my.Month), my.Year))
	}

	overpayment := 0
	if lastPaid != nil {
		totalPaidMonths := lastPaid.Year*12 + lastPaid.Month
		totalCurrentMonths := now.Year()*12 + int(now.Month())
		if diff := totalPaidMonths - totalCurrentMonths; diff > 0 {
			overpayment = diff
		}
	}

	return &CountPaymentDTO{
		Unpaid:      len(unpaid),
		Pending:     len(pending),
		MonthsDue:   monthsDue,
		Overpayment: overpayment,
	}, nil
}

// FindUnpaidMembers lists every active member with at least one unpaid
// month, using the same unified calculation as CountPayment.
func (u *PaymentUsecase) FindUnpaidMembers(ctx context.Context) ([]UnpaidMemberDTO, error) {
	members, err := u.memberRepo.FindAllActive(ctx)
	if err != nil {
		return nil, err
	}

	now := jakartaNow()
	result := make([]UnpaidMemberDTO, 0)

	for _, m := range members {
		approved, err := u.paymentRepo.FindApprovedByMemberID(ctx, m.ID)
		if err != nil {
			return nil, err
		}
		unpaid, _ := CalculateUnpaidMonths(approved, u.defaultStartMonth, u.defaultStartYear, now)
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

// FindPaymentStatus mengembalikan member yang sudah lunas sampai bulan
// berjalan (kebalikan dari FindUnpaidMembers), pakai kalkulasi yang sama
// (CalculateUnpaidMonths) biar konsisten - member yang belum pernah bayar
// sama sekali di-skip di sini (mereka sudah muncul di FindUnpaidMembers).
func (u *PaymentUsecase) FindPaymentStatus(ctx context.Context) ([]PaymentStatusDTO, error) {
	members, err := u.memberRepo.FindAllActive(ctx)
	if err != nil {
		return nil, err
	}

	now := jakartaNow()
	result := make([]PaymentStatusDTO, 0)

	for _, m := range members {
		approved, err := u.paymentRepo.FindApprovedByMemberID(ctx, m.ID)
		if err != nil {
			return nil, err
		}

		unpaid, lastPaid := CalculateUnpaidMonths(approved, u.defaultStartMonth, u.defaultStartYear, now)
		if lastPaid == nil || len(unpaid) > 0 {
			continue
		}

		result = append(result, PaymentStatusDTO{
			MemberID:    m.ID,
			Name:        m.Name,
			HouseNumber: m.HouseNumber,
			PaidUntil:   fmt.Sprintf("%s %d", MonthNameID(lastPaid.Month), lastPaid.Year),
		})
	}

	return result, nil
}

// CreatePaymentByAdmin lets an admin directly record an approved payment
// for a member (e.g. cash handed in person), optionally with a signature
// hash if it originated from a proof image.
func (u *PaymentUsecase) CreatePaymentByAdmin(ctx context.Context, memberID uint, nominal float64, sign string) (*CreatePaymentResult, error) {
	if u.amountPerMonth <= 0 {
		return nil, response.NewAPIError(500, "IURAN_AMOUNT tidak valid")
	}
	if int64(nominal)%int64(u.amountPerMonth) != 0 {
		return nil, response.NewAPIError(400, fmt.Sprintf("nominal harus kelipatan %.0f", u.amountPerMonth))
	}
	months := int(nominal / u.amountPerMonth)

	var result *CreatePaymentResult
	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		paymentRepo := repository.NewPaymentRepository(tx)
		cashFlowRepo := repository.NewCashFlowRepository(tx)
		logSignRepo := repository.NewLogSignTfRepository(tx)

		payments, err := u.createApprovedPaymentsTx(ctx, paymentRepo, cashFlowRepo, memberID, months)
		if err != nil {
			return err
		}

		if sign != "" {
			if err := logSignRepo.Create(ctx, memberID, nominal, sign); err != nil {
				return err
			}
		}

		result = &CreatePaymentResult{Nominal: nominal, Months: months, Payments: payments}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Kirim push notif SETELAH transaksi sukses commit - kalau ditaro di
	// dalam closure di atas, member bisa kekirim notif "sudah bayar"
	// padahal transaksinya masih mungkin di-rollback oleh error setelahnya.
	if u.notificationUsecase != nil && len(result.Payments) > 0 {
		last := result.Payments[len(result.Payments)-1]
		go u.notificationUsecase.SendToMember(context.Background(), memberID, PushPayload{
			Title: "Pembayaran Kas Berhasil",
			Body: fmt.Sprintf(
				"Terima kasih! Pembayaran kas untuk %d bulan sudah tercatat, lunas sampai %s %d.",
				len(result.Payments), MonthNameID(last.Month), last.Year,
			),
		})
	}

	return result, nil
}

// createApprovedPaymentsTx books `months` consecutive approved payments
// starting right after the member's last payment (or the configured
// default start), and upserts the corresponding cash flow entries - all
// within the caller's transaction.
func (u *PaymentUsecase) createApprovedPaymentsTx(ctx context.Context, paymentRepo repository.PaymentRepository, cashFlowRepo repository.CashFlowRepository, memberID uint, months int) ([]domain.Payment, error) {
	last, err := paymentRepo.FindLastByMemberID(ctx, memberID)
	if err != nil {
		return nil, err
	}

	startMonth, startYear := u.defaultStartMonth, u.defaultStartYear
	if last != nil {
		startMonth = last.Month + 1
		startYear = last.Year
		if startMonth > 12 {
			startMonth = 1
			startYear++
		}
	}

	monthsToPay := []domain.MonthYear{{Month: startMonth, Year: startYear}}
	monthsToPay = append(monthsToPay, NextMonthsAfter(startMonth, startYear, months-1)...)

	created := make([]domain.Payment, 0, months)
	for _, my := range monthsToPay {
		p := domain.Payment{
			MemberID: memberID,
			Month:    my.Month,
			Year:     my.Year,
			Amount:   u.amountPerMonth,
			Status:   domain.PaymentStatusApproved,
		}
		if err := paymentRepo.Create(ctx, &p); err != nil {
			return nil, err
		}
		desc := u.description(my.Month, my.Year)
		if _, err := cashFlowRepo.UpsertByDescription(ctx, domain.CashFlowTypeIn, domain.CashFlowSourceDues, desc, u.amountPerMonth); err != nil {
			return nil, err
		}
		created = append(created, p)
	}

	return created, nil
}

// CreatePaymentByProof runs OCR on an uploaded payment-proof screenshot,
// validates it, extracts the transferred amount, and (if everything
// checks out) auto-approves the corresponding payment(s).
//
// Preserves the original heuristics (reject camera photos, check edge
// density, match treasurer name, dedupe by content signature) but reads
// the treasurer name from configuration instead of a hardcoded literal,
// and books the resulting payments atomically (see createApprovedPaymentsTx).
func (u *PaymentUsecase) CreatePaymentByProof(ctx context.Context, memberID uint, imagePath string) (*CreatePaymentResult, error) {
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

	if u.amountPerMonth <= 0 || int64(nominal)%int64(u.amountPerMonth) != 0 {
		return nil, response.NewAPIError(400, fmt.Sprintf(
			"Nominal Rp%.0f bukan kelipatan iuran Rp%.0f", nominal, u.amountPerMonth))
	}

	dateStr := receiptparser.ExtractDate(cleanedText)
	sign := receiptparser.Signature(nominal, dateStr, cleanedText)

	existing, err := u.logSignTfRepo.FindBySignatureHash(ctx, sign)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, response.NewAPIError(400, "Bukti Transfer sudah pernah dikirim sebelumnya")
	}

	return u.CreatePaymentByAdmin(ctx, memberID, nominal, sign)
}

// jakartaNow returns the current time in the Asia/Jakarta timezone,
// matching the original countPaymentService's explicit timezone handling
// (and this project's past struggles with inconsistent timezone
// handling - see member notes on the authV2 UTC standardization work).
// Falls back to plain UTC if the tzdata database isn't available on the
// host.
func jakartaNow() time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.Now().UTC()
	}
	return time.Now().In(loc)
}
