package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPickBestRulesByStackingGroup(t *testing.T) {
	rules := []db.DiscountRule{
		{Name: "A1", DiscountAmount: 100, StackingGroup: pgtype.Text{String: "group_a", Valid: true}},
		{Name: "A2", DiscountAmount: 200, StackingGroup: pgtype.Text{String: "group_a", Valid: true}},
		{Name: "B1", DiscountAmount: 150, StackingGroup: pgtype.Text{String: "group_b", Valid: true}},
		{Name: "Default", DiscountAmount: 80},
	}

	selected := pickBestRulesByStackingGroup(rules)
	require.Len(t, selected, 3)

	picked := make(map[string]int64)
	for _, rule := range selected {
		picked[rule.Name] = rule.DiscountAmount
	}
	_, hasA1 := picked["A1"]
	_, hasA2 := picked["A2"]
	require.False(t, hasA1)
	require.True(t, hasA2)
	require.True(t, picked["Default"] > 0)
}

func TestPickBestRulesByStackingGroup_ExclusiveWins(t *testing.T) {
	rules := []db.DiscountRule{
		{Name: "A", DiscountAmount: 120, StackingGroup: pgtype.Text{String: "group_a", Valid: true}},
		{Name: "B", DiscountAmount: 100, StackingGroup: pgtype.Text{String: "group_b", Valid: true}},
		{Name: "EX", DiscountAmount: 90, StackingGroup: pgtype.Text{String: "exclusive", Valid: true}},
		{Name: "EX2", DiscountAmount: 180, StackingGroup: pgtype.Text{String: "exclusive", Valid: true}},
	}

	selected := pickBestRulesByStackingGroup(rules)
	require.Len(t, selected, 1)
	require.Equal(t, "EX2", selected[0].Name)
}

func TestIsRuleMatch(t *testing.T) {
	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	rule := db.DiscountRule{
		MinOrderAmount: 1000,
		ValidFrom:      now.Add(-time.Hour),
		ValidUntil:     now.Add(time.Hour),
	}

	require.True(t, isRuleMatch(rule, OrderContext{Subtotal: 1500}, now))
	require.False(t, isRuleMatch(rule, OrderContext{Subtotal: 500}, now))
	require.False(t, isRuleMatch(rule, OrderContext{Subtotal: 1500}, now.Add(2*time.Hour)))
}

func TestIsVoucherValid(t *testing.T) {
	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	voucher := db.GetUserVoucherRow{
		Status:            "unused",
		ExpiresAt:         now.Add(time.Hour),
		MerchantID:        10,
		MinOrderAmount:    1000,
		AllowedOrderTypes: []string{"takeaway"},
	}

	require.True(t, isVoucherValid(voucher, OrderContext{MerchantID: 10, OrderType: "takeaway", Subtotal: 1500}, now))
	require.False(t, isVoucherValid(voucher, OrderContext{MerchantID: 10, OrderType: "dine_in", Subtotal: 1500}, now))
	require.False(t, isVoucherValid(voucher, OrderContext{MerchantID: 10, OrderType: "takeaway", Subtotal: 500}, now))

	voucher.VoucherTemplateBlockReason = "inactive"
	require.False(t, isVoucherValid(voucher, OrderContext{MerchantID: 10, OrderType: "takeaway", Subtotal: 1500}, now))
}

func TestSuggestBestVoucher(t *testing.T) {
	userID := int64(1)
	merchantID := int64(2)
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserAvailableVouchersForMerchant(gomock.Any(), db.ListUserAvailableVouchersForMerchantParams{
			UserID:         userID,
			MerchantID:     merchantID,
			MinOrderAmount: 2000,
		}).
		Times(1).
		Return([]db.ListUserAvailableVouchersForMerchantRow{
			{ID: 10, Name: "Voucher A", Amount: 300, AllowedOrderTypes: []string{"takeaway"}},
			{ID: 20, Name: "Voucher B", Amount: 500, AllowedOrderTypes: []string{}},
		}, nil)

	result, err := suggestBestVoucher(ctx, store, OrderContext{UserID: userID, MerchantID: merchantID, OrderType: "takeaway", Subtotal: 2000})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(10), result.ID)
}

func TestCuratePaymentBalance(t *testing.T) {
	res := &PriceCalculationResult{TotalAmount: 1000, DeliveryFee: 200}
	assessment := &PaymentAssessment{PrincipalPart: 100, BonusPart: 1200}

	curatePaymentBalance(OrderContext{OrderType: "takeout"}, res, assessment)
	require.Equal(t, int64(800), assessment.BonusPart)
	require.Equal(t, "支付提示：您的赠送金额暂不可抵扣代取费", assessment.PaymentHint)

	res = &PriceCalculationResult{TotalAmount: 500}
	assessment = &PaymentAssessment{PrincipalPart: 100, BonusPart: 200}
	curatePaymentBalance(OrderContext{OrderType: "reservation"}, res, assessment)
	require.Equal(t, int64(0), assessment.BonusPart)
	require.Equal(t, "说明：包间预定定金需使用本金支付，暂不支持赠额抵扣", assessment.PaymentHint)

	res = &PriceCalculationResult{TotalAmount: 800}
	assessment = &PaymentAssessment{PrincipalPart: 200, BonusPart: 0}
	curatePaymentBalance(OrderContext{OrderType: "dine_in"}, res, assessment)
	require.False(t, assessment.IsBalancePayable)
	require.Equal(t, "可用余额不足支付本单", assessment.PaymentHint)
}

func TestCalculateFinalPrice_Minimal(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), int64(10)).
		Times(1).
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: 10, UserID: 20}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)
	store.EXPECT().
		ListUserAvailableVouchersForMerchant(gomock.Any(), db.ListUserAvailableVouchersForMerchantParams{
			UserID:         20,
			MerchantID:     10,
			MinOrderAmount: 1000,
		}).
		Times(1).
		Return([]db.ListUserAvailableVouchersForMerchantRow{}, nil)

	engine := NewPromotionEngine(store)
	result, err := engine.CalculateFinalPrice(ctx, OrderContext{MerchantID: 10, UserID: 20, OrderType: "takeaway", Subtotal: 1000})
	require.NoError(t, err)
	require.Equal(t, int64(1000), result.TotalAmount)
}

func TestCalculateFinalPrice_MembershipSplitWithCap(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), int64(10)).
		Times(1).
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: 10, UserID: 20}).
		Times(1).
		Return(db.MerchantMembership{PrincipalBalance: 800, BonusBalance: 400}, nil)
	store.EXPECT().
		GetMerchantMembershipSettings(gomock.Any(), int64(10)).
		Times(1).
		Return(db.MerchantMembershipSetting{
			MerchantID:          10,
			BalanceUsableScenes: []string{"takeaway"},
			BonusUsableScenes:   []string{"takeaway"},
			AllowWithVoucher:    true,
			AllowWithDiscount:   true,
			MaxDeductionPercent: 50,
		}, nil)
	store.EXPECT().
		ListUserAvailableVouchersForMerchant(gomock.Any(), db.ListUserAvailableVouchersForMerchantParams{
			UserID:         20,
			MerchantID:     10,
			MinOrderAmount: 2000,
		}).
		Times(1).
		Return([]db.ListUserAvailableVouchersForMerchantRow{}, nil)

	engine := NewPromotionEngine(store)
	result, err := engine.CalculateFinalPrice(ctx, OrderContext{MerchantID: 10, UserID: 20, OrderType: "takeaway", Subtotal: 2000})
	require.NoError(t, err)
	require.Equal(t, int64(600), result.PaymentAssessment.PrincipalPart)
	require.Equal(t, int64(400), result.PaymentAssessment.BonusPart)
}

func TestCalculateFinalPrice_MembershipBonusSceneBlocked(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), int64(10)).
		Times(1).
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: 10, UserID: 20}).
		Times(1).
		Return(db.MerchantMembership{PrincipalBalance: 500, BonusBalance: 500}, nil)
	store.EXPECT().
		GetMerchantMembershipSettings(gomock.Any(), int64(10)).
		Times(1).
		Return(db.MerchantMembershipSetting{
			MerchantID:          10,
			BalanceUsableScenes: []string{"takeaway"},
			BonusUsableScenes:   []string{"dine_in"},
			AllowWithVoucher:    true,
			AllowWithDiscount:   true,
			MaxDeductionPercent: 100,
		}, nil)
	store.EXPECT().
		ListUserAvailableVouchersForMerchant(gomock.Any(), db.ListUserAvailableVouchersForMerchantParams{
			UserID:         20,
			MerchantID:     10,
			MinOrderAmount: 1000,
		}).
		Times(1).
		Return([]db.ListUserAvailableVouchersForMerchantRow{}, nil)

	engine := NewPromotionEngine(store)
	result, err := engine.CalculateFinalPrice(ctx, OrderContext{MerchantID: 10, UserID: 20, OrderType: "takeaway", Subtotal: 1000})
	require.NoError(t, err)
	require.Equal(t, int64(500), result.PaymentAssessment.PrincipalPart)
	require.Equal(t, int64(0), result.PaymentAssessment.BonusPart)
}

func TestCalculateFinalPrice_MembershipBlockedByPromotionRule(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), int64(10)).
		Times(1).
		Return([]db.DiscountRule{
			{
				ID:                     1,
				Name:                   "会员互斥满减",
				MinOrderAmount:         1000,
				DiscountAmount:         200,
				ValidFrom:              now.Add(-time.Hour),
				ValidUntil:             now.Add(time.Hour),
				CanStackWithVoucher:    true,
				CanStackWithMembership: false,
				StackingGroup:          pgtype.Text{String: "group_a", Valid: true},
			},
		}, nil)
	store.EXPECT().
		ListUserAvailableVouchersForMerchant(gomock.Any(), db.ListUserAvailableVouchersForMerchantParams{
			UserID:         20,
			MerchantID:     10,
			MinOrderAmount: 1500,
		}).
		Times(1).
		Return([]db.ListUserAvailableVouchersForMerchantRow{}, nil)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: 10, UserID: 20}).
		Times(1).
		Return(db.MerchantMembership{PrincipalBalance: 500, BonusBalance: 300}, nil)
	store.EXPECT().
		GetMerchantMembershipSettings(gomock.Any(), int64(10)).
		Times(1).
		Return(db.MerchantMembershipSetting{
			MerchantID:          10,
			BalanceUsableScenes: []string{"takeaway"},
			BonusUsableScenes:   []string{"takeaway"},
			AllowWithVoucher:    true,
			AllowWithDiscount:   true,
			MaxDeductionPercent: 100,
		}, nil)

	engine := NewPromotionEngine(store)
	engine.now = func() time.Time { return now }

	result, err := engine.CalculateFinalPrice(ctx, OrderContext{MerchantID: 10, UserID: 20, OrderType: "takeaway", Subtotal: 1500})
	require.NoError(t, err)
	require.Equal(t, int64(200), result.MerchantDiscount)
	require.Equal(t, int64(0), result.PaymentAssessment.PrincipalPart)
	require.Equal(t, int64(0), result.PaymentAssessment.BonusPart)
	require.Equal(t, "会员余额不可与当前营销优惠叠加", result.PaymentAssessment.PaymentHint)
}

func TestCalculateFinalPrice_SelectsBestMatchingRuleWithinDefaultGroup(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), int64(10)).
		Times(1).
		Return([]db.DiscountRule{
			{Name: "满20减2", MinOrderAmount: 2000, DiscountAmount: 200, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)},
			{Name: "满50减8", MinOrderAmount: 5000, DiscountAmount: 800, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)},
		}, nil)
	store.EXPECT().
		ListUserAvailableVouchersForMerchant(gomock.Any(), db.ListUserAvailableVouchersForMerchantParams{
			UserID:         20,
			MerchantID:     10,
			MinOrderAmount: 3000,
		}).
		Times(1).
		Return([]db.ListUserAvailableVouchersForMerchantRow{}, nil)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: 10, UserID: 20}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	engine := NewPromotionEngine(store)
	engine.now = func() time.Time { return now }

	result, err := engine.CalculateFinalPrice(ctx, OrderContext{MerchantID: 10, UserID: 20, OrderType: "takeout", Subtotal: 3000})
	require.NoError(t, err)
	require.Equal(t, int64(200), result.MerchantDiscount)
	require.Equal(t, int64(2800), result.TotalAmount)
}
