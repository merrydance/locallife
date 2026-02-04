package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/worker"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	// TakeoutAutoCompleteAfter 外卖送达后自动完成时间
	TakeoutAutoCompleteAfter = time.Hour
)

// TakeoutAutoCompleteScheduler 外卖订单自动完成调度器
// 规则：骑手送达后，用户未手动完成且无索赔的情况下，1小时后自动完成
type TakeoutAutoCompleteScheduler struct {
	cron            *cron.Cron
	store           db.Store
	taskDistributor worker.TaskDistributor
}

func NewTakeoutAutoCompleteScheduler(store db.Store, taskDistributor worker.TaskDistributor) *TakeoutAutoCompleteScheduler {
	return &TakeoutAutoCompleteScheduler{
		cron:            cron.New(cron.WithSeconds()),
		store:           store,
		taskDistributor: taskDistributor,
	}
}

func (s *TakeoutAutoCompleteScheduler) Start() error {
	// 每5分钟扫描一次
	_, err := s.cron.AddFunc("0 */5 * * * *", s.autoCompleteTakeoutOrders)
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("takeout auto-complete scheduler started")
	return nil
}

func (s *TakeoutAutoCompleteScheduler) Stop() {
	s.cron.Stop()
	log.Info().Msg("takeout auto-complete scheduler stopped")
}

func (s *TakeoutAutoCompleteScheduler) autoCompleteTakeoutOrders() {
	ctx := context.Background()

	deliveredBefore := time.Now().Add(-TakeoutAutoCompleteAfter)
	orders, err := s.store.ListTakeoutOrdersDeliveredBefore(ctx, db.ListTakeoutOrdersDeliveredBeforeParams{
		DeliveredBefore: pgtype.Timestamptz{Time: deliveredBefore, Valid: true},
		Limit:           100,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to list takeout orders for auto completion")
		return
	}
	if len(orders) == 0 {
		return
	}

	completedCount := 0
	for _, order := range orders {
		hasClaim, err := s.hasClaimForOrder(ctx, order)
		if err != nil {
			log.Warn().Err(err).Int64("order_id", order.ID).Msg("failed to check claims, skip auto complete")
			continue
		}
		if hasClaim {
			continue
		}

		updated, err := s.store.AutoCompleteTakeoutOrder(ctx, order.ID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				continue
			}
			log.Warn().Err(err).Int64("order_id", order.ID).Msg("auto complete takeout order failed")
			continue
		}

		_, _ = s.store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      updated.ID,
			FromStatus:   pgtype.Text{String: order.Status, Valid: true},
			ToStatus:     "completed",
			OperatorID:   pgtype.Int8{Valid: false},
			OperatorType: pgtype.Text{String: "system", Valid: true},
			Notes:        pgtype.Text{String: "送达后1小时无操作自动完成", Valid: true},
		})

		// 完成触发分账（若是 profit_sharing）
		if s.taskDistributor != nil {
			po, err := s.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
				OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
				BusinessType: "order",
			})
			if err == nil && po.Status == "paid" && po.PaymentType == "profit_sharing" {
				_ = s.taskDistributor.DistributeTaskProcessProfitSharing(ctx, &worker.ProfitSharingPayload{
					PaymentOrderID: po.ID,
					OrderID:        order.ID,
				})
			}
		}

		completedCount++
	}

	log.Info().Int("completed", completedCount).Int("total", len(orders)).Msg("takeout auto-complete scan finished")
}

func (s *TakeoutAutoCompleteScheduler) hasClaimForOrder(ctx context.Context, order db.Order) (bool, error) {
	claims, err := s.store.ListUserClaimsInPeriod(ctx, db.ListUserClaimsInPeriodParams{
		UserID:    order.UserID,
		CreatedAt: order.CreatedAt,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	for _, c := range claims {
		if c.OrderID == order.ID {
			return true, nil
		}
	}

	return false, nil
}
