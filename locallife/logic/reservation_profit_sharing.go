package logic

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

func scheduleBaofuProfitSharingForCompletedReservation(ctx context.Context, store db.Store, taskScheduler TaskScheduler, merchant db.Merchant, reservation db.TableReservation) {
	if reservation.ID <= 0 || reservation.Status != reservationStatusCompleted || !reservation.CompletedAt.Valid {
		return
	}

	paymentOrders, err := store.GetPaymentOrdersByReservation(ctx, pgtype.Int8{Int64: reservation.ID, Valid: true})
	if err != nil {
		log.Warn().Err(err).Int64("reservation_id", reservation.ID).Msg("skip scheduling baofu profit sharing for completed reservation")
		return
	}
	config, err := ResolveBaofuProfitSharingConfig(ctx, store, db.OrderTypeReservation, merchant)
	if err != nil {
		log.Warn().Err(err).
			Int64("reservation_id", reservation.ID).
			Msg("skip scheduling baofu reservation profit sharing because profit sharing config cannot be resolved")
		return
	}

	service := NewBaofuProfitSharingService(store)
	for _, paymentOrder := range paymentOrders {
		if !reservationPaymentOrderReadyForBaofuProfitSharing(paymentOrder, reservation.ID) {
			continue
		}
		refundedAmount, err := store.GetTotalSuccessfulRefundedByPaymentOrder(ctx, paymentOrder.ID)
		if err != nil {
			log.Warn().Err(err).
				Int64("reservation_id", reservation.ID).
				Int64("payment_order_id", paymentOrder.ID).
				Msg("skip scheduling baofu reservation profit sharing because successful refund amount cannot be resolved")
			continue
		}
		if refundedAmount >= paymentOrder.Amount {
			continue
		}
		created, err := service.CreatePendingOrder(ctx, BaofuProfitSharingOrderInput{
			PaymentOrderID:  paymentOrder.ID,
			MerchantID:      reservation.MerchantID,
			OperatorID:      config.OperatorID,
			PlatformOwnerID: 0,
			OrderSource:     db.OrderTypeReservation,
			TotalAmountFen:  paymentOrder.Amount,
			RefundedFen:     refundedAmount,
			DeliveryFeeFen:  0,
			PlatformRateBps: config.PlatformRateBps,
			OperatorRateBps: config.OperatorRateBps,
			OutOrderNo:      fmt.Sprintf("BFPS%dR%d", paymentOrder.ID, reservation.ID),
		})
		if err != nil {
			log.Warn().Err(err).
				Int64("reservation_id", reservation.ID).
				Int64("payment_order_id", paymentOrder.ID).
				Msg("skip scheduling baofu reservation profit sharing because bill creation failed")
			continue
		}
		if created.ProfitSharingOrder.ID <= 0 {
			continue
		}
		if taskScheduler == nil {
			continue
		}
		if err := taskScheduler.ScheduleProfitSharing(ctx, created.ProfitSharingOrder.ID); err != nil {
			log.Warn().Err(err).
				Int64("reservation_id", reservation.ID).
				Int64("payment_order_id", paymentOrder.ID).
				Int64("profit_sharing_order_id", created.ProfitSharingOrder.ID).
				Msg("schedule baofu reservation profit sharing failed")
		}
	}
}

func reservationPaymentOrderReadyForBaofuProfitSharing(paymentOrder db.PaymentOrder, reservationID int64) bool {
	if paymentOrder.Status != paymentStatusPaid ||
		paymentOrder.PaymentChannel != db.PaymentChannelBaofuAggregate ||
		!db.PaymentOrderRequiresProfitSharing(paymentOrder) ||
		!paymentOrder.ReservationID.Valid ||
		paymentOrder.ReservationID.Int64 != reservationID {
		return false
	}
	return paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerReservation || paymentOrder.BusinessType == reservationAddonBusiness
}
