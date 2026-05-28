package logic

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	merchantcontracts "github.com/merrydance/locallife/baofu/merchantreport/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestBaofuMerchantReportServiceRequiresMerchantSharingMerID(t *testing.T) {
	store := &fakeBaofuMerchantReportStore{
		binding: db.BaofuAccountBinding{OwnerType: db.BaofuAccountOwnerTypeMerchant, OwnerID: 123, OpenState: db.BaofuAccountOpenStateActive},
	}
	service := NewBaofuMerchantReportService(store, &fakeMerchantReportClient{}, BaofuMerchantReportConfig{MiniProgramAppID: "wx1234567890abcdef"})

	_, err := service.SubmitWechatMerchantReport(context.Background(), SubmitBaofuMerchantReportInput{MerchantID: 123})

	require.ErrorIs(t, err, ErrBaofuMerchantReportReceiverRequired)
}

func TestBaofuPaymentReadinessRequiresMerchantSubMchIDAndAppletAuth(t *testing.T) {
	binding := activeBaofuBindingWithSharingMerID(123, "CM202605040001")
	report := succeededMerchantReportWithoutAppletAuth(123, "1900000109")

	readiness := ReadinessFromBaofuBindingAndMerchantReport(binding, report)

	require.False(t, readiness.PaymentReady)
	require.Equal(t, BaofuOnboardingStateWechatChannelPending, readiness.State)
	require.Equal(t, "微信支付通道待开通", readiness.Label)
	require.Equal(t, "1900000109", readiness.SubMchID)
}

func TestBaofuPaymentReadinessUsesMerchantReportSubMchIDAfterAppletAuth(t *testing.T) {
	binding := activeBaofuBindingWithSharingMerID(123, "CM202605040001")
	report := succeededMerchantReportWithoutAppletAuth(123, "1900000109")
	report.AppletAuthState = db.BaofuMerchantReportAppletAuthStateSucceeded

	readiness := ReadinessFromBaofuBindingAndMerchantReport(binding, report)

	require.True(t, readiness.PaymentReady)
	require.Equal(t, BaofuOnboardingStateReady, readiness.State)
	require.Equal(t, "1900000109", readiness.SubMchID)
	require.NotEqual(t, binding.SharingMerID.String, readiness.SubMchID)
}

func TestBaofuMerchantReportServiceBindsAppletAfterReportSuccess(t *testing.T) {
	store := &fakeBaofuMerchantReportStore{binding: activeBaofuBindingWithSharingMerID(123, "CM202605040001")}
	client := fakeMerchantReportClient{
		reportResult: &merchantcontracts.MerchantReportResult{ReportNo: "MR202605040001", ReportState: "SUCCESS", SubMchID: "1900000109", PlatformBizNo: "PB202605040001"},
		bindResult:   &merchantcontracts.BindSubConfigResult{SubMchID: "1900000109", AuthType: merchantcontracts.AuthTypeApplet, ResultCode: "SUCCESS"},
	}
	service := NewBaofuMerchantReportService(store, &client, BaofuMerchantReportConfig{CollectMerchantID: "100000", CollectTerminalID: "200000", MiniProgramAppID: "wx1234567890abcdef"})

	result, err := service.SubmitWechatMerchantReport(context.Background(), SubmitBaofuMerchantReportInput{
		MerchantID:          123,
		ReportNo:            "MR202605040001",
		MerchantName:        "上海某某餐饮有限公司",
		MerchantShortName:   "某某餐饮",
		ServicePhone:        "02112345678",
		ChannelID:           "channel-001",
		ChannelName:         "乐客来福",
		Business:            "758-2",
		BusinessLicenseType: merchantcontracts.WechatCertificateTypeNationalLegalMerge,
		BusinessLicense:     "91310000123456789X",
		AddressInfo:         merchantcontracts.WechatAddressInfo{ProvinceCode: "310000", CityCode: "310100", DistrictCode: "310115", Address: "世纪大道 1 号"},
		BankCardInfo:        merchantcontracts.WechatBankCardInfo{CardName: "上海某某餐饮有限公司", CardNo: "6222000000000000000", BankBranchName: "招商银行上海分行"},
	})

	require.NoError(t, err)
	require.Equal(t, "1900000109", result.SubMchID.String)
	require.Equal(t, db.BaofuMerchantReportStateSucceeded, result.ReportState)
	require.Equal(t, db.BaofuMerchantReportAppletAuthStateSucceeded, result.AppletAuthState)
	require.Equal(t, "CM202605040001", client.reportRequest.BCTMerchantID)
	require.Equal(t, "1900000109", client.bindRequest.SubMchID)
	require.Equal(t, "wx1234567890abcdef", client.bindRequest.AuthContent)
	require.Equal(t, db.ExternalPaymentCommandTypeBaofuMerchantReport, store.commands[0].CommandType)
}

func TestBaofuMerchantReportServiceRecoversProcessingReportAndBindsApplet(t *testing.T) {
	store := &fakeBaofuMerchantReportStore{
		report: db.BaofuMerchantReport{
			ID:              78,
			OwnerType:       db.BaofuAccountOwnerTypeMerchant,
			OwnerID:         123,
			ReportType:      db.BaofuMerchantReportTypeWechat,
			ReportNo:        "MR202605040002",
			BctMerID:        "CM202605040002",
			ReportState:     db.BaofuMerchantReportStateProcessing,
			AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
		},
	}
	client := fakeMerchantReportClient{
		reportResult: &merchantcontracts.MerchantReportResult{ReportNo: "MR202605040002", ReportState: "SUCCESS", SubMchID: "1900000110", PlatformBizNo: "PB202605040002"},
		bindResult:   &merchantcontracts.BindSubConfigResult{SubMchID: "1900000110", AuthType: merchantcontracts.AuthTypeApplet, ResultCode: "SUCCESS"},
	}
	service := NewBaofuMerchantReportService(store, &client, BaofuMerchantReportConfig{CollectMerchantID: "100000", CollectTerminalID: "200000", MiniProgramAppID: "wx1234567890abcdef"})

	result, err := service.RecoverWechatMerchantReport(context.Background(), store.report)

	require.NoError(t, err)
	require.Equal(t, db.BaofuMerchantReportStateSucceeded, result.ReportState)
	require.Equal(t, db.BaofuMerchantReportAppletAuthStateSucceeded, result.AppletAuthState)
	require.Equal(t, "MR202605040002", client.queryRequest.ReportNo)
	require.Equal(t, db.BaofuMerchantReportTypeWechat, client.queryRequest.ReportType)
	require.Equal(t, "1900000110", client.bindRequest.SubMchID)
	require.Equal(t, "wx1234567890abcdef", client.bindRequest.AuthContent)
}

func TestMapBaofuMerchantReportProviderErrorKeepsRawTextOutOfPublicMessage(t *testing.T) {
	providerErr := &baofu.ProviderError{
		Operation:       "merchant_report",
		Capability:      "baofu_merchant_report",
		UpstreamCode:    "MERCHANT_REPORT_LIMIT",
		UpstreamMessage: "raw upstream report message with license 91330100MA00000001",
	}

	err := mapBaofuMerchantReportError(providerErr, BaofuProviderErrorContext{
		FlowID:             701,
		OwnerType:          db.BaofuAccountOwnerTypeMerchant,
		OwnerID:            88,
		OpenTransSerialNo:  "BFO202605090701",
		CurrentState:       db.BaofuAccountOpeningStateMerchantReportProcessing,
		MerchantReportID:   901,
		MerchantReportNo:   "MR202605090701",
		ProviderOperation:  "merchant_report",
		ProviderCapability: "baofu_merchant_report",
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, reqErr.Status)
	require.Contains(t, reqErr.Err.Error(), "微信支付商户报备失败")
	require.NotContains(t, reqErr.Err.Error(), providerErr.UpstreamMessage)
	require.NotContains(t, reqErr.Err.Error(), providerErr.UpstreamCode)
	var provider *baofu.ProviderError
	require.True(t, errors.As(LoggableError(err), &provider))
	require.Same(t, providerErr, provider)

	ctx, ok := BaofuProviderErrorContextFromError(err)
	require.True(t, ok)
	require.Equal(t, int64(701), ctx.FlowID)
	require.Equal(t, db.BaofuAccountOwnerTypeMerchant, ctx.OwnerType)
	require.Equal(t, int64(88), ctx.OwnerID)
	require.Equal(t, int64(901), ctx.MerchantReportID)
	require.Equal(t, "MR202605090701", ctx.MerchantReportNo)
	require.Equal(t, "merchant_report", ctx.ProviderOperation)
}

func TestMapBaofuMerchantReportAppletAuthProviderErrorReturnsSpecificGuidance(t *testing.T) {
	providerErr := &baofu.ProviderError{
		Operation:       "bind_sub_config",
		Capability:      "baofu_merchant_report",
		UpstreamCode:    "NO_AUTH",
		UpstreamMessage: "raw upstream auth directory message wx123456",
	}

	err := mapBaofuMerchantReportError(providerErr, BaofuProviderErrorContext{
		FlowID:             702,
		OwnerType:          db.BaofuAccountOwnerTypeMerchant,
		OwnerID:            88,
		CurrentState:       db.BaofuAccountOpeningStateAppletAuthPending,
		MerchantReportID:   902,
		MerchantReportNo:   "MR202605090702",
		ProviderOperation:  "bind_sub_config",
		ProviderCapability: "baofu_merchant_report",
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, reqErr.Status)
	require.EqualError(t, reqErr.Err, "微信支付授权目录绑定失败，请联系平台处理后重试")
	require.NotContains(t, reqErr.Err.Error(), providerErr.UpstreamMessage)
	var provider *baofu.ProviderError
	require.True(t, errors.As(LoggableError(err), &provider))
	require.Same(t, providerErr, provider)
}

func TestBaofuMerchantReportServiceReturnsPersistedReportOnProviderFailures(t *testing.T) {
	tests := []struct {
		name       string
		report     db.BaofuMerchantReport
		client     fakeMerchantReportClient
		call       func(*BaofuMerchantReportService, context.Context, SubmitBaofuMerchantReportInput, db.BaofuMerchantReport) (db.BaofuMerchantReport, error)
		wantID     int64
		wantReport string
	}{
		{
			name: "submit",
			client: fakeMerchantReportClient{submitErr: &baofu.ProviderError{
				Operation:       "merchant_report",
				UpstreamCode:    "MERCHANT_REPORT_LIMIT",
				UpstreamMessage: "raw upstream report failure",
			}},
			call: func(service *BaofuMerchantReportService, ctx context.Context, input SubmitBaofuMerchantReportInput, _ db.BaofuMerchantReport) (db.BaofuMerchantReport, error) {
				return service.SubmitWechatMerchantReport(ctx, input)
			},
			wantID:     77,
			wantReport: "MR202605040099",
		},
		{
			name: "query",
			report: db.BaofuMerchantReport{
				ID:              78,
				OwnerType:       db.BaofuAccountOwnerTypeMerchant,
				OwnerID:         123,
				ReportType:      db.BaofuMerchantReportTypeWechat,
				ReportNo:        "MR202605040098",
				BctMerID:        "CM202605040098",
				ReportState:     db.BaofuMerchantReportStateProcessing,
				AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
			},
			client: fakeMerchantReportClient{queryErr: &baofu.ProviderError{
				Operation:       "merchant_report_query",
				UpstreamCode:    "SYSTEM_BUSY",
				UpstreamMessage: "raw upstream query failure",
			}},
			call: func(service *BaofuMerchantReportService, ctx context.Context, _ SubmitBaofuMerchantReportInput, report db.BaofuMerchantReport) (db.BaofuMerchantReport, error) {
				return service.RecoverWechatMerchantReport(ctx, report)
			},
			wantID:     78,
			wantReport: "MR202605040098",
		},
		{
			name: "bind",
			report: db.BaofuMerchantReport{
				ID:              79,
				OwnerType:       db.BaofuAccountOwnerTypeMerchant,
				OwnerID:         123,
				ReportType:      db.BaofuMerchantReportTypeWechat,
				ReportNo:        "MR202605040097",
				BctMerID:        "CM202605040097",
				ReportState:     db.BaofuMerchantReportStateSucceeded,
				AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
				SubMchID:        pgtype.Text{String: "1900000197", Valid: true},
			},
			client: fakeMerchantReportClient{bindErr: &baofu.ProviderError{
				Operation:       "bind_sub_config",
				UpstreamCode:    "NO_AUTH",
				UpstreamMessage: "raw upstream bind failure",
			}},
			call: func(service *BaofuMerchantReportService, ctx context.Context, _ SubmitBaofuMerchantReportInput, report db.BaofuMerchantReport) (db.BaofuMerchantReport, error) {
				return service.RecoverWechatMerchantReport(ctx, report)
			},
			wantID:     79,
			wantReport: "MR202605040097",
		},
	}

	input := SubmitBaofuMerchantReportInput{
		MerchantID:          123,
		ReportNo:            "MR202605040099",
		MerchantName:        "上海某某餐饮有限公司",
		MerchantShortName:   "某某餐饮",
		ServicePhone:        "02112345678",
		ChannelID:           "channel-001",
		ChannelName:         "乐客来福",
		Business:            "758-2",
		BusinessLicenseType: merchantcontracts.WechatCertificateTypeNationalLegalMerge,
		BusinessLicense:     "91310000123456789X",
		AddressInfo:         merchantcontracts.WechatAddressInfo{ProvinceCode: "310000", CityCode: "310100", DistrictCode: "310115", Address: "世纪大道 1 号"},
		BankCardInfo:        merchantcontracts.WechatBankCardInfo{CardName: "上海某某餐饮有限公司", CardNo: "6222000000000000000", BankBranchName: "招商银行上海分行"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeBaofuMerchantReportStore{
				binding: activeBaofuBindingWithSharingMerID(123, "CM202605040099"),
				report:  tt.report,
			}
			client := tt.client
			service := NewBaofuMerchantReportService(store, &client, BaofuMerchantReportConfig{CollectMerchantID: "100000", CollectTerminalID: "200000", MiniProgramAppID: "wx1234567890abcdef"})

			report, err := tt.call(service, context.Background(), input, tt.report)

			require.Error(t, err)
			require.Equal(t, tt.wantID, report.ID)
			require.Equal(t, tt.wantReport, report.ReportNo)
		})
	}
}

func activeBaofuBindingWithSharingMerID(ownerID int64, sharingMerID string) db.BaofuAccountBinding {
	return db.BaofuAccountBinding{
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      ownerID,
		AccountType:  db.BaofuAccountTypeBusiness,
		OpenState:    db.BaofuAccountOpenStateActive,
		SharingMerID: pgtype.Text{String: sharingMerID, Valid: true},
	}
}

func succeededMerchantReportWithoutAppletAuth(ownerID int64, subMchID string) db.BaofuMerchantReport {
	return db.BaofuMerchantReport{
		OwnerType:       db.BaofuAccountOwnerTypeMerchant,
		OwnerID:         ownerID,
		ReportType:      db.BaofuMerchantReportTypeWechat,
		ReportState:     db.BaofuMerchantReportStateSucceeded,
		AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
		SubMchID:        pgtype.Text{String: subMchID, Valid: true},
	}
}

type fakeBaofuMerchantReportStore struct {
	binding  db.BaofuAccountBinding
	report   db.BaofuMerchantReport
	commands []db.CreateExternalPaymentCommandParams
}

func (s *fakeBaofuMerchantReportStore) GetBaofuAccountBindingByOwner(ctx context.Context, arg db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error) {
	if s.binding.OwnerID == 0 {
		return db.BaofuAccountBinding{}, db.ErrRecordNotFound
	}
	return s.binding, nil
}

func (s *fakeBaofuMerchantReportStore) CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
	s.commands = append(s.commands, arg)
	return db.ExternalPaymentCommand{ID: int64(len(s.commands)), CommandType: arg.CommandType}, nil
}

func (s *fakeBaofuMerchantReportStore) GetExternalPaymentCommandByExternalObject(ctx context.Context, arg db.GetExternalPaymentCommandByExternalObjectParams) (db.ExternalPaymentCommand, error) {
	for i := len(s.commands) - 1; i >= 0; i-- {
		command := s.commands[i]
		if command.Provider == arg.Provider &&
			command.Channel == arg.Channel &&
			command.Capability == arg.Capability &&
			command.CommandType == arg.CommandType &&
			command.ExternalObjectType == arg.ExternalObjectType &&
			command.ExternalObjectKey == arg.ExternalObjectKey {
			return db.ExternalPaymentCommand{
				ID:                 int64(i + 1),
				Provider:           command.Provider,
				Channel:            command.Channel,
				Capability:         command.Capability,
				CommandType:        command.CommandType,
				ExternalObjectType: command.ExternalObjectType,
				ExternalObjectKey:  command.ExternalObjectKey,
			}, nil
		}
	}
	return db.ExternalPaymentCommand{}, db.ErrRecordNotFound
}

func (s *fakeBaofuMerchantReportStore) UpsertBaofuMerchantReportProcessing(ctx context.Context, arg db.UpsertBaofuMerchantReportProcessingParams) (db.BaofuMerchantReport, error) {
	s.report = db.BaofuMerchantReport{ID: 77, OwnerType: arg.OwnerType, OwnerID: arg.OwnerID, ReportType: arg.ReportType, ReportNo: arg.ReportNo, BctMerID: arg.BctMerID, ReportState: db.BaofuMerchantReportStateProcessing, AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending, RawSnapshot: arg.RawSnapshot}
	return s.report, nil
}

func (s *fakeBaofuMerchantReportStore) MarkBaofuMerchantReportSucceeded(ctx context.Context, arg db.MarkBaofuMerchantReportSucceededParams) (db.BaofuMerchantReport, error) {
	s.report.ReportState = db.BaofuMerchantReportStateSucceeded
	s.report.SubMchID = arg.SubMchID
	s.report.PlatformBizNo = arg.PlatformBizNo
	s.report.RawSnapshot = arg.RawSnapshot
	return s.report, nil
}

func (s *fakeBaofuMerchantReportStore) MarkBaofuMerchantReportFailed(ctx context.Context, arg db.MarkBaofuMerchantReportFailedParams) (db.BaofuMerchantReport, error) {
	s.report.ReportState = db.BaofuMerchantReportStateFailed
	s.report.FailureCode = arg.FailureCode
	s.report.FailureMessage = arg.FailureMessage
	s.report.RawSnapshot = arg.RawSnapshot
	return s.report, nil
}

func (s *fakeBaofuMerchantReportStore) MarkBaofuMerchantReportAppletAuthSucceeded(ctx context.Context, id int64) (db.BaofuMerchantReport, error) {
	s.report.AppletAuthState = db.BaofuMerchantReportAppletAuthStateSucceeded
	return s.report, nil
}

func (s *fakeBaofuMerchantReportStore) MarkBaofuMerchantReportAppletAuthFailed(ctx context.Context, arg db.MarkBaofuMerchantReportAppletAuthFailedParams) (db.BaofuMerchantReport, error) {
	s.report.AppletAuthState = db.BaofuMerchantReportAppletAuthStateFailed
	s.report.FailureCode = arg.FailureCode
	s.report.FailureMessage = arg.FailureMessage
	return s.report, nil
}

type fakeMerchantReportClient struct {
	reportResult  *merchantcontracts.MerchantReportResult
	bindResult    *merchantcontracts.BindSubConfigResult
	submitErr     error
	queryErr      error
	bindErr       error
	reportRequest merchantcontracts.WechatMerchantReportRequest
	queryRequest  merchantcontracts.MerchantReportQueryRequest
	bindRequest   merchantcontracts.BindSubConfigRequest
}

func (c *fakeMerchantReportClient) SubmitWechatReport(ctx context.Context, req merchantcontracts.WechatMerchantReportRequest) (*merchantcontracts.MerchantReportResult, error) {
	c.reportRequest = req
	if c.submitErr != nil {
		return nil, c.submitErr
	}
	return c.reportResult, nil
}

func (c *fakeMerchantReportClient) QueryReport(ctx context.Context, req merchantcontracts.MerchantReportQueryRequest) (*merchantcontracts.MerchantReportResult, error) {
	c.queryRequest = req
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	return c.reportResult, nil
}

func (c *fakeMerchantReportClient) BindSubConfig(ctx context.Context, req merchantcontracts.BindSubConfigRequest) (*merchantcontracts.BindSubConfigResult, error) {
	c.bindRequest = req
	if c.bindErr != nil {
		return nil, c.bindErr
	}
	return c.bindResult, nil
}
