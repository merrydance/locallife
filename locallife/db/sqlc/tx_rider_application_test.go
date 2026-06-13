package db

import (
	"context"
	"encoding/json"
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

func TestApproveRiderApplicationWithReviewTx_RollsBackWhenCredentialActivationFails(t *testing.T) {
	user := createRandomUser(t)
	app := prepareSubmittedRiderApplication(t, user.ID)
	asset := createRandomMediaAsset(t, user.ID)
	reviewRun := createQueuedRiderOnboardingReviewRunForTest(t, app.ID, user.ID)

	_, err := testStore.ApproveRiderApplicationWithReviewTx(context.Background(), ApproveRiderApplicationWithReviewTxParams{
		Approval: ApproveRiderApplicationTxParams{
			ApplicationID: app.ID,
			ReviewedBy:    pgtype.Int8{Int64: user.ID, Valid: true},
			RiderRealName: "张三",
			RiderIDCardNo: util.RandomString(18),
			RiderPhone:    "13812345678",
		},
		ReviewRunID: reviewRun.ID,
		ReviewCompletion: CompleteOnboardingReviewRunParams{
			ID:         reviewRun.ID,
			Stage:      "review",
			Outcome:    pgtype.Text{String: OnboardingReviewOutcomeApproved, Valid: true},
			ReasonCode: pgtype.Text{String: "auto_approved", Valid: true},
			Evidence:   []byte(`{}`),
			RuleHits:   []string{"rider.auto_approve"},
			OcrJobRefs: []int64{},
			Snapshot:   []byte(`{"status":"submitted"}`),
			ReviewedBy: pgtype.Int8{Int64: user.ID, Valid: true},
		},
		CredentialActivatedAt: pgtype.Timestamptz{Time: reviewRun.CreatedAt, Valid: true},
		CredentialEntries: []ActivateRiderCredentialLedgerEntry{{
			DocumentType:       "not_a_valid_document_type",
			RiderApplicationID: app.ID,
			ReviewRunID:        pgtype.Int8{Int64: reviewRun.ID, Valid: true},
			MediaAssetID:       asset.ID,
			NormalizedPayload:  []byte(`{"name":"张三"}`),
		}},
	})
	require.Error(t, err)

	persistedApp, getErr := testStore.GetRiderApplication(context.Background(), app.ID)
	require.NoError(t, getErr)
	require.Equal(t, RiderApplicationStatusSubmitted, persistedApp.Status)
	require.Nil(t, persistedApp.ReviewSummary)

	persistedRun, getErr := testStore.GetOnboardingReviewRun(context.Background(), reviewRun.ID)
	require.NoError(t, getErr)
	require.Equal(t, OnboardingReviewRunStatusQueued, persistedRun.RunStatus)
	require.False(t, persistedRun.Outcome.Valid)

	_, getErr = testStore.GetRiderByUserID(context.Background(), user.ID)
	require.ErrorIs(t, getErr, ErrRecordNotFound)

	_, getErr = testStore.GetUserRoleByType(context.Background(), GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   UserRoleRider,
	})
	require.ErrorIs(t, getErr, ErrRecordNotFound)
}

func createQueuedRiderOnboardingReviewRunForTest(t *testing.T, applicationID int64, requestedBy int64) OnboardingReviewRun {
	t.Helper()

	run, err := testStore.CreateRiderOnboardingReviewRun(context.Background(), CreateRiderOnboardingReviewRunParams{
		RiderApplicationID: pgtype.Int8{Int64: applicationID, Valid: true},
		RunStatus:          OnboardingReviewRunStatusQueued,
		Stage:              "review",
		Evidence:           []byte(`{}`),
		RuleHits:           []string{},
		OcrJobRefs:         []int64{},
		Snapshot:           []byte(`{"status":"submitted"}`),
		RequestedBy:        pgtype.Int8{Int64: requestedBy, Valid: true},
	})
	require.NoError(t, err)
	return run
}

func riderReviewSummaryJSONForTest(t *testing.T, runID int64) []byte {
	t.Helper()

	summary := onboardingReviewRunSummary{
		RunID:      runID,
		Stage:      "review",
		Outcome:    OnboardingReviewOutcomeApproved,
		ReasonCode: "auto_approved",
		RuleHits:   []string{"rider.auto_approve"},
		CreatedAt:  "2026-04-22T10:00:00Z",
	}
	encoded, err := json.Marshal(summary)
	require.NoError(t, err)
	return encoded
}
