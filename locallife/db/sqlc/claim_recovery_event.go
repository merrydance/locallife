package db

import (
	"context"
	"encoding/json"
	"fmt"
)

type claimRecoveryEventWriter interface {
	CreateClaimRecoveryEvent(ctx context.Context, arg CreateClaimRecoveryEventParams) (ClaimRecoveryEvent, error)
}

type claimRecoveryEventInspector interface {
	claimRecoveryEventWriter
	ListClaimRecoveryEventsByRecovery(ctx context.Context, recoveryID int64) ([]ClaimRecoveryEvent, error)
}

func WriteClaimRecoveryEvent(ctx context.Context, store claimRecoveryEventWriter, recovery ClaimRecovery, eventType string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal claim recovery event payload: %w", err)
	}

	if _, err := store.CreateClaimRecoveryEvent(ctx, CreateClaimRecoveryEventParams{
		RecoveryID: recovery.ID,
		DecisionID: recovery.DecisionID,
		EventType:  eventType,
		Payload:    raw,
	}); err != nil {
		return fmt.Errorf("create claim recovery event %s: %w", eventType, err)
	}

	return nil
}

func WriteClaimRecoveryClosedEventIfAbsent(ctx context.Context, store claimRecoveryEventInspector, recovery ClaimRecovery, payload any) error {
	events, err := store.ListClaimRecoveryEventsByRecovery(ctx, recovery.ID)
	if err != nil {
		return fmt.Errorf("list claim recovery events: %w", err)
	}
	for _, event := range events {
		if event.EventType == ClaimRecoveryEventTypeClosed {
			return nil
		}
	}

	return WriteClaimRecoveryEvent(ctx, store, recovery, ClaimRecoveryEventTypeClosed, payload)
}
