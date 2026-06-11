package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type baofuWithdrawClientRecorder struct {
	balanceReq           baofucontracts.BalanceQueryRequest
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
	c.balanceReq = req
	return c.balanceRes, nil
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
}

func TestBaofuWithdrawServiceCreateWithdrawalUsesPayoutMerchantAndRecordsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: "CM_BINDING", AvailableAmountFen: 5000},
		withdrawRes: &baofucontracts.WithdrawResult{
			TransSerialNo:   "WD_OUT_001",
			BaofuWithdrawNo: "BF_WITHDRAW_001",
			ContractNo:      "CM_BINDING",
			UpstreamState:   "2",
			Status:          db.BaofuWithdrawalStatusProcessing,
			Raw:             []byte(`{"state":"2"}`),
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
	store.EXPECT().CreateBaofuWithdrawalOrderWithSubmittedCommandTx(gomock.Any(), db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxParams{
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
		BusinessOwner: db.ExternalPaymentBusinessOwnerRiderIncome,
		SubmittedAt:   time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
	}).Return(db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult{
		WithdrawalOrder:  withdrawal,
		SubmittedCommand: db.ExternalPaymentCommand{ID: 101},
	}, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
		require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
		require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityBaofuWithdraw, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreateBaofuWithdraw, arg.CommandType)
		require.Equal(t, "baofu_withdrawal_order", arg.BusinessObjectType.String)
		require.Equal(t, withdrawal.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectWithdraw, arg.ExternalObjectType)
		require.Equal(t, withdrawal.OutRequestNo, arg.ExternalObjectKey)
		return db.ExternalPaymentCommand{ID: 101}, nil
	})
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), db.UpdateBaofuWithdrawalOrderToProcessingParams{
		ID:              withdrawal.ID,
		BaofuWithdrawNo: pgtype.Text{String: "BF_WITHDRAW_001", Valid: true},
		RawSnapshot:     []byte(`{"state":"2"}`),
	}).Return(withdrawal, nil)

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
	require.Equal(t, "PAYOUT_MER", client.withdrawReq.MerchantID)
	require.Equal(t, "PAYOUT_TER", client.withdrawReq.TerminalID)
	require.Equal(t, "CM_BINDING", client.withdrawReq.ContractNo)
	require.Equal(t, int64(1200), client.withdrawReq.AmountFen)
	require.Equal(t, "RIDER_SHARE_001", client.withdrawReq.FeeMemberID)
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/withdraw", client.withdrawReq.NotifyURL)
}

func TestBaofuWithdrawServiceCreateWithdrawalRecordsSubmittedCommandBeforeProviderCall(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	submittedCommandRecorded := false
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: "CM_BINDING", AvailableAmountFen: 5000},
		withdrawRes: &baofucontracts.WithdrawResult{
			TransSerialNo:   "WD_OUT_SUBMITTED_FIRST",
			BaofuWithdrawNo: "BF_WITHDRAW_SUBMITTED_FIRST",
			ContractNo:      "CM_BINDING",
			UpstreamState:   "1",
			Status:          db.BaofuWithdrawalStatusProcessing,
			Raw:             []byte(`{"state":"1"}`),
		},
		beforeCreateWithdraw: func() {
			require.True(t, submittedCommandRecorded, "submitted command must be durable before provider create")
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
	store.EXPECT().CreateBaofuWithdrawalOrderWithSubmittedCommandTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxParams) (db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult, error) {
		submittedCommandRecorded = true
		require.Equal(t, db.ExternalPaymentBusinessOwnerRiderIncome, arg.BusinessOwner)
		require.Equal(t, time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC), arg.SubmittedAt)
		require.Equal(t, withdrawal.OutRequestNo, arg.WithdrawalOrder.OutRequestNo)
		return db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult{
			WithdrawalOrder:  withdrawal,
			SubmittedCommand: db.ExternalPaymentCommand{ID: 101},
		}, nil
	})
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		if arg.CommandStatus == db.ExternalPaymentCommandStatusSubmitted {
			require.True(t, submittedCommandRecorded, "submitted tx must commit before provider create")
		}
		require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
		return db.ExternalPaymentCommand{ID: 101}, nil
	})
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), db.UpdateBaofuWithdrawalOrderToProcessingParams{
		ID:              withdrawal.ID,
		BaofuWithdrawNo: pgtype.Text{String: "BF_WITHDRAW_SUBMITTED_FIRST", Valid: true},
		RawSnapshot:     []byte(`{"state":"1"}`),
	}).Return(withdrawal, nil)

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
	store.EXPECT().CreateBaofuWithdrawalOrderWithSubmittedCommandTx(gomock.Any(), gomock.Any()).Times(0)

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
	store.EXPECT().CreateBaofuWithdrawalOrderWithSubmittedCommandTx(gomock.Any(), gomock.Any()).Times(0)

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
	store.EXPECT().CreateBaofuWithdrawalOrderWithSubmittedCommandTx(gomock.Any(), gomock.Any()).Times(0)
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
	store.EXPECT().CreateBaofuWithdrawalOrderWithSubmittedCommandTx(gomock.Any(), gomock.Any()).Times(0)

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
		withdrawRes: &baofucontracts.WithdrawResult{
			TransSerialNo:   "MBW_OUT_001",
			BaofuWithdrawNo: "BF_WITHDRAW_001",
			ContractNo:      "MERCHANT_CONTRACT",
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
	store.EXPECT().CreateBaofuWithdrawalOrderWithSubmittedCommandTx(gomock.Any(), gomock.Any()).Return(db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult{
		WithdrawalOrder:  withdrawal,
		SubmittedCommand: db.ExternalPaymentCommand{ID: 102},
	}, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner)
		require.Equal(t, db.ExternalPaymentCapabilityBaofuWithdraw, arg.Capability)
		return db.ExternalPaymentCommand{ID: 102}, nil
	})
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), gomock.Any()).Return(withdrawal, nil)

	_, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:      binding.OwnerType,
		OwnerID:        binding.OwnerID,
		AmountFen:      1200,
		OutRequestNo:   "MBW_OUT_001",
		IdempotencyKey: "withdraw-merchant-funds-1",
	})
	require.NoError(t, err)
	require.Equal(t, "MERCHANT_SHARE_001", client.withdrawReq.FeeMemberID)
}

func TestBaofuWithdrawServiceCreateWithdrawalMarksRejectedAcceptanceFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: "CM_BINDING", AvailableAmountFen: 5000},
		withdrawRes: &baofucontracts.WithdrawResult{
			TransSerialNo:   "WD_REJECTED_001",
			BaofuWithdrawNo: "BF_WITHDRAW_REJECTED",
			ContractNo:      "CM_BINDING",
			UpstreamState:   "2",
			Status:          db.BaofuWithdrawalStatusFailed,
			Remark:          "余额不足",
			Raw:             []byte(`{"state":"2","transRemark":"余额不足"}`),
		},
	}
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})
	service.now = func() time.Time { return time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC) }

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
		OutRequestNo:     "WD_REJECTED_001",
		Amount:           1200,
		Status:           db.BaofuWithdrawalStatusProcessing,
	}
	failedWithdrawal := withdrawal
	failedWithdrawal.Status = db.BaofuWithdrawalStatusFailed
	failedWithdrawal.BaofuWithdrawNo = pgtype.Text{String: "BF_WITHDRAW_REJECTED", Valid: true}

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: binding.OwnerType,
		OwnerID:   binding.OwnerID,
	}).Return(binding, nil)
	expectNoBaofuWithdrawalIdempotencyInLogic(store, binding.OwnerType, binding.OwnerID, "withdraw-rejected-1")
	store.EXPECT().CreateBaofuWithdrawalOrderWithSubmittedCommandTx(gomock.Any(), db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxParams{
		WithdrawalOrder: db.CreateBaofuWithdrawalOrderParams{
			OwnerType:        binding.OwnerType,
			OwnerID:          binding.OwnerID,
			AccountBindingID: binding.ID,
			OutRequestNo:     "WD_REJECTED_001",
			Amount:           1200,
			Status:           db.BaofuWithdrawalStatusProcessing,
			RawSnapshot:      []byte(`{"state":"submitted"}`),
			IdempotencyKey: pgtype.Text{
				String: "withdraw-rejected-1",
				Valid:  true,
			},
			IdempotencyRequestHash: pgtype.Text{
				String: baofuWithdrawalCreateRequestHash(binding.OwnerType, binding.OwnerID, 1200),
				Valid:  true,
			},
		},
		BusinessOwner: db.ExternalPaymentBusinessOwnerRiderIncome,
		SubmittedAt:   time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
	}).Return(db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult{
		WithdrawalOrder:  withdrawal,
		SubmittedCommand: db.ExternalPaymentCommand{ID: 103},
	}, nil)
	store.EXPECT().UpdateBaofuWithdrawalOrderStatus(gomock.Any(), db.UpdateBaofuWithdrawalOrderStatusParams{
		ID:              withdrawal.ID,
		Status:          db.BaofuWithdrawalStatusFailed,
		BaofuWithdrawNo: pgtype.Text{String: "BF_WITHDRAW_REJECTED", Valid: true},
		RawSnapshot:     []byte(`{"state":"2","transRemark":"余额不足"}`),
	}).Return(failedWithdrawal, nil)
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentCommandStatusRejected, arg.CommandStatus)
		require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
		require.Equal(t, db.ExternalPaymentCapabilityBaofuWithdraw, arg.Capability)
		require.Equal(t, "baofu_acceptance_rejected", arg.LastErrorCode.String)
		require.True(t, arg.LastErrorCode.Valid)
		require.Equal(t, "余额不足", arg.LastErrorMessage.String)
		require.True(t, arg.LastErrorMessage.Valid)
		require.True(t, arg.RejectedAt.Valid)
		return db.ExternalPaymentCommand{ID: 103}, nil
	})

	result, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:      binding.OwnerType,
		OwnerID:        binding.OwnerID,
		AmountFen:      1200,
		OutRequestNo:   "WD_REJECTED_001",
		IdempotencyKey: "withdraw-rejected-1",
	})
	require.ErrorIs(t, err, ErrBaofuWithdrawCreateRejected)
	require.Equal(t, failedWithdrawal.ID, result.WithdrawalOrder.ID)
	require.Equal(t, db.BaofuWithdrawalStatusFailed, result.WithdrawalOrder.Status)
	require.Equal(t, "rejected", result.SyncState)
	require.Equal(t, "提现申请未被受理，请刷新余额后重试", result.UserMessage)
}

func TestBaofuWithdrawServiceCreateWithdrawalReturnsUnknownAfterProviderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuWithdrawClientRecorder{
		balanceRes: &baofucontracts.BalanceResult{ContractNo: "CM_BINDING", AvailableAmountFen: 5000},
	}
	providerErr := &baofu.ProviderError{
		Operation:       "create_baofu_withdraw",
		Capability:      "baofu_withdraw",
		StatusCode:      200,
		RequestID:       "REQ_WITHDRAW_UNKNOWN",
		UpstreamCode:    "SYSTEM_BUSY",
		UpstreamMessage: "系统繁忙，请稍后重试，member_id=CP_WITHDRAW_001",
		DiagnosticSnapshot: []byte(
			`{"provider":"baofu","operation":"create_baofu_withdraw","upstream_code":"SYSTEM_BUSY","upstream_message_sanitized":"系统繁忙，请稍后重试，member_id=<redacted>"}`,
		),
	}
	client.withdrawErr = providerErr
	service := NewBaofuWithdrawService(store, client, BaofuWithdrawServiceConfig{
		CollectMerchantID: "COLLECT_MER",
		CollectTerminalID: "COLLECT_TER",
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})
	service.now = func() time.Time { return time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC) }

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
		OutRequestNo:     "WD_UNKNOWN_001",
		Amount:           1200,
		Status:           db.BaofuWithdrawalStatusProcessing,
	}

	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: binding.OwnerType,
		OwnerID:   binding.OwnerID,
	}).Return(binding, nil)
	expectNoBaofuWithdrawalIdempotencyInLogic(store, binding.OwnerType, binding.OwnerID, "withdraw-unknown-1")
	store.EXPECT().CreateBaofuWithdrawalOrderWithSubmittedCommandTx(gomock.Any(), gomock.Any()).Return(db.CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult{
		WithdrawalOrder:  withdrawal,
		SubmittedCommand: db.ExternalPaymentCommand{ID: 104},
	}, nil)
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateBaofuWithdrawalOrderStatus(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentCommandStatusUnknown, arg.CommandStatus)
		require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
		require.Equal(t, db.ExternalPaymentCapabilityBaofuWithdraw, arg.Capability)
		require.Equal(t, "SYSTEM_BUSY", arg.LastErrorCode.String)
		require.True(t, arg.LastErrorCode.Valid)
		require.Contains(t, arg.LastErrorMessage.String, "支付通道处理中")
		require.Contains(t, arg.LastErrorMessage.String, "retry_later")
		require.True(t, arg.LastErrorMessage.Valid)
		require.Contains(t, string(arg.ResponseSnapshot), `"upstream_code":"SYSTEM_BUSY"`)
		require.Contains(t, string(arg.ResponseSnapshot), "系统繁忙，请稍后重试")
		require.NotContains(t, string(arg.ResponseSnapshot), "CP_WITHDRAW_001")
		return db.ExternalPaymentCommand{ID: 104}, nil
	})

	result, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:      binding.OwnerType,
		OwnerID:        binding.OwnerID,
		AmountFen:      1200,
		OutRequestNo:   "WD_UNKNOWN_001",
		IdempotencyKey: "withdraw-unknown-1",
	})
	require.ErrorIs(t, err, ErrBaofuWithdrawCreateResultUnknown)
	require.Equal(t, withdrawal.ID, result.WithdrawalOrder.ID)
	require.Equal(t, db.BaofuWithdrawalStatusProcessing, result.WithdrawalOrder.Status)
	require.Equal(t, "unknown", result.SyncState)
	require.Equal(t, "提现申请已提交，结果正在确认，请勿重复提交", result.UserMessage)
}

func TestBaofuWithdrawCommandErrorMessageSanitizesGenericCause(t *testing.T) {
	got := baofuWithdrawCommandErrorMessage("", errors.New("provider timeout: member_id=CP_WITHDRAW_001"))

	require.Contains(t, got, "provider timeout")
	require.Contains(t, got, "member_id=<redacted>")
	require.NotContains(t, got, "CP_WITHDRAW_001")
}
