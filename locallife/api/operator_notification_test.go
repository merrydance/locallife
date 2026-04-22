package api

import (
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

func TestListOperatorNotificationsAPI(t *testing.T) {
	user, _ := randomUser(t)
	notification := db.Notification{
		ID:        101,
		UserID:    user.ID,
		Type:      "delivery",
		Title:     "待接单提醒",
		Content:   "区域内有订单超过3分钟未被骑手接单，请尽快提醒骑手接单。",
		ExtraData: []byte(`{"audience":"operator","category":"dispatch_timeout","level":"warning","summary":"有订单超过3分钟仍未被骑手接单","region_id":66,"region_name":"测试区域","wait_minutes":4}`),
		CreatedAt: time.Now(),
	}
	operator := db.Operator{ID: 88, UserID: user.ID, RegionID: 66, Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), gomock.Eq(user.ID)).
		Return([]db.UserRole{{Role: RoleOperator, Status: "active"}}, nil)
	store.EXPECT().
		GetOperatorByUser(gomock.Any(), gomock.Eq(user.ID)).
		Return(operator, nil)
	store.EXPECT().
		ListOperatorNotifications(gomock.Any(), gomock.Eq(db.ListOperatorNotificationsParams{
			UserID: user.ID,
			Limit:  10,
			Offset: 0,
		})).
		Return([]db.Notification{notification}, nil)
	store.EXPECT().
		CountOperatorNotifications(gomock.Any(), gomock.Eq(db.CountOperatorNotificationsParams{UserID: user.ID})).
		Return(int64(1), nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operators/me/notifications?limit=10&offset=0", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response listOperatorNotificationsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Len(t, response.Notifications, 1)
	require.Equal(t, operatorNotificationCategoryDispatchTimeout, response.Notifications[0].Category)
	require.EqualValues(t, 4, *response.Notifications[0].WaitMinutes)
	require.Equal(t, "测试区域", *response.Notifications[0].RegionName)
}

func TestGetOperatorNotificationSummaryAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{ID: 98, UserID: user.ID, RegionID: 77, Status: "active"}
	latest := db.Notification{
		ID:        202,
		UserID:    user.ID,
		Type:      "delivery",
		Title:     "待接单提醒",
		Content:   "区域内有订单超过3分钟未被骑手接单，请尽快提醒骑手接单。",
		ExtraData: []byte(`{"audience":"operator","category":"dispatch_timeout","summary":"有订单超过3分钟仍未被骑手接单"}`),
		CreatedAt: time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), gomock.Eq(user.ID)).
		Return([]db.UserRole{{Role: RoleOperator, Status: "active"}}, nil)
	store.EXPECT().
		GetOperatorByUser(gomock.Any(), gomock.Eq(user.ID)).
		Return(operator, nil)
	store.EXPECT().
		CountOperatorNotifications(gomock.Any(), gomock.Eq(db.CountOperatorNotificationsParams{
			UserID: user.ID,
			IsRead: pgtype.Bool{Bool: false, Valid: true},
		})).
		Return(int64(3), nil)
	store.EXPECT().
		ListOperatorNotifications(gomock.Any(), gomock.Eq(db.ListOperatorNotificationsParams{
			UserID: user.ID,
			Limit:  1,
			Offset: 0,
		})).
		Return([]db.Notification{latest}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operators/me/notifications/summary", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response operatorNotificationSummaryResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, int64(3), response.UnreadCount)
	require.NotNil(t, response.LatestNotification)
	require.Equal(t, latest.ID, response.LatestNotification.ID)
}

func TestGetOperatorNotificationAPI_RejectsNonOperatorAudience(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{ID: 98, UserID: user.ID, RegionID: 77, Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), gomock.Eq(user.ID)).
		Return([]db.UserRole{{Role: RoleOperator, Status: "active"}}, nil)
	store.EXPECT().
		GetOperatorByUser(gomock.Any(), gomock.Eq(user.ID)).
		Return(operator, nil)
	store.EXPECT().
		GetOperatorNotification(gomock.Any(), gomock.Eq(db.GetOperatorNotificationParams{
			ID:     202,
			UserID: user.ID,
		})).
		Return(db.Notification{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operators/me/notifications/202", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestMarkAllOperatorNotificationsAsReadAPI_ScopedToOperatorAudience(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{ID: 98, UserID: user.ID, RegionID: 77, Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), gomock.Eq(user.ID)).
		Return([]db.UserRole{{Role: RoleOperator, Status: "active"}}, nil)
	store.EXPECT().
		GetOperatorByUser(gomock.Any(), gomock.Eq(user.ID)).
		Return(operator, nil)
	store.EXPECT().
		MarkAllOperatorNotificationsAsRead(gomock.Any(), gomock.Eq(user.ID)).
		Return(nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodPut, "/v1/operators/me/notifications/read-all", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
}
