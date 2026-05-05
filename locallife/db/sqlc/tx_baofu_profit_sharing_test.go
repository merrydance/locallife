package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestCreateBaofuProfitSharingOrderTxCreatesOrderAndFeeLedgers(t *testing.T) {
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
		FeeBreakdown: UpdateProfitSharingOrderFeeBreakdownParams{
			CalculationVersion:           "baofu_fee_v2",
			SettlementMode:               "commission_share",
			ProviderPaymentFee:           30,
			ProviderPaymentFeeRateBps:    30,
			ProviderPaymentFeeBaseAmount: 10000,
			ProviderPaymentFeeSource:     "estimated",
			MerchantPaymentFee:           57,
			MerchantPaymentFeeRateBps:    60,
			MerchantPaymentFeeBaseAmount: 9500,
			RiderGrossAmount:             500,
			RiderPaymentFee:              3,
			RiderPaymentFeeRateBps:       60,
			RiderPaymentFeeBaseAmount:    500,
			CommissionBaseAmount:         9500,
			PlatformReceiverAmount:       220,
		},
		OrderPaymentFeeLedgers: []CreateOrderPaymentFeeLedgerParams{
			{
				Provider:           ExternalPaymentProviderBaofu,
				Channel:            PaymentChannelBaofuAggregate,
				PaymentOrderID:     paymentOrder.ID,
				FeeType:            OrderPaymentFeeTypeProviderPaymentFee,
				PayerType:          OrderPaymentFeePayerTypePlatform,
				PayeeType:          OrderPaymentFeePayeeTypeBaofu,
				BaseAmount:         10000,
				RateBps:            30,
				Amount:             30,
				AmountSource:       OrderPaymentFeeAmountSourceCalculated,
				Status:             OrderPaymentFeeStatusRecorded,
				CalculationVersion: "baofu_fee_v2",
			},
			{
				Provider:           ExternalPaymentProviderBaofu,
				Channel:            PaymentChannelBaofuAggregate,
				PaymentOrderID:     paymentOrder.ID,
				FeeType:            OrderPaymentFeeTypeMerchantPaymentServiceFee,
				PayerType:          OrderPaymentFeePayerTypeMerchant,
				PayerID:            pgtype.Int8{Int64: merchant.ID, Valid: true},
				PayeeType:          OrderPaymentFeePayeeTypePlatform,
				BaseAmount:         9500,
				RateBps:            60,
				Amount:             57,
				AmountSource:       OrderPaymentFeeAmountSourceCalculated,
				Status:             OrderPaymentFeeStatusRecorded,
				CalculationVersion: "baofu_fee_v2",
			},
			{
				Provider:           ExternalPaymentProviderBaofu,
				Channel:            PaymentChannelBaofuAggregate,
				PaymentOrderID:     paymentOrder.ID,
				FeeType:            OrderPaymentFeeTypeRiderPaymentServiceFee,
				PayerType:          OrderPaymentFeePayerTypeRider,
				PayerID:            pgtype.Int8{Int64: 202, Valid: true},
				PayeeType:          OrderPaymentFeePayeeTypePlatform,
				BaseAmount:         500,
				RateBps:            60,
				Amount:             3,
				AmountSource:       OrderPaymentFeeAmountSourceCalculated,
				Status:             OrderPaymentFeeStatusRecorded,
				CalculationVersion: "baofu_fee_v2",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, result.ProfitSharingOrder.PaymentOrderID, result.PaymentFeeLedger.BusinessObjectID)
	require.Equal(t, int64(30), result.PaymentFeeLedger.Amount)
	require.Equal(t, "baofu_fee_v2", result.ProfitSharingOrder.CalculationVersion)
	require.Equal(t, int64(57), result.ProfitSharingOrder.MerchantPaymentFee)
	require.Equal(t, int64(3), result.ProfitSharingOrder.RiderPaymentFee)
	require.Equal(t, int64(220), result.ProfitSharingOrder.PlatformReceiverAmount)
	require.Len(t, result.OrderPaymentFeeLedgers, 3)

	gotLedger, err := testStore.GetBaofuFeeLedgerByBusinessObject(context.Background(), GetBaofuFeeLedgerByBusinessObjectParams{
		FeeType:            BaofuFeeTypePaymentFee,
		BusinessObjectType: "payment_order",
		BusinessObjectID:   paymentOrder.ID,
	})
	require.NoError(t, err)
	require.Equal(t, result.PaymentFeeLedger.ID, gotLedger.ID)

	gotOrderFeeLedgers, err := testStore.ListOrderPaymentFeeLedgersByPayer(context.Background(), ListOrderPaymentFeeLedgersByPayerParams{
		PayerType:  OrderPaymentFeePayerTypeMerchant,
		PayerID:    pgtype.Int8{Int64: merchant.ID, Valid: true},
		LimitCount: 10,
	})
	require.NoError(t, err)
	require.Len(t, gotOrderFeeLedgers, 1)
	require.Equal(t, OrderPaymentFeeTypeMerchantPaymentServiceFee, gotOrderFeeLedgers[0].FeeType)
	require.Equal(t, int64(57), gotOrderFeeLedgers[0].Amount)
	require.Equal(t, result.ProfitSharingOrder.ID, gotOrderFeeLedgers[0].ProfitSharingOrderID.Int64)
}

func TestCreateBaofuProfitSharingOrderTxKeepsActualProviderFeeLedger(t *testing.T) {
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	operator := createRandomOperatorForRegion(t, merchant.RegionID)
	paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)
	fact := createRandomExternalPaymentFact(t, ExternalPaymentTerminalStatusSuccess, true)

	actualLedger, err := testStore.UpsertOrderPaymentFeeLedgerActual(context.Background(), UpsertOrderPaymentFeeLedgerActualParams{
		Provider:              ExternalPaymentProviderBaofu,
		Channel:               PaymentChannelBaofuAggregate,
		PaymentOrderID:        paymentOrder.ID,
		FeeType:               OrderPaymentFeeTypeProviderPaymentFee,
		PayerType:             OrderPaymentFeePayerTypePlatform,
		PayeeType:             OrderPaymentFeePayeeTypeBaofu,
		BaseAmount:            10000,
		RateBps:               30,
		Amount:                31,
		AmountSource:          OrderPaymentFeeAmountSourceActualCallback,
		ExternalPaymentFactID: pgtype.Int8{Int64: fact.ID, Valid: true},
		Status:                OrderPaymentFeeStatusRecorded,
		CalculationVersion:    "baofu_fee_v2",
	})
	require.NoError(t, err)

	result, err := testStore.CreateBaofuProfitSharingOrderTx(context.Background(), CreateBaofuProfitSharingOrderTxParams{
		ProfitSharingOrder: CreateProfitSharingOrderParams{
			PaymentOrderID:        paymentOrder.ID,
			MerchantID:            merchant.ID,
			OperatorID:            pgtype.Int8{Int64: operator.ID, Valid: true},
			OrderSource:           "reservation",
			TotalAmount:           10000,
			DistributableAmount:   10000,
			PlatformRate:          200,
			OperatorRate:          300,
			PlatformCommission:    200,
			OperatorCommission:    300,
			MerchantAmount:        9440,
			OutOrderNo:            "pso_baofu_actual_fee_" + util.RandomString(16),
			Status:                ProfitSharingOrderStatusPending,
			PaymentFee:            30,
			PaymentFeeRateBps:     30,
			Provider:              ExternalPaymentProviderBaofu,
			Channel:               PaymentChannelBaofuAggregate,
			SharingDetailSnapshot: []byte(`{"receivers":[]}`),
		},
		PaymentFeeLedger: CreateBaofuFeeLedgerParams{
			FeeType:            BaofuFeeTypePaymentFee,
			PayerType:          BaofuFeePayerTypePlatform,
			BusinessObjectType: "payment_order",
			BusinessObjectID:   paymentOrder.ID,
			Amount:             30,
			FeeRateBps:         pgtype.Int4{Int32: 30, Valid: true},
			Status:             "recorded",
		},
		FeeBreakdown: UpdateProfitSharingOrderFeeBreakdownParams{
			CalculationVersion:           "baofu_fee_v2",
			SettlementMode:               ProfitSharingSettlementModeCommissionShare,
			ProviderPaymentFee:           30,
			ProviderPaymentFeeRateBps:    30,
			ProviderPaymentFeeBaseAmount: 10000,
			ProviderPaymentFeeSource:     "estimated",
			MerchantPaymentFee:           60,
			MerchantPaymentFeeRateBps:    60,
			MerchantPaymentFeeBaseAmount: 10000,
			CommissionBaseAmount:         10000,
			PlatformReceiverAmount:       230,
		},
		OrderPaymentFeeLedgers: []CreateOrderPaymentFeeLedgerParams{
			{
				Provider:           ExternalPaymentProviderBaofu,
				Channel:            PaymentChannelBaofuAggregate,
				PaymentOrderID:     paymentOrder.ID,
				FeeType:            OrderPaymentFeeTypeProviderPaymentFee,
				PayerType:          OrderPaymentFeePayerTypePlatform,
				PayeeType:          OrderPaymentFeePayeeTypeBaofu,
				BaseAmount:         10000,
				RateBps:            30,
				Amount:             30,
				AmountSource:       OrderPaymentFeeAmountSourceCalculated,
				Status:             OrderPaymentFeeStatusRecorded,
				CalculationVersion: "baofu_fee_v2",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.OrderPaymentFeeLedgers, 1)
	require.Equal(t, actualLedger.ID, result.OrderPaymentFeeLedgers[0].ID)
	require.Equal(t, int64(31), result.OrderPaymentFeeLedgers[0].Amount)
	require.Equal(t, OrderPaymentFeeAmountSourceActualCallback, result.OrderPaymentFeeLedgers[0].AmountSource)
	require.Equal(t, result.ProfitSharingOrder.ID, result.OrderPaymentFeeLedgers[0].ProfitSharingOrderID.Int64)
	require.Equal(t, fact.ID, result.OrderPaymentFeeLedgers[0].ExternalPaymentFactID.Int64)
}
