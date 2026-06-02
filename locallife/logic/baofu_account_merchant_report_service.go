package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
	merchantcontracts "github.com/merrydance/locallife/baofu/merchantreport/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
)

type baofuAccountMerchantReportStore interface {
	GetBaofuAccountOpeningProfile(ctx context.Context, id int64) (db.BaofuAccountOpeningProfile, error)
	GetBaofuMerchantReportByOwner(ctx context.Context, arg db.GetBaofuMerchantReportByOwnerParams) (db.BaofuMerchantReport, error)
	GetMerchant(ctx context.Context, id int64) (db.Merchant, error)
	GetRegion(ctx context.Context, id int64) (db.Region, error)
	UpsertMerchantPaymentConfig(ctx context.Context, arg db.UpsertMerchantPaymentConfigParams) (db.MerchantPaymentConfig, error)
	MarkMerchantBaofuAccountOpeningReadyTx(ctx context.Context, arg db.MarkMerchantBaofuAccountOpeningReadyTxParams) (db.MarkMerchantBaofuAccountOpeningReadyTxResult, error)
	MarkBaofuAccountOpeningFlowMerchantReportProcessing(ctx context.Context, arg db.MarkBaofuAccountOpeningFlowMerchantReportProcessingParams) (db.BaofuAccountOpeningFlow, error)
	MarkBaofuAccountOpeningFlowAppletAuthPending(ctx context.Context, arg db.MarkBaofuAccountOpeningFlowAppletAuthPendingParams) (db.BaofuAccountOpeningFlow, error)
	MarkBaofuAccountOpeningFlowReady(ctx context.Context, arg db.MarkBaofuAccountOpeningFlowReadyParams) (db.BaofuAccountOpeningFlow, error)
	MarkBaofuAccountOpeningFlowFailed(ctx context.Context, arg db.MarkBaofuAccountOpeningFlowFailedParams) (db.BaofuAccountOpeningFlow, error)

	baofuMerchantReportStore
}

type BaofuAccountMerchantReportConfig struct {
	CollectMerchantID string
	CollectTerminalID string
	MiniProgramAppID  string
	ChannelID         string
	ChannelName       string
	Business          string
}

type BaofuAccountMerchantReportService struct {
	store     baofuAccountMerchantReportStore
	client    baofuMerchantReportClient
	encryptor util.DataEncryptor
	config    BaofuAccountMerchantReportConfig
	now       func() time.Time
}

func NewBaofuAccountMerchantReportService(store baofuAccountMerchantReportStore, client baofuMerchantReportClient, encryptor util.DataEncryptor, config BaofuAccountMerchantReportConfig) *BaofuAccountMerchantReportService {
	return &BaofuAccountMerchantReportService{store: store, client: client, encryptor: encryptor, config: config.normalized(), now: time.Now}
}

func (c BaofuAccountMerchantReportConfig) normalized() BaofuAccountMerchantReportConfig {
	c.CollectMerchantID = strings.TrimSpace(c.CollectMerchantID)
	c.CollectTerminalID = strings.TrimSpace(c.CollectTerminalID)
	c.MiniProgramAppID = strings.TrimSpace(c.MiniProgramAppID)
	c.ChannelID = strings.TrimSpace(c.ChannelID)
	c.ChannelName = strings.TrimSpace(c.ChannelName)
	c.Business = strings.TrimSpace(c.Business)
	if c.Business == "" {
		c.Business = "758-2"
	}
	return c
}

func (s *BaofuAccountMerchantReportService) RecoverMerchantReportFlow(ctx context.Context, flow db.BaofuAccountOpeningFlow) (db.BaofuAccountOpeningFlow, error) {
	if s == nil || s.store == nil || s.client == nil {
		return db.BaofuAccountOpeningFlow{}, ErrBaofuMerchantReportServiceNotConfigured
	}
	if strings.TrimSpace(flow.OwnerType) != db.BaofuAccountOwnerTypeMerchant {
		return flow, nil
	}
	switch strings.TrimSpace(flow.State) {
	case db.BaofuAccountOpeningStateMerchantReportProcessing, db.BaofuAccountOpeningStateAppletAuthPending:
	default:
		return flow, nil
	}
	cfg := s.config.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" || cfg.MiniProgramAppID == "" || cfg.ChannelID == "" || cfg.ChannelName == "" {
		return db.BaofuAccountOpeningFlow{}, ErrBaofuMerchantReportServiceNotConfigured
	}

	report, err := s.store.GetBaofuMerchantReportByOwner(ctx, db.GetBaofuMerchantReportByOwnerParams{
		OwnerType:  db.BaofuAccountOwnerTypeMerchant,
		OwnerID:    flow.OwnerID,
		ReportType: db.BaofuMerchantReportTypeWechat,
	})
	if errors.Is(err, db.ErrRecordNotFound) {
		report, err = s.submitMerchantReport(ctx, flow, cfg)
	}
	if err != nil {
		return db.BaofuAccountOpeningFlow{}, mapBaofuMerchantReportError(err, baofuMerchantReportErrorContext(flow, report, "merchant_report"))
	}
	if report.ID == 0 {
		return db.BaofuAccountOpeningFlow{}, errors.New("baofu merchant report is required")
	}

	reportService := NewBaofuMerchantReportService(s.store, s.client, BaofuMerchantReportConfig{
		CollectMerchantID: cfg.CollectMerchantID,
		CollectTerminalID: cfg.CollectTerminalID,
		MiniProgramAppID:  cfg.MiniProgramAppID,
	})
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateProcessing ||
		(strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateSucceeded && strings.TrimSpace(report.AppletAuthState) != db.BaofuMerchantReportAppletAuthStateSucceeded) {
		reportBeforeRecover := report
		report, err = reportService.RecoverWechatMerchantReport(ctx, report)
		if err != nil {
			return db.BaofuAccountOpeningFlow{}, mapBaofuMerchantReportError(err, baofuMerchantReportErrorContext(flow, reportBeforeRecover, baofuMerchantReportProviderOperation(reportBeforeRecover)))
		}
	}
	return s.applyMerchantReportResult(ctx, flow, report)
}

func baofuMerchantReportProviderOperation(report db.BaofuMerchantReport) string {
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateProcessing {
		return "merchant_report_query"
	}
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateSucceeded &&
		strings.TrimSpace(report.AppletAuthState) != db.BaofuMerchantReportAppletAuthStateSucceeded {
		return "bind_sub_config"
	}
	return "baofu_merchant_report_recover"
}

func baofuMerchantReportErrorContext(flow db.BaofuAccountOpeningFlow, report db.BaofuMerchantReport, operation string) BaofuProviderErrorContext {
	return BaofuProviderErrorContext{
		FlowID:             flow.ID,
		OwnerType:          flow.OwnerType,
		OwnerID:            flow.OwnerID,
		OpenTransSerialNo:  flow.OpenTransSerialNo.String,
		CurrentState:       flow.State,
		MerchantReportID:   report.ID,
		MerchantReportNo:   report.ReportNo,
		ProviderOperation:  operation,
		ProviderCapability: "baofu_merchant_report",
	}
}

func (s *BaofuAccountMerchantReportService) submitMerchantReport(ctx context.Context, flow db.BaofuAccountOpeningFlow, cfg BaofuAccountMerchantReportConfig) (db.BaofuMerchantReport, error) {
	input, err := s.buildMerchantReportInput(ctx, flow, cfg)
	if err != nil {
		return db.BaofuMerchantReport{}, err
	}
	reportService := NewBaofuMerchantReportService(s.store, s.client, BaofuMerchantReportConfig{
		CollectMerchantID: cfg.CollectMerchantID,
		CollectTerminalID: cfg.CollectTerminalID,
		MiniProgramAppID:  cfg.MiniProgramAppID,
	})
	return reportService.SubmitWechatMerchantReport(ctx, input)
}

func (s *BaofuAccountMerchantReportService) buildMerchantReportInput(ctx context.Context, flow db.BaofuAccountOpeningFlow, cfg BaofuAccountMerchantReportConfig) (SubmitBaofuMerchantReportInput, error) {
	if !flow.ProfileID.Valid {
		return SubmitBaofuMerchantReportInput{}, errors.New("baofu account opening profile is required for merchant report")
	}
	profile, err := s.store.GetBaofuAccountOpeningProfile(ctx, flow.ProfileID.Int64)
	if err != nil {
		return SubmitBaofuMerchantReportInput{}, err
	}
	merchant, err := s.store.GetMerchant(ctx, flow.OwnerID)
	if err != nil {
		return SubmitBaofuMerchantReportInput{}, err
	}
	licenseNo, err := decryptOptional(s.encryptor, profile.CertificateNoCiphertext.String)
	if err != nil {
		return SubmitBaofuMerchantReportInput{}, err
	}
	bankCardNo, err := decryptOptional(s.encryptor, profile.BankAccountNoCiphertext.String)
	if err != nil {
		return SubmitBaofuMerchantReportInput{}, err
	}
	contactMobile, err := decryptOptional(s.encryptor, profile.ContactMobileCiphertext.String)
	if err != nil {
		return SubmitBaofuMerchantReportInput{}, err
	}
	appData := merchantApplicationDataSnapshot{}
	if len(merchant.ApplicationData) > 0 {
		_ = json.Unmarshal(merchant.ApplicationData, &appData)
	}
	address, err := s.buildMerchantReportAddress(ctx, merchant)
	if err != nil {
		return SubmitBaofuMerchantReportInput{}, err
	}
	reportNo, err := util.GenerateOutTradeNo("BMR")
	if err != nil {
		return SubmitBaofuMerchantReportInput{}, err
	}
	servicePhone := firstTrimmed(contactMobile, merchant.Phone)
	businessLicenseType := merchantcontracts.WechatCertificateTypeNationalLegalMerge
	if strings.TrimSpace(profile.AccountType) == db.BaofuAccountTypePersonal {
		businessLicenseType = merchantcontracts.WechatCertificateTypeIdentityCard
	}
	return SubmitBaofuMerchantReportInput{
		MerchantID:          flow.OwnerID,
		ReportNo:            reportNo,
		MerchantName:        firstTrimmed(profile.LegalName.String, appData.MerchantName, merchant.Name),
		MerchantShortName:   truncateRunes(firstTrimmed(merchant.Name, profile.LegalName.String), 20),
		ServicePhone:        servicePhone,
		ChannelID:           cfg.ChannelID,
		ChannelName:         cfg.ChannelName,
		Business:            cfg.Business,
		BusinessLicenseType: businessLicenseType,
		BusinessLicense:     firstTrimmed(licenseNo, appData.BusinessLicenseNumber),
		AddressInfo:         address,
		BankCardInfo: merchantcontracts.WechatBankCardInfo{
			CardName:       truncateRunes(firstTrimmed(profile.CardUserName.String, profile.LegalName.String), 32),
			CardNo:         bankCardNo,
			BankBranchName: truncateRunes(profile.DepositBankName.String, 32),
		},
	}, nil
}

func (s *BaofuAccountMerchantReportService) buildMerchantReportAddress(ctx context.Context, merchant db.Merchant) (merchantcontracts.WechatAddressInfo, error) {
	regions, err := s.loadRegionChain(ctx, merchant.RegionID)
	if err != nil {
		return merchantcontracts.WechatAddressInfo{}, err
	}
	var province, city, district string
	for _, region := range regions {
		switch region.Level {
		case 1:
			province = firstTrimmed(province, region.Code)
		case 2:
			city = firstTrimmed(city, region.Code)
		case 3, 4:
			district = firstTrimmed(district, region.Code)
		}
	}
	if district == "" && len(regions) > 0 {
		district = regions[0].Code
	}
	if province == "" || city == "" || district == "" {
		return merchantcontracts.WechatAddressInfo{}, fmt.Errorf("merchant %d region chain is incomplete for baofu merchant report", merchant.ID)
	}
	return merchantcontracts.WechatAddressInfo{
		ProvinceCode: province,
		CityCode:     city,
		DistrictCode: district,
		Address:      truncateRunes(merchant.Address, 64),
		Longitude:    numericString(merchant.Longitude, 6),
		Latitude:     numericString(merchant.Latitude, 6),
		Type:         "BUSINESS_ADDRESS",
	}, nil
}

func (s *BaofuAccountMerchantReportService) loadRegionChain(ctx context.Context, regionID int64) ([]db.Region, error) {
	var regions []db.Region
	for regionID > 0 {
		region, err := s.store.GetRegion(ctx, regionID)
		if err != nil {
			return nil, err
		}
		regions = append(regions, region)
		if !region.ParentID.Valid || region.ParentID.Int64 <= 0 {
			break
		}
		regionID = region.ParentID.Int64
	}
	return regions, nil
}

func (s *BaofuAccountMerchantReportService) applyMerchantReportResult(ctx context.Context, flow db.BaofuAccountOpeningFlow, report db.BaofuMerchantReport) (db.BaofuAccountOpeningFlow, error) {
	reportID := pgtype.Int8{Int64: report.ID, Valid: report.ID > 0}
	subMchID := strings.TrimSpace(report.SubMchID.String)
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateFailed || strings.TrimSpace(report.AppletAuthState) == db.BaofuMerchantReportAppletAuthStateFailed {
		return s.store.MarkBaofuAccountOpeningFlowFailed(ctx, db.MarkBaofuAccountOpeningFlowFailedParams{
			ID:             flow.ID,
			FailureCode:    report.FailureCode,
			FailureMessage: report.FailureMessage,
			RawSnapshot:    report.RawSnapshot,
		})
	}
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateSucceeded && strings.TrimSpace(report.AppletAuthState) == db.BaofuMerchantReportAppletAuthStateSucceeded {
		if subMchID == "" {
			return db.BaofuAccountOpeningFlow{}, ErrBaofuMerchantReportSubMchIDRequired
		}
		result, err := s.store.MarkMerchantBaofuAccountOpeningReadyTx(ctx, db.MarkMerchantBaofuAccountOpeningReadyTxParams{
			PaymentConfig: db.UpsertMerchantPaymentConfigParams{
				MerchantID: flow.OwnerID,
				SubMchID:   subMchID,
				Status:     db.MerchantPaymentConfigStatusActive,
			},
			Flow: db.MarkBaofuAccountOpeningFlowReadyParams{
				ID:               flow.ID,
				AccountBindingID: flow.AccountBindingID,
				MerchantReportID: reportID,
				RawSnapshot:      report.RawSnapshot,
			},
		})
		if err != nil {
			return db.BaofuAccountOpeningFlow{}, err
		}
		return result.Flow, nil
	}
	if subMchID != "" && strings.TrimSpace(flow.OwnerType) == db.BaofuAccountOwnerTypeMerchant {
		if _, err := s.store.UpsertMerchantPaymentConfig(ctx, db.UpsertMerchantPaymentConfigParams{
			MerchantID: flow.OwnerID,
			SubMchID:   subMchID,
			Status:     db.MerchantPaymentConfigStatusActive,
		}); err != nil {
			return db.BaofuAccountOpeningFlow{}, err
		}
	}
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateSucceeded {
		return s.store.MarkBaofuAccountOpeningFlowAppletAuthPending(ctx, db.MarkBaofuAccountOpeningFlowAppletAuthPendingParams{
			ID:               flow.ID,
			MerchantReportID: reportID,
			RawSnapshot:      report.RawSnapshot,
		})
	}
	return s.store.MarkBaofuAccountOpeningFlowMerchantReportProcessing(ctx, db.MarkBaofuAccountOpeningFlowMerchantReportProcessingParams{
		ID:               flow.ID,
		AccountBindingID: flow.AccountBindingID,
		MerchantReportID: reportID,
		RawSnapshot:      report.RawSnapshot,
	})
}

type merchantApplicationDataSnapshot struct {
	MerchantName            string `json:"merchant_name"`
	BusinessLicenseNumber   string `json:"business_license_number"`
	LegalPersonName         string `json:"legal_person_name"`
	LegalPersonIDNumber     string `json:"legal_person_id_number"`
	BusinessLicenseMediaID  int64  `json:"business_license_media_asset_id"`
	IDCardFrontMediaAssetID int64  `json:"id_card_front_media_asset_id"`
	IDCardBackMediaAssetID  int64  `json:"id_card_back_media_asset_id"`
	FoodPermitMediaAssetID  int64  `json:"food_permit_media_asset_id"`
}

func truncateRunes(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}

func numericString(value pgtype.Numeric, precision int) string {
	if !value.Valid {
		return ""
	}
	floatValue, err := value.Float64Value()
	if err != nil || !floatValue.Valid {
		return ""
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(floatValue.Float64, 'f', precision, 64), "0"), ".")
}
