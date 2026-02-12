package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
)

// RegionAccessError captures region mismatch details for auditing.
type RegionAccessError struct {
	RequestErr       *RequestError
	MerchantRegionID int64
	RiderRegionID    int64
	OrderID          int64
}

func (e *RegionAccessError) Error() string {
	if e == nil || e.RequestErr == nil {
		return "region access denied"
	}
	return e.RequestErr.Error()
}

func (e *RegionAccessError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.RequestErr
}

// GrabOrderInput defines the grab order input.
type GrabOrderInput struct {
	UserID            int64
	OrderID           int64
	MaxDistanceMeters int
}

// GrabOrderResult contains data for follow-up actions after grabbing.
type GrabOrderResult struct {
	Delivery       db.Delivery
	Order          db.Order
	Merchant       db.Merchant
	Rider          db.Rider
	PreviousStatus string
	FreezeAmount   int64
}

// GrabDeliveryOrder validates and executes the grab order flow.
func GrabDeliveryOrder(ctx context.Context, store db.Store, input GrabOrderInput) (GrabOrderResult, error) {
	var result GrabOrderResult

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
		return result, NewRequestError(http.StatusBadRequest, errors.New("您尚未分配服务区域，请联系管理员"))
	}

	availableDeposit := rider.DepositAmount - rider.FrozenDeposit

	poolItem, err := store.GetDeliveryPoolByOrderID(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("订单不存在或已被接走"))
		}
		return result, err
	}
	if poolItem.ExpiresAt.Before(time.Now()) {
		return result, NewRequestError(http.StatusBadRequest, errors.New("订单已过期"))
	}

	highValueThreshold := int64(1000)
	isHighValueOrder := poolItem.DeliveryFee >= highValueThreshold
	premiumScore, err := store.GetRiderPremiumScore(ctx, rider.ID)
	if err != nil {
		if !errors.Is(err, db.ErrRecordNotFound) {
			return result, err
		}
		premiumScore = 0
	}
	if isHighValueOrder && premiumScore < 0 {
		return result, NewRequestError(http.StatusForbidden, errors.New("高值单资格积分不足，无法接取高值单（运费≥10元），请先完成普通订单积累积分"))
	}

	merchant, err := store.GetMerchant(ctx, poolItem.MerchantID)
	if err != nil {
		return result, err
	}
	if merchant.RegionID != rider.RegionID.Int64 {
		return result, &RegionAccessError{
			RequestErr:       &RequestError{Status: http.StatusForbidden, Err: errors.New("该订单不在您的服务区域内")},
			MerchantRegionID: merchant.RegionID,
			RiderRegionID:    rider.RegionID.Int64,
			OrderID:          input.OrderID,
		}
	}

	riderLng, riderLngOk := floatFromNumeric(rider.CurrentLongitude)
	riderLat, riderLatOk := floatFromNumeric(rider.CurrentLatitude)
	merchantLng, merchantLngOk := floatFromNumeric(merchant.Longitude)
	merchantLat, merchantLatOk := floatFromNumeric(merchant.Latitude)
	if riderLngOk && riderLatOk && merchantLngOk && merchantLatOk {
		riderLoc := algorithm.Location{Longitude: riderLng, Latitude: riderLat}
		merchantLoc := algorithm.Location{Longitude: merchantLng, Latitude: merchantLat}
		distance := algorithm.HaversineDistance(riderLoc, merchantLoc)
		if input.MaxDistanceMeters > 0 && distance > input.MaxDistanceMeters {
			return result, NewRequestError(http.StatusBadRequest, fmt.Errorf(
				"您距离商户太远（%.1f公里），请靠近后再抢单",
				float64(distance)/1000,
			))
		}
	}

	delivery, err := store.GetDeliveryByOrderID(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("该订单配置异常(缺少配送单信息)，无法抢单。如果是Mock数据，请确保deliveries表有相应记录"))
		}
		return result, err
	}

	order, err := store.GetOrder(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("订单不存在"))
		}
		return result, err
	}
	oldStatus := order.Status
	if !IsOrderStatusAllowedForDeliveryAction(order.Status, "grab") {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("当前订单状态(%s)不允许接单", order.Status))
	}

	freezeAmount := OrderFreezeAmount(order)
	if freezeAmount > 0 && availableDeposit < freezeAmount {
		return result, NewRequestError(http.StatusBadRequest, errors.New("押金余额不足，无法接单"))
	}

	txResult, err := store.GrabOrderTx(ctx, db.GrabOrderTxParams{
		DeliveryID:   delivery.ID,
		RiderID:      rider.ID,
		OrderID:      input.OrderID,
		FreezeAmount: freezeAmount,
	})
	if err != nil {
		return result, err
	}

	if _, err := store.UpdateOrderToCourierAccepted(ctx, input.OrderID); err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return result, err
	}
	if oldStatus != "courier_accepted" {
		_, _ = store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: oldStatus, Valid: true},
			ToStatus:     "courier_accepted",
			OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "rider", Valid: true},
			Notes:        pgtype.Text{String: "骑手接单", Valid: true},
		})
	}
	order.Status = "courier_accepted"

	result = GrabOrderResult{
		Delivery:       txResult.Delivery,
		Order:          order,
		Merchant:       merchant,
		Rider:          rider,
		PreviousStatus: oldStatus,
		FreezeAmount:   freezeAmount,
	}

	return result, nil
}
