package logic

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestOnboardingReviewServiceRecordMerchantReview(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOnboardingReviewService(store)
	service.now = func() time.Time { return time.Date(2026, 4, 22, 9, 30, 0, 0, time.UTC) }
	requestedBy := int64(88)

	store.EXPECT().
		CreateMerchantOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateMerchantOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.Equal(t, int64(41), arg.MerchantApplicationID.Int64)
			require.Equal(t, "queued", arg.RunStatus)
			require.Equal(t, "review", arg.Stage)
			require.Equal(t, []int64{101, 202}, arg.OcrJobRefs)
			require.Equal(t, []string{"merchant.auto_approve"}, arg.RuleHits)
			return db.OnboardingReviewRun{ID: 501, ApplicationType: "merchant", RunStatus: "queued", Stage: "review", CreatedAt: service.now()}, nil
		})

	store.EXPECT().
		MarkOnboardingReviewRunProcessing(gomock.Any(), int64(501)).
		Return(db.OnboardingReviewRun{ID: 501, ApplicationType: "merchant", RunStatus: "processing", Stage: "review", CreatedAt: service.now()}, nil)

	store.EXPECT().
		CompleteOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CompleteOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.Equal(t, int64(501), arg.ID)
			require.Equal(t, "approved", arg.Outcome.String)
			require.Equal(t, "auto_approved", arg.ReasonCode.String)
			return db.OnboardingReviewRun{
				ID:         501,
				Stage:      "review",
				RunStatus:  "completed",
				Outcome:    pgtype.Text{String: "approved", Valid: true},
				ReasonCode: pgtype.Text{String: "auto_approved", Valid: true},
				RuleHits:   []string{"merchant.auto_approve"},
				OcrJobRefs: []int64{101, 202},
				CreatedAt:  service.now(),
				UpdatedAt:  service.now(),
			}, nil
		})

	store.EXPECT().
		UpdateMerchantApplicationReviewSummary(gomock.Any(), gomock.Any()).
		Times(2).
		DoAndReturn(func() func(context.Context, db.UpdateMerchantApplicationReviewSummaryParams) (db.MerchantApplication, error) {
			callCount := 0
			return func(_ context.Context, arg db.UpdateMerchantApplicationReviewSummaryParams) (db.MerchantApplication, error) {
				callCount++
				require.Equal(t, int64(41), arg.ID)
				var summary map[string]any
				require.NoError(t, json.Unmarshal(arg.ReviewSummary, &summary))
				switch callCount {
				case 1:
					require.Equal(t, float64(501), summary["run_id"])
					require.Equal(t, "", summary["outcome"])
				case 2:
					require.Equal(t, "approved", summary["outcome"])
					require.Equal(t, "auto_approved", summary["reason_code"])
				default:
					t.Fatalf("unexpected merchant review summary update call %d", callCount)
				}
				return db.MerchantApplication{ID: 41, ReviewSummary: arg.ReviewSummary}, nil
			}
		}())

	run, err := service.RecordMerchantReview(context.Background(), 41, OnboardingReviewDecision{
		Outcome:     "approved",
		ReasonCode:  "auto_approved",
		RuleHits:    []string{"merchant.auto_approve", "merchant.auto_approve"},
		OCRJobRefs:  []int64{202, 101, 202},
		RequestedBy: &requestedBy,
		Snapshot:    map[string]any{"application_id": int64(41)},
	})
	require.NoError(t, err)
	require.Equal(t, int64(501), run.ID)
}

func TestOnboardingReviewServiceUsesEmptyAuditArraysWhenNoHits(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOnboardingReviewService(store)
	service.now = func() time.Time { return time.Date(2026, 6, 1, 13, 5, 0, 0, time.UTC) }

	store.EXPECT().
		CreateMerchantOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateMerchantOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.NotNil(t, arg.RuleHits)
			require.Empty(t, arg.RuleHits)
			require.NotNil(t, arg.OcrJobRefs)
			require.Empty(t, arg.OcrJobRefs)
			return db.OnboardingReviewRun{
				ID:              502,
				ApplicationType: "merchant",
				RunStatus:       "queued",
				Stage:           "review",
				RuleHits:        arg.RuleHits,
				OcrJobRefs:      arg.OcrJobRefs,
				CreatedAt:       service.now(),
			}, nil
		})

	store.EXPECT().
		MarkOnboardingReviewRunProcessing(gomock.Any(), int64(502)).
		Return(db.OnboardingReviewRun{ID: 502, ApplicationType: "merchant", RunStatus: "processing", Stage: "review", CreatedAt: service.now()}, nil)

	store.EXPECT().
		CompleteOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CompleteOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.NotNil(t, arg.RuleHits)
			require.Empty(t, arg.RuleHits)
			require.NotNil(t, arg.OcrJobRefs)
			require.Empty(t, arg.OcrJobRefs)
			return db.OnboardingReviewRun{
				ID:         502,
				Stage:      "review",
				RunStatus:  "completed",
				Outcome:    pgtype.Text{String: "approved", Valid: true},
				ReasonCode: pgtype.Text{String: "auto_approved", Valid: true},
				RuleHits:   arg.RuleHits,
				OcrJobRefs: arg.OcrJobRefs,
				CreatedAt:  service.now(),
				UpdatedAt:  service.now(),
			}, nil
		})

	store.EXPECT().
		UpdateMerchantApplicationReviewSummary(gomock.Any(), gomock.Any()).
		Times(2).
		Return(db.MerchantApplication{ID: 42}, nil)

	_, err := service.RecordMerchantReview(context.Background(), 42, OnboardingReviewDecision{
		Outcome:    "approved",
		ReasonCode: "auto_approved",
		Snapshot:   map[string]any{"application_id": int64(42)},
	})
	require.NoError(t, err)
}

func TestOnboardingReviewServiceRecordRiderReview(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOnboardingReviewService(store)
	service.now = func() time.Time { return time.Date(2026, 4, 22, 9, 45, 0, 0, time.UTC) }

	store.EXPECT().
		CreateRiderOnboardingReviewRun(gomock.Any(), gomock.Any()).
		Return(db.OnboardingReviewRun{ID: 801, ApplicationType: "rider", RunStatus: "queued", Stage: "review", CreatedAt: service.now()}, nil)

	store.EXPECT().
		MarkOnboardingReviewRunProcessing(gomock.Any(), int64(801)).
		Return(db.OnboardingReviewRun{ID: 801, ApplicationType: "rider", RunStatus: "processing", Stage: "review", CreatedAt: service.now()}, nil)

	store.EXPECT().
		CompleteOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CompleteOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.Equal(t, "needs_resubmit", arg.Outcome.String)
			require.Equal(t, "rule_document_expired", arg.ReasonCode.String)
			return db.OnboardingReviewRun{
				ID:            801,
				Stage:         "review",
				RunStatus:     "completed",
				Outcome:       pgtype.Text{String: "needs_resubmit", Valid: true},
				ReasonCode:    pgtype.Text{String: "rule_document_expired", Valid: true},
				ReasonMessage: pgtype.Text{String: "身份证已过期，请更换有效身份证后重新申请", Valid: true},
				CreatedAt:     service.now(),
				UpdatedAt:     service.now(),
			}, nil
		})

	store.EXPECT().
		UpdateRiderApplicationReviewSummary(gomock.Any(), gomock.Any()).
		Times(2).
		DoAndReturn(func() func(context.Context, db.UpdateRiderApplicationReviewSummaryParams) (db.RiderApplication, error) {
			callCount := 0
			return func(_ context.Context, arg db.UpdateRiderApplicationReviewSummaryParams) (db.RiderApplication, error) {
				callCount++
				var summary map[string]any
				require.NoError(t, json.Unmarshal(arg.ReviewSummary, &summary))
				switch callCount {
				case 1:
					require.Equal(t, float64(801), summary["run_id"])
					require.Equal(t, "review", summary["stage"])
					require.Equal(t, "", summary["outcome"])
				case 2:
					require.Equal(t, "needs_resubmit", summary["outcome"])
					require.Equal(t, "rule_document_expired", summary["reason_code"])
				default:
					t.Fatalf("unexpected review summary update call %d", callCount)
				}
				return db.RiderApplication{ID: 55, ReviewSummary: arg.ReviewSummary}, nil
			}
		}())

	_, err := service.RecordRiderReview(context.Background(), 55, OnboardingReviewDecision{
		Outcome:       "needs_resubmit",
		ReasonCode:    "rule_document_expired",
		ReasonMessage: "身份证已过期，请更换有效身份证后重新申请",
		Snapshot:      map[string]any{"application_id": int64(55)},
	})
	require.NoError(t, err)
}

func TestBuildOnboardingReviewSummaryJSON_PreservesManualOutcomeContract(t *testing.T) {
	run := db.OnboardingReviewRun{
		ID:            991,
		Stage:         "manual",
		Outcome:       pgtype.Text{String: "needs_manual", Valid: true},
		ReasonCode:    pgtype.Text{String: "manual_address_ambiguous", Valid: true},
		ReasonMessage: pgtype.Text{String: "定位与证照地址存在歧义，需人工复核", Valid: true},
		RuleHits:      []string{"risk_duplicate_location", "risk_duplicate_location", "manual_address_ambiguous"},
		OcrJobRefs:    []int64{22, 11, 22},
		CreatedAt:     time.Date(2026, 4, 22, 14, 0, 0, 0, time.UTC),
	}

	summaryJSON, err := buildOnboardingReviewSummaryJSON(run)
	require.NoError(t, err)

	var summary map[string]any
	require.NoError(t, json.Unmarshal(summaryJSON, &summary))
	require.Equal(t, float64(991), summary["run_id"])
	require.Equal(t, "manual", summary["stage"])
	require.Equal(t, "needs_manual", summary["outcome"])
	require.Equal(t, "manual_address_ambiguous", summary["reason_code"])
	require.Equal(t, "定位与证照地址存在歧义，需人工复核", summary["reason_message"])
	require.Equal(t, []any{"manual_address_ambiguous", "risk_duplicate_location"}, summary["rule_hits"])
	require.Equal(t, []any{11.0, 22.0}, summary["ocr_job_refs"])
	require.Equal(t, "2026-04-22T14:00:00Z", summary["created_at"])
}
