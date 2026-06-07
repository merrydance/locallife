package logic

import (
	"errors"
	"net/http"
	"strings"

	"github.com/merrydance/locallife/baofu"
)

type BaofuProviderErrorContext struct {
	FlowID             int64
	OwnerType          string
	OwnerID            int64
	OpenTransSerialNo  string
	CurrentState       string
	MerchantReportID   int64
	MerchantReportNo   string
	ProviderOperation  string
	ProviderCapability string
}

type baofuProviderContextError struct {
	err     error
	context BaofuProviderErrorContext
}

func NewBaofuProviderContextError(err error, context BaofuProviderErrorContext) error {
	if err == nil {
		return nil
	}
	return &baofuProviderContextError{err: err, context: context.normalized()}
}

func (e *baofuProviderContextError) Error() string {
	if e == nil || e.err == nil {
		return "baofu provider context error"
	}
	return e.err.Error()
}

func (e *baofuProviderContextError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func BaofuProviderErrorContextFromError(err error) (BaofuProviderErrorContext, bool) {
	var contextErr *baofuProviderContextError
	if !errors.As(err, &contextErr) || contextErr == nil {
		return BaofuProviderErrorContext{}, false
	}
	return contextErr.context, true
}

func (c BaofuProviderErrorContext) normalized() BaofuProviderErrorContext {
	c.OwnerType = strings.TrimSpace(c.OwnerType)
	c.OpenTransSerialNo = strings.TrimSpace(c.OpenTransSerialNo)
	c.CurrentState = strings.TrimSpace(c.CurrentState)
	c.MerchantReportNo = strings.TrimSpace(c.MerchantReportNo)
	c.ProviderOperation = strings.TrimSpace(c.ProviderOperation)
	c.ProviderCapability = strings.TrimSpace(c.ProviderCapability)
	return c
}

func mapBaofuAccountOpenError(err error) error {
	if err == nil {
		return nil
	}
	if message := strings.ToLower(err.Error()); strings.Contains(message, "baofu account") && strings.Contains(message, "not configured") {
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("宝付开户服务未配置，请联系平台处理"), err)
	}
	var providerErr *baofu.ProviderError
	if !errors.As(err, &providerErr) {
		return err
	}
	classified := baofu.ClassifyBaofuError(providerErr.UpstreamCode, providerErr.UpstreamMessage)
	if strings.TrimSpace(providerErr.Operation) == "T-1001-013-03" &&
		strings.EqualFold(strings.TrimSpace(providerErr.UpstreamCode), "BF00064") {
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("宝付开户结果暂未查询到，系统会继续确认，请稍后刷新"), err)
	}
	status := http.StatusBadGateway
	switch classified.Category {
	case baofu.BaofuErrorCategoryUserActionRequired:
		status = http.StatusBadRequest
	case baofu.BaofuErrorCategoryPlatformConfiguration,
		baofu.BaofuErrorCategoryRetryable:
		status = http.StatusServiceUnavailable
	case baofu.BaofuErrorCategoryManualReview:
		status = http.StatusBadGateway
	}
	return NewRequestErrorWithCause(status, errors.New(classified.PublicMessage), err)
}

func mapBaofuMerchantReportError(err error, context BaofuProviderErrorContext) error {
	if err == nil {
		return nil
	}
	context = context.normalized()
	if message := strings.ToLower(err.Error()); strings.Contains(message, "baofu merchant report") && strings.Contains(message, "not configured") {
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("宝付商户报备服务未配置，请联系平台处理"), NewBaofuProviderContextError(err, context))
	}
	var providerErr *baofu.ProviderError
	if !errors.As(err, &providerErr) {
		return err
	}
	status := http.StatusBadGateway
	classified := baofu.ClassifyBaofuErrorForOperation(providerErr.Operation, providerErr.UpstreamCode, providerErr.UpstreamMessage)
	switch classified.Category {
	case baofu.BaofuErrorCategoryUserActionRequired:
		status = http.StatusBadRequest
	case baofu.BaofuErrorCategoryPlatformConfiguration,
		baofu.BaofuErrorCategoryRetryable:
		status = http.StatusServiceUnavailable
	case baofu.BaofuErrorCategoryManualReview:
		status = http.StatusBadGateway
	}
	return NewRequestErrorWithCause(status, errors.New(baofuMerchantReportPublicMessage(context, providerErr)), NewBaofuProviderContextError(err, context))
}

func baofuMerchantReportPublicMessage(context BaofuProviderErrorContext, providerErr *baofu.ProviderError) string {
	operation := strings.TrimSpace(context.ProviderOperation)
	if operation == "" && providerErr != nil {
		operation = strings.TrimSpace(providerErr.Operation)
	}
	switch operation {
	case "bind_sub_config":
		return "微信支付授权目录绑定失败，请联系平台处理后重试"
	case "merchant_report":
		if providerErr != nil {
			classified := baofu.ClassifyBaofuErrorForOperation(operation, providerErr.UpstreamCode, providerErr.UpstreamMessage)
			if classified.Category == baofu.BaofuErrorCategoryUserActionRequired && strings.TrimSpace(classified.PublicMessage) != "" {
				return "微信支付商户报备失败，" + strings.TrimSpace(classified.PublicMessage)
			}
		}
		return "微信支付商户报备失败，请核对商户资料后重试；如持续失败请联系平台处理"
	default:
		return "微信支付商户报备失败，请核对商户资料后重试；如持续失败请联系平台处理"
	}
}

func mapBaofuPaymentCreateError(err error) error {
	if err == nil {
		return nil
	}
	if message := strings.ToLower(err.Error()); strings.Contains(message, "baofu") && strings.Contains(message, "not configured") {
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("宝付支付通道未配置，请联系平台处理"), err)
	}
	var providerErr *baofu.ProviderError
	if !errors.As(err, &providerErr) {
		return err
	}
	classified := baofu.ClassifyBaofuErrorForOperation(providerErr.Operation, providerErr.UpstreamCode, providerErr.UpstreamMessage)
	status := http.StatusBadGateway
	switch classified.Category {
	case baofu.BaofuErrorCategoryUserActionRequired:
		status = http.StatusBadRequest
	case baofu.BaofuErrorCategoryPlatformConfiguration,
		baofu.BaofuErrorCategoryRetryable:
		status = http.StatusServiceUnavailable
	case baofu.BaofuErrorCategoryManualReview:
		status = http.StatusBadGateway
	}
	return NewRequestErrorWithCause(status, errors.New(classified.PublicMessage), err)
}
