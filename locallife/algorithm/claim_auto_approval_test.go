package algorithm

import (
	"context"
	"encoding/json"
	"errors"
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

	// 模拟正常用户行为
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), int64(1)).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 20,
		Claims90d:        0,
		WarningCount:     0,
		RequiresEvidence: false,
		PlatformPayCount: 0,
	}, nil)

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
}

func TestEvaluateClaim_Damage_RiderPays(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	// 模拟正常用户行为
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), int64(1)).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 20,
		Claims90d:        0,
		WarningCount:     0,
	}, nil)

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
}

func TestEvaluateClaim_ForeignObject_MerchantPays(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	// 模拟正常用户行为
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), int64(1)).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 20,
		Claims90d:        0,
		WarningCount:     0,
	}, nil)

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
}

func TestEvaluateClaim_FirstWarning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	// 模拟触发首次警告条件：5单3索赔（当前是第3次）
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), int64(1)).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 5,
		Claims90d:        2, // 已有2次，这是第3次
		WarningCount:     0, // 之前没被警告过
		RequiresEvidence: false,
		PlatformPayCount: 0,
	}, nil)

	// 警告会记录
	store.EXPECT().GetUserClaimWarningStatus(gomock.Any(), int64(1)).Return(db.UserClaimWarning{}, errors.New("not found"))
	store.EXPECT().CreateUserClaimWarning(gomock.Any(), gomock.Any()).Return(db.UserClaimWarning{}, nil)

	decision, err := caa.EvaluateClaim(
		context.Background(),
		1,   // userID
		100, // orderID
		testCompensationContext(5000, 5000, 300),
		ClaimTypeDamage,
	)

	require.NoError(t, err)
	require.NotNil(t, decision)
	require.Equal(t, ApprovalTypeInstant, decision.Type) // 首次警告仍然秒赔
	require.True(t, decision.Approved)
	require.True(t, decision.ShouldWarn)
	require.Equal(t, ClaimBehaviorWarned, decision.BehaviorStatus)
	require.Contains(t, decision.Warning, "行为回溯审计")

	// 等待异步记录完成
	time.Sleep(100 * time.Millisecond)
}

func TestEvaluateClaim_WarnedAfterWarning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	// 模拟已被警告的用户
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), int64(1)).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 10,
		Claims90d:        3,
		WarningCount:     1,
		RequiresEvidence: true,
		PlatformPayCount: 0,
	}, nil)

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
	require.Equal(t, ClaimBehaviorWarned, decision.BehaviorStatus)
	require.False(t, decision.ShouldWarn)
	require.Contains(t, decision.Warning, "行为回溯审计")
}

func TestEvaluateClaim_PlatformPay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	// 模拟问题用户：已有1次平台垫付
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), int64(1)).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 5,
		Claims90d:        4,
		WarningCount:     1, // 有警告
		RequiresEvidence: false,
		PlatformPayCount: 1, // 已有1次平台垫付（需要>=2才触发RejectService）
	}, nil)

	decision, err := caa.EvaluateClaim(
		context.Background(),
		1,   // userID
		100, // orderID
		testCompensationContext(5000, 5000, 300),
		ClaimTypeDamage,
	)

	require.NoError(t, err)
	require.NotNil(t, decision)
	require.Equal(t, DecisionModePlatformFallback, decision.Type)
	require.True(t, decision.Approved)
	require.Equal(t, CompensationSourcePlatform, decision.CompensationSource)
}

func TestEvaluateClaim_UserRestricted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	// 模拟高风险用户（平台兜底记录>=2次）
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), int64(1)).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 5,
		Claims90d:        5,
		WarningCount:     3,
		RequiresEvidence: false,
		PlatformPayCount: 2, // 已有2次平台兜底记录 -> user_restricted
	}, nil)

	decision, err := caa.EvaluateClaim(
		context.Background(),
		1,   // userID
		100, // orderID
		testCompensationContext(5000, 5000, 300),
		ClaimTypeDamage,
	)

	require.NoError(t, err)
	require.NotNil(t, decision)
	require.Equal(t, DecisionModeUserRestricted, decision.Type)
	require.True(t, decision.Approved) // 照赔
	require.Equal(t, CompensationSourcePlatform, decision.CompensationSource)
	require.Equal(t, ClaimBehaviorUserRestricted, decision.BehaviorStatus)
}

func TestEvaluateClaim_BehaviorCheckFailed_FallbackToInstant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	// 模拟行为检查失败
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), int64(1)).Return(db.GetUserBehaviorStatsRow{}, errors.New("db error"))

	decision, err := caa.EvaluateClaim(
		context.Background(),
		1,   // userID
		100, // orderID
		testCompensationContext(5000, 5000, 300),
		ClaimTypeDamage,
	)

	require.NoError(t, err)
	require.NotNil(t, decision)
	require.Equal(t, ApprovalTypeInstant, decision.Type) // 降级秒赔
	require.True(t, decision.Approved)
	require.Contains(t, decision.Reason, "降级秒赔")
}

func TestEvaluateClaim_CapsRequestedAmountByEligibleOrderAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	store.EXPECT().GetUserBehaviorStats(gomock.Any(), int64(1)).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 20,
		Claims90d:        0,
		WarningCount:     0,
	}, nil)

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
// CheckUserClaimBehavior 测试
// ========================================

func TestCheckUserClaimBehavior_Normal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	store.EXPECT().GetUserBehaviorStats(gomock.Any(), int64(1)).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 30,
		Claims90d:        1,
		WarningCount:     0,
		RequiresEvidence: false,
		PlatformPayCount: 0,
	}, nil)

	result, err := caa.CheckUserClaimBehavior(context.Background(), 1)

	require.NoError(t, err)
	require.Equal(t, ClaimBehaviorNormal, result.Status)
	require.Equal(t, 30, result.TakeoutOrders)
	require.Equal(t, 1, result.ClaimCount)
}

func TestCheckUserClaimBehavior_HighRatioWarning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	// 订单较多但索赔比例异常高（>60%且>=3次）
	// 注意：需要 TakeoutOrders > 5 才会进入高比例判断（条件5）
	// 否则会先触发条件4（5单3索赔）
	store.EXPECT().GetUserBehaviorStats(gomock.Any(), int64(1)).Return(db.GetUserBehaviorStatsRow{
		TakeoutOrders90d: 10, // >5, 不触发条件4
		Claims90d:        6,  // 7次，比例70% > 60%
		WarningCount:     0,
		RequiresEvidence: false,
		PlatformPayCount: 0,
	}, nil)

	result, err := caa.CheckUserClaimBehavior(context.Background(), 1)

	require.NoError(t, err)
	require.Equal(t, ClaimBehaviorWarned, result.Status)
	require.True(t, result.ShouldWarn)
	require.Contains(t, result.Message, "异常高")
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

	// 创建索赔记录（含行为追溯）
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).Return(db.CreateClaimWithBehaviorTxResult{
		Claim: db.Claim{
			ID:          1,
			OrderID:     100,
			ClaimType:   ClaimTypeForeignObject,
			ClaimAmount: 7000,
			Status:      ClaimStatusAutoApproved,
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
	require.Equal(t, ClaimStatusAutoApproved, claim.Status)
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

func TestCreateClaimWithDecisionAndEvidence_AlignsPlatformFallbackFromPersistedDecision(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	decision := &Decision{
		Type:               ApprovalTypeInstant,
		Approved:           true,
		Amount:             3600,
		Reason:             "骑手责任成立",
		BehaviorStatus:     ClaimBehaviorNormal,
		CompensationSource: CompensationSourceRider,
	}

	dueAt := time.Now().Add(24 * time.Hour)
	recoveryPlan := &ClaimRecoveryPlan{
		ResponsibleParty: "rider",
		RecoveryTarget:   "rider",
		RecoveryAmount:   3600,
		DueAt:            dueAt,
		DecisionSnapshot: []byte(`{"decision_type":"instant"}`),
	}

	store.EXPECT().IncrementUserPlatformPayCount(gomock.Any(), db.IncrementUserPlatformPayCountParams{
		UserID:            1,
		LastWarningReason: pgtype.Text{String: "当前订单缺少取餐确认等关键责任事实，本次不向服务方追责，已由平台兜底处理", Valid: true},
	}).Return(nil)

	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, params db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.True(t, params.CreateRecovery)
			require.Equal(t, "rider", params.RecoveryTarget)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: db.Claim{
					ID:             11,
					OrderID:        params.OrderID,
					ClaimAmount:    params.ClaimAmount,
					Status:         params.Status,
					ApprovalType:   pgtype.Text{String: "auto", Valid: true},
					ApprovedAmount: pgtype.Int8{Int64: 3600, Valid: true},
				},
				BehaviorDecision: db.BehaviorDecision{
					DecisionMode:       pgtype.Text{String: db.BehaviorDecisionModePlatformFallback, Valid: true},
					FallbackReason:     pgtype.Text{String: "missing_pickup_confirmation", Valid: true},
					TraceSummary:       pgtype.Text{String: "当前订单缺少取餐确认等关键责任事实，本次不向服务方追责，已由平台兜底处理", Valid: true},
					CompensationSource: "platform",
				},
			}, nil
		})

	claim, err := caa.CreateClaimWithDecisionAndEvidence(
		context.Background(),
		101,
		1,
		ClaimTypeDamage,
		"骑手配送损坏",
		3600,
		decision,
		nil,
		recoveryPlan,
	)

	require.NoError(t, err)
	require.NotNil(t, claim)
	require.Equal(t, int64(11), claim.ID)
	require.Equal(t, DecisionModePlatformFallback, decision.Type)
	require.Equal(t, CompensationSourcePlatform, decision.CompensationSource)
	require.Equal(t, "当前订单缺少取餐确认等关键责任事实，本次不向服务方追责，已由平台兜底处理", decision.Reason)
	require.Equal(t, decision.Reason, decision.Warning)
	require.Empty(t, wsHub.riderMessages)
	require.Empty(t, wsHub.merchantMessages)
}

func TestCreateClaimWithDecisionAndEvidence_AlignsUserRestrictedFromPersistedDecision(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)
	store.EXPECT().
		GetActiveBehaviorBlocklist(gomock.Any(), db.GetActiveBehaviorBlocklistParams{
			EntityType: "user",
			EntityID:   int64(1),
		}).
		Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: "behavior_trace.reject_service_cooldown_days",
			ScopeType: "global",
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateBehaviorBlocklist(gomock.Any(), gomock.Any()).
		Return(db.BehaviorBlocklist{}, nil)

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
					TraceSummary:       pgtype.Text{String: "您的账号因索赔行为异常已被限制服务，本次索赔由平台兜底处理。", Valid: true},
					CompensationSource: "platform",
				},
				RestrictionAction: &db.BehaviorAction{
					ID:           22,
					DecisionID:   21,
					ActionType:   "block",
					TargetEntity: "user",
					Status:       "created",
					Detail:       []byte(`{"action":"apply_user_restriction","claim_id":21,"user_id":1,"decision_mode":"user_restricted","restriction_reason":"confirmed_high_user_risk","remark":"user restricted action created"}`),
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
	require.Equal(t, "您的账号因索赔行为异常已被限制服务，本次索赔由平台兜底处理。", decision.Reason)
	require.Equal(t, decision.Reason, decision.Warning)
	require.Empty(t, wsHub.riderMessages)
	require.Empty(t, wsHub.merchantMessages)
}

func TestCreateClaimWithDecisionAndEvidence_UserRestrictionLookupFailureIsNonBlocking(t *testing.T) {
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

	store.EXPECT().
		GetActiveBehaviorBlocklist(gomock.Any(), db.GetActiveBehaviorBlocklistParams{
			EntityType: "user",
			EntityID:   int64(1),
		}).
		Return(db.BehaviorBlocklist{}, errors.New("blocklist lookup failed"))

	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).Return(db.CreateClaimWithBehaviorTxResult{
		Claim: db.Claim{
			ID:             31,
			OrderID:        301,
			ClaimAmount:    2800,
			Status:         ClaimStatusAutoApproved,
			ApprovalType:   pgtype.Text{String: "auto", Valid: true},
			ApprovedAmount: pgtype.Int8{Int64: 2800, Valid: true},
		},
		BehaviorDecision: db.BehaviorDecision{
			DecisionMode:       pgtype.Text{String: db.BehaviorDecisionModeUserRestricted, Valid: true},
			RestrictionReason:  pgtype.Text{String: "confirmed_high_user_risk", Valid: true},
			TraceSummary:       pgtype.Text{String: "您的账号因索赔行为异常已被限制服务，本次索赔由平台兜底处理。", Valid: true},
			CompensationSource: "platform",
		},
		RestrictionAction: &db.BehaviorAction{
			ID:           32,
			DecisionID:   31,
			ActionType:   "block",
			TargetEntity: "user",
			Status:       "created",
			Detail:       []byte(`{"action":"apply_user_restriction","claim_id":31,"user_id":1,"decision_mode":"user_restricted","restriction_reason":"confirmed_high_user_risk","remark":"user restricted action created"}`),
		},
	}, nil)

	claim, err := caa.CreateClaimWithDecisionAndEvidence(
		context.Background(),
		301,
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
	require.Equal(t, int64(31), claim.ID)
	require.Equal(t, DecisionModeUserRestricted, decision.Type)
	require.Equal(t, ClaimBehaviorUserRestricted, decision.BehaviorStatus)
	require.Equal(t, CompensationSourcePlatform, decision.CompensationSource)
	require.Empty(t, wsHub.riderMessages)
	require.Empty(t, wsHub.merchantMessages)
}

func TestExecuteNotificationAction_RiderWebSocketFallbackWhenDistributorFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	notifier := &MockNotificationDistributor{err: errors.New("notification queue unavailable")}
	caa := NewClaimAutoApproval(store, wsHub)
	caa.SetNotificationDistributor(notifier)

	action := db.BehaviorAction{
		ID:           41,
		DecisionID:   40,
		ActionType:   "notify",
		TargetEntity: "rider",
		Status:       "created",
		Detail:       []byte(`{"action":"notify_responsible_party","claim_id":40,"target_entity":"rider","target_id":18,"recipient_user_id":118,"notification_type":"system","title":"异常订单判责通知","content":"订单ORD-101的餐损异常索赔已判定由您承担。平台已向用户先行赔付42.00元，并已生成42.00元追偿单，请尽快处理。 判责依据：餐损责任归属骑手。","related_type":"claim","related_id":40,"remark":"notification action created"}`),
	}

	require.NotPanics(t, func() {
		caa.executeNotificationAction(context.Background(), action)
	})
	require.Len(t, notifier.notifications, 1)
	require.Equal(t, int64(118), notifier.notifications[0].UserID)
	require.Len(t, wsHub.riderMessages[18], 1)
	var notificationData map[string]interface{}
	require.NoError(t, json.Unmarshal(wsHub.riderMessages[18][0].Data, &notificationData))
	require.Contains(t, notificationData["message"], "已判定由您承担")
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

	// 创建索赔记录（含行为追溯）
	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).Return(db.CreateClaimWithBehaviorTxResult{
		Claim: db.Claim{
			ID:          1,
			OrderID:     100,
			ClaimType:   ClaimTypeDamage,
			ClaimAmount: 5000,
			Status:      ClaimStatusAutoApproved,
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

func TestCreateClaimWithDecision_PlatformFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wsHub := NewMockWebSocketHub()
	caa := NewClaimAutoApproval(store, wsHub)

	decision := &Decision{
		Type:               DecisionModePlatformFallback,
		Approved:           true,
		Amount:             5000,
		Reason:             "高风险索赔，平台兜底",
		CompensationSource: CompensationSourcePlatform,
	}

	store.EXPECT().CreateClaimWithBehaviorTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, params db.CreateClaimWithBehaviorTxParams) (db.CreateClaimWithBehaviorTxResult, error) {
			require.Equal(t, ClaimStatusAutoApproved, params.Status)
			return db.CreateClaimWithBehaviorTxResult{
				Claim: db.Claim{
					ID:     1,
					Status: params.Status,
				},
			}, nil
		})

	claim, err := caa.CreateClaimWithDecision(
		context.Background(),
		100,
		1,
		ClaimTypeDamage,
		"餐损",
		5000,
		decision,
	)

	require.NoError(t, err)
	require.NotNil(t, claim)
	require.Equal(t, ClaimStatusAutoApproved, claim.Status)
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
