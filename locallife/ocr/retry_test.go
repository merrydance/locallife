package ocr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/merrydance/locallife/wechat"
)

type stubTimeoutError struct{}

func (stubTimeoutError) Error() string   { return "timeout" }
func (stubTimeoutError) Timeout() bool   { return true }
func (stubTimeoutError) Temporary() bool { return true }

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "context canceled", err: context.Canceled, want: false},
		{name: "aliyun rate limited", err: fmt.Errorf("wrap: %w", ErrAliyunOCRRateLimited), want: true},
		{name: "aliyun unavailable", err: fmt.Errorf("wrap: %w", ErrAliyunOCRUnavailable), want: true},
		{name: "aliyun forbidden", err: fmt.Errorf("wrap: %w", ErrAliyunOCRForbidden), want: false},
		{name: "image too large", err: fmt.Errorf("wrap: %w", wechat.ErrImageTooLarge), want: false},
		{name: "file missing", err: fmt.Errorf("wrap: %w", os.ErrNotExist), want: false},
		{name: "network timeout", err: stubTimeoutError{}, want: true},
		{name: "wechat 45009", err: errors.New("wechat ocr failed: code=45009"), want: true},
		{name: "wechat 48001", err: errors.New("wechat ocr failed: code=48001"), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsRetryableError(tc.err); got != tc.want {
				t.Fatalf("IsRetryableError() = %v, want %v", got, tc.want)
			}
		})
	}

	var _ net.Error = stubTimeoutError{}
	_ = time.Second
}

func TestErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "rate limited", err: ErrAliyunOCRRateLimited, want: "ocr_rate_limited"},
		{name: "unavailable", err: ErrAliyunOCRUnavailable, want: "ocr_provider_unavailable"},
		{name: "unauthorized", err: ErrAliyunOCRUnauthorized, want: "ocr_provider_unauthorized"},
		{name: "forbidden", err: ErrAliyunOCRForbidden, want: "ocr_provider_forbidden"},
		{name: "bad request", err: ErrAliyunOCRBadRequest, want: "ocr_bad_request"},
		{name: "media missing", err: os.ErrNotExist, want: "ocr_media_not_found"},
		{name: "fallback", err: errors.New("boom"), want: "ocr_execution_failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := errorCode(tc.err); got != tc.want {
				t.Fatalf("errorCode() = %s, want %s", got, tc.want)
			}
		})
	}
}
