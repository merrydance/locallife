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

func TestCreateBaofuProfitSharingOrderTxRejectsActiveRefund(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	paymentOrder, err := testStore.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
		UserID:                createRandomUser(t).ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          ExternalPaymentBusinessOwnerOrder,
		Amount:                10000,
		OutTradeNo:            "BFPS_REFUND_GUARD_" + util.RandomString(12),
	})
	require.NoError(t, err)
	paymentOrder, err = testStore.UpdatePaymentOrderToPaid(ctx, UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: "BF_TX_" + util.RandomString(20), Valid: true},
	})
	require.NoError(t, err)
	_ = createRandomRefundOrder(t, paymentOrder.ID, 500)

	_, err = testStore.CreateBaofuProfitSharingOrderTx(ctx, CreateBaofuProfitSharingOrderTxParams{
		ProfitSharingOrder: CreateProfitSharingOrderParams{
			PaymentOrderID:      paymentOrder.ID,
			MerchantID:          merchant.ID,
			OrderSource:         "takeout",
			TotalAmount:         paymentOrder.Amount,
			DistributableAmount: paymentOrder.Amount,
			PlatformRate:        200,
			OperatorRate:        300,
			PlatformCommission:  200,
			OperatorCommission:  300,
			MerchantAmount:      9440,
			OutOrderNo:          "pso_refund_guard_" + util.RandomString(16),
			Status:              ProfitSharingOrderStatusPending,
			PaymentFee:          30,
			PaymentFeeRateBps:   30,
			Provider:            ExternalPaymentProviderBaofu,
			Channel:             PaymentChannelBaofuAggregate,
		},
		PaymentFeeLedger: CreateBaofuFeeLedgerParams{
			FeeType:            BaofuFeeTypePaymentFee,
			PayerType:          BaofuFeePayerTypePlatform,
			BusinessObjectType: "payment_order",
			BusinessObjectID:   paymentOrder.ID,
			Amount:             30,
			Status:             "recorded",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "订单已有退款申请或退款成功记录")
	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, statusCode)

	_, getErr := testStore.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
	require.ErrorIs(t, getErr, ErrRecordNotFound)
}

func TestPrepareBaofuProfitSharingCommandTxRejectsActiveRefund(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	paymentOrder, profitSharingOrder := createPaidBaofuPaymentWithProfitSharingOrder(t, ctx, user.ID, merchant.ID, ProfitSharingOrderStatusPending, "pso_prepare_refund_")
	_ = createRandomRefundOrder(t, paymentOrder.ID, 500)

	_, err := testStore.PrepareBaofuProfitSharingCommandTx(ctx, PrepareBaofuProfitSharingCommandTxParams{
		ProfitSharingOrderID: profitSharingOrder.ID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "订单已有退款申请或退款成功记录")
	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, statusCode)

	current, getErr := testStore.GetProfitSharingOrder(ctx, profitSharingOrder.ID)
	require.NoError(t, getErr)
	require.Equal(t, ProfitSharingOrderStatusPending, current.Status)
}

func TestPrepareBaofuProfitSharingCommandTxMarksPendingOrderProcessing(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	paymentOrder, profitSharingOrder := createPaidBaofuPaymentWithProfitSharingOrder(t, ctx, user.ID, merchant.ID, ProfitSharingOrderStatusPending, "pso_prepare_pending_")

	result, err := testStore.PrepareBaofuProfitSharingCommandTx(ctx, PrepareBaofuProfitSharingCommandTxParams{
		ProfitSharingOrderID: profitSharingOrder.ID,
	})
	require.NoError(t, err)
	require.Equal(t, paymentOrder.ID, result.PaymentOrder.ID)
	require.Equal(t, profitSharingOrder.ID, result.ProfitSharingOrder.ID)
	require.Equal(t, ProfitSharingOrderStatusProcessing, result.ProfitSharingOrder.Status)
	require.False(t, result.ProfitSharingOrder.SharingOrderID.Valid)

	current, err := testStore.GetProfitSharingOrder(ctx, profitSharingOrder.ID)
	require.NoError(t, err)
	require.Equal(t, ProfitSharingOrderStatusProcessing, current.Status)
}

func TestPrepareBaofuProfitSharingCommandTxSetsCommandStartedAt(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	_, profitSharingOrder := createPaidBaofuPaymentWithProfitSharingOrder(t, ctx, user.ID, merchant.ID, ProfitSharingOrderStatusPending, "pso_prepare_started_")

	before := time.Now().UTC().Add(-time.Second)
	result, err := testStore.PrepareBaofuProfitSharingCommandTx(ctx, PrepareBaofuProfitSharingCommandTxParams{
		ProfitSharingOrderID: profitSharingOrder.ID,
	})
	after := time.Now().UTC().Add(time.Second)
	require.NoError(t, err)
	require.True(t, result.ProfitSharingOrder.CommandStartedAt.Valid)
	require.False(t, result.ProfitSharingOrder.CommandStartedAt.Time.Before(before))
	require.False(t, result.ProfitSharingOrder.CommandStartedAt.Time.After(after))
}

func TestPrepareBaofuProfitSharingCommandTxRetriesFailedOrder(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	_, profitSharingOrder := createPaidBaofuPaymentWithProfitSharingOrder(t, ctx, user.ID, merchant.ID, ProfitSharingOrderStatusFailed, "pso_prepare_failed_")

	result, err := testStore.PrepareBaofuProfitSharingCommandTx(ctx, PrepareBaofuProfitSharingCommandTxParams{
		ProfitSharingOrderID: profitSharingOrder.ID,
	})
	require.NoError(t, err)
	require.Equal(t, profitSharingOrder.ID, result.ProfitSharingOrder.ID)
	require.Equal(t, ProfitSharingOrderStatusProcessing, result.ProfitSharingOrder.Status)
}

func TestEnsureBaofuProfitSharingBillTxRejectsActiveRefund(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	paymentOrder, err := testStore.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
		UserID:                createRandomUser(t).ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          ExternalPaymentBusinessOwnerOrder,
		Amount:                10000,
		OutTradeNo:            "BFPS_BILL_REFUND_" + util.RandomString(12),
	})
	require.NoError(t, err)
	paymentOrder, err = testStore.UpdatePaymentOrderToPaid(ctx, UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: "BF_TX_" + util.RandomString(20), Valid: true},
	})
	require.NoError(t, err)
	_ = createRandomRefundOrder(t, paymentOrder.ID, 500)

	_, err = testStore.EnsureBaofuProfitSharingBillTx(ctx, CreateBaofuProfitSharingOrderTxParams{
		ProfitSharingOrder: CreateProfitSharingOrderParams{
			PaymentOrderID:        paymentOrder.ID,
			MerchantID:            merchant.ID,
			OrderSource:           "takeout",
			TotalAmount:           paymentOrder.Amount,
			DistributableAmount:   paymentOrder.Amount,
			PlatformRate:          200,
			OperatorRate:          300,
			PlatformCommission:    200,
			OperatorCommission:    300,
			MerchantAmount:        9440,
			OutOrderNo:            "pso_bill_refund_" + util.RandomString(16),
			Status:                ProfitSharingOrderStatusPending,
			PaymentFee:            30,
			PaymentFeeRateBps:     30,
			Provider:              ExternalPaymentProviderBaofu,
			Channel:               PaymentChannelBaofuAggregate,
			SharingDetailSnapshot: []byte(`{"receivers":[{"role":"merchant","sharing_mer_id":"MER_SHARE","amount":9440}]}`),
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
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "订单已有退款申请或退款成功记录")
	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, statusCode)

	_, getErr := testStore.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
	require.ErrorIs(t, getErr, ErrRecordNotFound)
}

func TestEnsureBaofuProfitSharingBillTxReturnsExistingIdenticalBill(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)
	arg := CreateBaofuProfitSharingOrderTxParams{
		ProfitSharingOrder: CreateProfitSharingOrderParams{
			PaymentOrderID:        paymentOrder.ID,
			MerchantID:            merchant.ID,
			OrderSource:           "reservation",
			TotalAmount:           10000,
			DistributableAmount:   10000,
			PlatformRate:          200,
			OperatorRate:          300,
			PlatformCommission:    200,
			OperatorCommission:    300,
			MerchantAmount:        9440,
			OutOrderNo:            "pso_bill_idempotent_" + util.RandomString(16),
			Status:                ProfitSharingOrderStatusPending,
			PaymentFee:            30,
			PaymentFeeRateBps:     30,
			Provider:              ExternalPaymentProviderBaofu,
			Channel:               PaymentChannelBaofuAggregate,
			SharingDetailSnapshot: []byte(`{"receivers":[{"role":"merchant","sharing_mer_id":"MER_SHARE","amount":9440}]}`),
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
	}

	first, err := testStore.EnsureBaofuProfitSharingBillTx(ctx, arg)
	require.NoError(t, err)
	second, err := testStore.EnsureBaofuProfitSharingBillTx(ctx, arg)
	require.NoError(t, err)
	require.Equal(t, first.ProfitSharingOrder.ID, second.ProfitSharingOrder.ID)

	rows, err := testStore.ListProfitSharingOrdersByStatus(ctx, ListProfitSharingOrdersByStatusParams{
		Status: ProfitSharingOrderStatusPending,
		Limit:  1000,
		Offset: 0,
	})
	require.NoError(t, err)
	var count int
	for _, row := range rows {
		if row.PaymentOrderID == paymentOrder.ID {
			count++
		}
	}
	require.Equal(t, 1, count)
}

func TestEnsureBaofuProfitSharingBillTxReturnsExistingBillAfterRiderAssigned(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)
	outOrderNo := "pso_bill_rider_" + util.RandomString(16)

	withRider := CreateBaofuProfitSharingOrderTxParams{
		ProfitSharingOrder: CreateProfitSharingOrderParams{
			PaymentOrderID:        paymentOrder.ID,
			MerchantID:            merchant.ID,
			OperatorID:            pgtype.Int8{Int64: 1, Valid: true},
			RiderID:               pgtype.Int8{Int64: 1, Valid: true},
			OrderSource:           "takeout",
			TotalAmount:           759,
			DeliveryFee:           559,
			RiderAmount:           556,
			DistributableAmount:   200,
			PlatformRate:          200,
			OperatorRate:          300,
			PlatformCommission:    4,
			OperatorCommission:    6,
			MerchantAmount:        189,
			OutOrderNo:            outOrderNo,
			Status:                ProfitSharingOrderStatusPending,
			PaymentFee:            2,
			PaymentFeeRateBps:     30,
			Provider:              ExternalPaymentProviderBaofu,
			Channel:               PaymentChannelBaofuAggregate,
			MerchantSharingMerID:  pgtype.Text{String: "MER_SHARE", Valid: true},
			RiderSharingMerID:     pgtype.Text{String: "RIDER_SHARE", Valid: true},
			OperatorSharingMerID:  pgtype.Text{String: "OP_SHARE", Valid: true},
			PlatformSharingMerID:  pgtype.Text{String: "PLATFORM_SHARE", Valid: true},
			SharingDetailSnapshot: []byte(`{"receivers":[{"role":"merchant","sharing_mer_id":"MER_SHARE","amount":189},{"role":"rider","sharing_mer_id":"RIDER_SHARE","amount":556},{"role":"operator","sharing_mer_id":"OP_SHARE","amount":6},{"role":"platform","sharing_mer_id":"PLATFORM_SHARE","amount":6}]}`),
		},
		PaymentFeeLedger: CreateBaofuFeeLedgerParams{
			FeeType:            BaofuFeeTypePaymentFee,
			PayerType:          BaofuFeePayerTypePlatform,
			BusinessObjectType: "payment_order",
			BusinessObjectID:   paymentOrder.ID,
			Amount:             2,
			FeeRateBps:         pgtype.Int4{Int32: 30, Valid: true},
			Status:             "recorded",
		},
		FeeBreakdown: UpdateProfitSharingOrderFeeBreakdownParams{
			CalculationVersion:           "baofu_fee_v2",
			SettlementMode:               ProfitSharingSettlementModeCommissionShare,
			ProviderPaymentFee:           2,
			ProviderPaymentFeeRateBps:    30,
			ProviderPaymentFeeBaseAmount: 759,
			ProviderPaymentFeeSource:     "estimated",
			MerchantPaymentFee:           1,
			MerchantPaymentFeeRateBps:    60,
			MerchantPaymentFeeBaseAmount: 200,
			RiderGrossAmount:             559,
			RiderPaymentFee:              3,
			RiderPaymentFeeRateBps:       60,
			RiderPaymentFeeBaseAmount:    559,
			CommissionBaseAmount:         200,
			PlatformReceiverAmount:       6,
		},
	}
	first, err := testStore.EnsureBaofuProfitSharingBillTx(ctx, withRider)
	require.NoError(t, err)

	withoutRider := withRider
	withoutRider.ProfitSharingOrder.RiderID = pgtype.Int8{}
	withoutRider.ProfitSharingOrder.RiderAmount = 0
	withoutRider.ProfitSharingOrder.MerchantAmount = 189
	withoutRider.ProfitSharingOrder.RiderSharingMerID = pgtype.Text{}
	withoutRider.ProfitSharingOrder.SharingDetailSnapshot = []byte(`{"receivers":[{"role":"merchant","sharing_mer_id":"MER_SHARE","amount":189},{"role":"operator","sharing_mer_id":"OP_SHARE","amount":6},{"role":"platform","sharing_mer_id":"PLATFORM_SHARE","amount":3}]}`)
	withoutRider.FeeBreakdown.RiderGrossAmount = 559
	withoutRider.FeeBreakdown.RiderPaymentFee = 0
	withoutRider.FeeBreakdown.RiderPaymentFeeBaseAmount = 0
	withoutRider.FeeBreakdown.PlatformReceiverAmount = 3

	second, err := testStore.EnsureBaofuProfitSharingBillTx(ctx, withoutRider)
	require.NoError(t, err)
	require.Equal(t, first.ProfitSharingOrder.ID, second.ProfitSharingOrder.ID)
	require.True(t, second.ProfitSharingOrder.RiderID.Valid)
	require.Equal(t, int64(1), second.ProfitSharingOrder.RiderID.Int64)
	require.Equal(t, int64(556), second.ProfitSharingOrder.RiderAmount)
	require.Equal(t, int64(6), second.ProfitSharingOrder.PlatformReceiverAmount)
}

func TestEnsureBaofuProfitSharingBillTxRejectsConflictingBill(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)
	arg := CreateBaofuProfitSharingOrderTxParams{
		ProfitSharingOrder: CreateProfitSharingOrderParams{
			PaymentOrderID:      paymentOrder.ID,
			MerchantID:          merchant.ID,
			OrderSource:         "reservation",
			TotalAmount:         10000,
			DistributableAmount: 10000,
			PlatformRate:        200,
			OperatorRate:        300,
			PlatformCommission:  200,
			OperatorCommission:  300,
			MerchantAmount:      9440,
			OutOrderNo:          "pso_bill_conflict_" + util.RandomString(16),
			Status:              ProfitSharingOrderStatusPending,
			PaymentFee:          30,
			PaymentFeeRateBps:   30,
			Provider:            ExternalPaymentProviderBaofu,
			Channel:             PaymentChannelBaofuAggregate,
		},
		PaymentFeeLedger: CreateBaofuFeeLedgerParams{
			FeeType:            BaofuFeeTypePaymentFee,
			PayerType:          BaofuFeePayerTypePlatform,
			BusinessObjectType: "payment_order",
			BusinessObjectID:   paymentOrder.ID,
			Amount:             30,
			Status:             "recorded",
		},
	}
	_, err := testStore.EnsureBaofuProfitSharingBillTx(ctx, arg)
	require.NoError(t, err)

	arg.ProfitSharingOrder.MerchantAmount = 9000
	_, err = testStore.EnsureBaofuProfitSharingBillTx(ctx, arg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "订单已存在不同的宝付分账账单")
}

func TestCreateBaofuProfitSharingOrderTxRejectsDuplicateProfitSharingOrder(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	paymentOrder, err := testStore.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
		UserID:                createRandomUser(t).ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
		BusinessType:          ExternalPaymentBusinessOwnerOrder,
		Amount:                10000,
		OutTradeNo:            "BFPS_DUP_" + util.RandomString(12),
	})
	require.NoError(t, err)
	paymentOrder, err = testStore.UpdatePaymentOrderToPaid(ctx, UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: "BF_TX_" + util.RandomString(20), Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.CreateProfitSharingOrder(ctx, CreateProfitSharingOrderParams{
		PaymentOrderID:      paymentOrder.ID,
		MerchantID:          merchant.ID,
		OrderSource:         "takeout",
		TotalAmount:         paymentOrder.Amount,
		DistributableAmount: paymentOrder.Amount,
		PlatformRate:        200,
		OperatorRate:        300,
		PlatformCommission:  200,
		OperatorCommission:  300,
		MerchantAmount:      9440,
		OutOrderNo:          "pso_dup_guard_" + util.RandomString(16),
		Status:              ProfitSharingOrderStatusPending,
		PaymentFee:          30,
		PaymentFeeRateBps:   30,
		Provider:            ExternalPaymentProviderBaofu,
		Channel:             PaymentChannelBaofuAggregate,
	})
	require.NoError(t, err)

	_, err = testStore.CreateBaofuProfitSharingOrderTx(ctx, CreateBaofuProfitSharingOrderTxParams{
		ProfitSharingOrder: CreateProfitSharingOrderParams{
			PaymentOrderID:      paymentOrder.ID,
			MerchantID:          merchant.ID,
			OrderSource:         "takeout",
			TotalAmount:         paymentOrder.Amount,
			DistributableAmount: paymentOrder.Amount,
			PlatformRate:        200,
			OperatorRate:        300,
			PlatformCommission:  200,
			OperatorCommission:  300,
			MerchantAmount:      9440,
			OutOrderNo:          "pso_dup_guard_2_" + util.RandomString(16),
			Status:              ProfitSharingOrderStatusPending,
			PaymentFee:          30,
			PaymentFeeRateBps:   30,
			Provider:            ExternalPaymentProviderBaofu,
			Channel:             PaymentChannelBaofuAggregate,
		},
		PaymentFeeLedger: CreateBaofuFeeLedgerParams{
			FeeType:            BaofuFeeTypePaymentFee,
			PayerType:          BaofuFeePayerTypePlatform,
			BusinessObjectType: "payment_order",
			BusinessObjectID:   paymentOrder.ID,
			Amount:             30,
			Status:             "recorded",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "订单已存在宝付分账单")
	statusCode, ok := IsRefundRequestError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, statusCode)
}

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
