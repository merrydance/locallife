package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/token"
)

type patchMerchantBusinessLicenseOCRFieldsRequest struct {
	EnterpriseName      *string `json:"enterprise_name" binding:"omitempty,max=100"`
	CreditCode          *string `json:"credit_code" binding:"omitempty,max=32"`
	RegNum              *string `json:"reg_num" binding:"omitempty,max=32"`
	LegalRepresentative *string `json:"legal_representative" binding:"omitempty,max=50"`
	Address             *string `json:"address" binding:"omitempty,max=200"`
	BusinessScope       *string `json:"business_scope" binding:"omitempty,max=500"`
	ValidPeriod         *string `json:"valid_period" binding:"omitempty,max=64"`
	Confirmed           bool    `json:"confirmed"`
}

type patchMerchantFoodPermitOCRFieldsRequest struct {
	PermitNo     *string `json:"permit_no" binding:"omitempty,max=64"`
	CompanyName  *string `json:"company_name" binding:"omitempty,max=100"`
	OperatorName *string `json:"operator_name" binding:"omitempty,max=50"`
	ValidFrom    *string `json:"valid_from" binding:"omitempty,max=32"`
	ValidTo      *string `json:"valid_to" binding:"omitempty,max=32"`
	Confirmed    bool    `json:"confirmed"`
}

type patchMerchantDocumentOCRFieldsRequest struct {
	EnterpriseName      *string `json:"enterprise_name,omitempty"`
	CreditCode          *string `json:"credit_code,omitempty"`
	RegNum              *string `json:"reg_num,omitempty"`
	LegalRepresentative *string `json:"legal_representative,omitempty"`
	Address             *string `json:"address,omitempty"`
	BusinessScope       *string `json:"business_scope,omitempty"`
	ValidPeriod         *string `json:"valid_period,omitempty"`
	PermitNo            *string `json:"permit_no,omitempty"`
	CompanyName         *string `json:"company_name,omitempty"`
	OperatorName        *string `json:"operator_name,omitempty"`
	ValidFrom           *string `json:"valid_from,omitempty"`
	ValidTo             *string `json:"valid_to,omitempty"`
	Confirmed           bool    `json:"confirmed,omitempty"`
}

// patchMerchantApplicationDocumentOCRFields godoc
// @Summary 更正或确认商户申请证照 OCR 识别字段
// @Description 允许商户在草稿状态下更正营业执照和食品经营许可证 OCR 字段。更正会写回 OCR JSON 并记录 correction 审计元数据；confirmed=true 时会记录商户确认快照，提交前必须完成当前 OCR 字段确认；法人身份证字段不支持此接口。
// @Tags 商户申请
// @Accept json
// @Produce json
// @Param document_type path string true "证照类型: business_license|food_permit"
// @Param request body patchMerchantDocumentOCRFieldsRequest true "证照 OCR 更正字段；business_license 使用营业执照字段，food_permit 使用食品经营许可证字段"
// @Success 200 {object} merchantApplicationDraftResponse "更新后的申请信息"
// @Failure 400 {object} ErrorResponse "参数错误或状态不允许修改"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/application/documents/{document_type}/ocr-fields [patch]
// @Security BearerAuth
func (server *Server) patchMerchantApplicationDocumentOCRFields(ctx *gin.Context) {
	documentType := merchantApplicationDocumentType(strings.TrimSpace(ctx.Param("document_type")))
	if documentType != merchantApplicationDocumentBusinessLicense && documentType != merchantApplicationDocumentFoodPermit {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only business_license and food_permit OCR fields can be corrected")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	app, err := server.store.GetMerchantApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant application draft for OCR correction: %w", err)))
		return
	}
	if app.Status != db.MerchantApplicationStatusDraft {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
		return
	}

	switch documentType {
	case merchantApplicationDocumentBusinessLicense:
		server.patchMerchantBusinessLicenseOCRFields(ctx, app, authPayload.UserID)
	case merchantApplicationDocumentFoodPermit:
		server.patchMerchantFoodPermitOCRFields(ctx, app, authPayload.UserID)
	}
}

func (server *Server) patchMerchantBusinessLicenseOCRFields(ctx *gin.Context, app db.MerchantApplication, userID int64) {
	var req patchMerchantBusinessLicenseOCRFieldsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if !app.BusinessLicenseMediaAssetID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("营业执照照片未上传")))
		return
	}
	if len(app.BusinessLicenseOcr) == 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("营业执照信息未识别，请重新上传清晰的营业执照照片")))
		return
	}

	ocrData, err := decodeMerchantBusinessLicenseOCRData(app.BusinessLicenseOcr)
	if err != nil || ocrData == nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("营业执照信息解析失败，请重新上传")))
		return
	}
	if !merchantOCRCorrectionStatusEditable(ocrData.Status) {
		ctx.JSON(http.StatusBadRequest, errorResponse(merchantOCRCorrectionStatusError("营业执照", ocrData.Status)))
		return
	}

	previous := merchantBusinessLicenseOCRPrevious(ocrData)
	next := *ocrData
	applyStringPointer(req.EnterpriseName, &next.EnterpriseName)
	applyStringPointer(req.CreditCode, &next.CreditCode)
	applyStringPointer(req.RegNum, &next.RegNum)
	applyStringPointer(req.LegalRepresentative, &next.LegalRepresentative)
	applyStringPointer(req.Address, &next.Address)
	applyStringPointer(req.BusinessScope, &next.BusinessScope)
	applyStringPointer(req.ValidPeriod, &next.ValidPeriod)

	if req.Confirmed && strings.TrimSpace(next.EnterpriseName) == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("营业执照企业名称未识别，请重新填写")))
		return
	}
	if strings.TrimSpace(next.CreditCode) == "" && strings.TrimSpace(next.RegNum) == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("营业执照统一信用代码未识别，请重新填写")))
		return
	}
	if strings.TrimSpace(next.ValidPeriod) == "" || !merchantOCRCorrectionValidPeriodValid(next.ValidPeriod, time.Now()) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("营业执照有效期无法识别，请重新填写")))
		return
	}

	fields := changedMerchantOCRFields(previous, merchantBusinessLicenseOCRPrevious(&next))
	if len(fields) == 0 && !req.Confirmed {
		server.writeMerchantApplicationDraftResponse(ctx, http.StatusOK, app)
		return
	}

	now := time.Now()
	if len(fields) > 0 {
		next.Error = ""
		next.ErrorCode = ""
		next.AlertEmittedAt = ""
		next.Readiness = buildMerchantBusinessLicenseOCRReadinessForAPI(&next)
		next.Correction = &OCRCorrection{
			CorrectedBy: userID,
			CorrectedAt: now.Format(time.RFC3339),
			Source:      "merchant",
			Fields:      fields,
			Previous:    previous,
		}
	}
	if req.Confirmed {
		next.Confirmation = buildMerchantOCRConfirmation(userID, now, merchantBusinessLicenseOCRPrevious(&next))
	}

	encoded, err := json.Marshal(next)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("marshal merchant business license OCR correction: %w", err)))
		return
	}

	licenseNumber := normalizeMerchantBusinessLicenseNumber(next.CreditCode)
	if licenseNumber == "" {
		licenseNumber = normalizeMerchantBusinessLicenseNumber(next.RegNum)
	}
	merchantNameBackfill := strings.TrimSpace(next.EnterpriseName)
	if strings.TrimSpace(app.MerchantName) != "" {
		merchantNameBackfill = ""
	}
	updated, err := server.store.UpdateMerchantApplicationBusinessLicense(ctx, db.UpdateMerchantApplicationBusinessLicenseParams{
		ID:                    app.ID,
		BusinessLicenseNumber: pgtype.Text{String: licenseNumber, Valid: licenseNumber != ""},
		BusinessScope:         pgtype.Text{String: strings.TrimSpace(next.BusinessScope), Valid: strings.TrimSpace(next.BusinessScope) != ""},
		LegalPersonName:       pgtype.Text{String: strings.TrimSpace(next.LegalRepresentative), Valid: strings.TrimSpace(next.LegalRepresentative) != ""},
		MerchantName:          pgtype.Text{String: merchantNameBackfill, Valid: merchantNameBackfill != ""},
		BusinessLicenseOcr:    encoded,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update merchant business license OCR correction: %w", err)))
		return
	}
	server.tryProjectMerchantSubjectProfile(ctx, updated, "business_license_ocr_correction")
	server.writeMerchantApplicationDraftResponse(ctx, http.StatusOK, updated)
}

func (server *Server) patchMerchantFoodPermitOCRFields(ctx *gin.Context, app db.MerchantApplication, userID int64) {
	var req patchMerchantFoodPermitOCRFieldsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if !app.FoodPermitMediaAssetID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("食品经营许可证照片未上传")))
		return
	}
	if len(app.FoodPermitOcr) == 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("食品经营许可证信息未识别，请重新上传清晰的食品经营许可证照片")))
		return
	}

	ocrData, err := decodeMerchantFoodPermitOCRData(app.FoodPermitOcr)
	if err != nil || ocrData == nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("食品经营许可证信息解析失败，请重新上传")))
		return
	}
	if !merchantOCRCorrectionStatusEditable(ocrData.Status) {
		ctx.JSON(http.StatusBadRequest, errorResponse(merchantOCRCorrectionStatusError("食品经营许可证", ocrData.Status)))
		return
	}

	previous := merchantFoodPermitOCRPrevious(ocrData)
	next := *ocrData
	applyStringPointer(req.PermitNo, &next.PermitNo)
	applyStringPointer(req.CompanyName, &next.CompanyName)
	applyStringPointer(req.OperatorName, &next.OperatorName)
	applyStringPointer(req.ValidFrom, &next.ValidFrom)
	applyStringPointer(req.ValidTo, &next.ValidTo)

	if strings.TrimSpace(next.CompanyName) == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("食品经营许可证主体名称未识别，请重新填写")))
		return
	}
	if req.Confirmed && strings.TrimSpace(next.PermitNo) == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("食品经营许可证编号未识别，请重新填写")))
		return
	}
	if strings.TrimSpace(next.ValidTo) == "" || !merchantOCRCorrectionDateAtLeastDaysAfterNow(next.ValidTo, 30, time.Now()) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("食品经营许可证有效期无法识别或不足30天，请重新填写")))
		return
	}

	fields := changedMerchantOCRFields(previous, merchantFoodPermitOCRPrevious(&next))
	if len(fields) == 0 && !req.Confirmed {
		server.writeMerchantApplicationDraftResponse(ctx, http.StatusOK, app)
		return
	}

	now := time.Now()
	if len(fields) > 0 {
		next.Error = ""
		next.ErrorCode = ""
		next.AlertEmittedAt = ""
		next.Readiness = buildMerchantFoodPermitOCRReadinessForAPI(&next)
		next.Correction = &OCRCorrection{
			CorrectedBy: userID,
			CorrectedAt: now.Format(time.RFC3339),
			Source:      "merchant",
			Fields:      fields,
			Previous:    previous,
		}
	}
	if req.Confirmed {
		next.Confirmation = buildMerchantOCRConfirmation(userID, now, merchantFoodPermitOCRPrevious(&next))
	}

	encoded, err := json.Marshal(next)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("marshal merchant food permit OCR correction: %w", err)))
		return
	}

	updated, err := server.store.UpdateMerchantApplicationFoodPermit(ctx, db.UpdateMerchantApplicationFoodPermitParams{
		ID:            app.ID,
		FoodPermitOcr: encoded,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("update merchant food permit OCR correction: %w", err)))
		return
	}
	server.tryProjectMerchantSubjectProfile(ctx, updated, "food_permit_ocr_correction")
	server.writeMerchantApplicationDraftResponse(ctx, http.StatusOK, updated)
}

func decodeMerchantBusinessLicenseOCRData(data []byte) (*BusinessLicenseOCRData, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var payload BusinessLicenseOCRData
	if err := decodeOCRPayload(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func decodeMerchantFoodPermitOCRData(data []byte) (*FoodPermitOCRData, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var payload FoodPermitOCRData
	if err := decodeOCRPayload(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func applyStringPointer(input *string, target *string) {
	if input == nil {
		return
	}
	*target = strings.TrimSpace(*input)
}

func merchantOCRCorrectionStatusEditable(status string) bool {
	switch strings.TrimSpace(status) {
	case "", "done", string(ocr.JobStatusSucceeded):
		return true
	default:
		return false
	}
}

func merchantOCRCorrectionStatusError(documentName string, status string) error {
	switch strings.TrimSpace(status) {
	case string(ocr.JobStatusPending), string(ocr.JobStatusProcessing):
		return fmt.Errorf("%sOCR处理中，请稍后再提交", documentName)
	case string(ocr.JobStatusFailed):
		return fmt.Errorf("%sOCR处理失败，请重新上传清晰的证照照片", documentName)
	default:
		return fmt.Errorf("%sOCR状态不允许更正，请重新上传清晰的证照照片", documentName)
	}
}

func merchantBusinessLicenseOCRPrevious(data *BusinessLicenseOCRData) map[string]string {
	return map[string]string{
		"enterprise_name":      strings.TrimSpace(data.EnterpriseName),
		"credit_code":          strings.TrimSpace(data.CreditCode),
		"reg_num":              strings.TrimSpace(data.RegNum),
		"legal_representative": strings.TrimSpace(data.LegalRepresentative),
		"address":              strings.TrimSpace(data.Address),
		"business_scope":       strings.TrimSpace(data.BusinessScope),
		"valid_period":         strings.TrimSpace(data.ValidPeriod),
	}
}

func merchantFoodPermitOCRPrevious(data *FoodPermitOCRData) map[string]string {
	return map[string]string{
		"permit_no":     strings.TrimSpace(data.PermitNo),
		"company_name":  strings.TrimSpace(data.CompanyName),
		"operator_name": strings.TrimSpace(data.OperatorName),
		"valid_from":    strings.TrimSpace(data.ValidFrom),
		"valid_to":      strings.TrimSpace(data.ValidTo),
	}
}

func changedMerchantOCRFields(previous map[string]string, next map[string]string) []string {
	fieldOrder := []string{
		"enterprise_name",
		"credit_code",
		"reg_num",
		"legal_representative",
		"address",
		"business_scope",
		"valid_period",
		"permit_no",
		"company_name",
		"operator_name",
		"valid_from",
		"valid_to",
	}
	fields := make([]string, 0, len(fieldOrder))
	for _, field := range fieldOrder {
		prevValue, ok := previous[field]
		if !ok {
			continue
		}
		if next[field] != prevValue {
			fields = append(fields, field)
		}
	}
	return fields
}

func buildMerchantBusinessLicenseOCRReadinessForAPI(data *BusinessLicenseOCRData) *OCRReadiness {
	required := []string{"enterprise_name", "legal_representative", "address", "business_scope", "valid_period", "credit_code|reg_num"}
	missing := make([]string, 0, len(required)+1)
	if strings.TrimSpace(data.EnterpriseName) == "" {
		missing = append(missing, "enterprise_name")
	}
	if strings.TrimSpace(data.LegalRepresentative) == "" {
		missing = append(missing, "legal_representative")
	}
	if strings.TrimSpace(data.Address) == "" {
		missing = append(missing, "address")
	}
	if strings.TrimSpace(data.BusinessScope) == "" {
		missing = append(missing, "business_scope")
	}
	if strings.TrimSpace(data.ValidPeriod) == "" {
		missing = append(missing, "valid_period")
	}
	if strings.TrimSpace(data.CreditCode) == "" && strings.TrimSpace(data.RegNum) == "" {
		missing = append(missing, "credit_code|reg_num")
	}
	if len(missing) == 0 {
		return &OCRReadiness{State: ocrReadinessStateReady, ReasonCode: "ok", RequiredFields: required}
	}
	return &OCRReadiness{
		State:          ocrReadinessStatePartial,
		ReasonCode:     ocrReadinessReasonRequiredFieldMissing,
		RequiredFields: required,
		MissingFields:  missing,
	}
}

func buildMerchantFoodPermitOCRReadinessForAPI(data *FoodPermitOCRData) *OCRReadiness {
	required := []string{"company_name", "valid_to"}
	missing := make([]string, 0, len(required))
	if strings.TrimSpace(data.CompanyName) == "" {
		missing = append(missing, "company_name")
	}
	if strings.TrimSpace(data.ValidTo) == "" {
		missing = append(missing, "valid_to")
	}
	if len(missing) == 0 {
		return &OCRReadiness{State: ocrReadinessStateReady, ReasonCode: "ok", RequiredFields: required}
	}
	return &OCRReadiness{
		State:          ocrReadinessStatePartial,
		ReasonCode:     ocrReadinessReasonRequiredFieldMissing,
		RequiredFields: required,
		MissingFields:  missing,
	}
}

func merchantOCRCorrectionValidPeriodValid(validPeriod string, now time.Time) bool {
	if strings.TrimSpace(validPeriod) == "" {
		return false
	}
	if strings.Contains(validPeriod, "长期") || strings.Contains(validPeriod, "永久") {
		return true
	}
	expiresAt, err := logic.ParseRiderFlexibleDocumentEndDate(validPeriod)
	if err != nil {
		return false
	}
	return expiresAt.After(now)
}

func merchantOCRCorrectionDateAtLeastDaysAfterNow(dateStr string, days int, now time.Time) bool {
	if strings.TrimSpace(dateStr) == "" {
		return false
	}
	if strings.Contains(dateStr, "长期") || strings.Contains(dateStr, "永久") {
		return true
	}
	parsed, err := logic.ParseRiderFlexibleDocumentEndDate(dateStr)
	if err != nil {
		return false
	}
	return parsed.After(now.AddDate(0, 0, days))
}
