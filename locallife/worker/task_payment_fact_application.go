package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/merrydance/locallife/logic"
	"github.com/rs/zerolog/log"
)

const TaskProcessPaymentFactApplication = "payment:process_fact_application"

type PaymentFactApplicationTaskDistributor interface {
	DistributeTaskProcessPaymentFactApplication(ctx context.Context, payload *PaymentFactApplicationPayload, opts ...asynq.Option) error
}

type PaymentFactApplicationPayload struct {
	ApplicationID int64 `json:"application_id"`
}

func (distributor *RedisTaskDistributor) DistributeTaskProcessPaymentFactApplication(
	ctx context.Context,
	payload *PaymentFactApplicationPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payment fact application payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessPaymentFactApplication, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue payment fact application task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("payment_fact_application_id", payload.ApplicationID).
		Msg("enqueued payment fact application task")
	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskPaymentFactApplication(ctx context.Context, task *asynq.Task) error {
	var payload PaymentFactApplicationPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payment fact application payload: %w", asynq.SkipRetry)
	}
	if payload.ApplicationID == 0 {
		return fmt.Errorf("payment fact application id is required: %w", asynq.SkipRetry)
	}

	service := logic.NewPaymentFactService(processor.store).
		WithEcommerceClient(processor.ecommerceClient).
		WithPaymentSuccessConfig(processor.config.RiderAverageSpeed, processor.config.DefaultPrepareTime)
	result, err := service.ApplyExternalPaymentFactApplication(ctx, payload.ApplicationID)
	if err != nil {
		return err
	}
	if result.Skipped {
		log.Info().Int64("payment_fact_application_id", payload.ApplicationID).Msg("payment fact application skipped")
		return nil
	}
	log.Info().
		Int64("payment_fact_application_id", result.Application.ID).
		Int64("payment_fact_id", result.Application.FactID).
		Str("consumer", result.Application.Consumer).
		Str("business_object_type", result.Application.BusinessObjectType).
		Int64("business_object_id", result.Application.BusinessObjectID).
		Msg("payment fact application applied")
	return nil
}
