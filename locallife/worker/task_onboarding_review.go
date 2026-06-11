package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

const (
	TaskOnboardingReview                    = "onboarding:review"
	onboardingReviewApplicationTypeMerchant = "merchant"
	onboardingReviewApplicationTypeRider    = "rider"
)

type OnboardingReviewPayload struct {
	ReviewRunID     int64  `json:"review_run_id"`
	ApplicationID   int64  `json:"application_id"`
	ApplicationType string `json:"application_type"`
	RequestedBy     int64  `json:"requested_by,omitempty"`
}

func (distributor *RedisTaskDistributor) DistributeTaskOnboardingReview(
	ctx context.Context,
	payload *OnboardingReviewPayload,
	opts ...asynq.Option,
) error {
	if payload == nil {
		return fmt.Errorf("onboarding review payload is required")
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal onboarding review payload: %w", err)
	}
	return distributor.enqueue(ctx, TaskOnboardingReview, jsonPayload, opts...)
}

func (processor *RedisTaskProcessor) ProcessTaskOnboardingReview(ctx context.Context, task *asynq.Task) error {
	var payload OnboardingReviewPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal onboarding review payload: %w", asynq.SkipRetry)
	}
	if payload.ReviewRunID <= 0 || payload.ApplicationID <= 0 {
		return fmt.Errorf("invalid onboarding review payload: %w", asynq.SkipRetry)
	}

	run, err := processor.store.GetOnboardingReviewRun(ctx, payload.ReviewRunID)
	if err != nil {
		return fmt.Errorf("get onboarding review run: %w", err)
	}
	if err := validateOnboardingReviewPayload(run, payload); err != nil {
		return fmt.Errorf("onboarding review payload does not match review run: %v: %w", err, asynq.SkipRetry)
	}
	if shouldSkipOnboardingReviewRun(run) {
		return nil
	}

	switch payload.ApplicationType {
	case onboardingReviewApplicationTypeMerchant:
		if processor.merchantReviewSvc == nil {
			return fmt.Errorf("merchant onboarding review service not configured: %w", asynq.SkipRetry)
		}
		result, err := processor.merchantReviewSvc.ProcessApplication(ctx, payload.ApplicationID, payload.RequestedBy, &payload.ReviewRunID)
		if err != nil {
			return fmt.Errorf("process merchant onboarding review: %w", err)
		}
		if result.RestoreReleased && result.Merchant != nil {
			if err := processor.distributeMerchantCredentialRestoreNotification(ctx, *result.Merchant, result.Application, result.ReviewRun, result.CredentialEntries); err != nil {
				return fmt.Errorf("distribute merchant credential restore notification: %w", err)
			}
		}
		return nil
	case onboardingReviewApplicationTypeRider:
		if processor.riderReviewSvc == nil {
			return fmt.Errorf("rider onboarding review service not configured: %w", asynq.SkipRetry)
		}
		result, err := processor.riderReviewSvc.ProcessApplication(ctx, payload.ApplicationID, payload.RequestedBy, &payload.ReviewRunID)
		if err != nil {
			return fmt.Errorf("process rider onboarding review: %w", err)
		}
		if result.RestoreReleased && result.Rider != nil {
			if err := processor.distributeRiderCredentialRestoreNotification(ctx, *result.Rider, result.Application, result.ReviewRun, result.CredentialEntries); err != nil {
				return fmt.Errorf("distribute rider credential restore notification: %w", err)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported onboarding review application type %q: %w", payload.ApplicationType, asynq.SkipRetry)
	}
}

func validateOnboardingReviewPayload(run db.OnboardingReviewRun, payload OnboardingReviewPayload) error {
	if run.ApplicationType != payload.ApplicationType {
		return fmt.Errorf("application_type mismatch: payload=%s run=%s", payload.ApplicationType, run.ApplicationType)
	}

	switch payload.ApplicationType {
	case onboardingReviewApplicationTypeMerchant:
		if !run.MerchantApplicationID.Valid || run.MerchantApplicationID.Int64 != payload.ApplicationID {
			return fmt.Errorf("merchant application mismatch: payload=%d run=%d", payload.ApplicationID, run.MerchantApplicationID.Int64)
		}
	case onboardingReviewApplicationTypeRider:
		if !run.RiderApplicationID.Valid || run.RiderApplicationID.Int64 != payload.ApplicationID {
			return fmt.Errorf("rider application mismatch: payload=%d run=%d", payload.ApplicationID, run.RiderApplicationID.Int64)
		}
	default:
		return fmt.Errorf("unsupported application type: %s", payload.ApplicationType)
	}

	return nil
}

func shouldSkipOnboardingReviewRun(run db.OnboardingReviewRun) bool {
	switch run.RunStatus {
	case db.OnboardingReviewRunStatusCancelled:
		return true
	case db.OnboardingReviewRunStatusCompleted:
		return run.ApplicationType != onboardingReviewApplicationTypeMerchant
	default:
		return false
	}
}

func (processor *RedisTaskProcessor) distributeMerchantCredentialRestoreNotification(
	ctx context.Context,
	merchant db.Merchant,
	application db.MerchantApplication,
	reviewRun *db.OnboardingReviewRun,
	entries []logic.CredentialActivationInput,
) error {
	if processor == nil || processor.distributor == nil || merchant.OwnerUserID <= 0 || len(entries) == 0 {
		return nil
	}
	documentTypes := onboardingCredentialDocumentTypes(entries)
	title, content := onboardingCredentialRestoreNotificationText("merchant", documentTypes)
	extraData := map[string]any{
		"subject_type":           "merchant",
		"document_types":         documentTypes,
		"notification_source":    "credential_governance_restore",
		"outcome":                "restored",
		"application_id":         application.ID,
		"merchant_id":            merchant.ID,
		"suspension_reason_code": db.CredentialSuspensionReasonDocumentExpired,
	}
	if reviewRun != nil {
		extraData["review_run_id"] = reviewRun.ID
	}
	return processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
		UserID:            merchant.OwnerUserID,
		Type:              "system",
		Title:             title,
		Content:           content,
		RelatedType:       "merchant",
		RelatedID:         merchant.ID,
		ExtraData:         extraData,
		IgnorePreferences: true,
	})
}

func (processor *RedisTaskProcessor) distributeRiderCredentialRestoreNotification(
	ctx context.Context,
	rider db.Rider,
	application db.RiderApplication,
	reviewRun *db.OnboardingReviewRun,
	entries []logic.CredentialActivationInput,
) error {
	if processor == nil || processor.distributor == nil || rider.UserID <= 0 || len(entries) == 0 {
		return nil
	}
	documentTypes := onboardingCredentialDocumentTypes(entries)
	title, content := onboardingCredentialRestoreNotificationText("rider", documentTypes)
	extraData := map[string]any{
		"subject_type":           "rider",
		"document_types":         documentTypes,
		"notification_source":    "credential_governance_restore",
		"outcome":                "restored",
		"application_id":         application.ID,
		"rider_id":               rider.ID,
		"suspension_reason_code": db.CredentialSuspensionReasonDocumentExpired,
	}
	if reviewRun != nil {
		extraData["review_run_id"] = reviewRun.ID
	}
	return processor.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
		UserID:            rider.UserID,
		Type:              "system",
		Title:             title,
		Content:           content,
		RelatedType:       "rider",
		RelatedID:         rider.ID,
		ExtraData:         extraData,
		IgnorePreferences: true,
	})
}

func onboardingCredentialDocumentTypes(entries []logic.CredentialActivationInput) []string {
	seen := make(map[string]struct{}, len(entries))
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.DocumentType == "" {
			continue
		}
		if _, ok := seen[entry.DocumentType]; ok {
			continue
		}
		seen[entry.DocumentType] = struct{}{}
		result = append(result, entry.DocumentType)
	}
	return result
}

func onboardingCredentialRestoreNotificationText(subjectType string, documentTypes []string) (string, string) {
	labels := make([]string, 0, len(documentTypes))
	for _, documentType := range documentTypes {
		labels = append(labels, onboardingCredentialDocumentLabel(documentType))
	}
	joinedLabels := strings.Join(labels, "、")
	if joinedLabels == "" {
		joinedLabels = "资质证照"
	}
	if subjectType == "merchant" {
		return fmt.Sprintf("%s复审已通过，外卖经营已恢复", joinedLabels),
			fmt.Sprintf("您提交的%s已通过复审，系统已恢复外卖经营。", joinedLabels)
	}
	return fmt.Sprintf("%s复审已通过，接单资格已恢复", joinedLabels),
		fmt.Sprintf("您提交的%s已通过复审，系统已恢复接单资格。", joinedLabels)
}

func onboardingCredentialDocumentLabel(documentType string) string {
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
