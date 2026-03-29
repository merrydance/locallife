package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func randomRiderApplication(userID int64) db.RiderApplication {
	return db.RiderApplication{
		ID:        1,
		UserID:    userID,
		Status:    "draft",
		CreatedAt: time.Now(),
	}
}

func randomRiderApplicationWithData(userID int64) db.RiderApplication {
	return db.RiderApplication{
		ID:                      1,
		UserID:                  userID,
		RealName:                pgtype.Text{String: "张三", Valid: true},
		Phone:                   pgtype.Text{String: "13812345678", Valid: true},
		IDCardFrontMediaAssetID: pgtype.Int8{Int64: 1, Valid: true},
		IDCardBackMediaAssetID:  pgtype.Int8{Int64: 2, Valid: true},
		IDCardOcr:               []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"20350101"}`),
		HealthCertMediaAssetID:  pgtype.Int8{Int64: 3, Valid: true},
		HealthCertOcr:           []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"2030年12月31日"}`),
		Status:                  "draft",
		CreatedAt:               time.Now(),
	}
}

func TestCheckRiderApplicationApproval_PermanentIDCardStillRequiresHealthCertValidation(t *testing.T) {
	server := &Server{}
	app := randomRiderApplicationWithData(1)
	app.IDCardOcr = []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"长期"}`)
	app.HealthCertOcr = []byte(`{"name":"李四","valid_end":"2030年12月31日"}`)

	approved, rejectReason := server.checkRiderApplicationApproval(app)

	require.False(t, approved)
	require.Equal(t, "健康证姓名与身份证姓名不一致", rejectReason)
}

func TestCheckRiderApplicationApproval_IgnoresHealthCertIDNumber(t *testing.T) {
	server := &Server{}
	app := randomRiderApplicationWithData(1)
	app.HealthCertOcr = []byte(`{"name":"张三","id_number":"320101199001011234","valid_end":"2030年12月31日"}`)

	approved, rejectReason := server.checkRiderApplicationApproval(app)

	require.True(t, approved)
	require.Empty(t, rejectReason)
}

// ==================== 创建/获取草稿测试 ====================

func TestCreateOrGetRiderApplicationDraft(t *testing.T) {
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
				// 没有现有申请
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.RiderApplication{}, db.ErrRecordNotFound)

				// 创建新草稿
				app := randomRiderApplication(user.ID)
				store.EXPECT().
					CreateRiderApplication(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)

				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "draft", resp.Status)
			},
		},
		{
			name: "GetExisting",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 已有申请
				app := randomRiderApplicationWithData(user.ID)
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.NotNil(t, resp.RealName)
				require.Equal(t, "张三", *resp.RealName)
			},
		},
		{
			name: "NoAuth",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
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

			request, err := http.NewRequest(http.MethodGet, "/v1/rider/application", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 更新基础信息测试 ====================

func TestUpdateRiderApplicationBasic(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]interface{}{
				"real_name": "李四",
				"phone":     "13912345678",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomRiderApplication(user.ID)
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				updatedApp := app
				updatedApp.RealName = pgtype.Text{String: "李四", Valid: true}
				updatedApp.Phone = pgtype.Text{String: "13912345678", Valid: true}
				store.EXPECT().
					UpdateRiderApplicationBasicInfo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(updatedApp, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NotDraft",
			body: map[string]interface{}{
				"real_name": "李四",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomRiderApplication(user.ID)
				app.Status = "submitted" // 非草稿状态
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidPhone",
			body: map[string]interface{}{
				"phone": "123", // 无效手机号
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
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

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPut, "/v1/rider/application/basic", bytes.NewReader(body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestDeleteRiderApplicationHealthCert(t *testing.T) {
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
				app := randomRiderApplicationWithData(user.ID)
				updatedApp := app
				updatedApp.HealthCertMediaAssetID = pgtype.Int8{}
				updatedApp.HealthCertOcr = nil

				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					ClearRiderApplicationHealthCert(gomock.Any(), app.ID).
					Times(1).
					DoAndReturn(func(_ any, id int64) (db.RiderApplication, error) {
						require.Equal(t, app.ID, id)
						return updatedApp, nil
					})

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(3)).
					Times(1).
					Return(db.MediaAsset{ID: 3, UploadedBy: user.ID}, nil)

				store.EXPECT().
					SoftDeleteMediaAsset(gomock.Any(), int64(3)).
					Times(1).
					Return(db.MediaAsset{ID: 3, UploadedBy: user.ID}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Nil(t, resp.HealthCertAssetID)
				require.Nil(t, resp.HealthCertOCR)
			},
		},
		{
			name: "NotDraft",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomRiderApplicationWithData(user.ID)
				app.Status = "submitted"
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
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

			request, err := http.NewRequest(http.MethodDelete, "/v1/rider/application/health-cert", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestDeleteRiderApplicationDocument(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		documentType  string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:         "OKIDCardFront",
			documentType: "id_card_front",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomRiderApplicationWithData(user.ID)
				updatedApp := app
				updatedApp.IDCardFrontMediaAssetID = pgtype.Int8{}
				updatedApp.IDCardOcr = []byte(`{"valid_end":"20350101"}`)

				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					ClearRiderApplicationIDCardFront(gomock.Any(), app.ID).
					Times(1).
					Return(updatedApp, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(1)).
					Times(1).
					Return(db.MediaAsset{ID: 1, UploadedBy: user.ID}, nil)

				store.EXPECT().
					SoftDeleteMediaAsset(gomock.Any(), int64(1)).
					Times(1).
					Return(db.MediaAsset{ID: 1, UploadedBy: user.ID}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Nil(t, resp.IDCardFrontAssetID)
				require.NotNil(t, resp.IDCardBackAssetID)
				require.NotNil(t, resp.IDCardOCR)
				require.Empty(t, resp.IDCardOCR.Name)
				require.Empty(t, resp.IDCardOCR.IDNumber)
				require.Equal(t, "20350101", resp.IDCardOCR.ValidEnd)
			},
		},
		{
			name:         "OKIDCardBack",
			documentType: "id_card_back",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomRiderApplicationWithData(user.ID)
				updatedApp := app
				updatedApp.IDCardBackMediaAssetID = pgtype.Int8{}
				updatedApp.IDCardOcr = []byte(`{"name":"张三","id_number":"110101199001011234"}`)

				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					ClearRiderApplicationIDCardBack(gomock.Any(), app.ID).
					Times(1).
					Return(updatedApp, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(2)).
					Times(1).
					Return(db.MediaAsset{ID: 2, UploadedBy: user.ID}, nil)

				store.EXPECT().
					SoftDeleteMediaAsset(gomock.Any(), int64(2)).
					Times(1).
					Return(db.MediaAsset{ID: 2, UploadedBy: user.ID}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.NotNil(t, resp.IDCardFrontAssetID)
				require.Nil(t, resp.IDCardBackAssetID)
				require.NotNil(t, resp.IDCardOCR)
				require.Equal(t, "张三", resp.IDCardOCR.Name)
				require.Equal(t, "110101199001011234", resp.IDCardOCR.IDNumber)
				require.Empty(t, resp.IDCardOCR.ValidEnd)
			},
		},
		{
			name:         "InvalidDocumentType",
			documentType: "invalid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
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

			request, err := http.NewRequest(http.MethodDelete, "/v1/rider/application/documents/"+tc.documentType, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 提交申请测试（自动审核） ====================

func TestSubmitRiderApplication(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "Approved_ValidIDCard",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 完整的申请数据，身份证有效期内
				app := randomRiderApplicationWithData(user.ID)
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				// 提交
				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitRiderApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				// 审核通过 + 创建骑手（事务）
				approvedApp := submittedApp
				approvedApp.Status = "approved"
				rider := randomRider(user.ID)
				store.EXPECT().
					ApproveRiderApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveRiderApplicationTxResult{
						Application: approvedApp,
						Rider:       rider,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "approved", resp.Status)
			},
		},
		{
			name: "Rejected_ExpiredIDCard",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 身份证已过期
				app := randomRiderApplicationWithData(user.ID)
				app.IDCardOcr = []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"20200101"}`) // 过期
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				// 提交
				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitRiderApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				// 拒绝
				rejectedApp := submittedApp
				rejectedApp.Status = "rejected"
				rejectedApp.RejectReason = pgtype.Text{String: "身份证已过期，请更换有效身份证后重新申请", Valid: true}
				store.EXPECT().
					RejectRiderApplication(gomock.Any(), gomock.Any()).
					Times(1).
					Return(rejectedApp, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "rejected", resp.Status)
				require.NotNil(t, resp.RejectReason)
				require.Contains(t, *resp.RejectReason, "身份证已过期")
			},
		},
		{
			name: "Rejected_NoOCRData",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 没有OCR数据
				app := randomRiderApplicationWithData(user.ID)
				app.IDCardOcr = nil
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				// 提交
				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitRiderApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				// 拒绝
				rejectedApp := submittedApp
				rejectedApp.Status = "rejected"
				store.EXPECT().
					RejectRiderApplication(gomock.Any(), gomock.Any()).
					Times(1).
					Return(rejectedApp, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "rejected", resp.Status)
			},
		},
		{
			name: "MissingHealthCert",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 缺少健康证
				app := randomRiderApplicationWithData(user.ID)
				app.HealthCertMediaAssetID = pgtype.Int8{}
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NotDraftStatus",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomRiderApplicationWithData(user.ID)
				app.Status = "approved" // 已通过，不能再提交
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "Approved_PermanentIDCard",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 长期有效身份证
				app := randomRiderApplicationWithData(user.ID)
				app.IDCardOcr = []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"长期"}`)
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitRiderApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				approvedApp := submittedApp
				approvedApp.Status = "approved"
				rider := randomRider(user.ID)
				store.EXPECT().
					ApproveRiderApplicationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ApproveRiderApplicationTxResult{
						Application: approvedApp,
						Rider:       rider,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "approved", resp.Status)
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

			request, err := http.NewRequest(http.MethodPost, "/v1/rider/application/submit", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 重置申请测试 ====================

func TestResetRiderApplication(t *testing.T) {
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
				app := randomRiderApplicationWithData(user.ID)
				app.Status = "rejected"
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				resetApp := app
				resetApp.Status = "draft"
				store.EXPECT().
					ResetRiderApplicationToDraft(gomock.Any(), app.ID).
					Times(1).
					Return(resetApp, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
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
				app := randomRiderApplicationWithData(user.ID)
				app.Status = "draft" // 不是rejected状态
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
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

			request, err := http.NewRequest(http.MethodPost, "/v1/rider/application/reset", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestCreateOrGetRiderApplicationDraft_WithMediaAssetIDs — Phase 5.7
// 当骑手申请草稿中已绑定证照媒体资产 ID 时，GET /v1/rider/application
// 响应中应包含 id_card_front_asset_id、id_card_back_asset_id、health_cert_asset_id
func TestCreateOrGetRiderApplicationDraft_WithMediaAssetIDs(t *testing.T) {
	user, _ := randomUser(t)
	app := randomRiderApplicationWithData(user.ID)
	// randomRiderApplicationWithData 已设置:
	//   IDCardFrontMediaAssetID = 1, IDCardBackMediaAssetID = 2, HealthCertMediaAssetID = 3

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderApplicationByUserID(gomock.Any(), user.ID).
		Times(1).
		Return(app, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/rider/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp riderApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.IDCardFrontAssetID)
	require.Equal(t, int64(1), *resp.IDCardFrontAssetID)
	require.NotNil(t, resp.IDCardBackAssetID)
	require.Equal(t, int64(2), *resp.IDCardBackAssetID)
	require.NotNil(t, resp.HealthCertAssetID)
	require.Equal(t, int64(3), *resp.HealthCertAssetID)
}

func TestCreateOrGetRiderApplicationDraft_ReturnsAsyncOCRFields(t *testing.T) {
	user, _ := randomUser(t)
	ocrJobID := int64(9201)
	idCardOCR, err := json.Marshal(IDCardOCRData{
		Status:    "done",
		QueuedAt:  "2026-03-25T16:20:00Z",
		StartedAt: "2026-03-25T16:20:05Z",
		OCRJobID:  &ocrJobID,
		Name:      "王五",
	})
	require.NoError(t, err)
	healthCertOCR, err := json.Marshal(HealthCertOCRData{
		Status:         "failed",
		ErrorCode:      "ocr_provider_unavailable",
		AlertEmittedAt: "2026-03-25T16:22:00Z",
		QueuedAt:       "2026-03-25T16:21:00Z",
		StartedAt:      "2026-03-25T16:21:01Z",
		OCRJobID:       &ocrJobID,
		Name:           "王五",
	})
	require.NoError(t, err)

	app := randomRiderApplication(user.ID)
	app.IDCardOcr = idCardOCR
	app.HealthCertOcr = healthCertOCR

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderApplicationByUserID(gomock.Any(), user.ID).
		Times(1).
		Return(app, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/rider/application", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp riderApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.NotNil(t, resp.IDCardOCR)
	require.Equal(t, "2026-03-25T16:20:00Z", resp.IDCardOCR.QueuedAt)
	require.Equal(t, "2026-03-25T16:20:05Z", resp.IDCardOCR.StartedAt)
	require.NotNil(t, resp.IDCardOCR.OCRJobID)
	require.Equal(t, ocrJobID, *resp.IDCardOCR.OCRJobID)
	require.NotNil(t, resp.HealthCertOCR)
	require.Equal(t, "failed", resp.HealthCertOCR.Status)
	require.Equal(t, "ocr_provider_unavailable", resp.HealthCertOCR.ErrorCode)
	require.Equal(t, "2026-03-25T16:22:00Z", resp.HealthCertOCR.AlertEmittedAt)
	require.NotNil(t, resp.HealthCertOCR.OCRJobID)
	require.Equal(t, ocrJobID, *resp.HealthCertOCR.OCRJobID)
}
