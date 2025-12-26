package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListNotificationsAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?limit=10&offset=0",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.ListUserNotificationsParams{
					UserID: user.ID,
					Limit:  10,
					Offset: 0,
				}

				notifications := []db.Notification{
					{
						ID:        1,
						UserID:    user.ID,
						Type:      "order",
						Title:     "测试通知",
						Content:   "测试内容",
						IsRead:    false,
						IsPushed:  false,
						CreatedAt: time.Now(),
					},
				}

				store.EXPECT().
					ListUserNotifications(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(notifications, nil)

				countArg := db.CountUserNotificationsParams{
					UserID: user.ID,
				}
				store.EXPECT().
					CountUserNotifications(gomock.Any(), gomock.Eq(countArg)).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response listNotificationsResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, int64(1), response.TotalCount)
				require.Len(t, response.Notifications, 1)
				require.Equal(t, "order", response.Notifications[0].Type)
			},
		},
		{
			name:  "Unauthorized",
			query: "?limit=10&offset=0",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加认证
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserNotifications(gomock.Any(), gomock.Any()).
					Times(0)
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

			url := "/v1/notifications" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetUnreadCountAPI(t *testing.T) {
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
				store.EXPECT().
					CountUnreadNotifications(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(int64(5), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response unreadCountResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, int64(5), response.Count)
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

			url := "/v1/notifications/unread/count"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestMarkNotificationAsReadAPI(t *testing.T) {
	user, _ := randomUser(t)

	notification := db.Notification{
		ID:        1,
		UserID:    user.ID,
		Type:      "order",
		Title:     "测试通知",
		Content:   "测试内容",
		IsRead:    false,
		IsPushed:  false,
		CreatedAt: time.Now(),
	}

	testCases := []struct {
		name           string
		notificationID int64
		setupAuth      func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs     func(store *mockdb.MockStore)
		checkResponse  func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:           "OK",
			notificationID: notification.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.MarkNotificationAsReadParams{
					ID:     notification.ID,
					UserID: user.ID,
				}

				updatedNotification := notification
				updatedNotification.IsRead = true
				now := time.Now()
				updatedNotification.ReadAt = pgtype.Timestamptz{Time: now, Valid: true}

				store.EXPECT().
					MarkNotificationAsRead(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(updatedNotification, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response notificationResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, notification.ID, response.ID)
				require.True(t, response.IsRead)
			},
		},
		{
			name:           "NotFound",
			notificationID: 999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					MarkNotificationAsRead(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Notification{}, sql.ErrNoRows)
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

			url := fmt.Sprintf("/v1/notifications/%d/read", tc.notificationID)
			request, err := http.NewRequest(http.MethodPut, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestMarkAllAsReadAPI(t *testing.T) {
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
				store.EXPECT().
					MarkAllNotificationsAsRead(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response markAllAsReadResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.True(t, response.Success)
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

			url := "/v1/notifications/read-all"
			request, err := http.NewRequest(http.MethodPut, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestDeleteNotificationAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name           string
		notificationID int64
		setupAuth      func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs     func(store *mockdb.MockStore)
		checkResponse  func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:           "OK",
			notificationID: 1,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.DeleteNotificationParams{
					ID:     1,
					UserID: user.ID,
				}

				store.EXPECT().
					DeleteNotification(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNoContent, recorder.Code)
			},
		},
		{
			name:           "NotFound",
			notificationID: 999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					DeleteNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(sql.ErrNoRows)
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

			url := fmt.Sprintf("/v1/notifications/%d", tc.notificationID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetNotificationPreferencesAPI(t *testing.T) {
	user, _ := randomUser(t)

	prefs := db.UserNotificationPreference{
		ID:                          1,
		UserID:                      user.ID,
		EnableOrderNotifications:    true,
		EnablePaymentNotifications:  true,
		EnableDeliveryNotifications: true,
		EnableSystemNotifications:   true,
		CreatedAt:                   time.Now(),
		UpdatedAt:                   pgtype.Timestamptz{Time: time.Now(), Valid: true},
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
				store.EXPECT().
					GetOrCreateUserNotificationPreferences(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(prefs, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response notificationPreferencesResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, user.ID, response.UserID)
				require.True(t, response.EnableOrderNotifications)
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

			url := "/v1/notifications/preferences"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateNotificationPreferencesAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"enable_order_notifications": false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.UpdateUserNotificationPreferencesParams{
					UserID:                   user.ID,
					EnableOrderNotifications: pgtype.Bool{Bool: false, Valid: true},
				}

				updatedPrefs := db.UserNotificationPreference{
					ID:                          1,
					UserID:                      user.ID,
					EnableOrderNotifications:    false,
					EnablePaymentNotifications:  true,
					EnableDeliveryNotifications: true,
					EnableSystemNotifications:   true,
					CreatedAt:                   time.Now(),
					UpdatedAt:                   pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}

				store.EXPECT().
					UpdateUserNotificationPreferences(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(updatedPrefs, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response notificationPreferencesResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.False(t, response.EnableOrderNotifications)
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

			url := "/v1/notifications/preferences"
			request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestListNotificationsWithFilters 测试带过滤条件的通知列表
func TestListNotificationsWithFiltersAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK_FilterByIsRead",
			query: "?is_read=true&limit=10&offset=0",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.ListUserNotificationsParams{
					UserID: user.ID,
					Limit:  10,
					Offset: 0,
					IsRead: pgtype.Bool{Bool: true, Valid: true},
				}

				store.EXPECT().
					ListUserNotifications(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return([]db.Notification{}, nil)

				countArg := db.CountUserNotificationsParams{
					UserID: user.ID,
					IsRead: pgtype.Bool{Bool: true, Valid: true},
				}
				store.EXPECT().
					CountUserNotifications(gomock.Any(), gomock.Eq(countArg)).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_FilterByType",
			query: "?type=order&limit=10&offset=0",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.ListUserNotificationsParams{
					UserID: user.ID,
					Limit:  10,
					Offset: 0,
					Type:   pgtype.Text{String: "order", Valid: true},
				}

				store.EXPECT().
					ListUserNotifications(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return([]db.Notification{}, nil)

				countArg := db.CountUserNotificationsParams{
					UserID: user.ID,
					Type:   pgtype.Text{String: "order", Valid: true},
				}
				store.EXPECT().
					CountUserNotifications(gomock.Any(), gomock.Eq(countArg)).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidType",
			query: "?type=invalid_type&limit=10&offset=0",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store，因为验证失败
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidLimit_Zero",
			query: "?limit=0&offset=0",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store，因为验证失败
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidLimit_TooLarge",
			query: "?limit=101&offset=0",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store，因为验证失败
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidOffset_Negative",
			query: "?limit=10&offset=-1",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store，因为验证失败
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InternalError_ListFailed",
			query: "?limit=10&offset=0",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserNotifications(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
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

			url := "/v1/notifications" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestMarkNotificationAsReadInvalidIDAPI 测试标记通知已读时的无效ID
func TestMarkNotificationAsReadInvalidIDAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name           string
		notificationID string
		setupAuth      func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs     func(store *mockdb.MockStore)
		checkResponse  func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:           "InvalidID_Zero",
			notificationID: "0",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:           "InvalidID_Negative",
			notificationID: "-1",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:           "InvalidID_NonNumeric",
			notificationID: "abc",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:           "NoAuthorization",
			notificationID: "1",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加认证
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store
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

			url := fmt.Sprintf("/v1/notifications/%s/read", tc.notificationID)
			request, err := http.NewRequest(http.MethodPut, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestUpdateNotificationPreferencesWithDoNotDisturbAPI 测试更新免打扰时段
func TestUpdateNotificationPreferencesWithDoNotDisturbAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_SetDoNotDisturb",
			body: gin.H{
				"do_not_disturb_start": "22:00:00",
				"do_not_disturb_end":   "08:00:00",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateUserNotificationPreferences(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UserNotificationPreference{
						ID:                          1,
						UserID:                      user.ID,
						EnableOrderNotifications:    true,
						EnablePaymentNotifications:  true,
						EnableDeliveryNotifications: true,
						EnableSystemNotifications:   true,
						DoNotDisturbStart:           pgtype.Time{Microseconds: 22 * 3600 * 1000000, Valid: true},
						DoNotDisturbEnd:             pgtype.Time{Microseconds: 8 * 3600 * 1000000, Valid: true},
						CreatedAt:                   time.Now(),
						UpdatedAt:                   pgtype.Timestamptz{Time: time.Now(), Valid: true},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response notificationPreferencesResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.NotNil(t, response.DoNotDisturbStart)
				require.NotNil(t, response.DoNotDisturbEnd)
				require.Equal(t, "22:00:00", *response.DoNotDisturbStart)
				require.Equal(t, "08:00:00", *response.DoNotDisturbEnd)
			},
		},
		{
			name: "InvalidTimeFormat_DoNotDisturbStart",
			body: gin.H{
				"do_not_disturb_start": "invalid",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store，因为验证失败
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidTimeFormat_WrongFormat",
			body: gin.H{
				"do_not_disturb_start": "22:00", // 缺少秒
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store，因为验证失败（len=8）
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidTimeFormat_NotTimeString",
			body: gin.H{
				"do_not_disturb_start": "25:00:00", // 无效小时
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store，因为time.Parse会失败
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InternalError",
			body: gin.H{
				"enable_order_notifications": false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateUserNotificationPreferences(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UserNotificationPreference{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
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

			url := "/v1/notifications/preferences"
			request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestDeleteNotificationInvalidIDAPI 测试删除通知时的无效ID
func TestDeleteNotificationInvalidIDAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name           string
		notificationID string
		setupAuth      func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs     func(store *mockdb.MockStore)
		checkResponse  func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:           "InvalidID_Zero",
			notificationID: "0",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:           "NoAuthorization",
			notificationID: "1",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加认证
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:           "InternalError",
			notificationID: "1",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					DeleteNotification(gomock.Any(), gomock.Any()).
					Times(1).
					Return(sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
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

			url := fmt.Sprintf("/v1/notifications/%s", tc.notificationID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestGetUnreadCountInternalErrorAPI 测试获取未读数量的内部错误
func TestGetUnreadCountInternalErrorAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "InternalError",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					CountUnreadNotifications(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(int64(0), sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加认证
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store
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

			url := "/v1/notifications/unread/count"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestMarkAllAsReadInternalErrorAPI 测试全部标记已读的内部错误
func TestMarkAllAsReadInternalErrorAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "InternalError",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					MarkAllNotificationsAsRead(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加认证
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store
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

			url := "/v1/notifications/read-all"
			request, err := http.NewRequest(http.MethodPut, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestGetNotificationPreferencesErrorAPI 测试获取偏好设置的错误场景
func TestGetNotificationPreferencesErrorAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "InternalError",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrCreateUserNotificationPreferences(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.UserNotificationPreference{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// 不添加认证
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用store
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

			url := "/v1/notifications/preferences"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
