package wechat

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type ecommerceRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn ecommerceRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+nX6sAAAAASUVORK5CYII="

func decodeTinyPNG(t *testing.T) []byte {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(tinyPNGBase64)
	require.NoError(t, err)
	return decoded
}

func minimalBMP() []byte {
	return []byte{
		0x42, 0x4d, 0x36, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x36, 0x00, 0x00, 0x00, 0x28, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x18, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
}

func parseUploadImageMultipartRequest(t *testing.T, req *http.Request) (map[string]string, []byte, string, string) {
	t.Helper()
	bodyBytes, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.NoError(t, req.Body.Close())
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	require.NotEmpty(t, params["boundary"])

	reader := multipart.NewReader(bytes.NewReader(bodyBytes), params["boundary"])
	meta := map[string]string{}
	var fileBytes []byte
	var fileName string
	var fileContentType string

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		partData, readErr := io.ReadAll(part)
		require.NoError(t, readErr)

		switch part.FormName() {
		case "meta":
			require.Equal(t, "application/json", part.Header.Get("Content-Type"))
			require.NoError(t, json.Unmarshal(partData, &meta))
		case "file":
			fileName = part.FileName()
			fileContentType = part.Header.Get("Content-Type")
			fileBytes = partData
		}
	}

	require.NotEmpty(t, meta)
	require.NotEmpty(t, fileName)
	require.NotEmpty(t, fileBytes)
	return meta, fileBytes, fileName, fileContentType
}

type scriptedNonceReader struct {
	callIndex   int
	failOnCalls map[int]error
}

func (r *scriptedNonceReader) Read(p []byte) (int, error) {
	r.callIndex++
	if err := r.failOnCalls[r.callIndex]; err != nil {
		return 0, err
	}
	for index := range p {
		p[index] = byte(r.callIndex + index + 1)
	}
	return len(p), nil
}

func withNonceRandomReaderForTest(t *testing.T, reader io.Reader) {
	t.Helper()
	previous := nonceRandomReader
	nonceRandomReader = reader
	t.Cleanup(func() {
		nonceRandomReader = previous
	})
}

func newTestUploadImageClient(t *testing.T, spMchID string) *EcommerceClient {
	t.Helper()

	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "base-mchid-001",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: spMchID,
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)
	return client
}

func TestUploadImage_SendsValidatedMultipartBodyWithServiceProviderMchID(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	fileBytes := decodeTinyPNG(t)
	expectedHash := sha256.Sum256(fileBytes)
	expectedHashHex := fmt.Sprintf("%x", expectedHash)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, merchantMediaUploadURL, req.URL.Path)
			require.Equal(t, "application/json", req.Header.Get("Accept"))
			require.NotEmpty(t, req.Header.Get("Request-ID"))
			authorization := req.Header.Get("Authorization")
			require.Contains(t, authorization, `mchid="service-mchid-001"`)
			require.NotContains(t, authorization, "ignored_base_mchid")

			meta, uploadedFile, uploadedFileName, uploadedContentType := parseUploadImageMultipartRequest(t, req)
			require.Equal(t, "tiny.png", meta["filename"])
			require.Equal(t, expectedHashHex, meta["sha256"])
			require.Equal(t, "tiny.png", uploadedFileName)
			require.Equal(t, "image/png", uploadedContentType)
			require.Equal(t, fileBytes, uploadedFile)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"media_id":"wx-media-id-001"}`)),
			}, nil
		}),
	}

	resp, err := client.UploadImage(context.Background(), "tiny.png", fileBytes)
	require.NoError(t, err)
	require.Equal(t, "wx-media-id-001", resp.MediaID)
}

func TestUploadImage_AcceptsBMPPayload(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	fileBytes := minimalBMP()
	expectedHash := sha256.Sum256(fileBytes)
	expectedHashHex := fmt.Sprintf("%x", expectedHash)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, merchantMediaUploadURL, req.URL.Path)

			meta, uploadedFile, uploadedFileName, uploadedContentType := parseUploadImageMultipartRequest(t, req)
			require.Equal(t, "tiny.bmp", meta["filename"])
			require.Equal(t, expectedHashHex, meta["sha256"])
			require.Equal(t, "tiny.bmp", uploadedFileName)
			require.Equal(t, "image/bmp", uploadedContentType)
			require.Equal(t, fileBytes, uploadedFile)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"media_id":"wx-media-id-bmp"}`)),
			}, nil
		}),
	}

	resp, err := client.UploadImage(context.Background(), "tiny.bmp", fileBytes)
	require.NoError(t, err)
	require.Equal(t, "wx-media-id-bmp", resp.MediaID)
}

func TestUploadImage_RejectsEmptyFile(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: ecommerceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}),
	}

	_, err = client.UploadImage(context.Background(), "empty.png", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "file is empty")
}

func TestUploadImage_RejectsNonImagePayload(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: ecommerceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}),
	}

	_, err = client.UploadImage(context.Background(), "fake.jpg", []byte("not-a-real-image"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "provide a real JPEG image")
}

func TestUploadImage_RejectsOversizedPayload(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	oversized := bytes.Repeat([]byte{0x01}, merchantMediaUploadMaxBytes+1)
	_, err = client.UploadImage(context.Background(), "too-large.png", oversized)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds the 2MB WeChat merchant media upload limit")
}

func TestUploadImage_RejectsMissingMediaIDInSuccessResponse(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	_, err = client.UploadImage(context.Background(), "tiny.png", decodeTinyPNG(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing media_id")
	require.Contains(t, err.Error(), "request_id=")
}

func TestUploadImage_WrapsWechatErrorsWithRequestID(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"code":"NO_AUTH","message":"商户权限异常"}`)),
			}, nil
		}),
	}

	_, err = client.UploadImage(context.Background(), "tiny.png", decodeTinyPNG(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "request_id=")
	require.Contains(t, err.Error(), "NO_AUTH")

	var wxErr *WechatPayError
	require.True(t, errors.As(err, &wxErr))
	require.Equal(t, "NO_AUTH", wxErr.Code)
	require.Equal(t, http.StatusForbidden, wxErr.StatusCode)
}

func TestUploadImage_RejectsImplicitServiceProviderMchIDFallback(t *testing.T) {
	client := newTestUploadImageClient(t, "")
	client.httpClient = &http.Client{
		Transport: ecommerceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}),
	}

	_, err := client.UploadImage(context.Background(), "tiny.png", decodeTinyPNG(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "request_id=")
	require.Contains(t, err.Error(), "service provider merchant id must be configured explicitly")
}

func TestUploadImage_ReturnsRequestIDWhenRequestIDGenerationFails(t *testing.T) {
	client := newTestUploadImageClient(t, "service-mchid-001")
	client.httpClient = &http.Client{
		Transport: ecommerceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}),
	}
	withNonceRandomReaderForTest(t, &scriptedNonceReader{failOnCalls: map[int]error{1: errors.New("rand unavailable")}})

	_, err := client.UploadImage(context.Background(), "tiny.png", decodeTinyPNG(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "request_id=merchant-media-upload-")
	require.Contains(t, err.Error(), "failed to generate request id")
}

func TestUploadImage_ReturnsRequestIDWhenSigningNonceGenerationFails(t *testing.T) {
	client := newTestUploadImageClient(t, "service-mchid-001")
	client.httpClient = &http.Client{
		Transport: ecommerceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}),
	}
	withNonceRandomReaderForTest(t, &scriptedNonceReader{failOnCalls: map[int]error{2: errors.New("rand unavailable")}})

	_, err := client.UploadImage(context.Background(), "tiny.png", decodeTinyPNG(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "request_id=")
	require.Contains(t, err.Error(), "failed to generate signing nonce")
}

func TestCreateEcommerceApplyment_SetsWechatpaySerialHeader(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "test_mch_id",
			AppID:                 "test_app_id",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "PUB_KEY_ID_0123456789", req.Header.Get("Wechatpay-Serial"))
			require.NotEmpty(t, req.Header.Get("Authorization"))

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "applyment-test-001", body["out_request_no"])
			require.Equal(t, "4", body["organization_type"])
			require.Equal(t, false, body["finance_institution"])
			require.NotContains(t, body, "need_account_info")

			accountInfo, ok := body["account_info"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "75", accountInfo["bank_account_type"])
			require.Equal(t, "402584040001", accountInfo["bank_branch_id"])

			contactInfo, ok := body["contact_info"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "65", contactInfo["contact_type"])

			idCardInfo, ok := body["id_card_info"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "2020-01-01", idCardInfo["id_card_valid_time_begin"])
			require.Equal(t, "长期", idCardInfo["id_card_valid_time"])

			salesSceneInfo, ok := body["sales_scene_info"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "测试门店", salesSceneInfo["store_name"])
			require.Equal(t, "https://example.com/store", salesSceneInfo["store_url"])

			settlementInfo, ok := body["settlement_info"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, float64(719), settlementInfo["settlement_id"])
			require.Equal(t, "餐饮", settlementInfo["qualification_type"])

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"applyment_id":123456789}`)),
			}, nil
		}),
	}

	resp, err := client.CreateEcommerceApplyment(context.Background(), &EcommerceApplymentRequest{
		OutRequestNo:       "applyment-test-001",
		OrganizationType:   "4",
		FinanceInstitution: false,
		BusinessLicense:    &BusinessLicenseInfo{BusinessLicenseCopy: "license_copy_media_id", BusinessLicenseNumber: "91440300TEST12345", MerchantName: "测试门店", LegalPerson: "张三", CompanyAddress: "深圳市南山区", BusinessTime: "[\"2020-01-01\",\"长期\"]"},
		MerchantShortname:  "测试运营商",
		IDCardInfo:         &ApplymentIDCardInfo{IDCardCopy: "copy_media_id", IDCardNational: "national_media_id", IDCardName: "encrypted_name", IDCardNumber: "encrypted_id_no", IDCardValidTimeBegin: "2020-01-01", IDCardValidTime: "长期"},
		AccountInfo:        &ApplymentBankAccountInfo{BankAccountType: "ACCOUNT_TYPE_PRIVATE", AccountBank: "其他银行", AccountName: "encrypted_account_name", BankAddressCode: "440300", BankBranchID: "402584040001", BankName: "深圳前海微众银行深圳南山支行", AccountNumber: "encrypted_account_no"},
		ContactInfo:        &ApplymentContactInfo{ContactType: "LEGAL", ContactName: "encrypted_contact_name", MobilePhone: "encrypted_mobile"},
		SalesSceneInfo:     &ApplymentSalesSceneInfo{StoreName: "测试门店", StoreURL: "https://example.com/store"},
		SettlementInfo:     &ApplymentSettlementInfo{SettlementID: 719, QualificationType: "餐饮"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(123456789), resp.ApplymentID)
}

func TestCreatePartnerJSAPIOrder_UsesDedicatedNotifyURL(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/fallback-notify",
		},
		SpMchID:          "service-mchid-001",
		SpAppID:          "service-appid-001",
		PartnerNotifyURL: "https://example.com/payment-notify",
		CombineNotifyURL: "https://example.com/combine-notify",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, ecommercePartnerJSAPIOrderURL, req.URL.Path)

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "https://example.com/payment-notify", body["notify_url"])
			require.Equal(t, "service-mchid-001", body["sp_mchid"])
			require.Equal(t, "sub-mchid-001", body["sub_mchid"])

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"prepay_id":"wx_partner_prepay_001"}`)),
			}, nil
		}),
	}

	resp, payParams, err := client.CreatePartnerJSAPIOrder(context.Background(), &PartnerJSAPIOrderRequest{
		SubMchID:      "sub-mchid-001",
		Description:   "测试普通支付",
		OutTradeNo:    "partner-order-001",
		ExpireTime:    time.Now().Add(30 * time.Minute),
		TotalAmount:   188,
		PayerOpenID:   "openid-001",
		ProfitSharing: true,
	})
	require.NoError(t, err)
	require.Equal(t, "wx_partner_prepay_001", resp.PrepayID)
	require.Equal(t, "prepay_id=wx_partner_prepay_001", payParams.Package)
}

func TestCreateCombineOrder_UsesServiceProviderAndSubMerchantFields(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/fallback-notify",
		},
		SpMchID:          "service-mchid-001",
		SpAppID:          "service-appid-001",
		PartnerNotifyURL: "https://example.com/payment-notify",
		CombineNotifyURL: "https://example.com/combine-notify",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, ecommerceCombineOrderURL, req.URL.Path)

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))

			require.Equal(t, "service-appid-001", body["combine_appid"])
			require.Equal(t, "service-mchid-001", body["combine_mchid"])
			require.Equal(t, "https://example.com/combine-notify", body["notify_url"])

			payerInfo, ok := body["combine_payer_info"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "openid-001", payerInfo["openid"])

			subOrders, ok := body["sub_orders"].([]any)
			require.True(t, ok)
			require.Len(t, subOrders, 1)

			subOrder, ok := subOrders[0].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "service-mchid-001", subOrder["mchid"])
			require.Equal(t, "sub-mchid-001", subOrder["sub_mchid"])
			require.Equal(t, "", subOrder["attach"])

			settleInfo, ok := subOrder["settle_info"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, true, settleInfo["profit_sharing"])

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"prepay_id":"wx_combined_prepay_001"}`)),
			}, nil
		}),
	}

	resp, payParams, err := client.CreateCombineOrder(context.Background(), &CombineOrderRequest{
		CombineOutTradeNo: "combine-order-001",
		SubOrders: []SubOrder{{
			SubMchID:      "sub-mchid-001",
			OutTradeNo:    "sub-order-001",
			Description:   "测试订单",
			Amount:        100,
			ProfitSharing: true,
			Attach:        "",
		}},
		PayerOpenID: "openid-001",
		ExpireTime:  time.Now().Add(30 * time.Minute),
		SceneInfo: &CombineSceneInfo{
			PayerClientIP: "127.0.0.1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "wx_combined_prepay_001", resp.PrepayID)
	require.Equal(t, "prepay_id=wx_combined_prepay_001", payParams.Package)
}

func TestQueryCombineOrder_ParsesServiceProviderFields(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/combine-transactions/out-trade-no/combine-order-001", req.URL.Path)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"combine_appid":"service-appid-001","combine_mchid":"service-mchid-001","combine_out_trade_no":"combine-order-001","combine_payer_info":{"openid":"openid-001","sub_openid":"sub-openid-001"},"scene_info":{"device_id":"POS-001"},"sub_orders":[{"mchid":"service-mchid-001","sub_mchid":"sub-mchid-001","sub_appid":"sub-appid-001","sub_openid":"sub-openid-001","out_trade_no":"sub-order-001","transaction_id":"wx_txn_001","trade_type":"JSAPI","trade_state":"SUCCESS","bank_type":"CMC","attach":"attach-001","amount":{"total_amount":100,"payer_amount":100,"currency":"CNY"},"success_time":"2024-11-14T10:00:00+08:00"}]}`)),
			}, nil
		}),
	}

	resp, err := client.QueryCombineOrder(context.Background(), "combine-order-001")
	require.NoError(t, err)
	require.Equal(t, "service-appid-001", resp.CombineAppID)
	require.Equal(t, "sub-openid-001", resp.CombinePayerInfo.SubOpenID)
	require.NotNil(t, resp.SceneInfo)
	require.Equal(t, "POS-001", resp.SceneInfo.DeviceID)
	require.Len(t, resp.SubOrders, 1)
	require.Equal(t, "sub-mchid-001", resp.SubOrders[0].SubMchID)
	require.Equal(t, "sub-appid-001", resp.SubOrders[0].SubAppID)
	require.Equal(t, "sub-openid-001", resp.SubOrders[0].SubOpenID)
	require.Equal(t, "JSAPI", resp.SubOrders[0].TradeType)
	require.Equal(t, "CMC", resp.SubOrders[0].BankType)
	require.Equal(t, "attach-001", resp.SubOrders[0].Attach)
}

func TestCloseCombineOrder_UsesSubMerchantFields(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, "/v3/combine-transactions/out-trade-no/combine-order-001/close", req.URL.Path)

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "service-appid-001", body["combine_appid"])

			subOrders, ok := body["sub_orders"].([]any)
			require.True(t, ok)
			require.Len(t, subOrders, 1)

			subOrder, ok := subOrders[0].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "service-mchid-001", subOrder["mchid"])
			require.Equal(t, "sub-mchid-001", subOrder["sub_mchid"])
			require.Equal(t, "sub-appid-001", subOrder["sub_appid"])
			require.Equal(t, "sub-order-001", subOrder["out_trade_no"])

			return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
		}),
	}

	err = client.CloseCombineOrder(context.Background(), "combine-order-001", []SubOrderClose{{
		SubMchID:   "sub-mchid-001",
		SubAppID:   "sub-appid-001",
		OutTradeNo: "sub-order-001",
	}})
	require.NoError(t, err)
}

func TestCreateEcommerceRefund_UsesLatestPlatformFields(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/default-refund-notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, ecommerceRefundURL, req.URL.Path)

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "sub-mchid-001", body["sub_mchid"])
			require.Equal(t, "service-appid-001", body["sp_appid"])
			require.Equal(t, "sub-appid-001", body["sub_appid"])
			require.Equal(t, "trade-001", body["out_trade_no"])
			require.Equal(t, "refund-001", body["out_refund_no"])
			require.Equal(t, "https://example.com/override-refund-notify", body["notify_url"])
			require.Equal(t, "REFUND_SOURCE_SUB_MERCHANT", body["refund_account"])

			amount, ok := body["amount"].(map[string]any)
			require.True(t, ok)
			require.EqualValues(t, 88, amount["refund"])
			require.EqualValues(t, 188, amount["total"])
			require.Equal(t, "CNY", amount["currency"])

			from, ok := amount["from"].([]any)
			require.True(t, ok)
			require.Len(t, from, 1)
			entry, ok := from[0].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "AVAILABLE", entry["account"])
			require.EqualValues(t, 88, entry["amount"])

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"refund_id":"refund-id-001","out_refund_no":"refund-001","create_time":"2024-11-04T10:34:56+08:00","amount":{"refund":88,"from":[{"account":"AVAILABLE","amount":88}],"payer_refund":88,"discount_refund":8,"currency":"CNY","advance":0},"promotion_detail":[{"promotion_id":"promo-001","scope":"SINGLE","type":"DISCOUNT","amount":8,"refund_amount":8}],"refund_account":"REFUND_SOURCE_SUB_MERCHANT"}`)),
			}, nil
		}),
	}

	resp, err := client.CreateEcommerceRefund(context.Background(), &EcommerceRefundRequest{
		SubMchID:     "sub-mchid-001",
		SubAppID:     "sub-appid-001",
		OutTradeNo:   "trade-001",
		OutRefundNo:  "refund-001",
		Reason:       "商品已售完",
		RefundAmount: 88,
		TotalAmount:  188,
		AmountFrom: []EcommerceRefundAmountFrom{{
			Account: "AVAILABLE",
			Amount:  88,
		}},
		NotifyURL:     "https://example.com/override-refund-notify",
		RefundAccount: "REFUND_SOURCE_SUB_MERCHANT",
	})
	require.NoError(t, err)
	require.Equal(t, "refund-id-001", resp.RefundID)
	require.Equal(t, "REFUND_SOURCE_SUB_MERCHANT", resp.RefundAccount)
	require.Len(t, resp.Amount.From, 1)
	require.Len(t, resp.PromotionDetail, 1)
}

func TestQueryEcommerceRefundByOutRefundNo_ParsesLatestFields(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/ecommerce/refunds/out-refund-no/refund-001", req.URL.Path)
			require.Equal(t, "sub_mchid=sub-mchid-001", req.URL.RawQuery)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"refund_id":"refund-id-001","out_refund_no":"refund-001","transaction_id":"wx-transaction-001","out_trade_no":"trade-001","channel":"ORIGINAL","user_received_account":"招商银行信用卡0403","success_time":"2024-11-04T11:00:00+08:00","create_time":"2024-11-04T10:34:56+08:00","status":"SUCCESS","amount":{"refund":88,"from":[{"account":"AVAILABLE","amount":88}],"payer_refund":80,"discount_refund":8,"currency":"CNY","advance":0},"promotion_detail":[{"promotion_id":"promo-001","scope":"SINGLE","type":"DISCOUNT","amount":8,"refund_amount":8}],"refund_account":"REFUND_SOURCE_SUB_MERCHANT","funds_account":"UNSETTLED"}`)),
			}, nil
		}),
	}

	resp, err := client.QueryEcommerceRefundByOutRefundNo(context.Background(), "sub-mchid-001", "refund-001")
	require.NoError(t, err)
	require.Equal(t, "wx-transaction-001", resp.TransactionID)
	require.Equal(t, "trade-001", resp.OutTradeNo)
	require.Equal(t, "ORIGINAL", resp.Channel)
	require.Equal(t, "招商银行信用卡0403", resp.UserReceivedAccount)
	require.Equal(t, "SUCCESS", resp.Status)
	require.Equal(t, "UNSETTLED", resp.FundsAccount)
	require.Len(t, resp.Amount.From, 1)
	require.Len(t, resp.PromotionDetail, 1)
}

func TestQueryEcommerceRefundByID_UsesRefundIDEndpoint(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/ecommerce/refunds/id/refund-id-001", req.URL.Path)
			require.Equal(t, "sub_mchid=sub-mchid-001", req.URL.RawQuery)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"refund_id":"refund-id-001","out_refund_no":"refund-001","status":"PROCESSING","create_time":"2024-11-04T10:34:56+08:00","amount":{"refund":88,"payer_refund":88}}`)),
			}, nil
		}),
	}

	resp, err := client.QueryEcommerceRefundByID(context.Background(), "sub-mchid-001", "refund-id-001")
	require.NoError(t, err)
	require.Equal(t, "refund-id-001", resp.RefundID)
	require.Equal(t, "refund-001", resp.OutRefundNo)
	require.Equal(t, RefundStatusProcessing, resp.Status)
}

func TestApplyEcommerceAbnormalRefund_UserBankCardEncryptsSensitiveFields(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, "/v3/ecommerce/refunds/refund-id-001/apply-abnormal-refund", req.URL.Path)
			require.Equal(t, "PUB_KEY_ID_0123456789", req.Header.Get("Wechatpay-Serial"))

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "sub-mchid-001", body["sub_mchid"])
			require.Equal(t, "refund-out-001", body["out_refund_no"])
			require.Equal(t, EcommerceAbnormalRefundTypeUserBankCard, body["type"])
			require.Equal(t, "ICBC_DEBIT", body["bank_type"])

			bankAccountCipher, ok := body["bank_account"].(string)
			require.True(t, ok)
			realNameCipher, ok := body["real_name"].(string)
			require.True(t, ok)
			require.NotEqual(t, "6222020202020202", bankAccountCipher)
			require.NotEqual(t, "张三", realNameCipher)

			decryptedBankAccount := decryptEcommerceTestCiphertext(t, platformPrivateKey, bankAccountCipher)
			decryptedRealName := decryptEcommerceTestCiphertext(t, platformPrivateKey, realNameCipher)
			require.Equal(t, "6222020202020202", decryptedBankAccount)
			require.Equal(t, "张三", decryptedRealName)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"refund_id":"refund-id-001","out_refund_no":"refund-out-001","status":"PROCESSING","create_time":"2025-06-06T10:34:56+08:00","amount":{"refund":88,"payer_refund":88}}`)),
			}, nil
		}),
	}

	resp, err := client.ApplyEcommerceAbnormalRefund(context.Background(), &EcommerceAbnormalRefundRequest{
		RefundID:    "refund-id-001",
		SubMchID:    "sub-mchid-001",
		OutRefundNo: "refund-out-001",
		Type:        EcommerceAbnormalRefundTypeUserBankCard,
		BankType:    "ICBC_DEBIT",
		BankAccount: "6222020202020202",
		RealName:    "张三",
	})
	require.NoError(t, err)
	require.Equal(t, "refund-id-001", resp.RefundID)
	require.Equal(t, RefundStatusProcessing, resp.Status)
}

func TestApplyEcommerceAbnormalRefund_MerchantBankCardUsesMinimalBody(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, "/v3/ecommerce/refunds/refund-id-002/apply-abnormal-refund", req.URL.Path)
			require.Equal(t, "PUB_KEY_ID_0123456789", req.Header.Get("Wechatpay-Serial"))

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "sub-mchid-002", body["sub_mchid"])
			require.Equal(t, "refund-out-002", body["out_refund_no"])
			require.Equal(t, EcommerceAbnormalRefundTypeMerchantBankCard, body["type"])
			_, hasBankType := body["bank_type"]
			_, hasBankAccount := body["bank_account"]
			_, hasRealName := body["real_name"]
			require.False(t, hasBankType)
			require.False(t, hasBankAccount)
			require.False(t, hasRealName)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"refund_id":"refund-id-002","out_refund_no":"refund-out-002","status":"SUCCESS","create_time":"2025-06-06T10:34:56+08:00","amount":{"refund":188,"payer_refund":188}}`)),
			}, nil
		}),
	}

	resp, err := client.ApplyEcommerceAbnormalRefund(context.Background(), &EcommerceAbnormalRefundRequest{
		RefundID:    "refund-id-002",
		SubMchID:    "sub-mchid-002",
		OutRefundNo: "refund-out-002",
		Type:        EcommerceAbnormalRefundTypeMerchantBankCard,
	})
	require.NoError(t, err)
	require.Equal(t, "refund-id-002", resp.RefundID)
	require.Equal(t, RefundStatusSuccess, resp.Status)
}

func TestCreateProfitSharing_EncryptsReceiverNameUsingLatestField(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, profitSharingURL, req.URL.Path)

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "service-appid-001", body["appid"])

			receivers, ok := body["receivers"].([]any)
			require.True(t, ok)
			require.Len(t, receivers, 1)

			receiver, ok := receivers[0].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "MERCHANT_ID", receiver["type"])
			require.Equal(t, "receiver-mchid-001", receiver["receiver_account"])
			require.NotContains(t, receiver, "encrypted_name")

			receiverNameCipher, ok := receiver["receiver_name"].(string)
			require.True(t, ok)
			require.NotEqual(t, "测试分账接收方", receiverNameCipher)
			require.Equal(t, "测试分账接收方", decryptEcommerceTestCiphertext(t, platformPrivateKey, receiverNameCipher))

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"sub_mchid":"sub-mchid-001","transaction_id":"wx-transaction-001","out_order_no":"ps-order-001","order_id":"ps-wx-001","status":"PROCESSING"}`)),
			}, nil
		}),
	}

	resp, err := client.CreateProfitSharing(context.Background(), &ProfitSharingRequest{
		SubMchID:      "sub-mchid-001",
		TransactionID: "wx-transaction-001",
		OutOrderNo:    "ps-order-001",
		Finish:        true,
		Receivers: []ProfitSharingReceiver{{
			Type:            ReceiverTypeMerchant,
			ReceiverAccount: "receiver-mchid-001",
			ReceiverName:    "测试分账接收方",
			Amount:          520,
			Description:     "平台分账",
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "ps-wx-001", resp.OrderID)
}

func TestAddProfitSharingReceiver_UsesNameFieldAndWechatpaySerial(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, profitSharingReceiverAddURL, req.URL.Path)
			require.Equal(t, "PUB_KEY_ID_0123456789", req.Header.Get("Wechatpay-Serial"))

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.NotContains(t, body, "encrypted_name")

			receiverNameCipher, ok := body["name"].(string)
			require.True(t, ok)
			require.Equal(t, "张三", decryptEcommerceTestCiphertext(t, platformPrivateKey, receiverNameCipher))

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"type":"PERSONAL_OPENID","account":"openid-001","relation_type":"OTHERS"}`)),
			}, nil
		}),
	}

	resp, err := client.AddProfitSharingReceiver(context.Background(), &AddReceiverRequest{
		AppID:        "service-appid-001",
		Type:         ReceiverTypePersonal,
		Account:      "openid-001",
		Name:         "张三",
		RelationType: RelationOthers,
	})
	require.NoError(t, err)
	require.Equal(t, ReceiverTypePersonal, resp.Type)
	require.Equal(t, RelationOthers, resp.RelationType)
}

func TestQueryProfitSharingReturn_UsesCollectionEndpointAndEscapedQuery(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, profitSharingReturnQueryURL, req.URL.Path)
			require.Equal(t, "sub-mchid-001", req.URL.Query().Get("sub_mchid"))
			require.Equal(t, "return no/001", req.URL.Query().Get("out_return_no"))
			require.Equal(t, "order no/001", req.URL.Query().Get("out_order_no"))

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"sub_mchid":"sub-mchid-001","out_order_no":"order no/001","out_return_no":"return no/001","return_no":"return-id-001","amount":520,"result":"PROCESSING"}`)),
			}, nil
		}),
	}

	resp, err := client.QueryProfitSharingReturn(context.Background(), "sub-mchid-001", "return no/001", "order no/001")
	require.NoError(t, err)
	require.Equal(t, "return-id-001", resp.ReturnID)
	require.Equal(t, "PROCESSING", resp.Result)
}

func TestQueryProfitSharingAmounts_UsesTransactionEndpoint(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/ecommerce/profitsharing/orders/wx-transaction-009/amounts", req.URL.Path)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"transaction_id":"wx-transaction-009","unsplit_amount":180}`)),
			}, nil
		}),
	}

	resp, err := client.QueryProfitSharingAmounts(context.Background(), "wx-transaction-009")
	require.NoError(t, err)
	require.Equal(t, "wx-transaction-009", resp.TransactionID)
	require.Equal(t, int64(180), resp.UnsplitAmount)
}

func TestIsProfitSharingReturnProcessingError(t *testing.T) {
	require.True(t, IsProfitSharingReturnProcessingError(&WechatPayError{Code: "NOT_ENOUGH", Message: "余额不足", StatusCode: http.StatusBadRequest}))
	require.True(t, IsProfitSharingReturnProcessingError(&WechatPayError{Code: "PAYER_ACCOUNT_ABNORMAL", Message: "分账方账户异常", StatusCode: http.StatusBadRequest}))
	require.False(t, IsProfitSharingReturnProcessingError(&WechatPayError{Code: "PARAM_ERROR", Message: "参数错误", StatusCode: http.StatusBadRequest}))
	require.False(t, IsProfitSharingReturnProcessingError(errors.New("network error")))
}

func TestQueryEcommerceFundBalanceByAccountType(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/ecommerce/fund/balance/sub-mchid-001", req.URL.Path)
			require.Equal(t, "account_type=FEES", req.URL.RawQuery)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"sub_mchid":"sub-mchid-001","available_amount":1234,"pending_amount":5,"account_type":"FEES"}`)),
			}, nil
		}),
	}

	resp, err := client.QueryEcommerceFundBalanceByAccountType(context.Background(), "sub-mchid-001", "FEES")
	require.NoError(t, err)
	require.Equal(t, "sub-mchid-001", resp.SubMchID)
	require.Equal(t, "FEES", resp.AccountType)
	require.Equal(t, int64(1234), resp.AvailableAmount)
	require.Equal(t, int64(1234), resp.WithdrawableAmount)
}

func TestQueryEcommerceFundDayEndBalance(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/ecommerce/fund/enddaybalance/sub-mchid-001", req.URL.Path)
			require.Equal(t, "account_type=DEPOSIT&date=2026-04-05", req.URL.RawQuery)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"sub_mchid":"sub-mchid-001","available_amount":88,"pending_amount":1,"account_type":"DEPOSIT"}`)),
			}, nil
		}),
	}

	resp, err := client.QueryEcommerceFundDayEndBalance(context.Background(), "sub-mchid-001", "2026-04-05", "DEPOSIT")
	require.NoError(t, err)
	require.Equal(t, "DEPOSIT", resp.AccountType)
	require.Equal(t, int64(88), resp.AvailableAmount)
	require.Equal(t, int64(88), resp.WithdrawableAmount)
}

func TestQueryPlatformFundBalance(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/merchant/fund/balance/OPERATION", req.URL.Path)
			require.Empty(t, req.URL.RawQuery)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"available_amount":5000,"pending_amount":20}`)),
			}, nil
		}),
	}

	resp, err := client.QueryPlatformFundBalance(context.Background(), "OPERATION")
	require.NoError(t, err)
	require.Equal(t, "OPERATION", resp.AccountType)
	require.Equal(t, int64(5000), resp.AvailableAmount)
	require.Equal(t, int64(20), resp.PendingAmount)
}

func TestQueryPlatformFundDayEndBalance(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/merchant/fund/dayendbalance/FEES", req.URL.Path)
			require.Equal(t, "date=2026-04-05", req.URL.RawQuery)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"available_amount":7000,"pending_amount":0}`)),
			}, nil
		}),
	}

	resp, err := client.QueryPlatformFundDayEndBalance(context.Background(), "FEES", "2026-04-05")
	require.NoError(t, err)
	require.Equal(t, "FEES", resp.AccountType)
	require.Equal(t, int64(7000), resp.AvailableAmount)
	require.Equal(t, int64(0), resp.PendingAmount)
}

func TestCreateEcommerceWithdraw_UsesDedicatedNotifyURL(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/fallback-notify",
		},
		SpMchID:           "service-mchid-001",
		SpAppID:           "service-appid-001",
		WithdrawNotifyURL: "https://example.com/withdraw-notify",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, ecommerceFundWithdrawURL, req.URL.Path)

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "1900000109", body["sub_mchid"])
			require.Equal(t, "MW202604060001", body["out_request_no"])
			require.Equal(t, float64(1200), body["amount"])
			require.Equal(t, "商户提现", body["remark"])
			require.Equal(t, "https://example.com/withdraw-notify", body["notify_url"])

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"sub_mchid":"1900000109","withdraw_id":"wd_001","out_request_no":"MW202604060001"}`)),
			}, nil
		}),
	}

	resp, err := client.CreateEcommerceWithdraw(context.Background(), &EcommerceWithdrawRequest{
		SubMchID:     "1900000109",
		OutRequestNo: "MW202604060001",
		Amount:       1200,
		Remark:       "商户提现",
	})
	require.NoError(t, err)
	require.Equal(t, "wd_001", resp.WithdrawID)
	require.Equal(t, "MW202604060001", resp.OutRequestNo)
}

func TestCreateViolationNotification_UsesConfiguredNotifyURL(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/fallback-notify",
		},
		SpMchID:            "service-mchid-001",
		SpAppID:            "service-appid-001",
		ViolationNotifyURL: "https://example.com/violation-notify",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, violationNotificationURL, req.URL.Path)

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "https://example.com/violation-notify", body["notify_url"])

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"notify_url":"https://example.com/violation-notify"}`)),
			}, nil
		}),
	}

	resp, err := client.CreateViolationNotification(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, resp.NotifyURL)
	require.Equal(t, "https://example.com/violation-notify", *resp.NotifyURL)
}

func TestQueryViolationNotification(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, violationNotificationURL, req.URL.Path)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"notify_url":"https://example.com/current-violation-notify"}`)),
			}, nil
		}),
	}

	resp, err := client.QueryViolationNotification(context.Background())
	require.NoError(t, err)
	require.NotNil(t, resp.NotifyURL)
	require.Equal(t, "https://example.com/current-violation-notify", *resp.NotifyURL)
}

func TestUpdateViolationNotification_UsesExplicitNotifyURL(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		},
		SpMchID:            "service-mchid-001",
		SpAppID:            "service-appid-001",
		ViolationNotifyURL: "https://example.com/fallback-violation-notify",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPut, req.Method)
			require.Equal(t, violationNotificationURL, req.URL.Path)

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "https://example.com/override-violation-notify", body["notify_url"])

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"notify_url":"https://example.com/override-violation-notify"}`)),
			}, nil
		}),
	}

	notifyURL := "https://example.com/override-violation-notify"
	resp, err := client.UpdateViolationNotification(context.Background(), &ViolationNotificationConfigRequest{NotifyURL: &notifyURL})
	require.NoError(t, err)
	require.NotNil(t, resp.NotifyURL)
	require.Equal(t, notifyURL, *resp.NotifyURL)
}

func TestDeleteViolationNotification(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodDelete, req.Method)
			require.Equal(t, violationNotificationURL, req.URL.Path)

			return &http.Response{
				StatusCode: http.StatusNoContent,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
	}

	require.NoError(t, client.DeleteViolationNotification(context.Background()))
}

func TestDecryptViolationNotification(t *testing.T) {
	validAPIV3Key := testAPIV3Key()

	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "ignored_base_mchid",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              validAPIV3Key,
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)
	client.apiV3Key = validAPIV3Key

	plaintext := `{"sub_mchid":"1900009231","company_name":"财付通支付科技有限公司","record_id":"200201820200101080076610000","punish_plan":"关闭支付权限","punish_time":"2015-05-20T13:29:35+08:00","punish_description":"利用特殊行业违规经营，加重处罚","risk_type":"ONE_YUAN_PURCHASES","risk_description":"涉嫌一元购"}`
	nonce := "0123456789ab"
	associatedData := "violation"
	ciphertext := encryptEcommerceNotificationResource(t, validAPIV3Key, plaintext, associatedData, nonce)

	notification := &PaymentNotification{ID: "notif_001", EventType: "VIOLATION.PUNISH", ResourceType: "encrypt-resource"}
	notification.Resource.Algorithm = "AEAD_AES_256_GCM"
	notification.Resource.Ciphertext = ciphertext
	notification.Resource.Nonce = nonce
	notification.Resource.AssociatedData = associatedData

	resource, err := client.DecryptViolationNotification(notification)
	require.NoError(t, err)
	require.Equal(t, "1900009231", resource.SubMchID)
	require.Equal(t, "财付通支付科技有限公司", resource.CompanyName)
	require.Equal(t, "200201820200101080076610000", resource.RecordID)
	require.Equal(t, "关闭支付权限", resource.PunishPlan)
	require.Equal(t, "利用特殊行业违规经营，加重处罚", resource.PunishDescription)
	require.Equal(t, "ONE_YUAN_PURCHASES", resource.RiskType)
	require.Equal(t, "涉嫌一元购", resource.RiskDescription)
	require.Equal(t, "2015-05-20T13:29:35+08:00", resource.PunishTime.Format(time.RFC3339))
}

func decryptEcommerceTestCiphertext(t *testing.T, privateKey *rsa.PrivateKey, ciphertext string) string {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	require.NoError(t, err)
	plaintext, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, privateKey, decoded, nil)
	require.NoError(t, err)
	return string(plaintext)
}

func TestQuerySubMerchantSettlement_UsesAccountNumberRule(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "test_mch_id",
			AppID:                 "test_app_id",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/apply4sub/sub_merchants/1900006491/settlement", req.URL.Path)
			require.Equal(t, "account_number_rule=ACCOUNT_NUMBER_RULE_MASK_V2", req.URL.RawQuery)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"account_type":"ACCOUNT_TYPE_BUSINESS","account_bank":"工商银行","account_number":"622202******8888","verify_result":"VERIFY_SUCCESS"}`)),
			}, nil
		}),
	}

	resp, err := client.QuerySubMerchantSettlement(context.Background(), "1900006491", "ACCOUNT_NUMBER_RULE_MASK_V2")
	require.NoError(t, err)
	require.Equal(t, "ACCOUNT_TYPE_BUSINESS", resp.AccountType)
	require.Equal(t, "VERIFY_SUCCESS", resp.VerifyResult)
}

func TestModifySubMerchantSettlement_PostsEncryptedPayload(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "test_mch_id",
			AppID:                 "test_app_id",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, "/v3/apply4sub/sub_merchants/1900006491/modify-settlement", req.URL.Path)
			require.Equal(t, "PUB_KEY_ID_0123456789", req.Header.Get("Wechatpay-Serial"))

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "ACCOUNT_TYPE_BUSINESS", body["account_type"])
			require.Equal(t, "工商银行", body["account_bank"])
			require.Equal(t, "402713354941", body["bank_branch_id"])
			require.Equal(t, "cipher-account-number", body["account_number"])
			require.Equal(t, "cipher-account-name", body["account_name"])

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"application_no":"102329389XXXX"}`)),
			}, nil
		}),
	}

	resp, err := client.ModifySubMerchantSettlement(context.Background(), "1900006491", &ModifySubMerchantSettlementRequest{
		AccountType:   "ACCOUNT_TYPE_BUSINESS",
		AccountBank:   "工商银行",
		BankBranchID:  "402713354941",
		AccountNumber: "cipher-account-number",
		AccountName:   "cipher-account-name",
	})
	require.NoError(t, err)
	require.Equal(t, "102329389XXXX", resp.ApplicationNo)
}

func TestQuerySubMerchantSettlementApplication_UsesApplicationAndMaskRule(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "test_mch_id",
			AppID:                 "test_app_id",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/apply4sub/sub_merchants/1511101111/application/102329389XXXX", req.URL.Path)
			require.Equal(t, "account_number_rule=ACCOUNT_NUMBER_RULE_MASK_V2", req.URL.RawQuery)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"account_name":"张*","account_type":"ACCOUNT_TYPE_BUSINESS","account_bank":"工商银行","account_number":"622202******8888","verify_result":"VERIFY_SUCCESS","verify_finish_time":"2015-05-20T13:29:35+08:00"}`)),
			}, nil
		}),
	}

	resp, err := client.QuerySubMerchantSettlementApplication(context.Background(), "1511101111", "102329389XXXX", "ACCOUNT_NUMBER_RULE_MASK_V2")
	require.NoError(t, err)
	require.Equal(t, "张*", resp.AccountName)
	require.Equal(t, "VERIFY_SUCCESS", resp.VerifyResult)
}

func encryptEcommerceNotificationResource(t *testing.T, apiV3Key, plaintext, associatedData, nonce string) string {
	t.Helper()
	block, err := aes.NewCipher([]byte(apiV3Key))
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	ciphertext := gcm.Seal(nil, []byte(nonce), []byte(plaintext), []byte(associatedData))
	return base64.StdEncoding.EncodeToString(ciphertext)
}
