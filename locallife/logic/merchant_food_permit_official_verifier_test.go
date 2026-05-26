package logic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMerchantFoodPermitOfficialVerifier_UsesAllowedQRCodeEndpoint(t *testing.T) {
	t.Parallel()

	var requestedPath string
	var requestedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		requestedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":true,"data":{"jyz":"宁晋县周鹏饭店","xcymc":"周松涛","permitNumber":"2130528020270","xcyshxydm":"92130528MA0A5XB46A","jycs":"河北省邢台市宁晋县经济开发区吉祥路与晶龙街交叉口东侧","yxrq":"2028-12-21 11:07:43"}}`))
	}))
	defer server.Close()

	baseURL := strings.TrimPrefix(server.URL, "http://")
	verifier := NewMerchantFoodPermitOfficialVerifier(MerchantFoodPermitOfficialVerifierConfig{
		HTTPClient:  server.Client(),
		AllowedHost: baseURL,
	})
	rawResult := []byte(`{"Data":"{\"codes\":[{\"data\":\"http://` + baseURL + `/OrcodeXcyXzf.jsp?flowId=86&zsId=655926252\",\"type\":\"QRcode\"}]}"}`)

	result, err := verifier.VerifyMerchantFoodPermit(context.Background(), rawResult)

	require.NoError(t, err)
	require.Equal(t, defaultMerchantFoodPermitLookupPath, requestedPath)
	require.Equal(t, "flowId=86&ids=655926252", requestedBody)
	require.Equal(t, "宁晋县周鹏饭店", result.CompanyName)
	require.Equal(t, "周松涛", result.OperatorName)
	require.Equal(t, "2130528020270", result.PermitNo)
	require.Equal(t, "92130528MA0A5XB46A", result.CreditCode)
	require.Equal(t, "2028年12月21日", result.ValidTo)
}

func TestMerchantFoodPermitOfficialVerifier_RejectsNonAllowlistedQRCodeURL(t *testing.T) {
	t.Parallel()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	verifier := NewMerchantFoodPermitOfficialVerifier(MerchantFoodPermitOfficialVerifierConfig{
		HTTPClient: server.Client(),
	})
	rawResult := []byte(`{"Data":"{\"codes\":[{\"data\":\"http://127.0.0.1:8081/OrcodeXcyXzf.jsp?flowId=86&zsId=655926252\",\"type\":\"QRcode\"}]}"}`)

	_, err := verifier.VerifyMerchantFoodPermit(context.Background(), rawResult)

	require.ErrorIs(t, err, ErrMerchantFoodPermitOfficialVerificationUnavailable)
	require.False(t, called)
}

func TestMerchantFoodPermitOfficialVerifier_DisablesRedirectsForInjectedClient(t *testing.T) {
	t.Parallel()

	redirected := false
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected = true
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectTarget.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	defer server.Close()

	baseURL := strings.TrimPrefix(server.URL, "http://")
	redirectFollowingClient := server.Client()
	redirectFollowingClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return nil
	}
	verifier := NewMerchantFoodPermitOfficialVerifier(MerchantFoodPermitOfficialVerifierConfig{
		HTTPClient:  redirectFollowingClient,
		AllowedHost: baseURL,
	})
	rawResult := []byte(`{"Data":"{\"codes\":[{\"data\":\"http://` + baseURL + `/OrcodeXcyXzf.jsp?flowId=86&zsId=655926252\",\"type\":\"QRcode\"}]}"}`)

	_, err := verifier.VerifyMerchantFoodPermit(context.Background(), rawResult)

	require.Error(t, err)
	require.False(t, redirected)
}

func TestRepairMerchantFoodPermitFromOfficialVerification_RepairsSuspiciousName(t *testing.T) {
	t.Parallel()

	foodPermit := MerchantReviewFoodPermitOCRData{
		CompanyName: "食品小作坊小餐饮登记证2130528020270",
		ValidTo:     "2028年12月21日",
		RawText:     "主体名称：食品小作坊小餐饮登记证2130528020270",
	}

	repairedPayload, changed, err := RepairMerchantFoodPermitFromOfficialVerification(&foodPermit, MerchantFoodPermitOfficialVerification{
		CompanyName:  "宁晋县周鹏饭店",
		OperatorName: "周松涛",
		PermitNo:     "2130528020270",
		CreditCode:   "92130528MA0A5XB46A",
		ValidTo:      "2028年12月21日",
	})

	require.NoError(t, err)
	require.True(t, changed)
	require.NotEmpty(t, repairedPayload)
	require.Equal(t, "宁晋县周鹏饭店", foodPermit.CompanyName)
	require.Equal(t, "周松涛", foodPermit.OperatorName)
	require.Equal(t, "2130528020270", foodPermit.PermitNo)
	require.Contains(t, foodPermit.RawText, "官方核验主体名称：宁晋县周鹏饭店")

	var persisted MerchantReviewFoodPermitOCRData
	require.NoError(t, json.Unmarshal(repairedPayload, &persisted))
	require.Equal(t, foodPermit.CompanyName, persisted.CompanyName)
}

func TestRepairMerchantFoodPermitFromOfficialVerification_OverridesMismatchedName(t *testing.T) {
	t.Parallel()

	foodPermit := MerchantReviewFoodPermitOCRData{
		CompanyName: "其他饭店",
		RawText:     "主体名称：其他饭店",
	}

	repairedPayload, changed, err := RepairMerchantFoodPermitFromOfficialVerification(&foodPermit, MerchantFoodPermitOfficialVerification{
		CompanyName: "宁晋县周鹏饭店",
	})

	require.NoError(t, err)
	require.True(t, changed)
	require.NotEmpty(t, repairedPayload)
	require.Equal(t, "宁晋县周鹏饭店", foodPermit.CompanyName)
}
