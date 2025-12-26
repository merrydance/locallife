package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRenewAccessTokenAPI(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		setupBody     func(t *testing.T, tokenMaker token.Maker) (map[string]interface{}, string)
		buildStubs    func(store *mockdb.MockStore, refreshToken string, userID int64)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder, tokenMaker token.Maker, userID int64)
	}{
		{
			name: "OK",
			setupBody: func(t *testing.T, tokenMaker token.Maker) (map[string]interface{}, string) {
				refreshToken, _, err := tokenMaker.CreateToken(user.ID, 24*time.Hour, token.TokenTypeRefreshToken)
				require.NoError(t, err)
				return map[string]interface{}{
					"refresh_token": refreshToken,
				}, refreshToken
			},
			buildStubs: func(store *mockdb.MockStore, refreshToken string, userID int64) {
				session := db.Session{
					ID:                    util.RandomInt(1, 1000),
					UserID:                userID,
					RefreshToken:          refreshToken,
					RefreshTokenExpiresAt: time.Now().Add(24 * time.Hour),
					IsRevoked:             false,
				}

				store.EXPECT().
					GetSessionByRefreshToken(gomock.Any(), gomock.Eq(refreshToken)).
					Times(1).
					Return(session, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, tokenMaker token.Maker, userID int64) {
				require.Equal(t, http.StatusOK, recorder.Code)

				// 解析响应
				var rsp renewAccessTokenResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &rsp)
				require.NoError(t, err)

				// 验证access_token不为空
				require.NotEmpty(t, rsp.AccessToken)

				// 验证过期时间合理（应该在未来1分钟内，允许2秒误差）
				require.WithinDuration(t, time.Now().Add(time.Minute), rsp.AccessTokenExpiresAt, 2*time.Second)

				// 最重要：验证返回的token真的可以被验证，且payload正确
				payload, err := tokenMaker.VerifyToken(rsp.AccessToken, token.TokenTypeAccessToken)
				require.NoError(t, err)
				require.Equal(t, userID, payload.UserID)
				require.Equal(t, token.TokenType(token.TokenTypeAccessToken), payload.Type)
				require.WithinDuration(t, time.Now(), payload.IssuedAt, 2*time.Second)

				// 验证token确实在将来过期
				require.True(t, payload.ExpiredAt.After(time.Now()))
			},
		},
		{
			name: "InvalidToken",
			setupBody: func(t *testing.T, tokenMaker token.Maker) (map[string]interface{}, string) {
				return map[string]interface{}{
					"refresh_token": "invalid.token.string",
				}, "invalid.token.string"
			},
			buildStubs: func(store *mockdb.MockStore, refreshToken string, userID int64) {
				// VerifyToken会失败,不会调用store方法
				store.EXPECT().
					GetSessionByRefreshToken(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, tokenMaker token.Maker, userID int64) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "ExpiredToken",
			setupBody: func(t *testing.T, tokenMaker token.Maker) (map[string]interface{}, string) {
				// 创建一个已过期的refresh token（duration为负数）
				refreshToken, _, err := tokenMaker.CreateToken(user.ID, -time.Hour, token.TokenTypeRefreshToken)
				require.NoError(t, err)
				return map[string]interface{}{
					"refresh_token": refreshToken,
				}, refreshToken
			},
			buildStubs: func(store *mockdb.MockStore, refreshToken string, userID int64) {
				// token验证会失败，因为已过期
				store.EXPECT().
					GetSessionByRefreshToken(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, tokenMaker token.Maker, userID int64) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "SessionExpired",
			setupBody: func(t *testing.T, tokenMaker token.Maker) (map[string]interface{}, string) {
				refreshToken, _, err := tokenMaker.CreateToken(user.ID, 24*time.Hour, token.TokenTypeRefreshToken)
				require.NoError(t, err)
				return map[string]interface{}{
					"refresh_token": refreshToken,
				}, refreshToken
			},
			buildStubs: func(store *mockdb.MockStore, refreshToken string, userID int64) {
				// Session在数据库中已过期
				session := db.Session{
					ID:                    util.RandomInt(1, 1000),
					UserID:                userID,
					RefreshToken:          refreshToken,
					RefreshTokenExpiresAt: time.Now().Add(-1 * time.Hour), // 已过期
					IsRevoked:             false,
				}

				store.EXPECT().
					GetSessionByRefreshToken(gomock.Any(), gomock.Eq(refreshToken)).
					Times(1).
					Return(session, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, tokenMaker token.Maker, userID int64) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "BlockedSession",
			setupBody: func(t *testing.T, tokenMaker token.Maker) (map[string]interface{}, string) {
				refreshToken, _, err := tokenMaker.CreateToken(user.ID, 24*time.Hour, token.TokenTypeRefreshToken)
				require.NoError(t, err)
				return map[string]interface{}{
					"refresh_token": refreshToken,
				}, refreshToken
			},
			buildStubs: func(store *mockdb.MockStore, refreshToken string, userID int64) {
				session := db.Session{
					ID:                    util.RandomInt(1, 1000),
					UserID:                userID,
					RefreshToken:          refreshToken,
					RefreshTokenExpiresAt: time.Now().Add(24 * time.Hour),
					IsRevoked:             true, // 已撤销
				}

				store.EXPECT().
					GetSessionByRefreshToken(gomock.Any(), gomock.Eq(refreshToken)).
					Times(1).
					Return(session, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, tokenMaker token.Maker, userID int64) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "MissingToken",
			setupBody: func(t *testing.T, tokenMaker token.Maker) (map[string]interface{}, string) {
				return map[string]interface{}{}, ""
			},
			buildStubs: func(store *mockdb.MockStore, refreshToken string, userID int64) {
				store.EXPECT().
					GetSessionByRefreshToken(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, tokenMaker token.Maker, userID int64) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)

			// 创建真实的tokenMaker用于测试
			config := util.Config{
				TokenSymmetricKey:   util.RandomString(32),
				AccessTokenDuration: time.Minute,
			}
			tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
			require.NoError(t, err)

			// 设置请求body并获取refreshToken
			body, refreshToken := tc.setupBody(t, tokenMaker)
			tc.buildStubs(store, refreshToken, user.ID)

			// 创建服务器（使用真实tokenMaker）
			server := &Server{
				config:     config,
				store:      store,
				tokenMaker: tokenMaker,
			}
			server.setupRouter()

			recorder := httptest.NewRecorder()

			data, err := json.Marshal(body)
			require.NoError(t, err)

			url := "/v1/auth/refresh"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder, tokenMaker, user.ID)
		})
	}
}
