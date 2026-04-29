package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestApproveOperatorRegionApplicationTx_AddsRegionAndApprovesApplicationAtomically(t *testing.T) {
	operator := createRandomOperatorForRegion(t, createRandomRegion(t).ID)
	targetRegion := createRandomRegion(t)
	application, err := testStore.CreateOperatorRegionApplication(context.Background(), CreateOperatorRegionApplicationParams{
		OperatorID: operator.ID,
		RegionID:   targetRegion.ID,
	})
	require.NoError(t, err)

	result, err := testStore.(*SQLStore).ApproveOperatorRegionApplicationTx(context.Background(), application.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", result.Application.Status)
	require.Equal(t, operator.ID, result.OperatorRegion.OperatorID)
	require.Equal(t, targetRegion.ID, result.OperatorRegion.RegionID)

	manages, err := testStore.CheckOperatorManagesRegion(context.Background(), CheckOperatorManagesRegionParams{
		OperatorID: operator.ID,
		RegionID:   targetRegion.ID,
	})
	require.NoError(t, err)
	require.True(t, manages)
}

func TestApproveOperatorRegionApplicationTx_RejectsRepeatedApprovalWithoutAddingRegion(t *testing.T) {
	operator := createRandomOperatorForRegion(t, createRandomRegion(t).ID)
	targetRegion := createRandomRegion(t)
	application, err := testStore.CreateOperatorRegionApplication(context.Background(), CreateOperatorRegionApplicationParams{
		OperatorID: operator.ID,
		RegionID:   targetRegion.ID,
	})
	require.NoError(t, err)
	_, err = testStore.RejectOperatorRegionApplication(context.Background(), RejectOperatorRegionApplicationParams{
		ID: application.ID,
	})
	require.NoError(t, err)

	_, err = testStore.(*SQLStore).ApproveOperatorRegionApplicationTx(context.Background(), application.ID)
	require.Error(t, err)

	manages, err := testStore.CheckOperatorManagesRegion(context.Background(), CheckOperatorManagesRegionParams{
		OperatorID: operator.ID,
		RegionID:   targetRegion.ID,
	})
	require.NoError(t, err)
	require.False(t, manages)
}

func TestApproveOperatorRegionApplicationTx_RollsBackApprovalWhenAddingRegionFails(t *testing.T) {
	operator := createRandomOperatorForRegion(t, createRandomRegion(t).ID)
	targetRegion := createRandomRegion(t)
	application, err := testStore.CreateOperatorRegionApplication(context.Background(), CreateOperatorRegionApplicationParams{
		OperatorID: operator.ID,
		RegionID:   targetRegion.ID,
	})
	require.NoError(t, err)
	_, err = testStore.AddOperatorRegion(context.Background(), AddOperatorRegionParams{
		OperatorID: operator.ID,
		RegionID:   targetRegion.ID,
	})
	require.NoError(t, err)

	_, err = testStore.(*SQLStore).ApproveOperatorRegionApplicationTx(context.Background(), application.ID)
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, UniqueViolation, pgErr.Code)

	reloaded, err := testStore.GetOperatorRegionApplication(context.Background(), application.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", reloaded.Status)
}
