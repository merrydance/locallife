package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type claimRecoveryBehaviorActionStore interface {
	CreateBehaviorAction(ctx context.Context, arg CreateBehaviorActionParams) (BehaviorAction, error)
	ListBehaviorDecisionsByOrder(ctx context.Context, orderID pgtype.Int8) ([]BehaviorDecision, error)
}

func CreateClaimRecoveryReleaseAction(ctx context.Context, store claimRecoveryBehaviorActionStore, recovery ClaimRecovery, remark string) (*BehaviorAction, error) {
	return CreateClaimRecoveryReleaseActionWithDecision(ctx, store, recovery, pgtype.Int8{}, remark)
}

func CreateClaimRecoveryReleaseActionWithDecision(ctx context.Context, store claimRecoveryBehaviorActionStore, recovery ClaimRecovery, decisionID pgtype.Int8, remark string) (*BehaviorAction, error) {
	if !recovery.RecoveryTarget.Valid {
		return nil, nil
	}

	switch recovery.RecoveryTarget.String {
	case "merchant", "rider":
	default:
		return nil, nil
	}

	resolvedDecisionID := decisionID.Int64
	if !decisionID.Valid {
		resolvedDecisionID = recovery.DecisionID.Int64
	}
	if !decisionID.Valid && !recovery.DecisionID.Valid {
		decisions, err := store.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: recovery.OrderID, Valid: true})
		if err != nil {
			return nil, fmt.Errorf("list behavior decisions for claim recovery release: %w", err)
		}
		if len(decisions) == 0 {
			return nil, fmt.Errorf("no behavior decision found for order %d", recovery.OrderID)
		}
		resolvedDecisionID = decisions[0].ID
	}

	detail, err := json.Marshal(map[string]any{
		"action":        "release_recovery_suspension",
		"claim_id":      recovery.ClaimID,
		"recovery_id":   recovery.ID,
		"order_id":      recovery.OrderID,
		"target_entity": recovery.RecoveryTarget.String,
		"remark":        remark,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal claim recovery release action detail: %w", err)
	}

	action, err := store.CreateBehaviorAction(ctx, CreateBehaviorActionParams{
		DecisionID:   resolvedDecisionID,
		ActionType:   "release",
		TargetEntity: recovery.RecoveryTarget.String,
		Status:       "created",
		Detail:       detail,
	})
	if err != nil {
		return nil, fmt.Errorf("create claim recovery release action: %w", err)
	}

	return &action, nil
}
