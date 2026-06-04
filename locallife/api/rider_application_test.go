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

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/worker"
	mockworker "github.com/merrydance/locallife/worker/mock"
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

func TestValidateRiderApplicationSubmissionReadiness_UsesIDCardReadiness(t *testing.T) {
	app := randomRiderApplicationWithData(1)
	app.IDCardOcr = []byte(`{"name":"张三","id_number":"110101199001011234","readiness":{"state":"partial","reason_code":"required_field_missing","missing_fields":["valid_end"]}}`)

	_, err := validateRiderApplicationSubmissionReadiness(app)

	require.EqualError(t, err, "身份证有效期未识别，请上传身份证背面照片")
}

func TestValidateRiderApplicationSubmissionReadiness_UsesHealthCertProviderFailure(t *testing.T) {
	app := randomRiderApplicationWithData(1)
	app.HealthCertOcr = []byte(`{"name":"张三","valid_end":"2030年12月31日","readiness":{"state":"provider_failed","reason_code":"provider_error"}}`)

	_, err := validateRiderApplicationSubmissionReadiness(app)

	require.EqualError(t, err, "健康证OCR处理失败，请重新上传清晰的健康证照片")
}

func TestValidateRiderApplicationSubmissionReadiness_BlocksPendingIDCardOCR(t *testing.T) {
	app := randomRiderApplicationWithData(1)
	app.IDCardOcr = []byte(`{"status":"pending","name":"张三","id_number":"110101199001011234","valid_end":"20350101"}`)

	_, err := validateRiderApplicationSubmissionReadiness(app)

	require.EqualError(t, err, "身份证OCR处理中，请稍后再提交")
}

func TestValidateRiderApplicationSubmissionReadiness_BlocksPendingHealthCertOCR(t *testing.T) {
	app := randomRiderApplicationWithData(1)
	app.HealthCertOcr = []byte(`{"status":"pending","name":"张三","valid_end":"2030年12月31日"}`)

	_, err := validateRiderApplicationSubmissionReadiness(app)

	require.EqualError(t, err, "健康证OCR处理中，请稍后再提交")
}

func TestValidateRiderApplicationSubmissionReadiness_AllowsCorrectedHealthCertValidEnd(t *testing.T) {
	app := randomRiderApplicationWithData(1)
	app.HealthCertOcr = []byte(`{"status":"done","name":"张三","valid_end":"2030年12月31日","readiness":{"state":"ready","reason_code":"ok","required_fields":["name","valid_end"]},"correction":{"corrected_by":1,"corrected_at":"2026-06-04T21:00:00+08:00","source":"rider","fields":["valid_end"],"previous":{"valid_end":""}}}`)

	_, err := validateRiderApplicationSubmissionReadiness(app)

	require.NoError(t, err)
}

// ==================== 创建/获取草稿测试 ====================

func TestCreateOrGetRiderApplicationDraft(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		requestBody   []byte
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
			name: "GetExisting_SubmittedPreserved",
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
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "submitted", resp.Status)
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
			name: "Submitted_Rejected",
			body: map[string]interface{}{
				"real_name": "李四",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomRiderApplication(user.ID)
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

func TestPatchRiderApplicationHealthCertOCRFields(t *testing.T) {
	user, _ := randomUser(t)
	futureValidEnd := time.Now().AddDate(1, 0, 0).Format("2006-01-02")

	testCases := []struct {
		name          string
		body          map[string]interface{}
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_CorrectsMissingValidEndWithAuditMetadata",
			body: map[string]interface{}{
				"cert_number": "A690",
				"valid_end":   futureValidEnd,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomRiderApplicationWithData(user.ID)
				app.HealthCertOcr = []byte(`{"status":"done","name":"张三","readiness":{"state":"partial","reason_code":"required_field_missing","required_fields":["name","valid_end"],"missing_fields":["valid_end"]},"ocr_job_id":173}`)
				updatedApp := app

				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				store.EXPECT().
					UpdateRiderApplicationHealthCert(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateRiderApplicationHealthCertParams) (db.RiderApplication, error) {
						require.Equal(t, app.ID, arg.ID)
						require.False(t, arg.HealthCertMediaAssetID.Valid)

						var payload map[string]any
						require.NoError(t, json.Unmarshal(arg.HealthCertOcr, &payload))
						require.Equal(t, "done", payload["status"])
						require.Equal(t, "张三", payload["name"])
						require.Equal(t, "A690", payload["cert_number"])
						require.Equal(t, futureValidEnd, payload["valid_end"])

						readiness, ok := payload["readiness"].(map[string]any)
						require.True(t, ok)
						require.Equal(t, "ready", readiness["state"])
						require.Equal(t, "ok", readiness["reason_code"])

						correction, ok := payload["correction"].(map[string]any)
						require.True(t, ok)
						require.Equal(t, float64(user.ID), correction["corrected_by"])
						require.NotEmpty(t, correction["corrected_at"])
						require.Equal(t, "rider", correction["source"])

						fields, ok := correction["fields"].([]any)
						require.True(t, ok)
						require.ElementsMatch(t, []any{"cert_number", "valid_end"}, fields)

						previous, ok := correction["previous"].(map[string]any)
						require.True(t, ok)
						require.Equal(t, "", previous["cert_number"])
						require.Equal(t, "", previous["valid_end"])

						updatedApp.HealthCertOcr = arg.HealthCertOcr
						return updatedApp, nil
					})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.NotNil(t, resp.HealthCertOCR)
				require.Equal(t, "A690", resp.HealthCertOCR.CertNumber)
				require.Equal(t, futureValidEnd, resp.HealthCertOCR.ValidEnd)
				require.Equal(t, "ready", resp.HealthCertOCR.Readiness.State)
			},
		},
		{
			name: "NoChangeReturnsCurrentApplicationWithoutOverwritingCorrection",
			body: map[string]interface{}{
				"cert_number": "A690",
				"valid_end":   futureValidEnd,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomRiderApplicationWithData(user.ID)
				app.HealthCertOcr = []byte(fmt.Sprintf(`{"status":"done","name":"张三","cert_number":"A690","valid_end":%q,"readiness":{"state":"ready","reason_code":"ok","required_fields":["name","valid_end"]},"correction":{"corrected_by":1,"corrected_at":"2026-06-04T21:00:00+08:00","source":"rider","fields":["cert_number","valid_end"],"previous":{"cert_number":"","valid_end":""}}}`, futureValidEnd))

				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					UpdateRiderApplicationHealthCert(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.NotNil(t, resp.HealthCertOCR)
				require.NotNil(t, resp.HealthCertOCR.Correction)
				require.Equal(t, []string{"cert_number", "valid_end"}, resp.HealthCertOCR.Correction.Fields)
				require.Equal(t, "", resp.HealthCertOCR.Correction.Previous["cert_number"])
				require.Equal(t, "", resp.HealthCertOCR.Correction.Previous["valid_end"])
			},
		},
		{
			name: "InvalidDateRejected",
			body: map[string]interface{}{
				"valid_end": "76年4月71日",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetRiderApplicationByUserID(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "健康证有效期格式无法识别")
			},
		},
		{
			name: "SubmittedRejected",
			body: map[string]interface{}{
				"valid_end": futureValidEnd,
			},
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

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPatch, "/v1/rider/application/documents/health_cert/ocr-fields", bytes.NewReader(body))
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
		requestBody   []byte
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
			name: "Submitted_Rejected",
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
		requestBody   []byte
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
			name: "Rejected_ExpiredIDCardReturnsDraft",
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

				submittedApp := app
				submittedApp.Status = "submitted"
				store.EXPECT().
					SubmitRiderApplication(gomock.Any(), app.ID).
					Times(1).
					Return(submittedApp, nil)

				returnedApp := submittedApp
				returnedApp.Status = "draft"
				returnedApp.RejectReason = pgtype.Text{String: "身份证已过期，请更换有效身份证后重新申请", Valid: true}
				store.EXPECT().
					ReturnRiderApplicationToDraft(gomock.Any(), gomock.Any()).
					Times(1).
					Return(returnedApp, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp riderApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, "draft", resp.Status)
				require.NotNil(t, resp.RejectReason)
				require.Contains(t, *resp.RejectReason, "身份证已过期")
			},
		},
		{
			name: "BadRequest_NoOCRDataKeepsDraft",
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
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "身份证信息未识别")
			},
		},
		{
			name: "BadRequest_MissingIDNumberKeepsDraft",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomRiderApplicationWithData(user.ID)
				app.IDCardOcr = []byte(`{"name":"张三","valid_end":"20350101"}`)
				store.EXPECT().
					GetRiderApplicationByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "身份证号未识别")
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
				require.Contains(t, recorder.Body.String(), "请先补充以下资料后再提交：健康证照片")
			},
		},
		{
			name:        "BadRequest_AgreementConsentMissingUserAgreementVersion",
			requestBody: []byte(`{"privacy_policy_version":"v1.0.0","consented_at":"2026-04-11T10:00:00Z"}`),
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetRiderApplicationByUserID(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "请先阅读并同意用户协议后再提交申请")
				require.Contains(t, recorder.Body.String(), "40101")
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
		{
			name: "Approved_IDCardDateRangeWithDots",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomRiderApplicationWithData(user.ID)
				app.IDCardOcr = []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"2020.01.01-2035.01.01"}`)
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

			request, err := http.NewRequest(http.MethodPost, "/v1/rider/application/submit", bytes.NewReader(tc.requestBody))
			require.NoError(t, err)
			if len(tc.requestBody) > 0 {
				request.Header.Set("Content-Type", "application/json")
			}

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestSubmitRiderApplication_QueuesOnboardingReviewWhenAsyncAvailable(t *testing.T) {
	user, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockworker.NewMockTaskDistributor(ctrl)

	application := randomRiderApplicationWithData(user.ID)
	store.EXPECT().
		GetRiderApplicationByUserID(gomock.Any(), user.ID).
		Return(application, nil)

	submitted := application
	submitted.Status = db.RiderApplicationStatusSubmitted
	store.EXPECT().
		SubmitRiderApplication(gomock.Any(), application.ID).
		Return(submitted, nil)

	store.EXPECT().
		CreateRiderOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateRiderOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.Equal(t, submitted.ID, arg.RiderApplicationID.Int64)
			require.Equal(t, "queued", arg.RunStatus)
			return db.OnboardingReviewRun{ID: 801, ApplicationType: "rider", RunStatus: "queued", Stage: "review", CreatedAt: time.Now()}, nil
		})

	store.EXPECT().
		UpdateRiderApplicationReviewSummary(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpdateRiderApplicationReviewSummaryParams) (db.RiderApplication, error) {
			require.Equal(t, submitted.ID, arg.ID)
			return db.RiderApplication{ID: submitted.ID, ReviewSummary: arg.ReviewSummary}, nil
		})

	distributor.EXPECT().
		DistributeTaskOnboardingReview(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.OnboardingReviewPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(801), payload.ReviewRunID)
			require.Equal(t, submitted.ID, payload.ApplicationID)
			require.Equal(t, "rider", payload.ApplicationType)
			require.Equal(t, user.ID, payload.RequestedBy)
			return nil
		})

	server := newTestServerWithTaskDistributor(t, store, distributor)
	server.onboardingReviewService = logic.NewOnboardingReviewService(store)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/rider/application/submit", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp riderApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, db.RiderApplicationStatusSubmitted, resp.Status)
	if resp.ReviewSummary != nil {
		require.Equal(t, int64(801), resp.ReviewSummary.RunID)
	}
}

func TestSubmitRiderApplication_FallsBackToSyncReviewWhenEnqueueFails(t *testing.T) {
	user, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockworker.NewMockTaskDistributor(ctrl)

	application := randomRiderApplicationWithData(user.ID)
	store.EXPECT().
		GetRiderApplicationByUserID(gomock.Any(), user.ID).
		Return(application, nil)

	submitted := application
	submitted.Status = db.RiderApplicationStatusSubmitted
	store.EXPECT().
		SubmitRiderApplication(gomock.Any(), application.ID).
		Return(submitted, nil)

	queuedAt := time.Now()
	store.EXPECT().
		CreateRiderOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateRiderOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.Equal(t, submitted.ID, arg.RiderApplicationID.Int64)
			require.Equal(t, "queued", arg.RunStatus)
			return db.OnboardingReviewRun{ID: 901, ApplicationType: "rider", RunStatus: "queued", Stage: "review", CreatedAt: queuedAt}, nil
		})

	store.EXPECT().
		UpdateRiderApplicationReviewSummary(gomock.Any(), gomock.Any()).
		Times(2).
		DoAndReturn(func() func(context.Context, db.UpdateRiderApplicationReviewSummaryParams) (db.RiderApplication, error) {
			callCount := 0
			return func(_ context.Context, arg db.UpdateRiderApplicationReviewSummaryParams) (db.RiderApplication, error) {
				callCount++
				var summary map[string]any
				require.NoError(t, json.Unmarshal(arg.ReviewSummary, &summary))
				require.Equal(t, submitted.ID, arg.ID)
				switch callCount {
				case 1:
					require.Equal(t, float64(901), summary["run_id"])
					require.Equal(t, "", summary["outcome"])
				case 2:
					require.Equal(t, float64(901), summary["run_id"])
					require.Equal(t, "approved", summary["outcome"])
					require.Equal(t, "auto_approved", summary["reason_code"])
				default:
					t.Fatalf("unexpected review summary update call %d", callCount)
				}
				return db.RiderApplication{ID: submitted.ID, ReviewSummary: arg.ReviewSummary}, nil
			}
		}())

	distributor.EXPECT().
		DistributeTaskOnboardingReview(gomock.Any(), gomock.Any()).
		Return(errors.New("redis unavailable"))

	approved := submitted
	approved.Status = db.RiderApplicationStatusApproved
	rider := randomRider(user.ID)
	store.EXPECT().
		ApproveRiderApplicationTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ApproveRiderApplicationTxParams) (db.ApproveRiderApplicationTxResult, error) {
			require.Equal(t, submitted.ID, arg.ApplicationID)
			return db.ApproveRiderApplicationTxResult{Application: approved, Rider: rider}, nil
		})

	store.EXPECT().
		MarkOnboardingReviewRunProcessing(gomock.Any(), int64(901)).
		Return(db.OnboardingReviewRun{ID: 901, ApplicationType: "rider", RunStatus: "processing", Stage: "review", CreatedAt: queuedAt}, nil)

	store.EXPECT().
		CompleteOnboardingReviewRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CompleteOnboardingReviewRunParams) (db.OnboardingReviewRun, error) {
			require.Equal(t, int64(901), arg.ID)
			require.Equal(t, "approved", arg.Outcome.String)
			require.Equal(t, "auto_approved", arg.ReasonCode.String)
			return db.OnboardingReviewRun{
				ID:         901,
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
	request, err := http.NewRequest(http.MethodPost, "/v1/rider/application/submit", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp riderApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, db.RiderApplicationStatusApproved, resp.Status)
	if resp.ReviewSummary == nil {
		t.Fatal("expected review summary after sync fallback completion")
	}
	require.Equal(t, int64(901), resp.ReviewSummary.RunID)
	require.Equal(t, "approved", resp.ReviewSummary.Outcome)
	require.Equal(t, "auto_approved", resp.ReviewSummary.ReasonCode)
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
				app.Status = "submitted"
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
			name: "NotSubmitted",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomRiderApplicationWithData(user.ID)
				app.Status = "draft" // 不是submitted状态
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
