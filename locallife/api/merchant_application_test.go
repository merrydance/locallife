package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/worker"
	mockworker "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 辅助函数 ====================

type stubFoodPermitOfficialVerifier struct {
	result logic.MerchantFoodPermitOfficialVerification
	err    error
}

func (stub stubFoodPermitOfficialVerifier) VerifyMerchantFoodPermit(ctx context.Context, rawResult []byte) (logic.MerchantFoodPermitOfficialVerification, error) {
	return stub.result, stub.err
}

func randomMerchantAppDraft(userID int64) db.MerchantApplication {
	return db.MerchantApplication{
		ID:                    1,
		UserID:                userID,
		MerchantName:          "",
		BusinessLicenseNumber: "",
		LegalPersonName:       "",
		LegalPersonIDNumber:   "",
		ContactPhone:          "",
		BusinessAddress:       "",
		Status:                "draft",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
}

func randomMerchantAppDraftWithData(userID int64) db.MerchantApplication {
	licenseOCR, _ := json.Marshal(BusinessLicenseOCRData{
		EnterpriseName:      "测试餐饮有限公司",
		CreditCode:          "91110000MA12345678",
		BusinessScope:       "餐饮服务",
		ValidPeriod:         "2020年01月01日至2040年01月01日",
		Address:             "北京市朝阳区测试路100号",
		LegalRepresentative: "张三",
		OCRAt:               time.Now().Format(time.RFC3339),
	})

	foodPermitOCR, _ := json.Marshal(FoodPermitOCRData{
		PermitNo:    "JY11105000000001",
		CompanyName: "测试餐饮有限公司",
		ValidTo:     "2030年12月31日",
		OCRAt:       time.Now().Format(time.RFC3339),
	})

	idCardFrontOCR, _ := json.Marshal(MerchantIDCardOCRData{
		Name:  "张三",
		OCRAt: time.Now().Format(time.RFC3339),
	})

	idCardBackOCR, _ := json.Marshal(MerchantIDCardOCRData{
		ValidDate: "2020.01.01-2030.01.01",
		OCRAt:     time.Now().Format(time.RFC3339),
	})

	return db.MerchantApplication{
		ID:                          1,
		UserID:                      userID,
		MerchantName:                "测试餐厅",
		BusinessLicenseNumber:       "91110000MA12345678",
		BusinessLicenseOcr:          licenseOCR,
		LegalPersonName:             "张三",
		LegalPersonIDNumber:         "110101199001011234",
		IDCardFrontOcr:              idCardFrontOCR,
		IDCardBackOcr:               idCardBackOCR,
		ContactPhone:                "13800138000",
		BusinessAddress:             "北京市朝阳区测试路100号1楼",
		BusinessScope:               pgtype.Text{String: "餐饮服务", Valid: true},
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: 2, Valid: true},
		IDCardFrontMediaAssetID:     pgtype.Int8{Int64: 3, Valid: true},
		IDCardBackMediaAssetID:      pgtype.Int8{Int64: 4, Valid: true},
		FoodPermitMediaAssetID:      pgtype.Int8{Int64: 1, Valid: true},
		FoodPermitOcr:               foodPermitOCR,
		Longitude:                   pgtype.Numeric{Int: big.NewInt(1163210000), Exp: -7, Valid: true},
		Latitude:                    pgtype.Numeric{Int: big.NewInt(399080000), Exp: -7, Valid: true},
		RegionID:                    pgtype.Int8{Int64: 1, Valid: true},
		Status:                      "draft",
		CreatedAt:                   time.Now(),
		UpdatedAt:                   time.Now(),
	}
}

func expectMerchantApplicationPublicDocumentLookups(store *mockdb.MockStore, userID int64) {
	assets := map[int64]db.MediaAsset{
		1: {
			ID:               1,
			UploadedBy:       userID,
			Visibility:       "public",
			MediaCategory:    "food_permit",
			ModerationStatus: "pending",
			ObjectKey:        "merchant/applications/1/food-permit.jpg",
		},
		2: {
			ID:               2,
			UploadedBy:       userID,
			Visibility:       "public",
			MediaCategory:    "business_license",
			ModerationStatus: "pending",
			ObjectKey:        "merchant/applications/1/business-license.jpg",
		},
	}

	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), gomock.Any()).
		AnyTimes().
		DoAndReturn(func(_ context.Context, mediaID int64) (db.MediaAsset, error) {
			asset, ok := assets[mediaID]
			if !ok {
				return db.MediaAsset{}, db.ErrRecordNotFound
			}
			return asset, nil
		})
}

func TestMerchantFoodPermitNeedsOfficialVerification(t *testing.T) {
	t.Parallel()

	license := logic.MerchantReviewBusinessLicenseOCRData{EnterpriseName: "宁晋县周鹏饭店"}

	require.False(t, merchantFoodPermitNeedsOfficialVerification(license, logic.MerchantReviewFoodPermitOCRData{CompanyName: "宁晋县周鹏饭店"}))
	require.False(t, merchantFoodPermitNeedsOfficialVerification(
		logic.MerchantReviewBusinessLicenseOCRData{EnterpriseName: "测试食品有限公司"},
		logic.MerchantReviewFoodPermitOCRData{CompanyName: "测试食品有限公司"},
	))
	require.True(t, merchantFoodPermitNeedsOfficialVerification(license, logic.MerchantReviewFoodPermitOCRData{CompanyName: "食品小作坊小餐饮登记证2130528020270"}))
	require.True(t, merchantFoodPermitNeedsOfficialVerification(license, logic.MerchantReviewFoodPermitOCRData{CompanyName: ""}))
}

func TestMerchantFoodPermitOfficialVerificationMatchesLicense(t *testing.T) {
	t.Parallel()

	app := db.MerchantApplication{BusinessLicenseNumber: "92130528MA0A5XB46A"}
	license := logic.MerchantReviewBusinessLicenseOCRData{CreditCode: " 92130528ma0a5xb46a "}

	require.True(t, merchantFoodPermitOfficialVerificationMatchesLicense(app, license, logic.MerchantFoodPermitOfficialVerification{
		CreditCode: "92130528MA0A5XB46A",
	}))
	require.True(t, merchantFoodPermitOfficialVerificationMatchesLicense(app, logic.MerchantReviewBusinessLicenseOCRData{}, logic.MerchantFoodPermitOfficialVerification{}))
	require.False(t, merchantFoodPermitOfficialVerificationMatchesLicense(app, license, logic.MerchantFoodPermitOfficialVerification{
		CreditCode: "91110000MA12345678",
	}))
}

func TestMerchantNameLocationQueriesUseOnlyLicenseBoundNames(t *testing.T) {
	app := randomMerchantAppDraftWithData(1)
	app.MerchantName = "附近热门店"
	review := logic.MerchantDocumentReviewResult{
		LicenseName:    "张三饭店",
		LicenseAddress: "河北省邢台市宁晋县天宝西街",
	}

	queries := merchantNameLocationQueries(app, review)

	require.NotEmpty(t, queries)
	for _, query := range queries {
		require.NotContains(t, query, "附近热门店")
	}
	require.Contains(t, queries, "张三饭店")
}

func TestCheckMerchantApplicationApproval_UsesBusinessLicenseReadiness(t *testing.T) {
	server := &Server{}
	app := randomMerchantAppDraftWithData(1)
	app.BusinessLicenseOcr = []byte(`{"enterprise_name":"测试餐饮有限公司","readiness":{"state":"partial","reason_code":"required_field_missing","missing_fields":["valid_period"]}}`)

	err := server.checkMerchantApplicationApproval(nil, app)

	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	require.Equal(t, ErrBusinessLicenseRequired.Code, apiErr.Code)
	require.Equal(t, "营业执照有效期未识别，请重新上传清晰完整的营业执照照片", apiErr.Message)
}

func TestCheckMerchantApplicationApproval_UsesFoodPermitProviderFailure(t *testing.T) {
	server := &Server{}
	app := randomMerchantAppDraftWithData(1)
	app.FoodPermitOcr = []byte(`{"company_name":"测试餐饮有限公司","valid_to":"2030年12月31日","readiness":{"state":"provider_failed","reason_code":"provider_error"}}`)

	err := server.checkMerchantApplicationApproval(nil, app)

	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	require.Equal(t, ErrFoodLicenseRequired.Code, apiErr.Code)
	require.Equal(t, "食品经营许可证OCR处理失败，请重新上传清晰完整的食品经营许可证照片", apiErr.Message)
}

// ==================== 获取或创建草稿测试 ====================

func TestGetOrCreateMerchantApplicationDraft(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "CreateNew",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(db.MerchantApplication{}, db.ErrRecordNotFound)

				newApp := randomMerchantAppDraft(user.ID)
				store.EXPECT().
					CreateMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(newApp, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "GetExisting",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				existingApp := randomMerchantAppDraft(user.ID)
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(existingApp, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "GetExisting_SubmittedAutoResetToDraft",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				existingApp := randomMerchantAppDraft(user.ID)
				existingApp.Status = "submitted"
				resetApp := existingApp
				resetApp.Status = "draft"

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(existingApp, nil)
				store.EXPECT().
					ResetMerchantApplicationTx(gomock.Any(), db.ResetMerchantApplicationTxParams{
						ApplicationID: existingApp.ID,
						UserID:        user.ID,
					}).
					Times(1).
					Return(db.ResetMerchantApplicationTxResult{Application: resetApp}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "AlreadyApproved",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				approvedApp := randomMerchantAppDraft(user.ID)
				approvedApp.Status = "approved"
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(approvedApp, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:       "NoAuth",
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectMerchantApplicationPublicDocumentLookups(store, user.ID)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			request, err := http.NewRequest(http.MethodGet, "/v1/merchant/application", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 更新基础信息测试 ====================

func TestUpdateMerchantApplicationBasicInfo(t *testing.T) {
	user, _ := randomUser(t)
	testCases := []struct {
		name          string
		body          updateMerchantBasicInfoRequest
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: updateMerchantBasicInfoRequest{
				MerchantName:          "新店名",
				ContactPhone:          "13800138001",
				BusinessAddress:       "北京市海淀区测试路200号",
				BusinessLicenseNumber: "91110000MA12345678",
				BusinessScope:         "餐饮服务;热食类食品制售",
				LegalPersonName:       "李四",
				LegalPersonIDNumber:   "110101199001011234",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomMerchantAppDraft(user.ID)
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				updatedApp := app
				updatedApp.MerchantName = "新店名"
				updatedApp.ContactPhone = "13800138001"
				updatedApp.BusinessAddress = "北京市海淀区测试路200号"
				updatedApp.BusinessLicenseNumber = "91110000MA12345678"
				updatedApp.BusinessScope = pgtype.Text{String: "餐饮服务;热食类食品制售", Valid: true}
				updatedApp.LegalPersonName = "李四"
				updatedApp.LegalPersonIDNumber = "110101199001011234"
				store.EXPECT().
					UpdateMerchantApplicationBasicInfo(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationBasicInfoParams) (db.MerchantApplication, error) {
						require.Equal(t, "新店名", arg.MerchantName.String)
						require.Equal(t, "13800138001", arg.ContactPhone.String)
						require.Equal(t, "北京市海淀区测试路200号", arg.BusinessAddress.String)
						require.Equal(t, "91110000MA12345678", arg.BusinessLicenseNumber.String)
						require.Equal(t, "餐饮服务;热食类食品制售", arg.BusinessScope.String)
						require.Equal(t, "李四", arg.LegalPersonName.String)
						require.Equal(t, "110101199001011234", arg.LegalPersonIDNumber.String)
						return updatedApp, nil
					})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Submitted_AutoResetToDraft",
			body: updateMerchantBasicInfoRequest{
				MerchantName: "新店名",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomMerchantAppDraft(user.ID)
				app.Status = "submitted"
				resetApp := app
				resetApp.Status = "draft"
				updatedApp := resetApp
				updatedApp.MerchantName = "新店名"
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					ResetMerchantApplicationTx(gomock.Any(), db.ResetMerchantApplicationTxParams{
						ApplicationID: app.ID,
						UserID:        user.ID,
					}).
					Times(1).
					Return(db.ResetMerchantApplicationTxResult{Application: resetApp}, nil)
				store.EXPECT().
					UpdateMerchantApplicationBasicInfo(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationBasicInfoParams) (db.MerchantApplication, error) {
						require.Equal(t, resetApp.ID, arg.ID)
						require.Equal(t, "新店名", arg.MerchantName.String)
						return updatedApp, nil
					})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Approved_AutoResetToDraft",
			body: updateMerchantBasicInfoRequest{
				MerchantName:    "修正后的店名",
				BusinessAddress: "北京市朝阳区修正路300号",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				app.Status = "approved"

				resetApp := app
				resetApp.Status = "draft"

				updatedApp := resetApp
				updatedApp.MerchantName = "修正后的店名"
				updatedApp.BusinessAddress = "北京市朝阳区修正路300号"

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					ResetMerchantApplicationTx(gomock.Any(), db.ResetMerchantApplicationTxParams{
						ApplicationID: app.ID,
						UserID:        user.ID,
					}).
					Times(1).
					Return(db.ResetMerchantApplicationTxResult{Application: resetApp}, nil)

				store.EXPECT().
					UpdateMerchantApplicationBasicInfo(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationBasicInfoParams) (db.MerchantApplication, error) {
						require.Equal(t, resetApp.ID, arg.ID)
						require.Equal(t, "修正后的店名", arg.MerchantName.String)
						require.Equal(t, "北京市朝阳区修正路300号", arg.BusinessAddress.String)
						return updatedApp, nil
					})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantApplicationDraftResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "draft", resp.Status)
				require.Equal(t, "修正后的店名", resp.MerchantName)
				require.Equal(t, "北京市朝阳区修正路300号", resp.BusinessAddress)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectMerchantApplicationPublicDocumentLookups(store, user.ID)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPut, "/v1/merchant/application/basic", bytes.NewReader(body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestDeleteMerchantApplicationDocument(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		documentType  string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:         "OKBusinessLicense",
			documentType: "business_license",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				updatedApp := app
				updatedApp.BusinessLicenseMediaAssetID = pgtype.Int8{}
				updatedApp.BusinessLicenseOcr = nil

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					ClearMerchantApplicationBusinessLicense(gomock.Any(), app.ID).
					Times(1).
					Return(updatedApp, nil)

				store.EXPECT().
					SoftDeleteMediaAsset(gomock.Any(), int64(2)).
					Times(1).
					Return(db.MediaAsset{ID: 2, UploadedBy: user.ID}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response merchantApplicationDraftResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Nil(t, response.BusinessLicenseMediaAssetID)
				require.Nil(t, response.BusinessLicenseOCR)
			},
		},
		{
			name:         "InvalidDocumentType",
			documentType: "invalid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
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
			expectMerchantApplicationPublicDocumentLookups(store, user.ID)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			request, err := http.NewRequest(http.MethodDelete, "/v1/merchant/application/documents/"+tc.documentType, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 提交申请测试 ====================

func TestSubmitMerchantApplication(t *testing.T) {
	user, _ := randomUser(t)
	testCases := []struct {
		name            string
		setupAuth       func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs      func(t *testing.T, store *mockdb.MockStore)
		configureServer func(server *Server)
		checkResponse   func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "Approved",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitMerchantApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{}, nil)

				store.EXPECT().
					CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
						BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
						ID:                    submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
						LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
						ID:                  submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				approvedApp := submittedApp
				approvedApp.Status = "approved"
				store.EXPECT().
					ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveMerchantApplicationTxResult{
						Application: approvedApp,
						Merchant:    db.Merchant{ID: 1},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantApplicationDraftResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "approved", resp.Status)
			},
		},
		{
			name: "Approved_AddressMatchesReverseGeocode",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitMerchantApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{}, nil)

				store.EXPECT().
					CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
						BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
						ID:                    submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
						LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
						ID:                  submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				approvedApp := submittedApp
				approvedApp.Status = "approved"
				store.EXPECT().
					ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveMerchantApplicationTxResult{
						Application: approvedApp,
						Merchant:    db.Merchant{ID: 1},
					}, nil)
			},
			configureServer: func(server *Server) {
				server.mapClient = stubMapClient{reverseResult: &maps.ReverseGeocodeResult{
					Address:          "北京市朝阳区测试路100号",
					FormattedAddress: "北京市朝阳区测试路100号",
					Province:         "北京市",
					City:             "北京市",
					District:         "朝阳区",
					Street:           "测试路",
					StreetNumber:     "100号",
				}}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "BadRequest_ReverseGeocodeAddressMismatch",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			configureServer: func(server *Server) {
				server.mapClient = stubMapClient{reverseResult: &maps.ReverseGeocodeResult{
					Address:          "北京市海淀区光华路200号",
					FormattedAddress: "北京市海淀区光华路200号",
					Province:         "北京市",
					City:             "北京市",
					District:         "海淀区",
					Street:           "光华路",
					StreetNumber:     "200号",
				}}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, ErrInvalidAddress.Code, response.Code)
				require.Contains(t, response.Error, "地图定位与营业执照注册地址不一致")
				require.Contains(t, response.Error, "北京市海淀区光华路200号")
			},
		},
		{
			name: "Approved_AddressMatchesGeocodedLicenseWithin1000Meters",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitMerchantApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{}, nil)

				store.EXPECT().
					CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
						BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
						ID:                    submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
						LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
						ID:                  submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				approvedApp := submittedApp
				approvedApp.Status = "approved"
				store.EXPECT().
					ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveMerchantApplicationTxResult{
						Application: approvedApp,
						Merchant:    db.Merchant{ID: 1},
					}, nil)
			},
			configureServer: func(server *Server) {
				server.mapClient = stubMapClient{
					geocodeResult: &maps.GeocodeResult{
						Location: maps.Location{Lat: 39.9152, Lng: 116.3255},
						Address:  "北京市朝阳区测试路100号",
					},
					reverseResult: &maps.ReverseGeocodeResult{
						Address:          "北京市海淀区光华路200号",
						FormattedAddress: "北京市海淀区光华路200号",
						Province:         "北京市",
						City:             "北京市",
						District:         "海淀区",
						Street:           "光华路",
						StreetNumber:     "200号",
					},
				}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Approved_AddressMatchesMerchantNamePOIWithin1000Meters",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				app.MerchantName = "宁晋县周鹏饭店"
				app.BusinessAddress = "河北省邢台市宁晋县凤凰路辅路"
				licenseOCR, err := json.Marshal(BusinessLicenseOCRData{
					EnterpriseName:      "宁晋县周鹏饭店",
					CreditCode:          "92130528MA0A5XB46A",
					BusinessScope:       "餐饮服务",
					ValidPeriod:         "2020年01月01日至2040年01月01日",
					Address:             "邢台市宁晋县天宝西街与宁米路交叉口北行100米路东",
					LegalRepresentative: "张三",
					OCRAt:               time.Now().Format(time.RFC3339),
				})
				require.NoError(t, err)
				app.BusinessLicenseOcr = licenseOCR
				foodPermitOCR, err := json.Marshal(FoodPermitOCRData{
					PermitNo:    "JY11105000000001",
					CompanyName: "宁晋县周鹏饭店",
					ValidTo:     "2030年12月31日",
					OCRAt:       time.Now().Format(time.RFC3339),
				})
				require.NoError(t, err)
				app.FoodPermitOcr = foodPermitOCR
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitMerchantApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{}, nil)

				store.EXPECT().
					CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
						BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
						ID:                    submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
						LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
						ID:                  submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				approvedApp := submittedApp
				approvedApp.Status = "approved"
				store.EXPECT().
					ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveMerchantApplicationTxResult{
						Application: approvedApp,
						Merchant:    db.Merchant{ID: 1},
					}, nil)
			},
			configureServer: func(server *Server) {
				server.mapClient = stubMapClient{
					geocodeFunc: func(_ context.Context, address string) (*maps.GeocodeResult, error) {
						if strings.Contains(address, "宁晋县周鹏饭店") {
							return &maps.GeocodeResult{
								Location: maps.Location{Lat: 39.9080, Lng: 116.3210},
								Address:  "河北省邢台市宁晋县天宝街辅路宁晋县周鹏饭店",
							}, nil
						}
						return &maps.GeocodeResult{
							Location: maps.Location{Lat: 39.9350, Lng: 116.3500},
							Address:  "河北省邢台市宁晋县天宝西街",
						}, nil
					},
					reverseResult: &maps.ReverseGeocodeResult{
						Address:          "河北省邢台市宁晋县凤凰路辅路",
						FormattedAddress: "河北省邢台市宁晋县凤凰路辅路",
						Province:         "河北省",
						City:             "邢台市",
						District:         "宁晋县",
						Street:           "凤凰路辅路",
					},
				}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "BadRequest_GeocodedLicenseAddressBeyond1000Meters",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			configureServer: func(server *Server) {
				server.mapClient = stubMapClient{
					geocodeResult: &maps.GeocodeResult{
						Location: maps.Location{Lat: 39.9350, Lng: 116.3500},
						Address:  "北京市朝阳区测试路100号",
					},
					reverseResult: &maps.ReverseGeocodeResult{
						Address:          "北京市海淀区光华路200号",
						FormattedAddress: "北京市海淀区光华路200号",
						Province:         "北京市",
						City:             "北京市",
						District:         "海淀区",
						Street:           "光华路",
						StreetNumber:     "200号",
					},
				}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, ErrInvalidAddress.Code, response.Code)
				require.Contains(t, response.Error, "地图定位与营业执照注册地址不一致")
			},
		},
		{
			name: "BadRequest_NoBusinessScopeKeepsDraft",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				licenseOCR, _ := json.Marshal(BusinessLicenseOCRData{
					EnterpriseName: "测试科技有限公司",
					CreditCode:     "91110000MA12345678",
					BusinessScope:  "软件开发、技术服务",
					ValidPeriod:    "2020年01月01日至2040年01月01日",
					Address:        "北京市朝阳区测试路100号",
				})
				app.BusinessLicenseOcr = licenseOCR

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "经营范围未识别到餐饮相关内容")
			},
		},
		{
			name: "Approved_FoodPermitRawTextFallback",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				ocrJobID := int64(501)
				foodPermitOCR, err := json.Marshal(FoodPermitOCRData{
					OCRJobID:     &ocrJobID,
					PermitNo:     "JY11105000000001",
					CompanyName:  "地址：生祠经营场所面积在50平米以上的小餐饮办理《食品河北省邢台市宁晋县经济开发区希望路北段路东",
					OperatorName: "张三",
					ValidTo:      "2030年12月31日",
					RawText:      "经营者名称：测试餐饮有限公司\n经营场所：北京市朝阳区测试路100号1楼\n许可证编号：JY11105000000001\n有效期至：2030年12月31日",
					OCRAt:        time.Now().Format(time.RFC3339),
				})
				require.NoError(t, err)
				app.FoodPermitOcr = foodPermitOCR

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					GetOCRJob(gomock.Any(), ocrJobID).
					Times(1).
					Return(db.OcrJob{}, db.ErrRecordNotFound)

				store.EXPECT().
					UpdateMerchantApplicationFoodPermit(gomock.Any(), gomock.Any()).
					Times(1).
					Return(app, nil)

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitMerchantApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{}, nil)

				store.EXPECT().
					CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
						BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
						ID:                    submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
						LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
						ID:                  submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				approvedApp := submittedApp
				approvedApp.Status = "approved"
				store.EXPECT().
					ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveMerchantApplicationTxResult{
						Application: approvedApp,
						Merchant:    db.Merchant{ID: 1},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantApplicationDraftResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "approved", resp.Status)
			},
		},
		{
			name: "Approved_FoodPermitOCRJobNormalizedFallback",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				ocrJobID := int64(777)
				foodPermitOCR, err := json.Marshal(FoodPermitOCRData{
					OCRJobID:    &ocrJobID,
					PermitNo:    "",
					CompanyName: "地址：生祠经营场所面积在50平米以上的小餐饮办理《食品河北省邢台市宁晋县经济开发区希望路北段路东",
					ValidTo:     "",
					RawText:     "经营场所：北京市朝阳区测试路100号1楼\n许可证编号：JY11105000000001",
					OCRAt:       time.Now().Format(time.RFC3339),
				})
				require.NoError(t, err)
				app.FoodPermitOcr = foodPermitOCR

				normalizedResult, err := ocr.MarshalNormalizedResult(ocr.NormalizedResult{
					DocumentType: ocr.DocumentTypeFoodPermit,
					RecognizedAt: time.Now(),
					FoodPermit: &ocr.FoodPermitResult{
						LicenseNumber: "JY11105000000001",
						BusinessName:  "测试餐饮有限公司",
						OperatorName:  "张三",
						ValidPeriod:   "2030年12月31日",
						RawText:       "主体名称：测试餐饮有限公司\n经营场所：北京市朝阳区测试路100号1楼\n许可证编号：JY11105000000001\n有效期：2030年12月31日",
					},
				})
				require.NoError(t, err)

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					GetOCRJob(gomock.Any(), ocrJobID).
					Times(1).
					Return(db.OcrJob{ID: ocrJobID, Provider: "aliyun", Status: "succeeded", NormalizedResult: normalizedResult}, nil)

				store.EXPECT().
					UpdateMerchantApplicationFoodPermit(gomock.Any(), gomock.Any()).
					Times(1).
					Return(app, nil)

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitMerchantApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{}, nil)

				store.EXPECT().
					CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
						BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
						ID:                    submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
						LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
						ID:                  submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				approvedApp := submittedApp
				approvedApp.Status = "approved"
				store.EXPECT().
					ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveMerchantApplicationTxResult{
						Application: approvedApp,
						Merchant:    db.Merchant{ID: 1},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantApplicationDraftResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "approved", resp.Status)
			},
		},
		{
			name: "Approved_FoodPermitOfficialQRCodeFallback",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				ocrJobID := int64(779)
				licenseOCR, err := json.Marshal(BusinessLicenseOCRData{
					EnterpriseName:      "宁晋县周鹏饭店",
					CreditCode:          "92130528MA0A5XB46A",
					BusinessScope:       "餐饮服务",
					ValidPeriod:         "长期",
					Address:             "河北省邢台市宁晋县经济开发区吉祥路与晶龙街交叉口东侧",
					LegalRepresentative: "周松涛",
					OCRAt:               time.Now().Format(time.RFC3339),
				})
				require.NoError(t, err)
				app.BusinessLicenseOcr = licenseOCR
				app.BusinessLicenseNumber = "92130528MA0A5XB46A"
				app.LegalPersonName = "周松涛"
				app.LegalPersonIDNumber = "130528199001011234"
				app.BusinessAddress = "河北省邢台市宁晋县经济开发区吉祥路与晶龙街交叉口东侧"
				idCardFrontOCR, err := json.Marshal(MerchantIDCardOCRData{
					Name:  "周松涛",
					OCRAt: time.Now().Format(time.RFC3339),
				})
				require.NoError(t, err)
				app.IDCardFrontOcr = idCardFrontOCR

				foodPermitOCR, err := json.Marshal(FoodPermitOCRData{
					OCRJobID:     &ocrJobID,
					PermitNo:     "",
					CompanyName:  "",
					OperatorName: "",
					ValidTo:      "2028年12月21日",
					RawText:      "主体名称：食品小作坊小餐饮登记证2130528020270\n地址：生祠经营场所面积在50平米以上的小餐饮办理《食品\n有效期至：2028年12月21日",
					Readiness: &OCRReadiness{
						State:      "ready",
						ReasonCode: "ok",
					},
					OCRAt: time.Now().Format(time.RFC3339),
				})
				require.NoError(t, err)
				app.FoodPermitOcr = foodPermitOCR

				normalizedResult, err := ocr.MarshalNormalizedResult(ocr.NormalizedResult{
					DocumentType: ocr.DocumentTypeFoodPermit,
					RecognizedAt: time.Now(),
					FoodPermit: &ocr.FoodPermitResult{
						BusinessName: "食品小作坊小餐饮登记证2130528020270",
						ValidPeriod:  "2028年12月21日",
						RawText:      "主体名称：食品小作坊小餐饮登记证2130528020270\n地址：生祠经营场所面积在50平米以上的小餐饮办理《食品\n有效期至：2028年12月21日",
					},
				})
				require.NoError(t, err)
				rawResult := []byte(`{"Data":"{\"codes\":[{\"data\":\"http://121.28.87.7:8081/OrcodeXcyXzf.jsp?flowId=86&zsId=655926252\",\"type\":\"QRcode\"}],\"data\":{\"operatorName\":\"食品小作坊小餐饮登记证2130528020270\",\"validToDate\":\"2028年12月21日\"}}"}`)

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					GetOCRJob(gomock.Any(), ocrJobID).
					Times(1).
					Return(db.OcrJob{ID: ocrJobID, Provider: "aliyun", Status: "succeeded", NormalizedResult: normalizedResult, RawResult: rawResult}, nil)

				store.EXPECT().
					UpdateMerchantApplicationFoodPermit(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationFoodPermitParams) (db.MerchantApplication, error) {
						require.Equal(t, app.ID, arg.ID)
						var repaired FoodPermitOCRData
						require.NoError(t, json.Unmarshal(arg.FoodPermitOcr, &repaired))
						require.Equal(t, "宁晋县周鹏饭店", repaired.CompanyName)
						require.Equal(t, "周松涛", repaired.OperatorName)
						require.Equal(t, "2130528020270", repaired.PermitNo)
						require.Equal(t, "2028年12月21日", repaired.ValidTo)
						require.Contains(t, repaired.RawText, "主体名称：宁晋县周鹏饭店")
						app.FoodPermitOcr = arg.FoodPermitOcr
						return app, nil
					})

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitMerchantApplication(gomock.Any(), app.ID).
					Times(1).
					DoAndReturn(func(context.Context, int64) (db.MerchantApplication, error) {
						submittedApp.FoodPermitOcr = app.FoodPermitOcr
						return submittedApp, nil
					})

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{}, nil)

				store.EXPECT().
					CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
						BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
						ID:                    submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
						LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
						ID:                  submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				approvedApp := submittedApp
				approvedApp.Status = "approved"
				store.EXPECT().
					ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveMerchantApplicationTxResult{
						Application: approvedApp,
						Merchant:    db.Merchant{ID: 1},
					}, nil)
			},
			configureServer: func(server *Server) {
				server.foodPermitOfficialVerifier = stubFoodPermitOfficialVerifier{
					result: logic.MerchantFoodPermitOfficialVerification{
						CompanyName:  "宁晋县周鹏饭店",
						OperatorName: "周松涛",
						PermitNo:     "2130528020270",
						CreditCode:   "92130528MA0A5XB46A",
						Address:      "河北省邢台市宁晋县经济开发区吉祥路与晶龙街交叉口东侧",
						ValidTo:      "2028年12月21日",
					},
				}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantApplicationDraftResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "approved", resp.Status)
			},
		},
		{
			name: "Approved_BusinessLicenseOCRRawValidDateFallback",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				ocrJobID := int64(778)
				businessLicenseOCR, err := json.Marshal(BusinessLicenseOCRData{
					OCRJobID:            &ocrJobID,
					EnterpriseName:      "测试餐饮有限公司",
					CreditCode:          "91110000MA12345678",
					BusinessScope:       "餐饮服务",
					Address:             "北京市朝阳区测试路100号",
					LegalRepresentative: "张三",
					Readiness: &OCRReadiness{
						State:         "partial",
						ReasonCode:    "required_field_missing",
						MissingFields: []string{"valid_period"},
					},
					OCRAt: time.Now().Format(time.RFC3339),
				})
				require.NoError(t, err)
				app.BusinessLicenseOcr = businessLicenseOCR

				rawResult := []byte(`{"Data":"{\"data\":{\"creditCode\":\"91110000MA12345678\",\"companyName\":\"测试餐饮有限公司\",\"businessAddress\":\"北京市朝阳区测试路100号\",\"legalPerson\":\"张三\",\"businessScope\":\"餐饮服务\",\"validFromDate\":\"20170104\",\"validToDate\":\"29991231\"}}"}`)

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					GetOCRJob(gomock.Any(), ocrJobID).
					Times(1).
					Return(db.OcrJob{ID: ocrJobID, Provider: "aliyun", Status: "succeeded", RawResult: rawResult}, nil)

				store.EXPECT().
					UpdateMerchantApplicationBusinessLicense(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationBusinessLicenseParams) (db.MerchantApplication, error) {
						require.Equal(t, app.ID, arg.ID)
						var repaired BusinessLicenseOCRData
						require.NoError(t, json.Unmarshal(arg.BusinessLicenseOcr, &repaired))
						require.Equal(t, "2017年01月04日至长期", repaired.ValidPeriod)
						app.BusinessLicenseOcr = arg.BusinessLicenseOcr
						return app, nil
					})

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitMerchantApplication(gomock.Any(), app.ID).
					Times(1).
					DoAndReturn(func(context.Context, int64) (db.MerchantApplication, error) {
						submittedApp.BusinessLicenseOcr = app.BusinessLicenseOcr
						return submittedApp, nil
					})

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{}, nil)

				store.EXPECT().
					CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
						BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
						ID:                    submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
						LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
						ID:                  submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				approvedApp := submittedApp
				approvedApp.Status = "approved"
				store.EXPECT().
					ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveMerchantApplicationTxResult{
						Application: approvedApp,
						Merchant:    db.Merchant{ID: 1},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantApplicationDraftResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "approved", resp.Status)
			},
		},
		{
			name: "Approved_FoodPermitRegistrationTitleIgnored",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				foodPermitOCR, err := json.Marshal(FoodPermitOCRData{
					PermitNo:     "2130528020946",
					CompanyName:  "食品小作坊小餐饮登记证",
					OperatorName: "张三",
					ValidTo:      "2030年12月31日",
					RawText:      "食品小作坊小餐饮登记证\n商号名称：测试餐饮有限公司\n经营者姓名：张三\n登记证编号：2130528020946\n有效期至：2030年12月31日",
					OCRAt:        time.Now().Format(time.RFC3339),
				})
				require.NoError(t, err)
				app.FoodPermitOcr = foodPermitOCR

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					UpdateMerchantApplicationFoodPermit(gomock.Any(), gomock.Any()).
					Times(1).
					Return(app, nil)

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitMerchantApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{}, nil)

				store.EXPECT().
					CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
						BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
						ID:                    submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
						LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
						ID:                  submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				approvedApp := submittedApp
				approvedApp.Status = "approved"
				store.EXPECT().
					ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveMerchantApplicationTxResult{
						Application: approvedApp,
						Merchant:    db.Merchant{ID: 1},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Approved_FoodPermitOperatorFallbackForRegistrationCert",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				foodPermitOCR, err := json.Marshal(FoodPermitOCRData{
					PermitNo:     "",
					CompanyName:  "",
					OperatorName: "",
					ValidTo:      "2030年12月31日",
					RawText:      "食品小作坊小餐饮登记证\n经营者姓名：张三\n登记证编号：2130528020946\n有效期至：2030年12月31日",
					OCRAt:        time.Now().Format(time.RFC3339),
				})
				require.NoError(t, err)
				app.FoodPermitOcr = foodPermitOCR

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					UpdateMerchantApplicationFoodPermit(gomock.Any(), gomock.Any()).
					Times(1).
					Return(app, nil)

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitMerchantApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{}, nil)

				store.EXPECT().
					CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
						BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
						ID:                    submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
						LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
						ID:                  submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				approvedApp := submittedApp
				approvedApp.Status = "approved"
				store.EXPECT().
					ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveMerchantApplicationTxResult{
						Application: approvedApp,
						Merchant:    db.Merchant{ID: 1},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "BadRequest_FoodPermitNameUnreadable",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				ocrJobID := int64(501)
				foodPermitOCR, err := json.Marshal(FoodPermitOCRData{
					OCRJobID:     &ocrJobID,
					PermitNo:     "JY11105000000001",
					CompanyName:  "地址：生祠经营场所面积在50平米以上的小餐饮办理《食品河北省邢台市宁晋县经济开发区希望路北段路东",
					OperatorName: "张三",
					ValidTo:      "2030年12月31日",
					RawText:      "经营场所：北京市朝阳区测试路100号1楼\n许可证编号：JY11105000000001\n有效期至：2030年12月31日",
					OCRAt:        time.Now().Format(time.RFC3339),
				})
				require.NoError(t, err)
				app.FoodPermitOcr = foodPermitOCR

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					GetOCRJob(gomock.Any(), ocrJobID).
					Times(2).
					Return(db.OcrJob{ID: ocrJobID, Provider: "wechat", Status: "succeeded"}, nil)

			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, ErrMerchantFoodPermitNameUnreadable.Code, response.Code)
				require.Equal(t, ErrMerchantFoodPermitNameUnreadable.Message, response.Error)
			},
		},
		{
			name: "BadRequest_FoodPermitNameMismatch",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				foodPermitOCR, err := json.Marshal(FoodPermitOCRData{
					PermitNo:     "JY11105000000001",
					CompanyName:  "另一家餐饮店",
					OperatorName: "李四",
					ValidTo:      "2030年12月31日",
					RawText:      "经营者名称：另一家餐饮店\n经营场所：北京市朝阳区测试路100号1楼\n许可证编号：JY11105000000001\n有效期至：2030年12月31日",
					OCRAt:        time.Now().Format(time.RFC3339),
				})
				require.NoError(t, err)
				app.FoodPermitOcr = foodPermitOCR

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response ErrorResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, ErrMerchantFoodPermitNameMismatch.Code, response.Code)
				require.Contains(t, response.Error, "食品经营许可证主体名称与营业执照企业名称不一致")
				require.Contains(t, response.Error, "营业执照：测试餐饮有限公司")
				require.Contains(t, response.Error, "食品经营许可证：另一家餐饮店")
			},
		},
		{
			name: "MissingFoodPermit",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				app.FoodPermitMediaAssetID = pgtype.Int8{}
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "MissingRegionID",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				app.RegionID = pgtype.Int8{Valid: false}
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "Approved_WithoutContactPhone",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				app.MerchantName = ""
				app.ContactPhone = ""

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					GetUser(gomock.Any(), user.ID).
					Times(1).
					Return(db.User{ID: user.ID}, nil)

				derivedApp := app
				derivedApp.MerchantName = "测试餐饮有限公司"
				store.EXPECT().
					UpdateMerchantApplicationBasicInfo(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationBasicInfoParams) (db.MerchantApplication, error) {
						require.Equal(t, app.ID, arg.ID)
						require.True(t, arg.MerchantName.Valid)
						require.Equal(t, "测试餐饮有限公司", arg.MerchantName.String)
						require.False(t, arg.ContactPhone.Valid)
						return derivedApp, nil
					})

				submittedApp := derivedApp
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitMerchantApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{}, nil)

				store.EXPECT().
					CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
						BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
						ID:                    submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
						LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
						ID:                  submittedApp.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				approvedApp := submittedApp
				approvedApp.Status = "approved"
				store.EXPECT().
					ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.ApproveMerchantApplicationTxParams) (db.ApproveMerchantApplicationTxResult, error) {
						require.Equal(t, "测试餐饮有限公司", arg.MerchantName)
						require.Equal(t, "", arg.Phone)
						return db.ApproveMerchantApplicationTxResult{
							Application: approvedApp,
							Merchant:    db.Merchant{ID: 1},
						}, nil
					})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantApplicationDraftResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "approved", resp.Status)
				require.Equal(t, "测试餐饮有限公司", resp.MerchantName)
				require.Equal(t, "", resp.ContactPhone)
			},
		},
		{
			name: "SubmittedStatus_Retry",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				app.Status = "submitted"

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), app.RegionID.Int64).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{}, nil)

				store.EXPECT().
					CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
						BusinessLicenseNumber: app.BusinessLicenseNumber,
						ID:                    app.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
						LegalPersonIDNumber: app.LegalPersonIDNumber,
						ID:                  app.ID,
					}).
					Times(1).
					Return(int64(0), nil)

				store.EXPECT().
					ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveMerchantApplicationTxResult{
						Application: db.MerchantApplication{ID: app.ID, Status: "approved"},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "BadRequest_AddressAlreadyExistsKeepsDraft",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(t *testing.T, store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					ListMerchantLocationsInRegion(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListMerchantLocationsInRegionRow{
						{
							OwnerUserID: user.ID + 1,
							Address:     "北京市朝阳区测试路100号",
							Latitude:    pgtype.Numeric{Int: big.NewInt(399080000), Exp: -7, Valid: true},
							Longitude:   pgtype.Numeric{Int: big.NewInt(1163210000), Exp: -7, Valid: true},
						},
					}, nil)

			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "附近已有其他商户完成入驻")
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectMerchantApplicationPublicDocumentLookups(store, user.ID)
			tc.buildStubs(t, store)

			server := newTestServer(t, store)
			if tc.configureServer != nil {
				tc.configureServer(server)
			}
			recorder := httptest.NewRecorder()

			request, err := http.NewRequest(http.MethodPost, "/v1/merchant/application/submit", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 重置申请测试 ====================

func TestSubmitMerchantApplication_QueuesOnboardingReviewWhenAsyncAvailable(t *testing.T) {
	user, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockworker.NewMockTaskDistributor(ctrl)

	app := randomMerchantAppDraftWithData(user.ID)
	store.EXPECT().
		GetMerchantApplicationDraft(gomock.Any(), user.ID).
		Return(app, nil)

	submittedApp := app
	submittedApp.Status = "submitted"
	store.EXPECT().
		SubmitMerchantApplication(gomock.Any(), app.ID).
		Return(submittedApp, nil)

	store.EXPECT().
		ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
		Return([]db.ListMerchantLocationsInRegionRow{}, nil)

	store.EXPECT().
		CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
			BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
			ID:                    submittedApp.ID,
		}).
		Return(int64(0), nil)

	store.EXPECT().
		CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
			LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
			ID:                  submittedApp.ID,
		}).
		Return(int64(0), nil)

	store.EXPECT().
		CreateMerchantOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateMerchantOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.Equal(t, submittedApp.ID, arg.MerchantApplicationID.Int64)
			require.Equal(t, "queued", arg.RunStatus)
			return db.OnboardingReviewRun{ID: 1201, ApplicationType: "merchant", RunStatus: "queued", Stage: "review", CreatedAt: time.Now()}, nil
		})

	store.EXPECT().
		UpdateMerchantApplicationReviewSummary(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationReviewSummaryParams) (db.MerchantApplication, error) {
			require.Equal(t, submittedApp.ID, arg.ID)
			return db.MerchantApplication{ID: submittedApp.ID, ReviewSummary: arg.ReviewSummary}, nil
		})

	distributor.EXPECT().
		DistributeTaskOnboardingReview(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.OnboardingReviewPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(1201), payload.ReviewRunID)
			require.Equal(t, submittedApp.ID, payload.ApplicationID)
			require.Equal(t, "merchant", payload.ApplicationType)
			require.Equal(t, user.ID, payload.RequestedBy)
			return nil
		})

	server := newTestServerWithTaskDistributor(t, store, distributor)
	server.onboardingReviewService = logic.NewOnboardingReviewService(store)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/application/submit", bytes.NewReader([]byte(`{"consented":true}`)))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantApplicationDraftResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "submitted", resp.Status)
	if resp.ReviewSummary == nil {
		t.Fatal("expected queued review summary in async merchant submit response")
	}
	require.Equal(t, int64(1201), resp.ReviewSummary.RunID)
}

func TestSubmitMerchantApplication_FallsBackToSyncReviewWhenEnqueueFails(t *testing.T) {
	user, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockworker.NewMockTaskDistributor(ctrl)

	app := randomMerchantAppDraftWithData(user.ID)
	store.EXPECT().
		GetMerchantApplicationDraft(gomock.Any(), user.ID).
		Return(app, nil)

	submittedApp := app
	submittedApp.Status = "submitted"
	store.EXPECT().
		SubmitMerchantApplication(gomock.Any(), app.ID).
		Return(submittedApp, nil)

	store.EXPECT().
		ListMerchantLocationsInRegion(gomock.Any(), submittedApp.RegionID.Int64).
		Return([]db.ListMerchantLocationsInRegionRow{}, nil)

	store.EXPECT().
		CheckBusinessLicenseExists(gomock.Any(), db.CheckBusinessLicenseExistsParams{
			BusinessLicenseNumber: submittedApp.BusinessLicenseNumber,
			ID:                    submittedApp.ID,
		}).
		Return(int64(0), nil)

	store.EXPECT().
		CheckLegalPersonIDExists(gomock.Any(), db.CheckLegalPersonIDExistsParams{
			LegalPersonIDNumber: submittedApp.LegalPersonIDNumber,
			ID:                  submittedApp.ID,
		}).
		Return(int64(0), nil)

	queuedAt := time.Now()
	store.EXPECT().
		CreateMerchantOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateMerchantOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.Equal(t, submittedApp.ID, arg.MerchantApplicationID.Int64)
			require.Equal(t, "queued", arg.RunStatus)
			return db.OnboardingReviewRun{ID: 1301, ApplicationType: "merchant", RunStatus: "queued", Stage: "review", CreatedAt: queuedAt}, nil
		})

	store.EXPECT().
		UpdateMerchantApplicationReviewSummary(gomock.Any(), gomock.Any()).
		Times(2).
		DoAndReturn(func() func(context.Context, db.UpdateMerchantApplicationReviewSummaryParams) (db.MerchantApplication, error) {
			callCount := 0
			return func(_ context.Context, arg db.UpdateMerchantApplicationReviewSummaryParams) (db.MerchantApplication, error) {
				callCount++
				var summary map[string]any
				require.NoError(t, json.Unmarshal(arg.ReviewSummary, &summary))
				require.Equal(t, submittedApp.ID, arg.ID)
				switch callCount {
				case 1:
					require.Equal(t, float64(1301), summary["run_id"])
					require.Equal(t, "", summary["outcome"])
				case 2:
					require.Equal(t, float64(1301), summary["run_id"])
					require.Equal(t, "approved", summary["outcome"])
					require.Equal(t, "auto_approved", summary["reason_code"])
				default:
					t.Fatalf("unexpected merchant review summary update call %d", callCount)
				}
				return db.MerchantApplication{ID: submittedApp.ID, ReviewSummary: arg.ReviewSummary}, nil
			}
		}())

	distributor.EXPECT().
		DistributeTaskOnboardingReview(gomock.Any(), gomock.Any()).
		Return(errors.New("redis unavailable"))

	approvedApp := submittedApp
	approvedApp.Status = "approved"
	store.EXPECT().
		ApproveMerchantApplicationTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ApproveMerchantApplicationTxParams) (db.ApproveMerchantApplicationTxResult, error) {
			require.Equal(t, submittedApp.ID, arg.ApplicationID)
			return db.ApproveMerchantApplicationTxResult{
				Application: approvedApp,
				Merchant: db.Merchant{
					ID:          88,
					OwnerUserID: user.ID,
				},
			}, nil
		})

	store.EXPECT().
		MarkOnboardingReviewRunProcessing(gomock.Any(), int64(1301)).
		Return(db.OnboardingReviewRun{ID: 1301, ApplicationType: "merchant", RunStatus: "processing", Stage: "review", CreatedAt: queuedAt}, nil)

	store.EXPECT().
		CompleteOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CompleteOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.Equal(t, int64(1301), arg.ID)
			require.Equal(t, "approved", arg.Outcome.String)
			require.Equal(t, "auto_approved", arg.ReasonCode.String)
			return db.OnboardingReviewRun{
				ID:         1301,
				Stage:      "review",
				RunStatus:  "completed",
				Outcome:    pgtype.Text{String: "approved", Valid: true},
				ReasonCode: pgtype.Text{String: "auto_approved", Valid: true},
				CreatedAt:  queuedAt,
				UpdatedAt:  queuedAt,
			}, nil
		})

	server := newTestServerWithTaskDistributor(t, store, distributor)
	server.onboardingReviewService = logic.NewOnboardingReviewService(store)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/application/submit", bytes.NewReader([]byte(`{"consented":true}`)))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantApplicationDraftResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "approved", resp.Status)
	if resp.ReviewSummary == nil {
		t.Fatal("expected review summary after merchant sync fallback completion")
	}
	require.Equal(t, int64(1301), resp.ReviewSummary.RunID)
	require.Equal(t, "approved", resp.ReviewSummary.Outcome)
	require.Equal(t, "auto_approved", resp.ReviewSummary.ReasonCode)
}

func TestResetMerchantApplication(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				app.Status = "rejected"
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				resetApp := app
				resetApp.Status = "draft"
				store.EXPECT().
					ResetMerchantApplicationTx(gomock.Any(), db.ResetMerchantApplicationTxParams{
						ApplicationID: app.ID,
						UserID:        user.ID,
					}).
					Times(1).
					Return(db.ResetMerchantApplicationTxResult{
						Application: resetApp,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantApplicationDraftResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "draft", resp.Status)
			},
		},
		{
			name: "NotRejected",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				app.Status = "draft" // 不是rejected状态
				store.EXPECT().
					GetUserMerchantApplication(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
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
			expectMerchantApplicationPublicDocumentLookups(store, user.ID)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			request, err := http.NewRequest(http.MethodPost, "/v1/merchant/application/reset", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 地址匹配测试 ====================

func TestIsAddressMatch(t *testing.T) {
	testCases := []struct {
		name     string
		license  string
		business string
		expected bool
	}{
		{
			name:     "完全相同地址",
			license:  "北京市朝阳区建国路100号",
			business: "北京市朝阳区建国路100号",
			expected: true,
		},
		{
			name:     "商户地址包含楼层",
			license:  "北京市朝阳区建国路100号",
			business: "北京市朝阳区建国路100号1层",
			expected: true,
		},
		{
			name:     "商户地址包含详细楼号",
			license:  "北京市朝阳区建国路100号",
			business: "北京市朝阳区建国路100号A座2楼",
			expected: true,
		},
		{
			name:     "不同门牌号",
			license:  "北京市朝阳区建国路100号",
			business: "北京市朝阳区建国路200号",
			expected: false,
		},
		{
			name:     "不同区",
			license:  "北京市朝阳区建国路100号",
			business: "北京市海淀区建国路100号",
			expected: false,
		},
		{
			name:     "不同路",
			license:  "北京市朝阳区建国路100号",
			business: "北京市朝阳区光华路100号",
			expected: false,
		},
		{
			name:     "省级地址匹配",
			license:  "浙江省杭州市西湖区文三路100号",
			business: "浙江省杭州市西湖区文三路100号1楼",
			expected: true,
		},
		{
			name:     "不同省",
			license:  "浙江省杭州市西湖区文三路100号",
			business: "江苏省杭州市西湖区文三路100号",
			expected: false,
		},
		{
			name:     "带街道的地址",
			license:  "北京市朝阳区朝外街道建国路100号",
			business: "北京市朝阳区建国路100号",
			expected: true, // 街道可以省略
		},
		{
			name:     "空地址",
			license:  "",
			business: "北京市朝阳区建国路100号",
			expected: false,
		},
		{
			name:     "无门牌号但路名相同",
			license:  "北京市朝阳区建国路",
			business: "北京市朝阳区建国路100号",
			expected: true, // 路名相同应该匹配
		},
		{
			name:     "县级行政区",
			license:  "四川省成都市双流县白家镇长征路50号",
			business: "四川省成都市双流县白家镇长征路50号1楼",
			expected: true,
		},
		{
			name:     "直辖市地址",
			license:  "上海市浦东新区世纪大道200号",
			business: "上海市浦东新区世纪大道200号环球金融中心",
			expected: true,
		},
		{
			name:     "腾讯逆解析省略道路方位并返回辅路",
			license:  "邢台市宁晋县天宝西街与宁米路交叉口北行100米路东",
			business: "河北省邢台市宁晋县天宝街辅路",
			expected: true,
		},
		{
			name:     "明确门牌号的方位道路不与主路合并",
			license:  "河北省邢台市宁晋县天宝西街100号",
			business: "河北省邢台市宁晋县天宝街100号",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isAddressMatch(tc.license, tc.business)
			require.Equal(t, tc.expected, result, "license: %s, business: %s", tc.license, tc.business)
		})
	}
}

// TestGetOrCreateMerchantApplicationDraft_WithMediaAssetIDs — Phase 5.6
// 当草稿中已绑定证照媒体资产 ID 时，GET /v1/merchant/application 响应中应包含
// business_license_media_asset_id 和 food_permit_media_asset_id 字段
func TestGetOrCreateMerchantApplicationDraft_WithMediaAssetIDs(t *testing.T) {
	user, _ := randomUser(t)
	app := randomMerchantAppDraftWithData(user.ID)
	// randomMerchantAppDraftWithData 已设置:
	//   BusinessLicenseMediaAssetID = 2, FoodPermitMediaAssetID = 1
	//   IDCardFrontMediaAssetID = 3, IDCardBackMediaAssetID = 4

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectMerchantApplicationPublicDocumentLookups(store, user.ID)
	store.EXPECT().
		GetMerchantApplicationDraft(gomock.Any(), user.ID).
		Times(1).
		Return(app, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantApplicationDraftResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.BusinessLicenseMediaAssetID)
	require.Equal(t, int64(2), *resp.BusinessLicenseMediaAssetID)
	require.NotNil(t, resp.BusinessLicenseURL)
	require.Contains(t, *resp.BusinessLicenseURL, "business-license.jpg")
	require.NotNil(t, resp.FoodPermitMediaAssetID)
	require.Equal(t, int64(1), *resp.FoodPermitMediaAssetID)
	require.NotNil(t, resp.FoodPermitURL)
	require.Contains(t, *resp.FoodPermitURL, "food-permit.jpg")
	require.NotNil(t, resp.IDCardFrontMediaAssetID)
	require.Equal(t, int64(3), *resp.IDCardFrontMediaAssetID)
	require.NotNil(t, resp.IDCardBackMediaAssetID)
	require.Equal(t, int64(4), *resp.IDCardBackMediaAssetID)
}

func TestGetOrCreateMerchantApplicationDraft_ReturnsAsyncOCRFields(t *testing.T) {
	user, _ := randomUser(t)
	app := randomMerchantAppDraft(user.ID)
	ocrJobID := int64(9001)

	businessLicenseOCR, err := json.Marshal(BusinessLicenseOCRData{
		Status:         "processing",
		ErrorCode:      "ocr_rate_limited",
		AlertEmittedAt: "2026-03-25T16:00:00Z",
		QueuedAt:       "2026-03-25T15:59:00Z",
		StartedAt:      "2026-03-25T15:59:05Z",
		OCRJobID:       &ocrJobID,
	})
	require.NoError(t, err)
	foodPermitOCR, err := json.Marshal(FoodPermitOCRData{
		Status:    "done",
		QueuedAt:  "2026-03-25T15:58:00Z",
		StartedAt: "2026-03-25T15:58:03Z",
		OCRJobID:  &ocrJobID,
	})
	require.NoError(t, err)
	idCardFrontOCR, err := json.Marshal(MerchantIDCardOCRData{
		Status:         "failed",
		ErrorCode:      "ocr_bad_request",
		AlertEmittedAt: "2026-03-25T16:01:00Z",
		QueuedAt:       "2026-03-25T16:00:30Z",
		StartedAt:      "2026-03-25T16:00:31Z",
		OCRJobID:       &ocrJobID,
	})
	require.NoError(t, err)

	app.BusinessLicenseOcr = businessLicenseOCR
	app.FoodPermitOcr = foodPermitOCR
	app.IDCardFrontOcr = idCardFrontOCR

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectMerchantApplicationPublicDocumentLookups(store, user.ID)
	store.EXPECT().
		GetMerchantApplicationDraft(gomock.Any(), user.ID).
		Times(1).
		Return(app, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantApplicationDraftResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.BusinessLicenseOCR)
	require.Equal(t, "processing", resp.BusinessLicenseOCR.Status)
	require.Equal(t, "ocr_rate_limited", resp.BusinessLicenseOCR.ErrorCode)
	require.Equal(t, "2026-03-25T16:00:00Z", resp.BusinessLicenseOCR.AlertEmittedAt)
	require.Equal(t, "2026-03-25T15:59:00Z", resp.BusinessLicenseOCR.QueuedAt)
	require.Equal(t, "2026-03-25T15:59:05Z", resp.BusinessLicenseOCR.StartedAt)
	require.NotNil(t, resp.BusinessLicenseOCR.OCRJobID)
	require.Equal(t, ocrJobID, *resp.BusinessLicenseOCR.OCRJobID)
	require.NotNil(t, resp.FoodPermitOCR)
	require.Equal(t, "2026-03-25T15:58:00Z", resp.FoodPermitOCR.QueuedAt)
	require.Equal(t, "2026-03-25T15:58:03Z", resp.FoodPermitOCR.StartedAt)
	require.NotNil(t, resp.FoodPermitOCR.OCRJobID)
	require.Equal(t, ocrJobID, *resp.FoodPermitOCR.OCRJobID)
	require.NotNil(t, resp.IDCardFrontOCR)
	require.Equal(t, "failed", resp.IDCardFrontOCR.Status)
	require.Equal(t, "ocr_bad_request", resp.IDCardFrontOCR.ErrorCode)
	require.Equal(t, "2026-03-25T16:01:00Z", resp.IDCardFrontOCR.AlertEmittedAt)
	require.NotNil(t, resp.IDCardFrontOCR.OCRJobID)
	require.Equal(t, ocrJobID, *resp.IDCardFrontOCR.OCRJobID)
}

func TestGetOrCreateMerchantApplicationDraft_RewritesPublicImageArraysInLocalMode(t *testing.T) {
	user, _ := randomUser(t)
	app := randomMerchantAppDraft(user.ID)

	storefrontImages, err := json.Marshal([]string{"uploads/merchants/12/storefront/cover.jpg"})
	require.NoError(t, err)
	environmentImages, err := json.Marshal([]string{"uploads/merchants/12/environment/room.jpg"})
	require.NoError(t, err)
	app.StorefrontImages = storefrontImages
	app.EnvironmentImages = environmentImages

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectMerchantApplicationPublicDocumentLookups(store, user.ID)
	store.EXPECT().
		GetMerchantApplicationDraft(gomock.Any(), user.ID).
		Times(1).
		Return(app, nil)

	server := newTestServer(t, store)
	server.config.FileStorageProvider = "local"
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp merchantApplicationDraftResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, []string{"/dev/uploads/merchants/12/storefront/cover.jpg"}, resp.StorefrontImages)
	require.Equal(t, []string{"/dev/uploads/merchants/12/environment/room.jpg"}, resp.EnvironmentImages)
}

func TestGetOrCreateMerchantApplicationDraft_ReturnsInternalServerErrorOnInvalidStorefrontImages(t *testing.T) {
	user, _ := randomUser(t)
	app := randomMerchantAppDraft(user.ID)
	app.StorefrontImages = []byte(`not-json`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantApplicationDraft(gomock.Any(), user.ID).
		Times(1).
		Return(app, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "internal server error", resp.Message)
}

func TestUpdateMerchantApplicationImages_ReturnsInternalServerErrorOnInvalidStoredStorefrontImages(t *testing.T) {
	user, _ := randomUser(t)
	app := randomMerchantAppDraft(user.ID)
	app.StorefrontImages = []byte(`not-json`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantApplicationDraft(gomock.Any(), user.ID).
		Times(1).
		Return(app, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body, err := json.Marshal(updateMerchantImagesRequest{
		StorefrontImages: []string{"uploads/merchants/12/storefront/new.jpg"},
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPut, "/v1/merchant/application/images", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "internal server error", resp.Message)
}
