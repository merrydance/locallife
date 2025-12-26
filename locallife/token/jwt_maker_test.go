package token

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestJWTMaker(t *testing.T) {
	maker, err := NewJWTMaker("12345678901234567890123456789012")
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

func TestExpiredJWTToken(t *testing.T) {
	maker, err := NewJWTMaker("12345678901234567890123456789012")
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

func TestInvalidJWTTokenAlgNone(t *testing.T) {
	payload, err := NewPayload(int64(12345), time.Minute, TokenTypeAccessToken)
	require.NoError(t, err)

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodNone, payload)
	token, err := jwtToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	maker, err := NewJWTMaker("12345678901234567890123456789012")
	require.NoError(t, err)

	payload, err = maker.VerifyToken(token, TokenTypeAccessToken)
	require.Error(t, err)
	require.EqualError(t, err, ErrInvalidToken.Error())
	require.Nil(t, payload)
}

func TestInvalidJWTTokenType(t *testing.T) {
	maker, err := NewJWTMaker("12345678901234567890123456789012")
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
