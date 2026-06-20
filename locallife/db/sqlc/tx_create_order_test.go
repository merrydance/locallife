package db

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// ==================== CreateOrderTx Transaction Tests ====================

func TestCreateOrderTx(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 准备订单参数
	orderNo := util.RandomString(20)
	orderParams := CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:             orderNo,
			UserID:              user.ID,
			MerchantID:          merchant.ID,
			OrderType:           "takeaway",
			DeliveryFee:         0,
			Subtotal:            5000,
			DiscountAmount:      0,
			DeliveryFeeDiscount: 0,
			TotalAmount:         5000,
			Status:              "pending",
			Notes:               pgtype.Text{String: "Test order", Valid: true},
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "Test Dish 1",
				UnitPrice: 2000,
				Quantity:  2,
				Subtotal:  4000,
			},
			{
				DishID:    pgtype.Int8{Int64: 2, Valid: true},
				Name:      "Test Dish 2",
				UnitPrice: 1000,
				Quantity:  1,
				Subtotal:  1000,
			},
		},
	}

	// 执行事务
	result, err := testStore.CreateOrderTx(context.Background(), orderParams)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证订单
	require.NotZero(t, result.Order.ID)
	require.Equal(t, orderNo, result.Order.OrderNo)
	require.Equal(t, user.ID, result.Order.UserID)
	require.Equal(t, merchant.ID, result.Order.MerchantID)
	require.Equal(t, "takeaway", result.Order.OrderType)
	require.Equal(t, int64(5000), result.Order.TotalAmount)
	require.Equal(t, "pending", result.Order.Status)

	// 验证订单项
	require.Len(t, result.Items, 2)

	// 验证第一个订单项
	require.Equal(t, result.Order.ID, result.Items[0].OrderID)
	require.Equal(t, "Test Dish 1", result.Items[0].Name)
	require.Equal(t, int64(2000), result.Items[0].UnitPrice)
	require.Equal(t, int16(2), result.Items[0].Quantity)
	require.Equal(t, int64(4000), result.Items[0].Subtotal)

	// 验证第二个订单项
	require.Equal(t, result.Order.ID, result.Items[1].OrderID)
	require.Equal(t, "Test Dish 2", result.Items[1].Name)
	require.Equal(t, int64(1000), result.Items[1].UnitPrice)
	require.Equal(t, int16(1), result.Items[1].Quantity)
	require.Equal(t, int64(1000), result.Items[1].Subtotal)

	// 验证数据库中的订单
	dbOrder, err := testStore.GetOrder(context.Background(), result.Order.ID)
	require.NoError(t, err)
	require.Equal(t, result.Order.ID, dbOrder.ID)
	require.Equal(t, result.Order.OrderNo, dbOrder.OrderNo)

	// 验证数据库中的订单项
	dbItems, err := testStore.ListOrderItemsByOrder(context.Background(), result.Order.ID)
	require.NoError(t, err)
	require.Len(t, dbItems, 2)
}

func TestCreateOrderTxWithCombo(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	orderNo := util.RandomString(20)
	orderParams := CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:             orderNo,
			UserID:              user.ID,
			MerchantID:          merchant.ID,
			OrderType:           "dine_in",
			DeliveryFee:         0,
			Subtotal:            8000,
			DiscountAmount:      0,
			DeliveryFeeDiscount: 0,
			TotalAmount:         8000,
			Status:              "pending",
		},
		Items: []CreateOrderItemParams{
			{
				ComboID:   pgtype.Int8{Int64: 1, Valid: true},
				Name:      "Test Combo",
				UnitPrice: 5000,
				Quantity:  1,
				Subtotal:  5000,
			},
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "Extra Dish",
				UnitPrice: 3000,
				Quantity:  1,
				Subtotal:  3000,
			},
		},
	}

	result, err := testStore.CreateOrderTx(context.Background(), orderParams)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证订单
	require.Equal(t, "dine_in", result.Order.OrderType)
	require.Equal(t, int64(8000), result.Order.TotalAmount)

	// 验证订单项
	require.Len(t, result.Items, 2)

	// 验证套餐项
	require.True(t, result.Items[0].ComboID.Valid)
	require.False(t, result.Items[0].DishID.Valid)
	require.Equal(t, "Test Combo", result.Items[0].Name)

	// 验证菜品项
	require.True(t, result.Items[1].DishID.Valid)
	require.False(t, result.Items[1].ComboID.Valid)
	require.Equal(t, "Extra Dish", result.Items[1].Name)
}

func TestCreateOrderTxWithCustomizations(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	customizations := []byte(`[{"name":"辣度","value":"微辣","extra_price":0},{"name":"加料","value":"加蛋","extra_price":200}]`)

	orderNo := util.RandomString(20)
	orderParams := CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:             orderNo,
			UserID:              user.ID,
			MerchantID:          merchant.ID,
			OrderType:           "takeaway",
			DeliveryFee:         0,
			Subtotal:            2200, // 2000 + 200 extra
			DiscountAmount:      0,
			DeliveryFeeDiscount: 0,
			TotalAmount:         2200,
			Status:              "pending",
		},
		Items: []CreateOrderItemParams{
			{
				DishID:         pgtype.Int8{Int64: 1, Valid: true},
				Name:           "Test Dish with Customizations",
				UnitPrice:      2200,
				Quantity:       1,
				Subtotal:       2200,
				Customizations: customizations,
			},
		},
	}

	result, err := testStore.CreateOrderTx(context.Background(), orderParams)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证订单项的定制选项
	require.Len(t, result.Items, 1)
	require.NotNil(t, result.Items[0].Customizations)
}

func TestCreateOrderTxTakeoutOrder(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	address := createRandomUserAddress(t, user)

	orderNo := util.RandomString(20)
	orderParams := CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:             orderNo,
			UserID:              user.ID,
			MerchantID:          merchant.ID,
			OrderType:           "takeout",
			AddressID:           pgtype.Int8{Int64: address.ID, Valid: true},
			DeliveryFee:         500,
			DeliveryDistance:    pgtype.Int4{Int32: 3000, Valid: true},
			Subtotal:            5000,
			DiscountAmount:      0,
			DeliveryFeeDiscount: 0,
			TotalAmount:         5500,
			Status:              "pending",
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "Takeout Dish",
				UnitPrice: 5000,
				Quantity:  1,
				Subtotal:  5000,
			},
		},
	}

	result, err := testStore.CreateOrderTx(context.Background(), orderParams)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证外卖订单特有字段
	require.Equal(t, "takeout", result.Order.OrderType)
	require.True(t, result.Order.AddressID.Valid)
	require.Equal(t, address.ID, result.Order.AddressID.Int64)
	require.Equal(t, int64(500), result.Order.DeliveryFee)
	require.True(t, result.Order.DeliveryDistance.Valid)
	require.Equal(t, int32(3000), result.Order.DeliveryDistance.Int32)
	require.Equal(t, int64(5500), result.Order.TotalAmount)
}

func TestCreateOrderTxPackagingSnapshotPersistsFeeAndItems(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	option, err := testStore.CreateMerchantPackagingOption(ctx, CreateMerchantPackagingOptionParams{
		MerchantID: merchant.ID,
		Name:       "环保餐盒",
		Price:      150,
		IsEnabled:  true,
	})
	require.NoError(t, err)

	result, err := testStore.CreateOrderTx(ctx, CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:           util.RandomString(20),
			UserID:            user.ID,
			MerchantID:        merchant.ID,
			OrderType:         OrderTypeTakeaway,
			Subtotal:          5000,
			PackagingFee:      150,
			TotalAmount:       5150,
			Status:            OrderStatusPending,
			FulfillmentStatus: FulfillmentStatusScheduled,
		},
		Items: []CreateOrderItemParams{{
			DishID:    pgtype.Int8{Int64: 1, Valid: true},
			Name:      "打包菜品",
			UnitPrice: 5000,
			Quantity:  1,
			Subtotal:  5000,
		}},
		PackagingItems: []CreateOrderPackagingItemParams{{
			PackagingOptionID: pgtype.Int8{Int64: option.ID, Valid: true},
			Name:              option.Name,
			UnitPrice:         option.Price,
			Quantity:          1,
			Subtotal:          option.Price,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, int64(150), result.Order.PackagingFee)
	require.Len(t, result.PackagingItems, 1)
	require.Equal(t, result.Order.ID, result.PackagingItems[0].OrderID)
	require.Equal(t, option.ID, result.PackagingItems[0].PackagingOptionID.Int64)
	require.Equal(t, "环保餐盒", result.PackagingItems[0].Name)
	require.Equal(t, int64(150), result.PackagingItems[0].Subtotal)

	persisted, err := testStore.GetOrder(ctx, result.Order.ID)
	require.NoError(t, err)
	require.Equal(t, int64(150), persisted.PackagingFee)
	snapshots, err := testStore.ListOrderPackagingItems(ctx, result.Order.ID)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.Equal(t, result.PackagingItems[0].ID, snapshots[0].ID)
}

func TestCreateOrderTxPackagingSnapshotRollbackOnError(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	orderNo := util.RandomString(20)

	_, err := testStore.CreateOrderTx(ctx, CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:           orderNo,
			UserID:            user.ID,
			MerchantID:        merchant.ID,
			OrderType:         OrderTypeTakeaway,
			Subtotal:          5000,
			PackagingFee:      150,
			TotalAmount:       5150,
			Status:            OrderStatusPending,
			FulfillmentStatus: FulfillmentStatusScheduled,
		},
		Items: []CreateOrderItemParams{{
			DishID:    pgtype.Int8{Int64: 1, Valid: true},
			Name:      "打包菜品",
			UnitPrice: 5000,
			Quantity:  1,
			Subtotal:  5000,
		}},
		PackagingItems: []CreateOrderPackagingItemParams{{
			Name:      "",
			UnitPrice: 150,
			Quantity:  1,
			Subtotal:  150,
		}},
	})
	require.Error(t, err)

	_, getErr := testStore.GetOrderByOrderNo(ctx, orderNo)
	require.ErrorIs(t, getErr, ErrRecordNotFound)
}

func TestCreateOrderTxPackagingIdempotencyReplayReturnsSnapshot(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	option, err := testStore.CreateMerchantPackagingOption(ctx, CreateMerchantPackagingOptionParams{
		MerchantID: merchant.ID,
		Name:       "保温袋",
		Price:      200,
		IsEnabled:  true,
	})
	require.NoError(t, err)
	idempotencyKey := "packaging-order-" + util.RandomString(12)
	requestHash := "sha256:" + util.RandomString(64)

	createParams := func(name string, price int64, hash string) CreateOrderTxParams {
		return CreateOrderTxParams{
			CreateOrderParams: CreateOrderParams{
				OrderNo:           util.RandomString(20),
				UserID:            user.ID,
				MerchantID:        merchant.ID,
				OrderType:         OrderTypeTakeaway,
				Subtotal:          5000,
				PackagingFee:      price,
				TotalAmount:       5000 + price,
				Status:            OrderStatusPending,
				FulfillmentStatus: FulfillmentStatusScheduled,
			},
			Items: []CreateOrderItemParams{{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "自提菜品",
				UnitPrice: 5000,
				Quantity:  1,
				Subtotal:  5000,
			}},
			PackagingItems: []CreateOrderPackagingItemParams{{
				PackagingOptionID: pgtype.Int8{Int64: option.ID, Valid: true},
				Name:              name,
				UnitPrice:         price,
				Quantity:          1,
				Subtotal:          price,
			}},
			IdempotencyOperationScope: "customer_order_create",
			IdempotencyActorUserID:    user.ID,
			IdempotencyKey:            idempotencyKey,
			IdempotencyRequestHash:    hash,
		}
	}

	first, err := testStore.CreateOrderTx(ctx, createParams("保温袋", 200, requestHash))
	require.NoError(t, err)
	require.False(t, first.IdempotencyReplayed)
	require.Len(t, first.PackagingItems, 1)

	replayed, err := testStore.CreateOrderTx(ctx, createParams("当前已改名", 999, "sha256:"+util.RandomString(64)))
	require.NoError(t, err)
	require.True(t, replayed.IdempotencyReplayed)
	require.Equal(t, first.Order.ID, replayed.Order.ID)
	require.Equal(t, int64(200), replayed.Order.PackagingFee)
	require.Len(t, replayed.PackagingItems, 1)
	require.Equal(t, first.PackagingItems[0].ID, replayed.PackagingItems[0].ID)
	require.Equal(t, "保温袋", replayed.PackagingItems[0].Name)
	require.Equal(t, int64(200), replayed.PackagingItems[0].Subtotal)

	snapshots, err := testStore.ListOrderPackagingItems(ctx, first.Order.ID)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
}

func TestCreateOrderTx_RequestIdempotencyReplayBoundOrderAndConflictBeforeBinding(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	address := createRandomUserAddress(t, user)

	items := []CreateOrderItemParams{{
		DishID:    pgtype.Int8{Int64: 1, Valid: true},
		Name:      "Idempotent Takeout Dish",
		UnitPrice: 2500,
		Quantity:  2,
		Subtotal:  5000,
	}}
	idempotencyKey := "takeout-order-" + util.RandomString(12)
	requestHash := "sha256:" + util.RandomString(64)

	createParams := func(orderNo string, total int64, hash string) CreateOrderTxParams {
		return CreateOrderTxParams{
			CreateOrderParams: CreateOrderParams{
				OrderNo:             orderNo,
				UserID:              user.ID,
				MerchantID:          merchant.ID,
				OrderType:           OrderTypeTakeout,
				AddressID:           pgtype.Int8{Int64: address.ID, Valid: true},
				DeliveryFee:         500,
				DeliveryDistance:    pgtype.Int4{Int32: 3000, Valid: true},
				Subtotal:            total - 500,
				DiscountAmount:      0,
				DeliveryFeeDiscount: 0,
				TotalAmount:         total,
				Status:              OrderStatusPending,
			},
			Items:                     items,
			IdempotencyOperationScope: "customer_order_create",
			IdempotencyActorUserID:    user.ID,
			IdempotencyKey:            idempotencyKey,
			IdempotencyRequestHash:    hash,
		}
	}

	first, err := testStore.CreateOrderTx(ctx, createParams(util.RandomString(20), 5500, requestHash))
	require.NoError(t, err)
	require.False(t, first.IdempotencyReplayed)

	replayed, err := testStore.CreateOrderTx(ctx, createParams(util.RandomString(20), 5500, requestHash))
	require.NoError(t, err)
	require.True(t, replayed.IdempotencyReplayed)
	require.Equal(t, first.Order.ID, replayed.Order.ID)
	require.Equal(t, first.Order.OrderNo, replayed.Order.OrderNo)
	require.Len(t, replayed.Items, 1)
	require.Equal(t, first.Items[0].ID, replayed.Items[0].ID)

	boundDifferentHashReplay, err := testStore.CreateOrderTx(ctx, createParams(util.RandomString(20), 6500, "sha256:"+util.RandomString(64)))
	require.NoError(t, err)
	require.True(t, boundDifferentHashReplay.IdempotencyReplayed)
	require.Equal(t, first.Order.ID, boundDifferentHashReplay.Order.ID)

	unboundKey := "takeout-order-unbound-" + util.RandomString(12)
	unboundHash := "sha256:" + util.RandomString(64)
	_, err = testStore.CreateOrderRequestIdempotency(ctx, CreateOrderRequestIdempotencyParams{
		OperationScope: "customer_order_create",
		ActorUserID:    user.ID,
		IdempotencyKey: unboundKey,
		RequestHash:    unboundHash,
	})
	require.NoError(t, err)

	conflictParams := createParams(util.RandomString(20), 6500, "sha256:"+util.RandomString(64))
	conflictParams.IdempotencyKey = unboundKey
	_, err = testStore.CreateOrderTx(ctx, conflictParams)
	require.Error(t, err)
	statusCode, ok := IsTxRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusConflict, statusCode)

	orders, err := testStore.ListOrdersByUser(ctx, ListOrdersByUserParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	var matchingOrders []ListOrdersByUserRow
	for _, order := range orders {
		if order.MerchantID == merchant.ID && order.OrderType == OrderTypeTakeout {
			matchingOrders = append(matchingOrders, order)
		}
	}
	require.Len(t, matchingOrders, 1)
	require.Equal(t, first.Order.ID, matchingOrders[0].ID)
}

func TestCreateOrderTx_ConcurrentSameIdempotencyKeyCreatesSingleOrder(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	address := createRandomUserAddress(t, user)
	idempotencyKey := "takeout-concurrent-" + util.RandomString(12)
	requestHash := "sha256:" + util.RandomString(64)

	createParams := func() CreateOrderTxParams {
		return CreateOrderTxParams{
			CreateOrderParams: CreateOrderParams{
				OrderNo:             util.RandomString(20),
				UserID:              user.ID,
				MerchantID:          merchant.ID,
				OrderType:           OrderTypeTakeout,
				AddressID:           pgtype.Int8{Int64: address.ID, Valid: true},
				DeliveryFee:         500,
				DeliveryDistance:    pgtype.Int4{Int32: 3000, Valid: true},
				Subtotal:            5000,
				DiscountAmount:      0,
				DeliveryFeeDiscount: 0,
				TotalAmount:         5500,
				Status:              OrderStatusPending,
			},
			Items: []CreateOrderItemParams{{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "Concurrent Idempotent Takeout",
				UnitPrice: 5000,
				Quantity:  1,
				Subtotal:  5000,
			}},
			IdempotencyOperationScope: "customer_order_create",
			IdempotencyActorUserID:    user.ID,
			IdempotencyKey:            idempotencyKey,
			IdempotencyRequestHash:    requestHash,
		}
	}

	type createResult struct {
		result CreateOrderTxResult
		err    error
	}
	results := make(chan createResult, 2)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := testStore.CreateOrderTx(ctx, createParams())
			results <- createResult{result: result, err: err}
		}()
	}
	wg.Wait()
	close(results)

	var created []CreateOrderTxResult
	for result := range results {
		require.NoError(t, result.err)
		created = append(created, result.result)
	}
	require.Len(t, created, 2)
	require.Equal(t, created[0].Order.ID, created[1].Order.ID)
	require.NotEqual(t, created[0].IdempotencyReplayed, created[1].IdempotencyReplayed)

	orders, err := testStore.ListOrdersByUser(ctx, ListOrdersByUserParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	var matchingOrders []ListOrdersByUserRow
	for _, order := range orders {
		if order.MerchantID == merchant.ID && order.OrderType == OrderTypeTakeout {
			matchingOrders = append(matchingOrders, order)
		}
	}
	require.Len(t, matchingOrders, 1)
	require.Equal(t, created[0].Order.ID, matchingOrders[0].ID)
}

func TestCreateOrderTxAllocatesMerchantDailyPickupCode(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	address := createRandomUserAddress(t, user)
	pickupTime := time.Date(2026, 4, 1, 10, 0, 0, 0, time.Local)

	newOrderParams := func(orderNo string, at time.Time) CreateOrderTxParams {
		return CreateOrderTxParams{
			CreateOrderParams: CreateOrderParams{
				OrderNo:     orderNo,
				UserID:      user.ID,
				MerchantID:  merchant.ID,
				OrderType:   OrderTypeTakeout,
				AddressID:   pgtype.Int8{Int64: address.ID, Valid: true},
				DeliveryFee: 200,
				Subtotal:    1800,
				TotalAmount: 2000,
				Status:      OrderStatusPending,
				PickupCode:  pgtype.Text{},
			},
			Items: []CreateOrderItemParams{{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "测试菜品",
				UnitPrice: 1800,
				Quantity:  1,
				Subtotal:  1800,
			}},
			PickupTime: at,
		}
	}

	first, err := testStore.CreateOrderTx(context.Background(), newOrderParams(util.RandomString(20), pickupTime))
	require.NoError(t, err)
	require.True(t, first.Order.PickupCode.Valid)
	require.Equal(t, "0001", first.Order.PickupCode.String)

	second, err := testStore.CreateOrderTx(context.Background(), newOrderParams(util.RandomString(20), pickupTime.Add(2*time.Hour)))
	require.NoError(t, err)
	require.True(t, second.Order.PickupCode.Valid)
	require.Equal(t, "0002", second.Order.PickupCode.String)

	third, err := testStore.CreateOrderTx(context.Background(), newOrderParams(util.RandomString(20), pickupTime.Add(24*time.Hour)))
	require.NoError(t, err)
	require.True(t, third.Order.PickupCode.Valid)
	require.Equal(t, "0001", third.Order.PickupCode.String)
}

func TestCreateOrderTxAllocatesPickupCodeForDineInOrder(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	pickupTime := time.Date(2026, 4, 1, 10, 0, 0, 0, time.Local)

	result, err := testStore.CreateOrderTx(context.Background(), CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:     util.RandomString(20),
			UserID:      user.ID,
			MerchantID:  merchant.ID,
			OrderType:   OrderTypeDineIn,
			Subtotal:    1800,
			TotalAmount: 1800,
			Status:      OrderStatusPending,
			PickupCode:  pgtype.Text{},
		},
		Items: []CreateOrderItemParams{{
			DishID:    pgtype.Int8{Int64: 1, Valid: true},
			Name:      "堂食菜品",
			UnitPrice: 1800,
			Quantity:  1,
			Subtotal:  1800,
		}},
		PickupTime: pickupTime,
	})
	require.NoError(t, err)
	require.True(t, result.Order.PickupCode.Valid)
	require.Equal(t, "0001", result.Order.PickupCode.String)
}

func TestCreateOrderTxEmptyItems(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	orderNo := util.RandomString(20)
	orderParams := CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:             orderNo,
			UserID:              user.ID,
			MerchantID:          merchant.ID,
			OrderType:           "takeaway",
			DeliveryFee:         0,
			Subtotal:            0,
			DiscountAmount:      0,
			DeliveryFeeDiscount: 0,
			TotalAmount:         0,
			Status:              "pending",
		},
		Items: []CreateOrderItemParams{}, // 空订单项
	}

	// 即使没有订单项，事务也应该成功（实际业务中会在 API 层验证）
	result, err := testStore.CreateOrderTx(context.Background(), orderParams)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	require.NotZero(t, result.Order.ID)
	require.Len(t, result.Items, 0)
}

func TestCreateOrderTxRejectsDuplicateActiveReservationOrder(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	room := createRandomRoom(t, merchant.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, room.ID, "confirmed")

	newReservationOrderParams := func(orderNo string) CreateOrderTxParams {
		return CreateOrderTxParams{
			CreateOrderParams: CreateOrderParams{
				OrderNo:       orderNo,
				UserID:        user.ID,
				MerchantID:    merchant.ID,
				OrderType:     OrderTypeReservation,
				ReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
				Subtotal:      5000,
				TotalAmount:   5000,
				Status:        OrderStatusPending,
			},
			EnforceSingleActiveReservationOrder: true,
		}
	}

	first, err := testStore.CreateOrderTx(context.Background(), newReservationOrderParams(util.RandomString(20)))
	require.NoError(t, err)
	require.NotZero(t, first.Order.ID)

	_, err = testStore.CreateOrderTx(context.Background(), newReservationOrderParams(util.RandomString(20)))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrReservationActiveOrderConflict)
	require.Equal(t, "reservation already has an active order", err.Error())

	latestOrder, err := testStore.GetLatestOrderByReservation(context.Background(), pgtype.Int8{Int64: reservation.ID, Valid: true})
	require.NoError(t, err)
	require.Equal(t, first.Order.ID, latestOrder.ID)
	require.False(t, latestOrder.ReplacedByOrderID.Valid)
	require.NotEqual(t, OrderStatusCancelled, latestOrder.Status)
}

// ==================== CreateOrderTx with Voucher Tests ====================

func TestCreateOrderTxWithVoucher(t *testing.T) {
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	// 创建优惠券模板
	voucher := createRandomVoucher(t, merchant.ID)

	// 用户领取优惠券
	userVoucher, err := testStore.ClaimVoucherTx(context.Background(), ClaimVoucherTxParams{
		VoucherID: voucher.ID,
		UserID:    user.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "unused", userVoucher.UserVoucher.Status)

	// 创建使用优惠券的订单
	orderNo := util.RandomString(20)
	voucherAmount := voucher.Amount
	subtotal := int64(10000) // 100元
	totalAmount := subtotal - voucherAmount

	orderParams := CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:             orderNo,
			UserID:              user.ID,
			MerchantID:          merchant.ID,
			OrderType:           "takeaway",
			DeliveryFee:         0,
			Subtotal:            subtotal,
			DiscountAmount:      0,
			VoucherAmount:       voucherAmount,
			DeliveryFeeDiscount: 0,
			TotalAmount:         totalAmount,
			Status:              "pending",
			UserVoucherID:       pgtype.Int8{Int64: userVoucher.UserVoucher.ID, Valid: true},
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "Test Dish",
				UnitPrice: 10000,
				Quantity:  1,
				Subtotal:  10000,
			},
		},
		UserVoucherID: &userVoucher.UserVoucher.ID,
		VoucherAmount: voucherAmount,
	}

	result, err := testStore.CreateOrderTx(context.Background(), orderParams)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证订单
	require.NotZero(t, result.Order.ID)
	require.Equal(t, totalAmount, result.Order.TotalAmount)
	require.Equal(t, voucherAmount, result.Order.VoucherAmount)
	require.True(t, result.Order.UserVoucherID.Valid)
	require.Equal(t, userVoucher.UserVoucher.ID, result.Order.UserVoucherID.Int64)

	// 验证优惠券已被标记为使用
	require.NotNil(t, result.UserVoucher)
	require.Equal(t, "used", result.UserVoucher.Status)
	require.True(t, result.UserVoucher.OrderID.Valid)
	require.Equal(t, result.Order.ID, result.UserVoucher.OrderID.Int64)

	// 从数据库再次确认
	dbVoucher, err := testStore.GetUserVoucher(context.Background(), userVoucher.UserVoucher.ID)
	require.NoError(t, err)
	require.Equal(t, "used", dbVoucher.Status)
}

// ==================== CreateOrderTx with Balance Tests ====================

func TestCreateOrderTxWithBalance(t *testing.T) {
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	// 创建会员并充值
	membership, err := testStore.JoinMembershipTx(context.Background(), JoinMembershipTxParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)

	// 手动增加余额（模拟充值）
	rechargeAmount := int64(50000) // 500元
	membership.Membership, err = testStore.UpdateMembershipBalance(context.Background(), UpdateMembershipBalanceParams{
		ID:               membership.Membership.ID,
		Balance:          rechargeAmount,
		PrincipalBalance: rechargeAmount,
		BonusBalance:     0,
		TotalRecharged:   rechargeAmount,
		TotalConsumed:    0,
	})
	require.NoError(t, err)
	require.Equal(t, rechargeAmount, membership.Membership.Balance)

	// 创建使用余额支付的订单
	orderNo := util.RandomString(20)
	subtotal := int64(20000) // 200元
	balancePaid := subtotal  // 全额余额支付

	orderParams := CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:             orderNo,
			UserID:              user.ID,
			MerchantID:          merchant.ID,
			OrderType:           "dine_in",
			DeliveryFee:         0,
			Subtotal:            subtotal,
			DiscountAmount:      0,
			DeliveryFeeDiscount: 0,
			TotalAmount:         subtotal,
			BalancePaid:         balancePaid,
			MembershipID:        pgtype.Int8{Int64: membership.Membership.ID, Valid: true},
			Status:              "pending",
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "Dine In Dish",
				UnitPrice: 20000,
				Quantity:  1,
				Subtotal:  20000,
			},
		},
		MembershipID: &membership.Membership.ID,
		BalancePaid:  balancePaid,
	}

	result, err := testStore.CreateOrderTx(context.Background(), orderParams)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证订单
	require.NotZero(t, result.Order.ID)
	require.Equal(t, OrderStatusPaid, result.Order.Status)
	require.Equal(t, balancePaid, result.Order.BalancePaid)
	require.True(t, result.Order.MembershipID.Valid)
	require.Equal(t, membership.Membership.ID, result.Order.MembershipID.Int64)
	require.True(t, result.Order.PaymentMethod.Valid)
	require.Equal(t, "balance", result.Order.PaymentMethod.String)
	require.True(t, result.Order.PaidAt.Valid)

	// 验证会员余额已扣减
	require.NotNil(t, result.Membership)
	require.Equal(t, rechargeAmount-balancePaid, result.Membership.Balance)
	require.Equal(t, balancePaid, result.Membership.TotalConsumed)

	// 验证交易记录
	require.NotNil(t, result.Transaction)
	require.Equal(t, "consume", result.Transaction.Type)
	require.Equal(t, -balancePaid, result.Transaction.Amount)
	require.Equal(t, rechargeAmount-balancePaid, result.Transaction.BalanceAfter)
	require.True(t, result.Transaction.RelatedOrderID.Valid)
	require.Equal(t, result.Order.ID, result.Transaction.RelatedOrderID.Int64)

	// 从数据库再次确认余额
	dbMembership, err := testStore.GetMembershipForUpdate(context.Background(), membership.Membership.ID)
	require.NoError(t, err)
	require.Equal(t, rechargeAmount-balancePaid, dbMembership.Balance)
}

func TestCreateOrderTxWithVoucherAndBalance(t *testing.T) {
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	// 创建优惠券并领取
	voucher := createRandomVoucher(t, merchant.ID)
	userVoucher, err := testStore.ClaimVoucherTx(context.Background(), ClaimVoucherTxParams{
		VoucherID: voucher.ID,
		UserID:    user.ID,
	})
	require.NoError(t, err)

	// 创建会员并充值
	membership, err := testStore.JoinMembershipTx(context.Background(), JoinMembershipTxParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)

	rechargeAmount := int64(50000) // 500元
	membership.Membership, err = testStore.UpdateMembershipBalance(context.Background(), UpdateMembershipBalanceParams{
		ID:               membership.Membership.ID,
		Balance:          rechargeAmount,
		PrincipalBalance: rechargeAmount,
		BonusBalance:     0,
		TotalRecharged:   rechargeAmount,
		TotalConsumed:    0,
	})
	require.NoError(t, err)

	// 创建订单，同时使用优惠券和余额
	orderNo := util.RandomString(20)
	subtotal := int64(30000)                      // 300元
	voucherAmount := voucher.Amount               // 优惠券金额
	totalAfterVoucher := subtotal - voucherAmount // 优惠券抵扣后
	balancePaid := totalAfterVoucher              // 余额支付剩余部分

	orderParams := CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:             orderNo,
			UserID:              user.ID,
			MerchantID:          merchant.ID,
			OrderType:           "dine_in",
			DeliveryFee:         0,
			Subtotal:            subtotal,
			DiscountAmount:      0,
			VoucherAmount:       voucherAmount,
			DeliveryFeeDiscount: 0,
			TotalAmount:         totalAfterVoucher,
			BalancePaid:         balancePaid,
			MembershipID:        pgtype.Int8{Int64: membership.Membership.ID, Valid: true},
			UserVoucherID:       pgtype.Int8{Int64: userVoucher.UserVoucher.ID, Valid: true},
			Status:              "pending",
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "Expensive Dish",
				UnitPrice: 30000,
				Quantity:  1,
				Subtotal:  30000,
			},
		},
		UserVoucherID: &userVoucher.UserVoucher.ID,
		VoucherAmount: voucherAmount,
		MembershipID:  &membership.Membership.ID,
		BalancePaid:   balancePaid,
	}

	result, err := testStore.CreateOrderTx(context.Background(), orderParams)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证订单
	require.NotZero(t, result.Order.ID)
	require.Equal(t, totalAfterVoucher, result.Order.TotalAmount)

	// 验证优惠券
	require.NotNil(t, result.UserVoucher)
	require.Equal(t, "used", result.UserVoucher.Status)

	// 验证余额
	require.NotNil(t, result.Membership)
	require.Equal(t, rechargeAmount-balancePaid, result.Membership.Balance)

	// 验证交易记录
	require.NotNil(t, result.Transaction)
	require.Equal(t, -balancePaid, result.Transaction.Amount)
}

func TestCreateOrderTxVoucherAlreadyUsed(t *testing.T) {
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	// 创建优惠券并领取
	voucher := createRandomVoucher(t, merchant.ID)
	userVoucher, err := testStore.ClaimVoucherTx(context.Background(), ClaimVoucherTxParams{
		VoucherID: voucher.ID,
		UserID:    user.ID,
	})
	require.NoError(t, err)

	// 手动将优惠券标记为已使用
	_, err = testStore.MarkUserVoucherAsUsed(context.Background(), MarkUserVoucherAsUsedParams{
		ID:      userVoucher.UserVoucher.ID,
		OrderID: pgtype.Int8{Int64: 999, Valid: true},
	})
	require.NoError(t, err)

	// 尝试用已使用的优惠券创建订单
	orderNo := util.RandomString(20)
	orderParams := CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:       orderNo,
			UserID:        user.ID,
			MerchantID:    merchant.ID,
			OrderType:     "takeaway",
			DeliveryFee:   0,
			Subtotal:      10000,
			TotalAmount:   9000,
			VoucherAmount: 1000,
			UserVoucherID: pgtype.Int8{Int64: userVoucher.UserVoucher.ID, Valid: true},
			Status:        "pending",
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "Test Dish",
				UnitPrice: 10000,
				Quantity:  1,
				Subtotal:  10000,
			},
		},
		UserVoucherID: &userVoucher.UserVoucher.ID,
		VoucherAmount: 1000,
	}

	// 应该失败，因为优惠券已使用
	_, err = testStore.CreateOrderTx(context.Background(), orderParams)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already used")
}

func TestCreateOrderTxRejectsUnavailableVoucherTemplate(t *testing.T) {
	cases := []struct {
		name  string
		block func(t *testing.T, voucher Voucher)
	}{
		{
			name: "inactive",
			block: func(t *testing.T, voucher Voucher) {
				_, err := testStore.UpdateVoucher(context.Background(), UpdateVoucherParams{
					ID:       voucher.ID,
					IsActive: pgtype.Bool{Bool: false, Valid: true},
				})
				require.NoError(t, err)
			},
		},
		{
			name: "deleted",
			block: func(t *testing.T, voucher Voucher) {
				require.NoError(t, testStore.DeleteVoucher(context.Background(), voucher.ID))
			},
		},
		{
			name: "not_started",
			block: func(t *testing.T, voucher Voucher) {
				_, err := testStore.UpdateVoucher(context.Background(), UpdateVoucherParams{
					ID:         voucher.ID,
					ValidFrom:  pgtype.Timestamptz{Time: time.Now().AddDate(0, 1, 0), Valid: true},
					ValidUntil: pgtype.Timestamptz{Time: time.Now().AddDate(0, 2, 0), Valid: true},
				})
				require.NoError(t, err)
			},
		},
		{
			name: "expired",
			block: func(t *testing.T, voucher Voucher) {
				_, err := testStore.UpdateVoucher(context.Background(), UpdateVoucherParams{
					ID:         voucher.ID,
					ValidFrom:  pgtype.Timestamptz{Time: time.Now().AddDate(0, -2, 0), Valid: true},
					ValidUntil: pgtype.Timestamptz{Time: time.Now().AddDate(0, -1, 0), Valid: true},
				})
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := createRandomUser(t)
			merchantOwner := createRandomUser(t)
			merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

			voucher := createRandomVoucher(t, merchant.ID)
			userVoucher, err := testStore.ClaimVoucherTx(context.Background(), ClaimVoucherTxParams{
				VoucherID: voucher.ID,
				UserID:    user.ID,
			})
			require.NoError(t, err)

			tc.block(t, voucher)

			orderNo := util.RandomString(20)
			orderParams := CreateOrderTxParams{
				CreateOrderParams: CreateOrderParams{
					OrderNo:       orderNo,
					UserID:        user.ID,
					MerchantID:    merchant.ID,
					OrderType:     "takeaway",
					DeliveryFee:   0,
					Subtotal:      10000,
					TotalAmount:   9000,
					VoucherAmount: 1000,
					UserVoucherID: pgtype.Int8{Int64: userVoucher.UserVoucher.ID, Valid: true},
					Status:        "pending",
				},
				Items: []CreateOrderItemParams{
					{
						DishID:    pgtype.Int8{Int64: 1, Valid: true},
						Name:      "Test Dish",
						UnitPrice: 10000,
						Quantity:  1,
						Subtotal:  10000,
					},
				},
				UserVoucherID: &userVoucher.UserVoucher.ID,
				VoucherAmount: 1000,
			}

			result, err := testStore.CreateOrderTx(context.Background(), orderParams)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrVoucherTemplateUnavailable)
			require.Empty(t, result)

			dbUserVoucher, err := testStore.GetUserVoucherForUpdate(context.Background(), userVoucher.UserVoucher.ID)
			require.NoError(t, err)
			require.Equal(t, "unused", dbUserVoucher.Status)
			require.False(t, dbUserVoucher.OrderID.Valid)
		})
	}
}

func TestCreateOrderTxInsufficientBalance(t *testing.T) {
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	// 创建会员但不充值（余额为0）
	membership, err := testStore.JoinMembershipTx(context.Background(), JoinMembershipTxParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), membership.Membership.Balance)

	// 尝试用余额支付
	orderNo := util.RandomString(20)
	orderParams := CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:      orderNo,
			UserID:       user.ID,
			MerchantID:   merchant.ID,
			OrderType:    "dine_in",
			DeliveryFee:  0,
			Subtotal:     10000,
			TotalAmount:  10000,
			BalancePaid:  10000,
			MembershipID: pgtype.Int8{Int64: membership.Membership.ID, Valid: true},
			Status:       "pending",
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "Test Dish",
				UnitPrice: 10000,
				Quantity:  1,
				Subtotal:  10000,
			},
		},
		MembershipID: &membership.Membership.ID,
		BalancePaid:  10000,
	}

	// 应该失败，因为余额不足
	_, err = testStore.CreateOrderTx(context.Background(), orderParams)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient balance")
}

func TestCreateOrderTx_BillingGroupAggregation(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	table := createRandomTable(t, merchant.ID)
	session := createOpenDiningSession(t, merchant.ID, table.ID, user.ID, pgtype.Int8{Valid: false})

	billingGroup, err := testStore.CreateBillingGroup(context.Background(), CreateBillingGroupParams{
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		TotalAmount:     9999,
		PaidAmount:      8888,
	})
	require.NoError(t, err)

	orderNo := util.RandomString(20)
	result, err := testStore.CreateOrderTx(context.Background(), CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:             orderNo,
			UserID:              user.ID,
			MerchantID:          merchant.ID,
			OrderType:           "dine_in",
			TableID:             pgtype.Int8{Int64: table.ID, Valid: true},
			DeliveryFee:         0,
			Subtotal:            1280,
			DiscountAmount:      0,
			DeliveryFeeDiscount: 0,
			TotalAmount:         1280,
			Status:              OrderStatusPending,
		},
		Items: []CreateOrderItemParams{{
			DishID:    pgtype.Int8{Int64: 1, Valid: true},
			Name:      "Billing Dish",
			UnitPrice: 1280,
			Quantity:  1,
			Subtotal:  1280,
		}},
		BillingGroupID: &billingGroup.ID,
	})
	require.NoError(t, err)

	orders, err := testStore.ListBillingGroupOrdersByGroup(context.Background(), billingGroup.ID)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	require.Equal(t, result.Order.ID, orders[0].OrderID)
	require.Equal(t, int64(1280), orders[0].Amount)
	require.Equal(t, "linked", orders[0].Status)

	amounts, err := testStore.GetBillingGroupAmounts(context.Background(), billingGroup.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1280), amounts.TotalAmount)
	require.Equal(t, int64(0), amounts.PaidAmount)
}

func TestGetBillingGroupAmounts_ExcludesCancelledAndReplacedOrders(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	table := createRandomTable(t, merchant.ID)
	session := createOpenDiningSession(t, merchant.ID, table.ID, user.ID, pgtype.Int8{Valid: false})

	billingGroup, err := testStore.CreateBillingGroup(context.Background(), CreateBillingGroupParams{
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		TotalAmount:     0,
		PaidAmount:      0,
	})
	require.NoError(t, err)

	pendingOrder := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)
	paidOrder := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)
	cancelledOrder := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)
	replacedOrder := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)

	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(), `
		UPDATE orders
		SET status = $2, total_amount = $3, replaced_by_order_id = $4, updated_at = now()
		WHERE id = $1`, pendingOrder.ID, OrderStatusPending, int64(1000), pgtype.Int8{Valid: false})
	require.NoError(t, err)
	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(), `
		UPDATE orders
		SET status = $2, total_amount = $3, updated_at = now()
		WHERE id = $1`, paidOrder.ID, OrderStatusPaid, int64(2000))
	require.NoError(t, err)
	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(), `
		UPDATE orders
		SET status = $2, total_amount = $3, updated_at = now()
		WHERE id = $1`, cancelledOrder.ID, OrderStatusCancelled, int64(3000))
	require.NoError(t, err)
	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(), `
		UPDATE orders
		SET status = $2, total_amount = $3, replaced_by_order_id = $4, updated_at = now()
		WHERE id = $1`, replacedOrder.ID, OrderStatusPaid, int64(4000), pgtype.Int8{Int64: paidOrder.ID, Valid: true})
	require.NoError(t, err)

	for _, entry := range []struct {
		orderID int64
		amount  int64
	}{
		{orderID: pendingOrder.ID, amount: 1000},
		{orderID: paidOrder.ID, amount: 2000},
		{orderID: cancelledOrder.ID, amount: 3000},
		{orderID: replacedOrder.ID, amount: 4000},
	} {
		_, err = testStore.CreateBillingGroupOrder(context.Background(), CreateBillingGroupOrderParams{
			BillingGroupID: billingGroup.ID,
			OrderID:        entry.orderID,
			Amount:         entry.amount,
			Status:         "linked",
		})
		require.NoError(t, err)
	}

	amounts, err := testStore.GetBillingGroupAmounts(context.Background(), billingGroup.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3000), amounts.TotalAmount)
	require.Equal(t, int64(2000), amounts.PaidAmount)
}

// ==================== ProcessOrderPaymentTx Transaction Tests ====================

// createMerchantWithLocation 创建带有经纬度的商户（用于代取测试）
func createMerchantWithLocation(t *testing.T, ownerID int64) Merchant {
	region := createRandomRegion(t)

	appData, _ := json.Marshal(map[string]string{"test": "data"})
	arg := CreateMerchantParams{
		OwnerUserID: ownerID,
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(50), Valid: true},

		Phone:   "13800138000",
		Address: "北京市朝阳区测试路" + util.RandomString(8), // 添加随机后缀避免重复
		// 设置经纬度（北京朝阳区）
		Latitude:        pgtype.Numeric{Int: big.NewInt(399282), Exp: -4, Valid: true},  // 39.9282
		Longitude:       pgtype.Numeric{Int: big.NewInt(1164507), Exp: -4, Valid: true}, // 116.4507
		Status:          "approved",
		ApplicationData: appData,
		RegionID:        region.ID,
	}

	merchant, err := testStore.CreateMerchant(context.Background(), arg)
	require.NoError(t, err)
	return merchant
}

func TestProcessOrderPaymentTx_TakeoutOrder(t *testing.T) {
	// 创建用户、商户（带经纬度）、地址
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createMerchantWithLocation(t, merchantOwner.ID)
	address := createRandomUserAddress(t, user)

	// 创建外卖订单
	orderNo := util.RandomString(20)
	createResult, err := testStore.CreateOrderTx(context.Background(), CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:          orderNo,
			UserID:           user.ID,
			MerchantID:       merchant.ID,
			OrderType:        "takeout",
			AddressID:        pgtype.Int8{Int64: address.ID, Valid: true},
			DeliveryFee:      800, // 8元运费
			DeliveryDistance: pgtype.Int4{Int32: 2500, Valid: true},
			Subtotal:         5000,
			TotalAmount:      5800,
			Status:           "pending",
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "外卖测试菜品",
				UnitPrice: 5000,
				Quantity:  1,
				Subtotal:  5000,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "takeout", createResult.Order.OrderType)

	// 执行支付处理事务
	result, err := testStore.ProcessOrderPaymentTx(context.Background(), ProcessOrderPaymentTxParams{
		OrderID: createResult.Order.ID,
	})
	require.NoError(t, err)
	require.Equal(t, OrderStatusPaid, result.Order.Status)
	require.Equal(t, FulfillmentStatusPendingKitchen, result.Order.FulfillmentStatus)

	// 验证订单已更新为支付完成且ID一致
	require.Equal(t, createResult.Order.ID, result.Order.ID)

	// ✅ 核心验证：外卖订单支付成功后既不创建代取单，也不立即进入代取池
	require.Nil(t, result.Delivery, "外卖订单支付成功后不应立即创建代取单")
	_, err = testStore.GetDeliveryByOrderID(context.Background(), createResult.Order.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)

	// ✅ 关键调整：支付成功后不立即进入代取池
	require.Nil(t, result.PoolItem, "外卖订单支付成功后不应立即进入代取池")
	_, err = testStore.GetDeliveryPoolByOrderID(context.Background(), createResult.Order.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestProcessOrderPaymentTx_TakeoutOrder_HighDeliveryFee(t *testing.T) {
	// 测试高运费订单的优先级
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createMerchantWithLocation(t, merchantOwner.ID)
	address := createRandomUserAddress(t, user)

	// 创建高运费外卖订单（>=20元 = 优先级3）
	orderNo := util.RandomString(20)
	createResult, err := testStore.CreateOrderTx(context.Background(), CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:          orderNo,
			UserID:           user.ID,
			MerchantID:       merchant.ID,
			OrderType:        "takeout",
			AddressID:        pgtype.Int8{Int64: address.ID, Valid: true},
			DeliveryFee:      2500, // 25元运费
			DeliveryDistance: pgtype.Int4{Int32: 8000, Valid: true},
			Subtotal:         5000,
			TotalAmount:      7500,
			Status:           "pending",
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "远距离外卖",
				UnitPrice: 5000,
				Quantity:  1,
				Subtotal:  5000,
			},
		},
	})
	require.NoError(t, err)

	result, err := testStore.ProcessOrderPaymentTx(context.Background(), ProcessOrderPaymentTxParams{
		OrderID: createResult.Order.ID,
	})
	require.NoError(t, err)
	require.Equal(t, FulfillmentStatusPendingKitchen, result.Order.FulfillmentStatus)

	// 验证高运费订单在支付成功后仍不会立即入池
	require.Nil(t, result.PoolItem)
}

func TestProcessOrderPaymentTx_DineInOrder(t *testing.T) {
	// 堂食订单不应该创建代取单和代取池
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	// 创建堂食订单
	orderNo := util.RandomString(20)
	createResult, err := testStore.CreateOrderTx(context.Background(), CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:     orderNo,
			UserID:      user.ID,
			MerchantID:  merchant.ID,
			OrderType:   "dine_in",
			Subtotal:    3000,
			TotalAmount: 3000,
			Status:      "pending",
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "堂食菜品",
				UnitPrice: 3000,
				Quantity:  1,
				Subtotal:  3000,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "dine_in", createResult.Order.OrderType)

	result, err := testStore.ProcessOrderPaymentTx(context.Background(), ProcessOrderPaymentTxParams{
		OrderID: createResult.Order.ID,
	})
	require.NoError(t, err)
	require.Equal(t, FulfillmentStatusPendingKitchen, result.Order.FulfillmentStatus)

	// ✅ 堂食订单不应该有代取单和代取池
	require.Nil(t, result.Delivery, "堂食订单不应该创建代取单")
	require.Nil(t, result.PoolItem, "堂食订单不应该进入代取池")
}

func TestProcessOrderPaymentTx_TakeawayOrder(t *testing.T) {
	// 外带订单（自取）不应该创建代取单
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	orderNo := util.RandomString(20)
	createResult, err := testStore.CreateOrderTx(context.Background(), CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:     orderNo,
			UserID:      user.ID,
			MerchantID:  merchant.ID,
			OrderType:   "takeaway", // 外带自取
			Subtotal:    3000,
			TotalAmount: 3000,
			Status:      "pending",
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "外带菜品",
				UnitPrice: 3000,
				Quantity:  1,
				Subtotal:  3000,
			},
		},
	})
	require.NoError(t, err)

	result, err := testStore.ProcessOrderPaymentTx(context.Background(), ProcessOrderPaymentTxParams{
		OrderID: createResult.Order.ID,
	})
	require.NoError(t, err)
	require.Equal(t, FulfillmentStatusPendingKitchen, result.Order.FulfillmentStatus)

	// ✅ 外带自取订单不应该有代取单
	require.Nil(t, result.Delivery, "外带订单不应该创建代取单")
	require.Nil(t, result.PoolItem, "外带订单不应该进入代取池")
}

func TestProcessOrderPaymentTx_TakeoutWithoutAddress(t *testing.T) {
	// 外卖订单缺少代取地址应该报错
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	// 创建外卖订单但不设置代取地址
	orderNo := util.RandomString(20)
	createResult, err := testStore.CreateOrderTx(context.Background(), CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:     orderNo,
			UserID:      user.ID,
			MerchantID:  merchant.ID,
			OrderType:   "takeout", // 外卖但无地址
			DeliveryFee: 500,
			Subtotal:    3000,
			TotalAmount: 3500,
			Status:      "pending",
			// AddressID 故意不设置
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "测试菜品",
				UnitPrice: 3000,
				Quantity:  1,
				Subtotal:  3000,
			},
		},
	})
	require.NoError(t, err)

	// 当前实现会先完成支付与库存扣减，代取单在商家后续推进履约时再创建。
	result, err := testStore.ProcessOrderPaymentTx(context.Background(), ProcessOrderPaymentTxParams{
		OrderID: createResult.Order.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "paid", result.Order.Status)
	require.Nil(t, result.Delivery)
	require.Nil(t, result.PoolItem)
}

func TestProcessOrderPaymentTx_TakeoutOrder_MediumDeliveryFee(t *testing.T) {
	// 测试中等运费订单的优先级（>=10元但<20元 = 优先级2）
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createMerchantWithLocation(t, merchantOwner.ID)
	address := createRandomUserAddress(t, user)

	// 创建中等运费外卖订单
	orderNo := util.RandomString(20)
	createResult, err := testStore.CreateOrderTx(context.Background(), CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:          orderNo,
			UserID:           user.ID,
			MerchantID:       merchant.ID,
			OrderType:        "takeout",
			AddressID:        pgtype.Int8{Int64: address.ID, Valid: true},
			DeliveryFee:      1500, // 15元运费
			DeliveryDistance: pgtype.Int4{Int32: 5000, Valid: true},
			Subtotal:         5000,
			TotalAmount:      6500,
			Status:           "pending",
		},
		Items: []CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: 1, Valid: true},
				Name:      "中等距离外卖",
				UnitPrice: 5000,
				Quantity:  1,
				Subtotal:  5000,
			},
		},
	})
	require.NoError(t, err)

	result, err := testStore.ProcessOrderPaymentTx(context.Background(), ProcessOrderPaymentTxParams{
		OrderID: createResult.Order.ID,
	})
	require.NoError(t, err)
	require.Equal(t, FulfillmentStatusPendingKitchen, result.Order.FulfillmentStatus)

	// 验证中等运费订单在支付成功后仍不会立即入池
	require.Nil(t, result.PoolItem)
}
