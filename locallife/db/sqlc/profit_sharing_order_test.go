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
