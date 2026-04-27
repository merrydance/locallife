package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
)

const receiverLifecycleFailureMessageLimit = 500

type ProfitSharingReceiverLifecycleService struct {
	store           profitSharingReceiverTargetStore
	ecommerceClient wechat.EcommerceClientInterface
}

type profitSharingReceiverTargetStore interface {
	ClaimProfitSharingReceiverTarget(ctx context.Context, arg db.ClaimProfitSharingReceiverTargetParams) (db.ProfitSharingReceiverTarget, error)
	CreateProfitSharingReceiverAttempt(ctx context.Context, arg db.CreateProfitSharingReceiverAttemptParams) (db.ProfitSharingReceiverAttempt, error)
	GetOperator(ctx context.Context, id int64) (db.Operator, error)
	GetRider(ctx context.Context, id int64) (db.Rider, error)
	GetUser(ctx context.Context, id int64) (db.User, error)
	MarkProfitSharingReceiverAttemptFailed(ctx context.Context, arg db.MarkProfitSharingReceiverAttemptFailedParams) (db.ProfitSharingReceiverAttempt, error)
	MarkProfitSharingReceiverAttemptSkipped(ctx context.Context, arg db.MarkProfitSharingReceiverAttemptSkippedParams) (db.ProfitSharingReceiverAttempt, error)
	MarkProfitSharingReceiverAttemptSucceeded(ctx context.Context, arg db.MarkProfitSharingReceiverAttemptSucceededParams) (db.ProfitSharingReceiverAttempt, error)
	MarkProfitSharingReceiverTargetFailed(ctx context.Context, arg db.MarkProfitSharingReceiverTargetFailedParams) (db.ProfitSharingReceiverTarget, error)
	MarkProfitSharingReceiverTargetSkipped(ctx context.Context, arg db.MarkProfitSharingReceiverTargetSkippedParams) (db.ProfitSharingReceiverTarget, error)
	MarkProfitSharingReceiverTargetSynced(ctx context.Context, arg db.MarkProfitSharingReceiverTargetSyncedParams) (db.ProfitSharingReceiverTarget, error)
	UpsertProfitSharingReceiverTarget(ctx context.Context, arg db.UpsertProfitSharingReceiverTargetParams) (db.ProfitSharingReceiverTarget, error)
}

type ProfitSharingReceiverTargetProcessResult struct {
	TargetID     int64
	OwnerType    string
	OwnerID      int64
	AttemptCount int32
	Action       string
	Status       string
	ErrorCode    string
}

type receiverLifecycleProcessFailure struct {
	code       string
	message    string
	retryAfter time.Duration
	skip       bool
}

func (e *receiverLifecycleProcessFailure) Error() string {
	if e == nil || e.message == "" {
		return "profit sharing receiver lifecycle process failed"
	}
	return e.message
}

func NewProfitSharingReceiverLifecycleService(store profitSharingReceiverTargetStore, ecommerceClient wechat.EcommerceClientInterface) *ProfitSharingReceiverLifecycleService {
	return &ProfitSharingReceiverLifecycleService{
		store:           store,
		ecommerceClient: ecommerceClient,
	}
}

func (s *ProfitSharingReceiverLifecycleService) RequestOperatorReceiverPresent(ctx context.Context, operator db.Operator) (db.ProfitSharingReceiverTarget, error) {
	return s.requestOperatorReceiverTarget(ctx, operator, db.ProfitSharingReceiverDesiredStatePresent)
}

func (s *ProfitSharingReceiverLifecycleService) RequestOperatorReceiverAbsent(ctx context.Context, operator db.Operator) (db.ProfitSharingReceiverTarget, error) {
	return s.requestOperatorReceiverTarget(ctx, operator, db.ProfitSharingReceiverDesiredStateAbsent)
}

func (s *ProfitSharingReceiverLifecycleService) RequestRiderReceiverPresent(ctx context.Context, rider db.Rider) (db.ProfitSharingReceiverTarget, error) {
	return s.requestRiderReceiverTarget(ctx, rider, db.ProfitSharingReceiverDesiredStatePresent)
}

func (s *ProfitSharingReceiverLifecycleService) RequestRiderReceiverAbsent(ctx context.Context, rider db.Rider) (db.ProfitSharingReceiverTarget, error) {
	return s.requestRiderReceiverTarget(ctx, rider, db.ProfitSharingReceiverDesiredStateAbsent)
}

func (s *ProfitSharingReceiverLifecycleService) ProcessOperatorReceiverTarget(ctx context.Context, targetID int64, now time.Time) (ProfitSharingReceiverTargetProcessResult, error) {
	return s.ProcessReceiverTarget(ctx, targetID, now)
}

func (s *ProfitSharingReceiverLifecycleService) ProcessReceiverTarget(ctx context.Context, targetID int64, now time.Time) (ProfitSharingReceiverTargetProcessResult, error) {
	if targetID <= 0 {
		return ProfitSharingReceiverTargetProcessResult{}, fmt.Errorf("invalid profit sharing receiver target id")
	}
	if now.IsZero() {
		now = time.Now()
	}

	result := ProfitSharingReceiverTargetProcessResult{TargetID: targetID}
	timestamp := receiverLifecycleTimestamp(now)
	target, err := s.store.ClaimProfitSharingReceiverTarget(ctx, db.ClaimProfitSharingReceiverTargetParams{
		NowAt: timestamp,
		ID:    targetID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			result.Status = "not_claimed"
			return result, nil
		}
		return result, fmt.Errorf("claim profit sharing receiver target: %w", err)
	}
	result.OwnerType = target.OwnerType
	result.OwnerID = target.OwnerID
	result.AttemptCount = target.AttemptCount

	result.Action = receiverLifecycleAttemptAction(target.DesiredState)
	if result.Action == "" {
		if _, markErr := s.store.MarkProfitSharingReceiverTargetSkipped(ctx, db.MarkProfitSharingReceiverTargetSkippedParams{
			SkippedAt: timestamp,
			ID:        target.ID,
		}); markErr != nil {
			return result, fmt.Errorf("mark unsupported receiver target skipped: %w", markErr)
		}
		result.Status = db.ProfitSharingReceiverSyncStatusSkipped
		result.ErrorCode = "unsupported_desired_state"
		return result, nil
	}

	attempt, err := s.store.CreateProfitSharingReceiverAttempt(ctx, db.CreateProfitSharingReceiverAttemptParams{
		TargetID:  target.ID,
		Action:    result.Action,
		Status:    db.ProfitSharingReceiverAttemptStatusProcessing,
		StartedAt: now,
	})
	if err != nil {
		markErr := s.markReceiverTargetFailed(ctx, target, timestamp, "attempt_create_failed", err.Error(), receiverLifecycleRetryDelay(target.AttemptCount))
		if markErr != nil {
			return result, fmt.Errorf("create receiver attempt: %w; mark target failed: %v", err, markErr)
		}
		return result, fmt.Errorf("create receiver attempt: %w", err)
	}

	idempotentSuccess, applyErr := s.applyReceiverTarget(ctx, target)
	if applyErr == nil {
		if _, err := s.store.MarkProfitSharingReceiverTargetSynced(ctx, db.MarkProfitSharingReceiverTargetSyncedParams{
			SyncedAt: timestamp,
			ID:       target.ID,
		}); err != nil {
			return result, fmt.Errorf("mark receiver target synced: %w", err)
		}
		if _, err := s.store.MarkProfitSharingReceiverAttemptSucceeded(ctx, db.MarkProfitSharingReceiverAttemptSucceededParams{
			IdempotentSuccess: idempotentSuccess,
			FinishedAt:        timestamp,
			ID:                attempt.ID,
		}); err != nil {
			return result, fmt.Errorf("mark receiver attempt succeeded: %w", err)
		}
		result.Status = db.ProfitSharingReceiverSyncStatusSynced
		return result, nil
	}

	failure := receiverLifecycleFailureFromError(applyErr, target.AttemptCount)
	result.ErrorCode = failure.code
	if failure.skip {
		if _, err := s.store.MarkProfitSharingReceiverTargetSkipped(ctx, db.MarkProfitSharingReceiverTargetSkippedParams{
			SkippedAt: timestamp,
			ID:        target.ID,
		}); err != nil {
			return result, fmt.Errorf("mark receiver target skipped: %w", err)
		}
		if _, err := s.store.MarkProfitSharingReceiverAttemptSkipped(ctx, db.MarkProfitSharingReceiverAttemptSkippedParams{
			FinishedAt: timestamp,
			ID:         attempt.ID,
		}); err != nil {
			return result, fmt.Errorf("mark receiver attempt skipped: %w", err)
		}
		result.Status = db.ProfitSharingReceiverSyncStatusSkipped
		return result, nil
	}

	if err := s.markReceiverTargetFailed(ctx, target, timestamp, failure.code, failure.message, failure.retryAfter); err != nil {
		return result, err
	}
	if _, err := s.store.MarkProfitSharingReceiverAttemptFailed(ctx, db.MarkProfitSharingReceiverAttemptFailedParams{
		ErrorCode:    receiverLifecycleText(failure.code),
		ErrorMessage: receiverLifecycleText(failure.message),
		FinishedAt:   timestamp,
		ID:           attempt.ID,
	}); err != nil {
		return result, fmt.Errorf("mark receiver attempt failed: %w", err)
	}
	result.Status = db.ProfitSharingReceiverSyncStatusFailed
	return result, nil
}

func (s *ProfitSharingReceiverLifecycleService) requestOperatorReceiverTarget(ctx context.Context, operator db.Operator, desiredState string) (db.ProfitSharingReceiverTarget, error) {
	if s.ecommerceClient == nil {
		return db.ProfitSharingReceiverTarget{}, fmt.Errorf("ecommerce client not configured")
	}

	appID := strings.TrimSpace(s.ecommerceClient.GetSpAppID())
	if appID == "" {
		return db.ProfitSharingReceiverTarget{}, fmt.Errorf("ecommerce client sp appid not configured")
	}

	user, err := s.store.GetUser(ctx, operator.UserID)
	if err != nil {
		return db.ProfitSharingReceiverTarget{}, fmt.Errorf("get operator user: %w", err)
	}

	openID := strings.TrimSpace(user.WechatOpenid)
	if openID == "" {
		return db.ProfitSharingReceiverTarget{}, NewRequestError(http.StatusBadRequest, ErrProfitSharingReceiverOpenIDRequired)
	}

	displayNameHash := pgtype.Text{}
	if displayName := strings.TrimSpace(operatorReceiverDisplayName(operator)); displayName != "" {
		displayNameHash = pgtype.Text{String: receiverLifecycleHash(displayName), Valid: true}
	}

	target, err := s.store.UpsertProfitSharingReceiverTarget(ctx, db.UpsertProfitSharingReceiverTargetParams{
		Provider:        db.ExternalPaymentProviderWechat,
		Channel:         db.PaymentChannelEcommerce,
		OwnerType:       db.ProfitSharingReceiverOwnerTypeOperator,
		OwnerID:         operator.ID,
		ReceiverType:    db.ProfitSharingReceiverTypePersonalOpenID,
		Appid:           appID,
		AccountHash:     receiverLifecycleHash(openID),
		DisplayNameHash: displayNameHash,
		DesiredState:    desiredState,
	})
	if err != nil {
		return db.ProfitSharingReceiverTarget{}, fmt.Errorf("upsert operator profit sharing receiver target: %w", err)
	}

	return target, nil
}

func (s *ProfitSharingReceiverLifecycleService) requestRiderReceiverTarget(ctx context.Context, rider db.Rider, desiredState string) (db.ProfitSharingReceiverTarget, error) {
	if s.ecommerceClient == nil {
		return db.ProfitSharingReceiverTarget{}, fmt.Errorf("ecommerce client not configured")
	}

	appID := strings.TrimSpace(s.ecommerceClient.GetSpAppID())
	if appID == "" {
		return db.ProfitSharingReceiverTarget{}, fmt.Errorf("ecommerce client sp appid not configured")
	}

	user, err := s.store.GetUser(ctx, rider.UserID)
	if err != nil {
		return db.ProfitSharingReceiverTarget{}, fmt.Errorf("get rider user: %w", err)
	}

	openID := strings.TrimSpace(user.WechatOpenid)
	if openID == "" {
		return db.ProfitSharingReceiverTarget{}, NewRequestError(http.StatusBadRequest, ErrProfitSharingReceiverOpenIDRequired)
	}

	displayNameHash := pgtype.Text{}
	if displayName := strings.TrimSpace(rider.RealName); displayName != "" {
		displayNameHash = pgtype.Text{String: receiverLifecycleHash(displayName), Valid: true}
	}

	target, err := s.store.UpsertProfitSharingReceiverTarget(ctx, db.UpsertProfitSharingReceiverTargetParams{
		Provider:        db.ExternalPaymentProviderWechat,
		Channel:         db.PaymentChannelEcommerce,
		OwnerType:       db.ProfitSharingReceiverOwnerTypeRider,
		OwnerID:         rider.ID,
		ReceiverType:    db.ProfitSharingReceiverTypePersonalOpenID,
		Appid:           appID,
		AccountHash:     receiverLifecycleHash(openID),
		DisplayNameHash: displayNameHash,
		DesiredState:    desiredState,
	})
	if err != nil {
		return db.ProfitSharingReceiverTarget{}, fmt.Errorf("upsert rider profit sharing receiver target: %w", err)
	}

	return target, nil
}

func (s *ProfitSharingReceiverLifecycleService) applyReceiverTarget(ctx context.Context, target db.ProfitSharingReceiverTarget) (bool, error) {
	switch target.OwnerType {
	case db.ProfitSharingReceiverOwnerTypeOperator:
		return s.applyOperatorReceiverTarget(ctx, target)
	case db.ProfitSharingReceiverOwnerTypeRider:
		return s.applyRiderReceiverTarget(ctx, target)
	default:
		return false, &receiverLifecycleProcessFailure{code: "unsupported_owner_type", message: "receiver target owner type is not supported", skip: true}
	}
}

func (s *ProfitSharingReceiverLifecycleService) applyOperatorReceiverTarget(ctx context.Context, target db.ProfitSharingReceiverTarget) (bool, error) {
	if target.OwnerType != db.ProfitSharingReceiverOwnerTypeOperator {
		return false, &receiverLifecycleProcessFailure{code: "unsupported_owner_type", message: "receiver target owner type is not operator", skip: true}
	}
	if target.Provider != db.ExternalPaymentProviderWechat || target.Channel != db.PaymentChannelEcommerce || target.ReceiverType != db.ProfitSharingReceiverTypePersonalOpenID {
		return false, &receiverLifecycleProcessFailure{code: "unsupported_receiver_target", message: "receiver target route is not supported", skip: true}
	}
	if s.ecommerceClient == nil {
		return false, &receiverLifecycleProcessFailure{code: "ecommerce_client_not_configured", message: "ecommerce client not configured", retryAfter: receiverLifecycleRetryDelay(target.AttemptCount)}
	}
	appID := strings.TrimSpace(s.ecommerceClient.GetSpAppID())
	if appID == "" {
		return false, &receiverLifecycleProcessFailure{code: "appid_not_configured", message: "ecommerce client sp appid not configured", retryAfter: receiverLifecycleRetryDelay(target.AttemptCount)}
	}
	if target.Appid != appID {
		return false, &receiverLifecycleProcessFailure{code: "appid_mismatch", message: "receiver target appid does not match ecommerce client appid", retryAfter: 24 * time.Hour}
	}

	operator, err := s.store.GetOperator(ctx, target.OwnerID)
	if err != nil {
		return false, receiverLifecycleStoreFailure("get_operator_failed", err, target.AttemptCount)
	}
	user, err := s.store.GetUser(ctx, operator.UserID)
	if err != nil {
		return false, receiverLifecycleStoreFailure("get_operator_user_failed", err, target.AttemptCount)
	}

	openID := strings.TrimSpace(user.WechatOpenid)
	if openID == "" {
		return false, &receiverLifecycleProcessFailure{code: "operator_openid_missing", message: "operator wechat openid is empty", retryAfter: 24 * time.Hour}
	}
	if receiverLifecycleHash(openID) != target.AccountHash {
		return false, &receiverLifecycleProcessFailure{code: "operator_openid_hash_mismatch", message: "operator wechat openid hash does not match receiver target", retryAfter: 24 * time.Hour}
	}

	switch target.DesiredState {
	case db.ProfitSharingReceiverDesiredStatePresent:
		return ensurePersonalOpenIDReceiverWithResult(ctx, s.ecommerceClient, openID, operatorReceiverDisplayName(operator))
	case db.ProfitSharingReceiverDesiredStateAbsent:
		return deletePersonalOpenIDReceiverWithResult(ctx, s.ecommerceClient, openID)
	default:
		return false, &receiverLifecycleProcessFailure{code: "unsupported_desired_state", message: "receiver target desired state is not supported", skip: true}
	}
}

func (s *ProfitSharingReceiverLifecycleService) applyRiderReceiverTarget(ctx context.Context, target db.ProfitSharingReceiverTarget) (bool, error) {
	if target.Provider != db.ExternalPaymentProviderWechat || target.Channel != db.PaymentChannelEcommerce || target.ReceiverType != db.ProfitSharingReceiverTypePersonalOpenID {
		return false, &receiverLifecycleProcessFailure{code: "unsupported_receiver_target", message: "receiver target route is not supported", skip: true}
	}
	if s.ecommerceClient == nil {
		return false, &receiverLifecycleProcessFailure{code: "ecommerce_client_not_configured", message: "ecommerce client not configured", retryAfter: receiverLifecycleRetryDelay(target.AttemptCount)}
	}
	appID := strings.TrimSpace(s.ecommerceClient.GetSpAppID())
	if appID == "" {
		return false, &receiverLifecycleProcessFailure{code: "appid_not_configured", message: "ecommerce client sp appid not configured", retryAfter: receiverLifecycleRetryDelay(target.AttemptCount)}
	}
	if target.Appid != appID {
		return false, &receiverLifecycleProcessFailure{code: "appid_mismatch", message: "receiver target appid does not match ecommerce client appid", retryAfter: 24 * time.Hour}
	}

	rider, err := s.store.GetRider(ctx, target.OwnerID)
	if err != nil {
		return false, receiverLifecycleStoreFailure("get_rider_failed", err, target.AttemptCount)
	}
	user, err := s.store.GetUser(ctx, rider.UserID)
	if err != nil {
		return false, receiverLifecycleStoreFailure("get_rider_user_failed", err, target.AttemptCount)
	}

	openID := strings.TrimSpace(user.WechatOpenid)
	if openID == "" {
		return false, &receiverLifecycleProcessFailure{code: "rider_openid_missing", message: "rider wechat openid is empty", retryAfter: 24 * time.Hour}
	}
	if receiverLifecycleHash(openID) != target.AccountHash {
		return false, &receiverLifecycleProcessFailure{code: "rider_openid_hash_mismatch", message: "rider wechat openid hash does not match receiver target", retryAfter: 24 * time.Hour}
	}

	switch target.DesiredState {
	case db.ProfitSharingReceiverDesiredStatePresent:
		return ensurePersonalOpenIDReceiverWithResult(ctx, s.ecommerceClient, openID, strings.TrimSpace(rider.RealName))
	case db.ProfitSharingReceiverDesiredStateAbsent:
		return deletePersonalOpenIDReceiverWithResult(ctx, s.ecommerceClient, openID)
	default:
		return false, &receiverLifecycleProcessFailure{code: "unsupported_desired_state", message: "receiver target desired state is not supported", skip: true}
	}
}

func (s *ProfitSharingReceiverLifecycleService) markReceiverTargetFailed(ctx context.Context, target db.ProfitSharingReceiverTarget, now pgtype.Timestamptz, code string, message string, retryAfter time.Duration) error {
	_, err := s.store.MarkProfitSharingReceiverTargetFailed(ctx, db.MarkProfitSharingReceiverTargetFailedParams{
		LastErrorCode:    receiverLifecycleText(code),
		LastErrorMessage: receiverLifecycleText(message),
		NextRetryAt:      receiverLifecycleTimestamp(now.Time.Add(retryAfter)),
		ID:               target.ID,
	})
	if err != nil {
		return fmt.Errorf("mark receiver target failed: %w", err)
	}
	return nil
}

func receiverLifecycleAttemptAction(desiredState string) string {
	switch desiredState {
	case db.ProfitSharingReceiverDesiredStatePresent:
		return db.ProfitSharingReceiverAttemptActionEnsure
	case db.ProfitSharingReceiverDesiredStateAbsent:
		return db.ProfitSharingReceiverAttemptActionDelete
	default:
		return ""
	}
}

func receiverLifecycleFailureFromError(err error, attemptCount int32) *receiverLifecycleProcessFailure {
	var failure *receiverLifecycleProcessFailure
	if errors.As(err, &failure) {
		if failure.retryAfter <= 0 && !failure.skip {
			failure.retryAfter = receiverLifecycleRetryDelay(attemptCount)
		}
		failure.message = truncateReceiverLifecycleMessage(failure.message)
		return failure
	}

	code := "receiver_sync_failed"
	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) && strings.TrimSpace(wxErr.Code) != "" {
		wechatCode := strings.ToLower(strings.TrimSpace(wxErr.Code))
		code = "wechat_" + wechatCode
		return &receiverLifecycleProcessFailure{
			code:       code,
			message:    "wechat receiver sync failed: " + wechatCode,
			retryAfter: receiverLifecycleRetryDelay(attemptCount),
		}
	}

	return &receiverLifecycleProcessFailure{
		code:       code,
		message:    truncateReceiverLifecycleMessage(err.Error()),
		retryAfter: receiverLifecycleRetryDelay(attemptCount),
	}
}

func receiverLifecycleStoreFailure(code string, err error, attemptCount int32) error {
	return &receiverLifecycleProcessFailure{code: code, message: err.Error(), retryAfter: receiverLifecycleRetryDelay(attemptCount)}
}

func receiverLifecycleRetryDelay(attemptCount int32) time.Duration {
	if attemptCount < 1 {
		attemptCount = 1
	}
	shift := attemptCount - 1
	if shift > 5 {
		shift = 5
	}
	delay := time.Duration(1<<shift) * time.Minute
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}

func receiverLifecycleText(value string) pgtype.Text {
	value = strings.TrimSpace(truncateReceiverLifecycleMessage(value))
	return pgtype.Text{String: value, Valid: value != ""}
}

func receiverLifecycleTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func truncateReceiverLifecycleMessage(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= receiverLifecycleFailureMessageLimit {
		return value
	}
	return value[:receiverLifecycleFailureMessageLimit]
}

func receiverLifecycleHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return "sha256:" + hex.EncodeToString(sum[:])
}
