package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type apiTestEnvelope struct {
	Code    *int            `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func unwrapAPIResponseData(_ *testing.T, body []byte) []byte {
	// If wrapped: {code,message,data}, return data (may be empty for some errors)
	var env apiTestEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Code != nil {
		if len(env.Data) == 0 {
			return []byte("null")
		}
		return env.Data
	}
	// Not wrapped: return original
	return body
}

func requireUnmarshalAPIResponseData(t *testing.T, body []byte, target any) {
	data := unwrapAPIResponseData(t, body)
	err := json.Unmarshal(data, target)
	require.NoError(t, err)
}
