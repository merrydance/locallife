package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestCloudPrinterAuthorizationSessionConsumedOnce(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	expiresAt := time.Now().Add(10 * time.Minute).UTC()

	session, err := testStore.CreateCloudPrinterAuthorizationSession(context.Background(), CreateCloudPrinterAuthorizationSessionParams{
		State:        "yly-state-" + util.RandomString(24),
		MerchantID:   merchant.ID,
		ProviderType: CloudPrinterProviderYilianyun,
		PrinterName:  pgtype.Text{String: "前台易联云", Valid: true},
		PrinterRole:  pgtype.Text{String: "front", Valid: true},
		CreatedBy:    pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
		ExpiresAt:    expiresAt,
	})
	require.NoError(t, err)
	require.Equal(t, merchant.ID, session.MerchantID)
	require.Equal(t, CloudPrinterProviderYilianyun, session.ProviderType)
	require.False(t, session.ConsumedAt.Valid)
	require.WithinDuration(t, expiresAt, session.ExpiresAt, time.Second)

	loaded, err := testStore.GetActiveCloudPrinterAuthorizationSessionForUpdate(context.Background(), session.State)
	require.NoError(t, err)
	require.Equal(t, session.ID, loaded.ID)

	consumed, err := testStore.ConsumeCloudPrinterAuthorizationSession(context.Background(), ConsumeCloudPrinterAuthorizationSessionParams{
		ID:         session.ID,
		ConsumedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	require.NoError(t, err)
	require.True(t, consumed.ConsumedAt.Valid)

	_, err = testStore.ConsumeCloudPrinterAuthorizationSession(context.Background(), ConsumeCloudPrinterAuthorizationSessionParams{
		ID:         session.ID,
		ConsumedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	require.ErrorIs(t, err, ErrRecordNotFound)

	_, err = testStore.GetActiveCloudPrinterAuthorizationSessionForUpdate(context.Background(), session.State)
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestCloudPrinterAuthorizationSessionCannotConsumeExpiredState(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	session, err := testStore.CreateCloudPrinterAuthorizationSession(context.Background(), CreateCloudPrinterAuthorizationSessionParams{
		State:        "yly-state-expired-" + util.RandomString(24),
		MerchantID:   merchant.ID,
		ProviderType: CloudPrinterProviderYilianyun,
		PrinterName:  pgtype.Text{String: "过期易联云", Valid: true},
		PrinterRole:  pgtype.Text{String: "front", Valid: true},
		CreatedBy:    pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
		ExpiresAt:    time.Now().Add(-time.Minute).UTC(),
	})
	require.NoError(t, err)

	_, err = testStore.GetActiveCloudPrinterAuthorizationSessionForUpdate(context.Background(), session.State)
	require.ErrorIs(t, err, ErrRecordNotFound)

	_, err = testStore.ConsumeCloudPrinterAuthorizationSession(context.Background(), ConsumeCloudPrinterAuthorizationSessionParams{
		ID:         session.ID,
		ConsumedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestAuthorizeYilianyunCloudPrinterTxConsumesStateAndUpsertsAuthorization(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	state := "yly-state-tx-" + util.RandomString(24)
	session, err := testStore.CreateCloudPrinterAuthorizationSession(context.Background(), CreateCloudPrinterAuthorizationSessionParams{
		State:        state,
		MerchantID:   merchant.ID,
		ProviderType: CloudPrinterProviderYilianyun,
		PrinterName:  pgtype.Text{String: "事务易联云", Valid: true},
		PrinterRole:  pgtype.Text{String: "front", Valid: true},
		CreatedBy:    pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
		ExpiresAt:    time.Now().Add(10 * time.Minute).UTC(),
	})
	require.NoError(t, err)

	consumedAt := time.Now().UTC()
	result, err := testStore.AuthorizeYilianyunCloudPrinterTx(context.Background(), AuthorizeYilianyunCloudPrinterTxParams{
		State: state,
		Authorization: UpsertCloudPrinterProviderAuthorizationParams{
			MerchantID:               merchant.ID,
			ProviderType:             CloudPrinterProviderYilianyun,
			MachineCode:              "YL-TX-" + util.RandomString(12),
			AccessTokenCiphertext:    "ciphertext-access",
			RefreshTokenCiphertext:   "ciphertext-refresh",
			AccessTokenExpiresAt:     time.Now().Add(30 * 24 * time.Hour).UTC(),
			RefreshTokenExpiresAt:    time.Now().Add(35 * 24 * time.Hour).UTC(),
			Status:                   CloudPrinterAuthorizationStatusActive,
			RefreshFailureCount:      0,
			RefreshLastAttemptedAt:   pgtype.Timestamptz{Valid: false},
			LastProviderError:        pgtype.Text{Valid: false},
			AuthorizedCloudPrinterID: pgtype.Int8{Valid: false},
		},
		ConsumedAt: consumedAt,
	})
	require.NoError(t, err)
	require.Equal(t, session.ID, result.Session.ID)
	require.True(t, result.Session.ConsumedAt.Valid)
	require.WithinDuration(t, consumedAt, result.Session.ConsumedAt.Time, time.Second)
	require.Equal(t, merchant.ID, result.Authorization.MerchantID)
	require.Equal(t, CloudPrinterAuthorizationStatusActive, result.Authorization.Status)

	_, err = testStore.AuthorizeYilianyunCloudPrinterTx(context.Background(), AuthorizeYilianyunCloudPrinterTxParams{
		State: state,
		Authorization: UpsertCloudPrinterProviderAuthorizationParams{
			MerchantID:               merchant.ID,
			ProviderType:             CloudPrinterProviderYilianyun,
			MachineCode:              result.Authorization.MachineCode,
			AccessTokenCiphertext:    "ciphertext-access-rotated",
			RefreshTokenCiphertext:   "ciphertext-refresh-rotated",
			AccessTokenExpiresAt:     time.Now().Add(30 * 24 * time.Hour).UTC(),
			RefreshTokenExpiresAt:    time.Now().Add(35 * 24 * time.Hour).UTC(),
			Status:                   CloudPrinterAuthorizationStatusActive,
			RefreshFailureCount:      0,
			RefreshLastAttemptedAt:   pgtype.Timestamptz{Valid: false},
			LastProviderError:        pgtype.Text{Valid: false},
			AuthorizedCloudPrinterID: pgtype.Int8{Valid: false},
		},
		ConsumedAt: time.Now().UTC(),
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestUpsertCloudPrinterProviderAuthorizationStoresEncryptedTokens(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	encryptor, err := util.NewAESEncryptor("12345678901234567890123456789012")
	require.NoError(t, err)

	accessCiphertext, err := util.EncryptSensitiveField(encryptor, "access-token-plain")
	require.NoError(t, err)
	refreshCiphertext, err := util.EncryptSensitiveField(encryptor, "refresh-token-plain")
	require.NoError(t, err)
	require.NotEqual(t, "access-token-plain", accessCiphertext)
	require.NotEqual(t, "refresh-token-plain", refreshCiphertext)

	accessExpiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
	refreshExpiresAt := time.Now().Add(35 * 24 * time.Hour).UTC()
	authorization, err := testStore.UpsertCloudPrinterProviderAuthorization(context.Background(), UpsertCloudPrinterProviderAuthorizationParams{
		MerchantID:               merchant.ID,
		ProviderType:             CloudPrinterProviderYilianyun,
		MachineCode:              "YL-" + util.RandomString(12),
		AccessTokenCiphertext:    accessCiphertext,
		RefreshTokenCiphertext:   refreshCiphertext,
		AccessTokenExpiresAt:     accessExpiresAt,
		RefreshTokenExpiresAt:    refreshExpiresAt,
		Status:                   CloudPrinterAuthorizationStatusActive,
		RefreshFailureCount:      0,
		RefreshLastAttemptedAt:   pgtype.Timestamptz{Valid: false},
		LastProviderError:        pgtype.Text{Valid: false},
		AuthorizedCloudPrinterID: pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)
	require.Equal(t, merchant.ID, authorization.MerchantID)
	require.Equal(t, CloudPrinterProviderYilianyun, authorization.ProviderType)
	require.Equal(t, CloudPrinterAuthorizationStatusActive, authorization.Status)
	require.Equal(t, accessCiphertext, authorization.AccessTokenCiphertext)
	require.Equal(t, refreshCiphertext, authorization.RefreshTokenCiphertext)
	require.NotContains(t, authorization.AccessTokenCiphertext, "access-token-plain")
	require.NotContains(t, authorization.RefreshTokenCiphertext, "refresh-token-plain")

	decryptedAccess, err := util.DecryptSensitiveField(encryptor, authorization.AccessTokenCiphertext)
	require.NoError(t, err)
	decryptedRefresh, err := util.DecryptSensitiveField(encryptor, authorization.RefreshTokenCiphertext)
	require.NoError(t, err)
	require.Equal(t, "access-token-plain", decryptedAccess)
	require.Equal(t, "refresh-token-plain", decryptedRefresh)

	rotatedAccessCiphertext, err := util.EncryptSensitiveField(encryptor, "access-token-rotated")
	require.NoError(t, err)
	rotated, err := testStore.UpsertCloudPrinterProviderAuthorization(context.Background(), UpsertCloudPrinterProviderAuthorizationParams{
		MerchantID:               merchant.ID,
		ProviderType:             authorization.ProviderType,
		MachineCode:              authorization.MachineCode,
		AccessTokenCiphertext:    rotatedAccessCiphertext,
		RefreshTokenCiphertext:   refreshCiphertext,
		AccessTokenExpiresAt:     accessExpiresAt.Add(24 * time.Hour),
		RefreshTokenExpiresAt:    refreshExpiresAt,
		Status:                   CloudPrinterAuthorizationStatusActive,
		RefreshFailureCount:      0,
		RefreshLastAttemptedAt:   pgtype.Timestamptz{Valid: false},
		LastProviderError:        pgtype.Text{Valid: false},
		AuthorizedCloudPrinterID: pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)
	require.Equal(t, authorization.ID, rotated.ID)
	require.Equal(t, rotatedAccessCiphertext, rotated.AccessTokenCiphertext)

	byMachine, err := testStore.GetCloudPrinterProviderAuthorizationByMerchantAndMachineCode(context.Background(), GetCloudPrinterProviderAuthorizationByMerchantAndMachineCodeParams{
		MerchantID:   merchant.ID,
		ProviderType: authorization.ProviderType,
		MachineCode:  authorization.MachineCode,
	})
	require.NoError(t, err)
	require.Equal(t, rotated.ID, byMachine.ID)

	list, err := testStore.ListCloudPrinterProviderAuthorizationsByMerchant(context.Background(), ListCloudPrinterProviderAuthorizationsByMerchantParams{
		MerchantID:   merchant.ID,
		ProviderType: CloudPrinterProviderYilianyun,
	})
	require.NoError(t, err)
	require.NotEmpty(t, list)
	require.Equal(t, rotated.ID, list[0].ID)
}

func TestGetCloudPrinterProviderAuthorizationByMerchantAndMachineCodeIsMerchantScoped(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	otherMerchant := createRandomMerchantForTest(t)
	machineCode := "YL-" + util.RandomString(12)

	created, err := testStore.UpsertCloudPrinterProviderAuthorization(context.Background(), UpsertCloudPrinterProviderAuthorizationParams{
		MerchantID:               merchant.ID,
		ProviderType:             CloudPrinterProviderYilianyun,
		MachineCode:              machineCode,
		AccessTokenCiphertext:    "ciphertext-access",
		RefreshTokenCiphertext:   "ciphertext-refresh",
		AccessTokenExpiresAt:     time.Now().Add(30 * 24 * time.Hour).UTC(),
		RefreshTokenExpiresAt:    time.Now().Add(35 * 24 * time.Hour).UTC(),
		Status:                   CloudPrinterAuthorizationStatusActive,
		RefreshFailureCount:      0,
		RefreshLastAttemptedAt:   pgtype.Timestamptz{Valid: false},
		LastProviderError:        pgtype.Text{Valid: false},
		AuthorizedCloudPrinterID: pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)

	found, err := testStore.GetCloudPrinterProviderAuthorizationByMerchantAndMachineCode(context.Background(), GetCloudPrinterProviderAuthorizationByMerchantAndMachineCodeParams{
		MerchantID:   merchant.ID,
		ProviderType: CloudPrinterProviderYilianyun,
		MachineCode:  machineCode,
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)

	_, err = testStore.GetCloudPrinterProviderAuthorizationByMerchantAndMachineCode(context.Background(), GetCloudPrinterProviderAuthorizationByMerchantAndMachineCodeParams{
		MerchantID:   otherMerchant.ID,
		ProviderType: CloudPrinterProviderYilianyun,
		MachineCode:  machineCode,
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestUpsertCloudPrinterProviderAuthorizationRejectsMerchantTakeover(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	otherMerchant := createRandomMerchantForTest(t)
	machineCode := "YL-" + util.RandomString(12)

	created, err := testStore.UpsertCloudPrinterProviderAuthorization(context.Background(), UpsertCloudPrinterProviderAuthorizationParams{
		MerchantID:               merchant.ID,
		ProviderType:             CloudPrinterProviderYilianyun,
		MachineCode:              machineCode,
		AccessTokenCiphertext:    "ciphertext-access-1",
		RefreshTokenCiphertext:   "ciphertext-refresh-1",
		AccessTokenExpiresAt:     time.Now().Add(30 * 24 * time.Hour).UTC(),
		RefreshTokenExpiresAt:    time.Now().Add(35 * 24 * time.Hour).UTC(),
		Status:                   CloudPrinterAuthorizationStatusActive,
		RefreshFailureCount:      0,
		RefreshLastAttemptedAt:   pgtype.Timestamptz{Valid: false},
		LastProviderError:        pgtype.Text{Valid: false},
		AuthorizedCloudPrinterID: pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)

	_, err = testStore.UpsertCloudPrinterProviderAuthorization(context.Background(), UpsertCloudPrinterProviderAuthorizationParams{
		MerchantID:               otherMerchant.ID,
		ProviderType:             CloudPrinterProviderYilianyun,
		MachineCode:              machineCode,
		AccessTokenCiphertext:    "ciphertext-access-2",
		RefreshTokenCiphertext:   "ciphertext-refresh-2",
		AccessTokenExpiresAt:     time.Now().Add(30 * 24 * time.Hour).UTC(),
		RefreshTokenExpiresAt:    time.Now().Add(35 * 24 * time.Hour).UTC(),
		Status:                   CloudPrinterAuthorizationStatusActive,
		RefreshFailureCount:      0,
		RefreshLastAttemptedAt:   pgtype.Timestamptz{Valid: false},
		LastProviderError:        pgtype.Text{Valid: false},
		AuthorizedCloudPrinterID: pgtype.Int8{Valid: false},
	})
	require.ErrorIs(t, err, ErrRecordNotFound)

	unchanged, err := testStore.GetCloudPrinterProviderAuthorizationByMerchantAndMachineCode(context.Background(), GetCloudPrinterProviderAuthorizationByMerchantAndMachineCodeParams{
		MerchantID:   merchant.ID,
		ProviderType: CloudPrinterProviderYilianyun,
		MachineCode:  machineCode,
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, unchanged.ID)
	require.Equal(t, merchant.ID, unchanged.MerchantID)
	require.Equal(t, "ciphertext-access-1", unchanged.AccessTokenCiphertext)
}

func TestUpsertCloudPrinterProviderAuthorizationRejectsPrinterFromOtherMerchant(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	otherMerchant := createRandomMerchantForTest(t)
	otherPrinter, err := testStore.CreateCloudPrinter(context.Background(), CreateCloudPrinterParams{
		MerchantID:       otherMerchant.ID,
		PrinterName:      "other merchant yilianyun",
		PrinterSn:        "YL-OTHER-" + util.RandomString(12),
		PrinterKey:       "not-a-token",
		PrinterType:      CloudPrinterProviderYilianyun,
		PrinterRole:      "front",
		PrintTakeout:     true,
		PrintDineIn:      true,
		PrintReservation: true,
	})
	require.NoError(t, err)

	_, err = testStore.UpsertCloudPrinterProviderAuthorization(context.Background(), UpsertCloudPrinterProviderAuthorizationParams{
		MerchantID:               merchant.ID,
		ProviderType:             CloudPrinterProviderYilianyun,
		MachineCode:              "YL-" + util.RandomString(12),
		AccessTokenCiphertext:    "ciphertext-access",
		RefreshTokenCiphertext:   "ciphertext-refresh",
		AccessTokenExpiresAt:     time.Now().Add(30 * 24 * time.Hour).UTC(),
		RefreshTokenExpiresAt:    time.Now().Add(35 * 24 * time.Hour).UTC(),
		Status:                   CloudPrinterAuthorizationStatusActive,
		RefreshFailureCount:      0,
		RefreshLastAttemptedAt:   pgtype.Timestamptz{Valid: false},
		LastProviderError:        pgtype.Text{Valid: false},
		AuthorizedCloudPrinterID: pgtype.Int8{Int64: otherPrinter.ID, Valid: true},
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}
