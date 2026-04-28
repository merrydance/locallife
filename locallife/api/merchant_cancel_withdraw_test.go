package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetMerchantCancelWithdrawEligibility_EcommerceClientUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/cancel-withdraw/eligibility", nil)
	require.NoError(t, err)
	ctx.Request = request

	server.getMerchantCancelWithdrawEligibility(ctx)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	resp := decodeMerchantCancelWithdrawErrorResponse(t, recorder)
	require.Equal(t, ErrMerchantCancelWithdrawServiceUnavailable.Message, resp.Error)
}

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
			expectedCode:   0,
			expectedError:  "internal server error",
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

func TestCreateMerchantCancelWithdrawApplicationRecordsAcceptedCommand(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{ID: 11, RegionID: 1, OwnerUserID: user.ID, Name: "测试商户", Status: "approved", IsOpen: true}
	paymentConfig := db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "1900000109", Status: "active"}
	record := db.MerchantCancelWithdrawApplication{
		ID:                         301,
		MerchantID:                 merchant.ID,
		CreatedByUserID:            user.ID,
		SubMchID:                   paymentConfig.SubMchID,
		OutRequestNo:               "MCW20260425001",
		Withdraw:                   db.MerchantCancelWithdrawModeNoWithdraw,
		ProofMediaAssetIds:         []byte(`[]`),
		AdditionalMaterialAssetIds: []byte(`[]`),
		LocalSyncState:             db.MerchantCancelWithdrawLocalSyncStateCreated,
		CreatedAt:                  time.Now(),
		UpdatedAt:                  time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(paymentConfig, nil)
	store.EXPECT().GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{SubjectType: "merchant", SubjectID: merchant.ID}).Return(db.EcommerceApplyment{OrganizationType: "4"}, nil)
	ecommerce.EXPECT().ValidateEcommerceCancelWithdraw(gomock.Any(), paymentConfig.SubMchID).Return(&wechatcontracts.CancelWithdrawEligibilityResponse{ValidateResult: "ALLOW_CANCEL_WITHDRAW"}, nil)
	store.EXPECT().GetMerchantCancelWithdrawApplicationByOutRequestNo(gomock.Any(), "MCW20260425001").Return(db.MerchantCancelWithdrawApplication{}, db.ErrRecordNotFound)
	store.EXPECT().CreateMerchantCancelWithdrawApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateMerchantCancelWithdrawApplicationParams) (db.MerchantCancelWithdrawApplication, error) {
		require.Equal(t, merchant.ID, arg.MerchantID)
		require.Equal(t, user.ID, arg.CreatedByUserID)
		require.Equal(t, paymentConfig.SubMchID, arg.SubMchID)
		require.Equal(t, "MCW20260425001", arg.OutRequestNo)
		require.Equal(t, db.MerchantCancelWithdrawModeNoWithdraw, arg.Withdraw)
		require.Equal(t, db.MerchantCancelWithdrawLocalSyncStateCreated, arg.LocalSyncState)
		return record, nil
	})
	ecommerce.EXPECT().CreateEcommerceCancelWithdraw(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechatcontracts.CancelWithdrawRequest) (*wechatcontracts.CancelWithdrawCreateResponse, error) {
		require.Equal(t, paymentConfig.SubMchID, req.SubMchID)
		require.Equal(t, "MCW20260425001", req.OutRequestNo)
		require.Equal(t, db.MerchantCancelWithdrawModeNoWithdraw, req.Withdraw)
		require.Nil(t, req.PayeeInfo)
		return &wechatcontracts.CancelWithdrawCreateResponse{ApplymentID: "applyment_301", OutRequestNo: req.OutRequestNo}, nil
	})
	ecommerce.EXPECT().QueryEcommerceCancelWithdrawByApplymentID(gomock.Any(), "applyment_301").Return(&wechatcontracts.CancelWithdrawQueryResponse{
		ApplymentID:            "applyment_301",
		OutRequestNo:           "MCW20260425001",
		SubMchID:               paymentConfig.SubMchID,
		CancelState:            db.MerchantCancelStateAccepted,
		CancelStateDescription: "已受理",
		Withdraw:               db.MerchantCancelWithdrawModeNoWithdraw,
	}, nil)
	queryResp := &wechatcontracts.CancelWithdrawQueryResponse{
		ApplymentID:            "applyment_301",
		OutRequestNo:           "MCW20260425001",
		SubMchID:               paymentConfig.SubMchID,
		CancelState:            db.MerchantCancelStateAccepted,
		CancelStateDescription: "已受理",
		Withdraw:               db.MerchantCancelWithdrawModeNoWithdraw,
	}
	expectMerchantCancelWithdrawQueryFact(t, store, record, queryResp, db.ExternalPaymentTerminalStatusProcessing, false)
	store.EXPECT().UpdateMerchantCancelWithdrawApplicationSync(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateMerchantCancelWithdrawApplicationSyncParams) (db.MerchantCancelWithdrawApplication, error) {
		require.Equal(t, record.ID, arg.ID)
		require.Equal(t, db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded, arg.LocalSyncState)
		require.True(t, arg.MarkSubmitted)
		require.Equal(t, "applyment_301", arg.ApplymentID.String)
		updated := record
		updated.ApplymentID = arg.ApplymentID
		updated.LocalSyncState = arg.LocalSyncState
		updated.SubmittedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return updated, nil
	})
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), int64(9102)).Return(db.ExternalPaymentFactApplication{
		ID:                 9102,
		FactID:             9101,
		Consumer:           merchantCancelWithdrawFactConsumerDomain,
		BusinessObjectType: merchantCancelWithdrawFactBusinessObject,
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), int64(9101)).Return(db.ExternalPaymentFact{
		ID:                   9101,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityCancelWithdraw,
		ExternalObjectType:   db.ExternalPaymentObjectCancelWithdraw,
		ExternalObjectKey:    record.OutRequestNo,
		ExternalSecondaryKey: pgtype.Text{String: queryResp.ApplymentID, Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectType:   pgtype.Text{String: merchantCancelWithdrawFactBusinessObject, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: record.ID, Valid: true},
		UpstreamState:        queryResp.CancelState,
		TerminalStatus:       db.ExternalPaymentTerminalStatusProcessing,
		IsTerminal:           false,
		RawResource:          merchantCancelWithdrawQueryFactResource(record, queryResp),
	}, nil)
	store.EXPECT().GetMerchantCancelWithdrawApplication(gomock.Any(), record.ID).Return(db.MerchantCancelWithdrawApplication{
		ID:             record.ID,
		MerchantID:     record.MerchantID,
		SubMchID:       record.SubMchID,
		OutRequestNo:   record.OutRequestNo,
		ApplymentID:    pgtype.Text{String: "applyment_301", Valid: true},
		LocalSyncState: db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded,
		SubmittedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}, nil)
	store.EXPECT().UpdateMerchantCancelWithdrawApplicationSync(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateMerchantCancelWithdrawApplicationSyncParams{})).DoAndReturn(func(_ context.Context, arg db.UpdateMerchantCancelWithdrawApplicationSyncParams) (db.MerchantCancelWithdrawApplication, error) {
		require.Equal(t, record.ID, arg.ID)
		require.Equal(t, db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded, arg.LocalSyncState)
		require.Equal(t, "applyment_301", arg.ApplymentID.String)
		require.Equal(t, db.MerchantCancelStateAccepted, arg.CancelState.String)
		require.False(t, arg.MarkSubmitted)
		updated := record
		updated.ApplymentID = arg.ApplymentID
		updated.LocalSyncState = arg.LocalSyncState
		updated.CancelState = arg.CancelState
		updated.CancelStateDescription = arg.CancelStateDescription
		updated.SubmittedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return updated, nil
	})
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateExternalPaymentFactProcessingStatusParams{})).Return(db.ExternalPaymentFact{ID: 9101}, nil)
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationAppliedParams{})).Return(db.ExternalPaymentFactApplication{ID: 9102}, nil)
	expectMerchantCancelWithdrawCommand(t, store, record.ID, "MCW20260425001", "applyment_301", db.ExternalPaymentCommandStatusAccepted, "", 9801)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)
	server.SetTaskDistributorForTest(nil)

	body := []byte(`{"out_request_no":"MCW20260425001","withdraw":"NOT_APPLY_WITHDRAW"}`)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/cancel-withdraw/applications", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusCreated, recorder.Code)

	var resp merchantCancelWithdrawCreateResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, record.ID, resp.Application.ID)
	require.Equal(t, db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded, resp.Application.LocalSyncState)
}

func TestGetMerchantCancelWithdrawApplicationSyncsLiveQueryAndRecordsFact(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{ID: 11, RegionID: 1, OwnerUserID: user.ID, Name: "测试商户", Status: "approved", IsOpen: true}
	record := db.MerchantCancelWithdrawApplication{
		ID:                         21,
		MerchantID:                 merchant.ID,
		SubMchID:                   "1900000109",
		OutRequestNo:               "MCW202604220001",
		ProofMediaAssetIds:         []byte(`[]`),
		AdditionalMaterialAssetIds: []byte(`[]`),
		LocalSyncState:             db.MerchantCancelWithdrawLocalSyncStateSubmitUnknown,
		CancelState:                pgtype.Text{String: db.MerchantCancelStateReviewing, Valid: true},
	}
	queryResp := &wechatcontracts.CancelWithdrawQueryResponse{
		ApplymentID:              "WX-CANCEL-21",
		OutRequestNo:             record.OutRequestNo,
		SubMchID:                 record.SubMchID,
		CancelState:              db.MerchantCancelStateFinish,
		CancelStateDescription:   "完成",
		WithdrawState:            db.MerchantCancelWithdrawStateSucceed,
		WithdrawStateDescription: "提现成功",
		ModifyTime:               "2026-04-26T12:00:00+08:00",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), merchant.ID).
		Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: record.SubMchID, Status: "active"}, nil)
	store.EXPECT().
		GetMerchantCancelWithdrawApplication(gomock.Any(), record.ID).
		Return(record, nil)
	ecommerce.EXPECT().QueryEcommerceCancelWithdrawByOutRequestNo(gomock.Any(), record.OutRequestNo).Return(queryResp, nil)
	expectMerchantCancelWithdrawQueryFact(t, store, record, queryResp, db.ExternalPaymentTerminalStatusSuccess, true)
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), int64(9102)).Return(db.ExternalPaymentFactApplication{
		ID:                 9102,
		FactID:             9101,
		Consumer:           merchantCancelWithdrawFactConsumerDomain,
		BusinessObjectType: merchantCancelWithdrawFactBusinessObject,
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), int64(9101)).Return(db.ExternalPaymentFact{
		ID:                   9101,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityCancelWithdraw,
		ExternalObjectType:   db.ExternalPaymentObjectCancelWithdraw,
		ExternalObjectKey:    record.OutRequestNo,
		ExternalSecondaryKey: pgtype.Text{String: queryResp.ApplymentID, Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectType:   pgtype.Text{String: merchantCancelWithdrawFactBusinessObject, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: record.ID, Valid: true},
		UpstreamState:        queryResp.CancelState,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
		RawResource:          merchantCancelWithdrawQueryFactResource(record, queryResp),
	}, nil)
	store.EXPECT().GetMerchantCancelWithdrawApplication(gomock.Any(), record.ID).Return(record, nil)
	store.EXPECT().
		UpdateMerchantCancelWithdrawApplicationSync(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateMerchantCancelWithdrawApplicationSyncParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateMerchantCancelWithdrawApplicationSyncParams) (db.MerchantCancelWithdrawApplication, error) {
			require.Equal(t, record.ID, arg.ID)
			require.Equal(t, db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded, arg.LocalSyncState)
			require.Equal(t, queryResp.ApplymentID, arg.ApplymentID.String)
			require.Equal(t, queryResp.CancelState, arg.CancelState.String)
			require.Equal(t, queryResp.WithdrawState, arg.WithdrawState.String)
			updated := record
			updated.ApplymentID = pgtype.Text{String: queryResp.ApplymentID, Valid: true}
			updated.LocalSyncState = db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded
			updated.CancelState = pgtype.Text{String: queryResp.CancelState, Valid: true}
			updated.CancelStateDescription = pgtype.Text{String: queryResp.CancelStateDescription, Valid: true}
			updated.WithdrawState = pgtype.Text{String: queryResp.WithdrawState, Valid: true}
			updated.WithdrawStateDescription = pgtype.Text{String: queryResp.WithdrawStateDescription, Valid: true}
			updated.LastQueryAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			return updated, nil
		})
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateExternalPaymentFactProcessingStatusParams{})).Return(db.ExternalPaymentFact{ID: 9101}, nil)
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationAppliedParams{})).Return(db.ExternalPaymentFactApplication{ID: 9102}, nil)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/cancel-withdraw/applications/21", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"applyment_id":"WX-CANCEL-21"`)
	require.Contains(t, recorder.Body.String(), `"cancel_state":"FINISH"`)
	require.Contains(t, recorder.Body.String(), `"withdraw_state":"WITHDRAW_SUCCEED"`)
}

func TestCreateMerchantCancelWithdrawApplicationRecordsUnknownCommand(t *testing.T) {
	testCreateMerchantCancelWithdrawApplicationRecordsSubmitFailureCommand(
		t,
		&wechat.WechatPayError{StatusCode: http.StatusServiceUnavailable, Code: "BIZ_ERR_NEED_RETRY", Message: "稍后重试"},
		db.MerchantCancelWithdrawLocalSyncStateSubmitUnknown,
		db.ExternalPaymentCommandStatusUnknown,
		http.StatusServiceUnavailable,
		9802,
	)
}

func TestCreateMerchantCancelWithdrawApplicationRecordsRejectedCommand(t *testing.T) {
	testCreateMerchantCancelWithdrawApplicationRecordsSubmitFailureCommand(
		t,
		&wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "PARAM_ERROR", Message: "参数错误"},
		db.MerchantCancelWithdrawLocalSyncStateSyncFailed,
		db.ExternalPaymentCommandStatusRejected,
		http.StatusBadRequest,
		9803,
	)
}

func testCreateMerchantCancelWithdrawApplicationRecordsSubmitFailureCommand(
	t *testing.T,
	createErr error,
	localSyncState string,
	commandStatus string,
	expectedHTTPStatus int,
	commandID int64,
) {
	t.Helper()
	user, _ := randomUser(t)
	merchant := db.Merchant{ID: 12, RegionID: 1, OwnerUserID: user.ID, Name: "测试商户", Status: "approved", IsOpen: true}
	paymentConfig := db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "1900000109", Status: "active"}
	record := db.MerchantCancelWithdrawApplication{
		ID:                         302,
		MerchantID:                 merchant.ID,
		CreatedByUserID:            user.ID,
		SubMchID:                   paymentConfig.SubMchID,
		OutRequestNo:               "MCW20260425002",
		Withdraw:                   db.MerchantCancelWithdrawModeNoWithdraw,
		ProofMediaAssetIds:         []byte(`[]`),
		AdditionalMaterialAssetIds: []byte(`[]`),
		LocalSyncState:             db.MerchantCancelWithdrawLocalSyncStateCreated,
		CreatedAt:                  time.Now(),
		UpdatedAt:                  time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(paymentConfig, nil)
	store.EXPECT().GetLatestEcommerceApplymentBySubject(gomock.Any(), db.GetLatestEcommerceApplymentBySubjectParams{SubjectType: "merchant", SubjectID: merchant.ID}).Return(db.EcommerceApplyment{OrganizationType: "4"}, nil)
	ecommerce.EXPECT().ValidateEcommerceCancelWithdraw(gomock.Any(), paymentConfig.SubMchID).Return(&wechatcontracts.CancelWithdrawEligibilityResponse{ValidateResult: "ALLOW_CANCEL_WITHDRAW"}, nil)
	store.EXPECT().GetMerchantCancelWithdrawApplicationByOutRequestNo(gomock.Any(), "MCW20260425002").Return(db.MerchantCancelWithdrawApplication{}, db.ErrRecordNotFound)
	store.EXPECT().CreateMerchantCancelWithdrawApplication(gomock.Any(), gomock.Any()).Return(record, nil)
	ecommerce.EXPECT().CreateEcommerceCancelWithdraw(gomock.Any(), gomock.Any()).Return(nil, createErr)
	ecommerce.EXPECT().QueryEcommerceCancelWithdrawByOutRequestNo(gomock.Any(), "MCW20260425002").Return(nil, errors.New("query timeout"))
	store.EXPECT().UpdateMerchantCancelWithdrawApplicationSync(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateMerchantCancelWithdrawApplicationSyncParams) (db.MerchantCancelWithdrawApplication, error) {
		require.Equal(t, record.ID, arg.ID)
		require.Equal(t, localSyncState, arg.LocalSyncState)
		updated := record
		updated.LocalSyncState = arg.LocalSyncState
		updated.LastError = arg.LastError
		return updated, nil
	})
	expectMerchantCancelWithdrawCommand(t, store, record.ID, "MCW20260425002", "", commandStatus, wechatErrorCode(createErr), commandID)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)
	server.SetTaskDistributorForTest(nil)

	body := []byte(`{"out_request_no":"MCW20260425002","withdraw":"NOT_APPLY_WITHDRAW"}`)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/merchant/finance/account/cancel-withdraw/applications", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, expectedHTTPStatus, recorder.Code)
}

func expectMerchantCancelWithdrawCommand(t *testing.T, store *mockdb.MockStore, applicationID int64, outRequestNo, applymentID, status, errorCode string, commandID int64) {
	t.Helper()
	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityCancelWithdraw, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreateCancelWithdraw, arg.CommandType)
		require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner)
		require.Equal(t, "merchant_cancel_withdraw_application", arg.BusinessObjectType.String)
		require.Equal(t, applicationID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectCancelWithdraw, arg.ExternalObjectType)
		require.Equal(t, outRequestNo, arg.ExternalObjectKey)
		require.Equal(t, status, arg.CommandStatus)
		if applymentID != "" {
			require.Equal(t, applymentID, arg.ExternalSecondaryKey.String)
		} else {
			require.False(t, arg.ExternalSecondaryKey.Valid)
		}
		if errorCode != "" {
			require.Equal(t, errorCode, arg.LastErrorCode.String)
			require.Contains(t, string(arg.ResponseSnapshot), errorCode)
		} else {
			require.False(t, arg.LastErrorCode.Valid)
		}
		snapshot := string(arg.ResponseSnapshot)
		require.Contains(t, snapshot, outRequestNo)
		require.NotContains(t, snapshot, "payee_info")
		require.NotContains(t, snapshot, "account_number")
		return db.ExternalPaymentCommand{ID: commandID}, nil
	})
}

func expectMerchantCancelWithdrawQueryFact(t *testing.T, store *mockdb.MockStore, record db.MerchantCancelWithdrawApplication, queryResp *wechatcontracts.CancelWithdrawQueryResponse, expectedTerminalStatus string, expectedTerminal bool) {
	t.Helper()
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentCapabilityCancelWithdraw, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectCancelWithdraw, arg.ExternalObjectType)
		require.Equal(t, record.OutRequestNo, arg.ExternalObjectKey)
		require.Equal(t, queryResp.ApplymentID, arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner.String)
		require.Equal(t, merchantCancelWithdrawFactBusinessObject, arg.BusinessObjectType.String)
		require.Equal(t, record.ID, arg.BusinessObjectID.Int64)
		require.Equal(t, queryResp.CancelState, arg.UpstreamState)
		require.Equal(t, expectedTerminalStatus, arg.TerminalStatus)
		require.Equal(t, expectedTerminal, arg.IsTerminal)
		require.Equal(t, merchantCancelWithdrawQueryFactDedupeKey(record.OutRequestNo, queryResp.CancelState, queryResp.WithdrawState, queryResp.ApplymentID), arg.DedupeKey)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(arg.RawResource, &payload))
		require.EqualValues(t, record.ID, payload["application_id"])
		require.EqualValues(t, record.MerchantID, payload["merchant_id"])
		require.Equal(t, record.SubMchID, payload["sub_mch_id"])
		require.Equal(t, queryResp.OutRequestNo, payload["out_request_no"])
		require.Equal(t, queryResp.ApplymentID, payload["applyment_id"])
		require.Equal(t, queryResp.CancelState, payload["cancel_state"])
		return db.ExternalPaymentFact{ID: 9101, TerminalStatus: arg.TerminalStatus, IsTerminal: arg.IsTerminal}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactApplicationParams{})).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactApplicationParams) (db.ExternalPaymentFactApplication, error) {
		require.Equal(t, int64(9101), arg.FactID)
		require.Equal(t, merchantCancelWithdrawFactConsumerDomain, arg.Consumer)
		require.Equal(t, merchantCancelWithdrawFactBusinessObject, arg.BusinessObjectType)
		require.Equal(t, record.ID, arg.BusinessObjectID)
		require.Equal(t, db.ExternalPaymentFactApplicationStatusPending, arg.Status)
		return db.ExternalPaymentFactApplication{ID: 9102, FactID: arg.FactID, Consumer: arg.Consumer, BusinessObjectType: arg.BusinessObjectType, BusinessObjectID: arg.BusinessObjectID, Status: arg.Status}, nil
	})
}

func wechatErrorCode(err error) string {
	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) && wxErr != nil {
		return wxErr.Code
	}
	return ""
}

func TestToMerchantCancelWithdrawItem_InvalidProofMediaJSONReturnsError(t *testing.T) {
	_, err := toMerchantCancelWithdrawItem(db.MerchantCancelWithdrawApplication{
		ID:                 301,
		OutRequestNo:       "MCW301",
		SubMchID:           "1900000109",
		ProofMediaAssetIds: []byte("{"),
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "unmarshal proof_media_asset_ids")
}

func TestGetMerchantCancelWithdrawApplication_InvalidStoredJSONReturnsInternalServerError(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          11,
		RegionID:    1,
		OwnerUserID: user.ID,
		Name:        "测试商户",
		Status:      "approved",
		IsOpen:      true,
	}
	record := db.MerchantCancelWithdrawApplication{
		ID:                 21,
		MerchantID:         merchant.ID,
		SubMchID:           "1900000109",
		OutRequestNo:       "MCW202604220001",
		CancelState:        pgtype.Text{String: db.MerchantCancelStateFinish, Valid: true},
		ProofMediaAssetIds: []byte("{"),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), merchant.ID).
		Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "1900000109", Status: "active"}, nil)
	store.EXPECT().
		GetMerchantCancelWithdrawApplication(gomock.Any(), record.ID).
		Return(record, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodGet, "/v1/merchant/finance/account/cancel-withdraw/applications/21", nil)
	require.NoError(t, err)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "id", Value: "21"}}
	ctx.Set(authorizationPayloadKey, &token.Payload{UserID: user.ID})

	server.getMerchantCancelWithdrawApplication(ctx)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	resp := decodeMerchantCancelWithdrawErrorResponse(t, recorder)
	require.Equal(t, "internal server error", resp.Error)
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
