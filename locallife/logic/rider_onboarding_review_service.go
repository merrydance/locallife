package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type riderOnboardingReviewStore interface {
	GetRiderApplication(ctx context.Context, id int64) (db.RiderApplication, error)
	ApproveRiderApplicationTx(ctx context.Context, arg db.ApproveRiderApplicationTxParams) (db.ApproveRiderApplicationTxResult, error)
	ReturnRiderApplicationToDraft(ctx context.Context, arg db.ReturnRiderApplicationToDraftParams) (db.RiderApplication, error)
}

type RiderOnboardingReviewService struct {
	store                       riderOnboardingReviewStore
	onboardingReviewService     *OnboardingReviewService
	credentialGovernanceService *CredentialGovernanceService
	now                         func() time.Time
}

type RiderOnboardingReviewResult struct {
	Application       db.RiderApplication
	Rider             *db.Rider
	ReviewRun         *db.OnboardingReviewRun
	Approved          bool
	ReasonCode        string
	RejectReason      string
	CredentialEntries []CredentialActivationInput
	RestoreReleased   bool
}

type riderReviewIDCardOCRData struct {
	Name     string `json:"name,omitempty"`
	IDNumber string `json:"id_number,omitempty"`
	ValidEnd string `json:"valid_end,omitempty"`
	OCRJobID *int64 `json:"ocr_job_id,omitempty"`
}

type riderReviewHealthCertOCRData struct {
	Name       string `json:"name,omitempty"`
	IDNumber   string `json:"id_number,omitempty"`
	CertNumber string `json:"cert_number,omitempty"`
	ValidStart string `json:"valid_start,omitempty"`
	ValidEnd   string `json:"valid_end,omitempty"`
	OCRJobID   *int64 `json:"ocr_job_id,omitempty"`
}

func NewRiderOnboardingReviewService(store riderOnboardingReviewStore, onboardingReviewService *OnboardingReviewService, credentialGovernanceService *CredentialGovernanceService) *RiderOnboardingReviewService {
	if store == nil {
		return nil
	}
	return &RiderOnboardingReviewService{
		store:                       store,
		onboardingReviewService:     onboardingReviewService,
		credentialGovernanceService: credentialGovernanceService,
		now:                         time.Now,
	}
}

func (service *RiderOnboardingReviewService) ProcessApplication(ctx context.Context, applicationID int64, requestedBy int64, existingRunID *int64) (RiderOnboardingReviewResult, error) {
	if service == nil || service.store == nil {
		return RiderOnboardingReviewResult{}, nil
	}

	application, err := service.store.GetRiderApplication(ctx, applicationID)
	if err != nil {
		return RiderOnboardingReviewResult{}, fmt.Errorf("get rider application: %w", err)
	}
	return service.ProcessSubmittedApplication(ctx, application, requestedBy, existingRunID)
}

func (service *RiderOnboardingReviewService) ProcessSubmittedApplication(ctx context.Context, application db.RiderApplication, requestedBy int64, existingRunID *int64) (RiderOnboardingReviewResult, error) {
	if service == nil || service.store == nil {
		return RiderOnboardingReviewResult{}, nil
	}

	if application.Status != db.RiderApplicationStatusSubmitted {
		return RiderOnboardingReviewResult{}, fmt.Errorf("rider application %d status %s is not submitted", application.ID, application.Status)
	}

	decision, idCardOCR, err := service.buildDecision(application, requestedBy)
	if err != nil {
		return RiderOnboardingReviewResult{}, err
	}

	result := RiderOnboardingReviewResult{
		Approved:     decision.Outcome == "approved",
		ReasonCode:   decision.ReasonCode,
		RejectReason: decision.ReasonMessage,
	}

	if result.Approved {
		approvedResult, err := service.store.ApproveRiderApplicationTx(ctx, db.ApproveRiderApplicationTxParams{
			ApplicationID: application.ID,
			ReviewedBy:    pgtype.Int8{},
			RiderRealName: application.RealName.String,
			RiderIDCardNo: idCardOCR.IDNumber,
			RiderPhone:    application.Phone.String,
			RegionID:      pgtype.Int8{},
		})
		if err != nil {
			return RiderOnboardingReviewResult{}, fmt.Errorf("approve rider application tx: %w", err)
		}
		result.Application = approvedResult.Application
		result.Rider = &approvedResult.Rider
	} else {
		returnedApplication, err := service.store.ReturnRiderApplicationToDraft(ctx, db.ReturnRiderApplicationToDraftParams{
			ID:           application.ID,
			RejectReason: pgtype.Text{String: decision.ReasonMessage, Valid: decision.ReasonMessage != ""},
			ReviewedBy:   pgtype.Int8{},
		})
		if err != nil {
			return RiderOnboardingReviewResult{}, fmt.Errorf("return rider application to draft: %w", err)
		}
		result.Application = returnedApplication
	}

	if service.onboardingReviewService != nil {
		var reviewRun db.OnboardingReviewRun
		if existingRunID != nil {
			reviewRun, err = service.onboardingReviewService.CompleteRiderReviewRun(ctx, *existingRunID, result.Application.ID, decision)
		} else {
			reviewRun, err = service.onboardingReviewService.RecordRiderReview(ctx, result.Application.ID, decision)
		}
		if err != nil {
			return RiderOnboardingReviewResult{}, fmt.Errorf("persist rider onboarding review run: %w", err)
		}
		result.ReviewRun = &reviewRun
		if summaryJSON, err := buildOnboardingReviewSummaryJSON(reviewRun); err == nil {
			result.Application.ReviewSummary = summaryJSON
		} else {
			return RiderOnboardingReviewResult{}, fmt.Errorf("build rider onboarding review summary: %w", err)
		}
	}

	if result.Approved && service.credentialGovernanceService != nil && result.Rider != nil {
		entries, err := buildRiderCredentialActivationInputs(result.Application)
		if err != nil {
			return RiderOnboardingReviewResult{}, fmt.Errorf("build rider credential activation inputs: %w", err)
		}
		result.CredentialEntries = entries
		if len(entries) > 0 {
			_, err = service.credentialGovernanceService.ActivateRiderCredentials(ctx, ActivateRiderCredentialsInput{
				RiderID:            result.Rider.ID,
				RiderApplicationID: result.Application.ID,
				ReviewRunID:        onboardingReviewRunID(result.ReviewRun),
				Entries:            entries,
			})
			if err != nil {
				return RiderOnboardingReviewResult{}, fmt.Errorf("activate rider credentials: %w", err)
			}
			restoreResult, err := service.credentialGovernanceService.RestoreRiderIfEligible(ctx, result.Rider.ID)
			if err != nil {
				return RiderOnboardingReviewResult{}, fmt.Errorf("restore rider credential governance: %w", err)
			}
			result.RestoreReleased = restoreResult.Released
		}
	}

	return result, nil
}

func (service *RiderOnboardingReviewService) buildDecision(application db.RiderApplication, requestedBy int64) (OnboardingReviewDecision, riderReviewIDCardOCRData, error) {
	approved, rejectReason, idCardOCR := evaluateRiderApplication(application, service.now())
	outcome := "needs_resubmit"
	if approved {
		outcome = "approved"
	}
	reasonCode := riderReviewReasonCode(outcome, rejectReason)

	decision := OnboardingReviewDecision{
		Outcome:       outcome,
		ReasonCode:    reasonCode,
		ReasonMessage: strings.TrimSpace(rejectReason),
		RuleHits:      riderReviewRuleHits(outcome, reasonCode),
		OCRJobRefs:    riderReviewOCRJobRefs(application, idCardOCR),
		RequestedBy:   &requestedBy,
		Snapshot: map[string]any{
			"application_id":   application.ID,
			"application_type": "rider",
			"status":           application.Status,
			"user_id":          application.UserID,
		},
	}
	return decision, idCardOCR, nil
}

func evaluateRiderApplication(application db.RiderApplication, now time.Time) (bool, string, riderReviewIDCardOCRData) {
	if !application.HealthCertMediaAssetID.Valid {
		return false, "健康证未上传", riderReviewIDCardOCRData{}
	}
	if len(application.IDCardOcr) == 0 {
		return false, "身份证信息未识别，请重新上传清晰的身份证照片", riderReviewIDCardOCRData{}
	}

	decodedIDCardOCR, err := decodeRiderReviewIDCardOCRData(application.IDCardOcr)
	if err != nil || decodedIDCardOCR == nil {
		return false, "身份证信息解析失败，请重新上传", riderReviewIDCardOCRData{}
	}
	idCardOCR := *decodedIDCardOCR
	if strings.TrimSpace(idCardOCR.IDNumber) == "" {
		return false, "身份证号未识别，请重新上传清晰的身份证正面照片", idCardOCR
	}
	if strings.TrimSpace(idCardOCR.ValidEnd) == "" {
		return false, "身份证有效期未识别，请上传身份证背面照片", idCardOCR
	}
	if !strings.Contains(idCardOCR.ValidEnd, "长期") && !strings.Contains(idCardOCR.ValidEnd, "永久") {
		endDate, err := parseRiderFlexibleDocumentEndDate(idCardOCR.ValidEnd)
		if err != nil {
			return false, "身份证有效期格式无法识别，请联系客服", idCardOCR
		}
		if now.After(endDate) {
			return false, "身份证已过期，请更换有效身份证后重新申请", idCardOCR
		}
	}

	if len(application.HealthCertOcr) == 0 {
		return false, "健康证信息未识别，请重新上传清晰的健康证照片", idCardOCR
	}
	decodedHealthOCR, err := decodeRiderReviewHealthCertOCRData(application.HealthCertOcr)
	if err != nil || decodedHealthOCR == nil {
		return false, "健康证信息解析失败，请重新上传", idCardOCR
	}
	healthOCR := *decodedHealthOCR

	idName := normalizeRiderPersonName(idCardOCR.Name)
	if idName == "" && application.RealName.Valid {
		idName = normalizeRiderPersonName(application.RealName.String)
	}
	healthName := normalizeRiderPersonName(healthOCR.Name)
	if idName == "" {
		return false, "身份证姓名未识别，请重新上传清晰的身份证正面照片", idCardOCR
	}
	if healthName == "" {
		return false, "健康证姓名未识别，请重新上传清晰的健康证照片", idCardOCR
	}
	if idName != healthName {
		return false, "健康证姓名与身份证姓名不一致", idCardOCR
	}
	if strings.TrimSpace(healthOCR.ValidEnd) == "" {
		return false, "健康证有效期未识别，请重新上传清晰的健康证照片", idCardOCR
	}
	if strings.Contains(healthOCR.ValidEnd, "长期") || strings.Contains(healthOCR.ValidEnd, "永久") {
		return true, "", idCardOCR
	}
	validEndDate, err := parseRiderFlexibleDocumentEndDate(healthOCR.ValidEnd)
	if err != nil {
		return false, "健康证有效期格式无法识别，请重新上传", idCardOCR
	}
	if !validEndDate.After(now.AddDate(0, 0, 7)) {
		return false, "健康证有效期需超过当日7天", idCardOCR
	}

	return true, "", idCardOCR
}

func decodeRiderReviewOCRPayload(data []byte, target any) error {
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, target); err == nil {
		return nil
	}

	var embedded string
	if err := json.Unmarshal(data, &embedded); err != nil {
		return err
	}
	if strings.TrimSpace(embedded) == "" {
		return nil
	}
	return json.Unmarshal([]byte(embedded), target)
}

func decodeRiderReviewIDCardOCRData(data []byte) (*riderReviewIDCardOCRData, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var payload riderReviewIDCardOCRData
	if err := decodeRiderReviewOCRPayload(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func decodeRiderReviewHealthCertOCRData(data []byte) (*riderReviewHealthCertOCRData, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var payload riderReviewHealthCertOCRData
	if err := decodeRiderReviewOCRPayload(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func normalizeRiderPersonName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "\t", "")
	return name
}

func parseRiderFlexibleDocumentEndDate(dateStr string) (time.Time, error) {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}

	eightDigitRegex := regexp.MustCompile(`\d{8}`)
	if match := eightDigitRegex.FindAllString(trimmed, -1); len(match) > 0 {
		return time.Parse("20060102", match[len(match)-1])
	}

	dateRegex := regexp.MustCompile(`\d{4}\s*(?:年|[./-])\s*\d{1,2}\s*(?:月|[./-])\s*\d{1,2}\s*日?`)
	matches := dateRegex.FindAllString(trimmed, -1)
	if len(matches) == 0 {
		return time.Time{}, fmt.Errorf("no date found in %q", dateStr)
	}

	last := matches[len(matches)-1]
	normalized := strings.TrimSpace(last)
	normalized = strings.ReplaceAll(normalized, " 年", "年")
	normalized = strings.ReplaceAll(normalized, "年 ", "年")
	normalized = strings.ReplaceAll(normalized, " 月", "月")
	normalized = strings.ReplaceAll(normalized, "月 ", "月")
	normalized = strings.ReplaceAll(normalized, " 日", "日")
	normalized = strings.ReplaceAll(normalized, "日 ", "日")
	normalized = strings.ReplaceAll(normalized, ".", "-")
	normalized = strings.ReplaceAll(normalized, "/", "-")
	normalized = strings.ReplaceAll(normalized, "年", "-")
	normalized = strings.ReplaceAll(normalized, "月", "-")
	normalized = strings.ReplaceAll(normalized, "日", "")
	normalized = strings.ReplaceAll(normalized, " ", "")

	return parseRiderISODate(normalized)
}

func parseRiderISODate(value string) (time.Time, error) {
	parts := strings.Split(value, "-")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid iso date: %s", value)
	}
	return time.Parse("2006-1-2", value)
}

func riderReviewReasonCode(outcome string, rejectReason string) string {
	trimmed := strings.TrimSpace(rejectReason)
	if outcome == "approved" {
		return "auto_approved"
	}
	switch {
	case strings.Contains(trimmed, "身份证已过期") || strings.Contains(trimmed, "健康证已过期"):
		return "rule_document_expired"
	case strings.Contains(trimmed, "姓名不一致"):
		return "rule_name_mismatch"
	case strings.Contains(trimmed, "有效期需超过当日7天"):
		return "rule_health_cert_too_soon"
	case strings.Contains(trimmed, "未识别"):
		return "readiness_required_field_missing"
	case strings.Contains(trimmed, "解析失败") || strings.Contains(trimmed, "格式无法识别"):
		return "readiness_field_unparseable"
	case strings.Contains(trimmed, "重新上传"):
		return "needs_resubmit"
	default:
		return "legacy_rejected"
	}
}

func riderReviewRuleHits(outcome string, reasonCode string) []string {
	if outcome == "approved" {
		return []string{"rider.auto_approve"}
	}
	if reasonCode == "" {
		return nil
	}
	return []string{"rider." + reasonCode}
}

func riderReviewOCRJobRefs(application db.RiderApplication, idCardOCR riderReviewIDCardOCRData) []int64 {
	refs := make([]int64, 0, 2)
	if idCardOCR.OCRJobID != nil {
		refs = append(refs, *idCardOCR.OCRJobID)
	} else if decodedIDCardOCR, err := decodeRiderReviewIDCardOCRData(application.IDCardOcr); err == nil && decodedIDCardOCR != nil && decodedIDCardOCR.OCRJobID != nil {
		refs = append(refs, *decodedIDCardOCR.OCRJobID)
	}
	if decodedHealthOCR, err := decodeRiderReviewHealthCertOCRData(application.HealthCertOcr); err == nil && decodedHealthOCR != nil && decodedHealthOCR.OCRJobID != nil {
		refs = append(refs, *decodedHealthOCR.OCRJobID)
	}
	return dedupeInt64s(refs)
}

func buildRiderCredentialActivationInputs(application db.RiderApplication) ([]CredentialActivationInput, error) {
	decodedHealthCert, err := decodeRiderReviewHealthCertOCRData(application.HealthCertOcr)
	if err != nil || decodedHealthCert == nil {
		return nil, fmt.Errorf("health_cert payload missing")
	}
	if !application.HealthCertMediaAssetID.Valid {
		return nil, fmt.Errorf("health_cert media missing")
	}
	healthCertExpiry, err := parseRiderCredentialExpiry(decodedHealthCert.ValidEnd)
	if err != nil {
		return nil, fmt.Errorf("parse health cert expiry: %w", err)
	}
	return []CredentialActivationInput{{
		DocumentType: db.CredentialDocumentTypeHealthCert,
		MediaAssetID: application.HealthCertMediaAssetID.Int64,
		ExpiresAt:    healthCertExpiry,
		NormalizedPayload: map[string]any{
			"name":        strings.TrimSpace(decodedHealthCert.Name),
			"id_number":   strings.TrimSpace(decodedHealthCert.IDNumber),
			"cert_number": strings.TrimSpace(decodedHealthCert.CertNumber),
			"valid_start": strings.TrimSpace(decodedHealthCert.ValidStart),
			"valid_end":   strings.TrimSpace(decodedHealthCert.ValidEnd),
		},
	}}, nil
}

func parseRiderCredentialExpiry(raw string) (*time.Time, error) {
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

func onboardingReviewRunID(run *db.OnboardingReviewRun) *int64 {
	if run == nil {
		return nil
	}
	return &run.ID
}
