package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// ApproveRiderApplicationTxParams contains input parameters for approving rider application
// and creating a rider record atomically.
type ApproveRiderApplicationTxParams struct {
	ApplicationID int64
	ReviewedBy    pgtype.Int8
	RiderRealName string
	RiderIDCardNo string
	RiderPhone    string
	RegionID      pgtype.Int8
}

// ApproveRiderApplicationTxResult contains the result of approval transaction
type ApproveRiderApplicationTxResult struct {
	Application RiderApplication
	Rider       Rider
}

// ApproveRiderApplicationWithReviewTxParams contains all durable writes required
// to approve a rider application from the onboarding review path.
type ApproveRiderApplicationWithReviewTxParams struct {
	Approval              ApproveRiderApplicationTxParams
	ReviewRunID           int64
	ReviewCreation        CreateRiderOnboardingReviewRunParams
	ReviewCompletion      CompleteOnboardingReviewRunParams
	CredentialActivatedAt pgtype.Timestamptz
	CredentialEntries     []ActivateRiderCredentialLedgerEntry
}

// ApproveRiderApplicationWithReviewTxResult contains the full durable approval result.
type ApproveRiderApplicationWithReviewTxResult struct {
	Application       RiderApplication
	Rider             Rider
	ReviewRun         OnboardingReviewRun
	CredentialLedgers []CredentialLedger
}

// ApproveRiderApplicationTx approves a rider application and creates the rider record
// in a single transaction to avoid leaving an approved application without a rider.
func (store *SQLStore) ApproveRiderApplicationTx(ctx context.Context, arg ApproveRiderApplicationTxParams) (ApproveRiderApplicationTxResult, error) {
	var result ApproveRiderApplicationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		approved, err := executeRiderApplicationApproval(ctx, q, arg)
		if err != nil {
			return err
		}
		result = approved
		return nil
	})

	return result, err
}

// ApproveRiderApplicationWithReviewTx completes the onboarding review run,
// approves the application, creates the rider role, and activates credential
// ledgers in one database transaction.
func (store *SQLStore) ApproveRiderApplicationWithReviewTx(ctx context.Context, arg ApproveRiderApplicationWithReviewTxParams) (ApproveRiderApplicationWithReviewTxResult, error) {
	var result ApproveRiderApplicationWithReviewTxResult

	if len(arg.CredentialEntries) == 0 {
		return result, fmt.Errorf("rider credential entries are required")
	}

	err := store.execTx(ctx, func(q *Queries) error {
		reviewRun, err := prepareRiderOnboardingReviewRunForApproval(ctx, q, arg)
		if err != nil {
			return err
		}
		result.ReviewRun = reviewRun

		approved, err := executeRiderApplicationApproval(ctx, q, arg.Approval)
		if err != nil {
			return err
		}
		result.Application = approved.Application
		result.Rider = approved.Rider

		reviewSummary, err := onboardingReviewRunSummaryJSON(result.ReviewRun)
		if err != nil {
			return fmt.Errorf("build rider onboarding review summary: %w", err)
		}
		result.Application, err = q.UpdateRiderApplicationReviewSummary(ctx, UpdateRiderApplicationReviewSummaryParams{
			ID:            result.Application.ID,
			ReviewSummary: reviewSummary,
		})
		if err != nil {
			return fmt.Errorf("update rider application review summary: %w", err)
		}

		result.CredentialLedgers, err = activateRiderCredentialLedgersForApproval(ctx, q, result.Rider.ID, result.Application.ID, result.ReviewRun.ID, arg)
		if err != nil {
			return err
		}

		return nil
	})

	return result, err
}

func executeRiderApplicationApproval(ctx context.Context, q *Queries, arg ApproveRiderApplicationTxParams) (ApproveRiderApplicationTxResult, error) {
	var result ApproveRiderApplicationTxResult
	var err error

	result.Application, err = q.ApproveRiderApplication(ctx, ApproveRiderApplicationParams{
		ID:         arg.ApplicationID,
		ReviewedBy: arg.ReviewedBy,
	})
	if err != nil {
		return result, fmt.Errorf("approve rider application: %w", err)
	}

	result.Rider, err = q.CreateRider(ctx, CreateRiderParams{
		UserID:   result.Application.UserID,
		RealName: arg.RiderRealName,
		IDCardNo: arg.RiderIDCardNo,
		Phone:    arg.RiderPhone,
		RegionID: arg.RegionID,
	})
	if err != nil {
		return result, fmt.Errorf("create rider: %w", err)
	}

	_, err = q.CreateUserRole(ctx, CreateUserRoleParams{
		UserID:          result.Application.UserID,
		Role:            UserRoleRider,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: result.Rider.ID, Valid: true},
	})
	if err != nil {
		return result, fmt.Errorf("create rider user role: %w", err)
	}

	return result, nil
}

func prepareRiderOnboardingReviewRunForApproval(ctx context.Context, q *Queries, arg ApproveRiderApplicationWithReviewTxParams) (OnboardingReviewRun, error) {
	var run OnboardingReviewRun
	var err error

	if arg.ReviewRunID > 0 {
		run, err = q.GetOnboardingReviewRun(ctx, arg.ReviewRunID)
		if err != nil {
			return OnboardingReviewRun{}, fmt.Errorf("get rider onboarding review run: %w", err)
		}
	} else {
		createArg := defaultRiderReviewCreationParams(arg.ReviewCreation, arg.Approval.ApplicationID)
		run, err = q.CreateRiderOnboardingReviewRun(ctx, createArg)
		if err != nil {
			return OnboardingReviewRun{}, fmt.Errorf("create rider onboarding review run: %w", err)
		}
	}

	if err := validateRiderReviewRunForApproval(run, arg.Approval.ApplicationID); err != nil {
		return OnboardingReviewRun{}, err
	}

	switch run.RunStatus {
	case OnboardingReviewRunStatusQueued:
		run, err = q.MarkOnboardingReviewRunProcessing(ctx, run.ID)
		if err != nil {
			return OnboardingReviewRun{}, fmt.Errorf("mark rider onboarding review run processing: %w", err)
		}
	case OnboardingReviewRunStatusProcessing:
	default:
		return OnboardingReviewRun{}, fmt.Errorf("rider onboarding review run %d status %s cannot approve application", run.ID, run.RunStatus)
	}

	completeArg := defaultRiderReviewCompletionParams(arg.ReviewCompletion, run.ID)
	run, err = q.CompleteOnboardingReviewRun(ctx, completeArg)
	if err != nil {
		return OnboardingReviewRun{}, fmt.Errorf("complete rider onboarding review run: %w", err)
	}

	if err := validateRiderReviewRunForApproval(run, arg.Approval.ApplicationID); err != nil {
		return OnboardingReviewRun{}, err
	}
	if !run.Outcome.Valid || run.Outcome.String != OnboardingReviewOutcomeApproved {
		return OnboardingReviewRun{}, fmt.Errorf("rider onboarding review run %d completed with non-approved outcome %s", run.ID, run.Outcome.String)
	}

	return run, nil
}

func defaultRiderReviewCreationParams(arg CreateRiderOnboardingReviewRunParams, applicationID int64) CreateRiderOnboardingReviewRunParams {
	if !arg.RiderApplicationID.Valid {
		arg.RiderApplicationID = pgtype.Int8{Int64: applicationID, Valid: true}
	}
	if arg.RunStatus == "" {
		arg.RunStatus = OnboardingReviewRunStatusQueued
	}
	if arg.Stage == "" {
		arg.Stage = "review"
	}
	if arg.Evidence == nil {
		arg.Evidence = []byte(`{}`)
	}
	if arg.RuleHits == nil {
		arg.RuleHits = []string{}
	}
	if arg.OcrJobRefs == nil {
		arg.OcrJobRefs = []int64{}
	}
	if arg.Snapshot == nil {
		arg.Snapshot = []byte(`{}`)
	}
	return arg
}

func defaultRiderReviewCompletionParams(arg CompleteOnboardingReviewRunParams, reviewRunID int64) CompleteOnboardingReviewRunParams {
	arg.ID = reviewRunID
	if arg.Stage == "" {
		arg.Stage = "review"
	}
	if arg.Evidence == nil {
		arg.Evidence = []byte(`{}`)
	}
	if arg.RuleHits == nil {
		arg.RuleHits = []string{}
	}
	if arg.OcrJobRefs == nil {
		arg.OcrJobRefs = []int64{}
	}
	if arg.Snapshot == nil {
		arg.Snapshot = []byte(`{}`)
	}
	return arg
}

func validateRiderReviewRunForApproval(run OnboardingReviewRun, applicationID int64) error {
	if run.ApplicationType != "rider" {
		return fmt.Errorf("onboarding review run %d is %s, not rider", run.ID, run.ApplicationType)
	}
	if !run.RiderApplicationID.Valid || run.RiderApplicationID.Int64 != applicationID {
		return fmt.Errorf("rider onboarding review run %d does not belong to application %d", run.ID, applicationID)
	}
	return nil
}

func activateRiderCredentialLedgersForApproval(ctx context.Context, q *Queries, riderID int64, applicationID int64, reviewRunID int64, arg ApproveRiderApplicationWithReviewTxParams) ([]CredentialLedger, error) {
	result := make([]CredentialLedger, 0, len(arg.CredentialEntries))

	for _, entry := range arg.CredentialEntries {
		if entry.RiderApplicationID == 0 {
			entry.RiderApplicationID = applicationID
		}
		if !entry.ReviewRunID.Valid {
			entry.ReviewRunID = pgtype.Int8{Int64: reviewRunID, Valid: true}
		}
		if _, err := q.DeactivateRiderActiveCredentialLedger(ctx, DeactivateRiderActiveCredentialLedgerParams{
			DeactivatedAt: arg.CredentialActivatedAt,
			RiderID:       pgtype.Int8{Int64: riderID, Valid: true},
			DocumentType:  entry.DocumentType,
		}); err != nil {
			return nil, fmt.Errorf("deactivate rider active credential %s: %w", entry.DocumentType, err)
		}

		ledger, err := q.CreateRiderCredentialLedger(ctx, CreateRiderCredentialLedgerParams{
			RiderID:            pgtype.Int8{Int64: riderID, Valid: true},
			DocumentType:       entry.DocumentType,
			RiderApplicationID: pgtype.Int8{Int64: entry.RiderApplicationID, Valid: entry.RiderApplicationID > 0},
			ReviewRunID:        entry.ReviewRunID,
			MediaAssetID:       entry.MediaAssetID,
			NormalizedPayload:  entry.NormalizedPayload,
			ExpiresAt:          entry.ExpiresAt,
			ActivatedAt:        timestamptzArgValue(arg.CredentialActivatedAt),
		})
		if err != nil {
			return nil, fmt.Errorf("create rider credential %s: %w", entry.DocumentType, err)
		}
		result = append(result, ledger)
	}

	return result, nil
}
