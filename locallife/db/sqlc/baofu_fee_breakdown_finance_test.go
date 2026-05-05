package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestMerchantFinanceQueriesUseMerchantPaymentFeeBreakdown(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	operator := createRandomOperatorForRegion(t, merchant.RegionID)
	paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)
	startAt := time.Now().Add(-time.Hour)
	endAt := time.Now().Add(time.Hour)

	order, err := testStore.CreateProfitSharingOrder(ctx, CreateProfitSharingOrderParams{
		PaymentOrderID:        paymentOrder.ID,
		MerchantID:            merchant.ID,
		OperatorID:            pgtype.Int8{Int64: operator.ID, Valid: true},
		OrderSource:           "reservation",
		TotalAmount:           10000,
		DeliveryFee:           0,
		DistributableAmount:   10000,
		PlatformRate:          200,
		OperatorRate:          300,
		PlatformCommission:    200,
		OperatorCommission:    300,
		MerchantAmount:        9440,
		OutOrderNo:            "pso_fee_finance_" + util.RandomString(16),
		Status:                ProfitSharingOrderStatusFinished,
		PaymentFee:            30,
		PaymentFeeRateBps:     30,
		Provider:              ExternalPaymentProviderBaofu,
		Channel:               PaymentChannelBaofuAggregate,
		SharingDetailSnapshot: []byte(`{"receivers":[]}`),
	})
	require.NoError(t, err)
	_, err = testStore.UpdateProfitSharingOrderFeeBreakdown(ctx, UpdateProfitSharingOrderFeeBreakdownParams{
		ID:                           order.ID,
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
	})
	require.NoError(t, err)

	overview, err := testStore.GetMerchantFinanceOverview(ctx, GetMerchantFinanceOverviewParams{
		MerchantID: merchant.ID,
		StartAt:    startAt,
		EndAt:      endAt,
	})
	require.NoError(t, err)
	require.Equal(t, int64(60), overview.TotalPaymentFee)

	serviceFees, err := testStore.GetMerchantServiceFeeDetail(ctx, GetMerchantServiceFeeDetailParams{
		MerchantID: merchant.ID,
		StartAt:    startAt,
		EndAt:      endAt,
	})
	require.NoError(t, err)
	require.NotEmpty(t, serviceFees)
	require.Equal(t, int64(60), serviceFees[0].PaymentFee)

	daily, err := testStore.GetMerchantDailyFinance(ctx, GetMerchantDailyFinanceParams{
		MerchantID: merchant.ID,
		StartAt:    startAt,
		EndAt:      endAt,
	})
	require.NoError(t, err)
	require.NotEmpty(t, daily)
	require.Equal(t, int64(60), daily[0].PaymentFee)
	require.Equal(t, int64(560), daily[0].TotalDeductionFee)

	rows, err := testStore.ListMerchantFinanceOrders(ctx, ListMerchantFinanceOrdersParams{
		MerchantID: merchant.ID,
		StartAt:    startAt,
		EndAt:      endAt,
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.Equal(t, int64(60), rows[0].PaymentFee)
}

func TestRiderFinanceQueriesExposeGrossAmountAndPaymentFee(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	operator := createRandomOperatorForRegion(t, merchant.RegionID)
	rider := createRandomRider(t)
	paymentOrder := createRandomPaymentOrder(t, createRandomUser(t).ID)
	startAt := time.Now().Add(-time.Hour)
	endAt := time.Now().Add(time.Hour)
	riderID := pgtype.Int8{Int64: rider.ID, Valid: true}

	order, err := testStore.CreateProfitSharingOrder(ctx, CreateProfitSharingOrderParams{
		PaymentOrderID:        paymentOrder.ID,
		MerchantID:            merchant.ID,
		OperatorID:            pgtype.Int8{Int64: operator.ID, Valid: true},
		OrderSource:           "takeout",
		TotalAmount:           10000,
		DeliveryFee:           500,
		RiderID:               riderID,
		RiderAmount:           497,
		DistributableAmount:   9500,
		PlatformRate:          200,
		OperatorRate:          300,
		PlatformCommission:    190,
		OperatorCommission:    285,
		MerchantAmount:        8968,
		OutOrderNo:            "pso_rider_fee_" + util.RandomString(16),
		Status:                ProfitSharingOrderStatusFinished,
		PaymentFee:            30,
		PaymentFeeRateBps:     30,
		Provider:              ExternalPaymentProviderBaofu,
		Channel:               PaymentChannelBaofuAggregate,
		SharingDetailSnapshot: []byte(`{"receivers":[]}`),
	})
	require.NoError(t, err)
	_, err = testStore.UpdateProfitSharingOrderFeeBreakdown(ctx, UpdateProfitSharingOrderFeeBreakdownParams{
		ID:                           order.ID,
		CalculationVersion:           "baofu_fee_v2",
		SettlementMode:               ProfitSharingSettlementModeCommissionShare,
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
	})
	require.NoError(t, err)

	stats, err := testStore.GetRiderProfitSharingStats(ctx, GetRiderProfitSharingStatsParams{
		RiderID: riderID,
		StartAt: startAt,
		EndAt:   endAt,
	})
	require.NoError(t, err)
	require.Equal(t, int64(497), stats.TotalRiderIncome)
	require.Equal(t, int64(500), stats.TotalRiderGrossAmount)
	require.Equal(t, int64(3), stats.TotalRiderPaymentFee)

	statusRows, err := testStore.GetRiderProfitSharingStatusSummary(ctx, GetRiderProfitSharingStatusSummaryParams{
		RiderID: riderID,
		StartAt: startAt,
		EndAt:   endAt,
	})
	require.NoError(t, err)
	require.Len(t, statusRows, 1)
	require.Equal(t, int64(3), statusRows[0].RiderPaymentFee)

	daily, err := testStore.GetRiderDailyIncome(ctx, GetRiderDailyIncomeParams{
		RiderID: riderID,
		StartAt: startAt,
		EndAt:   endAt,
	})
	require.NoError(t, err)
	require.NotEmpty(t, daily)
	require.Equal(t, int64(497), daily[0].DailyIncome)
	require.Equal(t, int64(500), daily[0].RiderGrossAmount)
	require.Equal(t, int64(3), daily[0].RiderPaymentFee)
}
