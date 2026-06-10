package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofunotification "github.com/merrydance/locallife/baofu/account/notification"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	baofuaggregatenotification "github.com/merrydance/locallife/baofu/aggregatepay/notification"
	"github.com/merrydance/locallife/baofu/merchantreport"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
			require.JSONEq(t, `{"provider":"baofu","capability":"account","source_path":"body.state","result_state":"1"}`, string(arg.RawSnapshot))
			binding.OpenState = db.BaofuAccountOpenStateActive
			binding.ContractNo = arg.ContractNo
			binding.SharingMerID = arg.SharingMerID
			return binding, nil
		})
	store.EXPECT().MarkBaofuAccountOpeningFlowReady(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowReadyParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, int64(7101), arg.ID)
			require.Equal(t, pgtype.Int8{Int64: 6101, Valid: true}, arg.AccountBindingID)
			require.JSONEq(t, `{"provider":"baofu","capability":"account","source_path":"body.state","result_state":"1"}`, string(arg.RawSnapshot))
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

func TestBaofuAccountOpenCallbackMerchantReportFailureStillAcksAccountCallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"
	server.config.WechatMiniAppID = "wx1234567890abcdef"
	server.config.BaofuMerchantReportChannelID = "CH001"
	server.config.BaofuMerchantReportChannelName = "LocalLife"
	server.config.BaofuAccountVerifyFeeFen = 200
	server.baofuMerchantReportClient = merchantreport.NewClient(testAPIBaofuRootClient(t, baofuFailingDoer{}))

	flow := db.BaofuAccountOpeningFlow{
		ID:                7106,
		OwnerType:         db.BaofuAccountOwnerTypeMerchant,
		OwnerID:           88,
		AccountType:       db.BaofuAccountTypeBusiness,
		ProfileID:         pgtype.Int8{Int64: 8106, Valid: true},
		State:             db.BaofuAccountOpeningStateOpeningProcessing,
		OpenTransSerialNo: pgtype.Text{String: "OPEN123", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOM0000000088", Valid: true},
	}
	binding := db.BaofuAccountBinding{
		ID:          6106,
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
	}).DoAndReturn(func(context.Context, db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error) {
		return binding, nil
	}).Times(2)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentFact{ID: 91, DedupeKey: "baofu:callback:account:OPEN123:1"}, nil)
	store.EXPECT().MarkBaofuAccountBindingActiveWithFeeLedgerTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountBindingActiveWithFeeLedgerTxParams) (db.MarkBaofuAccountBindingActiveWithFeeLedgerTxResult, error) {
			binding.OpenState = db.BaofuAccountOpenStateActive
			binding.ContractNo = arg.ActiveBinding.ContractNo
			binding.SharingMerID = arg.ActiveBinding.SharingMerID
			return db.MarkBaofuAccountBindingActiveWithFeeLedgerTxResult{Binding: binding}, nil
		})
	store.EXPECT().MarkBaofuAccountOpeningFlowMerchantReportProcessing(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowMerchantReportProcessingParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, int64(7106), arg.ID)
			require.Equal(t, pgtype.Int8{Int64: 6106, Valid: true}, arg.AccountBindingID)
			flow.State = db.BaofuAccountOpeningStateMerchantReportProcessing
			flow.AccountBindingID = arg.AccountBindingID
			flow.MerchantReportID = pgtype.Int8{Int64: 9106, Valid: true}
			return flow, nil
		})
	store.EXPECT().GetBaofuMerchantReportByOwner(gomock.Any(), db.GetBaofuMerchantReportByOwnerParams{
		OwnerType:  db.BaofuAccountOwnerTypeMerchant,
		OwnerID:    88,
		ReportType: db.BaofuMerchantReportTypeWechat,
	}).Return(db.BaofuMerchantReport{}, db.ErrRecordNotFound)
	store.EXPECT().GetBaofuAccountOpeningProfile(gomock.Any(), int64(8106)).
		Return(db.BaofuAccountOpeningProfile{
			ID:                      8106,
			OwnerType:               db.BaofuAccountOwnerTypeMerchant,
			OwnerID:                 88,
			AccountType:             db.BaofuAccountTypeBusiness,
			LegalName:               pgtype.Text{String: "测试餐饮有限公司", Valid: true},
			CertificateNoCiphertext: pgtype.Text{String: "91330100MA000001", Valid: true},
			BankAccountNoCiphertext: pgtype.Text{String: "6222020202020202", Valid: true},
			DepositBankName:         pgtype.Text{String: "招商银行杭州支行", Valid: true},
			ContactMobileCiphertext: pgtype.Text{String: "057112345678", Valid: true},
			SourceSnapshot:          []byte(`{"self_employed":false}`),
		}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(88)).
		Return(db.Merchant{ID: 88, Status: db.MerchantStatusApproved, Name: "测试餐饮", Phone: "057112345678", Address: "测试路 1 号", RegionID: 330106}, nil)
	store.EXPECT().GetRegion(gomock.Any(), int64(330106)).
		Return(db.Region{ID: 330106, Code: "330106", Level: 3, ParentID: pgtype.Int8{Int64: 330100, Valid: true}}, nil)
	store.EXPECT().GetRegion(gomock.Any(), int64(330100)).
		Return(db.Region{ID: 330100, Code: "330100", Level: 2, ParentID: pgtype.Int8{Int64: 330000, Valid: true}}, nil)
	store.EXPECT().GetRegion(gomock.Any(), int64(330000)).
		Return(db.Region{ID: 330000, Code: "330000", Level: 1}, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentCommandTypeBaofuMerchantReport, arg.CommandType)
			require.Equal(t, "baofu_merchant_report", arg.ExternalObjectType)
			return db.ExternalPaymentCommand{ID: 8106, CommandType: arg.CommandType}, nil
		})
	store.EXPECT().UpsertBaofuMerchantReportProcessing(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpsertBaofuMerchantReportProcessingParams) (db.BaofuMerchantReport, error) {
			require.Equal(t, db.BaofuAccountOwnerTypeMerchant, arg.OwnerType)
			require.Equal(t, int64(88), arg.OwnerID)
			require.Equal(t, db.BaofuMerchantReportTypeWechat, arg.ReportType)
			require.NotEmpty(t, arg.ReportNo)
			require.Equal(t, "CM_BCT_123", arg.BctMerID)
			return db.BaofuMerchantReport{
				ID:              9106,
				OwnerType:       arg.OwnerType,
				OwnerID:         arg.OwnerID,
				ReportType:      arg.ReportType,
				ReportNo:        arg.ReportNo,
				BctMerID:        arg.BctMerID,
				ReportState:     db.BaofuMerchantReportStateProcessing,
				AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
				RawSnapshot:     arg.RawSnapshot,
			}, nil
		})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
}

func TestBaofuAccountOpenCallbackAcceptsDuplicateFactBeforeAck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	flow := db.BaofuAccountOpeningFlow{
		ID:                7105,
		OwnerType:         db.BaofuAccountOwnerTypeMerchant,
		OwnerID:           88,
		AccountType:       db.BaofuAccountTypeBusiness,
		State:             db.BaofuAccountOpeningStateMerchantReportProcessing,
		OpenTransSerialNo: pgtype.Text{String: "OPEN123", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOM0000000088", Valid: true},
	}
	binding := db.BaofuAccountBinding{
		ID:           6105,
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      88,
		AccountType:  db.BaofuAccountTypeBusiness,
		ContractNo:   pgtype.Text{String: "CM_BCT_123", Valid: true},
		SharingMerID: pgtype.Text{String: "CM_BCT_123", Valid: true},
		LoginNo:      pgtype.Text{String: "LLBFOM0000000088", Valid: true},
		OpenState:    db.BaofuAccountOpenStateActive,
	}
	dedupeKey := "baofu:callback:account:OPEN123:1"

	store.EXPECT().GetBaofuAccountOpeningFlowByOpenTransSerialNo(gomock.Any(), pgtype.Text{String: "OPEN123", Valid: true}).Return(flow, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, dedupeKey, arg.DedupeKey)
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuAccount, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, "OPEN123", arg.ExternalObjectKey)
			require.Equal(t, "1", arg.UpstreamState)
			return db.ExternalPaymentFact{}, db.ErrRecordNotFound
		})
	store.EXPECT().GetExternalPaymentFactByDedupeKey(gomock.Any(), dedupeKey).
		Return(db.ExternalPaymentFact{
			ID:                   95,
			Provider:             db.ExternalPaymentProviderBaofu,
			Channel:              db.PaymentChannelBaofuAggregate,
			Capability:           db.ExternalPaymentCapabilityBaofuAccount,
			FactSource:           db.ExternalPaymentFactSourceCallback,
			SourceEventID:        pgtype.Text{String: "OPEN123", Valid: true},
			SourceEventType:      pgtype.Text{String: "BAOFU_ACCOUNT_OPEN", Valid: true},
			ExternalObjectType:   "baofu_account",
			ExternalObjectKey:    "OPEN123",
			ExternalSecondaryKey: pgtype.Text{String: "CM_BCT_123", Valid: true},
			BusinessOwner:        pgtype.Text{String: db.BaofuAccountOwnerTypeMerchant, Valid: true},
			BusinessObjectType:   pgtype.Text{String: "baofu_account_opening_flow", Valid: true},
			BusinessObjectID:     pgtype.Int8{Int64: 7105, Valid: true},
			UpstreamState:        "1",
			TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:           true,
			Currency:             "CNY",
			DedupeKey:            dedupeKey,
			ProcessingStatus:     db.ExternalPaymentFactProcessingStatusTerminalized,
		}, nil)
	store.EXPECT().GetBaofuAccountBindingByContractNo(gomock.Any(), pgtype.Text{String: "CM_BCT_123", Valid: true}).Return(binding, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   88,
	}).Return(binding, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
}

func TestBaofuAccountOpenCallbackDuplicateFactRecoversFailedFlowBeforeAck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	flow := db.BaofuAccountOpeningFlow{
		ID:                7115,
		OwnerType:         db.BaofuAccountOwnerTypeRider,
		OwnerID:           2,
		AccountType:       db.BaofuAccountTypePersonal,
		State:             db.BaofuAccountOpeningStateFailed,
		FailureCode:       pgtype.Text{String: "BF0003", Valid: true},
		FailureMessage:    pgtype.Text{String: "支付通道异常，请联系平台处理", Valid: true},
		OpenTransSerialNo: pgtype.Text{String: "OPEN123", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOR0000000002", Valid: true},
	}
	binding := db.BaofuAccountBinding{
		ID:                    6115,
		OwnerType:             db.BaofuAccountOwnerTypeRider,
		OwnerID:               2,
		AccountType:           db.BaofuAccountTypePersonal,
		ContractNo:            pgtype.Text{String: "CM_BCT_123", Valid: true},
		SharingMerID:          pgtype.Text{String: "CM_BCT_123", Valid: true},
		LoginNo:               pgtype.Text{String: "LLBFOR0000000002", Valid: true},
		OpenState:             db.BaofuAccountOpenStateActive,
		LastOpenTransSerialNo: pgtype.Text{String: "OPEN123", Valid: true},
		RawSnapshot:           []byte(`{"state":"active"}`),
	}
	dedupeKey := "baofu:callback:account:OPEN123:1"

	store.EXPECT().GetBaofuAccountOpeningFlowByOpenTransSerialNo(gomock.Any(), pgtype.Text{String: "OPEN123", Valid: true}).Return(flow, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentFact{}, db.ErrRecordNotFound)
	store.EXPECT().GetExternalPaymentFactByDedupeKey(gomock.Any(), dedupeKey).Return(db.ExternalPaymentFact{
		ID:                   115,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuAccount,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        pgtype.Text{String: "OPEN123", Valid: true},
		SourceEventType:      pgtype.Text{String: "BAOFU_ACCOUNT_OPEN", Valid: true},
		ExternalObjectType:   "baofu_account",
		ExternalObjectKey:    "OPEN123",
		ExternalSecondaryKey: pgtype.Text{String: "CM_BCT_123", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.BaofuAccountOwnerTypeRider, Valid: true},
		BusinessObjectType:   pgtype.Text{String: "baofu_account_opening_flow", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: 7115, Valid: true},
		UpstreamState:        "1",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
		Currency:             "CNY",
		DedupeKey:            dedupeKey,
		ProcessingStatus:     db.ExternalPaymentFactProcessingStatusReceived,
	}, nil)
	store.EXPECT().GetBaofuAccountBindingByContractNo(gomock.Any(), pgtype.Text{String: "CM_BCT_123", Valid: true}).Return(binding, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   2,
	}).Return(binding, nil).Times(2)
	store.EXPECT().RecoverFailedBaofuAccountOpeningFlowFromActiveBinding(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.RecoverFailedBaofuAccountOpeningFlowFromActiveBindingParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, int64(7115), arg.ID)
			require.Equal(t, pgtype.Int8{Int64: 6115, Valid: true}, arg.AccountBindingID)
			require.Equal(t, pgtype.Text{String: "OPEN123", Valid: true}, arg.OpenTransSerialNo)
			require.Equal(t, pgtype.Text{String: "CM_BCT_123", Valid: true}, arg.ContractNo)
			require.JSONEq(t, `{"provider":"baofu","capability":"account","source_path":"body.state","result_state":"1"}`, string(arg.RawSnapshot))
			flow.State = db.BaofuAccountOpeningStateReady
			flow.AccountBindingID = arg.AccountBindingID
			flow.FailureCode = pgtype.Text{}
			flow.FailureMessage = pgtype.Text{}
			return flow, nil
		})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
}

func TestBaofuAccountOpenCallbackRejectsMismatchedDuplicateFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	flow := db.BaofuAccountOpeningFlow{
		ID:                7106,
		OwnerType:         db.BaofuAccountOwnerTypeMerchant,
		OwnerID:           88,
		AccountType:       db.BaofuAccountTypeBusiness,
		State:             db.BaofuAccountOpeningStateMerchantReportProcessing,
		OpenTransSerialNo: pgtype.Text{String: "OPEN123", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOM0000000088", Valid: true},
	}
	dedupeKey := "baofu:callback:account:OPEN123:1"

	store.EXPECT().GetBaofuAccountOpeningFlowByOpenTransSerialNo(gomock.Any(), pgtype.Text{String: "OPEN123", Valid: true}).Return(flow, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentFact{}, db.ErrRecordNotFound)
	store.EXPECT().GetExternalPaymentFactByDedupeKey(gomock.Any(), dedupeKey).
		Return(db.ExternalPaymentFact{
			ID:                   96,
			Provider:             db.ExternalPaymentProviderBaofu,
			Channel:              db.PaymentChannelBaofuAggregate,
			Capability:           db.ExternalPaymentCapabilityBaofuAccount,
			FactSource:           db.ExternalPaymentFactSourceCallback,
			SourceEventID:        pgtype.Text{String: "OPEN123", Valid: true},
			SourceEventType:      pgtype.Text{String: "BAOFU_ACCOUNT_OPEN", Valid: true},
			ExternalObjectType:   "baofu_account",
			ExternalObjectKey:    "OPEN123",
			ExternalSecondaryKey: pgtype.Text{String: "CM_BCT_123", Valid: true},
			BusinessOwner:        pgtype.Text{String: db.BaofuAccountOwnerTypeMerchant, Valid: true},
			BusinessObjectType:   pgtype.Text{String: "baofu_account_opening_flow", Valid: true},
			BusinessObjectID:     pgtype.Int8{Int64: 9999, Valid: true},
			UpstreamState:        "1",
			TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
			IsTerminal:           true,
			Currency:             "CNY",
			DedupeKey:            dedupeKey,
			ProcessingStatus:     db.ExternalPaymentFactProcessingStatusTerminalized,
		}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Contains(t, recorder.Body.String(), "persist callback failed")
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
			require.JSONEq(t, `{"provider":"baofu","capability":"account","source_path":"body.state","result_state":"-1"}`, string(arg.RawSnapshot))
			return db.BaofuAccountBinding{ID: arg.ID, OpenState: db.BaofuAccountOpenStateAbnormal}, nil
		})
	store.EXPECT().MarkBaofuAccountOpeningFlowFailed(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowFailedParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, int64(7103), arg.ID)
			require.JSONEq(t, `{"state":"failed","provider_diagnostic":{"provider":"baofu","capability":"account","source_path":"body.state","result_state":"-1"}}`, string(arg.RawSnapshot))
			flow.State = db.BaofuAccountOpeningStateFailed
			return flow, nil
		})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
}

func TestBaofuAccountOpenCallbackPersistsFailureReasonBeforeAck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuFailedAccountParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	flow := db.BaofuAccountOpeningFlow{
		ID:                7104,
		OwnerType:         db.BaofuAccountOwnerTypeRider,
		OwnerID:           14,
		AccountType:       db.BaofuAccountTypePersonal,
		State:             db.BaofuAccountOpeningStateOpeningProcessing,
		OpenTransSerialNo: pgtype.Text{String: "OPEN_FAILED", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOR0000000014", Valid: true},
	}
	binding := db.BaofuAccountBinding{
		ID:          6104,
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     14,
		OpenState:   db.BaofuAccountOpenStateProcessing,
		AccountType: db.BaofuAccountTypePersonal,
	}
	store.EXPECT().GetBaofuAccountOpeningFlowByOpenTransSerialNo(gomock.Any(), pgtype.Text{String: "OPEN_FAILED", Valid: true}).Return(flow, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   14,
	}).Return(binding, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, pgtype.Text{String: db.BaofuAccountOwnerTypeRider, Valid: true}, arg.BusinessOwner)
			require.Equal(t, pgtype.Text{String: "baofu_account_opening_flow", Valid: true}, arg.BusinessObjectType)
			require.Equal(t, pgtype.Int8{Int64: 7104, Valid: true}, arg.BusinessObjectID)
			require.Equal(t, db.ExternalPaymentTerminalStatusFailed, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, "0", arg.UpstreamState)
			return db.ExternalPaymentFact{ID: 90, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().MarkBaofuAccountBindingFailed(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountBindingFailedParams) (db.BaofuAccountBinding, error) {
			require.Equal(t, int64(6104), arg.ID)
			require.JSONEq(t, `{"state":"failed","failure_code":"ID_CARD_CHECK_FAILED","provider_diagnostic":{"provider":"baofu","capability":"account","source_path":"body.errorCode","result_state":"0","result_error_code":"ID_CARD_CHECK_FAILED","result_error_message_sanitized":"身份证号码不合法","result_error_message_present":true}}`, string(arg.RawSnapshot))
			require.Contains(t, string(arg.RawSnapshot), "身份证号码不合法")
			return db.BaofuAccountBinding{ID: arg.ID, OpenState: db.BaofuAccountOpenStateFailed}, nil
		})
	store.EXPECT().MarkBaofuAccountOpeningFlowFailed(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowFailedParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, int64(7104), arg.ID)
			require.Equal(t, pgtype.Text{String: "ID_CARD_CHECK_FAILED", Valid: true}, arg.FailureCode)
			require.Equal(t, pgtype.Text{String: "身份证号码不合法", Valid: true}, arg.FailureMessage)
			require.JSONEq(t, `{"state":"failed","failure_code":"ID_CARD_CHECK_FAILED","provider_diagnostic":{"provider":"baofu","capability":"account","source_path":"body.errorCode","result_state":"0","result_error_code":"ID_CARD_CHECK_FAILED","result_error_message_sanitized":"身份证号码不合法","result_error_message_present":true}}`, string(arg.RawSnapshot))
			require.Contains(t, string(arg.RawSnapshot), "身份证号码不合法")
			flow.State = db.BaofuAccountOpeningStateFailed
			flow.FailureCode = arg.FailureCode
			flow.FailureMessage = arg.FailureMessage
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
			require.Equal(t, "CP_****ING", extra["contract_no_mask"])
			require.NotContains(t, string(arg.Extra), "CP_MISSING")
			require.NotContains(t, arg.Message, "CP_MISSING")
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
			require.Equal(t, "CP_****ING", extra["contract_no_mask"])
			require.NotContains(t, string(arg.Extra), "CP_EXISTING")
			require.NotContains(t, arg.Message, "CP_EXISTING")
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

func TestBaofuWithdrawCallbackPersistsFactBeforeEnqueueAndAck(t *testing.T) {
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
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      88,
		Amount:       12345,
	}
	store.EXPECT().GetBaofuWithdrawalOrderByOutRequestNo(gomock.Any(), "WD202605040001").Return(withdrawal, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuWithdraw, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectWithdraw, arg.ExternalObjectType)
			require.Equal(t, "WD202605040001", arg.ExternalObjectKey)
			require.Equal(t, pgtype.Text{String: "BFWD202605040001", Valid: true}, arg.ExternalSecondaryKey)
			require.Equal(t, pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true}, arg.BusinessOwner)
			require.Equal(t, pgtype.Text{String: "baofu_withdrawal_order", Valid: true}, arg.BusinessObjectType)
			require.Equal(t, pgtype.Int8{Int64: withdrawal.ID, Valid: true}, arg.BusinessObjectID)
			require.Equal(t, "3", arg.UpstreamState)
			require.Equal(t, db.ExternalPaymentTerminalStatusFailed, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, pgtype.Int8{Int64: 12345, Valid: true}, arg.Amount)
			require.Equal(t, "baofu:callback:withdraw:WD202605040001:BFWD202605040001", arg.DedupeKey)
			require.JSONEq(t, `{"transSerialNo":"WD202605040001","orderId":"BFWD202605040001","state":"3"}`, string(arg.RawResource))
			return db.ExternalPaymentFact{ID: 9101, DedupeKey: arg.DedupeKey}, nil
		})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/withdraw", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, []int64{withdrawal.ID}, taskRecorder.withdrawalOrderIDs)
	require.Equal(t, []string{"3"}, taskRecorder.upstreamStates)
	require.Equal(t, []string{"BFWD202605040001"}, taskRecorder.baofuWithdrawNos)
}

func TestBaofuWithdrawCallbackDoesNotEnqueueWhenFactPersistenceFails(t *testing.T) {
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
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      88,
		Amount:       12345,
	}
	store.EXPECT().GetBaofuWithdrawalOrderByOutRequestNo(gomock.Any(), "WD202605040001").Return(withdrawal, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentFact{}, errors.New("insert fact failed"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/withdraw", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Contains(t, recorder.Body.String(), "persist callback failed")
	require.Empty(t, taskRecorder.withdrawalOrderIDs)
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

func TestBaofuPaymentCallbackLogsProviderFieldsWhenFallbackQueryFails(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuPaymentTradeNoOnlyParser{})
	queryClient := &fakeBaofuAggregateQueryClient{
		paymentErr: baofu.NewProviderBusinessError("order_query", "ORDER_NOT_EXIST", "raw upstream card 6222020202020202"),
	}
	server.baofuAggregateClient = queryClient
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	store.EXPECT().GetPaymentOrderByTransactionId(gomock.Any(), pgtype.Text{String: "BFPAY_4002", Valid: true}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/payment", bytes.NewBufferString(`{"notifyId":"BFN_4002"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	body := logs.String()
	require.Contains(t, body, `"provider_operation":"order_query"`)
	require.Contains(t, body, `"provider_capability":"baofu"`)
	require.Contains(t, body, `"upstream_code":"ORDER_NOT_EXIST"`)
	require.Contains(t, body, `"upstream_message_sanitized"`)
	require.NotContains(t, body, "6222020202020202")
	require.Contains(t, body, "************0202")
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

func TestBaofuPaymentCallbackDirectRejectsNotificationIdentityMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuAggregateIdentityMismatchParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/payment", bytes.NewBufferString(`{"notifyId":"BFN_4001"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "callback verification failed")
}

func TestBaofuPaymentCallbackDirectRejectsNotificationIdentityMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuAggregateIdentityMissingParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/payment", bytes.NewBufferString(`{"notifyId":"BFN_4001"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "callback verification failed")
}

func TestBaofuShareCallbackDirectRejectsNotificationIdentityMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuAggregateIdentityMissingParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/share", bytes.NewBufferString(`{"notifyId":"BFSN_3001"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "callback verification failed")
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

func TestBaofuShareCallbackLogsProviderFieldsWhenFallbackQueryFails(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuShareTradeNoOnlyParser{})
	queryClient := &fakeBaofuAggregateQueryClient{
		shareErr: baofu.NewProviderBusinessError("share_query", "ORDER_NOT_EXIST", "raw upstream appid wx1234567890abcdef"),
	}
	server.baofuAggregateClient = queryClient
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/share", bytes.NewBufferString(`{"notifyId":"BFSN_3003"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	body := logs.String()
	require.Contains(t, body, `"provider_operation":"share_query"`)
	require.Contains(t, body, `"provider_capability":"baofu"`)
	require.Contains(t, body, `"upstream_code":"ORDER_NOT_EXIST"`)
	require.Contains(t, body, `"upstream_message_sanitized"`)
	require.NotContains(t, body, "wx1234567890abcdef")
	require.Contains(t, body, "appid=<redacted>")
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

func TestBaofuShareCallbackDirectRejectsNotificationIdentityMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuAggregateIdentityMismatchParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/share", bytes.NewBufferString(`{"notifyId":"BFSN_3001"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "callback verification failed")
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

func TestBaofuRefundCallbackUsesResultCodeWhenRefundStateIsAbsent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuRefundResultCodeOnlyParser{})
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	refundOrder := db.RefundOrder{
		ID:             5102,
		PaymentOrderID: 4002,
		OutRefundNo:    "BFRFD_5102",
		RefundAmount:   1200,
		Status:         "processing",
	}
	paymentOrder := db.PaymentOrder{
		ID:             4002,
		OrderID:        pgtype.Int8{Int64: 3102, Valid: true},
		Amount:         1200,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
	}
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), "BFRFD_5102").Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, aggregatecontracts.BusinessResultCodeSuccess, arg.UpstreamState)
			require.Equal(t, "baofu:callback:refund:BFRFD_5102:BFRN_5102", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 902, IsTerminal: true, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             902,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   refundOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 1002, FactID: 902, Consumer: paymentFactConsumerOrderDomain, BusinessObjectType: paymentFactBusinessObjectRefundOrder, BusinessObjectID: refundOrder.ID}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/refund", bytes.NewBufferString(`{"notifyId":"BFRN_5102"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, []int64{1002}, taskRecorder.applicationIDs)
}

func TestBaofuRefundCallbackRoutesReservationConsumerToReservationRefundApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuReservationRefundParser{})
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	refundOrder := db.RefundOrder{
		ID:             5103,
		PaymentOrderID: 4003,
		OutRefundNo:    "BFRFD_5103",
		RefundAmount:   980,
		Status:         "processing",
	}
	paymentOrder := db.PaymentOrder{
		ID:             4003,
		OrderID:        pgtype.Int8{Int64: 3103, Valid: true},
		ReservationID:  pgtype.Int8{Int64: 3203, Valid: true},
		Amount:         980,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   "reservation_addon",
	}
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), "BFRFD_5103").Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, arg.BusinessOwner.String)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuRefund, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			return db.ExternalPaymentFact{ID: 903, IsTerminal: true, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             903,
		Consumer:           paymentFactConsumerReservationDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   refundOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 1003, FactID: 903, Consumer: paymentFactConsumerReservationDomain, BusinessObjectType: paymentFactBusinessObjectRefundOrder, BusinessObjectID: refundOrder.ID}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/refund", bytes.NewBufferString(`{"notifyId":"BFRN_5103"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, []int64{1003}, taskRecorder.applicationIDs)
}

func TestBaofuRefundCallbackDirectRejectsNotificationIdentityMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuAggregateIdentityMismatchParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/refund", bytes.NewBufferString(`{"notifyId":"BFRN_5101"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "callback verification failed")
}

func TestBaofuRefundCallbackDirectRejectsNotificationIdentityMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuAggregateIdentityMissingParser{})
	server.config.BaofuCollectMerchantID = "102004465"
	server.config.BaofuCollectTerminalID = "200005200"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/refund", bytes.NewBufferString(`{"notifyId":"BFRN_5101"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "callback verification failed")
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

type fakeBaofuFailedAccountParser struct{}

func (fakeBaofuFailedAccountParser) ParseOpenAccountNotification(body []byte) (*baofunotification.AccountNotification, error) {
	return &baofunotification.AccountNotification{
		MemberID:      "102004465",
		TerminalID:    "200005200",
		OutRequestNo:  "OPEN_FAILED",
		UpstreamState: "0",
		OpenState:     db.BaofuAccountOpenStateFailed,
		FailCode:      "ID_CARD_CHECK_FAILED",
		FailMessage:   "身份证号码不合法",
		OccurredAt:    time.Now().UTC(),
		Raw:           []byte(`{"transSerialNo":"OPEN_FAILED","state":"0","errorCode":"ID_CARD_CHECK_FAILED","errorMsg":"身份证号码不合法"}`),
	}, nil
}

func (fakeBaofuFailedAccountParser) ParseWithdrawNotification(body []byte) (*baofunotification.WithdrawNotification, error) {
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
			MerchantID:       "102004465",
			TerminalID:       "200005200",
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
			MerchantID:       "102004465",
			TerminalID:       "200005200",
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
			MerchantID:       "102004465",
			TerminalID:       "200005200",
			OutTradeNo:       "BFRFD_5101",
			TradeNo:          "BFREFUND_UP_5101",
			TransactionState: aggregatecontracts.RefundStateSuccess,
			SuccessAmountFen: 1200,
			Raw:              []byte(`{"notifyId":"BFRN_5101","outTradeNo":"BFRFD_5101","tradeNo":"BFREFUND_UP_5101","refundState":"SUCCESS"}`),
		},
	}, nil
}

type fakeBaofuRefundResultCodeOnlyParser struct {
	fakeBaofuPaymentParser
}

func (fakeBaofuRefundResultCodeOnlyParser) ParseRefundNotification(body []byte) (*baofuaggregatenotification.RefundNotification, error) {
	return &baofuaggregatenotification.RefundNotification{
		NotifyID:       "BFRN_5102",
		NotifyType:     "REFUND",
		TerminalStatus: db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:     true,
		OccurredAt:     time.Now().UTC(),
		Raw:            []byte(`{"notifyId":"BFRN_5102","outTradeNo":"BFRFD_5102","tradeNo":"BFREFUND_UP_5102","resultCode":"SUCCESS"}`),
		Fact: aggregatecontracts.RefundFact{
			MerchantID:       "102004465",
			TerminalID:       "200005200",
			OutTradeNo:       "BFRFD_5102",
			TradeNo:          "BFREFUND_UP_5102",
			SuccessAmountFen: 1200,
			ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
			Raw:              []byte(`{"notifyId":"BFRN_5102","outTradeNo":"BFRFD_5102","tradeNo":"BFREFUND_UP_5102","resultCode":"SUCCESS"}`),
		},
	}, nil
}

type fakeBaofuReservationRefundParser struct {
	fakeBaofuPaymentParser
}

func (fakeBaofuReservationRefundParser) ParseRefundNotification(body []byte) (*baofuaggregatenotification.RefundNotification, error) {
	return &baofuaggregatenotification.RefundNotification{
		NotifyID:       "BFRN_5103",
		NotifyType:     "REFUND",
		TerminalStatus: db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:     true,
		OccurredAt:     time.Now().UTC(),
		Raw:            []byte(`{"notifyId":"BFRN_5103","outTradeNo":"BFRFD_5103","tradeNo":"BFREFUND_UP_5103","refundState":"SUCCESS"}`),
		Fact: aggregatecontracts.RefundFact{
			MerchantID:       "102004465",
			TerminalID:       "200005200",
			OutTradeNo:       "BFRFD_5103",
			TradeNo:          "BFREFUND_UP_5103",
			TransactionState: aggregatecontracts.RefundStateSuccess,
			SuccessAmountFen: 980,
			Raw:              []byte(`{"notifyId":"BFRN_5103","outTradeNo":"BFRFD_5103","tradeNo":"BFREFUND_UP_5103","refundState":"SUCCESS"}`),
		},
	}, nil
}

type fakeBaofuAggregateIdentityMismatchParser struct {
	fakeBaofuPaymentParser
}

type fakeBaofuAggregateIdentityMissingParser struct {
	fakeBaofuPaymentParser
}

func (fakeBaofuAggregateIdentityMissingParser) ParsePaymentNotification(body []byte) (*baofuaggregatenotification.PaymentNotification, error) {
	notification, err := fakeBaofuPaymentParser{}.ParsePaymentNotification(body)
	if notification != nil {
		notification.Fact.MerchantID = ""
		notification.Fact.TerminalID = ""
	}
	return notification, err
}

func (fakeBaofuAggregateIdentityMissingParser) ParseShareNotification(body []byte) (*baofuaggregatenotification.ShareNotification, error) {
	notification, err := fakeBaofuPaymentParser{}.ParseShareNotification(body)
	if notification != nil {
		notification.Fact.MerchantID = ""
		notification.Fact.TerminalID = ""
	}
	return notification, err
}

func (fakeBaofuAggregateIdentityMissingParser) ParseRefundNotification(body []byte) (*baofuaggregatenotification.RefundNotification, error) {
	notification, err := fakeBaofuPaymentParser{}.ParseRefundNotification(body)
	if notification != nil {
		notification.Fact.MerchantID = ""
		notification.Fact.TerminalID = ""
	}
	return notification, err
}

func (fakeBaofuAggregateIdentityMismatchParser) ParsePaymentNotification(body []byte) (*baofuaggregatenotification.PaymentNotification, error) {
	notification, err := fakeBaofuPaymentParser{}.ParsePaymentNotification(body)
	if notification != nil {
		notification.Fact.MerchantID = "102004999"
		notification.Fact.TerminalID = "200005200"
	}
	return notification, err
}

func (fakeBaofuAggregateIdentityMismatchParser) ParseShareNotification(body []byte) (*baofuaggregatenotification.ShareNotification, error) {
	notification, err := fakeBaofuPaymentParser{}.ParseShareNotification(body)
	if notification != nil {
		notification.Fact.MerchantID = "102004465"
		notification.Fact.TerminalID = "200005299"
	}
	return notification, err
}

func (fakeBaofuAggregateIdentityMismatchParser) ParseRefundNotification(body []byte) (*baofuaggregatenotification.RefundNotification, error) {
	notification, err := fakeBaofuPaymentParser{}.ParseRefundNotification(body)
	if notification != nil {
		notification.Fact.MerchantID = "102004999"
		notification.Fact.TerminalID = "200005200"
	}
	return notification, err
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
			MerchantID:       "102004465",
			TerminalID:       "200005200",
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
			MerchantID:       "102004465",
			TerminalID:       "200005200",
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
	paymentErr       error
	shareErr         error
	lastPaymentQuery aggregatecontracts.PaymentQueryRequest
	lastShareQuery   aggregatecontracts.ShareQueryRequest
}

func (c *fakeBaofuAggregateQueryClient) CreateUnifiedOrder(context.Context, aggregatecontracts.UnifiedOrderRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, errors.New("not implemented in baofu callback test")
}

func (c *fakeBaofuAggregateQueryClient) QueryPayment(_ context.Context, req aggregatecontracts.PaymentQueryRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	c.lastPaymentQuery = req
	if c.paymentErr != nil {
		return nil, c.paymentErr
	}
	return &aggregatecontracts.UnifiedOrderResult{OutTradeNo: c.outTradeNo, TradeNo: req.TradeNo}, nil
}

func (c *fakeBaofuAggregateQueryClient) CreateProfitSharing(context.Context, aggregatecontracts.ShareAfterPayRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, errors.New("not implemented in baofu callback test")
}

func (c *fakeBaofuAggregateQueryClient) QueryProfitSharing(_ context.Context, req aggregatecontracts.ShareQueryRequest) (*aggregatecontracts.ShareResult, error) {
	c.lastShareQuery = req
	if c.shareErr != nil {
		return nil, c.shareErr
	}
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

type baofuFailingDoer struct{}

func (baofuFailingDoer) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("baofu merchant report transport unavailable")
}

func testAPIBaofuRootClient(t *testing.T, doer baofu.HTTPDoer) *baofu.Client {
	t.Helper()
	privatePEM, publicPEM := generateAPIBaofuTestKeyPair(t)
	if recorder, ok := doer.(*baofuMerchantReportAPIDoer); ok {
		recorder.baofuPrivatePEM = privatePEM
	}
	client, err := baofu.NewClient(baofu.Config{
		Environment:        baofu.BaofuEnvironmentSandbox,
		CollectMerchantID:  "102004465",
		CollectTerminalID:  "200005200",
		PayoutMerchantID:   "102004466",
		PayoutTerminalID:   "200005201",
		AppID:              "wx1234567890abcdef",
		PrivateKeyPEM:      privatePEM,
		BaofuPublicKeyPEM:  publicPEM,
		NotifyBaseURL:      "https://api.example.com/v1/webhooks/baofu",
		SignSerialNo:       "1",
		EncryptionSerialNo: "1",
		Timeout:            5 * time.Second,
	}, doer)
	require.NoError(t, err)
	return client
}

func generateAPIBaofuTestKeyPair(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})), string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
}

var _ = gin.TestMode
