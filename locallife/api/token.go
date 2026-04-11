package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
)

type renewAccessTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required,min=1,max=1024"`
}

type renewAccessTokenResponse struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

// renewAccessToken godoc
// @Summary 刷新访问令牌
// @Description 使用刷新令牌获取新的访问令牌
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body renewAccessTokenRequest true "刷新令牌请求"
// @Success 200 {object} renewAccessTokenResponse "新的访问令牌"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "令牌无效或已过期"
// @Failure 404 {object} ErrorResponse "会话不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/auth/refresh [post]
func (server *Server) renewAccessToken(ctx *gin.Context) {
	var req renewAccessTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	refreshPayload, err := server.tokenMaker.VerifyToken(req.RefreshToken, token.TokenTypeRefreshToken)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, errorResponse(err))
		return
	}

	refreshTokenHash, err := util.HashToken(req.RefreshToken, server.config.TokenSymmetricKey)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	session, err := server.store.GetSessionByRefreshToken(ctx, db.GetSessionByRefreshTokenParams{
		RefreshToken:         refreshTokenHash,
		RefreshTokenFallback: req.RefreshToken,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	refreshTokenDuration := server.config.RefreshTokenDuration
	if isAppBindSessionUserAgent(session.UserAgent) {
		refreshTokenDuration = appRefreshTokenDuration
	}

	// 先生成新 token（不涉及 DB 写操作）
	accessToken, accessPayload, err := server.tokenMaker.CreateToken(
		refreshPayload.UserID,
		server.config.AccessTokenDuration,
		token.TokenTypeAccessToken,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	newRefreshToken, newRefreshPayload, err := server.tokenMaker.CreateToken(
		refreshPayload.UserID,
		refreshTokenDuration,
		token.TokenTypeRefreshToken,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	accessTokenHash, err := util.HashToken(accessToken, server.config.TokenSymmetricKey)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	newRefreshTokenHash, err := util.HashToken(newRefreshToken, server.config.TokenSymmetricKey)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// P1-012 修复：使用事务 + FOR UPDATE 原子地刷新会话
	// 防止并发刷新导致多个有效 token
	result, err := server.store.RefreshSessionTx(ctx, db.RefreshSessionTxParams{
		RefreshToken:          refreshTokenHash,
		RefreshTokenFallback:  req.RefreshToken,
		NewAccessToken:        accessTokenHash,
		NewRefreshToken:       newRefreshTokenHash,
		AccessTokenExpiresAt:  accessPayload.ExpiredAt,
		RefreshTokenExpiresAt: newRefreshPayload.ExpiredAt,
	})
	if err != nil {
		// 区分不同的错误类型
		errMsg := err.Error()
		if errMsg == "session is revoked" || errMsg == "refresh token expired" {
			ctx.JSON(http.StatusUnauthorized, errorResponse(err))
			return
		}
		if errors.Is(err, db.ErrRecordNotFound) || errMsg == "session not found: "+db.ErrRecordNotFound.Error() {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证会话所有者
	if result.Session.UserID != refreshPayload.UserID {
		ctx.JSON(http.StatusUnauthorized, errorResponse(fmt.Errorf("incorrect session user")))
		return
	}

	rsp := renewAccessTokenResponse{
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessPayload.ExpiredAt,
		RefreshToken:          newRefreshToken,
		RefreshTokenExpiresAt: newRefreshPayload.ExpiredAt,
	}
	ctx.JSON(http.StatusOK, rsp)
}
