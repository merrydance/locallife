package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type ActivateMerchantCredentialLedgerEntry struct {
	DocumentType          string
	MerchantApplicationID int64
	ReviewRunID           pgtype.Int8
	MediaAssetID          int64
	NormalizedPayload     []byte
	ExpiresAt             pgtype.Timestamptz
}

type ActivateMerchantCredentialLedgersTxParams struct {
	MerchantID  int64
	ActivatedAt pgtype.Timestamptz
	Entries     []ActivateMerchantCredentialLedgerEntry
}

type ActivateRiderCredentialLedgerEntry struct {
	DocumentType       string
	RiderApplicationID int64
	ReviewRunID        pgtype.Int8
	MediaAssetID       int64
	NormalizedPayload  []byte
	ExpiresAt          pgtype.Timestamptz
}

type ActivateRiderCredentialLedgersTxParams struct {
	RiderID     int64
	ActivatedAt pgtype.Timestamptz
	Entries     []ActivateRiderCredentialLedgerEntry
}

type RestoreMerchantCredentialGovernanceTxParams struct {
	MerchantID           int64
	CredentialLedgerIDs  []int64
	ResumedAt            pgtype.Timestamptz
	TakeoutSuspendReason pgtype.Text
}

type RestoreRiderCredentialGovernanceTxParams struct {
	RiderID             int64
	CredentialLedgerIDs []int64
	ResumedAt           pgtype.Timestamptz
	SuspendReason       pgtype.Text
}

func (store *SQLStore) ActivateMerchantCredentialLedgersTx(ctx context.Context, arg ActivateMerchantCredentialLedgersTxParams) ([]CredentialLedger, error) {
	result := make([]CredentialLedger, 0, len(arg.Entries))
	if len(arg.Entries) == 0 {
		return result, nil
	}

	err := store.execTx(ctx, func(q *Queries) error {
		for _, entry := range arg.Entries {
			if _, err := q.DeactivateMerchantActiveCredentialLedger(ctx, DeactivateMerchantActiveCredentialLedgerParams{
				DeactivatedAt: arg.ActivatedAt,
				MerchantID:    pgtype.Int8{Int64: arg.MerchantID, Valid: true},
				DocumentType:  entry.DocumentType,
			}); err != nil {
				return fmt.Errorf("deactivate merchant active credential %s: %w", entry.DocumentType, err)
			}

			ledger, err := q.CreateMerchantCredentialLedger(ctx, CreateMerchantCredentialLedgerParams{
				MerchantID:            pgtype.Int8{Int64: arg.MerchantID, Valid: true},
				DocumentType:          entry.DocumentType,
				MerchantApplicationID: pgtype.Int8{Int64: entry.MerchantApplicationID, Valid: entry.MerchantApplicationID > 0},
				ReviewRunID:           entry.ReviewRunID,
				MediaAssetID:          entry.MediaAssetID,
				NormalizedPayload:     entry.NormalizedPayload,
				ExpiresAt:             entry.ExpiresAt,
				ActivatedAt:           timestamptzArgValue(arg.ActivatedAt),
			})
			if err != nil {
				return fmt.Errorf("create merchant credential %s: %w", entry.DocumentType, err)
			}
			result = append(result, ledger)
		}
		return nil
	})

	return result, err
}

func (store *SQLStore) ActivateRiderCredentialLedgersTx(ctx context.Context, arg ActivateRiderCredentialLedgersTxParams) ([]CredentialLedger, error) {
	result := make([]CredentialLedger, 0, len(arg.Entries))
	if len(arg.Entries) == 0 {
		return result, nil
	}

	err := store.execTx(ctx, func(q *Queries) error {
		for _, entry := range arg.Entries {
			if _, err := q.DeactivateRiderActiveCredentialLedger(ctx, DeactivateRiderActiveCredentialLedgerParams{
				DeactivatedAt: arg.ActivatedAt,
				RiderID:       pgtype.Int8{Int64: arg.RiderID, Valid: true},
				DocumentType:  entry.DocumentType,
			}); err != nil {
				return fmt.Errorf("deactivate rider active credential %s: %w", entry.DocumentType, err)
			}

			ledger, err := q.CreateRiderCredentialLedger(ctx, CreateRiderCredentialLedgerParams{
				RiderID:            pgtype.Int8{Int64: arg.RiderID, Valid: true},
				DocumentType:       entry.DocumentType,
				RiderApplicationID: pgtype.Int8{Int64: entry.RiderApplicationID, Valid: entry.RiderApplicationID > 0},
				ReviewRunID:        entry.ReviewRunID,
				MediaAssetID:       entry.MediaAssetID,
				NormalizedPayload:  entry.NormalizedPayload,
				ExpiresAt:          entry.ExpiresAt,
				ActivatedAt:        timestamptzArgValue(arg.ActivatedAt),
			})
			if err != nil {
				return fmt.Errorf("create rider credential %s: %w", entry.DocumentType, err)
			}
			result = append(result, ledger)
		}
		return nil
	})

	return result, err
}

func timestamptzArgValue(value pgtype.Timestamptz) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func (store *SQLStore) RestoreMerchantCredentialGovernanceTx(ctx context.Context, arg RestoreMerchantCredentialGovernanceTxParams) (int64, error) {
	var releasedRows int64
	err := store.execTx(ctx, func(q *Queries) error {
		var err error
		releasedRows, err = q.ReleaseMerchantTakeoutSuspensionIfOwned(ctx, ReleaseMerchantTakeoutSuspensionIfOwnedParams{
			MerchantID:           arg.MerchantID,
			TakeoutSuspendReason: arg.TakeoutSuspendReason,
		})
		if err != nil {
			return fmt.Errorf("release merchant takeout suspension if owned: %w", err)
		}
		if releasedRows == 0 {
			return nil
		}
		for _, ledgerID := range arg.CredentialLedgerIDs {
			if _, err := q.MarkCredentialLedgerResumed(ctx, MarkCredentialLedgerResumedParams{
				ResumedAt: arg.ResumedAt,
				ID:        ledgerID,
			}); err != nil {
				return fmt.Errorf("mark merchant credential ledger resumed %d: %w", ledgerID, err)
			}
		}
		return nil
	})
	return releasedRows, err
}

func (store *SQLStore) RestoreRiderCredentialGovernanceTx(ctx context.Context, arg RestoreRiderCredentialGovernanceTxParams) (int64, error) {
	var releasedRows int64
	err := store.execTx(ctx, func(q *Queries) error {
		var err error
		releasedRows, err = q.ReleaseRiderSuspensionIfOwned(ctx, ReleaseRiderSuspensionIfOwnedParams{
			RiderID:       arg.RiderID,
			SuspendReason: arg.SuspendReason,
		})
		if err != nil {
			return fmt.Errorf("release rider suspension if owned: %w", err)
		}
		if releasedRows == 0 {
			return nil
		}
		for _, ledgerID := range arg.CredentialLedgerIDs {
			if _, err := q.MarkCredentialLedgerResumed(ctx, MarkCredentialLedgerResumedParams{
				ResumedAt: arg.ResumedAt,
				ID:        ledgerID,
			}); err != nil {
				return fmt.Errorf("mark rider credential ledger resumed %d: %w", ledgerID, err)
			}
		}
		return nil
	})
	return releasedRows, err
}
