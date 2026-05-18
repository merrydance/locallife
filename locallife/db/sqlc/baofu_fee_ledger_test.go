package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestCreateBaofuFeeLedgerRecordsPaymentFee(t *testing.T) {
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)

	ledger, err := testStore.CreateBaofuFeeLedger(context.Background(), CreateBaofuFeeLedgerParams{
		FeeType:            BaofuFeeTypePaymentFee,
		PayerType:          BaofuFeePayerTypeMerchant,
		PayerID:            pgtype.Int8{Int64: merchant.ID, Valid: true},
		BusinessObjectType: "payment_order",
		BusinessObjectID:   paymentOrder.ID,
		Amount:             30,
		FeeRateBps:         pgtype.Int4{Int32: 30, Valid: true},
		Status:             "recorded",
	})
	require.NoError(t, err)
	require.Equal(t, BaofuFeeTypePaymentFee, ledger.FeeType)
	require.Equal(t, BaofuFeePayerTypeMerchant, ledger.PayerType)
	require.Equal(t, int64(30), ledger.Amount)
	require.Equal(t, int32(30), ledger.FeeRateBps.Int32)

	_, err = testStore.CreateBaofuFeeLedger(context.Background(), CreateBaofuFeeLedgerParams{
		FeeType:            BaofuFeeTypePaymentFee,
		PayerType:          BaofuFeePayerTypeMerchant,
		PayerID:            pgtype.Int8{Int64: merchant.ID, Valid: true},
		BusinessObjectType: "payment_order",
		BusinessObjectID:   paymentOrder.ID,
		Amount:             30,
		FeeRateBps:         pgtype.Int4{Int32: 30, Valid: true},
		Status:             "recorded",
	})
	require.Error(t, err)
}

func TestGetBaofuFeeLedgerByBusinessObject(t *testing.T) {
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)
	created, err := testStore.CreateBaofuFeeLedger(context.Background(), CreateBaofuFeeLedgerParams{
		FeeType:            BaofuFeeTypePaymentFee,
		PayerType:          BaofuFeePayerTypeMerchant,
		PayerID:            pgtype.Int8{Int64: merchant.ID, Valid: true},
		BusinessObjectType: "payment_order",
		BusinessObjectID:   paymentOrder.ID,
		Amount:             30,
		FeeRateBps:         pgtype.Int4{Int32: 30, Valid: true},
		Status:             "recorded",
	})
	require.NoError(t, err)

	got, err := testStore.GetBaofuFeeLedgerByBusinessObject(context.Background(), GetBaofuFeeLedgerByBusinessObjectParams{
		FeeType:            BaofuFeeTypePaymentFee,
		BusinessObjectType: "payment_order",
		BusinessObjectID:   paymentOrder.ID,
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}
