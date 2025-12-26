package api

import (
	"bytes"
	"encoding/json"
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
		ID:             1,
		UserID:         userID,
		RealName:       pgtype.Text{String: "张三", Valid: true},
		Phone:          pgtype.Text{String: "13812345678", Valid: true},
		IDCardFrontUrl: pgtype.Text{String: "uploads/riders/1/idcard/front.jpg", Valid: true},
		IDCardBackUrl:  pgtype.Text{String: "uploads/riders/1/idcard/back.jpg", Valid: true},
		IDCardOcr:      []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"20350101"}`),
		HealthCertUrl:  pgtype.Text{String: "uploads/riders/1/healthcert/cert.jpg", Valid: true},
		HealthCertOcr:  []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"2030年12月31日"}`),
		Status:         "draft",
		CreatedAt:      time.Now(),
	}
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
					Return(db.RiderApplication{}, pgx.ErrNoRows)

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
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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

				// 审核通过
				approvedApp := submittedApp
				approvedApp.Status = "approved"
				store.EXPECT().
					ApproveRiderApplication(gomock.Any(), gomock.Any()).
					Times(1).
					Return(approvedApp, nil)

				// 创建骑手记录
				rider := randomRider(user.ID)
				store.EXPECT().
					CreateRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(rider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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
				app.HealthCertUrl = pgtype.Text{Valid: false}
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
				store.EXPECT().
					ApproveRiderApplication(gomock.Any(), gomock.Any()).
					Times(1).
					Return(approvedApp, nil)

				rider := randomRider(user.ID)
				store.EXPECT().
					CreateRider(gomock.Any(), gomock.Any()).
					Times(1).
					Return(rider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
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
