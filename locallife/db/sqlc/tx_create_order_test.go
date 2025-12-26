package db

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

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
		ID:             membership.Membership.ID,
		Balance:        rechargeAmount,
		TotalRecharged: rechargeAmount,
		TotalConsumed:  0,
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
	require.Equal(t, balancePaid, result.Order.BalancePaid)
	require.True(t, result.Order.MembershipID.Valid)
	require.Equal(t, membership.Membership.ID, result.Order.MembershipID.Int64)

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
		ID:             membership.Membership.ID,
		Balance:        rechargeAmount,
		TotalRecharged: rechargeAmount,
		TotalConsumed:  0,
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

// ==================== ProcessOrderPaymentTx Transaction Tests ====================

// createMerchantWithLocation 创建带有经纬度的商户（用于配送测试）
func createMerchantWithLocation(t *testing.T, ownerID int64) Merchant {
	region := createRandomRegion(t)

	appData, _ := json.Marshal(map[string]string{"test": "data"})
	arg := CreateMerchantParams{
		OwnerUserID: ownerID,
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(50), Valid: true},
		LogoUrl:     pgtype.Text{String: "https://example.com/logo.jpg", Valid: true},
		Phone:       "13800138000",
		Address:     "北京市朝阳区测试路" + util.RandomString(8), // 添加随机后缀避免重复
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

	// 验证订单状态没有在这个事务中更新（状态更新在外部）
	require.Equal(t, createResult.Order.ID, result.Order.ID)

	// ✅ 核心验证：外卖订单应该创建配送单
	require.NotNil(t, result.Delivery, "外卖订单应该创建配送单")
	require.Equal(t, createResult.Order.ID, result.Delivery.OrderID)
	require.Equal(t, "pending", result.Delivery.Status)
	require.Equal(t, merchant.Address, result.Delivery.PickupAddress)
	require.Equal(t, address.DetailAddress, result.Delivery.DeliveryAddress)
	require.Equal(t, int32(2500), result.Delivery.Distance)
	require.Equal(t, int64(800), result.Delivery.DeliveryFee)

	// ✅ 核心验证：外卖订单应该进入配送池
	require.NotNil(t, result.PoolItem, "外卖订单应该进入配送池")
	require.Equal(t, createResult.Order.ID, result.PoolItem.OrderID)
	require.Equal(t, merchant.ID, result.PoolItem.MerchantID)
	require.Equal(t, int64(800), result.PoolItem.DeliveryFee)
	require.Equal(t, int32(1), result.PoolItem.Priority) // 8元运费，优先级=1

	// 验证配送池可以被查询到
	poolItem, err := testStore.GetDeliveryPoolByOrderID(context.Background(), createResult.Order.ID)
	require.NoError(t, err)
	require.Equal(t, result.PoolItem.ID, poolItem.ID)
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

	// 验证高运费订单的优先级为3
	require.NotNil(t, result.PoolItem)
	require.Equal(t, int32(3), result.PoolItem.Priority, ">=20元运费应该是高优先级3")
}

func TestProcessOrderPaymentTx_DineInOrder(t *testing.T) {
	// 堂食订单不应该创建配送单和配送池
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

	// ✅ 堂食订单不应该有配送单和配送池
	require.Nil(t, result.Delivery, "堂食订单不应该创建配送单")
	require.Nil(t, result.PoolItem, "堂食订单不应该进入配送池")
}

func TestProcessOrderPaymentTx_TakeawayOrder(t *testing.T) {
	// 外带订单（自取）不应该创建配送单
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

	// ✅ 外带自取订单不应该有配送单
	require.Nil(t, result.Delivery, "外带订单不应该创建配送单")
	require.Nil(t, result.PoolItem, "外带订单不应该进入配送池")
}

func TestProcessOrderPaymentTx_TakeoutWithoutAddress(t *testing.T) {
	// 外卖订单缺少配送地址应该报错
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, merchantOwner.ID)

	// 创建外卖订单但不设置配送地址
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

	// 支付处理应该失败
	_, err = testStore.ProcessOrderPaymentTx(context.Background(), ProcessOrderPaymentTxParams{
		OrderID: createResult.Order.ID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing delivery address")
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

	// 验证中等运费订单的优先级为2
	require.NotNil(t, result.PoolItem)
	require.Equal(t, int32(2), result.PoolItem.Priority, ">=10元且<20元运费应该是中优先级2")
}
