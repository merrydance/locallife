package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
)

type packagingPolicyCandidateDishResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Price       int64  `json:"price"`
	IsAvailable bool   `json:"is_available"`
	IsOnline    bool   `json:"is_online"`
}

type packagingPolicyResponse struct {
	MerchantID           int64                                  `json:"merchant_id"`
	ApplicableOrderTypes []string                               `json:"applicable_order_types"`
	CandidateDishIDs     []int64                                `json:"candidate_dish_ids"`
	CandidateDishes      []packagingPolicyCandidateDishResponse `json:"candidate_dishes"`
}

type updatePackagingPolicyRequest struct {
	ApplicableOrderTypes []string `json:"applicable_order_types" binding:"omitempty,dive,oneof=takeout takeaway"`
	CandidateDishIDs     []int64  `json:"candidate_dish_ids" binding:"omitempty,dive,min=1"`
}

func newPackagingPolicyResponse(result logic.PackagingPolicyResult) packagingPolicyResponse {
	resp := packagingPolicyResponse{
		MerchantID:           result.MerchantID,
		ApplicableOrderTypes: append([]string{}, result.ApplicableOrderTypes...),
		CandidateDishIDs:     append([]int64{}, result.CandidateDishIDs...),
		CandidateDishes:      make([]packagingPolicyCandidateDishResponse, 0, len(result.CandidateDishes)),
	}

	for _, dish := range result.CandidateDishes {
		resp.CandidateDishes = append(resp.CandidateDishes, packagingPolicyCandidateDishResponse{
			ID:          dish.ID,
			Name:        dish.Name,
			Price:       dish.Price,
			IsAvailable: dish.IsAvailable,
			IsOnline:    dish.IsOnline,
		})
	}

	return resp
}

// getMerchantPackagingPolicy godoc
// @Summary 获取商户包装策略
// @Description 获取当前商户的订单级包装策略配置
// @Tags 商户
// @Produce json
// @Success 200 {object} packagingPolicyResponse "包装策略"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "仅老板可操作"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/me/packaging-policy [get]
// @Security BearerAuth
func (server *Server) getMerchantPackagingPolicy(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	if _, err := server.requireOwnedMerchantForUser(ctx, authPayload.UserID); err != nil {
		if writeMerchantSelectionError(ctx, err) {
			return
		}
		if errors.Is(err, errMerchantOwnerRequired) {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result, err := logic.GetPackagingPolicyForOwner(ctx, server.store, authPayload.UserID)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newPackagingPolicyResponse(result))
}

// updateMerchantPackagingPolicy godoc
// @Summary 更新商户包装策略
// @Description 覆盖更新当前商户的订单级包装策略配置
// @Tags 商户
// @Accept json
// @Produce json
// @Param request body updatePackagingPolicyRequest true "包装策略"
// @Success 200 {object} packagingPolicyResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "仅老板可操作"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/me/packaging-policy [put]
// @Security BearerAuth
func (server *Server) updateMerchantPackagingPolicy(ctx *gin.Context) {
	var req updatePackagingPolicyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	if _, err := server.requireOwnedMerchantForUser(ctx, authPayload.UserID); err != nil {
		if writeMerchantSelectionError(ctx, err) {
			return
		}
		if errors.Is(err, errMerchantOwnerRequired) {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result, err := logic.UpdatePackagingPolicyForOwner(ctx, server.store, logic.UpdatePackagingPolicyInput{
		OwnerUserID:          authPayload.UserID,
		ApplicableOrderTypes: req.ApplicableOrderTypes,
		CandidateDishIDs:     req.CandidateDishIDs,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newPackagingPolicyResponse(result))
}
