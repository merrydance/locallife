package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/merrydance/locallife/logic"
	"github.com/rs/zerolog/log"
)

const (
	TaskProcessBaofuAccountOpening = "baofu:process_account_opening"
	baofuAccountOpeningTaskUnique  = 30 * time.Minute
)

type BaofuAccountOpeningTaskDistributor interface {
	DistributeTaskProcessBaofuAccountOpening(ctx context.Context, payload *BaofuAccountOpeningPayload, opts ...asynq.Option) error
}

type BaofuAccountOpeningPayload struct {
	FlowID int64 `json:"flow_id"`
}

func baofuAccountOpeningDistributor(distributor TaskDistributor) BaofuAccountOpeningTaskDistributor {
	if distributor == nil {
		return nil
	}
	accountOpeningDistributor, _ := distributor.(BaofuAccountOpeningTaskDistributor)
	return accountOpeningDistributor
}

func (distributor *RedisTaskDistributor) DistributeTaskProcessBaofuAccountOpening(ctx context.Context, payload *BaofuAccountOpeningPayload, opts ...asynq.Option) error {
	if payload == nil || payload.FlowID <= 0 {
		return fmt.Errorf("baofu account opening flow id is required")
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal baofu account opening payload: %w", err)
	}
	if len(opts) == 0 {
		opts = append(opts, asynq.Queue(QueueCritical), asynq.MaxRetry(5), asynq.Unique(baofuAccountOpeningTaskUnique))
	}
	task := asynq.NewTask(TaskProcessBaofuAccountOpening, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			log.Info().
				Str("type", task.Type()).
				Int64("flow_id", payload.FlowID).
				Msg("duplicate baofu account opening task enqueue suppressed")
			return nil
		}
		return fmt.Errorf("enqueue baofu account opening task: %w", err)
	}
	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("flow_id", payload.FlowID).
		Msg("enqueued baofu account opening task")
	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskBaofuAccountOpening(ctx context.Context, task *asynq.Task) error {
	var payload BaofuAccountOpeningPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal baofu account opening payload: %w", asynq.SkipRetry)
	}
	if payload.FlowID <= 0 {
		return fmt.Errorf("baofu account opening flow id is required: %w", asynq.SkipRetry)
	}
	service := logic.NewBaofuAccountOnboardingService(processor.store, processor.baofuAccountClient, processor.directPaymentClient, processor.dataEncryptor, logic.BaofuAccountOnboardingConfig{
		VerifyFeeFen:      processor.config.BaofuAccountVerifyFeeFen,
		IndustryID:        processor.config.BaofuBusinessIndustryID,
		CollectMerchantID: processor.config.BaofuCollectMerchantID,
	})
	if processor.baofuMerchantReportClient != nil {
		service = service.WithMerchantReportContinuation(processor.baofuMerchantReportClient, logic.BaofuAccountMerchantReportConfig{
			CollectMerchantID: processor.config.BaofuCollectMerchantID,
			CollectTerminalID: processor.config.BaofuCollectTerminalID,
			MiniProgramAppID:  processor.config.WechatMiniAppID,
			ChannelID:         processor.config.BaofuMerchantReportChannelID,
			ChannelName:       processor.config.BaofuMerchantReportChannelName,
			Business:          processor.config.BaofuMerchantReportBusiness,
		})
	}
	_, err := service.ExecutePreparedOpening(ctx, payload.FlowID)
	return err
}
