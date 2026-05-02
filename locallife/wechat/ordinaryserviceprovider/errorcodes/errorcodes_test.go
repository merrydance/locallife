package errorcodes

import (
	"strings"
	"testing"
)

func TestClassifyProviderCodePreservesFrontendGuidance(t *testing.T) {
	tests := []struct {
		code         string
		statusCode   int
		wantCategory Category
		wantFrontend string
		wantRetry    bool
	}{
		{code: "NO_AUTH", statusCode: 403, wantCategory: CategoryAuthConfig, wantFrontend: "WECHAT_AUTH_CONFIG_REQUIRED"},
		{code: "SYSTEM_ERROR", statusCode: 500, wantCategory: CategoryRetryableProvider, wantFrontend: "WECHAT_PROVIDER_RETRYABLE", wantRetry: true},
		{code: "FREQENCY_LIMIT", statusCode: 429, wantCategory: CategoryRetryableProvider, wantFrontend: "WECHAT_PROVIDER_RETRYABLE", wantRetry: true},
		{code: "FREQUENCY_LIMIT_EXCEED", statusCode: 429, wantCategory: CategoryRetryableProvider, wantFrontend: "WECHAT_PROVIDER_RETRYABLE", wantRetry: true},
		{code: "NOT_ENOUGH", statusCode: 403, wantCategory: CategoryBusinessConflict, wantFrontend: "WECHAT_BUSINESS_CONFLICT"},
		{code: "APPLYMENT_NOT_EXIST", statusCode: 400, wantCategory: CategoryBusinessConflict, wantFrontend: "WECHAT_BUSINESS_CONFLICT"},
		{code: "ALREADY_EXISTS", statusCode: 409, wantCategory: CategoryBusinessConflict, wantFrontend: "WECHAT_BUSINESS_CONFLICT"},
		{code: "OPENID_MISMATCH", statusCode: 400, wantCategory: CategoryBusinessConflict, wantFrontend: "WECHAT_BUSINESS_CONFLICT"},
		{code: "REQUEST_BLOCKED", statusCode: 403, wantCategory: CategoryMerchantControl, wantFrontend: "WECHAT_MERCHANT_CONTROLLED"},
	}

	for _, tt := range tests {
		got := Classify(tt.code, tt.statusCode)
		if got.Category != tt.wantCategory {
			t.Fatalf("Classify(%q) category = %s, want %s", tt.code, got.Category, tt.wantCategory)
		}
		if got.FrontendCode != tt.wantFrontend {
			t.Fatalf("Classify(%q) frontend code = %s, want %s", tt.code, got.FrontendCode, tt.wantFrontend)
		}
		if got.Retryable != tt.wantRetry {
			t.Fatalf("Classify(%q) retryable = %v, want %v", tt.code, got.Retryable, tt.wantRetry)
		}
		if got.FrontendMessage == "" || got.OperatorAction == "" {
			t.Fatalf("Classify(%q) must include frontend message and operator action: %+v", tt.code, got)
		}
	}
}

func TestClassifyCoversOfficialOrdinaryServiceProviderErrorCodes(t *testing.T) {
	// Sourced from .github/standards/domains/wechat-payment/README.md 4.10 official endpoint docs.
	officialCodes := []string{
		"ALREADY_EXISTS",
		"APPID_MCHID_NOT_MATCH",
		"APPLYMENT_NOTEXIST",
		"APPLYMENT_NOT_EXIST",
		"FREQENCY_LIMIT",
		"FREQUENCY_LIMITED",
		"FREQUENCY_LIMIT_EXCEED",
		"INVALID_REQUEST",
		"MCH_NOT_EXISTS",
		"NOAUTH",
		"NOT_ENOUGH",
		"NOT_FOUND",
		"NO_AUTH",
		"OPENID_MISMATCH",
		"ORDER_CLOSED",
		"ORDER_NOT_EXIST",
		"OUT_TRADE_NO_USED",
		"PARAM_ERROR",
		"PROCESSING",
		"RATELIMIT_EXCEEDED",
		"RATE_LIMITED",
		"REQUEST_BLOCKED",
		"RESOURCE_NOT_EXISTS",
		"RULELIMIT",
		"RULE_LIMIT",
		"SIGN_ERROR",
		"SYSTEMERROR",
		"SYSTEM_ERROR",
		"TRADE_ERROR",
		"USER_ACCOUNT_ABNORMAL",
	}

	for _, code := range officialCodes {
		got := Classify(code, 400)
		if got.ProviderCode != code {
			t.Fatalf("Classify(%q) provider code = %q", code, got.ProviderCode)
		}
		if got.FrontendCode == "" || got.FrontendMessage == "" || got.OperatorAction == "" {
			t.Fatalf("Classify(%q) must return actionable frontend/operator guidance: %+v", code, got)
		}
	}
}

func TestClassifyUnknownServerStatusIsRetryable(t *testing.T) {
	got := Classify("", 503)
	if got.Category != CategoryRetryableProvider {
		t.Fatalf("server status category = %s, want %s", got.Category, CategoryRetryableProvider)
	}
	if !got.Retryable {
		t.Fatal("server status should be retryable")
	}
}

func TestMerchantControlGuidancePointsToOrdinaryDiagnostic(t *testing.T) {
	got := Classify("REQUEST_BLOCKED", 403)
	if !strings.Contains(got.OperatorAction, "平台财务-普通服务商商户管控诊断") {
		t.Fatalf("merchant-control operator action should point frontend/operator to ordinary diagnostic, got %q", got.OperatorAction)
	}
}

func TestDefaultProviderGuidanceDoesNotExposeDiagnosticTokens(t *testing.T) {
	got := Classify("UNEXPECTED_PROVIDER_CODE", 400)
	if strings.Contains(got.OperatorAction, "request_id") || strings.Contains(got.OperatorAction, "provider_code") {
		t.Fatalf("default operator action must not expose provider diagnostic tokens to frontend copy, got %q", got.OperatorAction)
	}
	if !strings.Contains(got.OperatorAction, "微信支付服务日志") {
		t.Fatalf("default operator action should direct operators to service logs, got %q", got.OperatorAction)
	}
}
