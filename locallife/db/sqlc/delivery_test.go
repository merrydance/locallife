package db

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// numericFromFloat 将 float64 转换为 pgtype.Numeric
func numericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	n.Int = big.NewInt(int64(f * 1000000))
	n.Exp = -6
	n.Valid = true
	return n
}

// ==================== Helper Functions ====================

func createRandomDelivery(t *testing.T) Delivery {
	order := createRandomOrder(t)
	return createRandomDeliveryWithOrder(t, order.ID)
}

func createRandomDeliveryWithOrder(t *testing.T, orderID int64) Delivery {
	arg := CreateDeliveryParams{
		OrderID:           orderID,
		PickupAddress:     "北京市朝阳区某商家地址",
		PickupLongitude:   numericFromFloat(116.404),
		PickupLatitude:    numericFromFloat(39.915),
		PickupContact:     pgtype.Text{String: "张三", Valid: true},
		PickupPhone:       pgtype.Text{String: "13800138000", Valid: true},
		DeliveryAddress:   "北京市朝阳区某小区地址",
		DeliveryLongitude: numericFromFloat(116.410),
		DeliveryLatitude:  numericFromFloat(39.920),
		DeliveryContact:   pgtype.Text{String: "李四", Valid: true},
		DeliveryPhone:     pgtype.Text{String: "13900139000", Valid: true},
		Distance:          2500,
		DeliveryFee:       500,
	}

	delivery, err := testStore.CreateDelivery(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, delivery)

	require.Equal(t, arg.OrderID, delivery.OrderID)
	require.Equal(t, arg.PickupAddress, delivery.PickupAddress)
	require.Equal(t, arg.DeliveryAddress, delivery.DeliveryAddress)
	require.Equal(t, arg.Distance, delivery.Distance)
	require.Equal(t, arg.DeliveryFee, delivery.DeliveryFee)
	require.Equal(t, "pending", delivery.Status)
	require.False(t, delivery.RiderID.Valid)
	require.NotZero(t, delivery.ID)
	require.NotZero(t, delivery.CreatedAt)

	return delivery
}

func createAssignedDelivery(t *testing.T, riderID int64) Delivery {
	delivery := createRandomDelivery(t)

	updated, err := testStore.AssignDelivery(context.Background(), AssignDeliveryParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: riderID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "assigned", updated.Status)
	require.True(t, updated.RiderID.Valid)
	require.Equal(t, riderID, updated.RiderID.Int64)

	return updated
}

// ==================== Delivery Tests ====================

func TestCreateDelivery(t *testing.T) {
	createRandomDelivery(t)
}

func TestGetDelivery(t *testing.T) {
	delivery1 := createRandomDelivery(t)

	delivery2, err := testStore.GetDelivery(context.Background(), delivery1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, delivery2)

	require.Equal(t, delivery1.ID, delivery2.ID)
	require.Equal(t, delivery1.OrderID, delivery2.OrderID)
	require.Equal(t, delivery1.Status, delivery2.Status)
	require.WithinDuration(t, delivery1.CreatedAt, delivery2.CreatedAt, time.Second)
}

func TestGetDeliveryByOrderID(t *testing.T) {
	delivery1 := createRandomDelivery(t)

	delivery2, err := testStore.GetDeliveryByOrderID(context.Background(), delivery1.OrderID)
	require.NoError(t, err)
	require.NotEmpty(t, delivery2)

	require.Equal(t, delivery1.ID, delivery2.ID)
	require.Equal(t, delivery1.OrderID, delivery2.OrderID)
}

func TestAssignDelivery(t *testing.T) {
	delivery := createRandomDelivery(t)
	rider := createOnlineRider(t)

	updated, err := testStore.AssignDelivery(context.Background(), AssignDeliveryParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "assigned", updated.Status)
	require.True(t, updated.RiderID.Valid)
	require.Equal(t, rider.ID, updated.RiderID.Int64)
	require.True(t, updated.AssignedAt.Valid)
}

func TestUpdateDeliveryToPickup(t *testing.T) {
	rider := createOnlineRider(t)
	delivery := createAssignedDelivery(t, rider.ID)

	updated, err := testStore.UpdateDeliveryToPickup(context.Background(), UpdateDeliveryToPickupParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "picking", updated.Status)
}

func TestUpdateDeliveryToPickup_RequiresAssignedStatus(t *testing.T) {
	rider := createOnlineRider(t)
	delivery := createAssignedDelivery(t, rider.ID)

	_, err := testStore.UpdateDeliveryToPickup(context.Background(), UpdateDeliveryToPickupParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateDeliveryToPickup(context.Background(), UpdateDeliveryToPickupParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestUpdateDeliveryToPicked(t *testing.T) {
	rider := createOnlineRider(t)
	delivery := createAssignedDelivery(t, rider.ID)

	// 先开始取餐
	_, err := testStore.UpdateDeliveryToPickup(context.Background(), UpdateDeliveryToPickupParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	// 确认取餐
	updated, err := testStore.UpdateDeliveryToPicked(context.Background(), UpdateDeliveryToPickedParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "picked", updated.Status)
	require.True(t, updated.PickedAt.Valid)
}

func TestUpdateDeliveryToDelivering(t *testing.T) {
	rider := createOnlineRider(t)
	delivery := createAssignedDelivery(t, rider.ID)

	// assigned -> picking
	_, err := testStore.UpdateDeliveryToPickup(context.Background(), UpdateDeliveryToPickupParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	// picking -> picked
	_, err = testStore.UpdateDeliveryToPicked(context.Background(), UpdateDeliveryToPickedParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	// picked -> delivering
	updated, err := testStore.UpdateDeliveryToDelivering(context.Background(), UpdateDeliveryToDeliveringParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "delivering", updated.Status)
}

func TestUpdateDeliveryToDelivered(t *testing.T) {
	rider := createOnlineRider(t)
	delivery := createAssignedDelivery(t, rider.ID)

	// assigned -> picking
	_, err := testStore.UpdateDeliveryToPickup(context.Background(), UpdateDeliveryToPickupParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	// picking -> picked
	_, err = testStore.UpdateDeliveryToPicked(context.Background(), UpdateDeliveryToPickedParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	// picked -> delivering
	_, err = testStore.UpdateDeliveryToDelivering(context.Background(), UpdateDeliveryToDeliveringParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	// delivering -> delivered
	updated, err := testStore.UpdateDeliveryToDelivered(context.Background(), UpdateDeliveryToDeliveredParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "delivered", updated.Status)
	require.True(t, updated.DeliveredAt.Valid)
}

func TestUpdateDeliveryToCompleted(t *testing.T) {
	rider := createOnlineRider(t)
	delivery := createAssignedDelivery(t, rider.ID)

	// 需要完整的状态转换链：assigned -> picking -> picked -> delivering -> delivered -> completed
	_, err := testStore.UpdateDeliveryToPickup(context.Background(), UpdateDeliveryToPickupParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateDeliveryToPicked(context.Background(), UpdateDeliveryToPickedParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateDeliveryToDelivering(context.Background(), UpdateDeliveryToDeliveringParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateDeliveryToDelivered(context.Background(), UpdateDeliveryToDeliveredParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	// 完成代取
	updated, err := testStore.UpdateDeliveryToCompleted(context.Background(), UpdateDeliveryToCompletedParams{
		ID:            delivery.ID,
		RiderEarnings: 400,
	})
	require.NoError(t, err)
	require.Equal(t, "completed", updated.Status)
	require.True(t, updated.CompletedAt.Valid)
}

func TestUpdateDeliveryToCancelled(t *testing.T) {
	delivery := createRandomDelivery(t)

	updated, err := testStore.UpdateDeliveryToCancelled(context.Background(), delivery.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", updated.Status)
}

// ==================== 边缘用例测试 ====================

// TestUpdateDeliveryToPicked_WrongStatus 测试在错误状态下尝试确认取餐
func TestUpdateDeliveryToPicked_WrongStatus(t *testing.T) {
	rider := createOnlineRider(t)
	delivery := createAssignedDelivery(t, rider.ID)

	// 尝试在assigned状态直接确认取餐（应该失败，因为SQL要求status='picking'）
	_, err := testStore.UpdateDeliveryToPicked(context.Background(), UpdateDeliveryToPickedParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.Error(t, err, "应该因为状态不是picking而失败")
}

// TestUpdateDeliveryToDelivering_WrongStatus 测试在错误状态下尝试开始代取
func TestUpdateDeliveryToDelivering_WrongStatus(t *testing.T) {
	rider := createOnlineRider(t)
	delivery := createAssignedDelivery(t, rider.ID)

	// 尝试在assigned状态直接开始代取（应该失败，因为SQL要求status='picked'）
	_, err := testStore.UpdateDeliveryToDelivering(context.Background(), UpdateDeliveryToDeliveringParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.Error(t, err, "应该因为状态不是picked而失败")
}

// TestUpdateDeliveryToDelivered_WrongStatus 测试在错误状态下尝试确认送达
func TestUpdateDeliveryToDelivered_WrongStatus(t *testing.T) {
	rider := createOnlineRider(t)
	delivery := createAssignedDelivery(t, rider.ID)

	// 尝试在assigned状态直接确认送达（应该失败，因为SQL要求status='delivering'）
	_, err := testStore.UpdateDeliveryToDelivered(context.Background(), UpdateDeliveryToDeliveredParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.Error(t, err, "应该因为状态不是delivering而失败")
}

// TestUpdateDeliveryToPicked_WrongRider 测试非本单骑手尝试操作
func TestUpdateDeliveryToPicked_WrongRider(t *testing.T) {
	rider1 := createOnlineRider(t)
	rider2 := createOnlineRider(t)
	delivery := createAssignedDelivery(t, rider1.ID)

	// rider1的代取单，先正常开始取餐
	_, err := testStore.UpdateDeliveryToPickup(context.Background(), UpdateDeliveryToPickupParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider1.ID, Valid: true},
	})
	require.NoError(t, err)

	// rider2尝试确认取餐（应该失败，因为rider_id不匹配）
	_, err = testStore.UpdateDeliveryToPicked(context.Background(), UpdateDeliveryToPickedParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider2.ID, Valid: true},
	})
	require.Error(t, err, "应该因为骑手不匹配而失败")
}

// TestAssignDelivery_AlreadyAssigned 测试重复分配（已分配的代取单不能再分配）
func TestAssignDelivery_AlreadyAssigned(t *testing.T) {
	rider1 := createOnlineRider(t)
	rider2 := createOnlineRider(t)

	delivery := createAssignedDelivery(t, rider1.ID)

	// 尝试将已分配的代取单再次分配给rider2（应该失败，因为SQL要求rider_id IS NULL）
	_, err := testStore.AssignDelivery(context.Background(), AssignDeliveryParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider2.ID, Valid: true},
	})
	require.Error(t, err, "应该因为代取单已分配而失败")
}

// TestGetDelivery_NotFound 测试查询不存在的代取单
func TestGetDelivery_NotFound(t *testing.T) {
	_, err := testStore.GetDelivery(context.Background(), 999999999)
	require.Error(t, err, "应该返回不存在错误")
}

// TestGetDeliveryByOrderID_NotFound 测试按订单ID查询不存在的代取单
func TestGetDeliveryByOrderID_NotFound(t *testing.T) {
	_, err := testStore.GetDeliveryByOrderID(context.Background(), 999999999)
	require.Error(t, err, "应该返回不存在错误")
}

func TestListRiderActiveDeliveries(t *testing.T) {
	rider := createOnlineRider(t)

	// 创建几个分配给该骑手的代取单
	for i := 0; i < 3; i++ {
		createAssignedDelivery(t, rider.ID)
	}

	deliveries, err := testStore.ListRiderActiveDeliveries(context.Background(), pgtype.Int8{Int64: rider.ID, Valid: true})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(deliveries), 3)

	for _, d := range deliveries {
		require.Equal(t, rider.ID, d.RiderID.Int64)
		require.NotEqual(t, "completed", d.Status)
		require.NotEqual(t, "cancelled", d.Status)
	}
}

// TestListRiderActiveDeliveries 已在上方测试
// TestGetDeliveryByOrderID 已在上方测试

// ==================== Delivery Pool Tests ====================

func createRandomDeliveryPoolItem(t *testing.T) DeliveryPool {
	order := createRandomOrder(t)
	_, err := testStore.UpdateOrderStatus(context.Background(), UpdateOrderStatusParams{
		ID:             order.ID,
		Status:         OrderStatusReady,
		ExpectedStatus: order.Status,
	})
	require.NoError(t, err)
	merchant, err := testStore.GetMerchant(context.Background(), order.MerchantID)
	require.NoError(t, err)

	arg := AddToDeliveryPoolParams{
		OrderID:           order.ID,
		MerchantID:        merchant.ID,
		PickupLongitude:   numericFromFloat(116.404),
		PickupLatitude:    numericFromFloat(39.915),
		DeliveryLongitude: numericFromFloat(116.410),
		DeliveryLatitude:  numericFromFloat(39.920),
		Distance:          2500,
		DeliveryFee:       500,
		ExpectedPickupAt:  time.Now().Add(30 * time.Minute),
		ExpiresAt:         time.Now().Add(10 * time.Minute),
		Priority:          1,
	}

	pool, err := testStore.AddToDeliveryPool(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, pool)

	require.Equal(t, arg.OrderID, pool.OrderID)
	require.Equal(t, arg.MerchantID, pool.MerchantID)
	require.Equal(t, arg.Distance, pool.Distance)
	require.Equal(t, arg.DeliveryFee, pool.DeliveryFee)
	require.Equal(t, arg.Priority, pool.Priority)
	require.NotZero(t, pool.ID)
	require.NotZero(t, pool.CreatedAt)

	return pool
}

func TestAddToDeliveryPool(t *testing.T) {
	createRandomDeliveryPoolItem(t)
}

func TestGetDeliveryPoolByOrderID(t *testing.T) {
	pool1 := createRandomDeliveryPoolItem(t)

	pool2, err := testStore.GetDeliveryPoolByOrderID(context.Background(), pool1.OrderID)
	require.NoError(t, err)
	require.NotEmpty(t, pool2)

	require.Equal(t, pool1.ID, pool2.ID)
	require.Equal(t, pool1.OrderID, pool2.OrderID)
}

func TestRemoveFromDeliveryPool(t *testing.T) {
	pool := createRandomDeliveryPoolItem(t)

	err := testStore.RemoveFromDeliveryPool(context.Background(), pool.OrderID)
	require.NoError(t, err)

	// 验证已删除
	_, err = testStore.GetDeliveryPoolByOrderID(context.Background(), pool.OrderID)
	require.Error(t, err)
}

func TestDeliveryListQueriesUseIDTieBreaker(t *testing.T) {
	rider := createOnlineRider(t)
	delivery1 := createAssignedDelivery(t, rider.ID)
	delivery2 := createAssignedDelivery(t, rider.ID)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)

	store, ok := testStore.(*SQLStore)
	require.True(t, ok)

	_, err := store.connPool.Exec(context.Background(),
		"UPDATE deliveries SET created_at = $1 WHERE id = ANY($2)",
		tiedCreatedAt,
		[]int64{delivery1.ID, delivery2.ID},
	)
	require.NoError(t, err)

	deliveries, err := testStore.ListDeliveriesByRider(context.Background(), ListDeliveriesByRiderParams{
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		Limit:   2,
		Offset:  0,
	})
	require.NoError(t, err)
	require.Len(t, deliveries, 2)
	require.Equal(t, delivery2.ID, deliveries[0].ID)
	require.Equal(t, delivery1.ID, deliveries[1].ID)

	assignedDeliveries, err := testStore.ListDeliveriesByRiderAndStatus(context.Background(), ListDeliveriesByRiderAndStatusParams{
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		Status:  "assigned",
		Limit:   2,
		Offset:  0,
	})
	require.NoError(t, err)
	require.Len(t, assignedDeliveries, 2)
	require.Equal(t, delivery2.ID, assignedDeliveries[0].ID)
	require.Equal(t, delivery1.ID, assignedDeliveries[1].ID)
}

func TestListDeliveriesByRiderHistoryFiltersByStatusAndCreatedAt(t *testing.T) {
	rider := createOnlineRider(t)
	beforeRange := createAssignedDelivery(t, rider.ID)
	inRange := createAssignedDelivery(t, rider.ID)
	endBoundary := createAssignedDelivery(t, rider.ID)
	rangeStart := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2026, time.May, 4, 0, 0, 0, 0, time.UTC)

	store, ok := testStore.(*SQLStore)
	require.True(t, ok)

	_, err := store.connPool.Exec(context.Background(),
		`UPDATE deliveries
		 SET created_at = CASE id
		   WHEN $1 THEN $4::timestamptz
		   WHEN $2 THEN $5::timestamptz
		   WHEN $3 THEN $6::timestamptz
		 END,
		 status = CASE id
		   WHEN $1 THEN 'completed'
		   WHEN $2 THEN 'completed'
		   WHEN $3 THEN 'completed'
		 END
		 WHERE id = ANY($7)`,
		beforeRange.ID,
		inRange.ID,
		endBoundary.ID,
		rangeStart.Add(-time.Second),
		rangeStart.Add(36*time.Hour),
		rangeEnd,
		[]int64{beforeRange.ID, inRange.ID, endBoundary.ID},
	)
	require.NoError(t, err)

	arg := ListDeliveriesByRiderHistoryParams{
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		Status:  pgtype.Text{String: "completed", Valid: true},
		StartAt: pgtype.Timestamptz{Time: rangeStart, Valid: true},
		EndAt:   pgtype.Timestamptz{Time: rangeEnd, Valid: true},
		Limit:   10,
		Offset:  0,
	}
	deliveries, err := testStore.ListDeliveriesByRiderHistory(context.Background(), arg)
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	require.Equal(t, inRange.ID, deliveries[0].ID)

	count, err := testStore.CountDeliveriesByRiderHistory(context.Background(), CountDeliveriesByRiderHistoryParams{
		RiderID: arg.RiderID,
		Status:  arg.Status,
		StartAt: arg.StartAt,
		EndAt:   arg.EndAt,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func TestListDeliveryPool(t *testing.T) {
	// 创建几个订单池项
	for i := 0; i < 3; i++ {
		createRandomDeliveryPoolItem(t)
	}

	pools, err := testStore.ListDeliveryPool(context.Background(), ListDeliveryPoolParams{
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(pools), 3)

	// 验证返回了有效优先级字段（动态计算的优先级）
	for _, p := range pools {
		require.GreaterOrEqual(t, p.EffectivePriority, int32(0), "应该有有效优先级")
	}
}

func TestListDeliveryPoolNearby(t *testing.T) {
	// 创建几个订单池项
	for i := 0; i < 3; i++ {
		createRandomDeliveryPoolItem(t)
	}

	// 搜索附近订单（5公里范围）
	pools, err := testStore.ListDeliveryPoolNearby(context.Background(), ListDeliveryPoolNearbyParams{
		RiderLat:    39.915,
		RiderLng:    116.404,
		MaxDistance: 5000,
		ResultLimit: 10,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(pools), 1)

	// 验证距离排序
	for i := 0; i < len(pools)-1; i++ {
		require.LessOrEqual(t, pools[i].DistanceToRider, pools[i+1].DistanceToRider)
	}
}

func TestCountDeliveryPool(t *testing.T) {
	createRandomDeliveryPoolItem(t)

	count, err := testStore.CountDeliveryPool(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(1))
}

// TestListDeliveryPool_EffectivePriority 测试动态优先级计算
// 验证等待时间越长，有效优先级越高
func TestListDeliveryPool_EffectivePriority(t *testing.T) {
	// 创建一个订单，使用独特的经纬度以便精确查找
	order := createRandomOrder(t)
	merchant, err := testStore.GetMerchant(context.Background(), order.MerchantID)
	require.NoError(t, err)

	// 使用远离常用测试坐标的经纬度，避免共享测试库中出现大量同坐标记录
	coordOffset := float64(order.ID%1000) / 1000000.0
	uniqueLat := 10.123456 + coordOffset
	uniqueLng := 20.654321 + coordOffset

	arg := AddToDeliveryPoolParams{
		OrderID:           order.ID,
		MerchantID:        merchant.ID,
		PickupLongitude:   numericFromFloat(uniqueLng),
		PickupLatitude:    numericFromFloat(uniqueLat),
		DeliveryLongitude: numericFromFloat(uniqueLng + 0.001),
		DeliveryLatitude:  numericFromFloat(uniqueLat - 0.001),
		Distance:          2500,
		DeliveryFee:       500,
		ExpectedPickupAt:  time.Now().Add(30 * time.Minute),
		ExpiresAt:         time.Now().Add(10 * time.Minute),
		Priority:          1,
	}

	poolItem, err := testStore.AddToDeliveryPool(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, poolItem)

	// 直接通过 order_id 获取该订单验证基础功能
	foundPool, err := testStore.GetDeliveryPoolByOrderID(context.Background(), poolItem.OrderID)
	require.NoError(t, err)
	require.Equal(t, poolItem.ID, foundPool.ID)

	// 使用 ListDeliveryPoolNearby 获取并检查 effective_priority
	// 使用相同坐标保证距离为 0
	riderLat := uniqueLat
	riderLng := uniqueLng
	pools, err := testStore.ListDeliveryPoolNearby(context.Background(), ListDeliveryPoolNearbyParams{
		RiderLat:    riderLat,
		RiderLng:    riderLng,
		MaxDistance: 1000, // 1km 范围内
		ResultLimit: 200,
	})
	require.NoError(t, err)

	// 找到我们创建的订单（因为 pickup 位置在搜索中心，距离为0）
	var found bool
	for _, p := range pools {
		if p.ID == poolItem.ID {
			found = true
			// 刚创建的订单，等待时间接近0，有效优先级应该接近基础优先级
			// effective_priority = priority + (等待秒数 / 600)
			// 刚创建时 effective_priority ≈ priority (差异在0-1范围内)
			require.GreaterOrEqual(t, p.EffectivePriority, poolItem.Priority)
			// 但不应该超过 priority + 1（刚创建，等待时间<600秒）
			require.LessOrEqual(t, p.EffectivePriority, poolItem.Priority+1)
			break
		}
	}
	require.True(t, found, "应该能找到刚创建的代取池项")
}

// TestRemoveFromDeliveryPool_NotFound 测试删除不存在的代取池项
func TestRemoveFromDeliveryPool_NotFound(t *testing.T) {
	// 删除不存在的订单应该不报错（DELETE不报no rows）
	err := testStore.RemoveFromDeliveryPool(context.Background(), 999999999)
	require.NoError(t, err)
}

// TestAddToDeliveryPool_DuplicateOrder 测试重复添加同一订单到代取池
func TestAddToDeliveryPool_DuplicateOrder(t *testing.T) {
	// 创建第一个代取池项
	poolItem := createRandomDeliveryPoolItem(t)

	// 尝试用同一订单ID再次添加（应该失败，order_id有唯一约束）
	_, err := testStore.AddToDeliveryPool(context.Background(), AddToDeliveryPoolParams{
		OrderID:           poolItem.OrderID, // 重复的order_id
		MerchantID:        poolItem.MerchantID,
		PickupLongitude:   numericFromFloat(116.404),
		PickupLatitude:    numericFromFloat(39.915),
		DeliveryLongitude: numericFromFloat(116.410),
		DeliveryLatitude:  numericFromFloat(39.920),
		Distance:          3000,
		DeliveryFee:       600,
		ExpectedPickupAt:  time.Now().Add(30 * time.Minute),
		ExpiresAt:         time.Now().Add(365 * 24 * time.Hour),
		Priority:          1,
	})
	require.Error(t, err, "重复订单ID应该触发唯一约束错误")
}

// ==================== Transaction Tests ====================

// TestGrabOrderTx 测试抢单事务
func TestGrabOrderTx(t *testing.T) {
	// 创建在线骑手（需要有足够的押金）
	rider := createOnlineRider(t)

	// 确保骑手有足够押金
	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 10000, // 100元
		FrozenDeposit: 0,
	})
	require.NoError(t, err)

	// 创建代取池订单
	poolItem := createRandomDeliveryPoolItem(t)

	// 创建对应的代取单
	delivery := createRandomDeliveryWithOrder(t, poolItem.OrderID)

	// 执行抢单事务
	result, err := testStore.GrabOrderTx(context.Background(), GrabOrderTxParams{
		DeliveryID:   delivery.ID,
		RiderID:      rider.ID,
		RiderUserID:  rider.UserID,
		OrderID:      poolItem.OrderID,
		FreezeAmount: 500, // 冻结5元
	})
	require.NoError(t, err)

	// 验证代取单已分配
	require.Equal(t, "assigned", result.Delivery.Status)
	require.Equal(t, rider.ID, result.Delivery.RiderID.Int64)

	// 验证押金流水已创建
	require.Equal(t, rider.ID, result.DepositLog.RiderID)
	require.Equal(t, int64(500), result.DepositLog.Amount)
	require.Equal(t, "freeze", result.DepositLog.Type)
	require.Equal(t, OrderStatusCourierAccepted, result.Order.Status)
	require.Equal(t, OrderStatusCourierAccepted, result.StatusLog.ToStatus)
	require.True(t, result.StatusLog.FromStatus.Valid)
	require.Equal(t, OrderStatusReady, result.StatusLog.FromStatus.String)

	// 验证订单已从代取池移除
	_, err = testStore.GetDeliveryPoolByOrderID(context.Background(), poolItem.OrderID)
	require.Error(t, err, "订单应该已从代取池移除")

	// 验证骑手押金已冻结
	updatedRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, int64(500), updatedRider.FrozenDeposit)
}

func TestGrabOrderTx_PreparingPooledOrder(t *testing.T) {
	rider := createOnlineRider(t)
	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 10000,
		FrozenDeposit: 0,
	})
	require.NoError(t, err)

	poolItem := createRandomDeliveryPoolItem(t)
	order, err := testStore.GetOrder(context.Background(), poolItem.OrderID)
	require.NoError(t, err)
	_, err = testStore.UpdateOrderStatus(context.Background(), UpdateOrderStatusParams{
		ID:                order.ID,
		Status:            OrderStatusPreparing,
		ExpectedStatus:    order.Status,
		FulfillmentStatus: pgtype.Text{String: FulfillmentStatusPreparing, Valid: true},
	})
	require.NoError(t, err)

	delivery := createRandomDeliveryWithOrder(t, poolItem.OrderID)

	result, err := testStore.GrabOrderTx(context.Background(), GrabOrderTxParams{
		DeliveryID:   delivery.ID,
		RiderID:      rider.ID,
		RiderUserID:  rider.UserID,
		OrderID:      poolItem.OrderID,
		FreezeAmount: 500,
	})
	require.NoError(t, err)
	require.Equal(t, "assigned", result.Delivery.Status)
	require.Equal(t, OrderStatusCourierAccepted, result.Order.Status)
	require.Equal(t, OrderStatusCourierAccepted, result.StatusLog.ToStatus)
	require.True(t, result.StatusLog.FromStatus.Valid)
	require.Equal(t, OrderStatusPreparing, result.StatusLog.FromStatus.String)

	_, err = testStore.GetDeliveryPoolByOrderID(context.Background(), poolItem.OrderID)
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestUpdateDeliveryToPickedTx_CourierAcceptedButNotReadyRejected(t *testing.T) {
	rider := createOnlineRider(t)
	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 10000,
		FrozenDeposit: 0,
	})
	require.NoError(t, err)

	poolItem := createRandomDeliveryPoolItem(t)
	order, err := testStore.GetOrder(context.Background(), poolItem.OrderID)
	require.NoError(t, err)
	_, err = testStore.UpdateOrderStatus(context.Background(), UpdateOrderStatusParams{
		ID:                order.ID,
		Status:            OrderStatusPreparing,
		ExpectedStatus:    order.Status,
		FulfillmentStatus: pgtype.Text{String: FulfillmentStatusPreparing, Valid: true},
	})
	require.NoError(t, err)

	delivery := createRandomDeliveryWithOrder(t, poolItem.OrderID)
	grabResult, err := testStore.GrabOrderTx(context.Background(), GrabOrderTxParams{
		DeliveryID:   delivery.ID,
		RiderID:      rider.ID,
		RiderUserID:  rider.UserID,
		OrderID:      poolItem.OrderID,
		FreezeAmount: 500,
	})
	require.NoError(t, err)
	require.Equal(t, OrderStatusCourierAccepted, grabResult.Order.Status)

	pickingResult, err := testStore.UpdateDeliveryToPickupTx(context.Background(), UpdateDeliveryToPickupTxParams{
		DeliveryID: grabResult.Delivery.ID,
		RiderID:    rider.ID,
		OrderID:    poolItem.OrderID,
	})
	require.NoError(t, err)
	require.Equal(t, "picking", pickingResult.Delivery.Status)

	_, err = testStore.UpdateDeliveryToPickedTx(context.Background(), UpdateDeliveryToPickedTxParams{
		DeliveryID: grabResult.Delivery.ID,
		RiderID:    rider.ID,
		OrderID:    poolItem.OrderID,
	})
	require.ErrorIs(t, err, ErrDeliveryStateTransitionConflict)
}

func TestUpdateDeliveryToPickupTx_RollsBackWhenOrderSyncFails(t *testing.T) {
	rider := createOnlineRider(t)
	delivery := createAssignedDelivery(t, rider.ID)

	result, err := testStore.UpdateDeliveryToPickupTx(context.Background(), UpdateDeliveryToPickupTxParams{
		DeliveryID: delivery.ID,
		RiderID:    rider.ID,
		OrderID:    delivery.OrderID,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDeliveryStateTransitionConflict)
	require.Equal(t, delivery.ID, result.Delivery.ID)

	latestDelivery, getErr := testStore.GetDelivery(context.Background(), delivery.ID)
	require.NoError(t, getErr)
	require.Equal(t, "assigned", latestDelivery.Status)
}

// TestGrabOrderTx_InsufficientDeposit 测试押金不足时抢单失败
func TestGrabOrderTx_InsufficientDeposit(t *testing.T) {
	rider := createOnlineRider(t)

	// 设置骑手押金为0
	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 100, // 只有1元
		FrozenDeposit: 0,
	})
	require.NoError(t, err)

	poolItem := createRandomDeliveryPoolItem(t)
	delivery := createRandomDeliveryWithOrder(t, poolItem.OrderID)

	// 尝试抢单（需要冻结500，但只有100可用）
	_, err = testStore.GrabOrderTx(context.Background(), GrabOrderTxParams{
		DeliveryID:   delivery.ID,
		RiderID:      rider.ID,
		RiderUserID:  rider.UserID,
		OrderID:      poolItem.OrderID,
		FreezeAmount: 500,
	})
	require.Error(t, err, "应该因为押金不足而失败")
	require.Contains(t, err.Error(), "押金余额不足")
}

// TestGrabOrderTx_Concurrent 测试并发抢单
func TestGrabOrderTx_Concurrent(t *testing.T) {
	// 创建两个有足够押金的骑手
	rider1 := createOnlineRider(t)
	rider2 := createOnlineRider(t)

	for _, rider := range []Rider{rider1, rider2} {
		_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
			ID:            rider.ID,
			DepositAmount: 10000,
			FrozenDeposit: 0,
		})
		require.NoError(t, err)
	}

	// 创建一个代取池订单
	poolItem := createRandomDeliveryPoolItem(t)
	delivery := createRandomDeliveryWithOrder(t, poolItem.OrderID)

	// 并发抢单
	errChan := make(chan error, 2)
	successChan := make(chan GrabOrderTxResult, 2)

	for _, rider := range []Rider{rider1, rider2} {
		go func(r Rider) {
			result, err := testStore.GrabOrderTx(context.Background(), GrabOrderTxParams{
				DeliveryID:   delivery.ID,
				RiderID:      r.ID,
				RiderUserID:  r.UserID,
				OrderID:      poolItem.OrderID,
				FreezeAmount: 500,
			})
			if err != nil {
				errChan <- err
			} else {
				successChan <- result
			}
		}(rider)
	}

	// 等待两个goroutine完成
	var successCount, errorCount int
	for i := 0; i < 2; i++ {
		select {
		case <-successChan:
			successCount++
		case <-errChan:
			errorCount++
		}
	}

	// 只有一个骑手应该成功抢单
	require.Equal(t, 1, successCount, "应该只有一个骑手成功抢单")
	require.Equal(t, 1, errorCount, "另一个骑手应该失败")
}

// TestGrabOrderTx_AlreadyAssignedDelivery 测试抢已分配的代取单
func TestGrabOrderTx_AlreadyAssignedDelivery(t *testing.T) {
	rider1 := createOnlineRider(t)
	rider2 := createOnlineRider(t)

	// 两个骑手都有足够押金
	for _, rider := range []Rider{rider1, rider2} {
		_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
			ID:            rider.ID,
			DepositAmount: 10000,
			FrozenDeposit: 0,
		})
		require.NoError(t, err)
	}

	// 创建代取池订单和代取单
	poolItem := createRandomDeliveryPoolItem(t)
	delivery := createRandomDeliveryWithOrder(t, poolItem.OrderID)

	// rider1 先抢单成功
	_, err := testStore.GrabOrderTx(context.Background(), GrabOrderTxParams{
		DeliveryID:   delivery.ID,
		RiderID:      rider1.ID,
		RiderUserID:  rider1.UserID,
		OrderID:      poolItem.OrderID,
		FreezeAmount: 500,
	})
	require.NoError(t, err)

	// rider2 再次尝试抢同一单（代取单已分配，应该失败）
	_, err = testStore.GrabOrderTx(context.Background(), GrabOrderTxParams{
		DeliveryID:   delivery.ID,
		RiderID:      rider2.ID,
		RiderUserID:  rider2.UserID,
		OrderID:      poolItem.OrderID,
		FreezeAmount: 500,
	})
	require.Error(t, err, "代取单已分配，应该无法再抢")
}

// TestGrabOrderTx_NotFoundDelivery 测试抢不存在的代取单
func TestGrabOrderTx_NotFoundDelivery(t *testing.T) {
	rider := createOnlineRider(t)

	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 10000,
		FrozenDeposit: 0,
	})
	require.NoError(t, err)

	// 尝试抢一个不存在的代取单
	_, err = testStore.GrabOrderTx(context.Background(), GrabOrderTxParams{
		DeliveryID:   999999999,
		RiderID:      rider.ID,
		RiderUserID:  rider.UserID,
		OrderID:      999999999,
		FreezeAmount: 500,
	})
	require.Error(t, err, "代取单不存在，应该失败")
}

// TestGrabOrderTx_RiderNotFound 测试不存在的骑手抢单
func TestGrabOrderTx_RiderNotFound(t *testing.T) {
	poolItem := createRandomDeliveryPoolItem(t)
	delivery := createRandomDeliveryWithOrder(t, poolItem.OrderID)

	// 使用不存在的骑手ID
	_, err := testStore.GrabOrderTx(context.Background(), GrabOrderTxParams{
		DeliveryID:   delivery.ID,
		RiderID:      999999999,
		RiderUserID:  999999999,
		OrderID:      poolItem.OrderID,
		FreezeAmount: 500,
	})
	require.Error(t, err, "骑手不存在，应该失败")
}

// TestCompleteDeliveryTx 测试完成代取事务
func TestCompleteDeliveryTx(t *testing.T) {
	rider := createOnlineRider(t)

	// 设置骑手押金并冻结
	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 10000,
		FrozenDeposit: 500, // 已冻结500
	})
	require.NoError(t, err)

	// 创建一个已分配的代取单并走完状态流程到delivering
	delivery := createAssignedDelivery(t, rider.ID)

	// assigned -> picking
	_, err = testStore.UpdateDeliveryToPickup(context.Background(), UpdateDeliveryToPickupParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	// picking -> picked
	_, err = testStore.UpdateDeliveryToPicked(context.Background(), UpdateDeliveryToPickedParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	// picked -> delivering
	_, err = testStore.UpdateDeliveryToDelivering(context.Background(), UpdateDeliveryToDeliveringParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	// 获取关联订单
	deliveryData, err := testStore.GetDelivery(context.Background(), delivery.ID)
	require.NoError(t, err)

	// 执行完成代取事务
	result, err := testStore.CompleteDeliveryTx(context.Background(), CompleteDeliveryTxParams{
		DeliveryID:     delivery.ID,
		RiderID:        rider.ID,
		OrderID:        deliveryData.OrderID,
		UnfreezeAmount: 500,
		DeliveryFee:    800, // 8元代取费
	})
	require.NoError(t, err)

	// 验证代取状态
	require.Equal(t, "delivered", result.Delivery.Status)

	// 验证押金流水
	require.Equal(t, rider.ID, result.DepositLog.RiderID)
	require.Equal(t, int64(500), result.DepositLog.Amount)
	require.Equal(t, "unfreeze", result.DepositLog.Type)

	// 验证骑手押金已解冻
	updatedRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), updatedRider.FrozenDeposit)

	// 验证骑手统计已更新
	require.Equal(t, int32(1), updatedRider.TotalOrders)
}

func TestCompleteDeliveryTx_AutoOfflineNonActiveRiderAfterLastDelivery(t *testing.T) {
	rider := createOnlineRider(t)

	_, err := testStore.UpdateRiderStatus(context.Background(), UpdateRiderStatusParams{
		ID:     rider.ID,
		Status: RiderStatusApproved,
	})
	require.NoError(t, err)

	_, err = testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 10000,
		FrozenDeposit: 500,
	})
	require.NoError(t, err)

	delivery := createAssignedDelivery(t, rider.ID)

	_, err = testStore.UpdateDeliveryToPickup(context.Background(), UpdateDeliveryToPickupParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateDeliveryToPicked(context.Background(), UpdateDeliveryToPickedParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateDeliveryToDelivering(context.Background(), UpdateDeliveryToDeliveringParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)

	deliveryData, err := testStore.GetDelivery(context.Background(), delivery.ID)
	require.NoError(t, err)

	_, err = testStore.CompleteDeliveryTx(context.Background(), CompleteDeliveryTxParams{
		DeliveryID:     delivery.ID,
		RiderID:        rider.ID,
		OrderID:        deliveryData.OrderID,
		UnfreezeAmount: 500,
		DeliveryFee:    800,
	})
	require.NoError(t, err)

	updatedRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, RiderStatusApproved, updatedRider.Status)
	require.False(t, updatedRider.IsOnline)
}

// TestCompleteDeliveryTx_WrongStatus 测试在错误状态下完成代取
func TestCompleteDeliveryTx_WrongStatus(t *testing.T) {
	rider := createOnlineRider(t)

	_, err := testStore.UpdateRiderDeposit(context.Background(), UpdateRiderDepositParams{
		ID:            rider.ID,
		DepositAmount: 10000,
		FrozenDeposit: 500,
	})
	require.NoError(t, err)

	// 创建一个刚分配的代取单（状态为assigned，不是delivering）
	delivery := createAssignedDelivery(t, rider.ID)
	deliveryData, err := testStore.GetDelivery(context.Background(), delivery.ID)
	require.NoError(t, err)

	// 尝试完成代取（应该失败，因为状态不是delivering）
	_, err = testStore.CompleteDeliveryTx(context.Background(), CompleteDeliveryTxParams{
		DeliveryID:     delivery.ID,
		RiderID:        rider.ID,
		OrderID:        deliveryData.OrderID,
		UnfreezeAmount: 500,
		DeliveryFee:    800,
	})
	require.Error(t, err, "应该因为状态不是delivering而失败")
}

// ==================== Recommend Config Tests ====================

func createRandomRecommendConfig(t *testing.T) RecommendConfig {
	arg := CreateRecommendConfigParams{
		Name:           util.RandomString(10) + "_config",
		DistanceWeight: numericFromFloat(0.4),
		RouteWeight:    numericFromFloat(0.3),
		UrgencyWeight:  numericFromFloat(0.2),
		ProfitWeight:   numericFromFloat(0.1),
		MaxDistance:    5000,
		MaxResults:     20,
		IsActive:       true,
	}

	config, err := testStore.CreateRecommendConfig(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, config)

	require.Equal(t, arg.Name, config.Name)
	require.Equal(t, arg.MaxDistance, config.MaxDistance)
	require.Equal(t, arg.MaxResults, config.MaxResults)
	require.Equal(t, arg.IsActive, config.IsActive)
	require.NotZero(t, config.ID)
	require.NotZero(t, config.CreatedAt)

	return config
}

func TestCreateRecommendConfig(t *testing.T) {
	createRandomRecommendConfig(t)
}

func TestGetRecommendConfig(t *testing.T) {
	config1 := createRandomRecommendConfig(t)

	// GetRecommendConfig 接受 name 参数
	config2, err := testStore.GetRecommendConfig(context.Background(), config1.Name)
	require.NoError(t, err)
	require.NotEmpty(t, config2)

	require.Equal(t, config1.ID, config2.ID)
	require.Equal(t, config1.Name, config2.Name)
}

func TestGetActiveRecommendConfig(t *testing.T) {
	// 先停用所有配置
	configs, _ := testStore.ListRecommendConfigs(context.Background())
	for _, c := range configs {
		testStore.UpdateRecommendConfig(context.Background(), UpdateRecommendConfigParams{
			ID:             c.ID,
			DistanceWeight: c.DistanceWeight,
			RouteWeight:    c.RouteWeight,
			UrgencyWeight:  c.UrgencyWeight,
			ProfitWeight:   c.ProfitWeight,
			MaxDistance:    c.MaxDistance,
			MaxResults:     c.MaxResults,
			IsActive:       false,
		})
	}

	// 创建一个激活的配置
	config := createRandomRecommendConfig(t)

	activeConfig, err := testStore.GetActiveRecommendConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, config.ID, activeConfig.ID)
	require.True(t, activeConfig.IsActive)
}

func TestListRecommendConfigs(t *testing.T) {
	createRandomRecommendConfig(t)

	configs, err := testStore.ListRecommendConfigs(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(configs), 1)
}

func TestUpdateRecommendConfig(t *testing.T) {
	config := createRandomRecommendConfig(t)

	updated, err := testStore.UpdateRecommendConfig(context.Background(), UpdateRecommendConfigParams{
		ID:             config.ID,
		DistanceWeight: numericFromFloat(0.5),
		RouteWeight:    numericFromFloat(0.2),
		UrgencyWeight:  numericFromFloat(0.2),
		ProfitWeight:   numericFromFloat(0.1),
		MaxDistance:    6000,
		MaxResults:     30,
		IsActive:       false,
	})
	require.NoError(t, err)
	// Name 不可更新，验证其他字段
	require.Equal(t, config.Name, updated.Name) // 保持原名
	require.Equal(t, int32(6000), updated.MaxDistance)
	require.Equal(t, int32(30), updated.MaxResults)
	require.False(t, updated.IsActive)
}
