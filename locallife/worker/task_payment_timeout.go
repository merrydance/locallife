package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog/log"
)

const (
	TaskPaymentOrderTimeout = "payment_order:timeout"
)

// PayloadPaymentOrderTimeout 支付订单超时任务载荷
type PayloadPaymentOrderTimeout struct {
	PaymentOrderNo string `json:"payment_order_no"`
}

// DistributeTaskPaymentOrderTimeout 分发支付订单超时任务
func (d *RedisTaskDistributor) DistributeTaskPaymentOrderTimeout(
	ctx context.Context,
	payload *PayloadPaymentOrderTimeout,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskPaymentOrderTimeout, jsonPayload, opts...)
	info, err := d.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int("max_retry", info.MaxRetry).
		Str("payment_order_no", payload.PaymentOrderNo).
		Msg("enqueued payment order timeout task")

	return nil
}

// ProcessTaskPaymentOrderTimeout 处理支付订单超时任务
func (p *RedisTaskProcessor) ProcessTaskPaymentOrderTimeout(ctx context.Context, task *asynq.Task) error {
	var payload PayloadPaymentOrderTimeout
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	log.Info().
		Str("type", task.Type()).
		Str("payment_order_no", payload.PaymentOrderNo).
		Msg("processing payment order timeout task")

	// 获取支付订单
	paymentOrder, err := p.store.GetPaymentOrderByOutTradeNo(ctx, payload.PaymentOrderNo)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}

	// 检查是否已超时（在关闭之前先检查，避免尚未到期就被错误触发）
	if paymentOrder.ExpiresAt.Valid && time.Now().Before(paymentOrder.ExpiresAt.Time) {
		log.Info().
			Str("payment_order_no", payload.PaymentOrderNo).
			Time("expire_time", paymentOrder.ExpiresAt.Time).
			Msg("payment order not expired yet")
		return nil
	}

	// 状态机：
	// - pending  → 关闭支付单，然后取消业务订单
	// - closed   → 支付单已关闭（可能是上次失败后重试），直接检查业务订单并继续
	// - 其他状态 → 已由其他流程处理，幂等退出
	switch paymentOrder.Status {
	case "pending":
		remoteClose, stop, err := p.preparePaymentOrderTimeoutClose(ctx, paymentOrder)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
		if err := p.closeRemotePaymentOrderForTimeout(ctx, paymentOrder, remoteClose); err != nil {
			return err
		}
		if _, err := p.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID); err != nil {
			return fmt.Errorf("update payment order to closed: %w", err)
		}
	case "closed":
		// 支付单已经关闭，可能是上次任务执行到一半后失败重试进来的
		// 继续向下检查业务订单是否也已取消
		log.Info().
			Str("payment_order_no", payload.PaymentOrderNo).
			Msg("payment order already closed, checking business order state")
	default:
		log.Info().
			Str("payment_order_no", payload.PaymentOrderNo).
			Str("status", paymentOrder.Status).
			Msg("payment order in terminal state, skip timeout processing")
		return nil
	}

	// 取消关联的业务订单（无论是本次刚关闭还是上次已关闭）
	if paymentOrder.OrderID.Valid {
		order, err := p.store.GetOrderForUpdate(ctx, paymentOrder.OrderID.Int64)
		if err != nil {
			return fmt.Errorf("get order for timeout cancel: %w", err)
		}

		if order.Status == db.OrderStatusPending {
			_, err = p.store.CancelOrderTx(ctx, db.CancelOrderTxParams{
				OrderID:      order.ID,
				OldStatus:    order.Status,
				CancelReason: "支付超时未完成",
				OperatorID:   order.UserID,
				OperatorType: "system",
			})
			if err != nil {
				return fmt.Errorf("cancel order after payment timeout: %w", err)
			}
		} else {
			log.Info().
				Int64("order_id", order.ID).
				Str("status", order.Status).
				Msg("order already moved past pending, skip timeout cancel")
		}
	}

	log.Info().
		Str("payment_order_no", payload.PaymentOrderNo).
		Msg("payment order timeout processed successfully")

	return nil
}

type paymentOrderTimeoutRemoteClose struct {
	required bool
	direct   bool
	ordinary bool
	subMchID string
}

func (p *RedisTaskProcessor) preparePaymentOrderTimeoutClose(ctx context.Context, paymentOrder db.PaymentOrder) (paymentOrderTimeoutRemoteClose, bool, error) {
	if paymentOrderUsesEcommerceChannel(paymentOrder) {
		return p.prepareEcommercePaymentOrderTimeoutClose(ctx, paymentOrder)
	}
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		return p.prepareOrdinaryServiceProviderPaymentOrderTimeoutClose(ctx, paymentOrder)
	}
	if paymentOrder.PaymentChannel == db.PaymentChannelDirect {
		return p.prepareDirectPaymentOrderTimeoutClose(ctx, paymentOrder)
	}
	return paymentOrderTimeoutRemoteClose{}, false, nil
}

func (p *RedisTaskProcessor) prepareOrdinaryServiceProviderPaymentOrderTimeoutClose(ctx context.Context, paymentOrder db.PaymentOrder) (paymentOrderTimeoutRemoteClose, bool, error) {
	if p.ordinarySPClient == nil {
		return paymentOrderTimeoutRemoteClose{}, false, fmt.Errorf("ordinary service provider client not configured for payment timeout query")
	}
	subMchID, err := p.resolvePaymentOrderTimeoutSubMchID(ctx, paymentOrder)
	if err != nil {
		return paymentOrderTimeoutRemoteClose{}, false, fmt.Errorf("resolve ordinary payment timeout sub_mchid: %w", err)
	}

	queryResp, err := p.queryOrdinaryServiceProviderPaymentOrderForTimeout(ctx, paymentOrder, subMchID)
	if err != nil {
		return paymentOrderTimeoutRemoteClose{}, false, fmt.Errorf("query ordinary service provider payment order before timeout close: %w", err)
	}
	if queryResp == nil {
		return paymentOrderTimeoutRemoteClose{}, false, fmt.Errorf("query ordinary service provider payment order before timeout close returned nil response")
	}

	stop, err := p.handleOrdinaryServiceProviderPaymentTimeoutQueryResult(ctx, paymentOrder, queryResp)
	if err != nil || stop {
		return paymentOrderTimeoutRemoteClose{}, stop, err
	}
	return paymentOrderTimeoutRemoteClose{required: true, ordinary: true, subMchID: subMchID}, false, nil
}

func (p *RedisTaskProcessor) prepareEcommercePaymentOrderTimeoutClose(ctx context.Context, paymentOrder db.PaymentOrder) (paymentOrderTimeoutRemoteClose, bool, error) {
	if p.ecommerceClient == nil {
		return paymentOrderTimeoutRemoteClose{}, false, fmt.Errorf("ecommerce client not configured for payment timeout query")
	}
	subMchID, err := p.resolvePaymentOrderTimeoutSubMchID(ctx, paymentOrder)
	if err != nil {
		return paymentOrderTimeoutRemoteClose{}, false, fmt.Errorf("resolve payment timeout sub_mchid: %w", err)
	}

	queryResp, err := p.queryEcommercePaymentOrderForTimeout(ctx, paymentOrder, subMchID)
	if err != nil {
		return paymentOrderTimeoutRemoteClose{}, false, fmt.Errorf("query ecommerce payment order before timeout close: %w", err)
	}
	if queryResp == nil {
		return paymentOrderTimeoutRemoteClose{}, false, fmt.Errorf("query ecommerce payment order before timeout close returned nil response")
	}

	stop, err := p.handleEcommercePaymentTimeoutQueryResult(ctx, paymentOrder, queryResp)
	if err != nil || stop {
		return paymentOrderTimeoutRemoteClose{}, stop, err
	}
	return paymentOrderTimeoutRemoteClose{required: true, subMchID: subMchID}, false, nil
}

func (p *RedisTaskProcessor) prepareDirectPaymentOrderTimeoutClose(ctx context.Context, paymentOrder db.PaymentOrder) (paymentOrderTimeoutRemoteClose, bool, error) {
	if p.directPaymentClient == nil {
		return paymentOrderTimeoutRemoteClose{}, false, fmt.Errorf("direct payment client not configured for payment timeout query")
	}

	queryResp, err := p.directPaymentClient.QueryOrderByOutTradeNo(ctx, paymentOrder.OutTradeNo)
	if err != nil {
		return paymentOrderTimeoutRemoteClose{}, false, fmt.Errorf("query direct payment order before timeout close: %w", err)
	}
	if queryResp == nil {
		return paymentOrderTimeoutRemoteClose{}, false, fmt.Errorf("query direct payment order before timeout close returned nil response")
	}

	stop, err := p.handleDirectPaymentTimeoutQueryResult(ctx, paymentOrder, queryResp)
	if err != nil || stop {
		return paymentOrderTimeoutRemoteClose{}, stop, err
	}
	return paymentOrderTimeoutRemoteClose{required: true, direct: true}, false, nil
}

func (p *RedisTaskProcessor) queryEcommercePaymentOrderForTimeout(ctx context.Context, paymentOrder db.PaymentOrder, subMchID string) (*wechatcontracts.PartnerOrderQueryResponse, error) {
	if paymentOrder.TransactionID.Valid && strings.TrimSpace(paymentOrder.TransactionID.String) != "" {
		return p.ecommerceClient.QueryPartnerOrderByTransactionID(ctx, paymentOrder.TransactionID.String, subMchID)
	}
	return p.ecommerceClient.QueryPartnerOrderByOutTradeNo(ctx, paymentOrder.OutTradeNo, subMchID)
}

func (p *RedisTaskProcessor) queryOrdinaryServiceProviderPaymentOrderForTimeout(ctx context.Context, paymentOrder db.PaymentOrder, subMchID string) (*ospcontracts.PaymentQueryResponse, error) {
	req := ospcontracts.PaymentQueryRequest{SubMchID: subMchID, OutTradeNo: paymentOrder.OutTradeNo}
	if paymentOrder.TransactionID.Valid && strings.TrimSpace(paymentOrder.TransactionID.String) != "" {
		req.TransactionID = paymentOrder.TransactionID.String
		req.OutTradeNo = ""
	}
	return p.ordinarySPClient.QueryPayment(ctx, req)
}

func (p *RedisTaskProcessor) handleEcommercePaymentTimeoutQueryResult(ctx context.Context, paymentOrder db.PaymentOrder, queryResp *wechatcontracts.PartnerOrderQueryResponse) (bool, error) {
	tradeState := normalizePaymentTimeoutTradeState(queryResp.TradeState)
	switch tradeState {
	case "SUCCESS":
		if queryResp.Amount.Total != paymentOrder.Amount {
			p.publishPaymentTimeoutRemoteAmountMismatchAlert(ctx, paymentOrder, queryResp.Amount.Total, tradeState)
			return true, nil
		}
		updatedPaymentOrder, err := p.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
			ID:            paymentOrder.ID,
			TransactionID: pgtype.Text{String: queryResp.TransactionID, Valid: queryResp.TransactionID != ""},
		})
		if err != nil {
			return false, fmt.Errorf("mark ecommerce payment order %d paid from timeout query: %w", paymentOrder.ID, err)
		}
		paymentOrder = updatedPaymentOrder
		application, err := recordEcommercePaymentTimeoutQueryFact(ctx, p.store, paymentOrder, queryResp)
		if err != nil {
			return false, fmt.Errorf("record ecommerce payment timeout query fact: %w", err)
		}
		if err := enqueueOrderPaymentFactApplication(ctx, p.distributor, application); err != nil {
			return false, fmt.Errorf("enqueue ecommerce payment timeout query fact application: %w", err)
		}
		log.Info().
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("transaction_id", queryResp.TransactionID).
			Msg("payment timeout query found remote paid ecommerce order; local close skipped")
		return true, nil
	case "NOTPAY", "CLOSED", "PAYERROR", "REVOKED":
		return false, nil
	case "USERPAYING":
		return false, fmt.Errorf("payment order %s remote state USERPAYING blocks timeout close", paymentOrder.OutTradeNo)
	default:
		p.publishPaymentTimeoutUnexpectedRemoteStateAlert(ctx, paymentOrder, tradeState)
		return true, nil
	}
}

func (p *RedisTaskProcessor) handleDirectPaymentTimeoutQueryResult(ctx context.Context, paymentOrder db.PaymentOrder, queryResp *wechatcontracts.DirectOrderQueryResponse) (bool, error) {
	tradeState := normalizePaymentTimeoutTradeState(queryResp.TradeState)
	switch tradeState {
	case "SUCCESS":
		if queryResp.Amount.Total != paymentOrder.Amount {
			p.publishPaymentTimeoutRemoteAmountMismatchAlert(ctx, paymentOrder, queryResp.Amount.Total, tradeState)
			return true, nil
		}
		updatedPaymentOrder, err := p.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
			ID:            paymentOrder.ID,
			TransactionID: pgtype.Text{String: queryResp.TransactionID, Valid: queryResp.TransactionID != ""},
		})
		if err != nil {
			return false, fmt.Errorf("mark direct payment order %d paid from timeout query: %w", paymentOrder.ID, err)
		}
		paymentOrder = updatedPaymentOrder
		application, err := recordDirectPaymentTimeoutQueryFact(ctx, p.store, paymentOrder, queryResp)
		if err != nil {
			return false, fmt.Errorf("record direct payment timeout query fact: %w", err)
		}
		if err := enqueueOrderPaymentFactApplication(ctx, p.distributor, application); err != nil {
			return false, fmt.Errorf("enqueue direct payment timeout query fact application: %w", err)
		}
		log.Info().
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("transaction_id", queryResp.TransactionID).
			Msg("payment timeout query found remote paid direct order; local close skipped")
		return true, nil
	case "NOTPAY", "CLOSED", "PAYERROR", "REVOKED":
		return false, nil
	case "USERPAYING":
		return false, fmt.Errorf("payment order %s remote state USERPAYING blocks timeout close", paymentOrder.OutTradeNo)
	default:
		p.publishPaymentTimeoutUnexpectedRemoteStateAlert(ctx, paymentOrder, tradeState)
		return true, nil
	}
}

func (p *RedisTaskProcessor) handleOrdinaryServiceProviderPaymentTimeoutQueryResult(ctx context.Context, paymentOrder db.PaymentOrder, queryResp *ospcontracts.PaymentQueryResponse) (bool, error) {
	tradeState := normalizePaymentTimeoutTradeState(string(queryResp.TradeState))
	switch tradeState {
	case "SUCCESS":
		if queryResp.Amount == nil {
			p.publishPaymentTimeoutUnexpectedRemoteStateAlert(ctx, paymentOrder, "SUCCESS_WITHOUT_AMOUNT")
			return true, nil
		}
		if queryResp.Amount.Total != paymentOrder.Amount {
			p.publishPaymentTimeoutRemoteAmountMismatchAlert(ctx, paymentOrder, queryResp.Amount.Total, tradeState)
			return true, nil
		}
		updatedPaymentOrder, err := p.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
			ID:            paymentOrder.ID,
			TransactionID: pgtype.Text{String: queryResp.TransactionID, Valid: queryResp.TransactionID != ""},
		})
		if err != nil {
			return false, fmt.Errorf("mark ordinary service provider payment order %d paid from timeout query: %w", paymentOrder.ID, err)
		}
		paymentOrder = updatedPaymentOrder
		var application *db.ExternalPaymentFactApplication
		var factErr error
		if shouldRecordReservationPaymentFactForOrder(paymentOrder) {
			application, factErr = recordOrdinaryServiceProviderReservationPaymentTimeoutQueryFact(ctx, p.store, paymentOrder, queryResp)
		} else {
			application, factErr = recordOrdinaryServiceProviderPaymentTimeoutQueryFact(ctx, p.store, paymentOrder, queryResp)
		}
		if factErr != nil {
			return false, fmt.Errorf("record ordinary service provider payment timeout query fact: %w", factErr)
		}
		if err := enqueueOrderPaymentFactApplication(ctx, p.distributor, application); err != nil {
			return false, fmt.Errorf("enqueue ordinary service provider payment timeout query fact application: %w", err)
		}
		log.Info().
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("transaction_id", queryResp.TransactionID).
			Msg("payment timeout query found remote paid ordinary service provider order; local close skipped")
		return true, nil
	case "NOTPAY", "CLOSED", "PAYERROR", "REVOKED":
		return false, nil
	case "USERPAYING":
		return false, fmt.Errorf("payment order %s remote state USERPAYING blocks timeout close", paymentOrder.OutTradeNo)
	default:
		p.publishPaymentTimeoutUnexpectedRemoteStateAlert(ctx, paymentOrder, tradeState)
		return true, nil
	}
}

func (p *RedisTaskProcessor) closeRemotePaymentOrderForTimeout(ctx context.Context, paymentOrder db.PaymentOrder, remoteClose paymentOrderTimeoutRemoteClose) error {
	if !remoteClose.required {
		return nil
	}
	if remoteClose.direct {
		if err := p.directPaymentClient.CloseOrder(ctx, paymentOrder.OutTradeNo); err != nil && !wechatPayErrorCodeIs(err, "ORDER_CLOSED") {
			return fmt.Errorf("close direct payment order before local timeout close: %w", err)
		}
		return nil
	}
	if remoteClose.ordinary {
		if err := p.ordinarySPClient.ClosePayment(ctx, ospcontracts.PaymentCloseRequest{
			SpMchID:    p.ordinarySPClient.ServiceProviderMchID(),
			SubMchID:   remoteClose.subMchID,
			OutTradeNo: paymentOrder.OutTradeNo,
		}); err != nil && !ordinaryProviderErrorCodeIs(err, "ORDER_CLOSED") {
			return fmt.Errorf("close ordinary service provider payment order before local timeout close: %w", err)
		}
		return nil
	}
	if err := p.ecommerceClient.ClosePartnerOrder(ctx, paymentOrder.OutTradeNo, remoteClose.subMchID); err != nil && !wechatPayErrorCodeIs(err, "ORDER_CLOSED") {
		return fmt.Errorf("close ecommerce payment order before local timeout close: %w", err)
	}
	return nil
}

func (p *RedisTaskProcessor) resolvePaymentOrderTimeoutSubMchID(ctx context.Context, paymentOrder db.PaymentOrder) (string, error) {
	if subMchID := paymentTimeoutSubMchIDFromAttach(paymentOrder.Attach.String, paymentOrder.Attach.Valid); subMchID != "" {
		return subMchID, nil
	}

	var merchantID int64
	if paymentOrder.OrderID.Valid {
		order, err := p.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
		if err != nil {
			return "", fmt.Errorf("get order for payment order %d: %w", paymentOrder.ID, err)
		}
		merchantID = order.MerchantID
	} else if paymentOrder.ReservationID.Valid {
		reservation, err := p.store.GetTableReservation(ctx, paymentOrder.ReservationID.Int64)
		if err != nil {
			return "", fmt.Errorf("get reservation for payment order %d: %w", paymentOrder.ID, err)
		}
		merchantID = reservation.MerchantID
	} else {
		return "", fmt.Errorf("payment order %d missing order and reservation reference", paymentOrder.ID)
	}

	config, err := p.store.GetMerchantPaymentConfig(ctx, merchantID)
	if err != nil {
		return "", fmt.Errorf("get merchant payment config for payment order %d: %w", paymentOrder.ID, err)
	}
	if strings.TrimSpace(config.SubMchID) == "" {
		return "", fmt.Errorf("merchant payment config missing sub_mchid for payment order %d", paymentOrder.ID)
	}
	return config.SubMchID, nil
}

func recordEcommercePaymentTimeoutQueryFact(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, queryResp *wechatcontracts.PartnerOrderQueryResponse) (*db.ExternalPaymentFactApplication, error) {
	if paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerOrder {
		return recordOrderPaymentQueryFact(ctx, store, paymentOrder, queryResp)
	}
	if shouldRecordReservationPaymentFactForOrder(paymentOrder) {
		return recordReservationPaymentQueryFact(ctx, store, paymentOrder, queryResp)
	}
	return nil, fmt.Errorf("unsupported ecommerce payment timeout fact owner %q for payment order %d", paymentOrder.BusinessType, paymentOrder.ID)
}

func normalizePaymentTimeoutTradeState(tradeState string) string {
	return strings.ToUpper(strings.TrimSpace(tradeState))
}

func paymentTimeoutSubMchIDFromAttach(attach string, valid bool) string {
	if !valid {
		return ""
	}
	for _, segment := range strings.Split(strings.TrimSpace(attach), ";") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		key, value, ok := strings.Cut(segment, ":")
		if !ok || strings.TrimSpace(key) != "sub_mchid" {
			continue
		}
		return strings.TrimSpace(value)
	}
	return ""
}

func wechatPayErrorCodeIs(err error, code string) bool {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(wxErr.Code), code)
}

func ordinaryProviderErrorCodeIs(err error, code string) bool {
	var providerErr *ordinaryserviceprovider.ProviderError
	if !errors.As(err, &providerErr) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(providerErr.ProviderCode), code)
}

func (p *RedisTaskProcessor) publishPaymentTimeoutRemoteAmountMismatchAlert(ctx context.Context, paymentOrder db.PaymentOrder, remoteAmount int64, remoteState string) {
	p.publishAlert(ctx, AlertData{
		AlertType:   AlertTypePaymentTimeout,
		Level:       AlertLevelCritical,
		Title:       "支付超时扫描发现远端支付金额不一致",
		Message:     fmt.Sprintf("支付单 %s 超时扫描发现微信侧状态为 %s，但远端金额 %d 与本地金额 %d 不一致，系统已停止自动关单。", paymentOrder.OutTradeNo, remoteState, remoteAmount, paymentOrder.Amount),
		RelatedID:   paymentOrder.ID,
		RelatedType: "payment_order",
		Extra: map[string]interface{}{
			"out_trade_no":    paymentOrder.OutTradeNo,
			"remote_state":    remoteState,
			"expected_amount": paymentOrder.Amount,
			"actual_amount":   remoteAmount,
		},
	})
}

func (p *RedisTaskProcessor) publishPaymentTimeoutUnexpectedRemoteStateAlert(ctx context.Context, paymentOrder db.PaymentOrder, remoteState string) {
	p.publishAlert(ctx, AlertData{
		AlertType:   AlertTypePaymentTimeout,
		Level:       AlertLevelCritical,
		Title:       "支付超时扫描遇到异常远端状态",
		Message:     fmt.Sprintf("支付单 %s 超时扫描发现微信侧状态为 %s，系统已停止自动关单，请人工核对。", paymentOrder.OutTradeNo, remoteState),
		RelatedID:   paymentOrder.ID,
		RelatedType: "payment_order",
		Extra: map[string]interface{}{
			"out_trade_no": paymentOrder.OutTradeNo,
			"remote_state": remoteState,
		},
	})
}
