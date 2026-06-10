package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
)

// ==================== 员工管理 ====================

type staffResponse struct {
	ID         int64     `json:"id"`
	MerchantID int64     `json:"merchant_id"`
	UserID     int64     `json:"user_id"`
	Role       string    `json:"role"`
	Status     string    `json:"status"`
	FullName   string    `json:"full_name,omitempty"`
	AvatarURL  string    `json:"avatar_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type staffListResponse struct {
	Staff interface{} `json:"staff"`
	Count int         `json:"count"`
}

type staffBindResponse struct {
	Message      string `json:"message"`
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	Role         string `json:"role"`
}

// listMerchantStaff 列出商户员工
// @Summary 列出商户员工
// @Description 商户老板或店长列出所有员工
// @Tags 员工管理
// @Produce json
// @Success 200 {object} []staffResponse "员工列表"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/staff [get]
// @Security BearerAuth
func (server *Server) listMerchantStaff(ctx *gin.Context) {
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant not found in context")))
		return
	}

	staffList, err := server.store.ListMerchantStaffByMerchant(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]staffResponse, len(staffList))
	for i, s := range staffList {
		resp[i] = staffResponse{
			ID:         s.ID,
			MerchantID: s.MerchantID,
			UserID:     s.UserID,
			Role:       s.Role,
			Status:     s.Status,
			FullName:   s.FullName,
			CreatedAt:  s.CreatedAt,
		}
		if s.AvatarMediaAssetID.Valid {
			resp[i].AvatarURL = server.publicImageURL(ctx, &s.AvatarMediaAssetID.Int64, media.VariantOriginal)
		}
	}

	ctx.JSON(http.StatusOK, staffListResponse{Staff: resp, Count: len(resp)})
}

type addStaffRequest struct {
	UserID int64  `json:"user_id" binding:"required,min=1"`
	Role   string `json:"role" binding:"required,oneof=manager chef cashier"`
}

// addMerchantStaff 添加员工
// @Summary 添加员工
// @Description 商户老板添加员工（需要用户ID）
// @Tags 员工管理
// @Accept json
// @Produce json
// @Param request body addStaffRequest true "员工信息"
// @Success 201 {object} staffResponse "添加成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 409 {object} ErrorResponse "员工已存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/staff [post]
// @Security BearerAuth
func (server *Server) addMerchantStaff(ctx *gin.Context) {
	var req addStaffRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant not found in context")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 检查员工是否已存在；disabled 记录允许老板重新确认角色并恢复。
	existingStaff, err := server.store.GetMerchantStaff(ctx, db.GetMerchantStaffParams{
		MerchantID: merchant.ID,
		UserID:     req.UserID,
	})
	if err == nil && existingStaff.Status != db.MerchantStaffStatusDisabled {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("staff already exists")))
		return
	}
	if err != nil && !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 创建员工记录
	result, err := server.store.AddMerchantStaffTx(ctx, db.AddMerchantStaffTxParams{
		MerchantID: merchant.ID,
		UserID:     req.UserID,
		Role:       req.Role,
		InvitedBy:  pgtype.Int8{Int64: authPayload.UserID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, db.ErrMerchantStaffAlreadyExists) {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("staff already exists")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	staff := result.Staff

	ctx.JSON(http.StatusCreated, staffResponse{
		ID:         staff.ID,
		MerchantID: staff.MerchantID,
		UserID:     staff.UserID,
		Role:       staff.Role,
		Status:     staff.Status,
		CreatedAt:  staff.CreatedAt,
	})
}

type updateStaffRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=manager chef cashier"`
}

// updateMerchantStaffRole 更新员工角色
// @Summary 更新员工角色
// @Description 商户老板更新员工角色
// @Tags 员工管理
// @Accept json
// @Produce json
// @Param id path int true "员工ID"
// @Param request body updateStaffRoleRequest true "新角色"
// @Success 200 {object} staffResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 404 {object} ErrorResponse "员工不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/staff/{id}/role [patch]
// @Security BearerAuth
func (server *Server) updateMerchantStaffRole(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateStaffRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant not found in context")))
		return
	}

	// 更新角色
	result, err := server.store.AssignMerchantStaffRoleTx(ctx, db.AssignMerchantStaffRoleTxParams{
		MerchantID: merchant.ID,
		StaffID:    uriReq.ID,
		Role:       req.Role,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("staff not found")))
			return
		}
		if errors.Is(err, db.ErrMerchantStaffMerchantMismatch) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("staff does not belong to this merchant")))
			return
		}
		if errors.Is(err, db.ErrMerchantStaffOwnerMutation) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("cannot modify owner role")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	updatedStaff := result.Staff

	ctx.JSON(http.StatusOK, staffResponse{
		ID:         updatedStaff.ID,
		MerchantID: updatedStaff.MerchantID,
		UserID:     updatedStaff.UserID,
		Role:       updatedStaff.Role,
		Status:     updatedStaff.Status,
		CreatedAt:  updatedStaff.CreatedAt,
	})
}

// deleteMerchantStaff 移除员工
// @Summary 移除员工
// @Description 商户老板移除员工
// @Tags 员工管理
// @Produce json
// @Param id path int true "员工ID"
// @Success 200 {object} map[string]string "删除成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 404 {object} ErrorResponse "员工不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/staff/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteMerchantStaff(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant not found in context")))
		return
	}

	// 软删除员工记录（设置 status='disabled'）
	_, err := server.store.RemoveMerchantStaffTx(ctx, db.RemoveMerchantStaffTxParams{
		MerchantID: merchant.ID,
		StaffID:    uriReq.ID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("staff not found")))
			return
		}
		if errors.Is(err, db.ErrMerchantStaffMerchantMismatch) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("staff does not belong to this merchant")))
			return
		}
		if errors.Is(err, db.ErrMerchantStaffOwnerMutation) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("cannot remove owner")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, successMessage("staff removed successfully"))
}

type bindMerchantRequest struct {
	InviteCode string `json:"invite_code" binding:"required,len=32"`
}

// bindMerchant 员工扫码绑定商户
// @Summary 员工绑定商户
// @Description 员工通过邀请码绑定到商户
// @Tags 员工管理
// @Accept json
// @Produce json
// @Param request body bindMerchantRequest true "邀请码"
// @Success 200 {object} map[string]interface{} "绑定成功"
// @Failure 400 {object} ErrorResponse "参数错误或邀请码无效"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 409 {object} ErrorResponse "已绑定"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/bind-merchant [post]
// @Security BearerAuth
func (server *Server) bindMerchant(ctx *gin.Context) {
	var req bindMerchantRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 通过邀请码查找商户
	merchant, err := server.store.GetMerchantByBindCode(ctx, pgtype.Text{String: req.InviteCode, Valid: true})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid or expired invite code")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查邀请码是否过期
	if merchant.BindCodeExpiresAt.Valid && merchant.BindCodeExpiresAt.Time.Before(time.Now()) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invite code has expired")))
		return
	}

	// 检查用户是否已绑定该商户；disabled 记录允许通过有效邀请码重新进入 pending。
	existingStaff, err := server.store.GetMerchantStaff(ctx, db.GetMerchantStaffParams{
		MerchantID: merchant.ID,
		UserID:     authPayload.UserID,
	})
	if err == nil && existingStaff.Status != db.MerchantStaffStatusDisabled {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("you are already a staff of this merchant")))
		return
	}
	if err != nil && !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 创建员工记录（role='pending' 表示待分配角色，status='active' 表示在职）
	result, err := server.store.AddMerchantStaffTx(ctx, db.AddMerchantStaffTxParams{
		MerchantID: merchant.ID,
		UserID:     authPayload.UserID,
		Role:       "pending", // 无角色，等待老板分配
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, db.ErrMerchantStaffAlreadyExists) {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("you are already a staff of this merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	staff := result.Staff

	ctx.JSON(http.StatusOK, staffBindResponse{
		Message:      "successfully bound to merchant",
		MerchantID:   merchant.ID,
		MerchantName: merchant.Name,
		Role:         staff.Role,
	})
}

// isDuplicateKeyError 检查是否是重复 key 错误
func isDuplicateKeyError(err error) bool {
	return db.ErrorCode(err) == db.UniqueViolation
}
