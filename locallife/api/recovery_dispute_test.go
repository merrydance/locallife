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

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ====================== Helper Functions ======================

func randomRecoveryDispute(claimID, appellantID, regionID int64, appellantType string) db.RecoveryDispute {
	return db.RecoveryDispute{
		ID:            util.RandomInt(1, 1000),
		ClaimID:       claimID,
		AppellantType: appellantType,
		AppellantID:   appellantID,
		Reason:        "订单包装完好，顾客收货时已当面核对",
		Status:        "submitted",
		RegionID:      regionID,
		CreatedAt:     time.Now(),
	}
}

func reviewedRecoveryDispute(base db.RecoveryDispute, status, notes string) db.RecoveryDispute {
	return db.RecoveryDispute{
		ID:                 base.ID,
		ClaimID:            base.ClaimID,
		AppellantType:      base.AppellantType,
		AppellantID:        base.AppellantID,
		Reason:             base.Reason,
		Status:             status,
		ReviewerID:         base.ReviewerID,
		ReviewNotes:        pgtype.Text{String: notes, Valid: true},
		ReviewedAt:         pgtype.Timestamptz{Time: time.Now(), Valid: true},
		CompensationAmount: base.CompensationAmount,
		CompensatedAt:      base.CompensatedAt,
		RegionID:           base.RegionID,
		CreatedAt:          base.CreatedAt,
	}
}

// randomRecoveryDisputeRider 生成随机骑手（带区域ID）用于申诉测试
func randomRecoveryDisputeRider(userID, regionID int64) db.Rider {
	return db.Rider{
		ID:            util.RandomInt(1, 1000),
		UserID:        userID,
		RealName:      util.RandomString(6),
		Phone:         "139" + util.RandomString(8),
		IDCardNo:      util.RandomString(18),
		Status:        "active",
		DepositAmount: 30000,
		IsOnline:      true,
		RegionID:      pgtype.Int8{Int64: regionID, Valid: true},
		CreatedAt:     time.Now(),
	}
}

// randomRecoveryDisputeOperator 生成随机运营商（带区域ID）用于申诉测试
func randomRecoveryDisputeOperator(userID, regionID int64) db.Operator {
	return db.Operator{
		ID:           util.RandomInt(1, 1000),
		UserID:       userID,
		Name:         util.RandomString(10),
		ContactName:  util.RandomString(6),
		ContactPhone: util.RandomString(11),
		RegionID:     regionID,
		Status:       "active",
		CreatedAt:    time.Now(),
	}
}

// ====================== Merchant Claims Tests ======================

func TestListMerchantClaimsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	region := randomRegion()
	merchant.RegionID = region.ID

	claim := db.ListMerchantClaimsForMerchantRow{
		ID:             1,
		OrderID:        100,
		UserID:         200,
		ClaimType:      "missing-item",
		Description:    "缺少饮料",
		ClaimAmount:    500,
		Status:         "approved",
		OrderNo:        "20240101120000123456",
		OrderAmount:    3000,
		UserPhone:      pgtype.Text{String: "13800138000", Valid: true},
		UserName:       "张三",
		RecoveryStatus: "pending",
		CreatedAt:      time.Now(),
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantClaimsForMerchant(gomock.Any(), db.ListMerchantClaimsForMerchantParams{
						MerchantID: merchant.ID,
						Bucket:     pgtype.Text{},
						Limit:      10,
						Offset:     0,
					}).
					Times(1).
					Return([]db.ListMerchantClaimsForMerchantRow{claim}, nil)

				store.EXPECT().
					CountMerchantClaimsForMerchant(gomock.Any(), db.CountMerchantClaimsForMerchantParams{
						MerchantID: merchant.ID,
						Bucket:     pgtype.Text{},
					}).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)

				claims := response["claims"].([]interface{})
				require.Len(t, claims, 1)
				require.Equal(t, float64(1), response["total"])
				require.Equal(t, false, response["has_more"])
			},
		},
		{
			name:  "OKWithBucketFilter",
			query: "?page_id=1&page_size=10&bucket=pending_action",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantClaimsForMerchant(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.ListMerchantClaimsForMerchantParams) ([]db.ListMerchantClaimsForMerchantRow, error) {
						require.Equal(t, merchant.ID, arg.MerchantID)
						require.True(t, arg.Bucket.Valid)
						require.Equal(t, "pending_action", arg.Bucket.String)
						require.Equal(t, int32(10), arg.Limit)
						require.Equal(t, int32(0), arg.Offset)
						return []db.ListMerchantClaimsForMerchantRow{claim}, nil
					})

				store.EXPECT().
					CountMerchantClaimsForMerchant(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.CountMerchantClaimsForMerchantParams) (int64, error) {
						require.Equal(t, merchant.ID, arg.MerchantID)
						require.True(t, arg.Bucket.Valid)
						require.Equal(t, "pending_action", arg.Bucket.String)
						return int64(1), nil
					})
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)

				claims := response["claims"].([]interface{})
				require.Len(t, claims, 1)
			},
		},
		{
			name:  "NotMerchant",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:  "InvalidPageID",
			query: "?page_id=0&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "Unauthorized",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No auth header
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
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

			url := "/v1/merchant/claims" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Create Recovery Dispute Tests ======================

func TestCreateMerchantRecoveryDisputeAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	region := randomRegion()
	merchant.RegionID = region.ID

	claim := struct {
		ID          int64
		OrderID     int64
		ClaimType   string
		ClaimAmount int64
		MerchantID  int64
		RegionID    int64
		CreatedAt   time.Time
	}{
		ID:          1,
		OrderID:     100,
		ClaimType:   "missing-item",
		ClaimAmount: 500,
		MerchantID:  merchant.ID,
		RegionID:    region.ID,
		CreatedAt:   time.Now(),
	}

	recoveryDispute := randomRecoveryDispute(claim.ID, merchant.ID, region.ID, "merchant")
	autoApprovedRecoveryDispute := reviewedRecoveryDispute(recoveryDispute, "approved", "系统复核发现最新行为判责已不再指向当前申诉方，自动撤销原追偿安排。")
	decision := db.BehaviorDecision{
		ID:                 91,
		ClaimID:            pgtype.Int8{Int64: claim.ID, Valid: true},
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		DecisionStatus:     "decided",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	recoveryContext := db.GetClaimRecoveryContextByClaimIDRow{
		ClaimID:        claim.ID,
		OrderID:        claim.OrderID,
		MerchantID:     claim.MerchantID,
		RegionID:       claim.RegionID,
		ClaimCreatedAt: claim.CreatedAt,
	}

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"claim_id": claim.ID,
				"reason":   "订单包装完好，顾客收货时已当面核对",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetClaimRecoveryContextByClaimID(gomock.Any(), claim.ID).
					Times(2).
					Return(recoveryContext, nil)

				store.EXPECT().
					CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{
						ClaimID:       claim.ID,
						AppellantType: "merchant",
					}).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					CreateRecoveryDisputeWithRecoveryTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateRecoveryDisputeWithRecoveryTxResult{RecoveryDispute: recoveryDispute}, nil)

				store.EXPECT().
					ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).
					Times(1).
					Return([]db.BehaviorDecision{decision}, nil)

				store.EXPECT().
					ReviewRecoveryDisputeWithCompensationTx(gomock.Any(), db.ReviewRecoveryDisputeWithCompensationTxParams{
						ID:                 recoveryDispute.ID,
						Status:             "approved",
						DecisionID:         pgtype.Int8{Int64: decision.ID, Valid: true},
						ReviewerID:         pgtype.Int8{},
						ReviewNotes:        pgtype.Text{String: "系统复核发现最新行为判责已不再指向当前申诉方，自动撤销原追偿安排。", Valid: true},
						CompensationAmount: pgtype.Int8{},
					}).
					Times(1).
					Return(db.ReviewRecoveryDisputeWithCompensationTxResult{
						RecoveryDispute: autoApprovedRecoveryDispute,
						PostProcess: db.GetRecoveryDisputeForPostProcessRow{
							RecoveryDisputeID: recoveryDispute.ID,
							ClaimID:           recoveryDispute.ClaimID,
							AppellantType:     "merchant",
							AppellantID:       merchant.ID,
							ClaimantUserID:    300,
							ClaimType:         claim.ClaimType,
							ClaimAmount:       claim.ClaimAmount,
							OrderNo:           "20240101120000123456",
						},
					}, nil)

				taskDistributor.EXPECT().
					DistributeTaskProcessRecoveryDisputeResult(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var response recoveryDisputeResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, recoveryDispute.ID, response.ID)
				require.Equal(t, "approved", response.Status)
			},
		},
		{
			name: "RecoveryStateTransitionFailureReturns500",
			body: gin.H{
				"claim_id": claim.ID,
				"reason":   "订单包装完好，顾客收货时已当面核对",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetClaimRecoveryContextByClaimID(gomock.Any(), claim.ID).
					Times(1).
					Return(recoveryContext, nil)

				store.EXPECT().
					CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{
						ClaimID:       claim.ID,
						AppellantType: "merchant",
					}).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					CreateRecoveryDisputeWithRecoveryTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateRecoveryDisputeWithRecoveryTxResult{}, assertAnError("mark claim recovery disputed: update failed"))
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "AutoResolveQueueFailureFallsBackInline",
			body: gin.H{
				"claim_id": claim.ID,
				"reason":   "订单包装完好，顾客收货时已当面核对",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetClaimRecoveryContextByClaimID(gomock.Any(), claim.ID).
					Times(2).
					Return(recoveryContext, nil)

				store.EXPECT().
					CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{
						ClaimID:       claim.ID,
						AppellantType: "merchant",
					}).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					CreateRecoveryDisputeWithRecoveryTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateRecoveryDisputeWithRecoveryTxResult{RecoveryDispute: recoveryDispute}, nil)

				store.EXPECT().
					ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).
					Times(1).
					Return([]db.BehaviorDecision{decision}, nil)

				store.EXPECT().
					ReviewRecoveryDisputeWithCompensationTx(gomock.Any(), db.ReviewRecoveryDisputeWithCompensationTxParams{
						ID:                 recoveryDispute.ID,
						Status:             "approved",
						DecisionID:         pgtype.Int8{Int64: decision.ID, Valid: true},
						ReviewerID:         pgtype.Int8{},
						ReviewNotes:        pgtype.Text{String: "系统复核发现最新行为判责已不再指向当前申诉方，自动撤销原追偿安排。", Valid: true},
						CompensationAmount: pgtype.Int8{},
					}).
					Times(1).
					Return(db.ReviewRecoveryDisputeWithCompensationTxResult{
						RecoveryDispute: autoApprovedRecoveryDispute,
						PostProcess: db.GetRecoveryDisputeForPostProcessRow{
							RecoveryDisputeID: recoveryDispute.ID,
							ClaimID:           recoveryDispute.ClaimID,
							AppellantType:     "merchant",
							AppellantID:       merchant.ID,
							ClaimantUserID:    300,
							ClaimType:         claim.ClaimType,
							ClaimAmount:       claim.ClaimAmount,
							OrderNo:           "20240101120000123456",
						},
					}, nil)

				store.EXPECT().
					GetClaimRecoveryByClaimID(gomock.Any(), claim.ID).
					Times(1).
					Return(db.ClaimRecovery{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetActiveBehaviorBlocklist(gomock.Any(), db.GetActiveBehaviorBlocklistParams{EntityType: "user", EntityID: 300}).
					Times(1).
					Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{ConfigKey: "behavior_trace.reject_service_cooldown_days", ScopeType: "global", ScopeID: pgtype.Int8{Valid: false}}).
					Times(1).
					Return(db.PlatformConfig{}, db.ErrRecordNotFound)

				store.EXPECT().
					CreateBehaviorBlocklist(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.BehaviorBlocklist{ID: 1}, nil)

				store.EXPECT().
					GetUserClaimWarningStatus(gomock.Any(), int64(300)).
					Times(1).
					Return(db.UserClaimWarning{}, db.ErrRecordNotFound)

				store.EXPECT().
					CreateUserClaimWarning(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UserClaimWarning{ID: 1, UserID: 300}, nil)

				taskDistributor.EXPECT().
					DistributeTaskProcessRecoveryDisputeResult(gomock.Any(), gomock.Any()).
					Times(1).
					Return(assertAnError("recovery dispute queue unavailable"))

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetUserNotificationPreferences(gomock.Any(), int64(300)).
					Times(1).
					Return(db.UserNotificationPreference{}, db.ErrRecordNotFound)

				store.EXPECT().
					CreateNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Notification{ID: 1, UserID: 300, Type: "recovery_dispute", Title: "索赔申诉结果通知", Content: "test", CreatedAt: time.Now()}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var response recoveryDisputeResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, recoveryDispute.ID, response.ID)
				require.Equal(t, "approved", response.Status)
			},
		},
		{
			name: "ClaimNotFound",
			body: gin.H{
				"claim_id": 99999,
				"reason":   "这是一个充分的测试申诉理由",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetClaimRecoveryContextByClaimID(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.GetClaimRecoveryContextByClaimIDRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "AppealAlreadyExists",
			body: gin.H{
				"claim_id": claim.ID,
				"reason":   "这是一个充分的测试申诉理由",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetClaimRecoveryContextByClaimID(gomock.Any(), claim.ID).
					Times(1).
					Return(recoveryContext, nil)

				store.EXPECT().
					CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{
						ClaimID:       claim.ID,
						AppellantType: "merchant",
					}).
					Times(1).
					Return(true, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "ClaimNotBelongToMerchant",
			body: gin.H{
				"claim_id": claim.ID,
				"reason":   "这是一个充分的测试申诉理由",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// 返回属于其他商户的索赔
				otherContext := recoveryContext
				otherContext.MerchantID = merchant.ID + 1
				store.EXPECT().
					GetClaimRecoveryContextByClaimID(gomock.Any(), claim.ID).
					Times(1).
					Return(otherContext, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "ReasonTooShort",
			body: gin.H{
				"claim_id": claim.ID,
				"reason":   "短",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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
			taskDistributor := mockwk.NewMockTaskDistributor(ctrl)
			tc.buildStubs(store, taskDistributor)

			server := newTestServerWithTaskDistributor(t, store, taskDistributor)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/merchant/recovery-disputes"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== List Merchant Recovery Disputes Tests ======================

func TestListMerchantRecoveryDisputesAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	region := randomRegion()
	merchant.RegionID = region.ID

	recoveryDispute := db.ListMerchantRecoveryDisputesForMerchantRow{
		ID:               1,
		ClaimID:          100,
		AppellantType:    "merchant",
		AppellantID:      merchant.ID,
		Reason:           "测试申诉理由",
		Status:           "submitted",
		RegionID:         region.ID,
		ClaimType:        "missing-item",
		ClaimAmount:      500,
		ClaimDescription: "缺少饮料",
		OrderNo:          "20240101120000123456",
		CreatedAt:        time.Now(),
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantRecoveryDisputesForMerchant(gomock.Any(), db.ListMerchantRecoveryDisputesForMerchantParams{
						AppellantID: merchant.ID,
						Limit:       10,
						Offset:      0,
					}).
					Times(1).
					Return([]db.ListMerchantRecoveryDisputesForMerchantRow{recoveryDispute}, nil)

				store.EXPECT().
					CountMerchantRecoveryDisputesForMerchant(gomock.Any(), db.CountMerchantRecoveryDisputesForMerchantParams{
						AppellantID: merchant.ID,
						Status:      pgtype.Text{},
					}).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)

				disputes := response["disputes"].([]interface{})
				require.Len(t, disputes, 1)
				require.Equal(t, false, response["has_more"])
			},
		},
		{
			name:  "OKWithStatusFilter",
			query: "?page_id=1&page_size=10&status=submitted",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListMerchantRecoveryDisputesForMerchant(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.ListMerchantRecoveryDisputesForMerchantParams) ([]db.ListMerchantRecoveryDisputesForMerchantRow, error) {
						require.Equal(t, merchant.ID, arg.AppellantID)
						require.True(t, arg.Status.Valid)
						require.Equal(t, "submitted", arg.Status.String)
						require.Equal(t, int32(10), arg.Limit)
						require.Equal(t, int32(0), arg.Offset)
						return []db.ListMerchantRecoveryDisputesForMerchantRow{recoveryDispute}, nil
					})

				store.EXPECT().
					CountMerchantRecoveryDisputesForMerchant(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.CountMerchantRecoveryDisputesForMerchantParams) (int64, error) {
						require.Equal(t, merchant.ID, arg.AppellantID)
						require.True(t, arg.Status.Valid)
						require.Equal(t, "submitted", arg.Status.String)
						return int64(1), nil
					})
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)

				disputes := response["disputes"].([]interface{})
				require.Len(t, disputes, 1)
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

			url := "/v1/merchant/recovery-disputes" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Rider Recovery Dispute Tests ======================

func TestCreateRiderRecoveryDisputeAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomRecoveryDisputeRider(user.ID, region.ID)

	claim := struct {
		ID          int64
		OrderID     int64
		ClaimType   string
		ClaimAmount int64
		MerchantID  int64
		RegionID    int64
		RiderID     pgtype.Int8
		CreatedAt   time.Time
	}{
		ID:          1,
		OrderID:     100,
		ClaimType:   "delay",
		ClaimAmount: 300,
		MerchantID:  200,
		RegionID:    region.ID,
		RiderID:     pgtype.Int8{Int64: rider.ID, Valid: true},
		CreatedAt:   time.Now(),
	}

	recoveryDispute := randomRecoveryDispute(claim.ID, rider.ID, region.ID, "rider")
	autoRejectedRecoveryDispute := reviewedRecoveryDispute(recoveryDispute, "rejected", "系统复核确认最新行为判责仍指向当前申诉方，维持原判。")
	decision := db.BehaviorDecision{
		ID:                 92,
		ClaimID:            pgtype.Int8{Int64: claim.ID, Valid: true},
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		DecisionStatus:     "decided",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	recoveryContext := db.GetClaimRecoveryContextByClaimIDRow{
		ClaimID:        claim.ID,
		OrderID:        claim.OrderID,
		MerchantID:     claim.MerchantID,
		RegionID:       claim.RegionID,
		RiderID:        claim.RiderID,
		ClaimCreatedAt: claim.CreatedAt,
	}

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"claim_id": claim.ID,
				"reason":   "因恶劣天气导致配送延迟，非骑手原因",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetClaimRecoveryContextByClaimID(gomock.Any(), claim.ID).
					Times(2).
					Return(recoveryContext, nil)

				store.EXPECT().
					CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{
						ClaimID:       claim.ID,
						AppellantType: "rider",
					}).
					Times(1).
					Return(false, nil)

				store.EXPECT().
					CreateRecoveryDisputeWithRecoveryTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateRecoveryDisputeWithRecoveryTxResult{RecoveryDispute: recoveryDispute}, nil)

				store.EXPECT().
					ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).
					Times(1).
					Return([]db.BehaviorDecision{decision}, nil)

				store.EXPECT().
					ReviewRecoveryDisputeWithCompensationTx(gomock.Any(), db.ReviewRecoveryDisputeWithCompensationTxParams{
						ID:                 recoveryDispute.ID,
						Status:             "rejected",
						DecisionID:         pgtype.Int8{Int64: decision.ID, Valid: true},
						ReviewerID:         pgtype.Int8{},
						ReviewNotes:        pgtype.Text{String: "系统复核确认最新行为判责仍指向当前申诉方，维持原判。", Valid: true},
						CompensationAmount: pgtype.Int8{},
					}).
					Times(1).
					Return(db.ReviewRecoveryDisputeWithCompensationTxResult{
						RecoveryDispute: autoRejectedRecoveryDispute,
						PostProcess: db.GetRecoveryDisputeForPostProcessRow{
							RecoveryDisputeID: recoveryDispute.ID,
							ClaimID:           recoveryDispute.ClaimID,
							AppellantType:     "rider",
							AppellantID:       rider.ID,
							ClaimantUserID:    300,
							ClaimType:         claim.ClaimType,
							ClaimAmount:       claim.ClaimAmount,
							OrderNo:           "20240101120000123456",
						},
					}, nil)

				taskDistributor.EXPECT().
					DistributeTaskProcessRecoveryDisputeResult(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var response recoveryDisputeResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "rejected", response.Status)
			},
		},
		{
			name: "NotRider",
			body: gin.H{
				"claim_id": claim.ID,
				"reason":   "因恶劣天气导致配送延迟，非骑手原因",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "ClaimNotRelatedToRider",
			body: gin.H{
				"claim_id": claim.ID,
				"reason":   "因恶劣天气导致配送延迟，非骑手原因",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				// 返回属于其他骑手的索赔
				otherContext := recoveryContext
				otherContext.RiderID = pgtype.Int8{Int64: rider.ID + 1, Valid: true}
				store.EXPECT().
					GetClaimRecoveryContextByClaimID(gomock.Any(), claim.ID).
					Times(1).
					Return(otherContext, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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
			taskDistributor := mockwk.NewMockTaskDistributor(ctrl)
			tc.buildStubs(store, taskDistributor)

			server := newTestServerWithTaskDistributor(t, store, taskDistributor)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/rider/recovery-disputes"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestCreateMerchantRecoveryDisputeAPI_AutoResolveFailureWithoutTaskDistributorDoesNotPanic(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	region := randomRegion()
	merchant.RegionID = region.ID
	claim := struct {
		ID          int64
		OrderID     int64
		ClaimType   string
		ClaimAmount int64
		MerchantID  int64
		RegionID    int64
		CreatedAt   time.Time
	}{
		ID:          util.RandomInt(1, 1000),
		MerchantID:  merchant.ID,
		RegionID:    region.ID,
		OrderID:     util.RandomInt(1, 1000),
		ClaimType:   "missing-item",
		ClaimAmount: 500,
		CreatedAt:   time.Now(),
	}
	recoveryDispute := randomRecoveryDispute(claim.ID, merchant.ID, region.ID, "merchant")
	autoApprovedRecoveryDispute := reviewedRecoveryDispute(recoveryDispute, "approved", "系统复核发现最新行为判责已不再指向当前申诉方，自动撤销原追偿安排。")
	decision := db.BehaviorDecision{
		ID:                 91,
		ClaimID:            pgtype.Int8{Int64: claim.ID, Valid: true},
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		DecisionStatus:     "decided",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	recoveryContext := db.GetClaimRecoveryContextByClaimIDRow{
		ClaimID:        claim.ID,
		OrderID:        claim.OrderID,
		MerchantID:     claim.MerchantID,
		RegionID:       claim.RegionID,
		ClaimCreatedAt: claim.CreatedAt,
	}

	store.EXPECT().
		GetClaimRecoveryContextByClaimID(gomock.Any(), claim.ID).
		Times(2).
		Return(recoveryContext, nil)

	store.EXPECT().
		CheckRecoveryDisputeExists(gomock.Any(), db.CheckRecoveryDisputeExistsParams{
			ClaimID:       claim.ID,
			AppellantType: "merchant",
		}).
		Times(1).
		Return(false, nil)

	store.EXPECT().
		CreateRecoveryDisputeWithRecoveryTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateRecoveryDisputeWithRecoveryTxResult{RecoveryDispute: recoveryDispute}, nil)

	store.EXPECT().
		ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).
		Times(1).
		Return([]db.BehaviorDecision{decision}, nil)

	store.EXPECT().
		ReviewRecoveryDisputeWithCompensationTx(gomock.Any(), db.ReviewRecoveryDisputeWithCompensationTxParams{
			ID:                 recoveryDispute.ID,
			Status:             "approved",
			DecisionID:         pgtype.Int8{Int64: decision.ID, Valid: true},
			ReviewerID:         pgtype.Int8{},
			ReviewNotes:        pgtype.Text{String: "系统复核发现最新行为判责已不再指向当前申诉方，自动撤销原追偿安排。", Valid: true},
			CompensationAmount: pgtype.Int8{},
		}).
		Times(1).
		Return(db.ReviewRecoveryDisputeWithCompensationTxResult{
			RecoveryDispute: autoApprovedRecoveryDispute,
			PostProcess: db.GetRecoveryDisputeForPostProcessRow{
				RecoveryDisputeID: recoveryDispute.ID,
				ClaimID:           recoveryDispute.ClaimID,
				AppellantType:     "merchant",
				AppellantID:       merchant.ID,
				ClaimantUserID:    300,
				ClaimType:         claim.ClaimType,
				ClaimAmount:       claim.ClaimAmount,
				OrderNo:           "20240101120000123456",
			},
		}, nil)

	store.EXPECT().
		GetClaimRecoveryByClaimID(gomock.Any(), claim.ID).
		Times(1).
		Return(db.ClaimRecovery{}, db.ErrRecordNotFound)

	store.EXPECT().
		GetActiveBehaviorBlocklist(gomock.Any(), db.GetActiveBehaviorBlocklistParams{EntityType: "user", EntityID: 300}).
		Times(1).
		Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)

	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{ConfigKey: "behavior_trace.reject_service_cooldown_days", ScopeType: "global", ScopeID: pgtype.Int8{Valid: false}}).
		Times(1).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)

	store.EXPECT().
		CreateBehaviorBlocklist(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.BehaviorBlocklist{ID: 1}, nil)

	store.EXPECT().
		GetUserClaimWarningStatus(gomock.Any(), int64(300)).
		Times(1).
		Return(db.UserClaimWarning{}, db.ErrRecordNotFound)

	store.EXPECT().
		CreateUserClaimWarning(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.UserClaimWarning{ID: 1, UserID: 300}, nil)

	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(db.Merchant{}, db.ErrRecordNotFound)

	store.EXPECT().
		GetUserNotificationPreferences(gomock.Any(), int64(300)).
		Times(1).
		Return(db.UserNotificationPreference{}, db.ErrRecordNotFound)

	store.EXPECT().
		CreateNotification(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Notification{ID: 1, UserID: 300, Type: "recovery_dispute", Title: "索赔申诉结果通知", Content: "test", CreatedAt: time.Now()}, nil)

	server := newTestServer(t, store)
	server.SetTaskDistributorForTest(nil)
	defer server.SetTaskDistributorForTest(worker.NewNoopTaskDistributor())

	recorder := httptest.NewRecorder()
	body, err := json.Marshal(gin.H{
		"claim_id": claim.ID,
		"reason":   "订单包装完好，顾客收货时已当面核对",
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/recovery-disputes", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	var response recoveryDisputeResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, recoveryDispute.ID, response.ID)
	require.Equal(t, "approved", response.Status)
}

// ====================== Operator Write Routes Removed Tests ======================

func TestOperatorRecoveryDisputeWriteRoutesRemoved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	testCases := []struct {
		name string
		path string
	}{
		{name: "ReviewRouteRemoved", path: "/v1/operator/recovery-disputes/1/review"},
		{name: "WaiveRouteRemoved", path: "/v1/operator/claims/1/recovery/waive"},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(http.MethodPost, tc.path, bytes.NewReader([]byte("{}")))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			server.router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

func TestDeriveAutomaticRecoveryDisputeResolution_UsesClaimScopedDecision(t *testing.T) {
	recoveryDispute := db.RecoveryDispute{
		ID:            1,
		ClaimID:       100,
		AppellantType: "merchant",
	}

	resolution := deriveAutomaticRecoveryDisputeResolution(recoveryDispute, []db.BehaviorDecision{
		{
			ID:                 201,
			ClaimID:            pgtype.Int8{Int64: 999, Valid: true},
			ResponsibleParty:   "rider",
			CompensationSource: "platform",
		},
		{
			ID:                 202,
			ClaimID:            pgtype.Int8{Int64: 100, Valid: true},
			ResponsibleParty:   "merchant",
			CompensationSource: "merchant",
		},
	})

	require.Equal(t, "rejected", resolution.status)
	require.Equal(t, int64(202), resolution.decisionID.Int64)
	require.True(t, resolution.decisionID.Valid)
	require.Equal(t, "系统复核确认最新行为判责仍指向当前申诉方，维持原判。", resolution.reviewNotes)
}

func TestDeriveAutomaticRecoveryDisputeResolution_ApprovesInactiveDecision(t *testing.T) {
	recoveryDispute := db.RecoveryDispute{
		ID:            2,
		ClaimID:       101,
		AppellantType: "rider",
	}

	resolution := deriveAutomaticRecoveryDisputeResolution(recoveryDispute, []db.BehaviorDecision{
		{
			ID:               301,
			ClaimID:          pgtype.Int8{Int64: 101, Valid: true},
			ResponsibleParty: "rider",
			EffectiveStatus:  "overturned",
		},
	})

	require.Equal(t, "approved", resolution.status)
	require.Equal(t, int64(301), resolution.decisionID.Int64)
	require.Equal(t, "系统复核发现相关行为判责已失效，自动撤销原追偿安排。", resolution.reviewNotes)
}

func assertAnError(message string) error {
	return fmt.Errorf("%s", message)
}

// ====================== List Operator Recovery Disputes Tests ======================

func TestListOperatorRecoveryDisputesAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	operator := randomRecoveryDisputeOperator(user.ID, region.ID)

	recoveryDispute := db.ListOperatorRecoveryDisputesRow{
		ID:               1,
		ClaimID:          100,
		AppellantType:    "merchant",
		AppellantID:      200,
		Reason:           "测试申诉理由",
		Status:           "submitted",
		RegionID:         region.ID,
		ClaimType:        "missing-item",
		ClaimAmount:      500,
		ClaimDescription: "缺少饮料",
		OrderNo:          "20240101120000123456",
		MerchantID:       200,
		MerchantName:     "测试商户",
		AppellantName:    "测试商户",
		CreatedAt:        time.Now(),
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, region.ID)

				store.EXPECT().
					ListOperatorRecoveryDisputes(gomock.Any(), db.ListOperatorRecoveryDisputesParams{
						RegionID: region.ID,
						Column2:  "",
						Limit:    10,
						Offset:   0,
					}).
					Times(1).
					Return([]db.ListOperatorRecoveryDisputesRow{recoveryDispute}, nil)

				store.EXPECT().
					CountOperatorRecoveryDisputes(gomock.Any(), db.CountOperatorRecoveryDisputesParams{
						RegionID: region.ID,
						Column2:  "",
					}).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)

				disputes := response["disputes"].([]interface{})
				require.Len(t, disputes, 1)
			},
		},
		{
			name:  "AllManagedRegions",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				secondRegion := randomRegion()
				secondRecoveryDispute := recoveryDispute
				secondRecoveryDispute.ID = 2
				secondRecoveryDispute.RegionID = secondRegion.ID
				secondRecoveryDispute.CreatedAt = recoveryDispute.CreatedAt.Add(-time.Minute)

				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, region.ID, secondRegion.ID)

				store.EXPECT().
					ListOperatorRecoveryDisputes(gomock.Any(), db.ListOperatorRecoveryDisputesParams{
						RegionID: region.ID,
						Column2:  "",
						Limit:    10,
						Offset:   0,
					}).
					Times(1).
					Return([]db.ListOperatorRecoveryDisputesRow{recoveryDispute}, nil)

				store.EXPECT().
					CountOperatorRecoveryDisputes(gomock.Any(), db.CountOperatorRecoveryDisputesParams{
						RegionID: region.ID,
						Column2:  "",
					}).
					Times(1).
					Return(int64(1), nil)

				store.EXPECT().
					ListOperatorRecoveryDisputes(gomock.Any(), db.ListOperatorRecoveryDisputesParams{
						RegionID: secondRegion.ID,
						Column2:  "",
						Limit:    10,
						Offset:   0,
					}).
					Times(1).
					Return([]db.ListOperatorRecoveryDisputesRow{secondRecoveryDispute}, nil)

				store.EXPECT().
					CountOperatorRecoveryDisputes(gomock.Any(), db.CountOperatorRecoveryDisputesParams{
						RegionID: secondRegion.ID,
						Column2:  "",
					}).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response operatorRecoveryDisputesListResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.Disputes, 2)
				require.Equal(t, int64(2), response.Total)
				require.Equal(t, recoveryDispute.ID, response.Disputes[0].ID)
				require.Equal(t, int64(2), response.Disputes[1].ID)
			},
		},
		{
			name:  "FilterByStatus",
			query: "?page_id=1&page_size=10&status=submitted",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
				expectOperatorManagedRegions(store, operator, region.ID)

				store.EXPECT().
					ListOperatorRecoveryDisputes(gomock.Any(), db.ListOperatorRecoveryDisputesParams{
						RegionID: region.ID,
						Column2:  "submitted",
						Limit:    10,
						Offset:   0,
					}).
					Times(1).
					Return([]db.ListOperatorRecoveryDisputesRow{recoveryDispute}, nil)

				store.EXPECT().
					CountOperatorRecoveryDisputes(gomock.Any(), db.CountOperatorRecoveryDisputesParams{
						RegionID: region.ID,
						Column2:  "submitted",
					}).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidStatus",
			query: "?page_id=1&page_size=10&status=invalid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := "/v1/operator/recovery-disputes" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestListOperatorRecoveryDisputesSummaryAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomRecoveryDisputeOperator(user.ID, randomRegion().ID)
	secondRegion := randomRegion()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	expectOperatorManagedRegions(store, operator, operator.RegionID, secondRegion.ID)

	countMatrix := []struct {
		regionID int64
		status   string
		count    int64
	}{
		{regionID: operator.RegionID, status: "", count: 2},
		{regionID: secondRegion.ID, status: "", count: 3},
		{regionID: operator.RegionID, status: "submitted", count: 1},
		{regionID: secondRegion.ID, status: "submitted", count: 2},
		{regionID: operator.RegionID, status: "approved", count: 1},
		{regionID: secondRegion.ID, status: "approved", count: 0},
		{regionID: operator.RegionID, status: "rejected", count: 0},
		{regionID: secondRegion.ID, status: "rejected", count: 1},
	}

	for _, item := range countMatrix {
		store.EXPECT().
			CountOperatorRecoveryDisputes(gomock.Any(), db.CountOperatorRecoveryDisputesParams{
				RegionID: item.regionID,
				Column2:  item.status,
			}).
			Times(1).
			Return(item.count, nil)
	}

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/recovery-disputes/summary", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response recoveryDisputeSummaryResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, int64(5), response.Total)
	require.Equal(t, int64(3), response.Submitted)
	require.Equal(t, int64(1), response.Approved)
	require.Equal(t, int64(1), response.Rejected)
}

// ====================== Get Merchant Claim Detail Tests ======================

func TestGetMerchantClaimDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	region := randomRegion()
	merchant.RegionID = region.ID

	claim := db.GetMerchantClaimDetailForMerchantRow{
		ID:          1,
		OrderID:     100,
		UserID:      200,
		ClaimType:   "missing-item",
		Description: "缺少饮料",
		ClaimAmount: 500,
		Status:      "approved",
		OrderNo:     "20240101120000123456",
		OrderAmount: 3000,
		UserPhone:   pgtype.Text{String: "13800138000", Valid: true},
		UserName:    "张三",
		CreatedAt:   time.Now(),
	}

	testCases := []struct {
		name          string
		claimID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			claimID: claim.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMerchantClaimDetailForMerchant(gomock.Any(), db.GetMerchantClaimDetailForMerchantParams{
						ID:         claim.ID,
						MerchantID: merchant.ID,
					}).
					Times(1).
					Return(claim, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "ClaimNotFound",
			claimID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMerchantClaimDetailForMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantClaimDetailForMerchantRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "NotMerchant",
			claimID: claim.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/merchant/claims/%d", tc.claimID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Get Merchant Recovery Dispute Detail Tests ======================

func TestGetMerchantRecoveryDisputeDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	region := randomRegion()
	merchant.RegionID = region.ID

	recoveryDispute := db.GetMerchantRecoveryDisputeDetailRow{
		ID:               1,
		ClaimID:          100,
		AppellantType:    "merchant",
		AppellantID:      merchant.ID,
		Reason:           "订单包装完好",
		Status:           "submitted",
		RegionID:         region.ID,
		ClaimType:        "missing-item",
		ClaimAmount:      500,
		ClaimDescription: "缺少饮料",
		OrderNo:          "20240101120000123456",
		OrderAmount:      3000,
		UserPhone:        pgtype.Text{String: "13800138000", Valid: true},
		CreatedAt:        time.Now(),
	}

	testCases := []struct {
		name              string
		recoveryDisputeID int64
		setupAuth         func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs        func(store *mockdb.MockStore)
		checkResponse     func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:              "OK",
			recoveryDisputeID: recoveryDispute.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMerchantRecoveryDisputeDetail(gomock.Any(), db.GetMerchantRecoveryDisputeDetailParams{
						ID:          recoveryDispute.ID,
						AppellantID: merchant.ID,
					}).
					Times(1).
					Return(recoveryDispute, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:              "RecoveryDisputeNotFound",
			recoveryDisputeID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMerchantRecoveryDisputeDetail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetMerchantRecoveryDisputeDetailRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/merchant/recovery-disputes/%d", tc.recoveryDisputeID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== List Rider Claims Tests ======================

func TestListRiderClaimsAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomRecoveryDisputeRider(user.ID, region.ID)

	claim := db.ListRiderClaimsForRiderRow{
		ID:             1,
		OrderID:        100,
		UserID:         200,
		ClaimType:      "delay",
		Description:    "配送延迟",
		ClaimAmount:    300,
		Status:         "approved",
		RecoveryStatus: "pending",
		OrderNo:        "20240101120000123456",
		OrderAmount:    3000,
		UserPhone:      pgtype.Text{String: "13800138000", Valid: true},
		UserName:       "张三",
		CreatedAt:      time.Now(),
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderClaimsForRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListRiderClaimsForRiderRow{claim}, nil)

				store.EXPECT().
					CountRiderClaimsForRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response merchantClaimsListResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.Claims, 1)
				require.NotNil(t, response.Claims[0].RecoveryStatus)
				require.Equal(t, claim.RecoveryStatus, *response.Claims[0].RecoveryStatus)
			},
		},
		{
			name:  "WithBucket",
			query: "?page_id=2&page_size=5&bucket=pending_action",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderClaimsForRider(gomock.Any(), gomock.Eq(db.ListRiderClaimsForRiderParams{
						RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
						Limit:   5,
						Offset:  5,
						Bucket:  pgtype.Text{String: "pending_action", Valid: true},
					})).
					Times(1).
					Return([]db.ListRiderClaimsForRiderRow{claim}, nil)

				store.EXPECT().
					CountRiderClaimsForRider(gomock.Any(), gomock.Eq(db.CountRiderClaimsForRiderParams{
						RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
						Bucket:  pgtype.Text{String: "pending_action", Valid: true},
					})).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "NotRider",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := "/v1/rider/claims" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== List Rider Recovery Disputes Tests ======================

func TestListRiderRecoveryDisputesAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomRecoveryDisputeRider(user.ID, region.ID)

	recoveryDispute := db.ListRiderRecoveryDisputesRow{
		ID:               1,
		ClaimID:          100,
		AppellantType:    "rider",
		AppellantID:      rider.ID,
		Reason:           "恶劣天气导致延迟",
		Status:           "submitted",
		RegionID:         region.ID,
		ClaimType:        "delay",
		ClaimAmount:      300,
		ClaimDescription: "配送延迟",
		OrderNo:          "20240101120000123456",
		CreatedAt:        time.Now(),
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderRecoveryDisputes(gomock.Any(), db.ListRiderRecoveryDisputesParams{
						AppellantID: rider.ID,
						Status:      pgtype.Text{},
						Limit:       10,
						Offset:      0,
					}).
					Times(1).
					Return([]db.ListRiderRecoveryDisputesRow{recoveryDispute}, nil)

				store.EXPECT().
					CountRiderRecoveryDisputes(gomock.Any(), db.CountRiderRecoveryDisputesParams{
						AppellantID: rider.ID,
						Status:      pgtype.Text{},
					}).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response recoveryDisputesListResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.Disputes, 1)
				require.False(t, response.HasMore)
			},
		},
		{
			name:  "OKWithStatusFilterAndHasMore",
			query: "?page_id=2&page_size=1&status=submitted",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					ListRiderRecoveryDisputes(gomock.Any(), db.ListRiderRecoveryDisputesParams{
						AppellantID: rider.ID,
						Status:      pgtype.Text{String: "submitted", Valid: true},
						Limit:       1,
						Offset:      1,
					}).
					Times(1).
					Return([]db.ListRiderRecoveryDisputesRow{recoveryDispute}, nil)

				store.EXPECT().
					CountRiderRecoveryDisputes(gomock.Any(), db.CountRiderRecoveryDisputesParams{
						AppellantID: rider.ID,
						Status:      pgtype.Text{String: "submitted", Valid: true},
					}).
					Times(1).
					Return(int64(3), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response recoveryDisputesListResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.Disputes, 1)
				require.True(t, response.HasMore)
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

			url := "/v1/rider/recovery-disputes" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Get Operator Recovery Dispute Detail Tests ======================

func TestGetOperatorRecoveryDisputeDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	operator := randomRecoveryDisputeOperator(user.ID, region.ID)

	recoveryDispute := db.GetOperatorRecoveryDisputeDetailRow{
		ID:               1,
		ClaimID:          100,
		AppellantType:    "merchant",
		AppellantID:      200,
		Reason:           "订单包装完好",
		Status:           "submitted",
		RegionID:         region.ID,
		ClaimType:        "missing-item",
		ClaimAmount:      500,
		ClaimDescription: "缺少饮料",
		OrderNo:          "20240101120000123456",
		OrderAmount:      3000,
		OrderStatus:      "completed",
		MerchantID:       200,
		MerchantName:     "测试商户",
		MerchantPhone:    "13900139000",
		UserPhone:        pgtype.Text{String: "13800138000", Valid: true},
		UserName:         "张三",
		CreatedAt:        time.Now(),
	}

	testCases := []struct {
		name              string
		recoveryDisputeID int64
		setupAuth         func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs        func(store *mockdb.MockStore)
		checkResponse     func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:              "OK",
			recoveryDisputeID: recoveryDispute.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
				store.EXPECT().
					GetRecoveryDispute(gomock.Any(), recoveryDispute.ID).
					Times(1).
					Return(db.RecoveryDispute{ID: recoveryDispute.ID, RegionID: operator.RegionID}, nil)
				expectOperatorManagesRegion(store, operator, operator.RegionID, true)
				store.EXPECT().
					GetOperatorRecoveryDisputeDetail(gomock.Any(), db.GetOperatorRecoveryDisputeDetailParams{
						ID:       recoveryDispute.ID,
						RegionID: operator.RegionID,
					}).
					Times(1).
					Return(recoveryDispute, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:              "RecoveryDisputeNotFound",
			recoveryDisputeID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectActiveOperatorAuth(store, user.ID, operator)
				store.EXPECT().
					GetRecoveryDispute(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.RecoveryDispute{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/operator/recovery-disputes/%d", tc.recoveryDisputeID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ====================== Get Rider Claim Detail Tests ======================

func TestGetRiderClaimDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomRecoveryDisputeRider(user.ID, region.ID)

	claim := db.GetRiderClaimDetailForRiderRow{
		ID:          1,
		OrderID:     100,
		UserID:      200,
		ClaimType:   "delay",
		Description: "配送延迟",
		ClaimAmount: 300,
		Status:      "approved",
		OrderNo:     "20240101120000123456",
		OrderAmount: 3000,
		UserPhone:   pgtype.Text{String: "13800138000", Valid: true},
		UserName:    "张三",
		CreatedAt:   time.Now(),
	}

	testCases := []struct {
		name          string
		claimID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			claimID: claim.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderClaimDetailForRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(claim, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, float64(claim.ID), response["id"])
				require.Equal(t, claim.ClaimType, response["claim_type"])
			},
		},
		{
			name:    "ClaimNotFound",
			claimID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderClaimDetailForRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetRiderClaimDetailForRiderRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "NotRider",
			claimID: claim.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "InvalidClaimID",
			claimID: -1, // negative ID
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderClaimDetailForRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetRiderClaimDetailForRiderRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/rider/claims/%d", tc.claimID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestGetRiderClaimDecisionAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomRecoveryDisputeRider(user.ID, region.ID)

	claim := db.GetRiderClaimDetailForRiderRow{
		ID:          1,
		OrderID:     100,
		UserID:      200,
		ClaimType:   "damage",
		Description: "餐损",
		ClaimAmount: 300,
		Status:      "approved",
		OrderNo:     "20240101120000123456",
		OrderAmount: 3000,
		UserPhone:   pgtype.Text{String: "13800138000", Valid: true},
		UserName:    "张三",
		CreatedAt:   time.Now(),
	}

	decision := db.BehaviorDecision{
		ID:                 11,
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		DecisionStatus:     "decided",
		ReasonCodes:        []string{"instant", "normal"},
		TraceSummary:       pgtype.Text{String: "骑手责任，平台已先赔", Valid: true},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	testCases := []struct {
		name          string
		claimID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			claimID: claim.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderClaimDetailForRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(claim, nil)

				store.EXPECT().
					ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).
					Times(1).
					Return([]db.BehaviorDecision{decision}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response merchantClaimDecisionResult
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.NotNil(t, response.Decision)
				require.Equal(t, decision.ID, response.Decision.DecisionID)
				require.Equal(t, decision.ResponsibleParty, response.Decision.ResponsibleParty)
			},
		},
		{
			name:    "ClaimNotFound",
			claimID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderClaimDetailForRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetRiderClaimDetailForRiderRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "NotRider",
			claimID: claim.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/rider/claims/%d/decision", tc.claimID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestGetRiderClaimDecisionAPI_ReadOnlyConsumerDoesNotCreateBehaviorAction(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomRecoveryDisputeRider(user.ID, region.ID)

	claim := db.GetRiderClaimDetailForRiderRow{
		ID:          1,
		OrderID:     100,
		UserID:      200,
		ClaimType:   "damage",
		Description: "餐损",
		ClaimAmount: 300,
		Status:      "approved",
		OrderNo:     "20240101120000123456",
		OrderAmount: 3000,
		UserPhone:   pgtype.Text{String: "13800138000", Valid: true},
		UserName:    "张三",
		CreatedAt:   time.Now(),
	}

	decision := db.BehaviorDecision{
		ID:                 11,
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		DecisionStatus:     "decided",
		ReasonCodes:        []string{"rider_recovery"},
		TraceSummary:       pgtype.Text{String: "骑手责任，平台已先赔", Valid: true},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetRiderByUserID(gomock.Any(), user.ID).Times(1).Return(rider, nil)
	store.EXPECT().GetRiderClaimDetailForRider(gomock.Any(), gomock.Any()).Times(1).Return(claim, nil)
	store.EXPECT().ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).Times(1).Return([]db.BehaviorDecision{decision}, nil)
	store.EXPECT().CreateBehaviorAction(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	url := fmt.Sprintf("/v1/rider/claims/%d/decision", claim.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response merchantClaimDecisionResult
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.NotNil(t, response.Decision)
	require.Equal(t, decision.ID, response.Decision.DecisionID)
}

func TestGetMerchantClaimDecisionAPI_ReadOnlyConsumerDoesNotCreateBehaviorAction(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	claim := db.GetMerchantClaimDetailForMerchantRow{
		ID:          1,
		OrderID:     100,
		UserID:      200,
		ClaimType:   "foreign-object",
		Description: "异物",
		ClaimAmount: 300,
		Status:      "approved",
		OrderNo:     "20240101120000123456",
		OrderAmount: 3000,
		UserPhone:   pgtype.Text{String: "13800138000", Valid: true},
		UserName:    "张三",
		CreatedAt:   time.Now(),
	}

	decision := db.BehaviorDecision{
		ID:                 21,
		ResponsibleParty:   "merchant",
		CompensationSource: "merchant",
		DecisionStatus:     "decided",
		ReasonCodes:        []string{"merchant_recovery"},
		TraceSummary:       pgtype.Text{String: "商户责任，平台已先赔", Valid: true},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantClaimDetailForMerchant(gomock.Any(), db.GetMerchantClaimDetailForMerchantParams{
		ID:         claim.ID,
		MerchantID: merchant.ID,
	}).Times(1).Return(claim, nil)
	store.EXPECT().ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: claim.OrderID, Valid: true}).Times(1).Return([]db.BehaviorDecision{decision}, nil)
	store.EXPECT().CreateBehaviorAction(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	url := fmt.Sprintf("/v1/merchant/claims/%d/decision", claim.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response merchantClaimDecisionResult
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.NotNil(t, response.Decision)
	require.Equal(t, decision.ID, response.Decision.DecisionID)
}

// ====================== Get Rider Recovery Dispute Detail Tests ======================

func TestGetRiderRecoveryDisputeDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	region := randomRegion()
	rider := randomRecoveryDisputeRider(user.ID, region.ID)

	recoveryDispute := db.GetRiderRecoveryDisputeDetailRow{
		ID:               1,
		ClaimID:          100,
		AppellantType:    "rider",
		AppellantID:      rider.ID,
		Reason:           "恶劣天气导致延迟",
		Status:           "submitted",
		RegionID:         region.ID,
		ClaimType:        "delay",
		ClaimAmount:      300,
		ClaimDescription: "配送延迟",
		OrderNo:          "20240101120000123456",
		OrderAmount:      3000,
		UserPhone:        pgtype.Text{String: "13800138000", Valid: true},
		CreatedAt:        time.Now(),
	}

	testCases := []struct {
		name              string
		recoveryDisputeID int64
		setupAuth         func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs        func(store *mockdb.MockStore)
		checkResponse     func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:              "OK",
			recoveryDisputeID: recoveryDispute.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderRecoveryDisputeDetail(gomock.Any(), db.GetRiderRecoveryDisputeDetailParams{
						ID:          recoveryDispute.ID,
						AppellantID: rider.ID,
					}).
					Times(1).
					Return(recoveryDispute, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response map[string]interface{}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, float64(recoveryDispute.ID), response["id"])
				require.Equal(t, "rider", response["appellant_type"])
			},
		},
		{
			name:              "RecoveryDisputeNotFound",
			recoveryDisputeID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderRecoveryDisputeDetail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetRiderRecoveryDisputeDetailRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:              "NotRider",
			recoveryDisputeID: recoveryDispute.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:              "InvalidRecoveryDisputeID",
			recoveryDisputeID: -1, // negative ID
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)

				store.EXPECT().
					GetRiderRecoveryDisputeDetail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetRiderRecoveryDisputeDetailRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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

			url := fmt.Sprintf("/v1/rider/recovery-disputes/%d", tc.recoveryDisputeID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestBuildRecoveryDisputeNotificationContent_ApprovedClaimantKeepsExistingPayout(t *testing.T) {
	title, content := buildRecoveryDisputeNotificationContent(&worker.ProcessRecoveryDisputeResultPayload{
		Status:    "approved",
		OrderNo:   "20240101120000123456",
		ClaimType: "damage",
	}, false)

	require.Equal(t, "索赔申诉结果通知", title)
	require.Contains(t, content, "已发放赔付不再向您追回")
	require.NotContains(t, content, "相关赔付已撤回")
}
