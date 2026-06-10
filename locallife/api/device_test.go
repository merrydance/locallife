package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
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
		PrinterRole:      "front",
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
		ID:                   util.RandomInt(1, 1000),
		MerchantID:           merchantID,
		EnablePrint:          true,
		PrintTakeout:         true,
		PrintDineIn:          true,
		PrintReservation:     true,
		PrintDispatchMode:    "single_full",
		PrintTriggerMode:     "accepted",
		AutoAcceptPaidOrders: false,
		EnableVoice:          false,
		VoiceTakeout:         true,
		VoiceDineIn:          true,
		EnableKds:            false,
		KdsUrl:               pgtype.Text{String: "", Valid: false},
		CreatedAt:            time.Now(),
		UpdatedAt:            pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

type printerClientStub struct {
	addInputs             []cloudprint.AddPrinterInput
	removeInputs          []cloudprint.RemovePrinterInput
	printInputs           []cloudprint.PrintInput
	addErr                error
	removeErr             error
	printErr              error
	printOrderID          string
	queryOrderID          string
	queryPrinted          bool
	queryOrderErr         error
	queryPrinterSN        string
	queryPrinterStatus    string
	queryPrinterStatusErr error
	queryPrinterInfoSN    string
	queryPrinterInfo      cloudprint.PrinterInfo
	queryPrinterInfoErr   error
}

func (s *printerClientStub) AddPrinter(ctx context.Context, input cloudprint.AddPrinterInput) error {
	s.addInputs = append(s.addInputs, input)
	return s.addErr
}

func (s *printerClientStub) RemovePrinter(ctx context.Context, input cloudprint.RemovePrinterInput) error {
	s.removeInputs = append(s.removeInputs, input)
	return s.removeErr
}

func (s *printerClientStub) Print(ctx context.Context, input cloudprint.PrintInput) (string, error) {
	s.printInputs = append(s.printInputs, input)
	if s.printOrderID == "" {
		s.printOrderID = "vendor-order-id"
	}
	return s.printOrderID, s.printErr
}

func (s *printerClientStub) PrintResultCallbackEnabled() bool {
	return false
}

func (s *printerClientStub) QueryOrderState(ctx context.Context, orderID string) (bool, error) {
	s.queryOrderID = orderID
	return s.queryPrinted, s.queryOrderErr
}

func (s *printerClientStub) QueryPrinterStatus(ctx context.Context, sn string) (string, error) {
	s.queryPrinterSN = sn
	if s.queryPrinterStatus == "" {
		s.queryPrinterStatus = "在线，工作状态正常"
	}
	return s.queryPrinterStatus, s.queryPrinterStatusErr
}

func (s *printerClientStub) GetPrinterInfo(ctx context.Context, sn string) (cloudprint.PrinterInfo, error) {
	s.queryPrinterInfoSN = sn
	if s.queryPrinterInfo.Model == "" {
		s.queryPrinterInfo = cloudprint.PrinterInfo{Model: "FEIE-58", Status: "online"}
	}
	return s.queryPrinterInfo, s.queryPrinterInfoErr
}

type printerProviderManagerStub struct {
	providers map[string]cloudprint.Client
}

func (s printerProviderManagerStub) Provider(providerType string) (cloudprint.Client, bool) {
	provider, ok := s.providers[providerType]
	return provider, ok
}

func (s printerProviderManagerStub) Supported(providerType string) bool {
	_, ok := s.Provider(providerType)
	return ok
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetCloudPrinterBySN(gomock.Any(), gomock.Eq("SN123456789")).
					Times(1).
					Return(db.CloudPrinter{}, db.ErrRecordNotFound)

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
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "测试打印机", response.PrinterName)
				require.Equal(t, "SN123456789", response.PrinterSN)
			},
		},
		{
			name: "InvalidPrinterType",
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "YilianyunRequiresAuthorizationFlow",
			body: map[string]interface{}{
				"printer_name": "易联云前台",
				"printer_sn":   "YL-SN-001",
				"printer_key":  "should-not-be-used",
				"printer_type": "yilianyun",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
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
				expectNoMerchantAccessResolution(store)
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
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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

func TestCreatePrinterAPIRecordsReconciliationOnStoreFailureAfterRemoteRegistration(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	printerClient := &printerClientStub{}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetCloudPrinterBySN(gomock.Any(), gomock.Eq("SN123456789")).
		Times(1).
		Return(db.CloudPrinter{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateCloudPrinter(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CloudPrinter{}, fmt.Errorf("insert failed"))
	store.EXPECT().
		UpsertCloudPrinterReconciliationJob(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CloudPrinterReconciliationJob{}, nil)

	server := newTestServer(t, store)
	server.SetPrinterClientForTest(printerClient)
	recorder := httptest.NewRecorder()

	body := []byte(`{"printer_name":"测试打印机","printer_sn":"SN123456789","printer_key":"KEY123456789","printer_type":"feieyun"}`)
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/devices", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Len(t, printerClient.addInputs, 1)
	require.Empty(t, printerClient.removeInputs)
}

func TestCreatePrinterAPIRegistersShangpengRemotePrinter(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	shangpengClient := &printerClientStub{}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetCloudPrinterBySN(gomock.Any(), gomock.Eq("SP-SN-001")).
		Times(1).
		Return(db.CloudPrinter{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateCloudPrinter(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(ctx context.Context, arg db.CreateCloudPrinterParams) (db.CloudPrinter, error) {
			require.Equal(t, merchant.ID, arg.MerchantID)
			require.Equal(t, "商鹏前台", arg.PrinterName)
			require.Equal(t, "SP-SN-001", arg.PrinterSn)
			require.Equal(t, "SP-KEY-001", arg.PrinterKey)
			require.Equal(t, string(cloudprint.ProviderShangpeng), arg.PrinterType)
			return db.CloudPrinter{
				ID:               1,
				MerchantID:       merchant.ID,
				PrinterName:      arg.PrinterName,
				PrinterSn:        arg.PrinterSn,
				PrinterKey:       arg.PrinterKey,
				PrinterType:      arg.PrinterType,
				PrinterRole:      arg.PrinterRole,
				PrintTakeout:     arg.PrintTakeout,
				PrintDineIn:      arg.PrintDineIn,
				PrintReservation: arg.PrintReservation,
				IsActive:         true,
				CreatedAt:        time.Now(),
			}, nil
		})

	server := newTestServer(t, store)
	server.SetCloudPrinterManagerForTest(printerProviderManagerStub{providers: map[string]cloudprint.Client{
		string(cloudprint.ProviderShangpeng): shangpengClient,
	}})
	recorder := httptest.NewRecorder()

	body := []byte(`{"printer_name":"商鹏前台","printer_sn":"SP-SN-001","printer_key":"SP-KEY-001","printer_type":"shangpeng"}`)
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/devices", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Len(t, shangpengClient.addInputs, 1)
	require.Equal(t, "SP-SN-001", shangpengClient.addInputs[0].SN)
	require.Equal(t, "SP-KEY-001", shangpengClient.addInputs[0].Key)
	require.Equal(t, "商鹏前台", shangpengClient.addInputs[0].Name)
	require.Equal(t, fmt.Sprintf("%d", merchant.ID), shangpengClient.addInputs[0].Business)
}

func TestCreatePrinterAPIRejectsShangpengWhenProviderNotConfigured(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetCloudPrinterBySN(gomock.Any(), gomock.Eq("SP-SN-001")).
		Times(1).
		Return(db.CloudPrinter{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateCloudPrinter(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body := []byte(`{"printer_name":"商鹏前台","printer_sn":"SP-SN-001","printer_key":"SP-KEY-001","printer_type":"shangpeng"}`)
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/devices", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNotImplemented, recorder.Code)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListCloudPrintersByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(printers, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListActiveCloudPrintersByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(printers[:1], nil) // 只返回1个活跃的
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
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
				expectNoMerchantAccessResolution(store)
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
				expectResolveNoAccessibleMerchants(store, user.ID)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
					Times(1).
					Return(printer, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response printerResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
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
				expectNoMerchantAccessResolution(store)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.CloudPrinter{}, db.ErrRecordNotFound)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				expectNoMerchantAccessResolution(store)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.CloudPrinter{}, db.ErrRecordNotFound)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				expectNoMerchantAccessResolution(store)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.CloudPrinter{}, db.ErrRecordNotFound)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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

func TestDeletePrinterAPIRemovesRemotePrinter(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	printer := randomCloudPrinter(merchant.ID)
	printerClient := &printerClientStub{}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
		Times(1).
		Return(printer, nil)
	store.EXPECT().
		DeleteCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
		Times(1).
		Return(nil)

	server := newTestServer(t, store)
	server.SetPrinterClientForTest(printerClient)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/merchant/devices/%d", printer.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, printerClient.removeInputs, 1)
	require.Equal(t, printer.PrinterSn, printerClient.removeInputs[0].SN)
}

func TestDeletePrinterAPIRecordsReconciliationOnStoreFailureAfterRemoteDeletion(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	printer := randomCloudPrinter(merchant.ID)
	printerClient := &printerClientStub{}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
		Times(1).
		Return(printer, nil)
	store.EXPECT().
		DeleteCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
		Times(1).
		Return(fmt.Errorf("delete failed"))
	store.EXPECT().
		UpsertCloudPrinterReconciliationJob(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CloudPrinterReconciliationJob{}, nil)

	server := newTestServer(t, store)
	server.SetPrinterClientForTest(printerClient)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/merchant/devices/%d", printer.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Len(t, printerClient.removeInputs, 1)
	require.Empty(t, printerClient.addInputs)
}

func TestDeletePrinterAPIRemovesShangpengRemotePrinter(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	printer := randomCloudPrinter(merchant.ID)
	printer.PrinterType = string(cloudprint.ProviderShangpeng)
	printer.PrinterSn = "SP-SN-001"
	shangpengClient := &printerClientStub{}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
		Times(1).
		Return(printer, nil)
	store.EXPECT().
		DeleteCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
		Times(1).
		Return(nil)

	server := newTestServer(t, store)
	server.SetCloudPrinterManagerForTest(printerProviderManagerStub{providers: map[string]cloudprint.Client{
		string(cloudprint.ProviderShangpeng): shangpengClient,
	}})
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/merchant/devices/%d", printer.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, shangpengClient.removeInputs, 1)
	require.Equal(t, "SP-SN-001", shangpengClient.removeInputs[0].SN)
	require.Equal(t, fmt.Sprintf("%d", merchant.ID), shangpengClient.removeInputs[0].Business)
}

func TestDeletePrinterAPIRejectsShangpengWhenProviderNotConfigured(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	printer := randomCloudPrinter(merchant.ID)
	printer.PrinterType = string(cloudprint.ProviderShangpeng)
	printer.PrinterSn = "SP-SN-001"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).
		Times(1).
		Return(printer, nil)
	store.EXPECT().
		DeleteCloudPrinter(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/merchant/devices/%d", printer.ID), nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNotImplemented, recorder.Code)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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
				expectNoMerchantAccessResolution(store)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetCloudPrinter(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.CloudPrinter{}, db.ErrRecordNotFound)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

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

func TestGetPrinterLiveStatusAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	printer := randomCloudPrinter(merchant.ID)

	testCases := []struct {
		name          string
		printerID     int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		buildClient   func() *printerClientStub
		buildManager  func(client *printerClientStub) cloudprint.Manager
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder, client *printerClientStub)
	}{
		{
			name:      "OK",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
				store.EXPECT().GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).Times(1).Return(printer, nil)
			},
			buildClient: func() *printerClientStub {
				logo := true
				scan := false
				return &printerClientStub{
					queryPrinterStatus: "在线，工作状态正常",
					queryPrinterInfo: cloudprint.PrinterInfo{
						Model:      "FEIE-80",
						Status:     "online",
						PrintLogo:  &logo,
						ScanSwitch: &scan,
					},
				}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, client *printerClientStub) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp printerLiveStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, printer.ID, resp.PrinterID)
				require.True(t, resp.Online)
				require.True(t, resp.Working)
				require.NotNil(t, resp.Model)
				require.Equal(t, "FEIE-80", *resp.Model)
				require.Equal(t, printer.PrinterSn, client.queryPrinterSN)
				require.Equal(t, printer.PrinterSn, client.queryPrinterInfoSN)
			},
		},
		{
			name:      "ShangpengOK",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
				shangpengPrinter := printer
				shangpengPrinter.PrinterType = printerTypeShangpeng
				shangpengPrinter.PrinterSn = "SP-SN-001"
				store.EXPECT().GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).Times(1).Return(shangpengPrinter, nil)
			},
			buildClient: func() *printerClientStub {
				return &printerClientStub{
					queryPrinterStatus: "online",
					queryPrinterInfo: cloudprint.PrinterInfo{
						Model:  "SP-P1",
						Status: "online",
					},
				}
			},
			buildManager: func(client *printerClientStub) cloudprint.Manager {
				return printerProviderManagerStub{providers: map[string]cloudprint.Client{
					printerTypeShangpeng: client,
				}}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, client *printerClientStub) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp printerLiveStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, printerTypeShangpeng, resp.PrinterType)
				require.Equal(t, "online", resp.ProviderStatus)
				require.True(t, resp.Online)
				require.True(t, resp.Working)
				require.NotNil(t, resp.Model)
				require.Equal(t, "SP-P1", *resp.Model)
				require.Empty(t, client.queryPrinterSN)
				require.Equal(t, "SP-SN-001", client.queryPrinterInfoSN)
			},
		},
		{
			name:      "PrinterNotBelongToMerchant",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
				otherPrinter := printer
				otherPrinter.MerchantID = merchant.ID + 1
				store.EXPECT().GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).Times(1).Return(otherPrinter, nil)
			},
			buildClient: func() *printerClientStub { return &printerClientStub{} },
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, client *printerClientStub) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
				require.Empty(t, client.queryPrinterSN)
			},
		},
		{
			name:      "YilianyunLiveStatusNotEnabled",
			printerID: printer.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
				yilianyunPrinter := printer
				yilianyunPrinter.PrinterType = string(cloudprint.ProviderYilianyun)
				yilianyunPrinter.PrinterSn = "YL-SN-001"
				store.EXPECT().GetCloudPrinter(gomock.Any(), gomock.Eq(printer.ID)).Times(1).Return(yilianyunPrinter, nil)
			},
			buildClient: func() *printerClientStub { return &printerClientStub{} },
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, client *printerClientStub) {
				require.Equal(t, http.StatusNotImplemented, recorder.Code)
				require.Empty(t, client.queryPrinterSN)
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
			client := tc.buildClient()
			if tc.buildManager != nil {
				server.SetCloudPrinterManagerForTest(tc.buildManager(client))
			} else {
				server.SetPrinterClientForTest(client)
			}
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchant/devices/%d/status", tc.printerID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder, client)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetOrderDisplayConfigByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(config, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response getDisplayConfigResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, config.ID, response.ID)
				require.Equal(t, config.EnablePrint, response.EnablePrint)
				require.Equal(t, config.AutoAcceptPaidOrders, response.AutoAcceptPaidOrders)
			},
		},
		{
			name: "OK_NoConfigReturnsDefault",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetOrderDisplayConfigByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(db.OrderDisplayConfig{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response getDisplayConfigResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				// 验证默认值
				require.Equal(t, true, response.EnablePrint)
				require.Equal(t, true, response.PrintTakeout)
				require.Equal(t, false, response.AutoAcceptPaidOrders)
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
				expectNoMerchantAccessResolution(store)
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
				expectResolveNoAccessibleMerchants(store, user.ID)
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
				"enable_print":            false,
				"auto_accept_paid_orders": true,
				"enable_voice":            true,
				"voice_takeout":           false,
				"voice_dine_in":           false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetOrderDisplayConfigByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(config, nil)

				updatedConfig := config
				updatedConfig.EnablePrint = false
				updatedConfig.AutoAcceptPaidOrders = true
				store.EXPECT().
					UpdateOrderDisplayConfig(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.UpdateOrderDisplayConfigParams) (db.OrderDisplayConfig, error) {
						require.True(t, arg.AutoAcceptPaidOrders.Valid)
						require.True(t, arg.AutoAcceptPaidOrders.Bool)
						require.False(t, arg.EnableVoice.Valid)
						require.False(t, arg.VoiceTakeout.Valid)
						require.False(t, arg.VoiceDineIn.Valid)
						return updatedConfig, nil
					}).
					Times(1)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response getDisplayConfigResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, false, response.EnablePrint)
				require.Equal(t, true, response.AutoAcceptPaidOrders)
				require.Equal(t, config.EnableVoice, response.EnableVoice)
				require.Equal(t, config.VoiceTakeout, response.VoiceTakeout)
				require.Equal(t, config.VoiceDineIn, response.VoiceDineIn)
			},
		},
		{
			name: "OK_CreateNew",
			body: map[string]interface{}{
				"auto_accept_paid_orders": true,
				"enable_kds":              true,
				"enable_voice":            true,
				"voice_takeout":           false,
				"voice_dine_in":           false,
				"kds_url":                 "https://kds.example.com",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetOrderDisplayConfigByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(db.OrderDisplayConfig{}, db.ErrRecordNotFound)

				newConfig := config
				newConfig.AutoAcceptPaidOrders = true
				newConfig.EnableKds = true
				newConfig.KdsUrl = pgtype.Text{String: "https://kds.example.com", Valid: true}
				store.EXPECT().
					CreateOrderDisplayConfig(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.CreateOrderDisplayConfigParams) (db.OrderDisplayConfig, error) {
						require.True(t, arg.AutoAcceptPaidOrders)
						require.False(t, arg.EnableVoice)
						require.True(t, arg.VoiceTakeout)
						require.True(t, arg.VoiceDineIn)
						return newConfig, nil
					}).
					Times(1)
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
				expectNoMerchantAccessResolution(store)
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
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
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
