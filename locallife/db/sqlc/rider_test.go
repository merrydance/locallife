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
	require.Equal(t, RiderStatusApproved, rider.Status)
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

	// 创建骑手 profile，供后续信任画像与相关测试使用
	_, err = testStore.CreateRiderProfile(context.Background(), rider.ID)
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
	require.Equal(t, RiderStatusApproved, rider.Status)

	// 审核通过
	updated, err := testStore.UpdateRiderStatus(context.Background(), UpdateRiderStatusParams{
		ID:     rider.ID,
		Status: RiderStatusActive,
	})
	require.NoError(t, err)
	require.Equal(t, RiderStatusActive, updated.Status)

	// 禁用
	updated, err = testStore.UpdateRiderStatus(context.Background(), UpdateRiderStatusParams{
		ID:     rider.ID,
		Status: RiderStatusSuspended,
	})
	require.NoError(t, err)
	require.Equal(t, RiderStatusSuspended, updated.Status)
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
	createRandomRider(t) // approved
	createActiveRider(t) // active

	approvedCount, err := testStore.CountRidersByStatus(context.Background(), RiderStatusApproved)
	require.NoError(t, err)
	require.GreaterOrEqual(t, approvedCount, int64(1))

	activeCount, err := testStore.CountRidersByStatus(context.Background(), RiderStatusActive)
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
		RiderID: rider.ID,
		StartAt: now.Add(-10 * time.Minute),
		EndAt:   now.Add(time.Minute),
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
		RiderID: rider.ID,
		StartAt: startTime.Add(-time.Second), // 稍早于开始时间
		EndAt:   endTime,
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
