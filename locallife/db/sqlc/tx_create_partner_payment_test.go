package db

import (
	"context"
	"net/http"
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
	require.Equal(t, PaymentChannelOrdinaryServiceProvider, result.PaymentOrder.PaymentChannel)
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
