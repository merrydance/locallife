package db

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// CreatePartnerPaymentTxParams 为服务商单笔支付创建本地 payment_order。
// 该事务仅创建 payment_orders，不创建 combined_payment_orders。
type CreatePartnerPaymentTxParams struct {
	UserID        int64
	MerchantID    int64
	OrderID       int64
	ReservationID int64
	PaymentMode   string
	BusinessType  string
	Amount        int64
	OutTradeNo    string
	ExpiresAt     time.Time
	Attach        string
}

// CreatePartnerPaymentTxResult 返回创建结果及商户服务商配置。
type CreatePartnerPaymentTxResult struct {
	PaymentOrder PaymentOrder
	SubMchID     string
}

// CreatePartnerPaymentTx 创建服务商单笔支付记录并校验商户配置与并发 pending 冲突。
func (store *SQLStore) CreatePartnerPaymentTx(ctx context.Context, arg CreatePartnerPaymentTxParams) (CreatePartnerPaymentTxResult, error) {
	var result CreatePartnerPaymentTxResult
	var linkedReservationID pgtype.Int8
	resolvedMerchantID := arg.MerchantID
	requiresProfitSharing := false

	err := store.execTx(ctx, func(q *Queries) error {
		if arg.OrderID > 0 {
			order, err := q.GetOrderForUpdate(ctx, arg.OrderID)
			if err != nil {
				if errors.Is(err, ErrRecordNotFound) {
					return &requestError{statusCode: http.StatusNotFound, err: errors.New("order not found")}
				}
				return fmt.Errorf("get order %d: %w", arg.OrderID, err)
			}
			if order.UserID != arg.UserID {
				return &requestError{statusCode: http.StatusForbidden, err: fmt.Errorf("order %d does not belong to user", arg.OrderID)}
			}
			if order.Status != "pending" {
				return &requestError{statusCode: http.StatusBadRequest, err: fmt.Errorf("order %d status is %s, expect pending", arg.OrderID, order.Status)}
			}
			payAmount, err := OrderRemainingPayableAmount(order)
			if err != nil {
				return fmt.Errorf("resolve order %d payable amount: %w", arg.OrderID, err)
			}
			if payAmount != arg.Amount {
				return &requestError{statusCode: http.StatusConflict, err: fmt.Errorf("order %d payable amount changed", arg.OrderID)}
			}
			resolvedMerchantID = order.MerchantID
			if arg.MerchantID > 0 && order.MerchantID != arg.MerchantID {
				return &requestError{statusCode: http.StatusConflict, err: fmt.Errorf("order %d merchant changed", arg.OrderID)}
			}
			linkedReservationID = order.ReservationID
			requiresProfitSharing = OrderRequiresProfitSharing(order)
			if arg.ReservationID > 0 {
				if !order.ReservationID.Valid || order.ReservationID.Int64 != arg.ReservationID {
					return &requestError{statusCode: http.StatusConflict, err: fmt.Errorf("order %d reservation link changed", arg.OrderID)}
				}
			}
			existingPO, err := q.GetLatestPaymentOrderByOrder(ctx, GetLatestPaymentOrderByOrderParams{
				OrderID:      pgtype.Int8{Int64: arg.OrderID, Valid: true},
				BusinessType: arg.BusinessType,
			})
			if err == nil && existingPO.Status == "pending" {
				return &requestError{statusCode: http.StatusConflict, err: fmt.Errorf("order %d has pending payment order: %w", arg.OrderID, ErrOrderPendingPaymentConflict)}
			}
			if err != nil && err != ErrRecordNotFound {
				return fmt.Errorf("get latest payment order for order %d: %w", arg.OrderID, err)
			}
		}

		if arg.ReservationID > 0 && arg.BusinessType == "reservation" {
			reservation, err := q.GetTableReservationForUpdate(ctx, arg.ReservationID)
			if err != nil {
				if errors.Is(err, ErrRecordNotFound) {
					return &requestError{statusCode: http.StatusNotFound, err: errors.New("reservation not found")}
				}
				return fmt.Errorf("get reservation %d: %w", arg.ReservationID, err)
			}
			if reservation.UserID != arg.UserID {
				return &requestError{statusCode: http.StatusForbidden, err: fmt.Errorf("reservation %d does not belong to user", arg.ReservationID)}
			}
			if reservation.Status != "pending" {
				return &requestError{statusCode: http.StatusBadRequest, err: fmt.Errorf("reservation %d status is %s, expect pending", arg.ReservationID, reservation.Status)}
			}
			resolvedMerchantID = reservation.MerchantID
			if arg.MerchantID > 0 && reservation.MerchantID != arg.MerchantID {
				return &requestError{statusCode: http.StatusConflict, err: fmt.Errorf("reservation %d merchant changed", arg.ReservationID)}
			}
			if arg.PaymentMode != "" && reservation.PaymentMode != arg.PaymentMode {
				return &requestError{statusCode: http.StatusConflict, err: fmt.Errorf("reservation %d payment mode changed", arg.ReservationID)}
			}
			payAmount := reservation.PrepaidAmount
			if reservation.PaymentMode == "deposit" {
				payAmount = reservation.DepositAmount
			}
			if payAmount != arg.Amount {
				return &requestError{statusCode: http.StatusConflict, err: fmt.Errorf("reservation %d payable amount changed", arg.ReservationID)}
			}
			existingPO, err := q.GetLatestPaymentOrderByReservation(ctx, GetLatestPaymentOrderByReservationParams{
				ReservationID: pgtype.Int8{Int64: arg.ReservationID, Valid: true},
				BusinessType:  arg.BusinessType,
			})
			if err == nil && existingPO.Status == "pending" {
				return &requestError{statusCode: http.StatusConflict, err: fmt.Errorf("reservation %d has pending payment order: %w", arg.ReservationID, ErrOrderPendingPaymentConflict)}
			}
			if err != nil && err != ErrRecordNotFound {
				return fmt.Errorf("get latest payment order for reservation %d: %w", arg.ReservationID, err)
			}
		}

		paymentConfig, err := q.GetMerchantPaymentConfig(ctx, resolvedMerchantID)
		if err != nil {
			return fmt.Errorf("get merchant payment config for merchant %d: %w", resolvedMerchantID, err)
		}
		if paymentConfig.Status != "active" || paymentConfig.SubMchID == "" {
			return &requestError{statusCode: http.StatusBadRequest, err: fmt.Errorf("merchant %d payment config invalid or inactive", resolvedMerchantID)}
		}
		result.SubMchID = paymentConfig.SubMchID
		enrichedAttach := arg.Attach
		if enrichedAttach != "" {
			enrichedAttach = enrichedAttach + ";sub_mchid:" + paymentConfig.SubMchID
		}

		createParams := CreatePaymentOrderParams{
			UserID:                arg.UserID,
			PaymentType:           "miniprogram",
			PaymentChannel:        PaymentChannelOrdinaryServiceProvider,
			RequiresProfitSharing: requiresProfitSharing,
			BusinessType:          arg.BusinessType,
			Amount:                arg.Amount,
			OutTradeNo:            arg.OutTradeNo,
			ExpiresAt:             pgtype.Timestamptz{Time: arg.ExpiresAt, Valid: true},
		}
		if arg.OrderID > 0 {
			createParams.OrderID = pgtype.Int8{Int64: arg.OrderID, Valid: true}
		}
		if arg.ReservationID > 0 {
			createParams.ReservationID = pgtype.Int8{Int64: arg.ReservationID, Valid: true}
		} else if linkedReservationID.Valid {
			createParams.ReservationID = linkedReservationID
		}
		if enrichedAttach != "" {
			createParams.Attach = pgtype.Text{String: enrichedAttach, Valid: true}
		}

		result.PaymentOrder, err = q.CreatePaymentOrder(ctx, createParams)
		if err != nil {
			return fmt.Errorf("create payment order: %w", err)
		}

		return nil
	})

	return result, err
}

func IsPartnerPaymentRequestError(err error) (int, bool) {
	var re *requestError
	if errors.As(err, &re) {
		return re.statusCode, true
	}
	return 0, false
}
