package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

type CreateMerchantSelectableTagTxParams struct {
	MerchantID      int64
	Name            string
	Type            string
	SortOrder       int16
	CreatedByUserID pgtype.Int8
}

func isMerchantSelectableTagType(tagType string) bool {
	switch tagType {
	case TagTypeDish, TagTypeTable, TagTypeCombo, TagTypeCustomization:
		return true
	default:
		return false
	}
}

// CreateMerchantSelectableTagTx creates or reuses an active global tag, then
// idempotently links it into the merchant's selectable tag set.
func (store *SQLStore) CreateMerchantSelectableTagTx(ctx context.Context, arg CreateMerchantSelectableTagTxParams) (Tag, error) {
	var result Tag

	err := store.execTx(ctx, func(q *Queries) error {
		name := strings.TrimSpace(arg.Name)
		if name == "" {
			return ErrInvalidTagName
		}
		if !isMerchantSelectableTagType(arg.Type) {
			return ErrTagTypeNotSelectable
		}

		if _, err := q.LockMerchantForUpdate(ctx, arg.MerchantID); err != nil {
			return fmt.Errorf("lock merchant: %w", err)
		}

		tag, err := q.UpsertActiveTagByNameAndType(ctx, UpsertActiveTagByNameAndTypeParams{
			Name:      name,
			Type:      arg.Type,
			SortOrder: arg.SortOrder,
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrTagNameReservedInactive
			}
			return fmt.Errorf("upsert active tag: %w", err)
		}

		link, err := q.LinkMerchantSelectableTag(ctx, LinkMerchantSelectableTagParams{
			MerchantID:      arg.MerchantID,
			TagID:           tag.ID,
			SortOrder:       arg.SortOrder,
			CreatedByUserID: arg.CreatedByUserID,
		})
		if err != nil {
			return fmt.Errorf("link merchant selectable tag: %w", err)
		}

		tag.SortOrder = link.SortOrder
		result = tag
		return nil
	})

	return result, err
}
