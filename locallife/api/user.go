package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
)

type userResponse struct {
	ID           int64                   `json:"id"`
	WechatOpenID string                  `json:"wechat_openid"`
	FullName     string                  `json:"full_name"`
	Phone        *string                 `json:"phone,omitempty"`
	AvatarURL    *string                 `json:"avatar_url,omitempty"`
	Roles        []string                `json:"roles,omitempty"`
	Workbenches  []userWorkbenchResponse `json:"workbenches,omitempty"`
	CreatedAt    time.Time               `json:"created_at"`
}

type userWorkbenchResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	MerchantID   *int64 `json:"merchant_id,omitempty"`
	MerchantName string `json:"merchant_name,omitempty"`
	StaffRole    string `json:"staff_role,omitempty"`
	Message      string `json:"message,omitempty"`
}

func newUserResponse(user db.User, roles []string, workbenches []userWorkbenchResponse) userResponse {
	resp := userResponse{
		ID:           user.ID,
		WechatOpenID: user.WechatOpenid,
		FullName:     user.FullName,
		CreatedAt:    user.CreatedAt,
		Roles:        roles,
		Workbenches:  workbenches,
	}

	if user.Phone.Valid {
		resp.Phone = &user.Phone.String
	}
	if user.AvatarUrl.Valid && user.AvatarUrl.String != "" {
		avatarURL := user.AvatarUrl.String
		resp.AvatarURL = &avatarURL
	}

	return resp
}

func normalizeUserRoles(userRoles []db.UserRole) []string {
	roles := make([]string, len(userRoles))
	for i, r := range userRoles {
		roles[i] = r.Role
	}
	return roles
}

func filterRole(roles []string, target string) []string {
	filtered := make([]string, 0, len(roles))
	for _, role := range roles {
		if role == target {
			continue
		}
		filtered = append(filtered, role)
	}
	return filtered
}

func appendWorkbench(workbenches []userWorkbenchResponse, workbench userWorkbenchResponse) []userWorkbenchResponse {
	for _, existing := range workbenches {
		if existing.ID == workbench.ID {
			return workbenches
		}
	}
	return append(workbenches, workbench)
}

func (server *Server) buildUserAccessProfile(ctx context.Context, userID int64, userRoles []db.UserRole) ([]string, []userWorkbenchResponse, error) {
	roles := normalizeUserRoles(userRoles)
	roleSet := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		roleSet[role] = struct{}{}
	}

	workbenches := make([]userWorkbenchResponse, 0, 4)
	merchantWorkbench, hasGrantedMerchantStaffAccess, err := server.buildMerchantWorkbench(ctx, userID, roleSet)
	if err != nil {
		return nil, nil, err
	}
	if merchantWorkbench != nil {
		workbenches = append(workbenches, *merchantWorkbench)
	}

	if _, ok := roleSet[RoleMerchantStaff]; ok && !hasGrantedMerchantStaffAccess {
		roles = filterRole(roles, RoleMerchantStaff)
	}

	if _, ok := roleSet[RoleRider]; ok {
		workbenches = appendWorkbench(workbenches, userWorkbenchResponse{ID: "rider", Status: "granted"})
	}
	if _, ok := roleSet[RoleOperator]; ok {
		workbenches = appendWorkbench(workbenches, userWorkbenchResponse{ID: "operator", Status: "granted"})
	}
	if _, ok := roleSet[RoleAdmin]; ok {
		workbenches = appendWorkbench(workbenches, userWorkbenchResponse{ID: "admin", Status: "granted"})
	}

	return roles, workbenches, nil
}

func (server *Server) buildMerchantWorkbench(ctx context.Context, userID int64, roleSet map[string]struct{}) (*userWorkbenchResponse, bool, error) {
	staffMerchants, err := server.store.ListMerchantsByStaff(ctx, userID)
	if err != nil {
		return nil, false, err
	}

	var grantedMerchant *db.Merchant
	var pendingMerchant *db.Merchant
	grantedStaffRole := ""
	hasGrantedMerchantStaffAccess := false

	for i := range staffMerchants {
		merchant := staffMerchants[i]
		staffRole, err := server.store.GetUserMerchantRole(ctx, db.GetUserMerchantRoleParams{
			MerchantID: merchant.ID,
			UserID:     userID,
		})
		if err != nil {
			if isNotFoundError(err) {
				continue
			}
			return nil, false, err
		}

		if staffRole == "pending" {
			if pendingMerchant == nil {
				pendingMerchant = &merchant
			}
			continue
		}

		hasGrantedMerchantStaffAccess = true
		if grantedMerchant == nil {
			grantedMerchant = &merchant
			grantedStaffRole = staffRole
		}
	}

	if _, ok := roleSet[RoleMerchantOwner]; ok {
		ownedMerchants, err := server.store.ListMerchantsByOwner(ctx, userID)
		if err != nil {
			return nil, false, err
		}

		workbench := userWorkbenchResponse{
			ID:        "merchant",
			Status:    "granted",
			StaffRole: "owner",
		}
		if len(ownedMerchants) > 0 {
			workbench.MerchantID = &ownedMerchants[0].ID
			workbench.MerchantName = ownedMerchants[0].Name
		}
		return &workbench, hasGrantedMerchantStaffAccess, nil
	}

	if grantedMerchant != nil {
		return &userWorkbenchResponse{
			ID:           "merchant",
			Status:       "granted",
			MerchantID:   &grantedMerchant.ID,
			MerchantName: grantedMerchant.Name,
			StaffRole:    grantedStaffRole,
		}, hasGrantedMerchantStaffAccess, nil
	}

	if pendingMerchant != nil {
		return &userWorkbenchResponse{
			ID:           "merchant",
			Status:       "pending_assignment",
			MerchantID:   &pendingMerchant.ID,
			MerchantName: pendingMerchant.Name,
			StaffRole:    "pending",
			Message:      "已加入商户，等待老板分配岗位后即可进入工作台。",
		}, hasGrantedMerchantStaffAccess, nil
	}

	return nil, hasGrantedMerchantStaffAccess, nil
}

func (server *Server) resolveCurrentUserAvatarURL(ctx *gin.Context, user db.User) *string {
	if user.AvatarMediaAssetID.Valid {
		asset, err := server.store.GetMediaAssetByID(ctx, user.AvatarMediaAssetID.Int64)
		if err == nil && asset.Visibility == string(media.VisibilityPublic) {
			if asset.ModerationStatus == "approved" || (asset.MediaCategory == string(media.CategoryAvatar) && asset.UploadedBy == user.ID) {
				avatarURL := server.mediaResolver.PublicURL(asset.ObjectKey, media.VariantOriginal)
				return &avatarURL
			}
		}
	}
	if user.AvatarUrl.Valid && user.AvatarUrl.String != "" {
		avatarURL := user.AvatarUrl.String
		return &avatarURL
	}
	return nil
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
		if isNotFoundError(err) {
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

	roles, workbenches, err := server.buildUserAccessProfile(ctx, user.ID, userRoles)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := newUserResponse(user, roles, workbenches)
	resp.AvatarURL = server.resolveCurrentUserAvatarURL(ctx, user)
	ctx.JSON(http.StatusOK, resp)
}

type updateUserRequest struct {
	FullName           *string `json:"full_name" binding:"omitempty,min=1,max=50"`
	AvatarMediaAssetID *int64  `json:"avatar_media_asset_id" binding:"omitempty,min=1"`
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

	if req.AvatarMediaAssetID != nil {
		arg.AvatarMediaAssetID = pgtype.Int8{Int64: *req.AvatarMediaAssetID, Valid: true}
	}

	user, err := server.store.UpdateUser(ctx, arg)
	if err != nil {
		if isNotFoundError(err) {
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

	roles, workbenches, err := server.buildUserAccessProfile(ctx, user.ID, userRoles)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := newUserResponse(user, roles, workbenches)
	resp.AvatarURL = server.resolveCurrentUserAvatarURL(ctx, user)
	ctx.JSON(http.StatusOK, resp)
}
