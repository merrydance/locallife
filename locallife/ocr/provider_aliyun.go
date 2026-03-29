package ocr

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/merrydance/locallife/util"
)

var (
	ErrAliyunOCRUnauthorized = errors.New("aliyun ocr unauthorized")
	ErrAliyunOCRForbidden    = errors.New("aliyun ocr forbidden")
	ErrAliyunOCRRateLimited  = errors.New("aliyun ocr rate limited")
	ErrAliyunOCRSigning      = errors.New("aliyun ocr signing failed")
	ErrAliyunOCRUnavailable  = errors.New("aliyun ocr unavailable")
	ErrAliyunOCRBadRequest   = errors.New("aliyun ocr bad request")
)

const aliyunACSDateFormat = "2006-01-02T15:04:05Z"

type aliyunOperation struct {
	Action  string
	Version string
}

var aliyunCapabilityMap = map[Capability]aliyunOperation{
	CapabilityAliyunBusinessLicense: {Action: "RecognizeBusinessLicense", Version: "2021-07-07"},
	CapabilityAliyunIDCard:          {Action: "RecognizeIdcard", Version: "2021-07-07"},
	CapabilityAliyunFoodPermit:      {Action: "RecognizeFoodManageLicense", Version: "2021-07-07"},
	CapabilityAliyunHealthCert:      {Action: "RecognizeHealthCert", Version: "2021-07-07"},
}

// AliyunAPIError is the normalized error model returned by Aliyun OCR RPC APIs.
type AliyunAPIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *AliyunAPIError) Error() string {
	return fmt.Sprintf("aliyun ocr error: code=%s, message=%s, status=%d", e.Code, e.Message, e.StatusCode)
}

// AliyunOCRClient defines the subset of Aliyun OCR operations used by the provider.
type AliyunOCRClient interface {
	Recognize(ctx context.Context, capability Capability, req RecognizeRequest) (json.RawMessage, error)
}

// AliyunProvider is the primary OCR provider implementation.
type AliyunProvider struct {
	client AliyunOCRClient
}

// NewAliyunProviderFromConfig creates an Aliyun OCR provider from validated config.
func NewAliyunProviderFromConfig(config util.Config) (*AliyunProvider, error) {
	if err := config.ValidateAliyunOCRConfig(); err != nil {
		return nil, err
	}
	if !config.AliyunOCREnabled {
		return nil, fmt.Errorf("aliyun ocr provider is disabled")
	}
	client, err := NewAliyunOpenAPIClient(config)
	if err != nil {
		return nil, err
	}
	return NewAliyunProvider(client), nil
}

// NewAliyunProvider creates a provider with explicit client dependency.
func NewAliyunProvider(client AliyunOCRClient) *AliyunProvider {
	return &AliyunProvider{client: client}
}

// Name returns the provider name.
func (p *AliyunProvider) Name() ProviderName {
	return ProviderNameAliyun
}

// Recognize executes OCR through the Aliyun client and returns normalized provider metadata.
func (p *AliyunProvider) Recognize(ctx context.Context, capability Capability, req RecognizeRequest) (RecognizeResponse, error) {
	raw, err := p.client.Recognize(ctx, capability, req)
	if err != nil {
		return RecognizeResponse{}, MapAliyunOCRAPIError(err)
	}
	normalized := NormalizedResult{
		DocumentType: req.DocumentType,
		Side:         req.Side,
		RecognizedAt: time.Now().UTC(),
	}
	if capability == CapabilityAliyunFoodPermit {
		normalized.FoodPermit = normalizeAliyunFoodPermitResult(raw)
	}
	if capability == CapabilityAliyunBusinessLicense {
		normalized.BusinessLicense = normalizeAliyunBusinessLicenseResult(raw)
	}
	if capability == CapabilityAliyunIDCard {
		normalized.IDCard = normalizeAliyunIDCardResult(raw)
	}
	return RecognizeResponse{
		Provider:   ProviderNameAliyun,
		RawResult:  raw,
		Normalized: normalized,
	}, nil
}

func normalizeAliyunFoodPermitResult(raw json.RawMessage) *FoodPermitResult {
	fields := collectAliyunStringFields(raw)
	licenseNumber := firstAliyunField(fields,
		"license_number", "licensenumber", "permit_no", "permitnumber", "foodpermitnumber",
		"registrationnumber", "registernumber", "certificatenumber", "number",
		"licence_number", "licencenumber", "licence_no", "licenceno")
	businessName := firstAliyunField(fields,
		"business_name", "businessname", "company_name", "companyname", "merchantname",
		"unitname", "entityname", "shopname", "subjectname",
		"operator_name", "operatorname")
	operatorName := firstAliyunField(fields,
		"legal_representative", "legalrepresentative", "legal_person", "legalperson",
		"ownername", "managername", "principal", "personincharge")
	address := firstAliyunField(fields,
		"address", "business_address", "businessaddress", "office_address", "officeaddress", "site")
	validFrom := firstAliyunField(fields,
		"valid_from", "validfrom", "fromdate", "startdate", "effective_date", "effectivedate",
		"issue_date", "issuedate")
	validTo := firstAliyunField(fields,
		"valid_to", "validto", "todate", "enddate", "expiredate", "expirydate", "expirationdate",
		"valid_to_date", "validtodate", "standardized_valid_to_date", "standardizedvalidtodate")
	validPeriod := firstAliyunField(fields, "valid_period", "validperiod")
	rawText := firstAliyunField(fields, "raw_text", "rawtext", "ocr_text", "ocrtext", "alltext", "fulltext", "content", "text")
	if rawText == "" {
		rawText = buildAliyunFoodPermitRawText(licenseNumber, businessName, operatorName, address, validFrom, validTo, validPeriod)
	}
	if licenseNumber == "" && businessName == "" && operatorName == "" && address == "" && validFrom == "" && validTo == "" && validPeriod == "" && rawText == "" {
		return nil
	}
	if validPeriod == "" {
		switch {
		case validFrom != "" && validTo != "":
			validPeriod = validFrom + "至" + validTo
		case validTo != "":
			validPeriod = validTo
		}
	}
	return &FoodPermitResult{
		LicenseNumber: licenseNumber,
		OperatorName:  operatorName,
		BusinessName:  businessName,
		Address:       address,
		ValidPeriod:   validPeriod,
		RawText:       rawText,
	}
}

func normalizeAliyunBusinessLicenseResult(raw json.RawMessage) *BusinessLicenseResult {
	fields := collectAliyunStringFields(raw)
	creditCode := firstAliyunField(fields, "credit_code", "creditcode", "unifiedsocialcreditcode", "socialcreditcode")
	registrationNumber := firstAliyunField(fields, "registration_number", "registrationnumber", "reg_num", "regnum", "licensenumber")
	enterpriseName := firstAliyunField(fields, "enterprise_name", "enterprisename", "company_name", "companyname", "merchantname")
	legalRepresentative := firstAliyunField(fields, "legal_representative", "legalrepresentative", "legal_person", "legalperson", "operator_name", "operatorname")
	address := firstAliyunField(fields, "address", "business_address", "businessaddress")
	businessScope := firstAliyunField(fields, "business_scope", "businessscope", "scope")
	validPeriod := firstAliyunField(fields, "valid_period", "validperiod", "period")
	if creditCode == "" && registrationNumber == "" && enterpriseName == "" && legalRepresentative == "" && address == "" && businessScope == "" && validPeriod == "" {
		return nil
	}
	return &BusinessLicenseResult{
		CreditCode:          creditCode,
		RegistrationNumber:  registrationNumber,
		EnterpriseName:      enterpriseName,
		LegalRepresentative: legalRepresentative,
		Address:             address,
		BusinessScope:       businessScope,
		ValidPeriod:         validPeriod,
	}
}

func normalizeAliyunIDCardResult(raw json.RawMessage) *IDCardResult {
	fields := collectAliyunStringFields(raw)
	name := firstAliyunField(fields, "name", "fullname")
	idNumber := firstAliyunField(fields, "id_number", "idnumber", "id_num", "idnum", "number")
	gender := firstAliyunField(fields, "gender", "sex")
	ethnicity := firstAliyunField(fields, "ethnicity", "nation", "nationality")
	address := firstAliyunField(fields, "address", "addr")
	birthDate := firstAliyunField(fields, "birth_date", "birthdate", "birthday")
	authority := firstAliyunField(fields, "authority", "issueauthority", "issuingauthority")
	validPeriod := firstAliyunField(fields, "valid_period", "validperiod", "valid_date", "validdate")
	if name == "" && idNumber == "" && gender == "" && ethnicity == "" && address == "" && birthDate == "" && authority == "" && validPeriod == "" {
		return nil
	}
	return &IDCardResult{
		Name:        name,
		IDNumber:    idNumber,
		Gender:      gender,
		Ethnicity:   ethnicity,
		Address:     address,
		BirthDate:   birthDate,
		Authority:   authority,
		ValidPeriod: validPeriod,
	}
}

func buildAliyunFoodPermitRawText(licenseNumber, businessName, operatorName, address, validFrom, validTo, validPeriod string) string {
	lines := make([]string, 0, 5)
	if businessName != "" {
		lines = append(lines, "主体名称："+businessName)
	}
	if operatorName != "" {
		lines = append(lines, "经营者姓名："+operatorName)
	}
	if licenseNumber != "" {
		lines = append(lines, "许可证编号："+licenseNumber)
	}
	if address != "" {
		lines = append(lines, "地址："+address)
	}
	if validTo != "" {
		lines = append(lines, "有效期至："+validTo)
	} else if validPeriod != "" {
		lines = append(lines, "有效期："+validPeriod)
	} else if validFrom != "" {
		lines = append(lines, "有效期自："+validFrom)
	}
	return strings.Join(lines, "\n")
}

func collectAliyunStringFields(raw json.RawMessage) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	fields := make(map[string]string)
	collectAliyunFieldValues("", payload, fields)
	return fields
}

func collectAliyunFieldValues(prefix string, value any, fields map[string]string) {
	switch v := value.(type) {
	case map[string]any:
		for key, nested := range v {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			collectAliyunFieldValues(next, nested, fields)
		}
	case []any:
		for _, item := range v {
			collectAliyunFieldValues(prefix, item, fields)
		}
	case string:
		storeAliyunFieldValue(prefix, v, fields)
	case float64:
		storeAliyunFieldValue(prefix, fmt.Sprintf("%v", v), fields)
	case bool:
		storeAliyunFieldValue(prefix, fmt.Sprintf("%t", v), fields)
	}
}

func storeAliyunFieldValue(prefix, value string, fields map[string]string) {
	value = strings.TrimSpace(value)
	if prefix == "" || value == "" {
		return
	}
	fullKey := normalizeAliyunFieldKey(prefix)
	if fullKey != "" && fields[fullKey] == "" {
		fields[fullKey] = value
	}
	segments := strings.Split(fullKey, ".")
	leafKey := segments[len(segments)-1]
	if leafKey != "" && fields[leafKey] == "" {
		fields[leafKey] = value
	}
}

func normalizeAliyunFieldKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	replacer := strings.NewReplacer("-", "_", " ", "_", "[", ".", "]", "", "__", "_")
	key = replacer.Replace(key)
	key = strings.Trim(key, "._")
	return key
}

func firstAliyunField(fields map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fields[normalizeAliyunFieldKey(key)]); value != "" {
			return value
		}
	}
	return ""
}

// AliyunOpenAPIClient is a minimal RPC client for Aliyun OCR OpenAPI.
type AliyunOpenAPIClient struct {
	endpoint        string
	region          string
	accessKeyID     string
	accessKeySecret string
	httpClient      *http.Client
	clock           func() time.Time
	nonce           func() string
	signer          func(httpReq *http.Request, payload []byte) error
}

// NewAliyunOpenAPIClient creates the default Aliyun OCR OpenAPI client.
func NewAliyunOpenAPIClient(config util.Config) (*AliyunOpenAPIClient, error) {
	if err := config.ValidateAliyunOCRConfig(); err != nil {
		return nil, err
	}
	if config.AliyunOCRSTSEnabled {
		return nil, fmt.Errorf("aliyun ocr sts mode is not implemented yet")
	}
	client := &AliyunOpenAPIClient{
		endpoint:        config.AliyunOCREndpoint,
		region:          config.AliyunOCRRegion,
		accessKeyID:     config.AliyunOCRAccessKeyID,
		accessKeySecret: config.AliyunOCRAccessKeySecret,
		httpClient:      &http.Client{Timeout: config.AliyunOCRHTTPTimeout},
		clock:           time.Now,
		nonce: func() string {
			return fmt.Sprintf("%d", time.Now().UnixNano())
		},
	}
	client.signer = client.defaultSigner
	return client, nil
}

// Recognize performs a signed RPC call to the Aliyun OCR endpoint.
func (c *AliyunOpenAPIClient) Recognize(ctx context.Context, capability Capability, req RecognizeRequest) (json.RawMessage, error) {
	op, ok := aliyunCapabilityMap[capability]
	if !ok {
		return nil, fmt.Errorf("unsupported aliyun capability: %s", capability)
	}
	requestURL, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, err
	}
	if len(req.Data) == 0 {
		return nil, fmt.Errorf("aliyun ocr request body is empty for capability=%s media_asset_id=%d", capability, req.MediaAssetID)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), bytes.NewReader(req.Data))
	if err != nil {
		return nil, err
	}
	httpReq.ContentLength = int64(len(req.Data))
	httpReq.Header.Set("Content-Type", "application/octet-stream")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-acs-version", op.Version)
	httpReq.Header.Set("x-acs-action", op.Action)
	if err := c.signer(httpReq, req.Data); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAliyunOCRSigning, err)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAliyunOCRUnavailable, err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: action=%s body_len=%d media_asset_id=%d", parseAliyunAPIError(resp.StatusCode, payload), op.Action, len(req.Data), req.MediaAssetID)
	}
	return json.RawMessage(payload), nil
}

func (c *AliyunOpenAPIClient) defaultSigner(httpReq *http.Request, payload []byte) error {
	if c.accessKeyID == "" || c.accessKeySecret == "" {
		return fmt.Errorf("missing access key")
	}
	now := c.clock().UTC()
	httpReq.Header.Set("Host", httpReq.URL.Host)
	httpReq.Header.Set("x-acs-date", now.Format(aliyunACSDateFormat))
	httpReq.Header.Set("x-acs-signature-nonce", c.nonce())
	payloadHash := hexSHA256(payload)
	httpReq.Header.Set("x-acs-content-sha256", payloadHash)
	signedHeaders := canonicalSignedHeaderNames(httpReq.Header)
	canonicalRequest := buildAliyunCanonicalRequest(httpReq, signedHeaders, payloadHash)
	stringToSign := "ACS3-HMAC-SHA256\n" + hexSHA256([]byte(canonicalRequest))
	signature := hexHMACSHA256([]byte(c.accessKeySecret), []byte(stringToSign))
	httpReq.Header.Set("Authorization", fmt.Sprintf(
		"ACS3-HMAC-SHA256 Credential=%s,SignedHeaders=%s,Signature=%s",
		c.accessKeyID,
		strings.Join(signedHeaders, ";"),
		signature,
	))
	return nil
}

// MapAliyunOCRAPIError converts provider-specific API errors into retry/permission classes.
func MapAliyunOCRAPIError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrAliyunOCRSigning) {
		return err
	}
	var apiErr *AliyunAPIError
	if !errors.As(err, &apiErr) {
		return err
	}
	code := strings.ToLower(apiErr.Code)
	switch {
	case apiErr.StatusCode == http.StatusUnauthorized || strings.Contains(code, "invalidaccesskey") || strings.Contains(code, "signature"):
		return fmt.Errorf("%w: %s", ErrAliyunOCRUnauthorized, err)
	case apiErr.StatusCode == http.StatusForbidden || strings.Contains(code, "forbidden") || strings.Contains(code, "permission"):
		return fmt.Errorf("%w: %s", ErrAliyunOCRForbidden, err)
	case apiErr.StatusCode == http.StatusTooManyRequests || strings.Contains(code, "throttl") || strings.Contains(code, "ratelimit"):
		return fmt.Errorf("%w: %s", ErrAliyunOCRRateLimited, err)
	case apiErr.StatusCode >= 500:
		return fmt.Errorf("%w: %s", ErrAliyunOCRUnavailable, err)
	default:
		return fmt.Errorf("%w: %s", ErrAliyunOCRBadRequest, err)
	}
}

func parseAliyunAPIError(statusCode int, payload []byte) error {
	var apiErr struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(payload, &apiErr); err != nil {
		return &AliyunAPIError{StatusCode: statusCode, Code: fmt.Sprintf("http_%d", statusCode), Message: string(payload)}
	}
	return &AliyunAPIError{StatusCode: statusCode, Code: apiErr.Code, Message: apiErr.Message}
}

func buildAliyunCanonicalRequest(httpReq *http.Request, signedHeaders []string, payloadHash string) string {
	canonicalURI := httpReq.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := canonicalQueryString(httpReq.URL)
	canonicalHeaders := canonicalHeaderString(httpReq.Header, signedHeaders)
	if canonicalHeaders != "" {
		canonicalHeaders += "\n"
	}
	return strings.Join([]string{
		httpReq.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		strings.Join(signedHeaders, ";"),
		payloadHash,
	}, "\n")
}

func canonicalSignedHeaderNames(header http.Header) []string {
	names := make([]string, 0, len(header))
	for key := range header {
		names = append(names, strings.ToLower(key))
	}
	sort.Strings(names)
	return names
}

func canonicalHeaderString(header http.Header, signedHeaders []string) string {
	parts := make([]string, 0, len(signedHeaders))
	for _, key := range signedHeaders {
		values := header.Values(key)
		normalized := make([]string, 0, len(values))
		for _, value := range values {
			normalized = append(normalized, strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
		}
		parts = append(parts, key+":"+strings.Join(normalized, ","))
	}
	return strings.Join(parts, "\n")
}

func canonicalQueryString(endpoint *url.URL) string {
	if endpoint == nil {
		return ""
	}
	query := endpoint.Query()
	if len(query) == 0 {
		return ""
	}
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		values := append([]string(nil), query[key]...)
		sort.Strings(values)
		escapedKey := url.QueryEscape(key)
		for _, value := range values {
			pairs = append(pairs, escapedKey+"="+url.QueryEscape(value))
		}
	}
	return strings.Join(pairs, "&")
}

func hexSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hexHMACSHA256(secret, data []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}
