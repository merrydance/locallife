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
}

func (stub *riderWorkbenchStoreStub) GetRiderByUserID(ctx context.Context, userID int64) (db.Rider, error) {
	return stub.rider, stub.riderErr
}

func (stub *riderWorkbenchStoreStub) ListRiderActiveDeliveries(ctx context.Context, riderID pgtype.Int8) ([]db.Delivery, error) {
	return stub.activeDeliveries, stub.activeDeliveriesErr
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

func sectionAvailable(sections []RiderWorkbenchSectionStatus, section string) bool {
	for _, item := range sections {
		if item.Section == section {
			return item.Available
		}
	}
	return false
}

func requireRequestErrorStatus(t *testing.T, err error, status int) {
	t.Helper()
	var requestErr *RequestError
	require.ErrorAs(t, err, &requestErr)
	require.Equal(t, status, requestErr.Status)
}
