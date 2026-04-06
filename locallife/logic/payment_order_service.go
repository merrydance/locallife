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
	concurrentPaymentRetry  = 2
	orderTypeDineIn         = "dine_in"
	orderTypeTakeaway       = "takeaway"
)

// PaymentOrderService encapsulates payment order creation logic.
type PaymentOrderService struct {
	store           db.Store
	paymentClient   wechat.PaymentClientInterface
	ecommerceClient wechat.EcommerceClientInterface
	now             func() time.Time
}

// NewPaymentOrderService creates a payment order service.
func NewPaymentOrderService(store db.Store, paymentClient wechat.PaymentClientInterface, ecommerceClient wechat.EcommerceClientInterface) *PaymentOrderService {
	return &PaymentOrderService{
		store:           store,
		paymentClient:   paymentClient,
		ecommerceClient: ecommerceClient,
		now:             time.Now,
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
	var orderType string
	var reservationLinkedOrder bool
	var reservationPaymentMode string

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
		reservationPaymentMode = reservation.PaymentMode
		if reservation.PaymentMode == paymentModeDeposit {
			amount = reservation.DepositAmount
		} else {
			amount = reservation.PrepaidAmount
		}
		attach = buildReservationPaymentAttach(reservation.ID, reservation.PaymentMode)
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

		amount, err = db.OrderRemainingPayableAmount(order)
		if err != nil {
			return result, fmt.Errorf("resolve order payable amount: %w", err)
		}
		merchantID = order.MerchantID
		orderType = order.OrderType
		reservationLinkedOrder = order.ReservationID.Valid
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
		if input.BusinessType == businessTypeReservation {
			if !shouldReuseReservationPendingPayment(existingPayment, amount, attach) {
				if closeErr := svc.supersedePendingPaymentOrder(ctx, existingPayment); closeErr != nil {
					return result, closeErr
				}
			} else {
				result.PaymentOrder = existingPayment
				result.PayParams = svc.signExistingPaymentOrder(existingPayment)
				return result, nil
			}
		} else if existingPayment.Amount != amount {
			if closeErr := svc.supersedePendingPaymentOrder(ctx, existingPayment); closeErr != nil {
				return result, closeErr
			}
		} else {
			result.PaymentOrder = existingPayment
			result.PayParams = svc.signExistingPaymentOrder(existingPayment)
			return result, nil
		}
	}

	expiresAt := svc.now().Add(30 * time.Minute)

	// ==================== 单订单/预定走收付通单笔支付 ====================
	if input.BusinessType == businessTypeReservation {
		return svc.createReservationEcommercePayment(ctx, input, merchantID, merchantName, reservationPaymentMode, amount, attach, expiresAt)
	}
	if input.BusinessType == businessTypeOrder {
		return svc.createOrderEcommercePayment(ctx, input, merchantID, merchantName, reservationLinkedOrder || shouldEnableOrderProfitSharing(orderType), amount, attach, expiresAt)
	}

	// ==================== 订单走直连或扫码支付 ====================
	var paymentOrder db.PaymentOrder
	var outTradeNo string
	err = nil
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
		createParams.OrderID = pgtype.Int8{Int64: input.OrderID, Valid: true}
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
			_, _ = svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
			return result, fmt.Errorf("get user: %w", err)
		}
		if user.WechatOpenid == "" {
			_, _ = svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
			return result, NewRequestError(http.StatusBadRequest, errors.New("wechat openid not found"))
		}

		if merchantID > 0 {
			merchant, err := svc.store.GetMerchant(ctx, merchantID)
			if err == nil {
				merchantName = merchant.Name + " - Order Payment"
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
			_, _ = svc.store.UpdatePaymentOrderToFailed(ctx, paymentOrder.ID)
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

// createReservationEcommercePayment 通过收付通单笔支付创建预定支付单。
func (svc *PaymentOrderService) createReservationEcommercePayment(
	ctx context.Context,
	input CreatePaymentOrderInput,
	merchantID int64,
	merchantName string,
	paymentMode string,
	amount int64,
	attach string,
	expiresAt time.Time,
) (CreatePaymentOrderResult, error) {
	var result CreatePaymentOrderResult

	if svc.ecommerceClient == nil {
		return result, fmt.Errorf("ecommerce client: not configured")
	}

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
			if paymentMode == paymentModeFull {
				merchantName = merchant.Name + " - Reservation Prepaid"
			} else {
				merchantName = merchant.Name + " - Reservation Deposit"
			}
		}
	}

	var txResult db.CreatePartnerPaymentTxResult
	for attempt := 1; attempt <= concurrentPaymentRetry; attempt++ {
		outTradeNo, genErr := generateOutTradeNoWithPrefix("RS")
		if genErr != nil {
			return result, fmt.Errorf("generate out trade no: %w", genErr)
		}
		txResult, err = svc.store.CreatePartnerPaymentTx(ctx, db.CreatePartnerPaymentTxParams{
			UserID:        input.UserID,
			MerchantID:    merchantID,
			ReservationID: input.OrderID,
			PaymentMode:   paymentMode,
			BusinessType:  input.BusinessType,
			Amount:        amount,
			OutTradeNo:    outTradeNo,
			ExpiresAt:     expiresAt,
			Attach:        attach,
		})
		if err == nil {
			break
		}
		if errors.Is(err, db.ErrOrderPendingPaymentConflict) {
			resolved, handled, resolveErr := svc.resolveConcurrentReservationPayment(ctx, input, amount, attach)
			if resolveErr != nil {
				return result, resolveErr
			}
			if handled {
				return resolved, nil
			}
			if attempt < concurrentPaymentRetry {
				continue
			}
			return result, NewRequestError(http.StatusConflict, errors.New("payment order is being recreated, please retry"))
		}
		if isOutTradeNoConflict(err) && attempt < outTradeNoMaxRetry {
			if !sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
				return result, NewRequestError(http.StatusRequestTimeout, errors.New("request canceled"))
			}
			continue
		}
		// 非直连支付错误，检查是否为商户配置问题
		return result, mapReservationEcommerceError(err)
	}

	result.PaymentOrder = txResult.PaymentOrder
	paymentAttach := attach
	if txResult.PaymentOrder.Attach.Valid && strings.TrimSpace(txResult.PaymentOrder.Attach.String) != "" {
		paymentAttach = txResult.PaymentOrder.Attach.String
	}

	orderResp, payParams, err := svc.ecommerceClient.CreatePartnerJSAPIOrder(ctx, &wechat.PartnerJSAPIOrderRequest{
		SubMchID:      txResult.SubMchID,
		Description:   merchantName,
		OutTradeNo:    txResult.PaymentOrder.OutTradeNo,
		ExpireTime:    expiresAt,
		Attach:        paymentAttach,
		TotalAmount:   amount,
		PayerOpenID:   user.WechatOpenid,
		PayerClientIP: input.ClientIP,
		ProfitSharing: true,
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = svc.store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID)
		return result, fmt.Errorf("create partner jsapi order: %w", err)
	}
	if orderResp == nil || strings.TrimSpace(orderResp.PrepayID) == "" {
		cleanupCtx := context.Background()
		_, _ = svc.store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID)
		return result, fmt.Errorf("create partner jsapi order: empty prepay id")
	}

	updatedPayment, err := svc.store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       txResult.PaymentOrder.ID,
		PrepayID: pgtype.Text{String: orderResp.PrepayID, Valid: true},
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = svc.store.UpdatePaymentOrderToFailed(cleanupCtx, txResult.PaymentOrder.ID)
		if closeErr := svc.ecommerceClient.ClosePartnerOrder(cleanupCtx, txResult.PaymentOrder.OutTradeNo, txResult.SubMchID); closeErr != nil {
			log.Warn().Err(closeErr).Str("out_trade_no", txResult.PaymentOrder.OutTradeNo).Msg("close partner order after prepay update failure")
		}
		return result, fmt.Errorf("update prepay id: %w", err)
	}

	result.PaymentOrder = updatedPayment
	result.PayParams = payParams
	return result, nil
}

func (svc *PaymentOrderService) createOrderEcommercePayment(
	ctx context.Context,
	input CreatePaymentOrderInput,
	merchantID int64,
	merchantName string,
	profitSharing bool,
	expectedAmount int64,
	attach string,
	expiresAt time.Time,
) (CreatePaymentOrderResult, error) {
	var result CreatePaymentOrderResult

	if svc.ecommerceClient == nil {
		return result, fmt.Errorf("ecommerce client: not configured")
	}

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
			merchantName = merchant.Name + " - Order Payment"
		}
	}

	var txResult db.CreatePartnerPaymentTxResult
	for attempt := 1; attempt <= concurrentPaymentRetry; attempt++ {
		outTradeNo, genErr := generateOutTradeNoWithPrefix("OC")
		if genErr != nil {
			return result, fmt.Errorf("generate out trade no: %w", genErr)
		}
		txResult, err = svc.store.CreatePartnerPaymentTx(ctx, db.CreatePartnerPaymentTxParams{
			UserID:       input.UserID,
			MerchantID:   merchantID,
			OrderID:      input.OrderID,
			BusinessType: input.BusinessType,
			Amount:       expectedAmount,
			OutTradeNo:   outTradeNo,
			ExpiresAt:    expiresAt,
			Attach:       attach,
		})
		if err == nil {
			break
		}
		if errors.Is(err, db.ErrOrderPendingPaymentConflict) {
			resolved, handled, resolveErr := svc.resolveConcurrentOrderPayment(ctx, input, expectedAmount)
			if resolveErr != nil {
				return result, resolveErr
			}
			if handled {
				return resolved, nil
			}
			if attempt < concurrentPaymentRetry {
				continue
			}
			return result, NewRequestError(http.StatusConflict, errors.New("payment order is being recreated, please retry"))
		}
		if isOutTradeNoConflict(err) && attempt < outTradeNoMaxRetry {
			if !sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
				return result, NewRequestError(http.StatusRequestTimeout, errors.New("request canceled"))
			}
			continue
		}
		return result, mapReservationEcommerceError(err)
	}

	paymentAttach := attach
	if txResult.PaymentOrder.Attach.Valid && strings.TrimSpace(txResult.PaymentOrder.Attach.String) != "" {
		paymentAttach = txResult.PaymentOrder.Attach.String
	}

	orderResp, payParams, err := svc.ecommerceClient.CreatePartnerJSAPIOrder(ctx, &wechat.PartnerJSAPIOrderRequest{
		SubMchID:      txResult.SubMchID,
		Description:   merchantName,
		OutTradeNo:    txResult.PaymentOrder.OutTradeNo,
		ExpireTime:    expiresAt,
		Attach:        paymentAttach,
		TotalAmount:   txResult.PaymentOrder.Amount,
		PayerOpenID:   user.WechatOpenid,
		PayerClientIP: input.ClientIP,
		ProfitSharing: profitSharing,
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = svc.store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID)
		return result, fmt.Errorf("create partner jsapi order: %w", err)
	}
	if orderResp == nil || strings.TrimSpace(orderResp.PrepayID) == "" {
		cleanupCtx := context.Background()
		_, _ = svc.store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID)
		return result, fmt.Errorf("create partner jsapi order: empty prepay id")
	}

	updatedPayment, err := svc.store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       txResult.PaymentOrder.ID,
		PrepayID: pgtype.Text{String: orderResp.PrepayID, Valid: true},
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = svc.store.UpdatePaymentOrderToFailed(cleanupCtx, txResult.PaymentOrder.ID)
		if closeErr := svc.ecommerceClient.ClosePartnerOrder(cleanupCtx, txResult.PaymentOrder.OutTradeNo, txResult.SubMchID); closeErr != nil {
			log.Warn().Err(closeErr).Str("out_trade_no", txResult.PaymentOrder.OutTradeNo).Msg("close partner order after prepay update failure")
		}
		return result, fmt.Errorf("update prepay id: %w", err)
	}

	result.PaymentOrder = updatedPayment
	result.PayParams = payParams
	return result, nil
}

func (svc *PaymentOrderService) signExistingPaymentOrder(paymentOrder db.PaymentOrder) *wechat.JSAPIPayParams {
	if !paymentOrder.PrepayID.Valid {
		return nil
	}

	switch paymentOrder.PaymentType {
	case "profit_sharing":
		if svc.ecommerceClient != nil {
			if payParams, err := svc.ecommerceClient.GenerateJSAPIPayParams(paymentOrder.PrepayID.String); err == nil {
				return payParams
			}
		}
	default:
		if svc.paymentClient != nil {
			if payParams, err := svc.paymentClient.GenerateJSAPIPayParams(paymentOrder.PrepayID.String); err == nil {
				return payParams
			}
		}
	}

	return nil
}

func (svc *PaymentOrderService) supersedePendingPaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) error {
	if paymentOrder.PrepayID.Valid {
		_, err := svc.closePendingPaymentOrder(ctx, paymentOrder)
		return err
	}

	if _, err := svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID); err != nil {
		return err
	}
	if paymentOrder.CombinedPaymentID.Valid {
		if _, err := svc.store.UpdateCombinedPaymentOrderToClosed(ctx, paymentOrder.CombinedPaymentID.Int64); err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			return err
		}
	}

	return nil
}

func (svc *PaymentOrderService) resolveConcurrentOrderPayment(
	ctx context.Context,
	input CreatePaymentOrderInput,
	expectedAmount int64,
) (CreatePaymentOrderResult, bool, error) {
	var result CreatePaymentOrderResult

	for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
		paymentOrder, err := svc.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: input.BusinessType,
		})
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				if attempt < outTradeNoMaxRetry {
					if !sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
						return result, true, NewRequestError(http.StatusRequestTimeout, errors.New("request canceled"))
					}
					continue
				}
				return result, false, nil
			}
			return result, true, fmt.Errorf("get latest payment order after concurrent conflict: %w", err)
		}

		if paymentOrder.Status != paymentStatusPending {
			return result, false, nil
		}

		if paymentOrder.Amount != expectedAmount {
			if err := svc.supersedePendingPaymentOrder(ctx, paymentOrder); err != nil {
				return result, true, err
			}
			return result, false, nil
		}

		result.PaymentOrder = paymentOrder
		result.PayParams = svc.signExistingPaymentOrder(paymentOrder)
		if result.PayParams != nil {
			return result, true, nil
		}
		if attempt == outTradeNoMaxRetry {
			return result, true, NewRequestError(http.StatusConflict, errors.New("payment order is still preparing, please retry"))
		}

		if !sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
			return result, true, NewRequestError(http.StatusRequestTimeout, errors.New("request canceled"))
		}
	}

	return result, false, nil
}

func (svc *PaymentOrderService) resolveConcurrentReservationPayment(
	ctx context.Context,
	input CreatePaymentOrderInput,
	expectedAmount int64,
	expectedAttach string,
) (CreatePaymentOrderResult, bool, error) {
	var result CreatePaymentOrderResult

	for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
		paymentOrder, err := svc.store.GetLatestPaymentOrderByReservation(ctx, db.GetLatestPaymentOrderByReservationParams{
			ReservationID: pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType:  input.BusinessType,
		})
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				if attempt < outTradeNoMaxRetry {
					if !sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
						return result, true, NewRequestError(http.StatusRequestTimeout, errors.New("request canceled"))
					}
					continue
				}
				return result, false, nil
			}
			return result, true, fmt.Errorf("get latest payment order after concurrent conflict: %w", err)
		}

		if paymentOrder.Status != paymentStatusPending {
			return result, false, nil
		}

		if !shouldReuseReservationPendingPayment(paymentOrder, expectedAmount, expectedAttach) {
			if err := svc.supersedePendingPaymentOrder(ctx, paymentOrder); err != nil {
				return result, true, err
			}
			return result, false, nil
		}

		result.PaymentOrder = paymentOrder
		result.PayParams = svc.signExistingPaymentOrder(paymentOrder)
		if result.PayParams != nil {
			return result, true, nil
		}
		if attempt == outTradeNoMaxRetry {
			return result, true, NewRequestError(http.StatusConflict, errors.New("payment order is still preparing, please retry"))
		}

		if !sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
			return result, true, NewRequestError(http.StatusRequestTimeout, errors.New("request canceled"))
		}
	}

	return result, false, nil
}

// generateCombineOutTradeNoForSingle 生成单子商户合单主单号
func generateCombineOutTradeNoForSingle(prefix string) (string, error) {
	return generateOutTradeNoWithPrefix(prefix + "C")
}

func mapReservationEcommerceError(err error) error {
	if err == nil {
		return nil
	}
	if status, ok := db.IsPartnerPaymentRequestError(err); ok {
		return NewRequestError(status, errors.New(err.Error()))
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "payment config invalid") || strings.Contains(msg, "inactive"):
		return NewRequestError(http.StatusBadRequest, errors.New("merchant payment config invalid or not activated"))
	case strings.Contains(msg, "does not belong to user"):
		return NewRequestError(http.StatusForbidden, errors.New("payment target does not belong to you"))
	case strings.Contains(msg, "status is") || strings.Contains(msg, "expect pending"):
		return NewRequestError(http.StatusBadRequest, errors.New("payment target is not in pending status"))
	case strings.Contains(msg, "payable amount changed") || strings.Contains(msg, "payment mode changed"):
		return NewRequestError(http.StatusConflict, errors.New("payment target changed, please retry"))
	case strings.Contains(msg, "has pending payment order"):
		return NewRequestError(http.StatusConflict, errors.New("payment order already exists, please retry"))
	}
	return fmt.Errorf("create ecommerce payment: %w", err)
}

func buildReservationPaymentAttach(reservationID int64, paymentMode string) string {
	return fmt.Sprintf("reservation_id:%d;payment_mode:%s", reservationID, paymentMode)
}

func subMchIDFromPaymentAttach(paymentOrder db.PaymentOrder) string {
	if !paymentOrder.Attach.Valid {
		return ""
	}
	return parsePaymentAttach(paymentOrder.Attach.String)["sub_mchid"]
}

func parsePaymentAttach(attach string) map[string]string {
	parts := map[string]string{}
	for _, segment := range strings.Split(strings.TrimSpace(attach), ";") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		pair := strings.SplitN(segment, ":", 2)
		if len(pair) != 2 {
			continue
		}
		key := strings.TrimSpace(pair[0])
		value := strings.TrimSpace(pair[1])
		if key == "" || value == "" {
			continue
		}
		parts[key] = value
	}
	return parts
}



func shouldReuseReservationPendingPayment(paymentOrder db.PaymentOrder, expectedAmount int64, expectedAttach string) bool {
	if paymentOrder.Amount != expectedAmount || !paymentOrder.Attach.Valid {
		return false
	}
	existing := parsePaymentAttach(paymentOrder.Attach.String)
	expected := parsePaymentAttach(expectedAttach)
	return existing["reservation_id"] == expected["reservation_id"] && existing["payment_mode"] == expected["payment_mode"]
}

func shouldEnableOrderProfitSharing(orderType string) bool {
	switch orderType {
	case orderTypeDineIn, orderTypeTakeaway:
		return false
	default:
		return true
	}
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
	return svc.closePendingPaymentOrder(ctx, paymentOrder)
}

func (svc *PaymentOrderService) closePendingPaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) (ClosePaymentOrderResult, error) {
	if paymentOrder.CombinedPaymentID.Valid && paymentOrder.PaymentType == "profit_sharing" {
		return svc.closeCombinedPaymentOrder(ctx, paymentOrder)
	}
	if paymentOrder.PaymentType == "profit_sharing" {
		return svc.closePartnerPaymentOrder(ctx, paymentOrder)
	}

	updatedPayment, err := svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
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

func (svc *PaymentOrderService) closePartnerPaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) (ClosePaymentOrderResult, error) {
	if svc.ecommerceClient == nil {
		return ClosePaymentOrderResult{}, fmt.Errorf("ecommerce client: not configured")
	}

	updatedPayment, err := svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
	if err != nil {
		return ClosePaymentOrderResult{}, err
	}

	if paymentOrder.PrepayID.Valid {
		subMchID, resolveErr := svc.resolvePaymentOrderSubMchID(ctx, paymentOrder)
		if resolveErr != nil {
			log.Warn().Err(resolveErr).Str("out_trade_no", paymentOrder.OutTradeNo).Msg("resolve partner payment sub_mchid failed, order will remain locally closed")
			return ClosePaymentOrderResult{PaymentOrder: updatedPayment}, nil
		}
		if err := svc.ecommerceClient.ClosePartnerOrder(ctx, paymentOrder.OutTradeNo, subMchID); err != nil {
			log.Warn().Err(err).Str("out_trade_no", paymentOrder.OutTradeNo).Msg("close partner order failed, order will auto-expire")
		}
	}

	return ClosePaymentOrderResult{PaymentOrder: updatedPayment}, nil
}

func (svc *PaymentOrderService) resolvePaymentOrderSubMchID(ctx context.Context, paymentOrder db.PaymentOrder) (string, error) {
	if attachSubMchID := subMchIDFromPaymentAttach(paymentOrder); attachSubMchID != "" {
		return attachSubMchID, nil
	}

	var merchantID int64

	if paymentOrder.OrderID.Valid {
		order, err := svc.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
		if err != nil {
			return "", fmt.Errorf("get order for payment order %d: %w", paymentOrder.ID, err)
		}
		merchantID = order.MerchantID
	} else if paymentOrder.ReservationID.Valid {
		reservation, err := svc.store.GetTableReservation(ctx, paymentOrder.ReservationID.Int64)
		if err != nil {
			return "", fmt.Errorf("get reservation for payment order %d: %w", paymentOrder.ID, err)
		}
		merchantID = reservation.MerchantID
	} else {
		return "", fmt.Errorf("payment order %d missing order and reservation reference", paymentOrder.ID)
	}

	config, err := svc.store.GetMerchantPaymentConfig(ctx, merchantID)
	if err != nil {
		return "", fmt.Errorf("get merchant payment config for payment order %d: %w", paymentOrder.ID, err)
	}
	if config.SubMchID == "" {
		return "", fmt.Errorf("merchant payment config missing sub_mchid for payment order %d", paymentOrder.ID)
	}

	return config.SubMchID, nil
}

func (svc *PaymentOrderService) closeCombinedPaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) (ClosePaymentOrderResult, error) {
	if svc.ecommerceClient == nil {
		return ClosePaymentOrderResult{}, fmt.Errorf("ecommerce client: not configured")
	}

	combinedPayment, err := svc.store.GetCombinedPaymentOrder(ctx, paymentOrder.CombinedPaymentID.Int64)
	if err != nil {
		return ClosePaymentOrderResult{}, err
	}
	if combinedPayment.Status != paymentStatusPending {
		return ClosePaymentOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("only pending payment orders can be closed"))
	}

	subOrders, err := svc.store.ListCombinedPaymentSubOrders(ctx, combinedPayment.ID)
	if err != nil {
		return ClosePaymentOrderResult{}, err
	}
	closeSubs := make([]wechat.SubOrderClose, 0, len(subOrders))
	subOutTradeNos := make([]string, 0, len(subOrders))
	for _, sub := range subOrders {
		if sub.SubMchid == "" || sub.OutTradeNo == "" {
			continue
		}
		closeSubs = append(closeSubs, wechat.SubOrderClose{SubMchID: sub.SubMchid, OutTradeNo: sub.OutTradeNo})
		subOutTradeNos = append(subOutTradeNos, sub.OutTradeNo)
	}
	if len(closeSubs) == 0 {
		return ClosePaymentOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("no sub orders available to close"))
	}

	if err := svc.ecommerceClient.CloseCombineOrder(ctx, combinedPayment.CombineOutTradeNo, closeSubs); err != nil {
		return ClosePaymentOrderResult{}, err
	}

	if _, err := svc.store.CloseCombinedPaymentOrderTx(ctx, db.CloseCombinedPaymentOrderTxParams{
		CombinedPaymentOrderID: combinedPayment.ID,
		SubOrderOutTradeNos:    subOutTradeNos,
	}); err != nil {
		return ClosePaymentOrderResult{}, err
	}

	updatedPayment, err := svc.store.GetPaymentOrder(ctx, paymentOrder.ID)
	if err != nil {
		return ClosePaymentOrderResult{}, err
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
