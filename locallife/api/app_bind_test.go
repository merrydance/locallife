package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGenerateAppBindCodeRedisUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/app-bind/code", nil)
	require.NoError(t, err)
	ctx.Request = req

	server.generateAppBindCode(ctx)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "绑定码服务暂不可用", resp.Error)
}

func TestVerifyAppBindCodeRedisUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/app-bind/verify", nil)
	require.NoError(t, err)
	ctx.Request = req

	server.verifyAppBindCode(ctx)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "绑定码服务暂不可用", resp.Error)
}

func TestGenerateAppBindCodeRegeneratesWhenUserIndexPointsToMissingCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		require.NoError(t, redisClient.Close())
	})

	const userID int64 = 42
	const merchantID int64 = 1001
	const staleCode = "123456"
	userKey := "app_bind:user:42"
	redisServer.Set(userKey, staleCode)
	redisServer.SetTTL(userKey, appBindCodeTTL)

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), userID).
		Times(1).
		Return([]db.UserRole{{
			UserID:          userID,
			Role:            "merchant_owner",
			RelatedEntityID: pgtype.Int8{Int64: merchantID, Valid: true},
		}}, nil)

	server := newTestServer(t, store)
	server.redisClient = redisClient

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/app-bind/code", nil)
	require.NoError(t, err)
	ctx.Request = req
	ctx.Set(authorizationPayloadKey, &token.Payload{UserID: userID})

	server.generateAppBindCode(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp generateAppBindCodeResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Len(t, resp.Code, appBindCodeLength)
	require.Greater(t, resp.ExpiresIn, 0)

	bindData, err := redisClient.Get(context.Background(), appBindCodePrefix+resp.Code).Result()
	require.NoError(t, err)
	require.Equal(t, "42:1001", bindData)
	indexCode, err := redisClient.Get(context.Background(), userKey).Result()
	require.NoError(t, err)
	require.Equal(t, resp.Code, indexCode)
	ttl, err := redisClient.TTL(context.Background(), appBindCodePrefix+resp.Code).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0))
}
