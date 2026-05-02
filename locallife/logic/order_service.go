package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/rules"
	"github.com/merrydance/locallife/wechat"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog/log"
)

const orderPaymentTimeoutMinutes = 30

const (
	printTriggerAccepted = "accepted"
	printTriggerReady    = "ready"
	printTriggerManual   = "manual"

	printDispatchModeSingleFull = "single_full"
)

type OrderService struct {
	store                 db.Store
	notificationPublisher NotificationPublisher
	auditLogger           AuditLogger
	eventPublisher        OrderEventPublisher
	taskScheduler         TaskScheduler
	normalizer            DishCustomizationNormalizer
	paymentClient         wechat.DirectPaymentClientInterface
	ecommerceClient       wechat.EcommerceClientInterface
	ordinarySPClient      ordinaryServiceProviderOrderClient
	clock                 Clock
	idGenerator           IDGenerator
	orderPolicy           OrderPolicy
}

type ordinaryServiceProviderOrderClient interface {
	ordinaryServiceProviderPaymentClient
	RefundNotifyURL() string
	CreateRefund(ctx context.Context, req ospcontracts.RefundCreateRequest) (*ospcontracts.RefundResponse, error)
}

func NewOrderService(
	store db.Store,
	notificationPublisher NotificationPublisher,
	auditLogger AuditLogger,
	eventPublisher OrderEventPublisher,
	taskScheduler TaskScheduler,
	normalizer DishCustomizationNormalizer,
	paymentClient wechat.DirectPaymentClientInterface,
	ecommerceClient wechat.EcommerceClientInterface,
	clock Clock,
	idGenerator IDGenerator,
	orderPolicy OrderPolicy,
	ordinaryClients ...ordinaryServiceProviderOrderClient,
) *OrderService {
	if clock == nil {
		clock = SystemClock{}
	}
	if idGenerator == nil {
		idGenerator = DefaultIDGenerator{}
	}
	if orderPolicy == nil {
		orderPolicy = DefaultOrderPolicy{}
	}

	var ordinaryClient ordinaryServiceProviderOrderClient
	if len(ordinaryClients) > 0 {
		ordinaryClient = ordinaryClients[0]
	}

	return &OrderService{
		store:                 store,
		notificationPublisher: notificationPublisher,
		auditLogger:           auditLogger,
		eventPublisher:        eventPublisher,
		taskScheduler:         taskScheduler,
		normalizer:            normalizer,
		paymentClient:         paymentClient,
		ecommerceClient:       ecommerceClient,
		ordinarySPClient:      ordinaryClient,
		clock:                 clock,
		idGenerator:           idGenerator,
		orderPolicy:           orderPolicy,
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, input CreateOrderCommandInput) (CreateOrderCommandResult, error) {
	if err := s.orderPolicy.ValidateCreateInput(input); err != nil {
		return CreateOrderCommandResult{}, NewRequestError(http.StatusBadRequest, err)
	}

	merchant, err := ValidateMerchantForOrder(ctx, s.store, input.MerchantID)
	if err != nil {
		return CreateOrderCommandResult{}, err
	}

	if input.OrderType == "takeout" {
		suspension, getErr := GetTakeoutSuspension(ctx, s.store, merchant.ID)
		if getErr != nil {
			return CreateOrderCommandResult{}, getErr
		}
		if suspension != nil {
			return CreateOrderCommandResult{}, NewRequestError(http.StatusForbidden, errors.New("merchant takeout ordering is suspended"))
		}
	}

	ruleDecision, hasRule, err := s.evaluateRules(ctx, input, merchant)
	if err != nil {
		return CreateOrderCommandResult{}, err
	}

	if input.OrderType == "dine_in" && input.TableID != nil {
		if err := ValidateTableOwnership(ctx, s.store, input.MerchantID, *input.TableID); err != nil {
			return CreateOrderCommandResult{}, err
		}
	}

	var diningSession *db.DiningSession
	var reservation *db.TableReservation
	if input.OrderType == "dine_in" || input.OrderType == "reservation" {
		orderSessionResult, validateErr := ValidateOrderSessionAndBilling(ctx, s.store, OrderSessionInput{
			UserID:         input.UserID,
			MerchantID:     input.MerchantID,
			OrderType:      input.OrderType,
			TableID:        input.TableID,
			ReservationID:  input.ReservationID,
			BillingGroupID: input.BillingGroupID,
		})
		if validateErr != nil {
			return CreateOrderCommandResult{}, validateErr
		}
		diningSession = orderSessionResult.DiningSession
		reservation = orderSessionResult.Reservation
		if orderSessionResult.BillingGroupID != nil {
			input.BillingGroupID = orderSessionResult.BillingGroupID
		}
		if orderSessionResult.TableID != nil {
			input.TableID = orderSessionResult.TableID
		}
	}

	if reservation != nil && reservation.PaymentMode == "deposit" {
		if err := EnsureReservationSingleActiveOrder(ctx, s.store, reservation.ID); err != nil {
			return CreateOrderCommandResult{}, err
		}
	}

	normalizeFn := s.buildNormalizerFunc()
	subtotal, items, err := CalculateOrderItems(ctx, s.store, input.MerchantID, input.Items, normalizeFn)
	if err != nil {
		return CreateOrderCommandResult{}, NewRequestError(http.StatusBadRequest, err)
	}
	if err := s.validatePackagingPolicy(ctx, input.MerchantID, input.OrderType, items); err != nil {
		return CreateOrderCommandResult{}, NewRequestError(http.StatusBadRequest, err)
	}

	var deliveryFee int64
	var deliveryDistance int32
	var deliveryFeeDiscount int64
	var deliveryDuration int32
	var takeoutAddress *db.UserAddress
	if input.OrderType == "takeout" && input.AddressID != nil {
		address, getErr := loadOwnedUserAddress(ctx, s.store, input.UserID, *input.AddressID)
		if getErr != nil {
			return CreateOrderCommandResult{}, getErr
		}
		takeoutAddress = &address

		if input.DeliveryFeeCalculator == nil {
			return CreateOrderCommandResult{}, fmt.Errorf("delivery fee calculator: not configured")
		}

		quote, calcErr := ComputeDeliveryQuote(ctx, DeliveryQuoteInput{
			UserID:    input.UserID,
			OrderType: input.OrderType,
			Subtotal:  subtotal,
			Merchant:  merchant,
			Address:   address,
		}, input.MapClient, input.DeliveryFeeCalculator)
		if calcErr != nil {
			return CreateOrderCommandResult{}, calcErr
		}

		deliveryDistance = quote.Distance
		deliveryDuration = quote.Duration
		deliveryFee = quote.Fee
		deliveryFeeDiscount = quote.Discount
	}

	discountAmount := int64(0)
	merchantDiscountResult := MerchantDiscountResult{AllowWithVoucher: true}
	if resolvedDiscount, getErr := ResolveMerchantDiscount(ctx, s.store, OrderContext{
		MerchantID: input.MerchantID,
		OrderType:  input.OrderType,
		Subtotal:   subtotal,
	}); getErr == nil {
		merchantDiscountResult = resolvedDiscount
		discountAmount = resolvedDiscount.DiscountAmount
	}

	var voucherAmount int64
	var userVoucherID *int64
	if input.UserVoucherID != nil {
		voucherResult, validateErr := ValidateVoucher(ctx, s.store, VoucherValidationInput{
			UserID:        input.UserID,
			MerchantID:    input.MerchantID,
			OrderType:     input.OrderType,
			Subtotal:      subtotal,
			UserVoucherID: input.UserVoucherID,
		})
		if validateErr != nil {
			return CreateOrderCommandResult{}, validateErr
		}
		if !merchantDiscountResult.AllowWithVoucher {
			return CreateOrderCommandResult{}, NewRequestError(http.StatusBadRequest, errors.New("当前活动不可与所选优惠券叠加"))
		}
		userVoucherID = voucherResult.UserVoucherID
		voucherAmount = voucherResult.VoucherAmount
	}

	var depositDeduction int64
	if reservation != nil && reservation.PaymentMode == "deposit" {
		depositDeduction, err = ResolveReservationDepositDeduction(ctx, s.store, reservation)
		if err != nil {
			return CreateOrderCommandResult{}, err
		}
	}

	var membershipID *int64
	var membershipBalance int64
	if input.UseBalance {
		membership, validateErr := ValidateMembershipPayment(ctx, s.store, MembershipPaymentInput{
			UserID:             input.UserID,
			MerchantID:         input.MerchantID,
			OrderType:          input.OrderType,
			RulesEngineEnabled: input.RulesEngineEnabled,
		})
		if validateErr != nil {
			return CreateOrderCommandResult{}, validateErr
		}
		membershipID = &membership.ID
		membershipBalance = membership.Balance
	}

	totals, err := ComputeOrderTotals(OrderTotalsInput{
		Subtotal:            subtotal,
		DiscountAmount:      discountAmount,
		VoucherAmount:       voucherAmount,
		DeliveryFee:         deliveryFee,
		DeliveryFeeDiscount: deliveryFeeDiscount,
		DepositDeduction:    depositDeduction,
		MembershipBalance:   membershipBalance,
		UseBalance:          input.UseBalance,
	})
	if err != nil {
		return CreateOrderCommandResult{}, err
	}

	if input.OrderType == "takeout" && !input.RulesEngineEnabled {
		blocked, checkErr := CheckTakeoutBlocklist(ctx, s.store, input.UserID)
		if checkErr != nil {
			return CreateOrderCommandResult{}, checkErr
		}
		if blocked {
			return CreateOrderCommandResult{}, NewRequestError(http.StatusForbidden, errors.New("外卖服务已被限制：该账号存在异常索赔记录"))
		}
	}

	orderNo, err := s.idGenerator.OrderNo(s.clock.Now())
	if err != nil {
		return CreateOrderCommandResult{}, fmt.Errorf("generate order no: %w", err)
	}
	createParams := db.CreateOrderParams{
		OrderNo:             orderNo,
		UserID:              input.UserID,
		MerchantID:          input.MerchantID,
		OrderType:           input.OrderType,
		DeliveryFee:         deliveryFee,
		Subtotal:            subtotal,
		DiscountAmount:      discountAmount,
		DeliveryFeeDiscount: deliveryFeeDiscount,
		TotalAmount:         totals.TotalAmount,
		Status:              db.OrderStatusPending,
		FulfillmentStatus:   db.FulfillmentStatusScheduled,
		VoucherAmount:       voucherAmount,
		BalancePaid:         totals.BalancePaid,
	}

	if input.AddressID != nil {
		createParams.AddressID = pgtype.Int8{Int64: *input.AddressID, Valid: true}
	}
	if takeoutAddress != nil {
		createParams.DeliveryContactNameSnapshot = pgtype.Text{String: takeoutAddress.ContactName, Valid: true}
		createParams.DeliveryContactPhoneSnapshot = pgtype.Text{String: takeoutAddress.ContactPhone, Valid: true}
		createParams.DeliveryAddressSnapshot = pgtype.Text{String: takeoutAddress.DetailAddress, Valid: true}
		createParams.DeliveryLongitudeSnapshot = takeoutAddress.Longitude
		createParams.DeliveryLatitudeSnapshot = takeoutAddress.Latitude
	}
	if deliveryDistance > 0 {
		createParams.DeliveryDistance = pgtype.Int4{Int32: deliveryDistance, Valid: true}
	}
	if deliveryDuration > 0 {
		createParams.DeliveryDuration = pgtype.Int4{Int32: deliveryDuration, Valid: true}
	}
	if input.TableID != nil {
		createParams.TableID = pgtype.Int8{Int64: *input.TableID, Valid: true}
	}
	if input.ReservationID != nil {
		createParams.ReservationID = pgtype.Int8{Int64: *input.ReservationID, Valid: true}
	}
	if input.Notes != "" {
		createParams.Notes = pgtype.Text{String: input.Notes, Valid: true}
	}
	if userVoucherID != nil {
		createParams.UserVoucherID = pgtype.Int8{Int64: *userVoucherID, Valid: true}
	}
	if membershipID != nil {
		createParams.MembershipID = pgtype.Int8{Int64: *membershipID, Valid: true}
	}

	txResult, err := s.store.CreateOrderTx(ctx, db.CreateOrderTxParams{
		CreateOrderParams:                   createParams,
		Items:                               items,
		BillingGroupID:                      input.BillingGroupID,
		EnforceSingleActiveReservationOrder: reservation != nil && reservation.PaymentMode == "deposit",
		UserVoucherID:                       userVoucherID,
		VoucherAmount:                       voucherAmount,
		MembershipID:                        membershipID,
		BalancePaid:                         totals.BalancePaid,
		DeliveryDuration:                    deliveryDuration,
		RiderAverageSpeed:                   input.RiderAverageSpeed,
		DefaultPrepareTime:                  input.DefaultPrepareTime,
		PickupTime:                          s.clock.Now(),
	})
	if err != nil {
		if errors.Is(err, db.ErrReservationActiveOrderConflict) {
			return CreateOrderCommandResult{}, NewRequestError(http.StatusConflict, errors.New("reservation already has an active order"))
		}
		if err.Error() == "voucher already used" {
			return CreateOrderCommandResult{}, NewRequestError(http.StatusBadRequest, errors.New("优惠券已被使用"))
		}
		if err.Error() == "voucher has expired" {
			return CreateOrderCommandResult{}, NewRequestError(http.StatusBadRequest, errors.New("优惠券已过期"))
		}
		if err.Error() == "insufficient balance" {
			return CreateOrderCommandResult{}, NewRequestError(http.StatusBadRequest, errors.New("会员余额不足"))
		}
		return CreateOrderCommandResult{}, err
	}

	if s.taskScheduler != nil && txResult.Order.Status == db.OrderStatusPending {
		timeoutAt := s.clock.Now().Add(orderPaymentTimeoutMinutes * time.Minute)
		_ = s.taskScheduler.ScheduleOrderPaymentTimeout(ctx, txResult.Order.ID, timeoutAt)
	}

	if input.OrderType == "dine_in" && s.eventPublisher != nil {
		if input.RulesEngineEnabled && hasRule && ruleDecision.Action == "alert" {
			message := ruleDecision.Reason
			if message == "" {
				message = "该顾客有多次恶意索赔记录，谨慎服务"
			}
			s.eventPublisher.PublishMerchantUserRiskAlert(ctx, input.MerchantID, MerchantUserRiskAlert{
				UserID:  input.UserID,
				OrderID: txResult.Order.ID,
				OrderNo: txResult.Order.OrderNo,
				Message: message,
			})
		}
	}

	if diningSession != nil {
		_ = BindDiningSessionActiveOrder(ctx, s.store, diningSession.ID, txResult.Order.ID)
	}

	if input.OrderType == "dine_in" || input.OrderType == "reservation" {
		_ = ClearDiningOrderCart(ctx, s.store, ClearDiningOrderCartInput{
			UserID:        input.UserID,
			MerchantID:    input.MerchantID,
			OrderType:     input.OrderType,
			TableID:       input.TableID,
			ReservationID: input.ReservationID,
		})
	}

	return CreateOrderCommandResult{
		Order:        txResult.Order,
		RuleDecision: ruleDecision,
		HasRule:      hasRule,
	}, nil
}

func (s *OrderService) CancelOrder(ctx context.Context, input CancelOrderInput) (CancelOrderResult, error) {
	result, err := CancelOrder(ctx, s.store, input)
	if err != nil {
		return CancelOrderResult{}, err
	}

	if result.Refund != nil {
		if s.taskScheduler == nil {
			log.Error().
				Int64("order_id", result.Order.ID).
				Int64("payment_order_id", result.Refund.PaymentOrderID).
				Msg("refund task scheduler not configured after order cancellation")
			s.recordCancelRefundSchedulingIssue(ctx, result.Order, *result.Refund, "task_scheduler_not_configured", nil)
		} else {
			scheduleErr := s.taskScheduler.ScheduleProcessRefund(ctx, ProcessRefundTaskInput{
				PaymentOrderID: result.Refund.PaymentOrderID,
				OrderID:        result.Order.ID,
				RefundAmount:   result.Refund.Amount,
				Reason:         result.Refund.Reason,
			})
			if scheduleErr != nil {
				log.Error().Err(scheduleErr).
					Int64("order_id", result.Order.ID).
					Int64("payment_order_id", result.Refund.PaymentOrderID).
					Msg("failed to schedule refund task")
				s.recordCancelRefundSchedulingIssue(ctx, result.Order, *result.Refund, "schedule_process_refund_failed", scheduleErr)
			}
		}
	}

	if s.eventPublisher != nil {
		s.eventPublisher.PublishMerchantOrderSnapshot(ctx, result.Order.MerchantID, result.Order, "order_update")
	}

	return result, nil
}

func (s *OrderService) recordCancelRefundSchedulingIssue(ctx context.Context, order db.Order, refund RefundTask, issue string, scheduleErr error) {
	if s.auditLogger == nil {
		return
	}

	targetID := order.ID
	metadata := map[string]interface{}{
		"issue":                    issue,
		"payment_order_id":         refund.PaymentOrderID,
		"refund_amount":            refund.Amount,
		"refund_reason":            refund.Reason,
		"refund_recovery_expected": true,
	}
	if scheduleErr != nil {
		metadata["error"] = scheduleErr.Error()
	}

	s.auditLogger.Write(ctx, AuditLogInput{
		ActorUserID: order.UserID,
		ActorRole:   "user",
		Action:      "order_cancel_refund_schedule_issue",
		TargetType:  "order",
		TargetID:    &targetID,
		Metadata:    metadata,
	})
}

func (s *OrderService) UrgeOrder(ctx context.Context, input UrgeOrderInput) (UrgeOrderResult, error) {
	result, err := UrgeOrder(ctx, s.store, input)
	if err != nil {
		return UrgeOrderResult{}, err
	}

	if s.notificationPublisher != nil {
		order := result.Order
		if result.NotifyMerchant {
			_ = s.notificationPublisher.Send(ctx, NotificationInput{
				UserID:      order.MerchantID,
				Title:       "用户催单提醒",
				Content:     fmt.Sprintf("订单 %s 的用户正在催单，请尽快处理", order.OrderNo),
				Type:        "order_urge",
				RelatedType: "order",
				RelatedID:   order.ID,
			})
		}

		if result.RiderID != nil {
			_ = s.notificationPublisher.Send(ctx, NotificationInput{
				UserID:      *result.RiderID,
				Title:       "用户催单提醒",
				Content:     fmt.Sprintf("订单 %s 的用户正在催单，请尽快送达", order.OrderNo),
				Type:        "order_urge",
				RelatedType: "order",
				RelatedID:   order.ID,
			})
		}
	}

	return result, nil
}

func (s *OrderService) ReplaceOrder(ctx context.Context, input ReplaceOrderInput) (ReplaceOrderResult, error) {
	normalize := s.buildNormalizerFunc()
	result, err := ReplaceReservationOrderWithOrdinaryServiceProvider(ctx, s.store, s.ecommerceClient, s.ordinarySPClient, input, normalize)
	if err != nil {
		return ReplaceOrderResult{}, err
	}

	if s.taskScheduler != nil && result.PaymentOrderID != nil {
		if scheduleErr := s.scheduleReplaceOrderPaymentTimeout(ctx, *result.PaymentOrderID); scheduleErr != nil {
			log.Warn().Err(scheduleErr).Int64("payment_order_id", *result.PaymentOrderID).Msg("schedule replace-order payment timeout failed")
		}
	}

	return result, nil
}

func (s *OrderService) scheduleReplaceOrderPaymentTimeout(ctx context.Context, paymentOrderID int64) error {
	paymentOrder, err := s.store.GetPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		return err
	}
	if paymentOrder.Status != paymentStatusPending || !paymentOrder.ExpiresAt.Valid {
		return nil
	}
	if paymentOrder.CombinedPaymentID.Valid {
		combinedPayment, err := s.store.GetCombinedPaymentOrder(ctx, paymentOrder.CombinedPaymentID.Int64)
		if err != nil {
			return err
		}
		return s.taskScheduler.ScheduleCombinedPaymentOrderTimeout(ctx, combinedPayment.CombineOutTradeNo, paymentOrder.ExpiresAt.Time)
	}
	return s.taskScheduler.SchedulePaymentOrderTimeout(ctx, paymentOrder.OutTradeNo, paymentOrder.ExpiresAt.Time)
}

func (s *OrderService) ConfirmOrder(ctx context.Context, input ConfirmOrderInput) (ConfirmOrderResult, error) {
	result, err := ConfirmTakeoutOrder(ctx, s.store, input)
	if err != nil {
		return ConfirmOrderResult{}, err
	}

	if result.AlreadyCompleted {
		return result, nil
	}

	if s.notificationPublisher != nil {
		_ = s.notificationPublisher.Send(ctx, NotificationInput{
			UserID:      result.Order.MerchantID,
			Type:        "order_completed",
			Title:       "订单已完成",
			Content:     fmt.Sprintf("订单 %s 用户已确认收货", result.Order.OrderNo),
			RelatedType: "order",
			RelatedID:   result.Order.ID,
		})

		if result.RiderID != nil {
			_ = s.notificationPublisher.Send(ctx, NotificationInput{
				UserID:      *result.RiderID,
				Type:        "delivery_completed",
				Title:       "配送已完成",
				Content:     fmt.Sprintf("订单 %s 用户已确认收货", result.Order.OrderNo),
				RelatedType: "order",
				RelatedID:   result.Order.ID,
			})
		}
	}

	return result, nil
}

func (s *OrderService) AcceptMerchantOrder(ctx context.Context, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error) {
	result, err := AcceptMerchantOrder(ctx, s.store, input)
	if err != nil {
		return MerchantOrderUpdateResult{}, err
	}

	if s.notificationPublisher != nil {
		expiresAt := s.clock.Now().Add(24 * time.Hour)
		_ = s.notificationPublisher.Send(ctx, NotificationInput{
			UserID:      result.Order.UserID,
			Type:        "order",
			Title:       "商家已接单",
			Content:     fmt.Sprintf("您的订单%s已被商家接单，正在准备中", result.Order.OrderNo),
			RelatedType: "order",
			RelatedID:   result.Order.ID,
			ExpiresAt:   &expiresAt,
			OrderNo:     result.Order.OrderNo,
			OrderStatus: db.OrderStatusPreparing,
		})
	}

	if s.eventPublisher != nil {
		s.eventPublisher.PublishMerchantOrderSnapshot(ctx, input.MerchantID, result.Order, "order_update")
	}

	s.scheduleOrderPrint(ctx, result.Order, printTriggerAccepted)

	return result, nil
}

func (s *OrderService) RejectMerchantOrder(ctx context.Context, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error) {
	result, err := RejectMerchantOrder(ctx, s.store, input)
	if err != nil {
		return MerchantOrderUpdateResult{}, err
	}

	if s.notificationPublisher != nil {
		expiresAt := s.clock.Now().Add(24 * time.Hour)
		_ = s.notificationPublisher.Send(ctx, NotificationInput{
			UserID:      result.Order.UserID,
			Type:        "order",
			Title:       "订单被商家取消",
			Content:     fmt.Sprintf("您的订单%s已被商家取消，原因：%s。支付金额将原路退回", result.Order.OrderNo, input.Reason),
			RelatedType: "order",
			RelatedID:   result.Order.ID,
			ExpiresAt:   &expiresAt,
			OrderNo:     result.Order.OrderNo,
			OrderStatus: db.OrderStatusCancelled,
		})
	}

	refundResult, refundErr := ProcessMerchantRejectRefundWithOrdinaryServiceProvider(ctx, s.store, s.ecommerceClient, s.ordinarySPClient, MerchantRejectRefundInput{
		MerchantID: input.MerchantID,
		OrderID:    result.Order.ID,
		Reason:     input.Reason,
	})
	if refundErr != nil {
		if refundResult.RefundOrder != nil {
			log.Error().Err(refundErr).Int64("refund_order_id", refundResult.RefundOrder.ID).Msg("merchant reject refund failed")
		} else {
			log.Error().Err(refundErr).Int64("order_id", result.Order.ID).Msg("merchant reject refund failed")
		}
	}

	if s.eventPublisher != nil {
		s.eventPublisher.PublishMerchantOrderSnapshot(ctx, input.MerchantID, result.Order, "order_update")
	}

	return result, nil
}

func (s *OrderService) MarkMerchantOrderReady(ctx context.Context, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error) {
	result, err := MarkMerchantOrderReady(ctx, s.store, input)
	if err != nil {
		return MerchantOrderUpdateResult{}, err
	}

	if s.notificationPublisher != nil {
		expiresAt := s.clock.Now().Add(24 * time.Hour)
		_ = s.notificationPublisher.Send(ctx, NotificationInput{
			UserID:      result.Order.UserID,
			Type:        "order",
			Title:       "订单已出餐",
			Content:     fmt.Sprintf("您的订单%s已出餐，请及时取餐", result.Order.OrderNo),
			RelatedType: "order",
			RelatedID:   result.Order.ID,
			ExpiresAt:   &expiresAt,
			OrderNo:     result.Order.OrderNo,
			OrderStatus: db.OrderStatusReady,
		})
	}

	if s.eventPublisher != nil {
		s.eventPublisher.PublishMerchantOrderSnapshot(ctx, input.MerchantID, result.Order, "order_update")
		if result.PoolItem != nil {
			s.eventPublisher.PublishTakeoutOrderPooled(ctx, result.Order, *result.PoolItem)
		}
	}

	s.scheduleOrderPrint(ctx, result.Order, printTriggerReady)
	return result, nil
}

func (s *OrderService) CompleteMerchantOrder(ctx context.Context, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error) {
	result, err := CompleteMerchantOrder(ctx, s.store, input)
	if err != nil {
		return MerchantOrderUpdateResult{}, err
	}

	if s.notificationPublisher != nil {
		expiresAt := s.clock.Now().Add(24 * time.Hour)
		_ = s.notificationPublisher.Send(ctx, NotificationInput{
			UserID:      result.Order.UserID,
			Type:        "order",
			Title:       "订单已完成",
			Content:     fmt.Sprintf("您的订单%s已完成，欢迎再次光临", result.Order.OrderNo),
			RelatedType: "order",
			RelatedID:   result.Order.ID,
			ExpiresAt:   &expiresAt,
			OrderNo:     result.Order.OrderNo,
			OrderStatus: db.OrderStatusCompleted,
		})
	}

	if s.eventPublisher != nil {
		s.eventPublisher.PublishMerchantOrderSnapshot(ctx, input.MerchantID, result.Order, "order_update")
	}
	return result, nil
}

func (s *OrderService) PrintMerchantOrder(ctx context.Context, input MerchantOrderPrintInput) (MerchantOrderPrintResult, error) {
	order, err := s.store.GetOrder(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return MerchantOrderPrintResult{}, NewRequestError(http.StatusNotFound, errors.New("order not found"))
		}
		return MerchantOrderPrintResult{}, err
	}
	if order.MerchantID != input.MerchantID {
		return MerchantOrderPrintResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to your merchant"))
	}
	if order.Status == db.OrderStatusCancelled {
		return MerchantOrderPrintResult{}, NewRequestError(http.StatusBadRequest, errors.New("cancelled orders cannot be printed"))
	}
	if s.taskScheduler == nil {
		return MerchantOrderPrintResult{}, fmt.Errorf("print scheduler: not configured")
	}

	config := s.loadOrderPrintConfig(ctx, order.MerchantID)
	if !config.EnablePrint || !displayConfigAllowsOrderType(config, order.OrderType) {
		return MerchantOrderPrintResult{}, NewRequestError(http.StatusBadRequest, errors.New("printing is not enabled for this order"))
	}
	if config.PrintTriggerMode != printTriggerManual {
		return MerchantOrderPrintResult{}, NewRequestError(http.StatusBadRequest, errors.New("manual printing is not enabled"))
	}

	if err := s.taskScheduler.ScheduleOrderPrint(ctx, OrderPrintTaskInput{OrderID: order.ID, Trigger: printTriggerManual}); err != nil {
		return MerchantOrderPrintResult{}, err
	}

	return MerchantOrderPrintResult{Order: order}, nil
}

func (s *OrderService) buildNormalizerFunc() NormalizeDishCustomizationsFunc {
	if s.normalizer == nil {
		return nil
	}

	return func(ctx context.Context, dishID int64, customizations map[string]interface{}) ([]byte, int64, error) {
		return s.normalizer.Normalize(ctx, dishID, customizations)
	}
}

func (s *OrderService) scheduleOrderPrint(ctx context.Context, order db.Order, trigger string) {
	if s.taskScheduler == nil {
		return
	}

	config := s.loadOrderPrintConfig(ctx, order.MerchantID)

	if !config.EnablePrint || !displayConfigAllowsOrderType(config, order.OrderType) || !displayConfigAllowsTrigger(config, trigger) {
		return
	}

	if err := s.taskScheduler.ScheduleOrderPrint(ctx, OrderPrintTaskInput{OrderID: order.ID, Trigger: trigger}); err != nil {
		log.Warn().Err(err).Int64("order_id", order.ID).Str("trigger", trigger).Msg("schedule order print failed")
	}
}

func (s *OrderService) loadOrderPrintConfig(ctx context.Context, merchantID int64) db.OrderDisplayConfig {
	config, err := s.store.GetOrderDisplayConfigByMerchant(ctx, merchantID)
	if err == nil {
		return config
	}
	if !errors.Is(err, db.ErrRecordNotFound) {
		log.Warn().Err(err).Int64("merchant_id", merchantID).Msg("load order display config for printing failed")
	}
	return defaultOrderPrintConfig()
}

func defaultOrderPrintConfig() db.OrderDisplayConfig {
	return db.OrderDisplayConfig{
		EnablePrint:       true,
		PrintTakeout:      true,
		PrintDineIn:       true,
		PrintReservation:  true,
		PrintDispatchMode: printDispatchModeSingleFull,
		PrintTriggerMode:  printTriggerAccepted,
	}
}

func displayConfigAllowsOrderType(config db.OrderDisplayConfig, orderType string) bool {
	switch orderType {
	case db.OrderTypeTakeout, "takeaway":
		return config.PrintTakeout
	case "dine_in":
		return config.PrintDineIn
	case db.OrderTypeReservation:
		return config.PrintReservation
	default:
		return false
	}
}

func displayConfigAllowsTrigger(config db.OrderDisplayConfig, trigger string) bool {
	switch config.PrintTriggerMode {
	case "", printTriggerAccepted:
		return trigger == printTriggerAccepted
	case printTriggerReady:
		return trigger == printTriggerReady
	case printTriggerManual:
		return trigger == printTriggerManual
	default:
		return false
	}
}

func (s *OrderService) evaluateRules(ctx context.Context, input CreateOrderCommandInput, merchant db.Merchant) (rules.Decision, bool, error) {
	if input.RulesEngine == nil {
		return rules.Decision{}, false, nil
	}

	ruleInput := rules.Context{
		Domain:     rules.DomainOrder,
		RegionID:   merchant.RegionID,
		MerchantID: merchant.ID,
		UserID:     input.UserID,
		OrderType:  input.OrderType,
		Metadata: map[string]interface{}{
			"items_count": len(input.Items),
			"use_balance": input.UseBalance,
		},
	}

	decision, err := EvaluateRules(ctx, RuleEvaluationInput{
		Enabled:   input.RulesEngineEnabled,
		Engine:    input.RulesEngine,
		Context:   ruleInput,
		ActorRole: "customer",
		OnDecision: func(ruleContext rules.Context, result rules.Decision, actorRole string) {
			if input.OnRuleDecision != nil {
				input.OnRuleDecision(ruleContext, result, actorRole)
			}
		},
	})
	if err != nil {
		return rules.Decision{}, false, err
	}

	return decision, true, nil
}
