package db

import (
	"context"
	"testing"
	"time"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomWechatAccessToken(t *testing.T, appType string) WechatAccessToken {
	arg := UpsertWechatAccessTokenParams{
		AppType:     appType,
		AccessToken: util.RandomString(64),
		ExpiresAt:   time.Now().Add(2 * time.Hour).UTC().Truncate(time.Microsecond),
	}

	token, err := testStore.UpsertWechatAccessToken(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	require.Equal(t, arg.AppType, token.AppType)
	require.Equal(t, arg.AccessToken, token.AccessToken)
	require.WithinDuration(t, arg.ExpiresAt, token.ExpiresAt, time.Second)

	require.NotZero(t, token.ID)
	require.NotZero(t, token.CreatedAt)

	return token
}

func TestUpsertWechatAccessToken(t *testing.T) {
	createRandomWechatAccessToken(t, "mini_program_"+util.RandomString(6))
}

func TestUpsertWechatAccessTokenUpdate(t *testing.T) {
	appType := "mini_program_" + util.RandomString(6)

	// Create initial token
	token1 := createRandomWechatAccessToken(t, appType)

	// Wait a bit to ensure different created_at
	time.Sleep(10 * time.Millisecond)

	// Upsert with same app_type but new access_token
	arg := UpsertWechatAccessTokenParams{
		AppType:     appType,
		AccessToken: util.RandomString(64),
		ExpiresAt:   time.Now().Add(3 * time.Hour).UTC().Truncate(time.Microsecond),
	}

	token2, err := testStore.UpsertWechatAccessToken(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, token2)

	// Should be the same record (same ID)
	require.Equal(t, token1.ID, token2.ID)
	require.Equal(t, token1.AppType, token2.AppType)

	// But with updated values
	require.Equal(t, arg.AccessToken, token2.AccessToken)
	require.NotEqual(t, token1.AccessToken, token2.AccessToken)
	require.WithinDuration(t, arg.ExpiresAt, token2.ExpiresAt, time.Second)

	// created_at should be updated by the ON CONFLICT clause
	require.True(t, token2.CreatedAt.After(token1.CreatedAt) ||
		token2.CreatedAt.Equal(token1.CreatedAt))
}

func TestGetWechatAccessToken(t *testing.T) {
	appType := "mini_program_" + util.RandomString(6)
	token1 := createRandomWechatAccessToken(t, appType)

	token2, err := testStore.GetWechatAccessToken(context.Background(), appType)
	require.NoError(t, err)
	require.NotEmpty(t, token2)

	require.Equal(t, token1.ID, token2.ID)
	require.Equal(t, token1.AppType, token2.AppType)
	require.Equal(t, token1.AccessToken, token2.AccessToken)
	require.WithinDuration(t, token1.ExpiresAt, token2.ExpiresAt, time.Second)
	require.WithinDuration(t, token1.CreatedAt, token2.CreatedAt, time.Second)
}

func TestGetWechatAccessTokenNotFound(t *testing.T) {
	appType := "non_existent_" + util.RandomString(6)

	token, err := testStore.GetWechatAccessToken(context.Background(), appType)
	require.Error(t, err)
	require.Empty(t, token)
}

func TestWechatAccessTokenExpiry(t *testing.T) {
	appType := "mini_program_" + util.RandomString(6)

	// Create token that expires in 1 hour
	expiresAt := time.Now().Add(1 * time.Hour).UTC().Truncate(time.Microsecond)
	arg := UpsertWechatAccessTokenParams{
		AppType:     appType,
		AccessToken: util.RandomString(64),
		ExpiresAt:   expiresAt,
	}

	token, err := testStore.UpsertWechatAccessToken(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Verify token is not expired
	require.True(t, token.ExpiresAt.After(time.Now()))

	// Create an expired token for different app type
	expiredAppType := "expired_" + util.RandomString(6)
	expiredArg := UpsertWechatAccessTokenParams{
		AppType:     expiredAppType,
		AccessToken: util.RandomString(64),
		ExpiresAt:   time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Microsecond), // Already expired
	}

	expiredToken, err := testStore.UpsertWechatAccessToken(context.Background(), expiredArg)
	require.NoError(t, err)
	require.NotEmpty(t, expiredToken)

	// Verify token is expired
	require.True(t, expiredToken.ExpiresAt.Before(time.Now()))
}
