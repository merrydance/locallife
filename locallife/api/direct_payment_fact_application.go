package api

import (
	"context"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

func (server *Server) enqueueDirectPaymentFactApplication(ctx context.Context, application *db.ExternalPaymentFactApplication) {
	if application == nil || server.taskDistributor == nil {
		return
	}
	distributor, ok := server.taskDistributor.(worker.PaymentFactApplicationTaskDistributor)
	if !ok {
		return
	}
	if err := distributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&worker.PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(worker.QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	); err != nil {
		log.Warn().Err(err).
			Int64("payment_fact_application_id", application.ID).
			Int64("payment_fact_id", application.FactID).
			Int64("payment_order_id", application.BusinessObjectID).
			Str("consumer", application.Consumer).
			Msg("enqueue direct payment fact application from callback failed; scheduler will retry")
	}
}

func isSupportedDirectPaymentFactBusinessType(businessType string) bool {
	switch businessType {
	case db.ExternalPaymentBusinessOwnerRiderDeposit,
		db.ExternalPaymentBusinessOwnerClaimRecovery,
		db.ExternalPaymentBusinessOwnerBaofuVerifyFee:
		return true
	default:
		return false
	}
}
