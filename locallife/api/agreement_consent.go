package api

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type agreementConsentRequest struct {
	UserAgreementVersion string `json:"user_agreement_version"`
	PrivacyPolicyVersion string `json:"privacy_policy_version"`
	ConsentedAt          string `json:"consented_at"`
}

func (r agreementConsentRequest) isEmpty() bool {
	return strings.TrimSpace(r.UserAgreementVersion) == "" && strings.TrimSpace(r.PrivacyPolicyVersion) == "" && strings.TrimSpace(r.ConsentedAt) == ""
}

func (r agreementConsentRequest) validate() error {
	if strings.TrimSpace(r.UserAgreementVersion) == "" {
		return ErrUserAgreementVersionRequired
	}
	if strings.TrimSpace(r.PrivacyPolicyVersion) == "" {
		return ErrPrivacyPolicyVersionRequired
	}
	if strings.TrimSpace(r.ConsentedAt) == "" {
		return ErrAgreementConsentedAtRequired
	}
	if _, err := time.Parse(time.RFC3339, r.ConsentedAt); err != nil {
		return ErrAgreementConsentedAtInvalid
	}
	return nil
}

func parseAgreementConsentRequest(ctx *gin.Context) (*agreementConsentRequest, error) {
	rawBody, err := ctx.GetRawData()
	if err != nil {
		return nil, nil
	}
	if len(strings.TrimSpace(string(rawBody))) == 0 {
		return nil, nil
	}

	var req agreementConsentRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return nil, err
	}

	if req.isEmpty() {
		return nil, nil
	}

	if err := req.validate(); err != nil {
		return nil, err
	}

	return &req, nil
}

func (server *Server) writeAgreementConsentAudit(ctx *gin.Context, actorUserID int64, action string, targetType string, targetID int64, consent *agreementConsentRequest, extraMetadata map[string]any) {
	if consent == nil {
		return
	}

	metadata := map[string]any{
		"user_agreement_version": consent.UserAgreementVersion,
		"privacy_policy_version": consent.PrivacyPolicyVersion,
		"consented_at":           consent.ConsentedAt,
		"source":                 "weapp_submit",
	}
	for key, value := range extraMetadata {
		metadata[key] = value
	}

	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: actorUserID,
		ActorRole:   RoleCustomer,
		Action:      action,
		TargetType:  targetType,
		TargetID:    &targetID,
		Metadata:    metadata,
	})
}
