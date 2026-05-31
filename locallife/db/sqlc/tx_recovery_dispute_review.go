package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type ReviewRecoveryDisputeWithCompensationTxParams struct {
	ID                 int64
	Status             string
	DecisionID         pgtype.Int8
	ReviewerID         pgtype.Int8
	ReviewNotes        pgtype.Text
	CompensationAmount pgtype.Int8
}

type ReviewRecoveryDisputeWithCompensationTxResult struct {
	RecoveryDispute    RecoveryDispute
	PostProcess        GetRecoveryDisputeForPostProcessRow
	CompensationAction *BehaviorAction
	ReleaseAction      *BehaviorAction
}

func (store *SQLStore) ReviewRecoveryDisputeWithCompensationTx(ctx context.Context, arg ReviewRecoveryDisputeWithCompensationTxParams) (ReviewRecoveryDisputeWithCompensationTxResult, error) {
	var result ReviewRecoveryDisputeWithCompensationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		appeal, err := q.ReviewRecoveryDispute(ctx, ReviewRecoveryDisputeParams{
			ID:                 arg.ID,
			Status:             arg.Status,
			ReviewerID:         arg.ReviewerID,
			ReviewNotes:        arg.ReviewNotes,
			CompensationAmount: arg.CompensationAmount,
		})
		if err != nil {
			return err
		}
		result.RecoveryDispute = appeal

		postProcess, err := q.GetRecoveryDisputeForPostProcess(ctx, arg.ID)
		if err != nil {
			return fmt.Errorf("get recovery dispute post process data: %w", err)
		}
		result.PostProcess = postProcess

		recoveryTarget := postProcess.AppellantType

		if arg.Status == "rejected" {
			recovery, recoveryErr := q.GetClaimRecoveryByClaimIDAndTarget(ctx, GetClaimRecoveryByClaimIDAndTargetParams{
				ClaimID:        postProcess.ClaimID,
				RecoveryTarget: pgtype.Text{String: recoveryTarget, Valid: recoveryTarget != ""},
			})
			if recoveryErr == nil {
				updatedRecovery, err := q.ResumeClaimRecoveryAfterDispute(ctx, recovery.ID)
				if err != nil && err != ErrRecordNotFound {
					return fmt.Errorf("resume claim recovery after rejected recovery dispute: %w", err)
				}
				if err == nil {
					if err := WriteClaimRecoveryEvent(ctx, q, updatedRecovery, ClaimRecoveryEventTypeResumed, map[string]any{
						"recovery_dispute_id": arg.ID,
						"claim_id":            updatedRecovery.ClaimID,
						"recovery_target":     updatedRecovery.RecoveryTarget.String,
						"recovery_amount":     updatedRecovery.RecoveryAmount,
						"status":              updatedRecovery.Status,
					}); err != nil {
						return fmt.Errorf("write claim recovery resumed event: %w", err)
					}
				}
			} else if recoveryErr != ErrRecordNotFound {
				return fmt.Errorf("get claim recovery for rejected recovery dispute: %w", recoveryErr)
			}
		}

		if arg.Status == "approved" {
			recovery, recoveryErr := q.GetClaimRecoveryByClaimIDAndTarget(ctx, GetClaimRecoveryByClaimIDAndTargetParams{
				ClaimID:        postProcess.ClaimID,
				RecoveryTarget: pgtype.Text{String: recoveryTarget, Valid: recoveryTarget != ""},
			})
			if recoveryErr == nil {
				if recovery.Status == "disputed" {
					updatedRecovery, err := q.MarkClaimRecoveryWaived(ctx, recovery.ID)
					if err != nil && err != ErrRecordNotFound {
						return fmt.Errorf("waive claim recovery after approved recovery dispute: %w", err)
					}
					if err == nil {
						if err := WriteClaimRecoveryEvent(ctx, q, updatedRecovery, ClaimRecoveryEventTypeWaived, map[string]any{
							"recovery_dispute_id": arg.ID,
							"claim_id":            updatedRecovery.ClaimID,
							"recovery_target":     updatedRecovery.RecoveryTarget.String,
							"recovery_amount":     updatedRecovery.RecoveryAmount,
							"status":              updatedRecovery.Status,
						}); err != nil {
							return fmt.Errorf("write claim recovery waived event: %w", err)
						}
						releaseAction, err := CreateClaimRecoveryReleaseActionWithDecision(ctx, q, updatedRecovery, arg.DecisionID, "approved recovery dispute release action created")
						if err != nil {
							return fmt.Errorf("create claim recovery release action after approved recovery dispute: %w", err)
						}
						result.ReleaseAction = releaseAction
					}
				}
			} else if recoveryErr != ErrRecordNotFound {
				return fmt.Errorf("get claim recovery for approved recovery dispute: %w", recoveryErr)
			}
		}

		if arg.Status != "approved" || !arg.CompensationAmount.Valid || arg.CompensationAmount.Int64 <= 0 {
			return nil
		}

		var payoutUserID int64
		switch postProcess.AppellantType {
		case "merchant":
			merchant, err := q.GetMerchant(ctx, postProcess.AppellantID)
			if err != nil {
				return fmt.Errorf("get merchant for recovery dispute compensation: %w", err)
			}
			payoutUserID = merchant.OwnerUserID
		case "rider":
			rider, err := q.GetRider(ctx, postProcess.AppellantID)
			if err != nil {
				return fmt.Errorf("get rider for recovery dispute compensation: %w", err)
			}
			payoutUserID = rider.UserID
		default:
			return fmt.Errorf("unsupported recovery dispute appellant type: %s", postProcess.AppellantType)
		}

		decisions, err := q.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: postProcess.OrderID, Valid: true})
		if err != nil {
			return fmt.Errorf("list behavior decisions for recovery dispute compensation: %w", err)
		}
		if len(decisions) == 0 {
			return fmt.Errorf("no behavior decision found for order %d", postProcess.OrderID)
		}

		detail, _ := json.Marshal(map[string]any{
			"action":              "recovery_dispute_compensation",
			"recovery_dispute_id": arg.ID,
			"claim_id":            postProcess.ClaimID,
			"user_id":             payoutUserID,
			"amount":              arg.CompensationAmount.Int64,
			"source_type":         "platform",
			"source_id":           0,
			"remark":              "recovery dispute compensation",
		})

		action, err := q.CreateBehaviorAction(ctx, CreateBehaviorActionParams{
			DecisionID:   decisions[0].ID,
			ActionType:   "payout",
			TargetEntity: "user",
			Status:       "created",
			Detail:       detail,
		})
		if err != nil {
			return fmt.Errorf("create recovery dispute compensation action: %w", err)
		}
		result.CompensationAction = &action

		return nil
	})

	return result, err
}
