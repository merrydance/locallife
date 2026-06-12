package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetPlatformTrafficSummaryAPI(t *testing.T) {
	admin, _ := randomUser(t)
	now := time.Now().UTC()
	originalRecorder := globalTrafficRecorder
	globalTrafficRecorder = newTrafficRecorder(16)
	defer func() {
		globalTrafficRecorder = originalRecorder
	}()

	globalTrafficRecorder.recordAt(now.Add(-10*time.Second), trafficObservation{
		Method:        http.MethodGet,
		Path:          "/v1/orders",
		Status:        http.StatusOK,
		RequestBytes:  123,
		ResponseBytes: 456,
		Duration:      80 * time.Millisecond,
	})
	globalTrafficRecorder.recordAt(now.Add(-5*time.Second), trafficObservation{
		Method:        http.MethodGet,
		Path:          "/v1/orders",
		Status:        http.StatusInternalServerError,
		RequestBytes:  10,
		ResponseBytes: 12,
		Duration:      120 * time.Millisecond,
	})
	globalTrafficRecorder.recordAt(now.Add(-3*time.Second), trafficObservation{
		Method:        http.MethodPost,
		Path:          "/v1/media/upload",
		Status:        http.StatusOK,
		RequestBytes:  2048,
		ResponseBytes: 1024,
		Duration:      45 * time.Millisecond,
	})

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{
			UserID: admin.ID,
			Role:   RoleAdmin,
			Status: "active",
		}}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/platform/stats/traffic/summary?window_seconds=60&route_limit=10", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp trafficSummaryResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, 60, resp.WindowSeconds)
	require.Equal(t, 10, resp.RouteLimit)
	require.Equal(t, int64(3), resp.Totals.Requests)
	require.Equal(t, int64(2181), resp.Totals.RequestBytes)
	require.Equal(t, int64(1492), resp.Totals.ResponseBytes)
	require.Equal(t, int64(1), resp.Totals.ErrorRequests)
	require.Len(t, resp.Routes, 2)
	require.Equal(t, "/v1/media/upload", resp.Routes[0].Path)
	require.Equal(t, int64(1), resp.Routes[0].Requests)
	require.Equal(t, map[string]int64{"200": 1}, resp.Routes[0].StatusCounts)
	require.Equal(t, "/v1/orders", resp.Routes[1].Path)
	require.Equal(t, int64(2), resp.Routes[1].Requests)
	require.Equal(t, map[string]int64{"200": 1, "500": 1}, resp.Routes[1].StatusCounts)
}

func TestTrafficSummaryPathWithWindow(t *testing.T) {
	require.Equal(t, "/v1/platform/stats/traffic/summary?window_seconds=300&route_limit=20", trafficSummaryPathWithWindow(300, 20))
}

func trafficSummaryPathWithWindow(windowSeconds int, routeLimit int) string {
	return "/v1/platform/stats/traffic/summary?window_seconds=" + strconv.Itoa(windowSeconds) + "&route_limit=" + strconv.Itoa(routeLimit)
}
