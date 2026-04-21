package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func setRiderDepositThresholdForTest(t *testing.T, amountFen int64) {
	t.Helper()

	payload, err := json.Marshal(map[string]int64{"amount_fen": amountFen})
	require.NoError(t, err)

	_, err = testStore.UpsertPlatformConfig(context.Background(), UpsertPlatformConfigParams{
		ConfigKey:   PlatformConfigKeyRiderDepositFen,
		ConfigValue: payload,
		ScopeType:   PlatformConfigScopeGlobal,
		ScopeID:     pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)
}

func createPaidRiderDepositPaymentOrder(t *testing.T, rider Rider, amount int64) PaymentOrder {
	if amount <= 0 {
		amount = 1
	}

	paymentOrder, err := testStore.CreatePaymentOrder(context.Background(), CreatePaymentOrderParams{
		UserID:                rider.UserID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelDirect,
		RequiresProfitSharing: false,
		BusinessType:          "rider_deposit",
		Amount:                amount,
		OutTradeNo:            "RD" + util.RandomString(30),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	paymentOrder, err = testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: "TX" + util.RandomString(28), Valid: true},
	})
	require.NoError(t, err)

	return paymentOrder
}

func TestProcessPaymentSuccessTx_RiderDepositCreatesCredit(t *testing.T) {
	setRiderDepositThresholdForTest(t, DefaultRiderDepositThresholdFen)

	rider := createRandomRider(t)
	paymentOrder := createPaidRiderDepositPaymentOrder(t, rider, 30000)

	result, err := testStore.(*SQLStore).ProcessPaymentSuccessTx(context.Background(), ProcessPaymentSuccessTxParams{
		PaymentOrderID: paymentOrder.ID,
	})
	require.NoError(t, err)
	require.True(t, result.Processed)
	require.True(t, result.PaymentOrder.ProcessedAt.Valid)

	updatedRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, paymentOrder.Amount, updatedRider.DepositAmount)
	require.Equal(t, RiderStatusActive, updatedRider.Status)

	depositLog, err := testStore.GetRiderDepositByPaymentOrderID(context.Background(), pgtype.Int8{Int64: paymentOrder.ID, Valid: true})
	require.NoError(t, err)
	require.Equal(t, rider.ID, depositLog.RiderID)
	require.Equal(t, paymentOrder.Amount, depositLog.Amount)
	require.Equal(t, "deposit", depositLog.Type)
	require.Equal(t, paymentOrder.Amount, depositLog.BalanceAfter)

	credit, err := testStore.GetRiderDepositCreditByPaymentOrderID(context.Background(), paymentOrder.ID)
	require.NoError(t, err)
	require.Equal(t, rider.ID, credit.RiderID)
	require.Equal(t, paymentOrder.ID, credit.PaymentOrderID)
	require.Equal(t, paymentOrder.Amount, credit.OriginalAmount)
	require.Equal(t, paymentOrder.Amount, credit.RefundableAmount)
	require.Equal(t, int64(0), credit.RefundedAmount)
	require.Equal(t, riderDepositCreditStatusActive, credit.Status)
	require.WithinDuration(t, paymentOrder.PaidAt.Time, credit.PaidAt, time.Second)
	require.WithinDuration(t, paymentOrder.PaidAt.Time.Add(riderDepositRefundWindow), credit.RefundableUntil, time.Second)
}

func TestProcessPaymentSuccessTx_RiderDepositIsIdempotent(t *testing.T) {
	setRiderDepositThresholdForTest(t, DefaultRiderDepositThresholdFen)

	rider := createRandomRider(t)
	paymentOrder := createPaidRiderDepositPaymentOrder(t, rider, 28000)

	firstResult, err := testStore.(*SQLStore).ProcessPaymentSuccessTx(context.Background(), ProcessPaymentSuccessTxParams{
		PaymentOrderID: paymentOrder.ID,
	})
	require.NoError(t, err)
	require.True(t, firstResult.Processed)

	secondResult, err := testStore.(*SQLStore).ProcessPaymentSuccessTx(context.Background(), ProcessPaymentSuccessTxParams{
		PaymentOrderID: paymentOrder.ID,
	})
	require.NoError(t, err)
	require.False(t, secondResult.Processed)

	updatedRider, err := testStore.GetRider(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, paymentOrder.Amount, updatedRider.DepositAmount)
	require.Equal(t, RiderStatusActive, updatedRider.Status)

	deposits, err := testStore.ListRiderDeposits(context.Background(), ListRiderDepositsParams{
		RiderID: rider.ID,
		Limit:   20,
		Offset:  0,
	})
	require.NoError(t, err)

	matchedDepositCount := 0
	for _, deposit := range deposits {
		if deposit.PaymentOrderID.Valid && deposit.PaymentOrderID.Int64 == paymentOrder.ID {
			matchedDepositCount++
		}
	}
	require.Equal(t, 1, matchedDepositCount)

	credit, err := testStore.GetRiderDepositCreditByPaymentOrderID(context.Background(), paymentOrder.ID)
	require.NoError(t, err)
	require.Equal(t, paymentOrder.Amount, credit.RefundableAmount)
	require.Equal(t, int64(0), credit.RefundedAmount)
}

func TestProcessPaymentSuccessTx_OrderSetsPaidFields(t *testing.T) {
	user := createRandomUser(t)
	merchantOwner := createRandomUser(t)
	merchant := createMerchantWithLocation(t, merchantOwner.ID)
	address := createRandomUserAddress(t, user)

	createResult, err := testStore.CreateOrderTx(context.Background(), CreateOrderTxParams{
		CreateOrderParams: CreateOrderParams{
			OrderNo:          util.RandomString(20),
			UserID:           user.ID,
			MerchantID:       merchant.ID,
			OrderType:        OrderTypeTakeout,
			AddressID:        pgtype.Int8{Int64: address.ID, Valid: true},
			DeliveryFee:      800,
			DeliveryDistance: pgtype.Int4{Int32: 2500, Valid: true},
			Subtotal:         5000,
			TotalAmount:      5800,
			Status:           OrderStatusPending,
		},
		Items: []CreateOrderItemParams{{
			DishID:    pgtype.Int8{Int64: 1, Valid: true},
			Name:      "支付成功外卖测试菜品",
			UnitPrice: 5000,
			Quantity:  1,
			Subtotal:  5000,
		}},
	})
	require.NoError(t, err)
	order := createResult.Order

	paymentOrder, err := testStore.CreatePaymentOrder(context.Background(), CreatePaymentOrderParams{
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelEcommerce,
		RequiresProfitSharing: true,
		BusinessType:          "order",
		Amount:                order.TotalAmount,
		OutTradeNo:            "PO" + util.RandomString(30),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	paymentOrder, err = testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: "TX" + util.RandomString(28), Valid: true},
	})
	require.NoError(t, err)

	result, err := testStore.(*SQLStore).ProcessPaymentSuccessTx(context.Background(), ProcessPaymentSuccessTxParams{
		PaymentOrderID: paymentOrder.ID,
	})
	require.NoError(t, err)
	require.True(t, result.Processed)
	require.NotNil(t, result.OrderResult)

	updatedOrder, err := testStore.GetOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPaid, updatedOrder.Status)
	require.True(t, updatedOrder.PaymentMethod.Valid)
	require.Equal(t, "wechat", updatedOrder.PaymentMethod.String)
	require.True(t, updatedOrder.PaidAt.Valid)

	_, err = testStore.GetDeliveryByOrderID(context.Background(), order.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)

	_, err = testStore.GetDeliveryPoolByOrderID(context.Background(), order.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestProcessPaymentSuccessTx_ClaimRecoveryKeepsMerchantSuspendedWhenAnotherBlockingRecoveryExists(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user1 := createRandomUser(t)
	user2 := createRandomUser(t)

	order1 := createCompletedOrderForStats(t, user1.ID, merchant.ID, 10000, OrderTypeTakeout, time.Now())
	claim1 := createRandomClaim(t, user1.ID, order1.ID)
	recovery1 := createRandomClaimRecovery(t, claim1.ID, order1.ID, "pending")

	order2 := createCompletedOrderForStats(t, user2.ID, merchant.ID, 12000, OrderTypeTakeout, time.Now())
	claim2 := createRandomClaim(t, user2.ID, order2.ID)
	_ = createRandomClaimRecovery(t, claim2.ID, order2.ID, "overdue")

	_, err := testStore.CreateMerchantProfile(context.Background(), merchant.ID)
	require.NoError(t, err)

	err = testStore.SuspendMerchantTakeout(context.Background(), SuspendMerchantTakeoutParams{
		MerchantID:           merchant.ID,
		TakeoutSuspendReason: pgtype.Text{String: "claim recovery overdue", Valid: true},
		TakeoutSuspendUntil:  pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	require.NoError(t, err)

	attach, err := json.Marshal(map[string]any{
		"claim_id":        claim1.ID,
		"recovery_id":     recovery1.ID,
		"recovery_target": "merchant",
	})
	require.NoError(t, err)

	paymentOrder, err := testStore.CreatePaymentOrder(context.Background(), CreatePaymentOrderParams{
		OrderID:               pgtype.Int8{Int64: order1.ID, Valid: true},
		UserID:                owner.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelDirect,
		RequiresProfitSharing: false,
		BusinessType:          "claim_recovery",
		Amount:                recovery1.RecoveryAmount,
		OutTradeNo:            "CR" + util.RandomString(30),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
		Attach:                pgtype.Text{String: string(attach), Valid: true},
	})
	require.NoError(t, err)

	paymentOrder, err = testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: "TX" + util.RandomString(28), Valid: true},
	})
	require.NoError(t, err)

	result, err := testStore.(*SQLStore).ProcessPaymentSuccessTx(context.Background(), ProcessPaymentSuccessTxParams{
		PaymentOrderID: paymentOrder.ID,
	})
	require.NoError(t, err)
	require.True(t, result.Processed)
	require.NotNil(t, result.ReleaseAction)
	require.Equal(t, "release", result.ReleaseAction.ActionType)
	require.Equal(t, "merchant", result.ReleaseAction.TargetEntity)

	updatedRecovery, err := testStore.GetClaimRecoveryByClaimID(context.Background(), claim1.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", updatedRecovery.Status)
	recoveryEvents, err := testStore.ListClaimRecoveryEventsByRecovery(context.Background(), updatedRecovery.ID)
	require.NoError(t, err)
	require.Len(t, recoveryEvents, 1)
	require.Equal(t, ClaimRecoveryEventTypePaid, recoveryEvents[0].EventType)

	updatedMerchant, err := testStore.GetMerchantProfile(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.True(t, updatedMerchant.IsTakeoutSuspended)
	require.True(t, updatedMerchant.TakeoutSuspendReason.Valid)
	require.Equal(t, "claim recovery overdue", updatedMerchant.TakeoutSuspendReason.String)
}
