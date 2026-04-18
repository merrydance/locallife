package api

import (
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

type claimRecoveryPaymentResponse struct {
	Recovery       claimRecoveryResponse `json:"recovery"`
	PaymentOrderID int64                 `json:"payment_order_id"`
	OutTradeNo     string                `json:"out_trade_no"`
	Amount         int64                 `json:"amount"`
	Status         string                `json:"status"`
	ExpiresAt      *time.Time            `json:"expires_at,omitempty"`
	PayParams      *miniProgramPayParams `json:"pay_params,omitempty"`
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

func newClaimRecoveryPaymentResponse(result logic.ClaimRecoveryPaymentResult) claimRecoveryPaymentResponse {
	resp := claimRecoveryPaymentResponse{
		Recovery:       newClaimRecoveryResponse(result.Recovery),
		PaymentOrderID: result.PaymentOrder.ID,
		OutTradeNo:     result.PaymentOrder.OutTradeNo,
		Amount:         result.PaymentOrder.Amount,
		Status:         result.PaymentOrder.Status,
	}
	if result.PaymentOrder.ExpiresAt.Valid {
		resp.ExpiresAt = &result.PaymentOrder.ExpiresAt.Time
	}
	if result.PayParams != nil {
		resp.PayParams = &miniProgramPayParams{
			TimeStamp: result.PayParams.TimeStamp,
			NonceStr:  result.PayParams.NonceStr,
			Package:   result.PayParams.Package,
			SignType:  result.PayParams.SignType,
			PaySign:   result.PayParams.PaySign,
		}
	}
	return resp
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

	regionIDs, err := server.listManagedOperatorRegionIDs(ctx)
	if err != nil {
		server.respondOperatorRegionSelectionError(ctx, err)
		return
	}

	recovery, err := logic.GetClaimRecoveryForOperator(ctx, server.store, logic.OperatorClaimRecoveryInput{
		ClaimID:   claimID,
		RegionIDs: regionIDs,
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

// payMerchantClaimRecovery 商户创建追偿支付单
// @Summary 商户创建追偿支付单
// @Description 商户为索赔追偿创建微信支付单，支付成功后系统再标记追偿单为已支付
// @Tags 商户索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "索赔ID"
// @Success 200 {object} claimRecoveryPaymentResponse "支付单创建成功"
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

	result, err := logic.CreateMerchantClaimRecoveryPayment(ctx, server.store, server.directPaymentClient, logic.CreateMerchantClaimRecoveryPaymentInput{
		ClaimID:     claimID,
		MerchantID:  merchant.ID,
		PayerUserID: authPayload.UserID,
		ClientIP:    ctx.ClientIP(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newClaimRecoveryPaymentResponse(result))
}

// payRiderClaimRecovery 骑手创建追偿支付单
// @Summary 骑手创建追偿支付单
// @Description 骑手为索赔追偿创建微信支付单，支付成功后系统再标记追偿单为已支付
// @Tags 骑手索赔管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "索赔ID"
// @Success 200 {object} claimRecoveryPaymentResponse "支付单创建成功"
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

	result, err := logic.CreateRiderClaimRecoveryPayment(ctx, server.store, server.directPaymentClient, logic.CreateRiderClaimRecoveryPaymentInput{
		ClaimID:     claimID,
		RiderID:     rider.ID,
		PayerUserID: authPayload.UserID,
		ClientIP:    ctx.ClientIP(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newClaimRecoveryPaymentResponse(result))
}
