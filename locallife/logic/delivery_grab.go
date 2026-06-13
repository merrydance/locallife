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
		return NewRequestError(http.StatusBadRequest, errors.New("订单已被接走或已进入代取流程"))
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
		return result, NewRequestError(http.StatusBadRequest, errors.New("骑手结算账户未开通，暂不能接收代取费分账订单"))
	}
	riderSharingMerID := settlementReadiness.SubMchID

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
			return result, NewRequestError(http.StatusNotFound, errors.New("该订单配置异常(缺少代取单信息)，无法抢单。如果是Mock数据，请确保deliveries表有相应记录"))
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
	riderBill, err := buildBaofuRiderProfitSharingBillUpdate(ctx, store, order, rider, riderSharingMerID)
	if err != nil {
		return result, err
	}

	txResult, err := store.GrabOrderTx(ctx, db.GrabOrderTxParams{
		DeliveryID:             delivery.ID,
		RiderID:                rider.ID,
		RiderUserID:            rider.UserID,
		OrderID:                input.OrderID,
		FreezeAmount:           freezeAmount,
		ProfitSharingRiderBill: riderBill,
	})
	if err != nil {
		if errors.Is(err, db.ErrTakeoutOrderPausedByFoodSafety) {
			return result, NewRequestError(http.StatusForbidden, errors.New("该外卖订单因食安事件已暂停履约，请等待平台处理"))
		}
		if errors.Is(err, db.ErrBaofuProfitSharingBillNotPending) {
			return result, NewRequestError(http.StatusConflict, errors.New("订单结算已进入处理，不能重新接单"))
		}
		if errors.Is(err, db.ErrRiderDeliveryEligibilityChanged) {
			return result, NewRequestError(http.StatusConflict, db.ErrRiderDeliveryEligibilityChanged)
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

func buildBaofuRiderProfitSharingBillUpdate(ctx context.Context, store db.Store, order db.Order, rider db.Rider, riderSharingMerID string) (*db.UpdateProfitSharingOrderRiderBillByPaymentOrderParams, error) {
	if order.OrderType != db.OrderTypeTakeout || order.DeliveryFee <= 0 {
		return nil, nil
	}
	paymentOrder, err := store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType: businessTypeOrder,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, NewRequestError(http.StatusConflict, errors.New("订单代取收益账单暂不可用，请稍后重试"))
		}
		return nil, fmt.Errorf("get latest payment order for rider profit sharing bill: %w", err)
	}
	if !db.PaymentOrderRequiresProfitSharing(paymentOrder) {
		return nil, nil
	}
	if paymentOrder.Status != paymentStatusPaid {
		return nil, NewRequestError(http.StatusConflict, errors.New("订单代取收益账单暂不可用，请稍后重试"))
	}
	if paymentOrder.OrderID.Valid && paymentOrder.OrderID.Int64 != order.ID {
		return nil, fmt.Errorf("payment order %d order id %d does not match grabbed order %d", paymentOrder.ID, paymentOrder.OrderID.Int64, order.ID)
	}
	bill, err := store.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, NewRequestError(http.StatusConflict, errors.New("订单代取收益账单暂不可用，请稍后重试"))
		}
		return nil, fmt.Errorf("get baofu profit sharing bill for rider accept: %w", err)
	}
	if bill.Status != db.ProfitSharingOrderStatusPending {
		return nil, NewRequestError(http.StatusConflict, errors.New("订单结算已进入处理，不能重新接单"))
	}
	if bill.Provider != db.ExternalPaymentProviderBaofu || bill.Channel != db.PaymentChannelBaofuAggregate {
		return nil, fmt.Errorf("profit sharing bill %d provider/channel is not baofu aggregate", bill.ID)
	}
	if bill.MerchantID != order.MerchantID || bill.PaymentOrderID != paymentOrder.ID || bill.TotalAmount != paymentOrder.Amount {
		return nil, NewRequestError(http.StatusConflict, errors.New("订单代取收益账单暂不可用，请稍后重试"))
	}
	if riderSharingMerID == "" {
		return nil, NewRequestError(http.StatusBadRequest, errors.New("骑手结算账户未开通，暂不能接收代取费分账订单"))
	}

	receivers := BaofuProfitSharingReceiverResult{
		MerchantSharingMerID: textValue(bill.MerchantSharingMerID),
		RiderSharingMerID:    riderSharingMerID,
		OperatorSharingMerID: textValue(bill.OperatorSharingMerID),
		PlatformSharingMerID: textValue(bill.PlatformSharingMerID),
	}
	amounts, err := CalculateBaofuSettlementAmounts(BaofuSettlementCalculationInput{
		OrderScene:                 bill.OrderSource,
		TotalAmountFen:             bill.TotalAmount,
		DeliveryFeeFen:             bill.DeliveryFee,
		ProviderPaymentFeeFen:      bill.ProviderPaymentFee,
		PlatformCommissionRateBps:  bill.PlatformRate,
		OperatorCommissionRateBps:  bill.OperatorRate,
		MerchantPaymentFeeRateBps:  bill.MerchantPaymentFeeRateBps,
		RiderPaymentFeeRateBps:     DefaultBaofuPaymentServiceFeeRateBps,
		HasRiderReceiver:           true,
		HasOperatorReceiver:        bill.OperatorID.Valid,
		RedirectMissingOperatorFee: true,
	})
	if err != nil {
		return nil, fmt.Errorf("calculate baofu rider profit sharing bill: %w", err)
	}
	snapshot, err := buildBaofuSharingDetailSnapshot(amounts, receivers)
	if err != nil {
		return nil, fmt.Errorf("build baofu rider profit sharing snapshot: %w", err)
	}
	return &db.UpdateProfitSharingOrderRiderBillByPaymentOrderParams{
		PaymentOrderID:               paymentOrder.ID,
		RiderID:                      pgtype.Int8{Int64: rider.ID, Valid: true},
		RiderSharingMerID:            pgtype.Text{String: riderSharingMerID, Valid: riderSharingMerID != ""},
		RiderAmount:                  amounts.RiderAmountFen,
		DistributableAmount:          amounts.MerchantPaymentFeeBaseFen,
		PlatformCommission:           amounts.PlatformCommissionFen,
		OperatorCommission:           amounts.OperatorCommissionFen,
		MerchantAmount:               amounts.MerchantAmountFen,
		SharingDetailSnapshot:        snapshot,
		RiderGrossAmount:             amounts.RiderGrossAmountFen,
		RiderPaymentFee:              amounts.RiderPaymentFeeFen,
		RiderPaymentFeeRateBps:       amounts.RiderPaymentFeeRateBps,
		RiderPaymentFeeBaseAmount:    amounts.RiderPaymentFeeBaseFen,
		MerchantPaymentFee:           amounts.MerchantPaymentFeeFen,
		MerchantPaymentFeeBaseAmount: amounts.MerchantPaymentFeeBaseFen,
		CommissionBaseAmount:         amounts.CommissionBaseFen,
		PlatformReceiverAmount:       amounts.PlatformReceiverAmountFen,
	}, nil
}

type riderBaofuBindingReader interface {
	GetBaofuAccountBindingByOwner(ctx context.Context, arg db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error)
}

func riderBaofuSettlementReadiness(ctx context.Context, store riderBaofuBindingReader, rider db.Rider) (BaofuAccountReadiness, error) {
	service := NewBaofuAccountService(nil, nil)
	binding, err := store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   rider.ID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return service.ReadinessFromBinding(db.BaofuAccountBinding{}, false), nil
		}
		return BaofuAccountReadiness{}, err
	}
	readiness := service.ReadinessFromBinding(binding, true)
	readiness.SubMchID = textValue(binding.SharingMerID)
	return readiness, nil
}

func textValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
