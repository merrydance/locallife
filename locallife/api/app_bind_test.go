package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestVerifyAppBindCodeRejectsChangedMerchantRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		require.NoError(t, redisClient.Close())
	})

	const userID int64 = 42
	const originalMerchantID int64 = 1001
	const currentMerchantID int64 = 2002
	const code = "123456"

	codeKey := appBindCodePrefix + code
	userKey := "app_bind:user:42"
	redisServer.Set(codeKey, fmt.Sprintf("%d:%d", userID, originalMerchantID))
	redisServer.SetTTL(codeKey, appBindCodeTTL)
	redisServer.Set(userKey, code)
	redisServer.SetTTL(userKey, appBindCodeTTL)

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), userID).
		Times(1).
		Return([]db.UserRole{{
			UserID:          userID,
			Role:            "merchant_owner",
			Status:          "active",
			RelatedEntityID: pgtype.Int8{Int64: currentMerchantID, Valid: true},
		}}, nil)

	server := newTestServer(t, store)
	server.redisClient = redisClient

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := strings.NewReader(`{"code":"123456","device_id":"android-device-1"}`)
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/app-bind/verify", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req

	server.verifyAppBindCode(ctx)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "用户不再具有该商户权限", resp.Error)
	require.False(t, redisServer.Exists(codeKey))
	require.False(t, redisServer.Exists(userKey))
}

func TestAppBindCodeGenerateVerifyCreatesSessionAndPreventsReuse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		require.NoError(t, redisClient.Close())
	})

	const userID int64 = 42
	const merchantID int64 = 1001

	user := db.User{
		ID:           userID,
		WechatOpenid: "openid-app-bind",
		FullName:     "App Bind Owner",
		CreatedAt:    time.Now(),
	}
	merchant := db.Merchant{
		ID:          merchantID,
		OwnerUserID: userID,
		Name:        "App Bind Merchant",
		Status:      "active",
	}
	roles := []db.UserRole{{
		UserID:          userID,
		Role:            "merchant_owner",
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: merchantID, Valid: true},
	}}

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), userID).
		Times(3).
		Return(roles, nil)
	store.EXPECT().
		GetUser(gomock.Any(), userID).
		Times(1).
		Return(user, nil)
	store.EXPECT().
		CreateSession(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateSessionParams) (db.Session, error) {
			require.Equal(t, userID, arg.UserID)
			require.True(t, strings.HasPrefix(arg.UserAgent, appBindSessionUserAgentPrefix))
			return db.Session{
				ID:                    88,
				UserID:                userID,
				AccessToken:           arg.AccessToken,
				RefreshToken:          arg.RefreshToken,
				AccessTokenExpiresAt:  arg.AccessTokenExpiresAt,
				RefreshTokenExpiresAt: arg.RefreshTokenExpiresAt,
				UserAgent:             arg.UserAgent,
				ClientIp:              arg.ClientIp,
			}, nil
		})
	store.EXPECT().
		ListMerchantsByStaff(gomock.Any(), userID).
		Times(1).
		Return([]db.Merchant{}, nil)
	store.EXPECT().
		ListMerchantsByOwner(gomock.Any(), userID).
		Times(1).
		Return([]db.Merchant{merchant}, nil)

	server := newTestServer(t, store)
	server.redisClient = redisClient

	generateRecorder := httptest.NewRecorder()
	generateCtx, _ := gin.CreateTestContext(generateRecorder)
	generateReq, err := http.NewRequest(http.MethodPost, "/v1/auth/app-bind/code", nil)
	require.NoError(t, err)
	generateCtx.Request = generateReq
	generateCtx.Set(authorizationPayloadKey, &token.Payload{UserID: userID})

	server.generateAppBindCode(generateCtx)

	require.Equal(t, http.StatusOK, generateRecorder.Code)
	var generated generateAppBindCodeResponse
	require.NoError(t, json.Unmarshal(generateRecorder.Body.Bytes(), &generated))
	require.Len(t, generated.Code, appBindCodeLength)
	require.Greater(t, generated.ExpiresIn, 0)

	codeKey := appBindCodePrefix + generated.Code
	userKey := fmt.Sprintf("%suser:%d", appBindCodePrefix, userID)
	require.True(t, redisServer.Exists(codeKey))
	require.True(t, redisServer.Exists(userKey))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := strings.NewReader(fmt.Sprintf(`{"code":%q,"device_id":"android-device-1"}`, generated.Code))
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/app-bind/verify", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "LocalLifeMerchantApp/1.0")
	ctx.Request = req

	server.verifyAppBindCode(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp verifyAppBindCodeResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, int64(88), resp.SessionID)
	require.NotEmpty(t, resp.AccessToken)
	require.NotEmpty(t, resp.RefreshToken)
	require.Equal(t, userID, resp.User.ID)
	require.False(t, redisServer.Exists(codeKey))
	require.False(t, redisServer.Exists(userKey))

	reuseRecorder := httptest.NewRecorder()
	reuseCtx, _ := gin.CreateTestContext(reuseRecorder)
	reuseReq, err := http.NewRequest(http.MethodPost, "/v1/auth/app-bind/verify", strings.NewReader(fmt.Sprintf(`{"code":%q,"device_id":"android-device-1"}`, generated.Code)))
	require.NoError(t, err)
	reuseReq.Header.Set("Content-Type", "application/json")
	reuseCtx.Request = reuseReq

	server.verifyAppBindCode(reuseCtx)

	require.Equal(t, http.StatusBadRequest, reuseRecorder.Code)
	var reuseResp ErrorResponse
	require.NoError(t, json.Unmarshal(reuseRecorder.Body.Bytes(), &reuseResp))
	require.Equal(t, "绑定码无效或已过期", reuseResp.Error)
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
