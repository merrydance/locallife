package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// DishWithQuantity represents a dish with its quantity in a combo set
type DishWithQuantity struct {
	DishID   int64
	Quantity int32
}

// CreateComboSetTxParams contains input parameters for creating a combo set with dishes
type CreateComboSetTxParams struct {
	MerchantID    int64
	Name          string
	Description   pgtype.Text
	OriginalPrice int64
	ComboPrice    int64
	IsOnline      bool
	Dishes        []DishWithQuantity // 菜品列表（带数量）
	TagIDs        []int64            // 标签ID列表
}

// CreateComboSetTxResult contains the result of creating a combo set
type CreateComboSetTxResult struct {
	ComboSet ComboSet
	Dishes   []ComboDish
	Tags     []ComboTag
}

// CreateComboSetTx creates a combo set with its dish associations in a single transaction.
// This ensures atomicity: if adding dishes fails, the combo set is rolled back.
func (store *SQLStore) CreateComboSetTx(ctx context.Context, arg CreateComboSetTxParams) (CreateComboSetTxResult, error) {
	var result CreateComboSetTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// Step 1: Create combo set
		result.ComboSet, err = q.CreateComboSet(ctx, CreateComboSetParams{
			MerchantID:    arg.MerchantID,
			Name:          arg.Name,
			Description:   arg.Description,
			OriginalPrice: arg.OriginalPrice,
			ComboPrice:    arg.ComboPrice,
			IsOnline:      arg.IsOnline,
		})
		if err != nil {
			return fmt.Errorf("create combo set: %w", err)
		}

		// Step 2: Add dish associations with quantities
		result.Dishes = make([]ComboDish, 0, len(arg.Dishes))
		for _, dish := range arg.Dishes {
			qty := dish.Quantity
			if qty <= 0 {
				qty = 1 // 默认数量为1
			}
			cd, err := q.AddComboDish(ctx, AddComboDishParams{
				ComboID:  result.ComboSet.ID,
				DishID:   dish.DishID,
				Quantity: int16(qty),
			})
			if err != nil {
				return fmt.Errorf("add combo dish %d: %w", dish.DishID, err)
			}
			result.Dishes = append(result.Dishes, cd)
		}

		// Step 3: Add tag associations
		result.Tags = make([]ComboTag, 0, len(arg.TagIDs))
		for _, tagID := range arg.TagIDs {
			ct, err := q.AddComboTag(ctx, AddComboTagParams{
				ComboID: result.ComboSet.ID,
				TagID:   tagID,
			})
			if err != nil {
				return fmt.Errorf("add combo tag %d: %w", tagID, err)
			}
			result.Tags = append(result.Tags, ct)
		}

		return nil
	})

	return result, err
}
