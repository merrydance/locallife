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

func createRandomOrderItem(t *testing.T, orderID int64) OrderItem {
	// 获取订单以获得商户ID
	order, err := testStore.GetOrder(context.Background(), orderID)
	require.NoError(t, err)

	// 创建真实的菜品，避免外键约束错误
	merchant, err := testStore.GetMerchant(context.Background(), order.MerchantID)
	require.NoError(t, err)

	category := createRandomDishCategory(t)
	dish, err := testStore.CreateDish(context.Background(), CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        util.RandomString(10),
		Price:       util.RandomMoney(),
		IsAvailable: true,
		IsOnline:    true,
	})
	require.NoError(t, err)

	arg := CreateOrderItemParams{
		OrderID:   orderID,
		DishID:    pgtype.Int8{Int64: dish.ID, Valid: true},
		Name:      dish.Name,
		UnitPrice: dish.Price,
		Quantity:  int16(util.RandomInt(1, 5)),
		Subtotal:  util.RandomMoney(),
	}

	item, err := testStore.CreateOrderItem(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, item)

	require.Equal(t, arg.OrderID, item.OrderID)
	require.Equal(t, arg.Name, item.Name)
	require.Equal(t, arg.UnitPrice, item.UnitPrice)
	require.Equal(t, arg.Quantity, item.Quantity)
	require.NotZero(t, item.ID)

	return item
}

// ==================== Order Item Tests ====================

func TestCreateOrderItem(t *testing.T) {
	order := createRandomOrder(t)
	createRandomOrderItem(t, order.ID)
}

func TestGetOrderItem(t *testing.T) {
	order := createRandomOrder(t)
	item1 := createRandomOrderItem(t, order.ID)

	item2, err := testStore.GetOrderItem(context.Background(), item1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, item2)

	require.Equal(t, item1.ID, item2.ID)
	require.Equal(t, item1.OrderID, item2.OrderID)
	require.Equal(t, item1.Name, item2.Name)
}

func TestListOrderItemsByOrder(t *testing.T) {
	order := createRandomOrder(t)

	// 创建多个订单项
	for i := 0; i < 3; i++ {
		createRandomOrderItem(t, order.ID)
	}

	items, err := testStore.ListOrderItemsByOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(items), 3)

	for _, item := range items {
		require.Equal(t, order.ID, item.OrderID)
	}
}

func TestCreateOrderItemWithCombo(t *testing.T) {
	order := createRandomOrder(t)
	// 创建真实的 combo_set 来满足外键约束
	combo := createRandomComboSet(t, order.MerchantID)

	arg := CreateOrderItemParams{
		OrderID:   order.ID,
		ComboID:   pgtype.Int8{Int64: combo.ID, Valid: true},
		Name:      "Test Combo",
		UnitPrice: 5000,
		Quantity:  1,
		Subtotal:  5000,
	}

	item, err := testStore.CreateOrderItem(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, item)

	require.False(t, item.DishID.Valid)
	require.True(t, item.ComboID.Valid)
	require.Equal(t, "Test Combo", item.Name)
}

func TestCreateOrderItemWithCustomizations(t *testing.T) {
	order := createRandomOrder(t)
	// 创建真实的 dish 来满足外键约束
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, order.MerchantID, category.ID)

	customizations := []byte(`[{"name":"辣度","value":"微辣","extra_price":0}]`)

	arg := CreateOrderItemParams{
		OrderID:        order.ID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Name:           "Test Dish",
		UnitPrice:      2000,
		Quantity:       2,
		Subtotal:       4000,
		Customizations: customizations,
	}

	item, err := testStore.CreateOrderItem(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, item)

	require.NotNil(t, item.Customizations)
}

// ==================== Payment Order Tests ====================

func createRandomPaymentOrder(t *testing.T, userID int64) PaymentOrder {
	return createRandomPaymentOrderWithOrder(t, userID, nil)
}

func createRandomPaymentOrderWithOrder(t *testing.T, userID int64, orderID *int64) PaymentOrder {
	outTradeNo := util.RandomString(32)

	arg := CreatePaymentOrderParams{
		UserID:       userID,
		PaymentType:  "miniprogram",
		BusinessType: "order",
		Amount:       util.RandomMoney(),
		OutTradeNo:   outTradeNo,
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
	}

	if orderID != nil {
		arg.OrderID = pgtype.Int8{Int64: *orderID, Valid: true}
	}

	payment, err := testStore.CreatePaymentOrder(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, payment)

	require.Equal(t, arg.OutTradeNo, payment.OutTradeNo)
	require.Equal(t, arg.UserID, payment.UserID)
	require.Equal(t, arg.Amount, payment.Amount)
	require.Equal(t, "pending", payment.Status) // default status
	require.NotZero(t, payment.ID)
	require.NotZero(t, payment.CreatedAt)

	return payment
}

func TestCreatePaymentOrder(t *testing.T) {
	user := createRandomUser(t)
	createRandomPaymentOrder(t, user.ID)
}

func TestGetPaymentOrder(t *testing.T) {
	user := createRandomUser(t)
	payment1 := createRandomPaymentOrder(t, user.ID)

	payment2, err := testStore.GetPaymentOrder(context.Background(), payment1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, payment2)

	require.Equal(t, payment1.ID, payment2.ID)
	require.Equal(t, payment1.OutTradeNo, payment2.OutTradeNo)
	require.Equal(t, payment1.UserID, payment2.UserID)
	require.Equal(t, payment1.Amount, payment2.Amount)
}

func TestGetPaymentOrderByOutTradeNo(t *testing.T) {
	user := createRandomUser(t)
	payment1 := createRandomPaymentOrder(t, user.ID)

	payment2, err := testStore.GetPaymentOrderByOutTradeNo(context.Background(), payment1.OutTradeNo)
	require.NoError(t, err)
	require.NotEmpty(t, payment2)

	require.Equal(t, payment1.ID, payment2.ID)
	require.Equal(t, payment1.OutTradeNo, payment2.OutTradeNo)
}

func TestListPaymentOrdersByUser(t *testing.T) {
	user := createRandomUser(t)

	// 创建多个支付订单
	for i := 0; i < 3; i++ {
		createRandomPaymentOrder(t, user.ID)
	}

	arg := ListPaymentOrdersByUserParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	}

	payments, err := testStore.ListPaymentOrdersByUser(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(payments), 3)

	for _, payment := range payments {
		require.Equal(t, user.ID, payment.UserID)
	}
}

func TestUpdatePaymentOrderToPaid(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)
	require.Equal(t, "pending", payment.Status)

	transactionID := util.RandomString(32)
	arg := UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: transactionID, Valid: true},
	}

	updatedPayment, err := testStore.UpdatePaymentOrderToPaid(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedPayment)

	require.Equal(t, payment.ID, updatedPayment.ID)
	require.Equal(t, "paid", updatedPayment.Status)
	require.True(t, updatedPayment.TransactionID.Valid)
	require.Equal(t, transactionID, updatedPayment.TransactionID.String)
	require.True(t, updatedPayment.PaidAt.Valid)
}

func TestUpdatePaymentOrderToClosed(t *testing.T) {
	user := createRandomUser(t)
	payment := createRandomPaymentOrder(t, user.ID)
	require.Equal(t, "pending", payment.Status)

	updatedPayment, err := testStore.UpdatePaymentOrderToClosed(context.Background(), payment.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updatedPayment)

	require.Equal(t, payment.ID, updatedPayment.ID)
	require.Equal(t, "closed", updatedPayment.Status)
}

func TestGetPaymentOrdersByOrder(t *testing.T) {
	order := createRandomOrder(t)

	// 创建关联到订单的支付单
	outTradeNo := util.RandomString(32)
	arg := CreatePaymentOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		UserID:       order.UserID,
		PaymentType:  "miniprogram",
		BusinessType: "order",
		Amount:       order.TotalAmount,
		OutTradeNo:   outTradeNo,
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
	}

	_, err := testStore.CreatePaymentOrder(context.Background(), arg)
	require.NoError(t, err)

	payments, err := testStore.GetPaymentOrdersByOrder(context.Background(), pgtype.Int8{Int64: order.ID, Valid: true})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(payments), 1)

	for _, payment := range payments {
		require.True(t, payment.OrderID.Valid)
		require.Equal(t, order.ID, payment.OrderID.Int64)
	}
}

func TestGetLatestPaymentOrderByOrder(t *testing.T) {
	order := createRandomOrder(t)

	// 创建多个关联到订单的支付单
	for i := 0; i < 2; i++ {
		outTradeNo := util.RandomString(32)
		arg := CreatePaymentOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			UserID:       order.UserID,
			PaymentType:  "miniprogram",
			BusinessType: "order",
			Amount:       order.TotalAmount,
			OutTradeNo:   outTradeNo,
			ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
		}

		_, err := testStore.CreatePaymentOrder(context.Background(), arg)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // 确保时间不同
	}

	latestPayment, err := testStore.GetLatestPaymentOrderByOrder(context.Background(), pgtype.Int8{Int64: order.ID, Valid: true})
	require.NoError(t, err)
	require.NotEmpty(t, latestPayment)

	require.Equal(t, order.ID, latestPayment.OrderID.Int64)
}

func TestGetPaymentOrderForUpdate(t *testing.T) {
	user := createRandomUser(t)
	payment1 := createRandomPaymentOrder(t, user.ID)

	payment2, err := testStore.GetPaymentOrderForUpdate(context.Background(), payment1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, payment2)

	require.Equal(t, payment1.ID, payment2.ID)
	require.Equal(t, payment1.OutTradeNo, payment2.OutTradeNo)
}
