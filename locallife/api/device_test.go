package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== Helper Functions ====================

func randomCloudPrinter(merchantID int64) db.CloudPrinter {
	return db.CloudPrinter{
		ID:               util.RandomInt(1, 1000),
		MerchantID:       merchantID,
		PrinterName:      "测试打印机",
		PrinterSn:        util.RandomString(20),
		PrinterKey:       util.RandomString(32),
		PrinterType:      "feieyun",
		PrintTakeout:     true,
		PrintDineIn:      true,
		PrintReservation: true,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

func randomOrderDisplayConfig(merchantID int64) db.OrderDisplayConfig {
	return db.OrderDisplayConfig{
		ID:               util.RandomInt(1, 1000),
		MerchantID:       merchantID,
		EnablePrint:      true,
		PrintTakeout:     true,
		PrintDineIn:      true,
		PrintReservation: true,
		EnableVoice:      false,
		VoiceTakeout:     true,
		VoiceDineIn:      true,
		EnableKds:        false,
		KdsUrl:           pgtype.Text{String: "", Valid: false},
		CreatedAt:        time.Now(),
		UpdatedAt:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

// ==================== 创建打印机测试 ====================

func TestCreatePrinterAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"printer_name": "测试打印机",
				"printer_sn":   "SN123456789",
				"printer_key":  "KEY123456789",
				"printer_type": "feieyun",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCloudPrinterBySN(gomock.Any(), gomock.Eq("SN123456789")).
					Times(1).
					Return(db.CloudPrinter{}, sql.ErrNoRows)

				store.EXPECT().
					CreateCloudPrinter(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CloudPrinter{
						ID:               1,
						MerchantID:       merchant.ID,
						PrinterName:      "测试打印机",
						PrinterSn:        "SN123456789",
						PrinterType:      "feieyun",
						PrintTakeout:     true,
						PrintDineIn:      true,
						PrintReservation: true,
						IsActive:         true,
						CreatedAt:        time.Now(),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var response printerResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "测试打印机", response.PrinterName)
				require.Equal(t, "SN123456789", response.PrinterSN)
			},
		},
		{
			name: "OK_WithOtherPrinterType",
			body: map[string]interface{}{
				"printer_name": "其他打印机",
				"printer_sn":   "SN_OTHER_001",
				"printer_key":  "KEY_OTHER_001",
				"printer_type": "other",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCloudPrinterBySN(gomock.Any(), gomock.Eq("SN_OTHER_001")).
					Times(1).
					Return(db.CloudPrinter{}, sql.ErrNoRows)

				store.EXPECT().
					CreateCloudPrinter(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CloudPrinter{
						ID:          1,
						MerchantID:  merchant.ID,
						PrinterName: "其他打印机",
						PrinterSn:   "SN_OTHER_001",
						PrinterType: "other",
						IsActive:    true,
						CreatedAt:   time.Now(),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var response printerResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "other", response.PrinterType)
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]interface{}{
				"printer_name": "测试打印机",
				"printer_sn":   "SN123456789",
				"printer_key":  "KEY123456789",
				"printer_type": "feieyun",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "MerchantNotFound",
			body: map[string]interface{}{
				"printer_name": "测试打印机",
				"printer_sn":   "SN123456789",
				"printer_key":  "KEY123456789",
				"printer_type": "feieyun",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "PrinterSNAlreadyExists",
			body: map[string]interface{}{
				"printer_name": "测试打印机",
				"printer_sn":   "SN123456789",
				"printer_key":  "KEY123456789",
				"printer_type": "feieyun",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCloudPrinterBySN(gomock.Any(), gomock.Eq("SN123456789")).
					Times(1).
					Return(db.CloudPrinter{ID: 1}, nil) // Printer exists
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "MissingPrinterName",
			body: map[string]interface{}{
				"printer_sn":   "SN123456789",
				"printer_key":  "KEY123456789",
				"printer_type": "feieyun",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "MissingPrinterSN",
			body: map[string]interface{}{
				"printer_name": "测试打印机",
				"printer_key":  "KEY123456789",
				"printer_type": "feieyun",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidPrinterType",
			body: map[string]interface{}{
				"printer_name": "测试打印机",
				"printer_sn":   "SN123456789",
				"printer_key":  "KEY123456789",
				"printer_type": "invalid_type",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "PrinterNameTooLong",
			body: map[string]interface{}{
				"printer_name": util.RandomString(101), // > 100
				"printer_sn":   "SN123456789",
				"printer_key":  "KEY123456789",
				"printer_type": "feieyun",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/merchant/devices"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 获取打印机列表测试 ====================

func TestListPrintersAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	printers := []db.CloudPrinter{
		randomCloudPrinter(merchant.ID),
		randomCloudPrinter(merchant.ID),
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListCloudPrintersByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(printers, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, float64(2), response["total"])
			},
		},
		{
			name:  "OK_OnlyActive",
			query: "?only_active=true",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListActiveCloudPrintersByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(printers[:1], nil) // 只返回1个活跃的
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, float64(1), response["total"])
			},
		},
		{
			name:  "NoAuthorization",
			query: "",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:  "MerchantNotFound",
			query: "",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/devices" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 获取单个打印机测试 ====================

func TestGetPrinterAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	printer := randomCloudPrinter(merchant.ID)

	testCases := []struct {
		name          string
		printerID     int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:      "OK",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
					Times(1).
					Return(printer, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response printerResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, printer.ID, response.ID)
			},
		},
		{
			name:      "NoAuthorization",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:      "PrinterNotFound",
			printerID: 999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.CloudPrinter{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "PrinterNotBelongToMerchant",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				otherPrinter := printer
				otherPrinter.MerchantID = merchant.ID + 1 // 不同商户
				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
					Times(1).
					Return(otherPrinter, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:      "InvalidPrinterID",
			printerID: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchant/devices/%d", tc.printerID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 更新打印机测试 ====================

func TestUpdatePrinterAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	printer := randomCloudPrinter(merchant.ID)

	testCases := []struct {
		name          string
		printerID     int64
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:      "OK",
			printerID: printer.ID,
			body: map[string]interface{}{
				"printer_name": "更新后的打印机名",
				"is_active":    false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
					Times(1).
					Return(printer, nil)

				updatedPrinter := printer
				updatedPrinter.PrinterName = "更新后的打印机名"
				updatedPrinter.IsActive = false
				store.EXPECT().
					UpdateCloudPrinter(gomock.Any(), gomock.Any()).
					Times(1).
					Return(updatedPrinter, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response printerResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "更新后的打印机名", response.PrinterName)
				require.Equal(t, false, response.IsActive)
			},
		},
		{
			name:      "OK_UpdatePrintSettings",
			printerID: printer.ID,
			body: map[string]interface{}{
				"print_takeout": false,
				"print_dine_in": true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
					Times(1).
					Return(printer, nil)

				updatedPrinter := printer
				updatedPrinter.PrintTakeout = false
				store.EXPECT().
					UpdateCloudPrinter(gomock.Any(), gomock.Any()).
					Times(1).
					Return(updatedPrinter, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:      "NoAuthorization",
			printerID: printer.ID,
			body: map[string]interface{}{
				"printer_name": "更新后的打印机名",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:      "PrinterNotFound",
			printerID: 999,
			body: map[string]interface{}{
				"printer_name": "更新后的打印机名",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.CloudPrinter{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "PrinterNotBelongToMerchant",
			printerID: printer.ID,
			body: map[string]interface{}{
				"printer_name": "更新后的打印机名",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				otherPrinter := printer
				otherPrinter.MerchantID = merchant.ID + 1
				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
					Times(1).
					Return(otherPrinter, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:      "PrinterNameTooLong",
			printerID: printer.ID,
			body: map[string]interface{}{
				"printer_name": util.RandomString(101),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/merchant/devices/%d", tc.printerID)
			request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
			require.NoError(t, err)

			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 删除打印机测试 ====================

func TestDeletePrinterAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	printer := randomCloudPrinter(merchant.ID)

	testCases := []struct {
		name          string
		printerID     int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:      "OK",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
					Times(1).
					Return(printer, nil)

				store.EXPECT().
					DeleteCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:      "NoAuthorization",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:      "PrinterNotFound",
			printerID: 999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.CloudPrinter{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "PrinterNotBelongToMerchant",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				otherPrinter := printer
				otherPrinter.MerchantID = merchant.ID + 1
				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
					Times(1).
					Return(otherPrinter, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchant/devices/%d", tc.printerID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 测试打印机测试 ====================

func TestTestPrinterAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	printer := randomCloudPrinter(merchant.ID)

	testCases := []struct {
		name          string
		printerID     int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:      "NotImplemented",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
					Times(1).
					Return(printer, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// 测试打印功能尚未实现，返回501
				require.Equal(t, http.StatusNotImplemented, recorder.Code)
			},
		},
		{
			name:      "NoAuthorization",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:      "PrinterNotFound",
			printerID: 999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.CloudPrinter{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "PrinterNotBelongToMerchant",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				otherPrinter := printer
				otherPrinter.MerchantID = merchant.ID + 1
				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
					Times(1).
					Return(otherPrinter, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchant/devices/%d/test", tc.printerID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 获取展示配置测试 ====================

func TestGetDisplayConfigAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	config := randomOrderDisplayConfig(merchant.ID)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrderDisplayConfigByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(config, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response getDisplayConfigResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, config.ID, response.ID)
				require.Equal(t, config.EnablePrint, response.EnablePrint)
			},
		},
		{
			name: "OK_NoConfigReturnsDefault",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrderDisplayConfigByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(db.OrderDisplayConfig{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response getDisplayConfigResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				// 验证默认值
				require.Equal(t, true, response.EnablePrint)
				require.Equal(t, true, response.PrintTakeout)
				require.Equal(t, false, response.EnableVoice)
				require.Equal(t, false, response.EnableKDS)
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "MerchantNotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/merchant/display-config"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 更新展示配置测试 ====================

func TestUpdateDisplayConfigAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	config := randomOrderDisplayConfig(merchant.ID)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_UpdateExisting",
			body: map[string]interface{}{
				"enable_print": false,
				"enable_voice": true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrderDisplayConfigByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(config, nil)

				updatedConfig := config
				updatedConfig.EnablePrint = false
				updatedConfig.EnableVoice = true
				store.EXPECT().
					UpdateOrderDisplayConfig(gomock.Any(), gomock.Any()).
					Times(1).
					Return(updatedConfig, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response getDisplayConfigResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, false, response.EnablePrint)
				require.Equal(t, true, response.EnableVoice)
			},
		},
		{
			name: "OK_CreateNew",
			body: map[string]interface{}{
				"enable_kds": true,
				"kds_url":    "https://kds.example.com",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetOrderDisplayConfigByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(db.OrderDisplayConfig{}, sql.ErrNoRows)

				newConfig := config
				newConfig.EnableKds = true
				newConfig.KdsUrl = pgtype.Text{String: "https://kds.example.com", Valid: true}
				store.EXPECT().
					CreateOrderDisplayConfig(gomock.Any(), gomock.Any()).
					Times(1).
					Return(newConfig, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]interface{}{
				"enable_print": false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "MerchantNotFound",
			body: map[string]interface{}{
				"enable_print": false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "InvalidKdsURL",
			body: map[string]interface{}{
				"enable_kds": true,
				"kds_url":    "not-a-valid-url",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "KdsURLTooLong",
			body: map[string]interface{}{
				"enable_kds": true,
				"kds_url":    "https://example.com/" + util.RandomString(501),
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/merchant/display-config"
			request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
			require.NoError(t, err)

			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
