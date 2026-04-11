package ocr

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"

	"github.com/merrydance/locallife/wechat"
)

// IsRetryableError returns whether an OCR execution error should be retried automatically.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, wechat.ErrImageTooLarge) {
		return false
	}
	if errors.Is(err, ErrAliyunOCRUnauthorized) || errors.Is(err, ErrAliyunOCRForbidden) || errors.Is(err, ErrAliyunOCRBadRequest) || errors.Is(err, ErrAliyunOCRSigning) {
		return false
	}
	if errors.Is(err, ErrAliyunOCRRateLimited) || errors.Is(err, ErrAliyunOCRUnavailable) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	errText := strings.ToLower(err.Error())
	if strings.Contains(errText, "45009") || strings.Contains(errText, "rate limit") || strings.Contains(errText, "throttl") || strings.Contains(errText, "temporarily unavailable") || strings.Contains(errText, "timeout") || strings.Contains(errText, "connection reset") || strings.Contains(errText, "broken pipe") || strings.Contains(errText, "eof") {
		return true
	}
	if strings.Contains(errText, "48001") || strings.Contains(errText, "forbidden") || strings.Contains(errText, "permission") || strings.Contains(errText, "invalid image") || strings.Contains(errText, "img format") || strings.Contains(errText, "too large") {
		return false
	}
	return false
}

func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrAliyunOCRRateLimited):
		return "ocr_rate_limited"
	case errors.Is(err, ErrAliyunOCRUnavailable), errors.Is(err, context.DeadlineExceeded):
		return "ocr_provider_unavailable"
	case errors.Is(err, ErrAliyunOCRUnauthorized):
		return "ocr_provider_unauthorized"
	case errors.Is(err, ErrAliyunOCRForbidden):
		return "ocr_provider_forbidden"
	case errors.Is(err, ErrAliyunOCRBadRequest), errors.Is(err, wechat.ErrImageTooLarge):
		return "ocr_bad_request"
	case errors.Is(err, os.ErrNotExist):
		return "ocr_media_not_found"
	case IsRetryableError(err):
		return "ocr_retryable_error"
	default:
		return "ocr_execution_failed"
	}
}

func errorCode(err error) string {
	return ErrorCode(err)
}
