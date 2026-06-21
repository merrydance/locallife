package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type listOperatorFoodSafetyCasesRequest struct {
	Page   int32  `form:"page" binding:"omitempty,min=1"`
	Limit  int32  `form:"limit" binding:"omitempty,min=1,max=100"`
	Status string `form:"status" binding:"omitempty,oneof=merchant-suspended investigating resolved"`
}

type investigateOperatorFoodSafetyCaseRequest struct {
	InvestigationReport string `json:"investigation_report" binding:"required,min=10,max=4000"`
}

type resolveOperatorFoodSafetyCaseRequest struct {
	InvestigationReport         string `json:"investigation_report" binding:"omitempty,min=10,max=4000"`
	MerchantRectificationReport string `json:"merchant_rectification_report" binding:"required,min=10,max=4000"`
	Resolution                  string `json:"resolution" binding:"required,min=5,max=2000"`
}

type foodSafetyCaseListResponse struct {
	Items   []operatorFoodSafetyCaseResponse `json:"items"`
	Page    int32                            `json:"page"`
	Limit   int32                            `json:"limit"`
	HasMore bool                             `json:"has_more"`
	Total   int64                            `json:"total"`
}

type operatorFoodSafetyCaseResponse struct {
	ID                          int64   `json:"id"`
	MerchantID                  int64   `json:"merchant_id"`
	RegionID                    int64   `json:"region_id"`
	PrimaryProductKey           string  `json:"primary_product_key"`
	PrimaryProductLabel         string  `json:"primary_product_label"`
	Status                      string  `json:"status"`
	TriggerReason               string  `json:"trigger_reason"`
	InvestigationReport         *string `json:"investigation_report,omitempty"`
	MerchantRectificationReport *string `json:"merchant_rectification_report,omitempty"`
	Resolution                  *string `json:"resolution,omitempty"`
	SuspendedAt                 string  `json:"suspended_at"`
	ResolvedAt                  *string `json:"resolved_at,omitempty"`
	CreatedAt                   string  `json:"created_at"`
	UpdatedAt                   string  `json:"updated_at"`
}

type operatorFoodSafetyIncidentResponse struct {
	ID                  int64   `json:"id"`
	OrderID             int64   `json:"order_id"`
	MerchantID          int64   `json:"merchant_id"`
	UserID              int64   `json:"user_id"`
	IncidentType        string  `json:"incident_type"`
	Description         string  `json:"description"`
	Status              string  `json:"status"`
	PrimaryProductKey   string  `json:"primary_product_key"`
	PrimaryProductLabel string  `json:"primary_product_label"`
	CaseID              *int64  `json:"case_id,omitempty"`
	CreatedAt           string  `json:"created_at"`
	ResolvedAt          *string `json:"resolved_at,omitempty"`
}

type foodSafetyCaseDetailResponse struct {
	Case      operatorFoodSafetyCaseResponse       `json:"case"`
	Incidents []operatorFoodSafetyIncidentResponse `json:"incidents"`
}

func nullableText(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullableTime(value pgtype.Timestamptz) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.Format(time.RFC3339)
	return &formatted
}

func nullableInt64(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func newOperatorFoodSafetyCaseResponse(item db.FoodSafetyCase) operatorFoodSafetyCaseResponse {
	return operatorFoodSafetyCaseResponse{
		ID:                          item.ID,
		MerchantID:                  item.MerchantID,
		RegionID:                    item.RegionID,
		PrimaryProductKey:           item.PrimaryProductKey,
		PrimaryProductLabel:         item.PrimaryProductLabel,
		Status:                      item.Status,
		TriggerReason:               item.TriggerReason,
		InvestigationReport:         nullableText(item.InvestigationReport),
		MerchantRectificationReport: nullableText(item.MerchantRectificationReport),
		Resolution:                  nullableText(item.Resolution),
		SuspendedAt:                 item.SuspendedAt.Format(time.RFC3339),
		ResolvedAt:                  nullableTime(item.ResolvedAt),
		CreatedAt:                   item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                   item.UpdatedAt.Format(time.RFC3339),
	}
}

func newOperatorFoodSafetyIncidentResponse(item db.ListFoodSafetyIncidentsByCaseRow) operatorFoodSafetyIncidentResponse {
	return operatorFoodSafetyIncidentResponse{
		ID:                  item.ID,
		OrderID:             item.OrderID,
		MerchantID:          item.MerchantID,
		UserID:              item.UserID,
		IncidentType:        item.IncidentType,
		Description:         item.Description,
		Status:              item.Status,
		PrimaryProductKey:   item.PrimaryProductKey,
		PrimaryProductLabel: item.PrimaryProductLabel,
		CaseID:              nullableInt64(item.CaseID),
		CreatedAt:           item.CreatedAt.Format(time.RFC3339),
		ResolvedAt:          nullableTime(item.ResolvedAt),
	}
}

// listOperatorFoodSafetyCases godoc
// @Summary 获取食安案件列表
// @Description 获取当前运营商辖区内由顾客食安上报触发的案件列表
// @Tags 运营商功能
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param limit query int false "每页数量"
// @Param status query string false "案件状态" Enums(merchant-suspended,investigating,resolved)
// @Success 200 {object} foodSafetyCaseListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/operator/food-safety/cases [get]
func (server *Server) listOperatorFoodSafetyCases(ctx *gin.Context) {
	var req listOperatorFoodSafetyCasesRequest
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

	selection, err := server.resolveOperatorRegionSelection(ctx)
	if err != nil {
		server.respondOperatorRegionSelectionError(ctx, err)
		return
	}

	offset := (req.Page - 1) * req.Limit
	if req.Status == "" {
		cases, err := server.store.ListFoodSafetyCasesByRegions(ctx, db.ListFoodSafetyCasesByRegionsParams{
			RegionIds: selection.RegionIDs,
			Limit:     req.Limit,
			Offset:    offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		total, err := server.store.CountFoodSafetyCasesByRegions(ctx, selection.RegionIDs)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		items := make([]operatorFoodSafetyCaseResponse, 0, len(cases))
		for _, item := range cases {
			items = append(items, newOperatorFoodSafetyCaseResponse(item))
		}
		hasMore := int64(offset)+int64(len(cases)) < total
		ctx.JSON(http.StatusOK, foodSafetyCaseListResponse{Items: items, Page: req.Page, Limit: req.Limit, HasMore: hasMore, Total: total})
		return
	}

	cases, err := server.store.ListFoodSafetyCasesByRegionsAndStatus(ctx, db.ListFoodSafetyCasesByRegionsAndStatusParams{
		RegionIds: selection.RegionIDs,
		Status:    req.Status,
		Limit:     req.Limit,
		Offset:    offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountFoodSafetyCasesByRegionsAndStatus(ctx, db.CountFoodSafetyCasesByRegionsAndStatusParams{
		RegionIds: selection.RegionIDs,
		Status:    req.Status,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	items := make([]operatorFoodSafetyCaseResponse, 0, len(cases))
	for _, item := range cases {
		items = append(items, newOperatorFoodSafetyCaseResponse(item))
	}
	hasMore := int64(offset)+int64(len(cases)) < total
	ctx.JSON(http.StatusOK, foodSafetyCaseListResponse{Items: items, Page: req.Page, Limit: req.Limit, HasMore: hasMore, Total: total})
}

// getOperatorFoodSafetyCase godoc
// @Summary 获取食安案件详情
// @Description 获取当前运营商辖区内的单个食安案件和关联上报事件
// @Tags 运营商功能
// @Accept json
// @Produce json
// @Param id path int true "案件ID"
// @Success 200 {object} foodSafetyCaseDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/operator/food-safety/cases/{id} [get]
func (server *Server) getOperatorFoodSafetyCase(ctx *gin.Context) {
	caseID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	caseRecord, err := server.store.GetFoodSafetyCase(ctx, caseID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if _, err := server.checkOperatorManagesRegion(ctx, caseRecord.RegionID); err != nil {
		server.respondOperatorRegionSelectionError(ctx, err)
		return
	}

	incidents, err := server.store.ListFoodSafetyIncidentsByCase(ctx, pgtype.Int8{Int64: caseID, Valid: true})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	incidentItems := make([]operatorFoodSafetyIncidentResponse, 0, len(incidents))
	for _, item := range incidents {
		incidentItems = append(incidentItems, newOperatorFoodSafetyIncidentResponse(item))
	}

	ctx.JSON(http.StatusOK, foodSafetyCaseDetailResponse{Case: newOperatorFoodSafetyCaseResponse(caseRecord), Incidents: incidentItems})
}

// investigateOperatorFoodSafetyCase godoc
// @Summary 填写食安案件调查报告
// @Description 运营商对顾客食安上报触发的案件录入调查报告，并将案件置为调查中
// @Tags 运营商功能
// @Accept json
// @Produce json
// @Param id path int true "案件ID"
// @Param request body investigateOperatorFoodSafetyCaseRequest true "调查报告"
// @Success 200 {object} operatorFoodSafetyCaseResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/operator/food-safety/cases/{id}/investigate [post]
func (server *Server) investigateOperatorFoodSafetyCase(ctx *gin.Context) {
	caseID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req investigateOperatorFoodSafetyCaseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	caseRecord, err := server.store.GetFoodSafetyCase(ctx, caseID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if _, err := server.checkOperatorManagesRegion(ctx, caseRecord.RegionID); err != nil {
		server.respondOperatorRegionSelectionError(ctx, err)
		return
	}
	if caseRecord.Status == "resolved" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("resolved case cannot be investigated again")))
		return
	}

	investigationReport := strings.TrimSpace(req.InvestigationReport)
	if investigationReport == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("investigation_report cannot be blank")))
		return
	}

	updated, err := server.store.UpdateFoodSafetyCaseInvestigation(ctx, db.UpdateFoodSafetyCaseInvestigationParams{
		ID:                  caseID,
		InvestigationReport: pgtype.Text{String: investigationReport, Valid: true},
	})
	if err != nil {
		if isNotFoundError(err) || errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("resolved case cannot be investigated again")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newOperatorFoodSafetyCaseResponse(updated))
}

// resolveOperatorFoodSafetyCase godoc
// @Summary 完成食安案件处置并恢复商户
// @Description 运营商提交调查结论和商户整改报告，原子恢复商户并关闭关联事件
// @Tags 运营商功能
// @Accept json
// @Produce json
// @Param id path int true "案件ID"
// @Param request body resolveOperatorFoodSafetyCaseRequest true "处置信息"
// @Success 200 {object} operatorFoodSafetyCaseResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/operator/food-safety/cases/{id}/resolve [post]
func (server *Server) resolveOperatorFoodSafetyCase(ctx *gin.Context) {
	caseID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req resolveOperatorFoodSafetyCaseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	caseRecord, err := server.store.GetFoodSafetyCase(ctx, caseID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if _, err := server.checkOperatorManagesRegion(ctx, caseRecord.RegionID); err != nil {
		server.respondOperatorRegionSelectionError(ctx, err)
		return
	}
	if caseRecord.Status == "resolved" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("food safety case is already resolved")))
		return
	}

	investigationReport := strings.TrimSpace(req.InvestigationReport)
	merchantRectificationReport := strings.TrimSpace(req.MerchantRectificationReport)
	resolution := strings.TrimSpace(req.Resolution)
	effectiveInvestigationReport := investigationReport
	if effectiveInvestigationReport == "" {
		effectiveInvestigationReport = strings.TrimSpace(caseRecord.InvestigationReport.String)
	}
	if effectiveInvestigationReport == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(db.ErrFoodSafetyCaseMissingInvestigationReport))
		return
	}
	if merchantRectificationReport == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("merchant_rectification_report cannot be blank")))
		return
	}
	if resolution == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("resolution cannot be blank")))
		return
	}

	resolved, err := server.store.ResolveFoodSafetyCaseTx(ctx, db.ResolveFoodSafetyCaseTxParams{
		CaseID:                      caseID,
		RegionID:                    caseRecord.RegionID,
		InvestigationReport:         effectiveInvestigationReport,
		MerchantRectificationReport: merchantRectificationReport,
		Resolution:                  resolution,
	})
	if err != nil {
		if errors.Is(err, db.ErrFoodSafetyCaseMissingInvestigationReport) || errors.Is(err, db.ErrFoodSafetyCaseAlreadyResolved) {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newOperatorFoodSafetyCaseResponse(resolved.Case))
}
