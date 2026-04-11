package logic

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
)

// RecommendDeliveryInput defines rider and location data for recommendations.
type RecommendDeliveryInput struct {
	RiderID  int64
	RiderLat float64
	RiderLng float64
}

// RecommendDeliveryResult contains scored orders and enriched routes.
type RecommendDeliveryResult struct {
	Scored        []algorithm.ScoredOrder
	RealDistances map[int64]*maps.RouteResult
}

// RecommendDeliveryOrders builds recommended orders for a rider.
func RecommendDeliveryOrders(
	ctx context.Context,
	store db.Store,
	routeService *RouteService,
	input RecommendDeliveryInput,
) (RecommendDeliveryResult, error) {
	result := RecommendDeliveryResult{
		RealDistances: map[int64]*maps.RouteResult{},
	}

	config := algorithm.DefaultConfig()
	if dbConfig, err := store.GetActiveRecommendConfig(ctx); err == nil {
		dw, _ := dbConfig.DistanceWeight.Float64Value()
		rw, _ := dbConfig.RouteWeight.Float64Value()
		uw, _ := dbConfig.UrgencyWeight.Float64Value()
		pw, _ := dbConfig.ProfitWeight.Float64Value()
		config.DistanceWeight = dw.Float64
		config.RouteWeight = rw.Float64
		config.UrgencyWeight = uw.Float64
		config.ProfitWeight = pw.Float64
		config.MaxDistance = int(dbConfig.MaxDistance)
		config.MaxResults = int(dbConfig.MaxResults)
	}

	poolItems, err := store.ListDeliveryPoolNearby(ctx, db.ListDeliveryPoolNearbyParams{
		RiderLat:    input.RiderLat,
		RiderLng:    input.RiderLng,
		MaxDistance: float64(config.MaxDistance),
		ResultLimit: 100,
	})
	if err != nil {
		return result, err
	}

	availablePool := make([]algorithm.PoolOrder, 0, len(poolItems))
	for _, item := range poolItems {
		pickupLng, _ := item.PickupLongitude.Float64Value()
		pickupLat, _ := item.PickupLatitude.Float64Value()
		deliveryLng, _ := item.DeliveryLongitude.Float64Value()
		deliveryLat, _ := item.DeliveryLatitude.Float64Value()

		availablePool = append(availablePool, algorithm.PoolOrder{
			OrderID:    item.OrderID,
			MerchantID: item.MerchantID,
			PickupLocation: algorithm.Location{
				Longitude: pickupLng.Float64,
				Latitude:  pickupLat.Float64,
			},
			DeliveryLocation: algorithm.Location{
				Longitude: deliveryLng.Float64,
				Latitude:  deliveryLat.Float64,
			},
			Distance:           int(item.Distance),
			DeliveryFee:        item.DeliveryFee,
			ExpectedPickupAt:   item.ExpectedPickupAt,
			ExpectedDeliveryAt: item.ExpectedDeliveryAt.Time,
			ExpiresAt:          item.ExpiresAt,
			Priority:           int(item.Priority),
			CreatedAt:          item.CreatedAt,
		})
	}

	var activeOrders []algorithm.ActiveDelivery
	activeDeliveries, err := store.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: input.RiderID, Valid: true})
	if err == nil {
		for _, delivery := range activeDeliveries {
			pickupLng, _ := delivery.PickupLongitude.Float64Value()
			pickupLat, _ := delivery.PickupLatitude.Float64Value()
			deliveryLng, _ := delivery.DeliveryLongitude.Float64Value()
			deliveryLat, _ := delivery.DeliveryLatitude.Float64Value()

			ad := algorithm.ActiveDelivery{
				DeliveryID: delivery.ID,
				OrderID:    delivery.OrderID,
				PickupLocation: algorithm.Location{
					Longitude: pickupLng.Float64,
					Latitude:  pickupLat.Float64,
				},
				DeliveryLocation: algorithm.Location{
					Longitude: deliveryLng.Float64,
					Latitude:  deliveryLat.Float64,
				},
				Status: delivery.Status,
			}
			if delivery.PickedAt.Valid {
				ad.PickedAt = delivery.PickedAt.Time
			}
			activeOrders = append(activeOrders, ad)
		}
	}

	recommender := algorithm.NewSimpleRecommender()
	recommendInput := algorithm.RecommendInput{
		RiderID: input.RiderID,
		RiderLocation: algorithm.Location{
			Longitude: input.RiderLng,
			Latitude:  input.RiderLat,
		},
		ActiveOrders:  activeOrders,
		AvailablePool: availablePool,
		Config:        config,
	}

	scored, err := recommender.Recommend(ctx, recommendInput)
	if err != nil {
		return result, err
	}
	result.Scored = scored

	if routeService != nil {
		result.RealDistances = routeService.EnrichOrders(ctx, input.RiderLat, input.RiderLng, scored)
	}

	return result, nil
}
