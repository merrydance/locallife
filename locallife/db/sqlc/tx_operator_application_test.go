package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestApproveOperatorApplicationTx_CreatesOperatorAndRoleAtomically(t *testing.T) {
	app := createCompleteOperatorApplication(t)
	submitted, err := testStore.SubmitOperatorApplication(context.Background(), app.ID)
	require.NoError(t, err)

	reviewer := createRandomUser(t)
	now := time.Now()
	end := now.AddDate(2, 0, 0)

	var startDate pgtype.Date
	require.NoError(t, startDate.Scan(now))
	var endDate pgtype.Date
	require.NoError(t, endDate.Scan(end))

	result, err := testStore.(*SQLStore).ApproveOperatorApplicationTx(context.Background(), ApproveOperatorApplicationTxParams{
		ApplicationID:     submitted.ID,
		ReviewedBy:        pgtype.Int8{Int64: reviewer.ID, Valid: true},
		OperatorName:      "测试运营商",
		ContactName:       "张三",
		ContactPhone:      "13812345678",
		ContractStartDate: startDate,
		ContractEndDate:   endDate,
		ContractYears:     2,
	})
	require.NoError(t, err)
	require.Equal(t, "approved", result.Application.Status)
	require.Equal(t, "active", result.Operator.Status)

	operatorByUser, err := testStore.GetOperatorByUser(context.Background(), app.UserID)
	require.NoError(t, err)
	require.Equal(t, result.Operator.ID, operatorByUser.ID)

	regions, err := testStore.ListOperatorRegions(context.Background(), result.Operator.ID)
	require.NoError(t, err)
	require.Len(t, regions, 1)
	require.Equal(t, result.Operator.RegionID, regions[0].RegionID)

	role, err := testStore.GetUserRoleByType(context.Background(), GetUserRoleByTypeParams{
		UserID: app.UserID,
		Role:   "operator",
	})
	require.NoError(t, err)
	require.Equal(t, "active", role.Status)
	require.True(t, role.RelatedEntityID.Valid)
	require.Equal(t, result.Operator.ID, role.RelatedEntityID.Int64)
}
