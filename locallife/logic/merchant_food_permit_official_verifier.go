package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	defaultMerchantFoodPermitQRCodeScheme = "http"
	defaultMerchantFoodPermitQRCodeHost   = "121.28.87.7:8081"
	defaultMerchantFoodPermitQRCodePath   = "/OrcodeXcyXzf.jsp"
	defaultMerchantFoodPermitLookupPath   = "/xcyxszPermit_getById.action"
)

var ErrMerchantFoodPermitOfficialVerificationUnavailable = errors.New("merchant food permit official verification unavailable")

type MerchantFoodPermitOfficialVerification struct {
	CompanyName  string
	OperatorName string
	PermitNo     string
	CreditCode   string
	Address      string
	ValidTo      string
}

type MerchantFoodPermitOfficialVerifierConfig struct {
	HTTPClient    *http.Client
	AllowedScheme string
	AllowedHost   string
	QRPath        string
	LookupPath    string
	Timeout       time.Duration
}

type MerchantFoodPermitOfficialVerifier struct {
	client        *http.Client
	allowedScheme string
	allowedHost   string
	qrPath        string
	lookupPath    string
	timeout       time.Duration
}

func NewMerchantFoodPermitOfficialVerifier(config MerchantFoodPermitOfficialVerifierConfig) *MerchantFoodPermitOfficialVerifier {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{}
	} else {
		copied := *client
		client = &copied
	}
	if client.Timeout <= 0 {
		client.Timeout = timeout
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	allowedScheme := strings.TrimSpace(config.AllowedScheme)
	if allowedScheme == "" {
		allowedScheme = defaultMerchantFoodPermitQRCodeScheme
	}
	allowedHost := strings.TrimSpace(config.AllowedHost)
	if allowedHost == "" {
		allowedHost = defaultMerchantFoodPermitQRCodeHost
	}
	qrPath := strings.TrimSpace(config.QRPath)
	if qrPath == "" {
		qrPath = defaultMerchantFoodPermitQRCodePath
	}
	lookupPath := strings.TrimSpace(config.LookupPath)
	if lookupPath == "" {
		lookupPath = defaultMerchantFoodPermitLookupPath
	}
	return &MerchantFoodPermitOfficialVerifier{
		client:        client,
		allowedScheme: allowedScheme,
		allowedHost:   allowedHost,
		qrPath:        qrPath,
		lookupPath:    lookupPath,
		timeout:       timeout,
	}
}

func (v *MerchantFoodPermitOfficialVerifier) VerifyMerchantFoodPermit(ctx context.Context, rawResult []byte) (MerchantFoodPermitOfficialVerification, error) {
	if v == nil || len(rawResult) == 0 {
		return MerchantFoodPermitOfficialVerification{}, ErrMerchantFoodPermitOfficialVerificationUnavailable
	}
	if ctx == nil {
		return MerchantFoodPermitOfficialVerification{}, ErrMerchantFoodPermitOfficialVerificationUnavailable
	}
	qrURL, ok := v.findAllowedQRCodeURL(rawResult)
	if !ok {
		return MerchantFoodPermitOfficialVerification{}, ErrMerchantFoodPermitOfficialVerificationUnavailable
	}

	flowID := strings.TrimSpace(qrURL.Query().Get("flowId"))
	zsID := strings.TrimSpace(qrURL.Query().Get("zsId"))
	if !merchantFoodPermitOfficialIDPattern.MatchString(flowID) || !merchantFoodPermitOfficialIDPattern.MatchString(zsID) {
		return MerchantFoodPermitOfficialVerification{}, ErrMerchantFoodPermitOfficialVerificationUnavailable
	}

	lookupURL := url.URL{
		Scheme: qrURL.Scheme,
		Host:   qrURL.Host,
		Path:   v.lookupPath,
	}
	form := url.Values{}
	form.Set("flowId", flowID)
	form.Set("ids", zsID)

	requestCtx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, lookupURL.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return MerchantFoodPermitOfficialVerification{}, fmt.Errorf("build food permit official request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return MerchantFoodPermitOfficialVerification{}, fmt.Errorf("query food permit official endpoint: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return MerchantFoodPermitOfficialVerification{}, fmt.Errorf("query food permit official endpoint: status=%d", resp.StatusCode)
	}

	var payload merchantFoodPermitOfficialLookupResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return MerchantFoodPermitOfficialVerification{}, fmt.Errorf("decode food permit official response: %w", err)
	}
	if !payload.State || strings.TrimSpace(payload.Data.CompanyName) == "" {
		return MerchantFoodPermitOfficialVerification{}, ErrMerchantFoodPermitOfficialVerificationUnavailable
	}
	result := MerchantFoodPermitOfficialVerification{
		CompanyName:  strings.TrimSpace(payload.Data.CompanyName),
		OperatorName: strings.TrimSpace(firstTrimmed(payload.Data.OperatorName, payload.Data.LegalRepresentative)),
		PermitNo:     strings.TrimSpace(payload.Data.PermitNo),
		CreditCode:   strings.TrimSpace(payload.Data.CreditCode),
		Address:      strings.TrimSpace(payload.Data.Address),
		ValidTo:      merchantNormalizeOfficialPermitDate(payload.Data.ValidTo),
	}
	if merchantIsSuspiciousFoodPermitCompanyName(result.CompanyName) {
		return MerchantFoodPermitOfficialVerification{}, ErrMerchantFoodPermitOfficialVerificationUnavailable
	}
	return result, nil
}

func (v *MerchantFoodPermitOfficialVerifier) findAllowedQRCodeURL(rawResult []byte) (*url.URL, bool) {
	decoder := json.NewDecoder(bytes.NewReader(rawResult))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return nil, false
	}
	return v.findAllowedQRCodeURLInValue(payload)
}

func (v *MerchantFoodPermitOfficialVerifier) findAllowedQRCodeURLInValue(value any) (*url.URL, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for _, nested := range typed {
			if result, ok := v.findAllowedQRCodeURLInValue(nested); ok {
				return result, true
			}
		}
	case []any:
		for _, nested := range typed {
			if result, ok := v.findAllowedQRCodeURLInValue(nested); ok {
				return result, true
			}
		}
	case string:
		if result, ok := v.parseAllowedQRCodeURL(typed); ok {
			return result, true
		}
		if nested, ok := merchantDecodeRawJSONEmbeddedJSON(typed); ok {
			return v.findAllowedQRCodeURLInValue(nested)
		}
	}
	return nil, false
}

func (v *MerchantFoodPermitOfficialVerifier) parseAllowedQRCodeURL(raw string) (*url.URL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, false
	}
	if parsed.Scheme != v.allowedScheme || parsed.Host != v.allowedHost || parsed.Path != v.qrPath {
		return nil, false
	}
	if parsed.User != nil || parsed.RawQuery == "" {
		return nil, false
	}
	query := parsed.Query()
	flowID := strings.TrimSpace(query.Get("flowId"))
	zsID := strings.TrimSpace(query.Get("zsId"))
	if !merchantFoodPermitOfficialIDPattern.MatchString(flowID) || !merchantFoodPermitOfficialIDPattern.MatchString(zsID) {
		return nil, false
	}
	return parsed, true
}

func RepairMerchantFoodPermitFromOfficialVerification(foodPermit *MerchantReviewFoodPermitOCRData, verification MerchantFoodPermitOfficialVerification) ([]byte, bool, error) {
	if foodPermit == nil || strings.TrimSpace(verification.CompanyName) == "" {
		return nil, false, nil
	}

	changed := false
	if companyName := merchantNormalizeCompanyName(verification.CompanyName); companyName != "" && !merchantIsSuspiciousFoodPermitCompanyName(companyName) {
		if companyName != foodPermit.CompanyName {
			foodPermit.CompanyName = companyName
			changed = true
		}
	}
	if operatorName := strings.TrimSpace(verification.OperatorName); operatorName != "" && foodPermit.OperatorName == "" {
		foodPermit.OperatorName = operatorName
		changed = true
	}
	if permitNo := strings.TrimSpace(verification.PermitNo); permitNo != "" && foodPermit.PermitNo == "" {
		foodPermit.PermitNo = permitNo
		changed = true
	}
	if validTo := strings.TrimSpace(verification.ValidTo); validTo != "" && foodPermit.ValidTo == "" {
		foodPermit.ValidTo = validTo
		changed = true
	}
	rawText := merchantBuildFoodPermitOfficialRawText(foodPermit.RawText, verification)
	if rawText != "" && rawText != foodPermit.RawText {
		foodPermit.RawText = rawText
		changed = true
	}
	if !changed {
		return nil, false, nil
	}
	repairMerchantFoodPermitReadinessAfterOfficialVerification(foodPermit)
	payload, err := json.Marshal(foodPermit)
	if err != nil {
		return nil, false, err
	}
	return payload, true, nil
}

func repairMerchantFoodPermitReadinessAfterOfficialVerification(foodPermit *MerchantReviewFoodPermitOCRData) {
	if foodPermit == nil || foodPermit.Readiness == nil {
		return
	}
	if strings.TrimSpace(foodPermit.CompanyName) != "" {
		foodPermit.Readiness.MissingFields = merchantRemoveReadinessField(foodPermit.Readiness.MissingFields, "company_name")
		foodPermit.Readiness.UnparseableFields = merchantRemoveReadinessField(foodPermit.Readiness.UnparseableFields, "company_name")
	}
	if strings.TrimSpace(foodPermit.ValidTo) != "" {
		foodPermit.Readiness.MissingFields = merchantRemoveReadinessField(foodPermit.Readiness.MissingFields, "valid_to")
		foodPermit.Readiness.UnparseableFields = merchantRemoveReadinessField(foodPermit.Readiness.UnparseableFields, "valid_to")
	}
	if len(foodPermit.Readiness.MissingFields) == 0 && len(foodPermit.Readiness.UnparseableFields) == 0 {
		foodPermit.Readiness.State = merchantOCRReadinessStateReady
		foodPermit.Readiness.ReasonCode = "ok"
	}
}

func merchantBuildFoodPermitOfficialRawText(existing string, verification MerchantFoodPermitOfficialVerification) string {
	lines := make([]string, 0, 8)
	if trimmed := strings.TrimSpace(existing); trimmed != "" {
		lines = append(lines, trimmed)
	}
	appendLine := func(label string, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			lines = append(lines, label+"："+value)
		}
	}
	appendLine("官方核验主体名称", verification.CompanyName)
	appendLine("官方核验经营者姓名", verification.OperatorName)
	appendLine("官方核验登记证编号", verification.PermitNo)
	appendLine("官方核验统一社会信用代码", verification.CreditCode)
	appendLine("官方核验经营场所", verification.Address)
	appendLine("官方核验有效期至", verification.ValidTo)
	return strings.Join(lines, "\n")
}

func merchantNormalizeOfficialPermitDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := parseRiderFlexibleDocumentEndDate(raw)
	if err != nil {
		return raw
	}
	return parsed.Format("2006年01月02日")
}

var merchantFoodPermitOfficialIDPattern = regexp.MustCompile(`^\d{1,20}$`)

type merchantFoodPermitOfficialLookupResponse struct {
	State bool `json:"state"`
	Data  struct {
		CompanyName         string `json:"jyz"`
		OperatorName        string `json:"xcymc"`
		LegalRepresentative string `json:"fddbr"`
		PermitNo            string `json:"permitNumber"`
		CreditCode          string `json:"xcyshxydm"`
		Address             string `json:"jycs"`
		ValidTo             string `json:"yxrq"`
	} `json:"data"`
}
