package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrafficRecorderSummaryAggregatesWindowByRoute(t *testing.T) {
	recorder := newTrafficRecorder(10)
	base := time.Unix(1000, 0)

	recorder.recordAt(base.Add(-20*time.Second), trafficObservation{
		Method:        "GET",
		Path:          "/v1/orders",
		Status:        200,
		RequestBytes:  99,
		ResponseBytes: 999,
		Duration:      50 * time.Millisecond,
	})
	recorder.recordAt(base, trafficObservation{
		Method:        "GET",
		Path:          "/v1/orders",
		Status:        200,
		RequestBytes:  10,
		ResponseBytes: 100,
		Duration:      100 * time.Millisecond,
	})
	recorder.recordAt(base.Add(time.Second), trafficObservation{
		Method:        "GET",
		Path:          "/v1/orders",
		Status:        500,
		RequestBytes:  5,
		ResponseBytes: 50,
		Duration:      200 * time.Millisecond,
	})
	recorder.recordAt(base.Add(time.Second), trafficObservation{
		Method:        "POST",
		Path:          "/v1/media/complete",
		Status:        200,
		RequestBytes:  40,
		ResponseBytes: 25,
		Duration:      20 * time.Millisecond,
	})

	summary := recorder.summaryAt(base.Add(2*time.Second), 10, 10)

	require.Equal(t, int64(3), summary.Totals.Requests)
	require.Equal(t, int64(55), summary.Totals.RequestBytes)
	require.Equal(t, int64(175), summary.Totals.ResponseBytes)
	require.Equal(t, int64(1), summary.Totals.ErrorRequests)
	require.Len(t, summary.Routes, 2)

	orders := summary.Routes[0]
	require.Equal(t, "GET", orders.Method)
	require.Equal(t, "/v1/orders", orders.Path)
	require.Equal(t, int64(2), orders.Requests)
	require.Equal(t, int64(15), orders.RequestBytes)
	require.Equal(t, int64(150), orders.ResponseBytes)
	require.Equal(t, int64(1), orders.ErrorRequests)
	require.Equal(t, float64(150), orders.AverageLatencyMs)
	require.Equal(t, map[string]int64{"200": 1, "500": 1}, orders.StatusCounts)

	media := summary.Routes[1]
	require.Equal(t, "POST", media.Method)
	require.Equal(t, "/v1/media/complete", media.Path)
	require.Equal(t, int64(25), media.ResponseBytes)
	require.Equal(t, int64(0), media.ErrorRequests)
}

func TestTrafficRecorderSummaryKeepsNewestSamplesWhenBufferWraps(t *testing.T) {
	recorder := newTrafficRecorder(2)
	base := time.Unix(2000, 0)

	recorder.recordAt(base.Add(-2*time.Second), trafficObservation{
		Method:        "GET",
		Path:          "/v1/old",
		Status:        200,
		RequestBytes:  1,
		ResponseBytes: 1,
		Duration:      10 * time.Millisecond,
	})
	recorder.recordAt(base.Add(-1*time.Second), trafficObservation{
		Method:        "GET",
		Path:          "/v1/mid",
		Status:        200,
		RequestBytes:  2,
		ResponseBytes: 2,
		Duration:      20 * time.Millisecond,
	})
	recorder.recordAt(base, trafficObservation{
		Method:        "GET",
		Path:          "/v1/new",
		Status:        500,
		RequestBytes:  3,
		ResponseBytes: 3,
		Duration:      30 * time.Millisecond,
	})

	summary := recorder.summaryAt(base.Add(time.Second), 10, 10)

	require.Equal(t, int64(2), summary.Totals.Requests)
	require.Equal(t, int64(5), summary.Totals.RequestBytes)
	require.Len(t, summary.Routes, 2)
	require.Equal(t, "/v1/mid", summary.Routes[1].Path)
	require.Equal(t, "/v1/new", summary.Routes[0].Path)
	require.Equal(t, int64(1), summary.Totals.ErrorRequests)
}
