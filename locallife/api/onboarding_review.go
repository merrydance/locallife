package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/rs/zerolog/log"
)

type onboardingReviewSummaryResponse struct {
	RunID         int64    `json:"run_id"`
	Stage         string   `json:"stage"`
	Outcome       string   `json:"outcome"`
	ReasonCode    string   `json:"reason_code"`
	ReasonMessage string   `json:"reason_message,omitempty"`
	RuleHits      []string `json:"rule_hits,omitempty"`
	OCRJobRefs    []int64  `json:"ocr_job_refs,omitempty"`
	CreatedAt     string   `json:"created_at"`
}

type activeCredentialSummaryResponse struct {
	DocumentType    string     `json:"document_type"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	DaysUntilExpiry *int       `json:"days_until_expiry,omitempty"`
	LastRemindedAt  *time.Time `json:"last_reminded_at,omitempty"`
	Suspended       bool       `json:"suspended"`
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	ResumedAt       *time.Time `json:"resumed_at,omitempty"`
}

func (server *Server) recordMerchantBlockedReview(ctx context.Context, app db.MerchantApplication, requestedBy int64, reviewErr error) {
	if server.onboardingReviewService == nil || reviewErr == nil {
		return
	}
	reasonMessage := strings.TrimSpace(reviewErr.Error())
	reasonCode := mapMerchantReviewReasonCode(reasonMessage)
	decision := logic.OnboardingReviewDecision{
		Outcome:       merchantBlockedReviewOutcome(reasonCode),
		ReasonCode:    reasonCode,
		ReasonMessage: reasonMessage,
		RuleHits:      []string{"merchant." + reasonCode},
		OCRJobRefs:    merchantApplicationOCRJobRefs(app),
		RequestedBy:   &requestedBy,
		Snapshot: map[string]any{
			"application_id":   app.ID,
			"application_type": "merchant",
			"status":           app.Status,
			"user_id":          app.UserID,
			"merchant_name":    app.MerchantName,
		},
	}
	if _, err := server.onboardingReviewService.RecordMerchantReview(ctx, app.ID, decision); err != nil {
		log.Error().Err(err).Int64("application_id", app.ID).Msg("record merchant blocked onboarding review failed")
	}
}

func merchantBlockedReviewOutcome(reasonCode string) string {
	switch {
	case strings.HasPrefix(reasonCode, "readiness_"):
		return "needs_resubmit"
	case strings.HasPrefix(reasonCode, "manual_"):
		return "needs_manual"
	case strings.HasPrefix(reasonCode, "rule_"), strings.HasPrefix(reasonCode, "risk_"):
		return "rejected"
	default:
		return "needs_resubmit"
	}
}

func credentialDocumentLabel(documentType string) string {
	switch documentType {
	case db.CredentialDocumentTypeBusinessLicense:
		return "营业执照"
	case db.CredentialDocumentTypeFoodPermit:
		return "食品经营许可证"
	case db.CredentialDocumentTypeHealthCert:
		return "健康证"
	default:
		return documentType
	}
}

func credentialDocumentTypes(entries []logic.CredentialActivationInput) []string {
	seen := make(map[string]struct{}, len(entries))
	documentTypes := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.DocumentType == "" {
			continue
		}
		if _, ok := seen[entry.DocumentType]; ok {
			continue
		}
		seen[entry.DocumentType] = struct{}{}
		documentTypes = append(documentTypes, entry.DocumentType)
	}
	return documentTypes
}

func credentialDocumentLabels(documentTypes []string) []string {
	labels := make([]string, 0, len(documentTypes))
	for _, documentType := range documentTypes {
		labels = append(labels, credentialDocumentLabel(documentType))
	}
	return labels
}

func credentialRestoreNotificationText(subjectType string, documentTypes []string) (string, string) {
	labels := strings.Join(credentialDocumentLabels(documentTypes), "、")
	if labels == "" {
		labels = "资质证照"
	}
	if subjectType == "merchant" {
		return fmt.Sprintf("%s复审已通过，外卖经营已恢复", labels),
			fmt.Sprintf("您提交的%s已通过复审，系统已恢复外卖经营。", labels)
	}
	return fmt.Sprintf("%s复审已通过，接单资格已恢复", labels),
		fmt.Sprintf("您提交的%s已通过复审，系统已恢复接单资格。", labels)
}

func (server *Server) credentialGovernanceNotificationUserID(ctx context.Context, subjectType string, subjectID int64) (int64, error) {
	switch subjectType {
	case "merchant":
		merchant, err := server.store.GetMerchant(ctx, subjectID)
		if err != nil {
			return 0, err
		}
		return merchant.OwnerUserID, nil
	case "rider":
		rider, err := server.store.GetRider(ctx, subjectID)
		if err != nil {
			return 0, err
		}
		return rider.UserID, nil
	default:
		return 0, fmt.Errorf("unsupported subject type: %s", subjectType)
	}
}

func (server *Server) notifyCredentialGovernanceRestored(
	ctx context.Context,
	subjectType string,
	subjectID int64,
	applicationID int64,
	reviewRun *db.OnboardingReviewRun,
	entries []logic.CredentialActivationInput,
) {
	if server == nil || server.store == nil {
		return
	}

	documentTypes := credentialDocumentTypes(entries)
	userID, err := server.credentialGovernanceNotificationUserID(ctx, subjectType, subjectID)
	if err != nil {
		log.Error().Err(err).Str("subject_type", subjectType).Int64("subject_id", subjectID).Msg("resolve credential restore notification user failed")
		return
	}

	title, content := credentialRestoreNotificationText(subjectType, documentTypes)
	extraData := map[string]any{
		"subject_type":           subjectType,
		"document_types":         documentTypes,
		"notification_source":    "credential_governance_restore",
		"outcome":                "restored",
		"application_id":         applicationID,
		"suspension_reason_code": db.CredentialSuspensionReasonDocumentExpired,
	}
	if reviewRun != nil {
		extraData["review_run_id"] = reviewRun.ID
	}
	switch subjectType {
	case "merchant":
		extraData["merchant_id"] = subjectID
	case "rider":
		extraData["rider_id"] = subjectID
	}

	if err := server.SendNotification(ctx, SendNotificationParams{
		UserID:            userID,
		Type:              "system",
		Title:             title,
		Content:           content,
		RelatedType:       subjectType,
		RelatedID:         subjectID,
		ExtraData:         extraData,
		IgnorePreferences: true,
	}); err != nil {
		log.Error().Err(err).Str("subject_type", subjectType).Int64("subject_id", subjectID).Int64("user_id", userID).Msg("send credential restore notification failed")
	}
}

func merchantApplicationOCRJobRefs(app db.MerchantApplication) []int64 {
	refs := make([]int64, 0, 4)
	if ocrData := decodeOCRJobCarrier[BusinessLicenseOCRData](app.BusinessLicenseOcr); ocrData != nil && ocrData.OCRJobID != nil {
		refs = append(refs, *ocrData.OCRJobID)
	}
	if ocrData := decodeOCRJobCarrier[FoodPermitOCRData](app.FoodPermitOcr); ocrData != nil && ocrData.OCRJobID != nil {
		refs = append(refs, *ocrData.OCRJobID)
	}
	if ocrData := decodeOCRJobCarrier[IDCardOCRData](app.IDCardFrontOcr); ocrData != nil && ocrData.OCRJobID != nil {
		refs = append(refs, *ocrData.OCRJobID)
	}
	if ocrData := decodeOCRJobCarrier[IDCardOCRData](app.IDCardBackOcr); ocrData != nil && ocrData.OCRJobID != nil {
		refs = append(refs, *ocrData.OCRJobID)
	}
	return refs
}

func riderApplicationOCRJobRefs(app db.RiderApplication, idCardOCR *IDCardOCRData) []int64 {
	refs := make([]int64, 0, 2)
	if idCardOCR != nil && idCardOCR.OCRJobID != nil {
		refs = append(refs, *idCardOCR.OCRJobID)
	} else if decoded, err := decodeIDCardOCRData(app.IDCardOcr); err == nil && decoded != nil && decoded.OCRJobID != nil {
		refs = append(refs, *decoded.OCRJobID)
	}
	if decoded, err := decodeHealthCertOCRData(app.HealthCertOcr); err == nil && decoded != nil && decoded.OCRJobID != nil {
		refs = append(refs, *decoded.OCRJobID)
	}
	return refs
}

func mapMerchantReviewReasonCode(reasonMessage string) string {
	trimmed := strings.TrimSpace(reasonMessage)
	switch {
	case strings.Contains(trimmed, "经营范围"):
		return "rule_non_catering_scope"
	case strings.Contains(trimmed, "地图定位") || strings.Contains(trimmed, "注册地址不一致"):
		return "rule_address_mismatch"
	case strings.Contains(trimmed, "主体名称") || strings.Contains(trimmed, "企业名称不一致"):
		return "rule_name_mismatch"
	case strings.Contains(trimmed, "已过期"):
		return "rule_document_expired"
	case strings.Contains(trimmed, "位置附近已有其他商户"):
		return "risk_duplicate_location"
	case strings.Contains(trimmed, "未识别"):
		return "readiness_required_field_missing"
	case strings.Contains(trimmed, "解析失败") || strings.Contains(trimmed, "无法识别"):
		return "readiness_field_unparseable"
	default:
		return "legacy_submission_blocked"
	}
}

func decodeOCRJobCarrier[T any](payload []byte) *T {
	if len(payload) == 0 {
		return nil
	}
	var result T
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil
	}
	return &result
}

func decodeOnboardingReviewSummary(payload []byte) *onboardingReviewSummaryResponse {
	if len(payload) == 0 {
		return nil
	}
	var summary onboardingReviewSummaryResponse
	if err := json.Unmarshal(payload, &summary); err != nil {
		return nil
	}
	if summary.RunID == 0 && summary.Outcome == "" && summary.ReasonCode == "" {
		return nil
	}
	return &summary
}

func onboardingReviewRunID(run *db.OnboardingReviewRun) *int64 {
	if run == nil {
		return nil
	}
	return &run.ID
}

func onboardingReviewSummaryPayload(run *db.OnboardingReviewRun) []byte {
	if run == nil {
		return nil
	}
	summary := onboardingReviewSummaryResponse{
		RunID:         run.ID,
		Stage:         run.Stage,
		Outcome:       run.Outcome.String,
		ReasonCode:    run.ReasonCode.String,
		ReasonMessage: run.ReasonMessage.String,
		RuleHits:      append([]string(nil), run.RuleHits...),
		OCRJobRefs:    append([]int64(nil), run.OcrJobRefs...),
		CreatedAt:     run.CreatedAt.UTC().Format(time.RFC3339),
	}
	payload, err := json.Marshal(summary)
	if err != nil {
		return nil
	}
	return payload
}

func attachMerchantReviewSummary(app *db.MerchantApplication, run *db.OnboardingReviewRun) {
	if app == nil {
		return
	}
	if payload := onboardingReviewSummaryPayload(run); len(payload) > 0 {
		app.ReviewSummary = payload
	}
}

func attachRiderReviewSummary(app *db.RiderApplication, run *db.OnboardingReviewRun) {
	if app == nil {
		return
	}
	if payload := onboardingReviewSummaryPayload(run); len(payload) > 0 {
		app.ReviewSummary = payload
	}
}

func (server *Server) loadMerchantActiveCredentialSummaries(ctx context.Context, ownerUserID int64) []activeCredentialSummaryResponse {
	if server.store == nil || server.credentialGovernanceService == nil {
		return nil
	}
	merchant, err := server.store.GetMerchantByOwner(ctx, ownerUserID)
	if err != nil {
		if !isNotFoundError(err) {
			log.Warn().Err(err).Int64("owner_user_id", ownerUserID).Msg("load merchant active credentials failed")
		}
		return nil
	}
	ledgers, err := server.store.GetActiveMerchantCredentialLedgers(ctx, pgtype.Int8{Int64: merchant.ID, Valid: true})
	if err != nil {
		log.Warn().Err(err).Int64("merchant_id", merchant.ID).Msg("list merchant active credentials failed")
		return nil
	}
	return buildActiveCredentialSummaries(ledgers, time.Now().UTC())
}

func (server *Server) loadRiderActiveCredentialSummaries(ctx context.Context, userID int64) []activeCredentialSummaryResponse {
	if server.store == nil || server.credentialGovernanceService == nil {
		return nil
	}
	rider, err := server.store.GetRiderByUserID(ctx, userID)
	if err != nil {
		if !isNotFoundError(err) {
			log.Warn().Err(err).Int64("user_id", userID).Msg("load rider active credentials failed")
		}
		return nil
	}
	ledgers, err := server.store.GetActiveRiderCredentialLedgers(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	if err != nil {
		log.Warn().Err(err).Int64("rider_id", rider.ID).Msg("list rider active credentials failed")
		return nil
	}
	return buildActiveCredentialSummaries(ledgers, time.Now().UTC())
}

func buildActiveCredentialSummaries(ledgers []db.CredentialLedger, now time.Time) []activeCredentialSummaryResponse {
	if len(ledgers) == 0 {
		return nil
	}
	result := make([]activeCredentialSummaryResponse, 0, len(ledgers))
	for _, ledger := range ledgers {
		summary := activeCredentialSummaryResponse{
			DocumentType:   ledger.DocumentType,
			LastRemindedAt: timestamptzPtr(ledger.LastRemindedAt),
			SuspendedAt:    timestamptzPtr(ledger.SuspendedAt),
			ResumedAt:      timestamptzPtr(ledger.ResumedAt),
			Suspended:      isCredentialSuspended(ledger),
		}
		if ledger.ExpiresAt.Valid {
			expiresAt := ledger.ExpiresAt.Time.UTC()
			summary.ExpiresAt = &expiresAt
			daysUntilExpiry := wholeDayDelta(expiresAt, now)
			summary.DaysUntilExpiry = &daysUntilExpiry
		}
		result = append(result, summary)
	}
	return result
}

func timestamptzPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func wholeDayDelta(expiresAt time.Time, now time.Time) int {
	expiryDate := time.Date(expiresAt.UTC().Year(), expiresAt.UTC().Month(), expiresAt.UTC().Day(), 0, 0, 0, 0, time.UTC)
	nowDate := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)
	return int(expiryDate.Sub(nowDate).Hours() / 24)
}

func isCredentialSuspended(ledger db.CredentialLedger) bool {
	if !ledger.SuspendedAt.Valid {
		return false
	}
	if !ledger.ResumedAt.Valid {
		return true
	}
	return ledger.SuspendedAt.Time.After(ledger.ResumedAt.Time)
}
