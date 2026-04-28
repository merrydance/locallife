package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type MarkClaimRecoveryOverdueWithActionTxParams struct {
	RecoveryID    int64
	SuspendUntil  time.Time
	OverdueRemark string
}

type MarkClaimRecoveryOverdueWithActionTxResult struct {
	Recovery ClaimRecovery
	Action   BehaviorAction
}

func (store *SQLStore) MarkClaimRecoveryOverdueWithActionTx(ctx context.Context, arg MarkClaimRecoveryOverdueWithActionTxParams) (MarkClaimRecoveryOverdueWithActionTxResult, error) {
	var result MarkClaimRecoveryOverdueWithActionTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		updatedRecovery, err := q.MarkClaimRecoveryOverdue(ctx, arg.RecoveryID)
		if err != nil {
			return err
		}

		if err := WriteClaimRecoveryEvent(ctx, q, updatedRecovery, ClaimRecoveryEventTypeOverdue, map[string]any{
			"claim_id":        updatedRecovery.ClaimID,
			"recovery_target": updatedRecovery.RecoveryTarget.String,
			"recovery_amount": updatedRecovery.RecoveryAmount,
			"status":          updatedRecovery.Status,
		}); err != nil {
			return fmt.Errorf("write claim recovery overdue event: %w", err)
		}

		decisionID, err := resolveClaimRecoveryActionDecisionID(ctx, q, updatedRecovery)
		if err != nil {
			return err
		}

		actionDetail, targetEntity, err := buildClaimRecoveryOverdueActionDetail(ctx, q, updatedRecovery, arg.SuspendUntil, arg.OverdueRemark)
		if err != nil {
			return err
		}

		action, err := q.CreateBehaviorAction(ctx, CreateBehaviorActionParams{
			DecisionID:   decisionID,
			ActionType:   "block",
			TargetEntity: targetEntity,
			Status:       "created",
			Detail:       actionDetail,
		})
		if err != nil {
			return fmt.Errorf("create claim recovery overdue block action: %w", err)
		}

		result.Recovery = updatedRecovery
		result.Action = action
		return nil
	})

	return result, err
}

func resolveClaimRecoveryActionDecisionID(ctx context.Context, q *Queries, recovery ClaimRecovery) (int64, error) {
	if recovery.DecisionID.Valid {
		return recovery.DecisionID.Int64, nil
	}

	decisions, err := q.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: recovery.OrderID, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("list behavior decisions for overdue claim recovery: %w", err)
	}
	if len(decisions) == 0 {
		return 0, fmt.Errorf("no behavior decision found for order %d", recovery.OrderID)
	}

	return decisions[0].ID, nil
}

func buildClaimRecoveryOverdueActionDetail(ctx context.Context, q *Queries, recovery ClaimRecovery, suspendUntil time.Time, remark string) ([]byte, string, error) {
	reason := fmt.Sprintf("claim recovery overdue: claim_id=%d", recovery.ClaimID)

	if recovery.RecoveryTarget.Valid && recovery.RecoveryTarget.String == "merchant" {
		order, err := q.GetOrder(ctx, recovery.OrderID)
		if err != nil {
			return nil, "", fmt.Errorf("get order for overdue claim recovery: %w", err)
		}
		detail, err := json.Marshal(map[string]any{
			"action":         "suspend_takeout",
			"claim_id":       recovery.ClaimID,
			"recovery_id":    recovery.ID,
			"target_entity":  "merchant",
			"target_id":      order.MerchantID,
			"suspend_reason": reason,
			"suspend_until":  suspendUntil,
			"remark":         remark,
		})
		if err != nil {
			return nil, "", fmt.Errorf("marshal merchant overdue claim recovery action detail: %w", err)
		}
		return detail, "merchant", nil
	}

	if recovery.RecoveryTarget.Valid && recovery.RecoveryTarget.String == "rider" {
		delivery, err := q.GetDeliveryByOrderID(ctx, recovery.OrderID)
		if err != nil {
			return nil, "", fmt.Errorf("get delivery for overdue claim recovery: %w", err)
		}
		if !delivery.RiderID.Valid {
			return nil, "", fmt.Errorf("claim recovery %d has rider target without rider id", recovery.ID)
		}
		detail, err := json.Marshal(map[string]any{
			"action":         "suspend_rider",
			"claim_id":       recovery.ClaimID,
			"recovery_id":    recovery.ID,
			"target_entity":  "rider",
			"target_id":      delivery.RiderID.Int64,
			"suspend_reason": reason,
			"suspend_until":  suspendUntil,
			"remark":         remark,
		})
		if err != nil {
			return nil, "", fmt.Errorf("marshal rider overdue claim recovery action detail: %w", err)
		}
		return detail, "rider", nil
	}

	return nil, "", fmt.Errorf("claim recovery %d has unsupported recovery target %q", recovery.ID, recovery.RecoveryTarget.String)
}
