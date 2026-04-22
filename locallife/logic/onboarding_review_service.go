package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type onboardingReviewStore interface {
	CreateMerchantOnboardingReviewRun(ctx context.Context, arg db.CreateMerchantOnboardingReviewRunParams) (db.OnboardingReviewRun, error)
	CreateRiderOnboardingReviewRun(ctx context.Context, arg db.CreateRiderOnboardingReviewRunParams) (db.OnboardingReviewRun, error)
	CancelOnboardingReviewRun(ctx context.Context, arg db.CancelOnboardingReviewRunParams) (db.OnboardingReviewRun, error)
	MarkOnboardingReviewRunProcessing(ctx context.Context, id int64) (db.OnboardingReviewRun, error)
	CompleteOnboardingReviewRun(ctx context.Context, arg db.CompleteOnboardingReviewRunParams) (db.OnboardingReviewRun, error)
	UpdateMerchantApplicationReviewSummary(ctx context.Context, arg db.UpdateMerchantApplicationReviewSummaryParams) (db.MerchantApplication, error)
	UpdateRiderApplicationReviewSummary(ctx context.Context, arg db.UpdateRiderApplicationReviewSummaryParams) (db.RiderApplication, error)
}

type OnboardingReviewService struct {
	store onboardingReviewStore
	now   func() time.Time
}

type OnboardingReviewDecision struct {
	Stage         string
	Outcome       string
	ReasonCode    string
	ReasonMessage string
	Evidence      map[string]any
	RuleHits      []string
	OCRJobRefs    []int64
	Snapshot      map[string]any
	RequestedBy   *int64
	ReviewedBy    *int64
}

type onboardingReviewSummary struct {
	RunID         int64    `json:"run_id"`
	Stage         string   `json:"stage"`
	Outcome       string   `json:"outcome"`
	ReasonCode    string   `json:"reason_code"`
	ReasonMessage string   `json:"reason_message,omitempty"`
	RuleHits      []string `json:"rule_hits,omitempty"`
	OCRJobRefs    []int64  `json:"ocr_job_refs,omitempty"`
	CreatedAt     string   `json:"created_at"`
}

func NewOnboardingReviewService(store onboardingReviewStore) *OnboardingReviewService {
	if store == nil {
		return nil
	}
	return &OnboardingReviewService{store: store, now: time.Now}
}

func (service *OnboardingReviewService) RecordMerchantReview(ctx context.Context, applicationID int64, decision OnboardingReviewDecision) (db.OnboardingReviewRun, error) {
	if service == nil || service.store == nil {
		return db.OnboardingReviewRun{}, nil
	}
	return service.recordMerchantReview(ctx, applicationID, decision)
}

func (service *OnboardingReviewService) RecordRiderReview(ctx context.Context, applicationID int64, decision OnboardingReviewDecision) (db.OnboardingReviewRun, error) {
	if service == nil || service.store == nil {
		return db.OnboardingReviewRun{}, nil
	}
	return service.recordRiderReview(ctx, applicationID, decision)
}

func (service *OnboardingReviewService) CreateRiderReviewRun(ctx context.Context, applicationID int64, decision OnboardingReviewDecision) (db.OnboardingReviewRun, error) {
	if service == nil || service.store == nil {
		return db.OnboardingReviewRun{}, nil
	}
	return service.createRiderReviewRun(ctx, applicationID, decision)
}

func (service *OnboardingReviewService) CreateMerchantReviewRun(ctx context.Context, applicationID int64, decision OnboardingReviewDecision) (db.OnboardingReviewRun, error) {
	if service == nil || service.store == nil {
		return db.OnboardingReviewRun{}, nil
	}
	return service.createMerchantReviewRun(ctx, applicationID, decision)
}

func (service *OnboardingReviewService) CompleteRiderReviewRun(ctx context.Context, runID int64, applicationID int64, decision OnboardingReviewDecision) (db.OnboardingReviewRun, error) {
	if service == nil || service.store == nil {
		return db.OnboardingReviewRun{}, nil
	}
	return service.completeRiderReviewRun(ctx, runID, applicationID, decision)
}

func (service *OnboardingReviewService) CompleteMerchantReviewRun(ctx context.Context, runID int64, applicationID int64, decision OnboardingReviewDecision) (db.OnboardingReviewRun, error) {
	if service == nil || service.store == nil {
		return db.OnboardingReviewRun{}, nil
	}
	return service.completeMerchantReviewRun(ctx, runID, applicationID, decision)
}

func (service *OnboardingReviewService) CancelReviewRun(ctx context.Context, runID int64, reasonCode string, reasonMessage string) (db.OnboardingReviewRun, error) {
	if service == nil || service.store == nil {
		return db.OnboardingReviewRun{}, nil
	}
	cancelledRun, err := service.store.CancelOnboardingReviewRun(ctx, db.CancelOnboardingReviewRunParams{
		ReasonCode:    pgtype.Text{String: reasonCode, Valid: reasonCode != ""},
		ReasonMessage: pgtype.Text{String: reasonMessage, Valid: reasonMessage != ""},
		ID:            runID,
	})
	if err != nil {
		return db.OnboardingReviewRun{}, fmt.Errorf("cancel onboarding review run: %w", err)
	}
	return cancelledRun, nil
}

func (service *OnboardingReviewService) recordMerchantReview(ctx context.Context, applicationID int64, decision OnboardingReviewDecision) (db.OnboardingReviewRun, error) {
	createdRun, err := service.createMerchantReviewRun(ctx, applicationID, decision)
	if err != nil {
		return db.OnboardingReviewRun{}, err
	}

	return service.completeMerchantReviewRun(ctx, createdRun.ID, applicationID, decision)
}

func (service *OnboardingReviewService) createMerchantReviewRun(ctx context.Context, applicationID int64, decision OnboardingReviewDecision) (db.OnboardingReviewRun, error) {
	evidenceJSON, snapshotJSON, err := marshalOnboardingReviewPayloads(decision)
	if err != nil {
		return db.OnboardingReviewRun{}, err
	}

	createdRun, err := service.store.CreateMerchantOnboardingReviewRun(ctx, db.CreateMerchantOnboardingReviewRunParams{
		MerchantApplicationID: pgtype.Int8{Int64: applicationID, Valid: true},
		RunStatus:             "queued",
		Stage:                 normalizeReviewStage(decision.Stage),
		Evidence:              evidenceJSON,
		RuleHits:              dedupeStrings(decision.RuleHits),
		OcrJobRefs:            dedupeInt64s(decision.OCRJobRefs),
		Snapshot:              snapshotJSON,
		RequestedBy:           optionalInt8(decision.RequestedBy),
	})
	if err != nil {
		return db.OnboardingReviewRun{}, fmt.Errorf("create merchant onboarding review run: %w", err)
	}

	if err := service.updateMerchantApplicationReviewSummary(ctx, applicationID, createdRun); err != nil {
		return db.OnboardingReviewRun{}, err
	}

	return createdRun, nil
}

func (service *OnboardingReviewService) completeMerchantReviewRun(ctx context.Context, runID int64, applicationID int64, decision OnboardingReviewDecision) (db.OnboardingReviewRun, error) {
	evidenceJSON, snapshotJSON, err := marshalOnboardingReviewPayloads(decision)
	if err != nil {
		return db.OnboardingReviewRun{}, err
	}

	if _, err := service.store.MarkOnboardingReviewRunProcessing(ctx, runID); err != nil {
		return db.OnboardingReviewRun{}, fmt.Errorf("mark merchant onboarding review run processing: %w", err)
	}

	completedRun, err := service.store.CompleteOnboardingReviewRun(ctx, db.CompleteOnboardingReviewRunParams{
		Stage:         normalizeReviewStage(decision.Stage),
		Outcome:       pgtype.Text{String: decision.Outcome, Valid: decision.Outcome != ""},
		ReasonCode:    pgtype.Text{String: decision.ReasonCode, Valid: decision.ReasonCode != ""},
		ReasonMessage: pgtype.Text{String: decision.ReasonMessage, Valid: decision.ReasonMessage != ""},
		Evidence:      evidenceJSON,
		RuleHits:      dedupeStrings(decision.RuleHits),
		OcrJobRefs:    dedupeInt64s(decision.OCRJobRefs),
		Snapshot:      snapshotJSON,
		ReviewedBy:    optionalInt8(decision.ReviewedBy),
		ID:            runID,
	})
	if err != nil {
		return db.OnboardingReviewRun{}, fmt.Errorf("complete merchant onboarding review run: %w", err)
	}

	if err := service.updateMerchantApplicationReviewSummary(ctx, applicationID, completedRun); err != nil {
		return db.OnboardingReviewRun{}, err
	}

	return completedRun, nil
}

func (service *OnboardingReviewService) recordRiderReview(ctx context.Context, applicationID int64, decision OnboardingReviewDecision) (db.OnboardingReviewRun, error) {
	createdRun, err := service.createRiderReviewRun(ctx, applicationID, decision)
	if err != nil {
		return db.OnboardingReviewRun{}, err
	}

	return service.completeRiderReviewRun(ctx, createdRun.ID, applicationID, decision)
}

func (service *OnboardingReviewService) createRiderReviewRun(ctx context.Context, applicationID int64, decision OnboardingReviewDecision) (db.OnboardingReviewRun, error) {
	evidenceJSON, snapshotJSON, err := marshalOnboardingReviewPayloads(decision)
	if err != nil {
		return db.OnboardingReviewRun{}, err
	}

	createdRun, err := service.store.CreateRiderOnboardingReviewRun(ctx, db.CreateRiderOnboardingReviewRunParams{
		RiderApplicationID: pgtype.Int8{Int64: applicationID, Valid: true},
		RunStatus:          "queued",
		Stage:              normalizeReviewStage(decision.Stage),
		Evidence:           evidenceJSON,
		RuleHits:           dedupeStrings(decision.RuleHits),
		OcrJobRefs:         dedupeInt64s(decision.OCRJobRefs),
		Snapshot:           snapshotJSON,
		RequestedBy:        optionalInt8(decision.RequestedBy),
	})
	if err != nil {
		return db.OnboardingReviewRun{}, fmt.Errorf("create rider onboarding review run: %w", err)
	}

	if err := service.updateRiderApplicationReviewSummary(ctx, applicationID, createdRun); err != nil {
		return db.OnboardingReviewRun{}, err
	}

	return createdRun, nil
}

func (service *OnboardingReviewService) completeRiderReviewRun(ctx context.Context, runID int64, applicationID int64, decision OnboardingReviewDecision) (db.OnboardingReviewRun, error) {
	evidenceJSON, snapshotJSON, err := marshalOnboardingReviewPayloads(decision)
	if err != nil {
		return db.OnboardingReviewRun{}, err
	}

	if _, err := service.store.MarkOnboardingReviewRunProcessing(ctx, runID); err != nil {
		return db.OnboardingReviewRun{}, fmt.Errorf("mark rider onboarding review run processing: %w", err)
	}

	completedRun, err := service.store.CompleteOnboardingReviewRun(ctx, db.CompleteOnboardingReviewRunParams{
		Stage:         normalizeReviewStage(decision.Stage),
		Outcome:       pgtype.Text{String: decision.Outcome, Valid: decision.Outcome != ""},
		ReasonCode:    pgtype.Text{String: decision.ReasonCode, Valid: decision.ReasonCode != ""},
		ReasonMessage: pgtype.Text{String: decision.ReasonMessage, Valid: decision.ReasonMessage != ""},
		Evidence:      evidenceJSON,
		RuleHits:      dedupeStrings(decision.RuleHits),
		OcrJobRefs:    dedupeInt64s(decision.OCRJobRefs),
		Snapshot:      snapshotJSON,
		ReviewedBy:    optionalInt8(decision.ReviewedBy),
		ID:            runID,
	})
	if err != nil {
		return db.OnboardingReviewRun{}, fmt.Errorf("complete rider onboarding review run: %w", err)
	}

	if err := service.updateRiderApplicationReviewSummary(ctx, applicationID, completedRun); err != nil {
		return db.OnboardingReviewRun{}, err
	}

	return completedRun, nil
}

func (service *OnboardingReviewService) updateRiderApplicationReviewSummary(ctx context.Context, applicationID int64, run db.OnboardingReviewRun) error {
	summaryJSON, err := buildOnboardingReviewSummaryJSON(run)
	if err != nil {
		return err
	}

	if _, err := service.store.UpdateRiderApplicationReviewSummary(ctx, db.UpdateRiderApplicationReviewSummaryParams{
		ID:            applicationID,
		ReviewSummary: summaryJSON,
	}); err != nil {
		return fmt.Errorf("update rider application review summary: %w", err)
	}

	return nil
}

func (service *OnboardingReviewService) updateMerchantApplicationReviewSummary(ctx context.Context, applicationID int64, run db.OnboardingReviewRun) error {
	summaryJSON, err := buildOnboardingReviewSummaryJSON(run)
	if err != nil {
		return err
	}

	if _, err := service.store.UpdateMerchantApplicationReviewSummary(ctx, db.UpdateMerchantApplicationReviewSummaryParams{
		ID:            applicationID,
		ReviewSummary: summaryJSON,
	}); err != nil {
		return fmt.Errorf("update merchant application review summary: %w", err)
	}

	return nil
}

func marshalOnboardingReviewPayloads(decision OnboardingReviewDecision) ([]byte, []byte, error) {
	evidenceJSON, err := marshalOnboardingReviewMap(decision.Evidence)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal onboarding review evidence: %w", err)
	}
	snapshotJSON, err := marshalOnboardingReviewMap(decision.Snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal onboarding review snapshot: %w", err)
	}
	return evidenceJSON, snapshotJSON, nil
}

func marshalOnboardingReviewMap(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(value)
}

func buildOnboardingReviewSummaryJSON(run db.OnboardingReviewRun) ([]byte, error) {
	summary := onboardingReviewSummary{
		RunID:         run.ID,
		Stage:         run.Stage,
		Outcome:       run.Outcome.String,
		ReasonCode:    run.ReasonCode.String,
		ReasonMessage: run.ReasonMessage.String,
		RuleHits:      dedupeStrings(run.RuleHits),
		OCRJobRefs:    dedupeInt64s(run.OcrJobRefs),
		CreatedAt:     run.CreatedAt.UTC().Format(time.RFC3339),
	}
	encoded, err := json.Marshal(summary)
	if err != nil {
		return nil, fmt.Errorf("marshal onboarding review summary: %w", err)
	}
	return encoded, nil
}

func optionalInt8(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

func normalizeReviewStage(stage string) string {
	if stage == "" {
		return "review"
	}
	return stage
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func dedupeInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result
}
