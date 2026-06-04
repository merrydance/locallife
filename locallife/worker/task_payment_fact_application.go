package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
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

type BaofuVerifyFeeAsyncContinuation struct {
	service     *logic.BaofuAccountOnboardingService
	distributor BaofuAccountOpeningTaskDistributor
}

func NewBaofuVerifyFeeAsyncContinuation(service *logic.BaofuAccountOnboardingService, distributor TaskDistributor) BaofuVerifyFeeAsyncContinuation {
	return BaofuVerifyFeeAsyncContinuation{
		service:     service,
		distributor: baofuAccountOpeningDistributor(distributor),
	}
}

func (c BaofuVerifyFeeAsyncContinuation) ContinueAfterVerifyFeePaid(ctx context.Context, paymentOrder db.PaymentOrder) error {
	if c.service == nil {
		return logic.ErrBaofuAccountOnboardingNotConfigured
	}
	result, err := c.service.PrepareOpeningAfterVerifyFeePaid(ctx, paymentOrder)
	if err != nil {
		return err
	}
	if !result.ShouldEnqueueOpening {
		return nil
	}
	if c.distributor == nil {
		return financialTaskDistributorUnavailable("baofu account opening")
	}
	return c.distributor.DistributeTaskProcessBaofuAccountOpening(ctx, &BaofuAccountOpeningPayload{FlowID: result.Flow.ID}, asynq.Queue(QueueCritical), asynq.MaxRetry(5), asynq.Unique(baofuAccountOpeningTaskUnique))
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

	baofuContinuation := logic.NewBaofuAccountOnboardingService(processor.store, processor.baofuAccountClient, processor.directPaymentClient, processor.dataEncryptor, logic.BaofuAccountOnboardingConfig{
		VerifyFeeFen:      processor.config.BaofuAccountVerifyFeeFen,
		IndustryID:        processor.config.BaofuBusinessIndustryID,
		CollectMerchantID: processor.config.BaofuCollectMerchantID,
	})
	if processor.baofuMerchantReportClient != nil {
		baofuContinuation = baofuContinuation.WithMerchantReportContinuation(processor.baofuMerchantReportClient, logic.BaofuAccountMerchantReportConfig{
			CollectMerchantID: processor.config.BaofuCollectMerchantID,
			CollectTerminalID: processor.config.BaofuCollectTerminalID,
			MiniProgramAppID:  processor.config.WechatMiniAppID,
			ChannelID:         processor.config.BaofuMerchantReportChannelID,
			ChannelName:       processor.config.BaofuMerchantReportChannelName,
			Business:          processor.config.BaofuMerchantReportBusiness,
		})
	}

	service := logic.NewPaymentFactService(processor.store).
		WithPaymentSuccessConfig(processor.config.RiderAverageSpeed, processor.config.DefaultPrepareTime).
		WithBaofuVerifyFeeContinuation(NewBaofuVerifyFeeAsyncContinuation(baofuContinuation, processor.distributor))
	result, err := service.ApplyExternalPaymentFactApplication(ctx, payload.ApplicationID)
	if err != nil {
		return err
	}
	if result.Skipped {
		log.Info().Int64("payment_fact_application_id", payload.ApplicationID).Msg("payment fact application skipped")
		return nil
	}
	if result.ClaimRecoveryReleaseAction != nil {
		if processor.distributor == nil {
			log.Error().
				Int64("payment_fact_application_id", result.Application.ID).
				Int64("behavior_action_id", result.ClaimRecoveryReleaseAction.ID).
				Msg("claim recovery release action created but task distributor is not configured")
		} else if err := processor.distributor.DistributeTaskClaimBehaviorAction(
			ctx,
			&ClaimBehaviorActionPayload{ActionID: result.ClaimRecoveryReleaseAction.ID},
			asynq.Queue(QueueCritical),
			asynq.MaxRetry(10),
		); err != nil {
			log.Error().
				Err(err).
				Int64("payment_fact_application_id", result.Application.ID).
				Int64("behavior_action_id", result.ClaimRecoveryReleaseAction.ID).
				Msg("enqueue claim recovery release action failed; recovery scheduler will retry")
		}
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
