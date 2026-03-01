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
)

const orderPaymentTimeoutMinutes = 30

type OrderService struct {
	store                 db.Store
	notificationPublisher NotificationPublisher
	auditLogger           AuditLogger
	eventPublisher        OrderEventPublisher
	taskScheduler         TaskScheduler
	normalizer            DishCustomizationNormalizer
	paymentClient         wechat.PaymentClientInterface
	clock                 Clock
	idGenerator           IDGenerator
	orderPolicy           OrderPolicy
}

func NewOrderService(
	store db.Store,
	notificationPublisher NotificationPublisher,
	auditLogger AuditLogger,
	eventPublisher OrderEventPublisher,
	taskScheduler TaskScheduler,
	normalizer DishCustomizationNormalizer,
	paymentClient wechat.PaymentClientInterface,
	clock Clock,
	idGenerator IDGenerator,
	orderPolicy OrderPolicy,
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

	return &OrderService{
		store:                 store,
		notificationPublisher: notificationPublisher,
		auditLogger:           auditLogger,
		eventPublisher:        eventPublisher,
		taskScheduler:         taskScheduler,
		normalizer:            normalizer,
		paymentClient:         paymentClient,
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

	var deliveryFee int64
	var deliveryDistance int32
	var deliveryFeeDiscount int64
	var deliveryDuration int32
	if input.OrderType == "takeout" && input.AddressID != nil {
		address, getErr := s.store.GetUserAddress(ctx, *input.AddressID)
		if getErr != nil {
			if errors.Is(getErr, db.ErrRecordNotFound) {
				return CreateOrderCommandResult{}, NewRequestError(http.StatusNotFound, errors.New("address not found"))
			}
			return CreateOrderCommandResult{}, getErr
		}

		if input.DeliveryFeeCalculator == nil {
			return CreateOrderCommandResult{}, NewRequestError(http.StatusInternalServerError, errors.New("delivery fee calculator is required"))
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
	if bestAmount, getErr := GetBestDiscountAmount(ctx, s.store, input.MerchantID, subtotal); getErr == nil {
		discountAmount = bestAmount
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
		userVoucherID = voucherResult.UserVoucherID
		voucherAmount = voucherResult.VoucherAmount
	}

	var depositDeduction int64
	if reservation != nil && reservation.PaymentMode == "deposit" {
		depositDeduction = reservation.DepositAmount
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

	orderNo := s.idGenerator.OrderNo(s.clock.Now())
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

	if input.OrderType == "takeout" || input.OrderType == "takeaway" {
		createParams.PickupCode = pgtype.Text{String: s.idGenerator.PickupCode(s.clock.Now()), Valid: true}
	}
	if input.AddressID != nil {
		createParams.AddressID = pgtype.Int8{Int64: *input.AddressID, Valid: true}
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
		CreateOrderParams:  createParams,
		Items:              items,
		BillingGroupID:     input.BillingGroupID,
		UserVoucherID:      userVoucherID,
		VoucherAmount:      voucherAmount,
		MembershipID:       membershipID,
		BalancePaid:        totals.BalancePaid,
		DeliveryDuration:   deliveryDuration,
		RiderAverageSpeed:  input.RiderAverageSpeed,
		DefaultPrepareTime: input.DefaultPrepareTime,
	})
	if err != nil {
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
	return CancelOrder(ctx, s.store, input)
}

func (s *OrderService) UrgeOrder(ctx context.Context, input UrgeOrderInput) (UrgeOrderResult, error) {
	return UrgeOrder(ctx, s.store, input)
}

func (s *OrderService) ReplaceOrder(ctx context.Context, input ReplaceOrderInput) (ReplaceOrderResult, error) {
	normalize := s.buildNormalizerFunc()
	return ReplaceReservationOrder(ctx, s.store, s.paymentClient, input, normalize)
}

func (s *OrderService) ConfirmOrder(ctx context.Context, input ConfirmOrderInput) (ConfirmOrderResult, error) {
	return ConfirmTakeoutOrder(ctx, s.store, input)
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

	return result, nil
}

func (s *OrderService) RejectMerchantOrder(ctx context.Context, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error) {
	result, err := RejectMerchantOrder(ctx, s.store, input)
	if err != nil {
		return MerchantOrderUpdateResult{}, err
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
	if s.eventPublisher != nil {
		s.eventPublisher.PublishMerchantOrderSnapshot(ctx, input.MerchantID, result.Order, "order_update")
	}
	return result, nil
}

func (s *OrderService) CompleteMerchantOrder(ctx context.Context, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error) {
	result, err := CompleteMerchantOrder(ctx, s.store, input)
	if err != nil {
		return MerchantOrderUpdateResult{}, err
	}
	if s.eventPublisher != nil {
		s.eventPublisher.PublishMerchantOrderSnapshot(ctx, input.MerchantID, result.Order, "order_update")
	}
	return result, nil
}

func (s *OrderService) buildNormalizerFunc() NormalizeDishCustomizationsFunc {
	if s.normalizer == nil {
		return nil
	}

	return func(ctx context.Context, dishID int64, customizations map[string]interface{}) ([]byte, int64, error) {
		return s.normalizer.Normalize(ctx, dishID, customizations)
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
