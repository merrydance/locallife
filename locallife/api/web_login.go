package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
)

const defaultWebLoginSessionTTL = 5 * time.Minute

// ==================== Web 登录扫码 ====================

type webLoginSessionStatusResponse struct {
	Code        string     `json:"code"`
	Status      string     `json:"status"`
	ExpiresAt   time.Time  `json:"expires_at"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
	ConsumedAt  *time.Time `json:"consumed_at,omitempty"`
}

type webLoginConsumeResponse struct {
	SessionID             int64        `json:"session_id"`
	AccessToken           string       `json:"access_token"`
	AccessTokenExpiresAt  time.Time    `json:"access_token_expires_at"`
	RefreshToken          string       `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time    `json:"refresh_token_expires_at"`
	User                  userResponse `json:"user"`
}

type webLoginConfirmRequest struct {
	Code string `json:"code" binding:"required,min=1,max=128"`
}

type webLoginConsumeRequest struct {
	Code string `json:"code" binding:"required,min=1,max=128"`
}

func (server *Server) webLoginSessionTTL() time.Duration {
	if server.config.WebLoginSessionTTL > 0 {
		return server.config.WebLoginSessionTTL
	}
	return defaultWebLoginSessionTTL
}

func generateWebLoginCode() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func newWebLoginSessionStatusResponse(session db.WebLoginSession) webLoginSessionStatusResponse {
	return webLoginSessionStatusResponse{
		Code:        session.Code,
		Status:      session.Status,
		ExpiresAt:   session.ExpiresAt,
		ConfirmedAt: pgTimeToPtr(session.ConfirmedAt),
		ConsumedAt:  pgTimeToPtr(session.ConsumedAt),
	}
}

// createWebLoginSession godoc
// @Summary 创建 Web 登录会话
// @Description Web 端生成扫码登录会话
// @Tags 认证
// @Produce json
// @Success 200 {object} webLoginSessionStatusResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/auth/web-login/sessions [post]
func (server *Server) createWebLoginSession(ctx *gin.Context) {
	expiresAt := time.Now().Add(server.webLoginSessionTTL())
	ua := ctx.Request.UserAgent()
	ip := ctx.ClientIP()

	var lastErr error
	for i := 0; i < 5; i++ {
		code, err := generateWebLoginCode()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		session, err := server.store.CreateWebLoginSession(ctx, db.CreateWebLoginSessionParams{
			Code:         code,
			ExpiresAt:    expiresAt,
			WebUserAgent: pgtype.Text{String: ua, Valid: ua != ""},
			WebClientIp:  pgtype.Text{String: ip, Valid: ip != ""},
		})
		if err != nil {
			lastErr = err
			if isDuplicateKeyError(err) {
				continue
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		ctx.JSON(http.StatusOK, newWebLoginSessionStatusResponse(session))
		return
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("failed to generate login code")
	}
	ctx.JSON(http.StatusInternalServerError, internalError(ctx, lastErr))
}

// getWebLoginSessionStatus godoc
// @Summary 查询 Web 登录会话状态
// @Description Web 端轮询登录会话状态
// @Tags 认证
// @Produce json
// @Param code path string true "登录码"
// @Success 200 {object} webLoginSessionStatusResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/auth/web-login/sessions/{code} [get]
func (server *Server) getWebLoginSessionStatus(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("code is required")))
		return
	}

	session, err := server.store.GetWebLoginSessionByCode(ctx, code)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("session not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if time.Now().After(session.ExpiresAt) && session.Status != "consumed" && session.Status != "expired" {
		updated, err := server.store.ExpireWebLoginSession(ctx, session.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		session = updated
	}

	ctx.JSON(http.StatusOK, newWebLoginSessionStatusResponse(session))
}

// confirmWebLoginSession godoc
// @Summary 小程序确认 Web 登录
// @Description 小程序扫码确认 Web 登录
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body webLoginConfirmRequest true "确认请求"
// @Success 200 {object} webLoginSessionStatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/auth/web-login/confirm [post]
// @Security BearerAuth
func (server *Server) confirmWebLoginSession(ctx *gin.Context) {
	var req webLoginConfirmRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	session, err := server.store.GetWebLoginSessionByCode(ctx, req.Code)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("session not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if time.Now().After(session.ExpiresAt) {
		if session.Status != "expired" {
			updated, err := server.store.ExpireWebLoginSession(ctx, session.ID)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			session = updated
		}
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("session expired")))
		return
	}

	if session.Status == "consumed" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("session already consumed")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	if session.Status == "confirmed" {
		if session.UserID.Valid && session.UserID.Int64 == authPayload.UserID {
			ctx.JSON(http.StatusOK, newWebLoginSessionStatusResponse(session))
			return
		}
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("session already confirmed")))
		return
	}

	ip := ctx.ClientIP()
	confirmed, err := server.store.ConfirmWebLoginSession(ctx, db.ConfirmWebLoginSessionParams{
		ID:              session.ID,
		UserID:          pgtype.Int8{Int64: authPayload.UserID, Valid: true},
		ConfirmClientIp: pgtype.Text{String: ip, Valid: ip != ""},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newWebLoginSessionStatusResponse(confirmed))
}

// consumeWebLoginSession godoc
// @Summary Web 端消费登录会话
// @Description Web 端用 code 换取 token
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body webLoginConsumeRequest true "消费请求"
// @Success 200 {object} webLoginConsumeResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/auth/web-login/consume [post]
func (server *Server) consumeWebLoginSession(ctx *gin.Context) {
	var req webLoginConsumeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	session, err := server.store.GetWebLoginSessionByCode(ctx, req.Code)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("session not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if time.Now().After(session.ExpiresAt) {
		if session.Status != "expired" {
			updated, err := server.store.ExpireWebLoginSession(ctx, session.ID)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			session = updated
		}
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("session expired")))
		return
	}

	if session.Status == "consumed" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("session already consumed")))
		return
	}
	if session.Status != "confirmed" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("session not confirmed")))
		return
	}
	if !session.UserID.Valid {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("session missing user")))
		return
	}

	consumed, err := server.store.ConsumeWebLoginSession(ctx, session.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	user, err := server.store.GetUser(ctx, consumed.UserID.Int64)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("user not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
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
	webSession, err := server.store.CreateSession(ctx, db.CreateSessionParams{
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

	userRoles, err := server.store.ListUserRoles(ctx, user.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("list user roles: %w", err)))
		return
	}

	roles := make([]string, len(userRoles))
	for i, r := range userRoles {
		roles[i] = r.Role
	}

	rsp := webLoginConsumeResponse{
		SessionID:             webSession.ID,
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessPayload.ExpiredAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshPayload.ExpiredAt,
		User:                  newUserResponse(user, roles),
	}
	ctx.JSON(http.StatusOK, rsp)
}
