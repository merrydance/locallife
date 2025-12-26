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

// TestDeductRiderDepositAndRefundTx 测试骑手押金扣款并退款给用户
func TestDeductRiderDepositAndRefundTx(t *testing.T) {
	// 创建用户
	user := createRandomUser(t)

	// 创建骑手（需要有押金）
	rider := createRandomRider(t)

	// 给骑手充值押金
	depositAmount := int64(50000) // 500元押金
	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: depositAmount,
	})
	require.NoError(t, err)

	// 执行扣款并退款
	claimID := util.RandomInt(1, 10000)
	refundAmount := int64(3000) // 30元

	result, err := testStore.DeductRiderDepositAndRefundTx(context.Background(), DeductRiderDepositAndRefundTxParams{
		RiderID:   rider.ID,
		UserID:    user.ID,
		ClaimID:   claimID,
		Amount:    refundAmount,
		ClaimType: "damage",
	})
	require.NoError(t, err)

	// 验证骑手押金减少
	require.Equal(t, depositAmount-refundAmount, result.Rider.DepositAmount)

	// 验证用户余额增加
	require.Equal(t, refundAmount, result.UserBalance.Balance)
	require.Equal(t, refundAmount, result.UserBalance.TotalIncome)

	// 验证用户余额日志
	require.Equal(t, "claim_refund", result.BalanceLog.Type)
	require.Equal(t, refundAmount, result.BalanceLog.Amount)
	require.True(t, result.BalanceLog.SourceType.Valid)
	require.Equal(t, "rider_deposit", result.BalanceLog.SourceType.String)
}

// TestDeductRiderDepositAndRefundTx_Idempotent 测试骑手扣款退款幂等性
func TestDeductRiderDepositAndRefundTx_Idempotent(t *testing.T) {
	user := createRandomUser(t)
	rider := createRandomRider(t)

	// 给骑手充值押金
	depositAmount := int64(50000)
	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: depositAmount,
	})
	require.NoError(t, err)

	claimID := util.RandomInt(1, 10000)
	refundAmount := int64(3000)

	// 第一次执行
	result1, err := testStore.DeductRiderDepositAndRefundTx(context.Background(), DeductRiderDepositAndRefundTxParams{
		RiderID:   rider.ID,
		UserID:    user.ID,
		ClaimID:   claimID,
		Amount:    refundAmount,
		ClaimType: "damage",
	})
	require.NoError(t, err)
	require.Equal(t, depositAmount-refundAmount, result1.Rider.DepositAmount)
	require.Equal(t, refundAmount, result1.UserBalance.Balance)

	// 第二次执行（相同claimID）
	result2, err := testStore.DeductRiderDepositAndRefundTx(context.Background(), DeductRiderDepositAndRefundTxParams{
		RiderID:   rider.ID,
		UserID:    user.ID,
		ClaimID:   claimID, // 相同的索赔ID
		Amount:    refundAmount,
		ClaimType: "damage",
	})
	require.NoError(t, err)

	// 应该保持幂等，余额不变
	require.Equal(t, refundAmount, result2.UserBalance.Balance)
}

// TestDeductRiderDepositAndRefundTx_InsufficientDeposit 测试押金不足
func TestDeductRiderDepositAndRefundTx_InsufficientDeposit(t *testing.T) {
	user := createRandomUser(t)
	rider := createRandomRider(t)

	// 骑手押金很少
	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 1000, // 只有10元
	})
	require.NoError(t, err)

	// 尝试扣款超过押金的金额
	_, err = testStore.DeductRiderDepositAndRefundTx(context.Background(), DeductRiderDepositAndRefundTxParams{
		RiderID:   rider.ID,
		UserID:    user.ID,
		ClaimID:   util.RandomInt(1, 10000),
		Amount:    5000, // 50元，超过押金
		ClaimType: "damage",
	})
	require.Error(t, err) // 应该失败
}

// 注意：createRandomRider 已在 rider_test.go 中定义，此处不再重复
