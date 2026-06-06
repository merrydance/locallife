package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// RenameMerchantDishCategoryTxParams contains the input for the rename-category transaction.
type RenameMerchantDishCategoryTxParams struct {
	MerchantID    int64
	OldCategoryID int64
	NewName       string
	SortOrder     int16
}

// RenameMerchantDishCategoryTxResult contains the result of the rename-category transaction.
type RenameMerchantDishCategoryTxResult struct {
	NewCategoryID   int64
	NewCategoryName string
	SortOrder       int16
}

// RenameMerchantDishCategoryTx atomically renames a merchant's dish category by:
//  1. Creating (or reusing) a global dish category with the new name.
//  2. Linking the merchant to the new category.
//  3. Re-assigning all dishes under the old category to the new one.
//  4. Unlinking the merchant from the old category.
//
// All four steps run inside a single database transaction, ensuring no partial
// state is left behind if any step fails.
func (store *SQLStore) RenameMerchantDishCategoryTx(ctx context.Context, arg RenameMerchantDishCategoryTxParams) (RenameMerchantDishCategoryTxResult, error) {
	var result RenameMerchantDishCategoryTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		// Step 1: Create (or upsert via INSERT ... ON CONFLICT DO NOTHING RETURNING *) the
		// global dish category with the new name.
		newCategory, err := q.CreateDishCategory(ctx, arg.NewName)
		if err != nil {
			return fmt.Errorf("create new dish category: %w", err)
		}

		categoryIDs := []int64{arg.OldCategoryID}
		if newCategory.ID != arg.OldCategoryID {
			categoryIDs = append(categoryIDs, newCategory.ID)
			if categoryIDs[1] < categoryIDs[0] {
				categoryIDs[0], categoryIDs[1] = categoryIDs[1], categoryIDs[0]
			}
		}

		for _, categoryID := range categoryIDs {
			_, err = q.GetMerchantDishCategoryForUpdate(ctx, GetMerchantDishCategoryForUpdateParams{
				MerchantID: arg.MerchantID,
				CategoryID: categoryID,
			})
			if err != nil {
				if errors.Is(err, ErrRecordNotFound) {
					if categoryID == arg.OldCategoryID {
						return ErrMerchantDishCategoryNotLinked
					}
					continue
				}
				return fmt.Errorf("lock merchant dish category %d: %w", categoryID, err)
			}
		}

		// Step 2: Link the merchant to the new global category.
		mdc, err := q.LinkMerchantDishCategory(ctx, LinkMerchantDishCategoryParams{
			MerchantID: arg.MerchantID,
			CategoryID: newCategory.ID,
			SortOrder:  arg.SortOrder,
		})
		if err != nil {
			return fmt.Errorf("link merchant to new dish category: %w", err)
		}

		// Step 3: Migrate all dishes from the old category to the new one.
		if err = q.UpdateDishesCategory(ctx, UpdateDishesCategoryParams{
			MerchantID:    arg.MerchantID,
			OldCategoryID: pgtype.Int8{Int64: arg.OldCategoryID, Valid: true},
			NewCategoryID: pgtype.Int8{Int64: newCategory.ID, Valid: true},
		}); err != nil {
			return fmt.Errorf("migrate dishes to new category: %w", err)
		}

		// Step 4: Remove the merchant's link to the old category.
		if err = q.UnlinkMerchantDishCategory(ctx, UnlinkMerchantDishCategoryParams{
			MerchantID: arg.MerchantID,
			CategoryID: arg.OldCategoryID,
		}); err != nil {
			return fmt.Errorf("unlink old dish category: %w", err)
		}

		result.NewCategoryID = newCategory.ID
		result.NewCategoryName = newCategory.Name
		result.SortOrder = mdc.SortOrder
		return nil
	})

	return result, err
}

// UnlinkUnusedMerchantDishCategoryTx locks the merchant-category link before
// unlinking so concurrent dish create/update requests cannot add a live
// reference between the active-dish check and the unlink.
func (store *SQLStore) UnlinkUnusedMerchantDishCategoryTx(ctx context.Context, arg UnlinkUnusedMerchantDishCategoryParams) (MerchantDishCategory, error) {
	var result MerchantDishCategory

	err := store.execTx(ctx, func(q *Queries) error {
		_, err := q.GetMerchantDishCategoryForUpdate(ctx, GetMerchantDishCategoryForUpdateParams{
			MerchantID: arg.MerchantID,
			CategoryID: arg.CategoryID,
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrMerchantDishCategoryNotLinked
			}
			return fmt.Errorf("lock merchant dish category: %w", err)
		}

		result, err = q.UnlinkUnusedMerchantDishCategory(ctx, arg)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrMerchantDishCategoryHasActiveDishes
			}
			return fmt.Errorf("unlink unused merchant dish category: %w", err)
		}

		return nil
	})

	return result, err
}
