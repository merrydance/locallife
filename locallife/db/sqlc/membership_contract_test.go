package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMembershipBalanceDirectWritersRetired(t *testing.T) {
	files := []string{
		filepath.Join("..", "query", "membership.sql"),
		"membership.sql.go",
		"querier.go",
		filepath.Join("..", "mock", "store.go"),
	}
	forbidden := []string{
		"IncrementMembershipBalance",
		"DecrementMembershipBalance",
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		require.NoError(t, err)

		for _, name := range forbidden {
			require.NotContainsf(t, string(content), name, "%s must not expose direct membership balance writer %s; use transaction helpers that preserve ledger and idempotency semantics", file, name)
		}
	}
}
