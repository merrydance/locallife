package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

const (
	paymentTypeMiniProgram  = "miniprogram"
	paymentTypeNative       = "native"
	paymentStatusPending    = "pending"
	businessTypeOrder       = "order"
	businessTypeReservation = "reservation"
)

const (
	outTradeNoMaxRetry      = 3
	outTradeNoRetryBaseBack = 50 * time.Millisecond
)

// PaymentOrderService encapsulates payment order creation logic.
type PaymentOrderService struct {
	store         db.Store
	paymentClient wechat.PaymentClientInterface
	now           func() time.Time
}

// NewPaymentOrderService creates a payment order service.
func NewPaymentOrderService(store db.Store, paymentClient wechat.PaymentClientInterface) *PaymentOrderService {
	return &PaymentOrderService{
		store:         store,
		paymentClient: paymentClient,
		now:           time.Now,
	}
}

// CreatePaymentOrderInput defines the input for creating a payment order.
type CreatePaymentOrderInput struct {
	UserID       int64
	OrderID      int64
	PaymentType  string
	BusinessType string
	ClientIP     string
}

// CreatePaymentOrderResult holds the created payment order and pay params.
type CreatePaymentOrderResult struct {
	PaymentOrder db.PaymentOrder
	PayParams    *wechat.JSAPIPayParams
}

type GetPaymentOrderInput struct {
	UserID         int64
	PaymentOrderID int64
}

type GetPaymentOrderResult struct {
	PaymentOrder db.PaymentOrder
}

type ListPaymentOrdersInput struct {
	UserID   int64
	OrderID  *int64
	PageID   int32
	PageSize int32
}

type ListPaymentOrdersResult struct {
	PaymentOrders []db.PaymentOrder
	TotalCount    int64
}

type ClosePaymentOrderInput struct {
	UserID         int64
	PaymentOrderID int64
}

type ClosePaymentOrderResult struct {
	PaymentOrder db.PaymentOrder
}

// CreatePaymentOrder validates and creates a payment order.
func (svc *PaymentOrderService) CreatePaymentOrder(ctx context.Context, input CreatePaymentOrderInput) (CreatePaymentOrderResult, error) {
	var result CreatePaymentOrderResult

	if input.BusinessType != businessTypeOrder && input.BusinessType != businessTypeReservation {
		return result, NewRequestError(http.StatusBadRequest, errors.New("invalid business type"))
	}

	var amount int64
	merchantName := "Order Payment"
	var merchantID int64
	var attach string

	if input.BusinessType == businessTypeReservation {
		reservation, err := svc.store.GetTableReservation(ctx, input.OrderID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return result, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
			}
			return result, fmt.Errorf("get reservation: %w", err)
		}

		if reservation.UserID != input.UserID {
			return result, NewRequestError(http.StatusForbidden, errors.New("reservation does not belong to you"))
		}

		if reservation.Status != "pending" {
			return result, NewRequestError(http.StatusBadRequest, errors.New("reservation is not in pending status"))
		}

		merchantID = reservation.MerchantID
		if reservation.PaymentMode == "deposit" {
			amount = reservation.DepositAmount
		} else {
			amount = reservation.PrepaidAmount
		}
		attach = fmt.Sprintf("reservation_id:%d", reservation.ID)
	} else {
		order, err := svc.store.GetOrder(ctx, input.OrderID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return result, NewRequestError(http.StatusNotFound, errors.New("order not found"))
			}
			return result, fmt.Errorf("get order: %w", err)
		}

		if order.UserID != input.UserID {
			return result, NewRequestError(http.StatusForbidden, errors.New("order does not belong to you"))
		}

		if order.Status != "pending" {
			return result, NewRequestError(http.StatusBadRequest, errors.New("order is not in pending status"))
		}

		amount = order.TotalAmount
		merchantID = order.MerchantID
		attach = fmt.Sprintf("order_id:%d", order.ID)
	}

	if amount <= 0 {
		return result, NewRequestError(http.StatusBadRequest, errors.New("payment amount must be greater than 0"))
	}

	// Check existing pending payment order.
	var existingPayment db.PaymentOrder
	var err error
	if input.BusinessType == businessTypeReservation {
		existingPayment, err = svc.store.GetLatestPaymentOrderByReservation(ctx, db.GetLatestPaymentOrderByReservationParams{
			ReservationID: pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType:  input.BusinessType,
		})
	} else {
		existingPayment, err = svc.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: input.BusinessType,
		})
	}
	if err == nil && existingPayment.Status == paymentStatusPending {
		result.PaymentOrder = existingPayment
		// 干等返回时，若已有 prepay_id 则重新签名生成 pay_params，
		// 避免前端因收到 null pay_params 而无法调起支付。
		if svc.paymentClient != nil && existingPayment.PrepayID.Valid {
			if payParams, signErr := svc.paymentClient.GenerateJSAPIPayParams(existingPayment.PrepayID.String); signErr == nil {
				result.PayParams = payParams
			}
		}
		return result, nil
	}

	expiresAt := svc.now().Add(30 * time.Minute)

	var paymentOrder db.PaymentOrder
	var outTradeNo string
	for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
		var genErr error
		outTradeNo, genErr = generateOutTradeNo()
		if genErr != nil {
			return result, fmt.Errorf("generate out trade no: %w", genErr)
		}
		createParams := db.CreatePaymentOrderParams{
			UserID:       input.UserID,
			PaymentType:  input.PaymentType,
			BusinessType: input.BusinessType,
			Amount:       amount,
			OutTradeNo:   outTradeNo,
			ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
		}
		if input.BusinessType == businessTypeReservation {
			createParams.ReservationID = pgtype.Int8{Int64: input.OrderID, Valid: true}
		} else {
			createParams.OrderID = pgtype.Int8{Int64: input.OrderID, Valid: true}
		}
		paymentOrder, err = svc.store.CreatePaymentOrder(ctx, createParams)
		if err == nil {
			break
		}
		if isOutTradeNoConflict(err) && attempt < outTradeNoMaxRetry {
			if !sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
				return result, NewRequestError(http.StatusRequestTimeout, errors.New("request canceled"))
			}
			continue
		}
		return result, fmt.Errorf("create payment order: %w", err)
	}

	result.PaymentOrder = paymentOrder

	if svc.paymentClient != nil && input.PaymentType == paymentTypeMiniProgram {
		user, err := svc.store.GetUser(ctx, input.UserID)
		if err != nil {
			return result, fmt.Errorf("get user: %w", err)
		}
		if user.WechatOpenid == "" {
			return result, NewRequestError(http.StatusBadRequest, errors.New("wechat openid not found"))
		}

		if merchantID > 0 {
			merchant, err := svc.store.GetMerchant(ctx, merchantID)
			if err == nil {
				if input.BusinessType == businessTypeReservation {
					merchantName = merchant.Name + " - Reservation Deposit"
				} else {
					merchantName = merchant.Name + " - Order Payment"
				}
			}
		}

		wxResp, payParams, err := svc.paymentClient.CreateJSAPIOrder(ctx, &wechat.JSAPIOrderRequest{
			OutTradeNo:    outTradeNo,
			Description:   merchantName,
			TotalAmount:   amount,
			OpenID:        user.WechatOpenid,
			ExpireTime:    expiresAt,
			Attach:        attach,
			PayerClientIP: input.ClientIP,
		})
		if err != nil {
			_, _ = svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
			return result, fmt.Errorf("wechat pay: %w", err)
		}

		updatedPayment, err := svc.store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
			ID:       paymentOrder.ID,
			PrepayID: pgtype.Text{String: wxResp.PrepayID, Valid: true},
		})
		if err != nil {
			// 微信下单已成功但本地更新失败。
			// 策略：退出后调用分布式锁来临界区关闭本地支付单和微信单。
			// 微信单不需要主动关闭：用户不会收到 prepay_id 故无法发起调起支付，
			// 微信单会在 30 分钟后自动过期关闭。
			// 本地支付单则需要由超时消费者除理（已有 payment_timeout 任务干赔）。
			// 同时更新支付单为失败，避免卡在 pending 状态。
			_, _ = svc.store.UpdatePaymentOrderToFailed(ctx, paymentOrder.ID)
			// 尝试调用微信关单 API（如果微信单已存在）
			if svc.paymentClient != nil {
				if closeErr := svc.paymentClient.CloseOrder(ctx, outTradeNo); closeErr != nil {
					log.Warn().Err(closeErr).Str("out_trade_no", outTradeNo).Msg("close wechat order after prepay_id update failure")
				}
			}
			return result, fmt.Errorf("update prepay id: %w", err)
		}

		result.PaymentOrder = updatedPayment
		result.PayParams = payParams
	}

	return result, nil
}

func (svc *PaymentOrderService) GetPaymentOrder(ctx context.Context, input GetPaymentOrderInput) (GetPaymentOrderResult, error) {
	paymentOrder, err := svc.store.GetPaymentOrder(ctx, input.PaymentOrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return GetPaymentOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("payment order not found"))
		}
		return GetPaymentOrderResult{}, err
	}

	if paymentOrder.UserID != input.UserID {
		return GetPaymentOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("payment order does not belong to you"))
	}

	return GetPaymentOrderResult{PaymentOrder: paymentOrder}, nil
}

func (svc *PaymentOrderService) ListPaymentOrders(ctx context.Context, input ListPaymentOrdersInput) (ListPaymentOrdersResult, error) {
	pageID := input.PageID
	pageSize := input.PageSize
	if pageID == 0 {
		pageID = 1
	}
	if pageSize == 0 {
		pageSize = 10
	}

	if input.OrderID != nil {
		payment, err := svc.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: *input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		})
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return ListPaymentOrdersResult{PaymentOrders: []db.PaymentOrder{}, TotalCount: 0}, nil
			}
			return ListPaymentOrdersResult{}, err
		}
		if payment.UserID != input.UserID {
			return ListPaymentOrdersResult{PaymentOrders: []db.PaymentOrder{}, TotalCount: 0}, nil
		}
		return ListPaymentOrdersResult{PaymentOrders: []db.PaymentOrder{payment}, TotalCount: 1}, nil
	}

	offset := (pageID - 1) * pageSize
	paymentOrders, err := svc.store.ListPaymentOrdersByUser(ctx, db.ListPaymentOrdersByUserParams{
		UserID: input.UserID,
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return ListPaymentOrdersResult{}, err
	}

	return ListPaymentOrdersResult{PaymentOrders: paymentOrders, TotalCount: int64(len(paymentOrders))}, nil
}

func (svc *PaymentOrderService) ClosePaymentOrder(ctx context.Context, input ClosePaymentOrderInput) (ClosePaymentOrderResult, error) {
	paymentOrder, err := svc.store.GetPaymentOrder(ctx, input.PaymentOrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ClosePaymentOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("payment order not found"))
		}
		return ClosePaymentOrderResult{}, err
	}

	if paymentOrder.UserID != input.UserID {
		return ClosePaymentOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("payment order does not belong to you"))
	}

	if paymentOrder.Status != paymentStatusPending {
		return ClosePaymentOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("only pending payment orders can be closed"))
	}

	updatedPayment, err := svc.store.UpdatePaymentOrderToClosed(ctx, input.PaymentOrderID)
	if err != nil {
		return ClosePaymentOrderResult{}, err
	}

	if svc.paymentClient != nil && paymentOrder.PrepayID.Valid {
		if err := svc.paymentClient.CloseOrder(ctx, paymentOrder.OutTradeNo); err != nil {
			// 微信关单失败不阻断业务（订单会在 30 分钟后自动关闭），但必须记录
			log.Warn().Err(err).Str("out_trade_no", paymentOrder.OutTradeNo).Msg("close wechat order failed, order will auto-expire")
		}
	}

	return ClosePaymentOrderResult{PaymentOrder: updatedPayment}, nil
}

func generateOutTradeNo() (string, error) {
	return util.GenerateOutTradeNo("P")
}

func generateOutTradeNoWithPrefix(prefix string) (string, error) {
	return util.GenerateOutTradeNo(prefix)
}

func isOutTradeNoConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != "23505" {
		return false
	}
	if strings.Contains(pgErr.ConstraintName, "out_trade_no") {
		return true
	}
	return strings.Contains(pgErr.Detail, "out_trade_no")
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
