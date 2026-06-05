package api

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandleYilianyunPrintResultHealthReturnsOfficialOK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	server := newTestServer(t, mockdb.NewMockStore(ctrl))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/webhooks/yilianyun/print-result", http.NoBody)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"data":"OK"}`, recorder.Body.String())
}

func TestHandleYilianyunPrintResultNotifyMarksPrintLogSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureYilianyunCallbackTestServer(server)
	now := time.Now().UTC()
	originID := "origin-1001"
	printLog := db.PrintLog{
		ID:               501,
		OrderID:          1001,
		PrinterID:        21,
		Status:           "pending",
		VendorOrderID:    pgtype.Text{String: "yl-order-1001", Valid: true},
		ProviderOriginID: pgtype.Text{String: originID, Valid: true},
	}

	store.EXPECT().
		GetPrintLogByProviderAndOriginID(gomock.Any(), db.GetPrintLogByProviderAndOriginIDParams{
			PrinterType:      string(cloudprint.ProviderYilianyun),
			ProviderOriginID: pgtype.Text{String: originID, Valid: true},
		}).
		Return(printLog, nil)
	store.EXPECT().
		MarkProviderStatusPrintLogTerminal(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkProviderStatusPrintLogTerminalParams) (db.PrintLog, error) {
			require.Equal(t, printLog.ID, arg.ID)
			require.Equal(t, "success", arg.Status)
			require.False(t, arg.ErrorMessage.Valid)
			return db.PrintLog{ID: printLog.ID, Status: "success"}, nil
		})

	recorder := httptest.NewRecorder()
	request := newSignedYilianyunPrintCallbackRequest(t, server, now, url.Values{
		"cmd":          []string{"oauth_finish"},
		"machine_code": []string{"YL-MACHINE-001"},
		"order_id":     []string{"yl-order-1001"},
		"state":        []string{"1"},
		"print_time":   []string{strconv.FormatInt(now.Unix()-3, 10)},
		"origin_id":    []string{originID},
	})

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"data":"OK"}`, recorder.Body.String())
}

func TestHandleYilianyunPrintResultNotifyMarksPrintLogCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureYilianyunCallbackTestServer(server)
	now := time.Now().UTC()
	originID := "origin-cancelled"
	printLog := db.PrintLog{
		ID:               502,
		OrderID:          1002,
		PrinterID:        22,
		Status:           "pending",
		VendorOrderID:    pgtype.Text{String: "yl-order-1002", Valid: true},
		ProviderOriginID: pgtype.Text{String: originID, Valid: true},
	}

	store.EXPECT().
		GetPrintLogByProviderAndOriginID(gomock.Any(), gomock.Any()).
		Return(printLog, nil)
	store.EXPECT().
		MarkProviderStatusPrintLogTerminal(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkProviderStatusPrintLogTerminalParams) (db.PrintLog, error) {
			require.Equal(t, printLog.ID, arg.ID)
			require.Equal(t, "cancelled", arg.Status)
			require.True(t, arg.ErrorMessage.Valid)
			require.Equal(t, "yilianyun_print_exception", arg.ErrorMessage.String)
			return db.PrintLog{ID: printLog.ID, Status: "cancelled"}, nil
		})

	recorder := httptest.NewRecorder()
	request := newSignedYilianyunPrintCallbackRequest(t, server, now, url.Values{
		"cmd":          []string{"oauth_finish"},
		"machine_code": []string{"YL-MACHINE-002"},
		"order_id":     []string{"yl-order-1002"},
		"state":        []string{"2"},
		"print_time":   []string{strconv.FormatInt(now.Unix()-2, 10)},
		"origin_id":    []string{originID},
	})

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"data":"OK"}`, recorder.Body.String())
}

func TestHandleYilianyunPrintResultNotifyAcksDuplicateTerminalCallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureYilianyunCallbackTestServer(server)
	now := time.Now().UTC()
	originID := "origin-duplicate"
	printLog := db.PrintLog{
		ID:               503,
		OrderID:          1003,
		PrinterID:        23,
		Status:           "success",
		VendorOrderID:    pgtype.Text{String: "yl-order-1003", Valid: true},
		ProviderOriginID: pgtype.Text{String: originID, Valid: true},
	}

	store.EXPECT().GetPrintLogByProviderAndOriginID(gomock.Any(), gomock.Any()).Return(printLog, nil)
	store.EXPECT().MarkProviderStatusPrintLogTerminal(gomock.Any(), gomock.Any()).Times(0)

	recorder := httptest.NewRecorder()
	request := newSignedYilianyunPrintCallbackRequest(t, server, now, url.Values{
		"cmd":          []string{"oauth_finish"},
		"machine_code": []string{"YL-MACHINE-003"},
		"order_id":     []string{"yl-order-1003"},
		"state":        []string{"1"},
		"print_time":   []string{strconv.FormatInt(now.Unix()-1, 10)},
		"origin_id":    []string{originID},
	})

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"data":"OK"}`, recorder.Body.String())
}

func TestHandleYilianyunPrintResultNotifyRejectsMismatchedVendorOrderID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureYilianyunCallbackTestServer(server)
	now := time.Now().UTC()
	originID := "origin-mismatch"
	printLog := db.PrintLog{
		ID:               504,
		OrderID:          1004,
		PrinterID:        24,
		Status:           "pending",
		VendorOrderID:    pgtype.Text{String: "yl-order-local", Valid: true},
		ProviderOriginID: pgtype.Text{String: originID, Valid: true},
	}

	store.EXPECT().GetPrintLogByProviderAndOriginID(gomock.Any(), gomock.Any()).Return(printLog, nil)
	store.EXPECT().MarkProviderStatusPrintLogTerminal(gomock.Any(), gomock.Any()).Times(0)

	recorder := httptest.NewRecorder()
	request := newSignedYilianyunPrintCallbackRequest(t, server, now, url.Values{
		"cmd":          []string{"oauth_finish"},
		"machine_code": []string{"YL-MACHINE-007"},
		"order_id":     []string{"yl-order-callback"},
		"state":        []string{"1"},
		"print_time":   []string{strconv.FormatInt(now.Unix(), 10)},
		"origin_id":    []string{originID},
	})

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "OK")
}

func TestHandleYilianyunPrintResultNotifyRejectsInvalidSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	server := newTestServer(t, mockdb.NewMockStore(ctrl))
	configureYilianyunCallbackTestServer(server)
	now := time.Now().UTC()
	request := newSignedYilianyunPrintCallbackRequest(t, server, now, url.Values{
		"cmd":          []string{"oauth_finish"},
		"machine_code": []string{"YL-MACHINE-004"},
		"order_id":     []string{"yl-order-1004"},
		"state":        []string{"1"},
		"print_time":   []string{strconv.FormatInt(now.Unix(), 10)},
		"origin_id":    []string{"origin-invalid-sign"},
	})
	values := signedYilianyunPrintCallbackValues(t, server, now, url.Values{
		"cmd":          []string{"oauth_finish"},
		"machine_code": []string{"YL-MACHINE-004"},
		"order_id":     []string{"yl-order-1004"},
		"state":        []string{"1"},
		"print_time":   []string{strconv.FormatInt(now.Unix(), 10)},
		"origin_id":    []string{"origin-invalid-sign"},
	})
	values.Set("sign", "bad-sign")
	request = httptest.NewRequest(http.MethodPost, "/v1/webhooks/yilianyun/print-result", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "OK")
}

func TestHandleYilianyunPrintResultNotifyRejectsStaleOrFuturePushTime(t *testing.T) {
	for _, tc := range []struct {
		name     string
		pushTime time.Time
	}{
		{name: "stale", pushTime: time.Now().UTC().Add(-11 * time.Minute)},
		{name: "future", pushTime: time.Now().UTC().Add(11 * time.Minute)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			server := newTestServer(t, mockdb.NewMockStore(ctrl))
			configureYilianyunCallbackTestServer(server)
			request := newSignedYilianyunPrintCallbackRequest(t, server, tc.pushTime, url.Values{
				"cmd":          []string{"oauth_finish"},
				"machine_code": []string{"YL-MACHINE-005"},
				"order_id":     []string{"yl-order-1005"},
				"state":        []string{"1"},
				"print_time":   []string{strconv.FormatInt(tc.pushTime.Unix(), 10)},
				"origin_id":    []string{"origin-stale"},
			})
			recorder := httptest.NewRecorder()

			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			require.NotContains(t, recorder.Body.String(), "OK")
		})
	}
}

func TestHandleYilianyunPrintResultNotifyRetriesUnknownOriginID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	configureYilianyunCallbackTestServer(server)
	now := time.Now().UTC()

	store.EXPECT().
		GetPrintLogByProviderAndOriginID(gomock.Any(), gomock.Any()).
		Return(db.PrintLog{}, db.ErrRecordNotFound)

	recorder := httptest.NewRecorder()
	request := newSignedYilianyunPrintCallbackRequest(t, server, now, url.Values{
		"cmd":          []string{"oauth_finish"},
		"machine_code": []string{"YL-MACHINE-006"},
		"order_id":     []string{"yl-order-1006"},
		"state":        []string{"1"},
		"print_time":   []string{strconv.FormatInt(now.Unix(), 10)},
		"origin_id":    []string{"origin-missing"},
	})

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "OK")
}

func configureYilianyunCallbackTestServer(server *Server) {
	server.config.YilianyunAppID = "yly-app"
	server.config.YilianyunAppSecret = "yly-secret"
	server.config.YilianyunPrintCallbackFreshnessWindow = 10 * time.Minute
}

func newSignedYilianyunPrintCallbackRequest(t *testing.T, server *Server, pushTime time.Time, values url.Values) *http.Request {
	t.Helper()

	signedValues := signedYilianyunPrintCallbackValues(t, server, pushTime, values)
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/yilianyun/print-result", strings.NewReader(signedValues.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func signedYilianyunPrintCallbackValues(t *testing.T, server *Server, pushTime time.Time, values url.Values) url.Values {
	t.Helper()

	signedValues := cloneURLValues(values)
	pushTimeValue := strconv.FormatInt(pushTime.Unix(), 10)
	signedValues.Set("push_time", pushTimeValue)
	signedValues.Set("sign", buildYilianyunCallbackTestSign(server.config.YilianyunAppID, pushTimeValue, server.config.YilianyunAppSecret))
	return signedValues
}

func buildYilianyunCallbackTestSign(clientID, pushTime, clientSecret string) string {
	sum := md5.Sum([]byte(clientID + pushTime + clientSecret))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
