package wechat

import (
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"

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
