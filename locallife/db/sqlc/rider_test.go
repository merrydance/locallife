package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

func createRandomRider(t *testing.T) Rider {
	user := createRandomUser(t)
	return createRandomRiderWithUser(t, user.ID)
}

func createRandomRiderWithUser(t *testing.T, userID int64) Rider {
	arg := CreateRiderParams{
		UserID:   userID,
		RealName: util.RandomString(6),
		IDCardNo: util.RandomString(18),
		Phone:    "138" + util.RandomString(8),
	}

	rider, err := testStore.CreateRider(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, rider)

	require.Equal(t, arg.UserID, rider.UserID)
	require.Equal(t, arg.RealName, rider.RealName)
	require.Equal(t, arg.IDCardNo, rider.IDCardNo)
	require.Equal(t, arg.Phone, rider.Phone)
	require.Equal(t, "pending", rider.Status)
	require.Equal(t, int64(0), rider.DepositAmount)
	require.Equal(t, false, rider.IsOnline)
	require.NotZero(t, rider.ID)
	require.NotZero(t, rider.CreatedAt)

	return rider
}

func createActiveRider(t *testing.T) Rider {
	rider := createRandomRider(t)

	// 审核通过
	updated, err := testStore.UpdateRiderStatus(context.Background(), UpdateRiderStatusParams{
		ID:     rider.ID,
		Status: "active",
	})
	require.NoError(t, err)
	require.Equal(t, "active", updated.Status)

	// 充值押金
	updated, err = testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 30000, // 300元
		FrozenDeposit: 0,
	})
	require.NoError(t, err)
	require.Equal(t, int64(30000), updated.DepositAmount)

	// 创建骑手profile（高值单资格积分需要）
	_, err = testStore.CreateRiderProfile(context.Background(), CreateRiderProfileParams{
		RiderID:    rider.ID,
		TrustScore: 850, // 默认信任分
	})
	require.NoError(t, err)

	return updated
}

func createOnlineRider(t *testing.T) Rider {
	rider := createActiveRider(t)

	// 上线
	updated, err := testStore.UpdateRiderOnlineStatus(context.Background(), UpdateRiderOnlineStatusParams{
		ID:       rider.ID,
		IsOnline: true,
	})
	require.NoError(t, err)
	require.True(t, updated.IsOnline)

	// 更新位置，以便 ListNearbyRiders 能找到
	updated, err = testStore.UpdateRiderLocation(context.Background(), UpdateRiderLocationParams{
		ID:               rider.ID,
		CurrentLongitude: numericFromFloat(116.404),
		CurrentLatitude:  numericFromFloat(39.915),
	})
	require.NoError(t, err)

	return updated
}

// ==================== Rider Tests ====================

func TestCreateRider(t *testing.T) {
	createRandomRider(t)
}

// TestCreateRider_DuplicateUserID 测试同一用户重复申请骑手
func TestCreateRider_DuplicateUserID(t *testing.T) {
	user := createRandomUser(t)

	// 第一次创建
	createRandomRiderWithUser(t, user.ID)

	// 同一用户重复创建应该失败（唯一约束：user_id）
	arg := CreateRiderParams{
		UserID:   user.ID,
		RealName: util.RandomString(6),
		IDCardNo: util.RandomString(18),
		Phone:    "138" + util.RandomString(8),
	}

	_, err := testStore.CreateRider(context.Background(), arg)
	require.Error(t, err, "同一用户重复申请应该返回错误")
	require.Equal(t, UniqueViolation, ErrorCode(err))
}

// TestCreateRider_DuplicateIDCard 测试同一身份证号重复申请骑手
func TestCreateRider_DuplicateIDCard(t *testing.T) {
	rider1 := createRandomRider(t)

	// 使用相同身份证号创建另一个骑手
	user2 := createRandomUser(t)
	arg := CreateRiderParams{
		UserID:   user2.ID,
		RealName: util.RandomString(6),
		IDCardNo: rider1.IDCardNo, // 使用相同的身份证号
		Phone:    "139" + util.RandomString(8),
	}

	_, err := testStore.CreateRider(context.Background(), arg)
	require.Error(t, err, "同一身份证号重复申请应该返回错误")
	require.Equal(t, UniqueViolation, ErrorCode(err))
}

func TestGetRider(t *testing.T) {
	rider1 := createRandomRider(t)

	rider2, err := testStore.GetRider(context.Background(), rider1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, rider2)

	require.Equal(t, rider1.ID, rider2.ID)
	require.Equal(t, rider1.UserID, rider2.UserID)
	require.Equal(t, rider1.RealName, rider2.RealName)
	require.Equal(t, rider1.Status, rider2.Status)
	require.WithinDuration(t, rider1.CreatedAt, rider2.CreatedAt, time.Second)
}

func TestGetRiderByUserID(t *testing.T) {
	user := createRandomUser(t)
	rider1 := createRandomRiderWithUser(t, user.ID)

	rider2, err := testStore.GetRiderByUserID(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, rider2)

	require.Equal(t, rider1.ID, rider2.ID)
	require.Equal(t, rider1.UserID, rider2.UserID)
}

func TestUpdateRiderStatus(t *testing.T) {
	rider := createRandomRider(t)
	require.Equal(t, "pending", rider.Status)

	// 审核通过
	updated, err := testStore.UpdateRiderStatus(context.Background(), UpdateRiderStatusParams{
		ID:     rider.ID,
		Status: "active",
	})
	require.NoError(t, err)
	require.Equal(t, "active", updated.Status)

	// 禁用
	updated, err = testStore.UpdateRiderStatus(context.Background(), UpdateRiderStatusParams{
		ID:     rider.ID,
		Status: "suspended",
	})
	require.NoError(t, err)
	require.Equal(t, "suspended", updated.Status)
}

func TestUpdateRiderDeposit(t *testing.T) {
	rider := createRandomRider(t)
	require.Equal(t, int64(0), rider.DepositAmount)

	// 充值押金
	updated, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 30000,
		FrozenDeposit: 0,
	})
	require.NoError(t, err)
	require.Equal(t, int64(30000), updated.DepositAmount)
	require.Equal(t, int64(0), updated.FrozenDeposit)

	// 冻结部分押金
	updated, err = testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 30000,
		FrozenDeposit: 5000,
	})
	require.NoError(t, err)
	require.Equal(t, int64(30000), updated.DepositAmount)
	require.Equal(t, int64(5000), updated.FrozenDeposit)
}

func TestUpdateRiderOnlineStatus(t *testing.T) {
	rider := createActiveRider(t)
	require.False(t, rider.IsOnline)

	// 上线
	updated, err := testStore.UpdateRiderOnlineStatus(context.Background(), UpdateRiderOnlineStatusParams{
		ID:       rider.ID,
		IsOnline: true,
	})
	require.NoError(t, err)
	require.True(t, updated.IsOnline)

	// 下线
	updated, err = testStore.UpdateRiderOnlineStatus(context.Background(), UpdateRiderOnlineStatusParams{
		ID:       rider.ID,
		IsOnline: false,
	})
	require.NoError(t, err)
	require.False(t, updated.IsOnline)
}

func TestListOnlineRiders(t *testing.T) {
	// 创建几个在线骑手
	rider1 := createOnlineRider(t)
	rider2 := createOnlineRider(t)
	_ = createActiveRider(t) // 这个是离线的

	riders, err := testStore.ListOnlineRiders(context.Background())
	require.NoError(t, err)

	// 验证至少包含我们创建的在线骑手
	riderIDs := make(map[int64]bool)
	for _, r := range riders {
		riderIDs[r.ID] = true
		require.True(t, r.IsOnline)
		require.Equal(t, "active", r.Status)
	}
	require.True(t, riderIDs[rider1.ID])
	require.True(t, riderIDs[rider2.ID])
}

func TestListNearbyRiders(t *testing.T) {
	// 使用一个独特的位置来避免与历史测试数据冲突
	// 随机生成一个稍微偏移的位置
	uniqueLat := 40.0 + float64(util.RandomInt(1, 9999))/100000.0
	uniqueLng := 117.0 + float64(util.RandomInt(1, 9999))/100000.0

	// 创建一个活跃骑手
	rider := createActiveRider(t)

	// 上线
	updated, err := testStore.UpdateRiderOnlineStatus(context.Background(), UpdateRiderOnlineStatusParams{
		ID:       rider.ID,
		IsOnline: true,
	})
	require.NoError(t, err)
	require.True(t, updated.IsOnline)

	// 更新位置到唯一位置
	updated, err = testStore.UpdateRiderLocation(context.Background(), UpdateRiderLocationParams{
		ID:               rider.ID,
		CurrentLongitude: numericFromFloat(uniqueLng),
		CurrentLatitude:  numericFromFloat(uniqueLat),
	})
	require.NoError(t, err)

	// 确认骑手位置已设置
	require.True(t, updated.CurrentLongitude.Valid, "骑手经度应该有效")
	require.True(t, updated.CurrentLatitude.Valid, "骑手纬度应该有效")

	// 从数据库重新获取骑手确认数据已持久化
	dbRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	require.True(t, dbRider.IsOnline, "数据库中骑手应该在线")
	require.Equal(t, "active", dbRider.Status, "数据库中骑手状态应该是 active")

	// 搜索附近骑手（1公里范围内应该只有我们创建的骑手）
	riders, err := testStore.ListNearbyRiders(context.Background(), ListNearbyRidersParams{
		CenterLat:   uniqueLat,
		CenterLng:   uniqueLng,
		MaxDistance: 1000, // 1公里范围
		LimitCount:  100,
	})
	require.NoError(t, err)
	require.NotEmpty(t, riders, "应该至少找到一个附近骑手")

	// 验证能找到我们创建的骑手
	found := false
	for _, r := range riders {
		if r.ID == rider.ID {
			found = true
			require.True(t, r.IsOnline)
			// 由于位置相同，距离应该是0
			require.Equal(t, int32(0), r.Distance, "距离应该是0米")
			break
		}
	}
	require.True(t, found, "应该能找到附近的在线骑手")
}

func TestUpdateRiderStats(t *testing.T) {
	rider := createActiveRider(t)

	// 更新统计
	updated, err := testStore.UpdateRiderStats(context.Background(), UpdateRiderStatsParams{
		ID:            rider.ID,
		TotalOrders:   10,
		TotalEarnings: 50000, // 500元
	})
	require.NoError(t, err)
	require.Equal(t, int32(10), updated.TotalOrders)
	require.Equal(t, int64(50000), updated.TotalEarnings)
}

func TestCountRidersByStatus(t *testing.T) {
	createRandomRider(t) // pending
	createActiveRider(t) // active

	pendingCount, err := testStore.CountRidersByStatus(context.Background(), "pending")
	require.NoError(t, err)
	require.GreaterOrEqual(t, pendingCount, int64(1))

	activeCount, err := testStore.CountRidersByStatus(context.Background(), "active")
	require.NoError(t, err)
	require.GreaterOrEqual(t, activeCount, int64(1))
}

func TestCountOnlineRiders(t *testing.T) {
	createOnlineRider(t)

	count, err := testStore.CountOnlineRiders(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(1))
}

// ==================== Rider Deposit Tests ====================

func TestCreateRiderDeposit(t *testing.T) {
	rider := createActiveRider(t)

	arg := CreateRiderDepositParams{
		RiderID:      rider.ID,
		Amount:       10000,
		Type:         "deposit",
		BalanceAfter: rider.DepositAmount + 10000,
		Remark:       pgtype.Text{String: "充值押金", Valid: true},
	}

	deposit, err := testStore.CreateRiderDeposit(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, deposit)

	require.Equal(t, arg.RiderID, deposit.RiderID)
	require.Equal(t, arg.Amount, deposit.Amount)
	require.Equal(t, arg.Type, deposit.Type)
	require.Equal(t, arg.BalanceAfter, deposit.BalanceAfter)
	require.NotZero(t, deposit.ID)
	require.NotZero(t, deposit.CreatedAt)
}

func TestListRiderDeposits(t *testing.T) {
	rider := createActiveRider(t)

	// 创建几笔押金记录
	for i := 0; i < 3; i++ {
		_, err := testStore.CreateRiderDeposit(context.Background(), CreateRiderDepositParams{
			RiderID:      rider.ID,
			Amount:       int64((i + 1) * 10000),
			Type:         "deposit",
			BalanceAfter: int64((i + 1) * 10000),
		})
		require.NoError(t, err)
	}

	deposits, err := testStore.ListRiderDeposits(context.Background(), ListRiderDepositsParams{
		RiderID: rider.ID,
		Limit:   10,
		Offset:  0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(deposits), 3)

	// 验证按时间倒序
	for i := 0; i < len(deposits)-1; i++ {
		require.GreaterOrEqual(t, deposits[i].CreatedAt, deposits[i+1].CreatedAt)
	}
}

// ==================== Withdraw Deposit Transaction Tests ====================

func TestWithdrawDepositTx(t *testing.T) {
	rider := createActiveRider(t)

	// 先充值押金
	initialDeposit := int64(50000) // 500元
	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: initialDeposit,
		FrozenDeposit: 0,
	})
	require.NoError(t, err)

	// 执行提现
	withdrawAmount := int64(20000) // 200元
	result, err := testStore.WithdrawDepositTx(context.Background(), WithdrawDepositTxParams{
		RiderID: rider.ID,
		Amount:  withdrawAmount,
		Remark:  "测试提现",
	})
	require.NoError(t, err)

	// 验证骑手余额正确更新
	require.Equal(t, initialDeposit-withdrawAmount, result.Rider.DepositAmount)
	require.Equal(t, int64(0), result.Rider.FrozenDeposit)

	// 验证押金流水正确创建
	require.Equal(t, rider.ID, result.DepositLog.RiderID)
	require.Equal(t, withdrawAmount, result.DepositLog.Amount)
	require.Equal(t, "withdraw", result.DepositLog.Type)
	require.Equal(t, initialDeposit-withdrawAmount, result.DepositLog.BalanceAfter)

	// 再次查询骑手确认数据库已持久化
	updatedRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, initialDeposit-withdrawAmount, updatedRider.DepositAmount)
}

func TestWithdrawDepositTx_InsufficientBalance(t *testing.T) {
	rider := createActiveRider(t)

	// 充值少量押金
	initialDeposit := int64(10000) // 100元
	frozenDeposit := int64(5000)   // 冻结50元，可用50元
	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: initialDeposit,
		FrozenDeposit: frozenDeposit,
	})
	require.NoError(t, err)

	// 尝试提现超过可用余额
	withdrawAmount := int64(8000) // 80元，超过可用的50元
	_, err = testStore.WithdrawDepositTx(context.Background(), WithdrawDepositTxParams{
		RiderID: rider.ID,
		Amount:  withdrawAmount,
		Remark:  "测试提现失败",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "可用余额不足")

	// 验证余额未变化
	updatedRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, initialDeposit, updatedRider.DepositAmount) // 余额不变
}

func TestWithdrawDepositTx_Concurrent(t *testing.T) {
	rider := createActiveRider(t)

	// 充值押金
	initialDeposit := int64(100000) // 1000元
	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: initialDeposit,
		FrozenDeposit: 0,
	})
	require.NoError(t, err)

	// 并发提现测试：同时发起多个提现请求
	n := 5
	withdrawAmount := int64(30000) // 每次300元，5次共1500元，超过1000元
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		go func() {
			_, err := testStore.WithdrawDepositTx(context.Background(), WithdrawDepositTxParams{
				RiderID: rider.ID,
				Amount:  withdrawAmount,
				Remark:  "并发提现测试",
			})
			errs <- err
		}()
	}

	// 收集结果
	successCount := 0
	failCount := 0
	for i := 0; i < n; i++ {
		err := <-errs
		if err == nil {
			successCount++
		} else {
			failCount++
		}
	}

	// 验证：只有部分提现成功（余额不足时应失败）
	// 1000元最多成功3次300元提现
	require.LessOrEqual(t, successCount, 3)
	require.GreaterOrEqual(t, failCount, 2)

	// 验证最终余额一致
	finalRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	expectedBalance := initialDeposit - int64(successCount)*withdrawAmount
	require.Equal(t, expectedBalance, finalRider.DepositAmount)
	require.GreaterOrEqual(t, finalRider.DepositAmount, int64(0)) // 余额不能为负
}

func TestRollbackWithdrawTx(t *testing.T) {
	rider := createActiveRider(t)

	// 充值押金
	initialDeposit := int64(50000) // 500元
	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: initialDeposit,
		FrozenDeposit: 0,
	})
	require.NoError(t, err)

	// 先执行一次提现
	withdrawAmount := int64(20000) // 200元
	result, err := testStore.WithdrawDepositTx(context.Background(), WithdrawDepositTxParams{
		RiderID: rider.ID,
		Amount:  withdrawAmount,
		Remark:  "测试提现",
	})
	require.NoError(t, err)
	require.Equal(t, initialDeposit-withdrawAmount, result.Rider.DepositAmount)

	// 模拟微信支付失败，执行回滚
	err = testStore.RollbackWithdrawTx(context.Background(), RollbackWithdrawTxParams{
		RiderID: rider.ID,
		Amount:  withdrawAmount,
	})
	require.NoError(t, err)

	// 验证余额已恢复
	finalRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, initialDeposit, finalRider.DepositAmount)

	// 验证有回滚流水记录
	deposits, err := testStore.ListRiderDeposits(context.Background(), ListRiderDepositsParams{
		RiderID: rider.ID,
		Limit:   10,
		Offset:  0,
	})
	require.NoError(t, err)

	// 找到回滚记录
	foundRollback := false
	for _, d := range deposits {
		if d.Type == "withdraw_rollback" {
			foundRollback = true
			require.Equal(t, withdrawAmount, d.Amount)
			require.Equal(t, initialDeposit, d.BalanceAfter)
			break
		}
	}
	require.True(t, foundRollback, "应该存在 withdraw_rollback 类型的流水记录")
}

// ==================== Rider Location Tests ====================

func TestCreateRiderLocation(t *testing.T) {
	rider := createOnlineRider(t)

	arg := CreateRiderLocationParams{
		RiderID:    rider.ID,
		Longitude:  numericFromFloat(116.405),
		Latitude:   numericFromFloat(39.916),
		Accuracy:   pgtype.Numeric{Valid: false},
		Speed:      pgtype.Numeric{Valid: false},
		Heading:    pgtype.Numeric{Valid: false},
		RecordedAt: time.Now(),
	}

	location, err := testStore.CreateRiderLocation(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, location)

	require.Equal(t, arg.RiderID, location.RiderID)
	require.NotZero(t, location.ID)
	require.NotZero(t, location.RecordedAt)
}

func TestBatchCreateRiderLocations(t *testing.T) {
	rider := createOnlineRider(t)

	// 准备批量位置数据
	now := time.Now()
	locations := []BatchCreateRiderLocationsParams{
		{
			RiderID:    rider.ID,
			Longitude:  numericFromFloat(116.401),
			Latitude:   numericFromFloat(39.911),
			Accuracy:   numericFromFloat(10.0),
			Speed:      numericFromFloat(5.0),
			Heading:    numericFromFloat(90.0),
			RecordedAt: now.Add(-5 * time.Minute),
		},
		{
			RiderID:    rider.ID,
			Longitude:  numericFromFloat(116.402),
			Latitude:   numericFromFloat(39.912),
			Accuracy:   numericFromFloat(8.0),
			Speed:      numericFromFloat(10.0),
			Heading:    numericFromFloat(95.0),
			RecordedAt: now.Add(-3 * time.Minute),
		},
		{
			RiderID:    rider.ID,
			Longitude:  numericFromFloat(116.403),
			Latitude:   numericFromFloat(39.913),
			Accuracy:   numericFromFloat(6.0),
			Speed:      numericFromFloat(8.0),
			Heading:    numericFromFloat(100.0),
			RecordedAt: now.Add(-1 * time.Minute),
		},
	}

	// 批量插入
	count, err := testStore.BatchCreateRiderLocations(context.Background(), locations)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)

	// 验证数据已插入
	storedLocations, err := testStore.ListRiderLocations(context.Background(), ListRiderLocationsParams{
		RiderID:      rider.ID,
		RecordedAt:   now.Add(-10 * time.Minute),
		RecordedAt_2: now.Add(time.Minute),
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(storedLocations), 3)
}

func TestListRiderLocations(t *testing.T) {
	rider := createOnlineRider(t)

	// 获取当前时间作为基准
	startTime := time.Now()

	// 创建几个位置记录
	for i := 0; i < 5; i++ {
		_, err := testStore.CreateRiderLocation(context.Background(), CreateRiderLocationParams{
			RiderID:    rider.ID,
			Longitude:  numericFromFloat(116.404 + float64(i)*0.001),
			Latitude:   numericFromFloat(39.915 + float64(i)*0.001),
			RecordedAt: time.Now(), // 必须设置 RecordedAt
		})
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // 确保时间戳不同
	}

	endTime := time.Now().Add(time.Second)

	locations, err := testStore.ListRiderLocations(context.Background(), ListRiderLocationsParams{
		RiderID:      rider.ID,
		RecordedAt:   startTime.Add(-time.Second), // 稍早于开始时间
		RecordedAt_2: endTime,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(locations), 5)

	// 验证按时间升序
	for i := 0; i < len(locations)-1; i++ {
		require.LessOrEqual(t, locations[i].RecordedAt, locations[i+1].RecordedAt)
	}
}

func TestGetRiderLatestLocation(t *testing.T) {
	rider := createOnlineRider(t)

	// 创建几个位置记录
	var lastLng float64
	for i := 0; i < 3; i++ {
		lastLng = 116.404 + float64(i)*0.001
		_, err := testStore.CreateRiderLocation(context.Background(), CreateRiderLocationParams{
			RiderID:    rider.ID,
			Longitude:  numericFromFloat(lastLng),
			Latitude:   numericFromFloat(39.915),
			RecordedAt: time.Now(),
		})
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // 确保时间戳不同
	}

	location, err := testStore.GetRiderLatestLocation(context.Background(), rider.ID)
	require.NoError(t, err)

	lng, _ := location.Longitude.Float64Value()
	require.InDelta(t, lastLng, lng.Float64, 0.0001)
}

// ==================== Premium Score (高值单资格积分) Tests ====================

// 创建骑手并确保有rider_profile
// 注意：createActiveRider 已经自动创建了 rider_profile
func createRiderWithProfile(t *testing.T) Rider {
	return createActiveRider(t)
}

func TestGetRiderPremiumScore(t *testing.T) {
	rider := createRiderWithProfile(t)

	// 初始积分应该是0
	score, err := testStore.GetRiderPremiumScore(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, int16(0), score)
}

func TestUpdateRiderPremiumScore(t *testing.T) {
	rider := createRiderWithProfile(t)

	// 测试增加积分
	newScore, err := testStore.UpdateRiderPremiumScore(context.Background(), UpdateRiderPremiumScoreParams{
		RiderID:      rider.ID,
		PremiumScore: 1, // +1
	})
	require.NoError(t, err)
	require.Equal(t, int16(1), newScore)

	// 再增加
	newScore, err = testStore.UpdateRiderPremiumScore(context.Background(), UpdateRiderPremiumScoreParams{
		RiderID:      rider.ID,
		PremiumScore: 1, // +1
	})
	require.NoError(t, err)
	require.Equal(t, int16(2), newScore)

	// 测试减少积分（接高值单）
	newScore, err = testStore.UpdateRiderPremiumScore(context.Background(), UpdateRiderPremiumScoreParams{
		RiderID:      rider.ID,
		PremiumScore: -3, // -3
	})
	require.NoError(t, err)
	require.Equal(t, int16(-1), newScore)

	// 验证最终积分
	score, err := testStore.GetRiderPremiumScore(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, int16(-1), score)
}

func TestCreateRiderPremiumScoreLog(t *testing.T) {
	rider := createRiderWithProfile(t)

	// 创建一条积分变更日志
	log, err := testStore.CreateRiderPremiumScoreLog(context.Background(), CreateRiderPremiumScoreLogParams{
		RiderID:      rider.ID,
		ChangeAmount: 1,
		OldScore:     0,
		NewScore:     1,
		ChangeType:   "normal_order",
		Remark:       pgtype.Text{String: "完成普通单", Valid: true},
	})
	require.NoError(t, err)
	require.NotZero(t, log.ID)
	require.Equal(t, rider.ID, log.RiderID)
	require.Equal(t, int16(1), log.ChangeAmount)
	require.Equal(t, int16(0), log.OldScore)
	require.Equal(t, int16(1), log.NewScore)
	require.Equal(t, "normal_order", log.ChangeType)
	require.Equal(t, "完成普通单", log.Remark.String)
}

func TestListRiderPremiumScoreLogs(t *testing.T) {
	rider := createRiderWithProfile(t)

	// 创建多条积分变更日志
	for i := 0; i < 5; i++ {
		_, err := testStore.CreateRiderPremiumScoreLog(context.Background(), CreateRiderPremiumScoreLogParams{
			RiderID:      rider.ID,
			ChangeAmount: 1,
			OldScore:     int16(i),
			NewScore:     int16(i + 1),
			ChangeType:   "normal_order",
		})
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // 确保时间戳不同
	}

	// 查询日志
	logs, err := testStore.ListRiderPremiumScoreLogs(context.Background(), ListRiderPremiumScoreLogsParams{
		RiderID: rider.ID,
		Limit:   10,
		Offset:  0,
	})
	require.NoError(t, err)
	require.Len(t, logs, 5)

	// 验证按时间降序排列（最新的在前）
	for i := 0; i < len(logs)-1; i++ {
		require.GreaterOrEqual(t, logs[i].CreatedAt, logs[i+1].CreatedAt)
	}
}

func TestCountRiderPremiumScoreLogs(t *testing.T) {
	rider := createRiderWithProfile(t)

	// 创建3条日志
	for i := 0; i < 3; i++ {
		_, err := testStore.CreateRiderPremiumScoreLog(context.Background(), CreateRiderPremiumScoreLogParams{
			RiderID:      rider.ID,
			ChangeAmount: 1,
			OldScore:     int16(i),
			NewScore:     int16(i + 1),
			ChangeType:   "normal_order",
		})
		require.NoError(t, err)
	}

	// 统计数量
	count, err := testStore.CountRiderPremiumScoreLogs(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func TestGetRiderPremiumScoreWithProfile(t *testing.T) {
	rider := createRiderWithProfile(t)

	// 更新积分为5
	_, err := testStore.UpdateRiderPremiumScore(context.Background(), UpdateRiderPremiumScoreParams{
		RiderID:      rider.ID,
		PremiumScore: 5,
	})
	require.NoError(t, err)

	// 查询带档案的积分信息
	info, err := testStore.GetRiderPremiumScoreWithProfile(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, rider.ID, info.RiderID)
	require.Equal(t, rider.RealName, info.RealName)
	require.Equal(t, int16(5), info.PremiumScore)
	require.True(t, info.CanAcceptPremiumOrder)

	// 将积分减为负数
	_, err = testStore.UpdateRiderPremiumScore(context.Background(), UpdateRiderPremiumScoreParams{
		RiderID:      rider.ID,
		PremiumScore: -10,
	})
	require.NoError(t, err)

	// 再次查询
	info, err = testStore.GetRiderPremiumScoreWithProfile(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, int16(-5), info.PremiumScore)
	require.False(t, info.CanAcceptPremiumOrder)
}

func TestPremiumScoreBusinessRule(t *testing.T) {
	// 测试完整的业务规则：
	// - 初始积分为0
	// - 接普通单+1
	// - 接高值单-3
	// - 积分可为负

	rider := createRiderWithProfile(t)

	// 初始积分
	score, err := testStore.GetRiderPremiumScore(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, int16(0), score)

	// 模拟完成3个普通单
	for i := 0; i < 3; i++ {
		oldScore := score
		score, err = testStore.UpdateRiderPremiumScore(context.Background(), UpdateRiderPremiumScoreParams{
			RiderID:      rider.ID,
			PremiumScore: 1, // 普通单 +1
		})
		require.NoError(t, err)

		_, err = testStore.CreateRiderPremiumScoreLog(context.Background(), CreateRiderPremiumScoreLogParams{
			RiderID:      rider.ID,
			ChangeAmount: 1,
			OldScore:     oldScore,
			NewScore:     score,
			ChangeType:   "normal_order",
		})
		require.NoError(t, err)
	}
	require.Equal(t, int16(3), score)

	// 查询积分状态，应该可以接高值单
	info, err := testStore.GetRiderPremiumScoreWithProfile(context.Background(), rider.ID)
	require.NoError(t, err)
	require.True(t, info.CanAcceptPremiumOrder)

	// 模拟完成1个高值单
	oldScore := score
	score, err = testStore.UpdateRiderPremiumScore(context.Background(), UpdateRiderPremiumScoreParams{
		RiderID:      rider.ID,
		PremiumScore: -3, // 高值单 -3
	})
	require.NoError(t, err)
	require.Equal(t, int16(0), score)

	_, err = testStore.CreateRiderPremiumScoreLog(context.Background(), CreateRiderPremiumScoreLogParams{
		RiderID:      rider.ID,
		ChangeAmount: -3,
		OldScore:     oldScore,
		NewScore:     score,
		ChangeType:   "premium_order",
	})
	require.NoError(t, err)

	// 积分为0，仍然可以接高值单
	info, err = testStore.GetRiderPremiumScoreWithProfile(context.Background(), rider.ID)
	require.NoError(t, err)
	require.True(t, info.CanAcceptPremiumOrder)

	// 再接一个高值单，积分变为负数
	oldScore = score
	score, err = testStore.UpdateRiderPremiumScore(context.Background(), UpdateRiderPremiumScoreParams{
		RiderID:      rider.ID,
		PremiumScore: -3,
	})
	require.NoError(t, err)
	require.Equal(t, int16(-3), score)

	_, err = testStore.CreateRiderPremiumScoreLog(context.Background(), CreateRiderPremiumScoreLogParams{
		RiderID:      rider.ID,
		ChangeAmount: -3,
		OldScore:     oldScore,
		NewScore:     score,
		ChangeType:   "premium_order",
	})
	require.NoError(t, err)

	// 积分为负，不能接高值单
	info, err = testStore.GetRiderPremiumScoreWithProfile(context.Background(), rider.ID)
	require.NoError(t, err)
	require.False(t, info.CanAcceptPremiumOrder)

	// 验证日志数量
	count, err := testStore.CountRiderPremiumScoreLogs(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, int64(5), count) // 3普通单 + 2高值单
}

// ==================== 运营商骑手管理集成测试 ====================

func createRiderInRegion(t *testing.T, regionID int64, status string) Rider {
	user := createRandomUser(t)

	arg := CreateRiderParams{
		UserID:   user.ID,
		RealName: util.RandomString(6),
		IDCardNo: util.RandomString(18),
		Phone:    "138" + util.RandomString(8),
		RegionID: pgtype.Int8{Int64: regionID, Valid: true},
	}

	rider, err := testStore.CreateRider(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, rider)

	// 如果需要设置为非pending状态，更新状态
	if status != "pending" {
		rider, err = testStore.UpdateRiderStatus(context.Background(), UpdateRiderStatusParams{
			ID:     rider.ID,
			Status: status,
		})
		require.NoError(t, err)
	}

	return rider
}

func TestListRidersByRegion(t *testing.T) {
	region := createRandomRegion(t)
	regionID := pgtype.Int8{Int64: region.ID, Valid: true}

	// 在该区域创建3个骑手
	for i := 0; i < 3; i++ {
		createRiderInRegion(t, region.ID, "active")
	}

	// 查询该区域骑手
	riders, err := testStore.ListRidersByRegion(context.Background(), ListRidersByRegionParams{
		RegionID: regionID,
		Limit:    10,
		Offset:   0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(riders), 3)

	// 验证所有骑手都属于该区域
	for _, r := range riders {
		require.True(t, r.RegionID.Valid)
		require.Equal(t, region.ID, r.RegionID.Int64)
	}
}

func TestListRidersByRegion_Pagination(t *testing.T) {
	region := createRandomRegion(t)
	regionID := pgtype.Int8{Int64: region.ID, Valid: true}

	// 在该区域创建5个骑手
	for i := 0; i < 5; i++ {
		createRiderInRegion(t, region.ID, "active")
	}

	// 第一页
	page1, err := testStore.ListRidersByRegion(context.Background(), ListRidersByRegionParams{
		RegionID: regionID,
		Limit:    2,
		Offset:   0,
	})
	require.NoError(t, err)
	require.Len(t, page1, 2)

	// 第二页
	page2, err := testStore.ListRidersByRegion(context.Background(), ListRidersByRegionParams{
		RegionID: regionID,
		Limit:    2,
		Offset:   2,
	})
	require.NoError(t, err)
	require.Len(t, page2, 2)

	// 验证两页不重复
	require.NotEqual(t, page1[0].ID, page2[0].ID)
}

func TestCountRidersByRegion(t *testing.T) {
	region := createRandomRegion(t)
	regionID := pgtype.Int8{Int64: region.ID, Valid: true}

	// 创建3个骑手
	for i := 0; i < 3; i++ {
		createRiderInRegion(t, region.ID, "active")
	}

	count, err := testStore.CountRidersByRegion(context.Background(), regionID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(3))
}

func TestListRidersByRegionWithStatus(t *testing.T) {
	region := createRandomRegion(t)
	regionID := pgtype.Int8{Int64: region.ID, Valid: true}

	// 创建不同状态的骑手
	createRiderInRegion(t, region.ID, "active")
	createRiderInRegion(t, region.ID, "active")
	createRiderInRegion(t, region.ID, "suspended")

	// 只查询active状态
	activeRiders, err := testStore.ListRidersByRegionWithStatus(context.Background(), ListRidersByRegionWithStatusParams{
		RegionID: regionID,
		Status:   "active",
		Limit:    10,
		Offset:   0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(activeRiders), 2)
	for _, r := range activeRiders {
		require.Equal(t, "active", r.Status)
	}

	// 只查询suspended状态
	suspendedRiders, err := testStore.ListRidersByRegionWithStatus(context.Background(), ListRidersByRegionWithStatusParams{
		RegionID: regionID,
		Status:   "suspended",
		Limit:    10,
		Offset:   0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(suspendedRiders), 1)
	for _, r := range suspendedRiders {
		require.Equal(t, "suspended", r.Status)
	}
}

func TestCountRidersByRegionWithStatus(t *testing.T) {
	region := createRandomRegion(t)
	regionID := pgtype.Int8{Int64: region.ID, Valid: true}

	// 创建不同状态的骑手
	createRiderInRegion(t, region.ID, "active")
	createRiderInRegion(t, region.ID, "active")
	createRiderInRegion(t, region.ID, "suspended")

	activeCount, err := testStore.CountRidersByRegionWithStatus(context.Background(), CountRidersByRegionWithStatusParams{
		RegionID: regionID,
		Status:   "active",
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, activeCount, int64(2))

	suspendedCount, err := testStore.CountRidersByRegionWithStatus(context.Background(), CountRidersByRegionWithStatusParams{
		RegionID: regionID,
		Status:   "suspended",
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, suspendedCount, int64(1))
}

func TestListOnlineRidersByRegion(t *testing.T) {
	region := createRandomRegion(t)
	regionID := pgtype.Int8{Int64: region.ID, Valid: true}

	// 创建一个在线骑手
	rider := createRiderInRegion(t, region.ID, "active")
	_, err := testStore.UpdateRiderOnlineStatus(context.Background(), UpdateRiderOnlineStatusParams{
		ID:       rider.ID,
		IsOnline: true,
	})
	require.NoError(t, err)

	// 创建一个离线骑手
	createRiderInRegion(t, region.ID, "active")

	// 查询在线骑手
	onlineRiders, err := testStore.ListOnlineRidersByRegion(context.Background(), regionID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(onlineRiders), 1)

	// 验证只有在线骑手
	for _, r := range onlineRiders {
		require.True(t, r.IsOnline)
		require.Equal(t, "active", r.Status)
	}
}
