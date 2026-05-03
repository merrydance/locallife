package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
)

var (
	ErrBaofuAccountInvalidOwnerAccount  = errors.New("baofu account owner and account type do not match")
	ErrBaofuAccountInactive             = errors.New("baofu account is not active")
	ErrBaofuAccountReceiverRequired     = errors.New("baofu account receiver id is required")
	ErrBaofuAccountWechatSubMchRequired = errors.New("baofu merchant wechat channel identity is required")
	ErrBaofuAccountOutRequestNoRequired = errors.New("baofu account out request no is required")
)

type baofuAccountStore interface {
	UpsertBaofuAccountBinding(ctx context.Context, arg db.UpsertBaofuAccountBindingParams) (db.BaofuAccountBinding, error)
	CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error)
	MarkBaofuAccountBindingActive(ctx context.Context, arg db.MarkBaofuAccountBindingActiveParams) (db.BaofuAccountBinding, error)
	MarkBaofuAccountBindingFailed(ctx context.Context, arg db.MarkBaofuAccountBindingFailedParams) (db.BaofuAccountBinding, error)
}

type BaofuAccountClient interface {
	OpenAccount(ctx context.Context, req baofucontracts.OpenAccountRequest) (*baofucontracts.AccountResult, error)
	QueryAccount(ctx context.Context, req baofucontracts.QueryAccountRequest) (*baofucontracts.AccountResult, error)
}

type BaofuAccountService struct {
	store  baofuAccountStore
	client BaofuAccountClient
	now    func() time.Time
}

func NewBaofuAccountService(store baofuAccountStore, client BaofuAccountClient) *BaofuAccountService {
	return &BaofuAccountService{store: store, client: client, now: time.Now}
}

func (s *BaofuAccountService) ValidateOwnerAccount(ownerType, accountType string) error {
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeMerchant:
		if accountType == db.BaofuAccountTypeBusiness {
			return nil
		}
	case db.BaofuAccountOwnerTypeRider:
		if accountType == db.BaofuAccountTypePersonal {
			return nil
		}
	case db.BaofuAccountOwnerTypeOperator:
		if accountType == db.BaofuAccountTypeBusiness || accountType == db.BaofuAccountTypePlatform {
			return nil
		}
	case db.BaofuAccountOwnerTypePlatform:
		if accountType == db.BaofuAccountTypePlatform {
			return nil
		}
	}
	return ErrBaofuAccountInvalidOwnerAccount
}

func (s *BaofuAccountService) ValidatePaymentReady(binding db.BaofuAccountBinding) error {
	if strings.TrimSpace(binding.OpenState) != db.BaofuAccountOpenStateActive {
		return ErrBaofuAccountInactive
	}
	if strings.TrimSpace(binding.SharingMerID.String) == "" && strings.TrimSpace(binding.ContractNo.String) == "" {
		return ErrBaofuAccountReceiverRequired
	}
	if strings.TrimSpace(binding.OwnerType) == db.BaofuAccountOwnerTypeMerchant && strings.TrimSpace(binding.WechatSubMchID.String) == "" {
		return ErrBaofuAccountWechatSubMchRequired
	}
	return nil
}

func (s *BaofuAccountService) OpenAccount(ctx context.Context, req baofucontracts.OpenAccountRequest) (*baofucontracts.AccountResult, error) {
	if s == nil || s.store == nil || s.client == nil {
		return nil, errors.New("baofu account service is not configured")
	}
	if err := s.ValidateOwnerAccount(req.OwnerType, req.AccountType); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.OutRequestNo) == "" {
		return nil, ErrBaofuAccountOutRequestNoRequired
	}
	rawSnapshot := []byte(`{"state":"submitted"}`)
	binding, err := s.store.UpsertBaofuAccountBinding(ctx, db.UpsertBaofuAccountBindingParams{
		OwnerType:             strings.TrimSpace(req.OwnerType),
		OwnerID:               req.OwnerID,
		AccountType:           strings.TrimSpace(req.AccountType),
		LoginNo:               pgtype.Text{},
		OpenState:             db.BaofuAccountOpenStateProcessing,
		WechatSubMchID:        pgtype.Text{String: strings.TrimSpace(req.WechatSubMchID), Valid: strings.TrimSpace(req.WechatSubMchID) != ""},
		LastOpenTransSerialNo: pgtype.Text{String: strings.TrimSpace(req.OutRequestNo), Valid: strings.TrimSpace(req.OutRequestNo) != ""},
		RawSnapshot:           rawSnapshot,
	})
	if err != nil {
		return nil, err
	}
	_, err = s.store.CreateExternalPaymentCommand(ctx, db.CreateExternalPaymentCommandParams{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuAccount,
		CommandType:        db.ExternalPaymentCommandTypeOpenBaofuAccount,
		BusinessOwner:      businessOwnerForBaofuAccount(req.OwnerType),
		BusinessObjectType: pgtype.Text{String: "baofu_account_binding", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: binding.ID, Valid: true},
		ExternalObjectType: "baofu_account",
		ExternalObjectKey:  strings.TrimSpace(req.OutRequestNo),
		CommandStatus:      db.ExternalPaymentCommandStatusSubmitted,
		SubmittedAt:        s.now().UTC(),
		ResponseSnapshot:   rawSnapshot,
	})
	if err != nil {
		return nil, err
	}
	result, err := s.client.OpenAccount(ctx, req)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("baofu account open returned empty result")
	}
	normalized := result.Normalized()
	switch normalized.OpenState {
	case db.BaofuAccountOpenStateActive:
		_, err = s.store.MarkBaofuAccountBindingActive(ctx, db.MarkBaofuAccountBindingActiveParams{
			ID:           binding.ID,
			ContractNo:   pgtype.Text{String: normalized.ContractNo, Valid: normalized.ContractNo != ""},
			SharingMerID: pgtype.Text{String: normalized.SharingMerID, Valid: normalized.SharingMerID != ""},
			RawSnapshot:  baofuAccountRawSnapshot(normalized.Raw),
		})
	case db.BaofuAccountOpenStateFailed:
		_, err = s.store.MarkBaofuAccountBindingFailed(ctx, db.MarkBaofuAccountBindingFailedParams{ID: binding.ID, RawSnapshot: baofuAccountRawSnapshot(normalized.Raw)})
	}
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func baofuAccountRawSnapshot(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	return raw
}

func businessOwnerForBaofuAccount(ownerType string) string {
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeRider:
		return "rider"
	case db.BaofuAccountOwnerTypeOperator:
		return "operator"
	case db.BaofuAccountOwnerTypePlatform:
		return "platform"
	default:
		return db.ExternalPaymentBusinessOwnerApplyment
	}
}
