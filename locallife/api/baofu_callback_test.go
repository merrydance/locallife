package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	baofunotification "github.com/merrydance/locallife/baofu/account/notification"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	baofuaggregatenotification "github.com/merrydance/locallife/baofu/aggregatepay/notification"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBaofuAccountOpenCallbackAppliesTerminalStateBeforeAck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	flow := db.BaofuAccountOpeningFlow{
		ID:                7101,
		OwnerType:         db.BaofuAccountOwnerTypeRider,
		OwnerID:           12,
		AccountType:       db.BaofuAccountTypePersonal,
		State:             db.BaofuAccountOpeningStateOpeningProcessing,
		OpenTransSerialNo: pgtype.Text{String: "OPEN123", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOR0000000012", Valid: true},
	}
	binding := db.BaofuAccountBinding{
		ID:          6101,
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     12,
		AccountType: db.BaofuAccountTypePersonal,
		LoginNo:     pgtype.Text{String: "LLBFOR0000000012", Valid: true},
		OpenState:   db.BaofuAccountOpenStateProcessing,
	}
	store.EXPECT().GetBaofuAccountOpeningFlowByOpenTransSerialNo(gomock.Any(), pgtype.Text{String: "OPEN123", Valid: true}).Return(flow, nil)
	store.EXPECT().GetBaofuAccountBindingByContractNo(gomock.Any(), pgtype.Text{String: "CM_BCT_123", Valid: true}).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   12,
	}).Return(binding, nil)

	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuAccount, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, "OPEN123", arg.ExternalObjectKey)
			require.Equal(t, "baofu:callback:account:OPEN123:1", arg.DedupeKey)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, pgtype.Text{String: db.BaofuAccountOwnerTypeRider, Valid: true}, arg.BusinessOwner)
			require.Equal(t, pgtype.Text{String: "baofu_account_opening_flow", Valid: true}, arg.BusinessObjectType)
			require.Equal(t, pgtype.Int8{Int64: 7101, Valid: true}, arg.BusinessObjectID)
			return db.ExternalPaymentFact{ID: 88, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().MarkBaofuAccountBindingActive(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountBindingActiveParams) (db.BaofuAccountBinding, error) {
			require.Equal(t, int64(6101), arg.ID)
			require.Equal(t, pgtype.Text{String: "CM_BCT_123", Valid: true}, arg.ContractNo)
			require.Equal(t, pgtype.Text{String: "CM_BCT_123", Valid: true}, arg.SharingMerID)
			require.JSONEq(t, `{"transSerialNo":"OPEN123","state":"1","contractNo":"CM_BCT_123"}`, string(arg.RawSnapshot))
			binding.OpenState = db.BaofuAccountOpenStateActive
			binding.ContractNo = arg.ContractNo
			binding.SharingMerID = arg.SharingMerID
			return binding, nil
		})
	store.EXPECT().MarkBaofuAccountOpeningFlowReady(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowReadyParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, int64(7101), arg.ID)
			require.Equal(t, pgtype.Int8{Int64: 6101, Valid: true}, arg.AccountBindingID)
			require.JSONEq(t, `{"transSerialNo":"OPEN123","state":"1","contractNo":"CM_BCT_123"}`, string(arg.RawSnapshot))
			flow.State = db.BaofuAccountOpeningStateReady
			flow.AccountBindingID = arg.AccountBindingID
			return flow, nil
		})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
}

func TestBaofuAccountOpenCallbackMerchantActiveRecordsLedgerAndWaitsForReport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"
	server.config.BaofuAccountVerifyFeeFen = 200

	flow := db.BaofuAccountOpeningFlow{
		ID:                7102,
		OwnerType:         db.BaofuAccountOwnerTypeMerchant,
		OwnerID:           88,
		AccountType:       db.BaofuAccountTypeBusiness,
		State:             db.BaofuAccountOpeningStateOpeningProcessing,
		OpenTransSerialNo: pgtype.Text{String: "OPEN123", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOM0000000088", Valid: true},
	}
	binding := db.BaofuAccountBinding{
		ID:          6102,
		OwnerType:   db.BaofuAccountOwnerTypeMerchant,
		OwnerID:     88,
		AccountType: db.BaofuAccountTypeBusiness,
		LoginNo:     pgtype.Text{String: "LLBFOM0000000088", Valid: true},
		OpenState:   db.BaofuAccountOpenStateProcessing,
	}
	store.EXPECT().GetBaofuAccountOpeningFlowByOpenTransSerialNo(gomock.Any(), pgtype.Text{String: "OPEN123", Valid: true}).Return(flow, nil)
	store.EXPECT().GetBaofuAccountBindingByContractNo(gomock.Any(), pgtype.Text{String: "CM_BCT_123", Valid: true}).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   88,
	}).Return(binding, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, pgtype.Text{String: db.BaofuAccountOwnerTypeMerchant, Valid: true}, arg.BusinessOwner)
			require.Equal(t, pgtype.Text{String: "baofu_account_opening_flow", Valid: true}, arg.BusinessObjectType)
			require.Equal(t, pgtype.Int8{Int64: 7102, Valid: true}, arg.BusinessObjectID)
			return db.ExternalPaymentFact{ID: 90, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().MarkBaofuAccountBindingActiveWithFeeLedgerTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountBindingActiveWithFeeLedgerTxParams) (db.MarkBaofuAccountBindingActiveWithFeeLedgerTxResult, error) {
			require.Equal(t, int64(6102), arg.ActiveBinding.ID)
			require.Equal(t, pgtype.Text{String: "CM_BCT_123", Valid: true}, arg.ActiveBinding.ContractNo)
			require.Equal(t, pgtype.Text{String: "CM_BCT_123", Valid: true}, arg.ActiveBinding.SharingMerID)
			require.Equal(t, db.BaofuFeeTypeAccountOpenVerifyFee, arg.AccountOpenFeeLedger.FeeType)
			require.Equal(t, db.BaofuFeePayerTypePlatform, arg.AccountOpenFeeLedger.PayerType)
			require.Equal(t, int64(200), arg.AccountOpenFeeLedger.Amount)
			require.Equal(t, int64(6102), arg.AccountOpenFeeLedger.BusinessObjectID)
			binding.OpenState = db.BaofuAccountOpenStateActive
			return db.MarkBaofuAccountBindingActiveWithFeeLedgerTxResult{Binding: binding}, nil
		})
	store.EXPECT().MarkBaofuAccountOpeningFlowMerchantReportProcessing(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowMerchantReportProcessingParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, int64(7102), arg.ID)
			require.Equal(t, pgtype.Int8{Int64: 6102, Valid: true}, arg.AccountBindingID)
			flow.State = db.BaofuAccountOpeningStateMerchantReportProcessing
			flow.AccountBindingID = arg.AccountBindingID
			return flow, nil
		})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
}

func TestBaofuAccountOpenCallbackMarksAbnormalBeforeAck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuAbnormalAccountParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	flow := db.BaofuAccountOpeningFlow{
		ID:                7103,
		OwnerType:         db.BaofuAccountOwnerTypeRider,
		OwnerID:           13,
		AccountType:       db.BaofuAccountTypePersonal,
		State:             db.BaofuAccountOpeningStateOpeningProcessing,
		OpenTransSerialNo: pgtype.Text{String: "OPEN_ABNORMAL", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOR0000000013", Valid: true},
	}
	binding := db.BaofuAccountBinding{
		ID:          6103,
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     13,
		OpenState:   db.BaofuAccountOpenStateProcessing,
		AccountType: db.BaofuAccountTypePersonal,
	}
	store.EXPECT().GetBaofuAccountOpeningFlowByOpenTransSerialNo(gomock.Any(), pgtype.Text{String: "OPEN_ABNORMAL", Valid: true}).Return(flow, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   13,
	}).Return(binding, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, pgtype.Text{String: db.BaofuAccountOwnerTypeRider, Valid: true}, arg.BusinessOwner)
			require.Equal(t, pgtype.Text{String: "baofu_account_opening_flow", Valid: true}, arg.BusinessObjectType)
			require.Equal(t, pgtype.Int8{Int64: 7103, Valid: true}, arg.BusinessObjectID)
			require.Equal(t, db.ExternalPaymentTerminalStatusUnknown, arg.TerminalStatus)
			require.False(t, arg.IsTerminal)
			require.Equal(t, "-1", arg.UpstreamState)
			return db.ExternalPaymentFact{ID: 89, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().MarkBaofuAccountBindingAbnormal(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountBindingAbnormalParams) (db.BaofuAccountBinding, error) {
			require.Equal(t, int64(6103), arg.ID)
			require.JSONEq(t, `{"transSerialNo":"OPEN_ABNORMAL","state":"-1"}`, string(arg.RawSnapshot))
			return db.BaofuAccountBinding{ID: arg.ID, OpenState: db.BaofuAccountOpenStateAbnormal}, nil
		})
	store.EXPECT().MarkBaofuAccountOpeningFlowFailed(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowFailedParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, int64(7103), arg.ID)
			require.JSONEq(t, `{"transSerialNo":"OPEN_ABNORMAL","state":"-1"}`, string(arg.RawSnapshot))
			flow.State = db.BaofuAccountOpeningStateFailed
			return flow, nil
		})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
}

func TestBaofuAccountOpenCallbackRejectsCollectIdentityMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})
	server.config.BaofuCollectMerchantID = "EXPECTED_MER"
	server.config.BaofuCollectTerminalID = "EXPECTED_TER"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "callback verification failed")
}

func TestBaofuAccountOpenCallbackPersistsAlertWhenFlowCannotBeMatched(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuUnmatchedAccountParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	store.EXPECT().
		GetBaofuAccountOpeningFlowByOpenTransSerialNo(gomock.Any(), pgtype.Text{String: "OPEN_MISSING", Valid: true}).
		Return(db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreatePlatformAlertEvent(gomock.Any(), gomock.AssignableToTypeOf(db.CreatePlatformAlertEventParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreatePlatformAlertEventParams) (db.PlatformAlertEvent, error) {
			require.Equal(t, string(worker.AlertTypeSystemError), arg.AlertType)
			require.Equal(t, string(worker.AlertLevelCritical), arg.Level)
			require.Equal(t, int64(0), arg.RelatedID)
			require.Equal(t, "baofu_account_opening_flow", arg.RelatedType)

			var extra map[string]any
			require.NoError(t, json.Unmarshal(arg.Extra, &extra))
			require.Equal(t, "OPEN_MISSING", extra["out_request_no"])
			require.Equal(t, "CP_MISSING", extra["contract_no"])
			require.Equal(t, "1", extra["upstream_state"])
			require.Equal(t, "callback_serial_no_unmatched", extra["reason"])
			return db.PlatformAlertEvent{ID: 9302}, nil
		})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Contains(t, recorder.Body.String(), "persist callback failed")
}

func TestBaofuAccountOpenCallbackBlocksWhenOutRequestNoMissesEvenIfContractExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuMismatchedSerialAccountParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	store.EXPECT().
		GetBaofuAccountOpeningFlowByOpenTransSerialNo(gomock.Any(), pgtype.Text{String: "OPEN_CALLBACK_OTHER", Valid: true}).
		Return(db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreatePlatformAlertEvent(gomock.Any(), gomock.AssignableToTypeOf(db.CreatePlatformAlertEventParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreatePlatformAlertEventParams) (db.PlatformAlertEvent, error) {
			require.Equal(t, string(worker.AlertTypeSystemError), arg.AlertType)
			require.Equal(t, string(worker.AlertLevelCritical), arg.Level)
			require.Equal(t, "baofu_account_opening_flow", arg.RelatedType)

			var extra map[string]any
			require.NoError(t, json.Unmarshal(arg.Extra, &extra))
			require.Equal(t, "OPEN_CALLBACK_OTHER", extra["out_request_no"])
			require.Equal(t, "CP_EXISTING", extra["contract_no"])
			require.Equal(t, "callback_serial_no_unmatched", extra["reason"])
			return db.PlatformAlertEvent{ID: 9303}, nil
		})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Contains(t, recorder.Body.String(), "persist callback failed")
}

func TestBaofuAccountOpenCallbackAcceptsOfficialQueryStringGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	flow := db.BaofuAccountOpeningFlow{
		ID:                7104,
		OwnerType:         db.BaofuAccountOwnerTypeRider,
		OwnerID:           14,
		AccountType:       db.BaofuAccountTypePersonal,
		State:             db.BaofuAccountOpeningStateOpeningProcessing,
		OpenTransSerialNo: pgtype.Text{String: "OPEN123", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOR0000000014", Valid: true},
	}
	binding := db.BaofuAccountBinding{
		ID:          6104,
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     14,
		AccountType: db.BaofuAccountTypePersonal,
		LoginNo:     pgtype.Text{String: "LLBFOR0000000014", Valid: true},
		OpenState:   db.BaofuAccountOpenStateProcessing,
	}
	store.EXPECT().GetBaofuAccountOpeningFlowByOpenTransSerialNo(gomock.Any(), pgtype.Text{String: "OPEN123", Valid: true}).Return(flow, nil)
	store.EXPECT().GetBaofuAccountBindingByContractNo(gomock.Any(), pgtype.Text{String: "CM_BCT_123", Valid: true}).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   14,
	}).Return(binding, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentFact{ID: 91}, nil)
	store.EXPECT().MarkBaofuAccountBindingActive(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountBindingActiveParams) (db.BaofuAccountBinding, error) {
			binding.OpenState = db.BaofuAccountOpenStateActive
			binding.ContractNo = arg.ContractNo
			binding.SharingMerID = arg.SharingMerID
			return binding, nil
		})
	store.EXPECT().MarkBaofuAccountOpeningFlowReady(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowReadyParams) (db.BaofuAccountOpeningFlow, error) {
			flow.State = db.BaofuAccountOpeningStateReady
			flow.AccountBindingID = arg.AccountBindingID
			return flow, nil
		})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/webhooks/baofu/account/open?member_id=102004465&terminal_id=200005200&data_type=JSON&data_content=ciphertext", http.NoBody)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
}

func TestBaofuWithdrawCallbackEnqueuesFactApplicationBeforeAck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})
	taskRecorder := &baofuWithdrawalFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	withdrawal := db.BaofuWithdrawalOrder{
		ID:           6101,
		OutRequestNo: "WD202605040001",
		Status:       db.BaofuWithdrawalStatusProcessing,
	}
	store.EXPECT().GetBaofuWithdrawalOrderByOutRequestNo(gomock.Any(), "WD202605040001").Return(withdrawal, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/withdraw", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, []int64{withdrawal.ID}, taskRecorder.withdrawalOrderIDs)
	require.Equal(t, []string{"3"}, taskRecorder.upstreamStates)
	require.Equal(t, []string{"BFWD202605040001"}, taskRecorder.baofuWithdrawNos)
}

func TestBaofuWithdrawCallbackRejectsPayoutIdentityMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})
	server.taskDistributor = &baofuWithdrawalFactApplicationEnqueueRecorder{}
	server.config.BaofuPayoutMerchantID = "EXPECTED_MER"
	server.config.BaofuPayoutTerminalID = "EXPECTED_TER"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/withdraw", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "callback verification failed")
}

func TestBaofuAccountCallbackPayloadUsesRawQueryWhenBodyEmpty(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open?member_id=102004465&terminal_id=200005200&data_type=JSON&data_content=ciphertext", http.NoBody)
	ctx := &gin.Context{Request: request}

	payload := baofuAccountCallbackPayload(ctx, nil)

	require.Equal(t, "member_id=102004465&terminal_id=200005200&data_type=JSON&data_content=ciphertext", string(payload))
}

func TestBaofuPaymentCallbackPersistsFactAndEnqueuesApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuPaymentParser{})
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	paymentOrder := db.PaymentOrder{
		ID:             4001,
		OutTradeNo:     "PO_BAOFU_4001",
		Amount:         1200,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   "order",
	}
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), "PO_BAOFU_4001").Return(paymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectBaofuPaymentOrder, arg.ExternalObjectType)
			require.Equal(t, "PO_BAOFU_4001", arg.ExternalObjectKey)
			require.Equal(t, "BFPAY_4001", arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, int64(1200), arg.Amount.Int64)
			require.Equal(t, "baofu:callback:payment:PO_BAOFU_4001:BFN_4001", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 501, IsTerminal: true, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().UpsertOrderPaymentFeeLedgerActual(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpsertOrderPaymentFeeLedgerActualParams) (db.OrderPaymentFeeLedger, error) {
			require.Equal(t, db.OrderPaymentFeeTypeProviderPaymentFee, arg.FeeType)
			require.Equal(t, db.OrderPaymentFeePayerTypePlatform, arg.PayerType)
			require.Equal(t, db.OrderPaymentFeePayeeTypeBaofu, arg.PayeeType)
			require.Equal(t, paymentOrder.ID, arg.PaymentOrderID)
			require.Equal(t, int64(1200), arg.BaseAmount)
			require.Equal(t, int64(4), arg.Amount)
			require.Equal(t, db.OrderPaymentFeeAmountSourceActualCallback, arg.AmountSource)
			require.Equal(t, int64(501), arg.ExternalPaymentFactID.Int64)
			return db.OrderPaymentFeeLedger{ID: 701, Amount: arg.Amount, AmountSource: arg.AmountSource}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             501,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   paymentOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 601, FactID: 501, Consumer: paymentFactConsumerOrderDomain, BusinessObjectType: paymentFactBusinessObjectPaymentOrder, BusinessObjectID: paymentOrder.ID}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/payment", bytes.NewBufferString(`{"notifyId":"BFN_4001"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, []int64{601}, taskRecorder.applicationIDs)
}

func TestBaofuPaymentCallbackCanLoadOrderByBaofuTradeNoWhenOutTradeNoMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuPaymentTradeNoOnlyParser{})
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	paymentOrder := db.PaymentOrder{
		ID:             4002,
		OutTradeNo:     "PO_BAOFU_4002",
		TransactionID:  pgtype.Text{String: "BFPAY_4002", Valid: true},
		Amount:         1200,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   "order",
	}
	store.EXPECT().GetPaymentOrderByTransactionId(gomock.Any(), pgtype.Text{String: "BFPAY_4002", Valid: true}).Return(paymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, "PO_BAOFU_4002", arg.ExternalObjectKey)
			require.Equal(t, "BFPAY_4002", arg.ExternalSecondaryKey.String)
			require.Equal(t, "baofu:callback:payment:PO_BAOFU_4002:BFN_4002", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 502, IsTerminal: true, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             502,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   paymentOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 602, FactID: 502, Consumer: paymentFactConsumerOrderDomain, BusinessObjectType: paymentFactBusinessObjectPaymentOrder, BusinessObjectID: paymentOrder.ID}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/payment", bytes.NewBufferString(`{"notifyId":"BFN_4002"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, []int64{602}, taskRecorder.applicationIDs)
}

func TestBaofuPaymentCallbackQueriesBaofooWhenTradeNoOnlyIsNotPersisted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuPaymentTradeNoOnlyParser{})
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder
	queryClient := &fakeBaofuAggregateQueryClient{outTradeNo: "PO_BAOFU_4003"}
	server.baofuAggregateClient = queryClient
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	paymentOrder := db.PaymentOrder{
		ID:             4003,
		OutTradeNo:     "PO_BAOFU_4003",
		Amount:         1200,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   "order",
	}
	store.EXPECT().GetPaymentOrderByTransactionId(gomock.Any(), pgtype.Text{String: "BFPAY_4002", Valid: true}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), "PO_BAOFU_4003").Return(paymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, "PO_BAOFU_4003", arg.ExternalObjectKey)
			require.Equal(t, "BFPAY_4002", arg.ExternalSecondaryKey.String)
			require.Equal(t, "baofu:callback:payment:PO_BAOFU_4003:BFN_4002", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 503, IsTerminal: true, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             503,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   paymentOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 603, FactID: 503, Consumer: paymentFactConsumerOrderDomain, BusinessObjectType: paymentFactBusinessObjectPaymentOrder, BusinessObjectID: paymentOrder.ID}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/payment", bytes.NewBufferString(`{"notifyId":"BFN_4002"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, "BFPAY_4002", queryClient.lastPaymentQuery.TradeNo)
	require.Equal(t, "102004465", queryClient.lastPaymentQuery.MerchantID)
	require.Equal(t, "200005200", queryClient.lastPaymentQuery.TerminalID)
	require.Equal(t, []int64{603}, taskRecorder.applicationIDs)
}

func TestBaofuPaymentCallbackFallbackRejectsNotificationIdentityMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	queryClient := &fakeBaofuAggregateQueryClient{outTradeNo: "PO_BAOFU_4004"}
	server.baofuAggregateClient = queryClient
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"
	notification := &baofuaggregatenotification.PaymentNotification{
		Fact: aggregatecontracts.PaymentFact{
			MerchantID: "102004999",
			TerminalID: "200005200",
			TradeNo:    "BFPAY_4004",
		},
	}

	outTradeNo, err := server.queryBaofuPaymentOutTradeNoForCallback(context.Background(), notification)

	require.Error(t, err)
	require.Empty(t, outTradeNo)
	require.Contains(t, err.Error(), "merId does not match configured collect merchant")
	require.Empty(t, queryClient.lastPaymentQuery.TradeNo)
}

func TestBaofuShareCallbackPersistsFactAndEnqueuesApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuPaymentParser{})
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	profitSharingOrder := db.ProfitSharingOrder{
		ID:             3001,
		PaymentOrderID: 4001,
		OutOrderNo:     "BFSHARE_3001",
		Status:         db.ProfitSharingOrderStatusProcessing,
		MerchantAmount: 8970,
		RiderAmount:    500,
	}
	store.EXPECT().GetProfitSharingOrderByOutOrderNo(gomock.Any(), "BFSHARE_3001").Return(profitSharingOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuProfitSharing, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectProfitSharing, arg.ExternalObjectType)
			require.Equal(t, "BFSHARE_3001", arg.ExternalObjectKey)
			require.Equal(t, "BFSHARE_UP_3001", arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, int64(9470), arg.Amount.Int64)
			require.Equal(t, "baofu:callback:profit_sharing:BFSHARE_3001:BFSN_3001", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 701, IsTerminal: true, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             701,
		Consumer:           paymentFactConsumerProfitSharingDomain,
		BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder,
		BusinessObjectID:   profitSharingOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 801, FactID: 701, Consumer: paymentFactConsumerProfitSharingDomain, BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder, BusinessObjectID: profitSharingOrder.ID}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/share", bytes.NewBufferString(`{"notifyId":"BFSN_3001"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, []int64{801}, taskRecorder.applicationIDs)
}

func TestBaofuShareCallbackQueriesBaofooWhenOutTradeNoMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuShareTradeNoOnlyParser{})
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder
	queryClient := &fakeBaofuAggregateQueryClient{shareOutTradeNo: "BFSHARE_3003"}
	server.baofuAggregateClient = queryClient
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	profitSharingOrder := db.ProfitSharingOrder{
		ID:             3003,
		PaymentOrderID: 4003,
		OutOrderNo:     "BFSHARE_3003",
		Status:         db.ProfitSharingOrderStatusProcessing,
		MerchantAmount: 8970,
		RiderAmount:    500,
	}
	store.EXPECT().GetProfitSharingOrderByOutOrderNo(gomock.Any(), "BFSHARE_3003").Return(profitSharingOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, "BFSHARE_3003", arg.ExternalObjectKey)
			require.Equal(t, "BFSHARE_UP_3003", arg.ExternalSecondaryKey.String)
			require.Equal(t, "baofu:callback:profit_sharing:BFSHARE_3003:BFSN_3003", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 703, IsTerminal: true, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             703,
		Consumer:           paymentFactConsumerProfitSharingDomain,
		BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder,
		BusinessObjectID:   profitSharingOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 803, FactID: 703, Consumer: paymentFactConsumerProfitSharingDomain, BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder, BusinessObjectID: profitSharingOrder.ID}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/share", bytes.NewBufferString(`{"notifyId":"BFSN_3003"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, "BFSHARE_UP_3003", queryClient.lastShareQuery.TradeNo)
	require.Equal(t, "102004465", queryClient.lastShareQuery.MerchantID)
	require.Equal(t, "200005200", queryClient.lastShareQuery.TerminalID)
	require.Equal(t, []int64{803}, taskRecorder.applicationIDs)
}

func TestBaofuShareCallbackFallbackRejectsNotificationIdentityMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	queryClient := &fakeBaofuAggregateQueryClient{shareOutTradeNo: "BFSHARE_3004"}
	server.baofuAggregateClient = queryClient
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"
	notification := &baofuaggregatenotification.ShareNotification{
		Fact: aggregatecontracts.ShareFact{
			MerchantID: "102004465",
			TerminalID: "200005299",
			TradeNo:    "BFSHARE_UP_3004",
		},
	}

	outTradeNo, err := server.queryBaofuShareOutOrderNoForCallback(context.Background(), notification)

	require.Error(t, err)
	require.Empty(t, outTradeNo)
	require.Contains(t, err.Error(), "terId does not match configured collect terminal")
	require.Empty(t, queryClient.lastShareQuery.TradeNo)
}

func TestBaofuShareCallbackRejectsWhenSignedParserNotConfigured(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/share", bytes.NewBufferString(`{"notifyId":"BFSN_3002","notifyType":"SHARING","outTradeNo":"BFSHARE_3002","tradeNo":"BFSHARE_UP_3002","txnState":"SUCCESS","resultCode":"SUCCESS","succAmt":9470}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Empty(t, taskRecorder.applicationIDs)
}

func TestBaofuRefundCallbackPersistsFactAndEnqueuesApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuPaymentParser{})
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	refundOrder := db.RefundOrder{
		ID:             5101,
		PaymentOrderID: 4001,
		OutRefundNo:    "BFRFD_5101",
		RefundAmount:   1200,
		Status:         "processing",
	}
	paymentOrder := db.PaymentOrder{
		ID:             4001,
		OrderID:        pgtype.Int8{Int64: 3101, Valid: true},
		Amount:         1200,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
	}
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), "BFRFD_5101").Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuRefund, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectRefund, arg.ExternalObjectType)
			require.Equal(t, refundOrder.OutRefundNo, arg.ExternalObjectKey)
			require.Equal(t, "BFREFUND_UP_5101", arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, refundOrder.RefundAmount, arg.Amount.Int64)
			require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner.String)
			require.Equal(t, paymentFactBusinessObjectRefundOrder, arg.BusinessObjectType.String)
			require.Equal(t, refundOrder.ID, arg.BusinessObjectID.Int64)
			require.Equal(t, "baofu:callback:refund:BFRFD_5101:BFRN_5101", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 901, IsTerminal: true, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             901,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   refundOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 1001, FactID: 901, Consumer: paymentFactConsumerOrderDomain, BusinessObjectType: paymentFactBusinessObjectRefundOrder, BusinessObjectID: refundOrder.ID}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/refund", bytes.NewBufferString(`{"notifyId":"BFRN_5101"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, []int64{1001}, taskRecorder.applicationIDs)
}

type fakeBaofuOpenAccountParser struct{}

func (fakeBaofuOpenAccountParser) ParseOpenAccountNotification(body []byte) (*baofunotification.AccountNotification, error) {
	return &baofunotification.AccountNotification{
		MemberID:      "102004465",
		TerminalID:    "200005200",
		OutRequestNo:  "OPEN123",
		ContractNo:    "CM_BCT_123",
		UpstreamState: "1",
		OpenState:     db.BaofuAccountOpenStateActive,
		OccurredAt:    time.Now().UTC(),
		Raw:           []byte(`{"transSerialNo":"OPEN123","state":"1","contractNo":"CM_BCT_123"}`),
	}, nil
}

type fakeBaofuAbnormalAccountParser struct{}

func (fakeBaofuAbnormalAccountParser) ParseOpenAccountNotification(body []byte) (*baofunotification.AccountNotification, error) {
	return &baofunotification.AccountNotification{
		MemberID:      "102004465",
		TerminalID:    "200005200",
		OutRequestNo:  "OPEN_ABNORMAL",
		UpstreamState: "-1",
		OpenState:     db.BaofuAccountOpenStateAbnormal,
		OccurredAt:    time.Now().UTC(),
		Raw:           []byte(`{"transSerialNo":"OPEN_ABNORMAL","state":"-1"}`),
	}, nil
}

func (fakeBaofuAbnormalAccountParser) ParseWithdrawNotification(body []byte) (*baofunotification.WithdrawNotification, error) {
	return fakeBaofuOpenAccountParser{}.ParseWithdrawNotification(body)
}

type fakeBaofuUnmatchedAccountParser struct{}

func (fakeBaofuUnmatchedAccountParser) ParseOpenAccountNotification(body []byte) (*baofunotification.AccountNotification, error) {
	return &baofunotification.AccountNotification{
		MemberID:      "102004465",
		TerminalID:    "200005200",
		OutRequestNo:  "OPEN_MISSING",
		ContractNo:    "CP_MISSING",
		UpstreamState: "1",
		OpenState:     db.BaofuAccountOpenStateActive,
		OccurredAt:    time.Now().UTC(),
		Raw:           []byte(`{"transSerialNo":"OPEN_MISSING","state":"1","contractNo":"CP_MISSING"}`),
	}, nil
}

func (fakeBaofuUnmatchedAccountParser) ParseWithdrawNotification(body []byte) (*baofunotification.WithdrawNotification, error) {
	return fakeBaofuOpenAccountParser{}.ParseWithdrawNotification(body)
}

type fakeBaofuMismatchedSerialAccountParser struct{}

func (fakeBaofuMismatchedSerialAccountParser) ParseOpenAccountNotification(body []byte) (*baofunotification.AccountNotification, error) {
	return &baofunotification.AccountNotification{
		MemberID:      "102004465",
		TerminalID:    "200005200",
		OutRequestNo:  "OPEN_CALLBACK_OTHER",
		ContractNo:    "CP_EXISTING",
		UpstreamState: "1",
		OpenState:     db.BaofuAccountOpenStateActive,
		OccurredAt:    time.Now().UTC(),
		Raw:           []byte(`{"transSerialNo":"OPEN_CALLBACK_OTHER","state":"1","contractNo":"CP_EXISTING"}`),
	}, nil
}

func (fakeBaofuMismatchedSerialAccountParser) ParseWithdrawNotification(body []byte) (*baofunotification.WithdrawNotification, error) {
	return fakeBaofuOpenAccountParser{}.ParseWithdrawNotification(body)
}

func (fakeBaofuOpenAccountParser) ParseWithdrawNotification(body []byte) (*baofunotification.WithdrawNotification, error) {
	return &baofunotification.WithdrawNotification{
		MemberID:        "102004466",
		TerminalID:      "200005201",
		TransSerialNo:   "WD202605040001",
		BaofuWithdrawNo: "BFWD202605040001",
		ContractNo:      "CM_BCT_123",
		UpstreamState:   "3",
		Status:          db.BaofuWithdrawalStatusReturned,
		AmountFen:       12345,
		FeeFen:          100,
		TotalAmountFen:  12445,
		OccurredAt:      time.Now().UTC(),
		Raw:             []byte(`{"transSerialNo":"WD202605040001","orderId":"BFWD202605040001","state":"3"}`),
	}, nil
}

type baofuWithdrawalFactApplicationEnqueueRecorder struct {
	worker.NoopTaskDistributor
	withdrawalOrderIDs []int64
	upstreamStates     []string
	baofuWithdrawNos   []string
}

func (r *baofuWithdrawalFactApplicationEnqueueRecorder) DistributeTaskProcessBaofuWithdrawalFactApplication(_ context.Context, payload *worker.BaofuWithdrawalFactApplicationPayload, _ ...asynq.Option) error {
	r.withdrawalOrderIDs = append(r.withdrawalOrderIDs, payload.WithdrawalOrderID)
	r.upstreamStates = append(r.upstreamStates, payload.UpstreamState)
	r.baofuWithdrawNos = append(r.baofuWithdrawNos, payload.BaofuWithdrawNo)
	return nil
}

type fakeBaofuPaymentParser struct{}

func (fakeBaofuPaymentParser) ParsePaymentNotification(body []byte) (*baofuaggregatenotification.PaymentNotification, error) {
	return &baofuaggregatenotification.PaymentNotification{
		NotifyID:       "BFN_4001",
		NotifyType:     "PAYMENT",
		TerminalStatus: db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:     true,
		OccurredAt:     time.Now().UTC(),
		Raw:            []byte(`{"notifyId":"BFN_4001","outTradeNo":"PO_BAOFU_4001","tradeNo":"BFPAY_4001","txnState":"SUCCESS"}`),
		Fact: aggregatecontracts.PaymentFact{
			OutTradeNo:       "PO_BAOFU_4001",
			TradeNo:          "BFPAY_4001",
			TransactionState: aggregatecontracts.PaymentStateSuccess,
			SuccessAmountFen: 1200,
			FeeAmountFen:     4,
			Raw:              []byte(`{"notifyId":"BFN_4001","outTradeNo":"PO_BAOFU_4001","tradeNo":"BFPAY_4001","txnState":"SUCCESS"}`),
		},
	}, nil
}

func (fakeBaofuPaymentParser) ParseShareNotification(body []byte) (*baofuaggregatenotification.ShareNotification, error) {
	return &baofuaggregatenotification.ShareNotification{
		NotifyID:       "BFSN_3001",
		NotifyType:     "SHARING",
		TerminalStatus: db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:     true,
		OccurredAt:     time.Now().UTC(),
		Raw:            []byte(`{"notifyId":"BFSN_3001","outTradeNo":"BFSHARE_3001","tradeNo":"BFSHARE_UP_3001","txnState":"SUCCESS"}`),
		Fact: aggregatecontracts.ShareFact{
			OutTradeNo:       "BFSHARE_3001",
			TradeNo:          "BFSHARE_UP_3001",
			TransactionState: aggregatecontracts.ShareStateSuccess,
			SuccessAmountFen: 9470,
			Raw:              []byte(`{"notifyId":"BFSN_3001","outTradeNo":"BFSHARE_3001","tradeNo":"BFSHARE_UP_3001","txnState":"SUCCESS"}`),
		},
	}, nil
}

func (fakeBaofuPaymentParser) ParseRefundNotification(body []byte) (*baofuaggregatenotification.RefundNotification, error) {
	return &baofuaggregatenotification.RefundNotification{
		NotifyID:       "BFRN_5101",
		NotifyType:     "REFUND",
		TerminalStatus: db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:     true,
		OccurredAt:     time.Now().UTC(),
		Raw:            []byte(`{"notifyId":"BFRN_5101","outTradeNo":"BFRFD_5101","tradeNo":"BFREFUND_UP_5101","refundState":"SUCCESS"}`),
		Fact: aggregatecontracts.RefundFact{
			OutTradeNo:       "BFRFD_5101",
			TradeNo:          "BFREFUND_UP_5101",
			TransactionState: aggregatecontracts.RefundStateSuccess,
			SuccessAmountFen: 1200,
			Raw:              []byte(`{"notifyId":"BFRN_5101","outTradeNo":"BFRFD_5101","tradeNo":"BFREFUND_UP_5101","refundState":"SUCCESS"}`),
		},
	}, nil
}

type fakeBaofuPaymentTradeNoOnlyParser struct{}

func (fakeBaofuPaymentTradeNoOnlyParser) ParsePaymentNotification(body []byte) (*baofuaggregatenotification.PaymentNotification, error) {
	return &baofuaggregatenotification.PaymentNotification{
		NotifyID:       "BFN_4002",
		NotifyType:     "PAYMENT",
		TerminalStatus: db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:     true,
		OccurredAt:     time.Now().UTC(),
		Raw:            []byte(`{"notifyId":"BFN_4002","tradeNo":"BFPAY_4002","txnState":"SUCCESS"}`),
		Fact: aggregatecontracts.PaymentFact{
			TradeNo:          "BFPAY_4002",
			TransactionState: aggregatecontracts.PaymentStateSuccess,
			SuccessAmountFen: 1200,
			Raw:              []byte(`{"notifyId":"BFN_4002","tradeNo":"BFPAY_4002","txnState":"SUCCESS"}`),
		},
	}, nil
}

func (fakeBaofuPaymentTradeNoOnlyParser) ParseShareNotification(body []byte) (*baofuaggregatenotification.ShareNotification, error) {
	return fakeBaofuPaymentParser{}.ParseShareNotification(body)
}

func (fakeBaofuPaymentTradeNoOnlyParser) ParseRefundNotification(body []byte) (*baofuaggregatenotification.RefundNotification, error) {
	return fakeBaofuPaymentParser{}.ParseRefundNotification(body)
}

type fakeBaofuShareTradeNoOnlyParser struct{}

func (fakeBaofuShareTradeNoOnlyParser) ParsePaymentNotification(body []byte) (*baofuaggregatenotification.PaymentNotification, error) {
	return fakeBaofuPaymentParser{}.ParsePaymentNotification(body)
}

func (fakeBaofuShareTradeNoOnlyParser) ParseShareNotification(body []byte) (*baofuaggregatenotification.ShareNotification, error) {
	return &baofuaggregatenotification.ShareNotification{
		NotifyID:       "BFSN_3003",
		NotifyType:     "SHARING",
		TerminalStatus: db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:     true,
		OccurredAt:     time.Now().UTC(),
		Raw:            []byte(`{"notifyId":"BFSN_3003","tradeNo":"BFSHARE_UP_3003","txnState":"SUCCESS","resultCode":"SUCCESS"}`),
		Fact: aggregatecontracts.ShareFact{
			TradeNo:          "BFSHARE_UP_3003",
			TransactionState: aggregatecontracts.ShareStateSuccess,
			SuccessAmountFen: 9470,
			ResultCode:       "SUCCESS",
			Raw:              []byte(`{"notifyId":"BFSN_3003","tradeNo":"BFSHARE_UP_3003","txnState":"SUCCESS","resultCode":"SUCCESS"}`),
		},
	}, nil
}

func (fakeBaofuShareTradeNoOnlyParser) ParseRefundNotification(body []byte) (*baofuaggregatenotification.RefundNotification, error) {
	return fakeBaofuPaymentParser{}.ParseRefundNotification(body)
}

type fakeBaofuAggregateQueryClient struct {
	outTradeNo       string
	shareOutTradeNo  string
	lastPaymentQuery aggregatecontracts.PaymentQueryRequest
	lastShareQuery   aggregatecontracts.ShareQueryRequest
}

func (c *fakeBaofuAggregateQueryClient) CreateUnifiedOrder(context.Context, aggregatecontracts.UnifiedOrderRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, errors.New("not implemented in baofu callback test")
}

func (c *fakeBaofuAggregateQueryClient) QueryPayment(_ context.Context, req aggregatecontracts.PaymentQueryRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	c.lastPaymentQuery = req
	return &aggregatecontracts.UnifiedOrderResult{OutTradeNo: c.outTradeNo, TradeNo: req.TradeNo}, nil
}

func (c *fakeBaofuAggregateQueryClient) CreateProfitSharing(context.Context, aggregatecontracts.ShareAfterPayRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, errors.New("not implemented in baofu callback test")
}

func (c *fakeBaofuAggregateQueryClient) QueryProfitSharing(_ context.Context, req aggregatecontracts.ShareQueryRequest) (*aggregatecontracts.ShareResult, error) {
	c.lastShareQuery = req
	return &aggregatecontracts.ShareResult{OutTradeNo: c.shareOutTradeNo, TradeNo: req.TradeNo}, nil
}

func (c *fakeBaofuAggregateQueryClient) CreateRefund(context.Context, aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, errors.New("not implemented in baofu callback test")
}

func (c *fakeBaofuAggregateQueryClient) QueryRefund(context.Context, aggregatecontracts.RefundQueryRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, errors.New("not implemented in baofu callback test")
}

func (c *fakeBaofuAggregateQueryClient) CloseOrder(context.Context, aggregatecontracts.OrderCloseRequest) (*aggregatecontracts.OrderCloseResult, error) {
	return nil, errors.New("not implemented in baofu callback test")
}

var _ = gin.TestMode
