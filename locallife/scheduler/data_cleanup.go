package scheduler

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/worker"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
)

// DataCleanupScheduler 数据清理调度器
// 处理各种业务数据的过期/超时清理
type DataCleanupScheduler struct {
	cron            *cron.Cron
	store           db.Store
	taskDistributor worker.TaskDistributor
}

// NewDataCleanupScheduler 创建数据清理调度器
func NewDataCleanupScheduler(store db.Store, taskDistributor worker.TaskDistributor) *DataCleanupScheduler {
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
