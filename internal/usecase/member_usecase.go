package usecase

import (
	"context"

	"golang.org/x/crypto/bcrypt"

	"github.com/vmr/kas-vmr-backend/internal/domain"
	"github.com/vmr/kas-vmr-backend/internal/repository"
	"github.com/vmr/kas-vmr-backend/pkg/jwtutil"
	"github.com/vmr/kas-vmr-backend/pkg/response"
	"github.com/vmr/kas-vmr-backend/pkg/tokenstore"
)

type MemberSummary struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	PhoneNumber string  `json:"phone_number"`
	HouseNumber *string `json:"house_number,omitempty"`
}

type MemberUsecase struct {
	memberRepo    repository.MemberRepository
	signer        *jwtutil.Signer
	tokens        *tokenstore.Store
	setDefaultPin string
}

func NewMemberUsecase(memberRepo repository.MemberRepository, signer *jwtutil.Signer, tokens *tokenstore.Store, setDefaultPin string) *MemberUsecase {
	return &MemberUsecase{memberRepo: memberRepo, signer: signer, tokens: tokens, setDefaultPin: setDefaultPin}
}

// SetPin lets a member set (or change) their own PIN. Returns the
// member's name for the confirmation message, matching the original.
func (u *MemberUsecase) SetPin(ctx context.Context, memberID uint, pin string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	member, err := u.memberRepo.UpdatePin(ctx, memberID, string(hashed))
	if err != nil {
		return "", err
	}
	return member.Name, nil
}

// SetPinByAdmin resets a member's PIN to the configured default
// (SET_DEFAULT_PIN), e.g. when a member forgets their PIN.
func (u *MemberUsecase) SetPinByAdmin(ctx context.Context, memberID uint) (string, error) {
	if u.setDefaultPin == "" {
		return "", response.NewAPIError(500, "SET_DEFAULT_PIN tidak diatur di env")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(u.setDefaultPin), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	member, err := u.memberRepo.UpdatePin(ctx, memberID, string(hashed))
	if err != nil {
		return "", err
	}
	return member.Name, nil
}

// Login authenticates a member by phone + PIN. The phone number is
// normalized once via pkg/phoneutil (see that package's doc comment) -
// the same normalization the repository lookup itself performs, so there
// is exactly one rule instead of two that could drift apart.
func (u *MemberUsecase) Login(ctx context.Context, phone, pin string) (*AuthTokens, error) {
	if phone == "" {
		return nil, response.NewAPIError(400, "Nomor HP wajib diisi")
	}

	member, err := u.memberRepo.FindByPhoneOrSpouse(ctx, phone)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, response.NewAPIError(404, "Member tidak ditemukan")
	}

	if member.Pin == nil || *member.Pin == "" {
		// More helpful than the generic "PIN salah" the original code
		// would have returned here (bcrypt.compare against an empty
		// string just fails silently) - tell the member they need to set
		// a PIN first.
		return nil, response.NewAPIError(401, "Kamu belum mengatur PIN, silakan hubungi admin")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*member.Pin), []byte(pin)); err != nil {
		return nil, response.NewAPIError(401, "PIN tidak sesuai")
	}

	payload := jwtutil.Payload{ID: member.ID, Email: member.PhoneNumber, Role: domain.RoleMember}
	accessToken, err := u.signer.SignAccessToken(payload)
	if err != nil {
		return nil, err
	}
	refreshToken, err := u.signer.SignRefreshToken(payload)
	if err != nil {
		return nil, err
	}
	if err := u.tokens.Add(ctx, idToString(member.ID), refreshToken); err != nil {
		return nil, err
	}

	return &AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: map[string]interface{}{
			"name":  member.Name,
			"role":  domain.RoleMember,
			"phone": member.PhoneNumber,
		},
	}, nil
}

func (u *MemberUsecase) GetAllActive(ctx context.Context) ([]MemberSummary, error) {
	members, err := u.memberRepo.FindAllActive(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]MemberSummary, 0, len(members))
	for _, m := range members {
		result = append(result, MemberSummary{
			ID:          m.ID,
			Name:        m.Name,
			PhoneNumber: m.PhoneNumber,
			HouseNumber: m.HouseNumber,
		})
	}
	return result, nil
}

func (u *MemberUsecase) GetByID(ctx context.Context, id uint) (*domain.Member, error) {
	return u.memberRepo.FindByID(ctx, id)
}

// SetBankAccountSuffix mendaftarkan/update 4 digit terakhir nomor
// rekening SeaBank member - dipakai AutoConfirmUsecase buat cocokin
// pengirim transfer.
func (u *MemberUsecase) SetBankAccountSuffix(ctx context.Context, memberID uint, suffix string) error {
	if len(suffix) != 4 {
		return response.NewAPIError(400, "Suffix rekening harus 4 digit")
	}
	_, err := u.memberRepo.UpdateBankAccountSuffix(ctx, memberID, suffix)
	return err
}
