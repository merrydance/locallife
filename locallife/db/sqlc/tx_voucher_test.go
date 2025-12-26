package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// ==================== ClaimVoucherTx Tests ====================

func TestClaimVoucherTx(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 创建优惠券模板
	voucher, err := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "测试优惠券",
		Description:       pgtype.Text{String: "测试用", Valid: true},
		Amount:            1000, // 10元
		MinOrderAmount:    5000, // 满50元
		TotalQuantity:     100,
		ValidFrom:         time.Now(),
		ValidUntil:        time.Now().Add(30 * 24 * time.Hour),
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	require.NoError(t, err)

	// 领取优惠券
	arg := ClaimVoucherTxParams{
		VoucherID: voucher.ID,
		UserID:    user.ID,
	}

	result, err := testStore.ClaimVoucherTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证用户优惠券创建成功
	userVoucher := result.UserVoucher
	require.NotZero(t, userVoucher.ID)
	require.Equal(t, user.ID, userVoucher.UserID)
	require.Equal(t, voucher.ID, userVoucher.VoucherID)
	require.Equal(t, "unused", userVoucher.Status)
	require.False(t, userVoucher.UsedAt.Valid)

	// 验证优惠券领取数量增加
	dbVoucher, err := testStore.GetVoucher(context.Background(), voucher.ID)
	require.NoError(t, err)
	require.Equal(t, int32(1), dbVoucher.ClaimedQuantity)
}

// 测试防重复领取
func TestClaimVoucherTxDuplicateClaim(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	voucher, err := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "限领一次券",
		Description:       pgtype.Text{String: "限领一次", Valid: true},
		Amount:            1000,
		MinOrderAmount:    5000,
		TotalQuantity:     100,
		ValidFrom:         time.Now(),
		ValidUntil:        time.Now().Add(30 * 24 * time.Hour),
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	require.NoError(t, err)

	arg := ClaimVoucherTxParams{
		VoucherID: voucher.ID,
		UserID:    user.ID,
	}

	// 第一次领取
	result1, err := testStore.ClaimVoucherTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result1)

	// 第二次领取（应该失败）
	result2, err := testStore.ClaimVoucherTx(context.Background(), arg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already claimed")
	require.Empty(t, result2)

	// 验证领取数量没有再次增加
	dbVoucher, err := testStore.GetVoucher(context.Background(), voucher.ID)
	require.NoError(t, err)
	require.Equal(t, int32(1), dbVoucher.ClaimedQuantity)
}

// 测试库存不足
func TestClaimVoucherTxInsufficientStock(t *testing.T) {
	user1 := createRandomUser(t)
	user2 := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 创建只有1张的优惠券
	voucher, err := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "限量券",
		Description:       pgtype.Text{String: "仅1张", Valid: true},
		Amount:            1000,
		MinOrderAmount:    5000,
		TotalQuantity:     1, // 只有1张
		ValidFrom:         time.Now(),
		ValidUntil:        time.Now().Add(30 * 24 * time.Hour),
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	require.NoError(t, err)

	// 用户1领取成功
	_, err = testStore.ClaimVoucherTx(context.Background(), ClaimVoucherTxParams{
		VoucherID: voucher.ID,
		UserID:    user1.ID,
	})
	require.NoError(t, err)

	// 用户2领取失败（库存不足）
	result2, err := testStore.ClaimVoucherTx(context.Background(), ClaimVoucherTxParams{
		VoucherID: voucher.ID,
		UserID:    user2.ID,
	})
	require.Error(t, err)
	// 错误消息可能包含 "no rows" 或 "out of stock"
	require.NotEmpty(t, err.Error())
	require.Empty(t, result2)

	// 验证领取数量为1
	dbVoucher, err := testStore.GetVoucher(context.Background(), voucher.ID)
	require.NoError(t, err)
	require.Equal(t, int32(1), dbVoucher.ClaimedQuantity)
}

// 测试领取已过期优惠券
func TestClaimVoucherTxExpired(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 创建已过期的优惠券
	voucher, err := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "过期券",
		Description:       pgtype.Text{String: "已过期", Valid: true},
		Amount:            1000,
		MinOrderAmount:    5000,
		TotalQuantity:     100,
		ValidFrom:         time.Now().Add(-48 * time.Hour), // 48小时前开始
		ValidUntil:        time.Now().Add(-24 * time.Hour), // 24小时前过期
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	require.NoError(t, err)

	// 尝试领取
	result, err := testStore.ClaimVoucherTx(context.Background(), ClaimVoucherTxParams{
		VoucherID: voucher.ID,
		UserID:    user.ID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "expired")
	require.Empty(t, result)
}

// 测试领取非活跃状态的优惠券
func TestClaimVoucherTxInactive(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 创建已下架的优惠券
	voucher, err := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "已下架券",
		Description:       pgtype.Text{String: "已下架", Valid: true},
		Amount:            1000,
		MinOrderAmount:    5000,
		TotalQuantity:     100,
		ValidFrom:         time.Now(),
		ValidUntil:        time.Now().Add(30 * 24 * time.Hour),
		IsActive:          false, // 已下架
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	require.NoError(t, err)

	// 尝试领取
	result, err := testStore.ClaimVoucherTx(context.Background(), ClaimVoucherTxParams{
		VoucherID: voucher.ID,
		UserID:    user.ID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not active")
	require.Empty(t, result)
}

// ==================== UseVoucherTx Tests ====================

func TestUseVoucherTx(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 创建并领取优惠券
	voucher, err := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "可使用券",
		Description:       pgtype.Text{String: "测试使用", Valid: true},
		Amount:            1000,
		MinOrderAmount:    5000,
		TotalQuantity:     100,
		ValidFrom:         time.Now(),
		ValidUntil:        time.Now().Add(30 * 24 * time.Hour),
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	require.NoError(t, err)

	claimResult, err := testStore.ClaimVoucherTx(context.Background(), ClaimVoucherTxParams{
		VoucherID: voucher.ID,
		UserID:    user.ID,
	})
	require.NoError(t, err)

	// 使用优惠券
	arg := UseVoucherTxParams{
		UserVoucherID: claimResult.UserVoucher.ID,
		OrderID:       12345,
	}

	result, err := testStore.UseVoucherTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证优惠券标记为已使用
	require.Equal(t, "used", result.UserVoucher.Status)
	require.True(t, result.UserVoucher.UsedAt.Valid)
}

// 测试重复使用优惠券
func TestUseVoucherTxAlreadyUsed(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	voucher, err := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "一次性券",
		Description:       pgtype.Text{String: "仅用一次", Valid: true},
		Amount:            1000,
		MinOrderAmount:    5000,
		TotalQuantity:     100,
		ValidFrom:         time.Now(),
		ValidUntil:        time.Now().Add(30 * 24 * time.Hour),
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	require.NoError(t, err)

	claimResult, err := testStore.ClaimVoucherTx(context.Background(), ClaimVoucherTxParams{
		VoucherID: voucher.ID,
		UserID:    user.ID,
	})
	require.NoError(t, err)

	// 第一次使用
	_, err = testStore.UseVoucherTx(context.Background(), UseVoucherTxParams{
		UserVoucherID: claimResult.UserVoucher.ID,
		OrderID:       11111,
	})
	require.NoError(t, err)

	// 第二次使用（应该失败）
	result2, err := testStore.UseVoucherTx(context.Background(), UseVoucherTxParams{
		UserVoucherID: claimResult.UserVoucher.ID,
		OrderID:       22222,
	})
	require.Error(t, err)
	// 错误消息包含 "not unused" 或 "already used"
	require.Contains(t, err.Error(), "not unused")
	require.Empty(t, result2)
}

// 测试使用已过期的优惠券
func TestUseVoucherTxExpired(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 创建即将过期的优惠券
	voucher, err := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "即将过期券",
		Description:       pgtype.Text{String: "已过期", Valid: true},
		Amount:            1000,
		MinOrderAmount:    5000,
		TotalQuantity:     100,
		ValidFrom:         time.Now().Add(-48 * time.Hour),
		ValidUntil:        time.Now().Add(-1 * time.Hour), // 1小时前过期
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	require.NoError(t, err)

	// 直接创建已过期的用户优惠券（绕过领取验证）
	userVoucher, err := testStore.CreateUserVoucher(context.Background(), CreateUserVoucherParams{
		UserID:    user.ID,
		VoucherID: voucher.ID,
	})
	require.NoError(t, err)

	// 尝试使用（应该失败）
	result, err := testStore.UseVoucherTx(context.Background(), UseVoucherTxParams{
		UserVoucherID: userVoucher.ID,
		OrderID:       99999,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "expired")
	require.Empty(t, result)
}

// ==================== Concurrent Tests ====================

// 测试并发领取优惠券（验证库存扣减的正确性）
func TestClaimVoucherTxConcurrent(t *testing.T) {
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)

	// 创建有限库存的优惠券
	totalQuantity := int32(20)
	voucher, err := testStore.CreateVoucher(context.Background(), CreateVoucherParams{
		MerchantID:        merchant.ID,
		Code:              util.RandomString(10),
		Name:              "并发测试券",
		Description:       pgtype.Text{String: "并发测试", Valid: true},
		Amount:            1000,
		MinOrderAmount:    5000,
		TotalQuantity:     totalQuantity,
		ValidFrom:         time.Now(),
		ValidUntil:        time.Now().Add(30 * 24 * time.Hour),
		IsActive:          true,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"},
	})
	require.NoError(t, err)

	// 并发30个用户领取（只有20张券）
	n := 30
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		go func() {
			user := createRandomUser(t)
			_, err := testStore.ClaimVoucherTx(context.Background(), ClaimVoucherTxParams{
				VoucherID: voucher.ID,
				UserID:    user.ID,
			})
			errs <- err
		}()
	}

	// 统计成功和失败次数
	successCount := 0
	failureCount := 0
	for i := 0; i < n; i++ {
		err := <-errs
		if err == nil {
			successCount++
		} else {
			failureCount++
		}
	}

	// 验证成功领取的数量等于库存
	require.Equal(t, int(totalQuantity), successCount)
	require.Equal(t, n-int(totalQuantity), failureCount)

	// 验证数据库中的领取数量
	dbVoucher, err := testStore.GetVoucher(context.Background(), voucher.ID)
	require.NoError(t, err)
	require.Equal(t, totalQuantity, dbVoucher.ClaimedQuantity)
}
