package db

import (
	"context"
	"testing"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// TestClaimRefundTx 测试索赔退款事务
func TestClaimRefundTx(t *testing.T) {
	user := createRandomUser(t)

	// 执行索赔退款
	result, err := testStore.ClaimRefundTx(context.Background(), ClaimRefundTxParams{
		ClaimID:    util.RandomInt(1, 10000),
		UserID:     user.ID,
		Amount:     5000, // 50元
		SourceType: "rider_deposit",
		SourceID:   123,
		Remark:     "餐损索赔退款测试",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证余额
	require.Equal(t, int64(5000), result.UserBalance.Balance)
	require.Equal(t, int64(5000), result.UserBalance.TotalIncome)

	// 验证日志
	require.Equal(t, "claim_refund", result.BalanceLog.Type)
	require.Equal(t, int64(5000), result.BalanceLog.Amount)
	require.Equal(t, int64(0), result.BalanceLog.BalanceBefore)
	require.Equal(t, int64(5000), result.BalanceLog.BalanceAfter)
}

// TestClaimRefundTx_Idempotent 测试索赔退款幂等性
func TestClaimRefundTx_Idempotent(t *testing.T) {
	user := createRandomUser(t)
	claimID := util.RandomInt(1, 10000)

	// 第一次退款
	result1, err := testStore.ClaimRefundTx(context.Background(), ClaimRefundTxParams{
		ClaimID:    claimID,
		UserID:     user.ID,
		Amount:     5000,
		SourceType: "rider_deposit",
		SourceID:   123,
		Remark:     "餐损索赔退款",
	})
	require.NoError(t, err)
	require.Equal(t, int64(5000), result1.UserBalance.Balance)

	// 第二次退款（相同claimID）
	result2, err := testStore.ClaimRefundTx(context.Background(), ClaimRefundTxParams{
		ClaimID:    claimID, // 相同的索赔ID
		UserID:     user.ID,
		Amount:     5000,
		SourceType: "rider_deposit",
		SourceID:   123,
		Remark:     "餐损索赔退款",
	})
	require.NoError(t, err)

	// 余额应该还是5000，不应重复入账
	require.Equal(t, int64(5000), result2.UserBalance.Balance)
}

// 注意：createRandomRider 已在 rider_test.go 中定义，此处不再重复
