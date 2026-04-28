package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

const (
	applymentFactBusinessObjectApplyment = "ecommerce_applyment"
	applymentFactConsumerDomain          = "applyment_domain"
)

func RecordApplymentActivatedCallbackFact(
	ctx context.Context,
	store db.Store,
	applyment db.EcommerceApplyment,
	resource any,
	notificationID string,
	eventType string,
	subMchID string,
) (*db.ExternalPaymentFactApplication, error) {
	upstreamApplymentID, rawResource := applymentCallbackFactResource(applyment, resource)
	return recordApplymentStatusFact(ctx, store, applyment.OutRequestNo, upstreamApplymentID, applyment.ID, "FINISH", strings.TrimSpace(subMchID), db.ExternalPaymentFactSourceCallback, notificationID, eventType, rawResource)
}

func RecordApplymentTerminalCallbackFact(
	ctx context.Context,
	store db.Store,
	applyment db.EcommerceApplyment,
	resource any,
	notificationID string,
	eventType string,
	applymentState string,
	subMchID string,
) (*db.ExternalPaymentFactApplication, error) {
	upstreamApplymentID, rawResource := applymentCallbackFactResource(applyment, resource)
	return recordApplymentStatusFact(ctx, store, applyment.OutRequestNo, upstreamApplymentID, applyment.ID, applymentState, strings.TrimSpace(subMchID), db.ExternalPaymentFactSourceCallback, notificationID, eventType, rawResource)
}

func RecordApplymentPendingCallbackFact(
	ctx context.Context,
	store db.Store,
	applyment db.EcommerceApplyment,
	resource any,
	notificationID string,
	eventType string,
	applymentState string,
	subMchID string,
) (*db.ExternalPaymentFactApplication, error) {
	upstreamApplymentID, rawResource := applymentCallbackFactResource(applyment, resource)
	return recordApplymentStatusFact(ctx, store, applyment.OutRequestNo, upstreamApplymentID, applyment.ID, applymentState, strings.TrimSpace(subMchID), db.ExternalPaymentFactSourceCallback, notificationID, eventType, rawResource)
}

func recordApplymentStatusFact(
	ctx context.Context,
	store db.Store,
	outRequestNo string,
	upstreamApplymentID string,
	applymentID int64,
	applymentState string,
	subMchID string,
	factSource string,
	sourceEventID string,
	sourceEventType string,
	rawResource []byte,
) (*db.ExternalPaymentFactApplication, error) {
	service := logic.NewPaymentFactService(store)
	terminalStatus := applymentFactTerminalStatus(applymentState)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:                    db.ExternalPaymentProviderWechat,
		Channel:                     db.PaymentChannelEcommerce,
		Capability:                  db.ExternalPaymentCapabilityApplyment,
		FactSource:                  factSource,
		SourceEventID:               applymentFactStringPtrIfNotEmpty(sourceEventID),
		SourceEventType:             applymentFactStringPtrIfNotEmpty(sourceEventType),
		ExternalObjectType:          db.ExternalPaymentObjectApplyment,
		ExternalObjectKey:           outRequestNo,
		ExternalSecondaryKey:        applymentFactStringPtrIfNotEmpty(upstreamApplymentID),
		BusinessOwner:               orderPaymentStringPtr(db.ExternalPaymentBusinessOwnerApplyment),
		BusinessObjectType:          orderPaymentStringPtr(applymentFactBusinessObjectApplyment),
		BusinessObjectID:            orderPaymentInt64Ptr(applymentID),
		UpstreamState:               strings.TrimSpace(applymentState),
		TerminalStatus:              terminalStatus,
		Currency:                    "CNY",
		RawResource:                 rawResource,
		DedupeKey:                   applymentTerminalFactDedupeKey(factSource, outRequestNo, applymentState, subMchID, sourceEventID),
		AllowNonTerminalApplication: terminalStatus == db.ExternalPaymentTerminalStatusProcessing,
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           applymentFactConsumerDomain,
			BusinessObjectType: applymentFactBusinessObjectApplyment,
			BusinessObjectID:   applymentID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func recordApplymentActivatedQueryFact(ctx context.Context, store db.Store, applyment db.EcommerceApplymentPendingFollowUp, queryResp *wechatcontracts.EcommerceApplymentQueryResponse) (*db.ExternalPaymentFactApplication, error) {
	rawResource := applymentQueryFactResource(applyment, queryResp)
	upstreamApplymentID := ""
	if queryResp != nil && queryResp.ApplymentID > 0 {
		upstreamApplymentID = fmt.Sprintf("%d", queryResp.ApplymentID)
	}
	return recordApplymentStatusFact(ctx, store, applyment.OutRequestNo, upstreamApplymentID, applyment.ID, "FINISH", strings.TrimSpace(queryResp.SubMchID), db.ExternalPaymentFactSourceQuery, "", "", rawResource)
}

func recordApplymentTerminalQueryFact(ctx context.Context, store db.Store, applyment db.EcommerceApplymentPendingFollowUp, queryResp *wechatcontracts.EcommerceApplymentQueryResponse) (*db.ExternalPaymentFactApplication, error) {
	rawResource := applymentQueryFactResource(applyment, queryResp)
	upstreamApplymentID := ""
	if queryResp != nil && queryResp.ApplymentID > 0 {
		upstreamApplymentID = fmt.Sprintf("%d", queryResp.ApplymentID)
	}
	return recordApplymentStatusFact(ctx, store, applyment.OutRequestNo, upstreamApplymentID, applyment.ID, queryResp.ApplymentState, strings.TrimSpace(queryResp.SubMchID), db.ExternalPaymentFactSourceQuery, "", "", rawResource)
}

func recordApplymentPendingQueryFact(ctx context.Context, store db.Store, applyment db.EcommerceApplymentPendingFollowUp, queryResp *wechatcontracts.EcommerceApplymentQueryResponse) (*db.ExternalPaymentFactApplication, error) {
	rawResource := applymentQueryFactResource(applyment, queryResp)
	upstreamApplymentID := ""
	if queryResp != nil && queryResp.ApplymentID > 0 {
		upstreamApplymentID = fmt.Sprintf("%d", queryResp.ApplymentID)
	}
	return recordApplymentStatusFact(ctx, store, applyment.OutRequestNo, upstreamApplymentID, applyment.ID, queryResp.ApplymentState, strings.TrimSpace(queryResp.SubMchID), db.ExternalPaymentFactSourceQuery, "", "", rawResource)
}

func EnqueueApplymentPaymentFactApplication(ctx context.Context, distributor any, application *db.ExternalPaymentFactApplication) error {
	if application == nil {
		return nil
	}
	applicationDistributor, ok := distributor.(PaymentFactApplicationTaskDistributor)
	if !ok {
		return fmt.Errorf("payment fact application distributor not configured")
	}
	return applicationDistributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	)
}

func applymentFactStringPtrIfNotEmpty(value string) *string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return nil
	}
	return &trimmedValue
}

func applymentCallbackFactResource(applyment db.EcommerceApplyment, resource any) (string, []byte) {
	upstreamApplymentID := ""
	switch typed := resource.(type) {
	case map[string]any:
		if value, ok := typed["applyment_id"]; ok {
			upstreamApplymentID = fmt.Sprintf("%v", value)
		}
	case struct{ ApplymentID int64 }:
		if typed.ApplymentID > 0 {
			upstreamApplymentID = fmt.Sprintf("%d", typed.ApplymentID)
		}
	}
	if upstreamApplymentID == "" && applyment.ApplymentID.Valid && applyment.ApplymentID.Int64 > 0 {
		upstreamApplymentID = fmt.Sprintf("%d", applyment.ApplymentID.Int64)
	}
	applymentIDForPayload := any(applyment.ApplymentID.Int64)
	if strings.TrimSpace(upstreamApplymentID) != "" {
		applymentIDForPayload = strings.TrimSpace(upstreamApplymentID)
	}
	resourceMap := map[string]any{
		"applyment_id":    applymentIDForPayload,
		"local_applyment": applyment.ID,
		"out_request_no":  applyment.OutRequestNo,
		"subject_type":    applyment.SubjectType,
		"subject_id":      applyment.SubjectID,
		"resource":        resource,
	}
	raw, err := json.Marshal(resourceMap)
	if err != nil {
		return upstreamApplymentID, nil
	}
	return upstreamApplymentID, raw
}

func applymentQueryFactResource(applyment db.EcommerceApplymentPendingFollowUp, queryResp *wechatcontracts.EcommerceApplymentQueryResponse) []byte {
	payload := map[string]any{
		"local_applyment_id": applyment.ID,
		"out_request_no":     applyment.OutRequestNo,
		"subject_type":       applyment.SubjectType,
		"subject_id":         applyment.SubjectID,
		"source":             "applyment_recovery_query",
	}
	if queryResp != nil {
		payload["applyment_id"] = queryResp.ApplymentID
		payload["applyment_state"] = queryResp.ApplymentState
		payload["applyment_state_desc"] = queryResp.ApplymentStateDesc
		payload["reject_reason"] = getRejectReasonFromApplymentAuditDetail(queryResp.AuditDetail).String
		payload["sign_state"] = queryResp.SignState
		payload["sub_mch_id"] = queryResp.SubMchID
		if queryResp.SignURL != "" {
			payload["sign_url"] = queryResp.SignURL
		}
		if queryResp.LegalValidationURL != "" {
			payload["legal_validation_url"] = queryResp.LegalValidationURL
		}
		if queryResp.AccountValidation != nil {
			payload["account_validation"] = queryResp.AccountValidation
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return raw
}

func applymentFactTerminalStatus(applymentState string) string {
	switch logic.MapWechatApplymentStateToStatus(applymentState) {
	case "finish":
		return db.ExternalPaymentTerminalStatusSuccess
	case "account_need_verify", "to_be_confirmed", "to_be_signed":
		return db.ExternalPaymentTerminalStatusProcessing
	case "rejected":
		return db.ExternalPaymentTerminalStatusFailed
	case "frozen", "canceled":
		return db.ExternalPaymentTerminalStatusClosed
	default:
		return db.ExternalPaymentTerminalStatusUnknown
	}
}

func applymentTerminalFactDedupeKey(factSource, outRequestNo, applymentState, subMchID, sourceEventID string) string {
	normalizedState := strings.ToLower(strings.TrimSpace(applymentState))
	if sourceEventID != "" {
		return fmt.Sprintf("wechat:%s:applyment:%s:%s:%s", factSource, sourceEventID, normalizedState, strings.TrimSpace(subMchID))
	}
	return fmt.Sprintf("wechat:%s:ecommerce:applyment:%s:%s:%s", factSource, outRequestNo, normalizedState, strings.TrimSpace(subMchID))
}
