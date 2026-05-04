package baofu

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicEnvelopeValidateRequiresOfficialFields(t *testing.T) {
	env := validPublicEnvelopeForTest()

	require.NoError(t, env.Validate())

	env.Method = ""
	require.EqualError(t, env.Validate(), "baofu public envelope method is required")
}

func TestPublicEnvelopeRejectsInvalidFixedValues(t *testing.T) {
	env := validPublicEnvelopeForTest()
	env.Charset = "GBK"
	require.EqualError(t, env.Validate(), "baofu public envelope charset must be UTF-8")

	env = validPublicEnvelopeForTest()
	env.Format = "xml"
	require.EqualError(t, env.Validate(), "baofu public envelope format must be json")

	env = validPublicEnvelopeForTest()
	env.SignType = "MD5"
	require.EqualError(t, env.Validate(), "baofu public envelope signType is unsupported")
}

func TestPublicEnvelopeCanonicalJSONUsesOfficialFieldNames(t *testing.T) {
	env := validPublicEnvelopeForTest()

	raw, err := CanonicalJSON(env)

	require.NoError(t, err)
	require.Contains(t, string(raw), `"merId":"100000"`)
	require.Contains(t, string(raw), `"terId":"200000"`)
	require.Contains(t, string(raw), `"method":"unified_order"`)
	require.Contains(t, string(raw), `"bizContent":{"outTradeNo":"BF1"}`)
}

func TestPublicResponseEnvelopeValidateHandlesTransportStatus(t *testing.T) {
	env := PublicResponseEnvelope{
		ReturnCode:         PublicEnvelopeReturnCodeSuccess,
		ReturnMessage:      "OK",
		MerchantID:         "100000",
		TerminalID:         "200000",
		Charset:            PublicEnvelopeCharsetUTF8,
		Version:            PublicEnvelopeVersion10,
		Format:             PublicEnvelopeFormatJSON,
		SignType:           SignTypeSM2,
		SignSerialNo:       "1",
		EncryptionSerialNo: "1",
		SignString:         "abcd",
		BizContent:         json.RawMessage(`{"resultCode":"SUCCESS"}`),
	}

	require.NoError(t, env.Validate())

	env.ReturnCode = ""
	require.EqualError(t, env.Validate(), "baofu public response returnCode is required")
}

func validPublicEnvelopeForTest() PublicRequestEnvelope {
	return PublicRequestEnvelope{
		MerchantID:         "100000",
		TerminalID:         "200000",
		Method:             "unified_order",
		Charset:            PublicEnvelopeCharsetUTF8,
		Version:            PublicEnvelopeVersion10,
		Format:             PublicEnvelopeFormatJSON,
		Timestamp:          "20260504120000",
		SignType:           SignTypeSM2,
		SignSerialNo:       "1",
		EncryptionSerialNo: "1",
		DigitalEnvelope:    "encrypted-key",
		SignString:         "abcd",
		BizContent:         json.RawMessage(`{"outTradeNo":"BF1"}`),
	}
}
