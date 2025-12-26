package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

type userResponse struct {
	ID           int64     `json:"id"`
	WechatOpenID string    `json:"wechat_openid"`
	FullName     string    `json:"full_name"`
	Phone        *string   `json:"phone,omitempty"`
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	Roles        []string  `json:"roles,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func newUserResponse(user db.User, roles []string) userResponse {
	resp := userResponse{
		ID:           user.ID,
		WechatOpenID: user.WechatOpenid,
		FullName:     user.FullName,
		CreatedAt:    user.CreatedAt,
		Roles:        roles,
	}

	if user.Phone.Valid {
		resp.Phone = &user.Phone.String
	}

	if user.AvatarUrl.Valid {
		avatar := normalizeUploadURLForClient(user.AvatarUrl.String)
		resp.AvatarURL = &avatar
	}

	return resp
}

// getCurrentUser godoc
// @Summary 获取当前用户信息
// @Description 获取当前已认证用户的详细信息，包括基本资料和角色列表
// @Tags 用户
// @Accept json
// @Produce json
// @Success 200 {object} userResponse "用户信息"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "用户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/users/me [get]
// @Security BearerAuth
func (server *Server) getCurrentUser(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	user, err := server.store.GetUser(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取用户角色
	userRoles, err := server.store.ListUserRoles(ctx, user.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	roles := make([]string, len(userRoles))
	for i, r := range userRoles {
		roles[i] = r.Role
	}

	ctx.JSON(http.StatusOK, newUserResponse(user, roles))
}

type updateUserRequest struct {
	FullName  *string `json:"full_name" binding:"omitempty,min=1,max=50"`
	AvatarURL *string `json:"avatar_url" binding:"omitempty,min=1,max=2048"`
}

// updateCurrentUser godoc
// @Summary 更新当前用户信息
// @Description 更新当前已认证用户的个人资料
// @Tags 用户
// @Accept json
// @Produce json
// @Param request body updateUserRequest true "更新用户请求"
// @Success 200 {object} userResponse "更新后的用户信息"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "用户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/users/me [patch]
// @Security BearerAuth
func (server *Server) updateCurrentUser(ctx *gin.Context) {
	var req updateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	arg := db.UpdateUserParams{
		ID: authPayload.UserID,
	}

	if req.FullName != nil {
		arg.FullName = pgtype.Text{
			String: *req.FullName,
			Valid:  true,
		}
	}

	if req.AvatarURL != nil {
		arg.AvatarUrl = pgtype.Text{String: normalizeImageURLForStorage(*req.AvatarURL), Valid: true}
	}

	user, err := server.store.UpdateUser(ctx, arg)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取用户角色
	userRoles, err := server.store.ListUserRoles(ctx, user.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	roles := make([]string, len(userRoles))
	for i, r := range userRoles {
		roles[i] = r.Role
	}

	ctx.JSON(http.StatusOK, newUserResponse(user, roles))
}
