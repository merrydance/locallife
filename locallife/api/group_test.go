package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
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

	body := updateGroupApplicationBasicRequest{
		GroupName:    strPtr("测试集团"),
		ContactPhone: strPtr("13800138000"),
		Address:      strPtr("北京市朝阳区测试路"),
		RegionID:     int64Ptr(1),
	}

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
				updated.GroupName = "测试集团"
				store.EXPECT().
					UpdateGroupApplicationBasic(gomock.Any(), gomock.Any()).
					Times(1).
					Return(updated, nil)
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

			data, err := json.Marshal(body)
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
					ReviewGroupApplication(gomock.Any(), gomock.Any()).
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

	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), user.ID).
		Times(1).
		Return(merchant, nil)

	store.EXPECT().
		GetMerchantGroupBinding(gomock.Any(), merchant.ID).
		Times(1).
		Return(db.GetMerchantGroupBindingRow{GroupID: pgtype.Int8{Valid: false}, BrandID: pgtype.Int8{Valid: false}}, nil)

	store.EXPECT().
		GetMerchantGroup(gomock.Any(), groupID).
		Times(1).
		Return(db.MerchantGroup{ID: groupID, Name: "测试集团", Status: "active", OwnerUserID: user.ID}, nil)

	joinReq := randomGroupJoinRequest(groupID, merchant.ID, user.ID)
	store.EXPECT().
		CreateGroupJoinRequest(gomock.Any(), gomock.Any()).
		Times(1).
		Return(joinReq, nil)

	store.EXPECT().
		CreateGroupAuditLog(gomock.Any(), gomock.Any()).
		Times(1)

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

func TestCancelGroupJoinRequestAPI(t *testing.T) {
	user, _ := randomUser(t)
	groupID := util.RandomInt(1, 1000)
	requestID := util.RandomInt(1, 1000)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	joinReq := randomGroupJoinRequest(groupID, util.RandomInt(1, 1000), user.ID)
	store.EXPECT().
		GetGroupJoinRequest(gomock.Any(), requestID).
		Times(1).
		Return(joinReq, nil)

	updated := joinReq
	updated.Status = "cancelled"
	store.EXPECT().
		UpdateGroupJoinRequestStatus(gomock.Any(), gomock.Any()).
		Times(1).
		Return(updated, nil)

	store.EXPECT().
		CreateGroupAuditLog(gomock.Any(), gomock.Any()).
		Times(1)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	url := "/v1/groups/" + strconv.FormatInt(groupID, 10) + "/join-requests/" + strconv.FormatInt(requestID, 10) + "/cancel"
	request, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func strPtr(val string) *string {
	return &val
}

func int64Ptr(val int64) *int64 {
	return &val
}
