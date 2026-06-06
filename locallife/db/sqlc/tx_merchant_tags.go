package db

import (
	"context"
	"fmt"
)

type SetMerchantTagsTxParams struct {
	MerchantID int64
	TagIDs     []int64
}

type SetMerchantTagsTxResult struct {
	Tags []Tag
}

// SetMerchantTagsTx replaces all merchant category tags in a single transaction.
func (store *SQLStore) SetMerchantTagsTx(ctx context.Context, arg SetMerchantTagsTxParams) (SetMerchantTagsTxResult, error) {
	var result SetMerchantTagsTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		if _, err := q.LockMerchantForUpdate(ctx, arg.MerchantID); err != nil {
			return fmt.Errorf("lock merchant: %w", err)
		}

		if err := q.ClearMerchantTags(ctx, arg.MerchantID); err != nil {
			return fmt.Errorf("clear merchant tags: %w", err)
		}

		for _, tagID := range arg.TagIDs {
			if err := q.AddMerchantTag(ctx, AddMerchantTagParams{
				MerchantID: arg.MerchantID,
				TagID:      tagID,
			}); err != nil {
				return fmt.Errorf("add merchant tag %d: %w", tagID, err)
			}
		}

		tags, err := q.ListMerchantTags(ctx, arg.MerchantID)
		if err != nil {
			return fmt.Errorf("list merchant tags: %w", err)
		}
		result.Tags = tags
		return nil
	})

	return result, err
}
