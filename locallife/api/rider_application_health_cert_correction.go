package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/token"
)

type patchRiderHealthCertOCRFieldsRequest struct {
	CertNumber *string `json:"cert_number" binding:"omitempty,max=64"`
	ValidStart *string `json:"valid_start" binding:"omitempty,max=32"`
	ValidEnd   *string `json:"valid_end" binding:"required,max=32"`
}

// patchRiderApplicationDocumentOCRFields godoc
// @Summary 更正骑手申请健康证识别字段
// @Description 允许骑手在草稿状态下更正健康证号和有效期字段。更正会保留原 OCR 结果并写入 correction 审计元数据；身份证字段不支持此接口。
// @Tags 骑手申请
// @Accept json
// @Produce json
// @Param document_type path string true "证照类型: health_cert"
// @Param request body patchRiderHealthCertOCRFieldsRequest true "健康证识别字段"
// @Success 200 {object} riderApplicationResponse "更新后的申请信息"
// @Failure 400 {object} ErrorResponse "参数错误或状态不允许修改"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application/documents/{document_type}/ocr-fields [patch]
// @Security BearerAuth
func (server *Server) patchRiderApplicationDocumentOCRFields(ctx *gin.Context) {
	documentType := riderApplicationDocumentType(ctx.Param("document_type"))
	if documentType != riderApplicationDocumentHealthCert {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only health_cert OCR fields can be corrected")))
		return
	}

	var req patchRiderHealthCertOCRFieldsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.ValidEnd == nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("健康证有效期未识别，请重新上传清晰的健康证照片")))
		return
	}

	validEnd := strings.TrimSpace(*req.ValidEnd)
	if validEnd == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("健康证有效期未识别，请重新上传清晰的健康证照片")))
		return
	}
	validEndDate, err := logic.ParseRiderFlexibleDocumentEndDate(validEnd)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("健康证有效期格式无法识别，请重新填写")))
		return
	}
	if !validEndDate.After(time.Now().AddDate(0, 0, 7)) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("健康证有效期需超过当日7天")))
		return
	}

	validStart := ""
	if req.ValidStart != nil {
		validStart = strings.TrimSpace(*req.ValidStart)
	}
	if validStart != "" {
		if _, err := logic.ParseRiderFlexibleDocumentEndDate(validStart); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("健康证起始日期格式无法识别，请重新填写")))
			return
		}
	}

	certNumber := ""
	if req.CertNumber != nil {
		certNumber = strings.TrimSpace(*req.CertNumber)
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
	if !app.HealthCertMediaAssetID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("健康证照片未上传")))
		return
	}
	if len(app.HealthCertOcr) == 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("健康证信息未识别，请重新上传清晰的健康证照片")))
		return
	}

	ocrData, err := decodeHealthCertOCRData(app.HealthCertOcr)
	if err != nil || ocrData == nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("健康证信息解析失败，请重新上传")))
		return
	}
	switch strings.TrimSpace(ocrData.Status) {
	case string(ocr.JobStatusPending), string(ocr.JobStatusProcessing):
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("健康证OCR处理中，请稍后再提交")))
		return
	case string(ocr.JobStatusFailed):
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("健康证OCR处理失败，请重新上传清晰的健康证照片")))
		return
	}

	previous := map[string]string{
		"cert_number": strings.TrimSpace(ocrData.CertNumber),
		"valid_start": strings.TrimSpace(ocrData.ValidStart),
		"valid_end":   strings.TrimSpace(ocrData.ValidEnd),
	}
	nextCertNumber := previous["cert_number"]
	if req.CertNumber != nil {
		nextCertNumber = certNumber
	}
	nextValidStart := previous["valid_start"]
	if req.ValidStart != nil {
		nextValidStart = validStart
	}

	fields := make([]string, 0, 3)
	if nextCertNumber != previous["cert_number"] {
		fields = append(fields, "cert_number")
	}
	if nextValidStart != previous["valid_start"] {
		fields = append(fields, "valid_start")
	}
	if validEnd != previous["valid_end"] {
		fields = append(fields, "valid_end")
	}
	if len(fields) == 0 {
		ctx.JSON(http.StatusOK, server.newRiderApplicationResponse(ctx, app))
		return
	}

	ocrData.CertNumber = nextCertNumber
	ocrData.ValidStart = nextValidStart
	ocrData.ValidEnd = validEnd
	ocrData.Error = ""
	ocrData.ErrorCode = ""
	ocrData.AlertEmittedAt = ""
	ocrData.Readiness = buildHealthCertOCRReadinessForAPI(ocrData.Name, ocrData.ValidEnd)
	ocrData.Correction = &OCRCorrection{
		CorrectedBy: authPayload.UserID,
		CorrectedAt: time.Now().Format(time.RFC3339),
		Source:      "rider",
		Fields:      fields,
		Previous:    previous,
	}

	encoded, err := json.Marshal(ocrData)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("marshal rider health cert OCR correction: %w", err)))
		return
	}

	updated, err := server.store.UpdateRiderApplicationHealthCert(ctx, db.UpdateRiderApplicationHealthCertParams{
		ID:            app.ID,
		HealthCertOcr: encoded,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update rider health cert OCR correction: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, server.newRiderApplicationResponse(ctx, updated))
}
