package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

func createRandomMembership(t *testing.T, merchantID, userID int64) MerchantMembership {
	arg := CreateMerchantMembershipParams{
		MerchantID: merchantID,
		UserID:     userID,
	}

	membership, err := testStore.CreateMerchantMembership(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, membership.ID)
	require.Equal(t, merchantID, membership.MerchantID)
	require.Equal(t, userID, membership.UserID)
	require.Equal(t, int64(0), membership.Balance)
	require.Equal(t, int64(0), membership.TotalRecharged)
	require.Equal(t, int64(0), membership.TotalConsumed)

	return membership
}

func createRandomRechargeRule(t *testing.T, merchantID int64, rechargeAmount, bonusAmount int64) RechargeRule {
	now := time.Now()
	arg := CreateRechargeRuleParams{
		MerchantID:     merchantID,
		RechargeAmount: rechargeAmount,
		BonusAmount:    bonusAmount,
		IsActive:       true,
		ValidFrom:      now.AddDate(0, 0, -1),
		ValidUntil:     now.AddDate(0, 1, 0),
	}

	rule, err := testStore.CreateRechargeRule(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, rule.ID)

	return rule
}

func rechargeMembershipBalanceForTest(t *testing.T, membershipID, amount int64) MerchantMembership {
	result, err := testStore.RechargeTx(context.Background(), RechargeTxParams{
		MembershipID:   membershipID,
		RechargeAmount: amount,
		BonusAmount:    0,
		Notes:          "test recharge",
	})
	require.NoError(t, err)
	return result.Membership
}

func consumeMembershipBalanceForTest(t *testing.T, membershipID, amount int64) MerchantMembership {
	result, err := testStore.ConsumeTx(context.Background(), ConsumeTxParams{
		MembershipID:   membershipID,
		Amount:         amount,
		RelatedOrderID: membershipID,
		Notes:          "test consume",
	})
	require.NoError(t, err)
	return result.Membership
}

// ==================== CreateMerchantMembership Tests ====================

func TestCreateMerchantMembership(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	membership := createRandomMembership(t, merchant.ID, user.ID)

	require.Equal(t, merchant.ID, membership.MerchantID)
	require.Equal(t, user.ID, membership.UserID)
	require.Equal(t, int64(0), membership.Balance)
	require.Equal(t, int64(0), membership.TotalRecharged)
	require.Equal(t, int64(0), membership.TotalConsumed)
	require.NotZero(t, membership.CreatedAt)
}

func TestCreateMerchantMembership_Duplicate(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	// 第一次创建
	createRandomMembership(t, merchant.ID, user.ID)

	// 重复创建应该失败
	_, err := testStore.CreateMerchantMembership(context.Background(), CreateMerchantMembershipParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.Error(t, err) // 唯一约束冲突
}

// ==================== GetMerchantMembership Tests ====================

func TestGetMerchantMembership(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	created := createRandomMembership(t, merchant.ID, user.ID)

	got, err := testStore.GetMerchantMembership(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, created.MerchantID, got.MerchantID)
	require.Equal(t, created.UserID, got.UserID)
}

func TestGetMerchantMembership_NotFound(t *testing.T) {
	_, err := testStore.GetMerchantMembership(context.Background(), 99999999)
	require.Error(t, err)
}

func TestGetMembershipByMerchantAndUser(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	created := createRandomMembership(t, merchant.ID, user.ID)

	got, err := testStore.GetMembershipByMerchantAndUser(context.Background(), GetMembershipByMerchantAndUserParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}

// ==================== Balance Operations Tests ====================

func TestUpdateMembershipBalance(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	membership := createRandomMembership(t, merchant.ID, user.ID)

	// 直接设置余额
	updated, err := testStore.UpdateMembershipBalance(context.Background(), UpdateMembershipBalanceParams{
		ID:               membership.ID,
		Balance:          8000,
		PrincipalBalance: 8000,
		BonusBalance:     0,
		TotalRecharged:   10000,
		TotalConsumed:    2000,
	})
	require.NoError(t, err)
	require.Equal(t, int64(8000), updated.Balance)
	require.Equal(t, updated.Balance, updated.PrincipalBalance+updated.BonusBalance)
	require.Equal(t, int64(10000), updated.TotalRecharged)
	require.Equal(t, int64(2000), updated.TotalConsumed)
}

// ==================== List Memberships Tests ====================

func TestListUserMemberships(t *testing.T) {
	user := createRandomUser(t)

	// 创建3个商户并加入会员
	for i := 0; i < 3; i++ {
		owner := createRandomUser(t)
		merchant := createRandomMerchantWithOwner(t, owner.ID)
		membership := createRandomMembership(t, merchant.ID, user.ID)

		// 充值不同金额
		rechargeMembershipBalanceForTest(t, membership.ID, int64((i+1)*10000))
	}

	// 查询用户的会员列表
	memberships, err := testStore.ListUserMemberships(context.Background(), ListUserMembershipsParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, memberships, 3)

	// 按余额降序排列
	require.GreaterOrEqual(t, memberships[0].Balance, memberships[1].Balance)
	require.GreaterOrEqual(t, memberships[1].Balance, memberships[2].Balance)
}

func TestListMerchantMembers(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建3个用户并加入会员
	for i := 0; i < 3; i++ {
		user := createRandomUser(t)
		membership := createRandomMembership(t, merchant.ID, user.ID)

		// 消费不同金额
		rechargeMembershipBalanceForTest(t, membership.ID, int64((i+1)*30000))
		consumeMembershipBalanceForTest(t, membership.ID, int64((i+1)*10000))
	}

	// 查询商户的会员列表
	members, err := testStore.ListMerchantMembers(context.Background(), ListMerchantMembersParams{
		MerchantID: merchant.ID,
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, members, 3)

	// 按消费总额降序排列
	require.GreaterOrEqual(t, members[0].TotalConsumed, members[1].TotalConsumed)
	require.GreaterOrEqual(t, members[1].TotalConsumed, members[2].TotalConsumed)
}

// ==================== Recharge Rule Tests ====================

func TestCreateRechargeRule(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	rule := createRandomRechargeRule(t, merchant.ID, 10000, 2000)

	require.Equal(t, merchant.ID, rule.MerchantID)
	require.Equal(t, int64(10000), rule.RechargeAmount)
	require.Equal(t, int64(2000), rule.BonusAmount)
	require.True(t, rule.IsActive)
}

func TestGetRechargeRule(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	created := createRandomRechargeRule(t, merchant.ID, 10000, 2000)

	got, err := testStore.GetRechargeRule(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, created.RechargeAmount, got.RechargeAmount)
	require.Equal(t, created.BonusAmount, got.BonusAmount)
}

func TestListMerchantRechargeRules(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建不同档位的充值规则
	createRandomRechargeRule(t, merchant.ID, 10000, 1000)
	createRandomRechargeRule(t, merchant.ID, 20000, 3000)
	createRandomRechargeRule(t, merchant.ID, 50000, 10000)

	rules, err := testStore.ListMerchantRechargeRules(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Len(t, rules, 3)

	// 按充值金额升序排列
	require.Equal(t, int64(10000), rules[0].RechargeAmount)
	require.Equal(t, int64(20000), rules[1].RechargeAmount)
	require.Equal(t, int64(50000), rules[2].RechargeAmount)
}

func TestListActiveRechargeRules(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建活跃规则
	createRandomRechargeRule(t, merchant.ID, 10000, 1000)
	createRandomRechargeRule(t, merchant.ID, 20000, 3000)

	// 创建已过期规则
	now := time.Now()
	_, err := testStore.CreateRechargeRule(context.Background(), CreateRechargeRuleParams{
		MerchantID:     merchant.ID,
		RechargeAmount: 30000,
		BonusAmount:    5000,
		IsActive:       true,
		ValidFrom:      now.AddDate(0, 0, -10),
		ValidUntil:     now.AddDate(0, 0, -1), // 已过期
	})
	require.NoError(t, err)

	// 只查询活跃规则
	rules, err := testStore.ListActiveRechargeRules(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Len(t, rules, 2) // 只有2个活跃的
}

func TestGetMatchingRechargeRule(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	createRandomRechargeRule(t, merchant.ID, 10000, 1000)
	createRandomRechargeRule(t, merchant.ID, 20000, 3000)

	// 匹配 10000 的规则
	rule, err := testStore.GetMatchingRechargeRule(context.Background(), GetMatchingRechargeRuleParams{
		MerchantID:     merchant.ID,
		RechargeAmount: 10000,
	})
	require.NoError(t, err)
	require.Equal(t, int64(10000), rule.RechargeAmount)
	require.Equal(t, int64(1000), rule.BonusAmount)
}

func TestUpdateRechargeRule(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	rule := createRandomRechargeRule(t, merchant.ID, 10000, 1000)

	// 更新赠送金额
	newBonus := pgtype.Int8{Int64: 2000, Valid: true}
	updated, err := testStore.UpdateRechargeRule(context.Background(), UpdateRechargeRuleParams{
		ID:          rule.ID,
		BonusAmount: newBonus,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2000), updated.BonusAmount)
	require.Equal(t, rule.RechargeAmount, updated.RechargeAmount) // 未修改
}

func TestDeleteRechargeRule(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	rule := createRandomRechargeRule(t, merchant.ID, 10000, 1000)
	membership := createRandomMembership(t, merchant.ID, user.ID)
	tx, err := testStore.CreateMembershipTransaction(context.Background(), CreateMembershipTransactionParams{
		MembershipID:   membership.ID,
		Type:           "recharge",
		Amount:         rule.RechargeAmount,
		BalanceAfter:   rule.RechargeAmount + rule.BonusAmount,
		RelatedOrderID: pgtype.Int8{Valid: false},
		RechargeRuleID: pgtype.Int8{Int64: rule.ID, Valid: true},
		Notes:          pgtype.Text{String: "rule provenance", Valid: true},
	})
	require.NoError(t, err)

	err = testStore.DeleteRechargeRule(context.Background(), rule.ID)
	require.NoError(t, err)

	// Delete is a business deactivation, not a physical delete, so recharge
	// transaction provenance remains auditable.
	deactivated, err := testStore.GetRechargeRule(context.Background(), rule.ID)
	require.NoError(t, err)
	require.False(t, deactivated.IsActive)

	reloadedTx, err := testStore.GetMembershipTransaction(context.Background(), tx.ID)
	require.NoError(t, err)
	require.True(t, reloadedTx.RechargeRuleID.Valid)
	require.Equal(t, rule.ID, reloadedTx.RechargeRuleID.Int64)

	activeRules, err := testStore.ListActiveRechargeRules(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Empty(t, activeRules)

	_, err = testStore.GetMatchingRechargeRule(context.Background(), GetMatchingRechargeRuleParams{
		MerchantID:     merchant.ID,
		RechargeAmount: rule.RechargeAmount,
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

// ==================== Membership Transaction Tests ====================

func TestCreateMembershipTransaction(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	membership := createRandomMembership(t, merchant.ID, user.ID)

	arg := CreateMembershipTransactionParams{
		MembershipID:   membership.ID,
		Type:           "recharge",
		Amount:         10000,
		BalanceAfter:   10000,
		RelatedOrderID: pgtype.Int8{Valid: false},
		RechargeRuleID: pgtype.Int8{Valid: false},
		Notes:          pgtype.Text{String: "首次充值", Valid: true},
	}

	tx, err := testStore.CreateMembershipTransaction(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, tx.ID)
	require.Equal(t, membership.ID, tx.MembershipID)
	require.Equal(t, "recharge", tx.Type)
	require.Equal(t, int64(10000), tx.Amount)
	require.Equal(t, int64(10000), tx.BalanceAfter)
}

func TestCreateMembershipRechargeTransactionUniqueIdempotencyKey(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	membership := createRandomMembership(t, merchant.ID, user.ID)
	arg := CreateMembershipRechargeTransactionParams{
		MembershipID:    membership.ID,
		Amount:          10000,
		PrincipalAmount: 10000,
		BonusAmount:     0,
		BalanceAfter:    10000,
		RelatedOrderID:  pgtype.Int8{Valid: false},
		RechargeRuleID:  pgtype.Int8{Valid: false},
		Notes:           pgtype.Text{String: "首次充值", Valid: true},
		IdempotencyKey:  pgtype.Text{String: "merchant-recharge-1", Valid: true},
	}

	firstTx, err := testStore.CreateMembershipRechargeTransaction(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, firstTx.ID)

	_, err = testStore.CreateMembershipRechargeTransaction(context.Background(), arg)
	require.Error(t, err)
	require.Equal(t, UniqueViolation, ErrorCode(err))
}

func TestListMembershipTransactions(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	membership := createRandomMembership(t, merchant.ID, user.ID)

	// 创建多个交易记录
	for i := 0; i < 5; i++ {
		_, err := testStore.CreateMembershipTransaction(context.Background(), CreateMembershipTransactionParams{
			MembershipID:   membership.ID,
			Type:           "recharge",
			Amount:         int64((i + 1) * 1000),
			BalanceAfter:   int64((i + 1) * 1000),
			RelatedOrderID: pgtype.Int8{Valid: false},
			RechargeRuleID: pgtype.Int8{Valid: false},
		})
		require.NoError(t, err)
	}

	// 查询交易记录
	transactions, err := testStore.ListMembershipTransactions(context.Background(), ListMembershipTransactionsParams{
		MembershipID: membership.ID,
		Limit:        10,
		Offset:       0,
	})
	require.NoError(t, err)
	require.Len(t, transactions, 5)
}

func TestListMembershipTransactionsByType(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	membership := createRandomMembership(t, merchant.ID, user.ID)

	// 充值交易
	for i := 0; i < 3; i++ {
		_, err := testStore.CreateMembershipTransaction(context.Background(), CreateMembershipTransactionParams{
			MembershipID:   membership.ID,
			Type:           "recharge",
			Amount:         10000,
			BalanceAfter:   int64((i + 1) * 10000),
			RelatedOrderID: pgtype.Int8{Valid: false},
			RechargeRuleID: pgtype.Int8{Valid: false},
		})
		require.NoError(t, err)
	}

	// 消费交易
	for i := 0; i < 2; i++ {
		_, err := testStore.CreateMembershipTransaction(context.Background(), CreateMembershipTransactionParams{
			MembershipID:   membership.ID,
			Type:           "consume",
			Amount:         -5000,
			BalanceAfter:   int64(25000 - (i+1)*5000),
			RelatedOrderID: pgtype.Int8{Valid: false},
			RechargeRuleID: pgtype.Int8{Valid: false},
		})
		require.NoError(t, err)
	}

	// 只查询充值类型
	rechargeTransactions, err := testStore.ListMembershipTransactionsByType(context.Background(), ListMembershipTransactionsByTypeParams{
		MembershipID: membership.ID,
		Type:         "recharge",
		Limit:        10,
		Offset:       0,
	})
	require.NoError(t, err)
	require.Len(t, rechargeTransactions, 3)

	// 只查询消费类型
	consumeTransactions, err := testStore.ListMembershipTransactionsByType(context.Background(), ListMembershipTransactionsByTypeParams{
		MembershipID: membership.ID,
		Type:         "consume",
		Limit:        10,
		Offset:       0,
	})
	require.NoError(t, err)
	require.Len(t, consumeTransactions, 2)
}

func TestMembershipTransactionListsUseIDTieBreaker(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	membership := createRandomMembership(t, merchant.ID, user.ID)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)

	createTransaction := func(txType string, amount int64, balanceAfter int64) MembershipTransaction {
		tx, err := testStore.CreateMembershipTransaction(context.Background(), CreateMembershipTransactionParams{
			MembershipID:   membership.ID,
			Type:           txType,
			Amount:         amount,
			BalanceAfter:   balanceAfter,
			RelatedOrderID: pgtype.Int8{Valid: false},
			RechargeRuleID: pgtype.Int8{Valid: false},
		})
		require.NoError(t, err)
		return tx
	}

	firstRecharge := createTransaction("recharge", 1000, 1000)
	secondRecharge := createTransaction("recharge", 2000, 3000)

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE membership_transactions SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{firstRecharge.ID, secondRecharge.ID},
	)
	require.NoError(t, err)

	transactions, err := testStore.ListMembershipTransactions(context.Background(), ListMembershipTransactionsParams{
		MembershipID: membership.ID,
		Limit:        2,
		Offset:       0,
	})
	require.NoError(t, err)
	require.Len(t, transactions, 2)
	require.Equal(t, secondRecharge.ID, transactions[0].ID)
	require.Equal(t, firstRecharge.ID, transactions[1].ID)

	rechargeTransactions, err := testStore.ListMembershipTransactionsByType(context.Background(), ListMembershipTransactionsByTypeParams{
		MembershipID: membership.ID,
		Type:         "recharge",
		Limit:        2,
		Offset:       0,
	})
	require.NoError(t, err)
	require.Len(t, rechargeTransactions, 2)
	require.Equal(t, secondRecharge.ID, rechargeTransactions[0].ID)
	require.Equal(t, firstRecharge.ID, rechargeTransactions[1].ID)
}

func TestGetMembershipTransactionStats(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	membership := createRandomMembership(t, merchant.ID, user.ID)

	// 充值 30000
	for i := 0; i < 3; i++ {
		_, err := testStore.CreateMembershipTransaction(context.Background(), CreateMembershipTransactionParams{
			MembershipID: membership.ID,
			Type:         "recharge",
			Amount:       10000,
			BalanceAfter: int64((i + 1) * 10000),
		})
		require.NoError(t, err)
	}

	// 消费 8000
	_, err := testStore.CreateMembershipTransaction(context.Background(), CreateMembershipTransactionParams{
		MembershipID: membership.ID,
		Type:         "consume",
		Amount:       -8000,
		BalanceAfter: 22000,
	})
	require.NoError(t, err)

	stats, err := testStore.GetMembershipTransactionStats(context.Background(), membership.ID)
	require.NoError(t, err)
	require.Equal(t, int64(4), stats.TotalCount)
	require.Equal(t, int64(30000), stats.TotalRecharge)
	require.Equal(t, int64(8000), stats.TotalConsume)
}

// ==================== For Update Tests ====================

func TestGetMembershipForUpdate(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	membership := createRandomMembership(t, merchant.ID, user.ID)

	// ForUpdate 需要在事务中使用
	got, err := testStore.GetMembershipForUpdate(context.Background(), membership.ID)
	require.NoError(t, err)
	require.Equal(t, membership.ID, got.ID)
}

func TestGetMembershipByMerchantAndUserForUpdate(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	membership := createRandomMembership(t, merchant.ID, user.ID)

	got, err := testStore.GetMembershipByMerchantAndUserForUpdate(context.Background(), GetMembershipByMerchantAndUserForUpdateParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)
	require.Equal(t, membership.ID, got.ID)
}
