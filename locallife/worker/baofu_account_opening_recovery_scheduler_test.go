package worker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	merchantcontracts "github.com/merrydance/locallife/baofu/merchantreport/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBaofuAccountOpeningRecoverySchedulerQueriesOpeningFlowAndMarksReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuAccountOpeningRecoveryClient{
		result: &baofucontracts.AccountResult{
			ContractNo:    "CP202605080066",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
			Raw:           []byte(`{"state":"1"}`),
		},
	}
	flow := db.BaofuAccountOpeningFlow{
		ID:                66,
		OwnerType:         db.BaofuAccountOwnerTypeRider,
		OwnerID:           1006,
		AccountType:       db.BaofuAccountTypePersonal,
		ProfileID:         pgtype.Int8{Int64: 166, Valid: true},
		State:             db.BaofuAccountOpeningStateOpeningProcessing,
		OpenTransSerialNo: pgtype.Text{String: "BFO1006", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOR0000001006", Valid: true},
		UpdatedAt:         time.Now().Add(-10 * time.Minute),
	}
	binding := db.BaofuAccountBinding{
		ID:          266,
		OwnerType:   flow.OwnerType,
		OwnerID:     flow.OwnerID,
		AccountType: flow.AccountType,
		LoginNo:     flow.LoginNo,
		OpenState:   db.BaofuAccountOpenStateProcessing,
	}
	profile := db.BaofuAccountOpeningProfile{
		ID:                      166,
		OwnerType:               flow.OwnerType,
		OwnerID:                 flow.OwnerID,
		AccountType:             flow.AccountType,
		CertificateType:         pgtype.Text{String: baofucontracts.OfficialCertificateTypeID, Valid: true},
		CertificateNoCiphertext: pgtype.Text{String: "110101199001011234", Valid: true},
	}

	store.EXPECT().ListRecoverableBaofuAccountOpeningFlows(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ListRecoverableBaofuAccountOpeningFlowsParams) ([]db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, int32(100), arg.LimitCount)
			return []db.BaofuAccountOpeningFlow{flow}, nil
		})
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: flow.OwnerType,
		OwnerID:   flow.OwnerID,
	}).Return(binding, nil)
	store.EXPECT().GetBaofuAccountOpeningProfile(gomock.Any(), profile.ID).Return(profile, nil)
	store.EXPECT().GetBaofuAccountBindingByContractNo(gomock.Any(), pgtype.Text{String: "CP202605080066", Valid: true}).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: flow.OwnerType,
		OwnerID:   flow.OwnerID,
	}).Return(binding, nil)
	store.EXPECT().MarkBaofuAccountBindingActive(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountBindingActiveParams) (db.BaofuAccountBinding, error) {
			require.Equal(t, binding.ID, arg.ID)
			require.Equal(t, pgtype.Text{String: "CP202605080066", Valid: true}, arg.ContractNo)
			require.Equal(t, pgtype.Text{String: "CP202605080066", Valid: true}, arg.SharingMerID)
			binding.OpenState = db.BaofuAccountOpenStateActive
			return binding, nil
		})
	store.EXPECT().MarkBaofuAccountOpeningFlowReady(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowReadyParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, flow.ID, arg.ID)
			require.Equal(t, pgtype.Int8{Int64: binding.ID, Valid: true}, arg.AccountBindingID)
			flow.State = db.BaofuAccountOpeningStateReady
			return flow, nil
		})

	scheduler := worker.NewBaofuAccountOpeningRecoveryScheduler(store, client, nil, worker.BaofuAccountOpeningRecoveryConfig{
		VerifyFeeFen: 200,
		IndustryID:   "9931",
	})
	scheduler.RunOnce()

	require.Equal(t, "LLBFOR0000001006", client.queryReq.LoginNo)
	require.Equal(t, "110101199001011234", client.queryReq.CertificateNo)
	require.Empty(t, client.queryReq.PlatformNo)
}

func TestBaofuAccountOpeningRecoverySchedulerSubmitsMerchantReportAndMarksReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	accountClient := &baofuAccountOpeningRecoveryClient{}
	reportClient := &baofuAccountOpeningRecoveryMerchantReportClient{
		reportResult: &merchantcontracts.MerchantReportResult{
			MerchantID:    "100000",
			TerminalID:    "200000",
			ReportType:    db.BaofuMerchantReportTypeWechat,
			ReportNo:      "MR202605080088",
			ReportState:   "SUCCESS",
			ResultCode:    "SUCCESS",
			SubMchID:      "1900000118",
			PlatformBizNo: "PB202605080088",
		},
		bindResult: &merchantcontracts.BindSubConfigResult{
			MerchantID:  "100000",
			TerminalID:  "200000",
			SubMchID:    "1900000118",
			AuthType:    merchantcontracts.AuthTypeApplet,
			ResultCode:  "SUCCESS",
			AuthContent: "wx1234567890abcdef",
		},
	}
	flow := db.BaofuAccountOpeningFlow{
		ID:               88,
		OwnerType:        db.BaofuAccountOwnerTypeMerchant,
		OwnerID:          3008,
		AccountType:      db.BaofuAccountTypeBusiness,
		ProfileID:        pgtype.Int8{Int64: 188, Valid: true},
		State:            db.BaofuAccountOpeningStateMerchantReportProcessing,
		AccountBindingID: pgtype.Int8{Int64: 288, Valid: true},
		UpdatedAt:        time.Now().Add(-10 * time.Minute),
	}
	profile := db.BaofuAccountOpeningProfile{
		ID:                        188,
		OwnerType:                 flow.OwnerType,
		OwnerID:                   flow.OwnerID,
		AccountType:               flow.AccountType,
		ProfileStatus:             db.BaofuAccountOpeningProfileStatusComplete,
		LegalName:                 pgtype.Text{String: "上海某某餐饮有限公司", Valid: true},
		CertificateNoCiphertext:   pgtype.Text{String: "91310000123456789X", Valid: true},
		ContactName:               pgtype.Text{String: "王五", Valid: true},
		ContactMobileCiphertext:   pgtype.Text{String: "13800138000", Valid: true},
		EmailCiphertext:           pgtype.Text{String: "merchant@example.com", Valid: true},
		BankAccountNoCiphertext:   pgtype.Text{String: "6222000000000000000", Valid: true},
		DepositBankName:           pgtype.Text{String: "招商银行上海分行", Valid: true},
		CardUserName:              pgtype.Text{String: "上海某某餐饮有限公司", Valid: true},
		CorporateName:             pgtype.Text{String: "王五", Valid: true},
		CorporateCertIDCiphertext: pgtype.Text{String: "110101199001011234", Valid: true},
	}
	merchant := db.Merchant{
		ID:        flow.OwnerID,
		Name:      "某某餐饮",
		Phone:     "02112345678",
		Address:   "世纪大道1号",
		RegionID:  310115,
		Latitude:  numericForBaofuOpeningTest(31.2304),
		Longitude: numericForBaofuOpeningTest(121.4737),
		ApplicationData: []byte(`{
			"business_license_number":"91310000123456789X",
			"legal_person_name":"王五",
			"legal_person_id_number":"110101199001011234"
		}`),
	}
	district := db.Region{ID: 310115, Code: "310115", Name: "浦东新区", Level: 3, ParentID: pgtype.Int8{Int64: 310100, Valid: true}}
	city := db.Region{ID: 310100, Code: "310100", Name: "上海市", Level: 2, ParentID: pgtype.Int8{Int64: 310000, Valid: true}}
	province := db.Region{ID: 310000, Code: "310000", Name: "上海市", Level: 1}

	store.EXPECT().ListRecoverableBaofuAccountOpeningFlows(gomock.Any(), gomock.Any()).
		Return([]db.BaofuAccountOpeningFlow{flow}, nil)
	store.EXPECT().GetBaofuMerchantReportByOwner(gomock.Any(), db.GetBaofuMerchantReportByOwnerParams{
		OwnerType:  db.BaofuAccountOwnerTypeMerchant,
		OwnerID:    flow.OwnerID,
		ReportType: db.BaofuMerchantReportTypeWechat,
	}).Return(db.BaofuMerchantReport{}, db.ErrRecordNotFound)
	store.EXPECT().GetBaofuAccountOpeningProfile(gomock.Any(), profile.ID).Return(profile, nil)
	store.EXPECT().GetMerchant(gomock.Any(), flow.OwnerID).Return(merchant, nil)
	store.EXPECT().GetRegion(gomock.Any(), merchant.RegionID).Return(district, nil)
	store.EXPECT().GetRegion(gomock.Any(), district.ParentID.Int64).Return(city, nil)
	store.EXPECT().GetRegion(gomock.Any(), city.ParentID.Int64).Return(province, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   flow.OwnerID,
	}).Return(db.BaofuAccountBinding{
		ID:           flow.AccountBindingID.Int64,
		OwnerType:    flow.OwnerType,
		OwnerID:      flow.OwnerID,
		AccountType:  flow.AccountType,
		OpenState:    db.BaofuAccountOpenStateActive,
		SharingMerID: pgtype.Text{String: "CM202605080088", Valid: true},
	}, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentCommand{ID: 1, CommandType: db.ExternalPaymentCommandTypeBaofuMerchantReport}, nil)
	store.EXPECT().UpsertBaofuMerchantReportProcessing(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpsertBaofuMerchantReportProcessingParams) (db.BaofuMerchantReport, error) {
			require.Equal(t, flow.OwnerID, arg.OwnerID)
			require.Equal(t, "CM202605080088", arg.BctMerID)
			return db.BaofuMerchantReport{ID: 788, OwnerType: arg.OwnerType, OwnerID: arg.OwnerID, ReportType: arg.ReportType, ReportNo: arg.ReportNo, BctMerID: arg.BctMerID, ReportState: db.BaofuMerchantReportStateProcessing, AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending}, nil
		})
	store.EXPECT().MarkBaofuMerchantReportSucceeded(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuMerchantReportSucceededParams) (db.BaofuMerchantReport, error) {
			return db.BaofuMerchantReport{ID: 788, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, ReportType: db.BaofuMerchantReportTypeWechat, ReportNo: "MR202605080088", ReportState: db.BaofuMerchantReportStateSucceeded, AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending, SubMchID: arg.SubMchID}, nil
		})
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentCommand{ID: 2, CommandType: db.ExternalPaymentCommandTypeBaofuBindSubConfig}, nil)
	store.EXPECT().MarkBaofuMerchantReportAppletAuthSucceeded(gomock.Any(), int64(788)).
		Return(db.BaofuMerchantReport{ID: 788, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, ReportType: db.BaofuMerchantReportTypeWechat, ReportState: db.BaofuMerchantReportStateSucceeded, AppletAuthState: db.BaofuMerchantReportAppletAuthStateSucceeded, SubMchID: pgtype.Text{String: "1900000118", Valid: true}}, nil)
	store.EXPECT().MarkBaofuAccountOpeningFlowReady(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowReadyParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, flow.ID, arg.ID)
			require.Equal(t, pgtype.Int8{Int64: 788, Valid: true}, arg.MerchantReportID)
			flow.State = db.BaofuAccountOpeningStateReady
			flow.MerchantReportID = arg.MerchantReportID
			return flow, nil
		})

	scheduler := worker.NewBaofuAccountOpeningRecoveryScheduler(store, accountClient, nil, worker.BaofuAccountOpeningRecoveryConfig{
		VerifyFeeFen: 200,
		IndustryID:   "9931",
	})
	scheduler.SetMerchantReportClient(reportClient, worker.BaofuAccountOpeningMerchantReportRecoveryConfig{
		CollectMerchantID: "100000",
		CollectTerminalID: "200000",
		ChannelID:         "148717784",
		ChannelName:       "宝财通收单商户有限公司",
		Business:          "758-2",
		MiniProgramAppID:  "wx1234567890abcdef",
	})
	scheduler.RunOnce()

	require.Equal(t, "100000", reportClient.reportReq.MerchantID)
	require.Equal(t, "148717784", reportClient.reportReq.ReportInfo.ChannelID)
	require.Equal(t, "758-2", reportClient.reportReq.ReportInfo.Business)
	require.Equal(t, "91310000123456789X", reportClient.reportReq.ReportInfo.BusinessLicense)
	require.Equal(t, "310000", reportClient.reportReq.ReportInfo.AddressInfo.ProvinceCode)
	require.Equal(t, "310100", reportClient.reportReq.ReportInfo.AddressInfo.CityCode)
	require.Equal(t, "310115", reportClient.reportReq.ReportInfo.AddressInfo.DistrictCode)
	require.Equal(t, "6222000000000000000", reportClient.reportReq.ReportInfo.BankCardInfo.CardNo)
	require.Equal(t, "1900000118", reportClient.bindReq.SubMchID)
}

func TestBaofuAccountOpeningRecoverySchedulerMarksOpeningFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuAccountOpeningRecoveryClient{
		result: &baofucontracts.AccountResult{
			OpenState:     db.BaofuAccountOpenStateFailed,
			UpstreamState: "0",
			FailCode:      "VERIFY_FAILED",
			FailMessage:   "银行卡校验失败",
			Raw:           []byte(`{"state":"0","errorCode":"VERIFY_FAILED"}`),
		},
	}
	flow, binding, profile := baofuOpeningRecoveryPersonalFixtures(67, 1007)

	store.EXPECT().ListRecoverableBaofuAccountOpeningFlows(gomock.Any(), gomock.Any()).
		Return([]db.BaofuAccountOpeningFlow{flow}, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{OwnerType: flow.OwnerType, OwnerID: flow.OwnerID}).
		Return(binding, nil)
	store.EXPECT().GetBaofuAccountOpeningProfile(gomock.Any(), profile.ID).Return(profile, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{OwnerType: flow.OwnerType, OwnerID: flow.OwnerID}).
		Return(binding, nil)
	store.EXPECT().MarkBaofuAccountBindingFailed(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountBindingFailedParams) (db.BaofuAccountBinding, error) {
			require.Equal(t, binding.ID, arg.ID)
			binding.OpenState = db.BaofuAccountOpenStateFailed
			return binding, nil
		})
	store.EXPECT().MarkBaofuAccountOpeningFlowFailed(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowFailedParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, flow.ID, arg.ID)
			require.Equal(t, pgtype.Text{String: "VERIFY_FAILED", Valid: true}, arg.FailureCode)
			require.Equal(t, pgtype.Text{String: "银行卡校验失败", Valid: true}, arg.FailureMessage)
			flow.State = db.BaofuAccountOpeningStateFailed
			return flow, nil
		})

	scheduler := worker.NewBaofuAccountOpeningRecoveryScheduler(store, client, nil, worker.BaofuAccountOpeningRecoveryConfig{VerifyFeeFen: 200, IndustryID: "9931"})
	scheduler.RunOnce()

	require.Equal(t, "LLBFOR0000001007", client.queryReq.LoginNo)
}

func TestBaofuAccountOpeningRecoverySchedulerLeavesProviderProcessingFlowUnchanged(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuAccountOpeningRecoveryClient{
		result: &baofucontracts.AccountResult{
			OpenState:     db.BaofuAccountOpenStateProcessing,
			UpstreamState: "2",
			Raw:           []byte(`{"state":"2"}`),
		},
	}
	flow, binding, profile := baofuOpeningRecoveryPersonalFixtures(68, 1008)

	store.EXPECT().ListRecoverableBaofuAccountOpeningFlows(gomock.Any(), gomock.Any()).
		Return([]db.BaofuAccountOpeningFlow{flow}, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{OwnerType: flow.OwnerType, OwnerID: flow.OwnerID}).
		Return(binding, nil)
	store.EXPECT().GetBaofuAccountOpeningProfile(gomock.Any(), profile.ID).Return(profile, nil)

	scheduler := worker.NewBaofuAccountOpeningRecoveryScheduler(store, client, nil, worker.BaofuAccountOpeningRecoveryConfig{VerifyFeeFen: 200, IndustryID: "9931"})
	scheduler.RunOnce()

	require.Equal(t, "LLBFOR0000001008", client.queryReq.LoginNo)
}

func TestBaofuAccountOpeningRecoverySchedulerLogsMissingDependenciesAndReportClientNoop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	worker.NewBaofuAccountOpeningRecoveryScheduler(nil, nil, nil, worker.BaofuAccountOpeningRecoveryConfig{}).RunOnce()
	require.Contains(t, logs.String(), `"missing_store":true`)
	require.Contains(t, logs.String(), `"missing_account_client":true`)
	logs.Reset()

	store := mockdb.NewMockStore(ctrl)
	flow := db.BaofuAccountOpeningFlow{
		ID:                89,
		OwnerType:         db.BaofuAccountOwnerTypeMerchant,
		OwnerID:           3009,
		AccountType:       db.BaofuAccountTypeBusiness,
		State:             db.BaofuAccountOpeningStateMerchantReportProcessing,
		OpenTransSerialNo: pgtype.Text{String: "BFO3009", Valid: true},
		UpdatedAt:         time.Now().Add(-10 * time.Minute),
	}
	store.EXPECT().ListRecoverableBaofuAccountOpeningFlows(gomock.Any(), gomock.Any()).
		Return([]db.BaofuAccountOpeningFlow{flow}, nil)

	scheduler := worker.NewBaofuAccountOpeningRecoveryScheduler(store, &baofuAccountOpeningRecoveryClient{}, nil, worker.BaofuAccountOpeningRecoveryConfig{VerifyFeeFen: 200, IndustryID: "9931"})
	scheduler.RunOnce()

	require.Contains(t, logs.String(), `"flow_id":89`)
	require.Contains(t, logs.String(), `"provider_operation":"baofu_merchant_report_recover"`)
	require.Contains(t, logs.String(), `"current_state":"merchant_report_processing"`)
	require.Contains(t, logs.String(), `"missing_merchant_report_client":true`)
}

func TestBaofuAccountOpeningRecoverySchedulerLogsIncompleteMerchantReportConfigNoop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	store := mockdb.NewMockStore(ctrl)
	reportClient := &baofuAccountOpeningRecoveryMerchantReportClient{}
	flow := db.BaofuAccountOpeningFlow{
		ID:                90,
		OwnerType:         db.BaofuAccountOwnerTypeMerchant,
		OwnerID:           3010,
		AccountType:       db.BaofuAccountTypeBusiness,
		State:             db.BaofuAccountOpeningStateAppletAuthPending,
		OpenTransSerialNo: pgtype.Text{String: "BFO3010", Valid: true},
		UpdatedAt:         time.Now().Add(-10 * time.Minute),
	}
	store.EXPECT().ListRecoverableBaofuAccountOpeningFlows(gomock.Any(), gomock.Any()).
		Return([]db.BaofuAccountOpeningFlow{flow}, nil)

	scheduler := worker.NewBaofuAccountOpeningRecoveryScheduler(store, &baofuAccountOpeningRecoveryClient{}, nil, worker.BaofuAccountOpeningRecoveryConfig{VerifyFeeFen: 200, IndustryID: "9931"})
	scheduler.SetMerchantReportClient(reportClient, worker.BaofuAccountOpeningMerchantReportRecoveryConfig{
		CollectMerchantID: "100000",
		CollectTerminalID: "200000",
		ChannelID:         "148717784",
		ChannelName:       "",
		MiniProgramAppID:  "wx1234567890abcdef",
	})
	scheduler.RunOnce()

	body := logs.String()
	require.Contains(t, body, `"flow_id":90`)
	require.Contains(t, body, `"owner_type":"merchant"`)
	require.Contains(t, body, `"owner_id":3010`)
	require.Contains(t, body, `"open_trans_serial_no":"BFO3010"`)
	require.Contains(t, body, `"current_state":"applet_auth_pending"`)
	require.Contains(t, body, `"provider_operation":"baofu_merchant_report_recover"`)
	require.Contains(t, body, `"missing_config_fields"`)
	require.Contains(t, body, `"channel_name"`)
}

func TestBaofuAccountOpeningRecoverySchedulerLogsFlowContextOnFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	store := mockdb.NewMockStore(ctrl)
	client := &baofuAccountOpeningRecoveryClient{err: context.DeadlineExceeded}
	flow, binding, _ := baofuOpeningRecoveryPersonalFixtures(69, 1009)

	store.EXPECT().ListRecoverableBaofuAccountOpeningFlows(gomock.Any(), gomock.Any()).
		Return([]db.BaofuAccountOpeningFlow{flow}, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{OwnerType: flow.OwnerType, OwnerID: flow.OwnerID}).
		Return(binding, nil)
	store.EXPECT().GetBaofuAccountOpeningProfile(gomock.Any(), flow.ProfileID.Int64).
		Return(db.BaofuAccountOpeningProfile{}, context.DeadlineExceeded)

	scheduler := worker.NewBaofuAccountOpeningRecoveryScheduler(store, client, nil, worker.BaofuAccountOpeningRecoveryConfig{VerifyFeeFen: 200, IndustryID: "9931"})
	scheduler.RunOnce()

	body := logs.String()
	require.Contains(t, body, `"flow_id":69`)
	require.Contains(t, body, `"owner_type":"rider"`)
	require.Contains(t, body, `"owner_id":1009`)
	require.Contains(t, body, `"open_trans_serial_no":"BFO1009"`)
	require.Contains(t, body, `"current_state":"opening_processing"`)
	require.Contains(t, body, `"provider_operation":"baofu_account_query"`)
}

func TestBaofuAccountOpeningRecoverySchedulerLogsSafeProviderErrorDetails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	store := mockdb.NewMockStore(ctrl)
	client := &baofuAccountOpeningRecoveryClient{
		err: baofu.NewProviderContractError("T-1001-013-03", errors.New("baofu query account response contractNo is required")),
	}
	flow, binding, profile := baofuOpeningRecoveryPersonalFixtures(70, 1010)

	store.EXPECT().ListRecoverableBaofuAccountOpeningFlows(gomock.Any(), gomock.Any()).
		Return([]db.BaofuAccountOpeningFlow{flow}, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{OwnerType: flow.OwnerType, OwnerID: flow.OwnerID}).
		Return(binding, nil)
	store.EXPECT().GetBaofuAccountOpeningProfile(gomock.Any(), flow.ProfileID.Int64).
		Return(profile, nil)

	scheduler := worker.NewBaofuAccountOpeningRecoveryScheduler(store, client, nil, worker.BaofuAccountOpeningRecoveryConfig{VerifyFeeFen: 200, IndustryID: "9931"})
	scheduler.RunOnce()

	body := logs.String()
	require.Contains(t, body, `"flow_id":70`)
	require.Contains(t, body, `"provider_operation":"baofu_account_query"`)
	require.Contains(t, body, `"provider_method":"T-1001-013-03"`)
	require.Contains(t, body, `"upstream_code":"INVALID_DATA_CONTENT"`)
	require.Contains(t, body, `"http_status":200`)
	require.Contains(t, body, `"provider_error_cause":"baofu query account response contractNo is required"`)
}

type baofuAccountOpeningRecoveryClient struct {
	queryReq baofucontracts.QueryAccountRequest
	result   *baofucontracts.AccountResult
	err      error
}

func (c *baofuAccountOpeningRecoveryClient) OpenAccount(context.Context, baofucontracts.OpenAccountRequest) (*baofucontracts.AccountResult, error) {
	return nil, nil
}

func (c *baofuAccountOpeningRecoveryClient) QueryAccount(_ context.Context, req baofucontracts.QueryAccountRequest) (*baofucontracts.AccountResult, error) {
	c.queryReq = req
	if c.err != nil {
		return nil, c.err
	}
	return c.result, nil
}

type baofuAccountOpeningRecoveryMerchantReportClient struct {
	reportReq    merchantcontracts.WechatMerchantReportRequest
	queryReq     merchantcontracts.MerchantReportQueryRequest
	bindReq      merchantcontracts.BindSubConfigRequest
	reportResult *merchantcontracts.MerchantReportResult
	queryResult  *merchantcontracts.MerchantReportResult
	bindResult   *merchantcontracts.BindSubConfigResult
}

func (c *baofuAccountOpeningRecoveryMerchantReportClient) SubmitWechatReport(_ context.Context, req merchantcontracts.WechatMerchantReportRequest) (*merchantcontracts.MerchantReportResult, error) {
	c.reportReq = req
	return c.reportResult, nil
}

func (c *baofuAccountOpeningRecoveryMerchantReportClient) QueryReport(_ context.Context, req merchantcontracts.MerchantReportQueryRequest) (*merchantcontracts.MerchantReportResult, error) {
	c.queryReq = req
	return c.queryResult, nil
}

func (c *baofuAccountOpeningRecoveryMerchantReportClient) BindSubConfig(_ context.Context, req merchantcontracts.BindSubConfigRequest) (*merchantcontracts.BindSubConfigResult, error) {
	c.bindReq = req
	return c.bindResult, nil
}

func numericForBaofuOpeningTest(v float64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(int64(v * 1000000)), Exp: -6, Valid: true}
}

func baofuOpeningRecoveryPersonalFixtures(flowID, ownerID int64) (db.BaofuAccountOpeningFlow, db.BaofuAccountBinding, db.BaofuAccountOpeningProfile) {
	flow := db.BaofuAccountOpeningFlow{
		ID:                flowID,
		OwnerType:         db.BaofuAccountOwnerTypeRider,
		OwnerID:           ownerID,
		AccountType:       db.BaofuAccountTypePersonal,
		ProfileID:         pgtype.Int8{Int64: flowID + 100, Valid: true},
		State:             db.BaofuAccountOpeningStateOpeningProcessing,
		OpenTransSerialNo: pgtype.Text{String: "BFO" + big.NewInt(ownerID).String(), Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOR" + leftPadBaofuRecoveryID(ownerID), Valid: true},
		UpdatedAt:         time.Now().Add(-10 * time.Minute),
	}
	binding := db.BaofuAccountBinding{
		ID:          flowID + 200,
		OwnerType:   flow.OwnerType,
		OwnerID:     flow.OwnerID,
		AccountType: flow.AccountType,
		LoginNo:     flow.LoginNo,
		OpenState:   db.BaofuAccountOpenStateProcessing,
	}
	profile := db.BaofuAccountOpeningProfile{
		ID:                      flow.ProfileID.Int64,
		OwnerType:               flow.OwnerType,
		OwnerID:                 flow.OwnerID,
		AccountType:             flow.AccountType,
		CertificateType:         pgtype.Text{String: baofucontracts.OfficialCertificateTypeID, Valid: true},
		CertificateNoCiphertext: pgtype.Text{String: "110101199001011234", Valid: true},
	}
	return flow, binding, profile
}

func leftPadBaofuRecoveryID(id int64) string {
	raw := big.NewInt(id).String()
	for len(raw) < 10 {
		raw = "0" + raw
	}
	return raw
}

var _ = json.Valid
