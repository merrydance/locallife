package worker_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustMarshalJSON(t *testing.T, v any) []byte {
	t.Helper()
	payload, err := json.Marshal(v)
	require.NoError(t, err)
	return payload
}
