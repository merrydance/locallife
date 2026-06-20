package logic

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/rules"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

const orderPaymentTimeoutMinutes = 30

const (
	orderCreateIdempotencyScope        = "customer_order_create"
	orderCreateMaxIdempotencyKeyLength = 256
)

const (
	orderCreatePackagingSourceNone = "none"
	orderCreatePackagingSourceCart = "cart"
)

type orderCreatePackagingIdentity struct {
	Source           string
	CartID           *int64
	OptionID         *int64
	SelectionVersion *int64
}

type orderCreatePackagingResolution struct {
	Identity       orderCreatePackagingIdentity
	Fee            int64
	SnapshotParams []db.CreateOrderPackagingItemParams
}

const (
	printTriggerAccepted = "accepted"
	printTriggerReady    = "ready"
	printTriggerManual   = "manual"

	printDispatchModeSingleFull = "single_full"

	merchantOrderSnapshotMessageTypeOrderUpdate = "order_update"
)

type OrderService struct {
	store                 db.Store
	notificationPublisher NotificationPublisher
	auditLogger           AuditLogger
	eventPublisher        OrderEventPublisher
	taskScheduler         TaskScheduler
	normalizer            DishCustomizationNormalizer
	paymentClient         wechat.DirectPaymentClientInterface
	paymentFacade         PaymentFacade
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
	paymentClient wechat.DirectPaymentClientInterface,
	paymentFacade PaymentFacade,
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
		paymentFacade:         paymentFacade,
		clock:                 clock,
		idGenerator:           idGenerator,
		orderPolicy:           orderPolicy,
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, input CreateOrderCommandInput) (CreateOrderCommandResult, error) {
	if err := s.orderPolicy.ValidateCreateInput(input); err != nil {
		return CreateOrderCommandResult{}, NewRequestError(http.StatusBadRequest, err)
	}
	if idempotencyKey := strings.TrimSpace(input.IdempotencyKey); len(idempotencyKey) > orderCreateMaxIdempotencyKeyLength {
		return CreateOrderCommandResult{}, NewRequestError(http.StatusBadRequest, errors.New("Idempotency-Key header is too long"))
	}
	if replayed, ok, err := s.replayBoundOrderCreateIdempotency(ctx, input); err != nil {
		return CreateOrderCommandResult{}, err
	} else if ok {
		return replayed, nil
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
	packaging, err := s.resolveOrderCreatePackaging(ctx, input)
	if err != nil {
		return CreateOrderCommandResult{}, err
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
		PackagingFee:        packaging.Fee,
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
		PackagingFee:        packaging.Fee,
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
		PackagingItems:                      packaging.SnapshotParams,
		BillingGroupID:                      input.BillingGroupID,
		EnforceSingleActiveReservationOrder: reservation != nil && reservation.PaymentMode == "deposit",
		IdempotencyOperationScope:           orderCreateIdempotencyOperationScope(input),
		IdempotencyActorUserID:              orderCreateIdempotencyActor(input),
		IdempotencyKey:                      strings.TrimSpace(input.IdempotencyKey),
		IdempotencyRequestHash:              orderCreateRequestHash(input, packaging.Identity),
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
		if errors.Is(err, db.ErrVoucherTemplateUnavailable) {
			return CreateOrderCommandResult{}, NewRequestErrorWithCause(http.StatusBadRequest, errors.New("优惠券已停用或已失效"), err)
		}
		if errors.Is(err, db.ErrOrderCreateIdempotencyConflict) {
			return CreateOrderCommandResult{}, NewRequestErrorWithCause(http.StatusConflict, errors.New("订单请求状态已变化，请刷新后重试"), err)
		}
		if statusCode, ok := db.IsTxRequestError(err); ok && statusCode == http.StatusConflict {
			return CreateOrderCommandResult{}, NewRequestErrorWithCause(http.StatusConflict, errors.New("订单请求状态已变化，请刷新后重试"), err)
		}
		if err.Error() == "insufficient balance" {
			return CreateOrderCommandResult{}, NewRequestError(http.StatusBadRequest, errors.New("会员余额不足"))
		}
		return CreateOrderCommandResult{}, err
	}

	if s.taskScheduler != nil && txResult.Order.Status == db.OrderStatusPending && !txResult.IdempotencyReplayed {
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
		Order:          txResult.Order,
		PackagingItems: txResult.PackagingItems,
		RuleDecision:   ruleDecision,
		HasRule:        hasRule,
	}, nil
}

func orderCreateIdempotencyOperationScope(input CreateOrderCommandInput) string {
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return ""
	}
	return orderCreateIdempotencyScope
}

func orderCreateIdempotencyActor(input CreateOrderCommandInput) int64 {
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return 0
	}
	return input.UserID
}

func (s *OrderService) replayBoundOrderCreateIdempotency(ctx context.Context, input CreateOrderCommandInput) (CreateOrderCommandResult, bool, error) {
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		return CreateOrderCommandResult{}, false, nil
	}

	binding, err := s.store.GetOrderRequestIdempotency(ctx, db.GetOrderRequestIdempotencyParams{
		OperationScope: orderCreateIdempotencyScope,
		ActorUserID:    input.UserID,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return CreateOrderCommandResult{}, false, nil
		}
		return CreateOrderCommandResult{}, false, err
	}

	if binding.OrderID.Valid {
		order, err := s.store.GetOrder(ctx, binding.OrderID.Int64)
		if err != nil {
			return CreateOrderCommandResult{}, false, fmt.Errorf("get idempotent order: %w", err)
		}
		packagingItems, err := s.store.ListOrderPackagingItems(ctx, order.ID)
		if err != nil {
			return CreateOrderCommandResult{}, false, fmt.Errorf("list idempotent order packaging items: %w", err)
		}
		return CreateOrderCommandResult{
			Order:          order,
			PackagingItems: packagingItems,
		}, true, nil
	}
	if !orderCreateInputHasPackagingIdentity(input) {
		return CreateOrderCommandResult{}, false, nil
	}

	identity := orderCreatePackagingIdentity{
		Source:           orderCreatePackagingSourceNone,
		OptionID:         input.PackagingOptionID,
		SelectionVersion: input.PackagingSelectionVersion,
	}
	if allowedPackagingOrderType(input.OrderType) {
		cart, err := s.loadOrderCreatePackagingCart(ctx, input)
		if err != nil {
			return CreateOrderCommandResult{}, false, err
		}
		cartID := cart.ID
		identity.Source = orderCreatePackagingSourceCart
		identity.CartID = &cartID
	}
	if strings.TrimSpace(binding.RequestHash) != orderCreateRequestHash(input, identity) {
		return CreateOrderCommandResult{}, false, orderCreateStateConflict(db.ErrOrderCreateIdempotencyConflict)
	}
	return CreateOrderCommandResult{}, false, nil
}

func orderCreateInputHasPackagingIdentity(input CreateOrderCommandInput) bool {
	return input.PackagingOptionID != nil || input.PackagingSelectionVersion != nil
}

func (s *OrderService) resolveOrderCreatePackaging(ctx context.Context, input CreateOrderCommandInput) (orderCreatePackagingResolution, error) {
	result := orderCreatePackagingResolution{
		Identity: orderCreatePackagingIdentity{Source: orderCreatePackagingSourceNone},
	}
	if !allowedPackagingOrderType(input.OrderType) {
		return result, nil
	}

	settings, err := s.store.GetMerchantPackagingSettings(ctx, input.MerchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, nil
		}
		return result, err
	}
	if !settings.Enabled || !packagingAppliesToOrderType(settings.ApplicableOrderTypes, input.OrderType) {
		return result, nil
	}

	cart, err := s.loadOrderCreatePackagingCart(ctx, input)
	if err != nil {
		return result, err
	}
	cartID := cart.ID

	var selectedOptionID *int64
	selectionVersion := int64(0)
	selection, err := s.store.GetCartPackagingSelection(ctx, cart.ID)
	if err != nil {
		if !errors.Is(err, db.ErrRecordNotFound) {
			return result, err
		}
	} else {
		selectionVersion = selection.SelectionVersion
		if selection.PackagingOptionID.Valid {
			optionID := selection.PackagingOptionID.Int64
			selectedOptionID = &optionID
		}
	}

	if input.PackagingSelectionVersion == nil ||
		*input.PackagingSelectionVersion != selectionVersion ||
		!int64PtrValuesEqual(input.PackagingOptionID, selectedOptionID) {
		return result, orderCreateStateConflict(errors.New("packaging selection changed"))
	}
	if settings.Required && selectedOptionID == nil {
		return result, NewRequestError(http.StatusBadRequest, errors.New("请先选择包装方式"))
	}

	var selectedOption *db.MerchantPackagingOption
	if selectedOptionID != nil {
		option, err := s.store.GetMerchantPackagingOption(ctx, db.GetMerchantPackagingOptionParams{
			ID:         *selectedOptionID,
			MerchantID: input.MerchantID,
		})
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return result, NewRequestError(http.StatusBadRequest, errors.New("包装方式不可用"))
			}
			return result, err
		}
		if option.MerchantID != input.MerchantID || !option.IsEnabled || option.DeletedAt.Valid {
			return result, NewRequestError(http.StatusBadRequest, errors.New("包装方式不可用"))
		}
		selectedOption = &option
	}

	selectionVersionForHash := selectionVersion
	result.Identity = orderCreatePackagingIdentity{
		Source:           orderCreatePackagingSourceCart,
		CartID:           &cartID,
		OptionID:         selectedOptionID,
		SelectionVersion: &selectionVersionForHash,
	}
	if selectedOption != nil {
		snapshot := BuildOrderPackagingSnapshot(*selectedOption)
		result.Fee = snapshot.Subtotal
		result.SnapshotParams = []db.CreateOrderPackagingItemParams{orderPackagingSnapshotParam(snapshot)}
	}
	return result, nil
}

func (s *OrderService) loadOrderCreatePackagingCart(ctx context.Context, input CreateOrderCommandInput) (db.Cart, error) {
	cart, err := s.store.GetCartByUserAndMerchant(ctx, db.GetCartByUserAndMerchantParams{
		UserID:        input.UserID,
		MerchantID:    input.MerchantID,
		OrderType:     input.OrderType,
		TableID:       pgInt8FromInt64Ptr(input.TableID),
		ReservationID: pgInt8FromInt64Ptr(input.ReservationID),
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.Cart{}, orderCreateStateConflict(errors.New("cart not found for packaging selection"))
		}
		return db.Cart{}, err
	}
	if err := validateCartPackagingContext(cart, input.UserID, input.MerchantID, input.OrderType, input.TableID, input.ReservationID); err != nil {
		return db.Cart{}, err
	}
	return cart, nil
}

func orderPackagingSnapshotParam(snapshot OrderPackagingSnapshot) db.CreateOrderPackagingItemParams {
	return db.CreateOrderPackagingItemParams{
		PackagingOptionID: pgInt8FromInt64Ptr(snapshot.PackagingOptionID),
		Name:              snapshot.Name,
		UnitPrice:         snapshot.UnitPrice,
		Quantity:          snapshot.Quantity,
		Subtotal:          snapshot.Subtotal,
	}
}

func int64PtrValuesEqual(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func orderCreateStateConflict(cause error) error {
	return NewRequestErrorWithCause(http.StatusConflict, errors.New("订单请求状态已变化，请刷新后重试"), cause)
}

func orderCreateRequestHash(input CreateOrderCommandInput, identities ...orderCreatePackagingIdentity) string {
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return ""
	}
	packagingIdentity := orderCreatePackagingIdentity{Source: orderCreatePackagingSourceNone}
	if len(identities) > 0 {
		packagingIdentity = identities[0]
	}
	if packagingIdentity.Source == "" {
		packagingIdentity.Source = orderCreatePackagingSourceNone
	}
	parts := []string{
		"v2",
		orderCreateIdempotencyScope,
		strconv.FormatInt(input.UserID, 10),
		strconv.FormatInt(input.MerchantID, 10),
		strings.TrimSpace(input.OrderType),
		nullableInt64HashPart(input.AddressID),
		nullableInt64HashPart(input.TableID),
		nullableInt64HashPart(input.ReservationID),
		nullableInt64HashPart(input.BillingGroupID),
		nullableInt64HashPart(input.UserVoucherID),
		strconv.FormatBool(input.UseBalance),
		strings.TrimSpace(input.Notes),
		orderItemsHashPart(input.Items),
		packagingIdentity.Source,
		nullableInt64HashPart(packagingIdentity.CartID),
		nullableInt64HashPart(packagingIdentity.OptionID),
		nullableInt64HashPart(packagingIdentity.SelectionVersion),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("sha256:%x", sum[:])
}

func nullableInt64HashPart(value *int64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}

func orderItemsHashPart(items []OrderItemInput) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		customizations := ""
		if len(item.Customizations) > 0 {
			customizationJSON, err := json.Marshal(item.Customizations)
			if err == nil {
				customizations = string(customizationJSON)
			} else {
				keys := make([]string, 0, len(item.Customizations))
				for key := range item.Customizations {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				customizationParts := make([]string, 0, len(keys))
				for _, key := range keys {
					customizationParts = append(customizationParts, key+"="+fmt.Sprint(item.Customizations[key]))
				}
				customizations = strings.Join(customizationParts, "\x1f")
			}
		}
		parts = append(parts, strings.Join([]string{
			nullableInt64HashPart(item.DishID),
			nullableInt64HashPart(item.ComboID),
			strconv.FormatInt(int64(item.Quantity), 10),
			customizations,
		}, "\x1e"))
	}
	return strings.Join(parts, "\x1d")
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
		s.eventPublisher.PublishMerchantOrderSnapshot(ctx, result.Order.MerchantID, result.Order, merchantOrderSnapshotMessageTypeOrderUpdate)
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
	result, err := ReplaceReservationOrderWithBaofu(ctx, s.store, s.paymentFacade, input, normalize)
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
				Title:       "代取已完成",
				Content:     fmt.Sprintf("订单 %s 用户已确认收货", result.Order.OrderNo),
				RelatedType: "order",
				RelatedID:   result.Order.ID,
			})
		}
	}

	s.scheduleBaofuProfitSharingForCompletedOrder(ctx, result.Order)

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
		s.eventPublisher.PublishMerchantOrderSnapshot(ctx, input.MerchantID, result.Order, merchantOrderSnapshotMessageTypeOrderUpdate)
		if result.PoolItem != nil {
			s.eventPublisher.PublishTakeoutOrderPooled(ctx, result.Order, *result.PoolItem)
		}
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

	refundResult, refundErr := ProcessMerchantRejectRefund(ctx, s.store, s.paymentFacade, MerchantRejectRefundInput{
		MerchantID: input.MerchantID,
		OrderID:    result.Order.ID,
		Reason:     input.Reason,
	})
	if refundResult.Submission.Status != "" {
		submission := refundResult.Submission
		result.RefundSubmission = &submission
	}
	if refundErr != nil {
		if refundResult.RefundOrder != nil {
			log.Error().Err(refundErr).Int64("refund_order_id", refundResult.RefundOrder.ID).Msg("merchant reject refund failed")
		} else {
			log.Error().Err(refundErr).Int64("order_id", result.Order.ID).Msg("merchant reject refund failed")
		}
		if result.RefundSubmission == nil {
			result.RefundSubmission = &MerchantRefundSubmission{
				Status:  MerchantRefundSubmissionStatusManualRequired,
				Message: "订单已取消，但退款提交状态未知，请联系平台处理。",
			}
		}
	}

	if s.eventPublisher != nil {
		s.eventPublisher.PublishMerchantOrderSnapshot(ctx, input.MerchantID, result.Order, merchantOrderSnapshotMessageTypeOrderUpdate)
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
		s.eventPublisher.PublishMerchantOrderSnapshot(ctx, input.MerchantID, result.Order, merchantOrderSnapshotMessageTypeOrderUpdate)
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
		s.eventPublisher.PublishMerchantOrderSnapshot(ctx, input.MerchantID, result.Order, merchantOrderSnapshotMessageTypeOrderUpdate)
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

func (s *OrderService) scheduleBaofuProfitSharingForCompletedOrder(ctx context.Context, order db.Order) {
	if s.taskScheduler == nil {
		return
	}
	profitSharingOrder, err := ResolveCompletedOrderBaofuProfitSharingOrder(ctx, s.store, order)
	if err != nil {
		log.Warn().Err(err).Int64("order_id", order.ID).Msg("skip scheduling baofu profit sharing for completed order")
		return
	}
	if profitSharingOrder.ID <= 0 {
		return
	}
	if err := s.taskScheduler.ScheduleProfitSharing(ctx, profitSharingOrder.ID); err != nil {
		log.Warn().Err(err).
			Int64("order_id", order.ID).
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Msg("schedule baofu profit sharing failed")
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
