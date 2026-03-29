package ocr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/merrydance/locallife/util"
)

type stubAliyunClient struct {
	raw json.RawMessage
	err error
}

func (c stubAliyunClient) Recognize(ctx context.Context, capability Capability, req RecognizeRequest) (json.RawMessage, error) {
	_ = ctx
	_ = capability
	_ = req
	return c.raw, c.err
}

func TestAliyunProviderRecognizeMapsErrorClasses(t *testing.T) {
	provider := NewAliyunProvider(stubAliyunClient{err: &AliyunAPIError{StatusCode: http.StatusTooManyRequests, Code: "Throttling.User", Message: "limit"}})
	_, err := provider.Recognize(context.Background(), CapabilityAliyunBusinessLicense, RecognizeRequest{DocumentType: DocumentTypeBusinessLicense})
	if !errors.Is(err, ErrAliyunOCRRateLimited) {
		t.Fatalf("expected rate limit error, got %v", err)
	}
}

func TestAliyunProviderRecognizeReturnsRawPayload(t *testing.T) {
	raw := json.RawMessage(`{"RequestId":"abc"}`)
	provider := NewAliyunProvider(stubAliyunClient{raw: raw})
	resp, err := provider.Recognize(context.Background(), CapabilityAliyunIDCard, RecognizeRequest{DocumentType: DocumentTypeIDCard, Side: DocumentSideFront})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if string(resp.RawResult) != string(raw) {
		t.Fatalf("raw result = %s, want %s", string(resp.RawResult), string(raw))
	}
	if resp.Provider != ProviderNameAliyun {
		t.Fatalf("provider = %s, want %s", resp.Provider, ProviderNameAliyun)
	}
	if resp.Normalized.DocumentType != DocumentTypeIDCard {
		t.Fatalf("document type = %s, want %s", resp.Normalized.DocumentType, DocumentTypeIDCard)
	}
}

func TestAliyunProviderRecognizeNormalizesFoodPermit(t *testing.T) {
	raw := json.RawMessage(`{
		"Data": {
			"operatorName": "本地生活餐饮店",
			"legalRepresentative": "张三",
			"businessAddress": "测试路1号",
			"licenceNumber": "JY12345678901234",
			"validToDate": "2027年01月08日"
		}
	}`)
	provider := NewAliyunProvider(stubAliyunClient{raw: raw})
	resp, err := provider.Recognize(context.Background(), CapabilityAliyunFoodPermit, RecognizeRequest{DocumentType: DocumentTypeFoodPermit})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.FoodPermit == nil {
		t.Fatal("expected normalized food permit result")
	}
	if resp.Normalized.FoodPermit.LicenseNumber != "JY12345678901234" {
		t.Fatalf("license number = %s", resp.Normalized.FoodPermit.LicenseNumber)
	}
	if resp.Normalized.FoodPermit.BusinessName != "本地生活餐饮店" {
		t.Fatalf("business name = %s", resp.Normalized.FoodPermit.BusinessName)
	}
	if resp.Normalized.FoodPermit.OperatorName != "张三" {
		t.Fatalf("operator name = %s", resp.Normalized.FoodPermit.OperatorName)
	}
	if resp.Normalized.FoodPermit.Address != "测试路1号" {
		t.Fatalf("address = %s", resp.Normalized.FoodPermit.Address)
	}
	if resp.Normalized.FoodPermit.ValidPeriod != "2027年01月08日" {
		t.Fatalf("valid period = %s", resp.Normalized.FoodPermit.ValidPeriod)
	}
	if !strings.Contains(resp.Normalized.FoodPermit.RawText, "许可证编号：JY12345678901234") {
		t.Fatalf("raw text = %s", resp.Normalized.FoodPermit.RawText)
	}
	if !strings.Contains(resp.Normalized.FoodPermit.RawText, "主体名称：本地生活餐饮店") {
		t.Fatalf("raw text = %s", resp.Normalized.FoodPermit.RawText)
	}
	if !strings.Contains(resp.Normalized.FoodPermit.RawText, "经营者姓名：张三") {
		t.Fatalf("raw text = %s", resp.Normalized.FoodPermit.RawText)
	}
}

func TestAliyunOpenAPIClientRecognizeFoodPermitUsesCorrectAction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if got := r.Header.Get("x-acs-action"); got != "RecognizeFoodManageLicense" {
			t.Fatalf("x-acs-action = %s", got)
		}
		if got := r.URL.RawQuery; got != "" {
			t.Fatalf("raw query = %q", got)
		}
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"RequestId":"food-123"}`))
	}))
	defer server.Close()

	client := &AliyunOpenAPIClient{
		endpoint:        server.URL,
		region:          "cn-hangzhou",
		accessKeyID:     "test-ak",
		accessKeySecret: "test-sk",
		httpClient:      server.Client(),
		clock: func() time.Time {
			return time.Date(2026, time.March, 29, 1, 2, 3, 0, time.UTC)
		},
		nonce: func() string {
			return "nonce-food-123"
		},
	}
	client.signer = client.defaultSigner

	_, err := client.Recognize(context.Background(), CapabilityAliyunFoodPermit, RecognizeRequest{
		DocumentType: DocumentTypeFoodPermit,
		ContentType:  "image/jpeg",
		Data:         []byte("food-license-bytes"),
	})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
}

func TestAliyunOpenAPIClientRecognizeHealthCertUsesRecognizeAdvanced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if got := r.Header.Get("x-acs-action"); got != "RecognizeAdvanced" {
			t.Fatalf("x-acs-action = %s", got)
		}
		if got := r.URL.RawQuery; got != "" {
			t.Fatalf("raw query = %q", got)
		}
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Data":"{\"content\":\"姓名：张三\\n健康证号：JK20260001\\n有效期至：2030年12月31日\\n110101199001011234\"}"}`))
	}))
	defer server.Close()

	client := &AliyunOpenAPIClient{
		endpoint:        server.URL,
		region:          "cn-hangzhou",
		accessKeyID:     "test-ak",
		accessKeySecret: "test-sk",
		httpClient:      server.Client(),
		clock: func() time.Time {
			return time.Date(2026, time.March, 29, 1, 2, 3, 0, time.UTC)
		},
		nonce: func() string {
			return "nonce-health-123"
		},
	}
	client.signer = client.defaultSigner

	_, err := client.Recognize(context.Background(), CapabilityAliyunHealthCert, RecognizeRequest{
		DocumentType: DocumentTypeHealthCert,
		ContentType:  "image/jpeg",
		Data:         []byte("health-cert-bytes"),
	})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
}

func TestAliyunOpenAPIClientRecognizeTreatsTopLevelCodeAsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"RequestId":"43A29C77-405E-4CC0-BC55-EE694AD00655","Data":"{\"content\":\"2017年河北区实验小学\",\"prism_wordsInfo\":[{\"word\":\"2017年河北区实验小学\"}]}","Code":"noPermission","Message":"You are not authorized to perform this operation."}`))
	}))
	defer server.Close()

	client := &AliyunOpenAPIClient{
		endpoint:        server.URL,
		region:          "cn-hangzhou",
		accessKeyID:     "test-ak",
		accessKeySecret: "test-sk",
		httpClient:      server.Client(),
		clock: func() time.Time {
			return time.Date(2026, time.March, 29, 1, 2, 3, 0, time.UTC)
		},
		nonce: func() string {
			return "nonce-health-err"
		},
	}
	client.signer = client.defaultSigner

	_, err := client.Recognize(context.Background(), CapabilityAliyunHealthCert, RecognizeRequest{
		DocumentType: DocumentTypeHealthCert,
		ContentType:  "image/jpeg",
		Data:         []byte("health-cert-bytes"),
	})
	if err == nil {
		t.Fatal("expected error when top-level Code is present")
	}
	if !errors.Is(MapAliyunOCRAPIError(err), ErrAliyunOCRForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
	if !strings.Contains(err.Error(), "noPermission") {
		t.Fatalf("error = %v, want noPermission", err)
	}
}

func TestAliyunProviderRecognizeNormalizesAdvancedHealthCertPrismWordsInfo(t *testing.T) {
	raw := json.RawMessage(`{
		"Data": "{\"content\":\"2017年河北区实验小学\",\"prism_wordsInfo\":[{\"word\":\"2017年河北区实验小学\"},{\"word\":\"常熟市人民法院\"},{\"word\":\"送达证\"}]}"
	}`)
	provider := NewAliyunProvider(stubAliyunClient{raw: raw})
	resp, err := provider.Recognize(context.Background(), CapabilityAliyunHealthCert, RecognizeRequest{DocumentType: DocumentTypeHealthCert})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.HealthCert == nil {
		t.Fatal("expected normalized health cert result")
	}
	if !strings.Contains(resp.Normalized.HealthCert.RawText, "常熟市人民法院") {
		t.Fatalf("raw text = %s", resp.Normalized.HealthCert.RawText)
	}
	if !strings.Contains(resp.Normalized.HealthCert.RawText, "送达证") {
		t.Fatalf("raw text = %s", resp.Normalized.HealthCert.RawText)
	}
}

func TestAliyunProviderRecognizeNormalizesBusinessLicense(t *testing.T) {
	raw := json.RawMessage(`{
		"Data": {
			"CreditCode": "91310000123456789A",
			"EnterpriseName": "本地生活科技有限公司",
			"LegalRepresentative": "张三",
			"Address": "测试路1号",
			"BusinessScope": "餐饮服务",
			"ValidPeriod": "2020-01-01 至 2040-01-01"
		}
	}`)
	provider := NewAliyunProvider(stubAliyunClient{raw: raw})
	resp, err := provider.Recognize(context.Background(), CapabilityAliyunBusinessLicense, RecognizeRequest{DocumentType: DocumentTypeBusinessLicense})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.BusinessLicense == nil {
		t.Fatal("expected normalized business license result")
	}
	if resp.Normalized.BusinessLicense.CreditCode != "91310000123456789A" {
		t.Fatalf("credit code = %s", resp.Normalized.BusinessLicense.CreditCode)
	}
	if resp.Normalized.BusinessLicense.EnterpriseName != "本地生活科技有限公司" {
		t.Fatalf("enterprise name = %s", resp.Normalized.BusinessLicense.EnterpriseName)
	}
	if resp.Normalized.BusinessLicense.ValidPeriod != "2020-01-01 至 2040-01-01" {
		t.Fatalf("valid period = %s", resp.Normalized.BusinessLicense.ValidPeriod)
	}
}

func TestAliyunProviderRecognizeNormalizesIDCard(t *testing.T) {
	raw := json.RawMessage(`{
		"Data": {
			"Name": "张三",
			"IdNumber": "110101199001011234",
			"Gender": "男",
			"Nation": "汉",
			"Address": "测试路1号",
			"ValidDate": "2020.01.01-2030.01.01"
		}
	}`)
	provider := NewAliyunProvider(stubAliyunClient{raw: raw})
	resp, err := provider.Recognize(context.Background(), CapabilityAliyunIDCard, RecognizeRequest{DocumentType: DocumentTypeIDCard, Side: DocumentSideFront})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.IDCard == nil {
		t.Fatal("expected normalized id card result")
	}
	if resp.Normalized.IDCard.Name != "张三" {
		t.Fatalf("name = %s", resp.Normalized.IDCard.Name)
	}
	if resp.Normalized.IDCard.IDNumber != "110101199001011234" {
		t.Fatalf("id number = %s", resp.Normalized.IDCard.IDNumber)
	}
	if resp.Normalized.IDCard.ValidPeriod != "2020.01.01-2030.01.01" {
		t.Fatalf("valid period = %s", resp.Normalized.IDCard.ValidPeriod)
	}
}

func TestAliyunProviderRecognizeNormalizesStringifiedBusinessLicenseData(t *testing.T) {
	raw := json.RawMessage(`{
		"Data": "{\"data\":{\"creditCode\":\"91130532MA7JHADD6E\",\"companyName\":\"平乡县牛门宴餐饮服务有限公司\",\"businessAddress\":\"河北省邢台市平乡县城向阳街中段东侧\",\"legalPerson\":\"陈洪刚\",\"businessScope\":\"正餐服务\",\"validPeriod\":\"2022年03月18日至长期\"},\"prism_keyValueInfo\":[{\"key\":\"creditCode\",\"value\":\"91130532MA7JHADD6E\"}]}"
	}`)
	provider := NewAliyunProvider(stubAliyunClient{raw: raw})
	resp, err := provider.Recognize(context.Background(), CapabilityAliyunBusinessLicense, RecognizeRequest{DocumentType: DocumentTypeBusinessLicense})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.BusinessLicense == nil {
		t.Fatal("expected normalized business license result")
	}
	if resp.Normalized.BusinessLicense.CreditCode != "91130532MA7JHADD6E" {
		t.Fatalf("credit code = %s", resp.Normalized.BusinessLicense.CreditCode)
	}
	if resp.Normalized.BusinessLicense.EnterpriseName != "平乡县牛门宴餐饮服务有限公司" {
		t.Fatalf("enterprise name = %s", resp.Normalized.BusinessLicense.EnterpriseName)
	}
	if resp.Normalized.BusinessLicense.LegalRepresentative != "陈洪刚" {
		t.Fatalf("legal representative = %s", resp.Normalized.BusinessLicense.LegalRepresentative)
	}
	if resp.Normalized.BusinessLicense.Address != "河北省邢台市平乡县城向阳街中段东侧" {
		t.Fatalf("address = %s", resp.Normalized.BusinessLicense.Address)
	}
	if resp.Normalized.BusinessLicense.BusinessScope != "正餐服务" {
		t.Fatalf("business scope = %s", resp.Normalized.BusinessLicense.BusinessScope)
	}
	if resp.Normalized.BusinessLicense.ValidPeriod != "2022年03月18日至长期" {
		t.Fatalf("valid period = %s", resp.Normalized.BusinessLicense.ValidPeriod)
	}
}

func TestAliyunProviderRecognizeNormalizesStringifiedFoodPermitData(t *testing.T) {
	raw := json.RawMessage(`{
		"Data": "{\"data\":{\"operatorName\":\"平乡县牛门宴餐饮服务有限公司\",\"legalRepresentative\":\"陈洪刚\",\"businessAddress\":\"河北省邢台市平乡县城向阳街中段陈侧\",\"businessScope\":\"热食类食品制售,冷食类食品制售\",\"licenceNumber\":\"JY21305320011643\",\"validToDate\":\"2027年04月23日\"},\"prism_keyValueInfo\":[{\"key\":\"licenceNumber\",\"value\":\"JY21305320011643\"}]}"
	}`)
	provider := NewAliyunProvider(stubAliyunClient{raw: raw})
	resp, err := provider.Recognize(context.Background(), CapabilityAliyunFoodPermit, RecognizeRequest{DocumentType: DocumentTypeFoodPermit})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.FoodPermit == nil {
		t.Fatal("expected normalized food permit result")
	}
	if resp.Normalized.FoodPermit.LicenseNumber != "JY21305320011643" {
		t.Fatalf("license number = %s", resp.Normalized.FoodPermit.LicenseNumber)
	}
	if resp.Normalized.FoodPermit.BusinessName != "平乡县牛门宴餐饮服务有限公司" {
		t.Fatalf("business name = %s", resp.Normalized.FoodPermit.BusinessName)
	}
	if resp.Normalized.FoodPermit.OperatorName != "陈洪刚" {
		t.Fatalf("operator name = %s", resp.Normalized.FoodPermit.OperatorName)
	}
	if resp.Normalized.FoodPermit.Address != "河北省邢台市平乡县城向阳街中段陈侧" {
		t.Fatalf("address = %s", resp.Normalized.FoodPermit.Address)
	}
	if resp.Normalized.FoodPermit.ValidPeriod != "2027年04月23日" {
		t.Fatalf("valid period = %s", resp.Normalized.FoodPermit.ValidPeriod)
	}
	if !strings.Contains(resp.Normalized.FoodPermit.RawText, "许可证编号：JY21305320011643") {
		t.Fatalf("raw text = %s", resp.Normalized.FoodPermit.RawText)
	}
}

func TestAliyunProviderRecognizeNormalizesStringifiedHealthCertData(t *testing.T) {
	raw := json.RawMessage(`{
		"Data": "{\"data\":{\"holderName\":\"张三\",\"certificateNumber\":\"JK20260001\",\"validPeriod\":\"2030年12月31日\",\"text\":\"姓名：张三\\n健康证号：JK20260001\\n有效期至：2030年12月31日\\n110101199001011234\"}}"
	}`)
	provider := NewAliyunProvider(stubAliyunClient{raw: raw})
	resp, err := provider.Recognize(context.Background(), CapabilityAliyunHealthCert, RecognizeRequest{DocumentType: DocumentTypeHealthCert})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.HealthCert == nil {
		t.Fatal("expected normalized health cert result")
	}
	if resp.Normalized.HealthCert.Name != "张三" {
		t.Fatalf("name = %s", resp.Normalized.HealthCert.Name)
	}
	if resp.Normalized.HealthCert.Certificate != "JK20260001" {
		t.Fatalf("certificate = %s", resp.Normalized.HealthCert.Certificate)
	}
	if resp.Normalized.HealthCert.ValidPeriod != "2030年12月31日" {
		t.Fatalf("valid period = %s", resp.Normalized.HealthCert.ValidPeriod)
	}
	if !strings.Contains(resp.Normalized.HealthCert.RawText, "110101199001011234") {
		t.Fatalf("raw text = %s", resp.Normalized.HealthCert.RawText)
	}
}

func TestAliyunProviderRecognizeNormalizesAdvancedHealthCertText(t *testing.T) {
	raw := json.RawMessage(`{
		"Data": "{\"content\":\"姓名：张三\\n健康证号：JK20260001\\n有效期至：2030年12月31日\\n110101199001011234\"}"
	}`)
	provider := NewAliyunProvider(stubAliyunClient{raw: raw})
	resp, err := provider.Recognize(context.Background(), CapabilityAliyunHealthCert, RecognizeRequest{DocumentType: DocumentTypeHealthCert})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.HealthCert == nil {
		t.Fatal("expected normalized health cert result")
	}
	if resp.Normalized.HealthCert.RawText == "" {
		t.Fatal("expected raw text from recognize advanced result")
	}
	if !strings.Contains(resp.Normalized.HealthCert.RawText, "健康证号：JK20260001") {
		t.Fatalf("raw text = %s", resp.Normalized.HealthCert.RawText)
	}
}

func TestAliyunProviderRecognizeNormalizesAdvancedHealthCertWordFragments(t *testing.T) {
	raw := json.RawMessage(`{
		"Data": "{\"prism_wordsInfo\":[{\"word\":\"姓名：\"},{\"word\":\"张三\"},{\"word\":\"健康证号：\"},{\"word\":\"JK20260001\"},{\"word\":\"有效期至：\"},{\"word\":\"2030年12月31日\"}]}"
	}`)
	provider := NewAliyunProvider(stubAliyunClient{raw: raw})
	resp, err := provider.Recognize(context.Background(), CapabilityAliyunHealthCert, RecognizeRequest{DocumentType: DocumentTypeHealthCert})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.HealthCert == nil {
		t.Fatal("expected normalized health cert result")
	}
	if !strings.Contains(resp.Normalized.HealthCert.RawText, "姓名：") {
		t.Fatalf("raw text = %s", resp.Normalized.HealthCert.RawText)
	}
	if !strings.Contains(resp.Normalized.HealthCert.RawText, "2030年12月31日") {
		t.Fatalf("raw text = %s", resp.Normalized.HealthCert.RawText)
	}
}

func TestAliyunProviderRecognizeNormalizesStringifiedIDCardFrontData(t *testing.T) {
	raw := json.RawMessage(`{
		"Data": "{\"data\":{\"face\":{\"data\":{\"name\":\"张泽强\",\"sex\":\"男\",\"ethnicity\":\"汉\",\"birthDate\":\"1978年11月6日\",\"address\":\"河北省邢台市宁晋县耿庄桥镇西周家庄村永富西街1胡同1号\",\"idNumber\":\"132229197811067791\"}}}}"
	}`)
	provider := NewAliyunProvider(stubAliyunClient{raw: raw})
	resp, err := provider.Recognize(context.Background(), CapabilityAliyunIDCard, RecognizeRequest{DocumentType: DocumentTypeIDCard, Side: DocumentSideFront})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.IDCard == nil {
		t.Fatal("expected normalized id card result")
	}
	if resp.Normalized.IDCard.Name != "张泽强" {
		t.Fatalf("name = %s", resp.Normalized.IDCard.Name)
	}
	if resp.Normalized.IDCard.IDNumber != "132229197811067791" {
		t.Fatalf("id number = %s", resp.Normalized.IDCard.IDNumber)
	}
	if resp.Normalized.IDCard.Gender != "男" {
		t.Fatalf("gender = %s", resp.Normalized.IDCard.Gender)
	}
	if resp.Normalized.IDCard.Ethnicity != "汉" {
		t.Fatalf("ethnicity = %s", resp.Normalized.IDCard.Ethnicity)
	}
	if resp.Normalized.IDCard.BirthDate != "1978年11月6日" {
		t.Fatalf("birth date = %s", resp.Normalized.IDCard.BirthDate)
	}
}

func TestAliyunProviderRecognizeNormalizesStringifiedIDCardBackData(t *testing.T) {
	raw := json.RawMessage(`{
		"Data": "{\"data\":{\"back\":{\"data\":{\"issueAuthority\":\"宁晋县公安局\",\"validPeriod\":\"2008.05.07-2028.05.07\"}}}}"
	}`)
	provider := NewAliyunProvider(stubAliyunClient{raw: raw})
	resp, err := provider.Recognize(context.Background(), CapabilityAliyunIDCard, RecognizeRequest{DocumentType: DocumentTypeIDCard, Side: DocumentSideBack})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.IDCard == nil {
		t.Fatal("expected normalized id card result")
	}
	if resp.Normalized.IDCard.Authority != "宁晋县公安局" {
		t.Fatalf("authority = %s", resp.Normalized.IDCard.Authority)
	}
	if resp.Normalized.IDCard.ValidPeriod != "2008.05.07-2028.05.07" {
		t.Fatalf("valid period = %s", resp.Normalized.IDCard.ValidPeriod)
	}
}

func TestMapAliyunOCRAPIError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "unauthorized", err: &AliyunAPIError{StatusCode: http.StatusUnauthorized, Code: "InvalidAccessKeyId.NotFound", Message: "bad ak"}, want: ErrAliyunOCRUnauthorized},
		{name: "forbidden", err: &AliyunAPIError{StatusCode: http.StatusForbidden, Code: "Forbidden.RAM", Message: "no permission"}, want: ErrAliyunOCRForbidden},
		{name: "rate limit", err: &AliyunAPIError{StatusCode: http.StatusTooManyRequests, Code: "Throttling.User", Message: "limit"}, want: ErrAliyunOCRRateLimited},
		{name: "server error", err: &AliyunAPIError{StatusCode: http.StatusBadGateway, Code: "InternalError", Message: "upstream"}, want: ErrAliyunOCRUnavailable},
		{name: "bad request", err: &AliyunAPIError{StatusCode: http.StatusBadRequest, Code: "InvalidParameter.ImageType", Message: "invalid"}, want: ErrAliyunOCRBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MapAliyunOCRAPIError(tc.err)
			if !errors.Is(got, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestAliyunOpenAPIClientRecognizeReturnsSigningError(t *testing.T) {
	client := &AliyunOpenAPIClient{
		endpoint:   "https://ocr-api.cn-hangzhou.aliyuncs.com",
		region:     "cn-hangzhou",
		httpClient: &http.Client{Timeout: time.Second},
		signer: func(httpReq *http.Request, payload []byte) error {
			_ = httpReq
			_ = payload
			return errors.New("signature mismatch")
		},
	}
	_, err := client.Recognize(context.Background(), CapabilityAliyunBusinessLicense, RecognizeRequest{DocumentType: DocumentTypeBusinessLicense, ContentType: "image/jpeg", Data: []byte("img")})
	if !errors.Is(err, ErrAliyunOCRSigning) {
		t.Fatalf("expected signing error, got %v", err)
	}
}

func TestAliyunOpenAPIClientRecognizeSignsAndSendsRequest(t *testing.T) {
	var capturedAuth string
	var capturedHash string
	var capturedNonce string
	var capturedDate string
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		capturedBody = payload
		if got := r.URL.RawQuery; got != "" {
			t.Fatalf("raw query = %q", got)
		}
		capturedAuth = r.Header.Get("Authorization")
		capturedHash = r.Header.Get("x-acs-content-sha256")
		capturedNonce = r.Header.Get("x-acs-signature-nonce")
		capturedDate = r.Header.Get("x-acs-date")
		if got := r.Header.Get("x-acs-action"); got != "RecognizeIdcard" {
			t.Fatalf("x-acs-action = %s", got)
		}
		if got := r.Header.Get("x-acs-version"); got != "2021-07-07" {
			t.Fatalf("x-acs-version = %s", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("content-type = %s", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("accept = %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"RequestId":"abc123"}`))
	}))
	defer server.Close()

	client := &AliyunOpenAPIClient{
		endpoint:        server.URL,
		region:          "cn-hangzhou",
		accessKeyID:     "test-ak",
		accessKeySecret: "test-sk",
		httpClient:      server.Client(),
		clock: func() time.Time {
			return time.Date(2026, time.March, 25, 3, 4, 5, 0, time.UTC)
		},
		nonce: func() string {
			return "nonce-123"
		},
	}
	client.signer = client.defaultSigner

	raw, err := client.Recognize(context.Background(), CapabilityAliyunIDCard, RecognizeRequest{
		DocumentType: DocumentTypeIDCard,
		Side:         DocumentSideFront,
		ContentType:  "image/jpeg",
		Data:         []byte("img-bytes"),
	})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if string(raw) != `{"RequestId":"abc123"}` {
		t.Fatalf("raw = %s", string(raw))
	}
	if capturedNonce != "nonce-123" {
		t.Fatalf("nonce = %s", capturedNonce)
	}
	if capturedDate != "2026-03-25T03:04:05Z" {
		t.Fatalf("date = %s", capturedDate)
	}
	if !strings.Contains(capturedAuth, "ACS3-HMAC-SHA256 Credential=test-ak") {
		t.Fatalf("authorization = %s", capturedAuth)
	}
	if strings.Contains(capturedAuth, "placeholder") {
		t.Fatalf("authorization should not contain placeholder signature: %s", capturedAuth)
	}
	if !strings.Contains(capturedAuth, "SignedHeaders=accept;content-type;host;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version") {
		t.Fatalf("authorization signed headers = %s", capturedAuth)
	}
	wantHash := sha256.Sum256(capturedBody)
	if capturedHash != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("x-acs-content-sha256 = %s", capturedHash)
	}
	if string(capturedBody) != "img-bytes" {
		t.Fatalf("raw request body = %q", string(capturedBody))
	}
}

func TestAliyunOpenAPIClientRecognizeRejectsEmptyBody(t *testing.T) {
	client := &AliyunOpenAPIClient{endpoint: "https://ocr-api.cn-hangzhou.aliyuncs.com", region: "cn-hangzhou"}
	_, err := client.Recognize(context.Background(), CapabilityAliyunBusinessLicense, RecognizeRequest{DocumentType: DocumentTypeBusinessLicense, MediaAssetID: 77})
	if err == nil || !strings.Contains(err.Error(), "request body is empty") {
		t.Fatalf("expected empty body error, got %v", err)
	}
}

func TestBuildAliyunCanonicalRequest_MatchesACS3Shape(t *testing.T) {
	reqURL, err := url.Parse("https://ocr-api.cn-hangzhou.aliyuncs.com")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	req := &http.Request{Method: http.MethodPost, URL: reqURL, Header: make(http.Header)}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "ocr-api.cn-hangzhou.aliyuncs.com")
	req.Header.Set("x-acs-action", "RecognizeBusinessLicense")
	req.Header.Set("x-acs-content-sha256", "73d25da2e8017fd7e5c6340f8033661057fc73a6384dd77fffa955e08bbf3cdc")
	req.Header.Set("x-acs-date", "2026-03-28T13:30:17Z")
	req.Header.Set("x-acs-signature-nonce", "1774704617315644886")
	req.Header.Set("x-acs-version", "2021-07-07")

	signedHeaders := []string{"accept", "content-type", "host", "x-acs-action", "x-acs-content-sha256", "x-acs-date", "x-acs-signature-nonce", "x-acs-version"}
	payloadHash := "73d25da2e8017fd7e5c6340f8033661057fc73a6384dd77fffa955e08bbf3cdc"

	got := buildAliyunCanonicalRequest(req, signedHeaders, payloadHash)
	want := strings.Join([]string{
		"POST",
		"/",
		"",
		"accept:application/json\ncontent-type:application/json\nhost:ocr-api.cn-hangzhou.aliyuncs.com\nx-acs-action:RecognizeBusinessLicense\nx-acs-content-sha256:73d25da2e8017fd7e5c6340f8033661057fc73a6384dd77fffa955e08bbf3cdc\nx-acs-date:2026-03-28T13:30:17Z\nx-acs-signature-nonce:1774704617315644886\nx-acs-version:2021-07-07\n",
		"accept;content-type;host;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version",
		payloadHash,
	}, "\n")

	if got != want {
		t.Fatalf("canonical request mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestNewAliyunProviderFromConfigValidatesConfig(t *testing.T) {
	_, err := NewAliyunProviderFromConfig(util.Config{AliyunOCREnabled: true})
	if err == nil {
		t.Fatal("expected config validation error")
	}
}
