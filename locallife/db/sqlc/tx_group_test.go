package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createSubmittedGroupApplicationForTxTest(t *testing.T) MerchantGroupApplication {
	t.Helper()

	user := createRandomUser(t)
	region := createRandomRegion(t)
	app, err := testStore.CreateGroupApplicationDraft(context.Background(), user.ID)
	require.NoError(t, err)

	app, err = testStore.UpdateGroupApplicationBasic(context.Background(), UpdateGroupApplicationBasicParams{
		ID:            app.ID,
		GroupName:     "group_" + util.RandomString(8),
		ContactPhone:  "13800138000",
		LicenseNumber: pgtype.Text{String: "LIC-" + util.RandomString(8), Valid: true},
		Address:       pgtype.Text{String: "测试集团地址", Valid: true},
		RegionID:      pgtype.Int8{Int64: region.ID, Valid: true},
	})
	require.NoError(t, err)

	app, err = testStore.SubmitGroupApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", app.Status)
	return app
}

func TestApproveGroupApplicationTxReturnsConflictAfterTerminalReview(t *testing.T) {
	reviewer := createRandomUser(t)
	app := createSubmittedGroupApplicationForTxTest(t)

	result, err := testStore.ApproveGroupApplicationTx(context.Background(), ApproveGroupApplicationTxParams{
		ApplicationID:  app.ID,
		ReviewerUserID: reviewer.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "approved", result.Application.Status)
	require.NotZero(t, result.Group.ID)

	second, err := testStore.ApproveGroupApplicationTx(context.Background(), ApproveGroupApplicationTxParams{
		ApplicationID:  app.ID,
		ReviewerUserID: reviewer.ID,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrGroupApplicationReviewConflict))
	require.Zero(t, second.Group.ID)
}

func TestReviewSubmittedGroupApplicationDoesNotOverwriteApprovedApplication(t *testing.T) {
	reviewer := createRandomUser(t)
	app := createSubmittedGroupApplicationForTxTest(t)
	approved, err := testStore.ApproveGroupApplicationTx(context.Background(), ApproveGroupApplicationTxParams{
		ApplicationID:  app.ID,
		ReviewerUserID: reviewer.ID,
	})
	require.NoError(t, err)

	rejected, err := testStore.ReviewSubmittedGroupApplication(context.Background(), ReviewSubmittedGroupApplicationParams{
		ID:           app.ID,
		Status:       "rejected",
		RejectReason: pgtype.Text{String: "资料不完整", Valid: true},
		ReviewedBy:   pgtype.Int8{Int64: reviewer.ID, Valid: true},
		ReviewedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
	require.Empty(t, rejected.Status)

	current, err := testStore.GetGroupApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", current.Status)
	require.Equal(t, approved.Application.ID, current.ID)
}

func TestApproveGroupJoinRequestTxDoesNotOverwriteExistingMerchantAffiliation(t *testing.T) {
	reviewer := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	existingGroup := createGroupForJoinTxTest(t, reviewer.ID)
	targetGroup := createGroupForJoinTxTest(t, reviewer.ID)

	err := testStore.UpdateMerchantGroupAffiliation(context.Background(), UpdateMerchantGroupAffiliationParams{
		ID:      merchant.ID,
		GroupID: pgtype.Int8{Int64: existingGroup.ID, Valid: true},
		BrandID: pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)

	joinReq, err := testStore.CreateGroupJoinRequest(context.Background(), CreateGroupJoinRequestParams{
		GroupID:         targetGroup.ID,
		MerchantID:      merchant.ID,
		ApplicantUserID: owner.ID,
		Reason:          pgtype.Text{String: "申请加入集团", Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.ApproveGroupJoinRequestTx(context.Background(), ApproveGroupJoinRequestTxParams{
		RequestID:      joinReq.ID,
		GroupID:        targetGroup.ID,
		ReviewerUserID: reviewer.ID,
		BrandID:        pgtype.Int8{Valid: false},
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrMerchantAlreadyJoinedGroup))

	affiliation, err := testStore.GetMerchantGroupAffiliation(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.True(t, affiliation.GroupID.Valid)
	require.Equal(t, existingGroup.ID, affiliation.GroupID.Int64)

	currentReq, err := testStore.GetGroupJoinRequest(context.Background(), joinReq.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", currentReq.Status)
}

func createGroupForJoinTxTest(t *testing.T, ownerUserID int64) MerchantGroup {
	t.Helper()

	region := createRandomRegion(t)
	group, err := testStore.CreateMerchantGroup(context.Background(), CreateMerchantGroupParams{
		Name:                "group_" + util.RandomString(8),
		OwnerUserID:         ownerUserID,
		ContactPhone:        pgtype.Text{String: "13800138000", Valid: true},
		LicenseNumber:       pgtype.Text{String: "LIC-" + util.RandomString(8), Valid: true},
		LicenseMediaAssetID: pgtype.Int8{Valid: false},
		Address:             pgtype.Text{String: "测试地址", Valid: true},
		RegionID:            pgtype.Int8{Int64: region.ID, Valid: true},
		ApplicationData:     []byte(`{"source":"test"}`),
	})
	require.NoError(t, err)
	return group
}
