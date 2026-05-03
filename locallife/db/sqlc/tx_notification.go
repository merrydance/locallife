package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// TryClaimWechatNotification 原子性地尝试认领（占位）一个微信通知。
//
// 使用 INSERT ... ON CONFLICT (id) DO NOTHING 实现无锁竞争去重：
// - 返回 true：成功占位，调用方可以安全地执行后续业务逻辑
// - 返回 false：已被认领（重复通知），调用方应立即返回 SUCCESS 给微信
//
// 关键安全性保证：
//   - 并发的两个相同 notification_id 请求中，只有一个会得到 true
//   - 如果业务逻辑失败，调用方必须调用 ReleaseWechatNotificationClaim 释放占位，
//     以允许微信的下一次重试正常处理
func (store *SQLStore) TryClaimWechatNotification(ctx context.Context, arg CreateWechatNotificationParams) (bool, error) {
	result, err := store.connPool.Exec(ctx,
		`INSERT INTO wechat_notifications (id, event_type, resource_type, summary, out_trade_no, transaction_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO NOTHING`,
		arg.ID, arg.EventType, arg.ResourceType, arg.Summary, arg.OutTradeNo, arg.TransactionID,
	)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}

// ReleaseWechatNotificationClaim 删除一个已认领但未成功处理的通知占位记录。
//
// 仅在业务逻辑执行失败时调用，以允许微信下次重试进入处理流程。
// 如果删除本身失败，打印错误日志即可——下次重试将因重复而直接返回 SUCCESS，
// 此时 PaymentRecoveryScheduler 会兜底扫描并补偿。
func (store *SQLStore) ReleaseWechatNotificationClaim(ctx context.Context, id string) error {
	_, err := store.connPool.Exec(ctx, `DELETE FROM wechat_notifications WHERE id = $1`, id)
	return err
}

// MarkWechatNotificationProcessed 标记通知处理完成。
//
// 该标记用于区分“已处理完成的重复通知”（可直接返回 SUCCESS）与
// “仅占位成功但业务未完成的通知”（应继续返回 FAIL 触发微信重试）。
func (store *SQLStore) MarkWechatNotificationProcessed(ctx context.Context, id, outTradeNo, transactionID string) error {
	_, err := store.connPool.Exec(ctx,
		`UPDATE wechat_notifications
		 SET processed_at = NOW(),
		     out_trade_no = COALESCE(NULLIF($2, ''), out_trade_no),
		     transaction_id = COALESCE(NULLIF($3, ''), transaction_id)
		 WHERE id = $1`,
		id, outTradeNo, transactionID,
	)
	return err
}

// ===== 进件状态更新事务 (CB-4) =====

// ApplymentSubMchActivationTxParams 进件开通二级商户号事务参数
type ApplymentSubMchActivationTxParams struct {
	ApplymentID           int64
	WechatApplymentID     pgtype.Int8
	SubjectType           string
	SubjectID             int64
	SubMchID              string
	AccountAuthorizeState string
}

// ApplymentSubMchActivationTx 在单个事务中同步普通服务商进件完成和二级商户号。
//
// 普通服务商特约商户进件以 APPLYMENT_STATE_FINISHED + sub_mchid 作为开户完成点；
// 渠道商开户意愿授权状态不得作为本项目普通服务商交易能力激活门槛。
func (store *SQLStore) ApplymentSubMchActivationTx(ctx context.Context, arg ApplymentSubMchActivationTxParams) error {
	return store.execTx(ctx, func(q *Queries) error {
		applyment, err := q.GetEcommerceApplyment(ctx, arg.ApplymentID)
		if err != nil {
			return fmt.Errorf("get applyment: %w", err)
		}

		resolvedWechatApplymentID := applyment.ApplymentID
		if !resolvedWechatApplymentID.Valid && arg.WechatApplymentID.Valid {
			resolvedWechatApplymentID = arg.WechatApplymentID
		}
		accountAuthorizeState := strings.TrimSpace(arg.AccountAuthorizeState)
		if accountAuthorizeState == "" && applyment.AccountAuthorizeState.Valid {
			accountAuthorizeState = strings.TrimSpace(applyment.AccountAuthorizeState.String)
		}

		// step 1: 在激活事务内同步进件完成状态与 sub_mch_id，避免 finish owner path 漏写终态。
		if _, err := q.UpdateEcommerceApplymentStatus(ctx, UpdateEcommerceApplymentStatusParams{
			ID:                             arg.ApplymentID,
			ApplymentID:                    resolvedWechatApplymentID,
			Status:                         "finish",
			RejectReason:                   applyment.RejectReason,
			SignUrl:                        applyment.SignUrl,
			SignState:                      applyment.SignState,
			LegalValidationUrl:             applyment.LegalValidationUrl,
			AccountValidation:              applyment.AccountValidation,
			SubMchID:                       pgtype.Text{String: arg.SubMchID, Valid: true},
			AccountAuthorizeState:          pgtype.Text{String: accountAuthorizeState, Valid: accountAuthorizeState != ""},
			AccountAuthorizeStateCheckedAt: pgtype.Timestamptz{Time: time.Now(), Valid: accountAuthorizeState != ""},
		}); err != nil {
			return fmt.Errorf("update applyment status to finish: %w", err)
		}

		if arg.SubjectType != "merchant" {
			return nil
		}

		// step 2: 更新商户支付配置。普通服务商完成态不再等待渠道商开户意愿授权。
		if _, err := q.UpdateMerchantPaymentConfig(ctx, UpdateMerchantPaymentConfigParams{
			MerchantID: arg.SubjectID,
			SubMchID:   pgtype.Text{String: arg.SubMchID, Valid: true},
			Status:     pgtype.Text{String: MerchantPaymentConfigStatusActive, Valid: true},
		}); err != nil {
			return fmt.Errorf("update merchant payment config: %w", err)
		}

		// step 3: 更新商户状态为 active
		if _, err := q.UpdateMerchantStatus(ctx, UpdateMerchantStatusParams{
			ID:     arg.SubjectID,
			Status: MerchantStatusActive,
		}); err != nil {
			return fmt.Errorf("update merchant status: %w", err)
		}

		return nil
	})
}
