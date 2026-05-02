package ordinaryserviceprovider

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/merrydance/locallife/wechat/ordinaryserviceprovider/errorcodes"
	"github.com/rs/zerolog"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
)

type ErrorCategory = errorcodes.Category

const (
	ErrorCategoryRetryableProvider ErrorCategory = errorcodes.CategoryRetryableProvider
	ErrorCategoryBusinessConflict  ErrorCategory = errorcodes.CategoryBusinessConflict
	ErrorCategoryMerchantControl   ErrorCategory = errorcodes.CategoryMerchantControl
	ErrorCategoryAuthConfig        ErrorCategory = errorcodes.CategoryAuthConfig
	ErrorCategoryValidation        ErrorCategory = errorcodes.CategoryValidation
	ErrorCategoryProvider          ErrorCategory = errorcodes.CategoryProvider
)

type FrontendGuidance struct {
	Code      string
	Message   string
	Action    string
	Retryable bool
}

type ProviderError struct {
	Operation       string
	StatusCode      int
	RequestID       string
	ProviderCode    string
	ProviderMessage string
	Category        ErrorCategory
	Frontend        FrontendGuidance
	cause           error
}

func (e *ProviderError) Error() string {
	if e == nil {
		return "ordinary service provider error"
	}
	return fmt.Sprintf("ordinary service provider %s failed: category=%s provider_code=%s status=%d request_id=%s",
		e.Operation, e.Category, e.ProviderCode, e.StatusCode, e.RequestID)
}

func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

type ErrorLogContext struct {
	SubMchID    string
	OutTradeNo  string
	OutRefundNo string
	CommandID   string
	FactID      string
}

func mapSDKAPIError(operation string, err error) error {
	if err == nil {
		return nil
	}

	var apiErr *core.APIError
	if errors.As(err, &apiErr) {
		metadata := errorcodes.Classify(apiErr.Code, apiErr.StatusCode)
		return &ProviderError{
			Operation:       strings.TrimSpace(operation),
			StatusCode:      apiErr.StatusCode,
			RequestID:       requestIDFromHeader(apiErr.Header),
			ProviderCode:    metadata.ProviderCode,
			ProviderMessage: strings.TrimSpace(apiErr.Message),
			Category:        metadata.Category,
			Frontend:        frontendGuidanceFromMetadata(metadata),
			cause:           err,
		}
	}

	return &ProviderError{
		Operation: strings.TrimSpace(operation),
		Category:  ErrorCategoryRetryableProvider,
		Frontend:  frontendGuidanceForCategory(ErrorCategoryRetryableProvider),
		cause:     err,
	}
}

func LogProviderError(logger zerolog.Logger, err error, context ErrorLogContext) {
	if err == nil {
		return
	}

	var providerErr *ProviderError
	event := logger.Error().Err(err).Str("payment_channel", "ordinary_service_provider")
	if errors.As(err, &providerErr) {
		event = event.
			Str("wechat_operation", providerErr.Operation).
			Str("request_id", providerErr.RequestID).
			Str("wechat_code", providerErr.ProviderCode).
			Str("wechat_message", providerErr.ProviderMessage).
			Str("error_category", string(providerErr.Category)).
			Str("frontend_code", providerErr.Frontend.Code).
			Bool("retryable", providerErr.Frontend.Retryable)
		if providerErr.StatusCode != 0 {
			event = event.Int("status_code", providerErr.StatusCode)
		}
	}
	if strings.TrimSpace(context.SubMchID) != "" {
		event = event.Str("sub_mchid", strings.TrimSpace(context.SubMchID))
	}
	if strings.TrimSpace(context.OutTradeNo) != "" {
		event = event.Str("out_trade_no", strings.TrimSpace(context.OutTradeNo))
	}
	if strings.TrimSpace(context.OutRefundNo) != "" {
		event = event.Str("out_refund_no", strings.TrimSpace(context.OutRefundNo))
	}
	if strings.TrimSpace(context.CommandID) != "" {
		event = event.Str("command_id", strings.TrimSpace(context.CommandID))
	}
	if strings.TrimSpace(context.FactID) != "" {
		event = event.Str("fact_id", strings.TrimSpace(context.FactID))
	}
	event.Msg("wechat ordinary service provider request failed")
}

func frontendGuidanceForCategory(category ErrorCategory) FrontendGuidance {
	return frontendGuidanceFromMetadata(errorcodes.GuidanceForCategory(category))
}

func frontendGuidanceFromMetadata(metadata errorcodes.Metadata) FrontendGuidance {
	return FrontendGuidance{
		Code:      metadata.FrontendCode,
		Message:   metadata.FrontendMessage,
		Action:    metadata.OperatorAction,
		Retryable: metadata.Retryable,
	}
}

func requestIDFromHeader(header http.Header) string {
	for _, key := range []string{"Request-ID", "Request-Id", "Wechatpay-Request-Id"} {
		if value := strings.TrimSpace(header.Get(key)); value != "" {
			return value
		}
	}
	return ""
}
