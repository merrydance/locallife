package maps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTencentMapClientGetRouteReturnsDecodedPolylinePoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/ws/direction/v1/ebicycling/", r.URL.Path)
		require.Equal(t, "39.908722,116.397499", r.URL.Query().Get("from"))
		require.Equal(t, "39.914722,116.404499", r.URL.Query().Get("to"))
		require.Equal(t, "test-key", r.URL.Query().Get("key"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status": 0,
			"message": "query ok",
			"result": {
				"routes": [{
					"distance": 26,
					"duration": 7,
					"polyline": [
						23.020059,
						113.489437,
						-101,
						-219,
						0,
						0,
						-4,
						-11
					]
				}]
			}
		}`))
	}))
	defer server.Close()

	client := NewTencentMapClient("test-key")
	client.baseURL = server.URL
	route, err := client.GetBicyclingRoute(
		context.Background(),
		Location{Lat: 39.908722, Lng: 116.397499},
		Location{Lat: 39.914722, Lng: 116.404499},
	)

	require.NoError(t, err)
	require.Equal(t, 26, route.Distance)
	require.Equal(t, 420, route.Duration)
	require.Equal(t, []Location{
		{Lat: 23.020059, Lng: 113.489437},
		{Lat: 23.019958, Lng: 113.489218},
		{Lat: 23.019958, Lng: 113.489218},
		{Lat: 23.019954, Lng: 113.489207},
	}, route.Points)
}
