package api

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
)

func TestApproveRegionApplicationAdmin_WritesRegionThroughTransaction(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	application := db.OperatorRegionApplication{
		ID:         101,
		OperatorID: 201,
		RegionID:   301,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
	approved := application
	approved.Status = "approved"

	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().GetOperatorRegionApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), application.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().ApproveOperatorRegionApplicationTx(gomock.Any(), application.ID).Return(db.ApproveOperatorRegionApplicationTxResult{
		Application: approved,
		OperatorRegion: db.OperatorRegion{
			OperatorID: application.OperatorID,
			RegionID:   application.RegionID,
			Status:     "active",
		},
	}, nil)
	store.EXPECT().GetRegion(gomock.Any(), application.RegionID).Return(db.Region{ID: application.RegionID, Name: "测试区域"}, nil)

	request, err := http.NewRequest(http.MethodPost, "/v1/admin/operators/region-applications/101/approve", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp regionExpansionApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, approved.ID, resp.ID)
	require.Equal(t, "approved", resp.Status)
	require.Equal(t, "测试区域", resp.RegionName)
}

func TestApproveRegionApplicationAdmin_RejectsAlreadyReviewed(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	application := db.OperatorRegionApplication{ID: 102, OperatorID: 202, RegionID: 302, Status: "approved"}

	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().GetOperatorRegionApplication(gomock.Any(), application.ID).Return(application, nil)

	request, err := http.NewRequest(http.MethodPost, "/v1/admin/operators/region-applications/102/approve", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApproveRegionApplicationAdmin_MapsTransactionNoRowsToNotPending(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	application := db.OperatorRegionApplication{ID: 105, OperatorID: 205, RegionID: 305, Status: "pending"}

	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().GetOperatorRegionApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), application.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().ApproveOperatorRegionApplicationTx(gomock.Any(), application.ID).Return(db.ApproveOperatorRegionApplicationTxResult{}, errors.Join(errors.New("approve operator region application"), db.ErrRecordNotFound))

	request, err := http.NewRequest(http.MethodPost, "/v1/admin/operators/region-applications/105/approve", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestRejectRegionApplicationAdmin_RecordsReason(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	application := db.OperatorRegionApplication{ID: 103, OperatorID: 203, RegionID: 303, Status: "pending", CreatedAt: time.Now()}
	rejected := application
	rejected.Status = "rejected"
	rejected.RejectReason = pgtype.Text{String: "区域已有运营规划", Valid: true}

	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().GetOperatorRegionApplication(gomock.Any(), application.ID).Return(application, nil)
	store.EXPECT().RejectOperatorRegionApplication(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.RejectOperatorRegionApplicationParams) (db.OperatorRegionApplication, error) {
		require.Equal(t, application.ID, arg.ID)
		require.Equal(t, "区域已有运营规划", arg.RejectReason.String)
		return rejected, nil
	})
	store.EXPECT().GetRegion(gomock.Any(), application.RegionID).Return(db.Region{ID: application.RegionID, Name: "拒绝区域"}, nil)

	body := bytes.NewBufferString(`{"reject_reason":"区域已有运营规划"}`)
	request, err := http.NewRequest(http.MethodPost, "/v1/admin/operators/region-applications/103/reject", body)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp regionExpansionApplicationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "rejected", resp.Status)
	require.Equal(t, "区域已有运营规划", resp.RejectReason)
}

func TestRejectRegionApplicationAdmin_RejectsAlreadyReviewed(t *testing.T) {
	admin, _ := randomUser(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	application := db.OperatorRegionApplication{ID: 104, OperatorID: 204, RegionID: 304, Status: "rejected"}

	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().GetOperatorRegionApplication(gomock.Any(), application.ID).Return(application, nil)

	body := bytes.NewBufferString(`{"reject_reason":"重复拒绝"}`)
	request, err := http.NewRequest(http.MethodPost, "/v1/admin/operators/region-applications/104/reject", body)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
