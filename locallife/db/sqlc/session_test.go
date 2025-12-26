package db

import (
	"context"
	"testing"
	"time"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomSession(t *testing.T, user User) Session {
	arg := CreateSessionParams{
		UserID:                user.ID,
		AccessToken:           util.RandomString(32),
		RefreshToken:          util.RandomString(32),
		AccessTokenExpiresAt:  time.Now().Add(15 * time.Minute),
		RefreshTokenExpiresAt: time.Now().Add(24 * time.Hour),
		UserAgent:             "test-agent",
		ClientIp:              "127.0.0.1",
	}

	session, err := testStore.CreateSession(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, session)

	require.NotZero(t, session.ID)
	require.Equal(t, arg.UserID, session.UserID)
	require.Equal(t, arg.RefreshToken, session.RefreshToken)
	require.False(t, session.IsRevoked)
	require.NotZero(t, session.CreatedAt)

	return session
}

func TestCreateSession(t *testing.T) {
	user := createRandomUser(t)
	createRandomSession(t, user)
}

func TestGetSession(t *testing.T) {
	user := createRandomUser(t)
	session1 := createRandomSession(t, user)

	session2, err := testStore.GetSession(context.Background(), session1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, session2)

	require.Equal(t, session1.ID, session2.ID)
	require.Equal(t, session1.UserID, session2.UserID)
	require.Equal(t, session1.RefreshToken, session2.RefreshToken)
}

func TestGetSessionByRefreshToken(t *testing.T) {
	user := createRandomUser(t)
	session1 := createRandomSession(t, user)

	session2, err := testStore.GetSessionByRefreshToken(context.Background(), session1.RefreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, session2)

	require.Equal(t, session1.ID, session2.ID)
	require.Equal(t, session1.RefreshToken, session2.RefreshToken)
}

func TestRevokeSession(t *testing.T) {
	user := createRandomUser(t)
	session := createRandomSession(t, user)

	revokedSession, err := testStore.RevokeSession(context.Background(), session.ID)
	require.NoError(t, err)
	require.True(t, revokedSession.IsRevoked)

	// 再次获取验证
	session2, err := testStore.GetSession(context.Background(), session.ID)
	require.NoError(t, err)
	require.True(t, session2.IsRevoked)
}
