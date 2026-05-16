package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
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
		WithEcommerceClient(processor.ecommerceClient).
		WithRefundCreator(paymentFactApplicationRefundCreator{
			ecommerceClient: processor.ecommerceClient,
			ordinaryClient:  processor.ordinarySPClient,
		}).
		WithPaymentSuccessConfig(processor.config.RiderAverageSpeed, processor.config.DefaultPrepareTime).
		WithBaofuVerifyFeeContinuation(baofuContinuation)
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

type paymentFactApplicationRefundCreator struct {
	ecommerceClient wechat.EcommerceClientInterface
	ordinaryClient  OrdinaryServiceProviderWorkerClient
}

func (c paymentFactApplicationRefundCreator) CreateEcommerceRefund(ctx context.Context, req *wechatcontracts.EcommerceRefundRequest) (*wechatcontracts.EcommerceRefundCreateResponse, error) {
	return createEcommerceRefundContract(ctx, c.ecommerceClient, req)
}

func (c paymentFactApplicationRefundCreator) CreateOrdinaryServiceProviderRefund(ctx context.Context, req ospcontracts.RefundCreateRequest) (*ospcontracts.RefundResponse, error) {
	if c.ordinaryClient == nil {
		return nil, fmt.Errorf("ordinary service provider refund client not configured")
	}
	return c.ordinaryClient.CreateRefund(ctx, req)
}

func (c paymentFactApplicationRefundCreator) OrdinaryServiceProviderRefundNotifyURL() string {
	if c.ordinaryClient == nil {
		return ""
	}
	return c.ordinaryClient.RefundNotifyURL()
}
