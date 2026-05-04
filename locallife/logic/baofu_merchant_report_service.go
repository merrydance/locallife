package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	merchantcontracts "github.com/merrydance/locallife/baofu/merchantreport/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
)

var (
	ErrBaofuMerchantReportServiceNotConfigured = errors.New("baofu merchant report service is not configured")
	ErrBaofuMerchantReportReceiverRequired     = errors.New("baofu merchant report bct merchant id is required")
	ErrBaofuMerchantReportSubMchIDRequired     = errors.New("baofu merchant report sub mch id is required")
)

type baofuMerchantReportStore interface {
	GetBaofuAccountBindingByOwner(ctx context.Context, arg db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error)
	CreateExternalPaymentCommand(ctx context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error)
	UpsertBaofuMerchantReportProcessing(ctx context.Context, arg db.UpsertBaofuMerchantReportProcessingParams) (db.BaofuMerchantReport, error)
	MarkBaofuMerchantReportSucceeded(ctx context.Context, arg db.MarkBaofuMerchantReportSucceededParams) (db.BaofuMerchantReport, error)
	MarkBaofuMerchantReportFailed(ctx context.Context, arg db.MarkBaofuMerchantReportFailedParams) (db.BaofuMerchantReport, error)
	MarkBaofuMerchantReportAppletAuthSucceeded(ctx context.Context, id int64) (db.BaofuMerchantReport, error)
	MarkBaofuMerchantReportAppletAuthFailed(ctx context.Context, arg db.MarkBaofuMerchantReportAppletAuthFailedParams) (db.BaofuMerchantReport, error)
}

type baofuMerchantReportClient interface {
	SubmitWechatReport(ctx context.Context, req merchantcontracts.WechatMerchantReportRequest) (*merchantcontracts.MerchantReportResult, error)
	QueryReport(ctx context.Context, req merchantcontracts.MerchantReportQueryRequest) (*merchantcontracts.MerchantReportResult, error)
	BindSubConfig(ctx context.Context, req merchantcontracts.BindSubConfigRequest) (*merchantcontracts.BindSubConfigResult, error)
}

type BaofuMerchantReportConfig struct {
	CollectMerchantID string
	CollectTerminalID string
	MiniProgramAppID  string
}

type BaofuMerchantReportService struct {
	store  baofuMerchantReportStore
	client baofuMerchantReportClient
	config BaofuMerchantReportConfig
	now    func() time.Time
}

type SubmitBaofuMerchantReportInput struct {
	MerchantID          int64
	ReportNo            string
	MerchantName        string
	MerchantShortName   string
	ServicePhone        string
	ChannelID           string
	ChannelName         string
	Business            string
	BusinessLicenseType string
	BusinessLicense     string
	AddressInfo         merchantcontracts.WechatAddressInfo
	BankCardInfo        merchantcontracts.WechatBankCardInfo
}

func NewBaofuMerchantReportService(store baofuMerchantReportStore, client baofuMerchantReportClient, config BaofuMerchantReportConfig) *BaofuMerchantReportService {
	return &BaofuMerchantReportService{store: store, client: client, config: config.normalized(), now: time.Now}
}

func (c BaofuMerchantReportConfig) normalized() BaofuMerchantReportConfig {
	c.CollectMerchantID = strings.TrimSpace(c.CollectMerchantID)
	c.CollectTerminalID = strings.TrimSpace(c.CollectTerminalID)
	c.MiniProgramAppID = strings.TrimSpace(c.MiniProgramAppID)
	return c
}

func (s *BaofuMerchantReportService) SubmitWechatMerchantReport(ctx context.Context, input SubmitBaofuMerchantReportInput) (db.BaofuMerchantReport, error) {
	if s == nil || s.store == nil || s.client == nil {
		return db.BaofuMerchantReport{}, ErrBaofuMerchantReportServiceNotConfigured
	}
	cfg := s.config.normalized()
	binding, err := s.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{OwnerType: db.BaofuAccountOwnerTypeMerchant, OwnerID: input.MerchantID})
	if err != nil {
		return db.BaofuMerchantReport{}, err
	}
	if strings.TrimSpace(binding.OpenState) != db.BaofuAccountOpenStateActive || strings.TrimSpace(binding.SharingMerID.String) == "" {
		return db.BaofuMerchantReport{}, ErrBaofuMerchantReportReceiverRequired
	}
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" || cfg.MiniProgramAppID == "" {
		return db.BaofuMerchantReport{}, ErrBaofuMerchantReportServiceNotConfigured
	}
	req := s.buildWechatMerchantReportRequest(input, cfg, strings.TrimSpace(binding.SharingMerID.String))
	if err := req.Validate(); err != nil {
		return db.BaofuMerchantReport{}, err
	}
	if _, err := s.store.CreateExternalPaymentCommand(ctx, db.CreateExternalPaymentCommandParams{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuMerchantReport,
		CommandType:        db.ExternalPaymentCommandTypeBaofuMerchantReport,
		BusinessOwner:      db.ExternalPaymentBusinessOwnerApplyment,
		BusinessObjectType: pgtype.Text{String: "baofu_merchant_report", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: input.MerchantID, Valid: true},
		ExternalObjectType: "baofu_merchant_report",
		ExternalObjectKey:  strings.TrimSpace(req.ReportNo),
		CommandStatus:      db.ExternalPaymentCommandStatusSubmitted,
		SubmittedAt:        s.now().UTC(),
		ResponseSnapshot:   baofuMerchantReportCommandSnapshot(req.ReportNo, req.BCTMerchantID),
	}); err != nil {
		return db.BaofuMerchantReport{}, err
	}
	report, err := s.store.UpsertBaofuMerchantReportProcessing(ctx, db.UpsertBaofuMerchantReportProcessingParams{
		OwnerType:   db.BaofuAccountOwnerTypeMerchant,
		OwnerID:     input.MerchantID,
		ReportType:  db.BaofuMerchantReportTypeWechat,
		ReportNo:    strings.TrimSpace(req.ReportNo),
		BctMerID:    strings.TrimSpace(req.BCTMerchantID),
		RawSnapshot: baofuMerchantReportCommandSnapshot(req.ReportNo, req.BCTMerchantID),
	})
	if err != nil {
		return db.BaofuMerchantReport{}, err
	}
	upstreamResult, err := s.client.SubmitWechatReport(ctx, req)
	if err != nil {
		return db.BaofuMerchantReport{}, err
	}
	if upstreamResult == nil {
		return db.BaofuMerchantReport{}, ErrBaofuMerchantReportServiceNotConfigured
	}
	report, err = s.syncReportResult(ctx, report, upstreamResult)
	if err != nil {
		return db.BaofuMerchantReport{}, err
	}
	if report.ReportState != db.BaofuMerchantReportStateSucceeded {
		return report, nil
	}
	subMchID := strings.TrimSpace(report.SubMchID.String)
	if subMchID == "" {
		return db.BaofuMerchantReport{}, ErrBaofuMerchantReportSubMchIDRequired
	}
	return s.bindApplet(ctx, report, cfg, subMchID)
}

func (s *BaofuMerchantReportService) RecoverWechatMerchantReport(ctx context.Context, report db.BaofuMerchantReport) (db.BaofuMerchantReport, error) {
	if s == nil || s.store == nil || s.client == nil {
		return db.BaofuMerchantReport{}, ErrBaofuMerchantReportServiceNotConfigured
	}
	cfg := s.config.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" || cfg.MiniProgramAppID == "" {
		return db.BaofuMerchantReport{}, ErrBaofuMerchantReportServiceNotConfigured
	}
	if strings.TrimSpace(report.ReportType) != db.BaofuMerchantReportTypeWechat {
		return report, nil
	}
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateProcessing {
		req := merchantcontracts.MerchantReportQueryRequest{
			MerchantID: cfg.CollectMerchantID,
			TerminalID: cfg.CollectTerminalID,
			ReportType: db.BaofuMerchantReportTypeWechat,
			ReportNo:   strings.TrimSpace(report.ReportNo),
		}
		if err := req.Validate(); err != nil {
			return db.BaofuMerchantReport{}, err
		}
		if _, err := s.store.CreateExternalPaymentCommand(ctx, db.CreateExternalPaymentCommandParams{
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			Capability:         db.ExternalPaymentCapabilityBaofuMerchantReport,
			CommandType:        db.ExternalPaymentCommandTypeBaofuMerchantReportQuery,
			BusinessOwner:      db.ExternalPaymentBusinessOwnerApplyment,
			BusinessObjectType: pgtype.Text{String: "baofu_merchant_report", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: report.ID, Valid: true},
			ExternalObjectType: "baofu_merchant_report",
			ExternalObjectKey:  strings.TrimSpace(report.ReportNo),
			CommandStatus:      db.ExternalPaymentCommandStatusSubmitted,
			SubmittedAt:        s.now().UTC(),
			ResponseSnapshot:   baofuMerchantReportCommandSnapshot(report.ReportNo, report.BctMerID),
		}); err != nil {
			return db.BaofuMerchantReport{}, err
		}
		result, err := s.client.QueryReport(ctx, req)
		if err != nil {
			return db.BaofuMerchantReport{}, err
		}
		if result == nil {
			return db.BaofuMerchantReport{}, ErrBaofuMerchantReportServiceNotConfigured
		}
		var errSync error
		report, errSync = s.syncReportResult(ctx, report, result)
		if errSync != nil {
			return db.BaofuMerchantReport{}, errSync
		}
	}
	if strings.TrimSpace(report.ReportState) != db.BaofuMerchantReportStateSucceeded {
		return report, nil
	}
	if strings.TrimSpace(report.AppletAuthState) == db.BaofuMerchantReportAppletAuthStateSucceeded {
		return report, nil
	}
	subMchID := strings.TrimSpace(report.SubMchID.String)
	if subMchID == "" {
		return db.BaofuMerchantReport{}, ErrBaofuMerchantReportSubMchIDRequired
	}
	return s.bindApplet(ctx, report, cfg, subMchID)
}

func (s *BaofuMerchantReportService) buildWechatMerchantReportRequest(input SubmitBaofuMerchantReportInput, cfg BaofuMerchantReportConfig, bctMerID string) merchantcontracts.WechatMerchantReportRequest {
	return merchantcontracts.WechatMerchantReportRequest{
		MerchantID:    cfg.CollectMerchantID,
		TerminalID:    cfg.CollectTerminalID,
		ReportType:    merchantcontracts.ReportTypeWechat,
		ReportNo:      strings.TrimSpace(input.ReportNo),
		BCTMerchantID: strings.TrimSpace(bctMerID),
		ReportInfo: merchantcontracts.WechatReportInfo{
			MerchantName:        strings.TrimSpace(input.MerchantName),
			MerchantShortName:   strings.TrimSpace(input.MerchantShortName),
			ServicePhone:        strings.TrimSpace(input.ServicePhone),
			ChannelID:           strings.TrimSpace(input.ChannelID),
			ChannelName:         strings.TrimSpace(input.ChannelName),
			Business:            strings.TrimSpace(input.Business),
			ServiceCodes:        []string{merchantcontracts.WechatServiceTypeApplet},
			AddressInfo:         input.AddressInfo,
			BusinessLicenseType: strings.TrimSpace(input.BusinessLicenseType),
			BusinessLicense:     strings.TrimSpace(input.BusinessLicense),
			BankCardInfo:        input.BankCardInfo,
		},
	}
}

func (s *BaofuMerchantReportService) syncReportResult(ctx context.Context, report db.BaofuMerchantReport, result *merchantcontracts.MerchantReportResult) (db.BaofuMerchantReport, error) {
	snapshot := baofuMerchantReportResultSnapshot(result)
	switch result.NormalizedReportState() {
	case db.BaofuMerchantReportStateSucceeded:
		return s.store.MarkBaofuMerchantReportSucceeded(ctx, db.MarkBaofuMerchantReportSucceededParams{
			ID:            report.ID,
			SubMchID:      baofuReportText(strings.TrimSpace(result.SubMchID)),
			PlatformBizNo: baofuReportText(strings.TrimSpace(result.PlatformBizNo)),
			RawSnapshot:   snapshot,
		})
	case db.BaofuMerchantReportStateFailed:
		return s.store.MarkBaofuMerchantReportFailed(ctx, db.MarkBaofuMerchantReportFailedParams{
			ID:             report.ID,
			FailureCode:    baofuReportText(strings.TrimSpace(result.ErrorCode)),
			FailureMessage: baofuReportText(strings.TrimSpace(result.ErrorMessage)),
			RawSnapshot:    snapshot,
		})
	default:
		report.RawSnapshot = snapshot
		return report, nil
	}
}

func (s *BaofuMerchantReportService) bindApplet(ctx context.Context, report db.BaofuMerchantReport, cfg BaofuMerchantReportConfig, subMchID string) (db.BaofuMerchantReport, error) {
	bindReq := merchantcontracts.BindSubConfigRequest{
		MerchantID:  cfg.CollectMerchantID,
		TerminalID:  cfg.CollectTerminalID,
		SubMchID:    subMchID,
		AuthType:    merchantcontracts.AuthTypeApplet,
		AuthContent: cfg.MiniProgramAppID,
		Remark:      "LocalLife mini program",
	}
	if err := bindReq.Validate(); err != nil {
		return db.BaofuMerchantReport{}, err
	}
	if _, err := s.store.CreateExternalPaymentCommand(ctx, db.CreateExternalPaymentCommandParams{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuMerchantReport,
		CommandType:        db.ExternalPaymentCommandTypeBaofuBindSubConfig,
		BusinessOwner:      db.ExternalPaymentBusinessOwnerApplyment,
		BusinessObjectType: pgtype.Text{String: "baofu_merchant_report", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: report.ID, Valid: true},
		ExternalObjectType: "baofu_bind_sub_config",
		ExternalObjectKey:  subMchID,
		CommandStatus:      db.ExternalPaymentCommandStatusSubmitted,
		SubmittedAt:        s.now().UTC(),
		ResponseSnapshot:   baofuBindSubConfigCommandSnapshot(subMchID),
	}); err != nil {
		return db.BaofuMerchantReport{}, err
	}
	result, err := s.client.BindSubConfig(ctx, bindReq)
	if err != nil {
		return db.BaofuMerchantReport{}, err
	}
	if result != nil && strings.TrimSpace(result.ResultCode) != "" && strings.ToUpper(strings.TrimSpace(result.ResultCode)) != "SUCCESS" {
		return s.store.MarkBaofuMerchantReportAppletAuthFailed(ctx, db.MarkBaofuMerchantReportAppletAuthFailedParams{ID: report.ID, FailureCode: baofuReportText(result.ErrorCode), FailureMessage: baofuReportText(result.ErrorMessage)})
	}
	return s.store.MarkBaofuMerchantReportAppletAuthSucceeded(ctx, report.ID)
}

func baofuMerchantReportCommandSnapshot(reportNo, bctMerID string) []byte {
	body, err := json.Marshal(map[string]string{"reportNo": strings.TrimSpace(reportNo), "bctMerId": strings.TrimSpace(bctMerID)})
	if err != nil {
		return []byte(`{}`)
	}
	return body
}

func baofuMerchantReportResultSnapshot(result *merchantcontracts.MerchantReportResult) []byte {
	if result == nil {
		return []byte(`{}`)
	}
	body, err := json.Marshal(map[string]string{"reportNo": result.ReportNo, "reportState": result.ReportState, "subMchId": result.SubMchID, "platformBizNo": result.PlatformBizNo, "resultCode": result.ResultCode, "errCode": result.ErrorCode})
	if err != nil {
		return []byte(`{}`)
	}
	return body
}

func baofuBindSubConfigCommandSnapshot(subMchID string) []byte {
	body, err := json.Marshal(map[string]string{"subMchId": strings.TrimSpace(subMchID), "authType": merchantcontracts.AuthTypeApplet})
	if err != nil {
		return []byte(`{}`)
	}
	return body
}

func baofuReportText(value string) pgtype.Text {
	value = strings.TrimSpace(value)
	return pgtype.Text{String: value, Valid: value != ""}
}
