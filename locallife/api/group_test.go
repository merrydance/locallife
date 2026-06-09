package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func randomGroupApplication(userID int64) db.MerchantGroupApplication {
	return db.MerchantGroupApplication{
		ID:              util.RandomInt(1, 1000),
		ApplicantUserID: userID,
		GroupName:       "测试集团",
		ContactPhone:    "13800138000",
		Status:          "draft",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func randomGroupApplicationWithData(userID int64) db.MerchantGroupApplication {
	app := randomGroupApplication(userID)
	app.LicenseMediaAssetID = pgtype.Int8{Int64: 21, Valid: true}
	app.LicenseNumber = pgtype.Text{String: "91310000123456789A", Valid: true}
	app.ApplicationData = []byte(`{"business_license_ocr":{"status":"done","credit_code":"91310000123456789A"},"legal_person_name":"张三","legal_person_id_number":"110101199001011234","id_card_front_asset_id":22,"id_card_front_ocr":{"status":"done","name":"张三","id_number":"110101199001011234"},"id_card_back_asset_id":23,"id_card_back_ocr":{"status":"done","valid_date":"2035-01-01"}}`)
	return app
}

func groupTestMediaAsset(id, uploadedBy int64, category media.Category, uploadStatus string) db.MediaAsset {
	visibility := string(media.VisibilityPublic)
	if category == media.CategoryGroupLicense {
		visibility = string(media.VisibilityPrivate)
	}
	return db.MediaAsset{
		ID:            id,
		UploadedBy:    uploadedBy,
		MediaCategory: string(category),
		UploadStatus:  uploadStatus,
		Visibility:    visibility,
	}
}

func TestNewGroupApplicationResponseRejectsInvalidApplicationData(t *testing.T) {
	app := randomGroupApplication(1)
	app.ApplicationData = []byte(`not-json`)

	_, err := newGroupApplicationResponse(app)

	require.Error(t, err)
	require.ErrorContains(t, err, "decode group application")
}

func randomGroupJoinRequest(groupID, merchantID, userID int64) db.MerchantGroupJoinRequest {
	return db.MerchantGroupJoinRequest{
		ID:              util.RandomInt(1, 1000),
		GroupID:         groupID,
		MerchantID:      merchantID,
		ApplicantUserID: userID,
		Status:          "pending",
		CreatedAt:       time.Now(),
	}
}

// ==================== 集团申请草稿 ====================

func TestCreateGroupApplicationDraftAPI(t *testing.T) {
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
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(db.MerchantGroupApplication{}, db.ErrRecordNotFound)

				newApp := randomGroupApplication(user.ID)
				store.EXPECT().
					CreateGroupApplicationDraft(gomock.Any(), user.ID).
					Times(1).
					Return(newApp, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "ExistingDraft",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				existing := randomGroupApplication(user.ID)
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(existing, nil)
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
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			request, err := http.NewRequest(http.MethodPost, "/v1/groups/applications", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateGroupApplicationBasicAPI(t *testing.T) {
	user, _ := randomUser(t)
	licenseAssetID := int64(9001)

	body := updateGroupApplicationBasicRequest{
		GroupName:           strPtr("测试集团"),
		ContactPhone:        strPtr("13800138000"),
		LicenseImageAssetID: &licenseAssetID,
		Address:             strPtr("北京市朝阳区测试路"),
		RegionID:            int64Ptr(1),
	}
	noLicenseAssetBody := body
	noLicenseAssetBody.LicenseImageAssetID = nil

	testCases := []struct {
		name          string
		body          *updateGroupApplicationBasicRequest
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
				app := randomGroupApplication(user.ID)
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), licenseAssetID).
					Times(1).
					Return(groupTestMediaAsset(licenseAssetID, user.ID, media.CategoryBusinessLicense, "confirmed"), nil)

				updated := app
				updated.GroupName = "测试集团"
				store.EXPECT().
					UpdateGroupApplicationBasic(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateGroupApplicationBasicParams{})).
					Times(1).
					DoAndReturn(func(_ any, arg db.UpdateGroupApplicationBasicParams) (db.MerchantGroupApplication, error) {
						require.True(t, arg.LicenseMediaAssetID.Valid)
						require.Equal(t, licenseAssetID, arg.LicenseMediaAssetID.Int64)
						return updated, nil
					})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "OKWithoutLicenseAssetSkipsMediaValidation",
			body: &noLicenseAssetBody,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomGroupApplication(user.ID)
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				updated := app
				updated.GroupName = "测试集团"
				store.EXPECT().
					UpdateGroupApplicationBasic(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateGroupApplicationBasicParams{})).
					Times(1).
					DoAndReturn(func(_ any, arg db.UpdateGroupApplicationBasicParams) (db.MerchantGroupApplication, error) {
						require.False(t, arg.LicenseMediaAssetID.Valid)
						return updated, nil
					})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(db.MerchantGroupApplication{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "RejectsForeignLicenseAssetBeforeMutation",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomGroupApplication(user.ID)
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), licenseAssetID).
					Times(1).
					Return(groupTestMediaAsset(licenseAssetID, user.ID+99, media.CategoryGroupLicense, "confirmed"), nil)
				store.EXPECT().
					UpdateGroupApplicationBasic(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "RejectsMissingLicenseAssetBeforeMutation",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomGroupApplication(user.ID)
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), licenseAssetID).
					Times(1).
					Return(db.MediaAsset{}, db.ErrRecordNotFound)
				store.EXPECT().
					UpdateGroupApplicationBasic(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "RejectedApplicationInvalidLicenseAssetDoesNotResetOrMutate",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomGroupApplication(user.ID)
				app.Status = "rejected"
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), licenseAssetID).
					Times(1).
					Return(groupTestMediaAsset(licenseAssetID, user.ID+99, media.CategoryGroupLicense, "confirmed"), nil)
				store.EXPECT().
					ResetGroupApplicationToDraft(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateGroupApplicationBasic(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "RejectsWrongCategoryLicenseAssetBeforeMutation",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomGroupApplication(user.ID)
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), licenseAssetID).
					Times(1).
					Return(groupTestMediaAsset(licenseAssetID, user.ID, media.CategoryIDCardFront, "confirmed"), nil)
				store.EXPECT().
					UpdateGroupApplicationBasic(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "RejectsUnconfirmedLicenseAssetBeforeMutation",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomGroupApplication(user.ID)
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), licenseAssetID).
					Times(1).
					Return(groupTestMediaAsset(licenseAssetID, user.ID, media.CategoryGroupLicense, "uploaded"), nil)
				store.EXPECT().
					UpdateGroupApplicationBasic(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "MediaLookupFailureIsInternalAndDoesNotMutate",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomGroupApplication(user.ID)
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), licenseAssetID).
					Times(1).
					Return(db.MediaAsset{}, errors.New("database connection details"))
				store.EXPECT().
					UpdateGroupApplicationBasic(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, "internal server error", resp.Message)
				require.NotContains(t, recorder.Body.String(), "database connection details")
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

			requestBody := body
			if tc.body != nil {
				requestBody = *tc.body
			}
			data, err := json.Marshal(requestBody)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPut, "/v1/groups/applications/basic", bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestSubmitGroupApplicationAPI(t *testing.T) {
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
				app := randomGroupApplication(user.ID)
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)

				updated := app
				updated.Status = "submitted"
				store.EXPECT().
					SubmitGroupApplication(gomock.Any(), app.ID).
					Times(1).
					Return(updated, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "MissingFields",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomGroupApplication(user.ID)
				app.GroupName = ""
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NotDraft",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomGroupApplication(user.ID)
				app.Status = "submitted"
				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
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

			request, err := http.NewRequest(http.MethodPost, "/v1/groups/applications/submit", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestReviewGroupApplicationAPI(t *testing.T) {
	user, _ := randomUser(t)
	appID := int64(1)

	approveBody := reviewGroupApplicationRequest{Status: "approved"}
	rejectReason := "资料不完整"
	rejectBody := reviewGroupApplicationRequest{Status: "rejected", RejectReason: &rejectReason}

	testCases := []struct {
		name          string
		body          reviewGroupApplicationRequest
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "ApproveOK",
			body: approveBody,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{UserID: user.ID, Role: "admin", Status: "active"}}, nil)

				app := randomGroupApplication(user.ID)
				app.ID = appID
				app.Status = "submitted"
				store.EXPECT().
					GetGroupApplication(gomock.Any(), app.ID).
					Times(1).
					Return(app, nil)

				group := db.MerchantGroup{ID: util.RandomInt(1, 1000), Name: "测试集团", OwnerUserID: user.ID, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
				store.EXPECT().
					ApproveGroupApplicationTx(gomock.Any(), db.ApproveGroupApplicationTxParams{ApplicationID: app.ID, ReviewerUserID: user.ID}).
					Times(1).
					Return(db.ApproveGroupApplicationTxResult{Application: app, Group: group}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "RejectOK",
			body: rejectBody,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{UserID: user.ID, Role: "admin", Status: "active"}}, nil)

				app := randomGroupApplication(user.ID)
				app.ID = appID
				app.Status = "submitted"
				store.EXPECT().
					GetGroupApplication(gomock.Any(), app.ID).
					Times(1).
					Return(app, nil)

				updated := app
				updated.Status = "rejected"
				store.EXPECT().
					ReviewSubmittedGroupApplication(gomock.Any(), gomock.Any()).
					Times(1).
					Return(updated, nil)

				store.EXPECT().
					CreateGroupAuditLog(gomock.Any(), gomock.Any()).
					Times(1)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "RejectMissingReason",
			body: reviewGroupApplicationRequest{Status: "rejected"},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{UserID: user.ID, Role: "admin", Status: "active"}}, nil)

				app := randomGroupApplication(user.ID)
				app.ID = appID
				app.Status = "submitted"
				store.EXPECT().
					GetGroupApplication(gomock.Any(), app.ID).
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

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/groups/applications/" + strconv.FormatInt(appID, 10) + "/review"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestReviewGroupApplicationAPIConflict(t *testing.T) {
	user, _ := randomUser(t)
	appID := int64(1)
	app := randomGroupApplication(user.ID)
	app.ID = appID
	app.Status = "submitted"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), user.ID).
		Return([]db.UserRole{{UserID: user.ID, Role: "admin", Status: "active"}}, nil)
	store.EXPECT().
		GetGroupApplication(gomock.Any(), app.ID).
		Times(1).
		Return(app, nil)
	store.EXPECT().
		ApproveGroupApplicationTx(gomock.Any(), db.ApproveGroupApplicationTxParams{
			ApplicationID:  app.ID,
			ReviewerUserID: user.ID,
		}).
		Times(1).
		Return(db.ApproveGroupApplicationTxResult{}, db.ErrGroupApplicationReviewConflict)

	server := newTestServer(t, store)
	auditWriter := &auditSpyWriter{}
	server.auditWriter = auditWriter

	data, err := json.Marshal(reviewGroupApplicationRequest{Status: "approved"})
	require.NoError(t, err)
	url := "/v1/groups/applications/" + strconv.FormatInt(appID, 10) + "/review"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	requireAPIErrorCode(t, recorder, ErrGroupApplicationReviewConflict)
	require.NotEmpty(t, auditWriter.Entries())
}

func TestDeleteGroupApplicationDocumentAPI(t *testing.T) {
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
				app := randomGroupApplicationWithData(user.ID)
				updated := app
				updated.LicenseMediaAssetID = pgtype.Int8{}
				updated.LicenseNumber = pgtype.Text{}
				updated.ApplicationData = []byte(`{"legal_person_name":"张三","legal_person_id_number":"110101199001011234","id_card_front_asset_id":22,"id_card_front_ocr":{"status":"done","name":"张三"},"id_card_back_asset_id":23,"id_card_back_ocr":{"status":"done","valid_date":"2035-01-01"}}`)

				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					ClearGroupApplicationBusinessLicense(gomock.Any(), app.ID).
					Times(1).
					Return(updated, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(21)).
					Times(1).
					Return(db.MediaAsset{ID: 21, UploadedBy: user.ID}, nil)
				store.EXPECT().
					SoftDeleteMediaAsset(gomock.Any(), int64(21)).
					Times(1).
					Return(db.MediaAsset{ID: 21, UploadedBy: user.ID}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp groupApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Nil(t, resp.LicenseImageAssetID)
				require.Nil(t, resp.LicenseNumber)
				require.Nil(t, resp.BusinessLicenseOCR)
			},
		},
		{
			name:         "OKIDCardFront",
			documentType: "id_card_front",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomGroupApplicationWithData(user.ID)
				updated := app
				updated.ApplicationData = []byte(`{"business_license_ocr":{"status":"done","credit_code":"91310000123456789A"},"id_card_back_asset_id":23,"id_card_back_ocr":{"status":"done","valid_date":"2035-01-01"}}`)

				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					ClearGroupApplicationIDCardFront(gomock.Any(), app.ID).
					Times(1).
					Return(updated, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(22)).
					Times(1).
					Return(db.MediaAsset{ID: 22, UploadedBy: user.ID}, nil)
				store.EXPECT().
					SoftDeleteMediaAsset(gomock.Any(), int64(22)).
					Times(1).
					Return(db.MediaAsset{ID: 22, UploadedBy: user.ID}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp groupApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Nil(t, resp.IDCardFrontAssetID)
				require.Empty(t, resp.LegalPersonName)
				require.Empty(t, resp.LegalPersonIDNumber)
				require.Nil(t, resp.IDCardFrontOCR)
			},
		},
		{
			name:         "OKIDCardBack",
			documentType: "id_card_back",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				app := randomGroupApplicationWithData(user.ID)
				updated := app
				updated.ApplicationData = []byte(`{"business_license_ocr":{"status":"done","credit_code":"91310000123456789A"},"legal_person_name":"张三","legal_person_id_number":"110101199001011234","id_card_front_asset_id":22,"id_card_front_ocr":{"status":"done","name":"张三"}}`)

				store.EXPECT().
					GetLatestGroupApplicationByApplicant(gomock.Any(), user.ID).
					Times(1).
					Return(app, nil)
				store.EXPECT().
					ClearGroupApplicationIDCardBack(gomock.Any(), app.ID).
					Times(1).
					Return(updated, nil)
				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(23)).
					Times(1).
					Return(db.MediaAsset{ID: 23, UploadedBy: user.ID}, nil)
				store.EXPECT().
					SoftDeleteMediaAsset(gomock.Any(), int64(23)).
					Times(1).
					Return(db.MediaAsset{ID: 23, UploadedBy: user.ID}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp groupApplicationResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Nil(t, resp.IDCardBackAssetID)
				require.Nil(t, resp.IDCardBackOCR)
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

			request, err := http.NewRequest(http.MethodDelete, "/v1/groups/applications/documents/"+tc.documentType, nil)
			require.NoError(t, err)
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 加入集团流程 ====================

func TestCreateGroupJoinRequestAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "approved"
	groupID := util.RandomInt(1, 1000)

	body := createGroupJoinRequestRequest{Reason: strPtr("希望统一品牌管理")}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	store.EXPECT().
		GetMerchantGroupAffiliation(gomock.Any(), merchant.ID).
		Times(1).
		Return(db.GetMerchantGroupAffiliationRow{GroupID: pgtype.Int8{Valid: false}, BrandID: pgtype.Int8{Valid: false}}, nil)

	store.EXPECT().
		GetMerchantGroup(gomock.Any(), groupID).
		Times(1).
		Return(db.MerchantGroup{ID: groupID, Name: "测试集团", Status: "active", OwnerUserID: user.ID}, nil)

	joinReq := randomGroupJoinRequest(groupID, merchant.ID, user.ID)
	store.EXPECT().
		CreateGroupJoinRequestTx(gomock.Any(), db.CreateGroupJoinRequestTxParams{
			GroupID:         groupID,
			MerchantID:      merchant.ID,
			ApplicantUserID: user.ID,
			Reason:          pgtype.Text{String: *body.Reason, Valid: true},
		}).
		Times(1).
		Return(db.CreateGroupJoinRequestTxResult{Request: joinReq}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	data, err := json.Marshal(body)
	require.NoError(t, err)

	url := "/v1/groups/" + strconv.FormatInt(groupID, 10) + "/join-requests"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	require.NoError(t, err)

	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
}

func TestCreateGroupJoinRequestAPIConflictWhenTxFindsJoinedMerchant(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "approved"
	groupID := util.RandomInt(1, 1000)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetMerchantGroupAffiliation(gomock.Any(), merchant.ID).
		Times(1).
		Return(db.GetMerchantGroupAffiliationRow{GroupID: pgtype.Int8{Valid: false}, BrandID: pgtype.Int8{Valid: false}}, nil)
	store.EXPECT().
		GetMerchantGroup(gomock.Any(), groupID).
		Times(1).
		Return(db.MerchantGroup{ID: groupID, Name: "测试集团", Status: "active", OwnerUserID: user.ID}, nil)
	store.EXPECT().
		CreateGroupJoinRequestTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateGroupJoinRequestTxResult{}, db.ErrMerchantAlreadyJoinedGroup)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	url := "/v1/groups/" + strconv.FormatInt(groupID, 10) + "/join-requests"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	requireAPIErrorCode(t, recorder, ErrMerchantAlreadyJoinedGroup)
}

func TestCreateGroupJoinRequestAPIConflictWhenPrecheckFindsJoinedMerchant(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "approved"
	groupID := util.RandomInt(1, 1000)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetMerchantGroupAffiliation(gomock.Any(), merchant.ID).
		Times(1).
		Return(db.GetMerchantGroupAffiliationRow{GroupID: pgtype.Int8{Int64: groupID + 1, Valid: true}, BrandID: pgtype.Int8{Valid: false}}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	url := "/v1/groups/" + strconv.FormatInt(groupID, 10) + "/join-requests"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	requireAPIErrorCode(t, recorder, ErrMerchantAlreadyJoinedGroup)
}

func TestApproveGroupJoinRequestAPI(t *testing.T) {
	user, _ := randomUser(t)
	groupID := util.RandomInt(1, 1000)
	requestID := util.RandomInt(1, 1000)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	store.EXPECT().
		GetGroupMemberRole(gomock.Any(), db.GetGroupMemberRoleParams{GroupID: groupID, UserID: user.ID}).
		Times(1).
		Return("owner", nil)

	resultReq := randomGroupJoinRequest(groupID, util.RandomInt(1, 1000), user.ID)
	resultReq.Status = "approved"
	store.EXPECT().
		ApproveGroupJoinRequestTx(gomock.Any(), db.ApproveGroupJoinRequestTxParams{
			RequestID:      requestID,
			GroupID:        groupID,
			ReviewerUserID: user.ID,
			BrandID:        pgtype.Int8{Valid: false},
		}).
		Times(1).
		Return(db.ApproveGroupJoinRequestTxResult{Request: resultReq}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	url := "/v1/groups/" + strconv.FormatInt(groupID, 10) + "/join-requests/" + strconv.FormatInt(requestID, 10) + "/approve"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")

	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestApproveGroupJoinRequestAPIConflict(t *testing.T) {
	user, _ := randomUser(t)
	groupID := util.RandomInt(1, 1000)
	requestID := util.RandomInt(1, 1000)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetGroupMemberRole(gomock.Any(), db.GetGroupMemberRoleParams{GroupID: groupID, UserID: user.ID}).
		Times(1).
		Return("owner", nil)
	store.EXPECT().
		ApproveGroupJoinRequestTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.ApproveGroupJoinRequestTxResult{}, db.ErrMerchantAlreadyJoinedGroup)

	server := newTestServer(t, store)
	auditWriter := &auditSpyWriter{}
	server.auditWriter = auditWriter

	url := "/v1/groups/" + strconv.FormatInt(groupID, 10) + "/join-requests/" + strconv.FormatInt(requestID, 10) + "/approve"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	requireAPIErrorCode(t, recorder, ErrMerchantAlreadyJoinedGroup)
	require.NotEmpty(t, auditWriter.Entries())
}

func TestCancelGroupJoinRequestAPI(t *testing.T) {
	user, _ := randomUser(t)
	groupID := util.RandomInt(1, 1000)
	requestID := util.RandomInt(1, 1000)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	merchantID := util.RandomInt(1, 1000)
	joinReq := randomGroupJoinRequest(groupID, merchantID, user.ID)
	updated := joinReq
	updated.Status = "cancelled"
	store.EXPECT().
		CancelGroupJoinRequestTx(gomock.Any(), db.CancelGroupJoinRequestTxParams{
			RequestID:       requestID,
			GroupID:         groupID,
			ApplicantUserID: user.ID,
		}).
		Times(1).
		Return(db.CancelGroupJoinRequestTxResult{Request: updated}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	url := "/v1/groups/" + strconv.FormatInt(groupID, 10) + "/join-requests/" + strconv.FormatInt(requestID, 10) + "/cancel"
	request, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestRejectGroupJoinRequestAPIUsesAtomicTx(t *testing.T) {
	user, _ := randomUser(t)
	groupID := util.RandomInt(1, 1000)
	requestID := util.RandomInt(1, 1000)
	reason := "资料不完整"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetGroupMemberRole(gomock.Any(), db.GetGroupMemberRoleParams{GroupID: groupID, UserID: user.ID}).
		Times(1).
		Return("owner", nil)

	updated := randomGroupJoinRequest(groupID, util.RandomInt(1, 1000), user.ID)
	updated.Status = "rejected"
	store.EXPECT().
		RejectGroupJoinRequestTx(gomock.Any(), db.RejectGroupJoinRequestTxParams{
			RequestID:      requestID,
			GroupID:        groupID,
			ReviewerUserID: user.ID,
			Reason:         pgtype.Text{String: reason, Valid: true},
		}).
		Times(1).
		Return(db.RejectGroupJoinRequestTxResult{Request: updated}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body, err := json.Marshal(rejectGroupJoinRequestRequest{Reason: &reason})
	require.NoError(t, err)
	url := "/v1/groups/" + strconv.FormatInt(groupID, 10) + "/join-requests/" + strconv.FormatInt(requestID, 10) + "/reject"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")

	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestCancelGroupJoinRequestAPIConflict(t *testing.T) {
	user, _ := randomUser(t)
	groupID := util.RandomInt(1, 1000)
	requestID := util.RandomInt(1, 1000)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		CancelGroupJoinRequestTx(gomock.Any(), db.CancelGroupJoinRequestTxParams{
			RequestID:       requestID,
			GroupID:         groupID,
			ApplicantUserID: user.ID,
		}).
		Times(1).
		Return(db.CancelGroupJoinRequestTxResult{}, db.ErrGroupJoinRequestReviewConflict)

	server := newTestServer(t, store)
	auditWriter := &auditSpyWriter{}
	server.auditWriter = auditWriter

	url := "/v1/groups/" + strconv.FormatInt(groupID, 10) + "/join-requests/" + strconv.FormatInt(requestID, 10) + "/cancel"
	request, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	requireAPIErrorCode(t, recorder, ErrGroupJoinRequestReviewConflict)
	require.NotEmpty(t, auditWriter.Entries())
}

func strPtr(val string) *string {
	return &val
}

func int64Ptr(val int64) *int64 {
	return &val
}
