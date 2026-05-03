package contracts

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountResultNormalizesSharingMerIDFromContractNo(t *testing.T) {
	result := AccountResult{ContractNo: "CP123", Raw: json.RawMessage(`{"status":"1"}`)}

	normalized := result.Normalized()

	require.Equal(t, "CP123", normalized.SharingMerID)
	require.Equal(t, json.RawMessage(`{"status":"1"}`), normalized.Raw)
}

func TestOpenStateFromUpstream(t *testing.T) {
	require.Equal(t, OpenStateActive, OpenStateFromUpstream("1"))
	require.Equal(t, OpenStateFailed, OpenStateFromUpstream("0"))
	require.Equal(t, OpenStateAbnormal, OpenStateFromUpstream("-1"))
	require.Equal(t, OpenStateProcessing, OpenStateFromUpstream("2"))
	require.Equal(t, OpenStateAbnormal, OpenStateFromUpstream("unexpected"))
}
