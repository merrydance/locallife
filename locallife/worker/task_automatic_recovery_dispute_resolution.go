package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/rs/zerolog/log"
)

const (
	TaskAutomaticRecoveryDisputeResolution = "recovery_dispute:automatic_resolution"
)

type AutomaticRecoveryDisputeResolutionPayload struct {
	RecoveryDisputeID int64 `json:"recovery_dispute_id"`
}

func (distributor *RedisTaskDistributor) DistributeTaskAutomaticRecoveryDisputeResolution(
	ctx context.Context,
	payload *AutomaticRecoveryDisputeResolutionPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	queueOpts := []asynq.Option{asynq.MaxRetry(5), asynq.Queue(QueueDefault)}
	queueOpts = append(queueOpts, opts...)
	task := asynq.NewTask(TaskAutomaticRecoveryDisputeResolution, jsonPayload, queueOpts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Debug().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("recovery_dispute_id", payload.RecoveryDisputeID).
		Msg("enqueued automatic recovery dispute resolution task")

	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskAutomaticRecoveryDisputeResolution(ctx context.Context, task *asynq.Task) error {
	var payload AutomaticRecoveryDisputeResolutionPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	recoveryDispute, err := processor.store.GetRecoveryDispute(ctx, payload.RecoveryDisputeID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			log.Warn().Int64("recovery_dispute_id", payload.RecoveryDisputeID).Msg("recovery dispute not found during automatic resolution retry")
			return nil
		}
		return err
	}

	log.Info().Int64("recovery_dispute_id", recoveryDispute.ID).Str("status", recoveryDispute.Status).Msg("processing automatic recovery dispute resolution task")

	var postProcess db.GetRecoveryDisputeForPostProcessRow
	var resolution logic.AutomaticRecoveryDisputeResolution

	if recoveryDispute.Status == "submitted" {
		result, err := logic.ResolveRecoveryDisputeAutomatically(ctx, processor.store, recoveryDispute)
		if err != nil {
			return err
		}
		recoveryDispute = result.ReviewResult.RecoveryDispute
		postProcess = result.ReviewResult.PostProcess
		resolution = result.Resolution

		processor.writeAutomaticRecoveryDisputeAuditLog(ctx, recoveryDispute, resolution, "system_recovery_dispute_resolved_retry")

		return processor.processRecoveryDisputeResult(ctx, ProcessRecoveryDisputeResultPayload{
			RecoveryDisputeID: recoveryDispute.ID,
			ClaimID:           postProcess.ClaimID,
			RecoveryTarget:    postProcess.AppellantType,
			CompensationActionID: func() int64 {
				if result.ReviewResult.CompensationAction != nil {
					return result.ReviewResult.CompensationAction.ID
				}
				return 0
			}(),
			ReleaseActionID: func() int64 {
				if result.ReviewResult.ReleaseAction != nil {
					return result.ReviewResult.ReleaseAction.ID
				}
				return 0
			}(),
			Status:             recoveryDispute.Status,
			AppellantType:      postProcess.AppellantType,
			AppellantID:        postProcess.AppellantID,
			ClaimantUserID:     postProcess.ClaimantUserID,
			ClaimType:          postProcess.ClaimType,
			ClaimAmount:        postProcess.ClaimAmount,
			CompensationAmount: 0,
			OrderNo:            postProcess.OrderNo,
		})
	} else {
		postProcess, err = processor.store.GetRecoveryDisputeForPostProcess(ctx, recoveryDispute.ID)
		if err != nil {
			return err
		}
		resolution, err = logic.EvaluateAutomaticRecoveryDisputeResolution(ctx, processor.store, recoveryDispute)
		if err != nil {
			log.Warn().Err(err).Int64("recovery_dispute_id", recoveryDispute.ID).Msg("failed to recalculate automatic recovery dispute resolution during retry reconciliation")
		}
	}

	resultPayload, err := processor.buildProcessRecoveryDisputeResultPayload(ctx, recoveryDispute, postProcess, resolution)
	if err != nil {
		return err
	}
	return processor.processRecoveryDisputeResult(ctx, resultPayload)
}

func (processor *RedisTaskProcessor) buildProcessRecoveryDisputeResultPayload(ctx context.Context, recoveryDispute db.RecoveryDispute, postProcess db.GetRecoveryDisputeForPostProcessRow, resolution logic.AutomaticRecoveryDisputeResolution) (ProcessRecoveryDisputeResultPayload, error) {
	compensationAmount := int64(0)
	if recoveryDispute.CompensationAmount.Valid {
		compensationAmount = recoveryDispute.CompensationAmount.Int64
	}

	payload := ProcessRecoveryDisputeResultPayload{
		RecoveryDisputeID:    recoveryDispute.ID,
		ClaimID:              postProcess.ClaimID,
		RecoveryTarget:       postProcess.AppellantType,
		CompensationActionID: 0,
		ReleaseActionID:      0,
		Status:               recoveryDispute.Status,
		AppellantType:        postProcess.AppellantType,
		AppellantID:          postProcess.AppellantID,
		ClaimantUserID:       postProcess.ClaimantUserID,
		ClaimType:            postProcess.ClaimType,
		ClaimAmount:          postProcess.ClaimAmount,
		CompensationAmount:   compensationAmount,
		OrderNo:              postProcess.OrderNo,
	}
	recovery, err := processor.claimRecoveryForRecoveryDisputeResult(ctx, recoveryDispute.ClaimID, postProcess.AppellantType)
	if err != nil {
		return ProcessRecoveryDisputeResultPayload{}, err
	}
	decisionIDs := recoveryDisputeResultDecisionIDs(recovery, resolution)
	if len(decisionIDs) == 0 {
		return payload, nil
	}

	actions, err := processor.listRecoveryDisputeResultActions(ctx, decisionIDs)
	if err != nil {
		return ProcessRecoveryDisputeResultPayload{}, err
	}
	for _, action := range actions {
		switch {
		case action.ActionType == "release" && (action.TargetEntity == "merchant" || action.TargetEntity == "rider"):
			var detail claimRecoveryReleaseActionDetail
			if err := json.Unmarshal(action.Detail, &detail); err == nil && detail.ClaimID == postProcess.ClaimID && detail.RecoveryID == recoveryDisputeResultRecoveryID(recovery) {
				payload.ReleaseActionID = action.ID
			}
		case action.ActionType == "payout" && action.TargetEntity == "user":
			var detail claimPayoutActionDetail
			if err := json.Unmarshal(action.Detail, &detail); err == nil && detail.RecoveryDisputeID == recoveryDispute.ID {
				payload.CompensationActionID = action.ID
			}
		}
	}

	return payload, nil
}

func (processor *RedisTaskProcessor) claimRecoveryForRecoveryDisputeResult(ctx context.Context, claimID int64, recoveryTarget string) (*db.ClaimRecovery, error) {
	recovery, err := processor.store.GetClaimRecoveryByClaimIDAndTarget(ctx, db.GetClaimRecoveryByClaimIDAndTargetParams{
		ClaimID:        claimID,
		RecoveryTarget: pgtype.Text{String: recoveryTarget, Valid: recoveryTarget != ""},
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get claim recovery for automatic recovery dispute result: %w", err)
	}
	return &recovery, nil
}

func recoveryDisputeResultDecisionIDs(recovery *db.ClaimRecovery, resolution logic.AutomaticRecoveryDisputeResolution) []int64 {
	ids := make([]int64, 0, 2)
	seen := map[int64]struct{}{}
	var recoveryDecisionID pgtype.Int8
	if recovery != nil {
		recoveryDecisionID = recovery.DecisionID
	}
	for _, decisionID := range []pgtype.Int8{recoveryDecisionID, resolution.DecisionID} {
		if !decisionID.Valid {
			continue
		}
		if _, exists := seen[decisionID.Int64]; exists {
			continue
		}
		seen[decisionID.Int64] = struct{}{}
		ids = append(ids, decisionID.Int64)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func recoveryDisputeResultRecoveryID(recovery *db.ClaimRecovery) int64 {
	if recovery == nil {
		return 0
	}
	return recovery.ID
}

func (processor *RedisTaskProcessor) listRecoveryDisputeResultActions(ctx context.Context, decisionIDs []int64) ([]db.BehaviorAction, error) {
	actionsByID := make(map[int64]db.BehaviorAction, len(decisionIDs))
	for _, decisionID := range decisionIDs {
		actions, err := processor.store.ListBehaviorActionsByDecision(ctx, decisionID)
		if err != nil {
			return nil, fmt.Errorf("list behavior actions for automatic recovery dispute resolution decision %d: %w", decisionID, err)
		}
		for _, action := range actions {
			actionsByID[action.ID] = action
		}
	}

	actions := make([]db.BehaviorAction, 0, len(actionsByID))
	for _, action := range actionsByID {
		actions = append(actions, action)
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i].ID < actions[j].ID })
	return actions, nil
}

func (processor *RedisTaskProcessor) writeAutomaticRecoveryDisputeAuditLog(ctx context.Context, appeal db.RecoveryDispute, resolution logic.AutomaticRecoveryDisputeResolution, action string) {
	metadata, err := json.Marshal(map[string]any{
		"status":            resolution.Status,
		"review_notes":      resolution.ReviewNotes,
		"resolution_source": "system_retry",
		"decision_id": func() any {
			if resolution.DecisionID.Valid {
				return resolution.DecisionID.Int64
			}
			return nil
		}(),
	})
	if err != nil {
		log.Warn().Err(err).Int64("recovery_dispute_id", appeal.ID).Msg("failed to marshal automatic recovery dispute retry audit metadata")
		return
	}

	_, err = processor.store.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorUserID: pgtype.Int8{},
		ActorRole:   "system",
		Action:      action,
		TargetType:  "recovery_dispute",
		TargetID:    pgtype.Int8{Int64: appeal.ID, Valid: true},
		RegionID:    pgtype.Int8{Int64: appeal.RegionID, Valid: true},
		RequestID:   pgtype.Text{},
		TraceID:     pgtype.Text{},
		ClientIp:    pgtype.Text{},
		UserAgent:   pgtype.Text{},
		Metadata:    metadata,
	})
	if err != nil {
		log.Warn().Err(err).Int64("recovery_dispute_id", appeal.ID).Msg("failed to write automatic recovery dispute retry audit log")
	}
}
