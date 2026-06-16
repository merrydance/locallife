package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandleSelfCloudPrintResultNotifyMarksPrintLogSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureSelfCloudPrintCallbackTestServer(server)
	now := time.Now().UTC()
	body := selfCloudPrintCallbackBody(t, map[string]any{
		"event_id":     "evt_success_001",
		"print_job_id": "psj_000001",
		"app_id":       "local-life",
		"tenant_ref":   "merchant:1001",
		"printer_id":   "ptr_000001",
		"task_key":     "print_log:456",
		"status":       "success",
		"copies":       1,
		"metadata": map[string]string{
			"content_format":                "feie",
			"local_life_order_id":           "123",
			"local_life_print_log_id":       "456",
			"local_life_provider_origin_id": "origin-456",
			"scenario":                      "order_accepted",
		},
	})

	store.EXPECT().
		ProcessSelfCloudPrintCallbackTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ProcessSelfCloudPrintCallbackTxParams) (db.ProcessSelfCloudPrintCallbackTxResult, error) {
			require.Equal(t, "evt_success_001", arg.EventID)
			require.Equal(t, "psj_000001", arg.PrintJobID)
			require.Equal(t, int64(456), arg.PrintLogID.Int64)
			require.True(t, arg.PrintLogID.Valid)
			require.Equal(t, "success", arg.CallbackStatus)
			require.Equal(t, db.PrintLogStatusSuccess, arg.PrintLogStatus)
			require.False(t, arg.ErrorMessage.Valid)
			require.JSONEq(t, string(body), string(arg.RawPayload))
			return db.ProcessSelfCloudPrintCallbackTxResult{
				Event:    db.SelfCloudPrintCallbackEvent{EventID: arg.EventID},
				PrintLog: db.PrintLog{ID: 456, Status: db.PrintLogStatusSuccess},
			}, nil
		})

	recorder := httptest.NewRecorder()
	request := newSignedSelfCloudPrintCallbackRequest(t, server, now, body)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"data":"OK"}`, recorder.Body.String())
}

func TestHandleSelfCloudPrintResultNotifyAcksDuplicateEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureSelfCloudPrintCallbackTestServer(server)
	body := selfCloudPrintCallbackBody(t, map[string]any{
		"event_id":     "evt_duplicate_001",
		"print_job_id": "psj_duplicate",
		"app_id":       "local-life",
		"status":       "failed",
		"metadata": map[string]string{
			"local_life_print_log_id": "457",
		},
		"error_code":    "paper_out",
		"error_message": "paper is out",
	})

	store.EXPECT().
		ProcessSelfCloudPrintCallbackTx(gomock.Any(), gomock.Any()).
		Return(db.ProcessSelfCloudPrintCallbackTxResult{
			Duplicate: true,
			Event:     db.SelfCloudPrintCallbackEvent{EventID: "evt_duplicate_001"},
		}, nil)

	recorder := httptest.NewRecorder()
	request := newSignedSelfCloudPrintCallbackRequest(t, server, time.Now().UTC(), body)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"data":"OK"}`, recorder.Body.String())
}

func TestHandleSelfCloudPrintResultNotifyRejectsInvalidSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureSelfCloudPrintCallbackTestServer(server)
	body := selfCloudPrintCallbackBody(t, map[string]any{
		"event_id":     "evt_bad_sign",
		"print_job_id": "psj_bad_sign",
		"app_id":       "local-life",
		"status":       "success",
	})
	store.EXPECT().ProcessSelfCloudPrintCallbackTx(gomock.Any(), gomock.Any()).Times(0)

	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/self-cloudprint/print-result", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Print-Callback-Event-Id", "evt_bad_sign")
	request.Header.Set("X-Print-Callback-Timestamp", time.Now().UTC().Format(time.RFC3339))
	request.Header.Set("X-Print-Callback-Signature", "bad-signature")
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "OK")
}

func TestHandleSelfCloudPrintResultNotifyRejectsUnknownStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureSelfCloudPrintCallbackTestServer(server)
	body := selfCloudPrintCallbackBody(t, map[string]any{
		"event_id":     "evt_non_terminal",
		"print_job_id": "psj_non_terminal",
		"app_id":       "local-life",
		"status":       "queued",
	})
	store.EXPECT().ProcessSelfCloudPrintCallbackTx(gomock.Any(), gomock.Any()).Times(0)

	recorder := httptest.NewRecorder()
	request := newSignedSelfCloudPrintCallbackRequest(t, server, time.Now().UTC(), body)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "OK")
}

func TestHandleSelfCloudPrintResultNotifyRequestsRetryWhenPrintLogCannotBeMatched(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureSelfCloudPrintCallbackTestServer(server)
	body := selfCloudPrintCallbackBody(t, map[string]any{
		"event_id":     "evt_missing_log",
		"print_job_id": "psj_missing_log",
		"app_id":       "local-life",
		"status":       "success",
		"metadata": map[string]string{
			"local_life_print_log_id": "999999",
		},
	})

	store.EXPECT().
		ProcessSelfCloudPrintCallbackTx(gomock.Any(), gomock.Any()).
		Return(db.ProcessSelfCloudPrintCallbackTxResult{}, db.ErrRecordNotFound)

	recorder := httptest.NewRecorder()
	request := newSignedSelfCloudPrintCallbackRequest(t, server, time.Now().UTC(), body)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "OK")
}

func TestHandleSelfCloudPrintResultNotifyRejectsMismatchedHeaderEventID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureSelfCloudPrintCallbackTestServer(server)
	body := selfCloudPrintCallbackBody(t, map[string]any{
		"event_id":     "evt_body",
		"print_job_id": "psj_mismatch",
		"app_id":       "local-life",
		"status":       "success",
	})
	store.EXPECT().ProcessSelfCloudPrintCallbackTx(gomock.Any(), gomock.Any()).Times(0)

	request := newSignedSelfCloudPrintCallbackRequest(t, server, time.Now().UTC(), body)
	request.Header.Set("X-Print-Callback-Event-Id", "evt_header")
	request.Header.Set("X-Print-Callback-Signature", buildSelfCloudPrintCallbackTestSignature(
		server.config.PrintServerCallbackSigningSecret,
		"evt_header",
		request.Header.Get("X-Print-Callback-Timestamp"),
		body,
	))
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "OK")
}

func TestHandleSelfCloudPrintResultNotifyRejectsMissingOrMismatchedAppID(t *testing.T) {
	tests := []struct {
		name  string
		appID any
	}{
		{
			name: "missing app_id",
		},
		{
			name:  "mismatched app_id",
			appID: "other-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			server := newTestServer(t, store)
			configureSelfCloudPrintCallbackTestServer(server)
			payload := map[string]any{
				"event_id":     "evt_app_id",
				"print_job_id": "psj_app_id",
				"status":       "success",
			}
			if tt.appID != nil {
				payload["app_id"] = tt.appID
			}
			body := selfCloudPrintCallbackBody(t, payload)
			store.EXPECT().ProcessSelfCloudPrintCallbackTx(gomock.Any(), gomock.Any()).Times(0)

			recorder := httptest.NewRecorder()
			request := newSignedSelfCloudPrintCallbackRequest(t, server, time.Now().UTC(), body)

			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.NotContains(t, recorder.Body.String(), "OK")
		})
	}
}

func TestHandleSelfCloudPrintResultNotifyRejectsWhenConfiguredAppIDIsMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureSelfCloudPrintCallbackTestServer(server)
	server.config.PrintServerAppID = ""
	body := selfCloudPrintCallbackBody(t, map[string]any{
		"event_id":     "evt_missing_config_app",
		"print_job_id": "psj_missing_config_app",
		"app_id":       "local-life",
		"status":       "success",
	})
	store.EXPECT().ProcessSelfCloudPrintCallbackTx(gomock.Any(), gomock.Any()).Times(0)

	recorder := httptest.NewRecorder()
	request := newSignedSelfCloudPrintCallbackRequest(t, server, time.Now().UTC(), body)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "OK")
}

func TestHandleSelfCloudPrintResultNotifyRejectsStaleTimestamp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureSelfCloudPrintCallbackTestServer(server)
	body := selfCloudPrintCallbackBody(t, map[string]any{
		"event_id":     "evt_stale",
		"print_job_id": "psj_stale",
		"app_id":       "local-life",
		"status":       "success",
	})
	store.EXPECT().ProcessSelfCloudPrintCallbackTx(gomock.Any(), gomock.Any()).Times(0)

	recorder := httptest.NewRecorder()
	request := newSignedSelfCloudPrintCallbackRequest(t, server, time.Now().UTC().Add(-11*time.Minute), body)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "OK")
}

func configureSelfCloudPrintCallbackTestServer(server *Server) {
	server.config.PrintServerAppID = "local-life"
	server.config.PrintServerCallbackSigningSecret = "callback-secret"
	server.config.PrintServerCallbackFreshnessWindow = 10 * time.Minute
}

func selfCloudPrintCallbackBody(t *testing.T, payload map[string]any) []byte {
	t.Helper()

	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return body
}

func newSignedSelfCloudPrintCallbackRequest(t *testing.T, server *Server, timestamp time.Time, body []byte) *http.Request {
	t.Helper()

	var payload struct {
		EventID string `json:"event_id"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	eventID := strings.TrimSpace(payload.EventID)
	require.NotEmpty(t, eventID)

	timestampValue := timestamp.UTC().Format(time.RFC3339)
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/self-cloudprint/print-result", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Print-Callback-Event-Id", eventID)
	request.Header.Set("X-Print-Callback-Timestamp", timestampValue)
	request.Header.Set("X-Print-Callback-Signature", buildSelfCloudPrintCallbackTestSignature(
		server.config.PrintServerCallbackSigningSecret,
		eventID,
		timestampValue,
		body,
	))
	return request
}

func buildSelfCloudPrintCallbackTestSignature(secret, eventID, timestamp string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.Join([]string{eventID, timestamp, hex.EncodeToString(bodyHash[:])}, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}
