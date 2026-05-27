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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func expectAdminRoleForPlatformEntity(t *testing.T, store *mockdb.MockStore, admin db.User) {
	t.Helper()
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
}

func performAdminRequest(t *testing.T, server *Server, method string, target string, admin db.User) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(method, target, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)
	return recorder
}

func performAdminJSONRequest(t *testing.T, server *Server, method string, target string, body any, admin db.User) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(method, target, bytes.NewReader(payload))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)
	return recorder
}

func TestListPlatformRidersAPI_ReturnsEntityCards(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		ListPlatformRiderCards(gomock.Any(), db.ListPlatformRiderCardsParams{
			Limit:  20,
			Offset: 0,
		}).
		Return([]db.ListPlatformRiderCardsRow{{
			ID:             11,
			RealName:       "骑手甲",
			RegionID:       pgtype.Int8{Int64: 3101, Valid: true},
			RegionName:     pgtype.Text{String: "西湖区", Valid: true},
			Status:         db.RiderStatusActive,
			AcceptedIn3d:   true,
			ComplaintCount: 2,
		}}, nil)
	store.EXPECT().
		CountPlatformRiders(gomock.Any()).
		Return(int64(1), nil)

	recorder := performAdminRequest(t, server, http.MethodGet, "/v1/admin/riders?page=1&limit=20", admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp platformRiderListResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Riders, 1)
	require.Equal(t, int64(11), resp.Riders[0].ID)
	require.Equal(t, "骑手甲", resp.Riders[0].Name)
	require.Equal(t, "西湖区", resp.Riders[0].RegionName)
	require.True(t, resp.Riders[0].Active)
	require.Equal(t, int64(2), resp.Riders[0].ComplaintCount)
	require.False(t, resp.HasMore)
}

func TestListPlatformRidersAPI_ActiveUsesRecentAcceptedOrders(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		ListPlatformRiderCards(gomock.Any(), db.ListPlatformRiderCardsParams{
			Limit:  20,
			Offset: 0,
		}).
		Return([]db.ListPlatformRiderCardsRow{{
			ID:             12,
			RealName:       "骑手乙",
			RegionID:       pgtype.Int8{Int64: 3101, Valid: true},
			RegionName:     pgtype.Text{String: "西湖区", Valid: true},
			Status:         db.RiderStatusSuspended,
			AcceptedIn3d:   true,
			ComplaintCount: 0,
		}}, nil)
	store.EXPECT().
		CountPlatformRiders(gomock.Any()).
		Return(int64(1), nil)

	recorder := performAdminRequest(t, server, http.MethodGet, "/v1/admin/riders?page=1&limit=20", admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp platformRiderListResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Riders, 1)
	require.True(t, resp.Riders[0].Active)
}

func TestGetPlatformRiderDetailAPI_ReturnsCoreStatsAndComplaintBreakdown(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	now := time.Now()
	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		GetPlatformRiderDetail(gomock.Any(), int64(11)).
		Return(db.GetPlatformRiderDetailRow{
			ID:                11,
			RealName:          "骑手甲",
			RegionID:          pgtype.Int8{Int64: 3101, Valid: true},
			RegionName:        pgtype.Text{String: "西湖区", Valid: true},
			Status:            db.RiderStatusActive,
			AcceptedIn3d:      true,
			TotalOrders:       80,
			TotalEarnings:     123400,
			MonthOrders:       12,
			MonthIncome:       35600,
			ComplaintCount:    3,
			IDCardOcr:         []byte(`{"gender":"男","birth_date":"1990-01-02"}`),
			CreatedAt:         now,
			LocationUpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		}, nil)
	store.EXPECT().
		ListPlatformRiderComplaintCategories(gomock.Any(), int64(11)).
		Return([]db.ListPlatformRiderComplaintCategoriesRow{{
			ClaimType: "delay",
			Count:     2,
		}}, nil)

	recorder := performAdminRequest(t, server, http.MethodGet, "/v1/admin/riders/11", admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp platformRiderDetailResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(11), resp.ID)
	require.Equal(t, "骑手甲", resp.Name)
	require.Equal(t, int32(80), resp.OrderStats.TotalOrders)
	require.Equal(t, int64(123400), resp.OrderStats.TotalIncome)
	require.Equal(t, int32(12), resp.OrderStats.LastMonthOrders)
	require.Equal(t, int64(35600), resp.OrderStats.LastMonthIncome)
	require.Equal(t, int64(3), resp.Service.ComplaintCount)
	require.Equal(t, "delay", resp.Service.ComplaintCategories[0].Category)
	require.Equal(t, int64(2), resp.Service.ComplaintCategories[0].Count)
	require.NotNil(t, resp.Basic.Age)
	require.Equal(t, int32(36), *resp.Basic.Age)
	require.Equal(t, "男", resp.Basic.Gender)
}

func TestUpdatePlatformRiderAcceptingStatusAPI_MapsPauseAndResume(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		GetRider(gomock.Any(), int64(11)).
		Return(db.Rider{ID: 11, Status: db.RiderStatusActive}, nil)
	store.EXPECT().
		UpdateRiderStatus(gomock.Any(), db.UpdateRiderStatusParams{ID: 11, Status: db.RiderStatusSuspended}).
		Return(db.Rider{ID: 11, Status: db.RiderStatusSuspended}, nil)

	recorder := performAdminRequest(t, server, http.MethodPost, "/v1/admin/riders/11/pause-accepting", admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp platformRiderStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(11), resp.ID)
	require.Equal(t, db.RiderStatusSuspended, resp.Status)
}

func TestUpdatePlatformRiderAcceptingStatusAPI_RejectsInvalidCurrentStatus(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		GetRider(gomock.Any(), int64(11)).
		Return(db.Rider{ID: 11, Status: db.RiderStatusApproved}, nil)

	recorder := performAdminRequest(t, server, http.MethodPost, "/v1/admin/riders/11/pause-accepting", admin)
	require.Equal(t, http.StatusConflict, recorder.Code)
}

func TestListPlatformOperatorsAPI_ReturnsEntityCards(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		ListPlatformOperatorCards(gomock.Any(), db.ListPlatformOperatorCardsParams{
			Limit:  20,
			Offset: 0,
		}).
		Return([]db.ListPlatformOperatorCardsRow{{
			ID:             21,
			Name:           "运营商甲",
			RegionCount:    2,
			MerchantCount:  8,
			Status:         "active",
			ComplaintCount: 4,
		}}, nil)
	store.EXPECT().
		CountPlatformOperators(gomock.Any()).
		Return(int64(1), nil)

	recorder := performAdminRequest(t, server, http.MethodGet, "/v1/admin/operators?page=1&limit=20", admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp platformOperatorListResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Operators, 1)
	require.Equal(t, "运营商甲", resp.Operators[0].Name)
	require.Equal(t, int64(2), resp.Operators[0].RegionCount)
	require.Equal(t, int64(8), resp.Operators[0].MerchantCount)
	require.Equal(t, int64(4), resp.Operators[0].ComplaintCount)
}

func TestGetPlatformOperatorDetailAPI_ReturnsCoreStats(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	now := time.Now()

	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		GetPlatformOperatorDetail(gomock.Any(), int64(21)).
		Return(db.GetPlatformOperatorDetailRow{
			ID:             21,
			Name:           "运营商甲",
			ContactName:    "张三",
			ContactPhone:   "13800000000",
			Status:         "active",
			RegionID:       3101,
			RegionName:     pgtype.Text{String: "西湖区", Valid: true},
			RegionCount:    2,
			MerchantCount:  8,
			MonthOrders:    33,
			MonthRevenue:   880000,
			ComplaintCount: 4,
			CreatedAt:      now,
		}, nil)
	store.EXPECT().
		ListPlatformOperatorRegions(gomock.Any(), int64(21)).
		Return([]db.ListPlatformOperatorRegionsRow{{
			RegionID:   3101,
			RegionName: "西湖区",
			Status:     "active",
		}}, nil)
	store.EXPECT().
		ListPlatformOperatorComplaintCategories(gomock.Any(), int64(21)).
		Return([]db.ListPlatformOperatorComplaintCategoriesRow{{
			ClaimType: "quality",
			Count:     3,
		}}, nil)

	recorder := performAdminRequest(t, server, http.MethodGet, "/v1/admin/operators/21", admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp platformOperatorDetailResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(21), resp.ID)
	require.Equal(t, "运营商甲", resp.Name)
	require.Equal(t, int64(2), resp.RegionCount)
	require.Equal(t, int64(8), resp.MerchantCount)
	require.Equal(t, int32(33), resp.OrderStats.LastMonthOrders)
	require.Equal(t, int64(880000), resp.OrderStats.LastMonthIncome)
	require.Equal(t, int64(4), resp.Service.ComplaintCount)
	require.Equal(t, "quality", resp.Service.ComplaintCategories[0].Category)
	require.Len(t, resp.Regions, 1)
}

func TestUpdatePlatformOperatorStatusAPI_UsesStatusServiceRoute(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	operator := db.Operator{ID: 21, UserID: 321, RegionID: 3101, Status: "active"}
	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		GetOperator(gomock.Any(), int64(21)).
		Return(operator, nil)
	store.EXPECT().
		GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
			UserID: int64(321),
			Role:   RoleOperator,
		}).
		Return(db.UserRole{ID: 91, UserID: 321, Role: RoleOperator, Status: "active"}, nil)
	store.EXPECT().
		UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{
			ID:     int64(21),
			Status: "suspended",
		}).
		Return(db.Operator{ID: 21, UserID: 321, RegionID: 3101, Status: "suspended"}, nil)
	store.EXPECT().
		UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{
			ID:     int64(91),
			Status: "suspended",
		}).
		Return(db.UserRole{ID: 91, UserID: 321, Role: RoleOperator, Status: "suspended"}, nil)

	recorder := performAdminJSONRequest(t, server, http.MethodPost, "/v1/admin/operators/21/status", map[string]string{
		"status": "suspended",
	}, admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp adminOperatorStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(21), resp.ID)
	require.Equal(t, "suspended", resp.Status)
}

func TestListPlatformMerchantsAPI_ReturnsEntityCards(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		ListPlatformMerchantCards(gomock.Any(), db.ListPlatformMerchantCardsParams{
			Limit:  20,
			Offset: 0,
		}).
		Return([]db.ListPlatformMerchantCardsRow{{
			ID:             31,
			Name:           "商户甲",
			RegionID:       3101,
			RegionName:     pgtype.Text{String: "西湖区", Valid: true},
			Status:         "active",
			IsOpen:         true,
			MonthOrders:    18,
			ComplaintCount: 1,
		}}, nil)
	store.EXPECT().
		CountPlatformMerchants(gomock.Any()).
		Return(int64(1), nil)

	recorder := performAdminRequest(t, server, http.MethodGet, "/v1/admin/merchants?page=1&limit=20", admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp platformMerchantListResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Merchants, 1)
	require.Equal(t, "商户甲", resp.Merchants[0].Name)
	require.Equal(t, "西湖区", resp.Merchants[0].RegionName)
	require.True(t, resp.Merchants[0].IsOpen)
	require.Equal(t, int32(18), resp.Merchants[0].MonthOrders)
	require.Equal(t, int64(1), resp.Merchants[0].ComplaintCount)
}

func TestGetPlatformMerchantDetailAPI_ReturnsCoreStats(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	now := time.Now()

	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		GetPlatformMerchantDetail(gomock.Any(), int64(31)).
		Return(db.GetPlatformMerchantDetailRow{
			ID:             31,
			Name:           "商户甲",
			Phone:          "13800000000",
			Address:        "文三路 1 号",
			RegionID:       3101,
			RegionName:     pgtype.Text{String: "西湖区", Valid: true},
			Status:         "active",
			IsOpen:         true,
			TotalOrders:    120,
			TotalIncome:    998800,
			MonthOrders:    18,
			MonthIncome:    120000,
			ComplaintCount: 1,
			CreatedAt:      now,
		}, nil)
	store.EXPECT().
		ListPlatformMerchantComplaintCategories(gomock.Any(), int64(31)).
		Return([]db.ListPlatformMerchantComplaintCategoriesRow{{
			ClaimType: "missing-item",
			Count:     1,
		}}, nil)

	recorder := performAdminRequest(t, server, http.MethodGet, "/v1/admin/merchants/31", admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp platformMerchantDetailResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(31), resp.ID)
	require.Equal(t, "商户甲", resp.Basic.Name)
	require.Equal(t, int32(120), resp.OrderStats.TotalOrders)
	require.Equal(t, int64(998800), resp.OrderStats.TotalIncome)
	require.Equal(t, int32(18), resp.OrderStats.LastMonthOrders)
	require.Equal(t, int64(120000), resp.OrderStats.LastMonthIncome)
	require.Equal(t, int64(1), resp.Service.ComplaintCount)
	require.Equal(t, "missing-item", resp.Service.ComplaintCategories[0].Category)
}

func TestGetPlatformMerchantDetailAPI_OnlyActiveMerchantsCanSuspend(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	now := time.Now()

	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		GetPlatformMerchantDetail(gomock.Any(), int64(31)).
		Return(db.GetPlatformMerchantDetailRow{
			ID:        31,
			Name:      "商户甲",
			Phone:     "13800000000",
			Address:   "文三路 1 号",
			RegionID:  3101,
			Status:    "pending",
			IsOpen:    false,
			CreatedAt: now,
		}, nil)
	store.EXPECT().
		ListPlatformMerchantComplaintCategories(gomock.Any(), int64(31)).
		Return([]db.ListPlatformMerchantComplaintCategoriesRow{}, nil)

	recorder := performAdminRequest(t, server, http.MethodGet, "/v1/admin/merchants/31", admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp platformMerchantDetailResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.False(t, resp.CanSuspend)
	require.False(t, resp.CanResume)
}

func TestUpdatePlatformMerchantStatusAPI_MapsSuspendAndResume(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		GetMerchant(gomock.Any(), int64(31)).
		Return(db.Merchant{ID: 31, Status: db.MerchantStatusActive}, nil)
	store.EXPECT().
		UpdateMerchantStatus(gomock.Any(), db.UpdateMerchantStatusParams{ID: 31, Status: db.MerchantStatusSuspended}).
		Return(db.Merchant{ID: 31, Status: db.MerchantStatusSuspended}, nil)

	recorder := performAdminRequest(t, server, http.MethodPost, "/v1/admin/merchants/31/suspend", admin)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp platformMerchantStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(31), resp.ID)
	require.Equal(t, "suspended", resp.Status)
}

func TestUpdatePlatformMerchantStatusAPI_RejectsInvalidCurrentStatus(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		GetMerchant(gomock.Any(), int64(31)).
		Return(db.Merchant{ID: 31, Status: db.MerchantStatusActive}, nil)

	recorder := performAdminRequest(t, server, http.MethodPost, "/v1/admin/merchants/31/resume", admin)
	require.Equal(t, http.StatusConflict, recorder.Code)
}
