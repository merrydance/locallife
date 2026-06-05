package logic

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/merrydance/locallife/cloudprint"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestCloudPrinterAuthorizationServiceCreatesYilianyunAuthorizationSession(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	store := &fakeCloudPrinterAuthorizationStore{}
	oauth := &fakeYilianyunAuthorizationOAuthClient{}
	oauth.buildAuthorizeURL = func(state string) (string, error) {
		require.Equal(t, "state-fixed", state)
		return "https://open-api.10ss.net/oauth/authorize?state=" + state, nil
	}
	store.createSession = func(_ context.Context, arg db.CreateCloudPrinterAuthorizationSessionParams) (db.CloudPrinterAuthorizationSession, error) {
		require.Equal(t, "state-fixed", arg.State)
		require.Equal(t, int64(11), arg.MerchantID)
		require.Equal(t, db.CloudPrinterProviderYilianyun, arg.ProviderType)
		require.Equal(t, "前台打印机", arg.PrinterName.String)
		require.Equal(t, "front", arg.PrinterRole.String)
		require.Equal(t, int64(22), arg.CreatedBy.Int64)
		require.WithinDuration(t, now.Add(10*time.Minute), arg.ExpiresAt, time.Second)
		return db.CloudPrinterAuthorizationSession{
			ID:           99,
			State:        arg.State,
			MerchantID:   arg.MerchantID,
			ProviderType: arg.ProviderType,
			PrinterName:  arg.PrinterName,
			PrinterRole:  arg.PrinterRole,
			CreatedBy:    arg.CreatedBy,
			ExpiresAt:    arg.ExpiresAt,
		}, nil
	}
	service := NewCloudPrinterAuthorizationService(store, oauth, noopEncryptor{}, CloudPrinterAuthorizationServiceConfig{
		Now:            func() time.Time { return now },
		StateGenerator: func() (string, error) { return "state-fixed", nil },
	})

	result, err := service.CreateYilianyunAuthorizationSession(context.Background(), CreateYilianyunAuthorizationSessionInput{
		MerchantID:  11,
		CreatedBy:   22,
		PrinterName: " 前台打印机 ",
		PrinterRole: "front",
	})

	require.NoError(t, err)
	require.Equal(t, "state-fixed", result.State)
	require.Equal(t, "https://open-api.10ss.net/oauth/authorize?state=state-fixed", result.AuthorizeURL)
	require.WithinDuration(t, now.Add(10*time.Minute), result.ExpiresAt, time.Second)
}

func TestCloudPrinterAuthorizationServiceCompletesAuthorizationCodeWithEncryptedTokens(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	encryptor, err := util.NewAESEncryptor("12345678901234567890123456789012")
	require.NoError(t, err)
	store := &fakeCloudPrinterAuthorizationStore{}
	oauth := &fakeYilianyunAuthorizationOAuthClient{}
	session := db.CloudPrinterAuthorizationSession{
		ID:           77,
		State:        "state-ok",
		MerchantID:   11,
		ProviderType: db.CloudPrinterProviderYilianyun,
		ExpiresAt:    now.Add(5 * time.Minute),
	}
	store.getSession = func(_ context.Context, state string) (db.CloudPrinterAuthorizationSession, error) {
		require.Equal(t, "state-ok", state)
		return session, nil
	}
	oauth.exchangeAuthorizationCode = func(_ context.Context, code string) (cloudprint.YilianyunAuthorizationToken, error) {
		require.Equal(t, "code-ok", code)
		return cloudprint.YilianyunAuthorizationToken{
			AccessToken:      "access-token-secret",
			RefreshToken:     "refresh-token-secret",
			MachineCode:      "YL-MACHINE-001",
			ExpiresInSeconds: 3600,
		}, nil
	}
	store.authorizeTx = func(_ context.Context, arg db.AuthorizeYilianyunCloudPrinterTxParams) (db.AuthorizeYilianyunCloudPrinterTxResult, error) {
		require.Equal(t, "state-ok", arg.State)
		require.Equal(t, int64(11), arg.Authorization.MerchantID)
		require.Equal(t, db.CloudPrinterProviderYilianyun, arg.Authorization.ProviderType)
		require.Equal(t, "YL-MACHINE-001", arg.Authorization.MachineCode)
		require.NotEqual(t, "access-token-secret", arg.Authorization.AccessTokenCiphertext)
		require.NotEqual(t, "refresh-token-secret", arg.Authorization.RefreshTokenCiphertext)
		require.WithinDuration(t, now.Add(3600*time.Second), arg.Authorization.AccessTokenExpiresAt, time.Second)
		require.WithinDuration(t, now.Add(35*24*time.Hour), arg.Authorization.RefreshTokenExpiresAt, time.Second)
		require.Equal(t, db.CloudPrinterAuthorizationStatusActive, arg.Authorization.Status)
		decryptedAccess, err := encryptor.Decrypt(arg.Authorization.AccessTokenCiphertext)
		require.NoError(t, err)
		decryptedRefresh, err := encryptor.Decrypt(arg.Authorization.RefreshTokenCiphertext)
		require.NoError(t, err)
		require.Equal(t, "access-token-secret", decryptedAccess)
		require.Equal(t, "refresh-token-secret", decryptedRefresh)
		return db.AuthorizeYilianyunCloudPrinterTxResult{
			Session: session,
			Authorization: db.CloudPrinterProviderAuthorization{
				ID:                    101,
				MerchantID:            11,
				ProviderType:          db.CloudPrinterProviderYilianyun,
				MachineCode:           "YL-MACHINE-001",
				Status:                db.CloudPrinterAuthorizationStatusActive,
				CreatedAt:             now,
				UpdatedAt:             now,
				AccessTokenExpiresAt:  now.Add(3600 * time.Second),
				RefreshTokenExpiresAt: now.Add(35 * 24 * time.Hour),
			},
		}, nil
	}
	service := NewCloudPrinterAuthorizationService(store, oauth, encryptor, CloudPrinterAuthorizationServiceConfig{
		Now: func() time.Time { return now },
	})

	result, err := service.CompleteYilianyunAuthorizationCode(context.Background(), CompleteYilianyunAuthorizationCodeInput{
		State: "state-ok",
		Code:  "code-ok",
	})

	require.NoError(t, err)
	require.Equal(t, int64(101), result.AuthorizationID)
	require.Equal(t, "YL-MACHINE-001", result.MachineCode)
	require.Equal(t, db.CloudPrinterAuthorizationStatusActive, result.Status)
}

func TestCloudPrinterAuthorizationServiceProviderErrorUsesSafePublicMessage(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	store := &fakeCloudPrinterAuthorizationStore{}
	oauth := &fakeYilianyunAuthorizationOAuthClient{}
	store.getSession = func(_ context.Context, _ string) (db.CloudPrinterAuthorizationSession, error) {
		return db.CloudPrinterAuthorizationSession{
			ID:           77,
			State:        "state-ok",
			MerchantID:   11,
			ProviderType: db.CloudPrinterProviderYilianyun,
			ExpiresAt:    now.Add(5 * time.Minute),
		}, nil
	}
	oauth.exchangeAuthorizationCode = func(_ context.Context, _ string) (cloudprint.YilianyunAuthorizationToken, error) {
		return cloudprint.YilianyunAuthorizationToken{}, errors.New("provider leaked access-token-secret refresh-token-secret raw payload")
	}
	service := NewCloudPrinterAuthorizationService(store, oauth, noopEncryptor{}, CloudPrinterAuthorizationServiceConfig{
		Now: func() time.Time { return now },
	})

	_, err := service.CompleteYilianyunAuthorizationCode(context.Background(), CompleteYilianyunAuthorizationCodeInput{
		State: "state-ok",
		Code:  "code-ok",
	})

	var reqErr *RequestError
	require.ErrorAs(t, err, &reqErr)
	require.Equal(t, http.StatusBadGateway, reqErr.Status)
	require.NotContains(t, reqErr.Err.Error(), "access-token-secret")
	require.NotContains(t, reqErr.Err.Error(), "refresh-token-secret")
	require.Contains(t, reqErr.Err.Error(), "易联云授权失败")
}

func TestCloudPrinterAuthorizationServiceAuthorizesScannedPrinterWithExactlyOneCredential(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	encryptor, err := util.NewAESEncryptor("12345678901234567890123456789012")
	require.NoError(t, err)
	store := &fakeCloudPrinterAuthorizationStore{}
	oauth := &fakeYilianyunAuthorizationOAuthClient{}
	oauth.authorizeScannedPrinter = func(_ context.Context, input cloudprint.YilianyunScannedPrinterAuthorizationInput) (cloudprint.YilianyunAuthorizationToken, error) {
		require.Equal(t, "YL-MACHINE-002", input.MachineCode)
		require.Equal(t, "qr-secret", input.QRKey)
		require.Empty(t, input.MSign)
		return cloudprint.YilianyunAuthorizationToken{
			AccessToken:      "scan-access-secret",
			RefreshToken:     "scan-refresh-secret",
			MachineCode:      "YL-MACHINE-002",
			ExpiresInSeconds: 7200,
		}, nil
	}
	store.upsertAuthorization = func(_ context.Context, arg db.UpsertCloudPrinterProviderAuthorizationParams) (db.CloudPrinterProviderAuthorization, error) {
		require.Equal(t, int64(11), arg.MerchantID)
		require.Equal(t, "YL-MACHINE-002", arg.MachineCode)
		decryptedAccess, err := encryptor.Decrypt(arg.AccessTokenCiphertext)
		require.NoError(t, err)
		require.Equal(t, "scan-access-secret", decryptedAccess)
		return db.CloudPrinterProviderAuthorization{
			ID:                    102,
			MerchantID:            11,
			ProviderType:          db.CloudPrinterProviderYilianyun,
			MachineCode:           "YL-MACHINE-002",
			Status:                db.CloudPrinterAuthorizationStatusActive,
			AccessTokenExpiresAt:  arg.AccessTokenExpiresAt,
			RefreshTokenExpiresAt: arg.RefreshTokenExpiresAt,
		}, nil
	}
	service := NewCloudPrinterAuthorizationService(store, oauth, encryptor, CloudPrinterAuthorizationServiceConfig{
		Now: func() time.Time { return now },
	})

	result, err := service.AuthorizeScannedYilianyunPrinter(context.Background(), AuthorizeScannedYilianyunPrinterInput{
		MerchantID:  11,
		MachineCode: "YL-MACHINE-002",
		QRKey:       "qr-secret",
	})

	require.NoError(t, err)
	require.Equal(t, int64(102), result.AuthorizationID)
	require.Equal(t, "YL-MACHINE-002", result.MachineCode)

	_, err = service.AuthorizeScannedYilianyunPrinter(context.Background(), AuthorizeScannedYilianyunPrinterInput{
		MerchantID:  11,
		MachineCode: "YL-MACHINE-002",
		QRKey:       "qr-secret",
		MSign:       "msign-secret",
	})
	var reqErr *RequestError
	require.ErrorAs(t, err, &reqErr)
	require.Equal(t, http.StatusBadRequest, reqErr.Status)
}

type fakeCloudPrinterAuthorizationStore struct {
	createSession       func(context.Context, db.CreateCloudPrinterAuthorizationSessionParams) (db.CloudPrinterAuthorizationSession, error)
	getSession          func(context.Context, string) (db.CloudPrinterAuthorizationSession, error)
	authorizeTx         func(context.Context, db.AuthorizeYilianyunCloudPrinterTxParams) (db.AuthorizeYilianyunCloudPrinterTxResult, error)
	upsertAuthorization func(context.Context, db.UpsertCloudPrinterProviderAuthorizationParams) (db.CloudPrinterProviderAuthorization, error)
}

func (s *fakeCloudPrinterAuthorizationStore) CreateCloudPrinterAuthorizationSession(ctx context.Context, arg db.CreateCloudPrinterAuthorizationSessionParams) (db.CloudPrinterAuthorizationSession, error) {
	if s.createSession == nil {
		return db.CloudPrinterAuthorizationSession{}, errors.New("unexpected CreateCloudPrinterAuthorizationSession")
	}
	return s.createSession(ctx, arg)
}

func (s *fakeCloudPrinterAuthorizationStore) GetActiveCloudPrinterAuthorizationSessionForUpdate(ctx context.Context, state string) (db.CloudPrinterAuthorizationSession, error) {
	if s.getSession == nil {
		return db.CloudPrinterAuthorizationSession{}, errors.New("unexpected GetActiveCloudPrinterAuthorizationSessionForUpdate")
	}
	return s.getSession(ctx, state)
}

func (s *fakeCloudPrinterAuthorizationStore) AuthorizeYilianyunCloudPrinterTx(ctx context.Context, arg db.AuthorizeYilianyunCloudPrinterTxParams) (db.AuthorizeYilianyunCloudPrinterTxResult, error) {
	if s.authorizeTx == nil {
		return db.AuthorizeYilianyunCloudPrinterTxResult{}, errors.New("unexpected AuthorizeYilianyunCloudPrinterTx")
	}
	return s.authorizeTx(ctx, arg)
}

func (s *fakeCloudPrinterAuthorizationStore) UpsertCloudPrinterProviderAuthorization(ctx context.Context, arg db.UpsertCloudPrinterProviderAuthorizationParams) (db.CloudPrinterProviderAuthorization, error) {
	if s.upsertAuthorization == nil {
		return db.CloudPrinterProviderAuthorization{}, errors.New("unexpected UpsertCloudPrinterProviderAuthorization")
	}
	return s.upsertAuthorization(ctx, arg)
}

type fakeYilianyunAuthorizationOAuthClient struct {
	buildAuthorizeURL         func(string) (string, error)
	exchangeAuthorizationCode func(context.Context, string) (cloudprint.YilianyunAuthorizationToken, error)
	authorizeScannedPrinter   func(context.Context, cloudprint.YilianyunScannedPrinterAuthorizationInput) (cloudprint.YilianyunAuthorizationToken, error)
}

func (c *fakeYilianyunAuthorizationOAuthClient) BuildAuthorizeURL(state string) (string, error) {
	if c.buildAuthorizeURL == nil {
		return "", errors.New("unexpected BuildAuthorizeURL")
	}
	return c.buildAuthorizeURL(state)
}

func (c *fakeYilianyunAuthorizationOAuthClient) ExchangeAuthorizationCode(ctx context.Context, code string) (cloudprint.YilianyunAuthorizationToken, error) {
	if c.exchangeAuthorizationCode == nil {
		return cloudprint.YilianyunAuthorizationToken{}, errors.New("unexpected ExchangeAuthorizationCode")
	}
	return c.exchangeAuthorizationCode(ctx, code)
}

func (c *fakeYilianyunAuthorizationOAuthClient) AuthorizeScannedPrinter(ctx context.Context, input cloudprint.YilianyunScannedPrinterAuthorizationInput) (cloudprint.YilianyunAuthorizationToken, error) {
	if c.authorizeScannedPrinter == nil {
		return cloudprint.YilianyunAuthorizationToken{}, errors.New("unexpected AuthorizeScannedPrinter")
	}
	return c.authorizeScannedPrinter(ctx, input)
}

type noopEncryptor struct{}

func (noopEncryptor) Encrypt(plaintext string) (string, error)  { return "encrypted:" + plaintext, nil }
func (noopEncryptor) Decrypt(ciphertext string) (string, error) { return ciphertext, nil }
