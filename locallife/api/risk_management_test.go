package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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
		Status:         db.ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:   pgtype.Text{String: "auto", Valid: true},
		CreatedAt:      time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, order.ID, arg.OrderID)
			require.Equal(t, user.ID, arg.UserID)
			require.Equal(t, "foreign-object", arg.ClaimType)
			require.Equal(t, approvedAmount, arg.ClaimAmount)
			require.Equal(t, "instant", arg.ApprovalType)
			require.Equal(t, "merchant", arg.CompensationSource)
			require.NotNil(t, arg.ApprovedAmount)
			require.Equal(t, approvedAmount, *arg.ApprovedAmount)
			require.True(t, arg.CreateRecovery)
			require.Equal(t, "merchant", arg.RecoveryTarget)
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
					DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModeMerchantRecovery, Valid: true},
					ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainMerchant, Valid: true},
					CompensationSource:   "merchant",
					TraceSummary:         pgtype.Text{String: "销售侧异常索赔默认由商户承担责任", Valid: true},
				},
			}, nil
		},
	)

	server := newTestServer(t, store)
	server.SetTransferClientForTest(mockwechat.NewMockTransferClientInterface(ctrl))
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
	require.Equal(t, submitClaimStatusWaitingCustomerConfirm, resp.Status)
	require.Equal(t, submitClaimDecisionStatusAutoAdjudicated, resp.DecisionStatus)
	require.Equal(t, submitClaimCompensationStatusAwaiting, resp.CompensationStatus)
	require.Empty(t, resp.PayoutStatus)
	require.True(t, resp.CustomerActionRequired)
	require.Equal(t, claimCustomerActionConfirmContinue, resp.CustomerAction)
	require.NotNil(t, resp.ApprovedAmount)
	require.Equal(t, approvedAmount, *resp.ApprovedAmount)
	require.Equal(t, "merchant", resp.CompensationSource)
	require.Equal(t, "销售侧异常索赔默认由商户承担责任", resp.Reason)
	require.Nil(t, resp.Warning)

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
	require.Equal(t, resp.CompensationStatus, entries[0].Metadata["compensation_status"])
	require.Equal(t, order.ID, entries[0].Metadata["order_id"])
	require.Equal(t, "foreign-object", entries[0].Metadata["claim_type"])
	require.Equal(t, approvedAmount, entries[0].Metadata["requested_amount"])
	require.Equal(t, approvedAmount, entries[0].Metadata["approved_amount"])
	require.Equal(t, "merchant", entries[0].Metadata["compensation_source"])
	_, warningExists := entries[0].Metadata["warning"]
	require.False(t, warningExists)
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
		Status:         db.ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:   pgtype.Text{String: "auto", Valid: true},
		CreatedAt:      time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
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
	taskDistributor.EXPECT().DistributeTaskCheckMerchantForeignObject(gomock.Any(), order.MerchantID, gomock.Any(), gomock.Any()).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.SetTransferClientForTest(mockwechat.NewMockTransferClientInterface(ctrl))

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
	require.Equal(t, submitClaimStatusWaitingCustomerConfirm, resp.Status)
	require.Equal(t, submitClaimCompensationStatusAwaiting, resp.CompensationStatus)
	require.Empty(t, resp.PayoutStatus)
	require.True(t, resp.CustomerActionRequired)
	require.Equal(t, claimCustomerActionConfirmContinue, resp.CustomerAction)
	require.Equal(t, "merchant", resp.CompensationSource)
}

func TestSubmitClaimAPI_UsesPersistedRiderRecoveryOutcome(t *testing.T) {
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
		Status:             db.ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "服务侧异常索赔默认由骑手承担责任", Valid: true},
		CreatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
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
					DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModeRiderRecovery, Valid: true},
					ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainRider, Valid: true},
					CompensationSource:   "rider",
					TraceSummary:         pgtype.Text{String: "服务侧异常索赔默认由骑手承担责任", Valid: true},
				},
			}, nil
		},
	)

	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
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
	server.SetTransferClientForTest(mockwechat.NewMockTransferClientInterface(ctrl))

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
	require.Equal(t, submitClaimStatusWaitingCustomerConfirm, resp.Status)
	require.Equal(t, submitClaimDecisionStatusAutoAdjudicated, resp.DecisionStatus)
	require.Equal(t, submitClaimCompensationStatusAwaiting, resp.CompensationStatus)
	require.Empty(t, resp.PayoutStatus)
	require.True(t, resp.CustomerActionRequired)
	require.Equal(t, claimCustomerActionConfirmContinue, resp.CustomerAction)
	require.NotNil(t, resp.ApprovedAmount)
	require.Equal(t, approvedAmount, *resp.ApprovedAmount)
	require.Equal(t, "rider", resp.CompensationSource)
	require.Equal(t, "服务侧异常索赔默认由骑手承担责任", resp.Reason)
	require.Nil(t, resp.Warning)
}

func TestSubmitClaimAPI_LegacyPlatformRuleDoesNotOverrideDeterministicRecovery(t *testing.T) {
	user, _ := randomUser(t)
	riderUser, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.RegionID = 66
	_ = riderUser
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
		Description:        "遗留规则输出 platform-fallback 时，仍应保持服务侧追偿",
		ClaimAmount:        approvedAmount,
		ApprovedAmount:     pgtype.Int8{Int64: approvedAmount, Valid: true},
		Status:             db.ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "服务侧异常索赔默认由骑手承担责任", Valid: true},
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
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{
		OrderID: order.ID,
		RiderID: pgtype.Int8{Int64: 0, Valid: false},
	}, nil).Times(2)
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
					ID:           918,
					DecisionID:   917,
					ActionType:   "payout",
					TargetEntity: "user",
					Status:       "pending",
				},
				BehaviorDecision: db.BehaviorDecision{
					ID:                   917,
					OrderID:              pgtype.Int8{Int64: order.ID, Valid: true},
					DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModeRiderRecovery, Valid: true},
					ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainRider, Valid: true},
					CompensationSource:   "rider",
					TraceSummary:         pgtype.Text{String: "服务侧异常索赔默认由骑手承担责任", Valid: true},
				},
			}, nil
		},
	)
	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
	taskDistributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any()).Times(0)
	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.config.RulesEngineEnabled = true
	server.rulesEngine = stubRulesEngine{decision: rules.Decision{
		Allow:  true,
		Action: "platform-fallback",
		Reason: "遗留规则输出 platform-fallback",
	}}
	server.SetTransferClientForTest(mockwechat.NewMockTransferClientInterface(ctrl))

	body, err := json.Marshal(SubmitClaimRequest{
		OrderID:     order.ID,
		ClaimType:   "damage",
		ClaimAmount: approvedAmount,
		ClaimReason: "遗留规则输出 platform-fallback",
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
	require.Equal(t, submitClaimStatusWaitingCustomerConfirm, resp.Status)
	require.Equal(t, submitClaimCompensationStatusAwaiting, resp.CompensationStatus)
	require.Empty(t, resp.PayoutStatus)
	require.True(t, resp.CustomerActionRequired)
	require.Equal(t, claimCustomerActionConfirmContinue, resp.CustomerAction)
	require.Equal(t, "rider", resp.CompensationSource)
	require.Equal(t, "服务侧异常索赔默认由骑手承担责任", resp.Reason)
	require.Nil(t, resp.Warning)
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
		Status:             db.ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "您的账号因索赔行为异常已被限制服务；若确认继续索赔，平台将先行赔付并停止后续服务。", Valid: true},
		CreatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetPlatformConfig(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().CreateBehaviorBlocklist(gomock.Any(), gomock.Any()).Times(0)
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
					TraceSummary:         pgtype.Text{String: "您的账号因索赔行为异常已被限制服务；若确认继续索赔，平台将先行赔付并停止后续服务。", Valid: true},
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
	taskDistributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.SetTransferClientForTest(mockwechat.NewMockTransferClientInterface(ctrl))

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
	require.Equal(t, submitClaimStatusWaitingCustomerConfirm, resp.Status)
	require.Equal(t, submitClaimDecisionStatusAutoAdjudicated, resp.DecisionStatus)
	require.Equal(t, submitClaimCompensationStatusAwaiting, resp.CompensationStatus)
	require.Empty(t, resp.PayoutStatus)
	require.True(t, resp.CustomerActionRequired)
	require.Equal(t, claimCustomerActionConfirmContinue, resp.CustomerAction)
	require.NotNil(t, resp.ApprovedAmount)
	require.Equal(t, approvedAmount, *resp.ApprovedAmount)
	require.Equal(t, "platform", resp.CompensationSource)
	require.Equal(t, "您的账号因索赔行为异常已被限制服务；若确认继续索赔，平台将先行赔付并停止后续服务。", resp.Reason)
	require.NotNil(t, resp.Warning)
	require.Equal(t, resp.Reason, *resp.Warning)
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
		Status:             db.ClaimStatusWaitingCustomerConfirmation,
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
	server.SetTransferClientForTest(mockwechat.NewMockTransferClientInterface(ctrl))
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
	require.Equal(t, submitClaimStatusWaitingCustomerConfirm, resp.Status)
	require.Equal(t, submitClaimDecisionStatusAutoAdjudicated, resp.DecisionStatus)
	require.Equal(t, submitClaimCompensationStatusAwaiting, resp.CompensationStatus)
	require.Empty(t, resp.PayoutStatus)
	require.True(t, resp.CustomerActionRequired)
	require.Equal(t, claimCustomerActionConfirmContinue, resp.CustomerAction)
	require.NotNil(t, resp.ApprovedAmount)
	require.Equal(t, approvedAmount, *resp.ApprovedAmount)
	require.Equal(t, "merchant", resp.CompensationSource)
	require.Equal(t, "规则判定商户责任明确", resp.Reason)
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
		Status:             db.ClaimStatusWaitingCustomerConfirmation,
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
	taskDistributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any()).Times(0)
	taskDistributor.EXPECT().DistributeTaskCheckRiderDamage(gomock.Any(), rider.ID, gomock.Any(), gomock.Any()).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.config.RulesEngineEnabled = true
	server.rulesEngine = stubRulesEngine{decision: rules.Decision{
		Allow:  true,
		Action: "rider-recovery",
		Reason: "规则判定骑手责任明确",
	}}
	server.SetTransferClientForTest(mockwechat.NewMockTransferClientInterface(ctrl))

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
	require.Equal(t, submitClaimStatusWaitingCustomerConfirm, resp.Status)
	require.Equal(t, submitClaimDecisionStatusAutoAdjudicated, resp.DecisionStatus)
	require.Equal(t, submitClaimCompensationStatusAwaiting, resp.CompensationStatus)
	require.Empty(t, resp.PayoutStatus)
	require.True(t, resp.CustomerActionRequired)
	require.Equal(t, claimCustomerActionConfirmContinue, resp.CustomerAction)
	require.NotNil(t, resp.ApprovedAmount)
	require.Equal(t, approvedAmount, *resp.ApprovedAmount)
	require.Equal(t, "rider", resp.CompensationSource)
	require.Equal(t, "规则判定骑手责任明确", resp.Reason)
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
		Status:             db.ClaimStatusWaitingCustomerConfirmation,
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
	taskDistributor.EXPECT().DistributeTaskCheckRiderDamage(gomock.Any(), rider.ID, gomock.Any(), gomock.Any()).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.config.RulesEngineEnabled = true
	server.rulesEngine = stubRulesEngine{decision: rules.Decision{
		Allow:  true,
		Action: "rider-recovery",
		Reason: "规则判定骑手责任明确，但通知必须依赖 persisted action",
	}}
	server.SetTransferClientForTest(mockwechat.NewMockTransferClientInterface(ctrl))

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
	require.Equal(t, submitClaimCompensationStatusAwaiting, resp.CompensationStatus)
	require.Empty(t, resp.PayoutStatus)
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
		Description:        "规则命中高风险用户，进入限制服务与平台先行赔付流程",
		ClaimAmount:        approvedAmount,
		ApprovedAmount:     pgtype.Int8{Int64: approvedAmount, Valid: true},
		Status:             db.ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "规则命中高风险用户；若确认继续索赔，平台将先行赔付并停止后续服务", Valid: true},
		CreatedAt:          time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).Return([]db.Claim{}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().GetPlatformConfig(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.GetPlatformConfigParams) (db.PlatformConfig, error) {
			switch arg.ConfigKey {
			case "behavior_trace.window_days":
				return db.PlatformConfig{
					ConfigKey:   arg.ConfigKey,
					ConfigValue: []byte(`{"window_7d":7,"window_30d":30}`),
					ScopeType:   arg.ScopeType,
				}, nil
			case "behavior_trace.abnormal_thresholds":
				return db.PlatformConfig{
					ConfigKey:   arg.ConfigKey,
					ConfigValue: []byte(`{"user_claim_rate_7d":0.3,"user_claim_rate_30d":0.2,"user_claims_7d":3,"user_claims_30d":5,"merchant_abnormal_rate":0.08,"rider_abnormal_rate":0.06}`),
					ScopeType:   arg.ScopeType,
				}, nil
			default:
				return db.PlatformConfig{}, db.ErrRecordNotFound
			}
		},
	).Times(2)
	store.EXPECT().GetAbnormalStatsSummary(gomock.Any(), gomock.Any()).Return(db.GetAbnormalStatsSummaryRow{}, nil).Times(4)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{}, db.ErrRecordNotFound)
	store.EXPECT().CreateBehaviorBlocklist(gomock.Any(), gomock.Any()).Times(0)
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
					TraceSummary:         pgtype.Text{String: "规则命中高风险用户；若确认继续索赔，平台将先行赔付并停止后续服务", Valid: true},
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
	taskDistributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any()).Times(0)
	taskDistributor.EXPECT().DistributeTaskCheckMerchantForeignObject(gomock.Any(), order.MerchantID, gomock.Any(), gomock.Any()).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.config.RulesEngineEnabled = true
	server.rulesEngine = stubRulesEngine{decision: rules.Decision{
		Allow:  true,
		Action: "user-restricted",
		Reason: "规则命中高风险用户；若确认继续索赔，平台将先行赔付并停止后续服务",
	}}
	server.SetTransferClientForTest(mockwechat.NewMockTransferClientInterface(ctrl))

	body, err := json.Marshal(SubmitClaimRequest{
		OrderID:     order.ID,
		ClaimType:   "foreign-object",
		ClaimAmount: approvedAmount,
		ClaimReason: "规则命中高风险用户；若确认继续索赔，平台将先行赔付并停止后续服务",
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
	require.Equal(t, submitClaimStatusWaitingCustomerConfirm, resp.Status)
	require.Equal(t, submitClaimDecisionStatusAutoAdjudicated, resp.DecisionStatus)
	require.Equal(t, submitClaimCompensationStatusAwaiting, resp.CompensationStatus)
	require.Empty(t, resp.PayoutStatus)
	require.True(t, resp.CustomerActionRequired)
	require.Equal(t, claimCustomerActionConfirmContinue, resp.CustomerAction)
	require.NotNil(t, resp.ApprovedAmount)
	require.Equal(t, approvedAmount, *resp.ApprovedAmount)
	require.Equal(t, "platform", resp.CompensationSource)
	require.Equal(t, "规则命中高风险用户；若确认继续索赔，平台将先行赔付并停止后续服务", resp.Reason)
	require.NotNil(t, resp.Warning)
	require.Equal(t, resp.Reason, *resp.Warning)
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
		Status:             db.ClaimStatusWaitingCustomerConfirmation,
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
	taskDistributor.EXPECT().DistributeTaskCheckMerchantForeignObject(gomock.Any(), order.MerchantID, gomock.Any(), gomock.Any()).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	server.config.RulesEngineEnabled = true
	server.rulesEngine = stubRulesEngine{decision: rules.Decision{
		Allow:  true,
		Action: "user-restricted",
		Reason: "规则命中高风险用户，但限制动作必须依赖 persisted action",
	}}
	server.SetTransferClientForTest(mockwechat.NewMockTransferClientInterface(ctrl))

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
	require.Equal(t, submitClaimCompensationStatusAwaiting, resp.CompensationStatus)
	require.Empty(t, resp.PayoutStatus)
	require.NotNil(t, resp.Warning)
	require.Equal(t, "规则命中高风险用户，但限制动作必须依赖 persisted action", *resp.Warning)
}

func TestSubmitClaimAPI_ApprovedPayoutRequiresTransferClient(t *testing.T) {
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
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).Return(db.CreateClaimWithBehaviorTxResult{
		Claim: db.Claim{
			ID:             851,
			OrderID:        order.ID,
			UserID:         user.ID,
			ClaimType:      "foreign-object",
			Description:    "餐品里发现异物，需要平台介入处理",
			ClaimAmount:    approvedAmount,
			ApprovedAmount: pgtype.Int8{Int64: approvedAmount, Valid: true},
			Status:         db.ClaimStatusWaitingCustomerConfirmation,
			ApprovalType:   pgtype.Text{String: "auto", Valid: true},
			CreatedAt:      time.Now(),
		},
		BehaviorDecision: db.BehaviorDecision{
			ID:                   852,
			OrderID:              pgtype.Int8{Int64: order.ID, Valid: true},
			DecisionMode:         pgtype.Text{String: db.BehaviorDecisionModeMerchantRecovery, Valid: true},
			ResponsibilityDomain: pgtype.Text{String: db.BehaviorResponsibilityDomainMerchant, Valid: true},
			CompensationSource:   "merchant",
			TraceSummary:         pgtype.Text{String: "销售侧异常索赔默认由商户承担责任", Valid: true},
		},
	}, nil)

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

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp SubmitClaimResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, submitClaimCompensationStatusAwaiting, resp.CompensationStatus)
	require.Empty(t, resp.PayoutStatus)
}

func TestSubmitClaimAPI_DuplicateOrderClaimReturnsConflict(t *testing.T) {
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
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return([]db.UserDevice{}, nil)
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).Return(db.CreateClaimWithBehaviorTxResult{}, &pgconn.PgError{
		Code:           db.UniqueViolation,
		ConstraintName: "claims_order_id_unique",
	})

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

	require.Equal(t, http.StatusConflict, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrOrderAlreadyHasClaim.Message, resp.Error)
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
	require.Equal(t, submitClaimCompensationStatusCompensated, resp.Claims[0].CompensationStatus)
	require.Equal(t, submitClaimPayoutStatusPaid, resp.Claims[0].PayoutStatus)
	require.Equal(t, "平台已核验并完成赔付", resp.Claims[0].Reason)
	require.NotNil(t, resp.Claims[0].ProcessedAt)

	require.Equal(t, submitClaimStatusRejected, resp.Claims[1].Status)
	require.Equal(t, submitClaimDecisionStatusRejected, resp.Claims[1].DecisionStatus)
	require.Empty(t, resp.Claims[1].CompensationStatus)
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
	require.Equal(t, submitClaimCompensationStatusCompensating, resp.CompensationStatus)
	require.Equal(t, submitClaimPayoutStatusProcessing, resp.PayoutStatus)
	require.False(t, resp.CustomerActionRequired)
	require.Empty(t, resp.CustomerAction)
	require.Equal(t, "平台已完成自动裁定，赔付正在处理中", resp.Reason)
	require.NotNil(t, resp.ApprovedAmount)
	require.Equal(t, int64(1500), *resp.ApprovedAmount)
	require.Nil(t, resp.ProcessedAt)
}

func TestConfirmContinueClaimAPI_TriggersDeferredCompensation(t *testing.T) {
	user, _ := randomUser(t)
	now := time.Now()
	claim := db.Claim{
		ID:                 904,
		OrderID:            3004,
		UserID:             user.ID,
		ClaimType:          "foreign-object",
		Description:        "餐品中发现异物",
		ClaimAmount:        1800,
		ApprovedAmount:     pgtype.Int8{Int64: 1500, Valid: true},
		Status:             db.ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "平台已完成自动裁定，等待用户确认继续", Valid: true},
		CreatedAt:          now.Add(-30 * time.Minute),
	}
	processingClaim := claim
	processingClaim.Status = "approved"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetClaim(gomock.Any(), claim.ID).Return(claim, nil)
	store.EXPECT().CreateClaimCompensationTx(gomock.Any(), db.CreateClaimCompensationTxParams{ClaimID: claim.ID}).Return(db.CreateClaimCompensationTxResult{
		Claim: processingClaim,
		PayoutAction: &db.BehaviorAction{
			ID:           1001,
			DecisionID:   901,
			ActionType:   "payout",
			TargetEntity: "user",
			Status:       "created",
		},
	}, nil)
	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
	taskDistributor.EXPECT().DistributeTaskClaimPayout(
		gomock.Any(),
		&worker.ClaimPayoutPayload{ActionID: 1001},
		gomock.Any(),
		gomock.Any(),
	).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/claims/904/confirm-continue", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp userClaimResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, claim.ID, resp.ID)
	require.Equal(t, submitClaimStatusAccepted, resp.Status)
	require.Equal(t, submitClaimDecisionStatusAutoAdjudicated, resp.DecisionStatus)
	require.Equal(t, submitClaimCompensationStatusCompensating, resp.CompensationStatus)
	require.Equal(t, submitClaimPayoutStatusProcessing, resp.PayoutStatus)
	require.False(t, resp.CustomerActionRequired)
	require.Empty(t, resp.CustomerAction)
	require.Nil(t, resp.ProcessedAt)
	require.NotNil(t, resp.ApprovedAmount)
	require.Equal(t, int64(1500), *resp.ApprovedAmount)
}

func TestConfirmContinueClaimAPI_ReturnsProcessingWhenEnqueueFailsAfterPersistence(t *testing.T) {
	user, _ := randomUser(t)
	now := time.Now()
	claim := db.Claim{
		ID:                 9041,
		OrderID:            30041,
		UserID:             user.ID,
		ClaimType:          "foreign-object",
		Description:        "餐品中发现异物",
		ClaimAmount:        1800,
		ApprovedAmount:     pgtype.Int8{Int64: 1500, Valid: true},
		Status:             db.ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "平台已完成自动裁定，等待用户确认继续", Valid: true},
		CreatedAt:          now.Add(-30 * time.Minute),
	}
	processingClaim := claim
	processingClaim.Status = db.ClaimStatusApproved

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetClaim(gomock.Any(), claim.ID).Return(claim, nil)
	store.EXPECT().CreateClaimCompensationTx(gomock.Any(), db.CreateClaimCompensationTxParams{ClaimID: claim.ID}).Return(db.CreateClaimCompensationTxResult{
		Claim: processingClaim,
		PayoutAction: &db.BehaviorAction{
			ID:           10011,
			DecisionID:   9011,
			ActionType:   "payout",
			TargetEntity: "user",
			Status:       "created",
		},
	}, nil)
	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
	taskDistributor.EXPECT().DistributeTaskClaimPayout(
		gomock.Any(),
		&worker.ClaimPayoutPayload{ActionID: 10011},
		gomock.Any(),
		gomock.Any(),
	).Return(errors.New("queue unavailable"))

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/claims/9041/confirm-continue", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp userClaimResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, claim.ID, resp.ID)
	require.Equal(t, submitClaimStatusAccepted, resp.Status)
	require.Equal(t, submitClaimDecisionStatusAutoAdjudicated, resp.DecisionStatus)
	require.Equal(t, submitClaimCompensationStatusCompensating, resp.CompensationStatus)
	require.Equal(t, submitClaimPayoutStatusProcessing, resp.PayoutStatus)
	require.False(t, resp.CustomerActionRequired)
	require.Empty(t, resp.CustomerAction)
	require.Nil(t, resp.ProcessedAt)
	require.NotNil(t, resp.ApprovedAmount)
	require.Equal(t, int64(1500), *resp.ApprovedAmount)
}

func TestConfirmContinueClaimAPI_IsIdempotentWhenAlreadyProcessing(t *testing.T) {
	user, _ := randomUser(t)
	now := time.Now()
	claim := db.Claim{
		ID:             905,
		OrderID:        3005,
		UserID:         user.ID,
		ClaimType:      "damage",
		Description:    "餐品洒漏",
		ClaimAmount:    2200,
		ApprovedAmount: pgtype.Int8{Int64: 1800, Valid: true},
		Status:         "approved",
		ApprovalType:   pgtype.Text{String: "auto", Valid: true},
		DecisionReason: pgtype.Text{String: "平台已受理，补偿处理中", Valid: true},
		CreatedAt:      now.Add(-20 * time.Minute),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetClaim(gomock.Any(), claim.ID).Return(claim, nil)
	store.EXPECT().CreateClaimCompensationTx(gomock.Any(), db.CreateClaimCompensationTxParams{ClaimID: claim.ID}).Return(db.CreateClaimCompensationTxResult{
		Claim: claim,
		PayoutAction: &db.BehaviorAction{
			ID:           1002,
			DecisionID:   902,
			ActionType:   "payout",
			TargetEntity: "user",
			Status:       "created",
		},
	}, nil)
	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)
	taskDistributor.EXPECT().DistributeTaskClaimPayout(
		gomock.Any(),
		&worker.ClaimPayoutPayload{ActionID: 1002},
		gomock.Any(),
		gomock.Any(),
	).Return(nil)

	server := newTestServerWithTaskDistributor(t, store, taskDistributor)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/claims/905/confirm-continue", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp userClaimResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, submitClaimCompensationStatusCompensating, resp.CompensationStatus)
	require.Equal(t, submitClaimPayoutStatusProcessing, resp.PayoutStatus)
	require.False(t, resp.CustomerActionRequired)
	require.Empty(t, resp.CustomerAction)
	require.Equal(t, "平台已受理，补偿处理中", resp.Reason)
}

func TestWithdrawClaimAPI_ClosesAwaitingCompensationClaim(t *testing.T) {
	user, _ := randomUser(t)
	now := time.Now()
	claim := db.Claim{
		ID:                 909,
		OrderID:            3009,
		UserID:             user.ID,
		ClaimType:          "foreign-object",
		Description:        "餐品中发现异物",
		ClaimAmount:        2000,
		ApprovedAmount:     pgtype.Int8{Int64: 1500, Valid: true},
		Status:             db.ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:       pgtype.Text{String: "auto", Valid: true},
		AutoApprovalReason: pgtype.Text{String: "平台已完成自动裁定，等待用户确认继续", Valid: true},
		CreatedAt:          now.Add(-30 * time.Minute),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetClaim(gomock.Any(), claim.ID).Return(claim, nil)
	store.EXPECT().UpdateClaimStatusIfCurrent(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.UpdateClaimStatusIfCurrentParams) (db.Claim, error) {
			require.Equal(t, claim.ID, arg.ID)
			require.Equal(t, db.ClaimStatusWaitingCustomerConfirmation, arg.CurrentStatus)
			require.Equal(t, db.ClaimStatusWithdrawn, arg.Status)
			require.True(t, arg.ReviewNotes.Valid)
			require.Equal(t, claimReviewNoteCustomerWithdrawn, arg.ReviewNotes.String)
			require.True(t, arg.ReviewedAt.Valid)
			updated := claim
			updated.Status = db.ClaimStatusWithdrawn
			updated.ReviewNotes = arg.ReviewNotes
			updated.ReviewedAt = arg.ReviewedAt
			return updated, nil
		},
	)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/claims/909/withdraw", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp userClaimResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, claim.ID, resp.ID)
	require.Equal(t, submitClaimStatusClosed, resp.Status)
	require.Equal(t, submitClaimDecisionStatusAutoAdjudicated, resp.DecisionStatus)
	require.Empty(t, resp.CompensationStatus)
	require.Empty(t, resp.PayoutStatus)
	require.False(t, resp.CustomerActionRequired)
	require.Empty(t, resp.CustomerAction)
	require.Equal(t, claimReasonCustomerWithdrawn, resp.Reason)
	require.NotNil(t, resp.ProcessedAt)
	require.NotNil(t, resp.ApprovedAmount)
	require.Equal(t, int64(1500), *resp.ApprovedAmount)
}

func TestWithdrawClaimAPI_IsIdempotentWhenAlreadyWithdrawn(t *testing.T) {
	user, _ := randomUser(t)
	now := time.Now()
	claim := db.Claim{
		ID:             910,
		OrderID:        3010,
		UserID:         user.ID,
		ClaimType:      "damage",
		Description:    "餐品破损",
		ClaimAmount:    1800,
		ApprovedAmount: pgtype.Int8{Int64: 1200, Valid: true},
		Status:         db.ClaimStatusWithdrawn,
		ReviewNotes:    pgtype.Text{String: claimReviewNoteCustomerWithdrawn, Valid: true},
		ReviewedAt:     pgtype.Timestamptz{Time: now.Add(-5 * time.Minute), Valid: true},
		CreatedAt:      now.Add(-30 * time.Minute),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetClaim(gomock.Any(), claim.ID).Return(claim, nil)
	store.EXPECT().UpdateClaimStatus(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/claims/910/withdraw", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp userClaimResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, submitClaimStatusClosed, resp.Status)
	require.Equal(t, claimReasonCustomerWithdrawn, resp.Reason)
	require.False(t, resp.CustomerActionRequired)
	require.NotNil(t, resp.ProcessedAt)
}

func TestWithdrawClaimAPI_RejectsClaimAlreadyProcessing(t *testing.T) {
	user, _ := randomUser(t)
	now := time.Now()
	claim := db.Claim{
		ID:             911,
		OrderID:        3011,
		UserID:         user.ID,
		ClaimType:      "timeout",
		Description:    "订单超时送达",
		ClaimAmount:    1600,
		ApprovedAmount: pgtype.Int8{Int64: 1200, Valid: true},
		Status:         "approved",
		CreatedAt:      now.Add(-20 * time.Minute),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetClaim(gomock.Any(), claim.ID).Return(claim, nil)
	store.EXPECT().UpdateClaimStatus(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/claims/911/withdraw", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrClaimCannotBeWithdrawn.Message, resp.Error)
}

func TestConfirmContinueClaimAPI_RejectsIneligibleClaim(t *testing.T) {
	user, _ := randomUser(t)
	now := time.Now()
	claim := db.Claim{
		ID:          906,
		OrderID:     3006,
		UserID:      user.ID,
		ClaimType:   "timeout",
		Description: "订单超时送达",
		ClaimAmount: 1200,
		Status:      db.ClaimStatusWithdrawn,
		ReviewNotes: pgtype.Text{String: claimReviewNoteCustomerWithdrawn, Valid: true},
		ReviewedAt:  pgtype.Timestamptz{Time: now.Add(-5 * time.Minute), Valid: true},
		CreatedAt:   now.Add(-45 * time.Minute),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetClaim(gomock.Any(), claim.ID).Return(claim, nil)
	store.EXPECT().CreateClaimCompensationTx(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/claims/906/confirm-continue", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	errMsg := resp.Error
	if errMsg == "" {
		var envelope APIResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
		errMsg = envelope.Message
	}
	require.Equal(t, ErrClaimCannotContinue.Message, errMsg)
}

func TestConfirmContinueClaimAPI_RejectsClaimNotOwned(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	now := time.Now()
	claim := db.Claim{
		ID:             907,
		OrderID:        3007,
		UserID:         otherUser.ID,
		ClaimType:      "foreign-object",
		Description:    "餐品中发现异物",
		ClaimAmount:    1600,
		ApprovedAmount: pgtype.Int8{Int64: 1200, Valid: true},
		Status:         db.ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:   pgtype.Text{String: "auto", Valid: true},
		CreatedAt:      now.Add(-10 * time.Minute),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetClaim(gomock.Any(), claim.ID).Return(claim, nil)
	store.EXPECT().CreateClaimCompensationTx(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/claims/907/confirm-continue", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrClaimNotOwned.Message, resp.Error)
}

func TestConfirmContinueClaimAPI_RejectsClaimNotFound(t *testing.T) {
	user, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetClaim(gomock.Any(), int64(908)).Return(db.Claim{}, db.ErrRecordNotFound)
	store.EXPECT().CreateClaimCompensationTx(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/claims/908/confirm-continue", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNotFound, recorder.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, ErrClaimNotFound.Message, resp.Error)
}
