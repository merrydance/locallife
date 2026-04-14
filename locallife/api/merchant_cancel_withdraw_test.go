package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	"github.com/stretchr/testify/require"
)

func TestValidateCreateMerchantCancelWithdrawRequestRejectsInvalidProofMaterialCounts(t *testing.T) {
	err := validateCreateMerchantCancelWithdrawRequest(createMerchantCancelWithdrawRequest{
		Withdraw:           db.MerchantCancelWithdrawModeWithdraw,
		PayeeInfo:          &merchantCancelWithdrawPayeeInfoRequest{AccountType: "ACCOUNT_TYPE_CORPORATE"},
		ProofMediaAssetIDs: []int64{1, 2},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "proof_media_asset_ids must not exceed 1 item")
}

func TestValidateCreateMerchantCancelWithdrawRequestRejectsInvalidPersonalIDDocType(t *testing.T) {
	err := validateCreateMerchantCancelWithdrawRequest(createMerchantCancelWithdrawRequest{
		Withdraw: db.MerchantCancelWithdrawModeWithdraw,
		PayeeInfo: &merchantCancelWithdrawPayeeInfoRequest{
			AccountType: "ACCOUNT_TYPE_PERSONAL",
			IdentityInfo: &merchantCancelWithdrawIdentityInfoRequest{
				IDDocType: "UNSUPPORTED",
			},
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "payee_info.identity_info.id_doc_type is unsupported")
}

func TestValidateCreateMerchantCancelWithdrawRequestRejectsCorporateIdentityInfo(t *testing.T) {
	err := validateCreateMerchantCancelWithdrawRequest(createMerchantCancelWithdrawRequest{
		Withdraw: db.MerchantCancelWithdrawModeWithdraw,
		PayeeInfo: &merchantCancelWithdrawPayeeInfoRequest{
			AccountType:  "ACCOUNT_TYPE_CORPORATE",
			IdentityInfo: &merchantCancelWithdrawIdentityInfoRequest{IDDocType: "IDENTIFICATION_TYPE_ID_CARD"},
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "payee_info.identity_info is only allowed for personal account")
}

func TestValidateCreateMerchantCancelWithdrawRequestRejectsUnsupportedBusinessLicenseStatusDeclaration(t *testing.T) {
	err := validateCreateMerchantCancelWithdrawRequest(createMerchantCancelWithdrawRequest{
		Withdraw:                         db.MerchantCancelWithdrawModeWithdraw,
		BusinessLicenseStatusDeclaration: "UNKNOWN",
		PayeeInfo:                        &merchantCancelWithdrawPayeeInfoRequest{AccountType: "ACCOUNT_TYPE_CORPORATE"},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "business_license_status_declaration must be ACTIVE, CANCELED, or REVOKED")
}

func TestValidateCreateMerchantCancelWithdrawRequestRejectsBusinessLicenseStatusDeclarationWhenNoWithdraw(t *testing.T) {
	err := validateCreateMerchantCancelWithdrawRequest(createMerchantCancelWithdrawRequest{
		Withdraw:                         db.MerchantCancelWithdrawModeNoWithdraw,
		BusinessLicenseStatusDeclaration: db.MerchantCancelWithdrawBusinessLicenseStatusCanceled,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "business_license_status_declaration must be empty when withdraw=NOT_APPLY_WITHDRAW")
}

func TestRespondMerchantCancelWithdrawRequestPreparationErrorReturnsValidationMessage(t *testing.T) {
	ctx, recorder := newMerchantCancelWithdrawTestContext(t)

	handled := respondMerchantCancelWithdrawRequestPreparationError(
		ctx,
		101,
		"1900000109",
		"MCW202604140001",
		&merchantCancelWithdrawRequestPreparationValidationError{Message: "media asset 1 not found"},
	)
	require.True(t, handled)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "media asset 1 not found", decodeMerchantCancelWithdrawErrorResponse(t, recorder).Error)
}

func TestRespondMerchantCancelWithdrawRequestPreparationErrorMapsWechatUploadFailure(t *testing.T) {
	ctx, recorder := newMerchantCancelWithdrawTestContext(t)

	err := &merchantCancelWithdrawUpstreamPreparationError{
		Operation: "upload media asset 1 to wechat",
		Err:       fmt.Errorf("request_id=req-1: upload rejected: %w", &wechat.WechatPayError{StatusCode: 400, Code: "PARAM_ERROR", Message: "bad request"}),
	}

	handled := respondMerchantCancelWithdrawRequestPreparationError(ctx, 101, "1900000109", "MCW202604140001", err)
	require.True(t, handled)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t,
		"WeChat rejected the cancel-withdraw request: check sub_mchid, out_request_no, payee info, proof materials, and additional materials before retrying",
		decodeMerchantCancelWithdrawErrorResponse(t, recorder).Error,
	)
}

func TestRespondMerchantCancelWithdrawRequestPreparationErrorHidesUnexpectedUpstreamFailure(t *testing.T) {
	ctx, recorder := newMerchantCancelWithdrawTestContext(t)

	err := &merchantCancelWithdrawUpstreamPreparationError{
		Operation: "upload media asset 1 to wechat",
		Err:       errors.New("upstream transport timeout"),
	}

	handled := respondMerchantCancelWithdrawRequestPreparationError(ctx, 101, "1900000109", "MCW202604140001", err)
	require.True(t, handled)
	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.Equal(t,
		"WeChat cancel-withdraw service is temporarily unavailable; retry later",
		decodeMerchantCancelWithdrawErrorResponse(t, recorder).Error,
	)
}

func newMerchantCancelWithdrawTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/merchant/finance/account/cancel-withdraw/applications", nil)
	ctx.Set(RequestIDKey, "req-test-1")
	return ctx, recorder
}

func decodeMerchantCancelWithdrawErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp
}
