package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type ReviewAppealWithCompensationTxParams struct {
	ID                 int64
	Status             string
	ReviewerID         pgtype.Int8
	ReviewNotes        pgtype.Text
	CompensationAmount pgtype.Int8
}

type ReviewAppealWithCompensationTxResult struct {
	Appeal             Appeal
	PostProcess        GetAppealForPostProcessRow
	CompensationAction *BehaviorAction
}

func (store *SQLStore) ReviewAppealWithCompensationTx(ctx context.Context, arg ReviewAppealWithCompensationTxParams) (ReviewAppealWithCompensationTxResult, error) {
	var result ReviewAppealWithCompensationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		appeal, err := q.ReviewAppeal(ctx, ReviewAppealParams{
			ID:                 arg.ID,
			Status:             arg.Status,
			ReviewerID:         arg.ReviewerID,
			ReviewNotes:        arg.ReviewNotes,
			CompensationAmount: arg.CompensationAmount,
		})
		if err != nil {
			return err
		}
		result.Appeal = appeal

		postProcess, err := q.GetAppealForPostProcess(ctx, arg.ID)
		if err != nil {
			return fmt.Errorf("get appeal post process data: %w", err)
		}
		result.PostProcess = postProcess

		if arg.Status == "rejected" {
			recovery, recoveryErr := q.GetClaimRecoveryByClaimID(ctx, postProcess.ClaimID)
			if recoveryErr == nil {
				if _, err := q.ResumeClaimRecoveryAfterAppeal(ctx, recovery.ID); err != nil && err != ErrRecordNotFound {
					return fmt.Errorf("resume claim recovery after rejected appeal: %w", err)
				}
			} else if recoveryErr != ErrRecordNotFound {
				return fmt.Errorf("get claim recovery for rejected appeal: %w", recoveryErr)
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
				return fmt.Errorf("get merchant for appeal compensation: %w", err)
			}
			payoutUserID = merchant.OwnerUserID
		case "rider":
			rider, err := q.GetRider(ctx, postProcess.AppellantID)
			if err != nil {
				return fmt.Errorf("get rider for appeal compensation: %w", err)
			}
			payoutUserID = rider.UserID
		default:
			return fmt.Errorf("unsupported appeal appellant type: %s", postProcess.AppellantType)
		}

		decisions, err := q.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: postProcess.OrderID, Valid: true})
		if err != nil {
			return fmt.Errorf("list behavior decisions for appeal compensation: %w", err)
		}
		if len(decisions) == 0 {
			return fmt.Errorf("no behavior decision found for order %d", postProcess.OrderID)
		}

		detail, _ := json.Marshal(map[string]any{
			"action":      "appeal_compensation",
			"appeal_id":   arg.ID,
			"claim_id":    postProcess.ClaimID,
			"user_id":     payoutUserID,
			"amount":      arg.CompensationAmount.Int64,
			"source_type": "platform",
			"source_id":   0,
			"remark":      "appeal compensation",
		})

		action, err := q.CreateBehaviorAction(ctx, CreateBehaviorActionParams{
			DecisionID:   decisions[0].ID,
			ActionType:   "payout",
			TargetEntity: "user",
			Status:       "created",
			Detail:       detail,
		})
		if err != nil {
			return fmt.Errorf("create appeal compensation action: %w", err)
		}
		result.CompensationAction = &action

		return nil
	})

	return result, err
}
