package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

const (
	RiderWorkbenchSectionRiderStatus       = "rider_status"
	RiderWorkbenchSectionCurrentDeliveries = "current_deliveries"
	RiderWorkbenchSectionOrderPool         = "order_pool"
	RiderWorkbenchSectionToday             = "today"
	RiderWorkbenchSectionIncome            = "income"
	RiderWorkbenchSectionDeposit           = "deposit"
	RiderWorkbenchSectionClaims            = "claims"
	RiderWorkbenchSectionNotifications     = "notifications"
)

type RiderWorkbenchStore interface {
	GetRiderByUserID(ctx context.Context, userID int64) (db.Rider, error)
	ListRiderActiveDeliveries(ctx context.Context, riderID pgtype.Int8) ([]db.Delivery, error)
	CountDeliveryPool(ctx context.Context) (int64, error)
	CountRiderCompletedDeliveriesInRange(ctx context.Context, arg db.CountRiderCompletedDeliveriesInRangeParams) (int64, error)
	GetRiderProfitSharingStats(ctx context.Context, arg db.GetRiderProfitSharingStatsParams) (db.GetRiderProfitSharingStatsRow, error)
	GetRiderProfitSharingStatusSummary(ctx context.Context, arg db.GetRiderProfitSharingStatusSummaryParams) ([]db.GetRiderProfitSharingStatusSummaryRow, error)
	GetPendingRiderDepositRefundAmountByUserID(ctx context.Context, userID int64) (int64, error)
	GetRegionRuleConfigByRegion(ctx context.Context, regionID int64) (db.RegionRuleConfig, error)
	GetPlatformConfig(ctx context.Context, arg db.GetPlatformConfigParams) (db.PlatformConfig, error)
	CountRiderClaimsForRider(ctx context.Context, arg db.CountRiderClaimsForRiderParams) (int64, error)
	CountUnreadNotifications(ctx context.Context, userID int64) (int64, error)
}

type RiderWorkbenchService struct {
	store RiderWorkbenchStore
	now   func() time.Time
}

func NewRiderWorkbenchService(store RiderWorkbenchStore) *RiderWorkbenchService {
	return &RiderWorkbenchService{store: store, now: time.Now}
}

type RiderWorkbenchSummary struct {
	RiderStatus       RiderWorkbenchRiderStatus
	CurrentDeliveries RiderWorkbenchCurrentDeliveries
	OrderPool         RiderWorkbenchOrderPool
	Today             RiderWorkbenchToday
	Income            RiderWorkbenchIncome
	Deposit           RiderWorkbenchDeposit
	Claims            RiderWorkbenchClaims
	Notifications     RiderWorkbenchNotifications
	Sections          []RiderWorkbenchSectionStatus
}

type RiderWorkbenchSectionStatus struct {
	Section   string
	Available bool
	Message   string
}

type RiderWorkbenchRiderStatus struct {
	Status            string
	IsOnline          bool
	OnlineStatus      string
	ActiveDeliveries  int
	CurrentLongitude  *float64
	CurrentLatitude   *float64
	LocationUpdatedAt *time.Time
	CanGoOnline       bool
	CanGoOffline      bool
	OnlineBlockReason string
}

type RiderWorkbenchCurrentDeliveries struct {
	ActiveCount int
	Items       []RiderWorkbenchDeliveryItem
}

type RiderWorkbenchDeliveryItem struct {
	ID                  int64
	OrderID             int64
	Status              string
	DeliveryFee         int64
	RiderEarnings       int64
	PickupAddress       string
	DeliveryAddress     string
	EstimatedPickupAt   *time.Time
	EstimatedDeliveryAt *time.Time
	PickedAt            *time.Time
	DeliveredAt         *time.Time
	CreatedAt           time.Time
}

type RiderWorkbenchOrderPool struct {
	AvailableCount int64
}

type RiderWorkbenchToday struct {
	Date                string
	CompletedDeliveries int64
}

type RiderWorkbenchIncome struct {
	TotalDeliveries       int64
	TotalRiderIncome      int64
	TotalDeliveryFee      int64
	PendingRiderAmount    int64
	ProcessingRiderAmount int64
	FailedCount           int64
}

type RiderWorkbenchDeposit struct {
	TotalDeposit                  int64
	FrozenDeposit                 int64
	DeliveryFrozenDeposit         int64
	DepositRefundProcessingAmount int64
	AvailableDeposit              int64
	ThresholdAmount               int64
}

type RiderWorkbenchClaims struct {
	PendingActionCount int64
}

type RiderWorkbenchNotifications struct {
	UnreadCount int64
}

func (service *RiderWorkbenchService) GetSummary(ctx context.Context, userID int64) (RiderWorkbenchSummary, error) {
	rider, err := service.store.GetRiderByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return RiderWorkbenchSummary{}, NewRequestError(http.StatusNotFound, errors.New("rider not found"))
		}
		return RiderWorkbenchSummary{}, err
	}
	if !rider.RegionID.Valid || rider.RegionID.Int64 <= 0 {
		return RiderWorkbenchSummary{}, NewRequestError(http.StatusBadRequest, errors.New("当前定位区域未同步，请重新进入骑手中心"))
	}

	result := RiderWorkbenchSummary{
		RiderStatus: newRiderWorkbenchRiderStatus(rider),
		Sections:    defaultRiderWorkbenchSections(),
	}
	riderID := pgtype.Int8{Int64: rider.ID, Valid: true}
	now := service.now().UTC()
	startAt := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endAt := startAt.AddDate(0, 0, 1)

	activeDeliveries, err := service.store.ListRiderActiveDeliveries(ctx, riderID)
	if err != nil {
		result.degrade(RiderWorkbenchSectionRiderStatus, "骑手状态暂不可用")
		result.degrade(RiderWorkbenchSectionCurrentDeliveries, "当前任务暂不可用")
		log.Warn().Err(err).Int64("rider_id", rider.ID).Msg("rider workbench active deliveries degraded")
	} else {
		result.RiderStatus.ActiveDeliveries = len(activeDeliveries)
		result.RiderStatus.OnlineStatus = riderWorkbenchOnlineStatus(rider.IsOnline, len(activeDeliveries))
		result.RiderStatus.CanGoOffline = rider.IsOnline && len(activeDeliveries) == 0
		result.CurrentDeliveries = RiderWorkbenchCurrentDeliveries{
			ActiveCount: len(activeDeliveries),
			Items:       newRiderWorkbenchDeliveryItems(activeDeliveries),
		}
	}

	service.loadDeposit(ctx, userID, rider, &result)
	service.loadOrderPool(ctx, rider.ID, &result)
	service.loadToday(ctx, riderID, startAt, endAt, &result)
	service.loadIncome(ctx, riderID, startAt, endAt, &result)
	service.loadClaims(ctx, riderID, rider.ID, &result)
	service.loadNotifications(ctx, userID, rider.ID, &result)

	return result, nil
}

func (service *RiderWorkbenchService) loadDeposit(ctx context.Context, userID int64, rider db.Rider, result *RiderWorkbenchSummary) {
	withdrawalProcessingAmount, err := service.store.GetPendingRiderDepositRefundAmountByUserID(ctx, userID)
	if err != nil {
		result.degrade(RiderWorkbenchSectionDeposit, "押金摘要暂不可用")
		result.degrade(RiderWorkbenchSectionRiderStatus, "上线条件暂不可用")
		log.Warn().Err(err).Int64("rider_id", rider.ID).Msg("rider workbench deposit pending refund degraded")
		return
	}

	threshold, err := db.GetEffectiveRiderDepositThreshold(ctx, service.store, rider.RegionID)
	if err != nil {
		result.degrade(RiderWorkbenchSectionDeposit, "押金摘要暂不可用")
		result.degrade(RiderWorkbenchSectionRiderStatus, "上线条件暂不可用")
		log.Warn().Err(err).Int64("rider_id", rider.ID).Msg("rider workbench deposit threshold degraded")
		return
	}

	availability := db.CalculateRiderDepositAvailability(rider, withdrawalProcessingAmount)
	result.Deposit = RiderWorkbenchDeposit{
		TotalDeposit:                  rider.DepositAmount,
		FrozenDeposit:                 rider.FrozenDeposit,
		DeliveryFrozenDeposit:         availability.DeliveryFrozenDeposit,
		DepositRefundProcessingAmount: availability.WithdrawalProcessingAmount,
		AvailableDeposit:              availability.AvailableDeposit,
		ThresholdAmount:               threshold,
	}
	applyRiderWorkbenchOnlineEligibility(&result.RiderStatus, rider, availability.AvailableDeposit, threshold)
}

func (service *RiderWorkbenchService) loadOrderPool(ctx context.Context, riderID int64, result *RiderWorkbenchSummary) {
	count, err := service.store.CountDeliveryPool(ctx)
	if err != nil {
		result.degrade(RiderWorkbenchSectionOrderPool, "订单池暂不可用")
		log.Warn().Err(err).Int64("rider_id", riderID).Msg("rider workbench order pool degraded")
		return
	}
	result.OrderPool.AvailableCount = count
}

func (service *RiderWorkbenchService) loadToday(ctx context.Context, riderID pgtype.Int8, startAt, endAt time.Time, result *RiderWorkbenchSummary) {
	count, err := service.store.CountRiderCompletedDeliveriesInRange(ctx, db.CountRiderCompletedDeliveriesInRangeParams{
		RiderID: riderID,
		StartAt: pgtype.Timestamptz{Time: startAt, Valid: true},
		EndAt:   pgtype.Timestamptz{Time: endAt, Valid: true},
	})
	if err != nil {
		result.degrade(RiderWorkbenchSectionToday, "今日配送摘要暂不可用")
		log.Warn().Err(err).Int64("rider_id", riderID.Int64).Msg("rider workbench today summary degraded")
		return
	}
	result.Today = RiderWorkbenchToday{Date: startAt.Format("2006-01-02"), CompletedDeliveries: count}
}

func (service *RiderWorkbenchService) loadIncome(ctx context.Context, riderID pgtype.Int8, startAt, endAt time.Time, result *RiderWorkbenchSummary) {
	stats, err := service.store.GetRiderProfitSharingStats(ctx, db.GetRiderProfitSharingStatsParams{
		RiderID: riderID,
		StartAt: startAt,
		EndAt:   endAt.Add(-time.Nanosecond),
	})
	if err != nil {
		result.degrade(RiderWorkbenchSectionIncome, "配送费结算暂不可用")
		log.Warn().Err(err).Int64("rider_id", riderID.Int64).Msg("rider workbench income stats degraded")
		return
	}

	statusRows, err := service.store.GetRiderProfitSharingStatusSummary(ctx, db.GetRiderProfitSharingStatusSummaryParams{
		RiderID: riderID,
		StartAt: startAt,
		EndAt:   endAt.Add(-time.Nanosecond),
	})
	if err != nil {
		result.degrade(RiderWorkbenchSectionIncome, "配送费结算暂不可用")
		log.Warn().Err(err).Int64("rider_id", riderID.Int64).Msg("rider workbench income status summary degraded")
		return
	}

	result.Income = newRiderWorkbenchIncome(stats, statusRows)
}

func (service *RiderWorkbenchService) loadClaims(ctx context.Context, riderID pgtype.Int8, riderIDValue int64, result *RiderWorkbenchSummary) {
	count, err := service.store.CountRiderClaimsForRider(ctx, db.CountRiderClaimsForRiderParams{
		RiderID: riderID,
		Bucket:  pgtype.Text{String: "pending_action", Valid: true},
	})
	if err != nil {
		result.degrade(RiderWorkbenchSectionClaims, "追偿待处理摘要暂不可用")
		log.Warn().Err(err).Int64("rider_id", riderIDValue).Msg("rider workbench claims degraded")
		return
	}
	result.Claims.PendingActionCount = count
}

func (service *RiderWorkbenchService) loadNotifications(ctx context.Context, userID, riderID int64, result *RiderWorkbenchSummary) {
	count, err := service.store.CountUnreadNotifications(ctx, userID)
	if err != nil {
		result.degrade(RiderWorkbenchSectionNotifications, "未读通知暂不可用")
		log.Warn().Err(err).Int64("rider_id", riderID).Int64("user_id", userID).Msg("rider workbench notifications degraded")
		return
	}
	result.Notifications.UnreadCount = count
}

func defaultRiderWorkbenchSections() []RiderWorkbenchSectionStatus {
	sections := []string{
		RiderWorkbenchSectionRiderStatus,
		RiderWorkbenchSectionCurrentDeliveries,
		RiderWorkbenchSectionOrderPool,
		RiderWorkbenchSectionToday,
		RiderWorkbenchSectionIncome,
		RiderWorkbenchSectionDeposit,
		RiderWorkbenchSectionClaims,
		RiderWorkbenchSectionNotifications,
	}
	statuses := make([]RiderWorkbenchSectionStatus, 0, len(sections))
	for _, section := range sections {
		statuses = append(statuses, RiderWorkbenchSectionStatus{Section: section, Available: true})
	}
	return statuses
}

func (summary *RiderWorkbenchSummary) degrade(section, message string) {
	for index := range summary.Sections {
		if summary.Sections[index].Section == section {
			summary.Sections[index].Available = false
			summary.Sections[index].Message = message
			return
		}
	}
	summary.Sections = append(summary.Sections, RiderWorkbenchSectionStatus{Section: section, Available: false, Message: message})
}

func newRiderWorkbenchRiderStatus(rider db.Rider) RiderWorkbenchRiderStatus {
	status := RiderWorkbenchRiderStatus{
		Status:       rider.Status,
		IsOnline:     rider.IsOnline,
		OnlineStatus: riderWorkbenchOnlineStatus(rider.IsOnline, 0),
	}
	if rider.CurrentLongitude.Valid {
		longitude, _ := rider.CurrentLongitude.Float64Value()
		status.CurrentLongitude = &longitude.Float64
	}
	if rider.CurrentLatitude.Valid {
		latitude, _ := rider.CurrentLatitude.Float64Value()
		status.CurrentLatitude = &latitude.Float64
	}
	if rider.LocationUpdatedAt.Valid {
		timestamp := rider.LocationUpdatedAt.Time
		status.LocationUpdatedAt = &timestamp
	}
	return status
}

func riderWorkbenchOnlineStatus(isOnline bool, activeDeliveries int) string {
	if !isOnline {
		return "offline"
	}
	if activeDeliveries > 0 {
		return "delivering"
	}
	return "online"
}

func applyRiderWorkbenchOnlineEligibility(status *RiderWorkbenchRiderStatus, rider db.Rider, availableDeposit, threshold int64) {
	if rider.Status == db.RiderStatusSuspended {
		status.CanGoOnline = false
		status.OnlineBlockReason = "账号已停用"
		return
	}
	if rider.Status != db.RiderStatusActive {
		status.CanGoOnline = false
		status.OnlineBlockReason = fmt.Sprintf("押金不足，需要至少%s元", riderWorkbenchFenToYuanString(threshold))
		return
	}
	if availableDeposit < threshold {
		status.CanGoOnline = false
		status.OnlineBlockReason = fmt.Sprintf("可用押金不足，需要至少%s元", riderWorkbenchFenToYuanString(threshold))
		return
	}
	status.CanGoOnline = true
	status.OnlineBlockReason = ""
}

func riderWorkbenchFenToYuanString(fen int64) string {
	yuan := fen / 100
	cents := fen % 100
	if cents == 0 {
		return fmt.Sprintf("%d", yuan)
	}
	return fmt.Sprintf("%d.%02d", yuan, cents)
}

func newRiderWorkbenchDeliveryItems(deliveries []db.Delivery) []RiderWorkbenchDeliveryItem {
	items := make([]RiderWorkbenchDeliveryItem, 0, len(deliveries))
	for _, delivery := range deliveries {
		items = append(items, RiderWorkbenchDeliveryItem{
			ID:                  delivery.ID,
			OrderID:             delivery.OrderID,
			Status:              delivery.Status,
			DeliveryFee:         delivery.DeliveryFee,
			RiderEarnings:       delivery.RiderEarnings,
			PickupAddress:       delivery.PickupAddress,
			DeliveryAddress:     delivery.DeliveryAddress,
			EstimatedPickupAt:   pgTimestamptzPtr(delivery.EstimatedPickupAt),
			EstimatedDeliveryAt: pgTimestamptzPtr(delivery.EstimatedDeliveryAt),
			PickedAt:            pgTimestamptzPtr(delivery.PickedAt),
			DeliveredAt:         pgTimestamptzPtr(delivery.DeliveredAt),
			CreatedAt:           delivery.CreatedAt,
		})
	}
	return items
}

func newRiderWorkbenchIncome(stats db.GetRiderProfitSharingStatsRow, statusRows []db.GetRiderProfitSharingStatusSummaryRow) RiderWorkbenchIncome {
	income := RiderWorkbenchIncome{
		TotalDeliveries:  stats.TotalDeliveries,
		TotalRiderIncome: stats.TotalRiderIncome,
		TotalDeliveryFee: stats.TotalDeliveryFee,
	}
	for _, row := range completeRiderIncomeStatusSummary(statusRows) {
		switch row.Status {
		case db.ProfitSharingOrderStatusPending:
			income.PendingRiderAmount = row.RiderAmount
		case db.ProfitSharingOrderStatusProcessing:
			income.ProcessingRiderAmount = row.RiderAmount
		case db.ProfitSharingOrderStatusFailed:
			income.FailedCount = row.OrderCount
		}
	}
	return income
}

func pgTimestamptzPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	timestamp := value.Time
	return &timestamp
}
