package api

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNotifyCredentialGovernanceRestored_Merchant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	server := newTestServerWithTaskDistributor(t, store, distributor)

	merchant := db.Merchant{ID: 88, OwnerUserID: 66}
	reviewRun := &db.OnboardingReviewRun{ID: 501}
	entries := []logic.CredentialActivationInput{
		{DocumentType: db.CredentialDocumentTypeBusinessLicense},
		{DocumentType: db.CredentialDocumentTypeFoodPermit},
	}

	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
			require.Equal(t, merchant.OwnerUserID, payload.UserID)
			require.Equal(t, "merchant", payload.RelatedType)
			require.Equal(t, merchant.ID, payload.RelatedID)
			require.Contains(t, payload.Title, "营业执照")
			require.Contains(t, payload.Title, "食品经营许可证")
			require.Equal(t, "credential_governance_restore", payload.ExtraData["notification_source"])
			require.Equal(t, "restored", payload.ExtraData["outcome"])
			require.EqualValues(t, merchant.ID, payload.ExtraData["merchant_id"])
			require.EqualValues(t, reviewRun.ID, payload.ExtraData["review_run_id"])
			require.EqualValues(t, 3001, payload.ExtraData["application_id"])
			documentTypes, ok := payload.ExtraData["document_types"].([]string)
			require.True(t, ok)
			require.ElementsMatch(t, []string{db.CredentialDocumentTypeBusinessLicense, db.CredentialDocumentTypeFoodPermit}, documentTypes)
			return nil
		},
	)

	server.notifyCredentialGovernanceRestored(context.Background(), "merchant", merchant.ID, 3001, reviewRun, entries)
}

func TestNotifyCredentialGovernanceRestored_Rider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	server := newTestServerWithTaskDistributor(t, store, distributor)

	rider := db.Rider{ID: 77, UserID: 55}
	entries := []logic.CredentialActivationInput{{DocumentType: db.CredentialDocumentTypeHealthCert}}

	store.EXPECT().GetRider(gomock.Any(), rider.ID).Return(rider, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
			require.Equal(t, rider.UserID, payload.UserID)
			require.Equal(t, "rider", payload.RelatedType)
			require.Equal(t, rider.ID, payload.RelatedID)
			require.Contains(t, payload.Title, "健康证")
			require.Contains(t, payload.Content, "接单资格")
			require.EqualValues(t, rider.ID, payload.ExtraData["rider_id"])
			return nil
		},
	)

	server.notifyCredentialGovernanceRestored(context.Background(), "rider", rider.ID, 4001, nil, entries)
}

func TestMerchantBlockedReviewOutcome(t *testing.T) {
	require.Equal(t, "needs_resubmit", merchantBlockedReviewOutcome("readiness_required_field_missing"))
	require.Equal(t, "needs_manual", merchantBlockedReviewOutcome("manual_address_ambiguous"))
	require.Equal(t, "rejected", merchantBlockedReviewOutcome("rule_document_expired"))
	require.Equal(t, "rejected", merchantBlockedReviewOutcome("risk_duplicate_location"))
}

func TestAttachReviewSummary(t *testing.T) {
	run := &db.OnboardingReviewRun{
		ID:            701,
		Stage:         "review",
		Outcome:       pgtype.Text{String: "approved", Valid: true},
		ReasonCode:    pgtype.Text{String: "auto_approved", Valid: true},
		ReasonMessage: pgtype.Text{},
		RuleHits:      []string{"merchant.auto_approve"},
		OcrJobRefs:    []int64{101, 202},
		CreatedAt:     time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC),
	}

	merchantApp := db.MerchantApplication{ID: 1}
	attachMerchantReviewSummary(&merchantApp, run)
	merchantSummary := decodeOnboardingReviewSummary(merchantApp.ReviewSummary)
	require.NotNil(t, merchantSummary)
	require.Equal(t, int64(701), merchantSummary.RunID)
	require.Equal(t, "approved", merchantSummary.Outcome)

	riderApp := db.RiderApplication{ID: 2}
	attachRiderReviewSummary(&riderApp, run)
	riderSummary := decodeOnboardingReviewSummary(riderApp.ReviewSummary)
	require.NotNil(t, riderSummary)
	require.Equal(t, int64(701), riderSummary.RunID)
	require.Equal(t, "auto_approved", riderSummary.ReasonCode)
}
