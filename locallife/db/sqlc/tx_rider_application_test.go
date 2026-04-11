package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func prepareSubmittedRiderApplication(t *testing.T, userID int64) RiderApplication {
	t.Helper()

	app := createRandomRiderApplicationWithUser(t, userID)

	_, err := testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "张三", Valid: true},
		Phone:    pgtype.Text{String: "13812345678", Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{},
		IDCardBackMediaAssetID:  pgtype.Int8{},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{},
	})
	require.NoError(t, err)

	app, err = testStore.SubmitRiderApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", app.Status)

	return app
}

func TestApproveRiderApplicationTx_AssignsRiderRole(t *testing.T) {
	user := createRandomUser(t)
	app := prepareSubmittedRiderApplication(t, user.ID)
	idCardNo := util.RandomString(18)

	result, err := testStore.ApproveRiderApplicationTx(context.Background(), ApproveRiderApplicationTxParams{
		ApplicationID: app.ID,
		RiderRealName: "张三",
		RiderIDCardNo: idCardNo,
		RiderPhone:    "13812345678",
	})
	require.NoError(t, err)
	require.Equal(t, "approved", result.Application.Status)
	require.Equal(t, RiderStatusApproved, result.Rider.Status)

	role, err := testStore.GetUserRoleByType(context.Background(), GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   "rider",
	})
	require.NoError(t, err)
	require.Equal(t, "active", role.Status)
	require.True(t, role.RelatedEntityID.Valid)
	require.Equal(t, result.Rider.ID, role.RelatedEntityID.Int64)
}

func TestApproveRiderApplicationTx_FailsWhenRiderRoleAlreadyExists(t *testing.T) {
	user := createRandomUser(t)
	app := prepareSubmittedRiderApplication(t, user.ID)
	idCardNo := util.RandomString(18)

	staleRole := createRandomUserRoleForUser(t, user.ID, "rider")
	_, err := testStore.UpdateUserRoleStatus(context.Background(), UpdateUserRoleStatusParams{
		ID:     staleRole.ID,
		Status: "inactive",
	})
	require.NoError(t, err)

	_, err = testStore.ApproveRiderApplicationTx(context.Background(), ApproveRiderApplicationTxParams{
		ApplicationID: app.ID,
		RiderRealName: "张三",
		RiderIDCardNo: idCardNo,
		RiderPhone:    "13812345678",
	})
	require.Error(t, err)
}
