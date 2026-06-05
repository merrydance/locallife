package logic

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
)

const (
	defaultYilianyunAuthorizationSessionTTL = 10 * time.Minute
	yilianyunRefreshTokenTTL                = 35 * 24 * time.Hour
)

type cloudPrinterAuthorizationStore interface {
	CreateCloudPrinterAuthorizationSession(context.Context, db.CreateCloudPrinterAuthorizationSessionParams) (db.CloudPrinterAuthorizationSession, error)
	GetActiveCloudPrinterAuthorizationSessionForUpdate(context.Context, string) (db.CloudPrinterAuthorizationSession, error)
	AuthorizeYilianyunCloudPrinterWithDeviceTx(context.Context, db.AuthorizeYilianyunCloudPrinterWithDeviceTxParams) (db.AuthorizeYilianyunCloudPrinterWithDeviceTxResult, error)
	CreateAuthorizedYilianyunCloudPrinterTx(context.Context, db.CreateAuthorizedYilianyunCloudPrinterTxParams) (db.CreateAuthorizedYilianyunCloudPrinterTxResult, error)
}

type YilianyunAuthorizationOAuthClient interface {
	BuildAuthorizeURL(state string) (string, error)
	ExchangeAuthorizationCode(ctx context.Context, code string) (cloudprint.YilianyunAuthorizationToken, error)
	AuthorizeScannedPrinter(ctx context.Context, input cloudprint.YilianyunScannedPrinterAuthorizationInput) (cloudprint.YilianyunAuthorizationToken, error)
}

type CloudPrinterAuthorizationServiceConfig struct {
	SessionTTL     time.Duration
	Now            func() time.Time
	StateGenerator func() (string, error)
}

type CloudPrinterAuthorizationService struct {
	store     cloudPrinterAuthorizationStore
	oauth     YilianyunAuthorizationOAuthClient
	encryptor util.DataEncryptor
	config    CloudPrinterAuthorizationServiceConfig
}

type CreateYilianyunAuthorizationSessionInput struct {
	MerchantID  int64
	CreatedBy   int64
	PrinterName string
	PrinterRole string
}

type CreateYilianyunAuthorizationSessionResult struct {
	SessionID    int64
	State        string
	AuthorizeURL string
	ExpiresAt    time.Time
}

type CompleteYilianyunAuthorizationCodeInput struct {
	State string
	Code  string
}

type AuthorizeScannedYilianyunPrinterInput struct {
	MerchantID  int64
	MachineCode string
	QRKey       string
	MSign       string
	PrinterName string
	PrinterRole string
}

type YilianyunAuthorizationResult struct {
	AuthorizationID  int64
	MerchantID       int64
	ProviderType     string
	MachineCode      string
	Status           string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
	Printer          CloudPrinterDeviceResult
}

type CloudPrinterDeviceResult struct {
	ID               int64
	MerchantID       int64
	PrinterName      string
	PrinterSN        string
	PrinterKey       string
	PrinterType      string
	PrinterRole      string
	PrintTakeout     bool
	PrintDineIn      bool
	PrintReservation bool
	IsActive         bool
	CreatedAt        time.Time
	UpdatedAt        pgtype.Timestamptz
}

func NewCloudPrinterAuthorizationService(store cloudPrinterAuthorizationStore, oauth YilianyunAuthorizationOAuthClient, encryptor util.DataEncryptor, config CloudPrinterAuthorizationServiceConfig) *CloudPrinterAuthorizationService {
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	if config.StateGenerator == nil {
		config.StateGenerator = generateCloudPrinterAuthorizationState
	}
	if config.SessionTTL <= 0 {
		config.SessionTTL = defaultYilianyunAuthorizationSessionTTL
	}
	return &CloudPrinterAuthorizationService{
		store:     store,
		oauth:     oauth,
		encryptor: encryptor,
		config:    config,
	}
}

func (s *CloudPrinterAuthorizationService) CreateYilianyunAuthorizationSession(ctx context.Context, input CreateYilianyunAuthorizationSessionInput) (CreateYilianyunAuthorizationSessionResult, error) {
	if s == nil || s.store == nil {
		return CreateYilianyunAuthorizationSessionResult{}, NewRequestError(http.StatusServiceUnavailable, errors.New("云打印授权服务暂不可用，请联系平台处理"))
	}
	if s.oauth == nil {
		return CreateYilianyunAuthorizationSessionResult{}, NewRequestError(http.StatusServiceUnavailable, errors.New("易联云授权服务未配置，请联系平台处理"))
	}
	if input.MerchantID <= 0 {
		return CreateYilianyunAuthorizationSessionResult{}, NewRequestError(http.StatusBadRequest, errors.New("merchant_id is required"))
	}
	if input.CreatedBy <= 0 {
		return CreateYilianyunAuthorizationSessionResult{}, NewRequestError(http.StatusBadRequest, errors.New("created_by is required"))
	}
	printerRole := normalizeCloudPrinterRole(input.PrinterRole)
	if printerRole == "" {
		printerRole = "front"
	}
	if printerRole != "front" && printerRole != "kitchen" {
		return CreateYilianyunAuthorizationSessionResult{}, NewRequestError(http.StatusBadRequest, errors.New("printer_role must be front or kitchen"))
	}
	state, err := s.config.StateGenerator()
	if err != nil {
		return CreateYilianyunAuthorizationSessionResult{}, fmt.Errorf("generate yilianyun authorization state: %w", err)
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return CreateYilianyunAuthorizationSessionResult{}, fmt.Errorf("generate yilianyun authorization state: empty state")
	}

	expiresAt := s.config.Now().UTC().Add(s.config.SessionTTL)
	session, err := s.store.CreateCloudPrinterAuthorizationSession(ctx, db.CreateCloudPrinterAuthorizationSessionParams{
		State:        state,
		MerchantID:   input.MerchantID,
		ProviderType: db.CloudPrinterProviderYilianyun,
		PrinterName:  optionalPgText(input.PrinterName),
		PrinterRole:  pgtype.Text{String: printerRole, Valid: true},
		CreatedBy:    pgtype.Int8{Int64: input.CreatedBy, Valid: true},
		ExpiresAt:    expiresAt,
	})
	if err != nil {
		return CreateYilianyunAuthorizationSessionResult{}, fmt.Errorf("create yilianyun authorization session: %w", err)
	}
	authorizeURL, err := s.oauth.BuildAuthorizeURL(session.State)
	if err != nil {
		return CreateYilianyunAuthorizationSessionResult{}, NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("易联云授权服务未配置，请联系平台处理"), err)
	}
	return CreateYilianyunAuthorizationSessionResult{
		SessionID:    session.ID,
		State:        session.State,
		AuthorizeURL: authorizeURL,
		ExpiresAt:    session.ExpiresAt,
	}, nil
}

func (s *CloudPrinterAuthorizationService) CompleteYilianyunAuthorizationCode(ctx context.Context, input CompleteYilianyunAuthorizationCodeInput) (YilianyunAuthorizationResult, error) {
	if s == nil || s.store == nil {
		return YilianyunAuthorizationResult{}, NewRequestError(http.StatusServiceUnavailable, errors.New("云打印授权服务暂不可用，请联系平台处理"))
	}
	if s.oauth == nil {
		return YilianyunAuthorizationResult{}, NewRequestError(http.StatusServiceUnavailable, errors.New("易联云授权服务未配置，请联系平台处理"))
	}
	state := strings.TrimSpace(input.State)
	code := strings.TrimSpace(input.Code)
	if state == "" {
		return YilianyunAuthorizationResult{}, NewRequestError(http.StatusBadRequest, errors.New("state is required"))
	}
	if code == "" {
		return YilianyunAuthorizationResult{}, NewRequestError(http.StatusBadRequest, errors.New("code is required"))
	}

	session, err := s.store.GetActiveCloudPrinterAuthorizationSessionForUpdate(ctx, state)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return YilianyunAuthorizationResult{}, NewRequestError(http.StatusNotFound, errors.New("易联云授权会话已失效，请重新发起授权"))
		}
		return YilianyunAuthorizationResult{}, fmt.Errorf("load yilianyun authorization session: %w", err)
	}
	if session.ProviderType != db.CloudPrinterProviderYilianyun {
		return YilianyunAuthorizationResult{}, NewRequestError(http.StatusBadRequest, errors.New("unsupported cloud printer provider"))
	}

	token, err := s.oauth.ExchangeAuthorizationCode(ctx, code)
	if err != nil {
		return YilianyunAuthorizationResult{}, safeYilianyunProviderError("易联云授权失败，请稍后重试或联系平台处理", err)
	}
	params, err := s.buildAuthorizationParams(session.MerchantID, token)
	if err != nil {
		return YilianyunAuthorizationResult{}, err
	}
	result, err := s.store.AuthorizeYilianyunCloudPrinterWithDeviceTx(ctx, db.AuthorizeYilianyunCloudPrinterWithDeviceTxParams{
		State:         state,
		Authorization: params,
		Printer:       s.buildYilianyunPrinterParams(params, session.PrinterName.String, session.PrinterRole.String),
		ConsumedAt:    s.config.Now().UTC(),
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return YilianyunAuthorizationResult{}, NewRequestError(http.StatusConflict, errors.New("易联云授权会话已处理，请勿重复提交"))
		}
		return YilianyunAuthorizationResult{}, fmt.Errorf("persist yilianyun authorization: %w", err)
	}
	return newYilianyunAuthorizationResultWithPrinter(result.Authorization, result.Printer), nil
}

func (s *CloudPrinterAuthorizationService) AuthorizeScannedYilianyunPrinter(ctx context.Context, input AuthorizeScannedYilianyunPrinterInput) (YilianyunAuthorizationResult, error) {
	if s == nil || s.store == nil {
		return YilianyunAuthorizationResult{}, NewRequestError(http.StatusServiceUnavailable, errors.New("云打印授权服务暂不可用，请联系平台处理"))
	}
	if s.oauth == nil {
		return YilianyunAuthorizationResult{}, NewRequestError(http.StatusServiceUnavailable, errors.New("易联云授权服务未配置，请联系平台处理"))
	}
	machineCode := strings.TrimSpace(input.MachineCode)
	qrKey := strings.TrimSpace(input.QRKey)
	msign := strings.TrimSpace(input.MSign)
	if input.MerchantID <= 0 {
		return YilianyunAuthorizationResult{}, NewRequestError(http.StatusBadRequest, errors.New("merchant_id is required"))
	}
	if machineCode == "" {
		return YilianyunAuthorizationResult{}, NewRequestError(http.StatusBadRequest, errors.New("machine_code is required"))
	}
	if (qrKey == "" && msign == "") || (qrKey != "" && msign != "") {
		return YilianyunAuthorizationResult{}, NewRequestError(http.StatusBadRequest, errors.New("qr_key 和 msign 必须且只能填写一个"))
	}

	token, err := s.oauth.AuthorizeScannedPrinter(ctx, cloudprint.YilianyunScannedPrinterAuthorizationInput{
		MachineCode: machineCode,
		QRKey:       qrKey,
		MSign:       msign,
	})
	if err != nil {
		return YilianyunAuthorizationResult{}, safeYilianyunProviderError("易联云扫码授权失败，请稍后重试或联系平台处理", err)
	}
	if strings.TrimSpace(token.MachineCode) == "" {
		token.MachineCode = machineCode
	}
	params, err := s.buildAuthorizationParams(input.MerchantID, token)
	if err != nil {
		return YilianyunAuthorizationResult{}, err
	}
	result, err := s.store.CreateAuthorizedYilianyunCloudPrinterTx(ctx, db.CreateAuthorizedYilianyunCloudPrinterTxParams{
		Authorization: params,
		Printer:       s.buildYilianyunPrinterParams(params, input.PrinterName, input.PrinterRole),
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return YilianyunAuthorizationResult{}, NewRequestError(http.StatusConflict, errors.New("该易联云打印机已绑定其他商户或目标设备无效"))
		}
		return YilianyunAuthorizationResult{}, fmt.Errorf("persist scanned yilianyun authorization and printer: %w", err)
	}
	return newYilianyunAuthorizationResultWithPrinter(result.Authorization, result.Printer), nil
}

func (s *CloudPrinterAuthorizationService) buildAuthorizationParams(merchantID int64, token cloudprint.YilianyunAuthorizationToken) (db.UpsertCloudPrinterProviderAuthorizationParams, error) {
	machineCode := strings.TrimSpace(token.MachineCode)
	if machineCode == "" {
		return db.UpsertCloudPrinterProviderAuthorizationParams{}, safeYilianyunProviderError("易联云授权响应异常，请稍后重试或联系平台处理", errors.New("missing machine_code"))
	}
	if token.ExpiresInSeconds <= 0 {
		return db.UpsertCloudPrinterProviderAuthorizationParams{}, safeYilianyunProviderError("易联云授权响应异常，请稍后重试或联系平台处理", errors.New("invalid expires_in"))
	}
	accessTokenCiphertext, err := util.EncryptSensitiveField(s.encryptor, strings.TrimSpace(token.AccessToken))
	if err != nil {
		return db.UpsertCloudPrinterProviderAuthorizationParams{}, fmt.Errorf("encrypt yilianyun access token: %w", err)
	}
	refreshTokenCiphertext, err := util.EncryptSensitiveField(s.encryptor, strings.TrimSpace(token.RefreshToken))
	if err != nil {
		return db.UpsertCloudPrinterProviderAuthorizationParams{}, fmt.Errorf("encrypt yilianyun refresh token: %w", err)
	}
	if accessTokenCiphertext == "" || refreshTokenCiphertext == "" {
		return db.UpsertCloudPrinterProviderAuthorizationParams{}, safeYilianyunProviderError("易联云授权响应异常，请稍后重试或联系平台处理", errors.New("missing token"))
	}
	now := s.config.Now().UTC()
	return db.UpsertCloudPrinterProviderAuthorizationParams{
		MerchantID:               merchantID,
		ProviderType:             db.CloudPrinterProviderYilianyun,
		MachineCode:              machineCode,
		AuthorizedCloudPrinterID: pgtype.Int8{Valid: false},
		AccessTokenCiphertext:    accessTokenCiphertext,
		RefreshTokenCiphertext:   refreshTokenCiphertext,
		AccessTokenExpiresAt:     now.Add(time.Duration(token.ExpiresInSeconds) * time.Second),
		RefreshTokenExpiresAt:    now.Add(yilianyunRefreshTokenTTL),
		Status:                   db.CloudPrinterAuthorizationStatusActive,
		RefreshFailureCount:      0,
		RefreshLastAttemptedAt:   pgtype.Timestamptz{Valid: false},
		LastProviderError:        pgtype.Text{Valid: false},
	}, nil
}

func safeYilianyunProviderError(publicMessage string, cause error) error {
	return NewRequestErrorWithCause(http.StatusBadGateway, errors.New(publicMessage), errors.New("yilianyun provider authorization failed"))
}

func newYilianyunAuthorizationResult(authorization db.CloudPrinterProviderAuthorization) YilianyunAuthorizationResult {
	return YilianyunAuthorizationResult{
		AuthorizationID:  authorization.ID,
		MerchantID:       authorization.MerchantID,
		ProviderType:     authorization.ProviderType,
		MachineCode:      authorization.MachineCode,
		Status:           authorization.Status,
		AccessExpiresAt:  authorization.AccessTokenExpiresAt,
		RefreshExpiresAt: authorization.RefreshTokenExpiresAt,
	}
}

func (s *CloudPrinterAuthorizationService) buildYilianyunPrinterParams(authorization db.UpsertCloudPrinterProviderAuthorizationParams, printerName string, printerRole string) db.CreateCloudPrinterParams {
	role := normalizeCloudPrinterRole(printerRole)
	if role == "" {
		role = "front"
	}
	name := strings.TrimSpace(printerName)
	if name == "" {
		name = "易联云 " + authorization.MachineCode
	}

	return db.CreateCloudPrinterParams{
		MerchantID:       authorization.MerchantID,
		PrinterName:      name,
		PrinterSn:        authorization.MachineCode,
		PrinterKey:       "",
		PrinterType:      db.CloudPrinterProviderYilianyun,
		PrinterRole:      role,
		PrintTakeout:     true,
		PrintDineIn:      true,
		PrintReservation: true,
	}
}

func newYilianyunAuthorizationResultWithPrinter(authorization db.CloudPrinterProviderAuthorization, printer db.CloudPrinter) YilianyunAuthorizationResult {
	result := newYilianyunAuthorizationResult(authorization)
	result.Printer = newCloudPrinterDeviceResult(printer)
	return result
}

func newCloudPrinterDeviceResult(printer db.CloudPrinter) CloudPrinterDeviceResult {
	return CloudPrinterDeviceResult{
		ID:               printer.ID,
		MerchantID:       printer.MerchantID,
		PrinterName:      printer.PrinterName,
		PrinterSN:        printer.PrinterSn,
		PrinterKey:       printer.PrinterKey,
		PrinterType:      printer.PrinterType,
		PrinterRole:      printer.PrinterRole,
		PrintTakeout:     printer.PrintTakeout,
		PrintDineIn:      printer.PrintDineIn,
		PrintReservation: printer.PrintReservation,
		IsActive:         printer.IsActive,
		CreatedAt:        printer.CreatedAt,
		UpdatedAt:        printer.UpdatedAt,
	}
}

func normalizeCloudPrinterRole(role string) string {
	return strings.TrimSpace(role)
}

func optionalPgText(value string) pgtype.Text {
	trimmed := strings.TrimSpace(value)
	return pgtype.Text{String: trimmed, Valid: trimmed != ""}
}

func generateCloudPrinterAuthorizationState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
