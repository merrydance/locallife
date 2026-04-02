package api

import (
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
