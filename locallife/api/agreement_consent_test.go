package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	mockdb "github.com/merrydance/locallife/db/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestWriteAgreementConsentAuditExtraMetadataIsExplicit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	auditWriter := &auditSpyWriter{}
	server.auditWriter = auditWriter
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	consent := &agreementConsentRequest{
		UserAgreementVersion: "user-v1",
		PrivacyPolicyVersion: "privacy-v1",
		ConsentedAt:          "2026-06-10T12:00:00Z",
	}

	server.writeAgreementConsentAudit(ctx, 1001, "operator_application_consent_confirmed", "operator_application", 2001, consent, nil)
	server.writeAgreementConsentAudit(ctx, 1001, "merchant_application_submit_attempt_consent_confirmed", "merchant_application", 2002, consent, map[string]any{
		"event_scope": "submit_attempt",
	})

	entries := auditWriter.Entries()
	require.Len(t, entries, 2)
	require.Equal(t, "operator_application_consent_confirmed", entries[0].Action)
	require.NotContains(t, entries[0].Metadata, "event_scope")
	require.Equal(t, "merchant_application_submit_attempt_consent_confirmed", entries[1].Action)
	require.Equal(t, "submit_attempt", entries[1].Metadata["event_scope"])
}
