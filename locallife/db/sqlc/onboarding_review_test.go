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

func TestGetLatestActiveMerchantOnboardingReviewRunIgnoresTerminalRuns(t *testing.T) {
	user := createRandomUser(t)
	app := prepareSubmittedMerchantApplication(t, user.ID)

	activeRun, err := testStore.CreateMerchantOnboardingReviewRun(context.Background(), CreateMerchantOnboardingReviewRunParams{
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

	terminalRun, err := testStore.CreateMerchantOnboardingReviewRun(context.Background(), CreateMerchantOnboardingReviewRunParams{
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
	require.Greater(t, terminalRun.ID, activeRun.ID)

	terminalRun, err = testStore.CompleteOnboardingReviewRun(context.Background(), CompleteOnboardingReviewRunParams{
		ID:         terminalRun.ID,
		Stage:      "review",
		Outcome:    pgtype.Text{String: OnboardingReviewOutcomeApproved, Valid: true},
		ReasonCode: pgtype.Text{String: "auto_approved", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, OnboardingReviewRunStatusCompleted, terminalRun.RunStatus)

	run, err := testStore.GetLatestActiveMerchantOnboardingReviewRun(context.Background(), pgtype.Int8{Int64: app.ID, Valid: true})
	require.NoError(t, err)
	require.Equal(t, activeRun.ID, run.ID)
	require.Equal(t, OnboardingReviewRunStatusQueued, run.RunStatus)
}
