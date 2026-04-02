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

		amount, err = db.OrderRemainingPayableAmount(order)
		if err != nil {
			return result, fmt.Errorf("resolve order payable amount: %w", err)
		}
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
		if existingPayment.Amount != amount {
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

	// ==================== 预定走收付通合单支付 ====================
	if input.BusinessType == businessTypeReservation {
		return svc.createReservationEcommercePayment(ctx, input, merchantID, merchantName, amount, attach, expiresAt)
	}
	if input.BusinessType == businessTypeOrder {
		return svc.createOrderEcommercePayment(ctx, input, merchantName, amount, expiresAt)
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

// createReservationEcommercePayment 通过收付通合单支付创建预定支付单
// 预定金/全款皆走子商户收付通，确保资金进入商家的二级商户账户。
func (svc *PaymentOrderService) createReservationEcommercePayment(
	ctx context.Context,
	input CreatePaymentOrderInput,
	merchantID int64,
	merchantName string,
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
			merchantName = merchant.Name + " - Reservation Deposit"
		}
	}

	// 生成合单主单号和子单号
	combineOutTradeNo, err := generateCombineOutTradeNoForSingle("RS")
	if err != nil {
		return result, fmt.Errorf("generate combine out trade no: %w", err)
	}

	var txResult db.CreateEcommercePaymentTxResult
	for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
		outTradeNo, genErr := generateOutTradeNoWithPrefix("RS")
		if genErr != nil {
			return result, fmt.Errorf("generate out trade no: %w", genErr)
		}
		txResult, err = svc.store.CreateEcommercePaymentTx(ctx, db.CreateEcommercePaymentTxParams{
			UserID:            input.UserID,
			MerchantID:        merchantID,
			Amount:            amount,
			BusinessType:      input.BusinessType,
			ReservationID:     input.OrderID,
			CombineOutTradeNo: combineOutTradeNo,
			OutTradeNo:        outTradeNo,
			ExpiresAt:         expiresAt,
			Attach:            attach,
		})
		if err == nil {
			break
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

	// 调用收付通合单支付 API
	combineResp, payParams, err := svc.ecommerceClient.CreateCombineOrder(ctx, &wechat.CombineOrderRequest{
		CombineOutTradeNo: combineOutTradeNo,
		SubOrders: []wechat.SubOrder{
			{
				MchID:       txResult.SubMchID,
				Amount:      amount,
				OutTradeNo:  txResult.PaymentOrder.OutTradeNo,
				Description: merchantName,
				Attach:      attach,
			},
		},
		PayerOpenID: user.WechatOpenid,
		ExpireTime:  expiresAt,
		SceneInfo: &wechat.CombineSceneInfo{
			PayerClientIP: input.ClientIP,
		},
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = svc.store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID)
		_, _ = svc.store.UpdateCombinedPaymentOrderToClosed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		return result, fmt.Errorf("create combine order: %w", err)
	}
	if combineResp == nil || strings.TrimSpace(combineResp.PrepayID) == "" {
		cleanupCtx := context.Background()
		_, _ = svc.store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID)
		_, _ = svc.store.UpdateCombinedPaymentOrderToClosed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		return result, fmt.Errorf("create combine order: empty prepay id")
	}

	// 保存 prepay_id 到 payment_orders（供幂等查询重新签名）
	updatedPayment, err := svc.store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       txResult.PaymentOrder.ID,
		PrepayID: pgtype.Text{String: combineResp.PrepayID, Valid: true},
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = svc.store.UpdatePaymentOrderToFailed(cleanupCtx, txResult.PaymentOrder.ID)
		_, _ = svc.store.UpdateCombinedPaymentOrderToFailed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		if closeErr := svc.ecommerceClient.CloseCombineOrder(cleanupCtx, combineOutTradeNo, []wechat.SubOrderClose{
			{MchID: txResult.SubMchID, OutTradeNo: txResult.PaymentOrder.OutTradeNo},
		}); closeErr != nil {
			log.Warn().Err(closeErr).Str("combine_out_trade_no", combineOutTradeNo).Msg("close combine order after prepay update failure")
		}
		return result, fmt.Errorf("update prepay id: %w", err)
	}

	// 同步 prepay_id 到合单主记录
	_, _ = svc.store.UpdateCombinedPaymentOrderPrepay(ctx, db.UpdateCombinedPaymentOrderPrepayParams{
		ID:       txResult.CombinedPaymentOrder.ID,
		PrepayID: pgtype.Text{String: combineResp.PrepayID, Valid: true},
	})

	result.PaymentOrder = updatedPayment
	result.PayParams = payParams
	return result, nil
}

func (svc *PaymentOrderService) createOrderEcommercePayment(
	ctx context.Context,
	input CreatePaymentOrderInput,
	merchantName string,
	expectedAmount int64,
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

	combineOutTradeNo, err := generateCombineOutTradeNoForSingle("OC")
	if err != nil {
		return result, fmt.Errorf("generate combine out trade no: %w", err)
	}

	var txResult db.CreateCombinedPaymentTxResult
	for attempt := 1; attempt <= concurrentPaymentRetry; attempt++ {
		txResult, err = svc.store.CreateCombinedPaymentTx(ctx, db.CreateCombinedPaymentTxParams{
			UserID:            input.UserID,
			OrderIDs:          []int64{input.OrderID},
			CombineOutTradeNo: combineOutTradeNo,
			ExpiresAt:         expiresAt,
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
		if mapped := mapCombinedPaymentError(err); mapped != nil {
			return result, mapped
		}
		return result, fmt.Errorf("create combined payment: %w", err)
	}
	if len(txResult.OrderInfos) == 0 {
		return result, fmt.Errorf("create combined payment: empty order infos")
	}

	info := txResult.OrderInfos[0]
	if info.Merchant.Name != "" {
		merchantName = info.Merchant.Name + " - Order Payment"
	}

	combineResp, payParams, err := svc.ecommerceClient.CreateCombineOrder(ctx, &wechat.CombineOrderRequest{
		CombineOutTradeNo: combineOutTradeNo,
		SubOrders: []wechat.SubOrder{
			{
				MchID:       info.PaymentConfig.SubMchID,
				Amount:      info.PaymentOrder.Amount,
				OutTradeNo:  info.PaymentOrder.OutTradeNo,
				Description: merchantName,
				Attach:      info.PaymentOrder.Attach.String,
			},
		},
		PayerOpenID: user.WechatOpenid,
		ExpireTime:  expiresAt,
		SceneInfo: &wechat.CombineSceneInfo{
			PayerClientIP: input.ClientIP,
		},
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = svc.store.UpdatePaymentOrderToClosed(cleanupCtx, info.PaymentOrder.ID)
		_, _ = svc.store.UpdateCombinedPaymentOrderToClosed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		return result, fmt.Errorf("create combine order: %w", err)
	}
	if combineResp == nil || strings.TrimSpace(combineResp.PrepayID) == "" {
		cleanupCtx := context.Background()
		_, _ = svc.store.UpdatePaymentOrderToClosed(cleanupCtx, info.PaymentOrder.ID)
		_, _ = svc.store.UpdateCombinedPaymentOrderToClosed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		return result, fmt.Errorf("create combine order: empty prepay id")
	}

	updatedPayment, err := svc.store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       info.PaymentOrder.ID,
		PrepayID: pgtype.Text{String: combineResp.PrepayID, Valid: true},
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = svc.store.UpdatePaymentOrderToFailed(cleanupCtx, info.PaymentOrder.ID)
		_, _ = svc.store.UpdateCombinedPaymentOrderToFailed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		if closeErr := svc.ecommerceClient.CloseCombineOrder(cleanupCtx, combineOutTradeNo, []wechat.SubOrderClose{{
			MchID:      info.PaymentConfig.SubMchID,
			OutTradeNo: info.PaymentOrder.OutTradeNo,
		}}); closeErr != nil {
			log.Warn().Err(closeErr).Str("combine_out_trade_no", combineOutTradeNo).Msg("close combine order after prepay update failure")
		}
		return result, fmt.Errorf("update prepay id: %w", err)
	}

	_, _ = svc.store.UpdateCombinedPaymentOrderPrepay(ctx, db.UpdateCombinedPaymentOrderPrepayParams{
		ID:       txResult.CombinedPaymentOrder.ID,
		PrepayID: pgtype.Text{String: combineResp.PrepayID, Valid: true},
	})

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
		if result.PayParams != nil || attempt == outTradeNoMaxRetry {
			return result, true, nil
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
	msg := err.Error()
	if strings.Contains(msg, "payment config invalid") || strings.Contains(msg, "inactive") {
		return NewRequestError(http.StatusBadRequest, errors.New("merchant payment config invalid or not activated"))
	}
	return fmt.Errorf("create ecommerce payment: %w", err)
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
		closeSubs = append(closeSubs, wechat.SubOrderClose{MchID: sub.SubMchid, OutTradeNo: sub.OutTradeNo})
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
