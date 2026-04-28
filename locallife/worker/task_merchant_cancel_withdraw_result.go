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
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

const (
	TaskProcessMerchantCancelWithdrawResult = "payment:process_merchant_cancel_withdraw_result"
	merchantCancelWithdrawMaxRetry          = 5
	merchantCancelWithdrawFactBusinessType  = "merchant_cancel_withdraw_application"
)

type MerchantCancelWithdrawResultPayload struct {
	ApplicationID int64 `json:"application_id"`
	RetryCount    int   `json:"retry_count"`
}

func (distributor *RedisTaskDistributor) DistributeTaskProcessMerchantCancelWithdrawResult(
	ctx context.Context,
	payload *MerchantCancelWithdrawResultPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessMerchantCancelWithdrawResult, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("application_id", payload.ApplicationID).
		Int("retry_count", payload.RetryCount).
		Msg("enqueued merchant cancel withdraw result task")

	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskMerchantCancelWithdrawResult(ctx context.Context, task *asynq.Task) error {
	if processor.ecommerceClient == nil {
		return fmt.Errorf("ecommerce client not configured: %w", asynq.SkipRetry)
	}

	var payload MerchantCancelWithdrawResultPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	record, err := processor.store.GetMerchantCancelWithdrawApplication(ctx, payload.ApplicationID)
	if err != nil {
		if err == db.ErrRecordNotFound {
			return fmt.Errorf("merchant cancel withdraw application not found: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("get merchant cancel withdraw application: %w", err)
	}

	if logic.MerchantCancelWithdrawIsTerminal(record.CancelState.String) {
		return nil
	}

	queryResp, err := processor.queryMerchantCancelWithdrawResult(ctx, record)
	if err != nil {
		logMerchantCancelWithdrawResultQueryFailure(record, payload.RetryCount, err)
		if _, updateErr := processor.store.UpdateMerchantCancelWithdrawApplicationSync(
			ctx,
			mustBuildMerchantCancelWithdrawSyncParams(record, nil, record.LocalSyncState, logic.MerchantCancelWithdrawSafeErrorMessage(err)),
		); updateErr != nil {
			return fmt.Errorf("update merchant cancel withdraw sync error after query failure: %w", updateErr)
		}

		if payload.RetryCount >= merchantCancelWithdrawMaxRetry {
			processor.publishAlert(ctx, AlertData{
				AlertType:   AlertTypeSystemError,
				Level:       AlertLevelCritical,
				Title:       "商户注销提现结果查询失败",
				Message:     fmt.Sprintf("注销提现申请 %d 连续查询失败，当前仍需人工核对微信侧状态。", record.ID),
				RelatedID:   record.ID,
				RelatedType: "merchant_cancel_withdraw_application",
				Extra: map[string]interface{}{
					"out_request_no": record.OutRequestNo,
					"applyment_id":   record.ApplymentID.String,
					"retry_count":    payload.RetryCount,
					"error":          err.Error(),
				},
			})
			return fmt.Errorf("query merchant cancel withdraw result after retries: %w", asynq.SkipRetry)
		}

		processor.requeueMerchantCancelWithdrawResultTask(ctx, record.ID, payload.RetryCount+1)
		return nil
	}

	application, err := recordMerchantCancelWithdrawQueryFact(ctx, processor.store, record, queryResp)
	if err != nil {
		return fmt.Errorf("record merchant cancel withdraw query fact: %w", err)
	}

	updated, applied, err := applyMerchantCancelWithdrawFactApplication(ctx, processor.store, application)
	if err != nil {
		return fmt.Errorf("apply merchant cancel withdraw fact application: %w", err)
	}
	if !applied {
		updated = record
	}

	if logic.MerchantCancelWithdrawIsTerminal(updated.CancelState.String) {
		if strings.EqualFold(updated.CancelState.String, db.MerchantCancelStateRejected) || strings.EqualFold(updated.CancelState.String, db.MerchantCancelStateRevoked) {
			processor.publishAlert(ctx, AlertData{
				AlertType:   AlertTypeWithdrawFailed,
				Level:       AlertLevelWarning,
				Title:       "商户注销提现申请未完成",
				Message:     fmt.Sprintf("注销提现申请 %d 已进入 %s，请核对微信侧返回原因。", updated.ID, updated.CancelState.String),
				RelatedID:   updated.ID,
				RelatedType: "merchant_cancel_withdraw_application",
				Extra: map[string]interface{}{
					"out_request_no":             updated.OutRequestNo,
					"applyment_id":               updated.ApplymentID.String,
					"cancel_state":               updated.CancelState.String,
					"cancel_state_description":   updated.CancelStateDescription.String,
					"withdraw_state":             updated.WithdrawState.String,
					"withdraw_state_description": updated.WithdrawStateDescription.String,
				},
			})
		}
		return nil
	}

	if payload.RetryCount < merchantCancelWithdrawMaxRetry {
		processor.requeueMerchantCancelWithdrawResultTask(ctx, updated.ID, payload.RetryCount+1)
	}

	return nil
}

func (processor *RedisTaskProcessor) queryMerchantCancelWithdrawResult(ctx context.Context, record db.MerchantCancelWithdrawApplication) (*wechatcontracts.CancelWithdrawQueryResponse, error) {
	if record.ApplymentID.Valid && strings.TrimSpace(record.ApplymentID.String) != "" {
		resp, err := processor.ecommerceClient.QueryEcommerceCancelWithdrawByApplymentID(ctx, record.ApplymentID.String)
		if err == nil {
			return resp, nil
		}
		logMerchantCancelWithdrawResultQueryFailure(record, 0, err)
		if !isWechatMerchantCancelWithdrawRequestNotFound(err) {
			return nil, err
		}
	}
	return processor.ecommerceClient.QueryEcommerceCancelWithdrawByOutRequestNo(ctx, record.OutRequestNo)
}

func (processor *RedisTaskProcessor) requeueMerchantCancelWithdrawResultTask(ctx context.Context, applicationID int64, retryCount int) {
	if processor.distributor == nil {
		return
	}
	_ = processor.distributor.DistributeTaskProcessMerchantCancelWithdrawResult(
		ctx,
		&MerchantCancelWithdrawResultPayload{ApplicationID: applicationID, RetryCount: retryCount},
		asynq.ProcessIn(processor.config.ProfitSharingReturnRetryInterval),
		asynq.Queue(QueueDefault),
	)
}

func isWechatMerchantCancelWithdrawRequestNotFound(err error) bool {
	var payErr *wechat.WechatPayError
	if !errors.As(err, &payErr) {
		return false
	}
	code := strings.ToUpper(payErr.Code)
	return payErr.StatusCode == 404 || strings.Contains(code, "NOT_FOUND") || strings.Contains(code, "RESOURCE_NOT_EXISTS")
}

func mustBuildMerchantCancelWithdrawSyncParams(
	record db.MerchantCancelWithdrawApplication,
	queryResp *wechatcontracts.CancelWithdrawQueryResponse,
	localSyncState string,
	lastError string,
) db.UpdateMerchantCancelWithdrawApplicationSyncParams {
	params, err := logic.BuildMerchantCancelWithdrawSyncParams(record, queryResp, localSyncState, lastError, false, time.Now())
	if err != nil {
		return db.UpdateMerchantCancelWithdrawApplicationSyncParams{
			ApplymentID:              record.ApplymentID,
			LocalSyncState:           localSyncState,
			CancelState:              record.CancelState,
			CancelStateDescription:   record.CancelStateDescription,
			WithdrawState:            record.WithdrawState,
			WithdrawStateDescription: record.WithdrawStateDescription,
			ConfirmCancelUrl:         record.ConfirmCancelUrl,
			AccountInfo:              record.AccountInfo,
			AccountWithdrawResult:    record.AccountWithdrawResult,
			LatestQueryResponse:      record.LatestQueryResponse,
			ClearLastError:           strings.TrimSpace(lastError) == "",
			LastError:                pgtype.Text{String: strings.TrimSpace(lastError), Valid: strings.TrimSpace(lastError) != ""},
			ModifyTime:               record.ModifyTime,
			MarkSubmitted:            false,
			LastQueryAt:              pgtype.Timestamptz{Time: time.Now(), Valid: true},
			ID:                       record.ID,
		}
	}
	return params
}

func logMerchantCancelWithdrawResultQueryFailure(record db.MerchantCancelWithdrawApplication, retryCount int, err error) {
	evt := log.Error().
		Int64("application_id", record.ID).
		Int64("merchant_id", record.MerchantID).
		Str("sub_mchid", strings.TrimSpace(record.SubMchID)).
		Str("out_request_no", strings.TrimSpace(record.OutRequestNo)).
		Str("applyment_id", strings.TrimSpace(record.ApplymentID.String)).
		Int("retry_count", retryCount)

	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) && wxErr != nil {
		if wxErr.StatusCode < 500 && strings.TrimSpace(wxErr.Code) != "SIGN_ERROR" {
			evt = log.Warn().
				Int64("application_id", record.ID).
				Int64("merchant_id", record.MerchantID).
				Str("sub_mchid", strings.TrimSpace(record.SubMchID)).
				Str("out_request_no", strings.TrimSpace(record.OutRequestNo)).
				Str("applyment_id", strings.TrimSpace(record.ApplymentID.String)).
				Int("retry_count", retryCount)
		}
		evt = evt.
			Int("wechat_status_code", wxErr.StatusCode).
			Str("wechat_error_code", strings.TrimSpace(wxErr.Code)).
			Str("wechat_error_message", strings.TrimSpace(wxErr.Message)).
			Str("wechat_error_detail", strings.TrimSpace(wxErr.Detail))
	}

	evt.Err(err).Msg("merchant cancel withdraw result query failed")
}

func recordMerchantCancelWithdrawQueryFact(ctx context.Context, store db.Store, record db.MerchantCancelWithdrawApplication, queryResp *wechatcontracts.CancelWithdrawQueryResponse) (*db.ExternalPaymentFactApplication, error) {
	if queryResp == nil {
		return nil, nil
	}

	outRequestNo := strings.TrimSpace(queryResp.OutRequestNo)
	if outRequestNo == "" {
		outRequestNo = strings.TrimSpace(record.OutRequestNo)
	}
	if outRequestNo == "" {
		return nil, nil
	}

	service := logic.NewPaymentFactService(store)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityCancelWithdraw,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectCancelWithdraw,
		ExternalObjectKey:    outRequestNo,
		ExternalSecondaryKey: optionalPaymentFactStringPtr(strings.TrimSpace(queryResp.ApplymentID)),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerMerchantFunds),
		BusinessObjectType:   paymentFactStringPtr(merchantCancelWithdrawFactBusinessType),
		BusinessObjectID:     paymentFactInt64Ptr(record.ID),
		UpstreamState:        strings.TrimSpace(queryResp.CancelState),
		TerminalStatus:       merchantCancelWithdrawQueryTerminalStatus(queryResp.CancelState),
		Currency:             "CNY",
		OccurredAt:           parseMerchantCancelWithdrawFactTime(queryResp.ModifyTime),
		UpstreamUpdatedAt:    parseMerchantCancelWithdrawFactTime(queryResp.ModifyTime),
		RawResource:          merchantCancelWithdrawQueryFactResource(record, queryResp),
		DedupeKey:            merchantCancelWithdrawQueryFactDedupeKey(outRequestNo, queryResp.CancelState, queryResp.WithdrawState, queryResp.ApplymentID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           "merchant_funds_domain",
			BusinessObjectType: merchantCancelWithdrawFactBusinessType,
			BusinessObjectID:   record.ID,
		},
		AllowNonTerminalApplication: true,
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func applyMerchantCancelWithdrawFactApplication(ctx context.Context, store db.Store, application *db.ExternalPaymentFactApplication) (db.MerchantCancelWithdrawApplication, bool, error) {
	if application == nil {
		return db.MerchantCancelWithdrawApplication{}, false, nil
	}

	result, err := logic.NewPaymentFactService(store).ApplyExternalPaymentFactApplication(ctx, application.ID)
	if err != nil {
		return db.MerchantCancelWithdrawApplication{}, false, err
	}
	if result.MerchantCancelWithdraw == nil {
		return db.MerchantCancelWithdrawApplication{}, false, nil
	}
	return result.MerchantCancelWithdraw.Application, true, nil
}

func merchantCancelWithdrawQueryTerminalStatus(cancelState string) string {
	if !logic.MerchantCancelWithdrawIsTerminal(cancelState) {
		return db.ExternalPaymentTerminalStatusProcessing
	}

	switch logic.NormalizeMerchantCancelState(cancelState) {
	case db.MerchantCancelStateFinish:
		return db.ExternalPaymentTerminalStatusSuccess
	default:
		return db.ExternalPaymentTerminalStatusFailed
	}
}

func merchantCancelWithdrawQueryFactDedupeKey(outRequestNo string, cancelState string, withdrawState string, applymentID string) string {
	suffix := strings.TrimSpace(applymentID)
	if suffix == "" {
		suffix = "current"
	}
	return fmt.Sprintf(
		"wechat:query:ecommerce:cancel_withdraw:%s:%s:%s:%s",
		strings.TrimSpace(outRequestNo),
		strings.TrimSpace(cancelState),
		strings.TrimSpace(withdrawState),
		suffix,
	)
}

func merchantCancelWithdrawQueryFactResource(record db.MerchantCancelWithdrawApplication, queryResp *wechatcontracts.CancelWithdrawQueryResponse) []byte {
	confirmCancelURL := ""
	if queryResp != nil && queryResp.ConfirmCancel != nil {
		confirmCancelURL = strings.TrimSpace(queryResp.ConfirmCancel.ConfirmCancelURL)
	}

	raw, err := json.Marshal(map[string]any{
		"application_id":             record.ID,
		"merchant_id":                record.MerchantID,
		"sub_mch_id":                 strings.TrimSpace(record.SubMchID),
		"out_request_no":             strings.TrimSpace(queryResp.OutRequestNo),
		"applyment_id":               strings.TrimSpace(queryResp.ApplymentID),
		"cancel_state":               strings.TrimSpace(queryResp.CancelState),
		"cancel_state_description":   strings.TrimSpace(queryResp.CancelStateDescription),
		"withdraw_state":             strings.TrimSpace(queryResp.WithdrawState),
		"withdraw_state_description": strings.TrimSpace(queryResp.WithdrawStateDescription),
		"withdraw":                   strings.TrimSpace(queryResp.Withdraw),
		"modify_time":                strings.TrimSpace(queryResp.ModifyTime),
		"confirm_cancel_url":         confirmCancelURL,
		"account_info":               queryResp.AccountInfo,
		"account_withdraw_result":    queryResp.AccountWithdrawResult,
	})
	if err != nil {
		return nil
	}
	return raw
}

func parseMerchantCancelWithdrawFactTime(value string) *time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil
	}
	return &parsed
}
