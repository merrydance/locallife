package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/worker"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	riderDepositCreditReminderBatchLimit = int32(200)
	riderDepositCreditExpireBatchLimit   = int32(200)
	timedOutPrintAnomalyBatchLimit       = int32(200)
	timedOutPrintAnomalyThreshold        = 15 * time.Minute
	timedOutPrintAnomalyAlertInterval    = time.Hour
)

var riderDepositReminderOffsets = []int{30, 7, 1, 0}

// DataCleanupScheduler 数据清理调度器
// 处理各种业务数据的过期/超时清理
type DataCleanupScheduler struct {
	cron            *cron.Cron
	store           db.Store
	taskDistributor worker.TaskDistributor
	publisher       websocket.PubSubPublisher
	mu              sync.Mutex
	alertedPrintLog map[string]time.Time
}

// NewDataCleanupScheduler 创建数据清理调度器
func NewDataCleanupScheduler(store db.Store, taskDistributor worker.TaskDistributor, publisher websocket.PubSubPublisher) *DataCleanupScheduler {
	return &DataCleanupScheduler{
		cron: cron.New(
			cron.WithSeconds(),
			cron.WithChain(
				cron.SkipIfStillRunning(cron.DefaultLogger),
				cron.Recover(cron.DefaultLogger),
			),
		),
		store:           store,
		taskDistributor: taskDistributor,
		publisher:       publisher,
		alertedPrintLog: make(map[string]time.Time),
	}
}

// Start 启动调度器
func (s *DataCleanupScheduler) Start() error {
	// 每分钟执行 Web 登录会话过期（5分钟有效）
	_, err := s.cron.AddFunc("0 * * * * *", s.cleanupExpiredWebLoginSessions)
	if err != nil {
		return err
	}

	// 每5分钟执行支付订单过期清理
	_, err = s.cron.AddFunc("0 */5 * * * *", s.cleanupExpiredPaymentOrders)
	if err != nil {
		return err
	}

	// 每10分钟执行配送单超时检查
	_, err = s.cron.AddFunc("0 */10 * * * *", s.cleanupStaleDeliveries)
	if err != nil {
		return err
	}

	// 每小时执行用餐会话超时清理
	_, err = s.cron.AddFunc("0 0 * * * *", s.cleanupStaleDiningSessions)
	if err != nil {
		return err
	}

	// 每天凌晨2点40执行异常率告警检查
	_, err = s.cron.AddFunc("0 40 2 * * *", s.checkAbnormalStatsAlerts)
	if err != nil {
		return err
	}

	// 每天凌晨3点执行优惠券过期标记
	_, err = s.cron.AddFunc("0 0 3 * * *", s.markExpiredVouchers)
	if err != nil {
		return err
	}

	// 每天凌晨2点30执行异常统计回填（修正漂移）
	_, err = s.cron.AddFunc("0 30 2 * * *", s.backfillAbnormalStatsDaily)
	if err != nil {
		return err
	}

	// 每天凌晨4点清理长期未更新的购物车（7天）
	_, err = s.cron.AddFunc("0 0 4 * * *", s.cleanupExpiredCarts)
	if err != nil {
		return err
	}

	// 每小时15分清理超时OCR任务
	_, err = s.cron.AddFunc("0 15 * * * *", s.cleanupStaleOCRTasks)
	if err != nil {
		return err
	}

	// 每10分钟巡检一次超时未恢复的打印异常并发布平台告警
	_, err = s.cron.AddFunc("0 */10 * * * *", s.checkTimedOutPrintAnomalies)
	if err != nil {
		return err
	}

	// 每天早上9点发送骑手押金退款窗口提醒
	_, err = s.cron.AddFunc("0 0 9 * * *", s.remindExpiringRiderDepositCredits)
	if err != nil {
		return err
	}

	// 每天凌晨3点10分标记已过期的骑手押金退款凭证
	_, err = s.cron.AddFunc("0 10 3 * * *", s.markExpiredRiderDepositCredits)
	if err != nil {
		return err
	}

	// 每30分钟检查是否存在卡死在 processing 状态的退款单（微信回调可能永久丢失）
	_, err = s.cron.AddFunc("0 */30 * * * *", s.alertStuckProcessingRefundOrders)
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("data cleanup scheduler started")
	return nil
}

// Stop 停止调度器
func (s *DataCleanupScheduler) Stop() {
	s.cron.Stop()
	log.Info().Msg("data cleanup scheduler stopped")
}

// cleanupExpiredWebLoginSessions 清理过期的 Web 登录会话
// 超过5分钟未确认的会话标记为过期
func (s *DataCleanupScheduler) cleanupExpiredWebLoginSessions() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := s.store.ExpireWebLoginSessionsBefore(ctx, db.ExpireWebLoginSessionsBeforeParams{
		Status:    "pending",
		ExpiresAt: time.Now(),
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to expire web login sessions")
		return
	}

	if count > 0 {
		log.Info().Int64("count", count).Msg("expired web login sessions")
	}

}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func sameCalendarDay(a, b time.Time) bool {
	a = a.In(b.Location())
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func fenToYuanText(amount int64) string {
	return fmt.Sprintf("%.2f", float64(amount)/100)
}

func riderDepositReminderText(credit db.RiderDepositCredit, daysRemaining int) (string, string) {
	amountText := fenToYuanText(credit.RefundableAmount)
	deadlineText := credit.RefundableUntil.In(time.Local).Format("2006-01-02 15:04")

	if daysRemaining == 0 {
		return "骑手押金退款今日到期", fmt.Sprintf("你有 %s 元骑手押金可退款额度将在今天到期，请在 %s 前完成提现申请。", amountText, deadlineText)
	}

	return fmt.Sprintf("骑手押金退款还有 %d 天到期", daysRemaining), fmt.Sprintf("你有 %s 元骑手押金可退款额度将在 %d 天后到期，到期时间 %s，请及时处理。", amountText, daysRemaining, deadlineText)
}

func (s *DataCleanupScheduler) shouldUseNotificationTask() bool {
	if s.taskDistributor == nil {
		return false
	}

	_, isNoop := s.taskDistributor.(worker.NoopTaskDistributor)
	return !isNoop
}

func (s *DataCleanupScheduler) sendRiderDepositReminderNotification(
	ctx context.Context,
	userID int64,
	credit db.RiderDepositCredit,
	daysRemaining int,
	title string,
	content string,
) error {
	extraData := map[string]any{
		"credit_id":           credit.ID,
		"payment_order_id":    credit.PaymentOrderID,
		"rider_id":            credit.RiderID,
		"days_remaining":      daysRemaining,
		"refundable_amount":   credit.RefundableAmount,
		"refundable_until":    credit.RefundableUntil,
		"notification_source": "rider_deposit_credit_expiry",
	}

	if s.shouldUseNotificationTask() {
		err := s.taskDistributor.DistributeTaskSendNotification(ctx, &worker.SendNotificationPayload{
			UserID:            userID,
			Type:              "system",
			Title:             title,
			Content:           content,
			ExtraData:         extraData,
			IgnorePreferences: true,
		})
		if err == nil {
			return nil
		}

		log.Error().Err(err).Int64("credit_id", credit.ID).Int64("user_id", userID).Msg("failed to enqueue rider deposit expiry notification task, fallback to direct notification")
	}

	extraDataJSON, err := json.Marshal(extraData)
	if err != nil {
		return fmt.Errorf("marshal rider deposit reminder extra data: %w", err)
	}

	_, err = s.store.CreateNotification(ctx, db.CreateNotificationParams{
		UserID:    userID,
		Type:      "system",
		Title:     title,
		Content:   content,
		ExtraData: extraDataJSON,
	})
	if err != nil {
		return fmt.Errorf("create rider deposit reminder notification: %w", err)
	}

	return nil
}

func (s *DataCleanupScheduler) publishPlatformAlert(ctx context.Context, alert worker.AlertData) bool {
	if s.publisher == nil {
		return false
	}

	alert.Timestamp = time.Now()
	alertMsg := map[string]any{
		"type":      "alert",
		"data":      alert,
		"timestamp": alert.Timestamp,
	}

	payload, err := json.Marshal(alertMsg)
	if err != nil {
		log.Error().Err(err).Str("alert_type", string(alert.AlertType)).Msg("failed to marshal rider deposit alert payload")
		return false
	}

	if err := s.publisher.Publish(ctx, worker.AlertChannel, payload); err != nil {
		log.Error().Err(err).Str("alert_type", string(alert.AlertType)).Msg("failed to publish rider deposit alert")
		return false
	}

	return true
}

func (s *DataCleanupScheduler) filterTimedOutPrintAnomaliesForAlert(items []db.ListTimedOutPrintAnomaliesRow, now time.Time) []db.ListTimedOutPrintAnomaliesRow {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := make(map[string]struct{}, len(items))
	filtered := make([]db.ListTimedOutPrintAnomaliesRow, 0, len(items))
	for _, item := range items {
		key := timedOutPrintAnomalyKey(item)
		current[key] = struct{}{}
		lastAlertAt, exists := s.alertedPrintLog[key]
		if exists && now.Sub(lastAlertAt) < timedOutPrintAnomalyAlertInterval {
			continue
		}
		filtered = append(filtered, item)
	}

	for anomalyKey := range s.alertedPrintLog {
		if _, ok := current[anomalyKey]; !ok {
			delete(s.alertedPrintLog, anomalyKey)
		}
	}

	return filtered
}

func (s *DataCleanupScheduler) markTimedOutPrintAnomaliesAlerted(items []db.ListTimedOutPrintAnomaliesRow, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range items {
		s.alertedPrintLog[timedOutPrintAnomalyKey(item)] = now
	}
}

func timedOutPrintAnomalyKey(item db.ListTimedOutPrintAnomaliesRow) string {
	return fmt.Sprintf("%d:%d", item.OrderID, item.PrinterID)
}

func summarizeTimedOutPrintAnomalyMessage(items []db.ListTimedOutPrintAnomaliesRow, now time.Time) (string, map[string]interface{}) {
	failedCount := 0
	pendingCount := 0
	samples := make([]map[string]interface{}, 0, minInt(len(items), 5))
	oldestMinutes := 0
	for index, item := range items {
		ageMinutes := int(now.Sub(item.AnomalyCreatedAt).Minutes())
		if index == 0 || ageMinutes > oldestMinutes {
			oldestMinutes = ageMinutes
		}
		switch item.Status {
		case "failed":
			failedCount++
		case "pending":
			pendingCount++
		}
		if len(samples) < 5 {
			sample := map[string]interface{}{
				"print_log_id":  item.ID,
				"merchant_id":   item.MerchantID,
				"merchant_name": item.MerchantName,
				"order_id":      item.OrderID,
				"order_no":      item.OrderNo,
				"printer_id":    item.PrinterID,
				"printer_name":  item.PrinterName,
				"status":        item.Status,
				"age_minutes":   ageMinutes,
			}
			if item.ErrorMessage.Valid {
				sample["error_message"] = item.ErrorMessage.String
			}
			samples = append(samples, sample)
		}
	}

	message := fmt.Sprintf("当前有 %d 条打印异常已超过 %d 分钟仍未恢复，其中 failed %d 条、pending %d 条；最早已滞留约 %d 分钟。", len(items), int(timedOutPrintAnomalyThreshold.Minutes()), failedCount, pendingCount, oldestMinutes)
	extra := map[string]interface{}{
		"total":                len(items),
		"failed_count":         failedCount,
		"pending_count":        pendingCount,
		"threshold_minutes":    int(timedOutPrintAnomalyThreshold.Minutes()),
		"oldest_age_minutes":   oldestMinutes,
		"sample_print_anomaly": samples,
	}
	return message, extra
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *DataCleanupScheduler) checkTimedOutPrintAnomalies() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	now := time.Now()
	items, err := s.store.ListTimedOutPrintAnomalies(ctx, db.ListTimedOutPrintAnomaliesParams{
		CreatedAt: now.Add(-timedOutPrintAnomalyThreshold),
		Limit:     timedOutPrintAnomalyBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to list timed out print anomalies")
		return
	}
	if len(items) == 0 {
		return
	}

	toAlert := s.filterTimedOutPrintAnomaliesForAlert(items, now)
	if len(toAlert) == 0 {
		return
	}

	message, extra := summarizeTimedOutPrintAnomalyMessage(toAlert, now)
	if !s.publishPlatformAlert(ctx, worker.AlertData{
		AlertType:   worker.AlertTypePrintAnomalyTimeout,
		Level:       worker.AlertLevelWarning,
		Title:       "商户打印异常超时未恢复",
		Message:     message,
		RelatedType: "print",
		Extra:       extra,
	}) {
		return
	}

	s.markTimedOutPrintAnomaliesAlerted(toAlert, now)
	log.Warn().Int("count", len(toAlert)).Msg("published timed out print anomaly alert")
}

func (s *DataCleanupScheduler) remindExpiringRiderDepositCredits() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	now := time.Now()
	todayStart := startOfDay(now)
	reminded := 0
	todayDueCount := 0
	todayDueAmount := int64(0)

	for _, daysRemaining := range riderDepositReminderOffsets {
		windowStart := todayStart.AddDate(0, 0, daysRemaining)
		windowEnd := windowStart.Add(24 * time.Hour)

		credits, err := s.store.ListRiderDepositCreditsForReminderWindow(ctx, db.ListRiderDepositCreditsForReminderWindowParams{
			RefundableUntil:   windowStart,
			RefundableUntil_2: windowEnd,
			Limit:             riderDepositCreditReminderBatchLimit,
		})
		if err != nil {
			log.Error().Err(err).Int("days_remaining", daysRemaining).Msg("failed to list rider deposit credits for reminder window")
			continue
		}

		for _, credit := range credits {
			if credit.LastRemindedAt.Valid && sameCalendarDay(credit.LastRemindedAt.Time, now) {
				continue
			}

			rider, err := s.store.GetRider(ctx, credit.RiderID)
			if err != nil {
				log.Error().Err(err).Int64("credit_id", credit.ID).Int64("rider_id", credit.RiderID).Msg("failed to get rider for deposit credit reminder")
				continue
			}

			title, content := riderDepositReminderText(credit, daysRemaining)
			err = s.sendRiderDepositReminderNotification(ctx, rider.UserID, credit, daysRemaining, title, content)
			if err != nil {
				log.Error().Err(err).Int64("credit_id", credit.ID).Int64("user_id", rider.UserID).Msg("failed to create rider deposit expiry notification")
				continue
			}

			_, err = s.store.TouchRiderDepositCreditReminder(ctx, db.TouchRiderDepositCreditReminderParams{
				ID:             credit.ID,
				LastRemindedAt: pgtype.Timestamptz{Time: now, Valid: true},
			})
			if err != nil {
				log.Error().Err(err).Int64("credit_id", credit.ID).Msg("failed to touch rider deposit reminder timestamp")
				continue
			}

			reminded++
			if daysRemaining == 0 {
				todayDueCount++
				todayDueAmount += credit.RefundableAmount
			}
		}
	}

	if reminded > 0 {
		log.Info().Int("count", reminded).Msg("sent rider deposit expiry reminders")
	}

	if todayDueCount > 0 {
		s.publishPlatformAlert(ctx, worker.AlertData{
			AlertType:   worker.AlertTypeRiderDepositExpiry,
			Level:       worker.AlertLevelWarning,
			Title:       "骑手押金退款今日到期提醒已发送",
			Message:     fmt.Sprintf("今日共有 %d 笔骑手押金退款凭证到期，涉及 %.2f 元，请关注提现与客服咨询。", todayDueCount, float64(todayDueAmount)/100),
			RelatedType: "payment",
			Extra: map[string]interface{}{
				"window":       "today",
				"credit_count": todayDueCount,
				"total_amount": todayDueAmount,
			},
		})
	}
}

func (s *DataCleanupScheduler) markExpiredRiderDepositCredits() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	now := time.Now()
	expiredCount := 0
	expiredAmount := int64(0)

	for {
		credits, err := s.store.ListExpiredRiderDepositCredits(ctx, db.ListExpiredRiderDepositCreditsParams{
			RefundableUntil: now,
			Limit:           riderDepositCreditExpireBatchLimit,
		})
		if err != nil {
			log.Error().Err(err).Msg("failed to list expired rider deposit credits")
			return
		}

		if len(credits) == 0 {
			break
		}

		for _, credit := range credits {
			_, err := s.store.MarkRiderDepositCreditExpired(ctx, db.MarkRiderDepositCreditExpiredParams{
				ID:        credit.ID,
				ExpiredAt: pgtype.Timestamptz{Time: now, Valid: true},
			})
			if err != nil {
				log.Error().Err(err).Int64("credit_id", credit.ID).Msg("failed to mark rider deposit credit expired")
				continue
			}
			expiredCount++
			expiredAmount += credit.RefundableAmount
		}

		if len(credits) < int(riderDepositCreditExpireBatchLimit) {
			break
		}
	}

	if expiredCount > 0 {
		log.Info().Int("count", expiredCount).Msg("marked rider deposit credits as expired")
		s.publishPlatformAlert(ctx, worker.AlertData{
			AlertType:   worker.AlertTypeRiderDepositExpiry,
			Level:       worker.AlertLevelWarning,
			Title:       "骑手押金退款凭证已过期",
			Message:     fmt.Sprintf("本次扫描已将 %d 笔骑手押金退款凭证标记为 expired，涉及 %.2f 元。", expiredCount, float64(expiredAmount)/100),
			RelatedType: "payment",
			Extra: map[string]interface{}{
				"window":       "expired",
				"credit_count": expiredCount,
				"total_amount": expiredAmount,
			},
		})
	}
}

type abnormalAlertThresholds struct {
	UserRate30d     float64 `json:"user_rate_30d"`
	MerchantRate30d float64 `json:"merchant_rate_30d"`
	RiderRate30d    float64 `json:"rider_rate_30d"`
	MinClaims30d    int32   `json:"min_claims_30d"`
	Limit           int32   `json:"limit"`
}

func (s *DataCleanupScheduler) getAbnormalAlertThresholds(ctx context.Context) abnormalAlertThresholds {
	defaults := abnormalAlertThresholds{
		UserRate30d:     0.35,
		MerchantRate30d: 0.12,
		RiderRate30d:    0.1,
		MinClaims30d:    5,
		Limit:           100,
	}
	config, err := s.store.GetPlatformConfig(ctx, db.GetPlatformConfigParams{
		ConfigKey: "behavior_trace.alert_thresholds",
		ScopeType: "global",
		ScopeID:   pgtype.Int8{Valid: false},
	})
	if err != nil || len(config.ConfigValue) == 0 {
		return defaults
	}
	var payload abnormalAlertThresholds
	if err := json.Unmarshal(config.ConfigValue, &payload); err != nil {
		return defaults
	}
	if payload.UserRate30d > 0 {
		defaults.UserRate30d = payload.UserRate30d
	}
	if payload.MerchantRate30d > 0 {
		defaults.MerchantRate30d = payload.MerchantRate30d
	}
	if payload.RiderRate30d > 0 {
		defaults.RiderRate30d = payload.RiderRate30d
	}
	if payload.MinClaims30d > 0 {
		defaults.MinClaims30d = payload.MinClaims30d
	}
	if payload.Limit > 0 {
		defaults.Limit = payload.Limit
	}
	return defaults
}
func (s *DataCleanupScheduler) checkAbnormalStatsAlerts() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	thresholds := s.getAbnormalAlertThresholds(ctx)

	now := time.Now()
	start := now.AddDate(0, 0, -30)
	startDate := pgtype.Date{Time: start, Valid: true}
	endDate := pgtype.Date{Time: now, Valid: true}

	s.checkEntityAbnormalAlerts(ctx, "user", startDate, endDate, thresholds.UserRate30d, thresholds.MinClaims30d, thresholds.Limit)
	s.checkEntityAbnormalAlerts(ctx, "merchant", startDate, endDate, thresholds.MerchantRate30d, thresholds.MinClaims30d, thresholds.Limit)
	s.checkEntityAbnormalAlerts(ctx, "rider", startDate, endDate, thresholds.RiderRate30d, thresholds.MinClaims30d, thresholds.Limit)
}

func (s *DataCleanupScheduler) checkEntityAbnormalAlerts(ctx context.Context, entityType string, startDate, endDate pgtype.Date, minRate float64, minClaims int32, limit int32) {
	rows, err := s.store.ListAbnormalStatsAlerts(ctx, db.ListAbnormalStatsAlertsParams{
		EntityType: entityType,
		StartDate:  startDate,
		EndDate:    endDate,
		MinClaims:  minClaims,
		MinRate:    minRate,
		Limit:      limit,
	})
	if err != nil {
		log.Error().Err(err).Str("entity_type", entityType).Msg("failed to list abnormal stats alerts")
		return
	}
	for _, row := range rows {
		metadata, _ := json.Marshal(map[string]interface{}{
			"entity_type":     entityType,
			"entity_id":       row.EntityID,
			"total_orders":    row.TotalOrders,
			"abnormal_claims": row.AbnormalClaims,
			"abnormal_rate":   row.AbnormalRate,
			"window_days":     30,
			"min_rate":        minRate,
			"min_claims":      minClaims,
		})

		var regionID pgtype.Int8
		switch entityType {
		case "merchant":
			if merchant, err := s.store.GetMerchant(ctx, row.EntityID); err == nil {
				regionID = pgtype.Int8{Int64: merchant.RegionID, Valid: true}
			}
		case "rider":
			if rider, err := s.store.GetRider(ctx, row.EntityID); err == nil && rider.RegionID.Valid {
				regionID = rider.RegionID
			}
		}

		_, err := s.store.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorRole:  "system",
			Action:     "abnormal_stats_alert",
			TargetType: entityType,
			TargetID:   pgtype.Int8{Int64: row.EntityID, Valid: true},
			RegionID:   regionID,
			Metadata:   metadata,
		})
		if err != nil {
			log.Error().Err(err).Str("entity_type", entityType).Int64("entity_id", row.EntityID).Msg("failed to create abnormal stats alert")
		}
	}
}

// cleanupExpiredPaymentOrders 清理过期的支付订单
// 超过过期时间的 pending 支付订单关闭
func (s *DataCleanupScheduler) cleanupExpiredPaymentOrders() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := s.store.CloseExpiredPaymentOrders(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to close expired payment orders")
		return
	}

	if count > 0 {
		log.Info().Int64("count", count).Msg("closed expired payment orders")
	}
}

// backfillAbnormalStatsDaily 回填异常统计日表（默认回填最近3天）
func (s *DataCleanupScheduler) backfillAbnormalStatsDaily() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	start := startOfToday.AddDate(0, 0, -3)
	end := startOfToday.AddDate(0, 0, 1)

	startDateParam := pgtype.Timestamptz{Time: start, Valid: true}
	endDateParam := pgtype.Timestamptz{Time: end, Valid: true}

	err := s.store.BackfillAbnormalStatsDaily(ctx, db.BackfillAbnormalStatsDailyParams{
		CompletedAt:   startDateParam,
		CompletedAt_2: endDateParam,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to backfill abnormal stats daily")
		return
	}
	log.Info().Str("start", start.Format("2006-01-02")).Str("end", end.Format("2006-01-02")).Msg("backfilled abnormal stats daily")
}

// cleanupStaleDeliveries 清理过期的配送单
// 超过2小时未被接单的配送单，触发重新分配或人工处理警报
// cleanupStaleDeliveries 清理过期的配送单
// 1. 超过 20 分钟未接单：通知商户和运营商（触发告警）
// 2. 超过 60 分钟未接单：自动取消订单并已付款退款
func (s *DataCleanupScheduler) cleanupStaleDeliveries() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// 1. 处理严重超时（1小时）：自动取消
	cancelTime := time.Now().Add(-1 * time.Hour)
	staleDeliveries, err := s.store.ListPendingDeliveriesBefore(ctx, db.ListPendingDeliveriesBeforeParams{
		Status:    "pending",
		CreatedAt: cancelTime,
		Limit:     50,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to list stale deliveries for cancellation")
	} else if len(staleDeliveries) > 0 {
		log.Info().Int("count", len(staleDeliveries)).Msg("found stale deliveries to cancel")

		cancelledCount := 0
		for _, delivery := range staleDeliveries {
			// 如果没有 OrderID，无法处理关联，跳过（理论上不应存在）
			if delivery.OrderID == 0 {
				continue
			}

			// P1-025 fix: 首先获取订单状态，以确保 CancelOrderTx 能正确处理库存回滚
			order, err := s.store.GetOrder(ctx, delivery.OrderID)
			if err != nil {
				log.Error().Err(err).Int64("order_id", delivery.OrderID).Msg("failed to get order for stale delivery cancellation")
				continue
			}

			// 使用事务取消订单
			cancelResult, err := s.store.CancelOrderTx(ctx, db.CancelOrderTxParams{
				OrderID:      delivery.OrderID,
				OldStatus:    order.Status,
				CancelReason: "配送无人接单，系统自动取消",
				OperatorID:   0, // 系统
				OperatorType: "system",
			})

			// 如果 CancelOrderTx 成功，还需要更新 delivery 状态为 cancelled
			if err == nil {
				_, err = s.store.UpdateDeliveryToCancelled(ctx, delivery.ID)
			}

			if err != nil {
				log.Error().Err(err).Int64("delivery_id", delivery.ID).Int64("order_id", delivery.OrderID).Msg("failed to cancel stale delivery")
				continue
			}

			// 3. P1-025 fix: 显式触发退款任务（针对微信支付等外部支付）
			// CancelOrderTx 内部已处理余额支付的回滚，这里只处理外部支付
			// 需要查找该订单关联的成功支付记录
			paymentOrders, err := s.store.GetPaymentOrdersByOrder(ctx, pgtype.Int8{Int64: cancelResult.Order.ID, Valid: true})
			if err != nil {
				log.Error().Err(err).Int64("order_id", cancelResult.Order.ID).Msg("failed to get payment orders for refund check")
			} else {
				var successPayment *db.PaymentOrder
				for _, p := range paymentOrders {
					if p.Status == "success" {
						successPayment = &p
						break
					}
				}

				if successPayment != nil {
					payload := &worker.PayloadProcessRefund{
						PaymentOrderID: successPayment.ID,
						OrderID:        cancelResult.Order.ID,
						RefundAmount:   successPayment.Amount, // 全额退款
						Reason:         "配送无人接单，系统自动取消",
					}
					opts := []asynq.Option{
						asynq.MaxRetry(10),
						asynq.ProcessIn(10 * time.Second), // 稍后处理，给DB复制留点时间
						asynq.Queue(worker.QueueCritical),
					}
					if err := s.taskDistributor.DistributeTaskProcessRefund(ctx, payload, opts...); err != nil {
						log.Error().Err(err).Int64("order_id", cancelResult.Order.ID).Msg("failed to enqueue refund task for stale delivery")
						// 即使入队失败，订单已取消，后续可以通过 ProcessOrderPaymentTimeout 或人工补偿
					} else {
						log.Info().Int64("order_id", cancelResult.Order.ID).Msg("enqueued refund task for stale delivery cancellation")
					}
				}
			}

			cancelledCount++
		}
		log.Info().Int("cancelled_count", cancelledCount).Msg("cancelled stale deliveries")
	}

	// 2. 处理轻微超时（20分钟）：发送告警通知
	// 注意：为了避免重复告警，这里需要一种机制标记已告警（例如 is_delayed 字段）
	// 查看 db/query/delivery.sql 发现有 UpdateDeliveryDelayed

	alertTime := time.Now().Add(-20 * time.Minute)
	// 我们需要一个新的查询来查找 "pending 且未 delayed 且超过 20 分钟" 的配送单
	// 由于 SQLStore 生成的代码限制，我们先复用 ListPendingDeliveriesBefore，在内存中过滤 is_delayed (假设 struct 有这个字段)
	// 查阅 Model 定义： Delivery 应该有 IsDelayed bool

	pendingDeliveries, err := s.store.ListPendingDeliveriesBefore(ctx, db.ListPendingDeliveriesBeforeParams{
		Status:    "pending",
		CreatedAt: alertTime,
		Limit:     50, // 每次处理 50 条
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to list pending deliveries for alert")
		return
	}

	alertCount := 0
	for _, delivery := range pendingDeliveries {
		// 如果已经标记为延迟，或者已经达到取消时间的（上面会处理），跳过
		if delivery.IsDelayed || delivery.CreatedAt.Before(cancelTime) {
			continue
		}

		// 标记为 delayed，防止重复检查
		if _, err := s.store.UpdateDeliveryDelayed(ctx, delivery.ID); err != nil {
			log.Error().Err(err).Int64("delivery_id", delivery.ID).Msg("failed to mark delivery as delayed")
			continue
		}

		// 发送通知
		// 1. 获取订单关联的商户信息
		order, err := s.store.GetOrder(ctx, delivery.OrderID)
		if err != nil {
			log.Error().Err(err).Int64("order_id", delivery.OrderID).Msg("failed to get order for stale delivery alert")
			continue
		}

		// 2. 发送通知给商户
		// 注意：SendNotificationPayload 的 UserID 应该是商户绑定的 UserID。
		// 由于 Merchant 可能绑定多个员工，这里最好通知 Owner 或 MerchantBoss。
		// 简化起见，我们先查找 Merchant 的 Owner (通过 GetMerchantByOwner 反查或 GetMerchant 拿 OwnerUserID 如果有的话)
		// 目前 Merchant 表通常不直接存 OwnerUserID，而是有 merchant_bosses 表。
		// 或者我们发给 MerchantID，由 Notification Worker 处理分发逻辑。
		// 但 TaskSendNotification 目前设计是发给具体的 UserID。

		// 查找商户 Boss 的 UserID
		boss, err := s.store.GetMerchantBoss(ctx, db.GetMerchantBossParams{
			MerchantID: order.MerchantID,
		})

		merchantUserID := int64(0)
		if err == nil {
			merchantUserID = boss.UserID
		} else {
			// 如果没找到 Boss（比如是员工管理的），尝试另一种方式或跳过
			// 也可以尝试查询 Order 的创建者（如果是商户代客下单）
			// 这里做一个 Fallback：只打日志
			log.Warn().Int64("merchant_id", order.MerchantID).Msg("merchant boss not found, skipping user notification")
		}

		if merchantUserID > 0 && s.taskDistributor != nil {
			err = s.taskDistributor.DistributeTaskSendNotification(ctx, &worker.SendNotificationPayload{
				UserID:      merchantUserID,
				Type:        "delivery",
				Title:       "配送超时告警",
				Content:     "您的订单已有20分钟未接单，请及时处理或联系客服。",
				RelatedType: "delivery",
				RelatedID:   delivery.ID,
				ExtraData: map[string]any{
					"order_id": order.ID,
					"order_no": order.OrderNo,
				},
				// 标记为高优先级，忽略 DND 设置
				IgnorePreferences: true,
			})
			if err != nil {
				log.Error().Err(err).Int64("merchant_user_id", merchantUserID).Msg("failed to enqueue notification task")
			} else {
				log.Info().Int64("delivery_id", delivery.ID).Msg("sent stale delivery alert to merchant")
			}
		}

		alertCount++
	}

	if alertCount > 0 {
		log.Info().Int("alert_count", alertCount).Msg("processed delivery delay alerts")
	}
}

// cleanupStaleDiningSessions 清理过期的用餐会话
// 超过12小时未关闭的会话自动关闭
func (s *DataCleanupScheduler) cleanupStaleDiningSessions() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	staleTime := time.Now().Add(-12 * time.Hour)
	sessions, err := s.store.ListOpenDiningSessionsBefore(ctx, db.ListOpenDiningSessionsBeforeParams{
		Status:   "open",
		OpenedAt: staleTime,
		Limit:    50,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to list stale dining sessions")
		return
	}

	if len(sessions) == 0 {
		return
	}

	log.Info().Int("count", len(sessions)).Msg("found stale open dining sessions to close")

	closedCount := 0
	for _, session := range sessions {
		_, err := s.store.CloseDiningSessionTx(ctx, db.CloseDiningSessionTxParams{
			ID:         session.ID,
			MerchantID: session.MerchantID,
		})
		if err != nil {
			log.Warn().Err(err).Int64("session_id", session.ID).Msg("failed to close stale dining session")
			continue
		}
		closedCount++
	}

	log.Info().Int("closed", closedCount).Msg("closed stale dining sessions")
}

// markExpiredVouchers 标记过期的优惠券
func (s *DataCleanupScheduler) markExpiredVouchers() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := s.store.ExpireUnusedVouchers(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to expire unused vouchers")
		return
	}

	if count > 0 {
		log.Info().Int64("count", count).Msg("marked vouchers as expired")
	}
}

// cleanupExpiredCarts 清理长期未更新的购物车
// 超过7天未更新的购物车数据将被物理删除
func (s *DataCleanupScheduler) cleanupExpiredCarts() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// 7天前
	expireTime := time.Now().AddDate(0, 0, -7)

	err := s.store.CleanupOldCarts(ctx, expireTime)
	if err != nil {
		log.Error().Err(err).Msg("failed to cleanup expired carts")
		return
	}

	log.Info().Msg("cleanup expired carts (older than 7 days) completed")
}

// cleanupStaleOCRTasks 清理长期处于 processing 状态的 OCR 任务
// 超过1小时未更新的 OCR 标记为 failed，允许用户重试
func (s *DataCleanupScheduler) cleanupStaleOCRTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// 1小时前
	staleTime := time.Now().Add(-1 * time.Hour)

	err := s.store.ResetStaleMerchantOCRStatus(ctx, staleTime)
	if err != nil {
		log.Error().Err(err).Msg("failed to reset stale merchant ocr status")
		return
	}
}

const (
	stuckProcessingRefundThreshold  = 2 * time.Hour
	stuckProcessingRefundBatchLimit = int32(50)
)

// alertStuckProcessingRefundOrders 扫描持续处于 processing 状态超过阈值的退款单，
// 向运营平台发布告警。这类退款单的微信回调可能已永久丢失，需人工在微信商户平台核查。
func (s *DataCleanupScheduler) alertStuckProcessingRefundOrders() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	createdBefore := time.Now().Add(-stuckProcessingRefundThreshold)
	orders, err := s.store.ListStuckProcessingRefundOrders(ctx, db.ListStuckProcessingRefundOrdersParams{
		CreatedBefore: createdBefore,
		Limit:         stuckProcessingRefundBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list stuck processing refund orders failed")
		return
	}
	if len(orders) == 0 {
		return
	}

	log.Warn().
		Int("count", len(orders)).
		Time("threshold", createdBefore).
		Msg("found stuck processing refund orders — wechat callback may be lost")

	for _, ro := range orders {
		s.publishPlatformAlert(ctx, worker.AlertData{
			AlertType:   worker.AlertTypeRefundFailed,
			Level:       worker.AlertLevelCritical,
			Title:       "退款单长期卡在 processing — 可能缺失微信回调",
			Message:     fmt.Sprintf("退款单 %s（ID=%d）于 %s 提交退款请求后超过 %v 仍未收到微信回调，请在微信商户平台查询退款结果并手动更新状态。退款类型：%s。", ro.OutRefundNo, ro.ID, ro.CreatedAt.In(time.Local).Format("2006-01-02 15:04"), stuckProcessingRefundThreshold, ro.PaymentType),
			RelatedID:   ro.ID,
			RelatedType: "refund_order",
			Extra: map[string]interface{}{
				"out_refund_no": ro.OutRefundNo,
				"refund_id":     ro.RefundID,
				"refund_amount": ro.RefundAmount,
				"payment_type":  ro.PaymentType,
				"created_at":    ro.CreatedAt,
			},
		})
	}
}
