package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// CreateDishTxParams contains the input parameters for creating a dish with ingredients and tags
type CreateDishTxParams struct {
	MerchantID          int64
	CategoryID          pgtype.Int8
	Name                string
	Description         pgtype.Text
	ImageMediaAssetID   pgtype.Int8
	Price               int64
	MemberPrice         pgtype.Int8
	IsAvailable         bool
	IsOnline            bool
	IsPackaging         bool
	SortOrder           int16
	PrepareTime         int16 // 预估制作时间（分钟）
	IngredientIDs       []int64
	TagIDs              []int64
	CustomizationGroups []CustomizationGroupInput
}

// CreateDishTxResult contains the result of the create dish transaction
type CreateDishTxResult struct {
	Dish                  Dish
	Ingredients           []DishIngredient
	Tags                  []DishTag
	CustomizationGroups   []DishCustomizationGroupWithOptions
	CustomizationTagNames map[int64]string
}

// CreateDishTx creates a dish with its image, ingredients, tags, and customizations in a single transaction.
// This ensures atomicity: if any follow-up step fails, the dish creation is rolled back.
func (store *SQLStore) CreateDishTx(ctx context.Context, arg CreateDishTxParams) (CreateDishTxResult, error) {
	var result CreateDishTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		if arg.CategoryID.Valid {
			_, err = q.GetMerchantDishCategoryForUpdate(ctx, GetMerchantDishCategoryForUpdateParams{
				MerchantID: arg.MerchantID,
				CategoryID: arg.CategoryID.Int64,
			})
			if err != nil {
				if errors.Is(err, ErrRecordNotFound) {
					return ErrMerchantDishCategoryNotLinked
				}
				return fmt.Errorf("lock merchant dish category: %w", err)
			}
		}

		// Step 1: Create dish
		result.Dish, err = q.CreateDish(ctx, CreateDishParams{
			MerchantID:  arg.MerchantID,
			CategoryID:  arg.CategoryID,
			Name:        arg.Name,
			Description: arg.Description,
			Price:       arg.Price,
			MemberPrice: arg.MemberPrice,
			IsAvailable: arg.IsAvailable,
			IsOnline:    arg.IsOnline,
			IsPackaging: arg.IsPackaging,
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

		// Step 4: Set dish image if provided
		if arg.ImageMediaAssetID.Valid {
			result.Dish, err = q.UpdateDish(ctx, UpdateDishParams{
				ID:                result.Dish.ID,
				ImageMediaAssetID: arg.ImageMediaAssetID,
			})
			if err != nil {
				return fmt.Errorf("set dish image: %w", err)
			}
		}

		// Step 5: Create customization groups and options if provided
		if len(arg.CustomizationGroups) > 0 {
			result.CustomizationGroups, result.CustomizationTagNames, err = replaceDishCustomizationGroups(ctx, q, result.Dish.ID, arg.CustomizationGroups)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return result, err
}

// UpdateDishTxParams contains the input parameters for updating a dish with ingredients and tags
type UpdateDishTxParams struct {
	ID                int64
	CategoryID        pgtype.Int8
	Name              pgtype.Text
	Description       pgtype.Text
	ImageMediaAssetID pgtype.Int8
	Price             pgtype.Int8
	MemberPrice       pgtype.Int8
	IsAvailable       pgtype.Bool
	IsOnline          pgtype.Bool
	IsPackaging       pgtype.Bool
	SortOrder         pgtype.Int2
	PrepareTime       pgtype.Int2
	IngredientIDs     *[]int64 // nil means don't update, empty slice means clear all
	TagIDs            *[]int64 // nil means don't update, empty slice means clear all
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

		if arg.CategoryID.Valid {
			currentDish, err := q.GetDish(ctx, arg.ID)
			if err != nil {
				return fmt.Errorf("get dish: %w", err)
			}
			_, err = q.GetMerchantDishCategoryForUpdate(ctx, GetMerchantDishCategoryForUpdateParams{
				MerchantID: currentDish.MerchantID,
				CategoryID: arg.CategoryID.Int64,
			})
			if err != nil {
				if errors.Is(err, ErrRecordNotFound) {
					return ErrMerchantDishCategoryNotLinked
				}
				return fmt.Errorf("lock merchant dish category: %w", err)
			}
		}

		_, err = q.GetDishForUpdate(ctx, arg.ID)
		if err != nil {
			return fmt.Errorf("lock dish: %w", err)
		}

		// Step 1: Update dish
		result.Dish, err = q.UpdateDish(ctx, UpdateDishParams{
			ID:                arg.ID,
			CategoryID:        arg.CategoryID,
			Name:              arg.Name,
			Description:       arg.Description,
			ImageMediaAssetID: arg.ImageMediaAssetID,
			Price:             arg.Price,
			MemberPrice:       arg.MemberPrice,
			IsAvailable:       arg.IsAvailable,
			IsOnline:          arg.IsOnline,
			IsPackaging:       arg.IsPackaging,
			SortOrder:         arg.SortOrder,
			PrepareTime:       arg.PrepareTime,
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
	Name       string
	ExtraPrice int64
	SortOrder  int16
}

// SetDishCustomizationsTxResult contains the result of setting dish customizations
type SetDishCustomizationsTxResult struct {
	Groups   []DishCustomizationGroupWithOptions
	TagNames map[int64]string
}

type SetDishFeaturedTagsTxParams struct {
	DishID int64
	Tags   []string
}

type SetDishFeaturedTagsTxResult struct {
	Tags []Tag
}

// DishCustomizationGroupWithOptions represents a group with its options
type DishCustomizationGroupWithOptions struct {
	Group   DishCustomizationGroup
	Options []DishCustomizationOption
}

func resolveCustomizationOptionTag(ctx context.Context, q *Queries, option CustomizationOptionInput) (Tag, error) {
	if option.TagID > 0 {
		tag, err := q.GetTag(ctx, option.TagID)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return Tag{}, fmt.Errorf("%w: tag %d", ErrCustomizationTagUnavailable, option.TagID)
			}
			return Tag{}, fmt.Errorf("get customization tag %d: %w", option.TagID, err)
		}
		if tag.Type != TagTypeCustomization || tag.Status != TagStatusActive {
			return Tag{}, fmt.Errorf("%w: tag %d", ErrCustomizationTagUnavailable, option.TagID)
		}
		return tag, nil
	}
	if option.TagID < 0 {
		return Tag{}, fmt.Errorf("invalid customization tag id %d", option.TagID)
	}

	name := strings.TrimSpace(option.Name)
	if name == "" {
		return Tag{}, errors.New("customization option name is required")
	}

	tag, err := q.UpsertActiveTagByNameAndType(ctx, UpsertActiveTagByNameAndTypeParams{
		Name:      name,
		Type:      TagTypeCustomization,
		SortOrder: 0,
	})
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return Tag{}, fmt.Errorf("%w: %q", ErrCustomizationTagUnavailable, name)
		}
		return Tag{}, fmt.Errorf("upsert customization tag %s: %w", name, err)
	}

	return tag, nil
}

func replaceDishCustomizationGroups(ctx context.Context, q *Queries, dishID int64, groups []CustomizationGroupInput) ([]DishCustomizationGroupWithOptions, map[int64]string, error) {
	if err := q.DeleteAllDishCustomizationGroups(ctx, dishID); err != nil {
		return nil, nil, fmt.Errorf("delete all customization groups: %w", err)
	}

	resultGroups := make([]DishCustomizationGroupWithOptions, 0, len(groups))
	tagNames := make(map[int64]string)
	for _, g := range groups {
		group, err := q.CreateDishCustomizationGroup(ctx, CreateDishCustomizationGroupParams{
			DishID:     dishID,
			Name:       g.Name,
			IsRequired: g.IsRequired,
			SortOrder:  g.SortOrder,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("create customization group %s: %w", g.Name, err)
		}

		options := make([]DishCustomizationOption, 0, len(g.Options))
		optionTagIDs := make(map[int64]struct{}, len(g.Options))
		for _, o := range g.Options {
			tag, err := resolveCustomizationOptionTag(ctx, q, o)
			if err != nil {
				return nil, nil, err
			}
			if _, exists := optionTagIDs[tag.ID]; exists {
				return nil, nil, fmt.Errorf("%w: group %q tag %d", ErrDuplicateCustomizationOption, g.Name, tag.ID)
			}
			optionTagIDs[tag.ID] = struct{}{}
			tagNames[tag.ID] = tag.Name

			option, err := q.CreateDishCustomizationOption(ctx, CreateDishCustomizationOptionParams{
				GroupID:    group.ID,
				TagID:      tag.ID,
				ExtraPrice: o.ExtraPrice,
				SortOrder:  o.SortOrder,
			})
			if err != nil {
				return nil, nil, fmt.Errorf("create customization option for tag %d: %w", tag.ID, err)
			}
			options = append(options, option)
		}

		resultGroups = append(resultGroups, DishCustomizationGroupWithOptions{
			Group:   group,
			Options: options,
		})
	}

	return resultGroups, tagNames, nil
}

// SetDishCustomizationsTx replaces all customization groups and options for a dish in a single transaction.
// This ensures atomicity: either all groups/options are replaced or none are.
func (store *SQLStore) SetDishCustomizationsTx(ctx context.Context, arg SetDishCustomizationsTxParams) (SetDishCustomizationsTxResult, error) {
	var result SetDishCustomizationsTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		groups, tagNames, err := replaceDishCustomizationGroups(ctx, q, arg.DishID, arg.Groups)
		if err != nil {
			return err
		}
		result.Groups = groups
		result.TagNames = tagNames
		return nil
	})

	return result, err
}

// SetDishFeaturedTagsTx replaces only the featured system tags for a dish.
// Non-featured dish tags are preserved, and any failure rolls back the whole replacement.
func (store *SQLStore) SetDishFeaturedTagsTx(ctx context.Context, arg SetDishFeaturedTagsTxParams) (SetDishFeaturedTagsTxResult, error) {
	var result SetDishFeaturedTagsTxResult
	featuredTagNames := normalizeFeaturedDishTagNames(arg.Tags)

	err := store.execTx(ctx, func(q *Queries) error {
		currentTags, err := q.ListDishTags(ctx, arg.DishID)
		if err != nil {
			return fmt.Errorf("list dish tags: %w", err)
		}

		for _, tag := range currentTags {
			if !isFeaturedDishTagName(tag.Name) {
				continue
			}
			if err := q.RemoveDishTag(ctx, RemoveDishTagParams{
				DishID: arg.DishID,
				TagID:  tag.ID,
			}); err != nil {
				return fmt.Errorf("remove featured dish tag %d: %w", tag.ID, err)
			}
		}

		for _, name := range featuredTagNames {
			tag, err := q.GetSystemTagByName(ctx, name)
			if err != nil {
				return fmt.Errorf("get featured system tag %q: %w", name, err)
			}
			if err := q.UpsertDishTag(ctx, UpsertDishTagParams{
				DishID: arg.DishID,
				TagID:  tag.ID,
			}); err != nil {
				return fmt.Errorf("upsert featured dish tag %d: %w", tag.ID, err)
			}
		}

		result.Tags, err = q.ListDishTags(ctx, arg.DishID)
		if err != nil {
			return fmt.Errorf("list updated dish tags: %w", err)
		}

		return nil
	})

	return result, err
}

func isFeaturedDishTagName(name string) bool {
	return name == "推荐" || name == "热卖"
}

func normalizeFeaturedDishTagNames(names []string) []string {
	featuredNames := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if !isFeaturedDishTagName(name) || seen[name] {
			continue
		}
		featuredNames = append(featuredNames, name)
		seen[name] = true
	}
	return featuredNames
}
