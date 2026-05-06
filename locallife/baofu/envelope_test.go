package baofu

import (
	"encoding/json"
	"strings"
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

func TestPublicEnvelopeRejectsSerialsLongerThanOfficialS10(t *testing.T) {
	env := validPublicEnvelopeForTest()
	env.SignSerialNo = "12345678901"
	require.EqualError(t, env.Validate(), "baofu public envelope signSn must be at most 10 characters")

	env = validPublicEnvelopeForTest()
	env.EncryptionSerialNo = "12345678901"
	require.EqualError(t, env.Validate(), "baofu public envelope ncrptnSn must be at most 10 characters")
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

func TestPublicResponseEnvelopeAcceptsOfficialDataContent(t *testing.T) {
	var fromString PublicResponseEnvelope
	require.NoError(t, json.Unmarshal([]byte(`{"returnCode":"SUCCESS","returnMsg":"OK","dataContent":"{\"resultCode\":\"SUCCESS\"}"}`), &fromString))
	require.JSONEq(t, `{"resultCode":"SUCCESS"}`, string(fromString.BusinessContent()))

	var fromObject PublicResponseEnvelope
	require.NoError(t, json.Unmarshal([]byte(`{"returnCode":"SUCCESS","returnMsg":"OK","dataContent":{"resultCode":"SUCCESS"}}`), &fromObject))
	require.JSONEq(t, `{"resultCode":"SUCCESS"}`, string(fromObject.BusinessContent()))
}

func TestPublicResponseEnvelopeAcceptsLegacyBizContentFallback(t *testing.T) {
	var fromObject PublicResponseEnvelope
	require.NoError(t, json.Unmarshal([]byte(`{"returnCode":"FAIL","returnMsg":"参数错误","bizContent":{"errCode":"INVALID_PARAMETER"}}`), &fromObject))
	require.JSONEq(t, `{"errCode":"INVALID_PARAMETER"}`, string(fromObject.BusinessContent()))
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
		DataContent:        JSONString(`{"resultCode":"SUCCESS"}`),
	}

	require.NoError(t, env.Validate())

	env.ReturnCode = ""
	require.EqualError(t, env.Validate(), "baofu public response returnCode is required")

	env.ReturnCode = PublicEnvelopeReturnCodeFail
	env.ReturnMessage = ""
	require.EqualError(t, env.Validate(), "baofu public response returnMsg is required when returnCode is FAIL")

	env.ReturnMessage = strings.Repeat("错", 129)
	require.EqualError(t, env.Validate(), "baofu public response returnMsg must be at most 128 characters")
}

func TestPublicResponseEnvelopeValidationUpstreamCodeForMissingDataContent(t *testing.T) {
	env := PublicResponseEnvelope{
		ReturnCode:         PublicEnvelopeReturnCodeSuccess,
		ReturnMessage:      "OK",
		MerchantID:         "100000",
		TerminalID:         "200000",
		Charset:            PublicEnvelopeCharsetUTF8,
		Version:            PublicEnvelopeVersion10,
		Format:             PublicEnvelopeFormatJSON,
		SignType:           SignTypeRSA,
		SignSerialNo:       "1",
		EncryptionSerialNo: "1",
		SignString:         "abcd",
	}

	err := env.Validate()

	require.EqualError(t, err, "baofu public response dataContent is required")
	require.Equal(t, PublicEnvelopeUpstreamCodeMissingDataContent, env.ValidationUpstreamCode(err))
}

func TestPublicResponseEnvelopeRejectsSerialsLongerThanOfficialS10(t *testing.T) {
	env := PublicResponseEnvelope{
		ReturnCode:         PublicEnvelopeReturnCodeSuccess,
		ReturnMessage:      "OK",
		MerchantID:         "100000",
		TerminalID:         "200000",
		Charset:            PublicEnvelopeCharsetUTF8,
		Version:            PublicEnvelopeVersion10,
		Format:             PublicEnvelopeFormatJSON,
		SignType:           SignTypeRSA,
		SignSerialNo:       "12345678901",
		EncryptionSerialNo: "1",
		SignString:         "abcd",
		DataContent:        JSONString(`{"resultCode":"SUCCESS"}`),
	}
	require.EqualError(t, env.Validate(), "baofu public response signSn must be at most 10 characters")

	env.SignSerialNo = "1"
	env.EncryptionSerialNo = "12345678901"
	require.EqualError(t, env.Validate(), "baofu public response ncrptnSn must be at most 10 characters")
}

func TestPublicResponseEnvelopeVerifiesDataContentSignature(t *testing.T) {
	privatePEM, publicPEM := generateBaofuTestKeyPair(t)
	dataContent := JSONString(`{"resultCode":"SUCCESS","outTradeNo":"BF1"}`)
	signature, err := SignSHA256WithRSA(privatePEM, []byte(dataContent))
	require.NoError(t, err)
	env := PublicResponseEnvelope{
		ReturnCode:         PublicEnvelopeReturnCodeSuccess,
		ReturnMessage:      "OK",
		MerchantID:         "100000",
		TerminalID:         "200000",
		Charset:            PublicEnvelopeCharsetUTF8,
		Version:            PublicEnvelopeVersion10,
		Format:             PublicEnvelopeFormatJSON,
		SignType:           SignTypeRSA,
		SignSerialNo:       "1",
		EncryptionSerialNo: "1",
		SignString:         signature,
		DataContent:        dataContent,
	}

	require.NoError(t, env.Validate())
	require.NoError(t, env.VerifySignature(publicPEM))

	env.DataContent = JSONString(`{"resultCode":"SUCCESS","outTradeNo":"tampered"}`)
	require.ErrorIs(t, env.VerifySignature(publicPEM), ErrInvalidSignature)
}

func TestPublicNotificationEnvelopeVerifiesDataContentSignature(t *testing.T) {
	privatePEM, publicPEM := generateBaofuTestKeyPair(t)
	dataContent := JSONString(`{"merId":"100000","terId":"200000","payCode":"WECHAT_JSAPI"}`)
	signature, err := SignSHA256WithRSA(privatePEM, []byte(dataContent))
	require.NoError(t, err)
	env := PublicNotificationEnvelope{
		MerchantID:         "100000",
		TerminalID:         "200000",
		Charset:            PublicEnvelopeCharsetUTF8,
		Version:            PublicEnvelopeVersion10,
		Format:             PublicEnvelopeFormatJSON,
		NotifyType:         "PAYMENT",
		SignType:           SignTypeRSA,
		SignSerialNo:       "1",
		EncryptionSerialNo: "1",
		SignString:         signature,
		DataContent:        dataContent,
	}

	require.NoError(t, env.Validate())
	require.NoError(t, env.VerifySignature(publicPEM))

	env.SignString = "bad-signature"
	require.ErrorIs(t, env.VerifySignature(publicPEM), ErrInvalidSignature)
}

func TestPublicNotificationEnvelopeAcceptsNumericPublicScalars(t *testing.T) {
	var env PublicNotificationEnvelope

	err := json.Unmarshal([]byte(`{"merId":102004465,"terId":200005200,"charset":"UTF-8","version":"1.0","format":"json","notifyType":"PAYMENT","signType":"RSA","signSn":"1","ncrptnSn":"1","signStr":"abc","dataContent":{"resultCode":"SUCCESS"}}`), &env)

	require.NoError(t, err)
	require.Equal(t, "102004465", env.MerchantID)
	require.Equal(t, "200005200", env.TerminalID)
	require.JSONEq(t, `{"resultCode":"SUCCESS"}`, string(env.DataContent))
}

func TestPublicNotificationEnvelopeValidateRequiresOfficialFields(t *testing.T) {
	env := validPublicNotificationEnvelopeForTest()

	require.NoError(t, env.Validate())

	env.NotifyType = ""
	require.EqualError(t, env.Validate(), "baofu public notification notifyType is required")

	env = validPublicNotificationEnvelopeForTest()
	env.DataContent = nil
	require.EqualError(t, env.Validate(), "baofu public notification dataContent is required")

	env = validPublicNotificationEnvelopeForTest()
	env.Charset = "GBK"
	require.EqualError(t, env.Validate(), "baofu public notification charset must be UTF-8")
}

func TestPublicNotificationEnvelopeRejectsUnsupportedOfficialNotifyType(t *testing.T) {
	env := validPublicNotificationEnvelopeForTest()
	env.NotifyType = "SHARE"

	require.EqualError(t, env.Validate(), "baofu public notification notifyType is unsupported")
}

func TestPublicNotificationEnvelopeRejectsSerialsLongerThanOfficialS10(t *testing.T) {
	env := validPublicNotificationEnvelopeForTest()
	env.SignSerialNo = "12345678901"
	require.EqualError(t, env.Validate(), "baofu public notification signSn must be at most 10 characters")

	env = validPublicNotificationEnvelopeForTest()
	env.EncryptionSerialNo = "12345678901"
	require.EqualError(t, env.Validate(), "baofu public notification ncrptnSn must be at most 10 characters")
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

func validPublicNotificationEnvelopeForTest() PublicNotificationEnvelope {
	return PublicNotificationEnvelope{
		MerchantID:         "100000",
		TerminalID:         "200000",
		Charset:            PublicEnvelopeCharsetUTF8,
		Version:            PublicEnvelopeVersion10,
		Format:             PublicEnvelopeFormatJSON,
		NotifyType:         "PAYMENT",
		SignType:           SignTypeRSA,
		SignSerialNo:       "1",
		EncryptionSerialNo: "1",
		SignString:         "abcd",
		DataContent:        JSONString(`{"merId":"100000","terId":"200000","payCode":"WECHAT_JSAPI"}`),
	}
}
