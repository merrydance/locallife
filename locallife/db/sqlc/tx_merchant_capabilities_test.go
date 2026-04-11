package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestUpdateMerchantCapabilitiesTx_ReconcilesSystemLabels(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	result, err := testStore.UpdateMerchantCapabilitiesTx(context.Background(), UpdateMerchantCapabilitiesTxParams{
		MerchantID:        merchant.ID,
		OpenKitchenStatus: pgtype.Text{String: MerchantCapabilityStatusYes, Valid: true},
		DineInStatus:      pgtype.Text{String: MerchantCapabilityStatusNo, Valid: true},
		Source:            MerchantCapabilitySourceManualReview,
		Note:              pgtype.Text{String: "运营复核", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, MerchantCapabilityStatusYes, result.Capability.OpenKitchenStatus)
	require.Equal(t, MerchantCapabilityStatusNo, result.Capability.DineInStatus)
	require.Equal(t, MerchantCapabilitySourceManualReview, result.Capability.Source)
	require.True(t, result.Capability.Note.Valid)
	require.Equal(t, "运营复核", result.Capability.Note.String)

	labels, err := testStore.ListMerchantSystemLabels(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Len(t, labels, 2)
	require.Equal(t, SystemTagHasOpenKitchen, labels[0].Name)
	require.Equal(t, SystemTagNoDineIn, labels[1].Name)

	result, err = testStore.UpdateMerchantCapabilitiesTx(context.Background(), UpdateMerchantCapabilitiesTxParams{
		MerchantID:        merchant.ID,
		OpenKitchenStatus: pgtype.Text{String: MerchantCapabilityStatusNo, Valid: true},
		DineInStatus:      pgtype.Text{String: MerchantCapabilityStatusUnknown, Valid: true},
		Source:            MerchantCapabilitySourceManualReview,
	})
	require.NoError(t, err)
	require.Equal(t, MerchantCapabilityStatusNo, result.Capability.OpenKitchenStatus)
	require.Equal(t, MerchantCapabilityStatusUnknown, result.Capability.DineInStatus)

	labels, err = testStore.ListMerchantSystemLabels(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Len(t, labels, 1)
	require.Equal(t, SystemTagNoOpenKitchen, labels[0].Name)
}

func TestUpdateMerchantCapabilitiesTx_RefreshesRetainedLabelSourceWithoutTimestampChurnOnSameSource(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	_, err := testStore.UpdateMerchantCapabilitiesTx(context.Background(), UpdateMerchantCapabilitiesTxParams{
		MerchantID:        merchant.ID,
		OpenKitchenStatus: pgtype.Text{String: MerchantCapabilityStatusUnknown, Valid: true},
		DineInStatus:      pgtype.Text{String: MerchantCapabilityStatusUnknown, Valid: true},
		Source:            MerchantCapabilitySourceMigration,
	})
	require.NoError(t, err)

	links, err := testStore.ListMerchantSystemLabelLinks(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, MerchantSystemLabelSourceMigration, links[0].Source)
	initialUpdatedAt := links[0].UpdatedAt

	time.Sleep(20 * time.Millisecond)

	_, err = testStore.UpdateMerchantCapabilitiesTx(context.Background(), UpdateMerchantCapabilitiesTxParams{
		MerchantID:        merchant.ID,
		OpenKitchenStatus: pgtype.Text{String: MerchantCapabilityStatusUnknown, Valid: true},
		DineInStatus:      pgtype.Text{String: MerchantCapabilityStatusUnknown, Valid: true},
		Source:            MerchantCapabilitySourceManualReview,
	})
	require.NoError(t, err)

	links, err = testStore.ListMerchantSystemLabelLinks(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, MerchantSystemLabelSourceManualOverride, links[0].Source)
	require.True(t, links[0].UpdatedAt.After(initialUpdatedAt))
	updatedAtAfterSourceRefresh := links[0].UpdatedAt

	time.Sleep(20 * time.Millisecond)

	_, err = testStore.UpdateMerchantCapabilitiesTx(context.Background(), UpdateMerchantCapabilitiesTxParams{
		MerchantID:        merchant.ID,
		OpenKitchenStatus: pgtype.Text{String: MerchantCapabilityStatusUnknown, Valid: true},
		DineInStatus:      pgtype.Text{String: MerchantCapabilityStatusUnknown, Valid: true},
		Source:            MerchantCapabilitySourceManualReview,
	})
	require.NoError(t, err)

	links, err = testStore.ListMerchantSystemLabelLinks(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, MerchantSystemLabelSourceManualOverride, links[0].Source)
	require.True(t, links[0].UpdatedAt.Equal(updatedAtAfterSourceRefresh))
}
