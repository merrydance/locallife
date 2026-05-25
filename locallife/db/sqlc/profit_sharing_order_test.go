package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomOperatorForRegion(t *testing.T, regionID int64) Operator {
	operator, err := testStore.CreateOperator(context.Background(), CreateOperatorParams{
		UserID:            createRandomUser(t).ID,
		RegionID:          regionID,
		Name:              "op_" + util.RandomString(8),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	return operator
}

func TestProfitSharingListQueriesUseIDTieBreaker(t *testing.T) {
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	operator := createRandomOperatorForRegion(t, merchant.RegionID)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)
	startAt := tiedCreatedAt.Add(-time.Minute)
	endAt := tiedCreatedAt.Add(time.Minute)

	createProfitSharingOrder := func(status string) ProfitSharingOrder {
		paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)
		profitSharingOrder, err := testStore.CreateProfitSharingOrder(context.Background(), CreateProfitSharingOrderParams{
			PaymentOrderID:      paymentOrder.ID,
			MerchantID:          merchant.ID,
			OperatorID:          pgtype.Int8{Int64: operator.ID, Valid: true},
			OrderSource:         "takeout",
			TotalAmount:         10000,
			DeliveryFee:         500,
			RiderID:             pgtype.Int8{Valid: false},
			RiderAmount:         0,
			DistributableAmount: 9500,
			PlatformRate:        2,
			OperatorRate:        3,
			PlatformCommission:  200,
			OperatorCommission:  300,
			MerchantAmount:      9500,
			OutOrderNo:          "pso_" + util.RandomString(16),
			Status:              status,
		})
		require.NoError(t, err)
		return profitSharingOrder
	}

	firstOrder := createProfitSharingOrder("finished")
	secondOrder := createProfitSharingOrder("finished")

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE profit_sharing_orders SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{firstOrder.ID, secondOrder.ID},
	)
	require.NoError(t, err)

	byOperator, err := testStore.ListProfitSharingOrdersByOperator(context.Background(), ListProfitSharingOrdersByOperatorParams{
		OperatorID: pgtype.Int8{Int64: operator.ID, Valid: true},
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, byOperator, 2)
	require.Equal(t, secondOrder.ID, byOperator[0].ID)
	require.Equal(t, firstOrder.ID, byOperator[1].ID)

	settlements, err := testStore.ListMerchantSettlements(context.Background(), ListMerchantSettlementsParams{
		MerchantID: merchant.ID,
		StartAt:    startAt,
		EndAt:      endAt,
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, settlements, 2)
	require.Equal(t, secondOrder.ID, settlements[0].ID)
	require.Equal(t, firstOrder.ID, settlements[1].ID)

	settlementsByStatus, err := testStore.ListMerchantSettlementsByStatus(context.Background(), ListMerchantSettlementsByStatusParams{
		MerchantID: merchant.ID,
		Status:     "finished",
		StartAt:    startAt,
		EndAt:      endAt,
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, settlementsByStatus, 2)
	require.Equal(t, secondOrder.ID, settlementsByStatus[0].ID)
	require.Equal(t, firstOrder.ID, settlementsByStatus[1].ID)

	financeOrders, err := testStore.ListMerchantFinanceOrders(context.Background(), ListMerchantFinanceOrdersParams{
		MerchantID: merchant.ID,
		StartAt:    startAt,
		EndAt:      endAt,
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, financeOrders, 2)
	require.Equal(t, secondOrder.ID, financeOrders[0].ID)
	require.Equal(t, firstOrder.ID, financeOrders[1].ID)
}

func TestProfitSharingReconciliationSummaryUsesFinishedDateForFinishedOrders(t *testing.T) {
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	operator := createRandomOperatorForRegion(t, merchant.RegionID)
	rangeStart := time.Date(2096, time.Month(util.RandomInt(1, 12)), int(util.RandomInt(1, 20)), int(util.RandomInt(1, 20)), 0, 0, 0, time.UTC)
	rangeEnd := rangeStart.Add(time.Hour - time.Nanosecond)

	createOrder := func(outOrderNo string, amount int64, createdAt time.Time, finishedAt time.Time) {
		paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)
		profitSharingOrder, err := testStore.CreateProfitSharingOrder(context.Background(), CreateProfitSharingOrderParams{
			PaymentOrderID:      paymentOrder.ID,
			MerchantID:          merchant.ID,
			OperatorID:          pgtype.Int8{Int64: operator.ID, Valid: true},
			OrderSource:         "takeout",
			TotalAmount:         amount,
			DeliveryFee:         500,
			RiderID:             pgtype.Int8{Valid: false},
			RiderAmount:         0,
			DistributableAmount: amount - 500,
			PlatformRate:        2,
			OperatorRate:        3,
			PlatformCommission:  amount / 50,
			OperatorCommission:  amount / 25,
			MerchantAmount:      amount - amount/50 - amount/25,
			OutOrderNo:          outOrderNo,
			Status:              ProfitSharingOrderStatusFinished,
		})
		require.NoError(t, err)

		_, err = testStore.(*SQLStore).connPool.Exec(context.Background(),
			`UPDATE profit_sharing_orders SET created_at = $1, finished_at = $2 WHERE id = $3`,
			createdAt,
			finishedAt,
			profitSharingOrder.ID,
		)
		require.NoError(t, err)
	}

	createOrder("pso_finished_in_range_"+util.RandomString(16), 31000, rangeStart.AddDate(0, 0, -2), rangeStart.Add(30*time.Minute))
	createOrder("pso_created_in_range_finished_later_"+util.RandomString(16), 17000, rangeStart.Add(30*time.Minute), rangeStart.AddDate(0, 0, 1))

	rows, err := testStore.GetProfitSharingReconciliationSummary(context.Background(), GetProfitSharingReconciliationSummaryParams{
		StartAt: pgtype.Timestamptz{Time: rangeStart, Valid: true},
		EndAt:   pgtype.Timestamptz{Time: rangeEnd, Valid: true},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, ProfitSharingOrderStatusFinished, rows[0].Status)
	require.Equal(t, int64(1), rows[0].TotalOrders)
	require.Equal(t, int64(31000), rows[0].TotalAmount)
	require.Equal(t, int64(30500), rows[0].TotalMerchantFlow)
	require.Equal(t, int64(0), rows[0].TotalRiderFlow)
	require.Equal(t, int64(620), rows[0].TotalPlatformCommission)
	require.Equal(t, int64(1240), rows[0].TotalOperatorCommission)
	require.Equal(t, int64(29140), rows[0].TotalMerchantAmount)
	require.Equal(t, int64(0), rows[0].TotalRiderAmount)

	sla, err := testStore.GetProfitSharingSlaSummary(context.Background(), GetProfitSharingSlaSummaryParams{
		StartAt: pgtype.Timestamptz{Time: rangeStart, Valid: true},
		EndAt:   pgtype.Timestamptz{Time: rangeEnd, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), sla.TotalOrders)
	require.Equal(t, int64(1), sla.FinishedOrders)
}

func TestCreateProfitSharingOrderPersistsBaofuFields(t *testing.T) {
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	operator := createRandomOperatorForRegion(t, merchant.RegionID)
	paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)
	snapshot := []byte(`{"receivers":[{"role":"merchant","sharing_mer_id":"MER_SHARE","amount":8970},{"role":"rider","sharing_mer_id":"RIDER_SHARE","amount":500},{"role":"operator","sharing_mer_id":"OP_SHARE","amount":300},{"role":"platform","sharing_mer_id":"PLATFORM_SHARE","amount":200}],"payment_fee":30,"payment_fee_rate_bps":30}`)

	profitSharingOrder, err := testStore.CreateProfitSharingOrder(context.Background(), CreateProfitSharingOrderParams{
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
		OutOrderNo:            "pso_baofu_" + util.RandomString(16),
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
	})
	require.NoError(t, err)
	require.Equal(t, int64(30), profitSharingOrder.PaymentFee)
	require.Equal(t, int32(30), profitSharingOrder.PaymentFeeRateBps)
	require.Equal(t, ExternalPaymentProviderBaofu, profitSharingOrder.Provider)
	require.Equal(t, PaymentChannelBaofuAggregate, profitSharingOrder.Channel)
	require.Equal(t, "MER_SHARE", profitSharingOrder.MerchantSharingMerID.String)
	require.Equal(t, "RIDER_SHARE", profitSharingOrder.RiderSharingMerID.String)
	require.Equal(t, "OP_SHARE", profitSharingOrder.OperatorSharingMerID.String)
	require.Equal(t, "PLATFORM_SHARE", profitSharingOrder.PlatformSharingMerID.String)
	require.JSONEq(t, string(snapshot), string(profitSharingOrder.SharingDetailSnapshot))
}

func TestUpdateProfitSharingOrderRiderBillByPaymentOrderRequiresPending(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	operator := createRandomOperatorForRegion(t, merchant.RegionID)
	paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)

	order, err := testStore.CreateProfitSharingOrder(ctx, CreateProfitSharingOrderParams{
		PaymentOrderID:       paymentOrder.ID,
		MerchantID:           merchant.ID,
		OperatorID:           pgtype.Int8{Int64: operator.ID, Valid: true},
		OrderSource:          OrderTypeTakeout,
		TotalAmount:          10000,
		DeliveryFee:          500,
		DistributableAmount:  10000,
		PlatformRate:         200,
		OperatorRate:         300,
		PlatformCommission:   200,
		OperatorCommission:   300,
		MerchantAmount:       9440,
		OutOrderNo:           "pso_rider_bill_" + util.RandomString(16),
		Status:               ProfitSharingOrderStatusPending,
		PaymentFee:           30,
		PaymentFeeRateBps:    30,
		Provider:             ExternalPaymentProviderBaofu,
		Channel:              PaymentChannelBaofuAggregate,
		MerchantSharingMerID: pgtype.Text{String: "MER_SHARE", Valid: true},
		OperatorSharingMerID: pgtype.Text{String: "OP_SHARE", Valid: true},
		PlatformSharingMerID: pgtype.Text{String: "PLATFORM_SHARE", Valid: true},
	})
	require.NoError(t, err)

	updated, err := testStore.UpdateProfitSharingOrderRiderBillByPaymentOrder(ctx, UpdateProfitSharingOrderRiderBillByPaymentOrderParams{
		PaymentOrderID:               paymentOrder.ID,
		RiderID:                      pgtype.Int8{Int64: 202, Valid: true},
		RiderSharingMerID:            pgtype.Text{String: "RIDER_SHARE", Valid: true},
		RiderAmount:                  497,
		DistributableAmount:          9500,
		PlatformCommission:           190,
		OperatorCommission:           285,
		MerchantAmount:               8968,
		SharingDetailSnapshot:        []byte(`{"receivers":[{"role":"rider","sharing_mer_id":"RIDER_SHARE","amount":497}]}`),
		RiderGrossAmount:             500,
		RiderPaymentFee:              3,
		RiderPaymentFeeRateBps:       60,
		RiderPaymentFeeBaseAmount:    500,
		MerchantPaymentFee:           57,
		MerchantPaymentFeeBaseAmount: 9500,
		CommissionBaseAmount:         9500,
		PlatformReceiverAmount:       220,
	})
	require.NoError(t, err)
	require.Equal(t, order.ID, updated.ID)
	require.Equal(t, ProfitSharingOrderStatusPending, updated.Status)
	require.Equal(t, int64(202), updated.RiderID.Int64)
	require.Equal(t, int64(500), updated.RiderGrossAmount)
	require.Equal(t, int64(3), updated.RiderPaymentFee)
	require.Equal(t, int64(497), updated.RiderAmount)

	processing, err := testStore.UpdateProfitSharingOrderToProcessing(ctx, UpdateProfitSharingOrderToProcessingParams{
		ID:             order.ID,
		SharingOrderID: pgtype.Text{String: "BF_SHARING_" + util.RandomString(12), Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, ProfitSharingOrderStatusProcessing, processing.Status)

	_, err = testStore.UpdateProfitSharingOrderRiderBillByPaymentOrder(ctx, UpdateProfitSharingOrderRiderBillByPaymentOrderParams{
		PaymentOrderID: paymentOrder.ID,
		RiderID:        pgtype.Int8{Int64: 303, Valid: true},
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestListProfitSharingOrdersByOrderIDsForMerchantScopesMerchantAndOrders(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	otherMerchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	operator := createRandomOperatorForRegion(t, merchant.RegionID)
	user := createRandomUser(t)

	order := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)
	otherOrder := createRandomOrderWithUserAndMerchant(t, user.ID, otherMerchant.ID)
	paymentOrder := createRandomPaymentOrderWithOrder(t, user.ID, &order.ID)
	otherPaymentOrder := createRandomPaymentOrderWithOrder(t, user.ID, &otherOrder.ID)

	createProfitSharing := func(paymentOrder PaymentOrder, merchantID int64, suffix string) ProfitSharingOrder {
		row, err := testStore.CreateProfitSharingOrder(ctx, CreateProfitSharingOrderParams{
			PaymentOrderID:      paymentOrder.ID,
			MerchantID:          merchantID,
			OperatorID:          pgtype.Int8{Int64: operator.ID, Valid: true},
			OrderSource:         "takeout",
			TotalAmount:         10000,
			DeliveryFee:         500,
			RiderID:             pgtype.Int8{Valid: false},
			RiderAmount:         0,
			DistributableAmount: 9500,
			PlatformRate:        200,
			OperatorRate:        300,
			PlatformCommission:  190,
			OperatorCommission:  285,
			MerchantAmount:      8994,
			OutOrderNo:          "pso_scope_" + suffix + "_" + util.RandomString(12),
			Status:              ProfitSharingOrderStatusPending,
			PaymentFee:          31,
		})
		require.NoError(t, err)
		return row
	}

	expected := createProfitSharing(paymentOrder, merchant.ID, "expected")
	createProfitSharing(otherPaymentOrder, otherMerchant.ID, "other")

	rows, err := testStore.ListProfitSharingOrdersByOrderIDsForMerchant(ctx, ListProfitSharingOrdersByOrderIDsForMerchantParams{
		MerchantID: merchant.ID,
		OrderIds:   []int64{order.ID, otherOrder.ID},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, order.ID, rows[0].OrderID)
	require.Equal(t, expected.ID, rows[0].ProfitSharingOrder.ID)
	require.Equal(t, merchant.ID, rows[0].ProfitSharingOrder.MerchantID)
}

func TestUpdateProfitSharingOrderToFailedDoesNotRegressFinished(t *testing.T) {
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	operator := createRandomOperatorForRegion(t, merchant.RegionID)
	paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)

	order, err := testStore.CreateProfitSharingOrder(context.Background(), CreateProfitSharingOrderParams{
		PaymentOrderID:      paymentOrder.ID,
		MerchantID:          merchant.ID,
		OperatorID:          pgtype.Int8{Int64: operator.ID, Valid: true},
		OrderSource:         "takeout",
		TotalAmount:         10000,
		DeliveryFee:         500,
		RiderID:             pgtype.Int8{Valid: false},
		RiderAmount:         0,
		DistributableAmount: 9500,
		PlatformRate:        2,
		OperatorRate:        3,
		PlatformCommission:  200,
		OperatorCommission:  300,
		MerchantAmount:      9500,
		OutOrderNo:          "pso_guard_" + util.RandomString(16),
		Status:              ProfitSharingOrderStatusProcessing,
	})
	require.NoError(t, err)

	finished, err := testStore.UpdateProfitSharingOrderToFinished(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, ProfitSharingOrderStatusFinished, finished.Status)

	_, err = testStore.UpdateProfitSharingOrderToFailed(context.Background(), order.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)

	current, err := testStore.GetProfitSharingOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, ProfitSharingOrderStatusFinished, current.Status)
}
