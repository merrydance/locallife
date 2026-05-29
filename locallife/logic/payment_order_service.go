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

// PaymentOrderService owns user-facing single payment order operations.
type PaymentOrderService struct {
	store               db.Store
	directPaymentClient wechat.DirectPaymentClientInterface
	baofuPaymentService *BaofuPaymentService
	now                 func() time.Time
}

func NewPaymentOrderServiceWithDirectPayment(store db.Store, directPaymentClient wechat.DirectPaymentClientInterface) *PaymentOrderService {
	return &PaymentOrderService{
		store:               store,
		directPaymentClient: directPaymentClient,
		now:                 time.Now,
	}
}

func NewPaymentOrderServiceWithBaofu(store db.Store, directPaymentClient wechat.DirectPaymentClientInterface, baofuPaymentService *BaofuPaymentService) *PaymentOrderService {
	return &PaymentOrderService{
		store:               store,
		directPaymentClient: directPaymentClient,
		baofuPaymentService: baofuPaymentService,
		now:                 time.Now,
	}
}

type CreatePaymentOrderInput struct {
	UserID       int64
	OrderID      int64
	PaymentType  string
	BusinessType string
	ClientIP     string
	Amount       int64
}

type CreatePaymentOrderResult struct {
	PaymentOrder db.PaymentOrder
	PayParams    *wechat.JSAPIPayParams
}

type CreateReservationAdjustmentPaymentInput struct {
	UserID        int64
	ReservationID int64
	MerchantID    int64
	Items         []db.CreateReservationItemParams
	CurrentTotal  int64
	TargetTotal   int64
	DeltaAmount   int64
	ClientIP      string
	Now           time.Time
	ExpiresAt     time.Time
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

type baofuPaymentCreateInput struct {
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

func (svc *PaymentOrderService) CreatePaymentOrder(ctx context.Context, input CreatePaymentOrderInput) (CreatePaymentOrderResult, error) {
	var result CreatePaymentOrderResult
	if svc == nil || svc.store == nil {
		return result, fmt.Errorf("payment order service not configured")
	}
	if input.BusinessType != businessTypeOrder && input.BusinessType != businessTypeReservation && input.BusinessType != reservationAddonBusiness {
		return result, NewRequestError(http.StatusBadRequest, errors.New("invalid business type"))
	}
	if input.BusinessType == reservationAddonBusiness {
		return result, NewRequestError(http.StatusBadRequest, errors.New("reservation_addon payments must be created through reservation dish adjustment"))
	}

	var amount int64
	merchantName := "Order Payment"
	var merchantID int64
	var attach string
	var reservationPaymentMode string

	if input.BusinessType == businessTypeReservation || input.BusinessType == reservationAddonBusiness {
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
		if input.BusinessType == businessTypeReservation && reservation.Status != "pending" {
			return result, NewRequestError(http.StatusBadRequest, errors.New("reservation is not in pending status"))
		}
		if input.BusinessType == reservationAddonBusiness &&
			reservation.Status != reservationStatusPaid &&
			reservation.Status != reservationStatusConfirmed &&
			reservation.Status != reservationStatusCheckedIn {
			return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("reservation is not ready for add-on payment in %s status", reservation.Status))
		}
		merchantID = reservation.MerchantID
		reservationPaymentMode = reservation.PaymentMode
		if input.BusinessType == reservationAddonBusiness {
			amount = input.Amount
			attach = buildReservationAddonPaymentAttach(reservation.ID)
		} else if reservation.PaymentMode == paymentModeDeposit {
			amount = reservation.DepositAmount
			attach = buildReservationPaymentAttach(reservation.ID, reservation.PaymentMode)
		} else {
			amount = reservation.PrepaidAmount
			attach = buildReservationPaymentAttach(reservation.ID, reservation.PaymentMode)
		}
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

	existingPayment, err := svc.latestPaymentForBusiness(ctx, input)
	if err == nil && existingPayment.Status == paymentStatusPending {
		if !paymentOrderUsesBaofuAggregateChannel(existingPayment) {
			if closeErr := svc.supersedePendingPaymentOrder(ctx, existingPayment); closeErr != nil {
				return result, closeErr
			}
		} else if input.BusinessType == businessTypeReservation && !shouldReuseReservationPendingPayment(existingPayment, amount, attach) {
			if closeErr := svc.supersedePendingPaymentOrder(ctx, existingPayment); closeErr != nil {
				return result, closeErr
			}
		} else if input.BusinessType == businessTypeOrder && existingPayment.Amount != amount {
			if closeErr := svc.supersedePendingPaymentOrder(ctx, existingPayment); closeErr != nil {
				return result, closeErr
			}
		} else {
			return CreatePaymentOrderResult{PaymentOrder: existingPayment}, nil
		}
	} else if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return result, err
	}

	expiresAt := svc.now().Add(30 * time.Minute)
	if input.BusinessType == businessTypeReservation || input.BusinessType == reservationAddonBusiness {
		return svc.createReservationBaofuPayment(ctx, input, merchantID, merchantName, reservationPaymentMode, amount, attach, expiresAt)
	}
	return svc.createOrderBaofuPayment(ctx, input, merchantID, merchantName, amount, attach, expiresAt)
}

func (svc *PaymentOrderService) latestPaymentForBusiness(ctx context.Context, input CreatePaymentOrderInput) (db.PaymentOrder, error) {
	if input.BusinessType == businessTypeReservation || input.BusinessType == reservationAddonBusiness {
		return svc.store.GetLatestPaymentOrderByReservation(ctx, db.GetLatestPaymentOrderByReservationParams{
			ReservationID: pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType:  input.BusinessType,
		})
	}
	return svc.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: input.BusinessType,
	})
}

func (svc *PaymentOrderService) supersedePendingPaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) error {
	if paymentOrder.PrepayID.Valid && paymentOrderUsesBaofuAggregateChannel(paymentOrder) {
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

func (svc *PaymentOrderService) resolveConcurrentOrderPayment(ctx context.Context, input CreatePaymentOrderInput, expectedAmount int64) (CreatePaymentOrderResult, bool, error) {
	var result CreatePaymentOrderResult
	for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
		paymentOrder, err := svc.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: input.BusinessType,
		})
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				if attempt < outTradeNoMaxRetry && sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
					continue
				}
				return result, false, nil
			}
			return result, true, fmt.Errorf("get latest payment order after concurrent conflict: %w", err)
		}
		if paymentOrder.Status != paymentStatusPending {
			return result, false, nil
		}
		if paymentOrder.Amount != expectedAmount || !paymentOrderUsesBaofuAggregateChannel(paymentOrder) {
			if err := svc.supersedePendingPaymentOrder(ctx, paymentOrder); err != nil {
				return result, true, err
			}
			return result, false, nil
		}
		return CreatePaymentOrderResult{PaymentOrder: paymentOrder}, true, nil
	}
	return result, false, nil
}

func (svc *PaymentOrderService) resolveConcurrentReservationPayment(ctx context.Context, input CreatePaymentOrderInput, expectedAmount int64, expectedAttach string) (CreatePaymentOrderResult, bool, error) {
	var result CreatePaymentOrderResult
	for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
		paymentOrder, err := svc.store.GetLatestPaymentOrderByReservation(ctx, db.GetLatestPaymentOrderByReservationParams{
			ReservationID: pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType:  input.BusinessType,
		})
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				if attempt < outTradeNoMaxRetry && sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
					continue
				}
				return result, false, nil
			}
			return result, true, fmt.Errorf("get latest payment order after concurrent conflict: %w", err)
		}
		if paymentOrder.Status != paymentStatusPending {
			return result, false, nil
		}
		if !paymentOrderUsesBaofuAggregateChannel(paymentOrder) || !shouldReuseReservationPendingPayment(paymentOrder, expectedAmount, expectedAttach) {
			if err := svc.supersedePendingPaymentOrder(ctx, paymentOrder); err != nil {
				return result, true, err
			}
			return result, false, nil
		}
		return CreatePaymentOrderResult{PaymentOrder: paymentOrder}, true, nil
	}
	return result, false, nil
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

	paymentOrders, err := svc.store.ListPaymentOrdersByUser(ctx, db.ListPaymentOrdersByUserParams{
		UserID: input.UserID,
		Limit:  pageSize,
		Offset: (pageID - 1) * pageSize,
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
	if paymentOrder.PaymentChannel == db.PaymentChannelDirect {
		if svc.directPaymentClient != nil && paymentOrder.PrepayID.Valid && strings.TrimSpace(paymentOrder.OutTradeNo) != "" {
			if err := svc.directPaymentClient.CloseOrder(ctx, paymentOrder.OutTradeNo); err != nil {
				return ClosePaymentOrderResult{}, err
			}
		}
	}
	updatedPayment, err := svc.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
	if err != nil {
		return ClosePaymentOrderResult{}, err
	}
	return ClosePaymentOrderResult{PaymentOrder: updatedPayment}, nil
}

func buildReservationPaymentAttach(reservationID int64, paymentMode string) string {
	return fmt.Sprintf("reservation_id:%d;payment_mode:%s", reservationID, paymentMode)
}

func buildReservationAddonPaymentAttach(reservationID int64) string {
	return fmt.Sprintf("reservation_id:%d;payment_mode:%s;addon:true", reservationID, paymentModeFull)
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
		if key != "" && value != "" {
			parts[key] = value
		}
	}
	return parts
}

func shouldReuseReservationPendingPayment(paymentOrder db.PaymentOrder, expectedAmount int64, expectedAttach string) bool {
	if paymentOrder.Amount != expectedAmount || !paymentOrder.Attach.Valid {
		return false
	}
	existing := parsePaymentAttach(paymentOrder.Attach.String)
	expected := parsePaymentAttach(expectedAttach)
	if existing["reservation_id"] != expected["reservation_id"] || existing["payment_mode"] != expected["payment_mode"] {
		return false
	}
	return existing["addon"] == expected["addon"]
}

func shouldEnableOrderProfitSharing(orderType string) bool {
	switch orderType {
	case orderTypeDineIn, orderTypeTakeaway:
		return false
	default:
		return true
	}
}

func mapBaofuPaymentOrderCreateError(err error) error {
	if err == nil {
		return nil
	}
	if status, ok := db.IsPartnerPaymentRequestError(err); ok {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "config invalid") || strings.Contains(msg, "inactive"):
			return NewRequestError(status, errors.New("商户支付能力未完成配置，请联系平台处理后重试"))
		case strings.Contains(msg, "does not belong to user"):
			return NewRequestError(status, errors.New("当前支付对象不属于你"))
		case strings.Contains(msg, "addon amount must be greater than 0"):
			return NewRequestError(status, errors.New("补差金额必须大于 0，请返回预订页面重新确认菜品"))
		case strings.Contains(msg, "expect paid/confirmed/checked_in"):
			return NewRequestError(status, errors.New("当前预订状态不支持补差支付，请刷新后重试"))
		case strings.Contains(msg, "status is") || strings.Contains(msg, "expect pending"):
			return NewRequestError(status, errors.New("当前支付对象已不在待支付状态，请刷新页面确认"))
		case strings.Contains(msg, "payable amount changed") || strings.Contains(msg, "payment mode changed"):
			return NewRequestError(status, errors.New("支付金额或支付模式已变化，请返回订单页重新发起支付"))
		case strings.Contains(msg, "merchant changed"):
			return NewRequestError(status, errors.New("支付商户信息已变化，请刷新后重试"))
		case strings.Contains(msg, "has pending payment order"):
			return NewRequestError(status, errors.New("已有待支付补差订单，请先刷新支付结果后再决定是否重试"))
		default:
			return NewRequestError(status, errors.New("支付订单状态已变化，请刷新后重试"))
		}
	}
	if isOutTradeNoConflict(err) || errors.Is(err, db.ErrOrderPendingPaymentConflict) || db.ErrorCode(err) == db.UniqueViolation {
		return NewRequestError(http.StatusConflict, errors.New("支付订单状态已变化，请刷新后重试"))
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
	return fmt.Errorf("create baofu payment: %w", err)
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
	return strings.Contains(pgErr.ConstraintName, "out_trade_no") || strings.Contains(pgErr.Detail, "out_trade_no")
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

func (svc *PaymentOrderService) markPaymentOrderFailedForCleanup(ctx context.Context, paymentOrderID int64, message string) {
	if _, err := svc.store.UpdatePaymentOrderToFailed(ctx, paymentOrderID); err != nil {
		log.Error().Err(err).Int64("payment_order_id", paymentOrderID).Msg(message)
	}
}
