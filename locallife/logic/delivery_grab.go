package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
)

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

func newGrabOrderStatusError(status string) error {
	switch status {
	case db.OrderStatusPending:
		return NewRequestError(http.StatusBadRequest, errors.New("订单尚未支付完成，暂不可抢单"))
	case db.OrderStatusPaid:
		return NewRequestError(http.StatusBadRequest, errors.New("商户未接单，暂不可抢单"))
	case db.OrderStatusPreparing:
		return NewRequestError(http.StatusBadRequest, errors.New("商户未出餐，暂不可抢单"))
	case db.OrderStatusCourierAccepted, db.OrderStatusPicked, db.OrderStatusDelivering,
		db.OrderStatusRiderDelivered, db.OrderStatusUserDelivered, db.OrderStatusCompleted:
		return NewRequestError(http.StatusBadRequest, errors.New("订单已被接走或已进入配送流程"))
	case db.OrderStatusCancelled:
		return NewRequestError(http.StatusBadRequest, errors.New("订单已取消，无法抢单"))
	default:
		return NewRequestError(http.StatusBadRequest, fmt.Errorf("当前订单状态(%s)不允许接单", status))
	}
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
	if rider.Status != db.RiderStatusActive {
		return result, NewRequestError(http.StatusBadRequest, errors.New("押金不足或账号未激活，暂不可接单"))
	}
	suspension, err := GetRiderSuspension(ctx, store, rider.ID)
	if err != nil {
		return result, err
	}
	if suspension != nil {
		return result, NewRequestError(http.StatusForbidden, errors.New("骑手接单已暂停"))
	}
	settlementReadiness, err := riderBaofuSettlementReadiness(ctx, store, rider)
	if err != nil {
		return result, err
	}
	if !settlementReadiness.PaymentReady {
		return result, NewRequestError(http.StatusBadRequest, errors.New("骑手结算账户未开通，暂不能接收配送费分账订单"))
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

	merchant, err := store.GetMerchant(ctx, poolItem.MerchantID)
	if err != nil {
		return result, err
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
		return result, newGrabOrderStatusError(order.Status)
	}

	freezeAmount := OrderFreezeAmount(order)
	if freezeAmount > 0 && availableDeposit < freezeAmount {
		return result, NewRequestError(http.StatusBadRequest, errors.New("押金余额不足，无法接单"))
	}

	txResult, err := store.GrabOrderTx(ctx, db.GrabOrderTxParams{
		DeliveryID:   delivery.ID,
		RiderID:      rider.ID,
		RiderUserID:  rider.UserID,
		OrderID:      input.OrderID,
		FreezeAmount: freezeAmount,
	})
	if err != nil {
		if errors.Is(err, db.ErrTakeoutOrderPausedByFoodSafety) {
			return result, NewRequestError(http.StatusForbidden, errors.New("该外卖订单因食安事件已暂停履约，请等待平台处理"))
		}
		return result, err
	}
	order = txResult.Order

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

func riderBaofuSettlementReadiness(ctx context.Context, store db.Store, rider db.Rider) (BaofuAccountReadiness, error) {
	service := NewBaofuAccountService(nil, nil)
	binding, err := store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   rider.ID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return service.ReadinessFromBinding(db.BaofuAccountBinding{}, false, false), nil
		}
		return BaofuAccountReadiness{}, err
	}
	return service.ReadinessFromBinding(binding, true, false), nil
}
