package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

// ==================== 骑手申请数据结构 ====================

func buildRiderApplicationMissingFieldsError(missingFields []string) error {
	return &APIError{
		Code:    40105,
		Message: fmt.Sprintf("请先补充以下资料后再提交：%s", joinStrings(missingFields, "、")),
	}
}

// riderApplicationResponse 骑手申请响应
type riderApplicationResponse struct {
	ID                 int64                             `json:"id"`
	UserID             int64                             `json:"user_id"`
	RealName           *string                           `json:"real_name,omitempty"`
	Phone              *string                           `json:"phone,omitempty"`
	IDCardFrontAssetID *int64                            `json:"id_card_front_asset_id,omitempty"`
	IDCardBackAssetID  *int64                            `json:"id_card_back_asset_id,omitempty"`
	IDCardOCR          *IDCardOCRData                    `json:"id_card_ocr,omitempty"`
	HealthCertAssetID  *int64                            `json:"health_cert_asset_id,omitempty"`
	HealthCertOCR      *HealthCertOCRData                `json:"health_cert_ocr,omitempty"`
	Status             string                            `json:"status"`
	RejectReason       *string                           `json:"reject_reason,omitempty"`
	ReviewSummary      *onboardingReviewSummaryResponse  `json:"review_summary,omitempty"`
	ActiveCredentials  []activeCredentialSummaryResponse `json:"active_credentials,omitempty"`
	CreatedAt          time.Time                         `json:"created_at"`
	UpdatedAt          *time.Time                        `json:"updated_at,omitempty"`
	SubmittedAt        *time.Time                        `json:"submitted_at,omitempty"`
}

type riderApplicationDocumentType string

const (
	riderApplicationDocumentIDCardFront riderApplicationDocumentType = "id_card_front"
	riderApplicationDocumentIDCardBack  riderApplicationDocumentType = "id_card_back"
	riderApplicationDocumentHealthCert  riderApplicationDocumentType = "health_cert"
)

func (server *Server) newRiderApplicationResponse(ctx context.Context, app db.RiderApplication) riderApplicationResponse {
	resp := riderApplicationResponse{
		ID:                app.ID,
		UserID:            app.UserID,
		Status:            app.Status,
		CreatedAt:         app.CreatedAt,
		ReviewSummary:     decodeOnboardingReviewSummary(app.ReviewSummary),
		ActiveCredentials: server.loadRiderActiveCredentialSummaries(ctx, app.UserID),
	}

	if app.RealName.Valid {
		resp.RealName = &app.RealName.String
	}
	if app.Phone.Valid {
		resp.Phone = &app.Phone.String
	}
	resp.IDCardFrontAssetID = int64PtrFromPgInt8(app.IDCardFrontMediaAssetID)
	resp.IDCardBackAssetID = int64PtrFromPgInt8(app.IDCardBackMediaAssetID)
	resp.HealthCertAssetID = int64PtrFromPgInt8(app.HealthCertMediaAssetID)
	if app.RejectReason.Valid {
		resp.RejectReason = &app.RejectReason.String
	}
	if app.UpdatedAt.Valid {
		resp.UpdatedAt = &app.UpdatedAt.Time
	}
	if app.SubmittedAt.Valid {
		resp.SubmittedAt = &app.SubmittedAt.Time
	}

	// 解析身份证OCR数据
	if len(app.IDCardOcr) > 0 {
		ocrData, err := decodeIDCardOCRData(app.IDCardOcr)
		if err == nil {
			resp.IDCardOCR = ocrData
		}
	}

	// 解析健康证OCR数据
	if len(app.HealthCertOcr) > 0 {
		ocrData, err := decodeHealthCertOCRData(app.HealthCertOcr)
		if err == nil {
			resp.HealthCertOCR = ocrData
		}
	}

	return resp
}

func (server *Server) ensureEditableRiderApplication(app db.RiderApplication) (db.RiderApplication, error) {
	if app.Status == "draft" {
		return app, nil
	}

	return app, ErrApplicationNotDraft
}

// ==================== 创建/获取草稿 ====================

// createOrGetRiderApplicationDraft godoc
// @Summary 创建或获取骑手申请草稿
// @Description 如果用户已有申请则返回现有申请，否则创建新的草稿
// @Tags 骑手申请
// @Accept json
// @Produce json
// @Success 200 {object} riderApplicationResponse "申请信息"
// @Success 201 {object} riderApplicationResponse "新建草稿"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application [get]
// @Security BearerAuth
func (server *Server) createOrGetRiderApplicationDraft(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 检查是否已有申请
	app, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err == nil {
		ctx.JSON(http.StatusOK, server.newRiderApplicationResponse(ctx, app))
		return
	}
	if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	// 创建新草稿
	app, err = server.store.CreateRiderApplication(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("create rider application draft: %w", err)))
		return
	}

	ctx.JSON(http.StatusCreated, server.newRiderApplicationResponse(ctx, app))
}

// ==================== 更新基础信息 ====================

type updateRiderApplicationBasicRequest struct {
	RealName *string `json:"real_name" binding:"omitempty,min=2,max=50"`
	Phone    *string `json:"phone" binding:"omitempty,validPhone"`
}

// updateRiderApplicationBasic godoc
// @Summary 更新骑手申请基础信息
// @Description 更新姓名、手机号等基础信息，仅草稿状态可修改
// @Tags 骑手申请
// @Accept json
// @Produce json
// @Param request body updateRiderApplicationBasicRequest true "基础信息"
// @Success 200 {object} riderApplicationResponse "更新后的申请信息"
// @Failure 400 {object} ErrorResponse "参数错误或状态不允许修改"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application/basic [put]
// @Security BearerAuth
func (server *Server) updateRiderApplicationBasic(ctx *gin.Context) {
	var req updateRiderApplicationBasicRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	app, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	app, err = server.ensureEditableRiderApplication(app)
	if err != nil {
		if errors.Is(err, ErrApplicationNotDraft) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("ensure rider application editable: %w", err)))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
		return
	}

	arg := db.UpdateRiderApplicationBasicInfoParams{
		ID: app.ID,
	}
	if req.RealName != nil {
		arg.RealName = pgtype.Text{String: *req.RealName, Valid: true}
	}
	if req.Phone != nil {
		arg.Phone = pgtype.Text{String: *req.Phone, Valid: true}
	}

	updated, err := server.store.UpdateRiderApplicationBasicInfo(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update rider application basic info: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, server.newRiderApplicationResponse(ctx, updated))
}

// ==================== 重置申请（处理中） ====================

// resetRiderApplication godoc
// @Summary 重置骑手申请
// @Description 将待处理申请重置为草稿状态并清空审核痕迹
// @Tags 骑手申请
// @Accept json
// @Produce json
// @Success 200 {object} riderApplicationResponse "重置后的申请"
// @Failure 400 {object} ErrorResponse "状态不允许重置"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application/reset [post]
// @Security BearerAuth
func (server *Server) resetRiderApplication(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	app, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	if app.Status != db.RiderApplicationStatusSubmitted {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationCannotReset))
		return
	}

	reset, err := server.store.ResetRiderApplicationToDraft(ctx, app.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("reset rider application: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, server.newRiderApplicationResponse(ctx, reset))
}

// ==================== 辅助函数 ====================

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
