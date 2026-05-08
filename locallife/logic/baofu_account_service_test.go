package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestBaofuAccountServiceValidateOwnerAccountRules(t *testing.T) {
	service := NewBaofuAccountService(nil, nil)

	require.NoError(t, service.ValidateOwnerAccount(db.BaofuAccountOwnerTypeMerchant, db.BaofuAccountTypeBusiness))
	require.NoError(t, service.ValidateOwnerAccount(db.BaofuAccountOwnerTypeRider, db.BaofuAccountTypePersonal))
	require.NoError(t, service.ValidateOwnerAccount(db.BaofuAccountOwnerTypeOperator, db.BaofuAccountTypePersonal))
	require.NoError(t, service.ValidateOwnerAccount(db.BaofuAccountOwnerTypePlatform, db.BaofuAccountTypeBusiness))

	require.ErrorIs(t, service.ValidateOwnerAccount(db.BaofuAccountOwnerTypeMerchant, db.BaofuAccountTypePersonal), ErrBaofuAccountInvalidOwnerAccount)
	require.ErrorIs(t, service.ValidateOwnerAccount(db.BaofuAccountOwnerTypeRider, db.BaofuAccountTypeBusiness), ErrBaofuAccountInvalidOwnerAccount)
	require.ErrorIs(t, service.ValidateOwnerAccount(db.BaofuAccountOwnerTypeOperator, db.BaofuAccountTypeBusiness), ErrBaofuAccountInvalidOwnerAccount)
	require.ErrorIs(t, service.ValidateOwnerAccount(db.BaofuAccountOwnerTypePlatform, db.BaofuAccountTypePersonal), ErrBaofuAccountInvalidOwnerAccount)
	require.ErrorIs(t, service.ValidateOwnerAccount(db.BaofuAccountOwnerTypePlatform, "platform"), ErrBaofuAccountInvalidOwnerAccount)
}

func TestBaofuAccountServiceMerchantPaymentReadinessUsesCanonicalSharingMerIDOnly(t *testing.T) {
	service := NewBaofuAccountService(nil, nil)
	binding := db.BaofuAccountBinding{
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "CM123", Valid: true},
		SharingMerID: pgtype.Text{String: "CM123", Valid: true},
	}

	require.NoError(t, service.ValidatePaymentReady(binding))
}

func TestBaofuAccountServiceRiderPaymentReadinessDoesNotRequireWechatSubMchID(t *testing.T) {
	service := NewBaofuAccountService(nil, nil)
	binding := db.BaofuAccountBinding{
		OwnerType:    db.BaofuAccountOwnerTypeRider,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "CP123", Valid: true},
		SharingMerID: pgtype.Text{String: "CP123", Valid: true},
	}

	require.NoError(t, service.ValidatePaymentReady(binding))
}

func TestBaofuAccountServicePaymentReadinessRequiresCanonicalSharingMerID(t *testing.T) {
	service := NewBaofuAccountService(nil, nil)
	binding := db.BaofuAccountBinding{
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OpenState:   db.BaofuAccountOpenStateActive,
		ContractNo:  pgtype.Text{String: "CP123", Valid: true},
		AccountType: db.BaofuAccountTypePersonal,
	}

	err := service.ValidatePaymentReady(binding)

	require.ErrorIs(t, err, ErrBaofuAccountReceiverRequired)
}

func TestBaofuAccountServiceOpenAccountRecordsCommandBeforeClientCall(t *testing.T) {
	store := &fakeBaofuAccountStore{}
	client := &fakeBaofuAccountClient{result: &baofucontracts.AccountResult{
		OutRequestNo:  "OPEN123",
		ContractNo:    "CP123",
		SharingMerID:  "CP123",
		OpenState:     db.BaofuAccountOpenStateActive,
		UpstreamState: "1",
	}}
	service := NewBaofuAccountService(store, client)

	_, err := service.OpenAccount(context.Background(), baofucontracts.OpenAccountRequest{
		OwnerType:     db.BaofuAccountOwnerTypeRider,
		OwnerID:       42,
		AccountType:   db.BaofuAccountTypePersonal,
		OutRequestNo:  "OPEN123",
		LegalName:     "测试用户",
		CertificateNo: "110101199001010011",
		BankAccountNo: "6222020202020202020",
		BankMobile:    "13800138000",
	})
	require.NoError(t, err)
	require.True(t, store.commandCreatedBeforeClientCall)
	require.True(t, client.called)
	require.Equal(t, db.ExternalPaymentProviderBaofu, store.lastCommand.Provider)
	require.Equal(t, db.PaymentChannelBaofuAggregate, store.lastCommand.Channel)
	require.Equal(t, db.ExternalPaymentCapabilityBaofuAccount, store.lastCommand.Capability)
	require.Equal(t, db.ExternalPaymentCommandTypeOpenBaofuAccount, store.lastCommand.CommandType)
	require.JSONEq(t, `{}`, string(store.lastActive.RawSnapshot))
	require.Equal(t, db.BaofuFeeTypeAccountOpenVerifyFee, store.lastFeeLedger.FeeType)
	require.Equal(t, db.BaofuFeePayerTypePlatform, store.lastFeeLedger.PayerType)
	require.False(t, store.lastFeeLedger.PayerID.Valid)
	require.Equal(t, "baofu_account_binding", store.lastFeeLedger.BusinessObjectType)
	require.Equal(t, int64(7), store.lastFeeLedger.BusinessObjectID)
	require.Equal(t, int64(100), store.lastFeeLedger.Amount)
}

func TestBaofuAccountServiceOpenAccountMarksAbnormalForOfficialExceptionState(t *testing.T) {
	store := &fakeBaofuAccountStore{}
	client := &fakeBaofuAccountClient{result: &baofucontracts.AccountResult{
		OutRequestNo:  "OPEN_ABNORMAL",
		OpenState:     db.BaofuAccountOpenStateAbnormal,
		UpstreamState: "-1",
		Raw:           []byte(`{"state":"-1"}`),
	}}
	service := NewBaofuAccountService(store, client)

	_, err := service.OpenAccount(context.Background(), baofucontracts.OpenAccountRequest{
		OwnerType:     db.BaofuAccountOwnerTypeRider,
		OwnerID:       42,
		AccountType:   db.BaofuAccountTypePersonal,
		OutRequestNo:  "OPEN_ABNORMAL",
		LegalName:     "测试用户",
		CertificateNo: "110101199001010011",
		BankAccountNo: "6222020202020202020",
		BankMobile:    "13800138000",
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), store.lastAbnormal.ID)
	require.JSONEq(t, `{"state":"-1"}`, string(store.lastAbnormal.RawSnapshot))
	require.False(t, store.failedCalled)
}

func TestBaofuAccountServiceOpenAccountRequiresOutRequestNo(t *testing.T) {
	service := NewBaofuAccountService(&fakeBaofuAccountStore{}, &fakeBaofuAccountClient{})

	_, err := service.OpenAccount(context.Background(), baofucontracts.OpenAccountRequest{
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     42,
		AccountType: db.BaofuAccountTypePersonal,
	})

	require.ErrorIs(t, err, ErrBaofuAccountOutRequestNoRequired)
}

func TestBaofuAccountServiceOpenAccountRejectsInvalidOfficialInputBeforeWriting(t *testing.T) {
	store := &fakeBaofuAccountStore{}
	client := &fakeBaofuAccountClient{}
	service := NewBaofuAccountService(store, client)

	_, err := service.OpenAccount(context.Background(), baofucontracts.OpenAccountRequest{
		OwnerType:    db.BaofuAccountOwnerTypeRider,
		OwnerID:      42,
		AccountType:  db.BaofuAccountTypePersonal,
		OutRequestNo: "OPEN123",
	})

	require.EqualError(t, err, "baofu open account personal legalName is required")
	require.False(t, store.bindingUpserted)
	require.False(t, client.called)
}

type fakeBaofuAccountStore struct {
	lastCommand                    db.CreateExternalPaymentCommandParams
	lastActive                     db.MarkBaofuAccountBindingActiveParams
	lastAbnormal                   db.MarkBaofuAccountBindingAbnormalParams
	lastFeeLedger                  db.CreateBaofuFeeLedgerParams
	commandCreatedBeforeClientCall bool
	bindingUpserted                bool
	failedCalled                   bool
}

func (s *fakeBaofuAccountStore) UpsertBaofuAccountBinding(ctx context.Context, arg db.UpsertBaofuAccountBindingParams) (db.BaofuAccountBinding, error) {
	s.bindingUpserted = true
	return db.BaofuAccountBinding{ID: 7, OwnerType: arg.OwnerType, OwnerID: arg.OwnerID, AccountType: arg.AccountType, OpenState: arg.OpenState}, nil
}

func (s *fakeBaofuAccountStore) CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
	s.lastCommand = arg
	s.commandCreatedBeforeClientCall = true
	return db.ExternalPaymentCommand{ID: 9, Provider: arg.Provider, Channel: arg.Channel, Capability: arg.Capability, CommandType: arg.CommandType}, nil
}

func (s *fakeBaofuAccountStore) MarkBaofuAccountBindingActiveWithFeeLedgerTx(ctx context.Context, arg db.MarkBaofuAccountBindingActiveWithFeeLedgerTxParams) (db.MarkBaofuAccountBindingActiveWithFeeLedgerTxResult, error) {
	s.lastActive = arg.ActiveBinding
	s.lastFeeLedger = arg.AccountOpenFeeLedger
	return db.MarkBaofuAccountBindingActiveWithFeeLedgerTxResult{
		Binding:              db.BaofuAccountBinding{ID: arg.ActiveBinding.ID, OpenState: db.BaofuAccountOpenStateActive, ContractNo: arg.ActiveBinding.ContractNo, SharingMerID: arg.ActiveBinding.SharingMerID},
		AccountOpenFeeLedger: db.BaofuFeeLedger{ID: 11, FeeType: arg.AccountOpenFeeLedger.FeeType, PayerType: arg.AccountOpenFeeLedger.PayerType, Amount: arg.AccountOpenFeeLedger.Amount},
	}, nil
}

func (s *fakeBaofuAccountStore) MarkBaofuAccountBindingFailed(ctx context.Context, arg db.MarkBaofuAccountBindingFailedParams) (db.BaofuAccountBinding, error) {
	s.failedCalled = true
	return db.BaofuAccountBinding{ID: arg.ID, OpenState: db.BaofuAccountOpenStateFailed}, nil
}

func (s *fakeBaofuAccountStore) MarkBaofuAccountBindingAbnormal(ctx context.Context, arg db.MarkBaofuAccountBindingAbnormalParams) (db.BaofuAccountBinding, error) {
	s.lastAbnormal = arg
	return db.BaofuAccountBinding{ID: arg.ID, OpenState: db.BaofuAccountOpenStateAbnormal}, nil
}

type fakeBaofuAccountClient struct {
	called bool
	result *baofucontracts.AccountResult
	err    error
}

func (c *fakeBaofuAccountClient) OpenAccount(ctx context.Context, req baofucontracts.OpenAccountRequest) (*baofucontracts.AccountResult, error) {
	c.called = true
	if c.err != nil {
		return nil, c.err
	}
	if c.result == nil {
		return nil, errors.New("missing result")
	}
	return c.result, nil
}

func (c *fakeBaofuAccountClient) QueryAccount(ctx context.Context, req baofucontracts.QueryAccountRequest) (*baofucontracts.AccountResult, error) {
	return c.result, c.err
}
