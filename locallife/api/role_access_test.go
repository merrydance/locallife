package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildRoleAccessEntriesIncludesPlatformTrafficSummary(t *testing.T) {
	entries, err := buildRoleAccessEntries()
	require.NoError(t, err)

	var found bool
	for _, entry := range entries {
		if entry.PathPrefix == "/v1/platform/stats/traffic/summary" {
			found = true
			require.True(t, entry.AuthRequired)
			require.Contains(t, entry.Roles, "admin")
			require.Equal(t, "traffic snapshot", entry.Notes)
		}
	}

	require.True(t, found, "expected traffic summary path in role access metadata")
}
