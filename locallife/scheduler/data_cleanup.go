package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
)

// DataCleanupScheduler 数据清理调度器
// 处理各种业务数据的过期/超时清理
type DataCleanupScheduler struct {
	cron  *cron.Cron
	store db.Store
}

// NewDataCleanupScheduler 创建数据清理调度器
func NewDataCleanupScheduler(store db.Store) *DataCleanupScheduler {
	return &DataCleanupScheduler{
		cron:  cron.New(cron.WithSeconds()),
		store: store,
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

	// 每天凌晨3点执行优惠券过期标记
	_, err = s.cron.AddFunc("0 0 3 * * *", s.markExpiredVouchers)
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
	ctx := context.Background()

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

// cleanupExpiredPaymentOrders 清理过期的支付订单
// 超过过期时间的 pending 支付订单关闭
func (s *DataCleanupScheduler) cleanupExpiredPaymentOrders() {
	ctx := context.Background()

	count, err := s.store.CloseExpiredPaymentOrders(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to close expired payment orders")
		return
	}

	if count > 0 {
		log.Info().Int64("count", count).Msg("closed expired payment orders")
	}
}

// cleanupStaleDeliveries 清理过期的配送单
// 超过2小时未被接单的配送单，触发重新分配或人工处理警报
func (s *DataCleanupScheduler) cleanupStaleDeliveries() {
	ctx := context.Background()

	// 查找超时未接单的配送单（2小时）
	staleTime := time.Now().Add(-2 * time.Hour)
	deliveries, err := s.store.ListPendingDeliveriesBefore(ctx, db.ListPendingDeliveriesBeforeParams{
		Status:    "pending",
		CreatedAt: staleTime,
		Limit:     50,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to list stale deliveries")
		return
	}

	if len(deliveries) == 0 {
		return
	}

	// 记录告警日志（实际生产中应该发送告警通知）
	log.Warn().
		Int("count", len(deliveries)).
		Time("before", staleTime).
		Msg("found stale pending deliveries - need manual intervention")

	// 这里可以扩展：
	// 1. 发送告警通知给运营人员
	// 2. 自动取消订单并退款
	// 3. 调用第三方配送平台兜底
}

// cleanupStaleDiningSessions 清理过期的用餐会话
// 超过12小时未关闭的会话自动关闭
func (s *DataCleanupScheduler) cleanupStaleDiningSessions() {
	ctx := context.Background()

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
	ctx := context.Background()

	count, err := s.store.ExpireUnusedVouchers(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to expire unused vouchers")
		return
	}

	if count > 0 {
		log.Info().Int64("count", count).Msg("marked vouchers as expired")
	}
}
