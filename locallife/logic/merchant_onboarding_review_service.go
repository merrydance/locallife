package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type merchantOnboardingReviewStore interface {
	GetMerchantApplication(ctx context.Context, id int64) (db.MerchantApplication, error)
	GetMerchantOwnedByUser(ctx context.Context, ownerUserID int64) (db.Merchant, error)
	ApproveMerchantApplicationTx(ctx context.Context, arg db.ApproveMerchantApplicationTxParams) (db.ApproveMerchantApplicationTxResult, error)
}

type MerchantOnboardingReviewService struct {
	store                       merchantOnboardingReviewStore
	onboardingReviewService     *OnboardingReviewService
	credentialGovernanceService *CredentialGovernanceService
	subjectProfileService       *MerchantSubjectProfileService
}

type MerchantOnboardingReviewResult struct {
	Application       db.MerchantApplication
	Merchant          *db.Merchant
	ReviewRun         *db.OnboardingReviewRun
	CredentialEntries []CredentialActivationInput
	RestoreReleased   bool
}

type MerchantReviewOCRReadiness struct {
	State             string   `json:"state,omitempty"`
	ReasonCode        string   `json:"reason_code,omitempty"`
	MissingFields     []string `json:"missing_fields,omitempty"`
	UnparseableFields []string `json:"unparseable_fields,omitempty"`
}

type MerchantReviewOCRConfirmation struct {
	ConfirmedBy int64             `json:"confirmed_by,omitempty"`
	ConfirmedAt string            `json:"confirmed_at,omitempty"`
	Source      string            `json:"source,omitempty"`
	Snapshot    map[string]string `json:"snapshot,omitempty"`
}

type MerchantReviewBusinessLicenseOCRData struct {
	Readiness           *MerchantReviewOCRReadiness    `json:"readiness,omitempty"`
	Confirmation        *MerchantReviewOCRConfirmation `json:"confirmation,omitempty"`
	Correction          json.RawMessage                `json:"correction,omitempty"`
	OCRJobID            *int64                         `json:"ocr_job_id,omitempty"`
	RegNum              string                         `json:"reg_num,omitempty"`
	EnterpriseName      string                         `json:"enterprise_name,omitempty"`
	LegalRepresentative string                         `json:"legal_representative,omitempty"`
	Address             string                         `json:"address,omitempty"`
	BusinessScope       string                         `json:"business_scope,omitempty"`
	ValidPeriod         string                         `json:"valid_period,omitempty"`
	CreditCode          string                         `json:"credit_code,omitempty"`
	TypeOfEnterprise    string                         `json:"type_of_enterprise,omitempty"`
}

type MerchantReviewFoodPermitOCRData struct {
	Readiness    *MerchantReviewOCRReadiness    `json:"readiness,omitempty"`
	Confirmation *MerchantReviewOCRConfirmation `json:"confirmation,omitempty"`
	Correction   json.RawMessage                `json:"correction,omitempty"`
	OCRJobID     *int64                         `json:"ocr_job_id,omitempty"`
	RawText      string                         `json:"raw_text,omitempty"`
	PermitNo     string                         `json:"permit_no,omitempty"`
	CompanyName  string                         `json:"company_name,omitempty"`
	OperatorName string                         `json:"operator_name,omitempty"`
	ValidFrom    string                         `json:"valid_from,omitempty"`
	ValidTo      string                         `json:"valid_to,omitempty"`
}

type MerchantReviewIDCardOCRData struct {
	Readiness *MerchantReviewOCRReadiness `json:"readiness,omitempty"`
	OCRJobID  *int64                      `json:"ocr_job_id,omitempty"`
	Name      string                      `json:"name,omitempty"`
	IDNumber  string                      `json:"id_number,omitempty"`
	ValidDate string                      `json:"valid_date,omitempty"`
}

func NewMerchantOnboardingReviewService(store merchantOnboardingReviewStore, onboardingReviewService *OnboardingReviewService, credentialGovernanceService *CredentialGovernanceService) *MerchantOnboardingReviewService {
	if store == nil {
		return nil
	}
	return &MerchantOnboardingReviewService{
		store:                       store,
		onboardingReviewService:     onboardingReviewService,
		credentialGovernanceService: credentialGovernanceService,
	}
}

func (service *MerchantOnboardingReviewService) WithSubjectProfileService(subjectProfileService *MerchantSubjectProfileService) *MerchantOnboardingReviewService {
	if service == nil {
		return service
	}
	service.subjectProfileService = subjectProfileService
	return service
}

func (service *MerchantOnboardingReviewService) ProcessApplication(ctx context.Context, applicationID int64, requestedBy int64, existingRunID *int64) (MerchantOnboardingReviewResult, error) {
	if service == nil || service.store == nil {
		return MerchantOnboardingReviewResult{}, nil
	}

	application, err := service.store.GetMerchantApplication(ctx, applicationID)
	if err != nil {
		return MerchantOnboardingReviewResult{}, fmt.Errorf("get merchant application: %w", err)
	}

	return service.ProcessSubmittedApplication(ctx, application, requestedBy, existingRunID)
}

func (service *MerchantOnboardingReviewService) ProcessSubmittedApplication(ctx context.Context, application db.MerchantApplication, requestedBy int64, existingRunID *int64) (MerchantOnboardingReviewResult, error) {
	if service == nil || service.store == nil {
		return MerchantOnboardingReviewResult{}, nil
	}
	if application.Status != db.MerchantApplicationStatusSubmitted {
		if application.Status == db.MerchantApplicationStatusApproved && existingRunID != nil {
			return service.repairApprovedMerchantApplication(ctx, application, requestedBy, *existingRunID)
		}
		if existingRunID != nil && service.onboardingReviewService != nil {
			return service.cancelSupersededMerchantReviewRun(ctx, application, *existingRunID)
		}
		return MerchantOnboardingReviewResult{}, fmt.Errorf("merchant application %d status %s is not submitted", application.ID, application.Status)
	}

	subjectProfile, err := service.subjectProfileForSubmittedApplication(application)
	if err != nil {
		return MerchantOnboardingReviewResult{}, err
	}

	decision := merchantAutoApprovedDecision(application, requestedBy)
	txArg, err := buildMerchantApprovalTxParamsFromSubjectProfile(application, subjectProfile)
	if err != nil {
		return MerchantOnboardingReviewResult{}, err
	}

	txResult, err := service.store.ApproveMerchantApplicationTx(ctx, txArg)
	if err != nil {
		return MerchantOnboardingReviewResult{}, fmt.Errorf("approve merchant application tx: %w", err)
	}

	result := MerchantOnboardingReviewResult{
		Application: txResult.Application,
		Merchant:    &txResult.Merchant,
	}
	if txResult.SubjectProfile.ID > 0 {
		approvedSubjectProfile, err := merchantSubjectProfileFromDB(txResult.SubjectProfile)
		if err != nil {
			return MerchantOnboardingReviewResult{}, fmt.Errorf("decode approved merchant subject profile: %w", err)
		}
		subjectProfile = approvedSubjectProfile
	}

	if service.onboardingReviewService != nil {
		var reviewRun db.OnboardingReviewRun
		if existingRunID != nil {
			reviewRun, err = service.onboardingReviewService.CompleteMerchantReviewRun(ctx, *existingRunID, result.Application.ID, decision)
		} else {
			reviewRun, err = service.onboardingReviewService.RecordMerchantReview(ctx, result.Application.ID, decision)
		}
		if err != nil {
			return MerchantOnboardingReviewResult{}, fmt.Errorf("persist merchant onboarding review run: %w", err)
		}
		result.ReviewRun = &reviewRun
		summaryJSON, err := buildOnboardingReviewSummaryJSON(reviewRun)
		if err != nil {
			return MerchantOnboardingReviewResult{}, fmt.Errorf("build merchant onboarding review summary: %w", err)
		}
		result.Application.ReviewSummary = summaryJSON
	}

	if service.credentialGovernanceService != nil && result.Merchant != nil {
		entries, err := buildMerchantCredentialActivationInputsFromSubjectProfile(result.Application, subjectProfile)
		if err != nil {
			return MerchantOnboardingReviewResult{}, fmt.Errorf("build merchant credential activation inputs: %w", err)
		}
		result.CredentialEntries = entries
		if len(entries) > 0 {
			if err := service.activateMissingMerchantCredentials(ctx, result.Merchant.ID, result.Application.ID, merchantReviewRunID(result.ReviewRun), entries); err != nil {
				return MerchantOnboardingReviewResult{}, fmt.Errorf("activate merchant credentials: %w", err)
			}
			restoreResult, err := service.credentialGovernanceService.RestoreMerchantIfEligible(ctx, result.Merchant.ID)
			if err != nil {
				return MerchantOnboardingReviewResult{}, fmt.Errorf("restore merchant credential governance: %w", err)
			}
			result.RestoreReleased = restoreResult.Released
		}
	}

	return result, nil
}

func (service *MerchantOnboardingReviewService) repairApprovedMerchantApplication(ctx context.Context, application db.MerchantApplication, requestedBy int64, runID int64) (MerchantOnboardingReviewResult, error) {
	result := MerchantOnboardingReviewResult{Application: application}
	if runID <= 0 {
		return result, fmt.Errorf("merchant application %d status %s is not submitted", application.ID, application.Status)
	}
	if service.onboardingReviewService == nil {
		return MerchantOnboardingReviewResult{}, fmt.Errorf("onboarding review service not configured for approved merchant application repair")
	}

	merchant, err := service.store.GetMerchantOwnedByUser(ctx, application.UserID)
	if err != nil {
		return MerchantOnboardingReviewResult{}, fmt.Errorf("get approved merchant for application repair: %w", err)
	}
	result.Merchant = &merchant
	subjectProfile, err := service.subjectProfileForSubmittedApplication(application)
	if err != nil {
		return MerchantOnboardingReviewResult{}, err
	}
	approvedSubjectProfile, err := service.attachApprovedSubjectProfile(ctx, subjectProfile, result.Application, result.Merchant)
	if err != nil {
		return MerchantOnboardingReviewResult{}, err
	}
	if approvedSubjectProfile.ApplicationID > 0 {
		subjectProfile = approvedSubjectProfile
	}

	decision := merchantAutoApprovedDecision(application, requestedBy)
	reviewRun, err := service.onboardingReviewService.RepairApprovedMerchantReviewRun(ctx, runID, application.ID, decision)
	if err != nil {
		return MerchantOnboardingReviewResult{}, fmt.Errorf("repair approved merchant onboarding review run: %w", err)
	}
	result.ReviewRun = &reviewRun
	summaryJSON, err := buildOnboardingReviewSummaryJSON(reviewRun)
	if err != nil {
		return MerchantOnboardingReviewResult{}, fmt.Errorf("build approved merchant onboarding review summary: %w", err)
	}
	result.Application.ReviewSummary = summaryJSON

	if service.credentialGovernanceService != nil {
		entries, err := buildMerchantCredentialActivationInputsFromSubjectProfile(result.Application, subjectProfile)
		if err != nil {
			return MerchantOnboardingReviewResult{}, fmt.Errorf("build merchant credential activation inputs: %w", err)
		}
		result.CredentialEntries = entries
		if len(entries) > 0 {
			if err := service.activateMissingMerchantCredentials(ctx, merchant.ID, result.Application.ID, merchantReviewRunID(result.ReviewRun), entries); err != nil {
				return MerchantOnboardingReviewResult{}, fmt.Errorf("activate merchant credentials: %w", err)
			}
			restoreResult, err := service.credentialGovernanceService.RestoreMerchantIfEligible(ctx, merchant.ID)
			if err != nil {
				return MerchantOnboardingReviewResult{}, fmt.Errorf("restore merchant credential governance: %w", err)
			}
			result.RestoreReleased = restoreResult.Released
		}
	}

	return result, nil
}

func (service *MerchantOnboardingReviewService) activateMissingMerchantCredentials(ctx context.Context, merchantID int64, applicationID int64, reviewRunID *int64, entries []CredentialActivationInput) error {
	if service == nil || service.credentialGovernanceService == nil || service.credentialGovernanceService.store == nil || len(entries) == 0 {
		return nil
	}

	activeLedgers, err := service.credentialGovernanceService.store.GetActiveMerchantCredentialLedgers(ctx, pgtype.Int8{Int64: merchantID, Valid: true})
	if err != nil {
		return fmt.Errorf("get active merchant credential ledgers: %w", err)
	}

	missingEntries := missingMerchantCredentialEntries(entries, applicationID, reviewRunID, activeLedgers)
	if len(missingEntries) == 0 {
		return nil
	}

	_, err = service.credentialGovernanceService.ActivateMerchantCredentials(ctx, ActivateMerchantCredentialsInput{
		MerchantID:            merchantID,
		MerchantApplicationID: applicationID,
		ReviewRunID:           reviewRunID,
		Entries:               missingEntries,
	})
	return err
}

func (service *MerchantOnboardingReviewService) subjectProfileForSubmittedApplication(application db.MerchantApplication) (MerchantSubjectProfile, error) {
	return BuildMerchantSubjectProfileFromApplication(application)
}

func (service *MerchantOnboardingReviewService) attachApprovedSubjectProfile(ctx context.Context, profile MerchantSubjectProfile, application db.MerchantApplication, merchant *db.Merchant) (MerchantSubjectProfile, error) {
	if service == nil || service.subjectProfileService == nil || merchant == nil || merchant.ID <= 0 {
		return profile, nil
	}
	approvedProfile, err := service.subjectProfileService.SaveApplicationProfile(ctx, application, merchant.ID)
	if err != nil {
		return MerchantSubjectProfile{}, fmt.Errorf("save approved merchant subject profile: %w", err)
	}
	return approvedProfile, nil
}

func missingMerchantCredentialEntries(entries []CredentialActivationInput, applicationID int64, reviewRunID *int64, activeLedgers []db.CredentialLedger) []CredentialActivationInput {
	missing := make([]CredentialActivationInput, 0, len(entries))
	for _, entry := range entries {
		if !merchantCredentialEntryAlreadyActive(entry, applicationID, reviewRunID, activeLedgers) {
			missing = append(missing, entry)
		}
	}
	return missing
}

func merchantCredentialEntryAlreadyActive(entry CredentialActivationInput, applicationID int64, reviewRunID *int64, activeLedgers []db.CredentialLedger) bool {
	for _, ledger := range activeLedgers {
		if !ledger.Active || ledger.DocumentType != entry.DocumentType || ledger.MediaAssetID != entry.MediaAssetID {
			continue
		}
		if !ledger.MerchantApplicationID.Valid || ledger.MerchantApplicationID.Int64 != applicationID {
			continue
		}
		if reviewRunID == nil {
			if ledger.ReviewRunID.Valid {
				continue
			}
		} else if !ledger.ReviewRunID.Valid || ledger.ReviewRunID.Int64 != *reviewRunID {
			continue
		}
		return true
	}
	return false
}

func (service *MerchantOnboardingReviewService) cancelSupersededMerchantReviewRun(ctx context.Context, application db.MerchantApplication, runID int64) (MerchantOnboardingReviewResult, error) {
	result := MerchantOnboardingReviewResult{Application: application}
	if runID <= 0 {
		return result, fmt.Errorf("merchant application %d status %s is not submitted", application.ID, application.Status)
	}
	reviewRun, err := service.onboardingReviewService.CancelReviewRun(
		ctx,
		runID,
		db.OnboardingReviewReasonSupersededByEdit,
		db.OnboardingReviewReasonMessageSupersededByEdit,
	)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, nil
		}
		return MerchantOnboardingReviewResult{}, fmt.Errorf("cancel superseded merchant onboarding review run: %w", err)
	}
	if err := service.onboardingReviewService.updateMerchantApplicationReviewSummary(ctx, application.ID, reviewRun); err != nil {
		return MerchantOnboardingReviewResult{}, fmt.Errorf("update superseded merchant onboarding review summary: %w", err)
	}
	summaryJSON, err := buildOnboardingReviewSummaryJSON(reviewRun)
	if err != nil {
		return MerchantOnboardingReviewResult{}, fmt.Errorf("build superseded merchant onboarding review summary: %w", err)
	}
	result.Application.ReviewSummary = summaryJSON
	result.ReviewRun = &reviewRun
	return result, nil
}

func merchantAutoApprovedDecision(application db.MerchantApplication, requestedBy int64) OnboardingReviewDecision {
	return OnboardingReviewDecision{
		Outcome:     "approved",
		ReasonCode:  "auto_approved",
		RuleHits:    []string{"merchant.auto_approve"},
		OCRJobRefs:  merchantReviewOCRJobRefs(application),
		RequestedBy: &requestedBy,
		Snapshot: map[string]any{
			"application_id":   application.ID,
			"application_type": "merchant",
			"status":           application.Status,
			"user_id":          application.UserID,
			"merchant_name":    application.MerchantName,
		},
	}
}

func merchantReviewOCRJobRefs(application db.MerchantApplication) []int64 {
	refs := make([]int64, 0, 4)
	if ocrData, err := decodeMerchantReviewOCRData[MerchantReviewBusinessLicenseOCRData](application.BusinessLicenseOcr); err == nil && ocrData != nil && ocrData.OCRJobID != nil {
		refs = append(refs, *ocrData.OCRJobID)
	}
	if ocrData, err := decodeMerchantReviewOCRData[MerchantReviewFoodPermitOCRData](application.FoodPermitOcr); err == nil && ocrData != nil && ocrData.OCRJobID != nil {
		refs = append(refs, *ocrData.OCRJobID)
	}
	if ocrData, err := decodeMerchantReviewOCRData[MerchantReviewIDCardOCRData](application.IDCardFrontOcr); err == nil && ocrData != nil && ocrData.OCRJobID != nil {
		refs = append(refs, *ocrData.OCRJobID)
	}
	if ocrData, err := decodeMerchantReviewOCRData[MerchantReviewIDCardOCRData](application.IDCardBackOcr); err == nil && ocrData != nil && ocrData.OCRJobID != nil {
		refs = append(refs, *ocrData.OCRJobID)
	}
	return dedupeInt64s(refs)
}

func decodeMerchantReviewOCRData[T any](raw []byte) (*T, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var payload T
	if err := json.Unmarshal(raw, &payload); err == nil {
		return &payload, nil
	}
	var wrapped string
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(wrapped), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func parseMerchantCredentialExpiry(raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("credential expiry missing")
	}
	if strings.Contains(trimmed, "长期") || strings.Contains(trimmed, "永久") {
		return nil, nil
	}
	expiresAt, err := parseRiderFlexibleDocumentEndDate(trimmed)
	if err != nil {
		return nil, err
	}
	return &expiresAt, nil
}

func merchantReviewRunID(run *db.OnboardingReviewRun) *int64 {
	if run == nil {
		return nil
	}
	return &run.ID
}
