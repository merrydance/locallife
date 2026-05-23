package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

type riderWorkbenchStoreStub struct {
	rider                    db.Rider
	riderErr                 error
	activeDeliveries         []db.Delivery
	activeDeliveriesErr      error
	deliveryPoolCount        int64
	deliveryPoolErr          error
	completedCount           int64
	completedCountErr        error
	incomeStats              db.GetRiderProfitSharingStatsRow
	incomeStatsErr           error
	incomeStatusRows         []db.GetRiderProfitSharingStatusSummaryRow
	incomeStatusErr          error
	pendingRefundAmount      int64
	pendingRefundErr         error
	regionRuleConfig         db.RegionRuleConfig
	regionRuleErr            error
	operator                 db.Operator
	operatorErr              error
	platformConfig           db.PlatformConfig
	platformConfigErr        error
	pendingClaimCount        int64
	pendingClaimCountErr     error
	unreadNotificationCount  int64
	unreadNotificationErr    error
	completedDeliveriesRange db.CountRiderCompletedDeliveriesInRangeParams
	paymentOrdersByOrder     map[int64]db.PaymentOrder
	paymentOrderErr          error
	profitSharingByPayment   map[int64]db.ProfitSharingOrder
	profitSharingErr         error
}

func (stub *riderWorkbenchStoreStub) GetRiderByUserID(ctx context.Context, userID int64) (db.Rider, error) {
	return stub.rider, stub.riderErr
}

func (stub *riderWorkbenchStoreStub) ListRiderActiveDeliveries(ctx context.Context, riderID pgtype.Int8) ([]db.Delivery, error) {
	return stub.activeDeliveries, stub.activeDeliveriesErr
}

func (stub *riderWorkbenchStoreStub) GetLatestPaymentOrderByOrder(ctx context.Context, arg db.GetLatestPaymentOrderByOrderParams) (db.PaymentOrder, error) {
	if stub.paymentOrderErr != nil {
		return db.PaymentOrder{}, stub.paymentOrderErr
	}
	if stub.paymentOrdersByOrder != nil {
		if paymentOrder, ok := stub.paymentOrdersByOrder[arg.OrderID.Int64]; ok {
			return paymentOrder, nil
		}
	}
	return db.PaymentOrder{}, db.ErrRecordNotFound
}

func (stub *riderWorkbenchStoreStub) GetProfitSharingOrderByPaymentOrder(ctx context.Context, paymentOrderID int64) (db.ProfitSharingOrder, error) {
	if stub.profitSharingErr != nil {
		return db.ProfitSharingOrder{}, stub.profitSharingErr
	}
	if stub.profitSharingByPayment != nil {
		if profitSharingOrder, ok := stub.profitSharingByPayment[paymentOrderID]; ok {
			return profitSharingOrder, nil
		}
	}
	return db.ProfitSharingOrder{}, db.ErrRecordNotFound
}

func (stub *riderWorkbenchStoreStub) CountDeliveryPool(ctx context.Context) (int64, error) {
	return stub.deliveryPoolCount, stub.deliveryPoolErr
}

func (stub *riderWorkbenchStoreStub) CountRiderCompletedDeliveriesInRange(ctx context.Context, arg db.CountRiderCompletedDeliveriesInRangeParams) (int64, error) {
	stub.completedDeliveriesRange = arg
	return stub.completedCount, stub.completedCountErr
}

func (stub *riderWorkbenchStoreStub) GetRiderProfitSharingStats(ctx context.Context, arg db.GetRiderProfitSharingStatsParams) (db.GetRiderProfitSharingStatsRow, error) {
	return stub.incomeStats, stub.incomeStatsErr
}

func (stub *riderWorkbenchStoreStub) GetRiderProfitSharingStatusSummary(ctx context.Context, arg db.GetRiderProfitSharingStatusSummaryParams) ([]db.GetRiderProfitSharingStatusSummaryRow, error) {
	return stub.incomeStatusRows, stub.incomeStatusErr
}

func (stub *riderWorkbenchStoreStub) GetPendingRiderDepositRefundAmountByUserID(ctx context.Context, userID int64) (int64, error) {
	return stub.pendingRefundAmount, stub.pendingRefundErr
}

func (stub *riderWorkbenchStoreStub) GetRegionRuleConfigByRegion(ctx context.Context, regionID int64) (db.RegionRuleConfig, error) {
	return stub.regionRuleConfig, stub.regionRuleErr
}

func (stub *riderWorkbenchStoreStub) GetActiveOperatorByRegion(ctx context.Context, regionID int64) (db.Operator, error) {
	return stub.operator, stub.operatorErr
}

func (stub *riderWorkbenchStoreStub) GetPlatformConfig(ctx context.Context, arg db.GetPlatformConfigParams) (db.PlatformConfig, error) {
	return stub.platformConfig, stub.platformConfigErr
}

func (stub *riderWorkbenchStoreStub) CountRiderClaimsForRider(ctx context.Context, arg db.CountRiderClaimsForRiderParams) (int64, error) {
	return stub.pendingClaimCount, stub.pendingClaimCountErr
}

func (stub *riderWorkbenchStoreStub) CountUnreadNotifications(ctx context.Context, userID int64) (int64, error) {
	return stub.unreadNotificationCount, stub.unreadNotificationErr
}

func TestRiderWorkbenchServiceGetSummary(t *testing.T) {
	now := time.Date(2026, 4, 28, 14, 30, 0, 0, time.UTC)
	rider := db.Rider{
		ID:            7,
		UserID:        9,
		Status:        db.RiderStatusActive,
		IsOnline:      true,
		DepositAmount: 30000,
		FrozenDeposit: 5000,
		RegionID:      pgtype.Int8{Int64: 1, Valid: true},
	}
	deliveryCreatedAt := now.Add(-time.Hour)
	paymentOrder := db.PaymentOrder{
		ID:                    31,
		OrderID:               pgtype.Int8{Int64: 22, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	store := &riderWorkbenchStoreStub{
		rider: rider,
		activeDeliveries: []db.Delivery{{
			ID:              11,
			OrderID:         22,
			Status:          "delivering",
			DeliveryFee:     800,
			RiderEarnings:   720,
			PickupAddress:   "取餐地址",
			DeliveryAddress: "送达地址",
			CreatedAt:       deliveryCreatedAt,
		}},
		deliveryPoolCount:       3,
		completedCount:          2,
		incomeStats:             db.GetRiderProfitSharingStatsRow{TotalDeliveries: 1, TotalRiderIncome: 900, TotalDeliveryFee: 1000},
		incomeStatusRows:        []db.GetRiderProfitSharingStatusSummaryRow{{Status: db.ProfitSharingOrderStatusPending, OrderCount: 1, RiderAmount: 600}, {Status: db.ProfitSharingOrderStatusFailed, OrderCount: 1, RiderAmount: 300}},
		pendingRefundAmount:     1000,
		operator:                db.Operator{ID: 1, RiderDeposit: 20000},
		pendingClaimCount:       4,
		unreadNotificationCount: 5,
		paymentOrdersByOrder: map[int64]db.PaymentOrder{
			22: paymentOrder,
		},
		profitSharingByPayment: map[int64]db.ProfitSharingOrder{
			paymentOrder.ID: {
				ID:               41,
				PaymentOrderID:   paymentOrder.ID,
				Status:           db.ProfitSharingOrderStatusPending,
				RiderGrossAmount: 800,
				RiderPaymentFee:  5,
				RiderAmount:      795,
			},
		},
	}
	service := NewRiderWorkbenchService(store)
	service.now = func() time.Time { return now }

	result, err := service.GetSummary(context.Background(), rider.UserID)
	require.NoError(t, err)
	require.Equal(t, "delivering", result.RiderStatus.OnlineStatus)
	require.Equal(t, 1, result.RiderStatus.ActiveDeliveries)
	require.True(t, result.RiderStatus.CanGoOnline)
	require.False(t, result.RiderStatus.CanGoOffline)
	require.Equal(t, 1, result.CurrentDeliveries.ActiveCount)
	require.Equal(t, int64(800), result.CurrentDeliveries.Items[0].RiderGrossAmount)
	require.Equal(t, int64(5), result.CurrentDeliveries.Items[0].RiderPaymentFee)
	require.Equal(t, int64(795), result.CurrentDeliveries.Items[0].RiderNetEarnings)
	require.Equal(t, int64(41), result.CurrentDeliveries.Items[0].ProfitSharingOrderID)
	require.Equal(t, db.ProfitSharingOrderStatusPending, result.CurrentDeliveries.Items[0].ProfitSharingStatus)
	require.Equal(t, int64(3), result.OrderPool.AvailableCount)
	require.Equal(t, "2026-04-28", result.Today.Date)
	require.Equal(t, int64(2), result.Today.CompletedDeliveries)
	require.Equal(t, int64(900), result.Income.TotalRiderIncome)
	require.Equal(t, int64(600), result.Income.PendingRiderAmount)
	require.Equal(t, int64(1), result.Income.FailedCount)
	require.Equal(t, int64(25000), result.Deposit.AvailableDeposit)
	require.Equal(t, int64(4), result.Claims.PendingActionCount)
	require.Equal(t, int64(5), result.Notifications.UnreadCount)
	require.Equal(t, pgtype.Timestamptz{Time: time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC), Valid: true}, store.completedDeliveriesRange.StartAt)
	require.True(t, sectionAvailable(result.Sections, RiderWorkbenchSectionIncome))
}

func TestRiderWorkbenchServiceRiderNotFound(t *testing.T) {
	store := &riderWorkbenchStoreStub{riderErr: db.ErrRecordNotFound}
	service := NewRiderWorkbenchService(store)

	_, err := service.GetSummary(context.Background(), 9)
	require.Error(t, err)
	requireRequestErrorStatus(t, err, 404)
}

func TestRiderWorkbenchServiceDegradesOptionalSection(t *testing.T) {
	now := time.Date(2026, 4, 28, 14, 30, 0, 0, time.UTC)
	rider := db.Rider{ID: 7, UserID: 9, Status: db.RiderStatusActive, IsOnline: true, DepositAmount: 30000, RegionID: pgtype.Int8{Int64: 1, Valid: true}}
	store := &riderWorkbenchStoreStub{
		rider:                   rider,
		activeDeliveriesErr:     errors.New("delivery read failed"),
		deliveryPoolCount:       3,
		completedCount:          2,
		incomeStats:             db.GetRiderProfitSharingStatsRow{TotalRiderIncome: 900},
		pendingRefundAmount:     0,
		operator:                db.Operator{ID: 1, RiderDeposit: 20000},
		pendingClaimCount:       4,
		unreadNotificationCount: 5,
	}
	service := NewRiderWorkbenchService(store)
	service.now = func() time.Time { return now }

	result, err := service.GetSummary(context.Background(), rider.UserID)
	require.NoError(t, err)
	require.False(t, sectionAvailable(result.Sections, RiderWorkbenchSectionCurrentDeliveries))
	require.False(t, sectionAvailable(result.Sections, RiderWorkbenchSectionRiderStatus))
	require.True(t, sectionAvailable(result.Sections, RiderWorkbenchSectionOrderPool))
	require.Equal(t, int64(3), result.OrderPool.AvailableCount)
}

func TestRiderWorkbenchServiceDegradesCurrentDeliveriesWhenBillMissing(t *testing.T) {
	now := time.Date(2026, 4, 28, 14, 30, 0, 0, time.UTC)
	rider := db.Rider{
		ID:            7,
		UserID:        9,
		Status:        db.RiderStatusActive,
		IsOnline:      true,
		DepositAmount: 30000,
		RegionID:      pgtype.Int8{Int64: 1, Valid: true},
	}
	paymentOrder := db.PaymentOrder{
		ID:                    31,
		OrderID:               pgtype.Int8{Int64: 22, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	store := &riderWorkbenchStoreStub{
		rider: rider,
		activeDeliveries: []db.Delivery{{
			ID:              11,
			OrderID:         22,
			Status:          "delivering",
			DeliveryFee:     800,
			RiderEarnings:   720,
			PickupAddress:   "取餐地址",
			DeliveryAddress: "送达地址",
			CreatedAt:       now.Add(-time.Hour),
		}},
		deliveryPoolCount:       3,
		completedCount:          2,
		incomeStats:             db.GetRiderProfitSharingStatsRow{TotalRiderIncome: 900},
		pendingRefundAmount:     0,
		operator:                db.Operator{ID: 1, RiderDeposit: 20000},
		pendingClaimCount:       4,
		unreadNotificationCount: 5,
		paymentOrdersByOrder: map[int64]db.PaymentOrder{
			22: paymentOrder,
		},
	}
	service := NewRiderWorkbenchService(store)
	service.now = func() time.Time { return now }

	result, err := service.GetSummary(context.Background(), rider.UserID)
	require.NoError(t, err)
	require.Equal(t, 1, result.CurrentDeliveries.ActiveCount)
	require.Empty(t, result.CurrentDeliveries.Items[0].ProfitSharingStatus)
	require.False(t, sectionAvailable(result.Sections, RiderWorkbenchSectionCurrentDeliveries))
	require.Equal(t, "当前任务收益账单暂不可用，请稍后重试", sectionMessage(result.Sections, RiderWorkbenchSectionCurrentDeliveries))
}

func sectionAvailable(sections []RiderWorkbenchSectionStatus, section string) bool {
	for _, item := range sections {
		if item.Section == section {
			return item.Available
		}
	}
	return false
}

func sectionMessage(sections []RiderWorkbenchSectionStatus, section string) string {
	for _, item := range sections {
		if item.Section == section {
			return item.Message
		}
	}
	return ""
}

func requireRequestErrorStatus(t *testing.T, err error, status int) {
	t.Helper()
	var requestErr *RequestError
	require.ErrorAs(t, err, &requestErr)
	require.Equal(t, status, requestErr.Status)
}
