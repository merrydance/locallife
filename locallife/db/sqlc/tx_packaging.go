package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// UpsertMerchantPackagingSettingsTx validates and upserts merchant packaging
// settings while locking the default option when one is selected.
func (store *SQLStore) UpsertMerchantPackagingSettingsTx(ctx context.Context, arg UpsertMerchantPackagingSettingsParams) (MerchantPackagingSetting, error) {
	var result MerchantPackagingSetting

	err := store.execTx(ctx, func(q *Queries) error {
		if arg.DefaultOptionID.Valid {
			option, err := q.GetMerchantPackagingOptionForUpdate(ctx, GetMerchantPackagingOptionForUpdateParams{
				ID:         arg.DefaultOptionID.Int64,
				MerchantID: arg.MerchantID,
			})
			if err != nil {
				if errors.Is(err, ErrRecordNotFound) {
					return ErrMerchantPackagingDefaultOptionUnavailable
				}
				return fmt.Errorf("get merchant packaging default option for update: %w", err)
			}
			if !option.IsEnabled || option.DeletedAt.Valid {
				return ErrMerchantPackagingDefaultOptionUnavailable
			}
		}

		settings, err := q.UpsertMerchantPackagingSettings(ctx, arg)
		if err != nil {
			return fmt.Errorf("upsert merchant packaging settings: %w", err)
		}

		result = settings
		return nil
	})

	return result, err
}

// UpdateMerchantPackagingOptionTx updates a packaging option and clears it from
// merchant settings when the option is disabled.
func (store *SQLStore) UpdateMerchantPackagingOptionTx(ctx context.Context, arg UpdateMerchantPackagingOptionParams) (MerchantPackagingOption, error) {
	var result MerchantPackagingOption

	err := store.execTx(ctx, func(q *Queries) error {
		option, err := q.UpdateMerchantPackagingOption(ctx, arg)
		if err != nil {
			return fmt.Errorf("update merchant packaging option: %w", err)
		}
		if !option.IsEnabled {
			if err := q.ClearMerchantPackagingDefaultOptionIfMatches(ctx, ClearMerchantPackagingDefaultOptionIfMatchesParams{
				MerchantID:      option.MerchantID,
				DefaultOptionID: pgtype.Int8{Int64: option.ID, Valid: true},
			}); err != nil {
				return fmt.Errorf("clear merchant packaging default option: %w", err)
			}
		}

		result = option
		return nil
	})

	return result, err
}

// SoftDeleteMerchantPackagingOptionTx soft-deletes a packaging option and
// clears it from merchant settings in the same transaction.
func (store *SQLStore) SoftDeleteMerchantPackagingOptionTx(ctx context.Context, arg SoftDeleteMerchantPackagingOptionParams) (MerchantPackagingOption, error) {
	var result MerchantPackagingOption

	err := store.execTx(ctx, func(q *Queries) error {
		option, err := q.SoftDeleteMerchantPackagingOption(ctx, arg)
		if err != nil {
			return fmt.Errorf("soft delete merchant packaging option: %w", err)
		}

		if err := q.ClearMerchantPackagingDefaultOptionIfMatches(ctx, ClearMerchantPackagingDefaultOptionIfMatchesParams{
			MerchantID:      option.MerchantID,
			DefaultOptionID: pgtype.Int8{Int64: option.ID, Valid: true},
		}); err != nil {
			return fmt.Errorf("clear merchant packaging default option: %w", err)
		}

		result = option
		return nil
	})

	return result, err
}
