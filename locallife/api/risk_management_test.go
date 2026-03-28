package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/rules"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	mockworker "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type stubRulesEngine struct {
	decision rules.Decision
	err      error
}

func (s stubRulesEngine) Evaluate(ctx context.Context, input rules.Context) (rules.Decision, error) {
	return s.decision, s.err
}

type restrictionActionDetailFixture struct {
	Action            string `json:"action"`
	ClaimID           int64  `json:"claim_id"`
	UserID            int64  `json:"user_id"`
	DecisionMode      string `json:"decision_mode"`
	RestrictionReason string `json:"restriction_reason"`
	Remark            string `json:"remark"`
}

type notifyActionDetailFixture struct {
	Action           string `json:"action"`
	ClaimID          int64  `json:"claim_id"`
	TargetEntity     string `json:"target_entity"`
	TargetID         int64  `json:"target_id"`
	RecipientUserID  int64  `json:"recipient_user_id"`
	NotificationType string `json:"notification_type"`
	Title            string `json:"title"`
	Content          string `json:"content"`
	RelatedType      string `json:"related_type"`
	RelatedID        int64  `json:"related_id"`
	Remark           string `json:"remark"`
}

func mustMarshalJSONDetail(t *testing.T, value interface{}) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	require.NoError(t, err)

	return data
}

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
		ApprovalType:   pgtype.Text{String: "auto", Valid: true},
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
			require.Equal(t, "auto", arg.ApprovalType)
			require.Equal(t, "platform", arg.CompensationSource)
			require.NotNil(t, arg.ApprovedAmount)
			require.Equal(t, approvedAmount, *arg.ApprovedAmount)
			require.False(t, arg.CreateRecovery)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: claim,
				PayoutAction: &db.BehaviorAction{
					ID:           902,
					DecisionID:   901,
					ActionType:   "payout",
					TargetEntity: "user",
					Status:       "pending",
				},
				BehaviorDecision: db.BehaviorDecision{
					ID:                   901,
					OrderID:              pgtype.Int8{Int64: order.ID, Valid: true},
					DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModePlatformFallback, Valid: true},
					ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainMerchant, Valid: true},
					CompensationSource:   "platform",
				},
			}, nil
		},
	)

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
	require.Equal(t, "用户风险较高，本次由平台兜底处理", resp.Reason)
	require.NotNil(t, resp.PayoutETA)
	require.Equal(t, "1-3个工作日", *resp.PayoutETA)
	require.NotNil(t, resp.Warning)
	require.Contains(t, *resp.Warning, "平台兜底")

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

func TestSubmitClaimAPI_PayoutDispatchUsesTxActionWithoutBehaviorReload(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.OrderType = "takeout"
	order.TotalAmount = 4200

	approvedAmount := int64(1200)
	claim := db.Claim{
		ID:             806,
		OrderID:        order.ID,
		UserID:         user.ID,
		ClaimType:      "foreign-object",
		Description:    "餐品里发现异物，需要平台介入处理",
		ClaimAmount:    approvedAmount,
		ApprovedAmount: pgtype.Int8{Int64: approvedAmount, Valid: true},
		Status:         "auto-approved",
		ApprovalType:   pgtype.Text{String: "auto", Valid: true},
		CreatedAt:      time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), user.ID).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 6,
		Claims90d:        2,
		WarningCount:     0,
		PlatformPayCount: 0,
	}, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).Return(db.CreateClaimWithBehaviorTxResult{
		Claim: claim,
		PayoutAction: &db.BehaviorAction{
			ID:           906,
			DecisionID:   905,
			ActionType:   "payout",
			TargetEntity: "user",
			Status:       "pending",
		},
		BehaviorDecision: db.BehaviorDecision{
			ID:                   905,
			OrderID:              pgtype.Int8{Int64: order.ID, Valid: true},
			DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModeMerchantRecovery, Valid: true},
			ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainMerchant, Valid: true},
			CompensationSource:   "merchant",
		},
	}, nil)
	store.EXPECT().ListBehaviorDecisionsByOrder(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().ListBehaviorActionsByDecision(gomock.Any(), gomock.Any()).Times(0)

	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
	taskDistributor.EXPECT().DistributeTaskClaimPayout(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, payload any, _ any) error {
			require.IsType(t, &worker.ClaimPayoutPayload{}, payload)
			require.Equal(t, int64(906), payload.(*worker.ClaimPayoutPayload).ActionID)
			return nil
		},
	)
	taskDistributor.EXPECT().DistributeTaskCheckMerchantForeignObject(gomock.Any(), order.MerchantID, gomock.Any(), gomock.Any()).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.SetPaymentClientForTest(mockwechat.NewMockPaymentClientInterface(ctrl))

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
	require.Equal(t, submitClaimPayoutStatusProcessing, resp.PayoutStatus)
	require.Equal(t, "merchant", resp.CompensationSource)
}

func TestSubmitClaimAPI_UsesPersistedPlatformFallbackOutcome(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.OrderType = "takeout"
	order.TotalAmount = 5600

	approvedAmount := int64(1800)
	claim := db.Claim{
		ID:                 811,
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "damage",
		Description:        "配送中餐品破损，需要平台介入处理",
		ClaimAmount:        approvedAmount,
		ApprovedAmount:     pgtype.Int8{Int64: approvedAmount, Valid: true},
		Status:             "auto-approved",
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "当前订单缺少取餐确认等关键责任事实，本次不向服务方追责，已由平台兜底处理", Valid: true},
		CreatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), user.ID).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 6,
		Claims90d:        1,
		WarningCount:     0,
		PlatformPayCount: 0,
	}, nil)
	store.EXPECT().IncrementUserPlatformPayCount(gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, user.ID, arg.UserID)
			require.Equal(t, "damage", arg.ClaimType)
			require.Equal(t, "instant", arg.ApprovalType)
			require.Equal(t, "rider", arg.CompensationSource)
			require.True(t, arg.CreateRecovery)
			require.Equal(t, "rider", arg.RecoveryTarget)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: claim,
				PayoutAction: &db.BehaviorAction{
					ID:           912,
					DecisionID:   911,
					ActionType:   "payout",
					TargetEntity: "user",
					Status:       "pending",
				},
				BehaviorDecision: db.BehaviorDecision{
					ID:                   911,
					OrderID:              pgtype.Int8{Int64: order.ID, Valid: true},
					DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModePlatformFallback, Valid: true},
					ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainUnknown, Valid: true},
					CompensationSource:   "platform",
					FallbackReason:       pgtype.Text{String: "missing_pickup_confirmation", Valid: true},
					TraceSummary:         pgtype.Text{String: "当前订单缺少取餐确认等关键责任事实，本次不向服务方追责，已由平台兜底处理", Valid: true},
				},
			}, nil
		},
	)

	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
	taskDistributor.EXPECT().DistributeTaskClaimPayout(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
	).DoAndReturn(func(_ any, payload any, _ any) error {
		require.IsType(t, &worker.ClaimPayoutPayload{}, payload)
		require.Equal(t, int64(912), payload.(*worker.ClaimPayoutPayload).ActionID)
		return nil
	})
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{
		OrderID: order.ID,
		RiderID: pgtype.Int8{Int64: 919, Valid: true},
	}, nil)
	taskDistributor.EXPECT().DistributeTaskCheckRiderDamage(
		gomock.Any(),
		int64(919),
		gomock.Any(),
		gomock.Any(),
	).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.SetPaymentClientForTest(mockwechat.NewMockPaymentClientInterface(ctrl))

	body, err := json.Marshal(SubmitClaimRequest{
		OrderID:     order.ID,
		ClaimType:   "damage",
		ClaimAmount: approvedAmount,
		ClaimReason: "配送中餐品破损，需要平台介入处理",
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
	require.Equal(t, "当前订单缺少取餐确认等关键责任事实，本次不向服务方追责，已由平台兜底处理", resp.Reason)
	require.NotNil(t, resp.Warning)
	require.Equal(t, resp.Reason, *resp.Warning)
	require.NotNil(t, resp.PayoutETA)
	require.Equal(t, "1-3个工作日", *resp.PayoutETA)
}

func TestSubmitClaimAPI_RuleOverridePlatformFallbackDoesNotCreateRecovery(t *testing.T) {
	user, _ := randomUser(t)
	riderUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.RegionID = 66
	rider := randomRider(riderUser.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.OrderType = "takeout"
	order.TotalAmount = 5600

	approvedAmount := int64(1800)
	claim := db.Claim{
		ID:                 818,
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "damage",
		Description:        "规则覆盖为平台兜底，不应再创建服务方追偿",
		ClaimAmount:        approvedAmount,
		ApprovedAmount:     pgtype.Int8{Int64: approvedAmount, Valid: true},
		Status:             "auto-approved",
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "规则覆盖为平台兜底，不应再创建服务方追偿", Valid: true},
		CreatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().GetPlatformConfig(gomock.Any(), gomock.Any()).Return(db.PlatformConfig{}, db.ErrRecordNotFound).Times(2)
	store.EXPECT().GetAbnormalStatsSummary(gomock.Any(), gomock.Any()).Return(db.GetAbnormalStatsSummaryRow{}, nil).Times(6)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{
		OrderID: order.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	}, nil).Times(2)
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), user.ID).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 6,
		Claims90d:        1,
		WarningCount:     0,
		PlatformPayCount: 0,
	}, nil)
	store.EXPECT().IncrementUserPlatformPayCount(gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, user.ID, arg.UserID)
			require.Equal(t, "damage", arg.ClaimType)
			require.Equal(t, "auto", arg.ApprovalType)
			require.Equal(t, "platform", arg.CompensationSource)
			require.False(t, arg.CreateRecovery)
			require.Equal(t, "", arg.RecoveryTarget)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: claim,
				PayoutAction: &db.BehaviorAction{
					ID:           918,
					DecisionID:   917,
					ActionType:   "payout",
					TargetEntity: "user",
					Status:       "pending",
				},
				BehaviorDecision: db.BehaviorDecision{
					ID:                   917,
					OrderID:              pgtype.Int8{Int64: order.ID, Valid: true},
					DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModePlatformFallback, Valid: true},
					ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainUnknown, Valid: true},
					CompensationSource:   "platform",
					FallbackReason:       pgtype.Text{String: "rule_platform_fallback", Valid: true},
					TraceSummary:         pgtype.Text{String: "规则覆盖为平台兜底，不应再创建服务方追偿", Valid: true},
				},
			}, nil
		},
	)

	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
	taskDistributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any()).Times(0)
	taskDistributor.EXPECT().DistributeTaskClaimPayout(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, payload *worker.ClaimPayoutPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(918), payload.ActionID)
			return nil
		},
	)
	taskDistributor.EXPECT().DistributeTaskCheckRiderDamage(gomock.Any(), rider.ID, gomock.Any(), gomock.Any()).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.config.RulesEngineEnabled = true
	server.rulesEngine = stubRulesEngine{decision: rules.Decision{
		Allow:  true,
		Action: "platform-fallback",
		Reason: "规则覆盖为平台兜底，不应再创建服务方追偿",
	}}
	server.SetPaymentClientForTest(mockwechat.NewMockPaymentClientInterface(ctrl))

	body, err := json.Marshal(SubmitClaimRequest{
		OrderID:     order.ID,
		ClaimType:   "damage",
		ClaimAmount: approvedAmount,
		ClaimReason: "规则覆盖为平台兜底，不应再创建服务方追偿",
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
	require.Equal(t, submitClaimPayoutStatusProcessing, resp.PayoutStatus)
	require.Equal(t, "platform", resp.CompensationSource)
	require.NotNil(t, resp.Warning)
	require.Equal(t, "规则覆盖为平台兜底，不应再创建服务方追偿", *resp.Warning)
}

func TestSubmitClaimAPI_UsesPersistedUserRestrictedOutcome(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.OrderType = "dinein"
	order.TotalAmount = 5600

	approvedAmount := int64(1500)
	claim := db.Claim{
		ID:                 821,
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "damage",
		Description:        "用户索赔行为异常但本次仍赔付",
		ClaimAmount:        approvedAmount,
		ApprovedAmount:     pgtype.Int8{Int64: approvedAmount, Valid: true},
		Status:             "auto-approved",
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "您的账号因索赔行为异常已被限制服务，本次索赔由平台兜底处理。", Valid: true},
		CreatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound).Times(2)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), user.ID).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 6,
		Claims90d:        4,
		WarningCount:     2,
		PlatformPayCount: 2,
	}, nil)
	store.EXPECT().GetPlatformConfig(gomock.Any(), gomock.Any()).Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	store.EXPECT().CreateBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, user.ID, arg.UserID)
			require.Equal(t, "damage", arg.ClaimType)
			require.Equal(t, "auto", arg.ApprovalType)
			require.Equal(t, "platform", arg.CompensationSource)
			require.False(t, arg.CreateRecovery)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: claim,
				PayoutAction: &db.BehaviorAction{
					ID:           922,
					DecisionID:   921,
					ActionType:   "payout",
					TargetEntity: "user",
					Status:       "pending",
				},
				BehaviorDecision: db.BehaviorDecision{
					ID:                   921,
					OrderID:              pgtype.Int8{Int64: order.ID, Valid: true},
					DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModeUserRestricted, Valid: true},
					ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainUser, Valid: true},
					CompensationSource:   "platform",
					RestrictionReason:    pgtype.Text{String: "confirmed_high_user_risk", Valid: true},
					TraceSummary:         pgtype.Text{String: "您的账号因索赔行为异常已被限制服务，本次索赔由平台兜底处理。", Valid: true},
				},
				RestrictionAction: &db.BehaviorAction{
					ID:           923,
					DecisionID:   921,
					ActionType:   "block",
					TargetEntity: "user",
					Status:       "created",
					Detail: mustMarshalJSONDetail(t, restrictionActionDetailFixture{
						Action:            "apply_user_restriction",
						ClaimID:           claim.ID,
						UserID:            user.ID,
						DecisionMode:      "user_restricted",
						RestrictionReason: "confirmed_high_user_risk",
						Remark:            "user restricted action created",
					}),
				},
				NotificationAction: &db.BehaviorAction{
					ID:           924,
					DecisionID:   921,
					ActionType:   "notify",
					TargetEntity: "user",
					Status:       "created",
					Detail: mustMarshalJSONDetail(t, notifyActionDetailFixture{
						Action:           "notify_user_restriction",
						ClaimID:          claim.ID,
						TargetEntity:     "user",
						TargetID:         user.ID,
						RecipientUserID:  user.ID,
						NotificationType: "system",
						Title:            "账户状态变更通知",
						Content:          "由于您的账户存在异常索赔行为，服务已受到限制。如有疑问请联系客服。",
						RelatedType:      "claim",
						RelatedID:        claim.ID,
						Remark:           "notification action created",
					}),
				},
			}, nil
		},
	)

	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
	taskDistributor.EXPECT().DistributeTaskSendNotification(
		gomock.Any(),
		gomock.Any(),
	).DoAndReturn(func(_ any, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
		notifyPayload := payload
		require.Equal(t, user.ID, notifyPayload.UserID)
		require.Contains(t, notifyPayload.Content, "服务已受到限制")
		return nil
	})
	taskDistributor.EXPECT().DistributeTaskClaimPayout(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
	).DoAndReturn(func(_ any, payload any, _ any) error {
		require.IsType(t, &worker.ClaimPayoutPayload{}, payload)
		require.Equal(t, int64(922), payload.(*worker.ClaimPayoutPayload).ActionID)
		return nil
	})

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.SetPaymentClientForTest(mockwechat.NewMockPaymentClientInterface(ctrl))

	body, err := json.Marshal(SubmitClaimRequest{
		OrderID:     order.ID,
		ClaimType:   "damage",
		ClaimAmount: approvedAmount,
		ClaimReason: "用户索赔行为异常但本次仍赔付",
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
	require.Equal(t, "您的账号因索赔行为异常已被限制服务，本次索赔由平台兜底处理。", resp.Reason)
	require.NotNil(t, resp.Warning)
	require.Equal(t, resp.Reason, *resp.Warning)
	require.NotNil(t, resp.PayoutETA)
	require.Equal(t, "1-3个工作日", *resp.PayoutETA)
}

func TestSubmitClaimAPI_RuleActionMerchantRecoveryUsesFormalMode(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.RegionID = 88
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.OrderType = "dinein"
	order.TotalAmount = 5600

	approvedAmount := int64(1600)
	claim := db.Claim{
		ID:                 831,
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "foreign-object",
		Description:        "规则判定商户责任明确",
		ClaimAmount:        approvedAmount,
		ApprovedAmount:     pgtype.Int8{Int64: approvedAmount, Valid: true},
		Status:             "auto-approved",
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "规则判定商户责任明确", Valid: true},
		CreatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().GetPlatformConfig(gomock.Any(), gomock.Any()).Return(db.PlatformConfig{}, db.ErrRecordNotFound).Times(2)
	store.EXPECT().GetAbnormalStatsSummary(gomock.Any(), gomock.Any()).Return(db.GetAbnormalStatsSummaryRow{}, nil).Times(4)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{}, db.ErrRecordNotFound)
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), user.ID).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 12,
		Claims90d:        0,
		WarningCount:     0,
		PlatformPayCount: 0,
	}, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, user.ID, arg.UserID)
			require.Equal(t, "foreign-object", arg.ClaimType)
			require.Equal(t, "auto", arg.ApprovalType)
			require.Equal(t, "merchant", arg.CompensationSource)
			require.True(t, arg.CreateRecovery)
			require.Equal(t, "merchant", arg.RecoveryTarget)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: claim,
				PayoutAction: &db.BehaviorAction{
					ID:           932,
					DecisionID:   931,
					ActionType:   "payout",
					TargetEntity: "user",
					Status:       "pending",
				},
				BehaviorDecision: db.BehaviorDecision{
					ID:                   931,
					OrderID:              pgtype.Int8{Int64: order.ID, Valid: true},
					DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModeMerchantRecovery, Valid: true},
					ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainMerchant, Valid: true},
					CompensationSource:   "merchant",
					TraceSummary:         pgtype.Text{String: "规则判定商户责任明确", Valid: true},
				},
			}, nil
		},
	)

	server := newTestServer(t, store)
	server.config.RulesEngineEnabled = true
	server.rulesEngine = stubRulesEngine{decision: rules.Decision{
		Allow:  true,
		Action: "merchant-recovery",
		Reason: "规则判定商户责任明确",
	}}
	server.SetPaymentClientForTest(mockwechat.NewMockPaymentClientInterface(ctrl))
	server.wsHub = nil
	server.wsPubSub = nil

	body, err := json.Marshal(SubmitClaimRequest{
		OrderID:     order.ID,
		ClaimType:   "foreign-object",
		ClaimAmount: approvedAmount,
		ClaimReason: "规则判定商户责任明确",
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
	require.Equal(t, "merchant", resp.CompensationSource)
	require.Equal(t, "规则判定商户责任明确", resp.Reason)
	require.NotNil(t, resp.PayoutETA)
	require.Equal(t, "1-3个工作日", *resp.PayoutETA)
	require.Nil(t, resp.Warning)
}

func TestSubmitClaimAPI_RuleActionRiderRecoveryUsesFormalMode(t *testing.T) {
	user, _ := randomUser(t)
	riderUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.RegionID = 66
	rider := randomRider(riderUser.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.OrderType = "takeout"
	order.TotalAmount = 5600

	approvedAmount := int64(1900)
	claim := db.Claim{
		ID:                 836,
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "damage",
		Description:        "规则判定骑手责任明确",
		ClaimAmount:        approvedAmount,
		ApprovedAmount:     pgtype.Int8{Int64: approvedAmount, Valid: true},
		Status:             "auto-approved",
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "规则判定骑手责任明确", Valid: true},
		CreatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().GetPlatformConfig(gomock.Any(), gomock.Any()).Return(db.PlatformConfig{}, db.ErrRecordNotFound).Times(2)
	store.EXPECT().GetAbnormalStatsSummary(gomock.Any(), gomock.Any()).Return(db.GetAbnormalStatsSummaryRow{}, nil).Times(6)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{
		OrderID: order.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	}, nil).Times(2)
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), user.ID).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 10,
		Claims90d:        1,
		WarningCount:     0,
		PlatformPayCount: 0,
	}, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, user.ID, arg.UserID)
			require.Equal(t, "damage", arg.ClaimType)
			require.Equal(t, "auto", arg.ApprovalType)
			require.Equal(t, "rider", arg.CompensationSource)
			require.Equal(t, "rider", arg.ResponsibleParty)
			require.True(t, arg.CreateRecovery)
			require.Equal(t, "rider", arg.RecoveryTarget)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: claim,
				PayoutAction: &db.BehaviorAction{
					ID:           937,
					DecisionID:   936,
					ActionType:   "payout",
					TargetEntity: "user",
					Status:       "pending",
				},
				BehaviorDecision: db.BehaviorDecision{
					ID:                   936,
					OrderID:              pgtype.Int8{Int64: order.ID, Valid: true},
					DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModeRiderRecovery, Valid: true},
					ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainRider, Valid: true},
					CompensationSource:   "rider",
					TraceSummary:         pgtype.Text{String: "规则判定骑手责任明确", Valid: true},
				},
				NotificationAction: &db.BehaviorAction{
					ID:           938,
					DecisionID:   936,
					ActionType:   "notify",
					TargetEntity: "rider",
					Status:       "created",
					Detail: mustMarshalJSONDetail(t, notifyActionDetailFixture{
						Action:           "notify_responsible_party",
						ClaimID:          claim.ID,
						TargetEntity:     "rider",
						TargetID:         rider.ID,
						RecipientUserID:  rider.UserID,
						NotificationType: "system",
						Title:            "异常订单判责通知",
						Content:          "订单R20260328001的餐损异常索赔已判定由您承担。平台已向用户先行赔付19.00元，并已生成19.00元追偿单，请尽快处理。 判责依据：规则判定骑手责任明确。",
						RelatedType:      "claim",
						RelatedID:        claim.ID,
						Remark:           "notification action created",
					}),
				},
			}, nil
		},
	)

	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
	taskDistributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
			require.Equal(t, rider.UserID, payload.UserID)
			require.Equal(t, "异常订单判责通知", payload.Title)
			require.Contains(t, payload.Content, "已判定由您承担")
			require.Contains(t, payload.Content, "规则判定骑手责任明确")
			return nil
		},
	)
	taskDistributor.EXPECT().DistributeTaskClaimPayout(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, payload *worker.ClaimPayoutPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(937), payload.ActionID)
			return nil
		},
	)
	taskDistributor.EXPECT().DistributeTaskCheckRiderDamage(gomock.Any(), rider.ID, gomock.Any(), gomock.Any()).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.config.RulesEngineEnabled = true
	server.rulesEngine = stubRulesEngine{decision: rules.Decision{
		Allow:  true,
		Action: "rider-recovery",
		Reason: "规则判定骑手责任明确",
	}}
	server.SetPaymentClientForTest(mockwechat.NewMockPaymentClientInterface(ctrl))

	body, err := json.Marshal(SubmitClaimRequest{
		OrderID:     order.ID,
		ClaimType:   "damage",
		ClaimAmount: approvedAmount,
		ClaimReason: "规则判定骑手责任明确",
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
	require.Equal(t, "rider", resp.CompensationSource)
	require.Equal(t, "规则判定骑手责任明确", resp.Reason)
	require.NotNil(t, resp.PayoutETA)
	require.Equal(t, "1-3个工作日", *resp.PayoutETA)
	require.Nil(t, resp.Warning)
}

func TestSubmitClaimAPI_RuleOverrideDoesNotDispatchNotificationWithoutPersistedAction(t *testing.T) {
	user, _ := randomUser(t)
	riderUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.RegionID = 66
	rider := randomRider(riderUser.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.OrderType = "takeout"
	order.TotalAmount = 5600

	approvedAmount := int64(1900)
	claim := db.Claim{
		ID:                 837,
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "damage",
		Description:        "规则判定骑手责任明确，但通知必须依赖 persisted action",
		ClaimAmount:        approvedAmount,
		ApprovedAmount:     pgtype.Int8{Int64: approvedAmount, Valid: true},
		Status:             "auto-approved",
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "规则判定骑手责任明确，但通知必须依赖 persisted action", Valid: true},
		CreatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().GetPlatformConfig(gomock.Any(), gomock.Any()).Return(db.PlatformConfig{}, db.ErrRecordNotFound).Times(2)
	store.EXPECT().GetAbnormalStatsSummary(gomock.Any(), gomock.Any()).Return(db.GetAbnormalStatsSummaryRow{}, nil).Times(6)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{
		OrderID: order.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	}, nil).Times(2)
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), user.ID).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 10,
		Claims90d:        1,
		WarningCount:     0,
		PlatformPayCount: 0,
	}, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, user.ID, arg.UserID)
			require.Equal(t, "damage", arg.ClaimType)
			require.Equal(t, "auto", arg.ApprovalType)
			require.Equal(t, "rider", arg.CompensationSource)
			require.Equal(t, "rider", arg.ResponsibleParty)
			require.True(t, arg.CreateRecovery)
			require.Equal(t, "rider", arg.RecoveryTarget)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: claim,
				PayoutAction: &db.BehaviorAction{
					ID:           947,
					DecisionID:   946,
					ActionType:   "payout",
					TargetEntity: "user",
					Status:       "pending",
				},
				BehaviorDecision: db.BehaviorDecision{
					ID:                   946,
					OrderID:              pgtype.Int8{Int64: order.ID, Valid: true},
					DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModeRiderRecovery, Valid: true},
					ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainRider, Valid: true},
					CompensationSource:   "rider",
					TraceSummary:         pgtype.Text{String: "规则判定骑手责任明确，但通知必须依赖 persisted action", Valid: true},
				},
				NotificationAction: nil,
			}, nil
		},
	)

	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
	taskDistributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any()).Times(0)
	taskDistributor.EXPECT().DistributeTaskClaimPayout(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, payload *worker.ClaimPayoutPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(947), payload.ActionID)
			return nil
		},
	)
	taskDistributor.EXPECT().DistributeTaskCheckRiderDamage(gomock.Any(), rider.ID, gomock.Any(), gomock.Any()).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.config.RulesEngineEnabled = true
	server.rulesEngine = stubRulesEngine{decision: rules.Decision{
		Allow:  true,
		Action: "rider-recovery",
		Reason: "规则判定骑手责任明确，但通知必须依赖 persisted action",
	}}
	server.SetPaymentClientForTest(mockwechat.NewMockPaymentClientInterface(ctrl))

	body, err := json.Marshal(SubmitClaimRequest{
		OrderID:     order.ID,
		ClaimType:   "damage",
		ClaimAmount: approvedAmount,
		ClaimReason: "规则判定骑手责任明确，但通知必须依赖 persisted action",
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
	require.Equal(t, "rider", resp.CompensationSource)
	require.Equal(t, submitClaimPayoutStatusProcessing, resp.PayoutStatus)
	require.Nil(t, resp.Warning)
}

func TestSubmitClaimAPI_RuleActionUserRestrictedUsesFormalMode(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.RegionID = 77
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.OrderType = "dinein"
	order.TotalAmount = 5600

	approvedAmount := int64(1700)
	claim := db.Claim{
		ID:                 841,
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "foreign-object",
		Description:        "规则命中高风险用户，本次已限制服务并由平台兜底",
		ClaimAmount:        approvedAmount,
		ApprovedAmount:     pgtype.Int8{Int64: approvedAmount, Valid: true},
		Status:             "auto-approved",
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "规则命中高风险用户，本次已限制服务并由平台兜底", Valid: true},
		CreatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound).Times(2)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().GetPlatformConfig(gomock.Any(), gomock.Any()).Return(db.PlatformConfig{}, db.ErrRecordNotFound).Times(3)
	store.EXPECT().GetAbnormalStatsSummary(gomock.Any(), gomock.Any()).Return(db.GetAbnormalStatsSummaryRow{}, nil).Times(4)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{}, db.ErrRecordNotFound)
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), user.ID).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 12,
		Claims90d:        0,
		WarningCount:     0,
		PlatformPayCount: 0,
	}, nil)
	store.EXPECT().CreateBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, user.ID, arg.UserID)
			require.Equal(t, "foreign-object", arg.ClaimType)
			require.Equal(t, "auto", arg.ApprovalType)
			require.Equal(t, "platform", arg.CompensationSource)
			require.Equal(t, "user", arg.ResponsibleParty)
			require.False(t, arg.CreateRecovery)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: claim,
				PayoutAction: &db.BehaviorAction{
					ID:           942,
					DecisionID:   941,
					ActionType:   "payout",
					TargetEntity: "user",
					Status:       "pending",
				},
				BehaviorDecision: db.BehaviorDecision{
					ID:                   941,
					OrderID:              pgtype.Int8{Int64: order.ID, Valid: true},
					DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModeUserRestricted, Valid: true},
					ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainUser, Valid: true},
					CompensationSource:   "platform",
					RestrictionReason:    pgtype.Text{String: "confirmed_high_user_risk", Valid: true},
					TraceSummary:         pgtype.Text{String: "规则命中高风险用户，本次已限制服务并由平台兜底", Valid: true},
				},
				RestrictionAction: &db.BehaviorAction{
					ID:           943,
					DecisionID:   941,
					ActionType:   "block",
					TargetEntity: "user",
					Status:       "created",
					Detail: mustMarshalJSONDetail(t, restrictionActionDetailFixture{
						Action:            "apply_user_restriction",
						ClaimID:           claim.ID,
						UserID:            user.ID,
						DecisionMode:      "user_restricted",
						RestrictionReason: "confirmed_high_user_risk",
						Remark:            "user restricted action created",
					}),
				},
				NotificationAction: &db.BehaviorAction{
					ID:           944,
					DecisionID:   941,
					ActionType:   "notify",
					TargetEntity: "user",
					Status:       "created",
					Detail: mustMarshalJSONDetail(t, notifyActionDetailFixture{
						Action:           "notify_user_restriction",
						ClaimID:          claim.ID,
						TargetEntity:     "user",
						TargetID:         user.ID,
						RecipientUserID:  user.ID,
						NotificationType: "system",
						Title:            "账户状态变更通知",
						Content:          "由于您的账户存在异常索赔行为，服务已受到限制。如有疑问请联系客服。",
						RelatedType:      "claim",
						RelatedID:        claim.ID,
						Remark:           "notification action created",
					}),
				},
			}, nil
		},
	)

	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
	taskDistributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
			require.Equal(t, user.ID, payload.UserID)
			require.Contains(t, payload.Content, "服务已受到限制")
			return nil
		},
	)
	taskDistributor.EXPECT().DistributeTaskClaimPayout(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, payload *worker.ClaimPayoutPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(942), payload.ActionID)
			return nil
		},
	)
	taskDistributor.EXPECT().DistributeTaskCheckMerchantForeignObject(gomock.Any(), order.MerchantID, gomock.Any(), gomock.Any()).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.config.RulesEngineEnabled = true
	server.rulesEngine = stubRulesEngine{decision: rules.Decision{
		Allow:  true,
		Action: "user-restricted",
		Reason: "规则命中高风险用户，本次已限制服务并由平台兜底",
	}}
	server.SetPaymentClientForTest(mockwechat.NewMockPaymentClientInterface(ctrl))

	body, err := json.Marshal(SubmitClaimRequest{
		OrderID:     order.ID,
		ClaimType:   "foreign-object",
		ClaimAmount: approvedAmount,
		ClaimReason: "规则命中高风险用户，本次已限制服务并由平台兜底",
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
	require.Equal(t, "规则命中高风险用户，本次已限制服务并由平台兜底", resp.Reason)
	require.NotNil(t, resp.Warning)
	require.Equal(t, resp.Reason, *resp.Warning)
	require.NotNil(t, resp.PayoutETA)
	require.Equal(t, "1-3个工作日", *resp.PayoutETA)
}

func TestSubmitClaimAPI_RuleOverrideDoesNotApplyRestrictionWithoutPersistedAction(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.RegionID = 77
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.OrderType = "dinein"
	order.TotalAmount = 5600

	approvedAmount := int64(1700)
	claim := db.Claim{
		ID:                 842,
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "foreign-object",
		Description:        "规则命中高风险用户，但限制动作必须依赖 persisted action",
		ClaimAmount:        approvedAmount,
		ApprovedAmount:     pgtype.Int8{Int64: approvedAmount, Valid: true},
		Status:             "auto-approved",
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "规则命中高风险用户，但限制动作必须依赖 persisted action", Valid: true},
		CreatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().GetPlatformConfig(gomock.Any(), gomock.Any()).Return(db.PlatformConfig{}, db.ErrRecordNotFound).Times(2)
	store.EXPECT().GetAbnormalStatsSummary(gomock.Any(), gomock.Any()).Return(db.GetAbnormalStatsSummaryRow{}, nil).Times(4)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{}, db.ErrRecordNotFound)
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), user.ID).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 12,
		Claims90d:        0,
		WarningCount:     0,
		PlatformPayCount: 0,
	}, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateBehaviorBlocklist(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, user.ID, arg.UserID)
			require.Equal(t, "foreign-object", arg.ClaimType)
			require.Equal(t, "auto", arg.ApprovalType)
			require.Equal(t, "platform", arg.CompensationSource)
			require.Equal(t, "user", arg.ResponsibleParty)
			require.False(t, arg.CreateRecovery)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: claim,
				PayoutAction: &db.BehaviorAction{
					ID:           952,
					DecisionID:   951,
					ActionType:   "payout",
					TargetEntity: "user",
					Status:       "pending",
				},
				BehaviorDecision: db.BehaviorDecision{
					ID:                   951,
					OrderID:              pgtype.Int8{Int64: order.ID, Valid: true},
					DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModeUserRestricted, Valid: true},
					ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainUser, Valid: true},
					CompensationSource:   "platform",
					RestrictionReason:    pgtype.Text{String: "confirmed_high_user_risk", Valid: true},
					TraceSummary:         pgtype.Text{String: "规则命中高风险用户，但限制动作必须依赖 persisted action", Valid: true},
				},
				RestrictionAction: nil,
			}, nil
		},
	)

	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
	taskDistributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any()).Times(0)
	taskDistributor.EXPECT().DistributeTaskClaimPayout(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, payload *worker.ClaimPayoutPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(952), payload.ActionID)
			return nil
		},
	)
	taskDistributor.EXPECT().DistributeTaskCheckMerchantForeignObject(gomock.Any(), order.MerchantID, gomock.Any(), gomock.Any()).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.config.RulesEngineEnabled = true
	server.rulesEngine = stubRulesEngine{decision: rules.Decision{
		Allow:  true,
		Action: "user-restricted",
		Reason: "规则命中高风险用户，但限制动作必须依赖 persisted action",
	}}
	server.SetPaymentClientForTest(mockwechat.NewMockPaymentClientInterface(ctrl))

	body, err := json.Marshal(SubmitClaimRequest{
		OrderID:     order.ID,
		ClaimType:   "foreign-object",
		ClaimAmount: approvedAmount,
		ClaimReason: "规则命中高风险用户，但限制动作必须依赖 persisted action",
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
	require.Equal(t, "platform", resp.CompensationSource)
	require.Equal(t, submitClaimPayoutStatusProcessing, resp.PayoutStatus)
	require.NotNil(t, resp.Warning)
	require.Equal(t, "规则命中高风险用户，但限制动作必须依赖 persisted action", *resp.Warning)
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
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
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
