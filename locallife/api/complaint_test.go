package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func randomWechatComplaintForTest(merchantID int64) db.WechatComplaint {
	now := time.Now()
	return db.WechatComplaint{
		ID:              1,
		ComplaintID:     "complaint_test_001",
		ComplaintTime:   now,
		ComplaintDetail: "test complaint",
		ComplaintState:  "PROCESSING",
		MerchantID:      pgtype.Int8{Int64: merchantID, Valid: true},
		Amount:          1000,
		LastSyncedAt:    pgtype.Timestamptz{Time: now, Valid: true},
		CreatedAt:       now,
		UpdatedAt:       pgtype.Timestamptz{Time: now, Valid: true},
	}
}

func TestHandleComplaintNotify_SerialHeaderForwarded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	ecommerce.EXPECT().
		VerifyNotificationSignature(gomock.Eq("test_signature"), gomock.Eq("1712808000"), gomock.Eq("test_nonce"), gomock.Eq("test_serial"), gomock.Any()).
		Return(nil)
	store.EXPECT().CheckNotificationExists(gomock.Any(), "complaint-notify-001").Return(false, nil)
	ecommerce.EXPECT().DecryptComplaintNotification(gomock.Any()).Return(&wechat.ComplaintNotification{
		ComplaintID: "complaint_test_001",
		ActionType:  "COMPLAINT_STATE_CHANGE",
		State:       wechat.ComplaintStateProcessing,
	}, nil)
	store.EXPECT().UpdateWechatComplaintState(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.UpdateWechatComplaintStateParams) (db.WechatComplaint, error) {
		require.Equal(t, "complaint_test_001", arg.ComplaintID)
		require.Equal(t, string(wechat.ComplaintStateProcessing), arg.ComplaintState)
		require.True(t, arg.WxpayUpdateTime.Valid)
		return randomWechatComplaintForTest(0), nil
	})

	recorder := httptest.NewRecorder()
	req := newComplaintNotifyRequest(t)
	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"code":"SUCCESS","message":"OK"}`, recorder.Body.String())
}

func newComplaintNotifyRequest(t *testing.T) *http.Request {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"id":            "complaint-notify-001",
		"create_time":   "2026-04-11T12:00:00+08:00",
		"event_type":    "COMPLAINT.STATE_CHANGE",
		"resource_type": "encrypt-resource",
		"summary":       "complaint notify",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "ciphertext",
			"nonce":           "nonce-value",
			"associated_data": "complaint",
			"original_type":   "complaint",
		},
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-ecommerce/complaint-notify", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Wechatpay-Signature", "test_signature")
	req.Header.Set("Wechatpay-Serial", "test_serial")
	req.Header.Set("Wechatpay-Timestamp", "1712808000")
	req.Header.Set("Wechatpay-Nonce", "test_nonce")
	return req
}

func TestCompleteComplaintAPI(t *testing.T) {
	merchantUser, _ := randomUser(t)
	merchant := randomMerchant(merchantUser.ID)
	operatorUser, _ := randomUser(t)
	operator := randomOperator(operatorUser.ID)
	complaint := randomWechatComplaintForTest(merchant.ID)
	completedComplaint := complaint
	completedComplaint.ComplaintState = string(wechat.ComplaintStateProcessed)
	completedComplaint.CompletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}

	testCases := []struct {
		name          string
		path          string
		userID        int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "MerchantOwnComplaintOK",
			path:   "/v1/merchant/complaints/" + complaint.ComplaintID + "/complete",
			userID: merchantUser.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface) {
				expectResolveSingleOwnedMerchant(store, merchantUser.ID, merchant)

				store.EXPECT().
					GetWechatComplaintByComplaintIDForUpdate(gomock.Any(), gomock.Eq(complaint.ComplaintID)).
					Times(1).
					Return(complaint, nil)

				ecommerce.EXPECT().
					CompleteComplaint(gomock.Any(), gomock.Eq(complaint.ComplaintID)).
					Times(1).
					Return(nil)

				store.EXPECT().
					UpdateWechatComplaintCompleted(gomock.Any(), gomock.Eq(complaint.ID)).
					Times(1).
					Return(completedComplaint, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:   "MerchantOtherComplaintForbidden",
			path:   "/v1/merchant/complaints/" + complaint.ComplaintID + "/complete",
			userID: merchantUser.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface) {
				otherComplaint := complaint
				otherComplaint.MerchantID = pgtype.Int8{Int64: merchant.ID + 999, Valid: true}

				expectResolveSingleOwnedMerchant(store, merchantUser.ID, merchant)

				store.EXPECT().
					GetWechatComplaintByComplaintIDForUpdate(gomock.Any(), gomock.Eq(complaint.ComplaintID)).
					Times(1).
					Return(otherComplaint, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:   "OperatorCanCompleteComplaint",
			path:   "/v1/operators/me/complaints/" + complaint.ComplaintID + "/complete",
			userID: operatorUser.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, operatorUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface) {
				expectActiveOperatorAuth(store, operatorUser.ID, operator)

				store.EXPECT().
					GetWechatComplaintByComplaintIDForUpdate(gomock.Any(), gomock.Eq(complaint.ComplaintID)).
					Times(1).
					Return(complaint, nil)

				ecommerce.EXPECT().
					CompleteComplaint(gomock.Any(), gomock.Eq(complaint.ComplaintID)).
					Times(1).
					Return(nil)

				store.EXPECT().
					UpdateWechatComplaintCompleted(gomock.Any(), gomock.Eq(complaint.ID)).
					Times(1).
					Return(completedComplaint, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:   "OperatorComplaintWechatFailureReturnsStableBadGateway",
			path:   "/v1/operators/me/complaints/" + complaint.ComplaintID + "/complete",
			userID: operatorUser.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, operatorUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ecommerce *mockwechat.MockEcommerceClientInterface) {
				expectActiveOperatorAuth(store, operatorUser.ID, operator)

				store.EXPECT().
					GetWechatComplaintByComplaintIDForUpdate(gomock.Any(), gomock.Eq(complaint.ComplaintID)).
					Times(1).
					Return(complaint, nil)

				ecommerce.EXPECT().
					CompleteComplaint(gomock.Any(), gomock.Eq(complaint.ComplaintID)).
					Times(1).
					Return(assertAnError("wechat unavailable"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadGateway, recorder.Code)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, CodeBadGateway, resp.Code)
				require.Equal(t, "failed to complete complaint via WeChat", resp.Message)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, ecommerce)

			server := newTestServer(t, store)
			server.SetEcommerceClientForTest(ecommerce)

			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(http.MethodPost, tc.path, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
