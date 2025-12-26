package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCreateUserBalance 测试创建用户余额账户
func TestCreateUserBalance(t *testing.T) {
	user := createRandomUser(t)

	balance, err := testStore.CreateUserBalance(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, balance)

	require.Equal(t, user.ID, balance.UserID)
	require.Equal(t, int64(0), balance.Balance)
	require.Equal(t, int64(0), balance.FrozenBalance)
	require.Equal(t, int64(0), balance.TotalIncome)
	require.Equal(t, int64(0), balance.TotalExpense)
	require.Equal(t, int64(0), balance.TotalWithdraw)
}

// TestGetOrCreateUserBalance 测试获取或创建用户余额账户
func TestGetOrCreateUserBalance(t *testing.T) {
	user := createRandomUser(t)

	// 首次调用应该创建
	balance1, err := testStore.GetOrCreateUserBalance(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, balance1)
	require.Equal(t, user.ID, balance1.UserID)

	// 再次调用应该返回已存在的记录
	balance2, err := testStore.GetOrCreateUserBalance(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, balance1.UserID, balance2.UserID)
}

// TestAddUserBalance 测试增加用户余额
func TestAddUserBalance(t *testing.T) {
	user := createRandomUser(t)

	// 先创建账户
	_, err := testStore.CreateUserBalance(context.Background(), user.ID)
	require.NoError(t, err)

	// 增加余额
	addAmount := int64(10000) // 100元
	balance, err := testStore.AddUserBalance(context.Background(), AddUserBalanceParams{
		UserID:  user.ID,
		Balance: addAmount,
	})
	require.NoError(t, err)
	require.Equal(t, addAmount, balance.Balance)
	require.Equal(t, addAmount, balance.TotalIncome)
}

// TestDeductUserBalance 测试扣减用户余额
func TestDeductUserBalance(t *testing.T) {
	user := createRandomUser(t)

	// 先创建账户并充值
	_, err := testStore.CreateUserBalance(context.Background(), user.ID)
	require.NoError(t, err)

	initialAmount := int64(10000)
	_, err = testStore.AddUserBalance(context.Background(), AddUserBalanceParams{
		UserID:  user.ID,
		Balance: initialAmount,
	})
	require.NoError(t, err)

	// 扣减余额
	deductAmount := int64(3000)
	balance, err := testStore.DeductUserBalance(context.Background(), DeductUserBalanceParams{
		UserID:  user.ID,
		Balance: deductAmount,
	})
	require.NoError(t, err)
	require.Equal(t, initialAmount-deductAmount, balance.Balance)
	require.Equal(t, deductAmount, balance.TotalExpense)
}

// TestDeductUserBalance_InsufficientBalance 测试余额不足时扣款失败
func TestDeductUserBalance_InsufficientBalance(t *testing.T) {
	user := createRandomUser(t)

	// 先创建账户并充值少量
	_, err := testStore.CreateUserBalance(context.Background(), user.ID)
	require.NoError(t, err)

	_, err = testStore.AddUserBalance(context.Background(), AddUserBalanceParams{
		UserID:  user.ID,
		Balance: 1000, // 只有10元
	})
	require.NoError(t, err)

	// 尝试扣减超过余额的金额
	_, err = testStore.DeductUserBalance(context.Background(), DeductUserBalanceParams{
		UserID:  user.ID,
		Balance: 5000, // 50元，超过余额
	})
	require.Error(t, err) // 应该失败
}

// TestFreezeAndUnfreezeUserBalance 测试冻结和解冻余额
func TestFreezeAndUnfreezeUserBalance(t *testing.T) {
	user := createRandomUser(t)

	// 准备
	_, err := testStore.CreateUserBalance(context.Background(), user.ID)
	require.NoError(t, err)

	_, err = testStore.AddUserBalance(context.Background(), AddUserBalanceParams{
		UserID:  user.ID,
		Balance: 10000,
	})
	require.NoError(t, err)

	// 冻结
	freezeAmount := int64(3000)
	balance, err := testStore.FreezeUserBalance(context.Background(), FreezeUserBalanceParams{
		UserID:  user.ID,
		Balance: freezeAmount,
	})
	require.NoError(t, err)
	require.Equal(t, int64(7000), balance.Balance)
	require.Equal(t, freezeAmount, balance.FrozenBalance)

	// 解冻
	balance, err = testStore.UnfreezeUserBalance(context.Background(), UnfreezeUserBalanceParams{
		UserID:  user.ID,
		Balance: freezeAmount,
	})
	require.NoError(t, err)
	require.Equal(t, int64(10000), balance.Balance)
	require.Equal(t, int64(0), balance.FrozenBalance)
}

// TestCreateUserBalanceLog 测试创建余额变动日志
func TestCreateUserBalanceLog(t *testing.T) {
	user := createRandomUser(t)

	// 创建日志
	log, err := testStore.CreateUserBalanceLog(context.Background(), CreateUserBalanceLogParams{
		UserID:        user.ID,
		Type:          "claim_refund",
		Amount:        5000,
		BalanceBefore: 0,
		BalanceAfter:  5000,
	})
	require.NoError(t, err)
	require.NotEmpty(t, log)
	require.Equal(t, user.ID, log.UserID)
	require.Equal(t, "claim_refund", log.Type)
	require.Equal(t, int64(5000), log.Amount)
}

// TestListUserBalanceLogs 测试获取余额变动日志列表
func TestListUserBalanceLogs(t *testing.T) {
	user := createRandomUser(t)

	// 创建多条日志
	for i := 0; i < 5; i++ {
		_, err := testStore.CreateUserBalanceLog(context.Background(), CreateUserBalanceLogParams{
			UserID:        user.ID,
			Type:          "claim_refund",
			Amount:        int64(1000 * (i + 1)),
			BalanceBefore: int64(1000 * i),
			BalanceAfter:  int64(1000 * (i + 1)),
		})
		require.NoError(t, err)
	}

	// 查询日志
	logs, err := testStore.ListUserBalanceLogs(context.Background(), ListUserBalanceLogsParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, logs, 5)
}
