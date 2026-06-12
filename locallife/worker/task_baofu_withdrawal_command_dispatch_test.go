package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskBaofuWithdrawalCommandDispatchSendsProviderAndRecordsAccepted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &fakeBaofuWithdrawalCommandDispatchClient{withdrawResult: &baofucontracts.WithdrawResult{
		TransSerialNo:   "WD_ASYNC_001",
		BaofuWithdrawNo: "BF_WITHDRAW_ASYNC_001",
		ContractNo:      "CM_BINDING",
		UpstreamState:   "1",
		Status:          db.BaofuWithdrawalStatusProcessing,
		Raw:             []byte(`{"state":"1"}`),
	}}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuWithdrawClientForTest(client, worker.BaofuWithdrawalCommandDispatchConfig{
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	command := submittedBaofuWithdrawalCommand(501, 91, "WD_ASYNC_001")
	withdrawal := baofuWithdrawalForDispatch(91, db.BaofuAccountOwnerTypeMerchant, 19, "WD_ASYNC_001")
	binding := activeBaofuWithdrawalBinding(withdrawal.OwnerType, withdrawal.OwnerID, "CM_BINDING", "MERCHANT_SHARE_001")

	store.EXPECT().GetExternalPaymentCommand(gomock.Any(), command.ID).Return(command, nil)
	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: withdrawal.OwnerType,
		OwnerID:   withdrawal.OwnerID,
	}).Return(binding, nil)
	store.EXPECT().ClaimSubmittedExternalPaymentCommandForDispatch(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.ClaimSubmittedExternalPaymentCommandForDispatchParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, command.ID, arg.ID)
			require.Equal(t, "baofu_withdraw_dispatch_started", arg.LastErrorCode.String)
			require.True(t, arg.LastErrorCode.Valid)
			require.Equal(t, db.ExternalPaymentCommandStatusUnknown, arg.CommandStatus)
			require.Contains(t, string(arg.ResponseSnapshot), `"dispatch_state":"started"`)
			claimed := command
			claimed.CommandStatus = db.ExternalPaymentCommandStatusUnknown
			return claimed, nil
		},
	)
	store.EXPECT().UpdateExternalPaymentCommandOutcome(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateExternalPaymentCommandOutcomeParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, command.ID, arg.ID)
			require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
			require.True(t, arg.AcceptedAt.Valid)
			require.False(t, arg.RejectedAt.Valid)
			require.False(t, arg.LastErrorCode.Valid)
			require.Contains(t, string(arg.ResponseSnapshot), `"outcome":"accepted"`)
			require.Contains(t, string(arg.ResponseSnapshot), `"baofu_withdraw_no":"BF_WITHDRAW_ASYNC_001"`)
			return db.ExternalPaymentCommand{ID: command.ID, CommandStatus: db.ExternalPaymentCommandStatusAccepted}, nil
		},
	)
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), db.UpdateBaofuWithdrawalOrderToProcessingParams{
		ID:              withdrawal.ID,
		BaofuWithdrawNo: pgtype.Text{String: "BF_WITHDRAW_ASYNC_001", Valid: true},
		RawSnapshot:     []byte(`{"state":"1"}`),
	}).Return(withdrawal, nil)

	payloadBytes, err := json.Marshal(worker.BaofuWithdrawalCommandDispatchPayload{CommandID: command.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskBaofuWithdrawalCommandDispatch(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalCommandDispatch, payloadBytes))

	require.NoError(t, err)
	require.True(t, client.called)
	require.Equal(t, "PAYOUT_MER", client.withdrawReq.MerchantID)
	require.Equal(t, "PAYOUT_TER", client.withdrawReq.TerminalID)
	require.Equal(t, "CM_BINDING", client.withdrawReq.ContractNo)
	require.Equal(t, "WD_ASYNC_001", client.withdrawReq.TransSerialNo)
	require.Equal(t, int64(1200), client.withdrawReq.AmountFen)
	require.Equal(t, "MERCHANT_SHARE_001", client.withdrawReq.FeeMemberID)
	require.Equal(t, "https://api.example.com/v1/webhooks/baofu/withdraw", client.withdrawReq.NotifyURL)
}

func TestProcessTaskBaofuWithdrawalCommandDispatchMarksRejectedAcceptanceFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &fakeBaofuWithdrawalCommandDispatchClient{withdrawResult: &baofucontracts.WithdrawResult{
		TransSerialNo:   "WD_ASYNC_REJECTED",
		BaofuWithdrawNo: "BF_WITHDRAW_REJECTED",
		ContractNo:      "CM_BINDING",
		UpstreamState:   "2",
		Status:          db.BaofuWithdrawalStatusFailed,
		Remark:          "余额不足",
		Raw:             []byte(`{"state":"2","transRemark":"余额不足"}`),
	}}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuWithdrawClientForTest(client, worker.BaofuWithdrawalCommandDispatchConfig{
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	command := submittedBaofuWithdrawalCommand(502, 92, "WD_ASYNC_REJECTED")
	withdrawal := baofuWithdrawalForDispatch(92, db.BaofuAccountOwnerTypeRider, 9, "WD_ASYNC_REJECTED")
	binding := activeBaofuWithdrawalBinding(withdrawal.OwnerType, withdrawal.OwnerID, "CM_BINDING", "RIDER_SHARE_001")
	failedWithdrawal := withdrawal
	failedWithdrawal.Status = db.BaofuWithdrawalStatusFailed

	store.EXPECT().GetExternalPaymentCommand(gomock.Any(), command.ID).Return(command, nil)
	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Any()).Return(binding, nil)
	store.EXPECT().ClaimSubmittedExternalPaymentCommandForDispatch(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: command.ID, CommandStatus: db.ExternalPaymentCommandStatusUnknown}, nil)
	store.EXPECT().UpdateExternalPaymentCommandOutcome(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateExternalPaymentCommandOutcomeParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, command.ID, arg.ID)
			require.Equal(t, db.ExternalPaymentCommandStatusRejected, arg.CommandStatus)
			require.True(t, arg.RejectedAt.Valid)
			require.Equal(t, "baofu_acceptance_rejected", arg.LastErrorCode.String)
			require.Equal(t, "余额不足", arg.LastErrorMessage.String)
			require.Contains(t, string(arg.ResponseSnapshot), `"outcome":"rejected"`)
			return db.ExternalPaymentCommand{ID: command.ID, CommandStatus: db.ExternalPaymentCommandStatusRejected}, nil
		},
	)
	store.EXPECT().UpdateBaofuWithdrawalOrderStatus(gomock.Any(), db.UpdateBaofuWithdrawalOrderStatusParams{
		ID:              withdrawal.ID,
		Status:          db.BaofuWithdrawalStatusFailed,
		BaofuWithdrawNo: pgtype.Text{String: "BF_WITHDRAW_REJECTED", Valid: true},
		RawSnapshot:     []byte(`{"state":"2","transRemark":"余额不足"}`),
	}).Return(failedWithdrawal, nil)

	payloadBytes, err := json.Marshal(worker.BaofuWithdrawalCommandDispatchPayload{CommandID: command.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskBaofuWithdrawalCommandDispatch(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalCommandDispatch, payloadBytes))

	require.NoError(t, err)
	require.True(t, client.called)
}

func TestProcessTaskBaofuWithdrawalCommandDispatchDoesNotAcceptCommandBeforeOrderUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &fakeBaofuWithdrawalCommandDispatchClient{withdrawResult: &baofucontracts.WithdrawResult{
		TransSerialNo:   "WD_ASYNC_ORDER_FAIL",
		BaofuWithdrawNo: "BF_WITHDRAW_ORDER_FAIL",
		ContractNo:      "CM_BINDING",
		UpstreamState:   "1",
		Status:          db.BaofuWithdrawalStatusProcessing,
		Raw:             []byte(`{"state":"1"}`),
	}}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuWithdrawClientForTest(client, worker.BaofuWithdrawalCommandDispatchConfig{
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	command := submittedBaofuWithdrawalCommand(504, 94, "WD_ASYNC_ORDER_FAIL")
	withdrawal := baofuWithdrawalForDispatch(94, db.BaofuAccountOwnerTypeMerchant, 19, "WD_ASYNC_ORDER_FAIL")
	binding := activeBaofuWithdrawalBinding(withdrawal.OwnerType, withdrawal.OwnerID, "CM_BINDING", "MERCHANT_SHARE_001")

	store.EXPECT().GetExternalPaymentCommand(gomock.Any(), command.ID).Return(command, nil)
	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Any()).Return(binding, nil)
	store.EXPECT().ClaimSubmittedExternalPaymentCommandForDispatch(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: command.ID, CommandStatus: db.ExternalPaymentCommandStatusUnknown}, nil)
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), gomock.Any()).Return(db.BaofuWithdrawalOrder{}, errors.New("db unavailable"))
	store.EXPECT().UpdateExternalPaymentCommandOutcome(gomock.Any(), gomock.Any()).Times(0)

	payloadBytes, err := json.Marshal(worker.BaofuWithdrawalCommandDispatchPayload{CommandID: command.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskBaofuWithdrawalCommandDispatch(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalCommandDispatch, payloadBytes))

	require.ErrorContains(t, err, "update baofu withdrawal accepted reference")
	require.True(t, client.called)
}

func TestProcessTaskBaofuWithdrawalCommandDispatchDoesNotRejectCommandBeforeOrderUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &fakeBaofuWithdrawalCommandDispatchClient{withdrawResult: &baofucontracts.WithdrawResult{
		TransSerialNo:   "WD_ASYNC_REJECT_ORDER_FAIL",
		BaofuWithdrawNo: "BF_WITHDRAW_REJECT_ORDER_FAIL",
		ContractNo:      "CM_BINDING",
		UpstreamState:   "2",
		Status:          db.BaofuWithdrawalStatusFailed,
		Remark:          "余额不足",
		Raw:             []byte(`{"state":"2","transRemark":"余额不足"}`),
	}}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuWithdrawClientForTest(client, worker.BaofuWithdrawalCommandDispatchConfig{
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	command := submittedBaofuWithdrawalCommand(505, 95, "WD_ASYNC_REJECT_ORDER_FAIL")
	withdrawal := baofuWithdrawalForDispatch(95, db.BaofuAccountOwnerTypeRider, 9, "WD_ASYNC_REJECT_ORDER_FAIL")
	binding := activeBaofuWithdrawalBinding(withdrawal.OwnerType, withdrawal.OwnerID, "CM_BINDING", "RIDER_SHARE_001")

	store.EXPECT().GetExternalPaymentCommand(gomock.Any(), command.ID).Return(command, nil)
	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Any()).Return(binding, nil)
	store.EXPECT().ClaimSubmittedExternalPaymentCommandForDispatch(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: command.ID, CommandStatus: db.ExternalPaymentCommandStatusUnknown}, nil)
	store.EXPECT().UpdateBaofuWithdrawalOrderStatus(gomock.Any(), gomock.Any()).Return(db.BaofuWithdrawalOrder{}, errors.New("db unavailable"))
	store.EXPECT().UpdateExternalPaymentCommandOutcome(gomock.Any(), gomock.Any()).Times(0)

	payloadBytes, err := json.Marshal(worker.BaofuWithdrawalCommandDispatchPayload{CommandID: command.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskBaofuWithdrawalCommandDispatch(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalCommandDispatch, payloadBytes))

	require.ErrorContains(t, err, "mark baofu withdrawal create rejected")
	require.True(t, client.called)
}

func TestProcessTaskBaofuWithdrawalCommandDispatchRepairsAcceptedOutcomeAfterOrderUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &fakeBaofuWithdrawalCommandDispatchClient{}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuWithdrawClientForTest(client, worker.BaofuWithdrawalCommandDispatchConfig{
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	command := submittedBaofuWithdrawalCommand(507, 97, "WD_ASYNC_ACCEPT_REPAIR")
	command.CommandStatus = db.ExternalPaymentCommandStatusUnknown
	command.ResponseSnapshot = []byte(`{"state":"submitted","dispatch_mode":"async_worker","dispatch_state":"started"}`)
	withdrawal := baofuWithdrawalForDispatch(97, db.BaofuAccountOwnerTypeMerchant, 19, "WD_ASYNC_ACCEPT_REPAIR")
	withdrawal.BaofuWithdrawNo = pgtype.Text{String: "BF_WITHDRAW_ACCEPT_REPAIR", Valid: true}
	withdrawal.RawSnapshot = []byte(`{"state":1}`)

	store.EXPECT().GetExternalPaymentCommand(gomock.Any(), command.ID).Return(command, nil)
	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().UpdateExternalPaymentCommandOutcome(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateExternalPaymentCommandOutcomeParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, command.ID, arg.ID)
			require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
			require.True(t, arg.AcceptedAt.Valid)
			require.False(t, arg.RejectedAt.Valid)
			require.Contains(t, string(arg.ResponseSnapshot), `"outcome":"accepted"`)
			require.Contains(t, string(arg.ResponseSnapshot), `"baofu_withdraw_no":"BF_WITHDRAW_ACCEPT_REPAIR"`)
			require.Contains(t, string(arg.ResponseSnapshot), `"upstream_state":"1"`)
			return db.ExternalPaymentCommand{ID: command.ID, CommandStatus: db.ExternalPaymentCommandStatusAccepted}, nil
		},
	)

	payloadBytes, err := json.Marshal(worker.BaofuWithdrawalCommandDispatchPayload{CommandID: command.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskBaofuWithdrawalCommandDispatch(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalCommandDispatch, payloadBytes))

	require.NoError(t, err)
	require.False(t, client.called)
}

func TestProcessTaskBaofuWithdrawalCommandDispatchRepairsRejectedOutcomeAfterOrderUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &fakeBaofuWithdrawalCommandDispatchClient{}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuWithdrawClientForTest(client, worker.BaofuWithdrawalCommandDispatchConfig{
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	command := submittedBaofuWithdrawalCommand(508, 98, "WD_ASYNC_REJECT_REPAIR")
	command.CommandStatus = db.ExternalPaymentCommandStatusUnknown
	command.ResponseSnapshot = []byte(`{"state":"submitted","dispatch_mode":"async_worker","dispatch_state":"started"}`)
	withdrawal := baofuWithdrawalForDispatch(98, db.BaofuAccountOwnerTypeRider, 9, "WD_ASYNC_REJECT_REPAIR")
	withdrawal.Status = db.BaofuWithdrawalStatusFailed
	withdrawal.BaofuWithdrawNo = pgtype.Text{String: "BF_WITHDRAW_REJECT_REPAIR", Valid: true}
	withdrawal.RawSnapshot = []byte(`{"state":"2","transRemark":"余额不足"}`)

	store.EXPECT().GetExternalPaymentCommand(gomock.Any(), command.ID).Return(command, nil)
	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().UpdateExternalPaymentCommandOutcome(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateExternalPaymentCommandOutcomeParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, command.ID, arg.ID)
			require.Equal(t, db.ExternalPaymentCommandStatusRejected, arg.CommandStatus)
			require.False(t, arg.AcceptedAt.Valid)
			require.True(t, arg.RejectedAt.Valid)
			require.Equal(t, "baofu_acceptance_rejected", arg.LastErrorCode.String)
			require.Equal(t, "余额不足", arg.LastErrorMessage.String)
			require.Contains(t, string(arg.ResponseSnapshot), `"outcome":"rejected"`)
			require.Contains(t, string(arg.ResponseSnapshot), `"upstream_state":"2"`)
			return db.ExternalPaymentCommand{ID: command.ID, CommandStatus: db.ExternalPaymentCommandStatusRejected}, nil
		},
	)

	payloadBytes, err := json.Marshal(worker.BaofuWithdrawalCommandDispatchPayload{CommandID: command.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskBaofuWithdrawalCommandDispatch(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalCommandDispatch, payloadBytes))

	require.NoError(t, err)
	require.False(t, client.called)
}

func TestProcessTaskBaofuWithdrawalCommandDispatchProviderErrorLeavesUnknownWithoutOrderMutation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	providerErr := &baofu.ProviderError{
		Operation:       "create_baofu_withdraw",
		Capability:      "baofu_withdraw",
		UpstreamCode:    "SYSTEM_BUSY",
		UpstreamMessage: "系统繁忙，请稍后重试，member_id=CP_WITHDRAW_001",
	}
	client := &fakeBaofuWithdrawalCommandDispatchClient{err: providerErr}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuWithdrawClientForTest(client, worker.BaofuWithdrawalCommandDispatchConfig{
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	command := submittedBaofuWithdrawalCommand(503, 93, "WD_ASYNC_UNKNOWN")
	withdrawal := baofuWithdrawalForDispatch(93, db.BaofuAccountOwnerTypeMerchant, 19, "WD_ASYNC_UNKNOWN")
	binding := activeBaofuWithdrawalBinding(withdrawal.OwnerType, withdrawal.OwnerID, "CM_BINDING", "MERCHANT_SHARE_001")

	store.EXPECT().GetExternalPaymentCommand(gomock.Any(), command.ID).Return(command, nil)
	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), withdrawal.ID).Return(withdrawal, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Any()).Return(binding, nil)
	store.EXPECT().ClaimSubmittedExternalPaymentCommandForDispatch(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentCommand{ID: command.ID, CommandStatus: db.ExternalPaymentCommandStatusUnknown}, nil)
	store.EXPECT().UpdateExternalPaymentCommandOutcome(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateExternalPaymentCommandOutcomeParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, command.ID, arg.ID)
			require.Equal(t, db.ExternalPaymentCommandStatusUnknown, arg.CommandStatus)
			require.Equal(t, "SYSTEM_BUSY", arg.LastErrorCode.String)
			require.True(t, arg.LastErrorCode.Valid)
			require.Contains(t, arg.LastErrorMessage.String, "支付通道处理中")
			require.Contains(t, arg.LastErrorMessage.String, "retry_later")
			require.NotContains(t, string(arg.ResponseSnapshot), "CP_WITHDRAW_001")
			return db.ExternalPaymentCommand{ID: command.ID, CommandStatus: db.ExternalPaymentCommandStatusUnknown}, nil
		},
	)
	store.EXPECT().UpdateBaofuWithdrawalOrderToProcessing(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().UpdateBaofuWithdrawalOrderStatus(gomock.Any(), gomock.Any()).Times(0)

	payloadBytes, err := json.Marshal(worker.BaofuWithdrawalCommandDispatchPayload{CommandID: command.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskBaofuWithdrawalCommandDispatch(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalCommandDispatch, payloadBytes))

	require.NoError(t, err)
	require.True(t, client.called)
}

func TestProcessTaskBaofuWithdrawalCommandDispatchRejectsNonAsyncWorkerCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &fakeBaofuWithdrawalCommandDispatchClient{}
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuWithdrawClientForTest(client, worker.BaofuWithdrawalCommandDispatchConfig{
		PayoutMerchantID:  "PAYOUT_MER",
		PayoutTerminalID:  "PAYOUT_TER",
		WithdrawNotifyURL: "https://api.example.com/v1/webhooks/baofu/withdraw",
	})

	command := submittedBaofuWithdrawalCommand(506, 96, "WD_SYNC_LEGACY")
	command.ResponseSnapshot = []byte(`{"state":"submitted"}`)

	store.EXPECT().GetExternalPaymentCommand(gomock.Any(), command.ID).Return(command, nil)
	store.EXPECT().GetBaofuWithdrawalOrder(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().ClaimSubmittedExternalPaymentCommandForDispatch(gomock.Any(), gomock.Any()).Times(0)

	payloadBytes, err := json.Marshal(worker.BaofuWithdrawalCommandDispatchPayload{CommandID: command.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskBaofuWithdrawalCommandDispatch(context.Background(), asynq.NewTask(worker.TaskProcessBaofuWithdrawalCommandDispatch, payloadBytes))

	require.ErrorContains(t, err, "not async-worker dispatch")
	require.ErrorIs(t, err, asynq.SkipRetry)
	require.False(t, client.called)
}

func submittedBaofuWithdrawalCommand(id, withdrawalID int64, outRequestNo string) db.ExternalPaymentCommand {
	return db.ExternalPaymentCommand{
		ID:                 id,
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuWithdraw,
		CommandType:        db.ExternalPaymentCommandTypeCreateBaofuWithdraw,
		BusinessOwner:      db.ExternalPaymentBusinessOwnerMerchantFunds,
		BusinessObjectType: pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: withdrawalID, Valid: true},
		ExternalObjectType: db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:  outRequestNo,
		CommandStatus:      db.ExternalPaymentCommandStatusSubmitted,
		ResponseSnapshot:   []byte(`{"state":"submitted","dispatch_mode":"async_worker"}`),
	}
}

func baofuWithdrawalForDispatch(id int64, ownerType string, ownerID int64, outRequestNo string) db.BaofuWithdrawalOrder {
	return db.BaofuWithdrawalOrder{
		ID:               id,
		OwnerType:        ownerType,
		OwnerID:          ownerID,
		AccountBindingID: 81,
		OutRequestNo:     outRequestNo,
		Amount:           1200,
		Status:           db.BaofuWithdrawalStatusProcessing,
	}
}

func activeBaofuWithdrawalBinding(ownerType string, ownerID int64, contractNo string, sharingMerID string) db.BaofuAccountBinding {
	return db.BaofuAccountBinding{
		ID:           81,
		OwnerType:    ownerType,
		OwnerID:      ownerID,
		AccountType:  db.BaofuAccountTypeBusiness,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: contractNo, Valid: true},
		SharingMerID: pgtype.Text{String: sharingMerID, Valid: true},
	}
}

type fakeBaofuWithdrawalCommandDispatchClient struct {
	called         bool
	withdrawReq    baofucontracts.WithdrawRequest
	withdrawResult *baofucontracts.WithdrawResult
	err            error
}

func (c *fakeBaofuWithdrawalCommandDispatchClient) QueryBalance(context.Context, baofucontracts.BalanceQueryRequest) (*baofucontracts.BalanceResult, error) {
	return nil, nil
}

func (c *fakeBaofuWithdrawalCommandDispatchClient) CreateWithdraw(_ context.Context, req baofucontracts.WithdrawRequest) (*baofucontracts.WithdrawResult, error) {
	c.called = true
	c.withdrawReq = req
	if c.err != nil {
		return nil, c.err
	}
	return c.withdrawResult, nil
}

func (c *fakeBaofuWithdrawalCommandDispatchClient) QueryWithdraw(context.Context, baofucontracts.WithdrawQueryRequest) (*baofucontracts.WithdrawResult, error) {
	return nil, nil
}
