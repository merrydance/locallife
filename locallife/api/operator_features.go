package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
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

type listSafetyReportsRequest struct {
	Page   int32  `form:"page" binding:"omitempty,min=1"`
	Limit  int32  `form:"limit" binding:"omitempty,min=1,max=100"`
	Status string `form:"status" binding:"omitempty,oneof=pending resolved rejected"`
}

type resolveSafetyReportRequest struct {
	Status             string  `json:"status" binding:"required,oneof=resolved rejected"`
	ResolutionNotes    string  `json:"resolution_notes" binding:"required,min=5,max=1000"`
	RecoverMerchantIDs []int64 `json:"recover_merchant_ids" binding:"omitempty"`
	RecoverReason      string  `json:"recover_reason" binding:"omitempty,min=2,max=500"`
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
		Status:      SafetyReportStatusPending,
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
	if req.Level == SafetyReportLevelCritical {
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

// listSafetyReports 列表查询运营商区域食安事件
// @Summary 获取食安事件列表
// @Description 按运营商所属区域分页查询食安事件
// @Tags 运营商功能
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param limit query int false "每页条数"
// @Param status query string false "状态" Enums(pending,resolved,rejected)
// @Success 200 {object} map[string]interface{} "查询成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "无权限"
// @Security BearerAuth
// @Router /v1/operator/reports/safety [get]
func (server *Server) listSafetyReports(ctx *gin.Context) {
	var req listSafetyReportsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	regionID, err := server.getOperatorRegionID(ctx)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	offset := (req.Page - 1) * req.Limit
	if req.Status == "" {
		reports, err := server.store.ListSafetyReportsByRegion(ctx, db.ListSafetyReportsByRegionParams{
			RegionID: regionID,
			Limit:    req.Limit,
			Offset:   offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}

		total, err := server.store.CountSafetyReportsByRegion(ctx, regionID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}

		hasMore := int64(offset)+int64(len(reports)) < total

		ctx.JSON(http.StatusOK, gin.H{
			"items":    reports,
			"page":     req.Page,
			"limit":    req.Limit,
			"has_more": hasMore,
			"total":    total,
		})
		return
	}

	reports, err := server.store.ListSafetyReportsByRegionAndStatus(ctx, db.ListSafetyReportsByRegionAndStatusParams{
		RegionID: regionID,
		Status:   req.Status,
		Limit:    req.Limit,
		Offset:   offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	total, err := server.store.CountSafetyReportsByRegionAndStatus(ctx, db.CountSafetyReportsByRegionAndStatusParams{
		RegionID: regionID,
		Status:   req.Status,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	hasMore := int64(offset)+int64(len(reports)) < total

	ctx.JSON(http.StatusOK, gin.H{
		"items":    reports,
		"page":     req.Page,
		"limit":    req.Limit,
		"has_more": hasMore,
		"total":    total,
	})
}

// getSafetyReportDetail 获取食安事件详情
// @Summary 获取食安事件详情
// @Description 获取运营商区域内食安事件详情
// @Tags 运营商功能
// @Accept json
// @Produce json
// @Param id path int true "事件ID"
// @Success 200 {object} db.SafetyReport "查询成功"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "未找到"
// @Security BearerAuth
// @Router /v1/operator/reports/safety/{id} [get]
func (server *Server) getSafetyReportDetail(ctx *gin.Context) {
	reportID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	regionID, err := server.getOperatorRegionID(ctx)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	report, err := server.store.GetSafetyReport(ctx, reportID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	if report.RegionID != regionID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("forbidden")))
		return
	}

	ctx.JSON(http.StatusOK, report)
}

// resolveSafetyReport 处理食安事件并可恢复商户上线
// @Summary 处理食安事件
// @Description 运营商填写处置报告，支持恢复商户上线
// @Tags 运营商功能
// @Accept json
// @Produce json
// @Param id path int true "事件ID"
// @Param request body resolveSafetyReportRequest true "处理信息"
// @Success 200 {object} map[string]interface{} "处理成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "未找到"
// @Security BearerAuth
// @Router /v1/operator/reports/safety/{id}/resolve [post]
func (server *Server) resolveSafetyReport(ctx *gin.Context) {
	reportID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req resolveSafetyReportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if len(req.RecoverMerchantIDs) > 0 && req.RecoverReason == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("recover_reason is required when recover_merchant_ids is provided")))
		return
	}

	regionID, err := server.getOperatorRegionID(ctx)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	report, err := server.store.GetSafetyReport(ctx, reportID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	if report.RegionID != regionID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("forbidden")))
		return
	}

	updated, err := server.store.UpdateSafetyReportStatus(ctx, db.UpdateSafetyReportStatusParams{
		ID:     reportID,
		Status: req.Status,
		ResolutionNotes: pgtype.Text{
			String: req.ResolutionNotes,
			Valid:  true,
		},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	recoveredMerchantIDs := make([]int64, 0, len(req.RecoverMerchantIDs))
	for _, merchantID := range req.RecoverMerchantIDs {
		merchant, merchantErr := server.store.GetMerchant(ctx, merchantID)
		if merchantErr != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(merchantErr))
			return
		}
		if merchant.RegionID != regionID {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("forbidden")))
			return
		}

		if merchantErr = server.store.UnsuspendMerchant(ctx, merchantID); merchantErr != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(merchantErr))
			return
		}
		if merchantErr = server.store.UnsuspendMerchantTakeout(ctx, merchantID); merchantErr != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(merchantErr))
			return
		}
		if _, merchantErr = server.store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchantID, Status: "active"}); merchantErr != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(merchantErr))
			return
		}
		recoveredMerchantIDs = append(recoveredMerchantIDs, merchantID)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"report":                 updated,
		"recovered_merchant_ids": recoveredMerchantIDs,
	})
}
