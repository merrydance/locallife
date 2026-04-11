package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type UpdateMerchantCapabilitiesTxParams struct {
	MerchantID        int64
	OpenKitchenStatus pgtype.Text
	DineInStatus      pgtype.Text
	Source            string
	Note              pgtype.Text
}

type UpdateMerchantCapabilitiesTxResult struct {
	Capability   MerchantCapability
	SystemLabels []Tag
}

func merchantCapabilitySourceToSystemLabelSource(capabilitySource string) string {
	switch capabilitySource {
	case MerchantCapabilitySourceManualReview:
		return MerchantSystemLabelSourceManualOverride
	case MerchantCapabilitySourceMigration:
		return MerchantSystemLabelSourceMigration
	default:
		return MerchantSystemLabelSourceReconciler
	}
}

func (store *SQLStore) UpdateMerchantCapabilitiesTx(ctx context.Context, arg UpdateMerchantCapabilitiesTxParams) (UpdateMerchantCapabilitiesTxResult, error) {
	var result UpdateMerchantCapabilitiesTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		capability, err := q.UpsertMerchantCapabilities(ctx, UpsertMerchantCapabilitiesParams{
			MerchantID:        arg.MerchantID,
			OpenKitchenStatus: arg.OpenKitchenStatus,
			DineInStatus:      arg.DineInStatus,
			Source:            pgtype.Text{String: arg.Source, Valid: arg.Source != ""},
			Note:              arg.Note,
		})
		if err != nil {
			return fmt.Errorf("upsert merchant capabilities: %w", err)
		}
		result.Capability = capability

		catalog, err := LoadMerchantSystemLabelCatalog(ctx, q)
		if err != nil {
			return fmt.Errorf("load merchant system label catalog: %w", err)
		}
		labelSource := merchantCapabilitySourceToSystemLabelSource(arg.Source)
		if err := ReconcileMerchantSystemLabels(ctx, q, arg.MerchantID, catalog, labelSource); err != nil {
			return fmt.Errorf("reconcile merchant system labels: %w", err)
		}

		labels, err := q.ListMerchantSystemLabels(ctx, arg.MerchantID)
		if err != nil {
			return fmt.Errorf("list merchant system labels: %w", err)
		}
		result.SystemLabels = labels
		return nil
	})

	return result, err
}
