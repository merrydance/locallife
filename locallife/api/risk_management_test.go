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
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSubmitClaimAPI_ReturnsUserFacingLifecycleAndWritesAudit(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.TotalAmount = 5600

	approvedAmount := int64(1200)
	claim := db.Claim{
		ID:             801,
		OrderID:        order.ID,
		UserID:         user.ID,
		ClaimType:      "foreign-object",
		Description:    "餐品里发现异物，需要平台介入处理",
		ClaimAmount:    approvedAmount,
		ApprovedAmount: pgtype.Int8{Int64: approvedAmount, Valid: true},
		Status:         "auto-approved",
		ApprovalType:   pgtype.Text{String: "platform-pay", Valid: true},
		CreatedAt:      time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), user.ID).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 8,
		Claims90d:        3,
		WarningCount:     0,
		PlatformPayCount: 1,
	}, nil)
	store.EXPECT().IncrementUserPlatformPayCount(gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, user.ID, arg.UserID)
			require.Equal(t, "foreign-object", arg.ClaimType)
			require.Equal(t, approvedAmount, arg.ClaimAmount)
			require.Equal(t, "platform-pay", arg.ApprovalType)
			require.Equal(t, "platform", arg.CompensationSource)
			require.NotNil(t, arg.ApprovedAmount)
			require.Equal(t, approvedAmount, *arg.ApprovedAmount)
			require.False(t, arg.CreateRecovery)
			return db.CreateClaimWithBehaviorTxResult{Claim: claim}, nil
		},
	)
	store.EXPECT().ListBehaviorDecisionsByOrder(gomock.Any(), gomock.Any()).Return([]db.BehaviorDecision{{
		ID:      901,
		OrderID: pgtype.Int8{Int64: order.ID, Valid: true},
	}}, nil)
	store.EXPECT().ListBehaviorActionsByDecision(gomock.Any(), int64(901)).Return([]db.BehaviorAction{{
		ID:           902,
		DecisionID:   901,
		ActionType:   "payout",
		TargetEntity: "user",
		Status:       "pending",
	}}, nil)

	server := newTestServer(t, store)
	server.SetPaymentClientForTest(mockwechat.NewMockPaymentClientInterface(ctrl))
	auditWriter := &auditSpyWriter{}
	server.auditWriter = auditWriter

	body, err := json.Marshal(SubmitClaimRequest{
		OrderID:     order.ID,
		ClaimType:   "foreign-object",
		ClaimAmount: approvedAmount,
		ClaimReason: "餐品里发现异物，需要平台介入处理",
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/claims", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp SubmitClaimResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, claim.ID, resp.ClaimID)
	require.Equal(t, submitClaimStatusAccepted, resp.Status)
	require.Equal(t, submitClaimDecisionStatusAutoAdjudicated, resp.DecisionStatus)
	require.Equal(t, submitClaimPayoutStatusProcessing, resp.PayoutStatus)
	require.NotNil(t, resp.ApprovedAmount)
	require.Equal(t, approvedAmount, *resp.ApprovedAmount)
	require.Equal(t, "platform", resp.CompensationSource)
	require.Equal(t, "问题用户，平台垫付", resp.Reason)
	require.NotNil(t, resp.PayoutETA)
	require.Equal(t, "1-3个工作日", *resp.PayoutETA)
	require.NotNil(t, resp.Warning)
	require.Contains(t, *resp.Warning, "平台垫付")

	entries := auditWriter.Entries()
	require.Len(t, entries, 1)
	require.Equal(t, user.ID, entries[0].ActorUserID)
	require.Equal(t, "customer", entries[0].ActorRole)
	require.Equal(t, "user_claim_submitted", entries[0].Action)
	require.Equal(t, "claim", entries[0].TargetType)
	require.NotNil(t, entries[0].TargetID)
	require.Equal(t, claim.ID, *entries[0].TargetID)
	require.Equal(t, resp.Status, entries[0].Metadata["status"])
	require.Equal(t, resp.DecisionStatus, entries[0].Metadata["decision_status"])
	require.Equal(t, resp.PayoutStatus, entries[0].Metadata["payout_status"])
	require.Equal(t, order.ID, entries[0].Metadata["order_id"])
	require.Equal(t, "foreign-object", entries[0].Metadata["claim_type"])
	require.Equal(t, approvedAmount, entries[0].Metadata["requested_amount"])
	require.Equal(t, approvedAmount, entries[0].Metadata["approved_amount"])
	require.Equal(t, "platform", entries[0].Metadata["compensation_source"])
	require.Equal(t, "1-3个工作日", entries[0].Metadata["payout_eta"])
	require.Equal(t, *resp.Warning, entries[0].Metadata["warning"])
	require.Equal(t, true, entries[0].Metadata["auto_adjudicated"])
}

func TestSubmitClaimAPI_ApprovedPayoutRequiresPaymentClient(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.TotalAmount = 5600

	approvedAmount := int64(1200)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), user.ID).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 8,
		Claims90d:        3,
		WarningCount:     0,
		PlatformPayCount: 1,
	}, nil)
	store.EXPECT().IncrementUserPlatformPayCount(gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)

	server := newTestServer(t, store)

	body, err := json.Marshal(SubmitClaimRequest{
		OrderID:     order.ID,
		ClaimType:   "foreign-object",
		ClaimAmount: approvedAmount,
		ClaimReason: "餐品里发现异物，需要平台介入处理",
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/claims", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrClaimPayoutServiceUnavailable.Code, resp.Code)
	require.Equal(t, ErrClaimPayoutServiceUnavailable.Message, resp.Error)
}

func TestListUserClaimsAPI_ReturnsUserFacingLifecycle(t *testing.T) {
	user, _ := randomUser(t)
	now := time.Now()
	approvedClaim := db.Claim{
		ID:             901,
		OrderID:        3001,
		UserID:         user.ID,
		ClaimType:      "damage",
		Description:    "餐品破损，需要平台处理",
		ClaimAmount:    2600,
		ApprovedAmount: pgtype.Int8{Int64: 2600, Valid: true},
		Status:         "auto-approved",
		ApprovalType:   pgtype.Text{String: "instant", Valid: true},
		PaidAt:         pgtype.Timestamptz{Time: now, Valid: true},
		DecisionReason: pgtype.Text{String: "平台已核验并完成赔付", Valid: true},
		CreatedAt:      now.Add(-2 * time.Hour),
	}
	rejectedClaim := db.Claim{
		ID:              902,
		OrderID:         3002,
		UserID:          user.ID,
		ClaimType:       "timeout",
		Description:     "订单超时送达",
		ClaimAmount:     1200,
		Status:          "rejected",
		RejectionReason: pgtype.Text{String: "经核验，本次问题不在赔付范围内", Valid: true},
		ReviewedAt:      pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true},
		CreatedAt:       now.Add(-3 * time.Hour),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().ListUserClaims(gomock.Any(), db.ListUserClaimsParams{
		UserID: user.ID,
		Limit:  20,
		Offset: 0,
	}).Return([]db.Claim{approvedClaim, rejectedClaim}, nil)
	store.EXPECT().CountUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return(int64(2), nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/claims?page=1&page_size=20", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "\"user_id\"")
	require.NotContains(t, recorder.Body.String(), "\"approval_type\"")

	var resp userClaimsListResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Claims, 2)
	require.Equal(t, int64(2), resp.Total)
	require.Equal(t, 1, resp.Page)
	require.Equal(t, 20, resp.PageSize)

	require.Equal(t, submitClaimStatusAccepted, resp.Claims[0].Status)
	require.Equal(t, submitClaimDecisionStatusAutoAdjudicated, resp.Claims[0].DecisionStatus)
	require.Equal(t, submitClaimPayoutStatusPaid, resp.Claims[0].PayoutStatus)
	require.Equal(t, "平台已核验并完成赔付", resp.Claims[0].Reason)
	require.Nil(t, resp.Claims[0].PayoutETA)
	require.NotNil(t, resp.Claims[0].ProcessedAt)

	require.Equal(t, submitClaimStatusRejected, resp.Claims[1].Status)
	require.Equal(t, submitClaimDecisionStatusRejected, resp.Claims[1].DecisionStatus)
	require.Equal(t, "", resp.Claims[1].PayoutStatus)
	require.Equal(t, "经核验，本次问题不在赔付范围内", resp.Claims[1].Reason)
	require.Nil(t, resp.Claims[1].ApprovedAmount)
	require.NotNil(t, resp.Claims[1].ProcessedAt)
}

func TestGetClaimDetailAPI_ReturnsUserFacingLifecycle(t *testing.T) {
	user, _ := randomUser(t)
	now := time.Now()
	claim := db.Claim{
		ID:                 903,
		OrderID:            3003,
		UserID:             user.ID,
		ClaimType:          "foreign-object",
		Description:        "餐品中发现异物",
		ClaimAmount:        1800,
		ApprovedAmount:     pgtype.Int8{Int64: 1500, Valid: true},
		Status:             "auto-approved",
		ApprovalType:       pgtype.Text{String: "platform-pay", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "平台已完成自动裁定，赔付正在处理中", Valid: true},
		CreatedAt:          now.Add(-30 * time.Minute),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetClaim(gomock.Any(), claim.ID).Return(claim, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/claims/903", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "\"review_notes\"")

	var resp userClaimResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, claim.ID, resp.ID)
	require.Equal(t, submitClaimStatusAccepted, resp.Status)
	require.Equal(t, submitClaimDecisionStatusAutoAdjudicated, resp.DecisionStatus)
	require.Equal(t, submitClaimPayoutStatusProcessing, resp.PayoutStatus)
	require.NotNil(t, resp.PayoutETA)
	require.Equal(t, "1-3个工作日", *resp.PayoutETA)
	require.Equal(t, "平台已完成自动裁定，赔付正在处理中", resp.Reason)
	require.NotNil(t, resp.ApprovedAmount)
	require.Equal(t, int64(1500), *resp.ApprovedAmount)
	require.Nil(t, resp.ProcessedAt)
}
