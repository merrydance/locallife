package worker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildOCRReadiness_ReadyAndPartial(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		readiness := buildOCRReadiness([]string{"enterprise_name", "valid_period"}, func(field string) bool {
			return field == "enterprise_name" || field == "valid_period"
		})

		require.NotNil(t, readiness)
		require.Equal(t, ocrReadinessStateReady, readiness.State)
		require.Equal(t, ocrReadinessReasonOK, readiness.ReasonCode)
		require.Equal(t, []string{"enterprise_name", "valid_period"}, readiness.RequiredFields)
		require.Empty(t, readiness.MissingFields)
	})

	t.Run("partial", func(t *testing.T) {
		readiness := buildOCRReadiness([]string{"enterprise_name", "valid_period"}, func(field string) bool {
			return field == "enterprise_name"
		})

		require.NotNil(t, readiness)
		require.Equal(t, ocrReadinessStatePartial, readiness.State)
		require.Equal(t, ocrReadinessReasonRequiredFieldMissing, readiness.ReasonCode)
		require.Equal(t, []string{"enterprise_name", "valid_period"}, readiness.RequiredFields)
		require.Equal(t, []string{"valid_period"}, readiness.MissingFields)
	})
}

func TestFailedOCRReadiness_DefaultsProviderError(t *testing.T) {
	readiness := failedOCRReadiness("")

	require.NotNil(t, readiness)
	require.Equal(t, ocrReadinessStateProviderFailed, readiness.State)
	require.Equal(t, ocrReadinessReasonProviderError, readiness.ReasonCode)
}
