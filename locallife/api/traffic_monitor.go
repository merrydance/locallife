package api

import (
	"sort"
	"strconv"
	"sync"
	"time"
)

const (
	defaultTrafficWindowSeconds = 300
	defaultTrafficRouteLimit    = 20
	defaultTrafficMaxSamples    = 50000
	trafficSummaryRoutePath     = "/v1/platform/stats/traffic/summary"
)

type trafficObservation struct {
	Method        string
	Path          string
	Status        int
	RequestBytes  int64
	ResponseBytes int64
	Duration      time.Duration
}

type trafficSample struct {
	At time.Time
	trafficObservation
}

type trafficRecorder struct {
	mu      sync.RWMutex
	maxSize int
	samples []trafficSample
	next    int
	size    int
}

type trafficSummaryResponse struct {
	GeneratedAt   time.Time             `json:"generated_at"`
	WindowSeconds int                   `json:"window_seconds"`
	RouteLimit    int                   `json:"route_limit"`
	Totals        trafficTotalsResponse `json:"totals"`
	Routes        []trafficRouteSummary `json:"routes"`
}

type trafficTotalsResponse struct {
	Requests         int64   `json:"requests"`
	RequestBytes     int64   `json:"request_bytes"`
	ResponseBytes    int64   `json:"response_bytes"`
	ErrorRequests    int64   `json:"error_requests"`
	AverageLatencyMs float64 `json:"average_latency_ms"`
}

type trafficRouteSummary struct {
	Method           string           `json:"method"`
	Path             string           `json:"path"`
	Requests         int64            `json:"requests"`
	RequestBytes     int64            `json:"request_bytes"`
	ResponseBytes    int64            `json:"response_bytes"`
	ErrorRequests    int64            `json:"error_requests"`
	AverageLatencyMs float64          `json:"average_latency_ms"`
	StatusCounts     map[string]int64 `json:"status_counts"`

	latencyTotal time.Duration
}

type trafficRouteKey struct {
	method string
	path   string
}

var globalTrafficRecorder = newTrafficRecorder(defaultTrafficMaxSamples)

func newTrafficRecorder(maxSize int) *trafficRecorder {
	if maxSize <= 0 {
		maxSize = defaultTrafficMaxSamples
	}
	return &trafficRecorder{
		maxSize: maxSize,
		samples: make([]trafficSample, maxSize),
	}
}

func (r *trafficRecorder) record(obs trafficObservation) {
	r.recordAt(time.Now(), obs)
}

func (r *trafficRecorder) recordAt(at time.Time, obs trafficObservation) {
	if r == nil {
		return
	}
	if obs.Path == "" {
		obs.Path = "not_found"
	}
	if obs.RequestBytes < 0 {
		obs.RequestBytes = 0
	}
	if obs.ResponseBytes < 0 {
		obs.ResponseBytes = 0
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.samples[r.next] = trafficSample{At: at, trafficObservation: obs}
	r.next = (r.next + 1) % r.maxSize
	if r.size < r.maxSize {
		r.size++
	}
}

func (r *trafficRecorder) summary(windowSeconds, routeLimit int) trafficSummaryResponse {
	return r.summaryAt(time.Now(), windowSeconds, routeLimit)
}

func (r *trafficRecorder) summaryAt(now time.Time, windowSeconds, routeLimit int) trafficSummaryResponse {
	if windowSeconds <= 0 {
		windowSeconds = defaultTrafficWindowSeconds
	}
	if routeLimit <= 0 {
		routeLimit = defaultTrafficRouteLimit
	}

	summary := trafficSummaryResponse{
		GeneratedAt:   now.UTC(),
		WindowSeconds: windowSeconds,
		RouteLimit:    routeLimit,
		Routes:        []trafficRouteSummary{},
	}
	if r == nil {
		return summary
	}

	cutoff := now.Add(-time.Duration(windowSeconds) * time.Second)
	routeIndex := make(map[trafficRouteKey]*trafficRouteSummary)

	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := 0; i < r.size; i++ {
		idx := r.next - r.size + i
		if idx < 0 {
			idx += r.maxSize
		}
		sample := r.samples[idx]
		if sample.At.Before(cutoff) || sample.At.After(now) {
			continue
		}

		obs := sample.trafficObservation
		summary.Totals.Requests++
		summary.Totals.RequestBytes += obs.RequestBytes
		summary.Totals.ResponseBytes += obs.ResponseBytes
		summary.Totals.AverageLatencyMs += float64(obs.Duration) / float64(time.Millisecond)
		if obs.Status >= 500 {
			summary.Totals.ErrorRequests++
		}

		key := trafficRouteKey{method: obs.Method, path: obs.Path}
		route, ok := routeIndex[key]
		if !ok {
			route = &trafficRouteSummary{
				Method:       obs.Method,
				Path:         obs.Path,
				StatusCounts: make(map[string]int64),
			}
			routeIndex[key] = route
		}

		route.Requests++
		route.RequestBytes += obs.RequestBytes
		route.ResponseBytes += obs.ResponseBytes
		route.latencyTotal += obs.Duration
		if obs.Status >= 500 {
			route.ErrorRequests++
		}
		route.StatusCounts[strconv.Itoa(obs.Status)]++
	}

	if summary.Totals.Requests > 0 {
		summary.Totals.AverageLatencyMs = summary.Totals.AverageLatencyMs / float64(summary.Totals.Requests)
	}

	routes := make([]trafficRouteSummary, 0, len(routeIndex))
	for _, route := range routeIndex {
		if route.Requests > 0 {
			route.AverageLatencyMs = float64(route.latencyTotal) / float64(time.Millisecond) / float64(route.Requests)
		}
		route.latencyTotal = 0
		routes = append(routes, *route)
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].ResponseBytes != routes[j].ResponseBytes {
			return routes[i].ResponseBytes > routes[j].ResponseBytes
		}
		if routes[i].Requests != routes[j].Requests {
			return routes[i].Requests > routes[j].Requests
		}
		if routes[i].Method != routes[j].Method {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})

	if len(routes) > routeLimit {
		routes = routes[:routeLimit]
	}
	summary.Routes = routes
	return summary
}
