package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
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
	req.Code = strings.TrimSpace(req.Code)
	if !isValidWechatLoginCode(req.Code) {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid wechat code")))
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
		if errors.Is(err, wechat.ErrCode2SessionMissingOpenID) {
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "微信登录服务返回异常，请稍后重试", "wechat login upstream missing openid"))
			return
		}

		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("failed to get wechat session: %w", err)))
		return
	}
	openID := strings.TrimSpace(wechatResp.OpenID)
	if openID == "" {
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, errors.New("wechat login response missing openid"), "微信登录服务返回异常，请稍后重试", "wechat login upstream missing openid"))
		return
	}

	// 查找或创建用户
	user, err := server.store.GetUserByWechatOpenID(ctx, openID)

	if err != nil {
		if isNotFoundError(err) {
			// 用户不存在,创建新用户(使用事务确保原子性)
			txResult, err := server.store.CreateUserTx(ctx, db.CreateUserTxParams{
				WechatOpenid: openID,
				FullName:     "微信用户",
				DefaultRole:  "customer",
			})
			if err != nil {
				// 并发请求可能导致重复 key 冲突（TOCTOU），此时降级为查询已存在的用户
				if isDuplicateKeyError(err) {
					user, err = server.store.GetUserByWechatOpenID(ctx, openID)
					if err != nil {
						ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get user after duplicate: %w", err)))
						return
					}
				} else {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create user tx: %w", err)))
					return
				}
			} else {
				user = txResult.User
			}
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get user by openid: %w", err)))
			return
		}
	}

	// 记录设备信息（用于M9欺诈检测）。此操作为辅助性安全增强，失败不应阻断登录主路径。
	deviceArg := db.UpsertUserDeviceParams{
		UserID:            user.ID,
		DeviceID:          req.DeviceID,
		DeviceFingerprint: pgtype.Text{String: req.DeviceFingerprint, Valid: req.DeviceFingerprint != ""},
		DeviceType:        req.DeviceType,
	}
	if _, err = server.store.UpsertUserDevice(ctx, deviceArg); err != nil {
		log.Warn().Err(err).Int64("user_id", user.ID).Msg("failed to record device info, non-fatal")
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

	roles, workbenches, err := server.buildUserAccessProfile(ctx, user.ID, userRoles)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("build user access profile: %w", err)))
		return
	}

	rsp := wechatLoginResponse{
		SessionID:             session.ID,
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessPayload.ExpiredAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshPayload.ExpiredAt,
		User:                  newUserResponse(user, roles, workbenches),
	}
	ctx.JSON(http.StatusOK, rsp)
}

func isValidWechatLoginCode(code string) bool {
	if code == "" || len(code) > 256 {
		return false
	}
	for _, r := range code {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '.' || r == '_' || r == '~':
		default:
			return false
		}
	}
	return true
}
