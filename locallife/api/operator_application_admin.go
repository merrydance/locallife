package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

type listPendingOperatorApplicationsRequest struct {
	Page  int32 `form:"page" binding:"omitempty,min=1"`
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=100"`
}

type adminOperatorApplicationItem struct {
	ID                     int64      `json:"id"`
	UserID                 int64      `json:"user_id"`
	RegionID               int64      `json:"region_id"`
	RegionName             string     `json:"region_name"`
	RegionCode             string     `json:"region_code"`
	Name                   string     `json:"name"`
	ContactName            string     `json:"contact_name"`
	ContactPhone           string     `json:"contact_phone"`
	BusinessLicenseURL     string     `json:"business_license_url"`
	BusinessLicenseNumber  string     `json:"business_license_number"`
	LegalPersonName        string     `json:"legal_person_name"`
	LegalPersonIDNumber    string     `json:"legal_person_id_number"`
	RequestedContractYears int32      `json:"requested_contract_years"`
	Status                 string     `json:"status"`
	SubmittedAt            *time.Time `json:"submitted_at,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
}

type listPendingOperatorApplicationsResponse struct {
	Applications []adminOperatorApplicationItem `json:"applications"`
	Total        int64                          `json:"total"`
	Page         int32                          `json:"page"`
	Limit        int32                          `json:"limit"`
	HasMore      bool                           `json:"has_more"`
}

type operatorApplicationIDRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type rejectOperatorApplicationAdminRequest struct {
	RejectReason string `json:"reject_reason" binding:"required,min=2,max=200"`
}

func operatorNameFromApprovedApplication(app db.OperatorApplication) string {
	if app.BusinessLicenseUrl.Valid && strings.TrimSpace(app.BusinessLicenseUrl.String) != "" && len(app.BusinessLicenseOcr) > 0 {
		var ocr BusinessLicenseOCRData
		if err := json.Unmarshal(app.BusinessLicenseOcr, &ocr); err == nil {
			enterpriseName := strings.TrimSpace(ocr.EnterpriseName)
			if enterpriseName != "" {
				return enterpriseName
			}
		}
	}

	if name := strings.TrimSpace(app.Name.String); name != "" {
		return name
	}

	if legalName := strings.TrimSpace(app.LegalPersonName.String); legalName != "" {
		return legalName
	}

	if contactName := strings.TrimSpace(app.ContactName.String); contactName != "" {
		return contactName
	}

	return ""
}

func (server *Server) listPendingOperatorApplicationsAdmin(ctx *gin.Context) {
	var req listPendingOperatorApplicationsRequest
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

	rows, err := server.store.ListPendingOperatorApplications(ctx, db.ListPendingOperatorApplicationsParams{
		Limit:  req.Limit,
		Offset: pageOffset(req.Page, req.Limit),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountPendingOperatorApplications(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	applications := make([]adminOperatorApplicationItem, 0, len(rows))
	for _, row := range rows {
		item := adminOperatorApplicationItem{
			ID:                     row.ID,
			UserID:                 row.UserID,
			RegionID:               row.RegionID,
			RegionName:             row.RegionName,
			RegionCode:             row.RegionCode,
			Name:                   row.Name.String,
			ContactName:            row.ContactName.String,
			ContactPhone:           row.ContactPhone.String,
			BusinessLicenseURL:     row.BusinessLicenseUrl.String,
			BusinessLicenseNumber:  row.BusinessLicenseNumber.String,
			LegalPersonName:        row.LegalPersonName.String,
			LegalPersonIDNumber:    row.LegalPersonIDNumber.String,
			RequestedContractYears: row.RequestedContractYears,
			Status:                 row.Status,
			CreatedAt:              row.CreatedAt,
		}
		if row.SubmittedAt.Valid {
			t := row.SubmittedAt.Time
			item.SubmittedAt = &t
		}
		applications = append(applications, item)
	}

	hasMore := int64(req.Page*req.Limit) < total

	ctx.JSON(http.StatusOK, listPendingOperatorApplicationsResponse{
		Applications: applications,
		Total:        total,
		Page:         req.Page,
		Limit:        req.Limit,
		HasMore:      hasMore,
	})
}

func (server *Server) getOperatorApplicationDetailAdmin(ctx *gin.Context) {
	var uriReq operatorApplicationIDRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	app, err := server.store.GetOperatorApplicationByID(ctx, uriReq.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("申请不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionName := server.getRegionName(ctx, app.RegionID)
	ctx.JSON(http.StatusOK, newOperatorApplicationResponse(app, regionName))
}

func (server *Server) approveOperatorApplicationAdmin(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	var uriReq operatorApplicationIDRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	app, err := server.store.GetOperatorApplicationByID(ctx, uriReq.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("申请不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status != "submitted" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("仅可审核 submitted 状态申请")))
		return
	}

	if _, err := server.store.GetOperatorByUser(ctx, app.UserID); err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("该用户已是运营商")))
		return
	} else if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if _, err := server.store.GetOperatorByRegion(ctx, app.RegionID); err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("该区域已有运营商")))
		return
	} else if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	approved, err := server.store.ApproveOperatorApplication(ctx, db.ApproveOperatorApplicationParams{
		ID: uriReq.ID,
		ReviewedBy: pgtype.Int8{
			Int64: authPayload.UserID,
			Valid: true,
		},
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("申请状态已变化，无法审核通过")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	operatorName := operatorNameFromApprovedApplication(approved)
	if operatorName == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("运营商名称不能为空，请补全申请资料后再审核")))
		return
	}
	contactName := strings.TrimSpace(approved.ContactName.String)
	if contactName == "" {
		contactName = operatorName
	}
	contactPhone := strings.TrimSpace(approved.ContactPhone.String)

	years := approved.RequestedContractYears
	if years <= 0 {
		years = 1
	}
	now := time.Now()
	end := now.AddDate(int(years), 0, 0)

	var startDate pgtype.Date
	_ = startDate.Scan(now)
	var endDate pgtype.Date
	_ = endDate.Scan(end)

	operator, err := server.store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:       approved.UserID,
		RegionID:     approved.RegionID,
		Name:         operatorName,
		ContactName:  contactName,
		ContactPhone: contactPhone,
		WechatMchID: pgtype.Text{
			Valid: false,
		},
		CommissionRate:    numericFromFloat(0.10),
		Status:            "active",
		ContractStartDate: startDate,
		ContractEndDate:   endDate,
		ContractYears:     years,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 同步写入 operator_regions，使多区域查询（GetActiveOperatorByRegion / CheckOperatorManagesRegion）能找到初始区域
	if _, err := server.store.AddOperatorRegion(ctx, db.AddOperatorRegionParams{
		OperatorID: operator.ID,
		RegionID:   approved.RegionID,
	}); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if _, err := server.store.GetUserRoleByType(ctx, db.GetUserRoleByTypeParams{UserID: approved.UserID, Role: RoleOperator}); err != nil {
		if isNotFoundError(err) {
			_, createErr := server.store.CreateUserRole(ctx, db.CreateUserRoleParams{
				UserID: approved.UserID,
				Role:   RoleOperator,
				Status: "active",
				RelatedEntityID: pgtype.Int8{
					Int64: operator.ID,
					Valid: true,
				},
			})
			if createErr != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, createErr))
				return
			}
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	regionName := server.getRegionName(ctx, approved.RegionID)
	ctx.JSON(http.StatusOK, newOperatorApplicationResponse(approved, regionName))
}

func (server *Server) rejectOperatorApplicationAdmin(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	var uriReq operatorApplicationIDRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req rejectOperatorApplicationAdminRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rejected, err := server.store.RejectOperatorApplication(ctx, db.RejectOperatorApplicationParams{
		ID: uriReq.ID,
		RejectReason: pgtype.Text{
			String: req.RejectReason,
			Valid:  true,
		},
		ReviewedBy: pgtype.Int8{
			Int64: authPayload.UserID,
			Valid: true,
		},
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("申请状态已变化，无法驳回")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionName := server.getRegionName(ctx, rejected.RegionID)
	ctx.JSON(http.StatusOK, newOperatorApplicationResponse(rejected, regionName))
}
