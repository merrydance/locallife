package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

// RecommendDeliveryForUserInput defines request data to recommend delivery orders.
type RecommendDeliveryForUserInput struct {
	UserID   int64
	RiderLat float64
	RiderLng float64
}

// RecommendDeliveryForUserResult carries rider info and recommendations.
type RecommendDeliveryForUserResult struct {
	Rider           db.Rider
	Recommendations RecommendDeliveryResult
}

// RecommendDeliveryOrdersForUser loads rider info and returns delivery recommendations.
func RecommendDeliveryOrdersForUser(
	ctx context.Context,
	store db.Store,
	routeService *RouteService,
	input RecommendDeliveryForUserInput,
) (RecommendDeliveryForUserResult, error) {
	var result RecommendDeliveryForUserResult

	rider, err := store.GetRiderByUserID(ctx, input.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("您还不是骑手"))
		}
		return result, err
	}
	if !rider.IsOnline {
		return result, NewRequestError(http.StatusBadRequest, errors.New("请先上线"))
	}
	if !rider.RegionID.Valid {
		return result, NewRequestError(http.StatusBadRequest, ErrRiderRegionUnassigned)
	}

	recommendations, err := RecommendDeliveryOrders(ctx, store, routeService, RecommendDeliveryInput{
		RiderID:       rider.ID,
		RiderRegionID: rider.RegionID.Int64,
		RiderLat:      input.RiderLat,
		RiderLng:      input.RiderLng,
	})
	if err != nil {
		return result, err
	}

	result.Rider = rider
	result.Recommendations = recommendations
	return result, nil
}
