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

// createRandomOrder 创建一个随机订单（需要先创建用户和商户）
func createRandomOrder(t *testing.T) Order {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	return createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)
}

func createRandomOrderWithUserAndMerchant(t *testing.T, userID, merchantID int64) Order {
	orderNo := util.RandomString(20)

	arg := CreateOrderParams{
		OrderNo:             orderNo,
		UserID:              userID,
		MerchantID:          merchantID,
		OrderType:           "takeaway",
		DeliveryFee:         0,
		Subtotal:            util.RandomMoney(),
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         util.RandomMoney(),
		Status:              "pending",
		Notes:               pgtype.Text{String: "test order", Valid: true},
	}

	order, err := testStore.CreateOrder(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, order)

	require.Equal(t, arg.OrderNo, order.OrderNo)
	require.Equal(t, arg.UserID, order.UserID)
	require.Equal(t, arg.MerchantID, order.MerchantID)
	require.Equal(t, arg.OrderType, order.OrderType)
	require.Equal(t, arg.Status, order.Status)
	require.NotZero(t, order.ID)
	require.NotZero(t, order.CreatedAt)

	return order
}

func createRandomOrderWithStatus(t *testing.T, userID, merchantID int64, status string) Order {
	orderNo := util.RandomString(20)

	arg := CreateOrderParams{
		OrderNo:             orderNo,
		UserID:              userID,
		MerchantID:          merchantID,
		OrderType:           "takeaway",
		DeliveryFee:         0,
		Subtotal:            util.RandomMoney(),
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         util.RandomMoney(),
		Status:              status,
	}

	order, err := testStore.CreateOrder(context.Background(), arg)
	require.NoError(t, err)
	return order
}

// ==================== Order Tests ====================

func TestCreateOrder(t *testing.T) {
	createRandomOrder(t)
}

func TestGetOrder(t *testing.T) {
	order1 := createRandomOrder(t)

	order2, err := testStore.GetOrder(context.Background(), order1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, order2)

	require.Equal(t, order1.ID, order2.ID)
	require.Equal(t, order1.OrderNo, order2.OrderNo)
	require.Equal(t, order1.UserID, order2.UserID)
	require.Equal(t, order1.MerchantID, order2.MerchantID)
	require.Equal(t, order1.Status, order2.Status)
	require.WithinDuration(t, order1.CreatedAt, order2.CreatedAt, time.Second)
}

func TestGetOrderByOrderNo(t *testing.T) {
	order1 := createRandomOrder(t)

	order2, err := testStore.GetOrderByOrderNo(context.Background(), order1.OrderNo)
	require.NoError(t, err)
	require.NotEmpty(t, order2)

	require.Equal(t, order1.ID, order2.ID)
	require.Equal(t, order1.OrderNo, order2.OrderNo)
}

func TestListOrdersByUser(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 创建多个订单
	for i := 0; i < 5; i++ {
		createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)
	}

	arg := ListOrdersByUserParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	}

	orders, err := testStore.ListOrdersByUser(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(orders), 5)

	for _, order := range orders {
		require.Equal(t, user.ID, order.UserID)
	}
}

func TestListOrdersByUserAndStatus(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 创建不同状态的订单
	createRandomOrderWithStatus(t, user.ID, merchant.ID, "pending")
	createRandomOrderWithStatus(t, user.ID, merchant.ID, "pending")
	createRandomOrderWithStatus(t, user.ID, merchant.ID, "paid")

	arg := ListOrdersByUserAndStatusParams{
		UserID: user.ID,
		Status: "pending",
		Limit:  10,
		Offset: 0,
	}

	orders, err := testStore.ListOrdersByUserAndStatus(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(orders), 2)

	for _, order := range orders {
		require.Equal(t, user.ID, order.UserID)
		require.Equal(t, "pending", order.Status)
	}
}

func TestListOrdersByMerchant(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, user.ID)

	// 创建多个订单
	for i := 0; i < 3; i++ {
		createRandomOrderWithUserAndMerchant(t, createRandomUser(t).ID, merchant.ID)
	}

	arg := ListOrdersByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      10,
		Offset:     0,
	}

	orders, err := testStore.ListOrdersByMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(orders), 3)

	for _, order := range orders {
		require.Equal(t, merchant.ID, order.MerchantID)
	}
}

func TestListOrdersByMerchantAndStatus(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, user.ID)
	customer := createRandomUser(t)

	// 创建不同状态的订单
	createRandomOrderWithStatus(t, customer.ID, merchant.ID, "paid")
	createRandomOrderWithStatus(t, customer.ID, merchant.ID, "paid")
	createRandomOrderWithStatus(t, customer.ID, merchant.ID, "preparing")

	arg := ListOrdersByMerchantAndStatusParams{
		MerchantID: merchant.ID,
		Status:     "paid",
		Limit:      10,
		Offset:     0,
	}

	orders, err := testStore.ListOrdersByMerchantAndStatus(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(orders), 2)

	for _, order := range orders {
		require.Equal(t, merchant.ID, order.MerchantID)
		require.Equal(t, "paid", order.Status)
	}
}

func TestListMerchantPromotionOrdersUsesIDTieBreaker(t *testing.T) {
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)
	customer := createRandomUser(t)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)
	startAt := tiedCreatedAt.Add(-time.Minute)
	endAt := tiedCreatedAt.Add(time.Minute)

	createPromotionOrder := func() Order {
		order, err := testStore.CreateOrder(context.Background(), CreateOrderParams{
			OrderNo:             util.RandomString(20),
			UserID:              customer.ID,
			MerchantID:          merchant.ID,
			OrderType:           "takeaway",
			DeliveryFee:         500,
			Subtotal:            3000,
			DiscountAmount:      0,
			DeliveryFeeDiscount: 200,
			TotalAmount:         3300,
			Status:              "completed",
		})
		require.NoError(t, err)
		return order
	}

	firstOrder := createPromotionOrder()
	secondOrder := createPromotionOrder()

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE orders SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{firstOrder.ID, secondOrder.ID},
	)
	require.NoError(t, err)

	orders, err := testStore.ListMerchantPromotionOrders(context.Background(), ListMerchantPromotionOrdersParams{
		MerchantID: merchant.ID,
		StartAt:    startAt,
		EndAt:      endAt,
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, orders, 2)
	require.Equal(t, secondOrder.ID, orders[0].ID)
	require.Equal(t, firstOrder.ID, orders[1].ID)
}

func TestUpdateOrderStatus(t *testing.T) {
	order := createRandomOrder(t)
	require.Equal(t, "pending", order.Status)

	arg := UpdateOrderStatusParams{
		ID:             order.ID,
		Status:         "paid",
		ExpectedStatus: "pending",
	}

	updatedOrder, err := testStore.UpdateOrderStatus(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedOrder)

	require.Equal(t, order.ID, updatedOrder.ID)
	require.Equal(t, "paid", updatedOrder.Status)
}

func TestUpdateOrderToPaid(t *testing.T) {
	order := createRandomOrder(t)
	require.Equal(t, "pending", order.Status)

	arg := UpdateOrderToPaidParams{
		ID:                order.ID,
		PaymentMethod:     pgtype.Text{String: "wechat", Valid: true},
		FulfillmentStatus: pgtype.Text{},
	}

	updatedOrder, err := testStore.UpdateOrderToPaid(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedOrder)

	require.Equal(t, order.ID, updatedOrder.ID)
	require.Equal(t, "paid", updatedOrder.Status)
	require.True(t, updatedOrder.PaymentMethod.Valid)
	require.Equal(t, "wechat", updatedOrder.PaymentMethod.String)
	require.True(t, updatedOrder.PaidAt.Valid)
}

func TestUpdateOrderToCompleted(t *testing.T) {
	order := createRandomOrder(t)

	// 先设置为 paid 状态
	_, err := testStore.UpdateOrderStatus(context.Background(), UpdateOrderStatusParams{
		ID:             order.ID,
		Status:         "paid",
		ExpectedStatus: "pending",
	})
	require.NoError(t, err)

	updatedOrder, err := testStore.UpdateOrderToCompleted(context.Background(), order.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updatedOrder)

	require.Equal(t, order.ID, updatedOrder.ID)
	require.Equal(t, "completed", updatedOrder.Status)
	require.True(t, updatedOrder.CompletedAt.Valid)
}

func TestUpdateOrderToCancelled(t *testing.T) {
	order := createRandomOrder(t)
	require.Equal(t, "pending", order.Status)

	arg := UpdateOrderToCancelledParams{
		ID:             order.ID,
		CancelReason:   pgtype.Text{String: "用户取消", Valid: true},
		ExpectedStatus: "pending",
	}

	updatedOrder, err := testStore.UpdateOrderToCancelled(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedOrder)

	require.Equal(t, order.ID, updatedOrder.ID)
	require.Equal(t, "cancelled", updatedOrder.Status)
	require.True(t, updatedOrder.CancelledAt.Valid)
	require.True(t, updatedOrder.CancelReason.Valid)
	require.Equal(t, "用户取消", updatedOrder.CancelReason.String)
}

func TestCountOrdersByMerchant(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, user.ID)

	// 创建多个订单
	for i := 0; i < 3; i++ {
		createRandomOrderWithUserAndMerchant(t, createRandomUser(t).ID, merchant.ID)
	}

	count, err := testStore.CountOrdersByMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(3))
}

func TestCountOrdersByMerchantAndStatus(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, user.ID)
	customer := createRandomUser(t)

	// 创建不同状态的订单
	createRandomOrderWithStatus(t, customer.ID, merchant.ID, "pending")
	createRandomOrderWithStatus(t, customer.ID, merchant.ID, "pending")
	createRandomOrderWithStatus(t, customer.ID, merchant.ID, "paid")

	arg := CountOrdersByMerchantAndStatusParams{
		MerchantID: merchant.ID,
		Status:     "pending",
	}

	count, err := testStore.CountOrdersByMerchantAndStatus(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(2))
}

func TestListOrdersByMerchantWithFilters_ExcludesPendingWhenStatusOmitted(t *testing.T) {
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)
	customer := createRandomUser(t)

	pendingOrder := createRandomOrderWithStatus(t, customer.ID, merchant.ID, OrderStatusPending)
	paidOrder := createRandomOrderWithStatus(t, customer.ID, merchant.ID, OrderStatusPaid)
	preparingOrder := createRandomOrderWithStatus(t, customer.ID, merchant.ID, OrderStatusPreparing)

	orders, err := testStore.ListOrdersByMerchantWithFilters(context.Background(), ListOrdersByMerchantWithFiltersParams{
		MerchantID: merchant.ID,
		Status:     pgtype.Text{},
		OrderType:  pgtype.Text{},
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)

	orderIDs := make(map[int64]bool, len(orders))
	for _, order := range orders {
		orderIDs[order.ID] = true
		require.NotEqual(t, OrderStatusPending, order.Status)
	}
	require.False(t, orderIDs[pendingOrder.ID])
	require.True(t, orderIDs[paidOrder.ID])
	require.True(t, orderIDs[preparingOrder.ID])

	count, err := testStore.CountOrdersByMerchantWithFilters(context.Background(), CountOrdersByMerchantWithFiltersParams{
		MerchantID: merchant.ID,
		Status:     pgtype.Text{},
		OrderType:  pgtype.Text{},
	})
	require.NoError(t, err)
	require.Equal(t, int64(len(orders)), count)
}

func TestGetOrderForUpdate(t *testing.T) {
	order1 := createRandomOrder(t)

	order2, err := testStore.GetOrderForUpdate(context.Background(), order1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, order2)

	require.Equal(t, order1.ID, order2.ID)
	require.Equal(t, order1.OrderNo, order2.OrderNo)
}

// ==================== Order with Different Types Tests ====================

func TestCreateTakeoutOrder(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	// 先创建用户地址
	address := createRandomUserAddress(t, user)

	orderNo := util.RandomString(20)
	arg := CreateOrderParams{
		OrderNo:             orderNo,
		UserID:              user.ID,
		MerchantID:          merchant.ID,
		OrderType:           "takeout",
		AddressID:           pgtype.Int8{Int64: address.ID, Valid: true},
		DeliveryFee:         500,                                   // 5元代取费
		DeliveryDistance:    pgtype.Int4{Int32: 3000, Valid: true}, // 3公里
		Subtotal:            5000,
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         5500,
		Status:              "pending",
	}

	order, err := testStore.CreateOrder(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, order)

	require.Equal(t, "takeout", order.OrderType)
	require.True(t, order.AddressID.Valid)
	require.Equal(t, address.ID, order.AddressID.Int64)
	require.Equal(t, int64(500), order.DeliveryFee)
	require.True(t, order.DeliveryDistance.Valid)
	require.Equal(t, int32(3000), order.DeliveryDistance.Int32)
}

func TestGetOrderWithDetails_PrefersDeliverySnapshot(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	address := createRandomUserAddress(t, user)

	arg := CreateOrderParams{
		OrderNo:                      util.RandomString(20),
		UserID:                       user.ID,
		MerchantID:                   merchant.ID,
		OrderType:                    OrderTypeTakeout,
		AddressID:                    pgtype.Int8{Int64: address.ID, Valid: true},
		DeliveryContactNameSnapshot:  pgtype.Text{String: address.ContactName, Valid: true},
		DeliveryContactPhoneSnapshot: pgtype.Text{String: address.ContactPhone, Valid: true},
		DeliveryAddressSnapshot:      pgtype.Text{String: address.DetailAddress, Valid: true},
		DeliveryLongitudeSnapshot:    address.Longitude,
		DeliveryLatitudeSnapshot:     address.Latitude,
		DeliveryFee:                  500,
		DeliveryDistance:             pgtype.Int4{Int32: 3000, Valid: true},
		Subtotal:                     5000,
		DiscountAmount:               0,
		DeliveryFeeDiscount:          0,
		TotalAmount:                  5500,
		Status:                       OrderStatusPending,
	}

	order, err := testStore.CreateOrder(context.Background(), arg)
	require.NoError(t, err)

	updatedAddress, err := testStore.UpdateUserAddress(context.Background(), UpdateUserAddressParams{
		ID:            address.ID,
		UserID:        user.ID,
		ContactName:   pgtype.Text{String: "最新地址联系人", Valid: true},
		ContactPhone:  pgtype.Text{String: "13900001111", Valid: true},
		DetailAddress: pgtype.Text{String: "北京市海淀区最新地址", Valid: true},
		Longitude:     numericFromFloat(116.500001),
		Latitude:      numericFromFloat(39.900001),
	})
	require.NoError(t, err)
	require.Equal(t, "最新地址联系人", updatedAddress.ContactName)

	details, err := testStore.GetOrderWithDetails(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, address.ContactName, details.DeliveryContactName)
	require.Equal(t, address.ContactPhone, details.DeliveryContactPhone)
	require.Equal(t, address.DetailAddress, details.DeliveryAddress)
	require.False(t, details.CombinedPaymentID.Valid)
	require.Empty(t, details.CombineOutTradeNo)
	require.True(t, details.DeliveryAddressSnapshot.Valid)
	require.Equal(t, address.DetailAddress, details.DeliveryAddressSnapshot.String)
}

func TestCreateDineInOrder(t *testing.T) {
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)
	// 创建餐桌
	table := createRandomTableForMerchant(t, merchant.ID)

	orderNo := util.RandomString(20)
	arg := CreateOrderParams{
		OrderNo:             orderNo,
		UserID:              user.ID,
		MerchantID:          merchant.ID,
		OrderType:           "dine_in",
		TableID:             pgtype.Int8{Int64: table.ID, Valid: true},
		DeliveryFee:         0,
		Subtotal:            3000,
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         3000,
		Status:              "pending",
	}

	order, err := testStore.CreateOrder(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, order)

	require.Equal(t, "dine_in", order.OrderType)
	require.True(t, order.TableID.Valid)
	require.Equal(t, table.ID, order.TableID.Int64)
	require.Equal(t, int64(0), order.DeliveryFee)
}

// createRandomTableForMerchant 创建随机餐桌
func createRandomTableForMerchant(t *testing.T, merchantID int64) Table {
	arg := CreateTableParams{
		MerchantID:  merchantID,
		TableNo:     util.RandomString(5),
		TableType:   "table", // 有效值: 'table' 或 'room'
		Capacity:    int16(util.RandomInt(2, 10)),
		Description: pgtype.Text{String: "Test table", Valid: true},
		QrCodeUrl:   pgtype.Text{String: util.RandomString(20), Valid: true},
		Status:      "available",
	}

	table, err := testStore.CreateTable(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, table)

	return table
}
