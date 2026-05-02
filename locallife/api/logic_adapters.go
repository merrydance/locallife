package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

type apiNotificationPublisher struct {
	server *Server
}

func (p apiNotificationPublisher) Send(ctx context.Context, input logic.NotificationInput) error {
	extra := map[string]any{}
	if input.OrderNo != "" {
		extra["order_no"] = input.OrderNo
	}
	if input.OrderStatus != "" {
		extra["order_status"] = input.OrderStatus
	}

	params := SendNotificationParams{
		UserID:            input.UserID,
		Type:              input.Type,
		Title:             input.Title,
		Content:           input.Content,
		RelatedType:       input.RelatedType,
		RelatedID:         input.RelatedID,
		ExtraData:         extra,
		ExpiresAt:         input.ExpiresAt,
		IgnorePreferences: input.IgnorePreferences,
		PushToRider:       input.PushToRider,
		PushToMerchant:    input.PushToMerchant,
		RiderID:           input.RiderID,
		MerchantID:        input.MerchantID,
	}

	return p.server.SendNotification(ctx, params)
}

type apiOrderEventPublisher struct {
	server *Server
}

func (p apiOrderEventPublisher) PublishMerchantOrderSnapshot(ctx context.Context, merchantID int64, order db.Order, messageType string) {
	if p.server.wsHub == nil {
		return
	}

	response, err := newOrderResponse(order)
	if err != nil {
		log.Error().Err(err).
			Int64("merchant_id", merchantID).
			Int64("order_id", order.ID).
			Str("message_type", messageType).
			Msg("build merchant order snapshot failed")
		return
	}

	payload, err := json.Marshal(response)
	if err != nil {
		log.Error().Err(err).
			Int64("merchant_id", merchantID).
			Int64("order_id", order.ID).
			Str("message_type", messageType).
			Msg("marshal merchant order snapshot failed")
		return
	}
	p.server.wsHub.SendToMerchant(merchantID, websocket.Message{
		Type:      messageType,
		Data:      payload,
		Timestamp: time.Now(),
	})
}

func (p apiOrderEventPublisher) PublishMerchantUserRiskAlert(ctx context.Context, merchantID int64, alert logic.MerchantUserRiskAlert) {
	if p.server.wsHub == nil {
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"user_id":     alert.UserID,
		"order_id":    alert.OrderID,
		"order_no":    alert.OrderNo,
		"message":     alert.Message,
		"reason_code": alert.ReasonCode,
	})

	p.server.wsHub.SendToMerchant(merchantID, websocket.Message{
		Type:      "merchant_user_risk_alert",
		Data:      payload,
		Timestamp: time.Now(),
	})
}

func (p apiOrderEventPublisher) PublishTakeoutOrderPooled(ctx context.Context, order db.Order, poolItem db.DeliveryPool) {
	if p.server.deliveryBroadcast == nil {
		return
	}

	merchant, err := p.server.store.GetMerchant(ctx, order.MerchantID)
	if err != nil {
		log.Error().Err(err).Int64("merchant_id", order.MerchantID).Msg("get merchant for pooled takeout broadcast failed")
		return
	}

	_ = p.server.deliveryBroadcast.BroadcastNewOrderNotification(ctx, poolItem, merchant.Name)
}

type apiTaskScheduler struct {
	server *Server
}

func (s apiTaskScheduler) ScheduleOrderPaymentTimeout(ctx context.Context, orderID int64, at time.Time) error {
	if s.server.taskDistributor == nil {
		return nil
	}
	return s.server.taskDistributor.DistributeTaskOrderPaymentTimeout(ctx, &worker.PayloadOrderPaymentTimeout{OrderID: orderID}, asynq.ProcessAt(at))
}

func (s apiTaskScheduler) SchedulePaymentOrderTimeout(ctx context.Context, paymentOrderNo string, at time.Time) error {
	if s.server.taskDistributor == nil {
		return nil
	}
	return s.server.taskDistributor.DistributeTaskPaymentOrderTimeout(ctx, &worker.PayloadPaymentOrderTimeout{PaymentOrderNo: paymentOrderNo}, asynq.ProcessAt(at))
}

func (s apiTaskScheduler) ScheduleCombinedPaymentOrderTimeout(ctx context.Context, combineOutTradeNo string, at time.Time) error {
	if s.server.taskDistributor == nil {
		return nil
	}
	return s.server.taskDistributor.DistributeTaskCombinedPaymentOrderTimeout(ctx, &worker.PayloadCombinedPaymentOrderTimeout{CombineOutTradeNo: combineOutTradeNo}, asynq.ProcessAt(at))
}

func (s apiTaskScheduler) ScheduleProcessRefund(ctx context.Context, input logic.ProcessRefundTaskInput) error {
	if s.server.taskDistributor == nil {
		return nil
	}
	return s.server.taskDistributor.DistributeTaskProcessRefund(ctx, &worker.PayloadProcessRefund{
		PaymentOrderID: input.PaymentOrderID,
		OrderID:        input.OrderID,
		ReservationID:  input.ReservationID,
		RefundAmount:   input.RefundAmount,
		Reason:         input.Reason,
		OutRefundNo:    input.OutRefundNo,
	})
}

func (s apiTaskScheduler) ScheduleProfitSharing(ctx context.Context, paymentOrderID, orderID int64) error {
	if s.server.taskDistributor == nil {
		return nil
	}
	if orderID > 0 {
		order, err := s.server.store.GetOrder(ctx, orderID)
		if err != nil {
			return err
		}
		if order.OrderType == db.OrderTypeTakeout {
			log.Info().
				Int64("payment_order_id", paymentOrderID).
				Int64("order_id", orderID).
				Str("order_type", order.OrderType).
				Msg("skip takeout profit sharing scheduling outside settlement")
			return nil
		}
		if (order.OrderType == "dine_in" && !order.ReservationID.Valid) || order.OrderType == "takeaway" {
			log.Info().
				Int64("payment_order_id", paymentOrderID).
				Int64("order_id", orderID).
				Str("order_type", order.OrderType).
				Msg("skip non-profit-sharing order scheduling outside settlement")
			return nil
		}
	}
	return s.server.taskDistributor.DistributeTaskProcessProfitSharing(ctx, &worker.ProfitSharingPayload{
		PaymentOrderID: paymentOrderID,
		OrderID:        orderID,
	})
}

func (s apiTaskScheduler) ScheduleProfitSharingReturnResult(ctx context.Context, input logic.ProfitSharingReturnResultTaskInput) error {
	if s.server.taskDistributor == nil {
		return nil
	}
	return s.server.taskDistributor.DistributeTaskProcessProfitSharingReturnResult(ctx, &worker.ProfitSharingReturnResultPayload{
		ProfitSharingReturnID: input.ProfitSharingReturnID,
		OutReturnNo:           input.OutReturnNo,
		OutOrderNo:            input.OutOrderNo,
		SubMchID:              input.SubMchID,
		RefundOrderID:         input.RefundOrderID,
		RetryCount:            input.RetryCount,
	}, asynq.ProcessIn(input.Delay))
}

func (s apiTaskScheduler) ScheduleOrderPrint(ctx context.Context, input logic.OrderPrintTaskInput) error {
	if s.server.taskDistributor == nil {
		return nil
	}
	taskKey := buildStableOrderPrintTaskKey(input.OrderID, input.Trigger)
	if input.Trigger == "manual" {
		taskKey = buildUniqueOrderPrintTaskKey(input.OrderID, input.Trigger)
	}
	return s.server.taskDistributor.DistributeTaskPrintOrder(ctx, &worker.PrintOrderPayload{
		OrderID: input.OrderID,
		Trigger: input.Trigger,
		TaskKey: taskKey,
	})
}

type apiDishCustomizationNormalizer struct {
	server *Server
}

func (n apiDishCustomizationNormalizer) Normalize(ctx context.Context, dishID int64, customizations map[string]interface{}) ([]byte, int64, error) {
	groups, err := n.server.loadDishCustomizationMetaFromContext(ctx, dishID)
	if err != nil {
		return nil, 0, err
	}

	selections, err := parseCustomizationSelections(customizations)
	if err != nil {
		return nil, 0, err
	}
	if len(customizations) > 0 && len(selections) == 0 {
		return nil, 0, fmt.Errorf("customizations must include at least one valid selection")
	}

	selectedByGroup := make(map[int64]int64, len(selections))
	for _, selection := range selections {
		if _, exists := selectedByGroup[selection.GroupID]; exists {
			return nil, 0, fmt.Errorf("duplicate customization group %d", selection.GroupID)
		}
		selectedByGroup[selection.GroupID] = selection.OptionID
	}

	for _, group := range groups {
		if group.IsRequired {
			if _, exists := selectedByGroup[group.ID]; !exists {
				return nil, 0, fmt.Errorf("missing required customization group %s", group.Name)
			}
		}
	}

	normalized := make(map[string]interface{}, len(selectedByGroup)+1)
	var extraPrice int64
	for groupID, optionID := range selectedByGroup {
		group, exists := groups[groupID]
		if !exists {
			return nil, 0, fmt.Errorf("customization group %d not found", groupID)
		}
		option, exists := group.Options[optionID]
		if !exists {
			return nil, 0, fmt.Errorf("customization option %d not found in group %s", optionID, group.Name)
		}
		normalized[fmt.Sprintf("%d", groupID)] = optionID
		extraPrice += option.ExtraPrice
	}

	if summary := buildCustomizationSummary(groups, selectedByGroup); summary != "" {
		normalized["meta_specs"] = summary
	}

	if len(normalized) == 0 {
		return nil, extraPrice, nil
	}

	data, err := json.Marshal(normalized)
	if err != nil {
		return nil, 0, err
	}

	return data, extraPrice, nil
}

func (server *Server) loadDishCustomizationMetaFromContext(ctx context.Context, dishID int64) (map[int64]customizationGroupMeta, error) {
	dish, err := server.store.GetDishWithCustomizations(ctx, dishID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, fmt.Errorf("dish not found")
		}
		return nil, err
	}

	if dish.CustomizationGroups == nil {
		return map[int64]customizationGroupMeta{}, nil
	}

	groupsJSON, err := json.Marshal(dish.CustomizationGroups)
	if err != nil {
		return nil, fmt.Errorf("marshal customization groups: %w", err)
	}

	var groups []customizationGroupJSON
	if err := json.Unmarshal(groupsJSON, &groups); err != nil {
		return nil, fmt.Errorf("unmarshal customization groups: %w", err)
	}

	meta := make(map[int64]customizationGroupMeta, len(groups))
	for _, group := range groups {
		options := make(map[int64]customizationOptionMeta, len(group.Options))
		for _, option := range group.Options {
			options[option.ID] = customizationOptionMeta{
				ID:         option.ID,
				TagID:      option.TagID,
				TagName:    option.TagName,
				ExtraPrice: option.ExtraPrice,
				SortOrder:  option.SortOrder,
			}
		}
		meta[group.ID] = customizationGroupMeta{
			ID:         group.ID,
			Name:       group.Name,
			SortOrder:  group.SortOrder,
			IsRequired: group.IsRequired,
			Options:    options,
		}
	}

	return meta, nil
}

func (server *Server) buildOrderCommandService() logic.OrderCommandService {
	var eventPublisher logic.OrderEventPublisher
	if server.wsHub != nil || server.deliveryBroadcast != nil {
		eventPublisher = apiOrderEventPublisher{server: server}
	}

	var taskScheduler logic.TaskScheduler
	if server.taskDistributor != nil {
		taskScheduler = apiTaskScheduler{server: server}
	}

	service := logic.NewOrderService(
		server.store,
		apiNotificationPublisher{server: server},
		nil,
		eventPublisher,
		taskScheduler,
		apiDishCustomizationNormalizer{server: server},
		server.directPaymentClient,
		server.ecommerceClient,
		nil,
		nil,
		nil,
		server.ordinarySPClient,
	)
	if service == nil {
		log.Error().Msg("buildOrderCommandService: failed to initialize service")
	}
	return service
}

func (server *Server) buildOrderQueryService() logic.OrderQueryService {
	if server.orderCommandSvc != nil {
		if qs, ok := server.orderCommandSvc.(logic.OrderQueryService); ok {
			return qs
		}
	}

	var taskScheduler logic.TaskScheduler
	if server.taskDistributor != nil {
		taskScheduler = apiTaskScheduler{server: server}
	}

	service := logic.NewOrderService(
		server.store,
		apiNotificationPublisher{server: server},
		nil,
		nil,
		taskScheduler,
		apiDishCustomizationNormalizer{server: server},
		server.directPaymentClient,
		server.ecommerceClient,
		nil,
		nil,
		nil,
		server.ordinarySPClient,
	)
	if service == nil {
		log.Error().Msg("buildOrderQueryService: failed to initialize service")
	}
	return service
}

func (server *Server) buildPaymentFacade() logic.PaymentFacade {
	return logic.NewDefaultPaymentFacadeWithOrdinaryServiceProvider(server.store, server.directPaymentClient, server.ecommerceClient, server.ordinarySPClient)
}

func (server *Server) buildRefundOrchestrator() logic.RefundOrchestrator {
	var paymentFacade logic.PaymentFacade
	if server.directPaymentClient != nil || server.ecommerceClient != nil || server.ordinarySPClient != nil {
		paymentFacade = logic.NewDefaultPaymentFacadeWithOrdinaryServiceProvider(server.store, server.directPaymentClient, server.ecommerceClient, server.ordinarySPClient)
	}

	var taskScheduler logic.TaskScheduler
	if server.taskDistributor != nil {
		taskScheduler = apiTaskScheduler{server: server}
	}

	return logic.NewRefundService(
		server.store,
		paymentFacade,
		taskScheduler,
		nil,
		nil,
	)
}

func (server *Server) buildProfitSharingReceiverLifecycleService() *logic.ProfitSharingReceiverLifecycleService {
	return logic.NewProfitSharingReceiverLifecycleService(server.store, server.ecommerceClient)
}

func (server *Server) buildOperatorStatusService() *logic.OperatorStatusService {
	return logic.NewOperatorStatusService(server.store, server.ecommerceClient)
}

func (server *Server) buildRiderDepositRefundService() *logic.RiderDepositRefundService {
	return logic.NewRiderDepositRefundService(server.store, server.directPaymentClient, server.ecommerceClient)
}
