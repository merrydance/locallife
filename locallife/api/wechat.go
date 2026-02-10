package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
)

type wechatLoginRequest struct {
	Code              string `json:"code" binding:"required,min=1,max=256"`
	DeviceID          string `json:"device_id" binding:"required,min=1,max=128"`
	DeviceType        string `json:"device_type" binding:"required,oneof=ios android miniprogram h5"`
	DeviceFingerprint string `json:"device_fingerprint,omitempty" binding:"omitempty,max=256"`
}

type wechatLoginResponse struct {
	SessionID             int64        `json:"session_id"`
	AccessToken           string       `json:"access_token"`
	AccessTokenExpiresAt  time.Time    `json:"access_token_expires_at"`
	RefreshToken          string       `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time    `json:"refresh_token_expires_at"`
	User                  userResponse `json:"user"`
}

// wechatLogin godoc
// @Summary WeChat mini-program login
// @Description Authenticate user with WeChat code, create user if not exists
// @Tags auth
// @Accept json
// @Produce json
// @Param request body wechatLoginRequest true "WeChat login request"
// @Success 200 {object} wechatLoginResponse
// @Failure 400 {object} ErrorResponse "Invalid request parameters"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /v1/auth/wechat-login [post]
func (server *Server) wechatLogin(ctx *gin.Context) {
	var req wechatLoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 使用code换取openid
	wechatResp, err := server.wechatClient.Code2Session(ctx, req.Code)
	if err != nil {
		var apiErr *wechat.APIError
		if errors.As(err, &apiErr) {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid wechat code")))
			return
		}

		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("failed to get wechat session: %w", err)))
		return
	}

	// 查找或创建用户
	user, err := server.store.GetUserByWechatOpenID(ctx, wechatResp.OpenID)

	if err != nil {
		if isNotFoundError(err) {
			// 用户不存在,创建新用户(使用事务确保原子性)
			txResult, err := server.store.CreateUserTx(ctx, db.CreateUserTxParams{
				WechatOpenid: wechatResp.OpenID,
				FullName:     "微信用户",
				DefaultRole:  "customer",
			})
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create user tx: %w", err)))
				return
			}
			user = txResult.User
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get user by openid: %w", err)))
			return
		}
	}

	// 记录设备信息（用于M9欺诈检测）
	deviceArg := db.UpsertUserDeviceParams{
		UserID:            user.ID,
		DeviceID:          req.DeviceID,
		DeviceFingerprint: pgtype.Text{String: req.DeviceFingerprint, Valid: req.DeviceFingerprint != ""},
		DeviceType:        req.DeviceType,
	}
	_, err = server.store.UpsertUserDevice(ctx, deviceArg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("failed to record device info: %w", err)))
		return
	}

	// 生成访问令牌
	accessToken, accessPayload, err := server.tokenMaker.CreateToken(
		user.ID,
		server.config.AccessTokenDuration,
		token.TokenTypeAccessToken,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create access token: %w", err)))
		return
	}

	// 生成刷新令牌
	refreshToken, refreshPayload, err := server.tokenMaker.CreateToken(
		user.ID,
		server.config.RefreshTokenDuration,
		token.TokenTypeRefreshToken,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create refresh token: %w", err)))
		return
	}

	accessTokenHash, err := util.HashToken(accessToken, server.config.TokenSymmetricKey)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("hash access token: %w", err)))
		return
	}

	refreshTokenHash, err := util.HashToken(refreshToken, server.config.TokenSymmetricKey)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("hash refresh token: %w", err)))
		return
	}

	// 创建会话
	session, err := server.store.CreateSession(ctx, db.CreateSessionParams{
		UserID:                user.ID,
		AccessToken:           accessTokenHash,
		RefreshToken:          refreshTokenHash,
		AccessTokenExpiresAt:  accessPayload.ExpiredAt,
		RefreshTokenExpiresAt: refreshPayload.ExpiredAt,
		UserAgent:             ctx.Request.UserAgent(),
		ClientIp:              ctx.ClientIP(),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create session: %w", err)))
		return
	}

	// 获取用户角色
	userRoles, err := server.store.ListUserRoles(ctx, user.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list user roles: %w", err)))
		return
	}

	roles := make([]string, len(userRoles))
	for i, r := range userRoles {
		roles[i] = r.Role
	}

	rsp := wechatLoginResponse{
		SessionID:             session.ID,
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessPayload.ExpiredAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshPayload.ExpiredAt,
		User:                  newUserResponse(user, roles),
	}
	ctx.JSON(http.StatusOK, rsp)
}
