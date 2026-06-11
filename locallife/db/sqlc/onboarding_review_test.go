package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestCancelOnboardingReviewRunRecordsNeedsResubmitOutcome(t *testing.T) {
	user := createRandomUser(t)
	app := prepareSubmittedMerchantApplication(t, user.ID)

	run, err := testStore.CreateMerchantOnboardingReviewRun(context.Background(), CreateMerchantOnboardingReviewRunParams{
		MerchantApplicationID: pgtype.Int8{Int64: app.ID, Valid: true},
		RunStatus:             OnboardingReviewRunStatusQueued,
		Stage:                 "review",
		Evidence:              []byte(`{}`),
		RuleHits:              []string{},
		OcrJobRefs:            []int64{},
		Snapshot:              []byte(`{"status":"submitted"}`),
		RequestedBy:           pgtype.Int8{Int64: user.ID, Valid: true},
	})
	require.NoError(t, err)

	cancelled, err := testStore.CancelOnboardingReviewRun(context.Background(), CancelOnboardingReviewRunParams{
		ReasonCode:    pgtype.Text{String: OnboardingReviewReasonSupersededByEdit, Valid: true},
		ReasonMessage: pgtype.Text{String: OnboardingReviewReasonMessageSupersededByEdit, Valid: true},
		ID:            run.ID,
	})
	require.NoError(t, err)
	require.Equal(t, OnboardingReviewRunStatusCancelled, cancelled.RunStatus)
	require.Equal(t, OnboardingReviewOutcomeNeedsResubmit, cancelled.Outcome.String)
	require.Equal(t, OnboardingReviewReasonSupersededByEdit, cancelled.ReasonCode.String)
	require.Contains(t, cancelled.ReasonMessage.String, "重新编辑")
}
