package cloudprint

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestYilianyunPrintUsesAuthorizedTokenAndIdempotentOriginID(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/print/index", r.URL.Path)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.NoError(t, r.ParseForm())
		received = r.PostForm
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"error":             "0",
			"error_description": "success",
			"body": map[string]any{
				"id": "yly-order-123",
			},
		}))
	}))
	defer server.Close()

	client := newYilianyunClientFromConfig(util.Config{
		YilianyunEnabled:     true,
		YilianyunAPIBaseURL:  server.URL,
		YilianyunAppID:       "client-001",
		YilianyunAppSecret:   "secret-001",
		YilianyunHTTPTimeout: time.Second,
	}, fixedClock("1717555200"), fixedRequestID("3F2504E0-4F89-11D3-9A0C-0305E82C3301"))

	result, err := client.Print(context.Background(), YilianyunPrintInput{
		MachineCode:      "YL-SN-001",
		AccessToken:      "access-token-001",
		Content:          "hello\nworld",
		ProviderOriginID: "LLP9ABC123",
	})

	require.NoError(t, err)
	require.Equal(t, "yly-order-123", result.ProviderOrderID)
	require.Equal(t, "LLP9ABC123", result.ProviderOriginID)
	require.Equal(t, "client-001", received.Get("client_id"))
	require.Equal(t, "access-token-001", received.Get("access_token"))
	require.Equal(t, "1717555200", received.Get("timestamp"))
	require.Equal(t, "3F2504E0-4F89-11D3-9A0C-0305E82C3301", received.Get("id"))
	require.Equal(t, BuildYilianyunSign("client-001", "1717555200", "secret-001"), received.Get("sign"))
	require.Equal(t, "YL-SN-001", received.Get("machine_code"))
	require.Equal(t, "hello\nworld", received.Get("content"))
	require.Equal(t, "LLP9ABC123", received.Get("origin_id"))
	require.Equal(t, "1", received.Get("idempotence"))
}

func TestYilianyunQueryOrderStateAndPrinterInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		require.Equal(t, "client-001", r.PostForm.Get("client_id"))
		require.Equal(t, "access-token-001", r.PostForm.Get("access_token"))
		switch r.URL.Path {
		case "/printer/getorderstatus":
			require.Equal(t, "YL-SN-001", r.PostForm.Get("machine_code"))
			require.Equal(t, "yly-order-123", r.PostForm.Get("order_id"))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": "0",
				"body":  map[string]any{"status": "1"},
			}))
		case "/printer/getprintstatus":
			require.Equal(t, "YL-SN-001", r.PostForm.Get("machine_code"))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": "0",
				"body":  map[string]any{"state": 2},
			}))
		case "/printer/printinfo":
			require.Equal(t, "YL-SN-001", r.PostForm.Get("machine_code"))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": "0",
				"body": map[string]any{
					"version":     "K4-WH",
					"print_width": "80mm",
				},
			}))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newYilianyunClientFromConfig(util.Config{
		YilianyunEnabled:     true,
		YilianyunAPIBaseURL:  server.URL,
		YilianyunAppID:       "client-001",
		YilianyunAppSecret:   "secret-001",
		YilianyunHTTPTimeout: time.Second,
	}, fixedClock("1717555200"), fixedRequestID("request-id"))

	printed, err := client.QueryOrderState(context.Background(), YilianyunQueryOrderStateInput{
		AccessToken: "access-token-001",
		MachineCode: "YL-SN-001",
		OrderID:     "yly-order-123",
	})
	require.NoError(t, err)
	require.True(t, printed)

	status, err := client.QueryPrinterStatus(context.Background(), YilianyunPrinterStatusInput{
		AccessToken: "access-token-001",
		MachineCode: "YL-SN-001",
	})
	require.NoError(t, err)
	require.Equal(t, PrinterProviderStatusOutOfPaper, status.ProviderStatus)
	require.NotNil(t, status.Online)
	require.True(t, *status.Online)
	require.Equal(t, PrinterPaperStatusOutOfPaper, status.PaperStatus)

	info, err := client.GetPrinterInfo(context.Background(), YilianyunPrinterStatusInput{
		AccessToken: "access-token-001",
		MachineCode: "YL-SN-001",
	})
	require.NoError(t, err)
	require.Equal(t, "K4-WH", info.Model)
	require.Equal(t, "80mm", info.PrintWidth)
}

func TestYilianyunQueryPrintStatePreservesCancelledState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/printer/getorderstatus", r.URL.Path)
		require.NoError(t, r.ParseForm())
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"error": "0",
			"body":  map[string]any{"status": 2},
		}))
	}))
	defer server.Close()

	client := newYilianyunClientFromConfig(util.Config{
		YilianyunEnabled:     true,
		YilianyunAPIBaseURL:  server.URL,
		YilianyunAppID:       "client-001",
		YilianyunAppSecret:   "secret-001",
		YilianyunHTTPTimeout: time.Second,
	}, fixedClock("1717555200"), fixedRequestID("request-id"))

	state, err := client.QueryPrintState(context.Background(), YilianyunQueryOrderStateInput{
		AccessToken: "access-token-001",
		MachineCode: "YL-SN-001",
		OrderID:     "yly-order-123",
	})

	require.NoError(t, err)
	require.Equal(t, PrintStateCancelled, state.Status)
}

func TestYilianyunPrintResultCallbackEnabledRequiresConfiguredClient(t *testing.T) {
	var nilClient *YilianyunClient
	require.False(t, nilClient.PrintResultCallbackEnabled())

	client := newYilianyunClientFromConfig(util.Config{
		YilianyunEnabled:     true,
		YilianyunAPIBaseURL:  "https://open-api.10ss.net",
		YilianyunAppID:       "client-001",
		YilianyunAppSecret:   "secret-001",
		YilianyunHTTPTimeout: time.Second,
	}, fixedClock("1717555200"), fixedRequestID("request-id"))

	require.True(t, client.PrintResultCallbackEnabled())
}

func TestYilianyunAuthorizedProviderErrorsAreSafeAndFailClosed(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
		want     string
	}{
		{name: "provider error", response: map[string]any{"error": "11", "error_description": "signature failed access-token-secret"}, want: "yilianyun api error 11"},
		{name: "missing error", response: map[string]any{"body": map[string]any{"id": "1"}}, want: "missing error"},
		{name: "missing order id", response: map[string]any{"error": "0", "body": map[string]any{}}, want: "missing print order id"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, json.NewEncoder(w).Encode(tc.response))
			}))
			defer server.Close()

			client := newYilianyunClientFromConfig(util.Config{
				YilianyunEnabled:     true,
				YilianyunAPIBaseURL:  server.URL,
				YilianyunAppID:       "client-001",
				YilianyunAppSecret:   "secret-001",
				YilianyunHTTPTimeout: time.Second,
			}, fixedClock("1717555200"), fixedRequestID("request-id"))

			_, err := client.Print(context.Background(), YilianyunPrintInput{
				MachineCode:      "YL-SN-001",
				AccessToken:      "access-token-secret",
				Content:          "hello",
				ProviderOriginID: "LLP9ABC123",
			})

			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
			require.NotContains(t, err.Error(), "access-token-secret")
		})
	}
}

func TestYilianyunRejectsUnsupportedRemoteBindBeforeCallingProvider(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.Error(w, "should not call provider", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newYilianyunClientFromConfig(util.Config{
		YilianyunEnabled:     true,
		YilianyunAPIBaseURL:  server.URL,
		YilianyunAppID:       "client-001",
		YilianyunAppSecret:   "secret-001",
		YilianyunHTTPTimeout: time.Second,
	}, fixedClock("1717555200"), fixedRequestID("request-id"))

	err := client.AddPrinter(context.Background(), AddPrinterInput{SN: "YL-SN-001"})

	require.True(t, errors.Is(err, ErrUnsupportedCapability))
	require.False(t, called)
}
