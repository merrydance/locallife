package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// TestClaimPayoutTx 测试平台索赔赔付事务
func TestClaimPayoutTx(t *testing.T) {
	user := createRandomUser(t)

	// 执行平台索赔赔付
	result, err := testStore.ClaimPayoutTx(context.Background(), ClaimPayoutTxParams{
		ClaimID:    util.RandomInt(1, 10000),
		UserID:     user.ID,
		Amount:     5000, // 50元
		SourceType: "rider_deposit",
		SourceID:   123,
		Remark:     "餐损索赔赔付测试",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证余额
	require.Equal(t, int64(5000), result.UserBalance.Balance)
	require.Equal(t, int64(5000), result.UserBalance.TotalIncome)

	// 验证日志
	require.Equal(t, "claim_payout", result.BalanceLog.Type)
	require.Equal(t, int64(5000), result.BalanceLog.Amount)
	require.Equal(t, int64(0), result.BalanceLog.BalanceBefore)
	require.Equal(t, int64(5000), result.BalanceLog.BalanceAfter)
}

// TestClaimPayoutTx_Idempotent 测试平台索赔赔付幂等性
func TestClaimPayoutTx_Idempotent(t *testing.T) {
	user := createRandomUser(t)
	claimID := util.RandomInt(1, 10000)

	// 第一次赔付
	result1, err := testStore.ClaimPayoutTx(context.Background(), ClaimPayoutTxParams{
		ClaimID:    claimID,
		UserID:     user.ID,
		Amount:     5000,
		SourceType: "rider_deposit",
		SourceID:   123,
		Remark:     "餐损索赔赔付",
	})
	require.NoError(t, err)
	require.Equal(t, int64(5000), result1.UserBalance.Balance)

	// 第二次赔付（相同claimID）
	result2, err := testStore.ClaimPayoutTx(context.Background(), ClaimPayoutTxParams{
		ClaimID:    claimID, // 相同的索赔ID
		UserID:     user.ID,
		Amount:     5000,
		SourceType: "rider_deposit",
		SourceID:   123,
		Remark:     "餐损索赔赔付",
	})
	require.NoError(t, err)

	// 余额应该还是5000，不应重复入账
	require.Equal(t, int64(5000), result2.UserBalance.Balance)
}

func TestClaimPayoutTx_IgnoresOtherRelatedLogTypes(t *testing.T) {
	user := createRandomUser(t)
	claimID := util.RandomInt(1, 10000)

	_, err := testStore.GetOrCreateUserBalance(context.Background(), user.ID)
	require.NoError(t, err)

	_, err = testStore.CreateUserBalanceLog(context.Background(), CreateUserBalanceLogParams{
		UserID:        user.ID,
		Type:          "claim_payout_reversal",
		Amount:        5000,
		BalanceBefore: 0,
		BalanceAfter:  0,
		RelatedType:   pgtype.Text{String: "claim", Valid: true},
		RelatedID:     pgtype.Int8{Int64: claimID, Valid: true},
		Remark:        pgtype.Text{String: "unrelated reversal log", Valid: true},
	})
	require.NoError(t, err)

	result, err := testStore.ClaimPayoutTx(context.Background(), ClaimPayoutTxParams{
		ClaimID:    claimID,
		UserID:     user.ID,
		Amount:     5000,
		SourceType: "rider_deposit",
		SourceID:   123,
		Remark:     "餐损索赔赔付测试",
	})
	require.NoError(t, err)
	require.Equal(t, int64(5000), result.UserBalance.Balance)
	require.Equal(t, "claim_payout", result.BalanceLog.Type)
	require.Equal(t, int64(5000), result.BalanceLog.BalanceAfter)
}

// 注意：createRandomRider 已在 rider_test.go 中定义，此处不再重复
