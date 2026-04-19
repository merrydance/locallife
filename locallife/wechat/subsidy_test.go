package wechat

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/stretchr/testify/require"
)

func TestCreateSubsidy_AllowsEmptyBody(t *testing.T) {
	client := newSignedEcommerceClientForTest(t, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, subsidyCreateURL, req.URL.Path)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	})

	resp, err := client.CreateSubsidy(context.Background(), wechatcontracts.SubsidyRequest{
		SubMchID:      "sub-mchid-001",
		TransactionID: "wx-transaction-001",
		Amount:        1,
		Description:   "平台补差",
		OutSubsidyNo:  "subsidy-1",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestCreateSubsidy_RejectsContractDrift(t *testing.T) {
	client := newSignedEcommerceClientForTest(t, func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"result":"UNKNOWN"}`)),
		}, nil
	})

	_, err := client.CreateSubsidy(context.Background(), wechatcontracts.SubsidyRequest{
		SubMchID:      "sub-mchid-001",
		TransactionID: "wx-transaction-001",
		Amount:        1,
		Description:   "平台补差",
		OutSubsidyNo:  "subsidy-1",
	})
	require.EqualError(t, err, "create subsidy: result has unsupported value \"UNKNOWN\"")
}

func TestReturnSubsidy_RejectsContractDrift(t *testing.T) {
	client := newSignedEcommerceClientForTest(t, func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"result":"SUCCESS","from":[{"amount":1}]}`)),
		}, nil
	})

	_, err := client.ReturnSubsidy(context.Background(), wechatcontracts.SubsidyReturnRequest{
		SubMchID:      "sub-mchid-001",
		OutOrderNo:    "order-1",
		TransactionID: "wx-transaction-001",
		Amount:        1,
		Description:   "退款退回补差",
	})
	require.EqualError(t, err, "return subsidy: from[0].account is required")
}

func TestCancelSubsidy_RejectsContractDrift(t *testing.T) {
	client := newSignedEcommerceClientForTest(t, func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"result":"UNKNOWN"}`)),
		}, nil
	})

	_, err := client.CancelSubsidy(context.Background(), wechatcontracts.SubsidyCancelRequest{
		SubMchID:      "sub-mchid-001",
		TransactionID: "wx-transaction-001",
		Description:   "operator cancel",
	})
	require.EqualError(t, err, "cancel subsidy: result has unsupported value \"UNKNOWN\"")
}
