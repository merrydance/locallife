package maps

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// FallbackMapClient 实现多级 LBS 容灾兜底逻辑
type FallbackMapClient struct {
	providers []TencentMapClientInterface
}

// NewFallbackMapClient 创建一个带兜底能力的地图客户端
func NewFallbackMapClient(providers ...TencentMapClientInterface) *FallbackMapClient {
	activeProviders := make([]TencentMapClientInterface, 0)
	for _, p := range providers {
		if p != nil {
			activeProviders = append(activeProviders, p)
		}
	}
	return &FallbackMapClient{
		providers: activeProviders,
	}
}

func (c *FallbackMapClient) GetBicyclingRoute(ctx context.Context, from, to Location) (*RouteResult, error) {
	var lastErr error
	for _, p := range c.providers {
		res, err := p.GetBicyclingRoute(ctx, from, to)
		if err == nil {
			return res, nil
		}
		lastErr = err
		log.Warn().Err(err).Msg("fallback map client: provider failed, trying next")
	}
	return nil, fmt.Errorf("all map providers failed: %w", lastErr)
}

func (c *FallbackMapClient) GetWalkingRoute(ctx context.Context, from, to Location) (*RouteResult, error) {
	var lastErr error
	for _, p := range c.providers {
		res, err := p.GetWalkingRoute(ctx, from, to)
		if err == nil {
			return res, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("all map providers failed: %w", lastErr)
}

func (c *FallbackMapClient) GetDrivingRoute(ctx context.Context, from, to Location) (*RouteResult, error) {
	var lastErr error
	for _, p := range c.providers {
		res, err := p.GetDrivingRoute(ctx, from, to)
		if err == nil {
			return res, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("all map providers failed: %w", lastErr)
}

func (c *FallbackMapClient) GetDistanceMatrix(ctx context.Context, froms, tos []Location, mode string) (*DistanceMatrixResult, error) {
	var lastErr error
	for i, p := range c.providers {
		res, err := p.GetDistanceMatrix(ctx, froms, tos, mode)
		if err == nil {
			if i > 0 {
				log.Info().Int("provider_index", i).Msg("✅ Fallback LBS provider recovered the request")
			}
			return res, nil
		}
		lastErr = err
		log.Warn().Err(err).Int("provider_index", i).Msg("LBS provider failed, attempting fallback")
	}
	return nil, fmt.Errorf("all LBS providers failed distance matrix: %w", lastErr)
}

func (c *FallbackMapClient) Geocode(ctx context.Context, address string) (*GeocodeResult, error) {
	var lastErr error
	for _, p := range c.providers {
		res, err := p.Geocode(ctx, address)
		if err == nil {
			return res, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("all map providers failed geocode: %w", lastErr)
}

func (c *FallbackMapClient) ReverseGeocode(ctx context.Context, location Location) (*ReverseGeocodeResult, error) {
	var lastErr error
	for _, p := range c.providers {
		res, err := p.ReverseGeocode(ctx, location)
		if err == nil {
			return res, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("all map providers failed reverse geocode: %w", lastErr)
}

var _ TencentMapClientInterface = (*FallbackMapClient)(nil)
