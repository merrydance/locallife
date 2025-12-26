package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

type renewAccessTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required,min=1,max=1024"`
}

type renewAccessTokenResponse struct {
	AccessToken          string    `json:"access_token"`
	AccessTokenExpiresAt time.Time `json:"access_token_expires_at"`
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

	session, err := server.store.GetSessionByRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if session.IsRevoked {
		err := fmt.Errorf("revoked session")
		ctx.JSON(http.StatusUnauthorized, errorResponse(err))
		return
	}

	if session.UserID != refreshPayload.UserID {
		err := fmt.Errorf("incorrect session user")
		ctx.JSON(http.StatusUnauthorized, errorResponse(err))
		return
	}

	if time.Now().After(session.RefreshTokenExpiresAt) {
		err := fmt.Errorf("expired refresh token")
		ctx.JSON(http.StatusUnauthorized, errorResponse(err))
		return
	}

	accessToken, accessPayload, err := server.tokenMaker.CreateToken(
		refreshPayload.UserID,
		server.config.AccessTokenDuration,
		token.TokenTypeAccessToken,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := renewAccessTokenResponse{
		AccessToken:          accessToken,
		AccessTokenExpiresAt: accessPayload.ExpiredAt,
	}
	ctx.JSON(http.StatusOK, rsp)
}
