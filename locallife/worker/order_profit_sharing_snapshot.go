package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

const wechatProfitSharingPaymentFeeRateBps = int32(30)

func estimatedWechatProfitSharingPaymentFee(amount int64) int64 {
	if amount <= 0 {
		return 0
	}
	return (amount*int64(wechatProfitSharingPaymentFeeRateBps) + 9999) / 10000
}

func (processor *RedisTaskProcessor) ensureOrderProfitSharingSnapshot(ctx context.Context, paymentOrder db.PaymentOrder, order db.Order) error {
	_, err := processor.store.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, db.ErrRecordNotFound) {
		return fmt.Errorf("get profit sharing order by payment order: %w", err)
	}

	if paymentOrder.Amount <= 0 {
		return fmt.Errorf("payment order %d amount is required for profit sharing snapshot", paymentOrder.ID)
	}

	merchant, err := processor.store.GetMerchant(ctx, order.MerchantID)
	if err != nil {
		return fmt.Errorf("get merchant: %w", err)
	}

	_, err = processor.store.GetMerchantPaymentConfig(ctx, order.MerchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			log.Warn().Int64("merchant_id", order.MerchantID).Msg("merchant payment config not found, skip profit sharing snapshot")
			return nil
		}
		return fmt.Errorf("get merchant payment config: %w", err)
	}

	totalAmount := paymentOrder.Amount
	deliveryFee := order.DeliveryFee
	orderSource := order.OrderType
	if order.ReservationID.Valid && order.OrderType == "dine_in" {
		orderSource = db.OrderTypeReservation
	}

	config, err := processor.store.GetActiveProfitSharingConfig(ctx, db.GetActiveProfitSharingConfigParams{
		OrderSource: orderSource,
		MerchantID:  pgtype.Int8{Int64: order.MerchantID, Valid: true},
		RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: merchant.RegionID > 0},
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			config = db.ProfitSharingConfig{
				PlatformRate: 0,
				OperatorRate: 0,
				RiderEnabled: false,
			}
		} else {
			return fmt.Errorf("get profit sharing config: %w", err)
		}
	}

	platformRate := config.PlatformRate
	operatorRate := config.OperatorRate
	riderEnabled := config.RiderEnabled
	if orderSource == db.OrderTypeReservation {
		riderEnabled = false
	}
	if orderSource == "dine_in" || orderSource == "takeaway" {
		platformRate = 0
		operatorRate = 0
		riderEnabled = false
	}

	var operator db.Operator
	var hasOperator bool
	var operatorCommission int64
	var platformCommission int64
	var operatorCommissionRedirectedToPlatform bool
	merchantAmount := totalAmount

	regionID := merchant.RegionID
	if regionID > 0 {
		op, err := processor.store.GetActiveOperatorByRegion(ctx, regionID)
		if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("get operator: %w", err)
		}
		if err == nil {
			operator = op
			hasOperator = true
		}
	}

	var rider db.Rider
	var hasRider bool
	var riderAmount int64
	if order.ID > 0 && riderEnabled && deliveryFee > 0 {
		delivery, err := processor.store.GetDeliveryByOrderID(ctx, order.ID)
		if err != nil {
			if !errors.Is(err, db.ErrRecordNotFound) {
				return fmt.Errorf("get delivery by order id: %w", err)
			}
		} else if delivery.RiderID.Valid {
			rider, err = processor.store.GetRider(ctx, delivery.RiderID.Int64)
			if err != nil {
				return fmt.Errorf("get rider: %w", err)
			}
			user, err := processor.store.GetUser(ctx, rider.UserID)
			if err != nil {
				return fmt.Errorf("get rider user: %w", err)
			}
			if user.WechatOpenid == "" {
				log.Warn().
					Int64("order_id", order.ID).
					Int64("rider_id", rider.ID).
					Msg("rider wechat openid is empty, skip rider profit sharing snapshot")
			} else {
				hasRider = true
				riderAmount = deliveryFee
				if riderAmount > totalAmount {
					riderAmount = totalAmount
				}
			}
		}
	}

	distributableAmount := totalAmount - riderAmount
	if distributableAmount < 0 {
		distributableAmount = 0
	}

	platformCommission = distributableAmount * int64(platformRate) / 100
	operatorCommission = distributableAmount * int64(operatorRate) / 100
	if !hasOperator && operatorCommission > 0 {
		platformCommission += operatorCommission
		operatorCommission = 0
		operatorCommissionRedirectedToPlatform = true
	}
	merchantAmount = distributableAmount - platformCommission - operatorCommission
	if merchantAmount < 0 {
		log.Error().
			Int64("payment_order_id", paymentOrder.ID).
			Int64("total_amount", totalAmount).
			Int64("distributable_amount", distributableAmount).
			Int64("platform_commission", platformCommission).
			Int64("operator_commission", operatorCommission).
			Int64("rider_amount", riderAmount).
			Msg("merchant amount computed negative in profit sharing snapshot")
		processor.publishAlert(ctx, AlertData{
			AlertType:   AlertTypeSystemError,
			Level:       AlertLevelCritical,
			Title:       "分账配置错误",
			Message:     fmt.Sprintf("支付单 %d 分账计算商户金额为负（平台+运营商比例之和超过100%%），请检查分账配置", paymentOrder.ID),
			RelatedID:   paymentOrder.ID,
			RelatedType: "payment_order",
		})
		merchantAmount = 0
	}

	outOrderNo := fmt.Sprintf("PS%d%d", paymentOrder.ID, order.ID)
	var operatorID pgtype.Int8
	var riderID pgtype.Int8
	if hasOperator {
		operatorID = pgtype.Int8{Int64: operator.ID, Valid: true}
	}
	if hasRider {
		riderID = pgtype.Int8{Int64: rider.ID, Valid: true}
	}

	_, err = processor.store.CreateProfitSharingOrder(ctx, db.CreateProfitSharingOrderParams{
		PaymentOrderID:      paymentOrder.ID,
		MerchantID:          order.MerchantID,
		OperatorID:          operatorID,
		OrderSource:         orderSource,
		TotalAmount:         totalAmount,
		DeliveryFee:         deliveryFee,
		RiderID:             riderID,
		RiderAmount:         riderAmount,
		DistributableAmount: distributableAmount,
		PlatformRate:        platformRate,
		OperatorRate:        operatorRate,
		PlatformCommission:  platformCommission,
		OperatorCommission:  operatorCommission,
		MerchantAmount:      merchantAmount,
		OutOrderNo:          outOrderNo,
		Status:              db.ProfitSharingOrderStatusPending,
		PaymentFee:          estimatedWechatProfitSharingPaymentFee(totalAmount),
		PaymentFeeRateBps:   wechatProfitSharingPaymentFeeRateBps,
		Provider:            db.ExternalPaymentProviderWechat,
		Channel:             paymentOrder.PaymentChannel,
	})
	if err != nil {
		return fmt.Errorf("create profit sharing order: %w", err)
	}

	_ = operatorCommissionRedirectedToPlatform
	return nil
}
