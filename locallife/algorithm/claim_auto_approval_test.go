package algorithm

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"go.uber.org/mock/gomock"
)

// MockWebSocketHub 实现 WebSocketHub 接口
type MockWebSocketHub struct {
	merchantMessages map[int64][]websocket.Message
	riderMessages    map[int64][]websocket.Message
}

func NewMockWebSocketHub() *MockWebSocketHub {
	return &MockWebSocketHub{
		merchantMessages: make(map[int64][]websocket.Message),
		riderMessages:    make(map[int64][]websocket.Message),
	}
}

func (m *MockWebSocketHub) SendToMerchant(merchantID int64, msg websocket.Message) {
	m.merchantMessages[merchantID] = append(m.merchantMessages[merchantID], msg)
}

func (m *MockWebSocketHub) SendToRider(riderID int64, msg websocket.Message) {
	m.riderMessages[riderID] = append(m.riderMessages[riderID], msg)
}

// MockNotificationDistributor 实现 NotificationDistributor 接口
type MockNotificationDistributor struct {
	notifications []UserNotification
	err           error
}

type UserNotification struct {
	UserID           int64
	NotificationType string
	Title            string
	Content          string
	RelatedType      string
	RelatedID        int64
}

func (m *MockNotificationDistributor) SendUserNotification(ctx context.Context, userID int64, notificationType, title, content string, relatedType string, relatedID int64) error {
	m.notifications = append(m.notifications, UserNotification{
		UserID:           userID,
		NotificationType: notificationType,
		Title:            title,
		Content:          content,
		RelatedType:      relatedType,
		RelatedID:        relatedID,
	})
	return m.err
}

func testCompensationContext(requestedAmount, orderTotalAmount, deliveryFee int64) ClaimCompensationContext {
	return ClaimCompensationContext{
		RequestedAmount:     requestedAmount,
		OrderTotalAmount:    orderTotalAmount,
		DeliveryFee:         deliveryFee,
		DeliveryFeeDiscount: 0,
	}
}

// ========================================
// EvaluateClaim 测试
// ========================================

func TestEvaluateClaim_FoodSafety_UsesDedicatedWorkflow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	decision, err := caa.EvaluateClaim(
		context.Background(),
		1,   // userID
		100, // orderID
		testCompensationContext(5000, 5000, 300),
		ClaimTypeFoodSafety,
	)

	require.Error(t, err)
	require.Nil(t, decision)
}

func TestEvaluateClaim_Timeout_RiderPays(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	decision, err := caa.EvaluateClaim(
		context.Background(),
		1,   // userID
		100, // orderID
		testCompensationContext(5000, 5000, 300),
		ClaimTypeTimeout,
	)

	require.NoError(t, err)
	require.NotNil(t, decision)
	require.Equal(t, ApprovalTypeInstant, decision.Type)
	require.True(t, decision.Approved)
	require.Equal(t, int64(5000), decision.Amount)
	require.Equal(t, CompensationSourceRider, decision.CompensationSource)
	require.Equal(t, ClaimBehaviorNormal, decision.BehaviorStatus)
	require.Equal(t, "服务侧异常索赔默认由骑手承担责任", decision.Reason)
}

func TestEvaluateClaim_Damage_RiderPays(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	decision, err := caa.EvaluateClaim(
		context.Background(),
		1,   // userID
		100, // orderID
		testCompensationContext(5000, 5000, 300),
		ClaimTypeDamage,
	)

	require.NoError(t, err)
	require.NotNil(t, decision)
	require.Equal(t, ApprovalTypeInstant, decision.Type)
	require.True(t, decision.Approved)
	require.Equal(t, int64(5000), decision.Amount) // 餐损赔全额
	require.Equal(t, CompensationSourceRider, decision.CompensationSource)
	require.Equal(t, "服务侧异常索赔默认由骑手承担责任", decision.Reason)
}

func TestEvaluateClaim_ForeignObject_MerchantPays(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	decision, err := caa.EvaluateClaim(
		context.Background(),
		1,   // userID
		100, // orderID
		testCompensationContext(5000, 5000, 300),
		ClaimTypeForeignObject,
	)

	require.NoError(t, err)
	require.NotNil(t, decision)
	require.True(t, decision.Approved)
	require.Equal(t, int64(5000), decision.Amount)
	require.Equal(t, CompensationSourceMerchant, decision.CompensationSource)
	require.Equal(t, "销售侧异常索赔默认由商户承担责任", decision.Reason)
}

func TestEvaluateClaim_CapsRequestedAmountByEligibleOrderAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	decision, err := caa.EvaluateClaim(
		context.Background(),
		1,
		100,
		ClaimCompensationContext{
			RequestedAmount:     6000,
			OrderTotalAmount:    5000,
			DeliveryFee:         300,
			DeliveryFeeDiscount: 100,
		},
		ClaimTypeDamage,
	)

	require.NoError(t, err)
	require.NotNil(t, decision)
	require.Equal(t, int64(5000), decision.Amount)
}

// ========================================
// CheckRiderDamageHistory 测试
// ========================================

func TestCheckRiderDamageHistory_BelowThreshold(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	// 骑手最近7天只有2次餐损，低于阈值
	store.EXPECT().ListRiderClaims(gomock.Any(), gomock.Any()).Return([]db.Claim{
		{ClaimType: ClaimTypeDamage},
		{ClaimType: ClaimTypeDamage},
	}, nil)

	err := caa.CheckRiderDamageHistory(context.Background(), 1)
	require.NoError(t, err)
}

func TestCheckRiderDamageHistory_AtThreshold_TriggerPenalty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	// 骑手最近7天有3次餐损，达到阈值
	store.EXPECT().ListRiderClaims(gomock.Any(), gomock.Any()).Return([]db.Claim{
		{ClaimType: ClaimTypeDamage},
		{ClaimType: ClaimTypeDamage},
		{ClaimType: ClaimTypeDamage},
	}, nil)

	// 记录餐损统计
	store.EXPECT().IncrementRiderDamageIncident(gomock.Any(), int64(1)).Return(nil)
	store.EXPECT().SuspendRider(gomock.Any(), gomock.Any()).Return(nil)

	err := caa.CheckRiderDamageHistory(context.Background(), 1)
	require.NoError(t, err)

	// 等待异步通知
	time.Sleep(100 * time.Millisecond)

	// 验证骑手收到通知
	require.Len(t, wsHub.riderMessages[1], 1)
}

func TestCheckRiderDamageHistory_MixedClaimTypes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	// 骑手有多种索赔类型，只有2次是餐损
	store.EXPECT().ListRiderClaims(gomock.Any(), gomock.Any()).Return([]db.Claim{
		{ClaimType: ClaimTypeDamage},
		{ClaimType: ClaimTypeTimeout}, // 超时不算餐损
		{ClaimType: ClaimTypeDamage},
		{ClaimType: ClaimTypeTimeout},
	}, nil)

	err := caa.CheckRiderDamageHistory(context.Background(), 1)
	require.NoError(t, err)
	// 只有2次餐损，不触发处罚
}

// ========================================
// CreateClaimWithDecision 测试
// ========================================

func TestCreateClaimWithDecision_InstantApproval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	decision := &Decision{
		Type:               ApprovalTypeInstant,
		Approved:           true,
		Amount:             5000,
		Reason:             "正常用户秒赔",
		BehaviorStatus:     ClaimBehaviorNormal,
		CompensationSource: CompensationSourceMerchant,
	}

	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).Return(db.CreateClaimWithBehaviorTxResult{
		Claim: db.Claim{
			ID:          1,
			OrderID:     100,
			ClaimType:   ClaimTypeForeignObject,
			ClaimAmount: 7000,
			Status:      ClaimStatusWaitingCustomerConfirmation,
		},
	}, nil)

	claim, err := caa.CreateClaimWithDecision(
		context.Background(),
		100, // orderID
		1,   // userID
		ClaimTypeForeignObject,
		"发现异物",
		7000,
		decision,
	)

	require.NoError(t, err)
	require.NotNil(t, claim)
	require.Equal(t, int64(1), claim.ID)
	require.Equal(t, ClaimStatusWaitingCustomerConfirmation, claim.Status)
	require.Equal(t, int64(7000), claim.ClaimAmount)
}

func TestCreateClaimWithDecision_SeparatesRequestedAndApprovedAmounts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	decision := &Decision{
		Type:               ApprovalTypeInstant,
		Approved:           true,
		Amount:             5000,
		Reason:             "正常用户秒赔",
		BehaviorStatus:     ClaimBehaviorNormal,
		CompensationSource: CompensationSourceMerchant,
	}

	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, params db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, int64(7000), params.ClaimAmount)
			require.NotNil(t, params.ApprovedAmount)
			require.Equal(t, int64(5000), *params.ApprovedAmount)
			require.True(t, params.SkipActionCreation)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: db.Claim{ID: 1, ClaimAmount: params.ClaimAmount, Status: params.Status},
			}, nil
		})

	claim, err := caa.CreateClaimWithDecision(
		context.Background(),
		100,
		1,
		ClaimTypeForeignObject,
		"发现异物",
		7000,
		decision,
	)

	require.NoError(t, err)
	require.NotNil(t, claim)
	require.Equal(t, int64(7000), claim.ClaimAmount)
}

func TestCreateClaimWithDecisionAndEvidence_PassesRecoveryPlanIntoTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	decision := &Decision{
		Type:               ApprovalTypeInstant,
		Approved:           true,
		Amount:             5000,
		Reason:             "正常用户秒赔",
		BehaviorStatus:     ClaimBehaviorNormal,
		CompensationSource: CompensationSourceMerchant,
	}

	dueAt := time.Now().Add(24 * time.Hour)
	recoveryPlan := &ClaimRecoveryPlan{
		ResponsibleParty: "merchant",
		RecoveryTarget:   "merchant",
		RecoveryAmount:   5000,
		DueAt:            dueAt,
		DecisionSnapshot: []byte(`{"decision_type":"instant"}`),
	}

	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, params db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.True(t, params.CreateRecovery)
			require.True(t, params.SkipActionCreation)
			require.Equal(t, recoveryPlan.RecoveryTarget, params.RecoveryTarget)
			require.Equal(t, recoveryPlan.RecoveryAmount, params.RecoveryAmount)
			require.NotNil(t, params.RecoveryDueAt)
			require.WithinDuration(t, recoveryPlan.DueAt, *params.RecoveryDueAt, time.Second)
			require.Equal(t, recoveryPlan.DecisionSnapshot, params.DecisionSnapshot)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: db.Claim{ID: 1, OrderID: params.OrderID, ClaimAmount: params.ClaimAmount, Status: params.Status},
			}, nil
		})

	claim, err := caa.CreateClaimWithDecisionAndEvidence(
		context.Background(),
		1,
		1,
		ClaimTypeForeignObject,
		"发现异物",
		7000,
		decision,
		nil,
		recoveryPlan,
	)

	require.NoError(t, err)
	require.NotNil(t, claim)
	require.Equal(t, int64(7000), claim.ClaimAmount)
}

func TestCreateClaimWithDecisionAndEvidence_AlignsUserRestrictedFromPersistedDecision(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)
	decision := &Decision{
		Type:               DecisionModeMerchantRecovery,
		Approved:           true,
		Amount:             2800,
		Reason:             "商户责任候选",
		CompensationSource: CompensationSourceMerchant,
	}

	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, params db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, "merchant", params.ResponsibleParty)
			require.True(t, params.SkipActionCreation)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: db.Claim{
					ID:             21,
					OrderID:        params.OrderID,
					ClaimAmount:    params.ClaimAmount,
					Status:         params.Status,
					ApprovalType:   pgtype.Text{String: "auto", Valid: true},
					ApprovedAmount: pgtype.Int8{Int64: 2800, Valid: true},
				},
				BehaviorDecision: db.BehaviorDecision{
					DecisionMode:       pgtype.Text{String: db.BehaviorDecisionModeUserRestricted, Valid: true},
					RestrictionReason:  pgtype.Text{String: "confirmed_high_user_risk", Valid: true},
					TraceSummary:       pgtype.Text{String: "您的账号因索赔行为异常已被限制服务；若确认继续索赔，平台将先行赔付并停止后续服务。", Valid: true},
					CompensationSource: "platform",
				},
			}, nil
		})

	claim, err := caa.CreateClaimWithDecisionAndEvidence(
		context.Background(),
		201,
		1,
		ClaimTypeDamage,
		"异常索赔",
		2800,
		decision,
		nil,
		nil,
	)

	require.NoError(t, err)
	require.NotNil(t, claim)
	require.Equal(t, int64(21), claim.ID)
	require.Equal(t, DecisionModeUserRestricted, decision.Type)
	require.Equal(t, ClaimBehaviorUserRestricted, decision.BehaviorStatus)
	require.Equal(t, CompensationSourcePlatform, decision.CompensationSource)
	require.Equal(t, "您的账号因索赔行为异常已被限制服务；若确认继续索赔，平台将先行赔付并停止后续服务。", decision.Reason)
	require.Equal(t, decision.Reason, decision.Warning)
	require.Empty(t, wsHub.riderMessages)
	require.Empty(t, wsHub.merchantMessages)
}

func TestSendNotification_IgnoresTypedNilWebSocketHub(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	var nilHub *MockWebSocketHub
	caa := NewClaimAutoApproval(store, nilHub)

	require.NotPanics(t, func() {
		caa.sendNotification("merchant", "标题", "内容", 1)
	})
}

func TestCreateClaimWithDecision_RiderDeposit_DeductAndRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	decision := &Decision{
		Type:               ApprovalTypeInstant,
		Approved:           true,
		Amount:             5000,
		Reason:             "正常用户秒赔",
		BehaviorStatus:     ClaimBehaviorNormal,
		CompensationSource: CompensationSourceRider, // 骑手押金
	}

	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).Return(db.CreateClaimWithBehaviorTxResult{
		Claim: db.Claim{
			ID:          1,
			OrderID:     100,
			ClaimType:   ClaimTypeDamage,
			ClaimAmount: 5000,
			Status:      ClaimStatusWaitingCustomerConfirmation,
		},
	}, nil)

	claim, err := caa.CreateClaimWithDecision(
		context.Background(),
		100, // orderID
		1,   // userID
		ClaimTypeDamage,
		"餐损",
		5000,
		decision,
	)

	require.NoError(t, err)
	require.NotNil(t, claim)
}

// ========================================
// 边界情况测试
// ========================================

func TestSendNotification_NilHub(t *testing.T) {
	caa := &ClaimAutoApproval{
		wsHub: nil,
	}

	// 不应该 panic
	caa.sendNotification("rider", "Test", "Test message", 1)
}
