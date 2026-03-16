package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

// ─────────────────────────────────────────────
// 响应结构
// ─────────────────────────────────────────────

type regionExpansionApplicationResponse struct {
	ID           int64  `json:"id"`
	OperatorID   int64  `json:"operator_id"`
	RegionID     int64  `json:"region_id"`
	RegionName   string `json:"region_name"`
	Status       string `json:"status"`
	RejectReason string `json:"reject_reason,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type regionExpansionApplicationsResponse struct {
	Applications interface{} `json:"applications"`
}

type adminRegionExpansionListResponse struct {
	Applications interface{} `json:"applications"`
	Total        int64       `json:"total"`
	Page         int32       `json:"page"`
	Limit        int32       `json:"limit"`
}

func newRegionExpansionResponse(app db.OperatorRegionApplication, regionName string) regionExpansionApplicationResponse {
	resp := regionExpansionApplicationResponse{
		ID:         app.ID,
		OperatorID: app.OperatorID,
		RegionID:   app.RegionID,
		RegionName: regionName,
		Status:     app.Status,
		CreatedAt:  app.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if app.RejectReason.Valid {
		resp.RejectReason = app.RejectReason.String
	}
	return resp
}

// ─────────────────────────────────────────────
// 运营商：提交区域扩展申请
// ─────────────────────────────────────────────

type applyRegionExpansionRequest struct {
	RegionID int64 `json:"region_id" binding:"required,gt=0"`
}

// applyOperatorRegionExpansion godoc
// @Summary 申请运营更多区域
// @Description 已入驻运营商申请扩展管理新区域，需后台审批
// @Tags 运营商
// @Accept json
// @Produce json
// @Param request body applyRegionExpansionRequest true "目标区域ID"
// @Success 201 {object} regionExpansionApplicationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/operator/region-expansion [post]
// @Security BearerAuth
func (server *Server) applyOperatorRegionExpansion(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	var req applyRegionExpansionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrTargetRegionRequired))
		return
	}

	// 获取运营商信息（依赖中间件已加载）
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		var err error
		operator, err = server.store.GetOperatorByUser(ctx, authPayload.UserID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(ErrNotOperator))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	// 验证区域存在
	region, err := server.store.GetRegion(ctx, req.RegionID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRegionNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查自己是否已经管理该区域
	manages, err := server.store.CheckOperatorManagesRegion(ctx, db.CheckOperatorManagesRegionParams{
		OperatorID: operator.ID,
		RegionID:   req.RegionID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if manages {
		ctx.JSON(http.StatusConflict, errorResponse(ErrRegionAlreadyManaged))
		return
	}

	// 检查是否已有待审核申请（利用 UNIQUE(operator_id, region_id) 约束兜底）
	existing, err := server.store.GetOperatorRegionApplicationByOperatorAndRegion(ctx, db.GetOperatorRegionApplicationByOperatorAndRegionParams{
		OperatorID: operator.ID,
		RegionID:   req.RegionID,
	})
	if err == nil {
		if existing.Status == "pending" {
			ctx.JSON(http.StatusConflict, errorResponse(ErrRegionExpansionPending))
			return
		}
		if existing.Status == "approved" {
			ctx.JSON(http.StatusConflict, errorResponse(ErrRegionAlreadyManaged))
			return
		}
		// 已拒绝：允许删除旧记录后重新申请
		if delErr := server.store.DeleteOperatorRegionApplication(ctx, existing.ID); delErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, delErr))
			return
		}
	} else if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 创建申请
	app, err := server.store.CreateOperatorRegionApplication(ctx, db.CreateOperatorRegionApplicationParams{
		OperatorID: operator.ID,
		RegionID:   req.RegionID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newRegionExpansionResponse(app, region.Name))
}

// ─────────────────────────────────────────────
// 运营商：查看自己的区域扩展申请列表
// ─────────────────────────────────────────────

// listOperatorRegionApplications godoc
// @Summary 获取自己的区域扩展申请列表
// @Tags 运营商
// @Produce json
// @Success 200 {array} regionExpansionApplicationResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/operator/region-expansion [get]
// @Security BearerAuth
func (server *Server) listOperatorRegionApplications(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		var err error
		operator, err = server.store.GetOperatorByUser(ctx, authPayload.UserID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(ErrNotOperator))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	rows, err := server.store.ListOperatorRegionApplicationsByOperator(ctx, operator.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]regionExpansionApplicationResponse, 0, len(rows))
	for _, row := range rows {
		app := db.OperatorRegionApplication{
			ID:           row.ID,
			OperatorID:   row.OperatorID,
			RegionID:     row.RegionID,
			Status:       row.Status,
			RejectReason: row.RejectReason,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		}
		resp = append(resp, newRegionExpansionResponse(app, row.RegionName))
	}
	ctx.JSON(http.StatusOK, regionExpansionApplicationsResponse{Applications: resp})
}

// ─────────────────────────────────────────────
// 管理后台：列出待审核的区域扩展申请
// ─────────────────────────────────────────────

type listPendingRegionExpansionQuery struct {
	Page  int32 `form:"page,default=1" binding:"min=1"`
	Limit int32 `form:"limit,default=20" binding:"min=1,max=100"`
}

type adminRegionExpansionApplicationResponse struct {
	ID           int64  `json:"id"`
	OperatorID   int64  `json:"operator_id"`
	OperatorName string `json:"operator_name"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	RegionID     int64  `json:"region_id"`
	RegionName   string `json:"region_name"`
	RegionCode   string `json:"region_code"`
	Status       string `json:"status"`
	RejectReason string `json:"reject_reason,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// listPendingRegionApplicationsAdmin godoc
// @Summary [管理] 列出区域扩展申请（支持状态过滤）
// @Tags 管理-运营商
// @Produce json
// @Param status query string false "状态过滤：pending/approved/rejected，不传返回全部"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/admin/operators/region-applications [get]
func (server *Server) listPendingRegionApplicationsAdmin(ctx *gin.Context) {
	var query listPendingRegionExpansionQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	statusFilter := ctx.Query("status") // "" 表示不过滤

	offset := (query.Page - 1) * query.Limit
	rows, err := server.store.ListAllRegionApplicationsAdmin(ctx, db.ListAllRegionApplicationsAdminParams{
		Limit:  query.Limit,
		Offset: offset,
		Status: pgtype.Text{String: statusFilter, Valid: statusFilter != ""},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountAllRegionApplicationsAdmin(ctx, pgtype.Text{String: statusFilter, Valid: statusFilter != ""})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]adminRegionExpansionApplicationResponse, 0, len(rows))
	for _, row := range rows {
		item := adminRegionExpansionApplicationResponse{
			ID:           row.ID,
			OperatorID:   row.OperatorID,
			OperatorName: row.OperatorName,
			ContactName:  row.ContactName,
			ContactPhone: row.ContactPhone,
			RegionID:     row.RegionID,
			RegionName:   row.RegionName,
			RegionCode:   row.RegionCode,
			Status:       row.Status,
			CreatedAt:    row.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if row.RejectReason.Valid {
			item.RejectReason = row.RejectReason.String
		}
		resp = append(resp, item)
	}
	ctx.JSON(http.StatusOK, adminRegionExpansionListResponse{Applications: resp, Total: total, Page: query.Page, Limit: query.Limit})
}

// ─────────────────────────────────────────────
// 管理后台：审批
// ─────────────────────────────────────────────

type regionExpansionApplicationIDRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// approveRegionApplicationAdmin godoc
// @Summary [管理] 审批通过区域扩展申请
// @Tags 管理-运营商
// @Produce json
// @Param id path int true "申请ID"
// @Success 200 {object} adminRegionExpansionApplicationResponse
// @Security BearerAuth
// @Router /v1/admin/operators/region-applications/{id}/approve [post]
func (server *Server) approveRegionApplicationAdmin(ctx *gin.Context) {
	var uriReq regionExpansionApplicationIDRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	app, err := server.store.GetOperatorRegionApplication(ctx, uriReq.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if app.Status != "pending" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotPending))
		return
	}

	// 检查区域是否已被占满（已有其他运营商管理）
	active, err := server.store.GetActiveOperatorByRegion(ctx, app.RegionID)
	if err == nil && active.ID != app.OperatorID {
		ctx.JSON(http.StatusConflict, errorResponse(ErrRegionHasOperator))
		return
	} else if err != nil && !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 将区域加入运营商
	if _, err := server.store.AddOperatorRegion(ctx, db.AddOperatorRegionParams{
		OperatorID: app.OperatorID,
		RegionID:   app.RegionID,
	}); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 更新申请状态
	approved, err := server.store.ApproveOperatorRegionApplication(ctx, uriReq.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionName := server.getRegionName(ctx, approved.RegionID)
	ctx.JSON(http.StatusOK, newRegionExpansionResponse(approved, regionName))
}

// rejectRegionApplicationAdmin godoc
// @Summary [管理] 拒绝区域扩展申请
// @Tags 管理-运营商
// @Accept json
// @Produce json
// @Param id path int true "申请ID"
// @Success 200 {object} adminRegionExpansionApplicationResponse
// @Security BearerAuth
// @Router /v1/admin/operators/region-applications/{id}/reject [post]
func (server *Server) rejectRegionApplicationAdmin(ctx *gin.Context) {
	var uriReq regionExpansionApplicationIDRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	type rejectReq struct {
		Reason string `json:"reject_reason" binding:"required"`
	}
	var body rejectReq
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrRejectionReasonRequired))
		return
	}

	app, err := server.store.GetOperatorRegionApplication(ctx, uriReq.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if app.Status != "pending" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotPending))
		return
	}

	rejected, err := server.store.RejectOperatorRegionApplication(ctx, db.RejectOperatorRegionApplicationParams{
		ID:           uriReq.ID,
		RejectReason: pgtype.Text{String: body.Reason, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionName := server.getRegionName(ctx, rejected.RegionID)
	ctx.JSON(http.StatusOK, newRegionExpansionResponse(rejected, regionName))
}
