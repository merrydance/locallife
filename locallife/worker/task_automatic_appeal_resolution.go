package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/rs/zerolog/log"
)

const (
	TaskAutomaticAppealResolution = "appeal:automatic_resolution"
)

type AutomaticAppealResolutionPayload struct {
	AppealID int64 `json:"appeal_id"`
}

func (distributor *RedisTaskDistributor) DistributeTaskAutomaticAppealResolution(
	ctx context.Context,
	payload *AutomaticAppealResolutionPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	queueOpts := []asynq.Option{asynq.MaxRetry(5), asynq.Queue(QueueDefault)}
	queueOpts = append(queueOpts, opts...)
	task := asynq.NewTask(TaskAutomaticAppealResolution, jsonPayload, queueOpts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Debug().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("appeal_id", payload.AppealID).
		Msg("enqueued automatic appeal resolution task")

	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskAutomaticAppealResolution(ctx context.Context, task *asynq.Task) error {
	var payload AutomaticAppealResolutionPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	appeal, err := processor.store.GetAppeal(ctx, payload.AppealID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			log.Warn().Int64("appeal_id", payload.AppealID).Msg("appeal not found during automatic resolution retry")
			return nil
		}
		return err
	}

	log.Info().Int64("appeal_id", appeal.ID).Str("status", appeal.Status).Msg("processing automatic appeal resolution task")

	var postProcess db.GetAppealForPostProcessRow
	var resolution logic.AutomaticAppealResolution

	if appeal.Status == "pending" {
		result, err := logic.ResolveAppealAutomatically(ctx, processor.store, appeal)
		if err != nil {
			return err
		}
		appeal = result.ReviewResult.Appeal
		postProcess = result.ReviewResult.PostProcess
		resolution = result.Resolution

		if err := processor.ensureBehaviorAppealRecorded(ctx, appeal, resolution); err != nil {
			return err
		}
		processor.writeAutomaticAppealAuditLog(ctx, appeal, resolution, "system_appeal_resolved_retry")
	} else {
		postProcess, err = processor.store.GetAppealForPostProcess(ctx, appeal.ID)
		if err != nil {
			return err
		}
		resolution, err = logic.EvaluateAutomaticAppealResolution(ctx, processor.store, appeal)
		if err == nil {
			if ensureErr := processor.ensureBehaviorAppealRecorded(ctx, appeal, resolution); ensureErr != nil {
				return ensureErr
			}
		} else {
			log.Warn().Err(err).Int64("appeal_id", appeal.ID).Msg("failed to recalculate automatic appeal resolution during retry reconciliation")
		}
	}

	return processor.processAppealResult(ctx, buildProcessAppealResultPayload(appeal, postProcess))
}

func buildProcessAppealResultPayload(appeal db.Appeal, postProcess db.GetAppealForPostProcessRow) ProcessAppealResultPayload {
	compensationAmount := int64(0)
	if appeal.CompensationAmount.Valid {
		compensationAmount = appeal.CompensationAmount.Int64
	}

	return ProcessAppealResultPayload{
		AppealID:             appeal.ID,
		ClaimID:              postProcess.ClaimID,
		CompensationActionID: 0,
		Status:               appeal.Status,
		AppellantType:        postProcess.AppellantType,
		AppellantID:          postProcess.AppellantID,
		ClaimantUserID:       postProcess.ClaimantUserID,
		ClaimType:            postProcess.ClaimType,
		ClaimAmount:          postProcess.ClaimAmount,
		CompensationAmount:   compensationAmount,
		OrderNo:              postProcess.OrderNo,
	}
}

func (processor *RedisTaskProcessor) ensureBehaviorAppealRecorded(ctx context.Context, appeal db.Appeal, resolution logic.AutomaticAppealResolution) error {
	evidence := logic.BuildBehaviorAppealEvidence(appeal)
	items, err := processor.store.ListBehaviorAppealsByEntity(ctx, db.ListBehaviorAppealsByEntityParams{
		EntityType: appeal.AppellantType,
		EntityID:   appeal.AppellantID,
	})
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Evidence.Valid && item.Evidence.String == evidence {
			return nil
		}
	}

	_, err = processor.store.CreateBehaviorAppeal(ctx, db.CreateBehaviorAppealParams{
		EntityType: appeal.AppellantType,
		EntityID:   appeal.AppellantID,
		DecisionID: resolution.DecisionID,
		Reason:     appeal.Reason,
		Evidence:   pgtype.Text{String: evidence, Valid: true},
		Status:     "resolved",
		ReevalAt:   pgtype.Timestamptz{Time: appeal.CreatedAt, Valid: true},
	})
	return err
}

func (processor *RedisTaskProcessor) writeAutomaticAppealAuditLog(ctx context.Context, appeal db.Appeal, resolution logic.AutomaticAppealResolution, action string) {
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
		log.Warn().Err(err).Int64("appeal_id", appeal.ID).Msg("failed to marshal automatic appeal retry audit metadata")
		return
	}

	_, err = processor.store.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorUserID: pgtype.Int8{},
		ActorRole:   "system",
		Action:      action,
		TargetType:  "appeal",
		TargetID:    pgtype.Int8{Int64: appeal.ID, Valid: true},
		RegionID:    pgtype.Int8{Int64: appeal.RegionID, Valid: true},
		RequestID:   pgtype.Text{},
		TraceID:     pgtype.Text{},
		ClientIp:    pgtype.Text{},
		UserAgent:   pgtype.Text{},
		Metadata:    metadata,
	})
	if err != nil {
		log.Warn().Err(err).Int64("appeal_id", appeal.ID).Msg("failed to write automatic appeal retry audit log")
	}
}
