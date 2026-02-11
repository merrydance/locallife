package logic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/merrydance/locallife/algorithm"
	"github.com/merrydance/locallife/maps"
	"github.com/rs/zerolog/log"
)

type RouteCacheItem struct {
	Result    *maps.RouteResult
	ExpiresAt time.Time
}

type RouteService struct {
	mapClient maps.TencentMapClientInterface
	cache     sync.Map // key: "lat1,lng1-lat2,lng2" -> *RouteCacheItem
}

func NewRouteService(mapClient maps.TencentMapClientInterface) *RouteService {
	s := &RouteService{
		mapClient: mapClient,
	}
	// Start cleanup loop
	go s.cleanupLoop()
	return s
}

func (s *RouteService) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		s.cache.Range(func(key, value interface{}) bool {
			item := value.(*RouteCacheItem)
			if time.Now().After(item.ExpiresAt) {
				s.cache.Delete(key)
			}
			return true
		})
	}
}

// Round coordinates to ~100m precision (3 decimal places) to improve cache hit rate
// while maintaining sufficient accuracy for delivery estimation.
func roundCoord(val float64) string {
	return fmt.Sprintf("%.3f", val)
}

func cacheKey(from, to maps.Location) string {
	return fmt.Sprintf("%s,%s-%s,%s", roundCoord(from.Lat), roundCoord(from.Lng), roundCoord(to.Lat), roundCoord(to.Lng))
}

// GetBicyclingRoute gets the cycling route with caching
func (s *RouteService) GetBicyclingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	if s.mapClient == nil {
		return nil, fmt.Errorf("map client not initialized")
	}

	key := cacheKey(from, to)
	if val, ok := s.cache.Load(key); ok {
		item := val.(*RouteCacheItem)
		if time.Now().Before(item.ExpiresAt) {
			return item.Result, nil
		}
		s.cache.Delete(key)
	}

	// Call external service
	route, err := s.mapClient.GetBicyclingRoute(ctx, from, to)
	if err != nil {
		return nil, err
	}

	// Store in cache (TTL 30 minutes)
	s.cache.Store(key, &RouteCacheItem{
		Result:    route,
		ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	return route, nil
}

// EnrichOrders populates RealDistance for orders using cached routes.
// Safe to call with mapClient == nil (returns empty map).
func (s *RouteService) EnrichOrders(ctx context.Context, riderLat, riderLng float64, scored []algorithm.ScoredOrder) map[int64]*maps.RouteResult {
	result := make(map[int64]*maps.RouteResult)
	if s.mapClient == nil {
		return result
	}

	// Only enrich top N results to save resources
	maxOrders := 10
	if len(scored) < maxOrders {
		maxOrders = len(scored)
	}

	riderLoc := maps.Location{Lat: riderLat, Lng: riderLng}

	for i := 0; i < maxOrders; i++ {
		order := scored[i]
		pickupLoc := maps.Location{
			Lat: order.PoolOrder.PickupLocation.Latitude,
			Lng: order.PoolOrder.PickupLocation.Longitude,
		}

		route, err := s.GetBicyclingRoute(ctx, riderLoc, pickupLoc)
		if err != nil {
			log.Warn().Err(err).Int64("order_id", order.OrderID).Msg("failed to get route")
			continue
		}
		result[order.OrderID] = route
	}
	return result
}
