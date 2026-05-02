package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/rs/zerolog/log"
)

type listPendingOperatorApplicationsRequest struct {
	Page  int32 `form:"page" binding:"omitempty,min=1"`
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=100"`
}

type adminOperatorApplicationItem struct {
	ID                          int64      `json:"id"`
	UserID                      int64      `json:"user_id"`
	ApplicantName               string     `json:"applicant_name,omitempty"`
	ApplicantPhone              string     `json:"applicant_phone,omitempty"`
	RegionID                    int64      `json:"region_id"`
	RegionName                  string     `json:"region_name"`
	RegionCode                  string     `json:"region_code"`
	Name                        string     `json:"name"`
	ContactName                 string     `json:"contact_name"`
	ContactPhone                string     `json:"contact_phone"`
	BusinessLicenseMediaAssetID *int64     `json:"business_license_media_asset_id,omitempty"`
	BusinessLicenseNumber       string     `json:"business_license_number"`
	LegalPersonName             string     `json:"legal_person_name"`
	LegalPersonIDNumber         string     `json:"legal_person_id_number"`
	RequestedContractYears      int32      `json:"requested_contract_years"`
	Status                      string     `json:"status"`
	SubmittedAt                 *time.Time `json:"submitted_at,omitempty"`
	CreatedAt                   time.Time  `json:"created_at"`
}

type listPendingOperatorApplicationsResponse struct {
	Applications []adminOperatorApplicationItem `json:"applications"`
	Total        int64                          `json:"total"`
	Page         int32                          `json:"page"`
	Limit        int32                          `json:"limit"`
	HasMore      bool                           `json:"has_more"`
}

type operatorApplicationRegionsResponse struct {
	Regions interface{} `json:"regions"`
	Total   int         `json:"total"`
}

type operatorApplicationIDRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type rejectOperatorApplicationAdminRequest struct {
	RejectReason string `json:"reject_reason" binding:"required,min=2,max=200"`
}

type updateOperatorStatusAdminRequest struct {
	Status string `json:"status" binding:"required,oneof=active suspended"`
}

type batchUpdateOperatorStatusAdminRequest struct {
	OperatorIDs []int64 `json:"operator_ids" binding:"required,min=1,max=100,dive,min=1"`
	Status      string  `json:"status" binding:"required,oneof=active suspended"`
}

type adminOperatorStatusResponse struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	RegionID int64  `json:"region_id"`
	Status   string `json:"status"`
}

type batchAdminOperatorStatusFailure struct {
	OperatorID int64  `json:"operator_id"`
	Code       int    `json:"code,omitempty"`
	Error      string `json:"error"`
}

type batchAdminOperatorStatusResponse struct {
	Updated []adminOperatorStatusResponse     `json:"updated"`
	Failed  []batchAdminOperatorStatusFailure `json:"failed"`
	Message string                            `json:"message"`
}

type operatorApprovalTxStore interface {
	ApproveOperatorApplicationTx(ctx context.Context, arg db.ApproveOperatorApplicationTxParams) (db.ApproveOperatorApplicationTxResult, error)
}

func operatorNameFromApprovedApplication(app db.OperatorApplication) string {
	if app.BusinessLicenseMediaAssetID.Valid && len(app.BusinessLicenseOcr) > 0 {
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

func (server *Server) loadOperatorApplicationApplicant(ctx *gin.Context, userID int64) (string, string) {
	user, err := server.store.GetUser(ctx, userID)
	if err != nil {
		return "", ""
	}

	applicantName := strings.TrimSpace(user.FullName)
	applicantPhone := ""
	if user.Phone.Valid {
		applicantPhone = strings.TrimSpace(user.Phone.String)
	}

	return applicantName, applicantPhone
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
		applicantName := strings.TrimSpace(row.ApplicantName.String)
		applicantPhone := ""
		if row.ApplicantPhone.Valid {
			applicantPhone = strings.TrimSpace(row.ApplicantPhone.String)
		}
		item := adminOperatorApplicationItem{
			ID:                          row.ID,
			UserID:                      row.UserID,
			ApplicantName:               applicantName,
			ApplicantPhone:              applicantPhone,
			RegionID:                    row.RegionID,
			RegionName:                  row.RegionName,
			RegionCode:                  row.RegionCode,
			Name:                        row.Name.String,
			ContactName:                 row.ContactName.String,
			ContactPhone:                row.ContactPhone.String,
			BusinessLicenseMediaAssetID: int64PtrFromPgInt8(row.BusinessLicenseMediaAssetID),
			BusinessLicenseNumber:       row.BusinessLicenseNumber.String,
			LegalPersonName:             row.LegalPersonName.String,
			LegalPersonIDNumber:         row.LegalPersonIDNumber.String,
			RequestedContractYears:      row.RequestedContractYears,
			Status:                      row.Status,
			CreatedAt:                   row.CreatedAt,
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
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionName := server.getRegionName(ctx, app.RegionID)
	resp, err := newOperatorApplicationResponse(app, regionName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	applicantName, applicantPhone := server.loadOperatorApplicationApplicant(ctx, app.UserID)

	type adminOperatorApplicationDetailResponse struct {
		operatorApplicationResponse
		ApplicantName    string `json:"applicant_name,omitempty"`
		ApplicantPhone   string `json:"applicant_phone,omitempty"`
		OperatorEntityID int64  `json:"operator_id,omitempty"`
	}

	detailResp := adminOperatorApplicationDetailResponse{
		operatorApplicationResponse: resp,
		ApplicantName:               applicantName,
		ApplicantPhone:              applicantPhone,
	}

	// 若申请已通过，附带运营商实体 ID 以便前端查询多区域
	if app.Status == "approved" {
		if op, opErr := server.store.GetOperatorByUser(ctx, app.UserID); opErr == nil {
			detailResp.OperatorEntityID = op.ID
			ctx.JSON(http.StatusOK, detailResp)
			return
		}
	}

	ctx.JSON(http.StatusOK, detailResp)
}

// getOperatorRegionsAdmin godoc
// @Summary [管理] 获取运营商管理的区域列表
// @Tags 管理-运营商
// @Produce json
// @Param operator_id path int true "运营商实体 ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/admin/operators/{operator_id}/regions [get]
func (server *Server) getOperatorRegionsAdmin(ctx *gin.Context) {
	type uriReq struct {
		OperatorID int64 `uri:"operator_id" binding:"required,min=1"`
	}
	var uri uriReq
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if _, err := server.store.GetOperator(ctx, uri.OperatorID); err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrOperatorNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rows, err := server.store.ListOperatorRegions(ctx, uri.OperatorID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	type regionItem struct {
		ID         int64  `json:"id"`
		RegionID   int64  `json:"region_id"`
		RegionName string `json:"region_name"`
		RegionCode string `json:"region_code"`
		Status     string `json:"status"`
	}
	resp := make([]regionItem, 0, len(rows))
	for _, r := range rows {
		resp = append(resp, regionItem{
			ID:         r.ID,
			RegionID:   r.RegionID,
			RegionName: r.RegionName,
			RegionCode: r.RegionCode,
			Status:     r.Status,
		})
	}
	ctx.JSON(http.StatusOK, operatorApplicationRegionsResponse{Regions: resp, Total: len(resp)})
}

func buildAdminOperatorStatusResponse(operator db.Operator) adminOperatorStatusResponse {
	return adminOperatorStatusResponse{
		ID:       operator.ID,
		UserID:   operator.UserID,
		RegionID: operator.RegionID,
		Status:   operator.Status,
	}
}

func (server *Server) mapOperatorStatusUpdateError(ctx *gin.Context, operator db.Operator, targetStatus string, err error) (int, ErrorResponse) {
	var conflictErr *logic.OperatorActiveRegionConflictError

	switch {
	case errors.Is(err, logic.ErrUnsupportedOperatorStatus):
		return http.StatusBadRequest, errorResponse(err)
	case errors.As(err, &conflictErr):
		return http.StatusConflict, errorResponse(ErrRegionHasOperator)
	case errors.Is(err, logic.ErrProfitSharingReceiverOpenIDRequired):
		log.Error().Err(err).
			Int64("operator_id", operator.ID).
			Int64("user_id", operator.UserID).
			Str("target_status", targetStatus).
			Msg("operator status update rejected because receiver openid is missing")
		return http.StatusBadRequest, errorResponse(err)
	default:
		log.Error().Err(err).
			Int64("operator_id", operator.ID).
			Int64("user_id", operator.UserID).
			Str("target_status", targetStatus).
			Msg("update operator status failed")
		return http.StatusInternalServerError, internalError(ctx, err)
	}
}

func (server *Server) updateOperatorStatusWithService(
	ctx *gin.Context,
	statusService *logic.OperatorStatusService,
	operator db.Operator,
	targetStatus string,
) (db.Operator, int, ErrorResponse, bool) {
	updatedOperator, err := statusService.UpdateStatus(ctx, operator, targetStatus)
	if err != nil {
		statusCode, resp := server.mapOperatorStatusUpdateError(ctx, operator, targetStatus, err)
		return db.Operator{}, statusCode, resp, false
	}

	return updatedOperator, http.StatusOK, ErrorResponse{}, true
}

func (server *Server) updateOperatorStatusAdmin(ctx *gin.Context) {
	type uriReq struct {
		OperatorID int64 `uri:"operator_id" binding:"required,min=1"`
	}

	var uri uriReq
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateOperatorStatusAdminRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	operator, err := server.store.GetOperator(ctx, uri.OperatorID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrOperatorNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	updatedOperator, statusCode, errResp, ok := server.updateOperatorStatusWithService(
		ctx,
		server.buildOperatorStatusService(),
		operator,
		req.Status,
	)
	if !ok {
		ctx.JSON(statusCode, errResp)
		return
	}

	ctx.JSON(http.StatusOK, buildAdminOperatorStatusResponse(updatedOperator))
}

// batchUpdateOperatorStatusAdmin godoc
// @Summary [管理] 批量更新运营商状态
// @Tags 管理-运营商
// @Accept json
// @Produce json
// @Param request body batchUpdateOperatorStatusAdminRequest true "批量状态更新"
// @Success 200 {object} batchAdminOperatorStatusResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Router /v1/admin/operators/batch/status [post]
// @Security BearerAuth
func (server *Server) batchUpdateOperatorStatusAdmin(ctx *gin.Context) {
	var req batchUpdateOperatorStatusAdminRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	statusService := server.buildOperatorStatusService()
	updated := make([]adminOperatorStatusResponse, 0, len(req.OperatorIDs))
	failed := make([]batchAdminOperatorStatusFailure, 0)
	seen := make(map[int64]struct{}, len(req.OperatorIDs))

	for _, operatorID := range req.OperatorIDs {
		if _, ok := seen[operatorID]; ok {
			continue
		}
		seen[operatorID] = struct{}{}

		operator, err := server.store.GetOperator(ctx, operatorID)
		if err != nil {
			if isNotFoundError(err) {
				resp := errorResponse(ErrOperatorNotFound)
				failed = append(failed, batchAdminOperatorStatusFailure{
					OperatorID: operatorID,
					Code:       resp.Code,
					Error:      resp.Error,
				})
				continue
			}

			log.Error().Err(err).
				Int64("operator_id", operatorID).
				Str("target_status", req.Status).
				Msg("load operator for batch status update failed")
			resp := internalError(ctx, err)
			failed = append(failed, batchAdminOperatorStatusFailure{
				OperatorID: operatorID,
				Code:       resp.Code,
				Error:      resp.Error,
			})
			continue
		}

		updatedOperator, _, errResp, ok := server.updateOperatorStatusWithService(ctx, statusService, operator, req.Status)
		if !ok {
			failed = append(failed, batchAdminOperatorStatusFailure{
				OperatorID: operatorID,
				Code:       errResp.Code,
				Error:      errResp.Error,
			})
			continue
		}

		updated = append(updated, buildAdminOperatorStatusResponse(updatedOperator))
	}

	ctx.JSON(http.StatusOK, batchAdminOperatorStatusResponse{
		Updated: updated,
		Failed:  failed,
		Message: "批量更新运营商状态完成",
	})
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
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status != "submitted" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotSubmitted))
		return
	}

	if _, err := server.store.GetOperatorByUser(ctx, app.UserID); err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(ErrAlreadyOperator))
		return
	} else if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if _, err := server.getRegionActiveOperator(ctx, app.RegionID); err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(ErrRegionHasOperator))
		return
	} else if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	user, err := server.store.GetUser(ctx, app.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if strings.TrimSpace(user.WechatOpenid) == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(logic.ErrProfitSharingReceiverOpenIDRequired))
		return
	}

	operatorName := operatorNameFromApprovedApplication(app)
	if operatorName == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrOperatorNameRequired))
		return
	}
	contactName := strings.TrimSpace(app.ContactName.String)
	if contactName == "" {
		contactName = operatorName
	}
	contactPhone := strings.TrimSpace(app.ContactPhone.String)

	years := app.RequestedContractYears
	if years <= 0 {
		years = 1
	}
	now := time.Now()
	end := now.AddDate(int(years), 0, 0)

	var startDate pgtype.Date
	_ = startDate.Scan(now)
	var endDate pgtype.Date
	_ = endDate.Scan(end)

	if txStore, ok := server.store.(operatorApprovalTxStore); ok {
		result, txErr := txStore.ApproveOperatorApplicationTx(ctx, db.ApproveOperatorApplicationTxParams{
			ApplicationID:     app.ID,
			ReviewedBy:        pgtype.Int8{Int64: authPayload.UserID, Valid: true},
			OperatorName:      operatorName,
			ContactName:       contactName,
			ContactPhone:      contactPhone,
			ContractStartDate: startDate,
			ContractEndDate:   endDate,
			ContractYears:     years,
		})
		if txErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, txErr))
			return
		}

		if err := server.recordApprovedOperatorReceiverIntent(ctx, result.Operator); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		regionName := server.getRegionName(ctx, result.Application.RegionID)
		server.writeOperatorApplicationResponse(ctx, http.StatusOK, result.Application, regionName)
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
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationStateChanged))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	operator, err := server.store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:       approved.UserID,
		RegionID:     approved.RegionID,
		Name:         operatorName,
		ContactName:  contactName,
		ContactPhone: contactPhone,
		WechatMchID: pgtype.Text{
			Valid: false,
		},
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

	if err := server.recordApprovedOperatorReceiverIntent(ctx, operator); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionName := server.getRegionName(ctx, approved.RegionID)
	server.writeOperatorApplicationResponse(ctx, http.StatusOK, approved, regionName)
}

func (server *Server) recordApprovedOperatorReceiverIntent(_ *gin.Context, operator db.Operator) error {
	log.Info().
		Int64("operator_id", operator.ID).
		Int64("user_id", operator.UserID).
		Msg("skip global profit sharing receiver target intent after operator approval; ordinary service provider receiver sync is payment-order scoped")
	return nil
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
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationStateChanged))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionName := server.getRegionName(ctx, rejected.RegionID)
	server.writeOperatorApplicationResponse(ctx, http.StatusOK, rejected, regionName)
}
