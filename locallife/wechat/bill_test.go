package wechat

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeBillDownloadURLError_StatementCreating(t *testing.T) {
	err := fmt.Errorf("get bill download url: %w", normalizeBillDownloadURLError(&WechatPayError{
		StatusCode: 400,
		Code:       "STATEMENT_CREATING",
		Message:    "请求的账单正在生成中",
	}))

	require.ErrorIs(t, err, ErrBillNotReady)

	var wxErr *WechatPayError
	require.ErrorAs(t, err, &wxErr)
	require.Equal(t, "STATEMENT_CREATING", wxErr.Code)
}

func TestNormalizeBillDownloadURLError_Status404(t *testing.T) {
	err := fmt.Errorf("get bill download url: %w", normalizeBillDownloadURLError(errors.New("wechat pay api error: status=404, body=, request_id=req-1")))

	require.ErrorIs(t, err, ErrBillNotFound)
	require.Contains(t, err.Error(), "status=404")
}

func TestVerifyBillHash_SHA256(t *testing.T) {
	fileBytes := []byte("gzip-bill-content")
	sum := sha256.Sum256(fileBytes)

	err := verifyBillHash(fileBytes, "SHA256", fmt.Sprintf("%x", sum))
	require.NoError(t, err)
}

func TestVerifyBillHash_SHA1(t *testing.T) {
	fileBytes := []byte("gzip-bill-content")
	sum := sha1.Sum(fileBytes)

	err := verifyBillHash(fileBytes, "sha1", fmt.Sprintf("%x", sum))
	require.NoError(t, err)
}

func TestVerifyBillHash_Mismatch(t *testing.T) {
	err := verifyBillHash([]byte("gzip-bill-content"), "SHA256", "deadbeef")
	require.Error(t, err)
	require.Contains(t, err.Error(), "bill hash mismatch")
}

func TestVerifyBillHash_UnsupportedType(t *testing.T) {
	err := verifyBillHash([]byte("gzip-bill-content"), "MD5", "deadbeef")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported bill hash type")
}

func TestGetFundFlowBillDownloadURL(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewPaymentClient(PaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              "test_api_v3_key_32bytes_long__",
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedPaymentTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/bill/fundflowbill", req.URL.Path)
			require.Equal(t, "2026-04-10", req.URL.Query().Get("bill_date"))
			require.Equal(t, "BASIC", req.URL.Query().Get("account_type"))
			require.Equal(t, "GZIP", req.URL.Query().Get("tar_type"))

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"hash_type":"SHA1","hash_value":"abc123","download_url":"https://api.mch.weixin.qq.com/v3/billdownload/file?token=fund"}`)),
			}, nil
		}),
	}

	resp, err := client.GetFundFlowBillDownloadURL(context.Background(), time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), "BASIC", "GZIP")
	require.NoError(t, err)
	require.Equal(t, "SHA1", resp.HashType)
	require.Equal(t, "abc123", resp.HashValue)
	require.Equal(t, "https://api.mch.weixin.qq.com/v3/billdownload/file?token=fund", resp.DownloadURL)
}

func TestGetProfitSharingBillDownloadURL(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "service-mchid-001",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              "test_api_v3_key_32bytes_long__",
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
			require.Equal(t, "/v3/profitsharing/bills", req.URL.Path)
			require.Equal(t, "2026-04-10", req.URL.Query().Get("bill_date"))
			require.Equal(t, "19000000001", req.URL.Query().Get("sub_mchid"))
			require.Equal(t, "GZIP", req.URL.Query().Get("tar_type"))

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"hash_type":"SHA1","hash_value":"def456","download_url":"https://api.mch.weixin.qq.com/v3/billdownload/file?token=split"}`)),
			}, nil
		}),
	}

	resp, err := client.GetProfitSharingBillDownloadURL(context.Background(), time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), "19000000001", "GZIP")
	require.NoError(t, err)
	require.Equal(t, "SHA1", resp.HashType)
	require.Equal(t, "def456", resp.HashValue)
	require.Equal(t, "https://api.mch.weixin.qq.com/v3/billdownload/file?token=split", resp.DownloadURL)
}

func TestDownloadTradeBill_AcceptsAbsoluteDownloadURLFromBackupHost(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewPaymentClient(PaymentClientConfig{
		MchID:                 "test_mch_id",
		AppID:                 "test_app_id",
		SerialNumber:          "test_serial",
		APIV3Key:              "test_api_v3_key_32bytes_long__",
		PrivateKeyPath:        privateKeyPath,
		PlatformPublicKeyPath: publicKeyPath,
		PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
		NotifyURL:             "https://example.com/notify",
	})
	require.NoError(t, err)

	gzipBody := gzipBillFixture(t, "`商户订单号`,`微信订单号`,`订单金额`\n`order-001`,`wx-001`,`100.00`\n总金额,1\n")
	hash := sha1.Sum(gzipBody)

	client.httpClient = &http.Client{
		Transport: paymentRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v3/bill/tradebill":
				require.Equal(t, "2026-04-10", req.URL.Query().Get("bill_date"))
				require.Equal(t, "ALL", req.URL.Query().Get("bill_type"))
				require.Equal(t, "GZIP", req.URL.Query().Get("tar_type"))

				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"hash_type":"SHA1","hash_value":"%x","download_url":"https://api2.mch.weixin.qq.com/v3/billdownload/file?token=abc"}`, hash))),
				}
				return signTestHTTPResponse(t, platformPrivateKey, "PUB_KEY_ID_0123456789", resp), nil
			case "/v3/billdownload/file":
				require.Equal(t, "api2.mch.weixin.qq.com", req.URL.Host)
				require.Equal(t, "abc", req.URL.Query().Get("token"))
				require.NotEmpty(t, req.Header.Get("Authorization"))

				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader(gzipBody)),
				}, nil
			default:
				return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
			}
		}),
	}

	records, err := client.DownloadTradeBill(context.Background(), time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, BillRecord{
		OutTradeNo:    "order-001",
		TransactionID: "wx-001",
		Amount:        10000,
	}, records["order-001"])
}

func gzipBillFixture(t *testing.T, raw string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte(raw))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	return buf.Bytes()
}
