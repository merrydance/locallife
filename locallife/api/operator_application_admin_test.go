package api

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

func TestListPendingOperatorApplicationsAdminAPI(t *testing.T) {
	admin, _ := randomUser(t)
	now := time.Now()
	submittedAt := pgtype.Timestamptz{Time: now, Valid: true}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), admin.ID).
					Return([]db.UserRole{{
						UserID: admin.ID,
						Role:   RoleAdmin,
						Status: "active",
					}}, nil)

				store.EXPECT().
					ListPendingOperatorApplications(gomock.Any(), db.ListPendingOperatorApplicationsParams{
						Limit:  20,
						Offset: 0,
					}).
					Return([]db.ListPendingOperatorApplicationsRow{{
						ID:                     101,
						UserID:                 202,
						RegionID:               303,
						Name:                   pgtype.Text{String: "测试运营商", Valid: true},
						ContactName:            pgtype.Text{String: "联系人甲", Valid: true},
						ContactPhone:           pgtype.Text{String: "13800138000", Valid: true},
						LegalPersonName:        pgtype.Text{String: "法人甲", Valid: true},
						RequestedContractYears: 2,
						Status:                 "submitted",
						CreatedAt:              now,
						SubmittedAt:            submittedAt,
						ApplicantName:          pgtype.Text{String: "提交人甲", Valid: true},
						ApplicantPhone:         pgtype.Text{String: "13900139000", Valid: true},
						RegionName:             "测试区域",
						RegionCode:             "CN-TEST",
					}}, nil)

				store.EXPECT().
					CountPendingOperatorApplications(gomock.Any()).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp listPendingOperatorApplicationsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Applications, 1)
				require.Equal(t, "提交人甲", resp.Applications[0].ApplicantName)
				require.Equal(t, "13900139000", resp.Applications[0].ApplicantPhone)
				require.Equal(t, "联系人甲", resp.Applications[0].ContactName)
				require.False(t, resp.HasMore)
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

			request, err := http.NewRequest(http.MethodGet, "/v1/admin/operators/applications?page=1&limit=20", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestApproveOperatorApplicationAdmin_EnsuresProfitSharingReceiver(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServerWithEcommerce(t, store, ecommerceClient)

	application := db.OperatorApplication{
		ID:                     11,
		UserID:                 88,
		RegionID:               66,
		Name:                   pgtype.Text{String: "华北运营商", Valid: true},
		ContactName:            pgtype.Text{String: "张运营", Valid: true},
		ContactPhone:           pgtype.Text{String: "13800138000", Valid: true},
		RequestedContractYears: 2,
		Status:                 "submitted",
	}
	approved := application
	approved.Status = "approved"
	operator := db.Operator{
		ID:           99,
		UserID:       application.UserID,
		RegionID:     application.RegionID,
		Name:         "华北运营商",
		ContactName:  "张运营",
		ContactPhone: "13800138000",
		Status:       "active",
	}
	user := db.User{ID: application.UserID, WechatOpenid: "operator-openid-88"}

	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().
		GetOperatorApplicationByID(gomock.Any(), application.ID).
		Return(application, nil)
	store.EXPECT().
		GetOperatorByUser(gomock.Any(), application.UserID).
		Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetActiveOperatorByRegion(gomock.Any(), application.RegionID).
		Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetUser(gomock.Any(), application.UserID).
		Return(user, nil)
	store.EXPECT().
		ApproveOperatorApplication(gomock.Any(), gomock.Any()).
		Return(approved, nil)
	store.EXPECT().
		CreateOperator(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateOperatorParams) (db.Operator, error) {
			require.Equal(t, "active", arg.Status)
			require.Equal(t, application.UserID, arg.UserID)
			return operator, nil
		})
	store.EXPECT().
		AddOperatorRegion(gomock.Any(), db.AddOperatorRegionParams{OperatorID: operator.ID, RegionID: application.RegionID}).
		Return(db.OperatorRegion{OperatorID: operator.ID, RegionID: application.RegionID}, nil)
	store.EXPECT().
		GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: application.UserID, Role: RoleOperator}).
		Return(db.UserRole{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateUserRole(gomock.Any(), gomock.Any()).
		Return(db.UserRole{UserID: application.UserID, Role: RoleOperator, Status: "active"}, nil)
	store.EXPECT().
		GetUser(gomock.Any(), application.UserID).
		Return(user, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	ecommerceClient.EXPECT().EncryptSensitiveData("张运营").Return("enc-name", nil)
	ecommerceClient.EXPECT().
		AddProfitSharingReceiver(gomock.Any(), &wechatcontracts.AddReceiverRequest{
			AppID:         "wx_sp_app_123",
			Type:          wechatcontracts.ReceiverTypePersonal,
			Account:       user.WechatOpenid,
			EncryptedName: "enc-name",
			RelationType:  wechatcontracts.RelationOthers,
		}).
		Return(&wechatcontracts.AddReceiverResponse{Type: wechatcontracts.ReceiverTypePersonal, Account: user.WechatOpenid}, nil)
	store.EXPECT().
		GetRegion(gomock.Any(), application.RegionID).
		Return(db.Region{ID: application.RegionID, Name: "测试区域"}, nil)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/admin/operators/applications/11/approve", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp operatorApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, approved.ID, resp.ID)
	require.Equal(t, "approved", resp.Status)
}

func TestUpdateOperatorStatusAdmin_SuspendDeletesProfitSharingReceiver(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServerWithEcommerce(t, store, ecommerceClient)

	operator := db.Operator{
		ID:          35,
		UserID:      88,
		RegionID:    66,
		Name:        "运营商甲",
		ContactName: "运营商甲",
		Status:      "active",
	}
	role := db.UserRole{ID: 91, UserID: operator.UserID, Role: RoleOperator, Status: "active"}
	user := db.User{ID: operator.UserID, WechatOpenid: "operator-openid-88"}

	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().GetOperator(gomock.Any(), operator.ID).Return(operator, nil)
	store.EXPECT().
		UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: operator.ID, Status: "suspended"}).
		Return(db.Operator{ID: operator.ID, UserID: operator.UserID, RegionID: operator.RegionID, Name: operator.Name, ContactName: operator.ContactName, Status: "suspended"}, nil)
	store.EXPECT().
		GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: operator.UserID, Role: RoleOperator}).
		Return(role, nil)
	store.EXPECT().
		UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: role.ID, Status: "suspended"}).
		Return(db.UserRole{ID: role.ID, UserID: operator.UserID, Role: RoleOperator, Status: "suspended"}, nil)
	store.EXPECT().GetUser(gomock.Any(), operator.UserID).Return(user, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	ecommerceClient.EXPECT().DeleteProfitSharingReceiver(gomock.Any(), &wechatcontracts.DeleteReceiverRequest{
		AppID:   "wx_sp_app_123",
		Type:    wechatcontracts.ReceiverTypePersonal,
		Account: user.WechatOpenid,
	}).Return(&wechatcontracts.DeleteReceiverResponse{Type: wechatcontracts.ReceiverTypePersonal, Account: user.WechatOpenid}, nil)

	body := bytes.NewBufferString(`{"status":"suspended"}`)
	request, err := http.NewRequest(http.MethodPost, "/v1/admin/operators/35/status", body)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp adminOperatorStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, operator.ID, resp.ID)
	require.Equal(t, "suspended", resp.Status)
}

func TestUpdateOperatorStatusAdmin_ActivateChecksRegionConflict(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	operator := db.Operator{
		ID:       36,
		UserID:   89,
		RegionID: 77,
		Status:   "suspended",
	}
	otherActiveOperator := db.Operator{ID: 99, UserID: 199, RegionID: 77, Status: "active"}

	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().GetOperator(gomock.Any(), operator.ID).Return(operator, nil)
	store.EXPECT().ListOperatorRegions(gomock.Any(), operator.ID).Return([]db.ListOperatorRegionsRow{{OperatorID: operator.ID, RegionID: operator.RegionID, Status: "active"}}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), operator.RegionID).Return(otherActiveOperator, nil)

	body := bytes.NewBufferString(`{"status":"active"}`)
	request, err := http.NewRequest(http.MethodPost, "/v1/admin/operators/36/status", body)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusConflict, recorder.Code)
}

func TestUpdateOperatorStatusAdmin_ActivateReenablesRoleAndReceiver(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServerWithEcommerce(t, store, ecommerceClient)

	operator := db.Operator{
		ID:          37,
		UserID:      90,
		RegionID:    78,
		Name:        "运营商乙",
		ContactName: "李运营",
		Status:      "suspended",
	}
	role := db.UserRole{ID: 92, UserID: operator.UserID, Role: RoleOperator, Status: "suspended"}
	user := db.User{ID: operator.UserID, WechatOpenid: "operator-openid-90"}

	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().GetOperator(gomock.Any(), operator.ID).Return(operator, nil)
	store.EXPECT().ListOperatorRegions(gomock.Any(), operator.ID).Return([]db.ListOperatorRegionsRow{{OperatorID: operator.ID, RegionID: operator.RegionID, Status: "active"}}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), operator.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().
		UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: operator.ID, Status: "active"}).
		Return(db.Operator{ID: operator.ID, UserID: operator.UserID, RegionID: operator.RegionID, Name: operator.Name, ContactName: operator.ContactName, Status: "active"}, nil)
	store.EXPECT().
		GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: operator.UserID, Role: RoleOperator}).
		Return(role, nil)
	store.EXPECT().
		UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: role.ID, Status: "active"}).
		Return(db.UserRole{ID: role.ID, UserID: operator.UserID, Role: RoleOperator, Status: "active"}, nil)
	store.EXPECT().GetUser(gomock.Any(), operator.UserID).Return(user, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	ecommerceClient.EXPECT().EncryptSensitiveData("李运营").Return("enc-name", nil)
	ecommerceClient.EXPECT().AddProfitSharingReceiver(gomock.Any(), &wechatcontracts.AddReceiverRequest{
		AppID:         "wx_sp_app_123",
		Type:          wechatcontracts.ReceiverTypePersonal,
		Account:       user.WechatOpenid,
		EncryptedName: "enc-name",
		RelationType:  wechatcontracts.RelationOthers,
	}).Return(&wechatcontracts.AddReceiverResponse{Type: wechatcontracts.ReceiverTypePersonal, Account: user.WechatOpenid}, nil)

	body := bytes.NewBufferString(`{"status":"active"}`)
	request, err := http.NewRequest(http.MethodPost, "/v1/admin/operators/37/status", body)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp adminOperatorStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, operator.ID, resp.ID)
	require.Equal(t, "active", resp.Status)
}

func TestBatchUpdateOperatorStatusAdmin_SuspendContinuesAfterFailure(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServerWithEcommerce(t, store, ecommerceClient)

	first := db.Operator{ID: 41, UserID: 141, RegionID: 66, Name: "运营商甲", ContactName: "运营商甲", Status: "active"}
	third := db.Operator{ID: 43, UserID: 143, RegionID: 68, Name: "运营商丙", ContactName: "运营商丙", Status: "active"}
	firstRole := db.UserRole{ID: 301, UserID: first.UserID, Role: RoleOperator, Status: "active"}
	firstUser := db.User{ID: first.UserID, WechatOpenid: "operator-openid-141"}
	thirdUser := db.User{ID: third.UserID, WechatOpenid: "operator-openid-143"}

	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().GetOperator(gomock.Any(), first.ID).Return(first, nil)
	store.EXPECT().
		UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: first.ID, Status: "suspended"}).
		Return(db.Operator{ID: first.ID, UserID: first.UserID, RegionID: first.RegionID, Name: first.Name, ContactName: first.ContactName, Status: "suspended"}, nil)
	store.EXPECT().
		GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: first.UserID, Role: RoleOperator}).
		Return(firstRole, nil)
	store.EXPECT().
		UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: firstRole.ID, Status: "suspended"}).
		Return(db.UserRole{ID: firstRole.ID, UserID: first.UserID, Role: RoleOperator, Status: "suspended"}, nil)
	store.EXPECT().GetUser(gomock.Any(), first.UserID).Return(firstUser, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	ecommerceClient.EXPECT().DeleteProfitSharingReceiver(gomock.Any(), &wechatcontracts.DeleteReceiverRequest{
		AppID:   "wx_sp_app_123",
		Type:    wechatcontracts.ReceiverTypePersonal,
		Account: firstUser.WechatOpenid,
	}).Return(&wechatcontracts.DeleteReceiverResponse{Type: wechatcontracts.ReceiverTypePersonal, Account: firstUser.WechatOpenid}, nil)

	store.EXPECT().GetOperator(gomock.Any(), int64(42)).Return(db.Operator{}, db.ErrRecordNotFound)

	store.EXPECT().GetOperator(gomock.Any(), third.ID).Return(third, nil)
	store.EXPECT().GetUser(gomock.Any(), third.UserID).Return(thirdUser, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	ecommerceClient.EXPECT().DeleteProfitSharingReceiver(gomock.Any(), &wechatcontracts.DeleteReceiverRequest{
		AppID:   "wx_sp_app_123",
		Type:    wechatcontracts.ReceiverTypePersonal,
		Account: thirdUser.WechatOpenid,
	}).Return(nil, errors.New("wechat delete failed"))

	body := bytes.NewBufferString(`{"operator_ids":[41,42,43],"status":"suspended"}`)
	request, err := http.NewRequest(http.MethodPost, "/v1/admin/operators/batch/status", body)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp batchAdminOperatorStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Updated, 1)
	require.Equal(t, first.ID, resp.Updated[0].ID)
	require.Len(t, resp.Failed, 2)
	require.Equal(t, int64(42), resp.Failed[0].OperatorID)
	require.Equal(t, ErrOperatorNotFound.Code, resp.Failed[0].Code)
	require.Equal(t, ErrOperatorNotFound.Message, resp.Failed[0].Error)
	require.Equal(t, third.ID, resp.Failed[1].OperatorID)
	require.Contains(t, resp.Failed[1].Error, "operator receiver sync delete failed")
}

func TestBatchUpdateOperatorStatusAdmin_ActivateMixedResults(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	server := newTestServerWithEcommerce(t, store, ecommerceClient)

	first := db.Operator{ID: 51, UserID: 151, RegionID: 71, Status: "suspended"}
	second := db.Operator{ID: 52, UserID: 152, RegionID: 72, Name: "运营商乙", ContactName: "李运营", Status: "suspended"}
	otherActiveOperator := db.Operator{ID: 99, UserID: 199, RegionID: first.RegionID, Status: "active"}
	secondRole := db.UserRole{ID: 352, UserID: second.UserID, Role: RoleOperator, Status: "suspended"}
	secondUser := db.User{ID: second.UserID, WechatOpenid: "operator-openid-152"}

	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

	store.EXPECT().GetOperator(gomock.Any(), first.ID).Return(first, nil)
	store.EXPECT().ListOperatorRegions(gomock.Any(), first.ID).Return([]db.ListOperatorRegionsRow{{OperatorID: first.ID, RegionID: first.RegionID, Status: "active"}}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), first.RegionID).Return(otherActiveOperator, nil)

	store.EXPECT().GetOperator(gomock.Any(), second.ID).Return(second, nil)
	store.EXPECT().ListOperatorRegions(gomock.Any(), second.ID).Return([]db.ListOperatorRegionsRow{{OperatorID: second.ID, RegionID: second.RegionID, Status: "active"}}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), second.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().
		UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: second.ID, Status: "active"}).
		Return(db.Operator{ID: second.ID, UserID: second.UserID, RegionID: second.RegionID, Name: second.Name, ContactName: second.ContactName, Status: "active"}, nil)
	store.EXPECT().
		GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: second.UserID, Role: RoleOperator}).
		Return(secondRole, nil)
	store.EXPECT().
		UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: secondRole.ID, Status: "active"}).
		Return(db.UserRole{ID: secondRole.ID, UserID: second.UserID, Role: RoleOperator, Status: "active"}, nil)
	store.EXPECT().GetUser(gomock.Any(), second.UserID).Return(secondUser, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	ecommerceClient.EXPECT().EncryptSensitiveData(second.ContactName).Return("enc-name", nil)
	ecommerceClient.EXPECT().AddProfitSharingReceiver(gomock.Any(), &wechatcontracts.AddReceiverRequest{
		AppID:         "wx_sp_app_123",
		Type:          wechatcontracts.ReceiverTypePersonal,
		Account:       secondUser.WechatOpenid,
		EncryptedName: "enc-name",
		RelationType:  wechatcontracts.RelationOthers,
	}).Return(&wechatcontracts.AddReceiverResponse{Type: wechatcontracts.ReceiverTypePersonal, Account: secondUser.WechatOpenid}, nil)

	body := bytes.NewBufferString(`{"operator_ids":[51,52],"status":"active"}`)
	request, err := http.NewRequest(http.MethodPost, "/v1/admin/operators/batch/status", body)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp batchAdminOperatorStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Updated, 1)
	require.Equal(t, second.ID, resp.Updated[0].ID)
	require.Len(t, resp.Failed, 1)
	require.Equal(t, first.ID, resp.Failed[0].OperatorID)
	require.Equal(t, ErrRegionHasOperator.Code, resp.Failed[0].Code)
	require.Equal(t, ErrRegionHasOperator.Message, resp.Failed[0].Error)
}
