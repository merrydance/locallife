package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func expectMerchantCancelWithdrawQueryFact(t *testing.T, store *mockdb.MockStore, record db.MerchantCancelWithdrawApplication, queryResp *wechatcontracts.CancelWithdrawQueryResponse, expectedTerminalStatus string, expectedTerminal bool) {
	t.Helper()
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentCapabilityCancelWithdraw, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectCancelWithdraw, arg.ExternalObjectType)
		require.Equal(t, record.OutRequestNo, arg.ExternalObjectKey)
		require.Equal(t, queryResp.ApplymentID, arg.ExternalSecondaryKey.String)
		require.True(t, arg.ExternalSecondaryKey.Valid)
		require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner.String)
		require.Equal(t, "merchant_cancel_withdraw_application", arg.BusinessObjectType.String)
		require.Equal(t, record.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, queryResp.CancelState, arg.UpstreamState)
		require.Equal(t, expectedTerminalStatus, arg.TerminalStatus)
		require.Equal(t, expectedTerminal, arg.IsTerminal)
		require.Equal(t, "wechat:query:ecommerce:cancel_withdraw:MCW1001:FINISH:WITHDRAW_SUCCEED:WX-CANCEL-1001", arg.DedupeKey)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.RawResource, &payload))
		require.EqualValues(t, record.ID, payload["application_id"])
		require.EqualValues(t, record.MerchantID, payload["merchant_id"])
		require.Equal(t, record.SubMchID, payload["sub_mch_id"])
		require.Equal(t, queryResp.OutRequestNo, payload["out_request_no"])
		require.Equal(t, queryResp.ApplymentID, payload["applyment_id"])
		require.Equal(t, queryResp.CancelState, payload["cancel_state"])
		require.Equal(t, queryResp.WithdrawState, payload["withdraw_state"])
		return db.ExternalPaymentFact{ID: 7001, TerminalStatus: arg.TerminalStatus, IsTerminal: arg.IsTerminal}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactApplicationParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(7001), arg.FactID)
		require.Equal(t, "merchant_funds_domain", arg.Consumer)
		require.Equal(t, "merchant_cancel_withdraw_application", arg.BusinessObjectType)
		require.Equal(t, record.ID, arg.BusinessObjectID)
		require.Equal(t, db.ExternalPaymentFactApplicationStatusPending, arg.Status)
		return db.ExternalPaymentFactApplication{ID: 7002, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID, Status: arg.Status}, nil
	})
}

func TestProcessTaskMerchantCancelWithdrawResultSyncsTerminalState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	record := db.MerchantCancelWithdrawApplication{
		ID:             1001,
		MerchantID:     88,
		SubMchID:       "1900000109",
		OutRequestNo:   "MCW1001",
		LocalSyncState: db.MerchantCancelWithdrawLocalSyncStateSubmitUnknown,
	}
	queryResp := &wechatcontracts.CancelWithdrawQueryResponse{
		ApplymentID:              "WX-CANCEL-1001",
		OutRequestNo:             record.OutRequestNo,
		CancelState:              db.MerchantCancelStateFinish,
		CancelStateDescription:   "完成",
		WithdrawState:            db.MerchantCancelWithdrawStateSucceed,
		WithdrawStateDescription: "提现成功",
		ModifyTime:               "2026-04-26T10:00:00+08:00",
	}

	store.EXPECT().GetMerchantCancelWithdrawApplication(gomock.Any(), record.ID).Return(record, nil)
	ecommerceClient.EXPECT().QueryEcommerceCancelWithdrawByOutRequestNo(gomock.Any(), record.OutRequestNo).Return(queryResp, nil)
	expectMerchantCancelWithdrawQueryFact(t, store, record, queryResp, db.ExternalPaymentTerminalStatusSuccess, true)
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), int64(7002)).Return(db.ExternalPaymentFactApplication{
		ID:                 7002,
		FactID:             7001,
		Consumer:           "merchant_funds_domain",
		BusinessObjectType: "merchant_cancel_withdraw_application",
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), int64(7001)).Return(db.ExternalPaymentFact{
		ID:                   7001,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityCancelWithdraw,
		ExternalObjectType:   db.ExternalPaymentObjectCancelWithdraw,
		ExternalObjectKey:    record.OutRequestNo,
		ExternalSecondaryKey: pgtype.Text{String: queryResp.ApplymentID, Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectType:   pgtype.Text{String: "merchant_cancel_withdraw_application", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: record.ID, Valid: true},
		UpstreamState:        queryResp.CancelState,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
		RawResource:          mustMarshalMerchantCancelWithdrawQueryFactResource(t, record, queryResp),
	}, nil)
	store.EXPECT().GetMerchantCancelWithdrawApplication(gomock.Any(), record.ID).Return(record, nil)
	store.EXPECT().
		UpdateMerchantCancelWithdrawApplicationSync(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateMerchantCancelWithdrawApplicationSyncParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateMerchantCancelWithdrawApplicationSyncParams) (db.MerchantCancelWithdrawApplication, error) {
			require.Equal(t, record.ID, arg.ID)
			require.Equal(t, db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded, arg.LocalSyncState)
			require.Equal(t, db.MerchantCancelStateFinish, arg.CancelState.String)
			require.Equal(t, db.MerchantCancelWithdrawStateSucceed, arg.WithdrawState.String)
			require.Equal(t, "WX-CANCEL-1001", arg.ApplymentID.String)
			require.True(t, arg.LastQueryAt.Valid)
			return db.MerchantCancelWithdrawApplication{
				ID:                       record.ID,
				OutRequestNo:             record.OutRequestNo,
				ApplymentID:              pgtype.Text{String: "WX-CANCEL-1001", Valid: true},
				LocalSyncState:           db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded,
				CancelState:              pgtype.Text{String: db.MerchantCancelStateFinish, Valid: true},
				CancelStateDescription:   pgtype.Text{String: "完成", Valid: true},
				WithdrawState:            pgtype.Text{String: db.MerchantCancelWithdrawStateSucceed, Valid: true},
				WithdrawStateDescription: pgtype.Text{String: "提现成功", Valid: true},
			}, nil
		})
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateExternalPaymentFactProcessingStatusParams{})).Return(db.ExternalPaymentFact{ID: 7001}, nil)
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationAppliedParams{})).Return(db.ExternalPaymentFactApplication{ID: 7002}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.MerchantCancelWithdrawResultPayload{ApplicationID: record.ID, RetryCount: 0})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessMerchantCancelWithdrawResult, payloadBytes)
	err = processor.ProcessTaskMerchantCancelWithdrawResult(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskMerchantCancelWithdrawResult_FactPersistFailureStopsSync(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	record := db.MerchantCancelWithdrawApplication{
		ID:             1001,
		MerchantID:     88,
		SubMchID:       "1900000109",
		OutRequestNo:   "MCW1001",
		LocalSyncState: db.MerchantCancelWithdrawLocalSyncStateSubmitUnknown,
	}
	queryResp := &wechatcontracts.CancelWithdrawQueryResponse{
		ApplymentID:              "WX-CANCEL-1001",
		OutRequestNo:             record.OutRequestNo,
		CancelState:              db.MerchantCancelStateFinish,
		CancelStateDescription:   "完成",
		WithdrawState:            db.MerchantCancelWithdrawStateSucceed,
		WithdrawStateDescription: "提现成功",
	}

	store.EXPECT().GetMerchantCancelWithdrawApplication(gomock.Any(), record.ID).Return(record, nil)
	ecommerceClient.EXPECT().QueryEcommerceCancelWithdrawByOutRequestNo(gomock.Any(), record.OutRequestNo).Return(queryResp, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).Return(db.ExternalPaymentFact{}, errors.New("fact store unavailable"))
	store.EXPECT().UpdateMerchantCancelWithdrawApplicationSync(gomock.Any(), gomock.Any()).Times(0)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.MerchantCancelWithdrawResultPayload{ApplicationID: record.ID, RetryCount: 0})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessMerchantCancelWithdrawResult, payloadBytes)
	err = processor.ProcessTaskMerchantCancelWithdrawResult(context.Background(), task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "record merchant cancel withdraw query fact")
}

func mustMarshalMerchantCancelWithdrawQueryFactResource(t *testing.T, record db.MerchantCancelWithdrawApplication, queryResp *wechatcontracts.CancelWithdrawQueryResponse) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"application_id":             record.ID,
		"merchant_id":                record.MerchantID,
		"sub_mch_id":                 record.SubMchID,
		"out_request_no":             queryResp.OutRequestNo,
		"applyment_id":               queryResp.ApplymentID,
		"cancel_state":               queryResp.CancelState,
		"cancel_state_description":   queryResp.CancelStateDescription,
		"withdraw":                   queryResp.Withdraw,
		"withdraw_state":             queryResp.WithdrawState,
		"withdraw_state_description": queryResp.WithdrawStateDescription,
		"modify_time":                queryResp.ModifyTime,
		"account_info":               queryResp.AccountInfo,
		"account_withdraw_result":    queryResp.AccountWithdrawResult,
	})
	require.NoError(t, err)
	return raw
}
