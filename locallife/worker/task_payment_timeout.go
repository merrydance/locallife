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
		if paymentOrder.BusinessType == reservationPaymentAddonBusinessType {
			if err := p.expireReservationAdjustmentAfterPaymentTimeout(ctx, paymentOrder); err != nil {
				return err
			}
		}
	case "closed":
		// 支付单已经关闭，可能是上次任务执行到一半后失败重试进来的
		// 继续向下检查业务订单是否也已取消
		log.Info().
			Str("payment_order_no", payload.PaymentOrderNo).
			Msg("payment order already closed, checking business order state")
		if paymentOrder.BusinessType == reservationPaymentAddonBusinessType {
			if err := p.expireReservationAdjustmentAfterPaymentTimeout(ctx, paymentOrder); err != nil {
				return err
			}
		}
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

func (p *RedisTaskProcessor) expireReservationAdjustmentAfterPaymentTimeout(ctx context.Context, paymentOrder db.PaymentOrder) error {
	if _, err := p.store.CloseReservationAdjustmentForPaymentTx(ctx, db.CloseReservationAdjustmentForPaymentTxParams{
		PaymentOrderID: paymentOrder.ID,
		Status:         db.ReservationAdjustmentStatusExpired,
		Reason:         "payment timeout",
	}); err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return fmt.Errorf("expire reservation adjustment after payment timeout: %w", err)
	}
	return nil
}

type paymentOrderTimeoutRemoteClose struct {
	required bool
	baofu    bool
	direct   bool
}

func (p *RedisTaskProcessor) preparePaymentOrderTimeoutClose(ctx context.Context, paymentOrder db.PaymentOrder) (paymentOrderTimeoutRemoteClose, bool, error) {
	if paymentOrder.PaymentChannel == db.PaymentChannelBaofuAggregate {
		return p.prepareBaofuPaymentOrderTimeoutClose(ctx, paymentOrder)
	}
	if paymentOrder.PaymentChannel == db.PaymentChannelDirect {
		return p.prepareDirectPaymentOrderTimeoutClose(ctx, paymentOrder)
	}
	return paymentOrderTimeoutRemoteClose{}, false, nil
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

func (p *RedisTaskProcessor) closeRemotePaymentOrderForTimeout(ctx context.Context, paymentOrder db.PaymentOrder, remoteClose paymentOrderTimeoutRemoteClose) error {
	if !remoteClose.required {
		return nil
	}
	if remoteClose.baofu {
		return p.closeBaofuPaymentOrderForTimeout(ctx, paymentOrder)
	}
	if remoteClose.direct {
		if err := p.directPaymentClient.CloseOrder(ctx, paymentOrder.OutTradeNo); err != nil && !wechatPayErrorCodeIs(err, "ORDER_CLOSED") {
			return fmt.Errorf("close direct payment order before local timeout close: %w", err)
		}
		return nil
	}
	return nil
}

func normalizePaymentTimeoutTradeState(tradeState string) string {
	return strings.ToUpper(strings.TrimSpace(tradeState))
}

func wechatPayErrorCodeIs(err error, code string) bool {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(wxErr.Code), code)
}
