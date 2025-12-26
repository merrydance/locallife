package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// CreateDishTxParams contains the input parameters for creating a dish with ingredients and tags
type CreateDishTxParams struct {
	MerchantID    int64
	CategoryID    pgtype.Int8
	Name          string
	Description   pgtype.Text
	ImageUrl      pgtype.Text
	Price         int64
	MemberPrice   pgtype.Int8
	IsAvailable   bool
	IsOnline      bool
	SortOrder     int16
	PrepareTime   int16 // 预估制作时间（分钟）
	IngredientIDs []int64
	TagIDs        []int64
}

// CreateDishTxResult contains the result of the create dish transaction
type CreateDishTxResult struct {
	Dish        Dish
	Ingredients []DishIngredient
	Tags        []DishTag
}

// CreateDishTx creates a dish with all its ingredients and tags in a single transaction.
// This ensures atomicity: if adding ingredients or tags fails, the dish is rolled back.
func (store *SQLStore) CreateDishTx(ctx context.Context, arg CreateDishTxParams) (CreateDishTxResult, error) {
	var result CreateDishTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// Step 1: Create dish
		result.Dish, err = q.CreateDish(ctx, CreateDishParams{
			MerchantID:  arg.MerchantID,
			CategoryID:  arg.CategoryID,
			Name:        arg.Name,
			Description: arg.Description,
			ImageUrl:    arg.ImageUrl,
			Price:       arg.Price,
			MemberPrice: arg.MemberPrice,
			IsAvailable: arg.IsAvailable,
			IsOnline:    arg.IsOnline,
			SortOrder:   arg.SortOrder,
			PrepareTime: arg.PrepareTime,
		})
		if err != nil {
			return fmt.Errorf("create dish: %w", err)
		}

		// Step 2: Add ingredient associations
		result.Ingredients = make([]DishIngredient, 0, len(arg.IngredientIDs))
		for _, ingredientID := range arg.IngredientIDs {
			di, err := q.AddDishIngredient(ctx, AddDishIngredientParams{
				DishID:       result.Dish.ID,
				IngredientID: ingredientID,
			})
			if err != nil {
				return fmt.Errorf("add dish ingredient %d: %w", ingredientID, err)
			}
			result.Ingredients = append(result.Ingredients, di)
		}

		// Step 3: Add tag associations
		result.Tags = make([]DishTag, 0, len(arg.TagIDs))
		for _, tagID := range arg.TagIDs {
			dt, err := q.AddDishTag(ctx, AddDishTagParams{
				DishID: result.Dish.ID,
				TagID:  tagID,
			})
			if err != nil {
				return fmt.Errorf("add dish tag %d: %w", tagID, err)
			}
			result.Tags = append(result.Tags, dt)
		}

		return nil
	})

	return result, err
}

// UpdateDishTxParams contains the input parameters for updating a dish with ingredients and tags
type UpdateDishTxParams struct {
	ID            int64
	CategoryID    pgtype.Int8
	Name          pgtype.Text
	Description   pgtype.Text
	ImageUrl      pgtype.Text
	Price         pgtype.Int8
	MemberPrice   pgtype.Int8
	IsAvailable   pgtype.Bool
	IsOnline      pgtype.Bool
	SortOrder     pgtype.Int2
	PrepareTime   pgtype.Int2
	IngredientIDs *[]int64 // nil means don't update, empty slice means clear all
	TagIDs        *[]int64 // nil means don't update, empty slice means clear all
}

// UpdateDishTxResult contains the result of the update dish transaction
type UpdateDishTxResult struct {
	Dish        Dish
	Ingredients []DishIngredient
	Tags        []DishTag
}

// UpdateDishTx updates a dish with its ingredients and tags in a single transaction.
func (store *SQLStore) UpdateDishTx(ctx context.Context, arg UpdateDishTxParams) (UpdateDishTxResult, error) {
	var result UpdateDishTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// Step 1: Update dish
		result.Dish, err = q.UpdateDish(ctx, UpdateDishParams{
			ID:          arg.ID,
			CategoryID:  arg.CategoryID,
			Name:        arg.Name,
			Description: arg.Description,
			ImageUrl:    arg.ImageUrl,
			Price:       arg.Price,
			MemberPrice: arg.MemberPrice,
			IsAvailable: arg.IsAvailable,
			IsOnline:    arg.IsOnline,
			SortOrder:   arg.SortOrder,
			PrepareTime: arg.PrepareTime,
		})
		if err != nil {
			return fmt.Errorf("update dish: %w", err)
		}

		// Step 2: Update ingredients if provided
		if arg.IngredientIDs != nil {
			// Delete existing ingredient associations
			err = q.RemoveAllDishIngredients(ctx, arg.ID)
			if err != nil {
				return fmt.Errorf("delete dish ingredients: %w", err)
			}

			// Add new ingredient associations
			result.Ingredients = make([]DishIngredient, 0, len(*arg.IngredientIDs))
			for _, ingredientID := range *arg.IngredientIDs {
				di, err := q.AddDishIngredient(ctx, AddDishIngredientParams{
					DishID:       arg.ID,
					IngredientID: ingredientID,
				})
				if err != nil {
					return fmt.Errorf("add dish ingredient %d: %w", ingredientID, err)
				}
				result.Ingredients = append(result.Ingredients, di)
			}
		}

		// Step 3: Update tags if provided
		if arg.TagIDs != nil {
			// Delete existing tag associations
			err = q.RemoveAllDishTags(ctx, arg.ID)
			if err != nil {
				return fmt.Errorf("delete dish tags: %w", err)
			}

			// Add new tag associations
			result.Tags = make([]DishTag, 0, len(*arg.TagIDs))
			for _, tagID := range *arg.TagIDs {
				dt, err := q.AddDishTag(ctx, AddDishTagParams{
					DishID: arg.ID,
					TagID:  tagID,
				})
				if err != nil {
					return fmt.Errorf("add dish tag %d: %w", tagID, err)
				}
				result.Tags = append(result.Tags, dt)
			}
		}

		return nil
	})

	return result, err
}

// SetDishCustomizationsTxParams contains input parameters for setting dish customizations
type SetDishCustomizationsTxParams struct {
	DishID int64
	Groups []CustomizationGroupInput
}

// CustomizationGroupInput represents a customization group input
type CustomizationGroupInput struct {
	Name       string
	IsRequired bool
	SortOrder  int16
	Options    []CustomizationOptionInput
}

// CustomizationOptionInput represents a customization option input
type CustomizationOptionInput struct {
	TagID      int64
	ExtraPrice int64
	SortOrder  int16
}

// SetDishCustomizationsTxResult contains the result of setting dish customizations
type SetDishCustomizationsTxResult struct {
	Groups []DishCustomizationGroupWithOptions
}

// DishCustomizationGroupWithOptions represents a group with its options
type DishCustomizationGroupWithOptions struct {
	Group   DishCustomizationGroup
	Options []DishCustomizationOption
}

// SetDishCustomizationsTx replaces all customization groups and options for a dish in a single transaction.
// This ensures atomicity: either all groups/options are replaced or none are.
func (store *SQLStore) SetDishCustomizationsTx(ctx context.Context, arg SetDishCustomizationsTxParams) (SetDishCustomizationsTxResult, error) {
	var result SetDishCustomizationsTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		// Step 1: Delete all existing customization groups (CASCADE will delete options)
		err := q.DeleteAllDishCustomizationGroups(ctx, arg.DishID)
		if err != nil {
			return fmt.Errorf("delete all customization groups: %w", err)
		}

		// Step 2: Create new groups and options
		result.Groups = make([]DishCustomizationGroupWithOptions, 0, len(arg.Groups))
		for _, g := range arg.Groups {
			// Create group
			group, err := q.CreateDishCustomizationGroup(ctx, CreateDishCustomizationGroupParams{
				DishID:     arg.DishID,
				Name:       g.Name,
				IsRequired: g.IsRequired,
				SortOrder:  g.SortOrder,
			})
			if err != nil {
				return fmt.Errorf("create customization group %s: %w", g.Name, err)
			}

			// Create options
			options := make([]DishCustomizationOption, 0, len(g.Options))
			for _, o := range g.Options {
				option, err := q.CreateDishCustomizationOption(ctx, CreateDishCustomizationOptionParams{
					GroupID:    group.ID,
					TagID:      o.TagID,
					ExtraPrice: o.ExtraPrice,
					SortOrder:  o.SortOrder,
				})
				if err != nil {
					return fmt.Errorf("create customization option for tag %d: %w", o.TagID, err)
				}
				options = append(options, option)
			}

			result.Groups = append(result.Groups, DishCustomizationGroupWithOptions{
				Group:   group,
				Options: options,
			})
		}

		return nil
	})

	return result, err
}
