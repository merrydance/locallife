package logic

import (
	"errors"
	"net/http"
	"strings"

	"github.com/merrydance/locallife/baofu"
)

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
	classified := baofu.ClassifyBaofuError(providerErr.UpstreamCode, providerErr.UpstreamMessage)
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
