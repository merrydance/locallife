package token

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPasetoMaker(t *testing.T) {
	maker, err := NewPasetoMaker("12345678901234567890123456789012")
	require.NoError(t, err)

	userID := int64(12345)
	duration := time.Minute

	issuedAt := time.Now()
	expiredAt := issuedAt.Add(duration)

	token, payload, err := maker.CreateToken(userID, duration, TokenTypeAccessToken)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotNil(t, payload)

	payload, err = maker.VerifyToken(token, TokenTypeAccessToken)
	require.NoError(t, err)
	require.NotNil(t, payload)

	require.NotZero(t, payload.ID)
	require.Equal(t, userID, payload.UserID)
	require.Equal(t, byte(TokenTypeAccessToken), byte(payload.Type))
	require.WithinDuration(t, issuedAt, payload.IssuedAt, time.Second)
	require.WithinDuration(t, expiredAt, payload.ExpiredAt, time.Second)
}

func TestExpiredPasetoToken(t *testing.T) {
	maker, err := NewPasetoMaker("12345678901234567890123456789012")
	require.NoError(t, err)

	token, payload, err := maker.CreateToken(int64(12345), -time.Minute, TokenTypeAccessToken)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotNil(t, payload)

	payload, err = maker.VerifyToken(token, TokenTypeAccessToken)
	require.Error(t, err)
	require.EqualError(t, err, ErrExpiredToken.Error())
	require.Nil(t, payload)
}

func TestInvalidPasetoTokenType(t *testing.T) {
	maker, err := NewPasetoMaker("12345678901234567890123456789012")
	require.NoError(t, err)

	token, payload, err := maker.CreateToken(int64(12345), time.Minute, TokenTypeAccessToken)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotNil(t, payload)

	// Try to verify access token as refresh token
	payload, err = maker.VerifyToken(token, TokenTypeRefreshToken)
	require.Error(t, err)
	require.EqualError(t, err, ErrInvalidTokenType.Error())
	require.Nil(t, payload)
}
