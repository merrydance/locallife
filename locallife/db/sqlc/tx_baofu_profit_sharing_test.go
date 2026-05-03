package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestCreateBaofuProfitSharingOrderTxCreatesOrderAndFeeLedger(t *testing.T) {
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	operator := createRandomOperatorForRegion(t, merchant.RegionID)
	paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)
	snapshot := []byte(`{"receivers":[{"role":"merchant","sharing_mer_id":"MER_SHARE","amount":8970}],"payment_fee":30,"payment_fee_rate_bps":30}`)

	result, err := testStore.CreateBaofuProfitSharingOrderTx(context.Background(), CreateBaofuProfitSharingOrderTxParams{
		ProfitSharingOrder: CreateProfitSharingOrderParams{
			PaymentOrderID:        paymentOrder.ID,
			MerchantID:            merchant.ID,
			OperatorID:            pgtype.Int8{Int64: operator.ID, Valid: true},
			OrderSource:           "takeout",
			TotalAmount:           10000,
			DeliveryFee:           500,
			RiderID:               pgtype.Int8{Int64: 202, Valid: true},
			RiderAmount:           500,
			DistributableAmount:   9500,
			PlatformRate:          200,
			OperatorRate:          300,
			PlatformCommission:    200,
			OperatorCommission:    300,
			MerchantAmount:        8970,
			OutOrderNo:            "pso_baofu_tx_" + util.RandomString(16),
			Status:                ProfitSharingOrderStatusPending,
			PaymentFee:            30,
			PaymentFeeRateBps:     30,
			Provider:              ExternalPaymentProviderBaofu,
			Channel:               PaymentChannelBaofuAggregate,
			MerchantSharingMerID:  pgtype.Text{String: "MER_SHARE", Valid: true},
			RiderSharingMerID:     pgtype.Text{String: "RIDER_SHARE", Valid: true},
			OperatorSharingMerID:  pgtype.Text{String: "OP_SHARE", Valid: true},
			PlatformSharingMerID:  pgtype.Text{String: "PLATFORM_SHARE", Valid: true},
			SharingDetailSnapshot: snapshot,
		},
		PaymentFeeLedger: CreateBaofuFeeLedgerParams{
			FeeType:            BaofuFeeTypePaymentFee,
			PayerType:          BaofuFeePayerTypeMerchant,
			PayerID:            pgtype.Int8{Int64: merchant.ID, Valid: true},
			BusinessObjectType: "payment_order",
			BusinessObjectID:   paymentOrder.ID,
			Amount:             30,
			FeeRateBps:         pgtype.Int4{Int32: 30, Valid: true},
			Status:             "recorded",
		},
	})
	require.NoError(t, err)
	require.Equal(t, result.ProfitSharingOrder.PaymentOrderID, result.PaymentFeeLedger.BusinessObjectID)
	require.Equal(t, int64(30), result.PaymentFeeLedger.Amount)

	gotLedger, err := testStore.GetBaofuFeeLedgerByBusinessObject(context.Background(), GetBaofuFeeLedgerByBusinessObjectParams{
		FeeType:            BaofuFeeTypePaymentFee,
		BusinessObjectType: "payment_order",
		BusinessObjectID:   paymentOrder.ID,
	})
	require.NoError(t, err)
	require.Equal(t, result.PaymentFeeLedger.ID, gotLedger.ID)
}
