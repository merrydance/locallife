package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

// ==================== 申诉管理 (运营商视角) ====================

// 申诉列表接口已在 appeal.go 中实现 (listOperatorAppeals)

// ==================== 食安熔断 ====================

type submitSafetyReportRequest struct {
	Title       string   `json:"title" binding:"required,min=5,max=100"`
	Description string   `json:"description" binding:"required,min=10"`
	MerchantIDs []int64  `json:"merchant_ids" binding:"omitempty"` // 涉及商户
	Images      []string `json:"images" binding:"omitempty"`
	Level       string   `json:"level" binding:"required,oneof=low medium high critical"`
}

// submitSafetyReport 提交食安熔断报告
// @Summary 提交食安报告
// @Description 运营商提交食品安全事件报告，可能触发熔断机制
// @Tags 运营商功能
// @Accept json
// @Produce json
// @Param request body submitSafetyReportRequest true "报告内容"
// @Success 200 {object} MessageResponse "提交成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "无权限"
// @Security BearerAuth
// @Router /v1/operator/reports/safety [post]
func (server *Server) submitSafetyReport(ctx *gin.Context) {
	var req submitSafetyReportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	regionID, err := server.getOperatorRegionID(ctx)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	// 1. 获取当前用户ID
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 构建参数
	arg := db.CreateSafetyReportParams{
		ReporterID:  authPayload.UserID,
		RegionID:    regionID,
		Title:       req.Title,
		Description: req.Description,
		Level:       req.Level,
		Status:      "pending",
		MerchantIds: req.MerchantIDs,
		Images:      req.Images,
	}

	// 2. 存储报告
	_, err = server.store.CreateSafetyReport(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// 3. 触发熔断 (如果级别是 critical)
	if req.Level == "critical" {
		// 触发区域熔断：将区域状态设置为 suspended
		err = server.store.SuspendRegion(ctx, regionID)
		if err != nil {
			// 熔断操作失败应该作为严重错误记录，但可能不阻断报告提交的响应？
			// 或者返回部分成功？为了数据一致性，这里记录错误并返回给前端 warning，或者直接报错。
			// 考虑到安全性优先，这里记录错误但告知用户报告已提交但熔断失败可能需要人工介入
			// 简单起见，这里直接返回错误
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}
	}

	ctx.JSON(http.StatusOK, MessageResponse{
		Message: "Safety report submitted successfully",
	})
}
