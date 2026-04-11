package db

import (
	"context"
	"fmt"
)

type merchantCapabilityDefaultsQuerier interface {
	UpsertMerchantCapabilitiesDefaults(ctx context.Context, merchantID int64) error
	GetMerchantCapabilities(ctx context.Context, merchantID int64) (MerchantCapability, error)
}

type merchantSystemLabelCatalogQuerier interface {
	ListAllTagsByType(ctx context.Context, type_ string) ([]Tag, error)
}

type merchantSystemLabelReconcilerQuerier interface {
	merchantCapabilityDefaultsQuerier
	ListMerchantSystemLabelLinks(ctx context.Context, merchantID int64) ([]MerchantSystemLabel, error)
	UpsertMerchantSystemLabel(ctx context.Context, arg UpsertMerchantSystemLabelParams) error
	RemoveMerchantSystemLabel(ctx context.Context, arg RemoveMerchantSystemLabelParams) error
}

type MerchantSystemLabelCatalog struct {
	HasOpenKitchenTagID int64
	NoOpenKitchenTagID  int64
	NoDineInTagID       int64
}

func LoadMerchantSystemLabelCatalog(ctx context.Context, q merchantSystemLabelCatalogQuerier) (MerchantSystemLabelCatalog, error) {
	tags, err := q.ListAllTagsByType(ctx, "system")
	if err != nil {
		return MerchantSystemLabelCatalog{}, err
	}

	var catalog MerchantSystemLabelCatalog
	for _, tag := range tags {
		switch tag.Name {
		case SystemTagHasOpenKitchen:
			catalog.HasOpenKitchenTagID = tag.ID
		case SystemTagNoOpenKitchen:
			catalog.NoOpenKitchenTagID = tag.ID
		case SystemTagNoDineIn:
			catalog.NoDineInTagID = tag.ID
		}
	}

	if catalog.HasOpenKitchenTagID == 0 || catalog.NoOpenKitchenTagID == 0 || catalog.NoDineInTagID == 0 {
		return MerchantSystemLabelCatalog{}, fmt.Errorf("merchant system label catalog incomplete")
	}

	return catalog, nil
}

func EnsureMerchantCapabilities(ctx context.Context, q merchantCapabilityDefaultsQuerier, merchantID int64) (MerchantCapability, error) {
	if err := q.UpsertMerchantCapabilitiesDefaults(ctx, merchantID); err != nil {
		return MerchantCapability{}, err
	}

	return q.GetMerchantCapabilities(ctx, merchantID)
}

func DesiredMerchantSystemLabelTagIDs(capability MerchantCapability, catalog MerchantSystemLabelCatalog) map[int64]struct{} {
	desired := make(map[int64]struct{})

	switch capability.OpenKitchenStatus {
	case MerchantCapabilityStatusYes:
		desired[catalog.HasOpenKitchenTagID] = struct{}{}
	case MerchantCapabilityStatusUnknown, MerchantCapabilityStatusNo:
		desired[catalog.NoOpenKitchenTagID] = struct{}{}
	}

	if capability.DineInStatus == MerchantCapabilityStatusNo {
		desired[catalog.NoDineInTagID] = struct{}{}
	}

	return desired
}

func ReconcileMerchantSystemLabels(ctx context.Context, q merchantSystemLabelReconcilerQuerier, merchantID int64, catalog MerchantSystemLabelCatalog, source string) error {
	capability, err := EnsureMerchantCapabilities(ctx, q, merchantID)
	if err != nil {
		return err
	}

	desired := DesiredMerchantSystemLabelTagIDs(capability, catalog)
	current, err := q.ListMerchantSystemLabelLinks(ctx, merchantID)
	if err != nil {
		return err
	}

	currentByTagID := make(map[int64]MerchantSystemLabel, len(current))
	for _, link := range current {
		currentByTagID[link.TagID] = link
		if _, keep := desired[link.TagID]; !keep {
			if err := q.RemoveMerchantSystemLabel(ctx, RemoveMerchantSystemLabelParams{
				MerchantID: merchantID,
				TagID:      link.TagID,
			}); err != nil {
				return err
			}
		}
	}

	for tagID := range desired {
		if current, exists := currentByTagID[tagID]; exists && current.Source == source {
			continue
		}
		if err := q.UpsertMerchantSystemLabel(ctx, UpsertMerchantSystemLabelParams{
			MerchantID: merchantID,
			TagID:      tagID,
			Source:     source,
		}); err != nil {
			return err
		}
	}

	return nil
}
