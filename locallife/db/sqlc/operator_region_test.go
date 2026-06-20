package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListOperatorRegionRelationsIncludesSuspendedForDisplay(t *testing.T) {
	ctx := context.Background()
	activeRegion := createRandomRegion(t)
	suspendedRegion := createRandomRegion(t)
	operator := createRandomOperatorForRegion(t, activeRegion.ID)

	activeRelation, err := testStore.AddOperatorRegion(ctx, AddOperatorRegionParams{
		OperatorID: operator.ID,
		RegionID:   activeRegion.ID,
	})
	require.NoError(t, err)

	suspendedRelation, err := testStore.AddOperatorRegion(ctx, AddOperatorRegionParams{
		OperatorID: operator.ID,
		RegionID:   suspendedRegion.ID,
	})
	require.NoError(t, err)
	suspendedRelation, err = testStore.UpdateOperatorRegionStatus(ctx, UpdateOperatorRegionStatusParams{
		OperatorID: operator.ID,
		RegionID:   suspendedRegion.ID,
		Status:     OperatorRegionStatusSuspended,
	})
	require.NoError(t, err)

	activeOnly, err := testStore.ListOperatorRegions(ctx, operator.ID)
	require.NoError(t, err)
	require.Len(t, activeOnly, 1)
	require.Equal(t, activeRelation.RegionID, activeOnly[0].RegionID)
	require.Equal(t, OperatorRegionStatusActive, activeOnly[0].Status)

	displayRows, err := testStore.ListOperatorRegionRelations(ctx, operator.ID)
	require.NoError(t, err)
	require.Len(t, displayRows, 2)

	statusByRegion := map[int64]string{}
	for _, row := range displayRows {
		statusByRegion[row.RegionID] = row.Status
	}

	require.Equal(t, activeRelation.Status, statusByRegion[activeRelation.RegionID])
	require.Equal(t, suspendedRelation.Status, statusByRegion[suspendedRelation.RegionID])
}
