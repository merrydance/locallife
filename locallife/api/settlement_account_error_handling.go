package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

var (
	errSettlementMerchantApplymentNotFound            = errors.New("merchant onboarding record not found: complete the WeChat applyment before modifying the settlement account")
	errSettlementBankBranchRequired                   = errors.New("bank_branch_id or bank_name is required for the selected bank")
	errSettlementWechatParamError                     = errors.New("WeChat rejected the settlement account information: check the account type, bank, branch, account number, and account name before retrying")
	errSettlementWechatInvalidRequest                 = errors.New("WeChat rejected the settlement account change request: check the current application status, bank information, and daily submission limit before retrying")
	errSettlementApplicationQueryWechatParamError     = errors.New("WeChat rejected the settlement application query: check sub_mchid, application_no, and account_number_rule before retrying")
	errSettlementApplicationQueryWechatInvalidRequest = errors.New("WeChat rejected the settlement application query: verify the request URL, merchant configuration, and signing input before retrying")
	errSettlementWechatApplicationPending             = errors.New("a settlement account modification is still under review: wait for the current application result before submitting another change")
	errSettlementWechatDailyLimit                     = errors.New("WeChat only allows 5 settlement account modification requests per sub-merchant per day: retry after midnight")
	errSettlementWechatNameMismatch                   = errors.New("account name must match the merchant subject name; for a non-matching settlement account, submit the dedicated WeChat non-matching settlement account flow")
)

func respondSettlementClientError(ctx *gin.Context, status int, operation string, subjectType string, subjectID int64, subMchID string, applicationNo string, err error) {
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

	ctx.JSON(status, errorResponse(err))
}

func settlementWechatErrorResponse(ctx *gin.Context, operation string, subjectType string, subjectID int64, subMchID string, applicationNo string, err error) (int, ErrorResponse) {
	_ = ctx.Error(err)

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
				return http.StatusBadGateway, errorResponse(ErrSettlementWechatInvalidResponse)
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
				return http.StatusInternalServerError, errorResponse(ErrSettlementWechatServiceUnavailable)
			default:
				if wxErr.StatusCode >= http.StatusInternalServerError {
					return http.StatusInternalServerError, errorResponse(ErrSettlementWechatServiceUnavailable)
				}
				return http.StatusBadGateway, errorResponse(ErrSettlementWechatInvalidResponse)
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
			return http.StatusInternalServerError, errorResponse(ErrSettlementWechatServiceUnavailable)
		default:
			if wxErr.StatusCode >= http.StatusInternalServerError {
				return http.StatusInternalServerError, errorResponse(ErrSettlementWechatServiceUnavailable)
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
		return http.StatusBadGateway, errorResponse(ErrSettlementWechatInvalidResponse)
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
		return http.StatusBadGateway, errorResponse(ErrSettlementWechatInvalidResponse)
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
		return http.StatusInternalServerError, ErrorResponse{Error: "internal server error"}
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
		return http.StatusInternalServerError, ErrorResponse{Error: "internal server error"}
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

	return http.StatusInternalServerError, errorResponse(ErrSettlementWechatServiceUnavailable)
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
