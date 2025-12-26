package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ==================== JoinMembershipTx Tests ====================

func TestJoinMembershipTx(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	arg := JoinMembershipTxParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	}

	result, err := testStore.JoinMembershipTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证会员卡创建成功
	membership := result.Membership
	require.NotZero(t, membership.ID)
	require.Equal(t, user.ID, membership.UserID)
	require.Equal(t, merchant.ID, membership.MerchantID)
	require.Equal(t, int64(0), membership.Balance)
	require.Equal(t, int64(0), membership.TotalRecharged)
	require.Equal(t, int64(0), membership.TotalConsumed)

	// 验证数据库中的会员卡
	dbMembership, err := testStore.GetMerchantMembership(context.Background(), membership.ID)
	require.NoError(t, err)
	require.Equal(t, membership.ID, dbMembership.ID)
	require.Equal(t, membership.UserID, dbMembership.UserID)
}

// 测试幂等性：多次加入同一商户会员应返回相同结果
func TestJoinMembershipTxIdempotent(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	arg := JoinMembershipTxParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	}

	// 第一次加入
	result1, err := testStore.JoinMembershipTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result1)

	// 第二次加入（应返回相同会员卡）
	result2, err := testStore.JoinMembershipTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result2)

	// 验证返回的是同一张会员卡
	require.Equal(t, result1.Membership.ID, result2.Membership.ID)
	require.Equal(t, result1.Membership.Balance, result2.Membership.Balance)
}

// ==================== RechargeTx Tests ====================

func TestRechargeTx(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 先创建会员卡
	membership, err := testStore.CreateMerchantMembership(context.Background(), CreateMerchantMembershipParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)

	// 创建充值规则（充100送10）
	rule, err := testStore.CreateRechargeRule(context.Background(), CreateRechargeRuleParams{
		MerchantID:     merchant.ID,
		RechargeAmount: 10000,
		BonusAmount:    1000,
		IsActive:       true,
		ValidFrom:      time.Now(),
		ValidUntil:     time.Now().Add(30 * 24 * time.Hour),
	})
	require.NoError(t, err)

	// 执行充值
	ruleID := rule.ID
	arg := RechargeTxParams{
		MembershipID:   membership.ID,
		RechargeAmount: 10000, // 100元
		BonusAmount:    1000,  // 赠送10元
		RechargeRuleID: &ruleID,
		Notes:          "充值测试",
	}

	result, err := testStore.RechargeTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证余额更新正确 (100 + 10 = 110)
	require.Equal(t, int64(11000), result.Membership.Balance)
	require.Equal(t, int64(11000), result.Membership.TotalRecharged)
	require.Equal(t, int64(11000), result.Transaction.BalanceAfter)
	require.Equal(t, "recharge", result.Transaction.Type)
	require.Equal(t, int64(11000), result.Transaction.Amount) // 充值金额+赠送金额

	// 验证数据库中的余额
	dbMembership, err := testStore.GetMerchantMembership(context.Background(), membership.ID)
	require.NoError(t, err)
	require.Equal(t, int64(11000), dbMembership.Balance)
}

// 测试多次充值累加
func TestRechargeTxMultipleTimes(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	membership, err := testStore.CreateMerchantMembership(context.Background(), CreateMerchantMembershipParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)

	// 第一次充值 100 元
	_, err = testStore.RechargeTx(context.Background(), RechargeTxParams{
		MembershipID:   membership.ID,
		RechargeAmount: 10000,
		BonusAmount:    0,
		Notes:          "第一次充值",
	})
	require.NoError(t, err)

	// 第二次充值 200 元 + 赠送 20 元
	result2, err := testStore.RechargeTx(context.Background(), RechargeTxParams{
		MembershipID:   membership.ID,
		RechargeAmount: 20000,
		BonusAmount:    2000,
		Notes:          "第二次充值",
	})
	require.NoError(t, err)

	// 验证余额正确累加：100 + 200 + 20 = 320
	require.Equal(t, int64(32000), result2.Membership.Balance)
	require.Equal(t, int64(32000), result2.Membership.TotalRecharged)
}

// ==================== ConsumeTx Tests ====================

func TestConsumeTx(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 创建会员卡并充值
	membership, err := testStore.CreateMerchantMembership(context.Background(), CreateMerchantMembershipParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)

	// 先充值 100 元
	_, err = testStore.RechargeTx(context.Background(), RechargeTxParams{
		MembershipID:   membership.ID,
		RechargeAmount: 10000,
		BonusAmount:    0,
		Notes:          "初始充值",
	})
	require.NoError(t, err)

	// 执行消费 30 元
	arg := ConsumeTxParams{
		MembershipID:   membership.ID,
		Amount:         3000,
		RelatedOrderID: 12345,
		Notes:          "消费测试",
	}

	result, err := testStore.ConsumeTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证余额正确扣减 (100 - 30 = 70)
	require.Equal(t, int64(7000), result.Membership.Balance)
	require.Equal(t, int64(3000), result.Membership.TotalConsumed)
	require.Equal(t, "consume", result.Transaction.Type)
	require.Equal(t, int64(-3000), result.Transaction.Amount) // 负数表示扣减
	require.Equal(t, int64(7000), result.Transaction.BalanceAfter)
}

// 测试余额不足的情况
func TestConsumeTxInsufficientBalance(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 创建会员卡并充值少量金额
	membership, err := testStore.CreateMerchantMembership(context.Background(), CreateMerchantMembershipParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)

	// 充值 10 元
	_, err = testStore.RechargeTx(context.Background(), RechargeTxParams{
		MembershipID:   membership.ID,
		RechargeAmount: 1000,
		BonusAmount:    0,
		Notes:          "少量充值",
	})
	require.NoError(t, err)

	// 尝试消费 30 元（余额不足）
	arg := ConsumeTxParams{
		MembershipID:   membership.ID,
		Amount:         3000,
		RelatedOrderID: 99999,
		Notes:          "余额不足测试",
	}

	result, err := testStore.ConsumeTx(context.Background(), arg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient balance")
	require.Empty(t, result)

	// 验证余额未被扣减
	dbMembership, err := testStore.GetMerchantMembership(context.Background(), membership.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), dbMembership.Balance)
}

// 测试刚好消费完余额
func TestConsumeTxExactBalance(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	membership, err := testStore.CreateMerchantMembership(context.Background(), CreateMerchantMembershipParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)

	// 充值 50 元
	_, err = testStore.RechargeTx(context.Background(), RechargeTxParams{
		MembershipID:   membership.ID,
		RechargeAmount: 5000,
		BonusAmount:    0,
		Notes:          "充值50元",
	})
	require.NoError(t, err)

	// 消费 50 元（刚好用完）
	arg := ConsumeTxParams{
		MembershipID:   membership.ID,
		Amount:         5000,
		RelatedOrderID: 88888,
		Notes:          "刚好用完余额",
	}

	result, err := testStore.ConsumeTx(context.Background(), arg)
	require.NoError(t, err)
	require.Equal(t, int64(0), result.Membership.Balance)
}

// ==================== RefundTx Tests ====================

func TestRefundTx(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 创建会员卡并充值
	membership, err := testStore.CreateMerchantMembership(context.Background(), CreateMerchantMembershipParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)

	// 充值 50 元
	_, err = testStore.RechargeTx(context.Background(), RechargeTxParams{
		MembershipID:   membership.ID,
		RechargeAmount: 5000,
		BonusAmount:    0,
		Notes:          "初始充值",
	})
	require.NoError(t, err)

	// 执行退款 20 元
	arg := RefundTxParams{
		MembershipID:   membership.ID,
		Amount:         2000,
		RelatedOrderID: 77777,
		Notes:          "退款测试",
	}

	result, err := testStore.RefundTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证余额正确增加 (50 + 20 = 70)
	require.Equal(t, int64(7000), result.Membership.Balance)
	require.Equal(t, "refund", result.Transaction.Type)
	require.Equal(t, int64(2000), result.Transaction.Amount)
	require.Equal(t, int64(7000), result.Transaction.BalanceAfter)
}

// ==================== Concurrent Tests ====================

// 测试并发充值（验证行锁）
func TestRechargeTxConcurrent(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	membership, err := testStore.CreateMerchantMembership(context.Background(), CreateMerchantMembershipParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)

	// 并发执行 10 次充值，每次 100 元
	n := 10
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		go func() {
			_, err := testStore.RechargeTx(context.Background(), RechargeTxParams{
				MembershipID:   membership.ID,
				RechargeAmount: 10000,
				BonusAmount:    0,
				Notes:          "并发充值",
			})
			errs <- err
		}()
	}

	// 检查所有充值是否成功
	for i := 0; i < n; i++ {
		err := <-errs
		require.NoError(t, err)
	}

	// 验证最终余额正确：10 * 100 = 1000 元
	dbMembership, err := testStore.GetMerchantMembership(context.Background(), membership.ID)
	require.NoError(t, err)
	require.Equal(t, int64(100000), dbMembership.Balance)
	require.Equal(t, int64(100000), dbMembership.TotalRecharged)
}

// 测试并发消费（验证余额扣减的正确性）
func TestConsumeTxConcurrent(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	membership, err := testStore.CreateMerchantMembership(context.Background(), CreateMerchantMembershipParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)

	// 先充值 1000 元
	_, err = testStore.RechargeTx(context.Background(), RechargeTxParams{
		MembershipID:   membership.ID,
		RechargeAmount: 100000,
		BonusAmount:    0,
		Notes:          "大额充值",
	})
	require.NoError(t, err)

	// 并发执行 20 次消费，每次 10 元
	n := 20
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		go func(idx int) {
			_, err := testStore.ConsumeTx(context.Background(), ConsumeTxParams{
				MembershipID:   membership.ID,
				Amount:         1000,
				RelatedOrderID: int64(10000 + idx),
				Notes:          "并发消费",
			})
			errs <- err
		}(i)
	}

	// 检查所有消费是否成功
	for i := 0; i < n; i++ {
		err := <-errs
		require.NoError(t, err)
	}

	// 验证最终余额正确：1000 - 20*10 = 800 元
	dbMembership, err := testStore.GetMerchantMembership(context.Background(), membership.ID)
	require.NoError(t, err)
	require.Equal(t, int64(80000), dbMembership.Balance)
	require.Equal(t, int64(20000), dbMembership.TotalConsumed)
}
