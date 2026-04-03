package wechat

import (
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
