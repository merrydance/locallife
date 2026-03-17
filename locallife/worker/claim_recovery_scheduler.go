package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	claimRecoveryCron       = "*/5 * * * *"
	claimRecoveryBatchLimit = int32(200)
)

// ClaimRecoveryScheduler scans due claim recoveries and applies overdue actions.
type ClaimRecoveryScheduler struct {
	cron       *cron.Cron
	wg         sync.WaitGroup
	stopCtx    context.Context
	stopCancel context.CancelFunc
	runMu      sync.Mutex
	store      db.Store
}

// NewClaimRecoveryScheduler creates a new scheduler for claim recoveries.
func NewClaimRecoveryScheduler(store db.Store) *ClaimRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &ClaimRecoveryScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:    stopCtx,
		stopCancel: stopCancel,
		store:      store,
	}
}

// Start starts the recovery scheduler.
func (s *ClaimRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(claimRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("claim recovery scheduler started (every 5 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

// Stop stops the scheduler.
func (s *ClaimRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("claim recovery scheduler stopped")
}

func (s *ClaimRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("claim recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	recoveries, err := s.store.ListDueClaimRecoveries(ctx, db.ListDueClaimRecoveriesParams{
		DueAt: time.Now(),
		Limit: claimRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list due claim recoveries failed")
		return
	}

	for _, recovery := range recoveries {
		updated, err := s.store.MarkClaimRecoveryOverdue(ctx, recovery.ID)
		if err != nil {
			log.Error().Err(err).Int64("recovery_id", recovery.ID).Msg("mark claim recovery overdue failed")
			continue
		}

		reason := fmt.Sprintf("claim recovery overdue: claim_id=%d", updated.ClaimID)
		suspendUntil := pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true}

		if updated.RecoveryTarget.Valid && updated.RecoveryTarget.String == "merchant" {
			order, orderErr := s.store.GetOrder(ctx, updated.OrderID)
			if orderErr != nil {
				log.Error().Err(orderErr).Int64("order_id", updated.OrderID).Msg("get order for recovery failed")
				continue
			}
			if err := s.store.SuspendMerchantTakeout(ctx, db.SuspendMerchantTakeoutParams{
				MerchantID:           order.MerchantID,
				TakeoutSuspendReason: pgtype.Text{String: reason, Valid: true},
				TakeoutSuspendUntil:  suspendUntil,
			}); err != nil {
				log.Error().Err(err).Int64("merchant_id", order.MerchantID).Msg("suspend merchant for recovery failed")
			}
			if decisions, err := s.store.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: updated.OrderID, Valid: true}); err == nil && len(decisions) > 0 {
				detail, _ := json.Marshal(map[string]any{
					"action":        "suspend_takeout",
					"merchant_id":   order.MerchantID,
					"recovery_id":   updated.ID,
					"suspend_until": suspendUntil.Time,
				})
				_, _ = s.store.CreateBehaviorAction(ctx, db.CreateBehaviorActionParams{
					DecisionID:   decisions[0].ID,
					ActionType:   "block",
					TargetEntity: "merchant",
					Status:       "created",
					Detail:       detail,
				})
			}
		}

		if updated.RecoveryTarget.Valid && updated.RecoveryTarget.String == "rider" {
			delivery, deliveryErr := s.store.GetDeliveryByOrderID(ctx, updated.OrderID)
			if deliveryErr != nil || !delivery.RiderID.Valid {
				log.Error().Err(deliveryErr).Int64("order_id", updated.OrderID).Msg("get delivery for recovery failed")
				continue
			}
			if err := s.store.SuspendRider(ctx, db.SuspendRiderParams{
				RiderID:       delivery.RiderID.Int64,
				SuspendReason: pgtype.Text{String: reason, Valid: true},
				SuspendUntil:  suspendUntil,
			}); err != nil {
				log.Error().Err(err).Int64("rider_id", delivery.RiderID.Int64).Msg("suspend rider for recovery failed")
			}
			if decisions, err := s.store.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: updated.OrderID, Valid: true}); err == nil && len(decisions) > 0 {
				detail, _ := json.Marshal(map[string]any{
					"action":        "suspend_rider",
					"rider_id":      delivery.RiderID.Int64,
					"recovery_id":   updated.ID,
					"suspend_until": suspendUntil.Time,
				})
				_, _ = s.store.CreateBehaviorAction(ctx, db.CreateBehaviorActionParams{
					DecisionID:   decisions[0].ID,
					ActionType:   "block",
					TargetEntity: "rider",
					Status:       "created",
					Detail:       detail,
				})
			}
		}
	}
}
