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
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
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

func TestMerchantCancelWithdrawEligibilityBlockedErrorUsesBlockReasonDescriptions(t *testing.T) {
	err := merchantCancelWithdrawEligibilityBlockedError(&wechatcontracts.CancelWithdrawEligibilityResponse{
		ValidateResult: "NOT_ALLOW_CANCEL_WITHDRAW",
		BlockReasons: []wechatcontracts.CancelWithdrawBlockReason{
			{Type: "CONSUMER_COMPLAINT_UNPROCESSED", Description: "消费者投诉未处理"},
			{Type: "HAS_BLOCKING_CONTROL", Description: "存在不可注销管控"},
		},
	})
	require.Equal(t, "merchant is not eligible for cancel withdraw: 消费者投诉未处理; 存在不可注销管控", err.Error())
}

func TestMerchantCancelWithdrawEligibilityBlockedErrorFallsBackToReasonType(t *testing.T) {
	err := merchantCancelWithdrawEligibilityBlockedError(&wechatcontracts.CancelWithdrawEligibilityResponse{
		ValidateResult: "NOT_ALLOW_CANCEL_WITHDRAW",
		BlockReasons: []wechatcontracts.CancelWithdrawBlockReason{
			{Type: "OTHER_REASON"},
		},
	})
	require.Equal(t, "merchant is not eligible for cancel withdraw: OTHER_REASON", err.Error())
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
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t,
		ErrMerchantCancelWithdrawServiceUnavailable.Message,
		decodeMerchantCancelWithdrawErrorResponse(t, recorder).Error,
	)
}

func TestRespondMerchantCancelWithdrawWechatError(t *testing.T) {
	testCases := []struct {
		name           string
		operation      string
		wxErr          *wechat.WechatPayError
		expectedStatus int
		expectedCode   int
		expectedError  string
	}{
		{
			name:           "ParamErrorMapsToBadRequest",
			operation:      "create_cancel_withdraw",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "PARAM_ERROR", Message: "参数错误"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMerchantCancelWithdrawWechatParamError.Error(),
		},
		{
			name:           "NoAuthMapsToForbidden",
			operation:      "create_cancel_withdraw",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusForbidden, Code: "NO_AUTH", Message: "商户权限异常"},
			expectedStatus: http.StatusForbidden,
			expectedCode:   ErrMerchantCancelWithdrawWechatNoAuth.Code,
			expectedError:  ErrMerchantCancelWithdrawWechatNoAuth.Message,
		},
		{
			name:           "AlreadyExistsMapsToConflict",
			operation:      "create_cancel_withdraw",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusConflict, Code: "ALREADY_EXISTS", Message: "申请单已存在"},
			expectedStatus: http.StatusConflict,
			expectedCode:   ErrMerchantCancelWithdrawApplicationExists.Code,
			expectedError:  ErrMerchantCancelWithdrawApplicationExists.Message,
		},
		{
			name:           "BizErrNeedRetryMapsToServiceUnavailable",
			operation:      "create_cancel_withdraw",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusServiceUnavailable, Code: "BIZ_ERR_NEED_RETRY", Message: "请稍后重试"},
			expectedStatus: http.StatusServiceUnavailable,
			expectedCode:   ErrMerchantCancelWithdrawWechatRetryLater.Code,
			expectedError:  ErrMerchantCancelWithdrawWechatRetryLater.Message,
		},
		{
			name:           "FrequencyLimitMapsToTooManyRequests",
			operation:      "create_cancel_withdraw",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusTooManyRequests, Code: "FREQUENCY_LIMIT", Message: "频率超限"},
			expectedStatus: http.StatusTooManyRequests,
			expectedCode:   ErrMerchantCancelWithdrawWechatFrequencyLimit.Code,
			expectedError:  ErrMerchantCancelWithdrawWechatFrequencyLimit.Message,
		},
		{
			name:           "UploadRateLimitMapsToTooManyRequests",
			operation:      "prepare_cancel_withdraw_request",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusTooManyRequests, Code: "FREQUENCY_LIMIT_EXCEED", Message: "上传频率超限"},
			expectedStatus: http.StatusTooManyRequests,
			expectedCode:   ErrMerchantCancelWithdrawWechatFrequencyLimit.Code,
			expectedError:  ErrMerchantCancelWithdrawWechatFrequencyLimit.Message,
		},
		{
			name:           "SystemErrorMapsToInternalServerError",
			operation:      "create_cancel_withdraw",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusInternalServerError, Code: "SYSTEM_ERROR", Message: "系统异常"},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   ErrMerchantCancelWithdrawWechatServiceUnavailable.Code,
			expectedError:  ErrMerchantCancelWithdrawWechatServiceUnavailable.Message,
		},
		{
			name:           "UndocumentedCodeMapsToBadGateway",
			operation:      "create_cancel_withdraw",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "UNKNOWN_CODE", Message: "unknown"},
			expectedStatus: http.StatusBadGateway,
			expectedCode:   ErrMerchantCancelWithdrawWechatInvalidResponse.Code,
			expectedError:  ErrMerchantCancelWithdrawWechatInvalidResponse.Message,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, recorder := newMerchantCancelWithdrawTestContext(t)
			handled := respondMerchantCancelWithdrawWechatError(ctx, tc.operation, 101, "1900000109", "MCW202604140001", fmt.Errorf("cancel withdraw failed: %w", tc.wxErr))

			require.True(t, handled)
			require.Len(t, ctx.Errors, 1)
			require.Equal(t, tc.expectedStatus, recorder.Code)

			resp := decodeMerchantCancelWithdrawErrorResponse(t, recorder)
			require.Equal(t, tc.expectedCode, resp.Code)
			require.Equal(t, tc.expectedError, resp.Error)
		})
	}
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
