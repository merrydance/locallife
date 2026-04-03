package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestWithdrawOperatorAPI(t *testing.T) {
	user, _ := randomUser(t)
	activeOperator := db.Operator{
		ID:       1001,
		UserID:   user.ID,
		RegionID: 1,
		Status:   "active",
		SubMchID: pgtype.Text{String: "sub_mch_operator_001", Valid: true},
	}

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, req *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
		useEcommerce  bool
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"amount":         1200,
				"remark":         "测试提现",
				"out_request_no": "OW1001",
			},
			setupAuth: func(t *testing.T, req *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, req, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: activeOperator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(2).
					Return(activeOperator, nil)

				ecommerce.EXPECT().
					QueryEcommerceFundBalance(gomock.Any(), activeOperator.SubMchID.String).
					Return(&wechat.EcommerceFundBalanceResponse{
						SubMchID:           activeOperator.SubMchID.String,
						AvailableAmount:    100000,
						PendingAmount:      0,
						WithdrawableAmount: 100000,
					}, nil)

				store.EXPECT().
					GetWithdrawalRecordByOutRequestNo(gomock.Any(), pgtype.Text{String: "OW1001", Valid: true}).
					Return(db.WithdrawalRecord{}, db.ErrRecordNotFound)

				store.EXPECT().
					CreateWithdrawalRecord(gomock.Any(), gomock.Any()).
					Return(db.WithdrawalRecord{
						ID:        88,
						UserID:    user.ID,
						Amount:    1200,
						Status:    "pending",
						Channel:   operatorWithdrawChannel,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					}, nil)

				ecommerce.EXPECT().
					CreateEcommerceWithdraw(gomock.Any(), gomock.Any()).
					Return(&wechat.EcommerceWithdrawResponse{
						SubMchID:     activeOperator.SubMchID.String,
						OutRequestNo: "ow_test_001",
						WithdrawID:   "withdraw_test_001",
						Amount:       1200,
						Status:       "PROCESSING",
					}, nil)

				store.EXPECT().
					UpdateWithdrawalAccountInfo(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ interface{}, arg db.UpdateWithdrawalAccountInfoParams) (db.WithdrawalRecord, error) {
						require.Equal(t, int64(88), arg.ID)
						info := parseOperatorWithdrawAccountInfo(arg.AccountInfo)
						require.Equal(t, "OW1001", info.OutRequestNo)
						require.Equal(t, "withdraw_test_001", info.WithdrawID)
						return db.WithdrawalRecord{
							ID:          88,
							UserID:      user.ID,
							Amount:      1200,
							Status:      "pending",
							Channel:     operatorWithdrawChannel,
							AccountInfo: arg.AccountInfo,
							CreatedAt:   time.Now(),
							UpdatedAt:   time.Now(),
						}, nil
					})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.NotNil(t, resp["withdrawal"])
			},
			useEcommerce: true,
		},
		{
			name: "InsufficientWithdrawableBalance",
			body: map[string]interface{}{
				"amount": 9999,
			},
			setupAuth: func(t *testing.T, req *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, req, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: activeOperator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(2).
					Return(activeOperator, nil)

				ecommerce.EXPECT().
					QueryEcommerceFundBalance(gomock.Any(), activeOperator.SubMchID.String).
					Return(&wechat.EcommerceFundBalanceResponse{
						SubMchID:           activeOperator.SubMchID.String,
						WithdrawableAmount: 100,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
			useEcommerce: true,
		},
		{
			name: "EcommerceClientNotConfigured",
			body: map[string]interface{}{
				"amount": 1200,
			},
			setupAuth: func(t *testing.T, req *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, req, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{
						UserID:          user.ID,
						Role:            "operator",
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: activeOperator.RegionID, Valid: true},
					}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(activeOperator, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			},
			useEcommerce: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

				store := newMockStoreWithAlertSink(ctrl)
			var ecommerce *mockwechat.MockEcommerceClientInterface
			if tc.useEcommerce {
				ecommerce = mockwechat.NewMockEcommerceClientInterface(ctrl)
			}

			tc.buildStubs(store, ecommerce)

			server := newTestServer(t, store)
			if tc.useEcommerce {
				server.SetEcommerceClientForTest(ecommerce)
			}

			recorder := httptest.NewRecorder()
			bodyBytes, err := json.Marshal(tc.body)
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPost, "/v1/operators/me/finance/withdraw", bytes.NewReader(bodyBytes))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, req, server.tokenMaker)
			server.router.ServeHTTP(recorder, req)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetOperatorAccountBalanceAPI(t *testing.T) {
	user, _ := randomUser(t)
	activeOperator := db.Operator{
		ID:       1001,
		UserID:   user.ID,
		RegionID: 1,
		Status:   "active",
		SubMchID: pgtype.Text{String: "sub_mch_operator_001", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListUserRoles(gomock.Any(), user.ID).
		Return([]db.UserRole{{
			UserID:          user.ID,
			Role:            "operator",
			Status:          "active",
			RelatedEntityID: pgtype.Int8{Int64: activeOperator.RegionID, Valid: true},
		}}, nil)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(2).
		Return(activeOperator, nil)

	ecommerce.EXPECT().
		QueryEcommerceFundBalance(gomock.Any(), activeOperator.SubMchID.String).
		Return(&wechat.EcommerceFundBalanceResponse{
			SubMchID:           activeOperator.SubMchID.String,
			AvailableAmount:    123456,
			PendingAmount:      789,
			WithdrawableAmount: 120000,
		}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/operators/me/finance/account/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp operatorAccountBalanceResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(123456), resp.AvailableAmount)
	require.Equal(t, int64(120000), resp.WithdrawableAmount)
}

func TestGetOperatorAccountBalanceAPI_NotConfigured(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{
		ID:       1002,
		UserID:   user.ID,
		RegionID: 1,
		Status:   "active",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListUserRoles(gomock.Any(), user.ID).
		Return([]db.UserRole{{
			UserID:          user.ID,
			Role:            "operator",
			Status:          "active",
			RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
		}}, nil)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(2).
		Return(operator, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/operators/me/finance/account/balance", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp operatorAccountBalanceResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "not_configured", resp.AccountStatus)
	require.NotEmpty(t, resp.StatusDesc)
	require.Empty(t, resp.SubMchID)
}

func TestListOperatorWithdrawalsAPI(t *testing.T) {
	user, _ := randomUser(t)
	activeOperator := db.Operator{
		ID:       1001,
		UserID:   user.ID,
		RegionID: 1,
		Status:   "active",
		SubMchID: pgtype.Text{String: "sub_mch_operator_001", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	store.EXPECT().
		ListUserRoles(gomock.Any(), user.ID).
		Return([]db.UserRole{{
			UserID:          user.ID,
			Role:            "operator",
			Status:          "active",
			RelatedEntityID: pgtype.Int8{Int64: activeOperator.RegionID, Valid: true},
		}}, nil)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(2).
		Return(activeOperator, nil)

	accountInfoBytes, err := json.Marshal(operatorWithdrawAccountInfo{
		OperatorID:   activeOperator.ID,
		SubMchID:     activeOperator.SubMchID.String,
		OutRequestNo: "ow_test_001",
	})
	require.NoError(t, err)

	store.EXPECT().
		ListWithdrawalRecords(gomock.Any(), gomock.Eq(db.ListWithdrawalRecordsParams{
			UserID:  user.ID,
			Channel: operatorWithdrawChannel,
			Limit:   20,
			Offset:  0,
		})).
		Return([]db.WithdrawalRecord{
			{
				ID:          11,
				UserID:      user.ID,
				Amount:      1000,
				Status:      "pending",
				Channel:     operatorWithdrawChannel,
				AccountInfo: accountInfoBytes,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		}, nil)

	store.EXPECT().
		CountWithdrawalRecords(gomock.Any(), db.CountWithdrawalRecordsParams{
			UserID:  user.ID,
			Channel: operatorWithdrawChannel,
		}).
		Return(int64(1), nil)

	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/operators/me/finance/withdrawals?page=1&limit=20", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Withdrawals []operatorWithdrawItem `json:"withdrawals"`
		Total       int64                  `json:"total"`
	}
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Withdrawals, 1)
	require.Equal(t, int64(11), resp.Withdrawals[0].ID)
	require.Equal(t, int64(1), resp.Total)
}

func TestGetOperatorWithdrawalAPI(t *testing.T) {
	user, _ := randomUser(t)
	activeOperator := db.Operator{
		ID:       1001,
		UserID:   user.ID,
		RegionID: 1,
		Status:   "active",
		SubMchID: pgtype.Text{String: "sub_mch_operator_001", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListUserRoles(gomock.Any(), user.ID).
		Return([]db.UserRole{{
			UserID:          user.ID,
			Role:            "operator",
			Status:          "active",
			RelatedEntityID: pgtype.Int8{Int64: activeOperator.RegionID, Valid: true},
		}}, nil)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Times(2).
		Return(activeOperator, nil)

	accountInfoBytes, err := json.Marshal(operatorWithdrawAccountInfo{
		OperatorID:   activeOperator.ID,
		SubMchID:     activeOperator.SubMchID.String,
		OutRequestNo: "ow_test_001",
	})
	require.NoError(t, err)

	store.EXPECT().
		GetWithdrawalRecord(gomock.Any(), int64(88)).
		Return(db.WithdrawalRecord{
			ID:          88,
			UserID:      user.ID,
			Amount:      1200,
			Status:      "pending",
			Channel:     operatorWithdrawChannel,
			AccountInfo: accountInfoBytes,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}, nil)

	ecommerce.EXPECT().
		QueryEcommerceWithdrawByOutRequestNo(gomock.Any(), activeOperator.SubMchID.String, "ow_test_001").
		Return(&wechat.EcommerceWithdrawResponse{
			SubMchID:     activeOperator.SubMchID.String,
			OutRequestNo: "ow_test_001",
			WithdrawID:   "withdraw_test_001",
			Amount:       1200,
			Status:       "PROCESSING",
		}, nil)

	store.EXPECT().
		UpdateWithdrawalAccountInfo(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, arg db.UpdateWithdrawalAccountInfoParams) (db.WithdrawalRecord, error) {
			require.Equal(t, int64(88), arg.ID)
			info := parseOperatorWithdrawAccountInfo(arg.AccountInfo)
			require.Equal(t, "withdraw_test_001", info.WithdrawID)
			return db.WithdrawalRecord{
				ID:          88,
				UserID:      user.ID,
				Amount:      1200,
				Status:      "pending",
				Channel:     operatorWithdrawChannel,
				AccountInfo: arg.AccountInfo,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}, nil
		})

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/operators/me/finance/withdrawals/88", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Withdrawal operatorWithdrawItem `json:"withdrawal"`
	}
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(88), resp.Withdrawal.ID)
	require.Equal(t, "pending", resp.Withdrawal.Status)
	require.Equal(t, "withdraw_test_001", resp.Withdrawal.WithdrawID)
}
