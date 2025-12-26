package api

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 辅助函数 ====================

func randomMerchantAppDraft(userID int64) db.MerchantApplication {
	return db.MerchantApplication{
		ID:                      1,
		UserID:                  userID,
		MerchantName:            "",
		BusinessLicenseNumber:   "",
		BusinessLicenseImageUrl: "",
		LegalPersonName:         "",
		LegalPersonIDNumber:     "",
		LegalPersonIDFrontUrl:   "",
		LegalPersonIDBackUrl:    "",
		ContactPhone:            "",
		BusinessAddress:         "",
		Status:                  "draft",
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
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
		PermitNo: "JY11105000000001",
		CompanyName: "测试餐饮有限公司",
		ValidTo:  "2030年12月31日",
		OCRAt:    time.Now().Format(time.RFC3339),
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
		ID:                      1,
		UserID:                  userID,
		MerchantName:            "测试餐厅",
		BusinessLicenseNumber:   "91110000MA12345678",
		BusinessLicenseImageUrl: "uploads/merchants/1/business_license/test.jpg",
		BusinessLicenseOcr:      licenseOCR,
		LegalPersonName:         "张三",
		LegalPersonIDNumber:     "110101199001011234",
		LegalPersonIDFrontUrl:   "uploads/merchants/1/id_front/test.jpg",
		LegalPersonIDBackUrl:    "uploads/merchants/1/id_back/test.jpg",
		IDCardFrontOcr:          idCardFrontOCR,
		IDCardBackOcr:           idCardBackOCR,
		ContactPhone:            "13800138000",
		BusinessAddress:         "北京市朝阳区测试路100号1楼",
		BusinessScope:           pgtype.Text{String: "餐饮服务", Valid: true},
		FoodPermitUrl:           pgtype.Text{String: "uploads/merchants/1/food_permit/test.jpg", Valid: true},
		FoodPermitOcr:           foodPermitOCR,
		Longitude:               pgtype.Numeric{Int: big.NewInt(1163210000), Exp: -7, Valid: true},
		Latitude:                pgtype.Numeric{Int: big.NewInt(399080000), Exp: -7, Valid: true},
		RegionID:                pgtype.Int8{Int64: 1, Valid: true},
		Status:                  "draft",
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}
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
				// 没有草稿
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(db.MerchantApplication{}, pgx.ErrNoRows)

				// 创建新草稿
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
				// 有草稿
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
			name: "AlreadyApproved",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 已有通过的申请，现在允许获取并编辑
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
			name: "NoAuth",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加认证
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用任何方法
			},
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
				MerchantName:    "新店名",
				ContactPhone:    "13800138001",
				BusinessAddress: "北京市海淀区测试路200号",
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
				store.EXPECT().
					UpdateMerchantApplicationBasicInfo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(updatedApp, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NotDraft_AutoReset",
			body: updateMerchantBasicInfoRequest{
				MerchantName: "新店名",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomMerchantAppDraft(user.ID)
				app.Status = "submitted"
				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				// 应该自动重置为草稿
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

				store.EXPECT().
					UpdateMerchantApplicationBasicInfo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(resetApp, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
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

// ==================== 提交申请测试 ====================

func TestSubmitMerchantApplication(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "Approved",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
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

				// 检查地址唯一性
				store.EXPECT().
					CheckMerchantAddressExists(gomock.Any(), db.CheckMerchantAddressExistsParams{
						Address:     submittedApp.BusinessAddress,
						OwnerUserID: user.ID,
					}).
					Times(1).
					Return(false, nil)

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
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, "approved", resp.Status)
			},
		},
		{
			name: "Rejected_NoBusinessScope",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 营业执照经营范围不包含餐饮
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

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitMerchantApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				// 因为经营范围检查在地址检查之前失败，所以不会调用地址检查

				store.EXPECT().
					RejectMerchantApplication(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantApplication{
						ID:           app.ID,
						Status:       "rejected",
						RejectReason: pgtype.Text{String: "经营范围不包含餐饮相关内容", Valid: true},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantApplicationDraftResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, "rejected", resp.Status)
				require.NotNil(t, resp.RejectReason)
			},
		},
		{
			name: "MissingFoodPermit",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				app.FoodPermitUrl = pgtype.Text{Valid: false} // 没有食品许可证

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
			buildStubs: func(store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				app.RegionID = pgtype.Int8{Valid: false} // 没有区域ID

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
			name: "SubmittedStatus_Retry",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomMerchantAppDraftWithData(user.ID)
				app.Status = "submitted"

				store.EXPECT().
					GetMerchantApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				// 已经是 submitted，直接进入审核
				store.EXPECT().
					CheckMerchantAddressExists(gomock.Any(), db.CheckMerchantAddressExistsParams{
						Address:     app.BusinessAddress,
						OwnerUserID: user.ID,
					}).
					Times(1).
					Return(false, nil)

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
			name: "Rejected_AddressAlreadyExists",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
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

				// 地址已被占用
				store.EXPECT().
					CheckMerchantAddressExists(gomock.Any(), db.CheckMerchantAddressExistsParams{
						Address:     submittedApp.BusinessAddress,
						OwnerUserID: user.ID,
					}).
					Times(1).
					Return(true, nil)

				store.EXPECT().
					RejectMerchantApplication(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantApplication{
						ID:           app.ID,
						Status:       "rejected",
						RejectReason: pgtype.Text{String: "该地址已有商户入驻，同一地址不能注册两家餐厅", Valid: true},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp merchantApplicationDraftResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, "rejected", resp.Status)
				require.NotNil(t, resp.RejectReason)
				require.Contains(t, *resp.RejectReason, "地址已有商户入驻")
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
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isAddressMatch(tc.license, tc.business)
			require.Equal(t, tc.expected, result, "license: %s, business: %s", tc.license, tc.business)
		})
	}
}
