package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
	QRPayload   string     `json:"qr_payload,omitempty"`
	PollToken   string     `json:"poll_token,omitempty"`
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
	Sig  string `json:"sig" binding:"required,min=1,max=128"`
	TS   int64  `json:"ts" binding:"required"`
}

type webLoginConsumeRequest struct {
	PollToken string `json:"poll_token" binding:"required,min=1,max=256"`
}

func (server *Server) webLoginSessionTTL() time.Duration {
	if server.config.WebLoginSessionTTL > 0 {
		return server.config.WebLoginSessionTTL
	}
	return defaultWebLoginSessionTTL
}

func (server *Server) webLoginQRSigningKey() string {
	if server.config.WebLoginQRSigningKey != "" {
		return server.config.WebLoginQRSigningKey
	}
	return server.config.TokenSymmetricKey
}

func generateWebLoginCode() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func generateWebLoginPollToken() (string, error) {
	bytes := make([]byte, 24)
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

func newWebLoginSessionStatusResponseWithPollToken(session db.WebLoginSession, payload string, pollToken string) webLoginSessionStatusResponse {
	resp := newWebLoginSessionStatusResponse(session)
	resp.QRPayload = payload
	resp.PollToken = pollToken
	return resp
}

func signWebLoginQRCode(code string, ts int64, secret string) (string, error) {
	if secret == "" {
		return "", errors.New("缺少签名密钥")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(code))
	_, _ = mac.Write([]byte("|"))
	_, _ = mac.Write([]byte(strconv.FormatInt(ts, 10)))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func (server *Server) buildWebLoginQRPayload(code string, issuedAt time.Time) (string, error) {
	ts := issuedAt.Unix()
	sig, err := signWebLoginQRCode(code, ts, server.webLoginQRSigningKey())
	if err != nil {
		return "", err
	}
	if server.config.WebBaseURL != "" {
		base := strings.TrimRight(server.config.WebBaseURL, "/")
		return fmt.Sprintf("%s/merchant/login?code=%s&ts=%d&sig=%s", base, code, ts, sig), nil
	}
	return fmt.Sprintf("web-login:%s?ts=%d&sig=%s", code, ts, sig), nil
}

func (server *Server) verifyWebLoginQRSignature(code string, ts int64, sig string) error {
	if sig == "" || ts == 0 {
		return errors.New("缺少签名信息")
	}
	issuedAt := time.Unix(ts, 0)
	if time.Since(issuedAt) > server.webLoginSessionTTL() || time.Until(issuedAt) > server.webLoginSessionTTL() {
		return errors.New("签名已过期，请刷新二维码")
	}
	expected, err := signWebLoginQRCode(code, ts, server.webLoginQRSigningKey())
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return errors.New("签名校验失败")
	}
	return nil
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

		pollToken, err := generateWebLoginPollToken()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		session, err := server.store.CreateWebLoginSession(ctx, db.CreateWebLoginSessionParams{
			Code:         code,
			ExpiresAt:    expiresAt,
			WebUserAgent: pgtype.Text{String: ua, Valid: ua != ""},
			WebClientIp:  pgtype.Text{String: ip, Valid: ip != ""},
			PollToken:    pgtype.Text{String: pollToken, Valid: pollToken != ""},
		})
		if err != nil {
			lastErr = err
			if isDuplicateKeyError(err) {
				continue
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		payload, err := server.buildWebLoginQRPayload(session.Code, time.Now())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		ctx.JSON(http.StatusOK, newWebLoginSessionStatusResponseWithPollToken(session, payload, pollToken))
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
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("缺少登录码")))
		return
	}

	pollToken := strings.TrimSpace(ctx.Query("poll_token"))
	lastStatus := strings.TrimSpace(ctx.Query("last_status"))
	waitSeconds := int64(0)
	if rawWait := strings.TrimSpace(ctx.Query("wait")); rawWait != "" {
		if parsed, err := strconv.ParseInt(rawWait, 10, 64); err == nil {
			waitSeconds = parsed
		}
	}
	if waitSeconds < 0 {
		waitSeconds = 0
	}
	if waitSeconds > 30 {
		waitSeconds = 30
	}

	session, err := server.store.GetWebLoginSessionByCode(ctx, code)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("登录会话不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if pollToken != "" {
		if !session.PollToken.Valid || session.PollToken.String != pollToken {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("轮询凭证无效")))
			return
		}
	}
	if waitSeconds > 0 && pollToken == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("缺少轮询凭证")))
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

	if waitSeconds == 0 || lastStatus == "" || session.Status != lastStatus || session.Status == "consumed" || session.Status == "expired" {
		ctx.JSON(http.StatusOK, newWebLoginSessionStatusResponse(session))
		return
	}

	deadline := time.Now().Add(time.Duration(waitSeconds) * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		case <-ticker.C:
		}

		current, err := server.store.GetWebLoginSessionByCode(ctx, code)
		if err != nil {
			return
		}

		if time.Now().After(current.ExpiresAt) && current.Status != "consumed" && current.Status != "expired" {
			updated, err := server.store.ExpireWebLoginSession(ctx, current.ID)
			if err == nil {
				current = updated
			}
		}

		if current.Status != lastStatus || current.Status == "consumed" || current.Status == "expired" {
			ctx.JSON(http.StatusOK, newWebLoginSessionStatusResponse(current))
			return
		}

		if time.Now().After(deadline) {
			ctx.Status(http.StatusNoContent)
			return
		}
	}
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
	if err := server.verifyWebLoginQRSignature(req.Code, req.TS, req.Sig); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	session, err := server.store.GetWebLoginSessionByCode(ctx, req.Code)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("登录会话不存在")))
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
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("登录会话已过期")))
		return
	}

	if session.Status == "consumed" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("登录会话已被使用")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	if session.Status == "confirmed" {
		if session.UserID.Valid && session.UserID.Int64 == authPayload.UserID {
			ctx.JSON(http.StatusOK, newWebLoginSessionStatusResponse(session))
			return
		}
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("登录会话已被其他账号确认")))
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
// @Description Web 端用 poll_token 换取 token
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

	session, err := server.store.GetWebLoginSessionByPollToken(ctx, pgtype.Text{String: req.PollToken, Valid: req.PollToken != ""})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("登录会话不存在")))
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
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("登录会话已过期")))
		return
	}

	if session.Status == "consumed" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("登录会话已被使用")))
		return
	}
	if session.Status != "confirmed" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("登录会话尚未确认")))
		return
	}
	if !session.UserID.Valid {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("登录会话缺少用户信息")))
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
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("用户不存在")))
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
