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

func TestCreateGroupJoinRequestTxWritesAuditAndRequestTogether(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	group := createGroupForJoinTxTest(t, owner.ID)

	result, err := testStore.CreateGroupJoinRequestTx(context.Background(), CreateGroupJoinRequestTxParams{
		GroupID:         group.ID,
		MerchantID:      merchant.ID,
		ApplicantUserID: owner.ID,
		Reason:          pgtype.Text{String: "申请加入集团", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, group.ID, result.Request.GroupID)
	require.Equal(t, merchant.ID, result.Request.MerchantID)
	require.Equal(t, "pending", result.Request.Status)

	logs, err := testStore.ListGroupAuditLogsByGroup(context.Background(), pgtype.Int8{Int64: group.ID, Valid: true})
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	require.Equal(t, "group_join_request_created", logs[0].Action)
	require.Equal(t, merchant.ID, logs[0].TargetID.Int64)
}

func TestCreateGroupJoinRequestTxRejectsJoinedMerchantInsideTransaction(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	existingGroup := createGroupForJoinTxTest(t, owner.ID)
	targetGroup := createGroupForJoinTxTest(t, owner.ID)

	err := testStore.UpdateMerchantGroupAffiliation(context.Background(), UpdateMerchantGroupAffiliationParams{
		ID:      merchant.ID,
		GroupID: pgtype.Int8{Int64: existingGroup.ID, Valid: true},
		BrandID: pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)

	result, err := testStore.CreateGroupJoinRequestTx(context.Background(), CreateGroupJoinRequestTxParams{
		GroupID:         targetGroup.ID,
		MerchantID:      merchant.ID,
		ApplicantUserID: owner.ID,
		Reason:          pgtype.Text{String: "申请加入集团", Valid: true},
	})
	require.ErrorIs(t, err, ErrMerchantAlreadyJoinedGroup)
	require.Empty(t, result.Request.Status)

	rows, err := testStore.ListGroupJoinRequestsByGroup(context.Background(), targetGroup.ID)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestCreateGroupJoinRequestTxRejectsDuplicatePendingRequest(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	group := createGroupForJoinTxTest(t, owner.ID)

	first, err := testStore.CreateGroupJoinRequestTx(context.Background(), CreateGroupJoinRequestTxParams{
		GroupID:         group.ID,
		MerchantID:      merchant.ID,
		ApplicantUserID: owner.ID,
		Reason:          pgtype.Text{String: "申请加入集团", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "pending", first.Request.Status)

	second, err := testStore.CreateGroupJoinRequestTx(context.Background(), CreateGroupJoinRequestTxParams{
		GroupID:         group.ID,
		MerchantID:      merchant.ID,
		ApplicantUserID: owner.ID,
		Reason:          pgtype.Text{String: "重复申请加入集团", Valid: true},
	})
	require.Error(t, err)
	require.Equal(t, UniqueViolation, ErrorCode(err))
	require.Empty(t, second.Request.Status)

	var pendingCount int64
	err = testStore.(*SQLStore).connPool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_group_join_requests
		WHERE group_id = $1
		  AND merchant_id = $2
		  AND status = 'pending'
	`, group.ID, merchant.ID).Scan(&pendingCount)
	require.NoError(t, err)
	require.Equal(t, int64(1), pendingCount)
}

func TestListGroupJoinRequestsByMerchantReturnsApplicantHistory(t *testing.T) {
	owner := createRandomUser(t)
	otherOwner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	otherMerchant := createRandomMerchantWithOwner(t, otherOwner.ID)
	group := createGroupForJoinTxTest(t, owner.ID)
	otherGroup := createGroupForJoinTxTest(t, otherOwner.ID)

	firstReq := createGroupJoinRequestForTxTest(t, group.ID, merchant.ID, owner.ID)
	firstRejected, err := testStore.RejectGroupJoinRequestTx(context.Background(), RejectGroupJoinRequestTxParams{
		RequestID:      firstReq.ID,
		GroupID:        group.ID,
		ReviewerUserID: otherOwner.ID,
		Reason:         pgtype.Text{String: "资料不完整", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "rejected", firstRejected.Request.Status)

	secondReq := createGroupJoinRequestForTxTest(t, group.ID, merchant.ID, owner.ID)
	otherReq := createGroupJoinRequestForTxTest(t, otherGroup.ID, otherMerchant.ID, otherOwner.ID)

	rows, err := testStore.ListGroupJoinRequestsByMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, secondReq.ID, rows[0].ID)
	require.Equal(t, firstReq.ID, rows[1].ID)
	require.Equal(t, group.Name, rows[0].GroupName)

	for _, row := range rows {
		require.Equal(t, merchant.ID, row.MerchantID)
		require.NotEqual(t, otherReq.ID, row.ID)
	}
}

func TestRejectGroupJoinRequestTxWritesAuditAndRejectsOnlyPending(t *testing.T) {
	reviewer := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	group := createGroupForJoinTxTest(t, reviewer.ID)
	joinReq := createGroupJoinRequestForTxTest(t, group.ID, merchant.ID, owner.ID)

	result, err := testStore.RejectGroupJoinRequestTx(context.Background(), RejectGroupJoinRequestTxParams{
		RequestID:      joinReq.ID,
		GroupID:        group.ID,
		ReviewerUserID: reviewer.ID,
		Reason:         pgtype.Text{String: "资料不完整", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "rejected", result.Request.Status)

	logs, err := testStore.ListGroupAuditLogsByGroup(context.Background(), pgtype.Int8{Int64: group.ID, Valid: true})
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	require.Equal(t, "group_join_request_rejected", logs[0].Action)
	require.Equal(t, merchant.ID, logs[0].TargetID.Int64)

	second, err := testStore.RejectGroupJoinRequestTx(context.Background(), RejectGroupJoinRequestTxParams{
		RequestID:      joinReq.ID,
		GroupID:        group.ID,
		ReviewerUserID: reviewer.ID,
	})
	require.ErrorIs(t, err, ErrGroupJoinRequestReviewConflict)
	require.Empty(t, second.Request.Status)
}

func TestRejectGroupJoinRequestTxAllowsRepeatedRejectedHistory(t *testing.T) {
	reviewer := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	group := createGroupForJoinTxTest(t, reviewer.ID)

	firstReq := createGroupJoinRequestForTxTest(t, group.ID, merchant.ID, owner.ID)
	first, err := testStore.RejectGroupJoinRequestTx(context.Background(), RejectGroupJoinRequestTxParams{
		RequestID:      firstReq.ID,
		GroupID:        group.ID,
		ReviewerUserID: reviewer.ID,
		Reason:         pgtype.Text{String: "资料不完整", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "rejected", first.Request.Status)

	secondReq := createGroupJoinRequestForTxTest(t, group.ID, merchant.ID, owner.ID)
	second, err := testStore.RejectGroupJoinRequestTx(context.Background(), RejectGroupJoinRequestTxParams{
		RequestID:      secondReq.ID,
		GroupID:        group.ID,
		ReviewerUserID: reviewer.ID,
		Reason:         pgtype.Text{String: "仍需补充资料", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "rejected", second.Request.Status)
	require.NotEqual(t, first.Request.ID, second.Request.ID)

	var rejectedCount int64
	err = testStore.(*SQLStore).connPool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_group_join_requests
		WHERE group_id = $1
		  AND merchant_id = $2
		  AND status = 'rejected'
	`, group.ID, merchant.ID).Scan(&rejectedCount)
	require.NoError(t, err)
	require.Equal(t, int64(2), rejectedCount)
}

func TestCancelGroupJoinRequestTxDoesNotOverwriteApprovedRequest(t *testing.T) {
	reviewer := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	group := createGroupForJoinTxTest(t, reviewer.ID)
	joinReq := createGroupJoinRequestForTxTest(t, group.ID, merchant.ID, owner.ID)

	approved, err := testStore.ApproveGroupJoinRequestTx(context.Background(), ApproveGroupJoinRequestTxParams{
		RequestID:      joinReq.ID,
		GroupID:        group.ID,
		ReviewerUserID: reviewer.ID,
		BrandID:        pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)
	require.Equal(t, "approved", approved.Request.Status)

	cancelled, err := testStore.CancelGroupJoinRequestTx(context.Background(), CancelGroupJoinRequestTxParams{
		RequestID:       joinReq.ID,
		GroupID:         group.ID,
		ApplicantUserID: owner.ID,
	})
	require.ErrorIs(t, err, ErrGroupJoinRequestReviewConflict)
	require.Empty(t, cancelled.Request.Status)

	currentReq, err := testStore.GetGroupJoinRequest(context.Background(), joinReq.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", currentReq.Status)
}

func TestCancelGroupJoinRequestTxAllowsRepeatedCancelledHistory(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	group := createGroupForJoinTxTest(t, owner.ID)

	firstReq := createGroupJoinRequestForTxTest(t, group.ID, merchant.ID, owner.ID)
	first, err := testStore.CancelGroupJoinRequestTx(context.Background(), CancelGroupJoinRequestTxParams{
		RequestID:       firstReq.ID,
		GroupID:         group.ID,
		ApplicantUserID: owner.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "cancelled", first.Request.Status)

	secondReq := createGroupJoinRequestForTxTest(t, group.ID, merchant.ID, owner.ID)
	second, err := testStore.CancelGroupJoinRequestTx(context.Background(), CancelGroupJoinRequestTxParams{
		RequestID:       secondReq.ID,
		GroupID:         group.ID,
		ApplicantUserID: owner.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "cancelled", second.Request.Status)
	require.NotEqual(t, first.Request.ID, second.Request.ID)

	var cancelledCount int64
	err = testStore.(*SQLStore).connPool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_group_join_requests
		WHERE group_id = $1
		  AND merchant_id = $2
		  AND status = 'cancelled'
	`, group.ID, merchant.ID).Scan(&cancelledCount)
	require.NoError(t, err)
	require.Equal(t, int64(2), cancelledCount)
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

func createGroupJoinRequestForTxTest(t *testing.T, groupID, merchantID, applicantUserID int64) MerchantGroupJoinRequest {
	t.Helper()

	joinReq, err := testStore.CreateGroupJoinRequest(context.Background(), CreateGroupJoinRequestParams{
		GroupID:         groupID,
		MerchantID:      merchantID,
		ApplicantUserID: applicantUserID,
		Reason:          pgtype.Text{String: "申请加入集团", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "pending", joinReq.Status)
	return joinReq
}
