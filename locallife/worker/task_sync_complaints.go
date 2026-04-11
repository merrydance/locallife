package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

const (
	// TaskSyncComplaints 从微信批量拉取并同步投诉单
	TaskSyncComplaints = "payment:sync_complaints"
)

// SyncComplaintsPayload 同步投诉任务载荷
type SyncComplaintsPayload struct {
	// BeginDate 查询开始日期（格式 "2006-01-02"）
	BeginDate string `json:"begin_date"`
	// EndDate 查询结束日期（格式 "2006-01-02"）
	EndDate string `json:"end_date"`
	// SubMchID 限定二级商户号（空表示拉取所有）
	SubMchID string `json:"sub_mch_id,omitempty"`
}

// DistributeTaskSyncComplaints 分发投诉同步任务
func (distributor *RedisTaskDistributor) DistributeTaskSyncComplaints(
	ctx context.Context,
	payload *SyncComplaintsPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskSyncComplaints, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Debug().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Str("begin_date", payload.BeginDate).
		Str("end_date", payload.EndDate).
		Str("sub_mch_id", payload.SubMchID).
		Msg("enqueued sync complaints task")

	return nil
}

// ProcessTaskSyncComplaints 批量拉取微信投诉单并写入本地数据库（幂等）
// 每次翻页直到拉完所有记录；利用 UpsertWechatComplaint 保证幂等
func (processor *RedisTaskProcessor) ProcessTaskSyncComplaints(ctx context.Context, task *asynq.Task) error {
	var payload SyncComplaintsPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	if payload.BeginDate == "" {
		payload.BeginDate = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	if payload.EndDate == "" {
		payload.EndDate = time.Now().Format("2006-01-02")
	}

	log.Info().
		Str("begin_date", payload.BeginDate).
		Str("end_date", payload.EndDate).
		Str("sub_mch_id", payload.SubMchID).
		Msg("syncing complaints from WeChat")

	const pageSize = 50
	offset := 0
	totalSynced := 0

	for {
		page, err := processor.ecommerceClient.ListComplaints(ctx, wechat.ListComplaintsRequest{
			BeginDate: payload.BeginDate,
			EndDate:   payload.EndDate,
			SubMchID:  payload.SubMchID,
			Limit:     pageSize,
			Offset:    offset,
		})
		if err != nil {
			return fmt.Errorf("list complaints (offset=%d): %w", offset, err)
		}

		for _, c := range page.Data {
			if err := processor.upsertComplaint(ctx, c); err != nil {
				// 单条失败记录日志但继续处理其余条目
				log.Error().
					Err(err).
					Str("complaint_id", c.ComplaintID).
					Msg("failed to upsert complaint, skipping")
			} else {
				totalSynced++
			}
		}

		offset += len(page.Data)
		if offset >= page.TotalCount || len(page.Data) == 0 {
			break
		}
	}

	log.Info().
		Int("total_synced", totalSynced).
		Str("begin_date", payload.BeginDate).
		Str("end_date", payload.EndDate).
		Msg("complaints sync completed")

	return nil
}

// upsertComplaint 将单条微信投诉详情写入数据库（幂等）
// 通过 sub_mch_id（收付通场景）查找 merchant_id；
// 若无 sub_mch_id，则通过关联订单的 out_trade_no 回溯。
func (processor *RedisTaskProcessor) upsertComplaint(ctx context.Context, c wechat.ComplaintDetail) error {
	var outTradeNo pgtype.Text
	var transactionID pgtype.Text
	var merchantID pgtype.Int8
	var subMchID pgtype.Text

	// 从关联订单信息中提取 out_trade_no 和 transaction_id
	if len(c.ComplaintOrderInfo) > 0 {
		first := c.ComplaintOrderInfo[0]
		if first.OutTradeNo != "" {
			outTradeNo = pgtype.Text{String: first.OutTradeNo, Valid: true}
		}
		if first.TransactionID != "" {
			transactionID = pgtype.Text{String: first.TransactionID, Valid: true}
		}
	}
	// 顶层 transaction_id 优先（合单支付情形）
	if c.TransactionID != "" {
		transactionID = pgtype.Text{String: c.TransactionID, Valid: true}
	}

	// 优先：通过 sub_mch_id 查找商户支付配置，得到 merchant_id
	if c.SubMchID != "" {
		subMchID = pgtype.Text{String: c.SubMchID, Valid: true}
		payConfig, err := processor.store.GetMerchantPaymentConfigBySubMchID(ctx, c.SubMchID)
		if err == nil {
			merchantID = pgtype.Int8{Int64: payConfig.MerchantID, Valid: true}
		} else if !errors.Is(err, db.ErrRecordNotFound) {
			log.Warn().Err(err).Str("sub_mch_id", c.SubMchID).Msg("complaint sync: failed to look up merchant by sub_mch_id")
		}
	}

	// 回退：通过 out_trade_no 查找支付订单，再通过 order_id 找到商户
	if !merchantID.Valid && outTradeNo.Valid {
		order, err := processor.store.GetPaymentOrderByOutTradeNo(ctx, outTradeNo.String)
		if err == nil && order.OrderID.Valid {
			dbOrder, oErr := processor.store.GetOrder(ctx, order.OrderID.Int64)
			if oErr == nil {
				merchantID = pgtype.Int8{Int64: dbOrder.MerchantID, Valid: true}
			}
		} else if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			log.Warn().Err(err).Str("out_trade_no", outTradeNo.String).Msg("complaint sync: failed to look up payment order")
		}
	}

	var payerOpenID pgtype.Text
	if c.PayerOpenID != "" {
		payerOpenID = pgtype.Text{String: c.PayerOpenID, Valid: true}
	}

	var wxpayUpdateTime pgtype.Timestamptz
	if !c.UpdateTime.IsZero() {
		wxpayUpdateTime = pgtype.Timestamptz{Time: c.UpdateTime, Valid: true}
	}

	_, err := processor.store.UpsertWechatComplaint(ctx, db.UpsertWechatComplaintParams{
		ComplaintID:            c.ComplaintID,
		ComplaintTime:          c.ComplaintTime,
		PayerOpenid:            payerOpenID,
		ComplaintDetail:        c.ComplaintDetail,
		ComplaintState:         string(c.ComplaintState),
		TransactionID:          transactionID,
		OutTradeNo:             outTradeNo,
		SubMchID:               subMchID,
		MerchantID:             merchantID,
		PayerComplaintFullInfo: c.PayerComplaintFullInfo,
		Amount:                 c.Amount,
		WxpayUpdateTime:        wxpayUpdateTime,
	})
	return err
}
