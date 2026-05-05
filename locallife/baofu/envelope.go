package baofu

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

const (
	PublicEnvelopeCharsetUTF8 = "UTF-8"
	PublicEnvelopeVersion10   = "1.0"
	PublicEnvelopeFormatJSON  = "json"

	SignTypeSM2 = "SM2"
	SignTypeRSA = "RSA"

	PublicEnvelopeReturnCodeSuccess = "SUCCESS"
	PublicEnvelopeReturnCodeFail    = "FAIL"

	PublicEnvelopeUpstreamCodeMissingDataContent = "MISSING_DATA_CONTENT"
	PublicEnvelopeUpstreamCodeInvalidDataContent = "INVALID_DATA_CONTENT"
	PublicEnvelopeUpstreamCodeInvalidSignature   = "INVALID_SIGNATURE"

	PublicNotificationTypePayment = "PAYMENT"
	PublicNotificationTypeSharing = "SHARING"
	PublicNotificationTypeRefund  = "REFUND"
	PublicNotificationTypeSign    = "SIGN"
)

type PublicRequestEnvelope struct {
	MerchantID         string     `json:"merId"`
	TerminalID         string     `json:"terId"`
	Method             string     `json:"method"`
	Charset            string     `json:"charset"`
	Version            string     `json:"version"`
	Format             string     `json:"format"`
	Timestamp          string     `json:"timestamp"`
	SignType           string     `json:"signType"`
	SignSerialNo       string     `json:"signSn"`
	EncryptionSerialNo string     `json:"ncrptnSn"`
	DigitalEnvelope    string     `json:"dgtlEnvlp,omitempty"`
	SignString         string     `json:"signStr"`
	BizContent         JSONString `json:"bizContent"`
}

type PublicResponseEnvelope struct {
	ReturnCode         string     `json:"returnCode"`
	ReturnMessage      string     `json:"returnMsg"`
	MerchantID         string     `json:"merId,omitempty"`
	TerminalID         string     `json:"terId,omitempty"`
	Charset            string     `json:"charset,omitempty"`
	Version            string     `json:"version,omitempty"`
	Format             string     `json:"format,omitempty"`
	SignType           string     `json:"signType,omitempty"`
	SignSerialNo       string     `json:"signSn,omitempty"`
	EncryptionSerialNo string     `json:"ncrptnSn,omitempty"`
	DigitalEnvelope    string     `json:"dgtlEnvlp,omitempty"`
	SignString         string     `json:"signStr,omitempty"`
	DataContent        JSONString `json:"dataContent,omitempty"`
	BizContent         JSONString `json:"bizContent,omitempty"`
}

type PublicNotificationEnvelope struct {
	MerchantID         string     `json:"merId"`
	TerminalID         string     `json:"terId"`
	Charset            string     `json:"charset"`
	Version            string     `json:"version"`
	Format             string     `json:"format"`
	NotifyType         string     `json:"notifyType"`
	SignType           string     `json:"signType"`
	SignSerialNo       string     `json:"signSn"`
	EncryptionSerialNo string     `json:"ncrptnSn"`
	DigitalEnvelope    string     `json:"dgtlEnvlp,omitempty"`
	SignString         string     `json:"signStr"`
	DataContent        JSONString `json:"dataContent"`
}

type JSONString []byte

func (c JSONString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(c))
}

func (c *JSONString) UnmarshalJSON(raw []byte) error {
	if c == nil {
		return errors.New("baofu json string target is nil")
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		*c = JSONString(strings.TrimSpace(text))
		return nil
	}
	if !json.Valid(raw) {
		return errors.New("baofu json string content must be valid JSON")
	}
	*c = JSONString(raw)
	return nil
}

func (e PublicRequestEnvelope) Validate() error {
	if strings.TrimSpace(e.MerchantID) == "" {
		return errors.New("baofu public envelope merId is required")
	}
	if strings.TrimSpace(e.TerminalID) == "" {
		return errors.New("baofu public envelope terId is required")
	}
	if strings.TrimSpace(e.Method) == "" {
		return errors.New("baofu public envelope method is required")
	}
	if strings.TrimSpace(e.Charset) != PublicEnvelopeCharsetUTF8 {
		return errors.New("baofu public envelope charset must be UTF-8")
	}
	if strings.TrimSpace(e.Version) != PublicEnvelopeVersion10 {
		return errors.New("baofu public envelope version must be 1.0")
	}
	if strings.TrimSpace(e.Format) != PublicEnvelopeFormatJSON {
		return errors.New("baofu public envelope format must be json")
	}
	if strings.TrimSpace(e.Timestamp) == "" {
		return errors.New("baofu public envelope timestamp is required")
	}
	if !isSupportedSignType(e.SignType) {
		return errors.New("baofu public envelope signType is unsupported")
	}
	if strings.TrimSpace(e.SignSerialNo) == "" {
		return errors.New("baofu public envelope signSn is required")
	}
	if len(strings.TrimSpace(e.SignSerialNo)) > 10 {
		return errors.New("baofu public envelope signSn must be at most 10 characters")
	}
	if strings.TrimSpace(e.EncryptionSerialNo) == "" {
		return errors.New("baofu public envelope ncrptnSn is required")
	}
	if len(strings.TrimSpace(e.EncryptionSerialNo)) > 10 {
		return errors.New("baofu public envelope ncrptnSn must be at most 10 characters")
	}
	if strings.TrimSpace(e.SignString) == "" {
		return errors.New("baofu public envelope signStr is required")
	}
	if len(e.BizContent) == 0 {
		return errors.New("baofu public envelope bizContent is required")
	}
	if !json.Valid([]byte(e.BizContent)) {
		return errors.New("baofu public envelope bizContent must be valid JSON")
	}
	return nil
}

func (e PublicRequestEnvelope) FormValues() url.Values {
	values := url.Values{}
	values.Set("merId", strings.TrimSpace(e.MerchantID))
	values.Set("terId", strings.TrimSpace(e.TerminalID))
	values.Set("method", strings.TrimSpace(e.Method))
	values.Set("charset", strings.TrimSpace(e.Charset))
	values.Set("version", strings.TrimSpace(e.Version))
	values.Set("format", strings.TrimSpace(e.Format))
	values.Set("timestamp", strings.TrimSpace(e.Timestamp))
	values.Set("signType", strings.TrimSpace(e.SignType))
	values.Set("signSn", strings.TrimSpace(e.SignSerialNo))
	values.Set("ncrptnSn", strings.TrimSpace(e.EncryptionSerialNo))
	values.Set("dgtlEnvlp", strings.TrimSpace(e.DigitalEnvelope))
	values.Set("signStr", strings.TrimSpace(e.SignString))
	values.Set("bizContent", string(e.BizContent))
	return values
}

func (e PublicResponseEnvelope) Validate() error {
	switch strings.TrimSpace(e.ReturnCode) {
	case PublicEnvelopeReturnCodeSuccess:
	case PublicEnvelopeReturnCodeFail:
		return nil
	default:
		if strings.TrimSpace(e.ReturnCode) == "" {
			return errors.New("baofu public response returnCode is required")
		}
		return errors.New("baofu public response returnCode is unsupported")
	}
	if strings.TrimSpace(e.MerchantID) == "" {
		return errors.New("baofu public response merId is required")
	}
	if strings.TrimSpace(e.TerminalID) == "" {
		return errors.New("baofu public response terId is required")
	}
	if strings.TrimSpace(e.Charset) != PublicEnvelopeCharsetUTF8 {
		return errors.New("baofu public response charset must be UTF-8")
	}
	if strings.TrimSpace(e.Version) != PublicEnvelopeVersion10 {
		return errors.New("baofu public response version must be 1.0")
	}
	if strings.TrimSpace(e.Format) != PublicEnvelopeFormatJSON {
		return errors.New("baofu public response format must be json")
	}
	if !isSupportedSignType(e.SignType) {
		return errors.New("baofu public response signType is unsupported")
	}
	if strings.TrimSpace(e.SignSerialNo) == "" {
		return errors.New("baofu public response signSn is required")
	}
	if len(strings.TrimSpace(e.SignSerialNo)) > 10 {
		return errors.New("baofu public response signSn must be at most 10 characters")
	}
	if strings.TrimSpace(e.EncryptionSerialNo) == "" {
		return errors.New("baofu public response ncrptnSn is required")
	}
	if len(strings.TrimSpace(e.EncryptionSerialNo)) > 10 {
		return errors.New("baofu public response ncrptnSn must be at most 10 characters")
	}
	if strings.TrimSpace(e.SignString) == "" {
		return errors.New("baofu public response signStr is required")
	}
	businessContent := e.BusinessContent()
	if len(businessContent) == 0 {
		return errors.New("baofu public response dataContent is required")
	}
	if !json.Valid([]byte(businessContent)) {
		return errors.New("baofu public response dataContent must be valid JSON")
	}
	return nil
}

func (e PublicResponseEnvelope) VerifySignature(publicKeyPEM string) error {
	if err := verifyPublicEnvelopeSignature("baofu public response", e.SignType, publicKeyPEM, e.BusinessContent(), e.SignString); err != nil {
		return err
	}
	return nil
}

func (e PublicResponseEnvelope) ValidationUpstreamCode(err error) string {
	if err == nil || strings.TrimSpace(e.ReturnCode) != PublicEnvelopeReturnCodeSuccess {
		return strings.TrimSpace(e.ReturnCode)
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "dataContent is required"):
		return PublicEnvelopeUpstreamCodeMissingDataContent
	case strings.Contains(message, "dataContent must be valid JSON"):
		return PublicEnvelopeUpstreamCodeInvalidDataContent
	case errors.Is(err, ErrInvalidSignature), strings.Contains(message, "signature"):
		return PublicEnvelopeUpstreamCodeInvalidSignature
	default:
		return strings.TrimSpace(e.ReturnCode)
	}
}

func (e PublicResponseEnvelope) BusinessContent() JSONString {
	if len(e.DataContent) > 0 {
		return e.DataContent
	}
	return e.BizContent
}

func (e PublicNotificationEnvelope) Validate() error {
	if strings.TrimSpace(e.MerchantID) == "" {
		return errors.New("baofu public notification merId is required")
	}
	if strings.TrimSpace(e.TerminalID) == "" {
		return errors.New("baofu public notification terId is required")
	}
	if strings.TrimSpace(e.Charset) != PublicEnvelopeCharsetUTF8 {
		return errors.New("baofu public notification charset must be UTF-8")
	}
	if strings.TrimSpace(e.Version) != PublicEnvelopeVersion10 {
		return errors.New("baofu public notification version must be 1.0")
	}
	if strings.TrimSpace(e.Format) != PublicEnvelopeFormatJSON {
		return errors.New("baofu public notification format must be json")
	}
	if strings.TrimSpace(e.NotifyType) == "" {
		return errors.New("baofu public notification notifyType is required")
	}
	if !isSupportedPublicNotificationType(e.NotifyType) {
		return errors.New("baofu public notification notifyType is unsupported")
	}
	if !isSupportedSignType(e.SignType) {
		return errors.New("baofu public notification signType is unsupported")
	}
	if strings.TrimSpace(e.SignSerialNo) == "" {
		return errors.New("baofu public notification signSn is required")
	}
	if len(strings.TrimSpace(e.SignSerialNo)) > 10 {
		return errors.New("baofu public notification signSn must be at most 10 characters")
	}
	if strings.TrimSpace(e.EncryptionSerialNo) == "" {
		return errors.New("baofu public notification ncrptnSn is required")
	}
	if len(strings.TrimSpace(e.EncryptionSerialNo)) > 10 {
		return errors.New("baofu public notification ncrptnSn must be at most 10 characters")
	}
	if strings.TrimSpace(e.SignString) == "" {
		return errors.New("baofu public notification signStr is required")
	}
	if len(e.DataContent) == 0 {
		return errors.New("baofu public notification dataContent is required")
	}
	if !json.Valid([]byte(e.DataContent)) {
		return errors.New("baofu public notification dataContent must be valid JSON")
	}
	return nil
}

func (e PublicNotificationEnvelope) VerifySignature(publicKeyPEM string) error {
	return verifyPublicEnvelopeSignature("baofu public notification", e.SignType, publicKeyPEM, e.DataContent, e.SignString)
}

func isSupportedSignType(signType string) bool {
	switch strings.ToUpper(strings.TrimSpace(signType)) {
	case SignTypeSM2, SignTypeRSA:
		return true
	default:
		return false
	}
}

func isSupportedPublicNotificationType(notifyType string) bool {
	switch strings.ToUpper(strings.TrimSpace(notifyType)) {
	case PublicNotificationTypePayment, PublicNotificationTypeSharing, PublicNotificationTypeRefund, PublicNotificationTypeSign:
		return true
	default:
		return false
	}
}

func verifyPublicEnvelopeSignature(prefix, signType, publicKeyPEM string, message JSONString, signature string) error {
	if strings.TrimSpace(publicKeyPEM) == "" {
		return errors.New(prefix + " public key is required")
	}
	if len(message) == 0 {
		return errors.New(prefix + " dataContent is required")
	}
	switch strings.ToUpper(strings.TrimSpace(signType)) {
	case SignTypeRSA:
		return VerifySHA256WithRSA(publicKeyPEM, []byte(message), strings.TrimSpace(signature))
	case SignTypeSM2:
		return errors.New(prefix + " sm2 signature verification is not supported")
	default:
		return errors.New(prefix + " signType is unsupported")
	}
}
