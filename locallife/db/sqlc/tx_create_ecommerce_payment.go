package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// CreateEcommercePaymentTxParams 单子商户收付通合单支付事务入参
// 适用于预定押金、预定加菜等无 orders 表记录的业务。
// 注意：由于 combined_payment_sub_orders.order_id NOT NULL 约束，此事务只创建
// combined_payment_orders 和 payment_orders，不插入 sub_orders 行。
// 合单回调（handleCombinePaymentNotify）通过 payment_orders.out_trade_no 查找记录，
// 无需 sub_orders 表参与。
type CreateEcommercePaymentTxParams struct {
	UserID            int64
	MerchantID        int64
	Amount            int64
	BusinessType      string // "reservation" | "reservation_addon"
	ReservationID     int64  // 仅 reservation/reservation_addon 时非零
	CombineOutTradeNo string // 合单主单号
	OutTradeNo        string // 子单号（对应 payment_orders.out_trade_no 及微信 sub_out_trade_no）
	ExpiresAt         time.Time
	Attach            string // 附加信息（可空）
	PaymentChannel    string // empty keeps historical ecommerce behavior; active merchant callers pass ordinary_service_provider
}

// CreateEcommercePaymentTxResult 事务结果
type CreateEcommercePaymentTxResult struct {
	CombinedPaymentOrder CombinedPaymentOrder
	PaymentOrder         PaymentOrder
	SubMchID             string // 从 merchant_payment_configs 读取的子商户号，供调用 CreateCombineOrder 使用
}

// CreateEcommercePaymentTx 为单子商户业务创建收付通合单支付 DB 结构
func (store *SQLStore) CreateEcommercePaymentTx(ctx context.Context, arg CreateEcommercePaymentTxParams) (CreateEcommercePaymentTxResult, error) {
	var result CreateEcommercePaymentTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		// 1. 获取商户支付配置，取得 sub_mch_id
		paymentConfig, err := q.GetMerchantPaymentConfig(ctx, arg.MerchantID)
		if err != nil {
			return fmt.Errorf("get merchant payment config for merchant %d: %w", arg.MerchantID, err)
		}
		if paymentConfig.Status != "active" || paymentConfig.SubMchID == "" {
			return fmt.Errorf("merchant %d payment config invalid or inactive", arg.MerchantID)
		}
		result.SubMchID = paymentConfig.SubMchID

		// 2. 创建合单主记录
		result.CombinedPaymentOrder, err = q.CreateCombinedPaymentOrder(ctx, CreateCombinedPaymentOrderParams{
			UserID:            arg.UserID,
			CombineOutTradeNo: arg.CombineOutTradeNo,
			TotalAmount:       arg.Amount,
			Status:            "pending",
			ExpiresAt:         pgtype.Timestamptz{Time: arg.ExpiresAt, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create combined payment order: %w", err)
		}

		paymentChannel := arg.PaymentChannel
		if paymentChannel == "" {
			paymentChannel = PaymentChannelEcommerce
		}

		// 3. 创建 payment_orders 子单
		createParams := CreatePaymentOrderParams{
			UserID:         arg.UserID,
			PaymentType:    "miniprogram",
			PaymentChannel: paymentChannel,
			BusinessType:   arg.BusinessType,
			Amount:         arg.Amount,
			OutTradeNo:     arg.OutTradeNo,
			ExpiresAt:      pgtype.Timestamptz{Time: arg.ExpiresAt, Valid: true},
		}
		if arg.ReservationID > 0 {
			createParams.ReservationID = pgtype.Int8{Int64: arg.ReservationID, Valid: true}
		}
		if arg.Attach != "" {
			createParams.Attach = pgtype.Text{String: arg.Attach, Valid: true}
		}
		result.PaymentOrder, err = q.CreatePaymentOrder(ctx, createParams)
		if err != nil {
			return fmt.Errorf("create payment order: %w", err)
		}

		// 4. 关联 combined_payment_id
		result.PaymentOrder, err = q.SetPaymentOrderCombinedID(ctx, SetPaymentOrderCombinedIDParams{
			ID:                result.PaymentOrder.ID,
			CombinedPaymentID: pgtype.Int8{Int64: result.CombinedPaymentOrder.ID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("set combined id for payment order %d: %w", result.PaymentOrder.ID, err)
		}

		return nil
	})

	return result, err
}
