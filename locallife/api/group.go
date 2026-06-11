package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/token"
	"github.com/rs/zerolog/log"
)

// ==================== Group Application ====================

type groupApplicationResponse struct {
	ID                          int64                   `json:"id"`
	ApplicantUserID             int64                   `json:"applicant_user_id"`
	GroupName                   string                  `json:"group_name"`
	ContactPhone                string                  `json:"contact_phone"`
	LicenseNumber               *string                 `json:"license_number,omitempty"`
	LicenseImageAssetID         *int64                  `json:"license_image_asset_id,omitempty"`
	BusinessLicenseOCR          *BusinessLicenseOCRData `json:"business_license_ocr,omitempty"`
	LegalPersonName             string                  `json:"legal_person_name,omitempty"`
	LegalPersonIDNumber         string                  `json:"legal_person_id_number,omitempty"`
	IDCardFrontAssetID          *int64                  `json:"id_card_front_asset_id,omitempty"`
	IDCardBackAssetID           *int64                  `json:"id_card_back_asset_id,omitempty"`
	IDCardFrontOCR              *MerchantIDCardOCRData  `json:"id_card_front_ocr,omitempty"`
	IDCardBackOCR               *MerchantIDCardOCRData  `json:"id_card_back_ocr,omitempty"`
	TrademarkCertificateAssetID *int64                  `json:"trademark_certificate_asset_id,omitempty"`
	Address                     *string                 `json:"address,omitempty"`
	RegionID                    *int64                  `json:"region_id,omitempty"`
	Status                      string                  `json:"status"`
	RejectReason                *string                 `json:"reject_reason,omitempty"`
	ReviewedBy                  *int64                  `json:"reviewed_by,omitempty"`
	ReviewedAt                  *time.Time              `json:"reviewed_at,omitempty"`
	CreatedAt                   time.Time               `json:"created_at"`
	UpdatedAt                   time.Time               `json:"updated_at"`
}

type groupApplicationDataPayload struct {
	BusinessLicenseOCR          *BusinessLicenseOCRData `json:"business_license_ocr,omitempty"`
	LegalPersonName             string                  `json:"legal_person_name,omitempty"`
	LegalPersonIDNumber         string                  `json:"legal_person_id_number,omitempty"`
	IDCardFrontAssetID          *int64                  `json:"id_card_front_asset_id,omitempty"`
	IDCardBackAssetID           *int64                  `json:"id_card_back_asset_id,omitempty"`
	IDCardFrontOCR              *MerchantIDCardOCRData  `json:"id_card_front_ocr,omitempty"`
	IDCardBackOCR               *MerchantIDCardOCRData  `json:"id_card_back_ocr,omitempty"`
	TrademarkCertificateAssetID *int64                  `json:"trademark_certificate_asset_id,omitempty"`
}

type groupResponse struct {
	ID                  int64     `json:"id"`
	Name                string    `json:"name"`
	OwnerUserID         int64     `json:"owner_user_id"`
	Status              string    `json:"status"`
	ContactPhone        *string   `json:"contact_phone,omitempty"`
	LicenseNumber       *string   `json:"license_number,omitempty"`
	LicenseImageAssetID *int64    `json:"license_image_asset_id,omitempty"`
	Address             *string   `json:"address,omitempty"`
	RegionID            *int64    `json:"region_id,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type groupApplicationReviewResponse struct {
	Application groupApplicationResponse `json:"application"`
	Group       groupResponse            `json:"group"`
}

type groupMerchantResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	LogoAssetID *int64 `json:"-"`
	LogoURL     string `json:"logo_url,omitempty"`
	Address     string `json:"address"`
	Phone       string `json:"phone"`
	Status      string `json:"status"`
}

type brandResponse struct {
	ID          int64     `json:"id"`
	GroupID     int64     `json:"group_id"`
	Name        string    `json:"name"`
	LogoAssetID *int64    `json:"-"`
	LogoURL     string    `json:"logo_url,omitempty"`
	Description *string   `json:"description,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type groupJoinRequestResponse struct {
	ID              int64      `json:"id"`
	GroupID         int64      `json:"group_id"`
	GroupName       *string    `json:"group_name,omitempty"`
	MerchantID      int64      `json:"merchant_id"`
	ApplicantUserID int64      `json:"applicant_user_id"`
	Status          string     `json:"status"`
	Reason          *string    `json:"reason,omitempty"`
	ReviewedBy      *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type groupPoliciesResponse struct {
	GroupID       int64  `json:"group_id"`
	PricingMode   string `json:"pricing_mode"`
	MenuMode      string `json:"menu_mode"`
	InventoryMode string `json:"inventory_mode"`
	PromotionMode string `json:"promotion_mode"`
}

type groupTemplateResponse struct {
	ID        int64     `json:"id"`
	GroupID   int64     `json:"group_id"`
	Version   int32     `json:"version"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type groupApplicationDocumentType string

const (
	groupApplicationDocumentBusinessLicense      groupApplicationDocumentType = "business_license"
	groupApplicationDocumentIDCardFront          groupApplicationDocumentType = "id_card_front"
	groupApplicationDocumentIDCardBack           groupApplicationDocumentType = "id_card_back"
	groupApplicationDocumentTrademarkCertificate groupApplicationDocumentType = "trademark_certificate"
)

var errGroupApplicationRequiredDocumentsMissing = errors.New("请上传营业执照、负责人身份证正反面和注册商标证书后再提交")

type brandTemplateResponse struct {
	ID        int64     `json:"id"`
	BrandID   int64     `json:"brand_id"`
	Version   int32     `json:"version"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newGroupApplicationResponse(app db.MerchantGroupApplication) (groupApplicationResponse, error) {
	resp := groupApplicationResponse{
		ID:              app.ID,
		ApplicantUserID: app.ApplicantUserID,
		GroupName:       app.GroupName,
		ContactPhone:    app.ContactPhone,
		Status:          app.Status,
		CreatedAt:       app.CreatedAt,
		UpdatedAt:       app.UpdatedAt,
	}
	resp.LicenseNumber = pgTextToPtr(app.LicenseNumber)
	resp.LicenseImageAssetID = pgInt8ToPtr(app.LicenseMediaAssetID)
	resp.Address = pgTextToPtr(app.Address)
	resp.RegionID = pgInt8ToPtr(app.RegionID)
	resp.RejectReason = pgTextToPtr(app.RejectReason)
	resp.ReviewedBy = pgInt8ToPtr(app.ReviewedBy)
	resp.ReviewedAt = pgTimeToPtr(app.ReviewedAt)

	var payload groupApplicationDataPayload
	if len(app.ApplicationData) > 0 {
		if err := json.Unmarshal(app.ApplicationData, &payload); err != nil {
			return groupApplicationResponse{}, fmt.Errorf("decode group application %d application_data: %w", app.ID, err)
		}
	}
	resp.BusinessLicenseOCR = payload.BusinessLicenseOCR
	resp.LegalPersonName = payload.LegalPersonName
	if resp.LegalPersonName == "" && payload.IDCardFrontOCR != nil {
		resp.LegalPersonName = payload.IDCardFrontOCR.Name
	}
	resp.LegalPersonIDNumber = payload.LegalPersonIDNumber
	if resp.LegalPersonIDNumber == "" && payload.IDCardFrontOCR != nil {
		resp.LegalPersonIDNumber = payload.IDCardFrontOCR.IDNumber
	}
	resp.IDCardFrontAssetID = payload.IDCardFrontAssetID
	resp.IDCardBackAssetID = payload.IDCardBackAssetID
	resp.IDCardFrontOCR = payload.IDCardFrontOCR
	resp.IDCardBackOCR = payload.IDCardBackOCR
	resp.TrademarkCertificateAssetID = payload.TrademarkCertificateAssetID
	return resp, nil
}

func (server *Server) writeGroupApplicationResponse(ctx *gin.Context, status int, app db.MerchantGroupApplication) bool {
	resp, err := newGroupApplicationResponse(app)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return false
	}
	ctx.JSON(status, resp)
	return true
}

func newGroupResponse(group db.MerchantGroup) groupResponse {
	resp := groupResponse{
		ID:          group.ID,
		Name:        group.Name,
		OwnerUserID: group.OwnerUserID,
		Status:      group.Status,
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
	}
	resp.ContactPhone = pgTextToPtr(group.ContactPhone)
	resp.LicenseNumber = pgTextToPtr(group.LicenseNumber)
	resp.LicenseImageAssetID = pgInt8ToPtr(group.LicenseMediaAssetID)
	resp.Address = pgTextToPtr(group.Address)
	resp.RegionID = pgInt8ToPtr(group.RegionID)
	return resp
}

func newBrandResponse(brand db.MerchantBrand) brandResponse {
	resp := brandResponse{
		ID:        brand.ID,
		GroupID:   brand.GroupID,
		Name:      brand.Name,
		Status:    brand.Status,
		CreatedAt: brand.CreatedAt,
		UpdatedAt: brand.UpdatedAt,
	}
	resp.LogoAssetID = pgInt8ToPtr(brand.LogoMediaAssetID)
	resp.Description = pgTextToPtr(brand.Description)
	return resp
}

func newGroupJoinRequestResponse(req db.MerchantGroupJoinRequest) groupJoinRequestResponse {
	resp := groupJoinRequestResponse{
		ID:              req.ID,
		GroupID:         req.GroupID,
		MerchantID:      req.MerchantID,
		ApplicantUserID: req.ApplicantUserID,
		Status:          req.Status,
		CreatedAt:       req.CreatedAt,
	}
	resp.Reason = pgTextToPtr(req.Reason)
	resp.ReviewedBy = pgInt8ToPtr(req.ReviewedBy)
	resp.ReviewedAt = pgTimeToPtr(req.ReviewedAt)
	return resp
}

func newGroupJoinRequestWithGroupResponse(row db.ListGroupJoinRequestsByMerchantRow) groupJoinRequestResponse {
	resp := groupJoinRequestResponse{
		ID:              row.ID,
		GroupID:         row.GroupID,
		GroupName:       &row.GroupName,
		MerchantID:      row.MerchantID,
		ApplicantUserID: row.ApplicantUserID,
		Status:          row.Status,
		CreatedAt:       row.CreatedAt,
	}
	resp.Reason = pgTextToPtr(row.Reason)
	resp.ReviewedBy = pgInt8ToPtr(row.ReviewedBy)
	resp.ReviewedAt = pgTimeToPtr(row.ReviewedAt)
	return resp
}

func newGroupPoliciesResponse(policy db.GroupPolicy) groupPoliciesResponse {
	return groupPoliciesResponse{
		GroupID:       policy.GroupID,
		PricingMode:   policy.PricingMode,
		MenuMode:      policy.MenuMode,
		InventoryMode: policy.InventoryMode,
		PromotionMode: policy.PromotionMode,
	}
}

func (server *Server) requireGroupRole(ctx *gin.Context, groupID int64, allowedRoles ...string) (string, bool) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	role, err := server.store.GetGroupMemberRole(ctx, db.GetGroupMemberRoleParams{
		GroupID: groupID,
		UserID:  authPayload.UserID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(errors.New("you are not a member of this group")))
			return "", false
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
		return "", false
	}

	if len(allowedRoles) > 0 {
		allowed := false
		for _, r := range allowedRoles {
			if role == r {
				allowed = true
				break
			}
		}
		if !allowed {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(errors.New("insufficient group permissions")))
			return "", false
		}
	}

	return role, true
}

func (server *Server) createGroupAuditLog(ctx *gin.Context, groupID pgtype.Int8, actorUserID int64, action, targetType string, targetID pgtype.Int8, metadata []byte) error {
	_, err := server.store.CreateGroupAuditLog(ctx, db.CreateGroupAuditLogParams{
		GroupID:     groupID,
		ActorUserID: pgtype.Int8{Int64: actorUserID, Valid: true},
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Metadata:    metadata,
	})
	return err
}

// createGroupApplicationDraft godoc
// @Summary 创建集团入驻草稿
// @Description 创建集团入驻申请草稿（已有草稿则返回）
// @Tags 集团申请
// @Produce json
// @Success 201 {object} groupApplicationResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/applications [post]
// @Security BearerAuth
func (server *Server) createGroupApplicationDraft(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	existing, err := server.store.GetLatestGroupApplicationByApplicant(ctx, authPayload.UserID)
	if err == nil && existing.Status == "draft" {
		// 已存在草稿，直接返回 200（这是 get-or-create 的 found 分支，不是新建资源）
		server.writeGroupApplicationResponse(ctx, http.StatusOK, existing)
		return
	}
	if err != nil && !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	app, err := server.store.CreateGroupApplicationDraft(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.writeGroupApplicationResponse(ctx, http.StatusCreated, app)
}

// getOrCreateGroupApplicationDraft godoc
// @Summary 获取集团入驻草稿
// @Description 获取当前用户的集团入驻草稿，不存在则创建
// @Tags 集团申请
// @Produce json
// @Success 201 {object} groupApplicationResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/applications/me [get]
// @Security BearerAuth
func (server *Server) getOrCreateGroupApplicationDraft(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	app, err := server.store.GetLatestGroupApplicationByApplicant(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			app, err = server.store.CreateGroupApplicationDraft(ctx, authPayload.UserID)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	server.writeGroupApplicationResponse(ctx, http.StatusOK, app)
}

type updateGroupApplicationBasicRequest struct {
	GroupName                   *string `json:"group_name,omitempty"`
	ContactPhone                *string `json:"contact_phone,omitempty"`
	LicenseNumber               *string `json:"license_number,omitempty"`
	LicenseImageAssetID         *int64  `json:"license_image_asset_id,omitempty"`
	TrademarkCertificateAssetID *int64  `json:"trademark_certificate_asset_id,omitempty"`
	Address                     *string `json:"address,omitempty"`
	RegionID                    *int64  `json:"region_id,omitempty"`
}

// updateGroupApplicationBasic godoc
// @Summary 更新集团入驻基础信息
// @Description 更新集团入驻申请基础信息（可编辑状态）
// @Tags 集团申请
// @Accept json
// @Produce json
// @Param request body updateGroupApplicationBasicRequest true "更新内容"
// @Success 200 {object} groupApplicationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/applications/basic [put]
// @Security BearerAuth
func (server *Server) updateGroupApplicationBasic(ctx *gin.Context) {
	var req updateGroupApplicationBasicRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	app, err := server.store.GetLatestGroupApplicationByApplicant(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("application not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status == "submitted" || app.Status == "approved" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("application is not editable")))
		return
	}

	if req.LicenseImageAssetID != nil {
		status, err := server.validateGroupApplicationDocumentMediaAsset(ctx, authPayload.UserID, *req.LicenseImageAssetID, expectedGroupApplicationOCRMediaCategories(ocr.DocumentTypeBusinessLicense, ocr.DocumentSideUnknown))
		if err != nil {
			if status == http.StatusInternalServerError {
				ctx.JSON(status, internalError(ctx, err))
				return
			}
			ctx.JSON(status, errorResponse(err))
			return
		}
	}
	if req.TrademarkCertificateAssetID != nil {
		status, err := server.validateGroupApplicationDocumentMediaAsset(ctx, authPayload.UserID, *req.TrademarkCertificateAssetID, []media.Category{media.CategoryGroupTrademarkCertificate})
		if err != nil {
			if status == http.StatusInternalServerError {
				ctx.JSON(status, internalError(ctx, err))
				return
			}
			ctx.JSON(status, errorResponse(err))
			return
		}
	}
	if app.Status == "rejected" {
		app, err = server.store.ResetGroupApplicationToDraft(ctx, app.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	groupName := app.GroupName
	if req.GroupName != nil {
		groupName = *req.GroupName
	}
	contactPhone := app.ContactPhone
	if req.ContactPhone != nil {
		contactPhone = *req.ContactPhone
	}
	var applicationData []byte
	if req.TrademarkCertificateAssetID != nil {
		applicationData, err = marshalGroupApplicationDataPatch(map[string]any{
			"trademark_certificate_asset_id": *req.TrademarkCertificateAssetID,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	update := db.UpdateGroupApplicationBasicParams{
		ID:                  app.ID,
		GroupName:           groupName,
		ContactPhone:        contactPhone,
		LicenseNumber:       toPgText(req.LicenseNumber),
		LicenseMediaAssetID: toPgInt8(req.LicenseImageAssetID),
		Address:             toPgText(req.Address),
		RegionID:            toPgInt8(req.RegionID),
		ApplicationData:     applicationData,
	}

	updated, err := server.store.UpdateGroupApplicationBasic(ctx, update)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.writeGroupApplicationResponse(ctx, http.StatusOK, updated)
}

func (server *Server) validateGroupApplicationDocumentMediaAsset(ctx *gin.Context, applicantUserID, mediaAssetID int64, expectedCategories []media.Category) (int, error) {
	asset, err := server.store.GetMediaAssetByID(ctx, mediaAssetID)
	if err != nil {
		if isNotFoundError(err) {
			return http.StatusNotFound, errors.New("media asset not found")
		}
		return http.StatusInternalServerError, err
	}
	if asset.UploadedBy != applicantUserID {
		return http.StatusForbidden, errors.New("forbidden")
	}
	if strings.ToLower(strings.TrimSpace(asset.UploadStatus)) != "confirmed" {
		return http.StatusBadRequest, errOCRMediaNotConfirmed
	}
	if !ocrMediaCategoryAllowed(asset.MediaCategory, expectedCategories) {
		return http.StatusBadRequest, errOCRMediaWrongCategory
	}
	return 0, nil
}

func validateGroupApplicationSubmitDocuments(app db.MerchantGroupApplication) error {
	resp, err := newGroupApplicationResponse(app)
	if err != nil {
		return err
	}
	if resp.LicenseImageAssetID == nil || *resp.LicenseImageAssetID <= 0 {
		return errGroupApplicationRequiredDocumentsMissing
	}
	if resp.IDCardFrontAssetID == nil || *resp.IDCardFrontAssetID <= 0 {
		return errGroupApplicationRequiredDocumentsMissing
	}
	if resp.IDCardBackAssetID == nil || *resp.IDCardBackAssetID <= 0 {
		return errGroupApplicationRequiredDocumentsMissing
	}
	if resp.TrademarkCertificateAssetID == nil || *resp.TrademarkCertificateAssetID <= 0 {
		return errGroupApplicationRequiredDocumentsMissing
	}
	return nil
}

func (server *Server) deleteGroupApplicationDocumentByType(ctx *gin.Context, documentType groupApplicationDocumentType) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	switch documentType {
	case groupApplicationDocumentBusinessLicense, groupApplicationDocumentIDCardFront, groupApplicationDocumentIDCardBack, groupApplicationDocumentTrademarkCertificate:
	default:
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid document type")))
		return
	}

	app, err := server.store.GetLatestGroupApplicationByApplicant(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("application not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status == "submitted" || app.Status == "approved" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("application is not editable")))
		return
	}

	if app.Status == "rejected" {
		app, err = server.store.ResetGroupApplicationToDraft(ctx, app.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	resp, err := newGroupApplicationResponse(app)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	var (
		updated db.MerchantGroupApplication
		assetID int64
	)

	switch documentType {
	case groupApplicationDocumentBusinessLicense:
		if resp.LicenseImageAssetID != nil {
			assetID = *resp.LicenseImageAssetID
		}
		updated, err = server.store.ClearGroupApplicationBusinessLicense(ctx, app.ID)
	case groupApplicationDocumentIDCardFront:
		if resp.IDCardFrontAssetID != nil {
			assetID = *resp.IDCardFrontAssetID
		}
		updated, err = server.store.ClearGroupApplicationIDCardFront(ctx, app.ID)
	case groupApplicationDocumentIDCardBack:
		if resp.IDCardBackAssetID != nil {
			assetID = *resp.IDCardBackAssetID
		}
		updated, err = server.store.ClearGroupApplicationIDCardBack(ctx, app.ID)
	default:
		if resp.TrademarkCertificateAssetID != nil {
			assetID = *resp.TrademarkCertificateAssetID
		}
		updated, err = server.store.ClearGroupApplicationTrademarkCertificate(ctx, app.ID)
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if assetID > 0 {
		if err := server.mediaRegistry.SoftDelete(ctx, assetID, authPayload.UserID); err != nil {
			log.Warn().Err(err).Int64("asset_id", assetID).Str("document_type", string(documentType)).Msg("delete group application document: soft delete media failed")
		}
	}

	server.writeGroupApplicationResponse(ctx, http.StatusOK, updated)
}

// deleteGroupApplicationDocument godoc
// @Summary 删除集团申请证照
// @Description 删除集团草稿中的单个证照绑定，并清空对应 OCR 结果。支持证照类型：business_license、id_card_front、id_card_back、trademark_certificate。
// @Tags 集团申请
// @Produce json
// @Param document_type path string true "证照类型: business_license|id_card_front|id_card_back|trademark_certificate"
// @Success 200 {object} groupApplicationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/applications/documents/{document_type} [delete]
// @Security BearerAuth
func (server *Server) deleteGroupApplicationDocument(ctx *gin.Context) {
	documentType := groupApplicationDocumentType(ctx.Param("document_type"))
	server.deleteGroupApplicationDocumentByType(ctx, documentType)
}

// submitGroupApplication godoc
// @Summary 提交集团入驻申请
// @Description 提交集团入驻申请进入审核流程
// @Tags 集团申请
// @Produce json
// @Success 200 {object} groupApplicationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/applications/submit [post]
// @Security BearerAuth
func (server *Server) submitGroupApplication(ctx *gin.Context) {
	consentReq, err := parseAgreementConsentRequest(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	app, err := server.store.GetLatestGroupApplicationByApplicant(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("application not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status != "draft" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only draft applications can be submitted")))
		return
	}
	if app.GroupName == "" || app.ContactPhone == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("group_name and contact_phone are required")))
		return
	}
	if err := validateGroupApplicationSubmitDocuments(app); err != nil {
		if errors.Is(err, errGroupApplicationRequiredDocumentsMissing) {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.writeAgreementConsentAudit(ctx, authPayload.UserID, "group_application_consent_confirmed", "group_application", app.ID, consentReq, nil)

	updated, err := server.store.SubmitGroupApplication(ctx, app.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.writeGroupApplicationResponse(ctx, http.StatusOK, updated)
}

type reviewGroupApplicationRequest struct {
	Status       string  `json:"status" binding:"required"`
	RejectReason *string `json:"reject_reason,omitempty"`
}

// reviewGroupApplication godoc
// @Summary 审核集团入驻申请
// @Description 管理员审核集团入驻申请（通过/拒绝）
// @Tags 集团申请
// @Accept json
// @Produce json
// @Param id path int true "申请ID"
// @Param request body reviewGroupApplicationRequest true "审核信息"
// @Success 200 {object} map[string]any
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/applications/{id}/review [post]
// @Security BearerAuth
func (server *Server) reviewGroupApplication(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid application id")))
		return
	}

	var req reviewGroupApplicationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	app, err := server.store.GetGroupApplication(ctx, id)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("application not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if app.Status != "submitted" {
		server.logSecurityRejection(ctx, securityRejectionInput{
			ActorUserID: authPayload.UserID,
			ActorRole:   "admin",
			Action:      "group_application_review_conflict",
			TargetType:  "group_application",
			TargetID:    app.ID,
			Reason:      "application_status_not_submitted",
			Audit:       true,
			Metadata: map[string]any{
				"current_status": app.Status,
			},
		})
		ctx.JSON(http.StatusConflict, errorResponse(ErrGroupApplicationReviewConflict))
		return
	}

	switch req.Status {
	case "approved":
		result, err := server.store.ApproveGroupApplicationTx(ctx, db.ApproveGroupApplicationTxParams{
			ApplicationID:  app.ID,
			ReviewerUserID: authPayload.UserID,
		})
		if err != nil {
			if errors.Is(err, db.ErrGroupApplicationReviewConflict) {
				server.logSecurityRejection(ctx, securityRejectionInput{
					ActorUserID: authPayload.UserID,
					ActorRole:   "admin",
					Action:      "group_application_review_conflict",
					TargetType:  "group_application",
					TargetID:    app.ID,
					Reason:      "approve_conflict",
					Audit:       true,
					Metadata: map[string]any{
						"requested_status": "approved",
					},
				})
				ctx.JSON(http.StatusConflict, errorResponse(ErrGroupApplicationReviewConflict))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		resp, err := newGroupApplicationResponse(result.Application)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		ctx.JSON(http.StatusOK, groupApplicationReviewResponse{
			Application: resp,
			Group:       newGroupResponse(result.Group),
		})
	case "rejected":
		if req.RejectReason == nil || *req.RejectReason == "" {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("reject_reason is required")))
			return
		}
		reviewedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		updated, err := server.store.ReviewSubmittedGroupApplication(ctx, db.ReviewSubmittedGroupApplicationParams{
			ID:           app.ID,
			Status:       "rejected",
			RejectReason: toPgText(req.RejectReason),
			ReviewedBy:   pgtype.Int8{Int64: authPayload.UserID, Valid: true},
			ReviewedAt:   reviewedAt,
		})
		if err != nil {
			if isNotFoundError(err) {
				server.logSecurityRejection(ctx, securityRejectionInput{
					ActorUserID: authPayload.UserID,
					ActorRole:   "admin",
					Action:      "group_application_review_conflict",
					TargetType:  "group_application",
					TargetID:    app.ID,
					Reason:      "reject_conflict",
					Audit:       true,
					Metadata: map[string]any{
						"requested_status": "rejected",
					},
				})
				ctx.JSON(http.StatusConflict, errorResponse(ErrGroupApplicationReviewConflict))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		meta, _ := json.Marshal(map[string]any{
			"application_id": app.ID,
			"reason":         req.RejectReason,
		})
		if err := server.createGroupAuditLog(ctx, pgtype.Int8{Valid: false}, authPayload.UserID, "group_application_rejected", "group_application", pgtype.Int8{Int64: app.ID, Valid: true}, meta); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		server.writeGroupApplicationResponse(ctx, http.StatusOK, updated)
	default:
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid status")))
	}
}

// ==================== Groups & Brands ====================

type createGroupRequest struct {
	Name                string          `json:"name" binding:"required"`
	OwnerUserID         int64           `json:"owner_user_id" binding:"required"`
	ContactPhone        *string         `json:"contact_phone,omitempty"`
	LicenseNumber       *string         `json:"license_number,omitempty"`
	LicenseImageAssetID *int64          `json:"license_image_asset_id,omitempty"`
	Address             *string         `json:"address,omitempty"`
	RegionID            *int64          `json:"region_id,omitempty"`
	ApplicationData     json.RawMessage `json:"application_data,omitempty"`
}

// createGroup godoc
// @Summary 创建集团
// @Description 管理员创建集团（手动）
// @Tags 集团管理
// @Accept json
// @Produce json
// @Param request body createGroupRequest true "集团信息"
// @Success 200 {object} groupResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups [post]
// @Security BearerAuth
func (server *Server) createGroup(ctx *gin.Context) {
	var req createGroupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	group, err := server.store.CreateMerchantGroup(ctx, db.CreateMerchantGroupParams{
		Name:                req.Name,
		OwnerUserID:         req.OwnerUserID,
		ContactPhone:        toPgText(req.ContactPhone),
		LicenseNumber:       toPgText(req.LicenseNumber),
		LicenseMediaAssetID: toPgInt8(req.LicenseImageAssetID),
		Address:             toPgText(req.Address),
		RegionID:            toPgInt8(req.RegionID),
		ApplicationData:     req.ApplicationData,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	_, err = server.store.CreateGroupMember(ctx, db.CreateGroupMemberParams{
		GroupID:   group.ID,
		UserID:    req.OwnerUserID,
		Role:      "owner",
		InvitedBy: pgtype.Int8{Int64: authPayload.UserID, Valid: true},
	})
	if err != nil && !isDuplicateKeyError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	meta, _ := json.Marshal(map[string]any{"group_id": group.ID})
	if err := server.createGroupAuditLog(ctx, pgtype.Int8{Int64: group.ID, Valid: true}, authPayload.UserID, "group_created", "group", pgtype.Int8{Int64: group.ID, Valid: true}, meta); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newGroupResponse(group))
}

// searchGroups godoc
// @Summary 搜索集团
// @Description 按关键字搜索集团（仅返回 active）
// @Tags 集团管理
// @Produce json
// @Param keyword query string false "关键词"
// @Param limit query int false "分页大小"
// @Param offset query int false "偏移"
// @Success 201 {array} groupResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups [get]
// @Security BearerAuth
func (server *Server) searchGroups(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	limit := int32(20)
	offset := int32(0)
	if v := ctx.Query("limit"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 32); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}
	if v := ctx.Query("offset"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 32); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	params := db.ListMerchantGroupsParams{
		Column1: keyword,
		Limit:   limit,
		Offset:  offset,
	}

	groups, err := server.store.ListMerchantGroups(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]groupResponse, 0, len(groups))
	for _, g := range groups {
		resp = append(resp, newGroupResponse(g))
	}

	ctx.JSON(http.StatusOK, resp)
}

// getGroup godoc
// @Summary 获取集团详情
// @Description 获取集团详情（需为集团成员）
// @Tags 集团管理
// @Produce json
// @Param id path int true "集团ID"
// @Success 200 {object} groupResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id} [get]
// @Security BearerAuth
func (server *Server) getGroup(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}

	if _, ok := server.requireGroupRole(ctx, groupID); !ok {
		return
	}

	group, err := server.store.GetMerchantGroup(ctx, groupID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("group not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newGroupResponse(group))
}

type updateGroupRequest struct {
	Name                *string `json:"name,omitempty"`
	ContactPhone        *string `json:"contact_phone,omitempty"`
	LicenseNumber       *string `json:"license_number,omitempty"`
	LicenseImageAssetID *int64  `json:"license_image_asset_id,omitempty"`
	Address             *string `json:"address,omitempty"`
	RegionID            *int64  `json:"region_id,omitempty"`
	Status              *string `json:"status,omitempty"`
}

// updateGroup godoc
// @Summary 更新集团信息
// @Description 更新集团信息（owner/admin）
// @Tags 集团管理
// @Accept json
// @Produce json
// @Param id path int true "集团ID"
// @Param request body updateGroupRequest true "更新内容"
// @Success 200 {object} groupResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id} [patch]
// @Security BearerAuth
func (server *Server) updateGroup(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}

	if _, ok := server.requireGroupRole(ctx, groupID, "owner", "admin"); !ok {
		return
	}

	var req updateGroupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.Status != nil && *req.Status != "active" && *req.Status != "disabled" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid status")))
		return
	}

	current, err := server.store.GetMerchantGroup(ctx, groupID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("group not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	name := current.Name
	if req.Name != nil {
		name = *req.Name
	}
	status := current.Status
	if req.Status != nil {
		status = *req.Status
	}

	updated, err := server.store.UpdateMerchantGroup(ctx, db.UpdateMerchantGroupParams{
		ID:                  groupID,
		Name:                name,
		ContactPhone:        toPgText(req.ContactPhone),
		LicenseNumber:       toPgText(req.LicenseNumber),
		LicenseMediaAssetID: toPgInt8(req.LicenseImageAssetID),
		Address:             toPgText(req.Address),
		RegionID:            toPgInt8(req.RegionID),
		Status:              status,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newGroupResponse(updated))
}

// listGroupMerchants godoc
// @Summary 获取集团门店列表
// @Description 获取集团下所有门店（需为集团成员）
// @Tags 集团管理
// @Produce json
// @Param id path int true "集团ID"
// @Success 200 {array} groupMerchantResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id}/merchants [get]
// @Security BearerAuth
func (server *Server) listGroupMerchants(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}

	if _, ok := server.requireGroupRole(ctx, groupID); !ok {
		return
	}

	merchants, err := server.store.ListGroupMerchants(ctx, pgtype.Int8{Int64: groupID, Valid: true})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]groupMerchantResponse, 0, len(merchants))
	for _, m := range merchants {
		resp = append(resp, groupMerchantResponse{
			ID:          m.ID,
			Name:        m.Name,
			LogoAssetID: int64PtrFromPgInt8(m.LogoMediaAssetID),
			Address:     m.Address,
			Phone:       m.Phone,
			Status:      m.Status,
		})
	}

	for i := range resp {
		if resp[i].LogoAssetID != nil {
			resp[i].LogoURL = server.publicImageURL(ctx, resp[i].LogoAssetID, media.VariantCard)
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// listGroupBrands godoc
// @Summary 获取集团品牌列表
// @Description 获取集团下所有品牌（需为集团成员）
// @Tags 品牌管理
// @Produce json
// @Param id path int true "集团ID"
// @Success 200 {array} brandResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id}/brands [get]
// @Security BearerAuth
func (server *Server) listGroupBrands(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}

	if _, ok := server.requireGroupRole(ctx, groupID); !ok {
		return
	}

	brands, err := server.store.ListMerchantBrandsByGroup(ctx, groupID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]brandResponse, 0, len(brands))
	for _, b := range brands {
		resp = append(resp, newBrandResponse(b))
	}

	for i := range resp {
		if resp[i].LogoAssetID != nil {
			resp[i].LogoURL = server.publicImageURL(ctx, resp[i].LogoAssetID, media.VariantCard)
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

type createGroupBrandRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description,omitempty"`
	LogoAssetID *int64  `json:"logo_asset_id,omitempty"`
}

// createGroupBrand godoc
// @Summary 创建品牌
// @Description 在集团下创建品牌（owner/admin）
// @Tags 品牌管理
// @Accept json
// @Produce json
// @Param id path int true "集团ID"
// @Param request body createGroupBrandRequest true "品牌信息"
// @Success 200 {object} brandResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id}/brands [post]
// @Security BearerAuth
func (server *Server) createGroupBrand(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}

	if _, ok := server.requireGroupRole(ctx, groupID, "owner", "admin"); !ok {
		return
	}

	var req createGroupBrandRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	brand, err := server.store.CreateMerchantBrand(ctx, db.CreateMerchantBrandParams{
		GroupID:          groupID,
		Name:             req.Name,
		LogoMediaAssetID: toPgInt8(req.LogoAssetID),
		Description:      toPgText(req.Description),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	brandResp := newBrandResponse(brand)
	if brandResp.LogoAssetID != nil {
		brandResp.LogoURL = server.publicImageURL(ctx, brandResp.LogoAssetID, media.VariantCard)
	}
	ctx.JSON(http.StatusCreated, brandResp)
}

// getBrand godoc
// @Summary 获取品牌详情
// @Description 获取品牌详情（需为集团成员）
// @Tags 品牌管理
// @Produce json
// @Param id path int true "品牌ID"
// @Success 201 {object} brandResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/brands/{id} [get]
// @Security BearerAuth
func (server *Server) getBrand(ctx *gin.Context) {
	brandID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid brand id")))
		return
	}

	brand, err := server.store.GetMerchantBrand(ctx, brandID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("brand not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if _, ok := server.requireGroupRole(ctx, brand.GroupID); !ok {
		return
	}

	brandResp := newBrandResponse(brand)
	if brandResp.LogoAssetID != nil {
		brandResp.LogoURL = server.publicImageURL(ctx, brandResp.LogoAssetID, media.VariantCard)
	}
	ctx.JSON(http.StatusOK, brandResp)
}

// ==================== Join Requests ====================

type createGroupJoinRequestRequest struct {
	Reason *string `json:"reason,omitempty"`
}

// createGroupJoinRequest godoc
// @Summary 申请加入集团
// @Description 门店发起加入集团申请（需店主）
// @Tags 集团管理
// @Accept json
// @Produce json
// @Param id path int true "集团ID"
// @Param request body createGroupJoinRequestRequest false "申请原因"
// @Success 200 {object} groupJoinRequestResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id}/join-requests [post]
// @Security BearerAuth
func (server *Server) createGroupJoinRequest(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant not found")))
		return
	}

	affiliation, err := server.store.GetMerchantGroupAffiliation(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if affiliation.GroupID.Valid {
		ctx.JSON(http.StatusConflict, errorResponse(ErrMerchantAlreadyJoinedGroup))
		return
	}

	group, err := server.store.GetMerchantGroup(ctx, groupID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("group not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if group.Status != "active" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("group is not active")))
		return
	}

	var req createGroupJoinRequestRequest
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	result, err := server.store.CreateGroupJoinRequestTx(ctx, db.CreateGroupJoinRequestTxParams{
		GroupID:         groupID,
		MerchantID:      merchant.ID,
		ApplicantUserID: authPayload.UserID,
		Reason:          toPgText(req.Reason),
	})
	if err != nil {
		if isDuplicateKeyError(err) {
			ctx.JSON(http.StatusConflict, errorResponse(ErrGroupJoinRequestAlreadyPending))
			return
		}
		if errors.Is(err, db.ErrMerchantAlreadyJoinedGroup) {
			ctx.JSON(http.StatusConflict, errorResponse(ErrMerchantAlreadyJoinedGroup))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newGroupJoinRequestResponse(result.Request))
}

// listMyGroupJoinRequests godoc
// @Summary 获取当前商户加入集团申请
// @Description 获取当前商户发起过的集团加入申请列表（需店主）
// @Tags 集团管理
// @Produce json
// @Success 200 {array} groupJoinRequestResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/merchants/me/group-join-requests [get]
// @Security BearerAuth
func (server *Server) listMyGroupJoinRequests(ctx *gin.Context) {
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant not found")))
		return
	}

	requests, err := server.store.ListGroupJoinRequestsByMerchant(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]groupJoinRequestResponse, 0, len(requests))
	for _, r := range requests {
		resp = append(resp, newGroupJoinRequestWithGroupResponse(r))
	}
	ctx.JSON(http.StatusOK, resp)
}

// listGroupJoinRequests godoc
// @Summary 获取集团加入申请列表
// @Description 获取集团的门店加入申请列表
// @Tags 集团管理
// @Produce json
// @Param id path int true "集团ID"
// @Success 201 {array} groupJoinRequestResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id}/join-requests [get]
// @Security BearerAuth
func (server *Server) listGroupJoinRequests(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}

	if _, ok := server.requireGroupRole(ctx, groupID, "owner", "admin", "finance", "ops"); !ok {
		return
	}

	requests, err := server.store.ListGroupJoinRequestsByGroup(ctx, groupID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]groupJoinRequestResponse, 0, len(requests))
	for _, r := range requests {
		resp = append(resp, newGroupJoinRequestResponse(r))
	}
	ctx.JSON(http.StatusOK, resp)
}

type approveGroupJoinRequestRequest struct {
	BrandID *int64 `json:"brand_id,omitempty"`
}

// approveGroupJoinRequest godoc
// @Summary 审核通过加入申请
// @Description 集团审核通过门店加入申请（owner/admin）
// @Tags 集团管理
// @Accept json
// @Produce json
// @Param id path int true "集团ID"
// @Param request_id path int true "申请ID"
// @Param request body approveGroupJoinRequestRequest false "品牌归属"
// @Success 200 {object} groupJoinRequestResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id}/join-requests/{request_id}/approve [post]
// @Security BearerAuth
func (server *Server) approveGroupJoinRequest(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}

	if _, ok := server.requireGroupRole(ctx, groupID, "owner", "admin"); !ok {
		return
	}

	requestID, err := strconv.ParseInt(ctx.Param("request_id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid request id")))
		return
	}

	var req approveGroupJoinRequestRequest
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	brandID := pgtype.Int8{Valid: false}
	if req.BrandID != nil {
		brandID = pgtype.Int8{Int64: *req.BrandID, Valid: true}
		brand, err := server.store.GetMerchantBrand(ctx, *req.BrandID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("brand not found")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if brand.GroupID != groupID {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("brand does not belong to group")))
			return
		}
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	result, err := server.store.ApproveGroupJoinRequestTx(ctx, db.ApproveGroupJoinRequestTxParams{
		RequestID:      requestID,
		GroupID:        groupID,
		ReviewerUserID: authPayload.UserID,
		BrandID:        brandID,
	})
	if err != nil {
		if errors.Is(err, db.ErrGroupJoinRequestReviewConflict) {
			server.logSecurityRejection(ctx, securityRejectionInput{
				ActorUserID: authPayload.UserID,
				ActorRole:   "group",
				Action:      "group_join_request_review_conflict",
				TargetType:  "group_join_request",
				TargetID:    requestID,
				GroupID:     groupID,
				Reason:      "approve_conflict",
				Audit:       true,
			})
			ctx.JSON(http.StatusConflict, errorResponse(ErrGroupJoinRequestReviewConflict))
			return
		}
		if errors.Is(err, db.ErrMerchantAlreadyJoinedGroup) {
			server.logSecurityRejection(ctx, securityRejectionInput{
				ActorUserID: authPayload.UserID,
				ActorRole:   "group",
				Action:      "merchant_group_affiliation_conflict",
				TargetType:  "group_join_request",
				TargetID:    requestID,
				GroupID:     groupID,
				Reason:      "merchant_already_joined_group",
				Audit:       true,
			})
			ctx.JSON(http.StatusConflict, errorResponse(ErrMerchantAlreadyJoinedGroup))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newGroupJoinRequestResponse(result.Request))
}

type rejectGroupJoinRequestRequest struct {
	Reason *string `json:"reason,omitempty"`
}

// rejectGroupJoinRequest godoc
// @Summary 驳回加入申请
// @Description 集团驳回门店加入申请（owner/admin）
// @Tags 集团管理
// @Accept json
// @Produce json
// @Param id path int true "集团ID"
// @Param request_id path int true "申请ID"
// @Param request body rejectGroupJoinRequestRequest false "驳回原因"
// @Success 200 {object} groupJoinRequestResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id}/join-requests/{request_id}/reject [post]
// @Security BearerAuth
func (server *Server) rejectGroupJoinRequest(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}

	if _, ok := server.requireGroupRole(ctx, groupID, "owner", "admin"); !ok {
		return
	}

	requestID, err := strconv.ParseInt(ctx.Param("request_id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid request id")))
		return
	}

	var req rejectGroupJoinRequestRequest
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	result, err := server.store.RejectGroupJoinRequestTx(ctx, db.RejectGroupJoinRequestTxParams{
		RequestID:      requestID,
		GroupID:        groupID,
		ReviewerUserID: authPayload.UserID,
		Reason:         toPgText(req.Reason),
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("request not found")))
			return
		}
		if errors.Is(err, db.ErrGroupJoinRequestGroupMismatch) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("request does not belong to group")))
			return
		}
		if errors.Is(err, db.ErrGroupJoinRequestReviewConflict) {
			server.logSecurityRejection(ctx, securityRejectionInput{
				ActorUserID: authPayload.UserID,
				ActorRole:   "group",
				Action:      "group_join_request_review_conflict",
				TargetType:  "group_join_request",
				TargetID:    requestID,
				GroupID:     groupID,
				Reason:      "reject_conflict",
				Audit:       true,
			})
			ctx.JSON(http.StatusConflict, errorResponse(ErrGroupJoinRequestReviewConflict))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newGroupJoinRequestResponse(result.Request))
}

// cancelGroupJoinRequest godoc
// @Summary 撤回加入申请
// @Description 申请人撤回门店加入申请
// @Tags 集团管理
// @Produce json
// @Param id path int true "集团ID"
// @Param request_id path int true "申请ID"
// @Success 200 {object} groupJoinRequestResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id}/join-requests/{request_id}/cancel [post]
// @Security BearerAuth
func (server *Server) cancelGroupJoinRequest(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}
	requestID, err := strconv.ParseInt(ctx.Param("request_id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid request id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	result, err := server.store.CancelGroupJoinRequestTx(ctx, db.CancelGroupJoinRequestTxParams{
		RequestID:       requestID,
		GroupID:         groupID,
		ApplicantUserID: authPayload.UserID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("request not found")))
			return
		}
		if errors.Is(err, db.ErrGroupJoinRequestGroupMismatch) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("request does not belong to group")))
			return
		}
		if errors.Is(err, db.ErrGroupJoinRequestApplicantMismatch) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("only applicant can cancel")))
			return
		}
		if errors.Is(err, db.ErrGroupJoinRequestReviewConflict) {
			server.logSecurityRejection(ctx, securityRejectionInput{
				ActorUserID: authPayload.UserID,
				ActorRole:   "merchant",
				Action:      "group_join_request_cancel_conflict",
				TargetType:  "group_join_request",
				TargetID:    requestID,
				GroupID:     groupID,
				Reason:      "cancel_conflict",
				Audit:       true,
			})
			ctx.JSON(http.StatusConflict, errorResponse(ErrGroupJoinRequestReviewConflict))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newGroupJoinRequestResponse(result.Request))
}

// ==================== Policies & Templates ====================

type upsertGroupPoliciesRequest struct {
	PricingMode   string `json:"pricing_mode" binding:"required"`
	MenuMode      string `json:"menu_mode" binding:"required"`
	InventoryMode string `json:"inventory_mode" binding:"required"`
	PromotionMode string `json:"promotion_mode" binding:"required"`
}

// upsertGroupPolicies godoc
// @Summary 更新集团策略
// @Description 更新集团策略（owner/admin/ops）
// @Tags 集团管理
// @Accept json
// @Produce json
// @Param id path int true "集团ID"
// @Param request body upsertGroupPoliciesRequest true "策略配置"
// @Success 200 {object} groupPoliciesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id}/policies [put]
// @Security BearerAuth
func (server *Server) upsertGroupPolicies(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}

	if _, ok := server.requireGroupRole(ctx, groupID, "owner", "admin", "ops"); !ok {
		return
	}

	var req upsertGroupPoliciesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if !isValidPolicyMode(req.PricingMode) || !isValidPolicyMode(req.MenuMode) || !isValidPolicyMode(req.InventoryMode) || !isValidPolicyMode(req.PromotionMode) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid policy mode")))
		return
	}

	policy, err := server.store.UpsertGroupPolicies(ctx, db.UpsertGroupPoliciesParams{
		GroupID:       groupID,
		PricingMode:   req.PricingMode,
		MenuMode:      req.MenuMode,
		InventoryMode: req.InventoryMode,
		PromotionMode: req.PromotionMode,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newGroupPoliciesResponse(policy))
}

// getGroupPolicies godoc
// @Summary 获取集团策略
// @Description 获取集团策略信息（需为集团成员）
// @Tags 集团管理
// @Produce json
// @Param id path int true "集团ID"
// @Success 200 {object} groupPoliciesResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id}/policies [get]
// @Security BearerAuth
func (server *Server) getGroupPolicies(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}

	if _, ok := server.requireGroupRole(ctx, groupID); !ok {
		return
	}

	policy, err := server.store.GetGroupPolicies(ctx, groupID)
	if err != nil {
		if isNotFoundError(err) {
			// If not found, return default policies
			ctx.JSON(http.StatusOK, groupPoliciesResponse{
				GroupID:       groupID,
				PricingMode:   "store",
				MenuMode:      "store",
				InventoryMode: "store",
				PromotionMode: "store",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newGroupPoliciesResponse(policy))
}

type createGroupMenuTemplateRequest struct {
	Payload json.RawMessage `json:"payload" binding:"required"`
	Version *int32          `json:"version,omitempty"`
	Status  *string         `json:"status,omitempty"`
}

// createGroupMenuTemplate godoc
// @Summary 创建集团菜单模板
// @Description 创建集团菜单模板（owner/admin/ops）
// @Tags 集团管理
// @Accept json
// @Produce json
// @Param id path int true "集团ID"
// @Param request body createGroupMenuTemplateRequest true "模板信息"
// @Success 200 {object} groupTemplateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/groups/{id}/menu-templates [post]
// @Security BearerAuth
func (server *Server) createGroupMenuTemplate(ctx *gin.Context) {
	groupID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid group id")))
		return
	}

	if _, ok := server.requireGroupRole(ctx, groupID, "owner", "admin", "ops"); !ok {
		return
	}

	var req createGroupMenuTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	status := "active"
	if req.Status != nil {
		status = *req.Status
	}
	if status != "active" && status != "archived" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid status")))
		return
	}

	version := int32(1)
	if req.Version != nil && *req.Version > 0 {
		version = *req.Version
	}

	template, err := server.store.CreateGroupMenuTemplate(ctx, db.CreateGroupMenuTemplateParams{
		GroupID: groupID,
		Payload: req.Payload,
		Version: version,
		Status:  status,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, groupTemplateResponse{
		ID:        template.ID,
		GroupID:   template.GroupID,
		Version:   template.Version,
		Status:    template.Status,
		CreatedAt: template.CreatedAt,
		UpdatedAt: template.UpdatedAt,
	})
}

type createBrandMenuTemplateRequest struct {
	Payload json.RawMessage `json:"payload" binding:"required"`
	Version *int32          `json:"version,omitempty"`
	Status  *string         `json:"status,omitempty"`
}

// createBrandMenuTemplate godoc
// @Summary 创建品牌菜单模板
// @Description 创建品牌菜单模板（owner/admin/ops）
// @Tags 品牌管理
// @Accept json
// @Produce json
// @Param id path int true "品牌ID"
// @Param request body createBrandMenuTemplateRequest true "模板信息"
// @Success 201 {object} brandTemplateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/brands/{id}/menu-templates [post]
// @Security BearerAuth
func (server *Server) createBrandMenuTemplate(ctx *gin.Context) {
	brandID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid brand id")))
		return
	}

	brand, err := server.store.GetMerchantBrand(ctx, brandID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("brand not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if _, ok := server.requireGroupRole(ctx, brand.GroupID, "owner", "admin", "ops"); !ok {
		return
	}

	var req createBrandMenuTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	status := "active"
	if req.Status != nil {
		status = *req.Status
	}
	if status != "active" && status != "archived" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid status")))
		return
	}

	version := int32(1)
	if req.Version != nil && *req.Version > 0 {
		version = *req.Version
	}

	template, err := server.store.CreateBrandMenuTemplate(ctx, db.CreateBrandMenuTemplateParams{
		BrandID: brandID,
		Payload: req.Payload,
		Version: version,
		Status:  status,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, brandTemplateResponse{
		ID:        template.ID,
		BrandID:   template.BrandID,
		Version:   template.Version,
		Status:    template.Status,
		CreatedAt: template.CreatedAt,
		UpdatedAt: template.UpdatedAt,
	})
}

func isValidPolicyMode(mode string) bool {
	return mode == "central" || mode == "store"
}

func toPgText(val *string) pgtype.Text {
	if val == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *val, Valid: true}
}

func toPgInt8(val *int64) pgtype.Int8 {
	if val == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *val, Valid: true}
}
