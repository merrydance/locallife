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
	balanceReq  baofucontracts.BalanceQueryRequest
	withdrawReq baofucontracts.WithdrawRequest
	withdrawRes *baofucontracts.WithdrawResult
	balanceRes  *baofucontracts.BalanceResult
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
	c.withdrawReq = req
	return c.withdrawRes, nil
}

func (c *baofuWithdrawClientRecorder) QueryWithdraw(context.Context, baofucontracts.WithdrawQueryRequest) (*baofucontracts.WithdrawResult, error) {
	return nil, nil
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
	require.Equal(t, int64(1200), result.AvailableAmountFen)
	require.Equal(t, "COLLECT_MER", client.balanceReq.MerchantID)
	require.Equal(t, "COLLECT_TER", client.balanceReq.TerminalID)
	require.Equal(t, "CM_BINDING", client.balanceReq.ContractNo)
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
		ID:          81,
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     9,
		AccountType: db.BaofuAccountTypePersonal,
		OpenState:   db.BaofuAccountOpenStateActive,
		ContractNo:  pgtype.Text{String: "CM_BINDING", Valid: true},
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
	store.EXPECT().CreateBaofuWithdrawalOrder(gomock.Any(), db.CreateBaofuWithdrawalOrderParams{
		OwnerType:        binding.OwnerType,
		OwnerID:          binding.OwnerID,
		AccountBindingID: binding.ID,
		OutRequestNo:     "WD_OUT_001",
		Amount:           1200,
		Status:           db.BaofuWithdrawalStatusProcessing,
		RawSnapshot:      []byte(`{"state":"submitted"}`),
	}).Return(withdrawal, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
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
		OwnerType:    binding.OwnerType,
		OwnerID:      binding.OwnerID,
		AmountFen:    1200,
		OutRequestNo: "WD_OUT_001",
	})
	require.NoError(t, err)
	require.Equal(t, withdrawal.ID, result.WithdrawalOrder.ID)
	require.Equal(t, "PAYOUT_MER", client.withdrawReq.MerchantID)
	require.Equal(t, "PAYOUT_TER", client.withdrawReq.TerminalID)
	require.Equal(t, "CM_BINDING", client.withdrawReq.ContractNo)
	require.Equal(t, int64(1200), client.withdrawReq.AmountFen)
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/withdraw", client.withdrawReq.NotifyURL)
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
		ID:          81,
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     9,
		AccountType: db.BaofuAccountTypePersonal,
		OpenState:   db.BaofuAccountOpenStateActive,
		ContractNo:  pgtype.Text{String: "CM_BINDING", Valid: true},
	}, nil)
	store.EXPECT().CreateBaofuWithdrawalOrder(gomock.Any(), gomock.Any()).Times(0)

	_, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:    db.BaofuAccountOwnerTypeRider,
		OwnerID:      9,
		AmountFen:    1200,
		OutRequestNo: "WD_OUT_002",
	})
	require.ErrorIs(t, err, ErrBaofuWithdrawInsufficientBalance)
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
		ID:          82,
		OwnerType:   db.BaofuAccountOwnerTypeMerchant,
		OwnerID:     19,
		AccountType: db.BaofuAccountTypeBusiness,
		OpenState:   db.BaofuAccountOpenStateActive,
		ContractNo:  pgtype.Text{String: "MERCHANT_CONTRACT", Valid: true},
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
	store.EXPECT().CreateBaofuWithdrawalOrder(gomock.Any(), gomock.Any()).Return(withdrawal, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner)
		require.Equal(t, db.ExternalPaymentCapabilityBaofuWithdraw, arg.Capability)
		return db.ExternalPaymentCommand{ID: 102}, nil
	})
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), gomock.Any()).Return(withdrawal, nil)

	_, err := service.CreateWithdrawal(context.Background(), BaofuCreateWithdrawalInput{
		OwnerType:    binding.OwnerType,
		OwnerID:      binding.OwnerID,
		AmountFen:    1200,
		OutRequestNo: "MBW_OUT_001",
	})
	require.NoError(t, err)
}
