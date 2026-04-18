package db

import (
	"context"
	"fmt"
)

type CreateAppealWithRecoveryTxParams struct {
	ClaimID       int64
	AppellantType string
	AppellantID   int64
	Reason        string
	RegionID      int64
}

type CreateAppealWithRecoveryTxResult struct {
	Appeal Appeal
}

func (store *SQLStore) CreateAppealWithRecoveryTx(ctx context.Context, arg CreateAppealWithRecoveryTxParams) (CreateAppealWithRecoveryTxResult, error) {
	var result CreateAppealWithRecoveryTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		appeal, err := q.CreateAppeal(ctx, CreateAppealParams{
			ClaimID:       arg.ClaimID,
			AppellantType: arg.AppellantType,
			AppellantID:   arg.AppellantID,
			Reason:        arg.Reason,
			RegionID:      arg.RegionID,
		})
		if err != nil {
			return err
		}
		result.Appeal = appeal

		recovery, err := q.GetClaimRecoveryByClaimID(ctx, arg.ClaimID)
		if err == ErrRecordNotFound {
			return nil
		}
		if err != nil {
			return fmt.Errorf("get claim recovery for appeal: %w", err)
		}

		if _, err := q.MarkClaimRecoveryAppealed(ctx, recovery.ID); err != nil {
			return fmt.Errorf("mark claim recovery appealed: %w", err)
		}

		return nil
	})

	return result, err
}
