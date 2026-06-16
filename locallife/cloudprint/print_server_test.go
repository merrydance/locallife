package cloudprint

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestPrintServerClientSignsJSONPrintJobRequest(t *testing.T) {
	fixedNow := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/print-jobs", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "local-life", r.Header.Get("X-Print-App-Id"))
		require.Equal(t, fixedNow.Format(time.RFC3339), r.Header.Get("X-Print-Timestamp"))
		require.Equal(t, "nonce-001", r.Header.Get("X-Print-Nonce"))

		var bodyBytes []byte
		bodyBytes, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		expectedSignature := BuildPrintServerSignature(http.MethodPost, "/v1/print-jobs", fixedNow.Format(time.RFC3339), "nonce-001", bodyBytes, "secret-001")
		require.Equal(t, expectedSignature, r.Header.Get("X-Print-Signature"))
		require.NoError(t, json.Unmarshal(bodyBytes, &received))

		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"print_job_id": "psj_000001",
			"printer_sn":   "MDP000001",
			"task_key":     "print_log:456",
			"status":       "queued",
			"copies":       1,
			"metadata":     received["metadata"],
			"accepted_at":  fixedNow,
			"queued_at":    fixedNow,
			"expires_at":   fixedNow.Add(15 * time.Minute),
		}))
	}))
	defer server.Close()

	client := newPrintServerClientFromConfig(util.Config{
		PrintServerEnabled:     true,
		PrintServerAPIBaseURL:  server.URL,
		PrintServerAppID:       "local-life",
		PrintServerSecret:      "secret-001",
		PrintServerHTTPTimeout: time.Second,
	}, func() time.Time { return fixedNow }, func() string { return "nonce-001" })

	orderID, err := client.Print(context.Background(), PrintInput{
		OrderID:          123,
		PrintLogID:       456,
		MerchantID:       1001,
		PrinterID:        99,
		SN:               "MDP000001",
		Content:          "<CB>hello</CB><BR><CUT>",
		Copies:           1,
		TaskKey:          "order:123:accepted",
		ProviderOriginID: "origin-456",
	})

	require.NoError(t, err)
	require.Equal(t, "psj_000001", orderID)
	require.Equal(t, "MDP000001", received["printer_sn"])
	require.Equal(t, "print_log:456", received["task_key"])
	require.Equal(t, "<CB>hello</CB><BR><CUT>", received["content"])
	require.Equal(t, float64(1), received["copies"])
	require.Empty(t, received["callback_url"])
	metadata, ok := received["metadata"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "feie", metadata["content_format"])
	require.Equal(t, "123", metadata["local_life_order_id"])
	require.Equal(t, "456", metadata["local_life_print_log_id"])
	require.Equal(t, "origin-456", metadata["local_life_provider_origin_id"])
	require.Equal(t, "order:123:accepted", metadata["scenario"])
}

func TestPrintServerClientIncludesCallbackURLWhenPhase2Configured(t *testing.T) {
	fixedNow := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(bodyBytes, &received))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"print_job_id": "psj_callback_001",
			"printer_sn":   "MDP000001",
			"task_key":     "print_log:456",
			"status":       "queued",
			"copies":       1,
			"metadata":     received["metadata"],
		}))
	}))
	defer server.Close()

	client := newPrintServerClientFromConfig(util.Config{
		PrintServerEnabled:               true,
		PrintServerAPIBaseURL:            server.URL,
		PrintServerAppID:                 "local-life",
		PrintServerSecret:                "secret-001",
		PrintServerHTTPTimeout:           time.Second,
		PrintServerCallbackURL:           "https://api.example.com/v1/webhooks/self-cloudprint/print-result",
		PrintServerCallbackSigningSecret: "callback-secret",
	}, func() time.Time { return fixedNow }, func() string { return "nonce-001" })

	orderID, err := client.Print(context.Background(), PrintInput{
		OrderID:    123,
		PrintLogID: 456,
		SN:         "MDP000001",
		Content:    "<CB>hello</CB><BR><CUT>",
	})

	require.NoError(t, err)
	require.Equal(t, "psj_callback_001", orderID)
	require.True(t, client.PrintResultCallbackEnabled())
	require.Equal(t, "https://api.example.com/v1/webhooks/self-cloudprint/print-result", received["callback_url"])
}

func TestPrintServerClientOmitsCallbackURLWhenPrintCannotBeMatchedLocally(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(bodyBytes, &received))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"print_job_id": "psj_test_print_001",
			"printer_sn":   "MDP000001",
			"task_key":     "test_printer:99",
			"status":       "queued",
			"copies":       1,
			"metadata":     received["metadata"],
		}))
	}))
	defer server.Close()

	client := newPrintServerClientFromConfig(util.Config{
		PrintServerEnabled:               true,
		PrintServerAPIBaseURL:            server.URL,
		PrintServerAppID:                 "local-life",
		PrintServerSecret:                "secret-001",
		PrintServerHTTPTimeout:           time.Second,
		PrintServerCallbackURL:           "https://api.example.com/v1/webhooks/self-cloudprint/print-result",
		PrintServerCallbackSigningSecret: "callback-secret",
	}, func() time.Time { return time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC) }, func() string { return "nonce-001" })

	orderID, err := client.Print(context.Background(), PrintInput{
		SN:      "MDP000001",
		Content: "<CB>test</CB><BR><CUT>",
		TaskKey: "test_printer:99",
	})

	require.NoError(t, err)
	require.Equal(t, "psj_test_print_001", orderID)
	require.True(t, client.PrintResultCallbackEnabled())
	require.Empty(t, received["callback_url"])
	metadata, ok := received["metadata"].(map[string]any)
	require.True(t, ok)
	require.Empty(t, metadata["local_life_print_log_id"])
}

func TestBuildPrintServerCallbackSignatureMatchesContract(t *testing.T) {
	body := []byte(`{"event_id":"evt_001","print_job_id":"psj_001","status":"success"}`)
	signature := BuildPrintServerCallbackSignature("callback-secret", "evt_001", "2026-06-16T10:00:00Z", body)

	bodyHash := sha256.Sum256(body)
	mac := hmac.New(sha256.New, []byte("callback-secret"))
	_, _ = mac.Write([]byte("evt_001\n2026-06-16T10:00:00Z\n" + hex.EncodeToString(bodyHash[:])))
	require.Equal(t, hex.EncodeToString(mac.Sum(nil)), signature)
}

func TestPrintServerClientMapsTerminalFailureState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/print-jobs/psj_failed", r.URL.Path)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"print_job_id":     "psj_failed",
			"printer_sn":       "MDP000001",
			"task_key":         "print_log:456",
			"status":           "timeout",
			"error_code":       "paper_out",
			"error_message":    "缺纸",
			"content_hash":     "sha256:" + hex.EncodeToString(sha256.New().Sum(nil)),
			"content_redacted": "len=0 sha256:",
		}))
	}))
	defer server.Close()

	client := newPrintServerClientFromConfig(util.Config{
		PrintServerEnabled:     true,
		PrintServerAPIBaseURL:  server.URL,
		PrintServerAppID:       "local-life",
		PrintServerSecret:      "secret-001",
		PrintServerHTTPTimeout: time.Second,
	}, func() time.Time { return time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC) }, func() string { return "nonce-002" })

	state, err := client.QueryPrintState(context.Background(), "psj_failed")

	require.NoError(t, err)
	require.Equal(t, PrintStateTimeout, state.Status)
	require.Equal(t, "paper_out", state.ErrorCode)
	require.Equal(t, "缺纸", state.ErrorMessage)
}

func TestPrintServerClientAddAndRemovePrinterUseProviderContract(t *testing.T) {
	paths := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/v1/printers":
			var payload map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "MDP000001", payload["printer_sn"])
			require.Equal(t, "binding-secret", payload["printer_key"])
			require.Equal(t, "后厨打印机", payload["printer_name"])
			require.Equal(t, "merchant:1001", payload["merchant_ref"])
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"printer_sn": "MDP000001", "status": "registered"}))
		case "/v1/printers/MDP000001":
			require.Equal(t, http.MethodDelete, r.Method)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"printer_sn": "MDP000001", "status": "disabled"}))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newPrintServerClientFromConfig(util.Config{
		PrintServerEnabled:     true,
		PrintServerAPIBaseURL:  server.URL,
		PrintServerAppID:       "local-life",
		PrintServerSecret:      "secret-001",
		PrintServerHTTPTimeout: time.Second,
	}, func() time.Time { return time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC) }, func() string { return "nonce" })

	require.NoError(t, client.AddPrinter(context.Background(), AddPrinterInput{
		SN:       "MDP000001",
		Key:      "binding-secret",
		Name:     "后厨打印机",
		Business: "1001",
	}))
	require.NoError(t, client.RemovePrinter(context.Background(), RemovePrinterInput{SN: "MDP000001"}))
	require.Equal(t, []string{"POST /v1/printers", "DELETE /v1/printers/MDP000001"}, paths)
}
