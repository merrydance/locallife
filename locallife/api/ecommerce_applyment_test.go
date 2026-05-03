package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	mockordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 测试辅助函数 ====================

// newTestServerWithEcommerce 创建带 ecommerceClient 的测试服务器
func newTestServerWithEcommerce(t *testing.T, store db.Store, ecommerceClient wechat.EcommerceClientInterface) *Server {
	config := util.Config{
		Environment:         "test",
		TokenSymmetricKey:   util.RandomString(32),
		AccessTokenDuration: time.Minute,
		WebBaseURL:          "https://merchant.example.com",
	}

	server, err := NewServer(config, store, nil, nil, NewNoopAuditWriter())
	require.NoError(t, err)
	server.config.WechatMiniAppID = "wx-mini-test"
	server.config.WechatOrdinaryApplymentContactEmail = "applyment-contact@test.example.com"
	server.config.WechatOrdinaryApplymentSettlementIDIndividual = "719"
	server.config.WechatOrdinaryApplymentSettlementIDEnterprise = "716"
	server.config.WechatOrdinaryApplymentQualification = "餐饮"
	server.config.WechatOrdinaryApplymentServicePhone = "13800138000"
	server.SetEcommerceClientForTest(ecommerceClient)
	server.wsHub = nil
	server.wsPubSub = nil

	tempDir := t.TempDir()
	ls := media.NewLocalStorage("http://testserver", tempDir)
	server.mediaRegistry = media.NewRegistry(store, ls)
	server.mediaStorage = ls
	server.mediaResolver = media.NewURLResolver(media.ResolverConfig{
		CDNPublicBaseURL: "https://cdn.test.example.com",
		ThumbWidth:       200,
		CardWidth:        400,
		DetailWidth:      960,
	}, ls)

	return server
}

func newTestServerWithOrdinaryApplyment(t *testing.T, store db.Store, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) *Server {
	server := newTestServerWithEcommerce(t, store, nil)
	server.SetOrdinaryServiceProviderClientForTest(ordinaryClient)
	return server
}

func expectMerchantApplymentCommandAccepted(t *testing.T, store *mockdb.MockStore, upstreamApplymentID string) {
	t.Helper()

	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentCommandParams{})).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
			require.Equal(t, db.PaymentChannelOrdinaryServiceProvider, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityApplyment, arg.Capability)
			require.Equal(t, db.ExternalPaymentCommandTypeCreateApplyment, arg.CommandType)
			require.Equal(t, db.ExternalPaymentBusinessOwnerApplyment, arg.BusinessOwner)
			require.True(t, arg.BusinessObjectType.Valid)
			require.Equal(t, "ordinary_service_provider_applyment", arg.BusinessObjectType.String)
			require.True(t, arg.BusinessObjectID.Valid)
			require.Equal(t, db.ExternalPaymentObjectApplyment, arg.ExternalObjectType)
			require.NotEmpty(t, arg.ExternalObjectKey)
			require.True(t, arg.ExternalSecondaryKey.Valid)
			require.Equal(t, upstreamApplymentID, arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
			require.Contains(t, string(arg.ResponseSnapshot), upstreamApplymentID)
			return db.ExternalPaymentCommand{ID: util.RandomInt(1, 1000), CommandStatus: arg.CommandStatus}, nil
		})
}

func seedPrivateContactDocumentAsset(t *testing.T, server *Server, objectKey string, content []byte) {
	t.Helper()
	err := server.mediaStorage.PutObject(
		context.Background(),
		server.mediaStorage.PrivateBucket(),
		objectKey,
		"image/png",
		bytes.NewReader(content),
		int64(len(content)),
	)
	require.NoError(t, err)
}

// randomMerchantForApplyment 创建随机商户（进件测试专用）
func randomMerchantForApplyment(ownerID int64) db.Merchant {
	return db.Merchant{
		ID:          util.RandomInt(1, 1000),
		OwnerUserID: ownerID,
		Name:        util.RandomString(10),
		Phone:       "13800138000",
		Address:     util.RandomString(30),
		Status:      "pending_bindbank",
		CreatedAt:   time.Now(),
	}
}

// randomMerchantApplicationForApplyment 创建随机商户申请（进件测试专用）
func randomMerchantApplicationForApplyment(userID int64) db.MerchantApplication {
	return db.MerchantApplication{
		ID:                    util.RandomInt(1, 1000),
		UserID:                userID,
		MerchantName:          util.RandomString(10),
		BusinessLicenseNumber: util.RandomString(18),
		LegalPersonName:       util.RandomString(6),
		LegalPersonIDNumber:   "110101199001011234",
		ContactPhone:          "13800138000",
		BusinessAddress:       util.RandomString(30),
		Status:                "approved",
		BusinessLicenseOcr:    []byte(`{"type_of_enterprise":"个体工商户","address":"深圳市南山区","valid_period":"2020年01月01日至长期"}`),
		IDCardBackOcr:         []byte(`{"valid_date": "2020-01-01-2030-01-01"}`),
	}
}

// randomEcommerceApplymentForTest 创建随机进件记录（测试专用）
func randomEcommerceApplymentForTest(subjectType string, subjectID int64) db.EcommerceApplyment {
	return db.EcommerceApplyment{
		ID:               util.RandomInt(1, 1000),
		SubjectType:      subjectType,
		SubjectID:        subjectID,
		OutRequestNo:     util.RandomString(20),
		OrganizationType: "2401",
		Status:           "pending",
		CreatedAt:        time.Now(),
	}
}

// ==================== 商户开户测试 ====================

func TestResolveOrdinaryApplymentSettlementID(t *testing.T) {
	t.Parallel()

	config := util.Config{
		WechatOrdinaryApplymentSettlementIDIndividual: " 719 ",
		WechatOrdinaryApplymentSettlementIDEnterprise: " 716 ",
	}

	testCases := []struct {
		name             string
		organizationType string
		config           util.Config
		want             string
		wantErr          string
	}{
		{
			name:             "IndividualUsesIndividualSettlementRule",
			organizationType: "4",
			config:           config,
			want:             "719",
		},
		{
			name:             "EnterpriseUsesEnterpriseSettlementRule",
			organizationType: "2",
			config:           config,
			want:             "716",
		},
		{
			name:             "MissingEnterpriseRuleNamesRequiredEnv",
			organizationType: "2",
			config: util.Config{
				WechatOrdinaryApplymentSettlementIDIndividual: "719",
			},
			wantErr: "WECHAT_ORDINARY_APPLYMENT_SETTLEMENT_ID_ENTERPRISE",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveOrdinaryApplymentSettlementID(tc.config, tc.organizationType)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSplitOrdinaryApplymentActivitiesAdditions(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		[]string{"media-a", "media-b", "media-c"},
		splitOrdinaryApplymentActivitiesAdditions(" media-a,media-b; media-c\n"),
	)
	require.Nil(t, splitOrdinaryApplymentActivitiesAdditions(" , ; \n "))
}

func TestMerchantBindBankAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)
	application := randomMerchantApplicationForApplyment(user.ID)

	applicationWithTestURL := application

	testCases := []struct {
		name             string
		body             gin.H
		setupAuth        func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs       func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface)
		buildWechatStubs func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient)
		prepareServer    func(t *testing.T, server *Server)
		checkResponse    func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_WithOrdinaryServiceProviderClient",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "其他银行",
				"account_bank_code": 1099,
				"bank_alias":        "深圳前海微众银行",
				"bank_alias_code":   "1000009561",
				"need_bank_branch":  true,
				"bank_address_code": "440300",
				"bank_branch_id":    "402584040001",
				"bank_name":         "深圳前海微众银行深圳南山支行",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
				"contact_email":     "merchant-admin@example.com",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				// 获取商户
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// 检查是否有进行中的申请
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				// 获取商户申请信息 - 使用带测试服务器 URL 的版本
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(applicationWithTestURL, nil)

				// 创建进件记录
				store.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, arg db.CreateEcommerceApplymentParams) (db.EcommerceApplyment, error) {
						require.Equal(t, "4", arg.OrganizationType)
						require.Equal(t, "其他银行", arg.AccountBank)
						require.True(t, arg.AccountBankCode.Valid)
						require.Equal(t, int64(1099), arg.AccountBankCode.Int64)
						require.True(t, arg.BankAlias.Valid)
						require.Equal(t, "深圳前海微众银行", arg.BankAlias.String)
						require.True(t, arg.BankAliasCode.Valid)
						require.Equal(t, "1000009561", arg.BankAliasCode.String)
						require.True(t, arg.BankBranchID.Valid)
						require.Equal(t, "402584040001", arg.BankBranchID.String)
						require.Equal(t, applicationWithTestURL.ContactPhone, arg.MobilePhone)
						require.True(t, arg.ContactEmail.Valid)
						require.Equal(t, "merchant-admin@example.com", arg.ContactEmail.String)
						return randomEcommerceApplymentForTest("merchant", merchant.ID), nil
					})

				// Mock 加密
				ordinaryClient.EXPECT().
					EncryptSensitiveData(gomock.Any()).
					Times(8). // 法人信息、联系人信息、账户名、账号、手机、邮箱
					Return("encrypted_data", nil)

				expectedObjectKey := buildMerchantStorefrontQRCodeObjectKey(merchant.ID)
				ordinaryClient.EXPECT().
					UploadImage(gomock.Any(), path.Base(expectedObjectKey), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, filename string, fileData []byte) (*ospcontracts.MediaUploadResponse, error) {
						require.Equal(t, path.Base(expectedObjectKey), filename)
						require.NotEmpty(t, fileData)
						return &ospcontracts.MediaUploadResponse{MediaID: "wx_store_qr_media_id"}, nil
					})

				// Mock 提交进件
				ordinaryClient.EXPECT().
					SubmitApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, req ospcontracts.ApplymentSubmitRequest) (*ospcontracts.ApplymentSubmitResponse, error) {
						require.Equal(t, ospcontracts.SubjectTypeIndividual, req.SubjectInfo.SubjectType)
						require.Equal(t, "其他银行", req.BankAccountInfo.AccountBank)
						require.Equal(t, "402584040001", req.BankAccountInfo.BankBranchID)
						require.NotNil(t, req.SubjectInfo.IdentityInfo.IDCardInfo)
						require.Equal(t, "2020-01-01", req.SubjectInfo.IdentityInfo.IDCardInfo.CardPeriodBegin)
						require.Equal(t, "2030-01-01", req.SubjectInfo.IdentityInfo.IDCardInfo.CardPeriodEnd)
						require.Equal(t, "encrypted_data", req.ContactInfo.MobilePhone)
						require.Equal(t, "encrypted_data", req.ContactInfo.ContactEmail)
						require.NotNil(t, req.BusinessInfo.SalesInfo.MiniProgramInfo)
						require.Equal(t, "wx-mini-test", req.BusinessInfo.SalesInfo.MiniProgramInfo.MiniProgramAppID)
						require.Contains(t, req.BusinessInfo.SalesInfo.MiniProgramInfo.MiniProgramPics, "wx_store_qr_media_id")
						require.Nil(t, req.BusinessInfo.SalesInfo.StoreInfo)
						require.Equal(t, "719", req.SettlementInfo.SettlementID)
						require.Equal(t, "餐饮", req.SettlementInfo.QualificationType)
						require.Equal(t, "20191030111cff5b5e", req.SettlementInfo.ActivitiesID)
						require.Equal(t, "0.38", req.SettlementInfo.DebitActivitiesRate)
						require.Equal(t, "0.38", req.SettlementInfo.CreditActivitiesRate)
						require.Equal(t, []string{"media-a", "media-b"}, req.SettlementInfo.ActivitiesAdditions)
						return &ospcontracts.ApplymentSubmitResponse{ApplymentID: 123456789}, nil
					})

				// 更新进件状态
				store.EXPECT().
					UpdateEcommerceApplymentToSubmitted(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, nil)

				expectMerchantApplymentCommandAccepted(t, store, "123456789")

				// 更新商户状态
				store.EXPECT().
					UpdateMerchantStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Merchant{}, nil)

				store.EXPECT().
					UpdateEcommerceApplymentStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentStatusParams{})).
					Times(1).
					Return(db.EcommerceApplyment{}, nil)

				ordinaryClient.EXPECT().
					QueryApplymentByID(gomock.Any(), ospcontracts.ApplymentQueryByIDRequest{ApplymentID: 123456789}).
					Times(1).
					Return(&ospcontracts.ApplymentQueryResponse{
						ApplymentID:    123456789,
						ApplymentState: ospcontracts.ApplymentStateToBeSigned,
						SignURL:        "https://wx.example.com/sign/merchant",
					}, nil)
			},
			buildWechatStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				expectedObjectKey := buildMerchantStorefrontQRCodeObjectKey(merchant.ID)
				qrCodeData := buildTestQRCodePNG(t)

				store.EXPECT().
					GetMediaAssetByObjectKey(gomock.Any(), expectedObjectKey).
					Times(1).
					Return(db.MediaAsset{}, db.ErrRecordNotFound)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.AssignableToTypeOf(&wechat.WXACodeRequest{})).
					Times(1).
					DoAndReturn(func(_ context.Context, req *wechat.WXACodeRequest) ([]byte, error) {
						require.Equal(t, fmt.Sprintf("m_%d", merchant.ID), req.Scene)
						require.Equal(t, "pages/takeout/restaurant-detail/index", req.Page)
						require.NotNil(t, req.CheckPath)
						require.False(t, *req.CheckPath)
						require.Equal(t, "develop", req.EnvVersion)
						require.Equal(t, 430, req.Width)
						return qrCodeData, nil
					})

				store.EXPECT().
					CreateMediaAsset(gomock.Any(), gomock.AssignableToTypeOf(db.CreateMediaAssetParams{})).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.CreateMediaAssetParams) (db.MediaAsset, error) {
						require.Equal(t, expectedObjectKey, arg.ObjectKey)
						require.Equal(t, string(media.VisibilityPublic), arg.Visibility)
						require.Equal(t, string(media.CategoryStorefrontImage), arg.MediaCategory)
						require.Equal(t, "image/png", arg.MimeType)
						require.NotEmpty(t, arg.ChecksumSha256)
						return db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "pending"}, nil
					})

				store.EXPECT().
					SetMediaAssetModerationStatus(gomock.Any(), gomock.AssignableToTypeOf(db.SetMediaAssetModerationStatusParams{})).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.SetMediaAssetModerationStatusParams) (db.MediaAsset, error) {
						require.Equal(t, "approved", arg.ModerationStatus)
						return db.MediaAsset{ID: arg.ID, ModerationStatus: arg.ModerationStatus}, nil
					})
			},
			prepareServer: func(t *testing.T, server *Server) {
				server.config.WechatOrdinaryApplymentActivitiesID = "20191030111cff5b5e"
				server.config.WechatOrdinaryApplymentDebitActivitiesRate = "0.38"
				server.config.WechatOrdinaryApplymentCreditActivitiesRate = "0.38"
				server.config.WechatOrdinaryApplymentActivitiesAdditions = "media-a, media-b"
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantBindBankResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, int64(123456789), response.ApplymentID)
				require.Equal(t, "to_be_signed", response.Status)
				require.Equal(t, "待签约，请点击签约链接完成签约", response.StatusDesc)
				require.NotNil(t, response.SignURL)
				require.Equal(t, "https://wx.example.com/sign/merchant", *response.SignURL)
			},
		},
		{
			name: "BadRequest_MissingContactEmailDoesNotFallbackToPlatformEmail",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "其他银行",
				"account_bank_code": 1099,
				"bank_alias":        "深圳前海微众银行",
				"bank_alias_code":   "1000009561",
				"need_bank_branch":  true,
				"bank_address_code": "440300",
				"bank_branch_id":    "402584040001",
				"bank_name":         "深圳前海微众银行深圳南山支行",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(applicationWithTestURL, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "请填写超级管理员邮箱")
				require.NotContains(t, recorder.Body.String(), "WECHAT_ORDINARY_APPLYMENT_CONTACT_EMAIL")
				require.NotContains(t, recorder.Body.String(), "平台管理员")
			},
		},
		{
			name: "BadRequest_InvalidContactEmailReturnsUserFacingMessage",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "其他银行",
				"account_bank_code": 1099,
				"bank_alias":        "深圳前海微众银行",
				"bank_alias_code":   "1000009561",
				"need_bank_branch":  true,
				"bank_address_code": "440300",
				"bank_branch_id":    "402584040001",
				"bank_name":         "深圳前海微众银行深圳南山支行",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
				"contact_email":     "not-an-email",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "请填写正确的邮箱地址")
				require.NotContains(t, recorder.Body.String(), "ContactEmail")
			},
		},
		{
			name: "OK_WithSuperAdministratorContact",
			body: gin.H{
				"account_type":                      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":                      "其他银行",
				"account_bank_code":                 1099,
				"bank_alias":                        "深圳前海微众银行",
				"bank_alias_code":                   "1000009561",
				"need_bank_branch":                  true,
				"bank_address_code":                 "440300",
				"bank_branch_id":                    "402584040001",
				"bank_name":                         "深圳前海微众银行深圳南山支行",
				"account_number":                    "6214830012345678",
				"account_name":                      "张三",
				"contact_type":                      "SUPER",
				"contact_name":                      "李四",
				"contact_id_doc_type":               "IDENTIFICATION_TYPE_MAINLAND_IDCARD",
				"contact_id_card_number":            "110101199202023456",
				"contact_id_doc_copy_asset_id":      501,
				"contact_id_doc_copy_back_asset_id": 502,
				"contact_id_doc_period_begin":       "2020-01-01",
				"contact_id_doc_period_end":         "2030-01-01",
				"contact_email":                     "merchant-super@example.com",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(501)).
					AnyTimes().
					Return(db.MediaAsset{
						ID:               501,
						ObjectKey:        "id_card/front/1/20240101/contact-front.png",
						Visibility:       string(media.VisibilityPrivate),
						MediaCategory:    string(media.CategoryIDCardFront),
						MimeType:         "image/png",
						UploadedBy:       user.ID,
						UploadStatus:     "confirmed",
						ModerationStatus: "approved",
					}, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(502)).
					AnyTimes().
					Return(db.MediaAsset{
						ID:               502,
						ObjectKey:        "id_card/back/1/20240101/contact-back.png",
						Visibility:       string(media.VisibilityPrivate),
						MediaCategory:    string(media.CategoryIDCardBack),
						MimeType:         "image/png",
						UploadedBy:       user.ID,
						UploadStatus:     "confirmed",
						ModerationStatus: "approved",
					}, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(applicationWithTestURL, nil)

				store.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ any, arg db.CreateEcommerceApplymentParams) (db.EcommerceApplyment, error) {
						require.Equal(t, "李四", arg.ContactName)
						require.True(t, arg.ContactIDCardNumber.Valid)
						require.NotEmpty(t, arg.ContactIDCardNumber.String)
						require.Equal(t, applicationWithTestURL.ContactPhone, arg.MobilePhone)
						return randomEcommerceApplymentForTest("merchant", merchant.ID), nil
					})

				ordinaryClient.EXPECT().
					EncryptSensitiveData(gomock.Any()).
					Times(8).
					Return("encrypted_data", nil)

				expectedObjectKey := buildMerchantStorefrontQRCodeObjectKey(merchant.ID)
				ordinaryClient.EXPECT().
					UploadImage(gomock.Any(), "contact-front.png", gomock.Any()).
					Times(1).
					Return(&ospcontracts.MediaUploadResponse{MediaID: "wx_super_contact_front_media_id"}, nil)
				ordinaryClient.EXPECT().
					UploadImage(gomock.Any(), "contact-back.png", gomock.Any()).
					Times(1).
					Return(&ospcontracts.MediaUploadResponse{MediaID: "wx_super_contact_back_media_id"}, nil)
				ordinaryClient.EXPECT().
					UploadImage(gomock.Any(), path.Base(expectedObjectKey), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, filename string, fileData []byte) (*ospcontracts.MediaUploadResponse, error) {
						require.Equal(t, path.Base(expectedObjectKey), filename)
						require.NotEmpty(t, fileData)
						return &ospcontracts.MediaUploadResponse{MediaID: "wx_store_qr_media_id"}, nil
					})

				ordinaryClient.EXPECT().
					SubmitApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, req ospcontracts.ApplymentSubmitRequest) (*ospcontracts.ApplymentSubmitResponse, error) {
						require.Equal(t, ospcontracts.ContactTypeSuper, req.ContactInfo.ContactType)
						require.Equal(t, "encrypted_data", req.ContactInfo.ContactName)
						require.Equal(t, ospcontracts.IdentificationTypeIDCard, req.ContactInfo.ContactIDDocType)
						require.Equal(t, "encrypted_data", req.ContactInfo.ContactIDNumber)
						require.Equal(t, "wx_super_contact_front_media_id", req.ContactInfo.ContactIDDocCopy)
						require.Equal(t, "wx_super_contact_back_media_id", req.ContactInfo.ContactIDDocCopyBack)
						require.Equal(t, "2020-01-01", req.ContactInfo.ContactPeriodBegin)
						require.Equal(t, "2030-01-01", req.ContactInfo.ContactPeriodEnd)
						return &ospcontracts.ApplymentSubmitResponse{ApplymentID: 22334455}, nil
					})

				store.EXPECT().
					UpdateEcommerceApplymentToSubmitted(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, nil)

				expectMerchantApplymentCommandAccepted(t, store, "22334455")

				store.EXPECT().
					UpdateMerchantStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Merchant{}, nil)

				store.EXPECT().
					UpdateEcommerceApplymentStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentStatusParams{})).
					Times(1).
					Return(db.EcommerceApplyment{}, nil)

				ordinaryClient.EXPECT().
					QueryApplymentByID(gomock.Any(), ospcontracts.ApplymentQueryByIDRequest{ApplymentID: 22334455}).
					Times(1).
					Return(&ospcontracts.ApplymentQueryResponse{
						ApplymentID:    22334455,
						ApplymentState: ospcontracts.ApplymentStateAuditing,
					}, nil)
			},
			buildWechatStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				expectedObjectKey := buildMerchantStorefrontQRCodeObjectKey(merchant.ID)
				qrCodeData := buildTestQRCodePNG(t)

				store.EXPECT().
					GetMediaAssetByObjectKey(gomock.Any(), expectedObjectKey).
					Times(1).
					Return(db.MediaAsset{}, db.ErrRecordNotFound)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.AssignableToTypeOf(&wechat.WXACodeRequest{})).
					Times(1).
					Return(qrCodeData, nil)

				store.EXPECT().
					CreateMediaAsset(gomock.Any(), gomock.AssignableToTypeOf(db.CreateMediaAssetParams{})).
					Times(1).
					Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "pending"}, nil)

				store.EXPECT().
					SetMediaAssetModerationStatus(gomock.Any(), gomock.AssignableToTypeOf(db.SetMediaAssetModerationStatusParams{})).
					Times(1).
					Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "approved"}, nil)
			},
			prepareServer: func(t *testing.T, server *Server) {
				content := buildTestQRCodePNG(t)
				seedPrivateContactDocumentAsset(t, server, "id_card/front/1/20240101/contact-front.png", content)
				seedPrivateContactDocumentAsset(t, server, "id_card/back/1/20240101/contact-back.png", content)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantBindBankResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, int64(22334455), response.ApplymentID)
				require.Equal(t, "auditing", response.Status)
				require.Equal(t, "审核中", response.StatusDesc)
			},
		},
		{
			name: "BadRequest_MissingSuperAdministratorDocumentFields",
			body: gin.H{
				"account_type":           "ACCOUNT_TYPE_PRIVATE",
				"account_bank":           "其他银行",
				"account_bank_code":      1099,
				"bank_alias":             "深圳前海微众银行",
				"bank_alias_code":        "1000009561",
				"need_bank_branch":       true,
				"bank_address_code":      "440300",
				"bank_branch_id":         "402584040001",
				"bank_name":              "深圳前海微众银行深圳南山支行",
				"account_number":         "6214830012345678",
				"account_name":           "张三",
				"contact_type":           "SUPER",
				"contact_name":           "李四",
				"contact_id_card_number": "110101199202023456",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(applicationWithTestURL, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "BadRequest_SuperAdministratorDocumentUploadValidationFailed",
			body: gin.H{
				"account_type":                      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":                      "其他银行",
				"account_bank_code":                 1099,
				"bank_alias":                        "深圳前海微众银行",
				"bank_alias_code":                   "1000009561",
				"need_bank_branch":                  true,
				"bank_address_code":                 "440300",
				"bank_branch_id":                    "402584040001",
				"bank_name":                         "深圳前海微众银行深圳南山支行",
				"account_number":                    "6214830012345678",
				"account_name":                      "张三",
				"contact_type":                      "SUPER",
				"contact_name":                      "李四",
				"contact_id_doc_type":               "IDENTIFICATION_TYPE_MAINLAND_IDCARD",
				"contact_id_card_number":            "110101199202023456",
				"contact_id_doc_copy_asset_id":      501,
				"contact_id_doc_copy_back_asset_id": 502,
				"contact_id_doc_period_begin":       "2020-01-01",
				"contact_id_doc_period_end":         "2030-01-01",
				"contact_email":                     "merchant-upload@example.com",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(501)).
					AnyTimes().
					Return(db.MediaAsset{
						ID:               501,
						ObjectKey:        "id_card/front/1/20240101/contact-front.png",
						Visibility:       string(media.VisibilityPrivate),
						MediaCategory:    string(media.CategoryIDCardFront),
						MimeType:         "image/png",
						UploadedBy:       user.ID,
						UploadStatus:     "confirmed",
						ModerationStatus: "approved",
					}, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(502)).
					AnyTimes().
					Return(db.MediaAsset{
						ID:               502,
						ObjectKey:        "id_card/back/1/20240101/contact-back.png",
						Visibility:       string(media.VisibilityPrivate),
						MediaCategory:    string(media.CategoryIDCardBack),
						MimeType:         "image/png",
						UploadedBy:       user.ID,
						UploadStatus:     "confirmed",
						ModerationStatus: "approved",
					}, nil)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(applicationWithTestURL, nil)

				store.EXPECT().
					CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
					Times(1).
					Return(randomEcommerceApplymentForTest("merchant", merchant.ID), nil)

				ordinaryClient.EXPECT().
					EncryptSensitiveData(gomock.Any()).
					Times(8).
					Return("encrypted_data", nil)

				expectedObjectKey := buildMerchantStorefrontQRCodeObjectKey(merchant.ID)
				ordinaryClient.EXPECT().
					UploadImage(gomock.Any(), path.Base(expectedObjectKey), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, filename string, fileData []byte) (*ospcontracts.MediaUploadResponse, error) {
						require.Equal(t, path.Base(expectedObjectKey), filename)
						require.NotEmpty(t, fileData)
						return &ospcontracts.MediaUploadResponse{MediaID: "wx_store_qr_media_id"}, nil
					})

				ordinaryClient.EXPECT().
					UploadImage(gomock.Any(), "contact-front.png", gomock.Any()).
					Times(1).
					Return(nil, &wechat.UploadImageValidationError{Message: "upload image: file is empty; provide a non-empty JPG, JPEG, PNG, or BMP image"})
			},
			buildWechatStubs: func(store *mockdb.MockStore, wechatClient *mockwechat.MockWechatClient) {
				expectedObjectKey := buildMerchantStorefrontQRCodeObjectKey(merchant.ID)
				qrCodeData := buildTestQRCodePNG(t)

				store.EXPECT().
					GetMediaAssetByObjectKey(gomock.Any(), expectedObjectKey).
					Times(1).
					Return(db.MediaAsset{}, db.ErrRecordNotFound)

				wechatClient.EXPECT().
					GetWXACodeUnlimited(gomock.Any(), gomock.AssignableToTypeOf(&wechat.WXACodeRequest{})).
					Times(1).
					Return(qrCodeData, nil)

				store.EXPECT().
					CreateMediaAsset(gomock.Any(), gomock.AssignableToTypeOf(db.CreateMediaAssetParams{})).
					Times(1).
					Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "pending"}, nil)

				store.EXPECT().
					SetMediaAssetModerationStatus(gomock.Any(), gomock.AssignableToTypeOf(db.SetMediaAssetModerationStatusParams{})).
					Times(1).
					Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "approved"}, nil)
			},
			prepareServer: func(t *testing.T, server *Server) {
				content := buildTestQRCodePNG(t)
				seedPrivateContactDocumentAsset(t, server, "id_card/front/1/20240101/contact-front.png", content)
				seedPrivateContactDocumentAsset(t, server, "id_card/back/1/20240101/contact-back.png", content)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "图片文件为空，请上传非空的 JPG、JPEG、PNG 或 BMP 图片后重试")
			},
		},
		{
			name: "MerchantNotFound",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "StaffForbidden",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "InvalidMerchantStatus",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				invalidMerchant := merchant
				invalidMerchant.Status = "pending" // 还未审核通过
				expectResolveSingleOwnedMerchant(store, user.ID, invalidMerchant)
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "AlreadyHasPendingApplyment",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// 已有进行中的申请
				existingApplyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
				existingApplyment.Status = "auditing"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(existingApplyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "AlreadyHasPendingApplyment_AccountNeedVerify",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				existingApplyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
				existingApplyment.Status = "account_need_verify"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(existingApplyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "AlreadyHasPendingApplyment_Checking",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				existingApplyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
				existingApplyment.Status = "checking"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(existingApplyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "AlreadyHasPendingApplyment_ToBeConfirmed",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				existingApplyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
				existingApplyment.Status = "to_be_confirmed"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(existingApplyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "InvalidIDCardValidityPeriod",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				invalidApplication := applicationWithTestURL
				invalidApplication.IDCardBackOcr = []byte(`{"valid_date": "长期"}`)
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(invalidApplication, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, ErrApplymentIDCardValidityInvalid.Code, response.Code)
				require.Equal(t, ErrApplymentIDCardValidityInvalid.Message, response.Error)
			},
		},
		{
			name: "InvalidBusinessLicenseValidityPeriod",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				invalidApplication := applicationWithTestURL
				invalidApplication.BusinessLicenseOcr = []byte(`{"type_of_enterprise":"个体工商户","address":"深圳市南山区","valid_period":"无效文本"}`)
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(invalidApplication, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, ErrApplymentBusinessLicenseValidityInvalid.Code, response.Code)
				require.Equal(t, ErrApplymentBusinessLicenseValidityInvalid.Message, response.Error)
			},
		},
		{
			name: "EnterpriseMerchantRequiresBusinessAccount",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				enterpriseApplication := applicationWithTestURL
				enterpriseApplication.BusinessLicenseOcr = []byte(`{"type_of_enterprise":"有限责任公司","address":"深圳市南山区","valid_period":"2020年01月01日至长期"}`)
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(enterpriseApplication, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, ErrApplymentEnterprisePublicAccountRequired.Code, response.Code)
				require.Equal(t, ErrApplymentEnterprisePublicAccountRequired.Message, response.Error)
			},
		},
		{
			name: "UnsupportedMerchantOrganizationType",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_BUSINESS",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

				unsupportedApplication := applicationWithTestURL
				unsupportedApplication.BusinessLicenseOcr = []byte(`{"type_of_enterprise":"事业单位法人","address":"深圳市南山区","valid_period":"2020年01月01日至长期"}`)
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(unsupportedApplication, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, ErrMerchantApplymentOrganizationUnsupported.Code, response.Code)
				require.Equal(t, ErrMerchantApplymentOrganizationUnsupported.Message, response.Error)
			},
		},
		{
			name: "AlreadyFinished",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// 已完成
				existingApplyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
				existingApplyment.Status = "finish"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(existingApplyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: gin.H{
				"account_type":      "ACCOUNT_TYPE_PRIVATE",
				"account_bank":      "招商银行",
				"bank_address_code": "440300",
				"account_number":    "6214830012345678",
				"account_name":      "张三",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No auth
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				// No stubs needed
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "InvalidRequestBody",
			body: gin.H{
				"account_type": "INVALID_TYPE", // 无效的账户类型
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, ordinaryClient *mockordinaryserviceprovider.MockOrdinaryServiceProviderClientInterface) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
			wechatClient := mockwechat.NewMockWechatClient(ctrl)
			tc.buildStubs(store, ordinaryClient)
			if tc.buildWechatStubs != nil {
				tc.buildWechatStubs(store, wechatClient)
			}

			server := newTestServerWithOrdinaryApplyment(t, store, ordinaryClient)
			server.wechatClient = wechatClient
			if tc.prepareServer != nil {
				tc.prepareServer(t, server)
			}

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/merchant/applyment/bindbank"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestValidateApplymentBusinessLicenseValidity(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		validPeriod string
		wantErr     bool
	}{
		{
			name:        "EmptyIsAllowed",
			validPeriod: "",
			wantErr:     false,
		},
		{
			name:        "LongTermOnlyIsAllowed",
			validPeriod: "长期",
			wantErr:     false,
		},
		{
			name:        "PermanentKeywordIsAllowed",
			validPeriod: "永久有效",
			wantErr:     false,
		},
		{
			name:        "RangedLongTermIsAllowed",
			validPeriod: "2020年01月01日至长期",
			wantErr:     false,
		},
		{
			name:        "InvalidTextRejected",
			validPeriod: "无效文本",
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateApplymentBusinessLicenseValidity(tc.validPeriod)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrApplymentBusinessLicenseValidityInvalid)
				return
			}
			require.NoError(t, err)
		})
	}
}

// ==================== 商户开户状态查询测试 ====================

func TestGetMerchantApplymentStatusAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_Pending",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				applyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
				applyment.Status = "pending"
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(applyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "pending", response.Status)
				require.True(t, response.CanSubmit)
			},
		},
		{
			name: "OK_WithSignURL",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				applyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
				applyment.Status = "to_be_signed"
				applyment.SignUrl = pgtype.Text{String: "https://pay.weixin.qq.com/sign/xxx", Valid: true}
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(applyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "to_be_signed", response.Status)
				require.NotNil(t, response.SignURL)
				require.False(t, response.CanSubmit)
			},
		},
		{
			name: "OK_Finished",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				applyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
				applyment.Status = "finish"
				applyment.SubMchID = pgtype.Text{String: "1234567890", Valid: true}
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(applyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "finish", response.Status)
				require.NotNil(t, response.SubMchID)
				require.False(t, response.CanSubmit)
			},
		},
		{
			name: "InvalidStoredAccountValidationReturnsInternalServerError",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				applyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
				applyment.Status = "pending"
				applyment.AccountValidation = []byte("{")
				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(applyment, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				var response apiTestEnvelope
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, "internal server error", response.Message)
			},
		},
		{
			name: "NoApplyment",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "not_applied", response.Status)
				require.True(t, response.CanSubmit)
			},
		},
		{
			name: "NoApplyment_SuspendedMerchant",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				testMerchant := merchant
				testMerchant.Status = "suspended"
				expectResolveSingleOwnedMerchant(store, user.ID, testMerchant)

				store.EXPECT().
					GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response merchantApplymentStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, "not_applied", response.Status)
				require.False(t, response.CanSubmit)
				require.Equal(t, "当前商户状态不可用，暂不支持提交普通服务商进件。", response.BlockReason)
			},
		},
		{
			name: "MerchantNotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)

			url := "/v1/merchant/applyment/status"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestGetMerchantApplymentStatusNormalizesFinishWithoutSubMchID(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	applyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
	applyment.Status = "auditing"
	applyment.OutRequestNo = "APPLYMENT_STATUS_001"
	applyment.ApplymentID = pgtype.Int8{Int64: 123456789, Valid: true}

	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(applyment, nil)

	ordinaryClient.EXPECT().
		QueryApplymentByID(gomock.Any(), ospcontracts.ApplymentQueryByIDRequest{ApplymentID: 123456789}).
		Times(1).
		Return(&ospcontracts.ApplymentQueryResponse{
			ApplymentID:    123456789,
			BusinessCode:   "APPLYMENT_STATUS_001",
			ApplymentState: ospcontracts.ApplymentStateFinished,
		}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentStatusParams{})).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.UpdateEcommerceApplymentStatusParams) (db.EcommerceApplyment, error) {
			require.Equal(t, applyment.ID, arg.ID)
			require.Equal(t, "submitted", arg.Status)
			require.Equal(t, pgtype.Int8{Int64: 123456789, Valid: true}, arg.ApplymentID)
			require.Equal(t, pgtype.Text{}, arg.SubMchID)
			return db.EcommerceApplyment{ID: arg.ID, Status: arg.Status}, nil
		})

	server := newTestServerWithOrdinaryApplyment(t, store, ordinaryClient)
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/applyment/status", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response merchantApplymentStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "submitted", response.Status)
	require.False(t, response.CanSubmit)
}

func TestGetMerchantApplymentStatusOrdinaryFinishActivatesWithSubMchID(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	applyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
	applyment.Status = "auditing"
	applyment.OutRequestNo = "APPLYMENT_STATUS_002"
	applyment.ApplymentID = pgtype.Int8{Int64: 22334455, Valid: true}

	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(applyment, nil)

	ordinaryClient.EXPECT().
		QueryApplymentByID(gomock.Any(), ospcontracts.ApplymentQueryByIDRequest{ApplymentID: 22334455}).
		Times(1).
		Return(&ospcontracts.ApplymentQueryResponse{
			ApplymentID:    22334455,
			BusinessCode:   "APPLYMENT_STATUS_002",
			ApplymentState: ospcontracts.ApplymentStateFinished,
			SubMchID:       "sub_mch_22334455",
		}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentStatusParams{})).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.UpdateEcommerceApplymentStatusParams) (db.EcommerceApplyment, error) {
			require.Equal(t, applyment.ID, arg.ID)
			require.Equal(t, "finish", arg.Status)
			require.Equal(t, pgtype.Text{String: "sub_mch_22334455", Valid: true}, arg.SubMchID)
			return db.EcommerceApplyment{ID: arg.ID, Status: arg.Status}, nil
		})

	store.EXPECT().
		ApplymentSubMchActivationTx(gomock.Any(), db.ApplymentSubMchActivationTxParams{
			ApplymentID:           applyment.ID,
			SubjectType:           applyment.SubjectType,
			SubjectID:             applyment.SubjectID,
			SubMchID:              "sub_mch_22334455",
			AccountAuthorizeState: "",
		}).
		Times(1).
		Return(nil)

	server := newTestServerWithOrdinaryApplyment(t, store, ordinaryClient)
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/applyment/status", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response merchantApplymentStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "finish", response.Status)
	require.Empty(t, response.AccountAuthorizeState)
	require.Contains(t, response.StatusDesc, "微信支付已开通")
	require.NotContains(t, response.StatusDesc, "开户意愿")
	require.NotContains(t, response.ActionHint, "开户意愿")
	require.NotNil(t, response.SubMchID)
	require.Equal(t, "sub_mch_22334455", *response.SubMchID)
}

func TestGetMerchantApplymentStatusOrdinaryFinishActivationFailureReturnsSyncingGuidance(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	applyment := randomEcommerceApplymentForTest("merchant", merchant.ID)
	applyment.Status = "auditing"
	applyment.OutRequestNo = "APPLYMENT_STATUS_ACTIVATION_FAIL"
	applyment.ApplymentID = pgtype.Int8{Int64: 33445566, Valid: true}

	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(applyment, nil)

	ordinaryClient.EXPECT().
		QueryApplymentByID(gomock.Any(), ospcontracts.ApplymentQueryByIDRequest{ApplymentID: 33445566}).
		Times(1).
		Return(&ospcontracts.ApplymentQueryResponse{
			ApplymentID:    33445566,
			BusinessCode:   "APPLYMENT_STATUS_ACTIVATION_FAIL",
			ApplymentState: ospcontracts.ApplymentStateFinished,
			SubMchID:       "sub_mch_activation_fail",
		}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentStatusParams{})).
		Times(1).
		Return(db.EcommerceApplyment{ID: applyment.ID, Status: "finish"}, nil)

	store.EXPECT().
		ApplymentSubMchActivationTx(gomock.Any(), db.ApplymentSubMchActivationTxParams{
			ApplymentID:           applyment.ID,
			SubjectType:           applyment.SubjectType,
			SubjectID:             applyment.SubjectID,
			SubMchID:              "sub_mch_activation_fail",
			AccountAuthorizeState: "",
		}).
		Times(1).
		Return(errors.New("activation failed"))

	server := newTestServerWithOrdinaryApplyment(t, store, ordinaryClient)
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/applyment/status", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response merchantApplymentStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "finish", response.Status)
	require.Empty(t, response.AccountAuthorizeState)
	require.Contains(t, response.StatusDesc, "平台正在同步收款配置")
	require.Contains(t, response.ActionHint, "稍后")
	require.NotContains(t, response.StatusDesc, "开户意愿")
	require.NotNil(t, response.SubMchID)
	require.Equal(t, "sub_mch_activation_fail", *response.SubMchID)
}

func TestMapWechatApplymentStatus(t *testing.T) {
	testCases := []struct {
		name     string
		wxStatus string
		expected string
	}{
		{
			name:     "LatestAuditing",
			wxStatus: "AUDITING",
			expected: "auditing",
		},
		{
			name:     "LatestChecking",
			wxStatus: "CHECKING",
			expected: "checking",
		},
		{
			name:     "LatestAccountNeedVerify",
			wxStatus: "ACCOUNT_NEED_VERIFY",
			expected: "account_need_verify",
		},
		{
			name:     "LatestNeedSign",
			wxStatus: "NEED_SIGN",
			expected: "to_be_signed",
		},
		{
			name:     "LatestFinish",
			wxStatus: "FINISH",
			expected: "finish",
		},
		{
			name:     "LatestCanceled",
			wxStatus: "CANCELED",
			expected: "canceled",
		},
		{
			name:     "LatestFrozen",
			wxStatus: "FROZEN",
			expected: "frozen",
		},
		{
			name:     "UnknownState",
			wxStatus: "NEW_UPSTREAM_STATE",
			expected: "",
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, logic.MapWechatApplymentStateToStatus(tc.wxStatus))
		})
	}
}

func TestShouldUseRemoteApplymentStatusDesc(t *testing.T) {
	require.False(t, shouldUseRemoteApplymentStatusDesc(
		"submitted",
		pgtype.Text{},
		pgtype.Text{},
		nil,
	))
	require.False(t, shouldUseRemoteApplymentStatusDesc(
		"auditing",
		pgtype.Text{String: "UNSIGNED", Valid: true},
		pgtype.Text{},
		nil,
	))
	require.False(t, shouldUseRemoteApplymentStatusDesc(
		"auditing",
		pgtype.Text{},
		pgtype.Text{String: "https://wx.example.com/legal", Valid: true},
		nil,
	))
	require.True(t, shouldUseRemoteApplymentStatusDesc(
		"auditing",
		pgtype.Text{},
		pgtype.Text{},
		nil,
	))
}

// ==================== 进件回调测试 ====================

func TestHandleApplymentStateNotifyAPI(t *testing.T) {
	testCases := []struct {
		name          string
		body          string
		headers       map[string]string
		buildStubs    func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface, ecommerceClient *mockwechat.MockEcommerceClientInterface)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "NoEcommerceClient",
			body: `{"event_type": "APPLYMENT_STATE.CHANGE"}`,
			headers: map[string]string{
				"Wechatpay-Signature": "test_signature",
				"Wechatpay-Timestamp": "1234567890",
				"Wechatpay-Nonce":     "test_nonce",
			},
			buildStubs: func(store *mockdb.MockStore, paymentClient *mockwechat.MockDirectPaymentClientInterface, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
				// ecommerceClient is nil
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, paymentClient, ecommerceClient)

			// 不传 ecommerceClient 来测试错误情况
			server := newTestServer(t, store)

			url := "/v1/webhooks/wechat-ecommerce/applyment-notify"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(tc.body)))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			for k, v := range tc.headers {
				request.Header.Set(k, v)
			}

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestListMerchantApplymentBanksAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	ordinaryClient.EXPECT().
		ListPersonalBankingBanks(gomock.Any(), 0, applymentCatalogPageSize).
		Times(1).
		Return(&ospcontracts.CapitalBankListResponse{
			TotalCount: 2,
			Count:      2,
			Data: []ospcontracts.CapitalBank{{
				BankAlias:       "其他银行",
				BankAliasCode:   "1099",
				AccountBank:     "其他银行",
				AccountBankCode: 1099,
				NeedBankBranch:  true,
			}, {
				BankAlias:       "招商银行",
				BankAliasCode:   "1000009561",
				AccountBank:     "招商银行",
				AccountBankCode: 1001,
				NeedBankBranch:  false,
			}},
		}, nil)

	server := newTestServerWithOrdinaryApplyment(t, store, ordinaryClient)
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/applyment/banks?account_type=ACCOUNT_TYPE_PRIVATE", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response applymentBankListResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Len(t, response.Banks, 1)
	require.Equal(t, "招商银行", response.Banks[0].BankAlias)
	require.Equal(t, "招商银行", response.Banks[0].AccountBank)
	require.Equal(t, int64(1001), response.Banks[0].AccountBankCode)
}

// ==================== 无普通服务商 client 时的失败指引测试 ====================

func TestMerchantBindBankWithoutOrdinaryServiceProviderClient(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)
	application := randomMerchantApplicationForApplyment(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	// 获取商户
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	// 检查是否有进行中的申请
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

	// 获取商户申请信息
	store.EXPECT().
		GetUserMerchantApplication(gomock.Any(), user.ID).
		Times(1).
		Return(application, nil)

	// 创建不带普通服务商 client 的服务器，不能降级为“待人工处理”。
	server := newTestServer(t, store)

	body := gin.H{
		"account_type":      "ACCOUNT_TYPE_PRIVATE",
		"account_bank":      "招商银行",
		"bank_address_code": "440300",
		"account_number":    "6214830012345678",
		"account_name":      "张三",
		"contact_email":     "merchant-admin@example.com",
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	url := "/v1/merchant/applyment/bindbank"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")

	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "普通服务商进件服务暂不可用")
	require.Contains(t, recorder.Body.String(), "检查微信支付普通服务商证书、公钥、进件和回调配置")
}

// ==================== 加密失败测试 ====================

func TestMerchantBindBankEncryptFailed(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)
	application := randomMerchantApplicationForApplyment(user.ID)

	applicationWithTestURL := application

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)

	// 获取商户
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	// 检查是否有进行中的申请
	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

	// 获取商户申请信息
	store.EXPECT().
		GetUserMerchantApplication(gomock.Any(), user.ID).
		Times(1).
		Return(applicationWithTestURL, nil)

	// 创建进件记录
	store.EXPECT().
		CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
		Times(1).
		Return(randomEcommerceApplymentForTest("merchant", merchant.ID), nil)

	// Mock 加密失败
	ordinaryClient.EXPECT().
		EncryptSensitiveData(gomock.Any()).
		Times(1).
		Return("", fmt.Errorf("encrypt failed"))

	server := newTestServerWithOrdinaryApplyment(t, store, ordinaryClient)

	body := gin.H{
		"account_type":      "ACCOUNT_TYPE_PRIVATE",
		"account_bank":      "招商银行",
		"bank_address_code": "440300",
		"account_number":    "6214830012345678",
		"account_name":      "张三",
		"contact_email":     "merchant-admin@example.com",
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	url := "/v1/merchant/applyment/bindbank"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")

	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "商户开户资料加密失败")
	require.Contains(t, recorder.Body.String(), "检查微信支付敏感信息加密配置")
}

func TestRespondApplymentWechatError(t *testing.T) {
	testCases := []struct {
		name           string
		wxErr          *wechat.WechatPayError
		expectedStatus int
		expectedCode   int
		expectedError  string
	}{
		{
			name:           "ResourceAlreadyExistsMapsToConflict",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "RESOURCE_ALREADY_EXISTS", Message: "存在流程进行中的申请单，请检查是否重入"},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   ErrAccountApplymentPending.Code,
			expectedError:  ErrAccountApplymentPending.Message,
		},
		{
			name:           "ParamErrorMapsToBadRequest",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "PARAM_ERROR", Message: "参数错误"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  ErrApplymentWechatParamError.Error(),
		},
		{
			name:           "NoAuthMapsToForbidden",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusForbidden, Code: "NO_AUTH", Message: "商户权限异常"},
			expectedStatus: http.StatusForbidden,
			expectedCode:   ErrApplymentWechatNoAuth.Code,
			expectedError:  ErrApplymentWechatNoAuth.Message,
		},
		{
			name:           "InvalidRequestMapsToBadGateway",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "HTTP 请求不符合 APIv3 规则"},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   ErrApplymentWechatInvalidRequest.Code,
			expectedError:  ErrApplymentWechatInvalidRequest.Message,
		},
		{
			name:           "SignErrorMapsToBadGateway",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusUnauthorized, Code: "SIGN_ERROR", Message: "验证不通过"},
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   ErrApplymentWechatSignError.Code,
			expectedError:  ErrApplymentWechatSignError.Message,
		},
		{
			name:           "SystemErrorMapsToBadGateway",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusInternalServerError, Code: "SYSTEM_ERROR", Message: "系统异常"},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   0,
			expectedError:  "internal server error",
		},
		{
			name:           "ResourceNotExistsMapsToNotFound",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusNotFound, Code: "RESOURCE_NOT_EXISTS", Message: "申请单不存在"},
			expectedStatus: http.StatusNotFound,
			expectedCode:   ErrApplymentWechatNotFound.Code,
			expectedError:  ErrApplymentWechatNotFound.Message,
		},
		{
			name:           "UnknownWechatErrorMapsToBadGateway",
			wxErr:          &wechat.WechatPayError{StatusCode: http.StatusBadGateway, Code: "UNKNOWN_ERROR", Message: "unknown"},
			expectedStatus: http.StatusBadGateway,
			expectedCode:   ErrApplymentWechatServiceUnavailable.Code,
			expectedError:  ErrApplymentWechatServiceUnavailable.Message,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/merchant/applyment/bindbank", nil)

			handled := respondApplymentWechatError(ctx, 101, "OUT_REQ_001", fmt.Errorf("submit applyment: %w", tc.wxErr))

			require.True(t, handled)
			require.Len(t, ctx.Errors, 1)
			require.Equal(t, tc.expectedStatus, recorder.Code)

			var resp ErrorResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
			require.Equal(t, tc.expectedCode, resp.Code)
			require.Equal(t, tc.expectedError, resp.Error)
			require.NotContains(t, resp.Error, "request_id")
		})
	}
}

func TestMerchantBindBankReturnsActionableMessageWithoutRequestIDWhenContactDocumentUploadFails(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)
	application := randomMerchantApplicationForApplyment(user.ID)
	applicationWithTestURL := application

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	wechatClient := mockwechat.NewMockWechatClient(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), int64(501)).
		AnyTimes().
		Return(db.MediaAsset{
			ID:               501,
			ObjectKey:        "id_card/front/1/20240101/contact-front.png",
			Visibility:       string(media.VisibilityPrivate),
			MediaCategory:    string(media.CategoryIDCardFront),
			MimeType:         "image/png",
			UploadedBy:       user.ID,
			UploadStatus:     "confirmed",
			ModerationStatus: "approved",
		}, nil)
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), int64(502)).
		AnyTimes().
		Return(db.MediaAsset{
			ID:               502,
			ObjectKey:        "id_card/back/1/20240101/contact-back.png",
			Visibility:       string(media.VisibilityPrivate),
			MediaCategory:    string(media.CategoryIDCardBack),
			MimeType:         "image/png",
			UploadedBy:       user.ID,
			UploadStatus:     "confirmed",
			ModerationStatus: "approved",
		}, nil)

	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

	store.EXPECT().
		GetUserMerchantApplication(gomock.Any(), user.ID).
		Times(1).
		Return(applicationWithTestURL, nil)

	store.EXPECT().
		CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
		Times(1).
		Return(randomEcommerceApplymentForTest("merchant", merchant.ID), nil)

	ordinaryClient.EXPECT().
		EncryptSensitiveData(gomock.Any()).
		Times(8).
		Return("encrypted_data", nil)

	expectedObjectKey := buildMerchantStorefrontQRCodeObjectKey(merchant.ID)
	requestID := "req-contact-upload-001"
	ordinaryClient.EXPECT().
		UploadImage(gomock.Any(), path.Base(expectedObjectKey), gomock.Any()).
		Times(1).
		Return(&ospcontracts.MediaUploadResponse{MediaID: "wx_store_qr_media_id"}, nil)
	ordinaryClient.EXPECT().
		UploadImage(gomock.Any(), "contact-front.png", gomock.Any()).
		Times(1).
		Return(nil, fmt.Errorf("request_id=%s: upload image: failed to generate signing nonce: boom", requestID))

	qrCodeData := buildTestQRCodePNG(t)
	store.EXPECT().
		GetMediaAssetByObjectKey(gomock.Any(), expectedObjectKey).
		Times(1).
		Return(db.MediaAsset{}, db.ErrRecordNotFound)
	wechatClient.EXPECT().
		GetWXACodeUnlimited(gomock.Any(), gomock.AssignableToTypeOf(&wechat.WXACodeRequest{})).
		Times(1).
		Return(qrCodeData, nil)
	store.EXPECT().
		CreateMediaAsset(gomock.Any(), gomock.AssignableToTypeOf(db.CreateMediaAssetParams{})).
		Times(1).
		Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "pending"}, nil)
	store.EXPECT().
		SetMediaAssetModerationStatus(gomock.Any(), gomock.AssignableToTypeOf(db.SetMediaAssetModerationStatusParams{})).
		Times(1).
		Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "approved"}, nil)

	server := newTestServerWithOrdinaryApplyment(t, store, ordinaryClient)
	server.wechatClient = wechatClient
	content := buildTestQRCodePNG(t)
	seedPrivateContactDocumentAsset(t, server, "id_card/front/1/20240101/contact-front.png", content)
	seedPrivateContactDocumentAsset(t, server, "id_card/back/1/20240101/contact-back.png", content)

	body := gin.H{
		"account_type":                      "ACCOUNT_TYPE_PRIVATE",
		"account_bank":                      "其他银行",
		"account_bank_code":                 1099,
		"bank_alias":                        "深圳前海微众银行",
		"bank_alias_code":                   "1000009561",
		"need_bank_branch":                  true,
		"bank_address_code":                 "440300",
		"bank_branch_id":                    "402584040001",
		"bank_name":                         "深圳前海微众银行深圳南山支行",
		"account_number":                    "6214830012345678",
		"account_name":                      "张三",
		"contact_type":                      "SUPER",
		"contact_name":                      "李四",
		"contact_id_doc_type":               "IDENTIFICATION_TYPE_MAINLAND_IDCARD",
		"contact_id_card_number":            "110101199202023456",
		"contact_id_doc_copy_asset_id":      501,
		"contact_id_doc_copy_back_asset_id": 502,
		"contact_id_doc_period_begin":       "2020-01-01",
		"contact_id_doc_period_end":         "2030-01-01",
		"contact_email":                     "merchant-contact@example.com",
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/applyment/bindbank", bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var response APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, CodeServiceUnavail, response.Code)
	require.Contains(t, response.Message, "超级管理员证件图片上传到微信失败")
	require.Contains(t, response.Message, "请联系平台管理员")
	require.NotContains(t, response.Message, "request_id")
	require.NotContains(t, response.Message, requestID)
}

func TestMerchantBindBankReturnsConfigGuidanceWhenStoreQRCodeUploadFails(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)
	application := randomMerchantApplicationForApplyment(user.ID)
	applicationWithTestURL := application

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	wechatClient := mockwechat.NewMockWechatClient(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

	store.EXPECT().
		GetUserMerchantApplication(gomock.Any(), user.ID).
		Times(1).
		Return(applicationWithTestURL, nil)

	store.EXPECT().
		CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
		Times(1).
		Return(randomEcommerceApplymentForTest("merchant", merchant.ID), nil)

	ordinaryClient.EXPECT().
		EncryptSensitiveData(gomock.Any()).
		Times(8).
		Return("encrypted_data", nil)

	expectedObjectKey := buildMerchantStorefrontQRCodeObjectKey(merchant.ID)
	requestID := "req-store-qr-002"
	ordinaryClient.EXPECT().
		UploadImage(gomock.Any(), path.Base(expectedObjectKey), gomock.Any()).
		Times(1).
		Return(nil, fmt.Errorf("request_id=%s: upload image: service provider merchant id must be configured explicitly for /v3/merchant/media/upload", requestID))

	qrCodeData := buildTestQRCodePNG(t)
	store.EXPECT().
		GetMediaAssetByObjectKey(gomock.Any(), expectedObjectKey).
		Times(1).
		Return(db.MediaAsset{}, db.ErrRecordNotFound)
	wechatClient.EXPECT().
		GetWXACodeUnlimited(gomock.Any(), gomock.AssignableToTypeOf(&wechat.WXACodeRequest{})).
		Times(1).
		Return(qrCodeData, nil)
	store.EXPECT().
		CreateMediaAsset(gomock.Any(), gomock.AssignableToTypeOf(db.CreateMediaAssetParams{})).
		Times(1).
		Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "pending"}, nil)
	store.EXPECT().
		SetMediaAssetModerationStatus(gomock.Any(), gomock.AssignableToTypeOf(db.SetMediaAssetModerationStatusParams{})).
		Times(1).
		Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "approved"}, nil)

	server := newTestServerWithOrdinaryApplyment(t, store, ordinaryClient)
	server.wechatClient = wechatClient

	body := gin.H{
		"account_type":      "ACCOUNT_TYPE_PRIVATE",
		"account_bank":      "招商银行",
		"bank_address_code": "440300",
		"account_number":    "6214830012345678",
		"account_name":      "张三",
		"contact_email":     "merchant-admin@example.com",
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/applyment/bindbank", bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var response APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, CodeServiceUnavail, response.Code)
	require.Contains(t, response.Message, "店铺首页二维码上传到微信失败")
	require.Contains(t, response.Message, "普通服务商图片上传配置不完整")
	require.Contains(t, response.Message, "请联系平台管理员")
	require.NotContains(t, response.Message, "request_id")
	require.NotContains(t, response.Message, requestID)
}

func TestMerchantBindBankSubmittedStateSyncFailed(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchantForApplyment(user.ID)
	application := randomMerchantApplicationForApplyment(user.ID)
	applicationWithTestURL := application
	applyment := randomEcommerceApplymentForTest("merchant", merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	wechatClient := mockwechat.NewMockWechatClient(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	store.EXPECT().
		GetLatestEcommerceApplymentBySubject(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, db.ErrRecordNotFound)

	store.EXPECT().
		GetUserMerchantApplication(gomock.Any(), user.ID).
		Times(1).
		Return(applicationWithTestURL, nil)

	store.EXPECT().
		CreateEcommerceApplyment(gomock.Any(), gomock.Any()).
		Times(1).
		Return(applyment, nil)

	ordinaryClient.EXPECT().
		EncryptSensitiveData(gomock.Any()).
		Times(8).
		Return("encrypted_data", nil)

	expectedObjectKey := buildMerchantStorefrontQRCodeObjectKey(merchant.ID)
	qrCodeData := buildTestQRCodePNG(t)

	store.EXPECT().
		GetMediaAssetByObjectKey(gomock.Any(), expectedObjectKey).
		Times(1).
		Return(db.MediaAsset{}, db.ErrRecordNotFound)

	wechatClient.EXPECT().
		GetWXACodeUnlimited(gomock.Any(), gomock.AssignableToTypeOf(&wechat.WXACodeRequest{})).
		Times(1).
		Return(qrCodeData, nil)

	store.EXPECT().
		CreateMediaAsset(gomock.Any(), gomock.AssignableToTypeOf(db.CreateMediaAssetParams{})).
		Times(1).
		Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "pending"}, nil)

	store.EXPECT().
		SetMediaAssetModerationStatus(gomock.Any(), gomock.AssignableToTypeOf(db.SetMediaAssetModerationStatusParams{})).
		Times(1).
		Return(db.MediaAsset{ID: util.RandomInt(1, 1000), ModerationStatus: "approved"}, nil)

	ordinaryClient.EXPECT().
		UploadImage(gomock.Any(), path.Base(expectedObjectKey), gomock.Any()).
		Times(1).
		Return(&ospcontracts.MediaUploadResponse{MediaID: "wx_store_qr_media_id"}, nil)

	ordinaryClient.EXPECT().
		SubmitApplyment(gomock.Any(), gomock.Any()).
		Times(1).
		Return(&ospcontracts.ApplymentSubmitResponse{ApplymentID: 123456789}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentToSubmitted(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.EcommerceApplyment{}, fmt.Errorf("update submitted failed"))

	store.EXPECT().
		UpdateMerchantStatus(gomock.Any(), db.UpdateMerchantStatusParams{
			ID:     merchant.ID,
			Status: "bindbank_submitted",
		}).
		Times(1).
		Return(db.Merchant{}, nil)

	server := newTestServerWithOrdinaryApplyment(t, store, ordinaryClient)
	server.wechatClient = wechatClient

	body := gin.H{
		"account_type":      "ACCOUNT_TYPE_PRIVATE",
		"account_bank":      "招商银行",
		"bank_address_code": "440300",
		"account_number":    "6214830012345678",
		"account_name":      "张三",
		"contact_email":     "merchant-admin@example.com",
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/applyment/bindbank", bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "微信支付开户申请已提交前后发生状态同步异常")
	require.Contains(t, recorder.Body.String(), "避免重复提交")
}
