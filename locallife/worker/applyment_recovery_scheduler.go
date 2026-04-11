package worker

import (
	"context"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	applymentRecoveryCron       = "*/5 * * * *"
	applymentRecoveryBatchLimit = int32(100)
	applymentRecoveryMinAge     = 2 * time.Minute
)

type ApplymentRecoveryScheduler struct {
	cron            *cron.Cron
	wg              sync.WaitGroup
	stopCtx         context.Context
	stopCancel      context.CancelFunc
	runMu           sync.Mutex
	store           db.Store
	distributor     TaskDistributor
	ecommerceClient wechat.EcommerceClientInterface
}

func NewApplymentRecoveryScheduler(store db.Store, distributor TaskDistributor, ecommerceClient wechat.EcommerceClientInterface) *ApplymentRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &ApplymentRecoveryScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:         stopCtx,
		stopCancel:      stopCancel,
		store:           store,
		distributor:     distributor,
		ecommerceClient: ecommerceClient,
	}
}

func (s *ApplymentRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(applymentRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("applyment recovery scheduler started (every 5 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *ApplymentRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("applyment recovery scheduler stopped")
}

func (s *ApplymentRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *ApplymentRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("applyment recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip applyment recovery")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	applyments, err := s.store.ListEcommerceApplymentsPendingFollowUp(ctx, db.ListEcommerceApplymentsPendingFollowUpParams{
		UpdatedBefore: time.Now().Add(-applymentRecoveryMinAge),
		Limit:         applymentRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list ecommerce applyments for recovery failed")
		return
	}

	for _, applyment := range applyments {
		status := normalizeApplymentFollowUpStatus(applyment.Status, textValue(applyment.SubMchID))
		processedState := normalizeApplymentFollowUpStatus(textValue(applyment.ResultTaskProcessedState), textValue(applyment.SubMchID))

		if applymentStatusNeedsAsyncFollowUp(status) && processedState != status {
			s.enqueueApplymentFollowUp(ctx, applyment, "", status, textValue(applyment.SubMchID))
			continue
		}

		if !applymentStatusNeedsRemoteQuery(status) {
			continue
		}
		if s.ecommerceClient == nil {
			log.Warn().Int64("applyment_id", applyment.ID).Msg("ecommerce client not configured, cannot query applyment recovery status")
			continue
		}

		queryResp, err := s.queryApplymentStatus(ctx, applyment)
		if err != nil {
			log.Error().Err(err).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", applyment.OutRequestNo).
				Msg("query applyment recovery status failed")
			continue
		}

		resolvedStatus, resolvedSubMchID, err := s.reconcileApplymentStatus(ctx, applyment, queryResp)
		if err != nil {
			log.Error().Err(err).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", applyment.OutRequestNo).
				Msg("reconcile applyment recovery status failed")
			continue
		}

		if applymentStatusNeedsAsyncFollowUp(resolvedStatus) && processedState != resolvedStatus {
			s.enqueueApplymentFollowUp(ctx, applyment, queryResp.ApplymentState, resolvedStatus, resolvedSubMchID)
		}
	}
}

func (s *ApplymentRecoveryScheduler) queryApplymentStatus(ctx context.Context, applyment db.EcommerceApplymentPendingFollowUp) (*wechat.EcommerceApplymentQueryResponse, error) {
	if applyment.ApplymentID.Valid {
		return s.ecommerceClient.QueryEcommerceApplymentByID(ctx, applyment.ApplymentID.Int64)
	}
	return s.ecommerceClient.QueryEcommerceApplymentByOutRequestNo(ctx, applyment.OutRequestNo)
}

func (s *ApplymentRecoveryScheduler) reconcileApplymentStatus(ctx context.Context, applyment db.EcommerceApplymentPendingFollowUp, queryResp *wechat.EcommerceApplymentQueryResponse) (string, string, error) {
	resolvedStatus := normalizeApplymentFollowUpStatus(mapWechatApplymentStateToStatus(queryResp.ApplymentState), queryResp.SubMchID)
	resolvedSubMchID := queryResp.SubMchID

	if resolvedSubMchID != "" {
		if err := s.store.ApplymentSubMchActivationTx(ctx, db.ApplymentSubMchActivationTxParams{
			ApplymentID: applyment.ID,
			SubjectType: applyment.SubjectType,
			SubjectID:   applyment.SubjectID,
			SubMchID:    resolvedSubMchID,
		}); err != nil {
			return "", "", err
		}

		if applyment.SubjectType == "operator" {
			if _, err := s.store.UpdateOperatorSubMchID(ctx, db.UpdateOperatorSubMchIDParams{
				ID:       applyment.SubjectID,
				SubMchID: pgtype.Text{String: resolvedSubMchID, Valid: true},
			}); err != nil {
				return "", "", err
			}
			if _, err := s.store.UpdateOperatorStatus(ctx, db.UpdateOperatorStatusParams{
				ID:     applyment.SubjectID,
				Status: "active",
			}); err != nil {
				return "", "", err
			}
		}

		return normalizeApplymentFollowUpStatus("finish", resolvedSubMchID), resolvedSubMchID, nil
	}

	if _, err := s.store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
		ID:           applyment.ID,
		Status:       resolvedStatus,
		RejectReason: getRejectReasonFromApplymentAuditDetail(queryResp.AuditDetail),
		SignUrl:      pgtype.Text{String: queryResp.SignURL, Valid: queryResp.SignURL != ""},
		SignState:    pgtype.Text{String: queryResp.SignState, Valid: queryResp.SignState != ""},
		SubMchID:     pgtype.Text{},
	}); err != nil {
		return "", "", err
	}

	if applyment.SubjectType == "operator" && (resolvedStatus == "rejected" || resolvedStatus == "canceled") {
		if _, err := s.store.UpdateOperatorStatus(ctx, db.UpdateOperatorStatusParams{
			ID:     applyment.SubjectID,
			Status: "active",
		}); err != nil {
			return "", "", err
		}
	}

	return resolvedStatus, resolvedSubMchID, nil
}

func (s *ApplymentRecoveryScheduler) enqueueApplymentFollowUp(ctx context.Context, applyment db.EcommerceApplymentPendingFollowUp, applymentState, applymentStatus, subMchID string) {
	payload := buildApplymentResultPayload(applyment, applymentState, applymentStatus, subMchID)
	if err := s.distributor.DistributeTaskProcessApplymentResult(
		ctx,
		payload,
		asynq.MaxRetry(5),
		asynq.Queue(QueueCritical),
	); err != nil {
		log.Error().Err(err).
			Int64("applyment_id", applyment.ID).
			Str("applyment_status", payload.ApplymentStatus).
			Msg("enqueue applyment recovery follow-up failed")
		return
	}

	log.Info().
		Int64("applyment_id", applyment.ID).
		Str("applyment_status", payload.ApplymentStatus).
		Msg("applyment recovery follow-up task enqueued")
}
