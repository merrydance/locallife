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
	require.Contains(t, string(raw), `"bizContent":"{\"outTradeNo\":\"BF1\"}"`)
}

func TestPublicEnvelopeFormValuesUseOfficialWireFormat(t *testing.T) {
	env := validPublicEnvelopeForTest()

	values := env.FormValues()

	require.Equal(t, "100000", values.Get("merId"))
	require.Equal(t, "200000", values.Get("terId"))
	require.Equal(t, "unified_order", values.Get("method"))
	require.Equal(t, "json", values.Get("format"))
	require.Equal(t, `{"outTradeNo":"BF1"}`, values.Get("bizContent"))
	require.Equal(t, "encrypted-key", values.Get("dgtlEnvlp"))
}

func TestPublicResponseEnvelopeAcceptsStringOrObjectBizContent(t *testing.T) {
	var fromString PublicResponseEnvelope
	require.NoError(t, json.Unmarshal([]byte(`{"returnCode":"FAIL","returnMsg":"参数错误","bizContent":"{\"errCode\":\"INVALID_PARAMETER\"}"}`), &fromString))
	require.JSONEq(t, `{"errCode":"INVALID_PARAMETER"}`, string(fromString.BizContent))

	var fromObject PublicResponseEnvelope
	require.NoError(t, json.Unmarshal([]byte(`{"returnCode":"FAIL","returnMsg":"参数错误","bizContent":{"errCode":"INVALID_PARAMETER"}}`), &fromObject))
	require.JSONEq(t, `{"errCode":"INVALID_PARAMETER"}`, string(fromObject.BizContent))
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
		BizContent:         JSONString(`{"resultCode":"SUCCESS"}`),
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
		BizContent:         JSONString(`{"outTradeNo":"BF1"}`),
	}
}
