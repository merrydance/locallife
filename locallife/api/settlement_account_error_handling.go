package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	"github.com/rs/zerolog/log"
)

var (
	errSettlementMerchantApplymentNotFound            = errors.New("未找到商户普通服务商进件记录，请先完成微信支付进件后再修改结算账户")
	errSettlementBankBranchRequired                   = errors.New("所选开户银行要求填写支行信息，请选择支行或填写支行名称后重试")
	errSettlementWechatParamError                     = errors.New("微信支付拒绝了结算账户资料，请检查账户类型、开户银行、支行、账号和户名后重试")
	errSettlementWechatInvalidRequest                 = errors.New("微信支付拒绝了结算账户修改请求，请检查当前申请状态、银行信息和每日提交次数后重试")
	errSettlementApplicationQueryWechatParamError     = errors.New("微信支付拒绝了结算账户申请查询，请检查申请单号和账号展示规则后重试")
	errSettlementApplicationQueryWechatInvalidRequest = errors.New("微信支付拒绝了结算账户申请查询，请联系平台管理员检查商户配置、请求路径和签名参数")
	errSettlementWechatApplicationPending             = errors.New("结算账户修改申请仍在微信审核中，请等待当前申请出结果后再提交新的修改")
	errSettlementWechatDailyLimit                     = errors.New("微信支付限制每个特约商户每天最多提交 5 次结算账户修改申请，请次日 0 点后再试")
	errSettlementWechatNameMismatch                   = errors.New("开户名称必须与商户主体名称一致；如需使用非同名结算账户，请走微信支付专用非同名结算账户流程")
	errSettlementQueryPreparationFailed               = errors.New("普通服务商结算账户查询参数准备失败，请联系平台管理员检查商户号和账号展示规则配置后重试")
	errSettlementApplicationQueryPreparationFailed    = errors.New("普通服务商结算账户申请查询参数准备失败，请联系平台管理员检查商户号、申请单号和账号展示规则配置后重试")
)

func settlementPublicErrorResponse(err error) ErrorResponse {
	if apiErr := AsAPIError(err); apiErr != nil {
		return ErrorResponse{Error: apiErr.Message}
	}
	return errorResponse(err)
}

func respondSettlementClientError(ctx *gin.Context, status int, operation string, subjectType string, subjectID int64, subMchID string, applicationNo string, err error) {
	respondSettlementClientErrorWithMessage(ctx, status, operation, subjectType, subjectID, subMchID, applicationNo, err, "")
}

func respondSettlementClientErrorWithMessage(ctx *gin.Context, status int, operation string, subjectType string, subjectID int64, subMchID string, applicationNo string, err error, publicMessage string) {
	_ = ctx.Error(err)

	evt := log.Warn()
	if status >= http.StatusInternalServerError {
		evt = log.Error()
	}

	evt.
		Err(err).
		Str("request_id", GetRequestID(ctx)).
		Str("operation", operation).
		Str("subject_type", subjectType).
		Int64("subject_id", subjectID).
		Str("sub_mch_id", strings.TrimSpace(subMchID)).
		Str("application_no", strings.TrimSpace(applicationNo)).
		Int("http_status", status).
		Msg("settlement request rejected")

	if strings.TrimSpace(publicMessage) != "" {
		ctx.JSON(status, errorResponse(errors.New(publicMessage)))
		return
	}
	ctx.JSON(status, settlementPublicErrorResponse(err))
}

func settlementWechatErrorResponse(ctx *gin.Context, operation string, subjectType string, subjectID int64, subMchID string, applicationNo string, err error) (int, ErrorResponse) {
	_ = ctx.Error(err)

	var ordinaryErr *ordinaryserviceprovider.ProviderError
	if errors.As(err, &ordinaryErr) && ordinaryErr != nil {
		ordinaryserviceprovider.LogProviderError(log.Logger, err, ordinaryserviceprovider.ErrorLogContext{SubMchID: subMchID})
		switch ordinaryErr.Category {
		case ordinaryserviceprovider.ErrorCategoryValidation:
			return http.StatusBadRequest, errorResponse(errors.New(ordinaryErr.Frontend.Action))
		case ordinaryserviceprovider.ErrorCategoryAuthConfig:
			return http.StatusServiceUnavailable, errorResponse(errors.New("普通服务商结算账户服务配置不可用，请联系平台管理员检查微信支付服务商证书、公钥和权限配置后重试"))
		case ordinaryserviceprovider.ErrorCategoryMerchantControl:
			return http.StatusForbidden, errorResponse(errors.New("微信支付限制了该特约商户的结算账户能力，请在平台查看商户管控原因并按微信指引解除限制后重试"))
		case ordinaryserviceprovider.ErrorCategoryBusinessConflict:
			return http.StatusConflict, errorResponse(errors.New(ordinaryErr.Frontend.Action))
		case ordinaryserviceprovider.ErrorCategoryRetryableProvider:
			return http.StatusBadGateway, errorResponse(errors.New("微信支付普通服务商接口暂时不可用，请稍后重试；如持续失败请联系平台管理员查看结算账户服务日志并处理"))
		default:
			return http.StatusBadGateway, errorResponse(errors.New("微信支付普通服务商返回异常，请稍后重试或联系平台管理员处理"))
		}
	}

	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) && wxErr != nil {
		evt := log.Error()
		if wxErr.StatusCode < http.StatusInternalServerError && wxErr.Code != "SIGN_ERROR" {
			evt = log.Warn()
		}

		evt.
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("operation", operation).
			Str("subject_type", subjectType).
			Int64("subject_id", subjectID).
			Str("sub_mch_id", strings.TrimSpace(subMchID)).
			Str("application_no", strings.TrimSpace(applicationNo)).
			Int("wechat_status_code", wxErr.StatusCode).
			Str("wechat_error_code", strings.TrimSpace(wxErr.Code)).
			Str("wechat_error_message", strings.TrimSpace(wxErr.Message)).
			Str("wechat_error_detail", strings.TrimSpace(wxErr.Detail)).
			Msg("wechat settlement request failed")

		if operation == "query_settlement_application" && wxErr.StatusCode == http.StatusNotFound {
			switch strings.TrimSpace(wxErr.Code) {
			case "ORDER_NOT_EXIST", "RESOURCE_NOT_EXISTS", "NOT_FOUND":
				return http.StatusNotFound, errorResponse(ErrSettlementApplicationNotFound)
			default:
				return http.StatusBadGateway, settlementPublicErrorResponse(ErrSettlementWechatInvalidResponse)
			}
		}

		if operation == "query_settlement_application" {
			switch strings.TrimSpace(wxErr.Code) {
			case "PARAM_ERROR":
				return http.StatusBadRequest, errorResponse(errSettlementApplicationQueryWechatParamError)
			case "INVALID_REQUEST":
				return http.StatusBadRequest, errorResponse(errSettlementApplicationQueryWechatInvalidRequest)
			case "NO_AUTH":
				return http.StatusForbidden, errorResponse(ErrSettlementApplicationQueryNoAuth)
			case "SIGN_ERROR":
				return http.StatusUnauthorized, errorResponse(ErrSettlementWechatSignError)
			case "FREQENCY_LIMIT", "RATELIMIT_EXCEEDED":
				return http.StatusTooManyRequests, errorResponse(ErrSettlementApplicationQueryFrequencyLimit)
			case "SYSTEM_ERROR":
				return http.StatusServiceUnavailable, settlementPublicErrorResponse(ErrSettlementWechatServiceUnavailable)
			default:
				if wxErr.StatusCode >= http.StatusInternalServerError {
					return http.StatusServiceUnavailable, settlementPublicErrorResponse(ErrSettlementWechatServiceUnavailable)
				}
				return http.StatusBadGateway, settlementPublicErrorResponse(ErrSettlementWechatInvalidResponse)
			}
		}

		switch strings.TrimSpace(wxErr.Code) {
		case "PARAM_ERROR":
			return http.StatusBadRequest, errorResponse(errSettlementWechatParamError)
		case "INVALID_REQUEST":
			return http.StatusBadRequest, errorResponse(settlementWechatInvalidRequestError(wxErr))
		case "NO_AUTH":
			return http.StatusForbidden, errorResponse(ErrSettlementWechatNoAuth)
		case "SIGN_ERROR":
			return http.StatusUnauthorized, errorResponse(ErrSettlementWechatSignError)
		case "FREQENCY_LIMIT", "RATELIMIT_EXCEEDED":
			return http.StatusTooManyRequests, errorResponse(ErrSettlementWechatFrequencyLimit)
		case "SYSTEM_ERROR":
			return http.StatusServiceUnavailable, settlementPublicErrorResponse(ErrSettlementWechatServiceUnavailable)
		default:
			if wxErr.StatusCode >= http.StatusInternalServerError {
				return http.StatusServiceUnavailable, settlementPublicErrorResponse(ErrSettlementWechatServiceUnavailable)
			}
			return normalizedSettlementStatus(wxErr.StatusCode), errorResponse(errSettlementWechatInvalidRequest)
		}
	}

	var contractErr *wechatcontracts.SubMerchantSettlementContractError
	if errors.As(err, &contractErr) {
		log.Error().
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("operation", operation).
			Str("subject_type", subjectType).
			Int64("subject_id", subjectID).
			Str("sub_mch_id", strings.TrimSpace(subMchID)).
			Str("application_no", strings.TrimSpace(applicationNo)).
			Str("wechat_contract_error", strings.TrimSpace(contractErr.Error())).
			Msg("wechat settlement response contract validation failed")
		return http.StatusBadGateway, settlementPublicErrorResponse(ErrSettlementWechatInvalidResponse)
	}

	var applicationContractErr *wechatcontracts.SubMerchantSettlementApplicationContractError
	if errors.As(err, &applicationContractErr) {
		log.Error().
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("operation", operation).
			Str("subject_type", subjectType).
			Int64("subject_id", subjectID).
			Str("sub_mch_id", strings.TrimSpace(subMchID)).
			Str("application_no", strings.TrimSpace(applicationNo)).
			Str("wechat_contract_error", strings.TrimSpace(applicationContractErr.Error())).
			Msg("wechat settlement application response contract validation failed")
		return http.StatusBadGateway, settlementPublicErrorResponse(ErrSettlementWechatInvalidResponse)
	}

	var validationErr *wechatcontracts.SubMerchantSettlementQueryValidationError
	if errors.As(err, &validationErr) {
		log.Error().
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("operation", operation).
			Str("subject_type", subjectType).
			Int64("subject_id", subjectID).
			Str("sub_mch_id", strings.TrimSpace(subMchID)).
			Str("application_no", strings.TrimSpace(applicationNo)).
			Msg("wechat settlement request failed local validation before upstream call")
		return http.StatusServiceUnavailable, errorResponse(errSettlementQueryPreparationFailed)
	}

	var applicationValidationErr *wechatcontracts.SubMerchantSettlementApplicationQueryValidationError
	if errors.As(err, &applicationValidationErr) {
		log.Error().
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("operation", operation).
			Str("subject_type", subjectType).
			Int64("subject_id", subjectID).
			Str("sub_mch_id", strings.TrimSpace(subMchID)).
			Str("application_no", strings.TrimSpace(applicationNo)).
			Msg("wechat settlement application query failed local validation before upstream call")
		return http.StatusServiceUnavailable, errorResponse(errSettlementApplicationQueryPreparationFailed)
	}

	log.Error().
		Err(err).
		Str("request_id", GetRequestID(ctx)).
		Str("operation", operation).
		Str("subject_type", subjectType).
		Int64("subject_id", subjectID).
		Str("sub_mch_id", strings.TrimSpace(subMchID)).
		Str("application_no", strings.TrimSpace(applicationNo)).
		Msg("wechat settlement request failed")

	return http.StatusServiceUnavailable, settlementPublicErrorResponse(ErrSettlementWechatServiceUnavailable)
}

func settlementWechatInvalidRequestError(wxErr *wechat.WechatPayError) error {
	raw := strings.TrimSpace(wxErr.Detail)
	if raw == "" {
		raw = strings.TrimSpace(wxErr.Message)
	}

	switch {
	case settlementContainsAny(raw,
		"开户名称与主体名称不一致",
		"非同名银行结算账户",
		"same-name",
		"subject name",
		"account name",
	):
		return errSettlementWechatNameMismatch
	case settlementContainsAny(raw,
		"审核中",
		"继续等待审核结果",
		"无法再次提交",
		"账户尚未生效",
		"under review",
		"in review",
	):
		return errSettlementWechatApplicationPending
	case settlementContainsAny(raw,
		"每天仅能提交5次",
		"5次修改申请",
		"次日0点",
		"5 times",
		"midnight",
		"daily",
	):
		return errSettlementWechatDailyLimit
	default:
		return errSettlementWechatInvalidRequest
	}
}

func settlementContainsAny(raw string, parts ...string) bool {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if candidate == "" {
			continue
		}
		if strings.Contains(raw, candidate) || strings.Contains(normalized, strings.ToLower(candidate)) {
			return true
		}
	}
	return false
}

func normalizedSettlementStatus(status int) int {
	switch status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests:
		return status
	default:
		if status >= http.StatusInternalServerError {
			return http.StatusInternalServerError
		}
		return http.StatusBadRequest
	}
}

func settlementCommandErrorFields(err error) (*string, *string) {
	if err == nil {
		return nil, nil
	}
	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) {
		return stringPtrIfNotEmpty(strings.TrimSpace(wxErr.Code)), stringPtrIfNotEmpty(strings.TrimSpace(wxErr.Message))
	}
	return nil, stringPtrIfNotEmpty(strings.TrimSpace(err.Error()))
}
