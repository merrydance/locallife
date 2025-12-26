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

// createCompletedOrderForStats 创建一个已完成的订单用于统计测试
func createCompletedOrderForStats(t *testing.T, userID, merchantID int64, finalAmount int64, orderType string, createdAt time.Time) Order {
	orderNo := util.RandomString(20)

	arg := CreateOrderParams{
		OrderNo:             orderNo,
		UserID:              userID,
		MerchantID:          merchantID,
		OrderType:           orderType,
		DeliveryFee:         0,
		Subtotal:            finalAmount,
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         finalAmount,
		Status:              "completed",
	}

	order, err := testStore.CreateOrder(context.Background(), arg)
	require.NoError(t, err)

	// 更新 final_amount, platform_commission 和 created_at
	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(),
		"UPDATE orders SET final_amount = $1, platform_commission = $2, created_at = $3 WHERE id = $4",
		finalAmount, finalAmount/10, createdAt, order.ID)
	require.NoError(t, err)

	// 重新获取订单
	order, err = testStore.GetOrder(context.Background(), order.ID)
	require.NoError(t, err)

	return order
}

// createOrderItemForStats 创建订单项用于统计测试
func createOrderItemForStats(t *testing.T, orderID, dishID int64, quantity int16, subtotal int64) OrderItem {
	arg := CreateOrderItemParams{
		OrderID:   orderID,
		DishID:    pgtype.Int8{Int64: dishID, Valid: true},
		Name:      util.RandomString(10),
		UnitPrice: subtotal / int64(quantity),
		Quantity:  quantity,
		Subtotal:  subtotal,
	}

	item, err := testStore.CreateOrderItem(context.Background(), arg)
	require.NoError(t, err)
	return item
}

// ==================== GetMerchantDailyStats Tests ====================

func TestGetMerchantDailyStats(t *testing.T) {
	// 创建测试数据
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user1 := createRandomUser(t)
	user2 := createRandomUser(t)

	// 创建不同日期的订单
	today := time.Now().Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)
	twoDaysAgo := today.AddDate(0, 0, -2)

	// 今天: 2个外卖订单
	createCompletedOrderForStats(t, user1.ID, merchant.ID, 10000, "takeout", today)
	createCompletedOrderForStats(t, user2.ID, merchant.ID, 20000, "takeout", today)

	// 昨天: 1个堂食订单
	createCompletedOrderForStats(t, user1.ID, merchant.ID, 15000, "dine_in", yesterday)

	// 两天前: 1个外卖订单
	createCompletedOrderForStats(t, user2.ID, merchant.ID, 8000, "takeout", twoDaysAgo)

	// 查询统计
	startDate := twoDaysAgo
	endDate := today.Add(24 * time.Hour)

	stats, err := testStore.GetMerchantDailyStats(context.Background(), GetMerchantDailyStatsParams{
		MerchantID:  merchant.ID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	require.NoError(t, err)
	require.Len(t, stats, 3) // 3天有数据

	// 验证今天的数据（按日期降序，今天在第一位）
	require.Equal(t, int32(2), stats[0].OrderCount)
	require.Equal(t, int64(30000), stats[0].TotalSales) // 10000 + 20000
	require.Equal(t, int32(2), stats[0].TakeoutOrders)
	require.Equal(t, int32(0), stats[0].DineInOrders)

	// 验证昨天的数据
	require.Equal(t, int32(1), stats[1].OrderCount)
	require.Equal(t, int64(15000), stats[1].TotalSales)
	require.Equal(t, int32(0), stats[1].TakeoutOrders)
	require.Equal(t, int32(1), stats[1].DineInOrders)
}

func TestGetMerchantDailyStats_Empty(t *testing.T) {
	// 创建没有订单的商户
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	today := time.Now().Truncate(24 * time.Hour)
	stats, err := testStore.GetMerchantDailyStats(context.Background(), GetMerchantDailyStatsParams{
		MerchantID:  merchant.ID,
		CreatedAt:   today,
		CreatedAt_2: today.Add(24 * time.Hour),
	})
	require.NoError(t, err)
	require.Empty(t, stats)
}

func TestGetMerchantDailyStats_ExcludesPendingOrders(t *testing.T) {
	// 创建测试数据
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	today := time.Now().Truncate(24 * time.Hour)

	// 创建一个已完成订单
	createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", today)

	// 创建一个 pending 订单（不应该被统计）
	orderNo := util.RandomString(20)
	pendingOrder, err := testStore.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:     orderNo,
		UserID:      user.ID,
		MerchantID:  merchant.ID,
		OrderType:   "takeout",
		TotalAmount: 50000,
		Status:      "pending",
	})
	require.NoError(t, err)

	// 更新 pending 订单的 final_amount
	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(),
		"UPDATE orders SET final_amount = $1 WHERE id = $2", 50000, pendingOrder.ID)
	require.NoError(t, err)

	stats, err := testStore.GetMerchantDailyStats(context.Background(), GetMerchantDailyStatsParams{
		MerchantID:  merchant.ID,
		CreatedAt:   today,
		CreatedAt_2: today.Add(24 * time.Hour),
	})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, int32(1), stats[0].OrderCount) // 只有 completed 的订单
	require.Equal(t, int64(10000), stats[0].TotalSales)
}

// ==================== GetMerchantOverview Tests ====================

func TestGetMerchantOverview(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	today := time.Now().Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)

	// 创建订单
	createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", today)
	createCompletedOrderForStats(t, user.ID, merchant.ID, 20000, "takeout", today)
	createCompletedOrderForStats(t, user.ID, merchant.ID, 30000, "dine_in", yesterday)

	overview, err := testStore.GetMerchantOverview(context.Background(), GetMerchantOverviewParams{
		MerchantID:  merchant.ID,
		CreatedAt:   yesterday,
		CreatedAt_2: today.Add(24 * time.Hour),
	})
	require.NoError(t, err)

	require.Equal(t, int32(2), overview.TotalDays)         // 2天有订单
	require.Equal(t, int32(3), overview.TotalOrders)       // 3个订单
	require.Equal(t, int64(60000), overview.TotalSales)    // 10000+20000+30000
	require.Equal(t, int32(30000), overview.AvgDailySales) // 60000/2
}

func TestGetMerchantOverview_Empty(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	today := time.Now().Truncate(24 * time.Hour)

	overview, err := testStore.GetMerchantOverview(context.Background(), GetMerchantOverviewParams{
		MerchantID:  merchant.ID,
		CreatedAt:   today,
		CreatedAt_2: today.Add(24 * time.Hour),
	})
	require.NoError(t, err)

	require.Equal(t, int32(0), overview.TotalDays)
	require.Equal(t, int32(0), overview.TotalOrders)
	require.Equal(t, int64(0), overview.TotalSales)
	require.Equal(t, int32(0), overview.AvgDailySales)
}

// ==================== GetMerchantRepurchaseRate Tests ====================

func TestGetMerchantRepurchaseRate(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	user1 := createRandomUser(t) // 复购用户 - 3单
	user2 := createRandomUser(t) // 复购用户 - 2单
	user3 := createRandomUser(t) // 非复购用户 - 1单

	today := time.Now().Truncate(24 * time.Hour)

	// user1: 3个订单
	createCompletedOrderForStats(t, user1.ID, merchant.ID, 10000, "takeout", today)
	createCompletedOrderForStats(t, user1.ID, merchant.ID, 20000, "takeout", today)
	createCompletedOrderForStats(t, user1.ID, merchant.ID, 15000, "takeout", today)

	// user2: 2个订单
	createCompletedOrderForStats(t, user2.ID, merchant.ID, 30000, "dine_in", today)
	createCompletedOrderForStats(t, user2.ID, merchant.ID, 25000, "dine_in", today)

	// user3: 1个订单
	createCompletedOrderForStats(t, user3.ID, merchant.ID, 8000, "takeout", today)

	rate, err := testStore.GetMerchantRepurchaseRate(context.Background(), GetMerchantRepurchaseRateParams{
		MerchantID:  merchant.ID,
		CreatedAt:   today,
		CreatedAt_2: today.Add(24 * time.Hour),
	})
	require.NoError(t, err)

	// 3个顾客，2个复购
	require.Equal(t, int32(3), rate.TotalCustomers)
	require.Equal(t, int32(2), rate.RepeatCustomers)
	require.Equal(t, int32(6), rate.TotalOrders) // 3+2+1=6

	// 复购率: 2/3 = 66.67% = 6667 (万分比, numeric 除法有四舍五入)
	require.Equal(t, int32(6667), rate.RepurchaseRateBasisPoints)

	// 人均订单: 6/3 = 2.00 = 200 (百分数)
	require.Equal(t, int32(200), rate.AvgOrdersPerUserCents)
}

func TestGetMerchantRepurchaseRate_AllRepeat(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	today := time.Now().Truncate(24 * time.Hour)

	// 同一用户多单
	createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", today)
	createCompletedOrderForStats(t, user.ID, merchant.ID, 20000, "takeout", today)

	rate, err := testStore.GetMerchantRepurchaseRate(context.Background(), GetMerchantRepurchaseRateParams{
		MerchantID:  merchant.ID,
		CreatedAt:   today,
		CreatedAt_2: today.Add(24 * time.Hour),
	})
	require.NoError(t, err)

	require.Equal(t, int32(1), rate.TotalCustomers)
	require.Equal(t, int32(1), rate.RepeatCustomers)
	require.Equal(t, int32(10000), rate.RepurchaseRateBasisPoints) // 100% = 10000
	require.Equal(t, int32(200), rate.AvgOrdersPerUserCents)       // 2.00
}

func TestGetMerchantRepurchaseRate_NoRepeat(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	user1 := createRandomUser(t)
	user2 := createRandomUser(t)

	today := time.Now().Truncate(24 * time.Hour)

	// 每个用户只有一单
	createCompletedOrderForStats(t, user1.ID, merchant.ID, 10000, "takeout", today)
	createCompletedOrderForStats(t, user2.ID, merchant.ID, 20000, "takeout", today)

	rate, err := testStore.GetMerchantRepurchaseRate(context.Background(), GetMerchantRepurchaseRateParams{
		MerchantID:  merchant.ID,
		CreatedAt:   today,
		CreatedAt_2: today.Add(24 * time.Hour),
	})
	require.NoError(t, err)

	require.Equal(t, int32(2), rate.TotalCustomers)
	require.Equal(t, int32(0), rate.RepeatCustomers)
	require.Equal(t, int32(0), rate.RepurchaseRateBasisPoints) // 0%
	require.Equal(t, int32(100), rate.AvgOrdersPerUserCents)   // 1.00
}

func TestGetMerchantRepurchaseRate_Empty(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	today := time.Now().Truncate(24 * time.Hour)

	rate, err := testStore.GetMerchantRepurchaseRate(context.Background(), GetMerchantRepurchaseRateParams{
		MerchantID:  merchant.ID,
		CreatedAt:   today,
		CreatedAt_2: today.Add(24 * time.Hour),
	})
	require.NoError(t, err)

	require.Equal(t, int32(0), rate.TotalCustomers)
	require.Equal(t, int32(0), rate.RepeatCustomers)
	require.Equal(t, int32(0), rate.RepurchaseRateBasisPoints)
	require.Equal(t, int32(0), rate.AvgOrdersPerUserCents)
}

// ==================== CountMerchantCustomers Tests ====================

func TestCountMerchantCustomers(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	user1 := createRandomUser(t)
	user2 := createRandomUser(t)
	user3 := createRandomUser(t)

	today := time.Now().Truncate(24 * time.Hour)

	// user1 有多单，但只算1个顾客
	createCompletedOrderForStats(t, user1.ID, merchant.ID, 10000, "takeout", today)
	createCompletedOrderForStats(t, user1.ID, merchant.ID, 20000, "takeout", today)
	createCompletedOrderForStats(t, user2.ID, merchant.ID, 15000, "dine_in", today)
	createCompletedOrderForStats(t, user3.ID, merchant.ID, 8000, "takeout", today)

	count, err := testStore.CountMerchantCustomers(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, int32(3), count) // 3个不同顾客
}

func TestCountMerchantCustomers_Empty(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	count, err := testStore.CountMerchantCustomers(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, int32(0), count)
}

// ==================== GetMerchantOrderSourceStats Tests ====================

func TestGetMerchantOrderSourceStats(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	today := time.Now().Truncate(24 * time.Hour)

	// 3个外卖订单
	createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", today)
	createCompletedOrderForStats(t, user.ID, merchant.ID, 20000, "takeout", today)
	createCompletedOrderForStats(t, user.ID, merchant.ID, 30000, "takeout", today)

	// 1个堂食订单
	createCompletedOrderForStats(t, user.ID, merchant.ID, 50000, "dine_in", today)

	stats, err := testStore.GetMerchantOrderSourceStats(context.Background(), GetMerchantOrderSourceStatsParams{
		MerchantID:  merchant.ID,
		CreatedAt:   today,
		CreatedAt_2: today.Add(24 * time.Hour),
	})
	require.NoError(t, err)
	require.Len(t, stats, 2) // 2种订单类型

	// 按订单数降序排列
	require.Equal(t, "takeout", stats[0].OrderType)
	require.Equal(t, int32(3), stats[0].OrderCount)
	require.Equal(t, int64(60000), stats[0].TotalSales) // 10000+20000+30000

	require.Equal(t, "dine_in", stats[1].OrderType)
	require.Equal(t, int32(1), stats[1].OrderCount)
	require.Equal(t, int64(50000), stats[1].TotalSales)
}

// ==================== GetMerchantHourlyStats Tests ====================

func TestGetMerchantHourlyStats(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	// 使用固定时间点
	baseTime := time.Date(2025, 12, 10, 0, 0, 0, 0, time.Local)

	// 上午10点的订单
	tenAM := baseTime.Add(10 * time.Hour)
	createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", tenAM)
	createCompletedOrderForStats(t, user.ID, merchant.ID, 20000, "takeout", tenAM.Add(30*time.Minute))

	// 下午2点的订单
	twoPM := baseTime.Add(14 * time.Hour)
	createCompletedOrderForStats(t, user.ID, merchant.ID, 30000, "dine_in", twoPM)

	stats, err := testStore.GetMerchantHourlyStats(context.Background(), GetMerchantHourlyStatsParams{
		MerchantID:  merchant.ID,
		CreatedAt:   baseTime,
		CreatedAt_2: baseTime.Add(24 * time.Hour),
	})
	require.NoError(t, err)
	require.Len(t, stats, 2) // 2个小时有数据

	// 按小时升序排列
	require.Equal(t, int32(10), stats[0].Hour)
	require.Equal(t, int32(2), stats[0].OrderCount)
	require.Equal(t, int64(30000), stats[0].TotalSales)

	require.Equal(t, int32(14), stats[1].Hour)
	require.Equal(t, int32(1), stats[1].OrderCount)
	require.Equal(t, int64(30000), stats[1].TotalSales)
}

// ==================== GetTopSellingDishes Tests ====================

func TestGetTopSellingDishes(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	// 创建菜品
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)
	dish3 := createRandomDish(t, merchant.ID, category.ID)

	today := time.Now().Truncate(24 * time.Hour)

	// 创建订单
	order1 := createCompletedOrderForStats(t, user.ID, merchant.ID, 50000, "takeout", today)
	order2 := createCompletedOrderForStats(t, user.ID, merchant.ID, 30000, "takeout", today)

	// dish1 卖了 10 份
	createOrderItemForStats(t, order1.ID, dish1.ID, 5, 25000)
	createOrderItemForStats(t, order2.ID, dish1.ID, 5, 25000)

	// dish2 卖了 3 份
	createOrderItemForStats(t, order1.ID, dish2.ID, 3, 15000)

	// dish3 卖了 1 份
	createOrderItemForStats(t, order2.ID, dish3.ID, 1, 5000)

	topDishes, err := testStore.GetTopSellingDishes(context.Background(), GetTopSellingDishesParams{
		MerchantID:  merchant.ID,
		CreatedAt:   today,
		CreatedAt_2: today.Add(24 * time.Hour),
		Limit:       10,
	})
	require.NoError(t, err)
	require.Len(t, topDishes, 3)

	// 按销量降序
	require.Equal(t, dish1.ID, topDishes[0].DishID.Int64)
	require.Equal(t, int32(10), topDishes[0].TotalSold)
	require.Equal(t, int64(50000), topDishes[0].TotalRevenue)

	require.Equal(t, dish2.ID, topDishes[1].DishID.Int64)
	require.Equal(t, int32(3), topDishes[1].TotalSold)

	require.Equal(t, dish3.ID, topDishes[2].DishID.Int64)
	require.Equal(t, int32(1), topDishes[2].TotalSold)
}

func TestGetTopSellingDishes_WithLimit(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)
	dish3 := createRandomDish(t, merchant.ID, category.ID)

	today := time.Now().Truncate(24 * time.Hour)
	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 50000, "takeout", today)

	createOrderItemForStats(t, order.ID, dish1.ID, 10, 30000)
	createOrderItemForStats(t, order.ID, dish2.ID, 5, 15000)
	createOrderItemForStats(t, order.ID, dish3.ID, 1, 5000)

	// 只取前2名
	topDishes, err := testStore.GetTopSellingDishes(context.Background(), GetTopSellingDishesParams{
		MerchantID:  merchant.ID,
		CreatedAt:   today,
		CreatedAt_2: today.Add(24 * time.Hour),
		Limit:       2,
	})
	require.NoError(t, err)
	require.Len(t, topDishes, 2)
	require.Equal(t, dish1.ID, topDishes[0].DishID.Int64)
	require.Equal(t, dish2.ID, topDishes[1].DishID.Int64)
}

// ==================== GetMerchantCustomerStats Tests ====================

func TestGetMerchantCustomerStats(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	user1 := createRandomUser(t)
	user2 := createRandomUser(t)

	today := time.Now().Truncate(24 * time.Hour)

	// user1: 3个订单，总额 60000
	createCompletedOrderForStats(t, user1.ID, merchant.ID, 10000, "takeout", today)
	createCompletedOrderForStats(t, user1.ID, merchant.ID, 20000, "takeout", today)
	createCompletedOrderForStats(t, user1.ID, merchant.ID, 30000, "takeout", today)

	// user2: 1个订单，总额 50000
	createCompletedOrderForStats(t, user2.ID, merchant.ID, 50000, "dine_in", today)

	// 按消费金额排序
	customers, err := testStore.GetMerchantCustomerStats(context.Background(), GetMerchantCustomerStatsParams{
		MerchantID: merchant.ID,
		OrderBy:    "total_amount",
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, customers, 2)

	// user1 消费更多，排第一
	require.Equal(t, user1.ID, customers[0].UserID)
	require.Equal(t, int32(3), customers[0].TotalOrders)
	require.Equal(t, int64(60000), customers[0].TotalAmount)
	require.Equal(t, int32(20000), customers[0].AvgOrderAmount) // 60000/3

	require.Equal(t, user2.ID, customers[1].UserID)
	require.Equal(t, int32(1), customers[1].TotalOrders)
	require.Equal(t, int64(50000), customers[1].TotalAmount)
}

func TestGetMerchantCustomerStats_Pagination(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建5个顾客
	users := make([]User, 5)
	today := time.Now().Truncate(24 * time.Hour)

	for i := 0; i < 5; i++ {
		users[i] = createRandomUser(t)
		createCompletedOrderForStats(t, users[i].ID, merchant.ID, int64((i+1)*10000), "takeout", today)
	}

	// 第一页: 2条
	page1, err := testStore.GetMerchantCustomerStats(context.Background(), GetMerchantCustomerStatsParams{
		MerchantID: merchant.ID,
		OrderBy:    "total_amount",
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, page1, 2)

	// 第二页: 2条
	page2, err := testStore.GetMerchantCustomerStats(context.Background(), GetMerchantCustomerStatsParams{
		MerchantID: merchant.ID,
		OrderBy:    "total_amount",
		Limit:      2,
		Offset:     2,
	})
	require.NoError(t, err)
	require.Len(t, page2, 2)

	// 第三页: 1条
	page3, err := testStore.GetMerchantCustomerStats(context.Background(), GetMerchantCustomerStatsParams{
		MerchantID: merchant.ID,
		OrderBy:    "total_amount",
		Limit:      2,
		Offset:     4,
	})
	require.NoError(t, err)
	require.Len(t, page3, 1)
}

// ==================== GetCustomerMerchantDetail Tests ====================

func TestGetCustomerMerchantDetail(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	today := time.Now().Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)

	// 创建多个订单
	createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", yesterday)
	createCompletedOrderForStats(t, user.ID, merchant.ID, 20000, "takeout", today)
	createCompletedOrderForStats(t, user.ID, merchant.ID, 30000, "dine_in", today)

	detail, err := testStore.GetCustomerMerchantDetail(context.Background(), GetCustomerMerchantDetailParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)

	require.Equal(t, user.ID, detail.UserID)
	require.Equal(t, int32(3), detail.TotalOrders)
	require.Equal(t, int64(60000), detail.TotalAmount)
	require.Equal(t, int32(20000), detail.AvgOrderAmount)
	require.NotNil(t, detail.FirstOrderAt)
	require.NotNil(t, detail.LastOrderAt)
}

func TestGetCustomerMerchantDetail_NotFound(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t) // 没有在这个商户下单过

	_, err := testStore.GetCustomerMerchantDetail(context.Background(), GetCustomerMerchantDetailParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.Error(t, err)
}
