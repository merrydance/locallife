package api

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

func (server *Server) baofuSettlementAccountResponseFromResult(scope baofuSettlementAccountScope, result logic.BaofuAccountOpeningResult) baofuSettlementAccountResponse {
	resp := baofuSettlementAccountResponse{
		OwnerType:       scope.OwnerType,
		OwnerID:         scope.OwnerID,
		AccountType:     firstNonBlank(result.Flow.AccountType, result.Profile.AccountType, scope.AccountType),
		ProfileStatus:   strings.TrimSpace(result.Profile.ProfileStatus),
		FlowID:          result.Flow.ID,
		FlowState:       strings.TrimSpace(result.Flow.State),
		VerifyFeeAmount: server.config.BaofuAccountVerifyFeeFen,
		SubmittedAt:     timePtrIfNotZero(result.Flow.CreatedAt),
		UpdatedAt:       timePtrIfNotZero(result.Flow.UpdatedAt),
	}
	if resp.VerifyFeeAmount <= 0 {
		resp.VerifyFeeAmount = logic.BaofuAccountOpenVerifyFeeFen
	}
	resp.applyStatus(result.State, result.Label)
	if strings.TrimSpace(result.StatusDesc) != "" {
		resp.StatusDesc = result.StatusDesc
	}
	if resp.Status == db.BaofuAccountOpeningStateProfilePending ||
		strings.TrimSpace(result.Profile.ProfileStatus) != "" && strings.TrimSpace(result.Profile.ProfileStatus) != db.BaofuAccountOpeningProfileStatusComplete {
		resp.applyProfileGuidance(result.Profile, result.MissingFields, result.StatusDesc)
	}
	resp.addProfileMasks(result.Profile)
	if result.Binding != nil {
		if accountType := strings.TrimSpace(result.Binding.AccountType); accountType != "" {
			resp.AccountType = accountType
		}
		resp.OpenState = strings.TrimSpace(result.Binding.OpenState)
		resp.BankCardLast4 = pgTextString(result.Binding.BankCardLast4)
		resp.WechatSubMchIDMask = maskSensitiveTail(pgTextString(result.Binding.WechatSubMchID), 4)
		if result.Binding.UpdatedAt.After(result.Flow.UpdatedAt) {
			resp.UpdatedAt = &result.Binding.UpdatedAt
		}
	}
	resp.addPayment(result.PaymentOrder, newMiniProgramPayParams(result.PayParams))
	if resp.Status == db.BaofuAccountOpeningStateReady {
		resp.PaymentReady = true
	}
	return resp
}

func (resp *baofuSettlementAccountResponse) applyFlowState(state string) {
	resp.applyStatus(strings.TrimSpace(state), baofuSettlementAccountStateLabel(state))
	switch strings.TrimSpace(state) {
	case db.BaofuAccountOpeningStateVerifyFeePending:
		resp.StatusDesc = "支付 2 元核验费后继续开户，支付未完成可重新发起支付"
	case db.BaofuAccountOpeningStateVerifyFeeProcessing:
		resp.StatusDesc = "核验费支付结果确认中，可稍后刷新"
	case db.BaofuAccountOpeningStateFailed:
		resp.StatusDesc = "开户未通过，请核对资料后重试"
	}
}

func (resp *baofuSettlementAccountResponse) applyFlowFailure(flow db.BaofuAccountOpeningFlow) {
	resp.applyFlowState(flow.State)
	if strings.TrimSpace(flow.State) == db.BaofuAccountOpeningStateFailed {
		resp.StatusDesc = logic.BaofuAccountOpeningFailureStatusDesc(flow.FailureCode.String)
	}
}

func (resp *baofuSettlementAccountResponse) applyStatus(status, label string) {
	status = strings.TrimSpace(status)
	if status == "" {
		status = db.BaofuAccountOpeningStateProfilePending
	}
	resp.Status = status
	resp.State = status
	if strings.TrimSpace(label) == "" {
		label = baofuSettlementAccountStateLabel(status)
	}
	resp.Label = label
}

func (resp *baofuSettlementAccountResponse) addProfileMasks(profile db.BaofuAccountOpeningProfile) {
	resp.BankAccountNoMask = pgTextString(profile.BankAccountNoMask)
	resp.BankMobileMask = pgTextString(profile.BankMobileMask)
	resp.ContactMobileMask = pgTextString(profile.ContactMobileMask)
	resp.EmailMask = pgTextString(profile.EmailMask)
}

func (resp *baofuSettlementAccountResponse) applyProfileGuidance(profile db.BaofuAccountOpeningProfile, missingFields []string, statusDesc string) {
	if len(missingFields) == 0 {
		missingFields = logic.BaofuAccountOpeningProfileMissingFields(profile)
	}
	if len(missingFields) > 0 {
		resp.MissingFields = missingFields
	}
	if strings.TrimSpace(statusDesc) != "" {
		resp.StatusDesc = statusDesc
		return
	}
	if strings.TrimSpace(resp.Status) == db.BaofuAccountOpeningStateProfilePending ||
		strings.TrimSpace(profile.ProfileStatus) != "" && strings.TrimSpace(profile.ProfileStatus) != db.BaofuAccountOpeningProfileStatusComplete {
		resp.StatusDesc = logic.BaofuAccountOpeningProfilePendingStatusDesc(missingFields)
	}
}

func (resp *baofuSettlementAccountResponse) addPayment(payment db.PaymentOrder, payParams *miniProgramPayParams) {
	if payment.ID == 0 {
		return
	}
	expiresAt := pgTimePtr(payment.ExpiresAt)
	resp.PaymentOrderID = payment.ID
	resp.Amount = payment.Amount
	resp.BusinessType = payment.BusinessType
	resp.OutTradeNo = payment.OutTradeNo
	resp.PayParams = payParams
	resp.ExpiresAt = expiresAt
	resp.Payment = &baofuSettlementAccountPaymentResponse{
		PaymentOrderID: payment.ID,
		Amount:         payment.Amount,
		BusinessType:   payment.BusinessType,
		OutTradeNo:     payment.OutTradeNo,
		PayParams:      payParams,
		ExpiresAt:      expiresAt,
	}
}

func (resp *baofuSettlementAccountResponse) addPaymentFromFlow(ctx *gin.Context, server *Server, flow db.BaofuAccountOpeningFlow) error {
	if !flow.VerifyFeePaymentOrderID.Valid || flow.VerifyFeePaymentOrderID.Int64 <= 0 {
		return nil
	}
	payment, err := server.store.GetPaymentOrder(ctx, flow.VerifyFeePaymentOrderID.Int64)
	if err != nil {
		return baofuSettlementAccountPaymentLoadError{
			FlowID:         flow.ID,
			PaymentOrderID: flow.VerifyFeePaymentOrderID.Int64,
			Err:            err,
		}
	}
	var payParams *miniProgramPayParams
	if payment.Status == "pending" && payment.PrepayID.Valid {
		if server.directPaymentClient == nil {
			return baofuSettlementAccountPaymentLoadError{
				FlowID:         flow.ID,
				PaymentOrderID: payment.ID,
				Err:            errors.New("direct payment client is not configured"),
			}
		}
		params, err := server.directPaymentClient.GenerateJSAPIPayParams(payment.PrepayID.String)
		if err != nil {
			return baofuSettlementAccountPaymentLoadError{
				FlowID:         flow.ID,
				PaymentOrderID: payment.ID,
				Err:            fmt.Errorf("regenerate baofu verify fee pay params: %w", err),
			}
		}
		payParams = newMiniProgramPayParams(params)
	}
	resp.addPayment(payment, payParams)
	return nil
}

func baofuSettlementAccountStateLabel(state string) string {
	switch strings.TrimSpace(state) {
	case db.BaofuAccountOpeningStateReady:
		return "结算账户可用"
	case db.BaofuAccountOpeningStateProfilePending:
		return "资料待补充"
	case db.BaofuAccountOpeningStateVerifyFeePending, db.BaofuAccountOpeningStateVerifyFeeProcessing:
		return "核验费待确认"
	case db.BaofuAccountOpeningStateOpeningProcessing:
		return "宝付开户处理中"
	case db.BaofuAccountOpeningStateMerchantReportProcessing:
		return "商户报备处理中"
	case db.BaofuAccountOpeningStateAppletAuthPending:
		return "授权目录绑定中"
	case db.BaofuAccountOpeningStateFailed:
		return "开通失败"
	default:
		return "同步中"
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func timePtrIfNotZero(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func pgTimePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func pgTextString(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func maskSensitiveTail(value string, tail int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if tail <= 0 || len(value) <= tail {
		return "***"
	}
	return "***" + value[len(value)-tail:]
}
