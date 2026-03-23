package db

import (
	"context"
	"testing"
	"time"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomMerchantPaymentConfig(t *testing.T, merchant Merchant) MerchantPaymentConfig {
	arg := CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   util.RandomString(10),
		Status:     "active",
	}
	config, err := testStore.CreateMerchantPaymentConfig(context.Background(), arg)
	require.NoError(t, err)
	return config
}

func TestCreateCombinedPaymentTx(t *testing.T) {
	store := testStore

	// 1. Create a user
	user := createRandomUser(t)

	// 2. Create 2 merchants
	merchantFull := createRandomMerchantForTest(t)
	merchantFullConfig := createRandomMerchantPaymentConfig(t, merchantFull)

	merchantPartial := createRandomMerchantForTest(t)
	merchantPartialConfig := createRandomMerchantPaymentConfig(t, merchantPartial)

	// 3. Create 2 orders for this user
	order1 := createRandomOrderWithUserAndMerchant(t, user.ID, merchantFull.ID)
	order2 := createRandomOrderWithUserAndMerchant(t, user.ID, merchantPartial.ID)

	// 4. Input arguments
	arg := CreateCombinedPaymentTxParams{
		UserID:            user.ID,
		OrderIDs:          []int64{order1.ID, order2.ID},
		CombineOutTradeNo: "CP" + util.RandomString(10),
		ExpiresAt:         time.Now().Add(time.Hour),
	}

	// 5. Execute transaction
	result, err := store.CreateCombinedPaymentTx(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 6. Verify CombinedPaymentOrder
	require.NotEmpty(t, result.CombinedPaymentOrder)
	require.Equal(t, user.ID, result.CombinedPaymentOrder.UserID)
	require.Equal(t, arg.CombineOutTradeNo, result.CombinedPaymentOrder.CombineOutTradeNo)
	require.Equal(t, order1.TotalAmount+order2.TotalAmount, result.CombinedPaymentOrder.TotalAmount)
	require.Equal(t, "pending", result.CombinedPaymentOrder.Status)

	// 7. Verify PaymentOrders
	require.Len(t, result.PaymentOrders, 2)
	for _, po := range result.PaymentOrders {
		require.NotEmpty(t, po)
		require.Equal(t, user.ID, po.UserID)
		require.Equal(t, "profit_sharing", po.PaymentType)
		require.Equal(t, "order", po.BusinessType)
		require.Equal(t, result.CombinedPaymentOrder.ID, po.CombinedPaymentID.Int64)
		require.True(t, po.Attach.Valid)
	}

	// 8. Verify OrderInfos and SubMchIDs
	require.Len(t, result.OrderInfos, 2)
	for _, info := range result.OrderInfos {
		require.NotEmpty(t, info.PaymentConfig)
		switch info.Order.ID {
		case order1.ID:
			require.Equal(t, merchantFullConfig.SubMchID, info.PaymentConfig.SubMchID)
		case order2.ID:
			require.Equal(t, merchantPartialConfig.SubMchID, info.PaymentConfig.SubMchID)
		}
	}
}

func TestCreateCombinedPaymentTx_OrderMismatch(t *testing.T) {
	store := testStore
	user1 := createRandomUser(t)
	user2 := createRandomUser(t)
	merchant := createRandomMerchantForTest(t)
	_ = createRandomMerchantPaymentConfig(t, merchant)

	// Order belongs to user2
	order := createRandomOrderWithUserAndMerchant(t, user2.ID, merchant.ID)

	arg := CreateCombinedPaymentTxParams{
		UserID:            user1.ID, // Mismatch
		OrderIDs:          []int64{order.ID},
		CombineOutTradeNo: "CPFAIL" + util.RandomString(5),
		ExpiresAt:         time.Now(),
	}

	_, err := store.CreateCombinedPaymentTx(context.Background(), arg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not belong to user")
}
