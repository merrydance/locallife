package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
)

type claimRecoveryResponse struct {
	ID               int64     `json:"id"`
	ClaimID          int64     `json:"claim_id"`
	OrderID          int64     `json:"order_id"`
	ResponsibleParty string    `json:"responsible_party"`
	RecoveryTarget   *string   `json:"recovery_target,omitempty"`
	RecoveryAmount   int64     `json:"recovery_amount"`
	Status           string    `json:"status"`
	DueAt            time.Time `json:"due_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func newClaimRecoveryResponse(recovery db.ClaimRecovery) claimRecoveryResponse {
	var target *string
	if recovery.RecoveryTarget.Valid {
		value := recovery.RecoveryTarget.String
		target = &value
	}

	return claimRecoveryResponse{
		ID:               recovery.ID,
		ClaimID:          recovery.ClaimID,
		OrderID:          recovery.OrderID,
		ResponsibleParty: recovery.ResponsibleParty,
		RecoveryTarget:   target,
		RecoveryAmount:   recovery.RecoveryAmount,
		Status:           recovery.Status,
		DueAt:            recovery.DueAt,
		UpdatedAt:        recovery.UpdatedAt,
	}
}

// getMerchantClaimRecovery 商户查看追偿单
// @Summary 商户查看追偿单
// @Description 商户查看索赔对应的追偿单状态
// @Tags 商户索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "索赔ID"
// @Success 200 {object} claimRecoveryResponse "追偿单详情"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户或索赔不属于该商户"
// @Failure 404 {object} map[string]interface{} "追偿单不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/claims/{id}/recovery [get]
func (server *Server) getMerchantClaimRecovery(ctx *gin.Context) {
	claimID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	recovery, err := logic.GetClaimRecoveryForMerchant(ctx, server.store, logic.MerchantClaimRecoveryInput{
		ClaimID:    claimID,
		MerchantID: merchant.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newClaimRecoveryResponse(recovery))
}

// getRiderClaimRecovery 骑手查看追偿单
// @Summary 骑手查看追偿单
// @Description 骑手查看索赔对应的追偿单状态
// @Tags 骑手索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "索赔ID"
// @Success 200 {object} claimRecoveryResponse "追偿单详情"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非骑手用户或索赔不属于该骑手"
// @Failure 404 {object} map[string]interface{} "追偿单不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/rider/claims/{id}/recovery [get]
func (server *Server) getRiderClaimRecovery(ctx *gin.Context) {
	claimID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.getRiderFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	recovery, err := logic.GetClaimRecoveryForRider(ctx, server.store, logic.RiderClaimRecoveryInput{
		ClaimID: claimID,
		RiderID: rider.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newClaimRecoveryResponse(recovery))
}

// getOperatorClaimRecovery 运营商查看追偿单
// @Summary 运营商查看追偿单
// @Description 运营商查看索赔对应的追偿单状态
// @Tags 运营商申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "索赔ID"
// @Success 200 {object} claimRecoveryResponse "追偿单详情"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "无权限处理该区域"
// @Failure 404 {object} map[string]interface{} "追偿单不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/operator/claims/{id}/recovery [get]
func (server *Server) getOperatorClaimRecovery(ctx *gin.Context) {
	claimID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator not found")))
		return
	}

	recovery, err := logic.GetClaimRecoveryForOperator(ctx, server.store, logic.OperatorClaimRecoveryInput{
		ClaimID:  claimID,
		RegionID: operator.RegionID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newClaimRecoveryResponse(recovery))
}

// payMerchantClaimRecovery 商户支付追偿单
// @Summary 商户支付追偿单
// @Description 商户确认已支付索赔追偿，系统标记为已支付并恢复接单限制
// @Tags 商户索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "索赔ID"
// @Success 200 {object} claimRecoveryResponse "已支付"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非商户用户或索赔不属于该商户"
// @Failure 404 {object} map[string]interface{} "追偿单不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/claims/{id}/recovery/pay [post]
func (server *Server) payMerchantClaimRecovery(ctx *gin.Context) {
	claimID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	updated, err := logic.PayMerchantClaimRecovery(ctx, server.store, logic.PayMerchantClaimRecoveryInput{
		ClaimID:    claimID,
		MerchantID: merchant.ID,
		Now:        time.Now(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newClaimRecoveryResponse(updated))
}

// payRiderClaimRecovery 骑手支付追偿单
// @Summary 骑手支付追偿单
// @Description 骑手确认已支付索赔追偿，系统标记为已支付并恢复接单限制
// @Tags 骑手索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "索赔ID"
// @Success 200 {object} claimRecoveryResponse "已支付"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "非骑手用户或索赔不属于该骑手"
// @Failure 404 {object} map[string]interface{} "追偿单不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/rider/claims/{id}/recovery/pay [post]
func (server *Server) payRiderClaimRecovery(ctx *gin.Context) {
	claimID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.getRiderFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	updated, err := logic.PayRiderClaimRecovery(ctx, server.store, logic.PayRiderClaimRecoveryInput{
		ClaimID: claimID,
		RiderID: rider.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newClaimRecoveryResponse(updated))
}

// waiveClaimRecovery 运营商核销追偿单
// @Summary 运营商核销追偿单
// @Description 运营商核销或免除追偿，系统标记为已核销并恢复接单限制
// @Tags 运营商申诉管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "索赔ID"
// @Success 200 {object} claimRecoveryResponse "已核销"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "无权限处理该区域"
// @Failure 404 {object} map[string]interface{} "追偿单不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/operator/claims/{id}/recovery/waive [post]
func (server *Server) waiveClaimRecovery(ctx *gin.Context) {
	claimID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator not found")))
		return
	}

	updated, err := logic.WaiveClaimRecovery(ctx, server.store, logic.WaiveClaimRecoveryInput{
		ClaimID:  claimID,
		RegionID: operator.RegionID,
		Now:      time.Now(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newClaimRecoveryResponse(updated))
}
