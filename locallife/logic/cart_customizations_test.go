package logic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshalCustomizationsCanonical_SortsKeys(t *testing.T) {
	input := map[string]interface{}{
		"b": 1,
		"a": 2,
		"c": map[string]interface{}{
			"y": "yes",
			"x": "no",
		},
	}

	out, err := MarshalCustomizationsCanonical(input)
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &decoded))

	out2, err := MarshalCustomizationsCanonical(input)
	require.NoError(t, err)
	require.Equal(t, string(out), string(out2))
}
