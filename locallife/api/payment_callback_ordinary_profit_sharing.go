package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/websocket"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog/log"
)

func (server *Server) handleOrdinaryServiceProviderProfitSharingNotify(ctx *gin.Context) {
	envelope, notification, ok := server.readOrdinarySuccessNotification(ctx, "ordinary_profit_sharing", ordinaryserviceprovider.NotificationTargetProfitSharing)
	if !ok {
		return
	}

	if !server.tryClaimNotification(ctx, notification, "ordinary_profit_sharing") {
		return
	}

	resource, err := ordinaryProfitSharingResourceFromEnvelope(envelope)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_profit_sharing", "decode_resource").Inc()
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("decode ordinary profit sharing notification resource")
		server.releaseNotification(ctx, notification.ID, "ordinary_profit_sharing")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "decode resource failed"})
		return
	}

	if err := server.validateOrdinaryProfitSharingOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_profit_sharing", "ownership").Inc()
		log.Error().Err(err).
			Str("sp_mch_id", resource.SpMchID).
			Str("sub_mch_id", resource.SubMchID).
			Str("out_order_no", resource.OutOrderNo).
			Msg("ordinary profit sharing notification ownership validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "普通服务商分账回调归属校验失败",
			Message:     fmt.Sprintf("普通服务商分账回调 out_order_no=%s 归属校验失败，sp_mch_id=%s, sub_mch_id=%s。系统已返回 FAIL 等待微信重试。", resource.OutOrderNo, resource.SpMchID, resource.SubMchID),
			RelatedID:   0,
			RelatedType: "profit_sharing_order",
			Extra: map[string]interface{}{
				"out_order_no": resource.OutOrderNo,
				"order_id":     resource.OrderID,
				"sp_mch_id":    resource.SpMchID,
				"sub_mch_id":   resource.SubMchID,
			},
		})
		server.releaseNotification(ctx, notification.ID, "ordinary_profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "ownership validation failed"})
		return
	}
	profitSharingOrder, err := server.store.GetProfitSharingOrderByOutOrderNo(ctx, resource.OutOrderNo)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_profit_sharing", "query_profit_sharing_order").Inc()
		if isNotFoundError(err) {
			log.Error().Str("out_order_no", resource.OutOrderNo).Msg("ordinary profit sharing order not found")
			server.markNotificationProcessed(ctx, notification.ID, "", resource.TransactionID)
			writeWechatNotifySuccess(ctx, "ordinary_profit_sharing")
			return
		}
		log.Error().Err(err).Str("out_order_no", resource.OutOrderNo).Msg("get ordinary profit sharing order")
		server.releaseNotification(ctx, notification.ID, "ordinary_profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
		return
	}

	paymentOrder, err := server.store.GetPaymentOrder(ctx, profitSharingOrder.PaymentOrderID)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_profit_sharing", "query_payment_order").Inc()
		log.Error().Err(err).Int64("payment_order_id", profitSharingOrder.PaymentOrderID).Msg("get ordinary profit sharing payment order")
		server.releaseNotification(ctx, notification.ID, "ordinary_profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
		return
	}
	if !db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_profit_sharing", "channel_mismatch").Inc()
		log.Error().
			Int64("payment_order_id", paymentOrder.ID).
			Str("payment_channel", paymentOrder.PaymentChannel).
			Str("out_order_no", resource.OutOrderNo).
			Msg("ordinary profit sharing callback matched non-ordinary payment order")
		server.releaseNotification(ctx, notification.ID, "ordinary_profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "payment channel mismatch"})
		return
	}

	if err := server.validateOrdinaryProfitSharingSubMerchantOwnership(ctx, resource, profitSharingOrder); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_profit_sharing", "sub_merchant_ownership").Inc()
		log.Error().Err(err).
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Int64("merchant_id", profitSharingOrder.MerchantID).
			Str("out_order_no", resource.OutOrderNo).
			Str("sub_mch_id", resource.SubMchID).
			Msg("ordinary profit sharing notification sub merchant ownership validation failed")
		server.releaseNotification(ctx, notification.ID, "ordinary_profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "ownership validation failed"})
		return
	}

	queryResp, queryErr := server.ordinarySPClient.QueryProfitSharingOrder(ctx, ospcontracts.ProfitSharingQueryRequest{SubMchID: resource.SubMchID, TransactionID: resource.TransactionID, OutOrderNo: resource.OutOrderNo})
	if queryErr != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_profit_sharing", "query_profit_sharing").Inc()
		log.Error().Err(queryErr).Str("out_order_no", resource.OutOrderNo).Msg("query ordinary profit sharing detail failed")
		server.releaseNotification(ctx, notification.ID, "ordinary_profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "query profit sharing failed"})
		return
	}
	if queryResp == nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_profit_sharing", "query_profit_sharing_empty").Inc()
		log.Error().Str("out_order_no", resource.OutOrderNo).Msg("query ordinary profit sharing detail returned nil response")
		server.releaseNotification(ctx, notification.ID, "ordinary_profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "query profit sharing returned empty response"})
		return
	}
	canonicalQuery := ordinaryProfitSharingQueryResponse(queryResp)
	finalResult, finalFailReason := logic.ResolveProfitSharingQueryFinalResult(canonicalQuery)

	paymentFactApplication, err := server.recordOrdinaryProfitSharingCallbackFact(ctx, notification, profitSharingOrder, resource, queryResp, finalResult, finalFailReason)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_profit_sharing", "record_fact").Inc()
		log.Error().Err(err).
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Str("out_order_no", resource.OutOrderNo).
			Str("result", finalResult).
			Msg("record ordinary profit sharing payment fact failed")
		server.releaseNotification(ctx, notification.ID, "ordinary_profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "record payment fact failed"})
		return
	}
	server.enqueueProfitSharingPaymentFactApplication(ctx, paymentFactApplication)
	writeWechatNotifySuccess(ctx, "ordinary_profit_sharing")
	server.markNotificationProcessed(ctx, notification.ID, "", resource.TransactionID)
}

func ordinaryProfitSharingResourceFromEnvelope(envelope *ordinaryserviceprovider.NotificationEnvelope) (*ospcontracts.ProfitSharingNotificationPayload, error) {
	if envelope == nil {
		return nil, errors.New("ordinary profit sharing notification envelope is nil")
	}
	data, err := ordinaryNotificationResourceBytes(envelope)
	if err != nil {
		return nil, err
	}
	resource := &ospcontracts.ProfitSharingNotificationPayload{}
	if err := json.Unmarshal(data, resource); err != nil {
		return nil, fmt.Errorf("decode ordinary profit sharing notification resource: %w", err)
	}
	if err := ospcontracts.ValidateProfitSharingNotificationPayload(*resource); err != nil {
		return nil, err
	}
	return resource, nil
}

func (server *Server) validateOrdinaryProfitSharingOwnership(resource *ospcontracts.ProfitSharingNotificationPayload) error {
	if server.ordinarySPClient == nil {
		return errors.New("ordinary service provider client not configured")
	}
	if resource == nil {
		return errors.New("ordinary profit sharing resource is nil")
	}
	if resource.SpMchID == "" {
		return errors.New("sp_mchid missing")
	}
	if resource.SpMchID != server.ordinarySPClient.ServiceProviderMchID() {
		return fmt.Errorf("mchid mismatch")
	}
	return nil
}

func (server *Server) validateOrdinaryProfitSharingSubMerchantOwnership(ctx context.Context, resource *ospcontracts.ProfitSharingNotificationPayload, order db.ProfitSharingOrder) error {
	if resource == nil {
		return errors.New("profit sharing resource is nil")
	}
	if resource.SubMchID == "" {
		return errors.New("sub_mchid missing")
	}
	if order.MerchantID == 0 {
		return errors.New("profit sharing order missing merchant_id")
	}
	paymentConfig, err := server.store.GetMerchantPaymentConfig(ctx, order.MerchantID)
	if err != nil {
		return fmt.Errorf("get merchant payment config: %w", err)
	}
	if paymentConfig.SubMchID == "" {
		return errors.New("merchant payment config sub_mchid missing")
	}
	if resource.SubMchID != paymentConfig.SubMchID {
		return fmt.Errorf("sub_mchid mismatch")
	}
	return nil
}

func ordinaryProfitSharingQueryResponse(resp *ospcontracts.ProfitSharingOrderResponse) *wechatcontracts.ProfitSharingQueryResponse {
	if resp == nil {
		return nil
	}
	return &wechatcontracts.ProfitSharingQueryResponse{
		SubMchID:      resp.SubMchID,
		TransactionID: resp.TransactionID,
		OutOrderNo:    resp.OutOrderNo,
		OrderID:       resp.OrderID,
		Status:        string(resp.State),
		Receivers:     ordinaryProfitSharingReceiverResults(resp.Receivers),
	}
}

func ordinaryProfitSharingReceiverResults(receivers []ospcontracts.ProfitSharingReceiverDetail) []wechatcontracts.ProfitSharingReceiverResult {
	result := make([]wechatcontracts.ProfitSharingReceiverResult, 0, len(receivers))
	for _, receiver := range receivers {
		result = append(result, wechatcontracts.ProfitSharingReceiverResult{
			Type:            string(receiver.Type),
			ReceiverAccount: receiver.Account,
			Amount:          receiver.Amount,
			Description:     receiver.Description,
			Result:          string(receiver.Result),
			FinishTime:      receiver.FinishTime,
			FailReason:      string(receiver.FailReason),
			DetailID:        receiver.DetailID,
		})
	}
	return result
}
