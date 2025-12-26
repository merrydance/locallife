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

func createRandomVoucher(t *testing.T, merchantID int64) Voucher {
	now := time.Now()
	arg := CreateVoucherParams{
		MerchantID:        merchantID,
		Code:              util.RandomString(10),
		Name:              "测试代金券-" + util.RandomString(5),
		Description:       pgtype.Text{String: "测试用代金券", Valid: true},
		Amount:            1000,
		MinOrderAmount:    5000,
		TotalQuantity:     100,
		ValidFrom:         now.AddDate(0, 0, -1),
		ValidUntil:        now.AddDate(0, 1, 0),
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	}

	voucher, err := testStore.CreateVoucher(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, voucher.ID)

	return voucher
}

func createRandomUserVoucher(t *testing.T, voucherID, userID int64) UserVoucher {
	arg := CreateUserVoucherParams{
		VoucherID: voucherID,
		UserID:    userID,
		ExpiresAt: time.Now().AddDate(0, 1, 0),
	}

	uv, err := testStore.CreateUserVoucher(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, uv.ID)

	return uv
}

// ==================== Voucher Tests ====================

func TestCreateVoucher(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	voucher := createRandomVoucher(t, merchant.ID)

	require.Equal(t, merchant.ID, voucher.MerchantID)
	require.Equal(t, int64(1000), voucher.Amount)
	require.Equal(t, int64(5000), voucher.MinOrderAmount)
	require.Equal(t, int32(100), voucher.TotalQuantity)
	require.Equal(t, int32(0), voucher.ClaimedQuantity)
	require.Equal(t, int32(0), voucher.UsedQuantity)
	require.True(t, voucher.IsActive)
}

func TestGetVoucher(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	created := createRandomVoucher(t, merchant.ID)

	got, err := testStore.GetVoucher(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, created.Code, got.Code)
	require.Equal(t, created.Amount, got.Amount)
}

func TestGetVoucher_NotFound(t *testing.T) {
	_, err := testStore.GetVoucher(context.Background(), 99999999)
	require.Error(t, err)
}

func TestGetVoucherByCode(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	created := createRandomVoucher(t, merchant.ID)

	got, err := testStore.GetVoucherByCode(context.Background(), created.Code)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}

func TestListMerchantVouchers(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建5张代金券
	for i := 0; i < 5; i++ {
		createRandomVoucher(t, merchant.ID)
	}

	vouchers, err := testStore.ListMerchantVouchers(context.Background(), ListMerchantVouchersParams{
		MerchantID: merchant.ID,
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, vouchers, 5)
}

func TestListActiveVouchers(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建活跃代金券
	for i := 0; i < 3; i++ {
		createRandomVoucher(t, merchant.ID)
	}

	// 创建已过期代金券
	now := time.Now()
	_, err := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "过期代金券",
		Amount:            500,
		MinOrderAmount:    2000,
		TotalQuantity:     50,
		ValidFrom:         now.AddDate(0, 0, -30),
		ValidUntil:        now.AddDate(0, 0, -1), // 已过期
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	require.NoError(t, err)

	// 创建未激活的代金券
	_, err = testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "未激活代金券",
		Amount:            500,
		MinOrderAmount:    2000,
		TotalQuantity:     50,
		ValidFrom:         time.Now().AddDate(0, 0, -1),
		ValidUntil:        time.Now().AddDate(0, 1, 0),
		IsActive:          false, // 未激活
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	require.NoError(t, err)

	// 只查询活跃代金券
	vouchers, err := testStore.ListActiveVouchers(context.Background(), ListActiveVouchersParams{
		MerchantID: merchant.ID,
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, vouchers, 3) // 只有3个活跃的
}

func TestUpdateVoucher(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	voucher := createRandomVoucher(t, merchant.ID)

	// 更新金额
	newAmount := pgtype.Int8{Int64: 2000, Valid: true}
	updated, err := testStore.UpdateVoucher(context.Background(), UpdateVoucherParams{
		ID:     voucher.ID,
		Amount: newAmount,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2000), updated.Amount)
	require.Equal(t, voucher.Name, updated.Name) // 未修改
}

func TestIncrementVoucherClaimedQuantity(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	voucher := createRandomVoucher(t, merchant.ID)

	// 领取一张
	updated, err := testStore.IncrementVoucherClaimedQuantity(context.Background(), voucher.ID)
	require.NoError(t, err)
	require.Equal(t, int32(1), updated.ClaimedQuantity)

	// 再领取一张
	updated, err = testStore.IncrementVoucherClaimedQuantity(context.Background(), voucher.ID)
	require.NoError(t, err)
	require.Equal(t, int32(2), updated.ClaimedQuantity)
}

func TestIncrementVoucherClaimedQuantity_Exhausted(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建一张数量为1的代金券
	now := time.Now()
	voucher, err := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "限量代金券",
		Amount:            1000,
		MinOrderAmount:    5000,
		TotalQuantity:     1, // 只有1张
		ValidFrom:         now.AddDate(0, 0, -1),
		ValidUntil:        now.AddDate(0, 1, 0),
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	require.NoError(t, err)

	// 领取第一张
	_, err = testStore.IncrementVoucherClaimedQuantity(context.Background(), voucher.ID)
	require.NoError(t, err)

	// 尝试领取第二张（应该失败）
	_, err = testStore.IncrementVoucherClaimedQuantity(context.Background(), voucher.ID)
	require.Error(t, err) // 已领完
}

func TestIncrementVoucherUsedQuantity(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	voucher := createRandomVoucher(t, merchant.ID)

	// 先领取一张
	_, err := testStore.IncrementVoucherClaimedQuantity(context.Background(), voucher.ID)
	require.NoError(t, err)

	// 然后使用
	updated, err := testStore.IncrementVoucherUsedQuantity(context.Background(), voucher.ID)
	require.NoError(t, err)
	require.Equal(t, int32(1), updated.UsedQuantity)
}

func TestDeleteVoucher(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	voucher := createRandomVoucher(t, merchant.ID)

	err := testStore.DeleteVoucher(context.Background(), voucher.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = testStore.GetVoucher(context.Background(), voucher.ID)
	require.Error(t, err)
}

// ==================== User Voucher Tests ====================

func TestCreateUserVoucher(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	voucher := createRandomVoucher(t, merchant.ID)
	uv := createRandomUserVoucher(t, voucher.ID, user.ID)

	require.Equal(t, voucher.ID, uv.VoucherID)
	require.Equal(t, user.ID, uv.UserID)
	require.Equal(t, "unused", uv.Status)
	require.False(t, uv.OrderID.Valid)
}

func TestGetUserVoucher(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	voucher := createRandomVoucher(t, merchant.ID)
	created := createRandomUserVoucher(t, voucher.ID, user.ID)

	got, err := testStore.GetUserVoucher(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, voucher.MerchantID, got.MerchantID)
	require.Equal(t, voucher.Code, got.Code)
	require.Equal(t, voucher.Amount, got.Amount)
}

func TestListUserVouchers(t *testing.T) {
	user := createRandomUser(t)

	// 从3个商户领取代金券
	for i := 0; i < 3; i++ {
		owner := createRandomUser(t)
		merchant := createRandomMerchantWithOwner(t, owner.ID)
		voucher := createRandomVoucher(t, merchant.ID)
		createRandomUserVoucher(t, voucher.ID, user.ID)
	}

	vouchers, err := testStore.ListUserVouchers(context.Background(), ListUserVouchersParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, vouchers, 3)
}

func TestListUserAvailableVouchers(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建3张可用代金券
	for i := 0; i < 3; i++ {
		voucher := createRandomVoucher(t, merchant.ID)
		createRandomUserVoucher(t, voucher.ID, user.ID)
	}

	// 创建一张已过期代金券
	expiredVoucher := createRandomVoucher(t, merchant.ID)
	_, err := testStore.CreateUserVoucher(context.Background(), CreateUserVoucherParams{
		VoucherID: expiredVoucher.ID,
		UserID:    user.ID,
		ExpiresAt: time.Now().AddDate(0, 0, -1), // 已过期
	})
	require.NoError(t, err)

	// 只查询可用代金券
	vouchers, err := testStore.ListUserAvailableVouchers(context.Background(), ListUserAvailableVouchersParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, vouchers, 3) // 只有3张可用
}

func TestListUserAvailableVouchersForMerchant(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建不同门槛的代金券
	// 满50可用
	v1, _ := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "满50减10",
		Amount:            1000,
		MinOrderAmount:    5000,
		TotalQuantity:     100,
		ValidFrom:         time.Now().AddDate(0, 0, -1),
		ValidUntil:        time.Now().AddDate(0, 1, 0),
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	createRandomUserVoucher(t, v1.ID, user.ID)

	// 满100可用
	v2, _ := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "满100减20",
		Amount:            2000,
		MinOrderAmount:    10000,
		TotalQuantity:     100,
		ValidFrom:         time.Now().AddDate(0, 0, -1),
		ValidUntil:        time.Now().AddDate(0, 1, 0),
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	createRandomUserVoucher(t, v2.ID, user.ID)

	// 订单金额70元，只能用满50的
	vouchers, err := testStore.ListUserAvailableVouchersForMerchant(context.Background(), ListUserAvailableVouchersForMerchantParams{
		UserID:         user.ID,
		MerchantID:     merchant.ID,
		MinOrderAmount: 7000, // 订单金额 70元
	})
	require.NoError(t, err)
	require.Len(t, vouchers, 1)
	require.Equal(t, v1.ID, vouchers[0].VoucherID)

	// 订单金额150元，两张都能用
	vouchers, err = testStore.ListUserAvailableVouchersForMerchant(context.Background(), ListUserAvailableVouchersForMerchantParams{
		UserID:         user.ID,
		MerchantID:     merchant.ID,
		MinOrderAmount: 15000, // 订单金额 150元
	})
	require.NoError(t, err)
	require.Len(t, vouchers, 2)
}

func TestCheckUserVoucherExists(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	voucher := createRandomVoucher(t, merchant.ID)

	// 未领取前
	exists, err := testStore.CheckUserVoucherExists(context.Background(), CheckUserVoucherExistsParams{
		VoucherID: voucher.ID,
		UserID:    user.ID,
	})
	require.NoError(t, err)
	require.False(t, exists)

	// 领取后
	createRandomUserVoucher(t, voucher.ID, user.ID)

	exists, err = testStore.CheckUserVoucherExists(context.Background(), CheckUserVoucherExistsParams{
		VoucherID: voucher.ID,
		UserID:    user.ID,
	})
	require.NoError(t, err)
	require.True(t, exists)
}

func TestMarkUserVoucherAsUsed(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	voucher := createRandomVoucher(t, merchant.ID)
	uv := createRandomUserVoucher(t, voucher.ID, user.ID)

	// 创建订单
	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())

	// 标记为已使用
	updated, err := testStore.MarkUserVoucherAsUsed(context.Background(), MarkUserVoucherAsUsedParams{
		ID:      uv.ID,
		OrderID: pgtype.Int8{Int64: order.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "used", updated.Status)
	require.True(t, updated.OrderID.Valid)
	require.Equal(t, order.ID, updated.OrderID.Int64)
	require.True(t, updated.UsedAt.Valid)
}

func TestMarkUserVoucherAsUsed_AlreadyUsed(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	voucher := createRandomVoucher(t, merchant.ID)
	uv := createRandomUserVoucher(t, voucher.ID, user.ID)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())

	// 第一次标记
	_, err := testStore.MarkUserVoucherAsUsed(context.Background(), MarkUserVoucherAsUsedParams{
		ID:      uv.ID,
		OrderID: pgtype.Int8{Int64: order.ID, Valid: true},
	})
	require.NoError(t, err)

	// 再次标记（应该失败）
	_, err = testStore.MarkUserVoucherAsUsed(context.Background(), MarkUserVoucherAsUsedParams{
		ID:      uv.ID,
		OrderID: pgtype.Int8{Int64: order.ID, Valid: true},
	})
	require.Error(t, err) // 已经使用过
}

func TestCountUserVouchersByStatus(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建3张可用代金券
	for i := 0; i < 3; i++ {
		voucher := createRandomVoucher(t, merchant.ID)
		createRandomUserVoucher(t, voucher.ID, user.ID)
	}

	// 创建1张已使用的代金券
	usedVoucher := createRandomVoucher(t, merchant.ID)
	uv := createRandomUserVoucher(t, usedVoucher.ID, user.ID)
	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	_, err := testStore.MarkUserVoucherAsUsed(context.Background(), MarkUserVoucherAsUsedParams{
		ID:      uv.ID,
		OrderID: pgtype.Int8{Int64: order.ID, Valid: true},
	})
	require.NoError(t, err)

	// 创建1张已过期代金券
	expiredVoucher := createRandomVoucher(t, merchant.ID)
	_, err = testStore.CreateUserVoucher(context.Background(), CreateUserVoucherParams{
		VoucherID: expiredVoucher.ID,
		UserID:    user.ID,
		ExpiresAt: time.Now().AddDate(0, 0, -1), // 已过期
	})
	require.NoError(t, err)

	counts, err := testStore.CountUserVouchersByStatus(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), counts.AvailableCount) // 3张可用
	require.Equal(t, int64(1), counts.UsedCount)      // 1张已使用
	require.Equal(t, int64(1), counts.ExpiredCount)   // 1张已过期
}

func TestGetVoucherUsageStats(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	voucher := createRandomVoucher(t, merchant.ID)

	// 3个用户领取
	for i := 0; i < 3; i++ {
		user := createRandomUser(t)
		createRandomUserVoucher(t, voucher.ID, user.ID)
		_, err := testStore.IncrementVoucherClaimedQuantity(context.Background(), voucher.ID)
		require.NoError(t, err)
	}

	// 1个用户使用
	useUser := createRandomUser(t)
	uv := createRandomUserVoucher(t, voucher.ID, useUser.ID)
	_, err := testStore.IncrementVoucherClaimedQuantity(context.Background(), voucher.ID)
	require.NoError(t, err)

	order := createCompletedOrderForStats(t, useUser.ID, merchant.ID, 10000, "takeout", time.Now())
	_, err = testStore.MarkUserVoucherAsUsed(context.Background(), MarkUserVoucherAsUsedParams{
		ID:      uv.ID,
		OrderID: pgtype.Int8{Int64: order.ID, Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.IncrementVoucherUsedQuantity(context.Background(), voucher.ID)
	require.NoError(t, err)

	stats, err := testStore.GetVoucherUsageStats(context.Background(), voucher.ID)
	require.NoError(t, err)
	require.Equal(t, int32(100), stats.TotalQuantity)
	require.Equal(t, int32(4), stats.ClaimedQuantity)
	require.Equal(t, int32(1), stats.UsedQuantity)
	require.Equal(t, int64(3), stats.ActiveCount)       // 3张可用
	require.Equal(t, int64(1), stats.UsedCountVerified) // 1张已使用
}
