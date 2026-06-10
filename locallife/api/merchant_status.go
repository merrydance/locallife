package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/rs/zerolog/log"
)

type updateMerchantStatusRequest struct {
	IsOpen      *bool  `json:"is_open" binding:"required"`               // true=开店营业, false=打烊
	AutoCloseAt string `json:"auto_close_at" binding:"omitempty,max=50"` // 可选，自动打烊时间 (RFC3339格式)
}

type merchantStatusResponse struct {
	IsOpen            bool                              `json:"is_open"`
	AutoCloseAt       *time.Time                        `json:"auto_close_at,omitempty"`
	Message           string                            `json:"message"`
	SettlementAccount *baofuSettlementReadinessResponse `json:"settlement_account,omitempty"`
}

// updateMerchantOpenStatus godoc
// @Summary 更新商户营业状态
// @Description 商户设置开店/打烊状态，可设置自动打烊时间
// @Tags 商户
// @Accept json
// @Produce json
// @Param request body updateMerchantStatusRequest true "状态更新"
// @Success 200 {object} merchantStatusResponse "更新后的状态"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "商户被暂停或无权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/me/status [patch]
// @Security BearerAuth
func (server *Server) updateMerchantOpenStatus(ctx *gin.Context) {
	var req updateMerchantStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if writeMerchantSelectionError(ctx, err) {
			return
		}
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	merchantProfile, err := server.store.GetMerchantProfile(ctx, merchant.ID)
	if err == nil && merchantProfile.IsSuspended {
		ctx.JSON(http.StatusForbidden, merchantSuspendedResponse{
			Error:         "merchant is suspended due to food safety issues",
			SuspendReason: merchantProfile.SuspendReason.String,
			SuspendUntil:  merchantProfile.SuspendUntil.Time,
		})
		return
	}

	if *req.IsOpen {
		if err := server.ensureMerchantBaofuPaymentReady(ctx, merchant); err != nil {
			if errors.Is(err, errMerchantBaofuAccountMissing) || errors.Is(err, errMerchantBaofuWechatChannelPending) {
				ctx.JSON(http.StatusBadRequest, errorResponse(err))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if err := server.ensureMerchantPaymentConfigReady(ctx, merchant.ID); err != nil {
			if errors.Is(err, errMerchantPaymentConfigInactive) {
				ctx.JSON(http.StatusBadRequest, errorResponse(err))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	var autoCloseAt pgtype.Timestamptz
	if req.AutoCloseAt != "" && *req.IsOpen {
		t, err := time.Parse(time.RFC3339, req.AutoCloseAt)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid auto_close_at format, use RFC3339")))
			return
		}
		if t.Before(time.Now()) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("auto_close_at must be in the future")))
			return
		}
		autoCloseAt = pgtype.Timestamptz{Time: t, Valid: true}
	}

	manualOpenStatusUntil, err := server.manualOpenStatusUntil(ctx, merchant)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	updatedMerchant, err := server.store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{
		ID:                    merchant.ID,
		IsOpen:                *req.IsOpen,
		AutoCloseAt:           autoCloseAt,
		ManualOpenStatusUntil: manualOpenStatusUntil,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if server.merchantStatusChangePublisher != nil {
		autoCloseAtPtr := pgTimeToPtr(updatedMerchant.AutoCloseAt)
		if err := server.merchantStatusChangePublisher.PublishMerchantStatusChange(ctx, updatedMerchant.ID, updatedMerchant.IsOpen, autoCloseAtPtr, "manual"); err != nil {
			log.Error().Err(err).
				Int64("merchant_id", updatedMerchant.ID).
				Msg("failed to publish merchant status change event")
		}
	}

	message := "店铺已打烊"
	if *req.IsOpen {
		message = "店铺已开始营业"
		if autoCloseAt.Valid {
			message = fmt.Sprintf("店铺已开始营业，将于 %s 自动打烊", autoCloseAt.Time.Format("15:04"))
		}
	}

	resp := merchantStatusResponse{
		IsOpen:  *req.IsOpen,
		Message: message,
	}
	if updatedMerchant.AutoCloseAt.Valid {
		resp.AutoCloseAt = &updatedMerchant.AutoCloseAt.Time
	}

	ctx.JSON(http.StatusOK, resp)
}

// getMerchantOpenStatus godoc
// @Summary 获取商户营业状态
// @Description 获取当前商户的开店/打烊状态
// @Tags 商户
// @Produce json
// @Success 200 {object} merchantStatusResponse "营业状态"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/me/status [get]
// @Security BearerAuth
func (server *Server) getMerchantOpenStatus(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if writeMerchantSelectionError(ctx, err) {
			return
		}
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	status, err := server.store.GetMerchantIsOpen(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	message := "店铺已打烊"
	if status.IsOpen {
		message = "店铺营业中"
		if status.AutoCloseAt.Valid {
			message = fmt.Sprintf("店铺营业中，将于 %s 自动打烊", status.AutoCloseAt.Time.Format("15:04"))
		}
	}

	resp := merchantStatusResponse{
		IsOpen:  status.IsOpen,
		Message: message,
	}
	if status.AutoCloseAt.Valid {
		resp.AutoCloseAt = &status.AutoCloseAt.Time
	}
	readiness, err := server.getMerchantBaofuSettlementReadiness(ctx, merchant)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	resp.SettlementAccount = newBaofuSettlementReadinessResponse(readiness)

	ctx.JSON(http.StatusOK, resp)
}
