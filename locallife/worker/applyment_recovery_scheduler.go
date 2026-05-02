package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	applymentRecoveryCron       = "*/5 * * * *"
	applymentRecoveryBatchLimit = int32(100)
	applymentRecoveryMinAge     = 2 * time.Minute
)

type ApplymentRecoveryScheduler struct {
	cron           *cron.Cron
	wg             sync.WaitGroup
	stopCtx        context.Context
	stopCancel     context.CancelFunc
	runMu          sync.Mutex
	store          db.Store
	distributor    TaskDistributor
	ordinaryClient ordinaryserviceprovider.OrdinaryServiceProviderClientInterface
}

func NewApplymentRecoveryScheduler(store db.Store, distributor TaskDistributor, ordinaryClient ordinaryserviceprovider.OrdinaryServiceProviderClientInterface) *ApplymentRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &ApplymentRecoveryScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:        stopCtx,
		stopCancel:     stopCancel,
		store:          store,
		distributor:    distributor,
		ordinaryClient: ordinaryClient,
	}
}

func (s *ApplymentRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(applymentRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("applyment recovery scheduler started (every 5 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *ApplymentRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("applyment recovery scheduler stopped")
}

func (s *ApplymentRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *ApplymentRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("applyment recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip applyment recovery")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	applyments, err := s.store.ListEcommerceApplymentsPendingFollowUp(ctx, db.ListEcommerceApplymentsPendingFollowUpParams{
		UpdatedBefore: time.Now().Add(-applymentRecoveryMinAge),
		Limit:         applymentRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list ecommerce applyments for recovery failed")
		return
	}

	for _, applyment := range applyments {
		if applyment.SubjectType != "merchant" {
			log.Info().
				Int64("applyment_id", applyment.ID).
				Str("subject_type", applyment.SubjectType).
				Msg("skip non-merchant applyment recovery record")
			continue
		}

		status := normalizeApplymentFollowUpStatus(applyment.Status, textValue(applyment.SubMchID))
		processedState := normalizeApplymentFollowUpStatus(textValue(applyment.ResultTaskProcessedState), textValue(applyment.SubMchID))
		signState := textValue(applyment.SignState)
		resolvedProcessedState := resolveApplymentResultStatus(ApplymentResultPayload{
			ApplymentStatus: processedState,
			SignState:       signState,
			SubMchID:        textValue(applyment.SubMchID),
		})

		if applymentStatusNeedsAsyncFollowUp(status, signState) {
			resolvedStatus := resolveApplymentResultStatus(ApplymentResultPayload{
				ApplymentStatus: status,
				SignState:       signState,
				SubMchID:        textValue(applyment.SubMchID),
			})
			if resolvedProcessedState != resolvedStatus && !applymentStatusHandledByFact(resolvedStatus, textValue(applyment.SubMchID)) {
				s.enqueueApplymentFollowUp(ctx, applyment, "", status, signState, textValue(applyment.SubMchID))
				continue
			}
		}

		if applymentNeedsAccountAuthorizeRecovery(status, textValue(applyment.SubMchID), textValue(applyment.AccountAuthorizeState)) {
			if s.ordinaryClient == nil {
				log.Warn().Int64("applyment_id", applyment.ID).Msg("ordinary service provider client not configured, cannot query account authorize recovery state")
				continue
			}
			if err := s.recoverApplymentAccountAuthorizeState(ctx, applyment); err != nil {
				log.Error().Err(err).
					Int64("applyment_id", applyment.ID).
					Str("out_request_no", applyment.OutRequestNo).
					Str("sub_mch_id", textValue(applyment.SubMchID)).
					Msg("recover applyment account authorize state failed")
			}
			continue
		}

		if !applymentStatusNeedsRemoteQuery(status) {
			continue
		}
		if s.ordinaryClient == nil {
			log.Warn().Int64("applyment_id", applyment.ID).Msg("ordinary service provider client not configured, cannot query applyment recovery status")
			continue
		}

		queryResp, err := s.queryApplymentStatus(ctx, applyment)
		if err != nil {
			log.Error().Err(err).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", applyment.OutRequestNo).
				Msg("query applyment recovery status failed")
			continue
		}

		resolvedStatus, resolvedSubMchID, err := s.reconcileApplymentStatus(ctx, applyment, queryResp)
		if err != nil {
			log.Error().Err(err).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", applyment.OutRequestNo).
				Msg("reconcile applyment recovery status failed")
			continue
		}

		resolvedFollowUpState := resolveApplymentResultStatus(ApplymentResultPayload{
			ApplymentStatus: resolvedStatus,
			ApplymentState:  string(queryResp.ApplymentState),
			SubMchID:        resolvedSubMchID,
		})
		if applymentStatusNeedsAsyncFollowUp(resolvedStatus, "") && resolvedProcessedState != resolvedFollowUpState && !applymentStatusHandledByFact(resolvedStatus, resolvedSubMchID) {
			s.enqueueApplymentFollowUp(ctx, applyment, string(queryResp.ApplymentState), resolvedStatus, "", resolvedSubMchID)
		}
	}
}

func (s *ApplymentRecoveryScheduler) queryApplymentStatus(ctx context.Context, applyment db.EcommerceApplymentPendingFollowUp) (*ospcontracts.ApplymentQueryResponse, error) {
	if applyment.ApplymentID.Valid {
		resp, err := s.ordinaryClient.QueryApplymentByID(ctx, ospcontracts.ApplymentQueryByIDRequest{ApplymentID: applyment.ApplymentID.Int64})
		if err == nil {
			log.Info().
				Int64("applyment_id", applyment.ID).
				Str("query_key", "applyment_id").
				Int64("wechat_applyment_id", resp.ApplymentID).
				Str("business_code", strings.TrimSpace(resp.BusinessCode)).
				Str("applyment_state", strings.TrimSpace(string(resp.ApplymentState))).
				Bool("has_sign_url", strings.TrimSpace(resp.SignURL) != "").
				Msg("query applyment recovery status succeeded")
			return resp, nil
		}
		if strings.TrimSpace(applyment.OutRequestNo) == "" {
			return nil, err
		}

		log.Warn().Err(err).
			Int64("applyment_id", applyment.ID).
			Int64("wechat_applyment_id", applyment.ApplymentID.Int64).
			Str("out_request_no", applyment.OutRequestNo).
			Msg("query applyment recovery by id failed, fallback to business_code")
	}
	resp, err := s.ordinaryClient.QueryApplymentByBusinessCode(ctx, ospcontracts.ApplymentQueryByBusinessCodeRequest{BusinessCode: applyment.OutRequestNo})
	if err == nil {
		log.Info().
			Int64("applyment_id", applyment.ID).
			Str("query_key", "business_code").
			Int64("wechat_applyment_id", resp.ApplymentID).
			Str("business_code", strings.TrimSpace(resp.BusinessCode)).
			Str("applyment_state", strings.TrimSpace(string(resp.ApplymentState))).
			Bool("has_sign_url", strings.TrimSpace(resp.SignURL) != "").
			Msg("query applyment recovery status succeeded")
	}
	return resp, err
}

func (s *ApplymentRecoveryScheduler) reconcileApplymentStatus(ctx context.Context, applyment db.EcommerceApplymentPendingFollowUp, queryResp *ospcontracts.ApplymentQueryResponse) (string, string, error) {
	mappedStatus := mapOrdinaryApplymentStateToStatus(queryResp.ApplymentState)
	if mappedStatus == "" {
		s.persistUnsupportedApplymentRecoveryStateAlert(ctx, applyment, queryResp)
		log.Warn().
			Int64("applyment_id", applyment.ID).
			Str("out_request_no", applyment.OutRequestNo).
			Str("applyment_state", strings.TrimSpace(string(queryResp.ApplymentState))).
			Msg("applyment recovery query returned unsupported upstream state; local status update skipped")
		return "", "", nil
	}
	resolvedStatus := normalizeApplymentFollowUpStatus(mappedStatus, queryResp.SubMchID)
	resolvedSubMchID := queryResp.SubMchID

	if resolvedStatus == "finish" && resolvedSubMchID != "" {
		application, err := recordApplymentActivatedQueryFact(ctx, s.store, applyment, queryResp, "")
		if err != nil {
			return "", "", err
		}
		if err := EnqueueApplymentPaymentFactApplication(ctx, s.distributor, application); err != nil {
			return "", "", err
		}

		return normalizeApplymentFollowUpStatus("finish", resolvedSubMchID), resolvedSubMchID, nil
	}
	if resolvedStatus == "rejected" || resolvedStatus == "frozen" || resolvedStatus == "canceled" {
		application, err := recordApplymentTerminalQueryFact(ctx, s.store, applyment, queryResp)
		if err != nil {
			return "", "", err
		}
		if err := EnqueueApplymentPaymentFactApplication(ctx, s.distributor, application); err != nil {
			return "", "", err
		}

		return resolvedStatus, resolvedSubMchID, nil
	}
	if resolvedStatus == "account_need_verify" || resolvedStatus == "to_be_confirmed" || resolvedStatus == "to_be_signed" {
		application, err := recordApplymentPendingQueryFact(ctx, s.store, applyment, queryResp)
		if err != nil {
			return "", "", err
		}
		if err := EnqueueApplymentPaymentFactApplication(ctx, s.distributor, application); err != nil {
			return "", "", err
		}

		return resolvedStatus, resolvedSubMchID, nil
	}

	if _, err := s.store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
		ID:                 applyment.ID,
		ApplymentID:        pgtype.Int8{Int64: queryResp.ApplymentID, Valid: queryResp.ApplymentID > 0},
		Status:             resolvedStatus,
		RejectReason:       getRejectReasonFromOrdinaryApplymentAuditDetail(queryResp.AuditDetail),
		SignUrl:            pgtype.Text{String: queryResp.SignURL, Valid: queryResp.SignURL != ""},
		SignState:          pgtype.Text{},
		LegalValidationUrl: pgtype.Text{},
		AccountValidation:  nil,
		SubMchID:           pgtype.Text{String: resolvedSubMchID, Valid: resolvedSubMchID != ""},
	}); err != nil {
		return "", "", err
	}

	return resolvedStatus, resolvedSubMchID, nil
}

func applymentNeedsAccountAuthorizeRecovery(status, subMchID, accountAuthorizeState string) bool {
	if status != "finish" || strings.TrimSpace(subMchID) == "" {
		return false
	}
	return strings.TrimSpace(accountAuthorizeState) != db.AccountAuthorizeStateAuthorized
}

func (s *ApplymentRecoveryScheduler) recoverApplymentAccountAuthorizeState(ctx context.Context, applyment db.EcommerceApplymentPendingFollowUp) error {
	subMchID := strings.TrimSpace(textValue(applyment.SubMchID))
	if subMchID == "" {
		return errors.New("applyment sub_mch_id missing")
	}
	authResp, err := s.ordinaryClient.QueryAccountAuthorizeState(ctx, ospcontracts.AccountAuthorizeStateRequest{SubMchID: subMchID})
	if err != nil {
		return fmt.Errorf("query account authorize state: %w", err)
	}
	accountAuthorizeState := ""
	if authResp != nil {
		accountAuthorizeState = strings.TrimSpace(string(authResp.AuthorizeState))
	}
	if accountAuthorizeState == "" {
		return errors.New("account authorize state missing")
	}
	queryResp := &ospcontracts.ApplymentQueryResponse{
		ApplymentID:    applyment.ApplymentID.Int64,
		BusinessCode:   applyment.OutRequestNo,
		ApplymentState: ospcontracts.ApplymentStateFinished,
		SubMchID:       subMchID,
	}
	application, err := recordApplymentActivatedQueryFact(ctx, s.store, applyment, queryResp, accountAuthorizeState)
	if err != nil {
		return err
	}
	if err := EnqueueApplymentPaymentFactApplication(ctx, s.distributor, application); err != nil {
		return err
	}
	return nil
}

func (s *ApplymentRecoveryScheduler) persistUnsupportedApplymentRecoveryStateAlert(ctx context.Context, applyment db.EcommerceApplymentPendingFollowUp, queryResp *ospcontracts.ApplymentQueryResponse) {
	applymentState := ""
	stateDesc := ""
	signState := ""
	wechatApplymentID := int64(0)
	wechatOutRequestNo := ""
	subMchID := textValue(applyment.SubMchID)
	if queryResp != nil {
		applymentState = strings.TrimSpace(string(queryResp.ApplymentState))
		stateDesc = strings.TrimSpace(queryResp.ApplymentStateMsg)
		wechatApplymentID = queryResp.ApplymentID
		wechatOutRequestNo = strings.TrimSpace(queryResp.BusinessCode)
		if strings.TrimSpace(queryResp.SubMchID) != "" {
			subMchID = strings.TrimSpace(queryResp.SubMchID)
		}
	}

	err := SavePlatformAlertEvent(
		ctx,
		s.store,
		string(AlertTypeSystemError),
		string(AlertLevelCritical),
		"进件恢复查询返回未知状态",
		fmt.Sprintf("进件申请 %s 查询微信侧返回未知状态 %s，系统已停止直接更新本地状态，请人工核对并补齐状态映射或 terminalizer。", applyment.OutRequestNo, applymentState),
		applyment.ID,
		applymentFactBusinessObjectApplyment,
		map[string]any{
			"applyment_id":          applyment.ID,
			"out_request_no":        applyment.OutRequestNo,
			"wechat_out_request_no": wechatOutRequestNo,
			"wechat_applyment_id":   wechatApplymentID,
			"applyment_state":       applymentState,
			"applyment_state_desc":  stateDesc,
			"sign_state":            signState,
			"local_status":          applyment.Status,
			"sub_mch_id":            subMchID,
			"requires_mapping":      true,
		},
		time.Now(),
	)
	if err != nil {
		log.Error().Err(err).
			Int64("applyment_id", applyment.ID).
			Str("out_request_no", applyment.OutRequestNo).
			Str("applyment_state", applymentState).
			Msg("persist unsupported applyment recovery state alert failed")
	}
}

func (s *ApplymentRecoveryScheduler) enqueueApplymentFollowUp(ctx context.Context, applyment db.EcommerceApplymentPendingFollowUp, applymentState, applymentStatus, signState, subMchID string) {
	payload := buildApplymentResultPayload(applyment, applymentState, applymentStatus, signState, subMchID)
	if err := s.distributor.DistributeTaskProcessApplymentResult(
		ctx,
		payload,
		asynq.MaxRetry(5),
		asynq.Queue(QueueCritical),
	); err != nil {
		log.Error().Err(err).
			Int64("applyment_id", applyment.ID).
			Str("applyment_status", payload.ApplymentStatus).
			Msg("enqueue applyment recovery follow-up failed")
		return
	}

	log.Info().
		Int64("applyment_id", applyment.ID).
		Str("applyment_status", payload.ApplymentStatus).
		Msg("applyment recovery follow-up task enqueued")
}
