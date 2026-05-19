package logic

import (
	"context"
	"encoding/json"
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
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
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
	store                      db.Store
	directPaymentClient        wechat.DirectPaymentClientInterface
	ecommerceClient            wechat.EcommerceClientInterface
	ordinaryProviderPayClient  ordinaryServiceProviderPaymentClient
	baofuPaymentService        *BaofuPaymentService
	mainBusinessPaymentChannel string
	now                        func() time.Time
}

type ordinaryServiceProviderPaymentClient interface {
	ServiceProviderAppID() string
	ServiceProviderMchID() string
	PaymentNotifyURL() string
	CreatePayment(ctx context.Context, req ospcontracts.PaymentPrepayRequest) (*ospcontracts.PaymentPrepayResponse, error)
	QueryPayment(ctx context.Context, req ospcontracts.PaymentQueryRequest) (*ospcontracts.PaymentQueryResponse, error)
	ClosePayment(ctx context.Context, req ospcontracts.PaymentCloseRequest) error
	CloseCombinePayment(ctx context.Context, req ospcontracts.CombineCloseRequest) error
	GenerateJSAPIPayParams(prepayID string) (*ospcontracts.JSAPIPayParams, error)
}

// NewPaymentOrderService creates a payment order service.
func NewPaymentOrderService(store db.Store, ecommerceClient wechat.EcommerceClientInterface) *PaymentOrderService {
	return NewPaymentOrderServiceWithClients(store, nil, ecommerceClient)
}

// NewPaymentOrderServiceWithClients creates a payment order service with all payment clients.
func NewPaymentOrderServiceWithClients(store db.Store, directPaymentClient wechat.DirectPaymentClientInterface, ecommerceClient wechat.EcommerceClientInterface) *PaymentOrderService {
	return &PaymentOrderService{
		store:                      store,
		directPaymentClient:        directPaymentClient,
		ecommerceClient:            ecommerceClient,
		mainBusinessPaymentChannel: db.PaymentChannelEcommerce,
		now:                        time.Now,
	}
}

func NewPaymentOrderServiceWithOrdinaryServiceProvider(store db.Store, directPaymentClient wechat.DirectPaymentClientInterface, ordinaryClient ordinaryServiceProviderPaymentClient) *PaymentOrderService {
	return &PaymentOrderService{
		store:                      store,
		directPaymentClient:        directPaymentClient,
		ordinaryProviderPayClient:  ordinaryClient,
		mainBusinessPaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		now:                        time.Now,
	}
}

func NewPaymentOrderServiceWithBaofu(store db.Store, directPaymentClient wechat.DirectPaymentClientInterface, baofuPaymentService *BaofuPaymentService) *PaymentOrderService {
	return &PaymentOrderService{
		store:                      store,
		directPaymentClient:        directPaymentClient,
		baofuPaymentService:        baofuPaymentService,
		mainBusinessPaymentChannel: db.PaymentChannelBaofuAggregate,
		now:                        time.Now,
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

type QueryPaymentOrderInput struct {
	UserID         int64
	PaymentOrderID int64
}

type QueryPaymentOrderResult struct {
	PaymentOrder db.PaymentOrder
	PayParams    *wechat.JSAPIPayParams
	WechatOrder  *QueryPaymentOrderWechatOrder
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
		if !svc.paymentOrderUsesMainBusinessChannel(existingPayment) {
			if closeErr := svc.supersedePendingPaymentOrder(ctx, existingPayment); closeErr != nil {
				return result, closeErr
			}
		} else if input.BusinessType == businessTypeReservation {
			if !shouldReuseReservationPendingPayment(existingPayment, amount, attach) {
				if closeErr := svc.supersedePendingPaymentOrder(ctx, existingPayment); closeErr != nil {
					return result, closeErr
				}
			} else {
				result.PaymentOrder = existingPayment
				result.PayParams, err = svc.signExistingPaymentOrder(existingPayment)
				if err != nil {
					return result, fmt.Errorf("sign existing payment order: %w", err)
				}
				return result, nil
			}
		} else if existingPayment.Amount != amount {
			if closeErr := svc.supersedePendingPaymentOrder(ctx, existingPayment); closeErr != nil {
				return result, closeErr
			}
		} else {
			result.PaymentOrder = existingPayment
			result.PayParams, err = svc.signExistingPaymentOrder(existingPayment)
			if err != nil {
				return result, fmt.Errorf("sign existing payment order: %w", err)
			}
			return result, nil
		}
	}

	expiresAt := svc.now().Add(30 * time.Minute)

	if svc.mainBusinessPaymentChannel == db.PaymentChannelOrdinaryServiceProvider {
		if input.BusinessType == businessTypeReservation {
			return svc.createReservationOrdinaryServiceProviderPayment(ctx, input, merchantID, merchantName, reservationPaymentMode, amount, attach, expiresAt)
		}
		if input.BusinessType == businessTypeOrder {
			return svc.createOrderOrdinaryServiceProviderPayment(ctx, input, merchantID, merchantName, reservationLinkedOrder || shouldEnableOrderProfitSharing(orderType), amount, attach, expiresAt)
		}
	}
	if svc.mainBusinessPaymentChannel == db.PaymentChannelBaofuAggregate {
		if input.BusinessType == businessTypeReservation {
			return svc.createReservationBaofuPayment(ctx, input, merchantID, merchantName, reservationPaymentMode, amount, attach, expiresAt)
		}
		if input.BusinessType == businessTypeOrder {
			return svc.createOrderBaofuPayment(ctx, input, merchantID, merchantName, amount, attach, expiresAt)
		}
	}

	// ==================== 历史收付通冷备路径 ====================
	if input.BusinessType == businessTypeReservation {
		return svc.createReservationEcommercePayment(ctx, input, merchantID, merchantName, reservationPaymentMode, amount, attach, expiresAt)
	}
	if input.BusinessType == businessTypeOrder {
		return svc.createOrderEcommercePayment(ctx, input, merchantID, merchantName, reservationLinkedOrder || shouldEnableOrderProfitSharing(orderType), amount, attach, expiresAt)
	}

	return result, fmt.Errorf("unsupported business type after validation: %s", input.BusinessType)
}

func (svc *PaymentOrderService) paymentOrderUsesMainBusinessChannel(paymentOrder db.PaymentOrder) bool {
	switch svc.mainBusinessPaymentChannel {
	case db.PaymentChannelOrdinaryServiceProvider:
		return db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder)
	case db.PaymentChannelBaofuAggregate:
		return paymentOrder.PaymentChannel == db.PaymentChannelBaofuAggregate
	case db.PaymentChannelEcommerce:
		return paymentOrderUsesEcommerceChannel(paymentOrder)
	default:
		return false
	}
}

func (svc *PaymentOrderService) createReservationOrdinaryServiceProviderPayment(
	ctx context.Context,
	input CreatePaymentOrderInput,
	merchantID int64,
	merchantName string,
	paymentMode string,
	amount int64,
	attach string,
	expiresAt time.Time,
) (CreatePaymentOrderResult, error) {
	if svc.ordinaryProviderPayClient == nil {
		return CreatePaymentOrderResult{}, fmt.Errorf("ordinary service provider client: not configured")
	}
	if err := ensureMerchantBaofuReadyForPayment(ctx, svc.store, merchantID); err != nil {
		return CreatePaymentOrderResult{}, err
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
	return svc.createOrdinaryServiceProviderPayment(ctx, ordinaryPaymentCreateInput{
		CreatePaymentOrderInput: input,
		MerchantID:              merchantID,
		MerchantName:            merchantName,
		PaymentMode:             paymentMode,
		Amount:                  amount,
		Attach:                  attach,
		ExpiresAt:               expiresAt,
		BusinessOwner:           db.ExternalPaymentBusinessOwnerReservation,
		ProfitSharing:           true,
	})
}

func (svc *PaymentOrderService) createOrderOrdinaryServiceProviderPayment(
	ctx context.Context,
	input CreatePaymentOrderInput,
	merchantID int64,
	merchantName string,
	profitSharing bool,
	expectedAmount int64,
	attach string,
	expiresAt time.Time,
) (CreatePaymentOrderResult, error) {
	if svc.ordinaryProviderPayClient == nil {
		return CreatePaymentOrderResult{}, fmt.Errorf("ordinary service provider client: not configured")
	}
	if err := ensureMerchantBaofuReadyForPayment(ctx, svc.store, merchantID); err != nil {
		return CreatePaymentOrderResult{}, err
	}
	if merchantID > 0 {
		merchant, err := svc.store.GetMerchant(ctx, merchantID)
		if err == nil {
			merchantName = merchant.Name + " - Order Payment"
		}
	}
	return svc.createOrdinaryServiceProviderPayment(ctx, ordinaryPaymentCreateInput{
		CreatePaymentOrderInput: input,
		MerchantID:              merchantID,
		MerchantName:            merchantName,
		Amount:                  expectedAmount,
		Attach:                  attach,
		ExpiresAt:               expiresAt,
		BusinessOwner:           db.ExternalPaymentBusinessOwnerOrder,
		ProfitSharing:           profitSharing,
	})
}

type ordinaryPaymentCreateInput struct {
	CreatePaymentOrderInput
	MerchantID    int64
	MerchantName  string
	PaymentMode   string
	Amount        int64
	Attach        string
	ExpiresAt     time.Time
	BusinessOwner string
	ProfitSharing bool
}

func (svc *PaymentOrderService) createOrdinaryServiceProviderPayment(ctx context.Context, createInput ordinaryPaymentCreateInput) (CreatePaymentOrderResult, error) {
	var result CreatePaymentOrderResult
	if svc.ordinaryProviderPayClient == nil {
		return result, fmt.Errorf("ordinary service provider client: not configured")
	}

	user, err := svc.store.GetUser(ctx, createInput.UserID)
	if err != nil {
		return result, fmt.Errorf("get user: %w", err)
	}
	if user.WechatOpenid == "" {
		return result, NewRequestError(http.StatusBadRequest, errors.New("wechat openid not found"))
	}

	var txResult db.CreatePartnerPaymentTxResult
	for attempt := 1; attempt <= concurrentPaymentRetry; attempt++ {
		prefix := "OC"
		orderID := createInput.OrderID
		reservationID := int64(0)
		if createInput.BusinessType == businessTypeReservation {
			prefix = "RS"
			orderID = 0
			reservationID = createInput.OrderID
		}
		outTradeNo, genErr := generateOutTradeNoWithPrefix(prefix)
		if genErr != nil {
			return result, fmt.Errorf("generate out trade no: %w", genErr)
		}
		txResult, err = svc.store.CreatePartnerPaymentTx(ctx, db.CreatePartnerPaymentTxParams{
			UserID:        createInput.UserID,
			MerchantID:    createInput.MerchantID,
			OrderID:       orderID,
			ReservationID: reservationID,
			PaymentMode:   createInput.PaymentMode,
			BusinessType:  createInput.BusinessType,
			Amount:        createInput.Amount,
			OutTradeNo:    outTradeNo,
			ExpiresAt:     createInput.ExpiresAt,
			Attach:        createInput.Attach,
		})
		if err == nil {
			break
		}
		if errors.Is(err, db.ErrOrderPendingPaymentConflict) {
			if createInput.BusinessType == businessTypeReservation {
				resolved, handled, resolveErr := svc.resolveConcurrentReservationPayment(ctx, createInput.CreatePaymentOrderInput, createInput.Amount, createInput.Attach)
				if resolveErr != nil {
					return result, resolveErr
				}
				if handled {
					return resolved, nil
				}
			} else {
				resolved, handled, resolveErr := svc.resolveConcurrentOrderPayment(ctx, createInput.CreatePaymentOrderInput, createInput.Amount)
				if resolveErr != nil {
					return result, resolveErr
				}
				if handled {
					return resolved, nil
				}
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

	result.PaymentOrder = txResult.PaymentOrder
	paymentAttach := createInput.Attach
	if txResult.PaymentOrder.Attach.Valid && strings.TrimSpace(txResult.PaymentOrder.Attach.String) != "" {
		paymentAttach = txResult.PaymentOrder.Attach.String
	}

	orderResp, err := svc.ordinaryProviderPayClient.CreatePayment(ctx, ospcontracts.PaymentPrepayRequest{
		SpAppID:     svc.ordinaryProviderPayClient.ServiceProviderAppID(),
		SpMchID:     svc.ordinaryProviderPayClient.ServiceProviderMchID(),
		SubMchID:    txResult.SubMchID,
		Description: createInput.MerchantName,
		OutTradeNo:  txResult.PaymentOrder.OutTradeNo,
		TimeExpire:  createInput.ExpiresAt.Format(time.RFC3339),
		Attach:      paymentAttach,
		NotifyURL:   svc.ordinaryProviderPayClient.PaymentNotifyURL(),
		SettleInfo:  &ospcontracts.PaymentSettleInfo{ProfitSharing: createInput.ProfitSharing},
		Amount:      ospcontracts.PaymentPrepayAmount{Total: txResult.PaymentOrder.Amount, Currency: ospcontracts.CurrencyCNY},
		Payer:       ospcontracts.PaymentPayer{SpOpenID: user.WechatOpenid},
		SceneInfo:   &ospcontracts.PaymentSceneInfo{PayerClientIP: createInput.ClientIP},
	})
	if err != nil {
		cleanupCtx := context.Background()
		if _, closeErr := svc.store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID); closeErr != nil {
			log.Error().Err(closeErr).Int64("payment_order_id", txResult.PaymentOrder.ID).Msg("failed to close ordinary service provider payment order after create rejection")
		} else {
			recordPartnerJSAPIPaymentCommandRejected(cleanupCtx, svc.store, txResult.PaymentOrder, createInput.BusinessOwner, err)
		}
		return result, mapPartnerJSAPIOrderCreateError(err)
	}
	if orderResp == nil || strings.TrimSpace(orderResp.PrepayID) == "" {
		cleanupCtx := context.Background()
		emptyPrepayErr := errors.New("create ordinary service provider payment: empty prepay id")
		if _, closeErr := svc.store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID); closeErr != nil {
			log.Error().Err(closeErr).Int64("payment_order_id", txResult.PaymentOrder.ID).Msg("failed to close ordinary service provider payment order after empty prepay id")
		} else {
			recordPartnerJSAPIPaymentCommandRejected(cleanupCtx, svc.store, txResult.PaymentOrder, createInput.BusinessOwner, emptyPrepayErr)
		}
		return result, mapPartnerJSAPIOrderCreateError(emptyPrepayErr)
	}

	payParams, err := svc.ordinaryProviderPayClient.GenerateJSAPIPayParams(orderResp.PrepayID)
	if err != nil {
		return result, fmt.Errorf("generate ordinary service provider pay params: %w", err)
	}

	updatedPayment, err := svc.store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       txResult.PaymentOrder.ID,
		PrepayID: pgtype.Text{String: orderResp.PrepayID, Valid: true},
	})
	if err != nil {
		cleanupCtx := context.Background()
		svc.markPaymentOrderFailedForCleanup(cleanupCtx, txResult.PaymentOrder.ID, "failed to mark ordinary service provider payment order failed after prepay update failure")
		if closeErr := svc.ordinaryProviderPayClient.ClosePayment(cleanupCtx, ospcontracts.PaymentCloseRequest{
			SpMchID:    svc.ordinaryProviderPayClient.ServiceProviderMchID(),
			SubMchID:   txResult.SubMchID,
			OutTradeNo: txResult.PaymentOrder.OutTradeNo,
		}); closeErr != nil {
			log.Warn().Err(closeErr).Str("out_trade_no", txResult.PaymentOrder.OutTradeNo).Msg("close ordinary service provider order after prepay update failure")
		}
		return result, fmt.Errorf("update prepay id: %w", err)
	}
	recordPartnerJSAPIPaymentCommandAccepted(ctx, svc.store, txResult.PaymentOrder, createInput.BusinessOwner, orderResp.PrepayID)

	result.PaymentOrder = updatedPayment
	result.PayParams = ordinaryJSAPIPayParamsToWechat(payParams)
	return result, nil
}

func (svc *PaymentOrderService) markPaymentOrderFailedForCleanup(ctx context.Context, paymentOrderID int64, message string) {
	if _, err := svc.store.UpdatePaymentOrderToFailed(ctx, paymentOrderID); err != nil {
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrderID).
			Msg(message)
	}
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

	orderResp, payParams, err := svc.ecommerceClient.CreatePartnerJSAPIOrder(ctx, &wechatcontracts.PartnerJSAPIOrderRequest{
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
		if _, closeErr := svc.store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID); closeErr != nil {
			log.Error().Err(closeErr).Int64("payment_order_id", txResult.PaymentOrder.ID).Msg("failed to close reservation payment order after create rejection")
		} else {
			recordPartnerJSAPIPaymentCommandRejected(cleanupCtx, svc.store, txResult.PaymentOrder, db.ExternalPaymentBusinessOwnerReservation, err)
		}
		return result, mapPartnerJSAPIOrderCreateError(err)
	}
	if orderResp == nil || strings.TrimSpace(orderResp.PrepayID) == "" {
		cleanupCtx := context.Background()
		emptyPrepayErr := errors.New("create partner jsapi order: empty prepay id")
		if _, closeErr := svc.store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID); closeErr != nil {
			log.Error().Err(closeErr).Int64("payment_order_id", txResult.PaymentOrder.ID).Msg("failed to close reservation payment order after empty prepay id")
		} else {
			recordPartnerJSAPIPaymentCommandRejected(cleanupCtx, svc.store, txResult.PaymentOrder, db.ExternalPaymentBusinessOwnerReservation, emptyPrepayErr)
		}
		return result, mapPartnerJSAPIOrderCreateError(emptyPrepayErr)
	}

	updatedPayment, err := svc.store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       txResult.PaymentOrder.ID,
		PrepayID: pgtype.Text{String: orderResp.PrepayID, Valid: true},
	})
	if err != nil {
		cleanupCtx := context.Background()
		svc.markPaymentOrderFailedForCleanup(cleanupCtx, txResult.PaymentOrder.ID, "failed to mark ecommerce reservation payment order failed after prepay update failure")
		if closeErr := svc.ecommerceClient.ClosePartnerOrder(cleanupCtx, txResult.PaymentOrder.OutTradeNo, txResult.SubMchID); closeErr != nil {
			log.Warn().Err(closeErr).Str("out_trade_no", txResult.PaymentOrder.OutTradeNo).Msg("close partner order after prepay update failure")
		}
		return result, fmt.Errorf("update prepay id: %w", err)
	}
	recordPartnerJSAPIPaymentCommandAccepted(ctx, svc.store, txResult.PaymentOrder, db.ExternalPaymentBusinessOwnerReservation, orderResp.PrepayID)

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

	orderResp, payParams, err := svc.ecommerceClient.CreatePartnerJSAPIOrder(ctx, &wechatcontracts.PartnerJSAPIOrderRequest{
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
		if _, closeErr := svc.store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID); closeErr != nil {
			log.Error().Err(closeErr).Int64("payment_order_id", txResult.PaymentOrder.ID).Msg("failed to close order payment order after create rejection")
		} else {
			recordPartnerJSAPIPaymentCommandRejected(cleanupCtx, svc.store, txResult.PaymentOrder, db.ExternalPaymentBusinessOwnerOrder, err)
		}
		return result, mapPartnerJSAPIOrderCreateError(err)
	}
	if orderResp == nil || strings.TrimSpace(orderResp.PrepayID) == "" {
		cleanupCtx := context.Background()
		emptyPrepayErr := errors.New("create partner jsapi order: empty prepay id")
		if _, closeErr := svc.store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID); closeErr != nil {
			log.Error().Err(closeErr).Int64("payment_order_id", txResult.PaymentOrder.ID).Msg("failed to close order payment order after empty prepay id")
		} else {
			recordPartnerJSAPIPaymentCommandRejected(cleanupCtx, svc.store, txResult.PaymentOrder, db.ExternalPaymentBusinessOwnerOrder, emptyPrepayErr)
		}
		return result, mapPartnerJSAPIOrderCreateError(emptyPrepayErr)
	}

	updatedPayment, err := svc.store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       txResult.PaymentOrder.ID,
		PrepayID: pgtype.Text{String: orderResp.PrepayID, Valid: true},
	})
	if err != nil {
		cleanupCtx := context.Background()
		svc.markPaymentOrderFailedForCleanup(cleanupCtx, txResult.PaymentOrder.ID, "failed to mark ecommerce payment order failed after prepay update failure")
		if closeErr := svc.ecommerceClient.ClosePartnerOrder(cleanupCtx, txResult.PaymentOrder.OutTradeNo, txResult.SubMchID); closeErr != nil {
			log.Warn().Err(closeErr).Str("out_trade_no", txResult.PaymentOrder.OutTradeNo).Msg("close partner order after prepay update failure")
		}
		return result, fmt.Errorf("update prepay id: %w", err)
	}
	recordPartnerJSAPIPaymentCommandAccepted(ctx, svc.store, txResult.PaymentOrder, db.ExternalPaymentBusinessOwnerOrder, orderResp.PrepayID)

	result.PaymentOrder = updatedPayment
	result.PayParams = payParams
	return result, nil
}

func recordPartnerJSAPIPaymentCommandAccepted(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, businessOwner string, prepayID string) {
	paymentCommandSvc := NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbPartnerJSAPIPaymentCommandInput(
		paymentOrder,
		businessOwner,
		db.ExternalPaymentCommandStatusAccepted,
		stringPtrIfNotEmpty(prepayID),
		nil,
		nil,
		partnerJSAPIPaymentCommandSnapshot(map[string]string{
			"out_trade_no": paymentOrder.OutTradeNo,
			"prepay_id":    prepayID,
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Msg("record partner jsapi payment command accepted failed")
	}
}

func recordPartnerJSAPIPaymentCommandRejected(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, businessOwner string, paymentErr error) {
	paymentCommandSvc := NewPaymentCommandService(store)
	errorCode, errorMessage := partnerPaymentCommandErrorFields(paymentErr)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbPartnerJSAPIPaymentCommandInput(
		paymentOrder,
		businessOwner,
		db.ExternalPaymentCommandStatusRejected,
		nil,
		errorCode,
		errorMessage,
		partnerJSAPIPaymentCommandSnapshot(map[string]string{
			"out_trade_no":  paymentOrder.OutTradeNo,
			"error_code":    stringValue(errorCode),
			"error_message": stringValue(errorMessage),
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Msg("record partner jsapi payment command rejected failed")
	}
}

func dbPartnerJSAPIPaymentCommandInput(
	paymentOrder db.PaymentOrder,
	businessOwner string,
	commandStatus string,
	externalSecondaryKey *string,
	lastErrorCode *string,
	lastErrorMessage *string,
	responseSnapshot []byte,
) RecordExternalPaymentCommandInput {
	businessObjectType := "payment_order"
	businessObjectID := paymentOrder.ID
	return RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              paymentOrder.PaymentChannel,
		Capability:           db.ExternalPaymentCapabilityPartnerJSAPIPayment,
		CommandType:          db.ExternalPaymentCommandTypeCreatePayment,
		BusinessOwner:        partnerJSAPIPaymentBusinessOwner(paymentOrder, businessOwner),
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    paymentOrder.OutTradeNo,
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        commandStatus,
		LastErrorCode:        lastErrorCode,
		LastErrorMessage:     lastErrorMessage,
		ResponseSnapshot:     responseSnapshot,
	}
}

func partnerJSAPIPaymentBusinessOwner(paymentOrder db.PaymentOrder, businessOwner string) string {
	if businessOwner != "" {
		return businessOwner
	}
	if paymentOrder.BusinessType == businessTypeReservation || paymentOrder.ReservationID.Valid {
		return db.ExternalPaymentBusinessOwnerReservation
	}
	return db.ExternalPaymentBusinessOwnerOrder
}

func partnerPaymentCommandErrorFields(err error) (*string, *string) {
	loggableErr := LoggableError(err)
	var wxErr *wechat.WechatPayError
	if errors.As(loggableErr, &wxErr) {
		return stringPtrIfNotEmpty(wxErr.Code), stringPtrIfNotEmpty(wxErr.Message)
	}
	var ordinaryErr *ordinaryserviceprovider.ProviderError
	if errors.As(loggableErr, &ordinaryErr) {
		return stringPtrIfNotEmpty(ordinaryErr.ProviderCode), stringPtrIfNotEmpty(ordinaryProviderFrontendCommandMessage(ordinaryErr.Frontend))
	}
	if loggableErr == nil {
		return nil, nil
	}
	return nil, stringPtrIfNotEmpty(loggableErr.Error())
}

func ordinaryProviderFrontendCommandMessage(frontend ordinaryserviceprovider.FrontendGuidance) string {
	message := strings.TrimSpace(frontend.Message)
	if action := strings.TrimSpace(frontend.Action); action != "" {
		message = strings.TrimSpace(message + "，" + action)
	}
	return message
}

func partnerJSAPIPaymentCommandSnapshot(values map[string]string) []byte {
	filtered := make(map[string]string, len(values))
	for key, value := range values {
		if value != "" {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return []byte(`{}`)
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func (svc *PaymentOrderService) signExistingPaymentOrder(paymentOrder db.PaymentOrder) (*wechat.JSAPIPayParams, error) {
	if !paymentOrder.PrepayID.Valid {
		return nil, nil
	}
	if paymentOrderUsesEcommerceChannel(paymentOrder) && svc.ecommerceClient != nil {
		payParams, err := svc.ecommerceClient.GenerateJSAPIPayParams(paymentOrder.PrepayID.String)
		if err != nil {
			return nil, err
		}
		return payParams, nil
	}
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) && svc.ordinaryProviderPayClient != nil {
		payParams, err := svc.ordinaryProviderPayClient.GenerateJSAPIPayParams(paymentOrder.PrepayID.String)
		if err != nil {
			return nil, err
		}
		return ordinaryJSAPIPayParamsToWechat(payParams), nil
	}

	return nil, nil
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
		result.PayParams, err = svc.signExistingPaymentOrder(paymentOrder)
		if err != nil {
			return result, true, fmt.Errorf("sign concurrent pending payment order: %w", err)
		}
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
		result.PayParams, err = svc.signExistingPaymentOrder(paymentOrder)
		if err != nil {
			return result, true, fmt.Errorf("sign concurrent reservation payment order: %w", err)
		}
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
		return NewRequestError(http.StatusBadRequest, errors.New("商户支付配置无效或尚未启用，请联系平台处理"))
	case strings.Contains(msg, "does not belong to user"):
		return NewRequestError(http.StatusForbidden, errors.New("当前支付对象不属于你"))
	case strings.Contains(msg, "status is") || strings.Contains(msg, "expect pending"):
		return NewRequestError(http.StatusBadRequest, errors.New("当前支付对象已不在待支付状态，请刷新页面确认"))
	case strings.Contains(msg, "payable amount changed") || strings.Contains(msg, "payment mode changed"):
		return NewRequestError(http.StatusConflict, errors.New("支付金额或支付模式已变化，请返回订单页重新发起支付"))
	case strings.Contains(msg, "has pending payment order"):
		return NewRequestError(http.StatusConflict, errors.New("已有待支付订单，请先刷新支付结果后再决定是否重试"))
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

func (svc *PaymentOrderService) queryPartnerPaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder, subMchID string) (*wechatcontracts.PartnerOrderQueryResponse, error) {
	if paymentOrder.TransactionID.Valid && strings.TrimSpace(paymentOrder.TransactionID.String) != "" {
		return svc.ecommerceClient.QueryPartnerOrderByTransactionID(ctx, paymentOrder.TransactionID.String, subMchID)
	}

	return svc.ecommerceClient.QueryPartnerOrderByOutTradeNo(ctx, paymentOrder.OutTradeNo, subMchID)
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
	if paymentOrderUsesBaofuAggregateChannel(paymentOrder) {
		return svc.closeBaofuAggregatePaymentOrder(ctx, paymentOrder)
	}
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if paymentOrder.CombinedPaymentID.Valid {
			return svc.closeCombinedPaymentOrder(ctx, paymentOrder)
		}
		return svc.closeOrdinaryServiceProviderPaymentOrder(ctx, paymentOrder)
	}
	if paymentOrder.CombinedPaymentID.Valid && paymentOrderUsesEcommerceChannel(paymentOrder) {
		return svc.closeCombinedPaymentOrder(ctx, paymentOrder)
	}
	if paymentOrderUsesEcommerceChannel(paymentOrder) {
		return svc.closePartnerPaymentOrder(ctx, paymentOrder)
	}

	updatedPayment, err := svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
	if err != nil {
		return ClosePaymentOrderResult{}, err
	}
	if paymentOrder.PrepayID.Valid {
		log.Warn().
			Int64("payment_order_id", paymentOrder.ID).
			Str("payment_type", paymentOrder.PaymentType).
			Msg("close pending main-business payment order without partner remote close because payment type is not profit_sharing")
	}

	return ClosePaymentOrderResult{PaymentOrder: updatedPayment}, nil
}

func (svc *PaymentOrderService) closeOrdinaryServiceProviderPaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) (ClosePaymentOrderResult, error) {
	if svc.ordinaryProviderPayClient == nil {
		return ClosePaymentOrderResult{}, fmt.Errorf("ordinary service provider client: not configured")
	}
	subMchID, resolveErr := svc.resolvePaymentOrderSubMchID(ctx, paymentOrder)
	if resolveErr != nil {
		return ClosePaymentOrderResult{}, fmt.Errorf("resolve payment order sub_mchid: %w", resolveErr)
	}
	if err := svc.ordinaryProviderPayClient.ClosePayment(ctx, ospcontracts.PaymentCloseRequest{
		SpMchID:    svc.ordinaryProviderPayClient.ServiceProviderMchID(),
		SubMchID:   subMchID,
		OutTradeNo: paymentOrder.OutTradeNo,
	}); err != nil {
		return ClosePaymentOrderResult{}, mapPartnerOrderCloseError(err)
	}
	updatedPayment, err := svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
	if err != nil {
		return ClosePaymentOrderResult{}, err
	}
	return ClosePaymentOrderResult{PaymentOrder: updatedPayment}, nil
}

func (svc *PaymentOrderService) closePartnerPaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) (ClosePaymentOrderResult, error) {
	if svc.ecommerceClient == nil {
		return ClosePaymentOrderResult{}, fmt.Errorf("ecommerce client: not configured")
	}

	subMchID, resolveErr := svc.resolvePaymentOrderSubMchID(ctx, paymentOrder)
	if resolveErr != nil {
		return ClosePaymentOrderResult{}, fmt.Errorf("resolve payment order sub_mchid: %w", resolveErr)
	}
	if err := svc.ecommerceClient.ClosePartnerOrder(ctx, paymentOrder.OutTradeNo, subMchID); err != nil {
		if !hasWechatPaymentCode(err, "ORDER_CLOSED") {
			return ClosePaymentOrderResult{}, mapPartnerOrderCloseError(err)
		}
	}

	updatedPayment, err := svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
	if err != nil {
		return ClosePaymentOrderResult{}, err
	}

	return ClosePaymentOrderResult{PaymentOrder: updatedPayment}, nil
}

func (svc *PaymentOrderService) signExistingPartnerPaymentOrder(paymentOrder db.PaymentOrder) (*wechat.JSAPIPayParams, error) {
	if !paymentOrder.PrepayID.Valid || strings.TrimSpace(paymentOrder.PrepayID.String) == "" {
		return nil, nil
	}
	if svc.ecommerceClient == nil {
		return nil, nil
	}
	return svc.ecommerceClient.GenerateJSAPIPayParams(paymentOrder.PrepayID.String)
}

func (svc *PaymentOrderService) shouldExposePartnerPaymentPayParams(paymentOrder db.PaymentOrder, wechatOrder *wechatcontracts.PartnerOrderQueryResponse) bool {
	if paymentOrder.Status != paymentStatusPending {
		return false
	}
	if !paymentOrder.PrepayID.Valid || strings.TrimSpace(paymentOrder.PrepayID.String) == "" {
		return false
	}
	if paymentOrder.ExpiresAt.Valid && !svc.now().Before(paymentOrder.ExpiresAt.Time) {
		return false
	}
	if wechatOrder == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(wechatOrder.TradeState), "NOTPAY")
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
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if svc.ordinaryProviderPayClient == nil {
			return ClosePaymentOrderResult{}, fmt.Errorf("ordinary service provider client: not configured")
		}
	} else if svc.ecommerceClient == nil {
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
	closeSubs := make([]wechatcontracts.SubOrderClose, 0, len(subOrders))
	ordinaryCloseSubs := make([]ospcontracts.CombineCloseSubOrder, 0, len(subOrders))
	subOutTradeNos := make([]string, 0, len(subOrders))
	usesOrdinary := db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder)
	ordinaryMchID := ""
	if usesOrdinary {
		ordinaryMchID = svc.ordinaryProviderPayClient.ServiceProviderMchID()
	}
	for _, sub := range subOrders {
		if sub.SubMchid == "" || sub.OutTradeNo == "" {
			continue
		}
		closeSubs = append(closeSubs, wechatcontracts.SubOrderClose{SubMchID: sub.SubMchid, OutTradeNo: sub.OutTradeNo})
		if usesOrdinary {
			ordinaryCloseSubs = append(ordinaryCloseSubs, ospcontracts.CombineCloseSubOrder{
				MchID:      ordinaryMchID,
				SubMchID:   sub.SubMchid,
				OutTradeNo: sub.OutTradeNo,
			})
		}
		subOutTradeNos = append(subOutTradeNos, sub.OutTradeNo)
	}
	if len(closeSubs) == 0 {
		return ClosePaymentOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("no sub orders available to close"))
	}

	if usesOrdinary {
		if err := svc.ordinaryProviderPayClient.CloseCombinePayment(ctx, ospcontracts.CombineCloseRequest{
			CombineAppID:      svc.ordinaryProviderPayClient.ServiceProviderAppID(),
			CombineMchID:      svc.ordinaryProviderPayClient.ServiceProviderMchID(),
			CombineOutTradeNo: combinedPayment.CombineOutTradeNo,
			SubOrders:         ordinaryCloseSubs,
		}); err != nil {
			return ClosePaymentOrderResult{}, mapCombineOrderCloseError(err)
		}
	} else {
		if err := svc.ecommerceClient.CloseCombineOrder(ctx, combinedPayment.CombineOutTradeNo, closeSubs); err != nil {
			return ClosePaymentOrderResult{}, mapCombineOrderCloseError(err)
		}
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
