package api

import (
	"strings"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

func (server *Server) loadBaofuSettlementAccount(ctx *gin.Context, scope baofuSettlementAccountScope) (baofuSettlementAccountResponse, error) {
	resp := baofuSettlementAccountResponse{
		OwnerType:       scope.OwnerType,
		OwnerID:         scope.OwnerID,
		AccountType:     scope.AccountType,
		VerifyFeeAmount: server.config.BaofuAccountVerifyFeeFen,
	}
	if resp.VerifyFeeAmount <= 0 {
		resp.VerifyFeeAmount = logic.BaofuAccountOpenVerifyFeeFen
	}

	binding, bindingFound, err := server.loadBaofuAccountBinding(ctx, scope)
	if err != nil {
		return baofuSettlementAccountResponse{}, err
	}
	if bindingFound {
		if accountType := strings.TrimSpace(binding.AccountType); accountType != "" {
			resp.AccountType = accountType
		}
		resp.OpenState = strings.TrimSpace(binding.OpenState)
		resp.BankCardLast4 = pgTextString(binding.BankCardLast4)
		resp.WechatSubMchIDMask = maskSensitiveTail(pgTextString(binding.WechatSubMchID), 4)
		resp.UpdatedAt = &binding.UpdatedAt
	}

	profile, profileFound, err := server.loadBaofuAccountOpeningProfile(ctx, scope)
	if err != nil {
		return baofuSettlementAccountResponse{}, err
	}
	if profileFound {
		if accountType := strings.TrimSpace(profile.AccountType); accountType != "" {
			resp.AccountType = accountType
		}
		resp.ProfileStatus = strings.TrimSpace(profile.ProfileStatus)
		resp.addProfileMasks(profile)
		if resp.UpdatedAt == nil || profile.UpdatedAt.After(*resp.UpdatedAt) {
			resp.UpdatedAt = &profile.UpdatedAt
		}
	}

	activeBindingFound := bindingFound && strings.TrimSpace(binding.OpenState) == db.BaofuAccountOpenStateActive
	failedOpeningFlowFound := false
	flow, flowFound, err := server.loadLatestBaofuAccountOpeningFlow(ctx, scope)
	if err != nil {
		return baofuSettlementAccountResponse{}, err
	}
	if flowFound {
		flowState := strings.TrimSpace(flow.State)
		failedOpeningFlowFound = flowState == db.BaofuAccountOpeningStateFailed
	}
	if flowFound && !activeBindingFound {
		if accountType := strings.TrimSpace(flow.AccountType); accountType != "" {
			resp.AccountType = accountType
		}
		resp.FlowID = flow.ID
		resp.FlowState = strings.TrimSpace(flow.State)
		resp.SubmittedAt = &flow.CreatedAt
		resp.applyFlowFailure(flow)
		if err := resp.addPaymentFromFlow(ctx, server, flow); err != nil {
			return baofuSettlementAccountResponse{}, err
		}
		if resp.UpdatedAt == nil || flow.UpdatedAt.After(*resp.UpdatedAt) {
			resp.UpdatedAt = &flow.UpdatedAt
		}
	}

	if activeBindingFound {
		if err := server.applyActiveBaofuSettlementAccountStatus(ctx, scope, binding, &resp); err != nil {
			return baofuSettlementAccountResponse{}, err
		}
		if flowFound && strings.TrimSpace(flow.State) == strings.TrimSpace(resp.Status) && !failedOpeningFlowFound {
			resp.FlowID = flow.ID
			resp.FlowState = strings.TrimSpace(flow.State)
			resp.SubmittedAt = &flow.CreatedAt
			if resp.UpdatedAt == nil || flow.UpdatedAt.After(*resp.UpdatedAt) {
				resp.UpdatedAt = &flow.UpdatedAt
			}
		}
	} else if resp.Status == "" {
		resp.applyStatus(db.BaofuAccountOpeningStateProfilePending, "资料待补充")
	}
	if resp.Status == db.BaofuAccountOpeningStateProfilePending && profileFound {
		resp.applyProfileGuidance(profile, nil, "")
	} else if resp.Status == db.BaofuAccountOpeningStateProfilePending {
		resp.StatusDesc = logic.BaofuAccountOpeningProfilePendingStatusDesc(nil)
	} else {
		resp.MissingFields = nil
	}
	if shouldIncludeBaofuSettlementAccountProfileDefaults(scope, resp.Status, failedOpeningFlowFound) {
		if err := server.applyBaofuSettlementAccountProfileDefaults(ctx, scope, &resp, profile, profileFound); err != nil {
			return baofuSettlementAccountResponse{}, err
		}
	}
	return resp, nil
}

func shouldIncludeBaofuSettlementAccountProfileDefaults(scope baofuSettlementAccountScope, status string, failedOpeningFlowFound bool) bool {
	switch strings.TrimSpace(status) {
	case db.BaofuAccountOpeningStateProfilePending:
		return true
	case db.BaofuAccountOpeningStateFailed:
		return failedOpeningFlowFound && strings.TrimSpace(scope.OwnerType) == db.BaofuAccountOwnerTypeMerchant
	default:
		return false
	}
}

func shouldMergeBaofuSettlementAccountProfileDefaults(scope baofuSettlementAccountScope, profile *logic.BaofuAccountOpeningProfileInput) bool {
	if profile == nil {
		return false
	}
	if strings.TrimSpace(scope.OwnerType) == db.BaofuAccountOwnerTypeMerchant {
		return true
	}
	return len(logic.BaofuAccountOpeningInputMissingFields(scope.OwnerType, *profile)) > 0
}

func baofuRiderDefaultProfile(rider db.Rider) *logic.BaofuAccountOpeningProfileInput {
	if firstNonBlank(rider.RealName, rider.IDCardNo) == "" {
		return nil
	}
	return &logic.BaofuAccountOpeningProfileInput{
		LegalName:     strings.TrimSpace(rider.RealName),
		CertificateNo: strings.TrimSpace(rider.IDCardNo),
		CardUserName:  strings.TrimSpace(rider.RealName),
	}
}

func baofuRiderDefaultProfileMasks(rider db.Rider) *baofuSettlementAccountProfileDefaults {
	if firstNonBlank(rider.RealName, rider.IDCardNo) == "" {
		return nil
	}
	return &baofuSettlementAccountProfileDefaults{
		Source:                    "rider_identity",
		LegalName:                 strings.TrimSpace(rider.RealName),
		CertificateNoMask:         maskSensitiveTail(rider.IDCardNo, 4),
		HasCertificateNo:          strings.TrimSpace(rider.IDCardNo) != "",
		HasSavedSensitiveDefaults: strings.TrimSpace(rider.IDCardNo) != "",
	}
}

func baofuOperatorDefaultProfile(operator db.Operator) *logic.BaofuAccountOpeningProfileInput {
	legalName := firstNonBlank(operator.ContactName, operator.Name)
	if legalName == "" {
		return nil
	}
	return &logic.BaofuAccountOpeningProfileInput{
		LegalName:    legalName,
		CardUserName: legalName,
	}
}

func baofuOperatorDefaultProfileMasks(operator db.Operator) *baofuSettlementAccountProfileDefaults {
	legalName := firstNonBlank(operator.ContactName, operator.Name)
	if legalName == "" {
		return nil
	}
	return &baofuSettlementAccountProfileDefaults{
		Source:    "operator_profile",
		LegalName: legalName,
	}
}

func (server *Server) applyActiveBaofuSettlementAccountStatus(ctx *gin.Context, scope baofuSettlementAccountScope, binding db.BaofuAccountBinding, resp *baofuSettlementAccountResponse) error {
	if strings.TrimSpace(scope.OwnerType) != db.BaofuAccountOwnerTypeMerchant {
		resp.applyStatus(db.BaofuAccountOpeningStateReady, "结算账户可用")
		resp.PaymentReady = true
		return nil
	}

	report, err := server.store.GetBaofuMerchantReportByOwner(ctx, db.GetBaofuMerchantReportByOwnerParams{
		OwnerType:  db.BaofuAccountOwnerTypeMerchant,
		OwnerID:    scope.OwnerID,
		ReportType: db.BaofuMerchantReportTypeWechat,
	})
	if err != nil {
		if isNotFoundError(err) {
			resp.applyStatus(db.BaofuAccountOpeningStateMerchantReportProcessing, baofuSettlementAccountStateLabel(db.BaofuAccountOpeningStateMerchantReportProcessing))
			return nil
		}
		return err
	}

	if subMchID := strings.TrimSpace(report.SubMchID.String); subMchID != "" {
		resp.WechatSubMchIDMask = maskSensitiveTail(subMchID, 4)
	}
	readiness := logic.ReadinessFromBaofuBindingAndMerchantReport(binding, report)
	if readiness.PaymentReady {
		resp.applyStatus(db.BaofuAccountOpeningStateReady, readiness.Label)
		resp.PaymentReady = true
		return nil
	}
	if strings.TrimSpace(readiness.State) == logic.BaofuOnboardingStateOpenFailed ||
		strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateFailed ||
		strings.TrimSpace(report.AppletAuthState) == db.BaofuMerchantReportAppletAuthStateFailed {
		resp.applyStatus(db.BaofuAccountOpeningStateFailed, baofuSettlementAccountStateLabel(db.BaofuAccountOpeningStateFailed))
		resp.StatusDesc = baofuMerchantReportFailureStatusDesc(report)
		return nil
	}
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateSucceeded {
		resp.applyStatus(db.BaofuAccountOpeningStateAppletAuthPending, baofuSettlementAccountStateLabel(db.BaofuAccountOpeningStateAppletAuthPending))
		return nil
	}
	resp.applyStatus(db.BaofuAccountOpeningStateMerchantReportProcessing, baofuSettlementAccountStateLabel(db.BaofuAccountOpeningStateMerchantReportProcessing))
	return nil
}

func baofuMerchantReportFailureStatusDesc(report db.BaofuMerchantReport) string {
	if strings.TrimSpace(report.AppletAuthState) == db.BaofuMerchantReportAppletAuthStateFailed {
		return "微信支付授权目录绑定失败，请联系平台处理后重试"
	}
	if strings.TrimSpace(report.ReportState) == db.BaofuMerchantReportStateFailed {
		return "微信支付商户报备失败，请核对商户资料后重试；如持续失败请联系平台处理"
	}
	return "开户未通过，请核对资料后重试"
}

func (server *Server) loadBaofuAccountBinding(ctx *gin.Context, scope baofuSettlementAccountScope) (db.BaofuAccountBinding, bool, error) {
	binding, err := server.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: scope.OwnerType,
		OwnerID:   scope.OwnerID,
	})
	if err != nil {
		if isNotFoundError(err) {
			return db.BaofuAccountBinding{}, false, nil
		}
		return db.BaofuAccountBinding{}, false, err
	}
	return binding, true, nil
}

func (server *Server) loadBaofuAccountOpeningProfile(ctx *gin.Context, scope baofuSettlementAccountScope) (db.BaofuAccountOpeningProfile, bool, error) {
	profile, err := server.store.GetBaofuAccountOpeningProfileByOwner(ctx, db.GetBaofuAccountOpeningProfileByOwnerParams{
		OwnerType: scope.OwnerType,
		OwnerID:   scope.OwnerID,
	})
	if err != nil {
		if isNotFoundError(err) {
			return db.BaofuAccountOpeningProfile{}, false, nil
		}
		return db.BaofuAccountOpeningProfile{}, false, err
	}
	return profile, true, nil
}

func (server *Server) loadLatestBaofuAccountOpeningFlow(ctx *gin.Context, scope baofuSettlementAccountScope) (db.BaofuAccountOpeningFlow, bool, error) {
	flow, err := server.store.GetLatestBaofuAccountOpeningFlowByOwner(ctx, db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
		OwnerType: scope.OwnerType,
		OwnerID:   scope.OwnerID,
	})
	if err != nil {
		if isNotFoundError(err) {
			return db.BaofuAccountOpeningFlow{}, false, nil
		}
		return db.BaofuAccountOpeningFlow{}, false, err
	}
	return flow, true, nil
}
