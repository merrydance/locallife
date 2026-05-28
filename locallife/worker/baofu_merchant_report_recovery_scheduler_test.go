package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	merchantcontracts "github.com/merrydance/locallife/baofu/merchantreport/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBaofuMerchantReportRecoverySchedulerQueriesProcessingReportsAndBindsApplet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &baofuMerchantReportRecoveryClient{
		queryResult: &merchantcontracts.MerchantReportResult{ReportNo: "MR202605040003", ReportState: "SUCCESS", SubMchID: "1900000111", PlatformBizNo: "PB202605040003"},
		bindResult:  &merchantcontracts.BindSubConfigResult{SubMchID: "1900000111", AuthType: merchantcontracts.AuthTypeApplet, ResultCode: "SUCCESS"},
	}
	report := db.BaofuMerchantReport{
		ID:              7901,
		OwnerType:       db.BaofuAccountOwnerTypeMerchant,
		OwnerID:         123,
		ReportType:      db.BaofuMerchantReportTypeWechat,
		ReportNo:        "MR202605040003",
		BctMerID:        "CM202605040003",
		ReportState:     db.BaofuMerchantReportStateProcessing,
		AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
		UpdatedAt:       time.Now().Add(-20 * time.Minute),
	}
	store.EXPECT().ListRecoverableBaofuMerchantReports(gomock.Any(), gomock.Any()).
		Return([]db.BaofuMerchantReport{report}, nil)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentCommand{ID: 1, CommandType: db.ExternalPaymentCommandTypeBaofuMerchantReportQuery}, nil)
	store.EXPECT().MarkBaofuMerchantReportSucceeded(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuMerchantReportSucceededParams) (db.BaofuMerchantReport, error) {
			report.ReportState = db.BaofuMerchantReportStateSucceeded
			report.SubMchID = arg.SubMchID
			report.PlatformBizNo = arg.PlatformBizNo
			return report, nil
		})
	store.EXPECT().GetExternalPaymentCommandByExternalObject(gomock.Any(), db.GetExternalPaymentCommandByExternalObjectParams{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuMerchantReport,
		CommandType:        db.ExternalPaymentCommandTypeBaofuBindSubConfig,
		ExternalObjectType: "baofu_bind_sub_config",
		ExternalObjectKey:  "1900000111",
	}).Return(db.ExternalPaymentCommand{}, db.ErrRecordNotFound)
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentCommand{ID: 2, CommandType: db.ExternalPaymentCommandTypeBaofuBindSubConfig}, nil)
	store.EXPECT().MarkBaofuMerchantReportAppletAuthSucceeded(gomock.Any(), report.ID).
		Return(db.BaofuMerchantReport{ID: report.ID, ReportState: db.BaofuMerchantReportStateSucceeded, AppletAuthState: db.BaofuMerchantReportAppletAuthStateSucceeded, SubMchID: pgtype.Text{String: "1900000111", Valid: true}}, nil)

	scheduler := worker.NewBaofuMerchantReportRecoveryScheduler(store, client, worker.BaofuMerchantReportRecoveryConfig{
		CollectMerchantID: "100000",
		CollectTerminalID: "200000",
		MiniProgramAppID:  "wx1234567890abcdef",
	})
	scheduler.RunOnce()

	require.Equal(t, "100000", client.queryReq.MerchantID)
	require.Equal(t, "200000", client.queryReq.TerminalID)
	require.Equal(t, "MR202605040003", client.queryReq.ReportNo)
	require.Equal(t, "1900000111", client.bindReq.SubMchID)
	require.Equal(t, "wx1234567890abcdef", client.bindReq.AuthContent)
}

type baofuMerchantReportRecoveryClient struct {
	queryReq    merchantcontracts.MerchantReportQueryRequest
	bindReq     merchantcontracts.BindSubConfigRequest
	queryResult *merchantcontracts.MerchantReportResult
	bindResult  *merchantcontracts.BindSubConfigResult
}

func (c *baofuMerchantReportRecoveryClient) SubmitWechatReport(_ context.Context, req merchantcontracts.WechatMerchantReportRequest) (*merchantcontracts.MerchantReportResult, error) {
	return nil, nil
}

func (c *baofuMerchantReportRecoveryClient) QueryReport(_ context.Context, req merchantcontracts.MerchantReportQueryRequest) (*merchantcontracts.MerchantReportResult, error) {
	c.queryReq = req
	return c.queryResult, nil
}

func (c *baofuMerchantReportRecoveryClient) BindSubConfig(_ context.Context, req merchantcontracts.BindSubConfigRequest) (*merchantcontracts.BindSubConfigResult, error) {
	c.bindReq = req
	return c.bindResult, nil
}
