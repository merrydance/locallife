package api

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandleFeieyunPrintResultNotifyMarksPrintLogSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	privateKey, publicPEM := newFeieyunCallbackTestKey(t)
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.config.FeieyunCallbackPublicKeyPEM = publicPEM

	store.EXPECT().
		GetPrintLogByVendorOrderID(gomock.Any(), pgtype.Text{String: "vendor-order-123", Valid: true}).
		Return(db.PrintLog{ID: 42, OrderID: 100, PrinterID: 7, Status: "pending"}, nil)
	store.EXPECT().
		UpdatePrintLogStatus(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpdatePrintLogStatusParams) (db.PrintLog, error) {
			require.Equal(t, int64(42), arg.ID)
			require.Equal(t, "success", arg.Status)
			require.False(t, arg.ErrorMessage.Valid)
			require.False(t, arg.VendorOrderID.Valid)
			return db.PrintLog{ID: arg.ID, Status: arg.Status}, nil
		})

	recorder := httptest.NewRecorder()
	request := newSignedFeieyunCallbackRequest(t, privateKey, url.Values{
		"orderId": []string{"vendor-order-123"},
		"status":  []string{"1"},
		"stime":   []string{"1625194910"},
	})

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "SUCCESS", strings.TrimSpace(recorder.Body.String()))
}

func TestHandleFeieyunPrintResultNotifyRejectsInvalidSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	privateKey, publicPEM := newFeieyunCallbackTestKey(t)
	server := newTestServer(t, mockdb.NewMockStore(ctrl))
	server.config.FeieyunCallbackPublicKeyPEM = publicPEM

	values := signedFeieyunCallbackValues(t, privateKey, url.Values{
		"orderId": []string{"vendor-order-123"},
		"status":  []string{"1"},
		"stime":   []string{"1625194910"},
	})
	values.Set("status", "2")
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/feieyun/print-result", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.NotEqual(t, "SUCCESS", strings.TrimSpace(recorder.Body.String()))
}

func TestHandleFeieyunPrintResultNotifyRequiresPublicKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	privateKey, _ := newFeieyunCallbackTestKey(t)
	server := newTestServer(t, mockdb.NewMockStore(ctrl))
	request := newSignedFeieyunCallbackRequest(t, privateKey, url.Values{
		"orderId": []string{"vendor-order-123"},
		"status":  []string{"1"},
		"stime":   []string{"1625194910"},
	})
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.NotEqual(t, "SUCCESS", strings.TrimSpace(recorder.Body.String()))
}

func TestHandleFeieyunPrintResultNotifyRejectsInvalidStime(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	privateKey, publicPEM := newFeieyunCallbackTestKey(t)
	server := newTestServer(t, mockdb.NewMockStore(ctrl))
	server.config.FeieyunCallbackPublicKeyPEM = publicPEM
	request := newSignedFeieyunCallbackRequest(t, privateKey, url.Values{
		"orderId": []string{"vendor-order-123"},
		"status":  []string{"1"},
		"stime":   []string{"not-a-timestamp"},
	})
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.NotEqual(t, "SUCCESS", strings.TrimSpace(recorder.Body.String()))
}

func TestHandleFeieyunPrintResultNotifyRetriesUnknownVendorOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	privateKey, publicPEM := newFeieyunCallbackTestKey(t)
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.config.FeieyunCallbackPublicKeyPEM = publicPEM

	store.EXPECT().
		GetPrintLogByVendorOrderID(gomock.Any(), pgtype.Text{String: "missing-vendor-order", Valid: true}).
		Return(db.PrintLog{}, db.ErrRecordNotFound)

	request := newSignedFeieyunCallbackRequest(t, privateKey, url.Values{
		"orderId": []string{"missing-vendor-order"},
		"status":  []string{"1"},
		"stime":   []string{"1625194910"},
	})
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.NotEqual(t, "SUCCESS", strings.TrimSpace(recorder.Body.String()))
}

func TestHandleFeieyunPrintResultNotifyAcksUnknownStatusWithoutUpdating(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	privateKey, publicPEM := newFeieyunCallbackTestKey(t)
	server := newTestServer(t, mockdb.NewMockStore(ctrl))
	server.config.FeieyunCallbackPublicKeyPEM = publicPEM

	request := newSignedFeieyunCallbackRequest(t, privateKey, url.Values{
		"orderId": []string{"vendor-order-123"},
		"status":  []string{"9"},
		"stime":   []string{"1625194910"},
	})
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "SUCCESS", strings.TrimSpace(recorder.Body.String()))
}

func newFeieyunCallbackTestKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	return privateKey, publicPEM
}

func newSignedFeieyunCallbackRequest(t *testing.T, privateKey *rsa.PrivateKey, values url.Values) *http.Request {
	t.Helper()

	signedValues := signedFeieyunCallbackValues(t, privateKey, values)
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/feieyun/print-result", strings.NewReader(signedValues.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func signedFeieyunCallbackValues(t *testing.T, privateKey *rsa.PrivateKey, values url.Values) url.Values {
	t.Helper()

	signedValues := cloneURLValues(values)
	digest := sha256.Sum256([]byte(cloudprint.BuildFeieyunCallbackCanonicalString(signedValues)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	require.NoError(t, err)
	signedValues.Set("sign", base64.StdEncoding.EncodeToString(signature))
	return signedValues
}

func cloneURLValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, list := range values {
		copied := make([]string, len(list))
		copy(copied, list)
		cloned[key] = copied
	}
	return cloned
}
