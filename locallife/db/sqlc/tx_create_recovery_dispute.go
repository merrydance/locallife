package db

import (
	"context"
	"fmt"
)

type CreateRecoveryDisputeWithRecoveryTxParams struct {
	ClaimID       int64
	AppellantType string
	AppellantID   int64
	Reason        string
	RegionID      int64
}

type CreateRecoveryDisputeWithRecoveryTxResult struct {
	RecoveryDispute RecoveryDispute
}

func (store *SQLStore) CreateRecoveryDisputeWithRecoveryTx(ctx context.Context, arg CreateRecoveryDisputeWithRecoveryTxParams) (CreateRecoveryDisputeWithRecoveryTxResult, error) {
	var result CreateRecoveryDisputeWithRecoveryTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		appeal, err := q.CreateRecoveryDispute(ctx, CreateRecoveryDisputeParams{
			ClaimID:       arg.ClaimID,
			AppellantType: arg.AppellantType,
			AppellantID:   arg.AppellantID,
			Reason:        arg.Reason,
			RegionID:      arg.RegionID,
		})
		if err != nil {
			return err
		}
		result.RecoveryDispute = appeal

		recovery, err := q.GetClaimRecoveryByClaimID(ctx, arg.ClaimID)
		if err == ErrRecordNotFound {
			return nil
		}
		if err != nil {
			return fmt.Errorf("get claim recovery for recovery dispute: %w", err)
		}

		updatedRecovery, err := q.MarkClaimRecoveryDisputed(ctx, recovery.ID)
		if err != nil {
			return fmt.Errorf("mark claim recovery disputed: %w", err)
		}
		if err := WriteClaimRecoveryEvent(ctx, q, updatedRecovery, ClaimRecoveryEventTypeDisputed, map[string]any{
			"recovery_dispute_id": appeal.ID,
			"claim_id":            updatedRecovery.ClaimID,
			"recovery_target":     updatedRecovery.RecoveryTarget.String,
			"recovery_amount":     updatedRecovery.RecoveryAmount,
			"status":              updatedRecovery.Status,
		}); err != nil {
			return fmt.Errorf("write claim recovery disputed event: %w", err)
		}

		return nil
	})

	return result, err
}
