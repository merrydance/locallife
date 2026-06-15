package db

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestCreatePartnerPaymentTx_ReservationPaymentModeMismatch(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_ = createRandomMerchantPaymentConfig(t, merchant)
	table := createRandomRoom(t, merchant.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "pending")

	_, err := testStore.CreatePartnerPaymentTx(context.Background(), CreatePartnerPaymentTxParams{
		UserID:        user.ID,
		MerchantID:    merchant.ID,
		ReservationID: reservation.ID,
		PaymentMode:   "full",
		BusinessType:  "reservation",
		Amount:        reservation.DepositAmount,
		OutTradeNo:    "RS" + util.RandomString(20),
		ExpiresAt:     time.Now().Add(time.Hour),
	})
	require.Error(t, err)
	status, ok := IsPartnerPaymentRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusConflict, status)
	require.Contains(t, err.Error(), "payment mode changed")
}

func TestCreatePartnerPaymentTx_ReservationAmountChanged(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_ = createRandomMerchantPaymentConfig(t, merchant)
	table := createRandomRoom(t, merchant.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "pending")

	_, err := testStore.CreatePartnerPaymentTx(context.Background(), CreatePartnerPaymentTxParams{
		UserID:        user.ID,
		MerchantID:    merchant.ID,
		ReservationID: reservation.ID,
		PaymentMode:   reservation.PaymentMode,
		BusinessType:  "reservation",
		Amount:        reservation.DepositAmount - 1,
		OutTradeNo:    "RS" + util.RandomString(20),
		ExpiresAt:     time.Now().Add(time.Hour),
	})
	require.Error(t, err)
	status, ok := IsPartnerPaymentRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusConflict, status)
	require.Contains(t, err.Error(), "payable amount changed")
}

func TestCreatePartnerPaymentTx_ReservationAddonLocksReservationAndCreatesReservationLinkedPayment(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	paymentConfig := createRandomMerchantPaymentConfig(t, merchant)
	table := createRandomRoom(t, merchant.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "confirmed")

	result, err := testStore.CreatePartnerPaymentTx(context.Background(), CreatePartnerPaymentTxParams{
		UserID:                user.ID,
		MerchantID:            merchant.ID,
		OrderID:               0,
		ReservationID:         reservation.ID,
		PaymentMode:           reservation.PaymentMode,
		BusinessType:          "reservation_addon",
		Amount:                3600,
		OutTradeNo:            "RA" + util.RandomString(20),
		ExpiresAt:             time.Now().Add(time.Hour),
		Attach:                "reservation_id:2007;payment_mode:full;addon:true",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	})
	require.NoError(t, err)
	require.False(t, result.PaymentOrder.OrderID.Valid)
	require.True(t, result.PaymentOrder.ReservationID.Valid)
	require.Equal(t, reservation.ID, result.PaymentOrder.ReservationID.Int64)
	require.Equal(t, "reservation_addon", result.PaymentOrder.BusinessType)
	require.Equal(t, PaymentChannelBaofuAggregate, result.PaymentOrder.PaymentChannel)
	require.True(t, result.PaymentOrder.RequiresProfitSharing)
	require.Equal(t, int64(3600), result.PaymentOrder.Amount)
	require.Equal(t, paymentConfig.SubMchID, result.SubMchID)

	_, err = testStore.CreatePartnerPaymentTx(context.Background(), CreatePartnerPaymentTxParams{
		UserID:                user.ID,
		MerchantID:            merchant.ID,
		OrderID:               0,
		ReservationID:         reservation.ID,
		PaymentMode:           reservation.PaymentMode,
		BusinessType:          "reservation_addon",
		Amount:                1800,
		OutTradeNo:            "RA" + util.RandomString(20),
		ExpiresAt:             time.Now().Add(time.Hour),
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	})
	require.Error(t, err)
	status, ok := IsPartnerPaymentRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusConflict, status)
	require.Contains(t, err.Error(), "pending payment order")
}

func TestCreatePartnerPaymentTx_CopiesReservationIDFromOrder(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	paymentConfig := createRandomMerchantPaymentConfig(t, merchant)
	table := createRandomRoom(t, merchant.ID)
	reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "confirmed")

	order, err := testStore.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:       util.RandomString(20),
		UserID:        user.ID,
		MerchantID:    merchant.ID,
		OrderType:     "dine_in",
		TableID:       pgtype.Int8{Int64: table.ID, Valid: true},
		ReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
		Subtotal:      1200,
		TotalAmount:   1200,
		Status:        "pending",
	})
	require.NoError(t, err)

	result, err := testStore.CreatePartnerPaymentTx(context.Background(), CreatePartnerPaymentTxParams{
		UserID:       user.ID,
		MerchantID:   merchant.ID,
		OrderID:      order.ID,
		BusinessType: "order",
		Amount:       1200,
		OutTradeNo:   "RO" + util.RandomString(20),
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.True(t, result.PaymentOrder.ReservationID.Valid)
	require.Equal(t, reservation.ID, result.PaymentOrder.ReservationID.Int64)
	require.Equal(t, PaymentChannelBaofuAggregate, result.PaymentOrder.PaymentChannel)
	require.Equal(t, paymentConfig.SubMchID, result.SubMchID)
}

func TestCreatePartnerPaymentTx_OrderMerchantMismatch(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_ = createRandomMerchantPaymentConfig(t, merchant)
	otherMerchant := createRandomMerchantWithOwner(t, owner.ID)
	_ = createRandomMerchantPaymentConfig(t, otherMerchant)

	order, err := testStore.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:     util.RandomString(20),
		UserID:      user.ID,
		MerchantID:  merchant.ID,
		OrderType:   "takeout",
		Subtotal:    1200,
		TotalAmount: 1200,
		Status:      "pending",
	})
	require.NoError(t, err)

	_, err = testStore.CreatePartnerPaymentTx(context.Background(), CreatePartnerPaymentTxParams{
		UserID:       user.ID,
		MerchantID:   otherMerchant.ID,
		OrderID:      order.ID,
		BusinessType: "order",
		Amount:       1200,
		OutTradeNo:   "RO" + util.RandomString(20),
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	require.Error(t, err)
	status, ok := IsPartnerPaymentRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusConflict, status)
	require.Contains(t, err.Error(), "merchant changed")
}

func TestCreatePartnerPaymentTx_PersistsStableSubMchIDInAttach(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	paymentConfig := createRandomMerchantPaymentConfig(t, merchant)

	order, err := testStore.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:     util.RandomString(20),
		UserID:      user.ID,
		MerchantID:  merchant.ID,
		OrderType:   "takeout",
		Subtotal:    1200,
		TotalAmount: 1200,
		Status:      "pending",
	})
	require.NoError(t, err)

	result, err := testStore.CreatePartnerPaymentTx(context.Background(), CreatePartnerPaymentTxParams{
		UserID:       user.ID,
		MerchantID:   merchant.ID,
		OrderID:      order.ID,
		BusinessType: "order",
		Amount:       1200,
		OutTradeNo:   "RO" + util.RandomString(20),
		ExpiresAt:    time.Now().Add(time.Hour),
		Attach:       "order_id:1234",
	})
	require.NoError(t, err)
	require.True(t, result.PaymentOrder.Attach.Valid)
	require.Equal(t, "order_id:1234;sub_mchid:"+paymentConfig.SubMchID, result.PaymentOrder.Attach.String)
	require.Equal(t, paymentConfig.SubMchID, result.SubMchID)
}

func TestCreatePartnerPaymentTx_ConcurrentOrderPaymentAllowsSinglePendingPayment(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_ = createRandomMerchantPaymentConfig(t, merchant)

	order, err := testStore.CreateOrder(ctx, CreateOrderParams{
		OrderNo:     util.RandomString(20),
		UserID:      user.ID,
		MerchantID:  merchant.ID,
		OrderType:   OrderTypeTakeout,
		Subtotal:    3600,
		TotalAmount: 3600,
		Status:      OrderStatusPending,
	})
	require.NoError(t, err)

	lockTx, err := testStore.(*SQLStore).connPool.Begin(ctx)
	require.NoError(t, err)
	defer func() {
		_ = lockTx.Rollback(ctx)
	}()
	var lockedOrderID int64
	err = lockTx.QueryRow(ctx, `SELECT id FROM orders WHERE id = $1 FOR UPDATE`, order.ID).Scan(&lockedOrderID)
	require.NoError(t, err)
	require.Equal(t, order.ID, lockedOrderID)

	type createResult struct {
		payment PaymentOrder
		err     error
	}
	results := make(chan createResult, 2)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := testStore.CreatePartnerPaymentTx(ctx, CreatePartnerPaymentTxParams{
				UserID:                user.ID,
				MerchantID:            merchant.ID,
				OrderID:               order.ID,
				BusinessType:          "order",
				Amount:                order.TotalAmount,
				OutTradeNo:            "RO" + util.RandomString(20),
				ExpiresAt:             time.Now().Add(time.Hour),
				PaymentChannel:        PaymentChannelBaofuAggregate,
				RequiresProfitSharing: true,
			})
			results <- createResult{payment: result.PaymentOrder, err: err}
		}()
	}

	var earlyResults []createResult
	select {
	case result := <-results:
		earlyResults = append(earlyResults, result)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, lockTx.Commit(ctx))
	wg.Wait()
	close(results)

	allResults := append([]createResult{}, earlyResults...)
	for result := range results {
		allResults = append(allResults, result)
	}
	require.Empty(t, earlyResults, "CreatePartnerPaymentTx must wait for the order row lock before checking pending payments")
	require.Len(t, allResults, 2)

	var created []PaymentOrder
	var conflicts int
	for _, result := range allResults {
		if result.err == nil {
			created = append(created, result.payment)
			continue
		}
		status, ok := IsPartnerPaymentRequestError(result.err)
		require.True(t, ok, "unexpected error: %v", result.err)
		require.Equal(t, http.StatusConflict, status)
		require.Contains(t, result.err.Error(), "pending payment order")
		conflicts++
	}

	require.Len(t, created, 1)
	require.Equal(t, 1, conflicts)
	require.Equal(t, PaymentChannelBaofuAggregate, created[0].PaymentChannel)
	require.True(t, created[0].RequiresProfitSharing)

	payments, err := testStore.GetPaymentOrdersByOrder(ctx, pgtype.Int8{Int64: order.ID, Valid: true})
	require.NoError(t, err)
	var pendingForOrder []PaymentOrder
	for _, payment := range payments {
		if payment.BusinessType == "order" && payment.Status == "pending" {
			pendingForOrder = append(pendingForOrder, payment)
		}
	}
	require.Len(t, pendingForOrder, 1)
	require.Equal(t, created[0].ID, pendingForOrder[0].ID)
}
