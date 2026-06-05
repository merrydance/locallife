package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestCreateYilianyunAuthorizationSessionAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	now := time.Now().UTC()

	testCases := []struct {
		name          string
		body          map[string]any
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		oauth         *fakeYilianyunOAuthClient
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]any{
				"printer_name": "前台易联云",
				"printer_role": "front",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
				store.EXPECT().CreateCloudPrinterAuthorizationSession(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.CreateCloudPrinterAuthorizationSessionParams) (db.CloudPrinterAuthorizationSession, error) {
						require.Equal(t, merchant.ID, arg.MerchantID)
						require.Equal(t, db.CloudPrinterProviderYilianyun, arg.ProviderType)
						require.Equal(t, "前台易联云", arg.PrinterName.String)
						require.Equal(t, "front", arg.PrinterRole.String)
						require.Equal(t, user.ID, arg.CreatedBy.Int64)
						return db.CloudPrinterAuthorizationSession{
							ID:           88,
							State:        arg.State,
							MerchantID:   merchant.ID,
							ProviderType: db.CloudPrinterProviderYilianyun,
							PrinterName:  arg.PrinterName,
							PrinterRole:  arg.PrinterRole,
							CreatedBy:    arg.CreatedBy,
							ExpiresAt:    now.Add(10 * time.Minute),
						}, nil
					})
			},
			oauth: &fakeYilianyunOAuthClient{
				buildAuthorizeURL: func(state string) (string, error) {
					return "https://open-api.10ss.net/oauth/authorize?state=" + state, nil
				},
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				var resp yilianyunAuthorizationSessionResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, int64(88), resp.SessionID)
				require.NotEmpty(t, resp.State)
				require.Contains(t, resp.AuthorizeURL, "/oauth/authorize")
			},
		},
		{
			name: "NoAuthorization",
			body: map[string]any{"printer_name": "前台易联云"},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
			},
			oauth: &fakeYilianyunOAuthClient{},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "OAuthNotConfigured",
			body: map[string]any{"printer_name": "前台易联云"},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			oauth: nil,
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
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
			if tc.oauth != nil {
				server.yilianyunOAuthClient = tc.oauth
			}

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)
			request, err := http.NewRequest(http.MethodPost, "/v1/merchant/devices/yilianyun/authorization-sessions", bytes.NewReader(body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server.tokenMaker)
			recorder := httptest.NewRecorder()

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestYilianyunAuthorizationCallbackDoesNotRequireBearerAndStoresEncryptedTokens(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	now := time.Now().UTC()
	session := db.CloudPrinterAuthorizationSession{
		ID:           99,
		State:        "state-ok",
		MerchantID:   11,
		ProviderType: db.CloudPrinterProviderYilianyun,
		ExpiresAt:    now.Add(5 * time.Minute),
	}
	store.EXPECT().GetActiveCloudPrinterAuthorizationSessionForUpdate(gomock.Any(), "state-ok").Return(session, nil)
	store.EXPECT().AuthorizeYilianyunCloudPrinterWithDeviceTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.AuthorizeYilianyunCloudPrinterWithDeviceTxParams) (db.AuthorizeYilianyunCloudPrinterWithDeviceTxResult, error) {
			require.Equal(t, "state-ok", arg.State)
			require.Equal(t, int64(11), arg.Authorization.MerchantID)
			require.Equal(t, "YL-MACHINE-CB", arg.Authorization.MachineCode)
			require.False(t, arg.Authorization.AuthorizedCloudPrinterID.Valid)
			require.NotContains(t, arg.Authorization.AccessTokenCiphertext, "callback-access-secret")
			require.NotContains(t, arg.Authorization.RefreshTokenCiphertext, "callback-refresh-secret")
			require.Equal(t, int64(11), arg.Printer.MerchantID)
			require.Equal(t, "YL-MACHINE-CB", arg.Printer.PrinterSn)
			require.Empty(t, arg.Printer.PrinterKey)
			require.Equal(t, db.CloudPrinterProviderYilianyun, arg.Printer.PrinterType)
			return db.AuthorizeYilianyunCloudPrinterWithDeviceTxResult{
				Session: session,
				Printer: db.CloudPrinter{
					ID:               321,
					MerchantID:       11,
					PrinterName:      "易联云 YL-MACHINE-CB",
					PrinterSn:        "YL-MACHINE-CB",
					PrinterType:      db.CloudPrinterProviderYilianyun,
					PrinterRole:      "front",
					PrintTakeout:     true,
					PrintDineIn:      true,
					PrintReservation: true,
					IsActive:         true,
					CreatedAt:        now,
				},
				Authorization: db.CloudPrinterProviderAuthorization{
					ID:           123,
					MerchantID:   11,
					ProviderType: db.CloudPrinterProviderYilianyun,
					MachineCode:  "YL-MACHINE-CB",
					Status:       db.CloudPrinterAuthorizationStatusActive,
					AuthorizedCloudPrinterID: pgtype.Int8{
						Int64: 321,
						Valid: true,
					},
					AccessTokenExpiresAt:  now.Add(time.Hour),
					RefreshTokenExpiresAt: now.Add(35 * 24 * time.Hour),
				},
			}, nil
		})
	server := newTestServer(t, store)
	server.dataEncryptor = mustTestEncryptor(t)
	server.yilianyunOAuthClient = &fakeYilianyunOAuthClient{
		exchangeAuthorizationCode: func(_ context.Context, code string) (cloudprint.YilianyunAuthorizationToken, error) {
			require.Equal(t, "code-ok", code)
			return cloudprint.YilianyunAuthorizationToken{
				AccessToken:      "callback-access-secret",
				RefreshToken:     "callback-refresh-secret",
				MachineCode:      "YL-MACHINE-CB",
				ExpiresInSeconds: 3600,
			}, nil
		},
	}

	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/devices/yilianyun/auth/callback?code=code-ok&state=state-ok", nil)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "callback-access-secret")
	require.NotContains(t, recorder.Body.String(), "callback-refresh-secret")
	var resp yilianyunAuthorizationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(123), resp.AuthorizationID)
	require.Equal(t, "YL-MACHINE-CB", resp.MachineCode)
	require.NotNil(t, resp.Printer)
	require.Equal(t, int64(321), resp.Printer.ID)
	require.Equal(t, "YL-MACHINE-CB", resp.Printer.PrinterSN)
}

func TestYilianyunAuthorizationCallbackProviderErrorUsesSafeResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetActiveCloudPrinterAuthorizationSessionForUpdate(gomock.Any(), "state-ok").Return(db.CloudPrinterAuthorizationSession{
		ID:           99,
		State:        "state-ok",
		MerchantID:   11,
		ProviderType: db.CloudPrinterProviderYilianyun,
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}, nil)
	server := newTestServer(t, store)
	server.yilianyunOAuthClient = &fakeYilianyunOAuthClient{
		exchangeAuthorizationCode: func(context.Context, string) (cloudprint.YilianyunAuthorizationToken, error) {
			return cloudprint.YilianyunAuthorizationToken{}, errors.New("raw provider payload access-token-secret refresh-token-secret")
		},
	}

	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/devices/yilianyun/auth/callback?code=code-ok&state=state-ok", nil)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "access-token-secret")
	require.NotContains(t, recorder.Body.String(), "refresh-token-secret")
	require.Contains(t, recorder.Body.String(), "易联云授权失败")
}

func TestAuthorizeScannedYilianyunPrinterAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	now := time.Now().UTC()

	testCases := []struct {
		name          string
		body          string
		buildStubs    func(store *mockdb.MockStore)
		oauth         *fakeYilianyunOAuthClient
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OKWithQRKey",
			body: `{"machine_code":"YL-SCAN-001","qr_key":"qr-secret","printer_name":"前台易联云","printer_role":"front"}`,
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
				store.EXPECT().CreateAuthorizedYilianyunCloudPrinterTx(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.CreateAuthorizedYilianyunCloudPrinterTxParams) (db.CreateAuthorizedYilianyunCloudPrinterTxResult, error) {
						require.Equal(t, merchant.ID, arg.Authorization.MerchantID)
						require.Equal(t, db.CloudPrinterProviderYilianyun, arg.Authorization.ProviderType)
						require.Equal(t, "YL-SCAN-001", arg.Authorization.MachineCode)
						require.False(t, arg.Authorization.AuthorizedCloudPrinterID.Valid)
						require.NotContains(t, arg.Authorization.AccessTokenCiphertext, "scan-access-secret")
						require.NotContains(t, arg.Authorization.RefreshTokenCiphertext, "scan-refresh-secret")
						require.Equal(t, merchant.ID, arg.Printer.MerchantID)
						require.Equal(t, "前台易联云", arg.Printer.PrinterName)
						require.Equal(t, "YL-SCAN-001", arg.Printer.PrinterSn)
						require.Empty(t, arg.Printer.PrinterKey)
						require.Equal(t, db.CloudPrinterProviderYilianyun, arg.Printer.PrinterType)
						require.Equal(t, "front", arg.Printer.PrinterRole)
						return db.CreateAuthorizedYilianyunCloudPrinterTxResult{
							Printer: db.CloudPrinter{
								ID:               654,
								MerchantID:       merchant.ID,
								PrinterName:      "前台易联云",
								PrinterSn:        "YL-SCAN-001",
								PrinterType:      db.CloudPrinterProviderYilianyun,
								PrinterRole:      "front",
								PrintTakeout:     true,
								PrintDineIn:      true,
								PrintReservation: true,
								IsActive:         true,
								CreatedAt:        now,
							},
							Authorization: db.CloudPrinterProviderAuthorization{
								ID:           456,
								MerchantID:   merchant.ID,
								ProviderType: db.CloudPrinterProviderYilianyun,
								MachineCode:  "YL-SCAN-001",
								Status:       db.CloudPrinterAuthorizationStatusActive,
								AuthorizedCloudPrinterID: pgtype.Int8{
									Int64: 654,
									Valid: true,
								},
								AccessTokenExpiresAt:  now.Add(time.Hour),
								RefreshTokenExpiresAt: now.Add(35 * 24 * time.Hour),
							},
						}, nil
					})
			},
			oauth: &fakeYilianyunOAuthClient{
				authorizeScannedPrinter: func(_ context.Context, input cloudprint.YilianyunScannedPrinterAuthorizationInput) (cloudprint.YilianyunAuthorizationToken, error) {
					require.Equal(t, "YL-SCAN-001", input.MachineCode)
					require.Equal(t, "qr-secret", input.QRKey)
					require.Empty(t, input.MSign)
					return cloudprint.YilianyunAuthorizationToken{
						AccessToken:      "scan-access-secret",
						RefreshToken:     "scan-refresh-secret",
						MachineCode:      "YL-SCAN-001",
						ExpiresInSeconds: 3600,
					}, nil
				},
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				require.NotContains(t, recorder.Body.String(), "scan-access-secret")
				var resp yilianyunAuthorizationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, int64(456), resp.AuthorizationID)
				require.Equal(t, "YL-SCAN-001", resp.MachineCode)
				require.NotNil(t, resp.Printer)
				require.Equal(t, int64(654), resp.Printer.ID)
				require.Equal(t, "前台易联云", resp.Printer.PrinterName)
				require.Equal(t, "YL-SCAN-001", resp.Printer.PrinterSN)
			},
		},
		{
			name: "RejectsBothQRKeyAndMSign",
			body: `{"machine_code":"YL-SCAN-001","qr_key":"qr-secret","msign":"msign-secret"}`,
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			oauth: &fakeYilianyunOAuthClient{},
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
			server.dataEncryptor = mustTestEncryptor(t)
			server.yilianyunOAuthClient = tc.oauth

			request, err := http.NewRequest(http.MethodPost, "/v1/merchant/devices/yilianyun/scan-authorizations", strings.NewReader(tc.body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			recorder := httptest.NewRecorder()

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func mustTestEncryptor(t *testing.T) util.DataEncryptor {
	encryptor, err := util.NewAESEncryptor("12345678901234567890123456789012")
	require.NoError(t, err)
	return encryptor
}

type fakeYilianyunOAuthClient struct {
	buildAuthorizeURL         func(string) (string, error)
	exchangeAuthorizationCode func(context.Context, string) (cloudprint.YilianyunAuthorizationToken, error)
	authorizeScannedPrinter   func(context.Context, cloudprint.YilianyunScannedPrinterAuthorizationInput) (cloudprint.YilianyunAuthorizationToken, error)
}

func (c *fakeYilianyunOAuthClient) BuildAuthorizeURL(state string) (string, error) {
	if c.buildAuthorizeURL == nil {
		return "", errors.New("unexpected BuildAuthorizeURL")
	}
	return c.buildAuthorizeURL(state)
}

func (c *fakeYilianyunOAuthClient) ExchangeAuthorizationCode(ctx context.Context, code string) (cloudprint.YilianyunAuthorizationToken, error) {
	if c.exchangeAuthorizationCode == nil {
		return cloudprint.YilianyunAuthorizationToken{}, errors.New("unexpected ExchangeAuthorizationCode")
	}
	return c.exchangeAuthorizationCode(ctx, code)
}

func (c *fakeYilianyunOAuthClient) AuthorizeScannedPrinter(ctx context.Context, input cloudprint.YilianyunScannedPrinterAuthorizationInput) (cloudprint.YilianyunAuthorizationToken, error) {
	if c.authorizeScannedPrinter == nil {
		return cloudprint.YilianyunAuthorizationToken{}, errors.New("unexpected AuthorizeScannedPrinter")
	}
	return c.authorizeScannedPrinter(ctx, input)
}
