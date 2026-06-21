package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type baofuWithdrawClientRecorder struct {
	balanceReq           baofucontracts.BalanceQueryRequest
	balanceCallCount     int
	balanceErr           error
	withdrawReq          baofucontracts.WithdrawRequest
	withdrawRes          *baofucontracts.WithdrawResult
	withdrawErr          error
	balanceRes           *baofucontracts.BalanceResult
	beforeCreateWithdraw func()
}

func (c *baofuWithdrawClientRecorder) OpenAccount(context.Context, baofucontracts.OpenAccountRequest) (*baofucontracts.AccountResult, error) {
	return nil, nil
}

func (c *baofuWithdrawClientRecorder) QueryAccount(context.Context, baofucontracts.QueryAccountRequest) (*baofucontracts.AccountResult, error) {
	return nil, nil
}

func (c *baofuWithdrawClientRecorder) QueryBalance(_ context.Context, req baofucontracts.BalanceQueryRequest) (*baofucontracts.BalanceResult, error) {
	c.balanceCallCount++
	c.balanceReq = req
	return c.balanceRes, c.balanceErr
}

func (c *baofuWithdrawClientRecorder) CreateWithdraw(_ context.Context, req baofucontracts.WithdrawRequest) (*baofucontracts.WithdrawResult, error) {
	if c.beforeCreateWithdraw != nil {
		c.beforeCreateWithdraw()
	}
	c.withdrawReq = req
	return c.withdrawRes, c.withdrawErr
}

func (c *baofuWithdrawClientRecorder) QueryWithdraw(context.Context, baofucontracts.WithdrawQueryRequest) (*baofucontracts.WithdrawResult, error) {
	return nil, nil
}

func expectNoBaofuWithdrawalIdempotencyInLogic(store *mockdb.MockStore, ownerType string, ownerID int64, idempotencyKey string) {
	store.EXPECT().GetBaofuWithdrawalOrderByIdempotency(gomock.Any(), db.GetBaofuWithdrawalOrderByIdempotencyParams{
		OwnerType: ownerType,
		OwnerID:   ownerID,
		IdempotencyKey: pgtype.Text{
			String: idempotencyKey,
			Valid:  true,
		},
	}).Return(db.BaofuWithdrawalOrder{}, db.ErrRecordNotFound)
}

func TestBaofuWithdrawServiceQueryBalanceUsesCollectMerchant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: "CM_BINDING", AvailableAmountFen: 1200, PendingAmountFen: 300, LedgerAmountFen: 1500},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
	})

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   9,
	}).Return(db.BaofuAccountBinding{
		ID:           81,
		OwnerType:    db.BaofuAccountOwnerTypeRider,
		OwnerID:      9,
		AccountType:  db.BaofuAccountTypePersonal,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "CM_BINDING", Valid: true},
		SharingMerID: pgtype.Text{String: "RIDER_SHARE_001", Valid: true},
	}, nil)

	result, err := service.QueryBalance(context.Background(), BaofuBalanceQueryInput{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   9,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1200), result.AvailableAmountFen)
	require.Equal(t, "COLLECT_MER", client.balanceReq.MerchantID)
	require.Equal(t, "COLLECT_TER", client.balanceReq.TerminalID)
	require.Equal(t, "CM_BINDING", client.balanceReq.ContractNo)
	require.Equal(t, db.BaofuAccountTypePersonal, client.balanceReq.AccountType)
	require.Equal(t, 1, client.balanceCallCount)
}

func TestBaofuWithdrawServiceQueryBalanceReturnsUnavailableWhenBindingMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{AvailableAmountFen: 9900},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
	})

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   9,
	}).Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)

	result, err := service.QueryBalance(context.Background(), BaofuBalanceQueryInput{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   9,
	})
	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateProfilePending, result.AccountStatus)
	require.Equal(t, "结算账户未开通", result.StatusDesc)
	require.Equal(t, "请先开通结算账户后再提现", result.DisabledReason)
	require.False(t, result.CanWithdraw)
	require.Zero(t, result.AvailableAmountFen)
	require.Zero(t, client.balanceCallCount)
}

func TestBaofuWithdrawServiceQueryBalanceReturnsUnavailableWhenBindingNotActive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{AvailableAmountFen: 9900},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
	})

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   9,
	}).Return(db.BaofuAccountBinding{
		ID:          81,
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     9,
		AccountType: db.BaofuAccountTypePersonal,
		OpenState:   db.BaofuAccountOpenStateProcessing,
	}, nil)

	result, err := service.QueryBalance(context.Background(), BaofuBalanceQueryInput{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   9,
	})
	require.NoError(t, err)
	require.Equal(t, BaofuWithdrawBalanceAccountStatusNotReady, result.AccountStatus)
	require.Equal(t, "结算账户未开通", result.StatusDesc)
	require.Equal(t, "请先开通结算账户后再提现", result.DisabledReason)
	require.False(t, result.CanWithdraw)
	require.Zero(t, result.AvailableAmountFen)
	require.Zero(t, client.balanceCallCount)
}

func TestBaofuWithdrawServiceQueryBalanceReturnsUnavailableWhenBindingMissingContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{AvailableAmountFen: 9900},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
	})

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   9,
	}).Return(db.BaofuAccountBinding{
		ID:           81,
		OwnerType:    db.BaofuAccountOwnerTypeRider,
		OwnerID:      9,
		AccountType:  db.BaofuAccountTypePersonal,
		OpenState:    db.BaofuAccountOpenStateActive,
		SharingMerID: pgtype.Text{String: "RIDER_SHARE_001", Valid: true},
	}, nil)

	result, err := service.QueryBalance(context.Background(), BaofuBalanceQueryInput{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   9,
	})
	require.NoError(t, err)
	require.Equal(t, BaofuWithdrawBalanceAccountStatusNotReady, result.AccountStatus)
	require.Equal(t, "结算账户状态异常", result.StatusDesc)
	require.Equal(t, "结算账户状态异常，请联系平台处理", result.DisabledReason)
	require.False(t, result.CanWithdraw)
	require.Zero(t, result.AvailableAmountFen)
	require.Zero(t, client.balanceCallCount)
}

func TestBaofuWithdrawServiceQueryBalanceReturnsUnavailableWhenBindingMissingFeeMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{AvailableAmountFen: 9900},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
	})

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   9,
	}).Return(db.BaofuAccountBinding{
		ID:          81,
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     9,
		AccountType: db.BaofuAccountTypePersonal,
		OpenState:   db.BaofuAccountOpenStateActive,
		ContractNo:  pgtype.Text{String: "CM_BINDING", Valid: true},
	}, nil)

	result, err := service.QueryBalance(context.Background(), BaofuBalanceQueryInput{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   9,
	})
	require.NoError(t, err)
	require.Equal(t, BaofuWithdrawBalanceAccountStatusNotReady, result.AccountStatus)
	require.Equal(t, "结算账户状态异常", result.StatusDesc)
	require.Equal(t, "结算账户状态异常，请联系平台处理", result.DisabledReason)
	require.False(t, result.CanWithdraw)
	require.Zero(t, result.AvailableAmountFen)
	require.Zero(t, client.balanceCallCount)
}

func TestBaofuWithdrawServiceCreateWithdrawalPersistsSubmittedCommandIntent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: "CM_BINDING", AvailableAmountFen: 5000},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})
	service.now = func() time.Time { return time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC) }

	binding := db.BaofuAccountBinding{
		ID:           81,
		OwnerType:    db.BaofuAccountOwnerTypeRider,
		OwnerID:      9,
		AccountType:  db.BaofuAccountTypePersonal,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "CM_BINDING", Valid: true},
		SharingMerID: pgtype.Text{String: "RIDER_SHARE_001", Valid: true},
	}
	withdrawal := db.BaofuWithdrawalOrder{
		ID:               91,
		OwnerType:        binding.OwnerType,
		OwnerID:          binding.OwnerID,
		AccountBindingID: binding.ID,
		OutRequestNo:     "WD_OUT_001",
		Amount:           1200,
		Status:           db.BaofuWithdrawalStatusProcessing,
	}

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: binding.OwnerType,
		OwnerID:   binding.OwnerID,
	}).Return(binding, nil)
	expectNoBaofuWithdrawalIdempotencyInLogic(store, binding.OwnerType, binding.OwnerID, "withdraw-service-1")
	store.EXPECT().CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams{
		WithdrawalOrder: db.CreateBaofuWithdrawalOrderParams{
			OwnerType:        binding.OwnerType,
			OwnerID:          binding.OwnerID,
			AccountBindingID: binding.ID,
			OutRequestNo:     "WD_OUT_001",
			Amount:           1200,
			Status:           db.BaofuWithdrawalStatusProcessing,
			RawSnapshot:      []byte(`{"state":"submitted"}`),
			IdempotencyKey: pgtype.Text{
				String: "withdraw-service-1",
				Valid:  true,
			},
			IdempotencyRequestHash: pgtype.Text{
				String: baofuWithdrawalCreateRequestHash(binding.OwnerType, binding.OwnerID, 1200),
				Valid:  true,
			},
		},
		BusinessOwner:              db.ExternalPaymentBusinessOwnerRiderIncome,
		SubmittedAt:                time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
		ProviderAvailableAmountFen: 5000,
		ProviderBalanceObservedAt:  time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
	}).Return(db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult{
		WithdrawalOrder:  withdrawal,
		SubmittedCommand: db.ExternalPaymentCommand{ID: 101},
	}, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateBaofuWithdrawalOrderStatus(gomock.Any(), gomock.Any()).Times(0)

	result, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:      binding.OwnerType,
		OwnerID:        binding.OwnerID,
		AmountFen:      1200,
		OutRequestNo:   "WD_OUT_001",
		IdempotencyKey: "withdraw-service-1",
	})
	require.NoError(t, err)
	require.Equal(t, withdrawal.ID, result.WithdrawalOrder.ID)
	require.Equal(t, db.BaofuAccountTypePersonal, client.balanceReq.AccountType)
	require.Equal(t, "unknown", result.SyncState)
	require.Equal(t, "提现申请已提交，结果正在确认，请勿重复提交", result.UserMessage)
	require.Empty(t, client.withdrawReq.TransSerialNo)
}

func TestBaofuWithdrawServiceCreateWithdrawalRecordsSubmittedCommandWithoutProviderCall(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	submittedCommandRecorded := false
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: "CM_BINDING", AvailableAmountFen: 5000},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})
	service.now = func() time.Time { return time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC) }

	binding := db.BaofuAccountBinding{
		ID:           81,
		OwnerType:    db.BaofuAccountOwnerTypeRider,
		OwnerID:      9,
		AccountType:  db.BaofuAccountTypePersonal,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "CM_BINDING", Valid: true},
		SharingMerID: pgtype.Text{String: "RIDER_SHARE_001", Valid: true},
	}
	withdrawal := db.BaofuWithdrawalOrder{
		ID:               91,
		OwnerType:        binding.OwnerType,
		OwnerID:          binding.OwnerID,
		AccountBindingID: binding.ID,
		OutRequestNo:     "WD_OUT_SUBMITTED_FIRST",
		Amount:           1200,
		Status:           db.BaofuWithdrawalStatusProcessing,
	}

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: binding.OwnerType,
		OwnerID:   binding.OwnerID,
	}).Return(binding, nil)
	expectNoBaofuWithdrawalIdempotencyInLogic(store, binding.OwnerType, binding.OwnerID, "withdraw-submitted-first")
	store.EXPECT().CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams) (db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult, error) {
		submittedCommandRecorded = true
		require.Equal(t, db.ExternalPaymentBusinessOwnerRiderIncome, arg.BusinessOwner)
		require.Equal(t, time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC), arg.SubmittedAt)
		require.Equal(t, int64(5000), arg.ProviderAvailableAmountFen)
		require.Equal(t, time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC), arg.ProviderBalanceObservedAt)
		require.Equal(t, withdrawal.OutRequestNo, arg.WithdrawalOrder.OutRequestNo)
		return db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult{
			WithdrawalOrder:  withdrawal,
			SubmittedCommand: db.ExternalPaymentCommand{ID: 101},
		}, nil
	})
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateBaofuWithdrawalOrderStatus(gomock.Any(), gomock.Any()).Times(0)

	result, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:      binding.OwnerType,
		OwnerID:        binding.OwnerID,
		AmountFen:      1200,
		OutRequestNo:   "WD_OUT_SUBMITTED_FIRST",
		IdempotencyKey: "withdraw-submitted-first",
	})
	require.NoError(t, err)
	require.Equal(t, withdrawal.ID, result.WithdrawalOrder.ID)
	require.True(t, submittedCommandRecorded)
	require.Empty(t, client.withdrawReq.TransSerialNo)
}

func TestBaofuWithdrawServiceCreateWithdrawalPersistsIntentWithoutProviderDispatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: "CM_BINDING", AvailableAmountFen: 5000},
		withdrawRes: &baofucontracts.WithdrawResult{
			TransSerialNo:   "WD_OUT_DEFERRED",
			BaofuWithdrawNo: "BF_WITHDRAW_DEFERRED",
			ContractNo:      "CM_BINDING",
			UpstreamState:   "1",
			Status:          db.BaofuWithdrawalStatusProcessing,
			Raw:             []byte(`{"state":"1"}`),
		},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})
	service.now = func() time.Time { return time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC) }

	binding := db.BaofuAccountBinding{
		ID:           81,
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      19,
		AccountType:  db.BaofuAccountTypeBusiness,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "CM_BINDING", Valid: true},
		SharingMerID: pgtype.Text{String: "MERCHANT_SHARE_001", Valid: true},
	}
	withdrawal := db.BaofuWithdrawalOrder{
		ID:               91,
		OwnerType:        binding.OwnerType,
		OwnerID:          binding.OwnerID,
		AccountBindingID: binding.ID,
		OutRequestNo:     "WD_OUT_DEFERRED",
		Amount:           1200,
		Status:           db.BaofuWithdrawalStatusProcessing,
	}

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: binding.OwnerType,
		OwnerID:   binding.OwnerID,
	}).Return(binding, nil)
	expectNoBaofuWithdrawalIdempotencyInLogic(store, binding.OwnerType, binding.OwnerID, "withdraw-deferred-dispatch-1")
	store.EXPECT().CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).Return(db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult{
		WithdrawalOrder:  withdrawal,
		SubmittedCommand: db.ExternalPaymentCommand{ID: 101},
	}, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateBaofuWithdrawalOrderStatus(gomock.Any(), gomock.Any()).Times(0)

	result, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:      binding.OwnerType,
		OwnerID:        binding.OwnerID,
		AmountFen:      1200,
		OutRequestNo:   "WD_OUT_DEFERRED",
		IdempotencyKey: "withdraw-deferred-dispatch-1",
	})
	require.NoError(t, err)
	require.Equal(t, withdrawal.ID, result.WithdrawalOrder.ID)
	require.Equal(t, "unknown", result.SyncState)
	require.Equal(t, "提现申请已提交，结果正在确认，请勿重复提交", result.UserMessage)
	require.Empty(t, client.withdrawReq.TransSerialNo)
}

func TestBaofuWithdrawServiceCreateWithdrawalRejectsInsufficientBalanceBeforeOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: "CM_BINDING", AvailableAmountFen: 1000},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   9,
	}).Return(db.BaofuAccountBinding{
		ID:           81,
		OwnerType:    db.BaofuAccountOwnerTypeRider,
		OwnerID:      9,
		AccountType:  db.BaofuAccountTypePersonal,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "CM_BINDING", Valid: true},
		SharingMerID: pgtype.Text{String: "RIDER_SHARE_001", Valid: true},
	}, nil)
	expectNoBaofuWithdrawalIdempotencyInLogic(store, db.BaofuAccountOwnerTypeRider, 9, "withdraw-insufficient-1")
	store.EXPECT().CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).Times(0)

	_, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:      db.BaofuAccountOwnerTypeRider,
		OwnerID:        9,
		AmountFen:      1200,
		OutRequestNo:   "WD_OUT_002",
		IdempotencyKey: "withdraw-insufficient-1",
	})
	require.ErrorIs(t, err, ErrBaofuWithdrawInsufficientBalance)
	require.Empty(t, client.withdrawReq.TransSerialNo)
}

func TestBaofuWithdrawServiceCreateWithdrawalRejectsInsufficientLocalReservationBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: "CM_BINDING", AvailableAmountFen: 5000},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})
	service.now = func() time.Time { return time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC) }

	binding := db.BaofuAccountBinding{
		ID:           81,
		OwnerType:    db.BaofuAccountOwnerTypeRider,
		OwnerID:      9,
		AccountType:  db.BaofuAccountTypePersonal,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "CM_BINDING", Valid: true},
		SharingMerID: pgtype.Text{String: "RIDER_SHARE_001", Valid: true},
	}
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: binding.OwnerType,
		OwnerID:   binding.OwnerID,
	}).Return(binding, nil)
	expectNoBaofuWithdrawalIdempotencyInLogic(store, binding.OwnerType, binding.OwnerID, "withdraw-local-reserved-insufficient-1")
	store.EXPECT().CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams) (db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult, error) {
		require.Equal(t, binding.ID, arg.WithdrawalOrder.AccountBindingID)
		require.Equal(t, int64(1200), arg.WithdrawalOrder.Amount)
		require.Equal(t, int64(5000), arg.ProviderAvailableAmountFen)
		return db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult{}, db.ErrBaofuWithdrawalInsufficientReservedBalance
	})
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateBaofuWithdrawalOrderStatus(gomock.Any(), gomock.Any()).Times(0)

	_, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:      binding.OwnerType,
		OwnerID:        binding.OwnerID,
		AmountFen:      1200,
		OutRequestNo:   "WD_OUT_LOCAL_RESERVED_INSUFFICIENT",
		IdempotencyKey: "withdraw-local-reserved-insufficient-1",
	})
	require.ErrorIs(t, err, ErrBaofuWithdrawInsufficientBalance)
	require.Empty(t, client.withdrawReq.TransSerialNo)
}

func TestBaofuWithdrawServiceCreateWithdrawalRequiresFeeMemberIDBeforeOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: "CM_BINDING", AvailableAmountFen: 5000},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   9,
	}).Return(db.BaofuAccountBinding{
		ID:          81,
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     9,
		AccountType: db.BaofuAccountTypePersonal,
		OpenState:   db.BaofuAccountOpenStateActive,
		ContractNo:  pgtype.Text{String: "CM_BINDING", Valid: true},
	}, nil)
	expectNoBaofuWithdrawalIdempotencyInLogic(store, db.BaofuAccountOwnerTypeRider, 9, "withdraw-missing-fee-1")
	store.EXPECT().CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).Times(0)

	_, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:      db.BaofuAccountOwnerTypeRider,
		OwnerID:        9,
		AmountFen:      1200,
		OutRequestNo:   "WD_OUT_003",
		IdempotencyKey: "withdraw-missing-fee-1",
	})
	require.ErrorIs(t, err, ErrBaofuWithdrawFeeMemberIDRequired)
	require.Empty(t, client.balanceReq.ContractNo)
	require.Empty(t, client.withdrawReq.TransSerialNo)
}

func TestBaofuWithdrawServiceCreateWithdrawalReplaysSameIdempotencyKeyWithoutProviderCall(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	existing := db.BaofuWithdrawalOrder{
		ID:                     91,
		OwnerType:              db.BaofuAccountOwnerTypeMerchant,
		OwnerID:                19,
		AccountBindingID:       82,
		OutRequestNo:           "MBW_OUT_REPLAY",
		Amount:                 1200,
		Status:                 db.BaofuWithdrawalStatusProcessing,
		IdempotencyKey:         pgtype.Text{String: "withdraw-replay-1", Valid: true},
		IdempotencyRequestHash: pgtype.Text{String: baofuWithdrawalCreateRequestHash(db.BaofuAccountOwnerTypeMerchant, 19, 1200), Valid: true},
	}
	store.EXPECT().GetBaofuWithdrawalOrderByIdempotency(gomock.Any(), db.GetBaofuWithdrawalOrderByIdempotencyParams{
		OwnerType:      db.BaofuAccountOwnerTypeMerchant,
		OwnerID:        19,
		IdempotencyKey: pgtype.Text{String: "withdraw-replay-1", Valid: true},
	}).Return(existing, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Times(0)

	result, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:      db.BaofuAccountOwnerTypeMerchant,
		OwnerID:        19,
		AmountFen:      1200,
		OutRequestNo:   "MBW_OUT_NEW",
		IdempotencyKey: "withdraw-replay-1",
	})
	require.NoError(t, err)
	require.True(t, result.IdempotencyReplayed)
	require.Equal(t, existing.ID, result.WithdrawalOrder.ID)
	require.Empty(t, client.balanceReq.ContractNo)
	require.Empty(t, client.withdrawReq.TransSerialNo)
}

func TestBaofuWithdrawServiceCreateWithdrawalRejectsConflictingIdempotencyKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	existing := db.BaofuWithdrawalOrder{
		ID:                     92,
		OwnerType:              db.BaofuAccountOwnerTypeMerchant,
		OwnerID:                19,
		Amount:                 1200,
		Status:                 db.BaofuWithdrawalStatusProcessing,
		IdempotencyKey:         pgtype.Text{String: "withdraw-conflict-1", Valid: true},
		IdempotencyRequestHash: pgtype.Text{String: baofuWithdrawalCreateRequestHash(db.BaofuAccountOwnerTypeMerchant, 19, 1200), Valid: true},
	}
	store.EXPECT().GetBaofuWithdrawalOrderByIdempotency(gomock.Any(), db.GetBaofuWithdrawalOrderByIdempotencyParams{
		OwnerType:      db.BaofuAccountOwnerTypeMerchant,
		OwnerID:        19,
		IdempotencyKey: pgtype.Text{String: "withdraw-conflict-1", Valid: true},
	}).Return(existing, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).Times(0)

	_, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:      db.BaofuAccountOwnerTypeMerchant,
		OwnerID:        19,
		AmountFen:      1300,
		OutRequestNo:   "MBW_OUT_CONFLICT",
		IdempotencyKey: "withdraw-conflict-1",
	})
	require.Error(t, err)
	var reqErr *RequestError
	require.ErrorAs(t, err, &reqErr)
	require.Equal(t, 409, reqErr.Status)
	require.Contains(t, reqErr.Error(), "idempotency key already used")
	require.Empty(t, client.balanceReq.ContractNo)
	require.Empty(t, client.withdrawReq.TransSerialNo)
}

func TestBaofuWithdrawServiceCreateMerchantWithdrawalRecordsMerchantFundsOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: "MERCHANT_CONTRACT", AvailableAmountFen: 5000},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	binding := db.BaofuAccountBinding{
		ID:           82,
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      19,
		AccountType:  db.BaofuAccountTypeBusiness,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "MERCHANT_CONTRACT", Valid: true},
		SharingMerID: pgtype.Text{String: "MERCHANT_SHARE_001", Valid: true},
	}
	withdrawal := db.BaofuWithdrawalOrder{
		ID:               92,
		OwnerType:        binding.OwnerType,
		OwnerID:          binding.OwnerID,
		AccountBindingID: binding.ID,
		OutRequestNo:     "MBW_OUT_001",
		Amount:           1200,
		Status:           db.BaofuWithdrawalStatusProcessing,
	}

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: binding.OwnerType,
		OwnerID:   binding.OwnerID,
	}).Return(binding, nil)
	expectNoBaofuWithdrawalIdempotencyInLogic(store, binding.OwnerType, binding.OwnerID, "withdraw-merchant-funds-1")
	store.EXPECT().CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams) (db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult, error) {
		require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner)
		require.Equal(t, int64(5000), arg.ProviderAvailableAmountFen)
		return db.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult{
			WithdrawalOrder:  withdrawal,
			SubmittedCommand: db.ExternalPaymentCommand{ID: 102},
		}, nil
	})
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateBaofuWithdrawalOrderStatus(gomock.Any(), gomock.Any()).Times(0)

	_, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:      binding.OwnerType,
		OwnerID:        binding.OwnerID,
		AmountFen:      1200,
		OutRequestNo:   "MBW_OUT_001",
		IdempotencyKey: "withdraw-merchant-funds-1",
	})
	require.NoError(t, err)
	require.Empty(t, client.withdrawReq.TransSerialNo)
}
