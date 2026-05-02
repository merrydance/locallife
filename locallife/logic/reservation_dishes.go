package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"

	"github.com/rs/zerolog/log"
)

const (
	reservationAddonBusiness = "reservation_addon"
	paymentStatusPaid        = "paid"
	reservationRefundReason  = "Reservation dish change refund"
)

// AddReservationDishesInput describes the add-dishes request.
type AddReservationDishesInput struct {
	UserID          int64
	ReservationID   int64
	Items           []ReservationItemInput
	Now             time.Time
	EcommerceClient wechat.EcommerceClientInterface
	OrdinaryClient  ordinaryServiceProviderCombineClient
	ClientIP        string
}

// AddReservationDishesResult returns the add-dishes outcome.
type AddReservationDishesResult struct {
	Reservation db.TableReservation
	AddedAmount int64
	Payment     *db.PaymentOrder
	PayParams   *wechat.JSAPIPayParams
}

// AddReservationDishes validates and appends reservation items, optionally creating a payment order.
func AddReservationDishes(ctx context.Context, store db.Store, input AddReservationDishesInput) (AddReservationDishesResult, error) {
	var result AddReservationDishesResult

	if len(input.Items) == 0 {
		return result, NewRequestError(http.StatusBadRequest, errors.New("at least one item is required"))
	}

	reservation, err := store.GetTableReservationForUpdate(ctx, input.ReservationID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
		}
		return result, err
	}

	if reservation.UserID != input.UserID {
		return result, NewRequestError(http.StatusForbidden, errors.New("you can only add dishes to your own reservation"))
	}
	if reservation.Status != reservationStatusPaid && reservation.Status != reservationStatusConfirmed {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("cannot add dishes to reservation in %s status", reservation.Status))
	}

	validatedItems, addedAmount, err := ValidateReservationItems(ctx, store, reservation.MerchantID, input.Items)
	if err != nil {
		return result, err
	}

	for _, item := range validatedItems {
		var dishID, comboID pgtype.Int8
		if item.DishID != nil {
			dishID = pgtype.Int8{Int64: *item.DishID, Valid: true}
		}
		if item.ComboID != nil {
			comboID = pgtype.Int8{Int64: *item.ComboID, Valid: true}
		}

		_, err := store.CreateReservationItem(ctx, db.CreateReservationItemParams{
			ReservationID: reservation.ID,
			DishID:        dishID,
			ComboID:       comboID,
			Quantity:      item.Quantity,
			UnitPrice:     item.UnitPrice,
			TotalPrice:    item.UnitPrice * int64(item.Quantity),
		})
		if err != nil {
			return result, err
		}
	}

	result.Reservation = reservation
	result.AddedAmount = addedAmount

	if reservation.PaymentMode == paymentModeFull {
		paymentOrder, payParams, err := createReservationAddonPaymentOrder(ctx, store, input.EcommerceClient, input.OrdinaryClient, reservation, input.UserID, addedAmount, input.Now, input.ClientIP)
		if err != nil {
			return result, err
		}
		result.Payment = &paymentOrder
		result.PayParams = payParams
	}

	return result, nil
}

// ModifyReservationDishesInput describes a modify-dishes request.
type ModifyReservationDishesInput struct {
	UserID          int64
	ReservationID   int64
	Items           []ReservationItemInput
	Now             time.Time
	EcommerceClient wechat.EcommerceClientInterface
	OrdinaryClient  ordinaryServiceProviderCombineClient
	ClientIP        string
	TaskScheduler   TaskScheduler
}

// ModifyReservationDishesResult returns modify-dishes outcomes.
type ModifyReservationDishesResult struct {
	Reservation     db.TableReservation
	Delta           int64
	Payment         *db.PaymentOrder
	PayParams       *wechat.JSAPIPayParams
	RefundAmount    int64
	RefundInitiated bool
}

type reservationRefundAllocation struct {
	PaymentOrder db.PaymentOrder
	RefundAmount int64
}

// ModifyReservationDishes replaces reservation items and handles payment/refund if needed.
func ModifyReservationDishes(
	ctx context.Context,
	store db.Store,
	input ModifyReservationDishesInput,
) (ModifyReservationDishesResult, error) {
	var result ModifyReservationDishesResult

	if len(input.Items) == 0 {
		return result, NewRequestError(http.StatusBadRequest, errors.New("at least one item is required"))
	}

	reservation, err := store.GetTableReservationForUpdate(ctx, input.ReservationID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
		}
		return result, err
	}

	if reservation.UserID != input.UserID {
		return result, NewRequestError(http.StatusForbidden, errors.New("you can only modify your own reservation"))
	}
	if reservation.Status != reservationStatusPaid && reservation.Status != reservationStatusConfirmed && reservation.Status != reservationStatusCheckedIn {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("cannot modify reservation in %s status", reservation.Status))
	}
	if reservation.CookingStartedAt.Valid {
		return result, NewRequestError(http.StatusConflict, errors.New("cooking already started, modification is not allowed"))
	}

	currentTotal, err := store.SumReservationItemsTotal(ctx, reservation.ID)
	if err != nil {
		return result, err
	}

	validatedItems, newTotal, err := ValidateReservationItems(ctx, store, reservation.MerchantID, input.Items)
	if err != nil {
		return result, err
	}

	delta := newTotal - currentTotal

	var refundAllocations []reservationRefundAllocation
	if reservation.PaymentMode == paymentModeFull && delta < 0 {
		refundAllocations, err = buildReservationRefundAllocations(ctx, store, reservation.ID, minInt64(-delta, reservation.PrepaidAmount))
		if err != nil {
			return result, err
		}
	}

	createItems := make([]db.CreateReservationItemParams, 0, len(validatedItems))
	for _, item := range validatedItems {
		var dishID, comboID pgtype.Int8
		if item.DishID != nil {
			dishID = pgtype.Int8{Int64: *item.DishID, Valid: true}
		}
		if item.ComboID != nil {
			comboID = pgtype.Int8{Int64: *item.ComboID, Valid: true}
		}
		createItems = append(createItems, db.CreateReservationItemParams{
			ReservationID: reservation.ID,
			DishID:        dishID,
			ComboID:       comboID,
			Quantity:      item.Quantity,
			UnitPrice:     item.UnitPrice,
			TotalPrice:    item.UnitPrice * int64(item.Quantity),
		})
	}

	if _, err := store.ReplaceReservationItemsTx(ctx, db.ReplaceReservationItemsTxParams{
		ReservationID: reservation.ID,
		Items:         createItems,
	}); err != nil {
		return result, err
	}

	result.Reservation = reservation
	result.Delta = delta

	if reservation.PaymentMode != paymentModeFull || delta == 0 {
		return result, nil
	}

	if delta > 0 {
		paymentOrder, payParams, err := createReservationAddonPaymentOrder(ctx, store, input.EcommerceClient, input.OrdinaryClient, reservation, input.UserID, delta, input.Now, input.ClientIP)
		if err != nil {
			return result, err
		}
		result.Payment = &paymentOrder
		result.PayParams = payParams
		return result, nil
	}

	refundAmount := sumReservationRefundAllocations(refundAllocations)
	if refundAmount <= 0 {
		return result, nil
	}
	for _, allocation := range refundAllocations {
		outRefundNo, genErr := generateOutRefundNo()
		if genErr != nil {
			return result, fmt.Errorf("generate out refund no: %w", genErr)
		}

		refundType := refundTypeForPaymentOrder(allocation.PaymentOrder)

		if _, createErr := store.CreateRefundOrderTx(ctx, db.CreateRefundOrderTxParams{
			PaymentOrderID: allocation.PaymentOrder.ID,
			RefundType:     refundType,
			RefundAmount:   allocation.RefundAmount,
			RefundReason:   reservationRefundReason,
			OutRefundNo:    outRefundNo,
		}); createErr != nil {
			if statusCode, ok := db.IsRefundRequestError(createErr); ok {
				return result, NewRequestError(statusCode, errors.Unwrap(createErr))
			}
			return result, fmt.Errorf("create reservation dish change refund order: %w", createErr)
		}

		if input.TaskScheduler == nil {
			log.Error().
				Int64("payment_order_id", allocation.PaymentOrder.ID).
				Str("out_refund_no", outRefundNo).
				Msg("reservation dish change refund task scheduler not configured; relying on recovery scheduler")
			continue
		}

		scheduleErr := input.TaskScheduler.ScheduleProcessRefund(ctx, ProcessRefundTaskInput{
			PaymentOrderID: allocation.PaymentOrder.ID,
			ReservationID:  reservation.ID,
			RefundAmount:   allocation.RefundAmount,
			Reason:         reservationRefundReason,
			OutRefundNo:    outRefundNo,
		})
		if scheduleErr != nil {
			log.Error().
				Err(scheduleErr).
				Int64("payment_order_id", allocation.PaymentOrder.ID).
				Str("out_refund_no", outRefundNo).
				Msg("failed to enqueue reservation dish change refund task; pending recovery remains available")
		}
	}

	result.RefundAmount = refundAmount
	result.RefundInitiated = true
	return result, nil
}

func buildReservationRefundAllocations(
	ctx context.Context,
	store db.Store,
	reservationID int64,
	desiredRefundAmount int64,
) ([]reservationRefundAllocation, error) {
	if desiredRefundAmount <= 0 {
		return nil, nil
	}

	paymentOrders, err := store.GetPaymentOrdersByReservation(ctx, pgtype.Int8{Int64: reservationID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("get payment orders by reservation: %w", err)
	}

	allocations := make([]reservationRefundAllocation, 0)
	remainingRefund := desiredRefundAmount
	for _, paymentOrder := range paymentOrders {
		if remainingRefund <= 0 {
			break
		}
		if paymentOrder.Status != paymentStatusPaid {
			continue
		}
		if paymentOrder.BusinessType != businessTypeReservation &&
			paymentOrder.BusinessType != reservationAddonBusiness &&
			!(paymentOrder.BusinessType == businessTypeOrder && paymentOrder.ReservationID.Valid) {
			continue
		}

		occupiedAmount, occupiedErr := store.GetTotalRefundedByPaymentOrder(ctx, paymentOrder.ID)
		if occupiedErr != nil {
			return nil, fmt.Errorf("get occupied refund amount by payment order %d: %w", paymentOrder.ID, occupiedErr)
		}

		availableAmount := paymentOrder.Amount - occupiedAmount
		if availableAmount <= 0 {
			continue
		}

		allocationAmount := minInt64(remainingRefund, availableAmount)
		allocations = append(allocations, reservationRefundAllocation{
			PaymentOrder: paymentOrder,
			RefundAmount: allocationAmount,
		})
		remainingRefund -= allocationAmount
	}

	return allocations, nil
}

func sumReservationRefundAllocations(allocations []reservationRefundAllocation) int64 {
	var total int64
	for _, allocation := range allocations {
		total += allocation.RefundAmount
	}
	return total
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func createReservationAddonPaymentOrder(
	ctx context.Context,
	store db.Store,
	ecommerceClient wechat.EcommerceClientInterface,
	ordinaryClient ordinaryServiceProviderCombineClient,
	reservation db.TableReservation,
	userID, amount int64,
	now time.Time,
	clientIP string,
) (db.PaymentOrder, *wechat.JSAPIPayParams, error) {
	if amount <= 0 {
		return db.PaymentOrder{}, nil, NewRequestError(http.StatusBadRequest, errors.New("payment amount must be greater than 0"))
	}
	if ordinaryClient == nil {
		return db.PaymentOrder{}, nil, fmt.Errorf("ordinary service provider client: not configured")
	}
	usesOrdinary := true

	user, err := store.GetUser(ctx, userID)
	if err != nil {
		return db.PaymentOrder{}, nil, fmt.Errorf("get user: %w", err)
	}
	if user.WechatOpenid == "" {
		return db.PaymentOrder{}, nil, NewRequestError(http.StatusBadRequest, errors.New("wechat openid not found"))
	}

	expiresAt := now.Add(30 * time.Minute)

	combineOutTradeNo, err := generateCombineOutTradeNoForSingle("RA")
	if err != nil {
		return db.PaymentOrder{}, nil, fmt.Errorf("generate combine out trade no: %w", err)
	}

	var txResult db.CreateEcommercePaymentTxResult
	paymentChannel := db.PaymentChannelEcommerce
	if usesOrdinary {
		paymentChannel = db.PaymentChannelOrdinaryServiceProvider
	}
	for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
		outTradeNo, genErr := generateOutTradeNoWithPrefix("RA")
		if genErr != nil {
			return db.PaymentOrder{}, nil, fmt.Errorf("generate out trade no: %w", genErr)
		}
		txResult, err = store.CreateEcommercePaymentTx(ctx, db.CreateEcommercePaymentTxParams{
			UserID:            userID,
			MerchantID:        reservation.MerchantID,
			Amount:            amount,
			BusinessType:      reservationAddonBusiness,
			ReservationID:     reservation.ID,
			CombineOutTradeNo: combineOutTradeNo,
			OutTradeNo:        outTradeNo,
			ExpiresAt:         expiresAt,
			Attach:            "",
			PaymentChannel:    paymentChannel,
		})
		if err == nil {
			break
		}
		if isOutTradeNoConflict(err) && attempt < outTradeNoMaxRetry {
			if !sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
				return db.PaymentOrder{}, nil, NewRequestError(http.StatusRequestTimeout, errors.New("request canceled"))
			}
			continue
		}
		return db.PaymentOrder{}, nil, mapReservationEcommerceError(err)
	}

	prepayID, payParams, err := createRemoteReservationAddonPayment(ctx, ecommerceClient, ordinaryClient, usesOrdinary, combineOutTradeNo, txResult.SubMchID, txResult.PaymentOrder.OutTradeNo, user.WechatOpenid, amount, expiresAt, clientIP)
	if err != nil {
		cleanupCtx := context.Background()
		if closeReservationAddonPaymentCommandAnchor(cleanupCtx, store, txResult.PaymentOrder, txResult.CombinedPaymentOrder) {
			recordReservationAddonCombinePaymentCommandRejected(cleanupCtx, store, txResult.PaymentOrder, txResult.CombinedPaymentOrder, err)
		}
		return db.PaymentOrder{}, nil, fmt.Errorf("create combine order: %w", err)
	}
	if prepayID == "" {
		cleanupCtx := context.Background()
		emptyPrepayErr := errors.New("create combine order: empty prepay id")
		if closeReservationAddonPaymentCommandAnchor(cleanupCtx, store, txResult.PaymentOrder, txResult.CombinedPaymentOrder) {
			recordReservationAddonCombinePaymentCommandRejected(cleanupCtx, store, txResult.PaymentOrder, txResult.CombinedPaymentOrder, emptyPrepayErr)
		}
		return db.PaymentOrder{}, nil, fmt.Errorf("create combine order: empty prepay id")
	}

	updatedPayment, err := store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       txResult.PaymentOrder.ID,
		PrepayID: pgtype.Text{String: prepayID, Valid: true},
	})
	if err != nil {
		cleanupCtx := context.Background()
		markReservationAddonPaymentOrderFailedForCleanup(cleanupCtx, store, txResult.PaymentOrder.ID, "failed to mark reservation addon payment order failed after prepay update failure")
		markReservationAddonCombinedPaymentOrderFailedForCleanup(cleanupCtx, store, txResult.CombinedPaymentOrder.ID, "failed to mark reservation addon combined payment order failed after prepay update failure")
		closeReservationAddonRemoteCombineForCleanup(cleanupCtx, ordinaryClient, txResult, "close ordinary service provider reservation addon combine order after prepay update failure")
		return db.PaymentOrder{}, nil, fmt.Errorf("update prepay id: %w", err)
	}

	_, err = store.UpdateCombinedPaymentOrderPrepay(ctx, db.UpdateCombinedPaymentOrderPrepayParams{
		ID:       txResult.CombinedPaymentOrder.ID,
		PrepayID: pgtype.Text{String: prepayID, Valid: true},
	})
	if err != nil {
		cleanupCtx := context.Background()
		markReservationAddonPaymentOrderFailedForCleanup(cleanupCtx, store, txResult.PaymentOrder.ID, "failed to mark reservation addon payment order failed after combined prepay update failure")
		markReservationAddonCombinedPaymentOrderFailedForCleanup(cleanupCtx, store, txResult.CombinedPaymentOrder.ID, "failed to mark reservation addon combined payment order failed after combined prepay update failure")
		closeReservationAddonRemoteCombineForCleanup(cleanupCtx, ordinaryClient, txResult, "close ordinary service provider reservation addon combine order after combined prepay update failure")
		return db.PaymentOrder{}, nil, fmt.Errorf("update combined payment prepay: %w", err)
	}
	recordReservationAddonCombinePaymentCommandAccepted(ctx, store, updatedPayment, txResult.CombinedPaymentOrder, prepayID)

	return updatedPayment, payParams, nil
}

func createRemoteReservationAddonPayment(
	ctx context.Context,
	ecommerceClient wechat.EcommerceClientInterface,
	ordinaryClient ordinaryServiceProviderCombineClient,
	usesOrdinary bool,
	combineOutTradeNo string,
	subMchID string,
	outTradeNo string,
	openID string,
	amount int64,
	expiresAt time.Time,
	clientIP string,
) (string, *wechat.JSAPIPayParams, error) {
	if usesOrdinary {
		combineResp, err := ordinaryClient.CreateCombinePayment(ctx, ospcontracts.CombinePrepayRequest{
			CombineAppID:      ordinaryClient.ServiceProviderAppID(),
			CombineMchID:      ordinaryClient.ServiceProviderMchID(),
			CombineOutTradeNo: combineOutTradeNo,
			CombinePayerInfo:  ospcontracts.CombinePayerInfo{OpenID: openID},
			SubOrders: []ospcontracts.CombineSubOrder{
				{
					MchID:       ordinaryClient.ServiceProviderMchID(),
					SubMchID:    subMchID,
					Amount:      ospcontracts.CombineSubOrderAmount{TotalAmount: amount, Currency: ospcontracts.CurrencyCNY},
					OutTradeNo:  outTradeNo,
					Description: "Reservation add-on",
				},
			},
			TimeExpire: expiresAt.Format(time.RFC3339),
			NotifyURL:  ordinaryClient.CombineNotifyURL(),
			SceneInfo:  &ospcontracts.CombineSceneInfo{PayerClientIP: clientIP},
		})
		if err != nil {
			return "", nil, err
		}
		if combineResp == nil || combineResp.PrepayID == "" {
			return "", nil, nil
		}
		payParams, err := ordinaryClient.GenerateJSAPIPayParams(combineResp.PrepayID)
		if err != nil {
			return "", nil, fmt.Errorf("generate ordinary service provider combine pay params: %w", err)
		}
		return combineResp.PrepayID, ordinaryJSAPIPayParamsToWechat(payParams), nil
	}

	combineResp, payParams, err := ecommerceClient.CreateCombineOrder(ctx, &wechatcontracts.CombineOrderRequest{
		CombineOutTradeNo: combineOutTradeNo,
		SubOrders: []wechatcontracts.SubOrder{
			{
				SubMchID:    subMchID,
				Amount:      amount,
				OutTradeNo:  outTradeNo,
				Description: "Reservation add-on",
				Attach:      "",
			},
		},
		PayerOpenID: openID,
		ExpireTime:  expiresAt,
		SceneInfo: &wechatcontracts.CombineSceneInfo{
			PayerClientIP: clientIP,
		},
	})
	if err != nil {
		return "", nil, err
	}
	if combineResp == nil {
		return "", payParams, nil
	}
	return combineResp.PrepayID, payParams, nil
}

func closeReservationAddonPaymentCommandAnchor(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, combinedPayment db.CombinedPaymentOrder) bool {
	allClosed := true
	if _, closeErr := store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID); closeErr != nil {
		allClosed = false
		log.Error().Err(closeErr).Int64("payment_order_id", paymentOrder.ID).Msg("failed to close reservation addon payment order after create rejection")
	}
	if _, closeErr := store.UpdateCombinedPaymentOrderToClosed(ctx, combinedPayment.ID); closeErr != nil {
		allClosed = false
		log.Error().Err(closeErr).Int64("combined_payment_order_id", combinedPayment.ID).Msg("failed to close reservation addon combined payment order after create rejection")
	}
	return allClosed
}

func markReservationAddonPaymentOrderFailedForCleanup(ctx context.Context, store db.Store, paymentOrderID int64, message string) {
	if _, err := store.UpdatePaymentOrderToFailed(ctx, paymentOrderID); err != nil {
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrderID).
			Msg(message)
	}
}

func markReservationAddonCombinedPaymentOrderFailedForCleanup(ctx context.Context, store db.Store, combinedPaymentOrderID int64, message string) {
	if _, err := store.UpdateCombinedPaymentOrderToFailed(ctx, combinedPaymentOrderID); err != nil {
		log.Error().Err(err).
			Int64("combined_payment_order_id", combinedPaymentOrderID).
			Msg(message)
	}
}

func closeReservationAddonRemoteCombineForCleanup(ctx context.Context, ordinaryClient ordinaryServiceProviderCombineClient, txResult db.CreateEcommercePaymentTxResult, message string) {
	if ordinaryClient == nil {
		return
	}
	if err := ordinaryClient.CloseCombinePayment(ctx, ospcontracts.CombineCloseRequest{
		CombineAppID:      ordinaryClient.ServiceProviderAppID(),
		CombineMchID:      ordinaryClient.ServiceProviderMchID(),
		CombineOutTradeNo: txResult.CombinedPaymentOrder.CombineOutTradeNo,
		SubOrders: []ospcontracts.CombineCloseSubOrder{
			{
				MchID:      ordinaryClient.ServiceProviderMchID(),
				SubMchID:   txResult.SubMchID,
				OutTradeNo: txResult.PaymentOrder.OutTradeNo,
			},
		},
	}); err != nil {
		log.Warn().Err(err).
			Int64("payment_order_id", txResult.PaymentOrder.ID).
			Int64("combined_payment_order_id", txResult.CombinedPaymentOrder.ID).
			Str("combine_out_trade_no", txResult.CombinedPaymentOrder.CombineOutTradeNo).
			Str("out_trade_no", txResult.PaymentOrder.OutTradeNo).
			Msg(message)
	}
}

func recordReservationAddonCombinePaymentCommandAccepted(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, combinedPayment db.CombinedPaymentOrder, prepayID string) {
	paymentCommandSvc := NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbReservationAddonCombinePaymentCommandInput(
		paymentOrder,
		combinedPayment,
		db.ExternalPaymentCommandStatusAccepted,
		stringPtrIfNotEmpty(prepayID),
		nil,
		nil,
		combinePaymentCommandSnapshot(map[string]string{
			"combine_out_trade_no": combinedPayment.CombineOutTradeNo,
			"out_trade_no":         paymentOrder.OutTradeNo,
			"prepay_id":            prepayID,
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrder.ID).
			Str("combine_out_trade_no", combinedPayment.CombineOutTradeNo).
			Msg("record reservation addon combine payment command accepted failed")
	}
}

func recordReservationAddonCombinePaymentCommandRejected(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, combinedPayment db.CombinedPaymentOrder, paymentErr error) {
	paymentCommandSvc := NewPaymentCommandService(store)
	errorCode, errorMessage := partnerPaymentCommandErrorFields(paymentErr)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbReservationAddonCombinePaymentCommandInput(
		paymentOrder,
		combinedPayment,
		db.ExternalPaymentCommandStatusRejected,
		nil,
		errorCode,
		errorMessage,
		combinePaymentCommandSnapshot(map[string]string{
			"combine_out_trade_no": combinedPayment.CombineOutTradeNo,
			"out_trade_no":         paymentOrder.OutTradeNo,
			"error_code":           stringValue(errorCode),
			"error_message":        stringValue(errorMessage),
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrder.ID).
			Str("combine_out_trade_no", combinedPayment.CombineOutTradeNo).
			Msg("record reservation addon combine payment command rejected failed")
	}
}

func dbReservationAddonCombinePaymentCommandInput(
	paymentOrder db.PaymentOrder,
	combinedPayment db.CombinedPaymentOrder,
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
		Capability:           db.ExternalPaymentCapabilityCombinePayment,
		CommandType:          db.ExternalPaymentCommandTypeCreatePayment,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerReservation,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectCombinedPayment,
		ExternalObjectKey:    combinedPayment.CombineOutTradeNo,
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        commandStatus,
		LastErrorCode:        lastErrorCode,
		LastErrorMessage:     lastErrorMessage,
		ResponseSnapshot:     responseSnapshot,
	}
}
