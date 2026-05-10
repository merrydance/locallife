package logic

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

func (svc *PaymentFactService) markExternalPaymentFactApplicationFailed(ctx context.Context, application db.ExternalPaymentFactApplication, applyErr error, facts ...db.ExternalPaymentFact) error {
	nextRetryAt := svc.now().UTC().Add(paymentFactApplicationRetryDelay)
	logger := log.Error().
		Err(applyErr).
		Int64("payment_fact_application_id", application.ID).
		Int64("payment_fact_id", application.FactID).
		Str("consumer", application.Consumer).
		Str("business_object_type", application.BusinessObjectType).
		Int64("business_object_id", application.BusinessObjectID).
		Time("next_retry_at", nextRetryAt)
	if len(facts) > 0 {
		fact := facts[0]
		logger = logger.
			Str("provider", fact.Provider).
			Str("channel", fact.Channel).
			Str("capability", fact.Capability).
			Str("external_object_key", fact.ExternalObjectKey).
			Str("terminal_status", fact.TerminalStatus)
	}
	_, markErr := svc.store.MarkExternalPaymentFactApplicationFailed(ctx, db.MarkExternalPaymentFactApplicationFailedParams{
		ID:          application.ID,
		LastError:   pgtype.Text{String: applyErr.Error(), Valid: true},
		NextRetryAt: pgtype.Timestamptz{Time: nextRetryAt, Valid: true},
	})
	if markErr != nil {
		logger.Err(markErr).Msg("mark external payment fact application failed after apply error")
		return fmt.Errorf("%w; mark external payment fact application failed: %v", applyErr, markErr)
	}
	logger.Msg("external payment fact application failed and scheduled for retry")
	return applyErr
}
