package baofu

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnionGWVerifyType1ContentRoundTrip(t *testing.T) {
	privatePEM, publicPEM := generateBaofuTestKeyPair(t)
	envelope, err := NewUnionGWRequestEnvelope("102004465", "200005200", "T-1001-013-06", map[string]string{"contractNo": "CM202605040001"})
	require.NoError(t, err)
	plaintext, err := CanonicalJSON(envelope)
	require.NoError(t, err)

	content, err := EncodeUnionGWVerifyType1Content(privatePEM, plaintext)
	require.NoError(t, err)
	require.NotContains(t, content, "CM202605040001")

	decoded, err := DecodeUnionGWVerifyType1Content(publicPEM, content)
	require.NoError(t, err)
	var decodedEnvelope UnionGWPlaintextEnvelope
	require.NoError(t, json.Unmarshal(decoded, &decodedEnvelope))
	require.Equal(t, "T-1001-013-06", decodedEnvelope.Header.ServiceType)
	require.JSONEq(t, `{"contractNo":"CM202605040001"}`, string(decodedEnvelope.Body))
}

func TestUnionGWResponseValidationRejectsMismatchedServiceType(t *testing.T) {
	envelope := UnionGWPlaintextEnvelope{
		Header: UnionGWHeader{
			MemberID:       "102004465",
			TerminalID:     "200005200",
			ServiceType:    "T-1001-013-03",
			SystemRespCode: UnionGWSystemRespSuccess,
		},
		Body: json.RawMessage(`{"retCode":"SUCCESS"}`),
	}

	err := envelope.ValidateResponse("102004465", "200005200", "T-1001-013-06")

	require.EqualError(t, err, "baofu union-gw response serviceTp mismatch")
}
