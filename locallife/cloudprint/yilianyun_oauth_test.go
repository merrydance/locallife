package cloudprint

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestBuildYilianyunSignUsesOfficialLowercaseMD5Rule(t *testing.T) {
	sign := BuildYilianyunSign("client-001", "1717555200", "secret-001")

	require.Equal(t, "05984f4692f61921ef5d83e7d8cc65a9", sign)
	require.Equal(t, strings.ToLower(sign), sign)
}

func TestYilianyunAuthorizeURLUsesOfficialOpenAppFields(t *testing.T) {
	client := newYilianyunOAuthClientFromConfig(util.Config{
		YilianyunEnabled:          true,
		YilianyunAPIBaseURL:       "https://open-api.10ss.net/",
		YilianyunAppID:            "client-001",
		YilianyunAppSecret:        "secret-001",
		YilianyunHTTPTimeout:      time.Second,
		YilianyunAuthCallbackURL:  "https://api.example.com/v1/cloud-printer/yilianyun/auth/callback?source=merchant",
		YilianyunPrintCallbackURL: "https://api.example.com/v1/webhooks/yilianyun/print-result",
	}, nil, nil)

	authorizeURL, err := client.BuildAuthorizeURL("state-opaque-001")

	require.NoError(t, err)
	parsed, err := url.Parse(authorizeURL)
	require.NoError(t, err)
	require.Equal(t, "https", parsed.Scheme)
	require.Equal(t, "open-api.10ss.net", parsed.Host)
	require.Equal(t, "/oauth/authorize", parsed.Path)
	require.Equal(t, "code", parsed.Query().Get("response_type"))
	require.Equal(t, "client-001", parsed.Query().Get("client_id"))
	require.Equal(t, "https://api.example.com/v1/cloud-printer/yilianyun/auth/callback?source=merchant", parsed.Query().Get("redirect_uri"))
	require.Equal(t, "state-opaque-001", parsed.Query().Get("state"))
}

func TestYilianyunExchangeAuthorizationCodePostsOfficialFields(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/oauth/oauth", r.URL.Path)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.NoError(t, r.ParseForm())
		received = r.PostForm
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"error":             "0",
			"error_description": "success",
			"body": map[string]any{
				"access_token":  "access-token-001",
				"refresh_token": "refresh-token-001",
				"machine_code":  "YL-SN-001",
				"expires_in":    2592000,
				"scope":         "all",
			},
		}))
	}))
	defer server.Close()

	client := newYilianyunOAuthClientFromConfig(util.Config{
		YilianyunEnabled:     true,
		YilianyunAPIBaseURL:  server.URL,
		YilianyunAppID:       "client-001",
		YilianyunAppSecret:   "secret-001",
		YilianyunHTTPTimeout: time.Second,
	}, fixedClock("1717555200"), fixedRequestID("3F2504E0-4F89-11D3-9A0C-0305E82C3301"))

	token, err := client.ExchangeAuthorizationCode(context.Background(), "auth-code-001")

	require.NoError(t, err)
	require.Equal(t, "access-token-001", token.AccessToken)
	require.Equal(t, "refresh-token-001", token.RefreshToken)
	require.Equal(t, "YL-SN-001", token.MachineCode)
	require.Equal(t, int64(2592000), token.ExpiresInSeconds)
	require.Equal(t, "client-001", received.Get("client_id"))
	require.Equal(t, "authorization_code", received.Get("grant_type"))
	require.Equal(t, "auth-code-001", received.Get("code"))
	require.Equal(t, "all", received.Get("scope"))
	require.Equal(t, "1717555200", received.Get("timestamp"))
	require.Equal(t, "3F2504E0-4F89-11D3-9A0C-0305E82C3301", received.Get("id"))
	require.Equal(t, BuildYilianyunSign("client-001", "1717555200", "secret-001"), received.Get("sign"))
	require.Empty(t, received.Get("access_token"))
	require.Empty(t, received.Get("refresh_token"))
}

func TestYilianyunAuthorizeScannedPrinterRequiresExactlyOneCredential(t *testing.T) {
	client := newYilianyunOAuthClientFromConfig(util.Config{
		YilianyunEnabled:     true,
		YilianyunAPIBaseURL:  "https://open-api.10ss.net",
		YilianyunAppID:       "client-001",
		YilianyunAppSecret:   "secret-001",
		YilianyunHTTPTimeout: time.Second,
	}, fixedClock("1717555200"), fixedRequestID("request-id"))

	_, err := client.AuthorizeScannedPrinter(context.Background(), YilianyunScannedPrinterAuthorizationInput{
		MachineCode: "YL-SN-001",
	})
	require.ErrorContains(t, err, "exactly one of qr_key or msign is required")

	_, err = client.AuthorizeScannedPrinter(context.Background(), YilianyunScannedPrinterAuthorizationInput{
		MachineCode: "YL-SN-001",
		QRKey:       "qr-key-001",
		MSign:       "msign-001",
	})
	require.ErrorContains(t, err, "exactly one of qr_key or msign is required")
}

func TestYilianyunAuthorizeScannedPrinterPostsOfficialFields(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/oauth/scancodemodel", r.URL.Path)
		require.NoError(t, r.ParseForm())
		received = r.PostForm
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"error":             "0",
			"error_description": "success",
			"body": map[string]any{
				"access_token":  "access-token-002",
				"refresh_token": "refresh-token-002",
				"expires_in":    "2592000",
				"scope":         "all",
			},
		}))
	}))
	defer server.Close()

	client := newYilianyunOAuthClientFromConfig(util.Config{
		YilianyunEnabled:     true,
		YilianyunAPIBaseURL:  server.URL,
		YilianyunAppID:       "client-001",
		YilianyunAppSecret:   "secret-001",
		YilianyunHTTPTimeout: time.Second,
	}, fixedClock("1717555200"), fixedRequestID("3F2504E0-4F89-11D3-9A0C-0305E82C3301"))

	token, err := client.AuthorizeScannedPrinter(context.Background(), YilianyunScannedPrinterAuthorizationInput{
		MachineCode: "YL-SN-002",
		QRKey:       "qr-key-002",
	})

	require.NoError(t, err)
	require.Equal(t, "access-token-002", token.AccessToken)
	require.Equal(t, "refresh-token-002", token.RefreshToken)
	require.Equal(t, "YL-SN-002", token.MachineCode)
	require.Equal(t, int64(2592000), token.ExpiresInSeconds)
	require.Equal(t, "client-001", received.Get("client_id"))
	require.Equal(t, "YL-SN-002", received.Get("machine_code"))
	require.Equal(t, "qr-key-002", received.Get("qr_key"))
	require.Empty(t, received.Get("msign"))
	require.Equal(t, "all", received.Get("scope"))
	require.Equal(t, "1717555200", received.Get("timestamp"))
	require.Equal(t, "3F2504E0-4F89-11D3-9A0C-0305E82C3301", received.Get("id"))
	require.Equal(t, BuildYilianyunSign("client-001", "1717555200", "secret-001"), received.Get("sign"))
}

func TestYilianyunProviderErrorsAreMappedWithoutTokenLeak(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"error":             "11",
			"error_description": "sign verify failed with access-token-should-not-leak",
		}))
	}))
	defer server.Close()

	client := newYilianyunOAuthClientFromConfig(util.Config{
		YilianyunEnabled:     true,
		YilianyunAPIBaseURL:  server.URL,
		YilianyunAppID:       "client-001",
		YilianyunAppSecret:   "secret-001",
		YilianyunHTTPTimeout: time.Second,
	}, fixedClock("1717555200"), fixedRequestID("request-id"))

	_, err := client.ExchangeAuthorizationCode(context.Background(), "auth-code-001")

	require.Error(t, err)
	require.Contains(t, err.Error(), "yilianyun api error 11")
	require.NotContains(t, err.Error(), "access-token-should-not-leak")
}

func TestYilianyunHTTPErrorDoesNotExposeResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "access-token-should-not-leak", http.StatusBadGateway)
	}))
	defer server.Close()

	client := newYilianyunOAuthClientFromConfig(util.Config{
		YilianyunEnabled:     true,
		YilianyunAPIBaseURL:  server.URL,
		YilianyunAppID:       "client-001",
		YilianyunAppSecret:   "secret-001",
		YilianyunHTTPTimeout: time.Second,
	}, fixedClock("1717555200"), fixedRequestID("request-id"))

	_, err := client.ExchangeAuthorizationCode(context.Background(), "auth-code-001")

	require.Error(t, err)
	require.Contains(t, err.Error(), "yilianyun oauth http status 502")
	require.NotContains(t, err.Error(), "access-token-should-not-leak")
}

func TestYilianyunMalformedRequiredTokenFieldsFailClosed(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
		want     string
	}{
		{name: "missing error", response: map[string]any{"body": map[string]any{}}, want: "missing error"},
		{name: "missing access token", response: map[string]any{"error": "0", "body": map[string]any{"refresh_token": "refresh-token", "machine_code": "YL-SN-001", "expires_in": 1}}, want: "missing access_token"},
		{name: "missing refresh token", response: map[string]any{"error": "0", "body": map[string]any{"access_token": "access-token", "machine_code": "YL-SN-001", "expires_in": 1}}, want: "missing refresh_token"},
		{name: "missing machine code", response: map[string]any{"error": "0", "body": map[string]any{"access_token": "access-token", "refresh_token": "refresh-token", "expires_in": 1}}, want: "missing machine_code"},
		{name: "missing expires in", response: map[string]any{"error": "0", "body": map[string]any{"access_token": "access-token", "refresh_token": "refresh-token", "machine_code": "YL-SN-001"}}, want: "missing expires_in"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, json.NewEncoder(w).Encode(tc.response))
			}))
			defer server.Close()

			client := newYilianyunOAuthClientFromConfig(util.Config{
				YilianyunEnabled:     true,
				YilianyunAPIBaseURL:  server.URL,
				YilianyunAppID:       "client-001",
				YilianyunAppSecret:   "secret-001",
				YilianyunHTTPTimeout: time.Second,
			}, fixedClock("1717555200"), fixedRequestID("request-id"))

			_, err := client.ExchangeAuthorizationCode(context.Background(), "auth-code-001")

			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func fixedRequestID(value string) func() string {
	return func() string { return value }
}
