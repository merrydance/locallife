package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

type merchantOnboardingReviewStore interface {
	GetMerchantApplication(ctx context.Context, id int64) (db.MerchantApplication, error)
	ApproveMerchantApplicationTx(ctx context.Context, arg db.ApproveMerchantApplicationTxParams) (db.ApproveMerchantApplicationTxResult, error)
}

type MerchantOnboardingReviewService struct {
	store                       merchantOnboardingReviewStore
	onboardingReviewService     *OnboardingReviewService
	credentialGovernanceService *CredentialGovernanceService
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

type MerchantReviewBusinessLicenseOCRData struct {
	Readiness           *MerchantReviewOCRReadiness `json:"readiness,omitempty"`
	OCRJobID            *int64                      `json:"ocr_job_id,omitempty"`
	RegNum              string                      `json:"reg_num,omitempty"`
	EnterpriseName      string                      `json:"enterprise_name,omitempty"`
	LegalRepresentative string                      `json:"legal_representative,omitempty"`
	Address             string                      `json:"address,omitempty"`
	BusinessScope       string                      `json:"business_scope,omitempty"`
	ValidPeriod         string                      `json:"valid_period,omitempty"`
	CreditCode          string                      `json:"credit_code,omitempty"`
}

type MerchantReviewFoodPermitOCRData struct {
	Readiness    *MerchantReviewOCRReadiness `json:"readiness,omitempty"`
	OCRJobID     *int64                      `json:"ocr_job_id,omitempty"`
	RawText      string                      `json:"raw_text,omitempty"`
	PermitNo     string                      `json:"permit_no,omitempty"`
	CompanyName  string                      `json:"company_name,omitempty"`
	OperatorName string                      `json:"operator_name,omitempty"`
	ValidFrom    string                      `json:"valid_from,omitempty"`
	ValidTo      string                      `json:"valid_to,omitempty"`
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
	if application.Status != "submitted" {
		return MerchantOnboardingReviewResult{}, fmt.Errorf("merchant application %d status %s is not submitted", application.ID, application.Status)
	}

	decision := merchantAutoApprovedDecision(application, requestedBy)
	txArg, err := buildMerchantApprovalTxParams(application)
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
		entries, err := buildMerchantCredentialActivationInputs(result.Application)
		if err != nil {
			return MerchantOnboardingReviewResult{}, fmt.Errorf("build merchant credential activation inputs: %w", err)
		}
		result.CredentialEntries = entries
		if len(entries) > 0 {
			_, err = service.credentialGovernanceService.ActivateMerchantCredentials(ctx, ActivateMerchantCredentialsInput{
				MerchantID:            result.Merchant.ID,
				MerchantApplicationID: result.Application.ID,
				ReviewRunID:           merchantReviewRunID(result.ReviewRun),
				Entries:               entries,
			})
			if err != nil {
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

func buildMerchantApprovalTxParams(application db.MerchantApplication) (db.ApproveMerchantApplicationTxParams, error) {
	if !application.RegionID.Valid {
		return db.ApproveMerchantApplicationTxParams{}, fmt.Errorf("merchant application %d missing region_id", application.ID)
	}

	storefrontImages, merchantStorefrontImages := merchantApprovalImagePayload(application.StorefrontImages, 3)
	environmentImages, merchantEnvironmentImages := merchantApprovalImagePayload(application.EnvironmentImages, 5)

	appData, err := json.Marshal(map[string]any{
		"business_license_number":         application.BusinessLicenseNumber,
		"legal_person_name":               application.LegalPersonName,
		"legal_person_id_number":          application.LegalPersonIDNumber,
		"business_license_media_asset_id": application.BusinessLicenseMediaAssetID.Int64,
		"id_card_front_media_asset_id":    application.IDCardFrontMediaAssetID.Int64,
		"id_card_back_media_asset_id":     application.IDCardBackMediaAssetID.Int64,
		"food_permit_media_asset_id":      application.FoodPermitMediaAssetID.Int64,
		"storefront_images":               storefrontImages,
		"environment_images":              environmentImages,
	})
	if err != nil {
		return db.ApproveMerchantApplicationTxParams{}, fmt.Errorf("marshal merchant application data: %w", err)
	}

	return db.ApproveMerchantApplicationTxParams{
		ApplicationID:     application.ID,
		UserID:            application.UserID,
		MerchantName:      application.MerchantName,
		Phone:             application.ContactPhone,
		Address:           application.BusinessAddress,
		Latitude:          application.Latitude,
		Longitude:         application.Longitude,
		RegionID:          application.RegionID.Int64,
		AppData:           appData,
		StorefrontImages:  merchantStorefrontImages,
		EnvironmentImages: merchantEnvironmentImages,
	}, nil
}

func merchantApprovalImagePayload(raw []byte, maxCount int) ([]string, []byte) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var rawImages []json.RawMessage
	if err := json.Unmarshal(raw, &rawImages); err != nil {
		return []string{}, nil
	}
	if len(rawImages) > maxCount {
		return []string{}, nil
	}
	images := make([]string, 0, len(rawImages))
	for _, rawImage := range rawImages {
		var image any
		if err := json.Unmarshal(rawImage, &image); err != nil {
			return []string{}, nil
		}
		imageString, ok := image.(string)
		if !ok {
			return []string{}, nil
		}
		images = append(images, imageString)
	}
	return images, raw
}

func buildMerchantCredentialActivationInputs(application db.MerchantApplication) ([]CredentialActivationInput, error) {
	businessLicenseOCR, err := decodeMerchantReviewOCRData[MerchantReviewBusinessLicenseOCRData](application.BusinessLicenseOcr)
	if err != nil || businessLicenseOCR == nil {
		return nil, fmt.Errorf("business_license payload missing")
	}
	if !application.BusinessLicenseMediaAssetID.Valid {
		return nil, fmt.Errorf("business_license media missing")
	}
	businessLicenseExpiry, err := parseMerchantCredentialExpiry(businessLicenseOCR.ValidPeriod)
	if err != nil {
		return nil, fmt.Errorf("parse business license expiry: %w", err)
	}

	foodPermitOCR, err := decodeMerchantReviewOCRData[MerchantReviewFoodPermitOCRData](application.FoodPermitOcr)
	if err != nil || foodPermitOCR == nil {
		return nil, fmt.Errorf("food_permit payload missing")
	}
	if !application.FoodPermitMediaAssetID.Valid {
		return nil, fmt.Errorf("food_permit media missing")
	}
	foodPermitExpiry, err := parseMerchantCredentialExpiry(foodPermitOCR.ValidTo)
	if err != nil {
		return nil, fmt.Errorf("parse food permit expiry: %w", err)
	}
	businessLicenseNumber := strings.TrimSpace(businessLicenseOCR.RegNum)
	if businessLicenseNumber == "" {
		businessLicenseNumber = strings.TrimSpace(businessLicenseOCR.CreditCode)
	}

	return []CredentialActivationInput{
		{
			DocumentType: db.CredentialDocumentTypeBusinessLicense,
			MediaAssetID: application.BusinessLicenseMediaAssetID.Int64,
			ExpiresAt:    businessLicenseExpiry,
			NormalizedPayload: map[string]any{
				"license_number":       businessLicenseNumber,
				"credit_code":          strings.TrimSpace(businessLicenseOCR.CreditCode),
				"enterprise_name":      strings.TrimSpace(businessLicenseOCR.EnterpriseName),
				"legal_representative": strings.TrimSpace(businessLicenseOCR.LegalRepresentative),
				"address":              strings.TrimSpace(businessLicenseOCR.Address),
				"business_scope":       strings.TrimSpace(businessLicenseOCR.BusinessScope),
				"valid_period":         strings.TrimSpace(businessLicenseOCR.ValidPeriod),
			},
		},
		{
			DocumentType: db.CredentialDocumentTypeFoodPermit,
			MediaAssetID: application.FoodPermitMediaAssetID.Int64,
			ExpiresAt:    foodPermitExpiry,
			NormalizedPayload: map[string]any{
				"permit_number": strings.TrimSpace(foodPermitOCR.PermitNo),
				"company_name":  strings.TrimSpace(foodPermitOCR.CompanyName),
				"operator_name": strings.TrimSpace(foodPermitOCR.OperatorName),
				"valid_from":    strings.TrimSpace(foodPermitOCR.ValidFrom),
				"valid_to":      strings.TrimSpace(foodPermitOCR.ValidTo),
			},
		},
	}, nil
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
