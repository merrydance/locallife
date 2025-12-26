package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// 中国经纬度范围常量
const (
	minLongitude = 73.0  // 中国最西端
	maxLongitude = 135.0 // 中国最东端
	minLatitude  = 3.0   // 中国最南端
	maxLatitude  = 54.0  // 中国最北端
)

// validateCoordinates 验证经纬度是否在有效范围内
func validateCoordinates(longitude, latitude float64) error {
	if longitude < minLongitude || longitude > maxLongitude {
		return fmt.Errorf("经度必须在 %.1f 到 %.1f 之间", minLongitude, maxLongitude)
	}
	if latitude < minLatitude || latitude > maxLatitude {
		return fmt.Errorf("纬度必须在 %.1f 到 %.1f 之间", minLatitude, maxLatitude)
	}
	return nil
}

// parseNumericString 将字符串转换为 pgtype.Numeric（用于经纬度等数值字段）
func parseNumericString(s string) (pgtype.Numeric, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Numeric{Valid: false}, fmt.Errorf("empty string")
	}

	// 使用 big.Float 解析数值
	f, _, err := big.ParseFloat(s, 10, 128, big.ToNearestEven)
	if err != nil {
		return pgtype.Numeric{Valid: false}, fmt.Errorf("invalid numeric string: %w", err)
	}

	// 转换为 big.Int 和 exponent
	// 先找到小数点位置确定精度
	exp := int32(0)
	dotIndex := strings.Index(s, ".")
	if dotIndex >= 0 {
		exp = int32(-(len(s) - dotIndex - 1))
	}

	// 移除小数点，得到整数部分
	intStr := strings.Replace(s, ".", "", 1)
	intVal := new(big.Int)
	intVal, ok := intVal.SetString(intStr, 10)
	if !ok {
		// 回退：使用 float 的方式
		intVal, _ = f.Int(nil)
		exp = 0
	}

	return pgtype.Numeric{
		Int:   intVal,
		Exp:   exp,
		Valid: true,
	}, nil
}

// ==================== 文件上传 ====================

type uploadImageRequest struct {
	Category string `form:"category" binding:"required,oneof=business_license id_front id_back logo storefront environment"`
}

type uploadImageResponse struct {
	ImageURL string `json:"image_url"`
}

// uploadMerchantImage godoc
// @Summary 上传商户图片
// @Description 上传商户入驻所需图片（营业执照、身份证、Logo、门头照、环境照）
// @Tags 商户
// @Accept multipart/form-data
// @Produce json
// @Param category formData string true "图片类别" Enums(business_license, id_front, id_back, logo, storefront, environment)
// @Param image formData file true "图片文件"
// @Success 200 {object} uploadImageResponse "上传成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/images/upload [post]
// @Security BearerAuth
func (server *Server) uploadMerchantImage(ctx *gin.Context) {
	var req uploadImageRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取上传的文件
	file, header, err := ctx.Request.FormFile("image")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("failed to get file: %w", err)))
		return
	}
	defer file.Close()

	// 商户入驻证照（营业执照/身份证）在审核通过前仅本人可见，不走内容安全；
	// 仅对会公开展示的图片（如 logo）执行内容安全检测。
	if req.Category == "logo" {
		if err := server.wechatClient.ImgSecCheck(ctx, file); err != nil {
			if errors.Is(err, wechat.ErrRiskyContent) {
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("图片内容安全检测未通过")))
				return
			}

			// 开发环境详尽报错
			errMsg := "微信图片安全检测服务异常"
			if server.config.Environment == "development" {
				errMsg = fmt.Sprintf("微信图片安全检测失败: %v", err)
			}
			ctx.JSON(http.StatusBadGateway, errorResponse(errors.New(errMsg)))

			internalError(ctx, fmt.Errorf("wechat img sec check (logo): %w", err))
			return
		}
		if _, err := file.Seek(0, 0); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	// 上传文件
	uploader := util.NewFileUploader("uploads")
	relativePath, err := uploader.UploadMerchantImage(authPayload.UserID, req.Category, file, header)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 返回文件URL（相对路径）
	ctx.JSON(http.StatusOK, uploadImageResponse{
		ImageURL: normalizeUploadURLForClient(relativePath),
	})
}

// ==================== 商户入驻申请 ====================

type createMerchantApplicationRequest struct {
	MerchantName            string  `json:"merchant_name" binding:"required,min=2,max=50"`
	BusinessLicenseNumber   string  `json:"business_license_number" binding:"required,min=8,max=30"` // 统一社会信用代码或注册号
	BusinessLicenseImageURL string  `json:"business_license_image_url" binding:"required,max=500"`
	LegalPersonName         string  `json:"legal_person_name" binding:"required,min=2,max=30"`
	LegalPersonIDNumber     string  `json:"legal_person_id_number" binding:"required,min=15,max=18"` // 身份证15或18位
	LegalPersonIDFrontURL   string  `json:"legal_person_id_front_url" binding:"required,max=500"`
	LegalPersonIDBackURL    string  `json:"legal_person_id_back_url" binding:"required,max=500"`
	ContactPhone            string  `json:"contact_phone" binding:"required,min=11,max=11"`
	BusinessAddress         string  `json:"business_address" binding:"required,min=5,max=200"`
	Longitude               *string `json:"longitude" binding:"required"` // 经度，前端地图选点
	Latitude                *string `json:"latitude" binding:"required"`  // 纬度，前端地图选点
	BusinessScope           string  `json:"business_scope" binding:"omitempty,max=200"`
	RegionID                int64   `json:"region_id" binding:"required,min=1"` // 区域ID，前端上报
}

type merchantApplicationResponse struct {
	ID                      int64      `json:"id"`
	UserID                  int64      `json:"user_id"`
	MerchantName            string     `json:"merchant_name"`
	BusinessLicenseNumber   string     `json:"business_license_number"`
	BusinessLicenseImageURL string     `json:"business_license_image_url"`
	LegalPersonName         string     `json:"legal_person_name"`
	LegalPersonIDNumber     string     `json:"legal_person_id_number"`
	LegalPersonIDFrontURL   string     `json:"legal_person_id_front_url"`
	LegalPersonIDBackURL    string     `json:"legal_person_id_back_url"`
	ContactPhone            string     `json:"contact_phone"`
	BusinessAddress         string     `json:"business_address"`
	BusinessScope           *string    `json:"business_scope,omitempty"`
	Status                  string     `json:"status"`
	RejectReason            *string    `json:"reject_reason,omitempty"`
	ReviewedBy              *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt              *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

func newMerchantApplicationResponse(app db.MerchantApplication) merchantApplicationResponse {
	resp := merchantApplicationResponse{
		ID:                      app.ID,
		UserID:                  app.UserID,
		MerchantName:            app.MerchantName,
		BusinessLicenseNumber:   app.BusinessLicenseNumber,
		BusinessLicenseImageURL: app.BusinessLicenseImageUrl,
		LegalPersonName:         app.LegalPersonName,
		LegalPersonIDNumber:     app.LegalPersonIDNumber,
		LegalPersonIDFrontURL:   app.LegalPersonIDFrontUrl,
		LegalPersonIDBackURL:    app.LegalPersonIDBackUrl,
		ContactPhone:            app.ContactPhone,
		BusinessAddress:         app.BusinessAddress,
		Status:                  app.Status,
		CreatedAt:               app.CreatedAt,
		UpdatedAt:               app.UpdatedAt,
	}

	if app.BusinessScope.Valid {
		resp.BusinessScope = &app.BusinessScope.String
	}
	if app.RejectReason.Valid {
		resp.RejectReason = &app.RejectReason.String
	}
	if app.ReviewedBy.Valid {
		resp.ReviewedBy = &app.ReviewedBy.Int64
	}
	if app.ReviewedAt.Valid {
		resp.ReviewedAt = &app.ReviewedAt.Time
	}

	return resp
}

// createMerchantApplication godoc
// @Summary 提交商户入驻申请
// @Description 提交商户入驻申请，包括营业执照、法人身份证等信息
// @Tags 商户
// @Accept json
// @Produce json
// @Param request body createMerchantApplicationRequest true "商户入驻申请信息"
// @Success 200 {object} merchantApplicationResponse "申请提交成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 409 {object} ErrorResponse "已存在待审核或已通过的申请"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/applications [post]
// @Security BearerAuth
func (server *Server) createMerchantApplication(ctx *gin.Context) {
	var req createMerchantApplicationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 检查用户是否已有待审核或已通过的申请
	existingApp, err := server.store.GetUserMerchantApplication(ctx, authPayload.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if existingApp.ID != 0 && (existingApp.Status == "pending" || existingApp.Status == "approved") {
		ctx.JSON(http.StatusConflict, errorResponse(fmt.Errorf("you already have a %s application", existingApp.Status)))
		return
	}

	// 可选功能：OCR识别营业执照和身份证
	// 当前版本需要用户手动填写信息，可通过 server.wechatClient.OCRBusinessLicense 集成
	// 示例: licenseOCR, err := server.wechatClient.OCRBusinessLicense(ctx, req.BusinessLicenseImageURL)

	// 解析经纬度
	longitude, err := parseNumericString(*req.Longitude)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid longitude: %w", err)))
		return
	}
	latitude, err := parseNumericString(*req.Latitude)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid latitude: %w", err)))
		return
	}

	// 验证经纬度范围
	lonFloat, _ := strconv.ParseFloat(*req.Longitude, 64)
	latFloat, _ := strconv.ParseFloat(*req.Latitude, 64)
	if err := validateCoordinates(lonFloat, latFloat); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 创建申请记录
	arg := db.CreateMerchantApplicationParams{
		UserID:                  authPayload.UserID,
		MerchantName:            req.MerchantName,
		BusinessLicenseNumber:   req.BusinessLicenseNumber,
		BusinessLicenseImageUrl: normalizeImageURLForStorage(req.BusinessLicenseImageURL),
		LegalPersonName:         req.LegalPersonName,
		LegalPersonIDNumber:     req.LegalPersonIDNumber,
		LegalPersonIDFrontUrl:   normalizeImageURLForStorage(req.LegalPersonIDFrontURL),
		LegalPersonIDBackUrl:    normalizeImageURLForStorage(req.LegalPersonIDBackURL),
		ContactPhone:            req.ContactPhone,
		BusinessAddress:         req.BusinessAddress,
		Longitude:               longitude,
		Latitude:                latitude,
		RegionID:                pgtype.Int8{Int64: req.RegionID, Valid: true},
	}

	if req.BusinessScope != "" {
		arg.BusinessScope = pgtype.Text{
			String: req.BusinessScope,
			Valid:  true,
		}
	}

	application, err := server.store.CreateMerchantApplication(ctx, arg)
	if err != nil {
		// 检查是否是唯一约束冲突
		if db.ErrorCode(err) == db.UniqueViolation {
			ctx.JSON(http.StatusConflict, errorResponse(fmt.Errorf("business license already registered")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantApplicationResponse(application))
}

// getUserMerchantApplication godoc
// @Summary 获取当前用户的商户入驻申请
// @Description 获取当前用户提交的商户入驻申请状态和详情
// @Tags 商户
// @Accept json
// @Produce json
// @Success 200 {object} merchantApplicationResponse "申请详情"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "未找到申请记录"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/applications/me [get]
// @Security BearerAuth
func (server *Server) getUserMerchantApplication(ctx *gin.Context) {
	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	application, err := server.store.GetUserMerchantApplication(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("no application found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantApplicationResponse(application))
}

// ==================== 商户审核（管理员）====================

type listMerchantApplicationsRequest struct {
	Status   string `form:"status" binding:"omitempty,oneof=pending approved rejected"`
	PageID   int32  `form:"page_id" binding:"required,min=1"`
	PageSize int32  `form:"page_size" binding:"required,min=5,max=50"`
}

// listMerchantApplications godoc
// @Summary 获取商户入驻申请列表（管理员）
// @Description 分页获取商户入驻申请列表，仅管理员可用
// @Tags 商户管理
// @Accept json
// @Produce json
// @Param status query string false "按状态筛选" Enums(pending, approved, rejected)
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {array} merchantApplicationResponse "申请列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无管理员权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/admin/merchants/applications [get]
// @Security BearerAuth
func (server *Server) listMerchantApplications(ctx *gin.Context) {
	var req listMerchantApplicationsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 检查管理员权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	_, err := server.store.GetUserRoleByType(ctx, db.GetUserRoleByTypeParams{
		UserID: authPayload.UserID,
		Role:   "admin",
	})
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("admin role required")))
		return
	}

	var applications []db.MerchantApplication

	if req.Status != "" {
		arg := db.ListMerchantApplicationsParams{
			Status: req.Status,
			Limit:  req.PageSize,
			Offset: (req.PageID - 1) * req.PageSize,
		}
		applications, err = server.store.ListMerchantApplications(ctx, arg)
	} else {
		arg := db.ListAllMerchantApplicationsParams{
			Limit:  req.PageSize,
			Offset: (req.PageID - 1) * req.PageSize,
		}
		applications, err = server.store.ListAllMerchantApplications(ctx, arg)
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换为响应格式
	responses := make([]merchantApplicationResponse, len(applications))
	for i, app := range applications {
		responses[i] = newMerchantApplicationResponse(app)
	}

	ctx.JSON(http.StatusOK, responses)
}

type reviewMerchantApplicationRequest struct {
	ApplicationID int64  `json:"application_id" binding:"required,min=1"`
	Approve       *bool  `json:"approve" binding:"required"`
	RejectReason  string `json:"reject_reason" binding:"omitempty,max=500"`
}

// reviewMerchantApplication godoc
// @Summary 审核商户入驻申请（管理员）
// @Description 通过或拒绝商户入驻申请，仅管理员可用
// @Tags 商户管理
// @Accept json
// @Produce json
// @Param request body reviewMerchantApplicationRequest true "审核决定"
// @Success 200 {object} merchantApplicationResponse "审核结果"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无管理员权限"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/admin/merchants/applications/review [post]
// @Security BearerAuth
func (server *Server) reviewMerchantApplication(ctx *gin.Context) {
	var req reviewMerchantApplicationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 检查管理员权限
	_, err := server.store.GetUserRoleByType(ctx, db.GetUserRoleByTypeParams{
		UserID: authPayload.UserID,
		Role:   "admin",
	})
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("admin role required")))
		return
	}

	// 获取申请详情
	application, err := server.store.GetMerchantApplication(ctx, req.ApplicationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("application not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查申请状态
	if application.Status != "pending" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("application already %s", application.Status)))
		return
	}

	// 更新申请状态
	now := time.Now()
	status := "rejected"
	if *req.Approve {
		status = "approved"
	}

	var rejectReason pgtype.Text
	if !*req.Approve && req.RejectReason != "" {
		rejectReason = pgtype.Text{
			String: req.RejectReason,
			Valid:  true,
		}
	}

	updatedApp, err := server.store.UpdateMerchantApplicationStatus(ctx, db.UpdateMerchantApplicationStatusParams{
		ID:           req.ApplicationID,
		Status:       status,
		RejectReason: rejectReason,
		ReviewedBy: pgtype.Int8{
			Int64: authPayload.UserID,
			Valid: true,
		},
		ReviewedAt: pgtype.Timestamptz{
			Time:  now,
			Valid: true,
		},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 如果审核通过，创建商户记录
	if *req.Approve {
		// 构造application_data JSON
		appData, err := json.Marshal(map[string]interface{}{
			"business_license_number":    application.BusinessLicenseNumber,
			"legal_person_name":          application.LegalPersonName,
			"legal_person_id_number":     application.LegalPersonIDNumber,
			"business_license_image_url": application.BusinessLicenseImageUrl,
			"legal_person_id_front_url":  application.LegalPersonIDFrontUrl,
			"legal_person_id_back_url":   application.LegalPersonIDBackUrl,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 创建商户记录，状态为 pending_bindbank（待开户）
		// 商户需要完成微信支付开户后才能正常营业
		_, err = server.store.CreateMerchant(ctx, db.CreateMerchantParams{
			OwnerUserID:     application.UserID,
			Name:            application.MerchantName,
			Description:     pgtype.Text{},
			LogoUrl:         pgtype.Text{},
			Phone:           application.ContactPhone,
			Address:         application.BusinessAddress,
			Latitude:        application.Latitude,  // 从申请记录获取
			Longitude:       application.Longitude, // 从申请记录获取
			Status:          "pending_bindbank",    // 待开户
			ApplicationData: appData,
			RegionID:        application.RegionID.Int64, // 从申请记录获取区域ID
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	ctx.JSON(http.StatusOK, newMerchantApplicationResponse(updatedApp))
}

// ==================== 商户管理 ====================

type merchantResponse struct {
	ID          int64     `json:"id"`
	OwnerUserID int64     `json:"owner_user_id"`
	RegionID    int64     `json:"region_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	LogoURL     *string   `json:"logo_url,omitempty"`
	Phone       string    `json:"phone"`
	Address     string    `json:"address"`
	Latitude    *string   `json:"latitude,omitempty"`
	Longitude   *string   `json:"longitude,omitempty"`
	Status      string    `json:"status"`
	IsOpen      bool      `json:"is_open"`
	Version     int32     `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func newMerchantResponse(merchant db.Merchant) merchantResponse {
	resp := merchantResponse{
		ID:          merchant.ID,
		OwnerUserID: merchant.OwnerUserID,
		RegionID:    merchant.RegionID,
		Name:        merchant.Name,
		Phone:       merchant.Phone,
		Address:     merchant.Address,
		Status:      merchant.Status,
		IsOpen:      merchant.IsOpen,
		Version:     merchant.Version,
		CreatedAt:   merchant.CreatedAt,
		UpdatedAt:   merchant.UpdatedAt,
	}

	if merchant.Description.Valid {
		resp.Description = &merchant.Description.String
	}
	if merchant.LogoUrl.Valid {
		logo := normalizeUploadURLForClient(merchant.LogoUrl.String)
		resp.LogoURL = &logo
	}
	if merchant.Latitude.Valid {
		lat, _ := parseNumericToFloat(merchant.Latitude)
		latStr := fmt.Sprintf("%.6f", lat)
		resp.Latitude = &latStr
	}
	if merchant.Longitude.Valid {
		lng, _ := parseNumericToFloat(merchant.Longitude)
		lngStr := fmt.Sprintf("%.6f", lng)
		resp.Longitude = &lngStr
	}

	return resp
}

// getCurrentMerchant godoc
// @Summary 获取当前商户信息
// @Description 获取当前用户关联的商户详细信息
// @Tags 商户
// @Accept json
// @Produce json
// @Success 200 {object} merchantResponse "商户信息"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/me [get]
// @Security BearerAuth
func (server *Server) getCurrentMerchant(ctx *gin.Context) {
	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantResponse(merchant))
}

type updateMerchantRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=2,max=50"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	LogoURL     *string `json:"logo_url" binding:"omitempty,max=500"`
	Phone       *string `json:"phone" binding:"omitempty,min=11,max=11"`
	Address     *string `json:"address" binding:"omitempty,min=5,max=200"`
	Latitude    *string `json:"latitude"`
	Longitude   *string `json:"longitude"`
	Version     int32   `json:"version" binding:"required"` // ✅ P1-2: 乐观锁版本号
}

// updateCurrentMerchant godoc
// @Summary 更新商户信息
// @Description 更新商户基本信息（使用乐观锁防止并发冲突）
// @Tags 商户
// @Accept json
// @Produce json
// @Param request body updateMerchantRequest true "商户更新信息"
// @Success 200 {object} merchantResponse "更新后的商户信息"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 409 {object} ErrorResponse "版本冲突"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/me [patch]
// @Security BearerAuth
func (server *Server) updateCurrentMerchant(ctx *gin.Context) {
	var req updateMerchantRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户ID
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// ✅ P1-2: 检查版本号，防止并发更新冲突
	if merchant.Version != req.Version {
		ctx.JSON(http.StatusConflict, gin.H{
			"error":           "merchant has been modified by another request",
			"current_version": merchant.Version,
			"your_version":    req.Version,
		})
		return
	}

	// 构造更新参数
	arg := db.UpdateMerchantParams{
		ID:      merchant.ID,
		Version: req.Version,
	}

	if req.Name != nil {
		arg.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		arg.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.LogoURL != nil {
		arg.LogoUrl = pgtype.Text{String: normalizeImageURLForStorage(*req.LogoURL), Valid: true}
	}
	if req.Phone != nil {
		arg.Phone = pgtype.Text{String: *req.Phone, Valid: true}
	}
	if req.Address != nil {
		arg.Address = pgtype.Text{String: *req.Address, Valid: true}
	}
	if req.Latitude != nil {
		// 将 string 转换为 pgtype.Numeric
		if lat, err := parseNumericString(*req.Latitude); err == nil {
			latFloat, _ := strconv.ParseFloat(*req.Latitude, 64)
			if latFloat < minLatitude || latFloat > maxLatitude {
				ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("纬度必须在 %.1f 到 %.1f 之间", minLatitude, maxLatitude)))
				return
			}
			arg.Latitude = lat
		} else {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid latitude: %w", err)))
			return
		}
	}
	if req.Longitude != nil {
		// 将 string 转换为 pgtype.Numeric
		if lng, err := parseNumericString(*req.Longitude); err == nil {
			lngFloat, _ := strconv.ParseFloat(*req.Longitude, 64)
			if lngFloat < minLongitude || lngFloat > maxLongitude {
				ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("经度必须在 %.1f 到 %.1f 之间", minLongitude, maxLongitude)))
				return
			}
			arg.Longitude = lng
		} else {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid longitude: %w", err)))
			return
		}
	}

	updatedMerchant, err := server.store.UpdateMerchant(ctx, arg)
	if err != nil {
		// 检查是否是乐观锁冲突（没有返回结果 = version不匹配）
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusConflict, gin.H{
				"error": "merchant has been modified, please refresh and try again",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantResponse(updatedMerchant))
}

// ==================== 商户营业状态管理 ====================

type updateMerchantStatusRequest struct {
	IsOpen      *bool  `json:"is_open" binding:"required"`               // true=开店营业, false=打烊
	AutoCloseAt string `json:"auto_close_at" binding:"omitempty,max=50"` // 可选，自动打烊时间 (RFC3339格式)
}

type merchantStatusResponse struct {
	IsOpen      bool       `json:"is_open"`
	AutoCloseAt *time.Time `json:"auto_close_at,omitempty"`
	Message     string     `json:"message"`
}

// updateMerchantOpenStatus godoc
// @Summary 更新商户营业状态
// @Description 商户设置开店/打烊状态，可设置自动打烊时间
// @Tags 商户
// @Accept json
// @Produce json
// @Param request body updateMerchantStatusRequest true "状态更新"
// @Success 200 {object} merchantStatusResponse "更新后的状态"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "商户被暂停或无权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/me/status [patch]
// @Security BearerAuth
func (server *Server) updateMerchantOpenStatus(ctx *gin.Context) {
	var req updateMerchantStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查商户是否被暂停（食安熔断）
	merchantProfile, err := server.store.GetMerchantProfile(ctx, merchant.ID)
	if err == nil && merchantProfile.IsSuspended {
		ctx.JSON(http.StatusForbidden, gin.H{
			"error":          "merchant is suspended due to food safety issues",
			"suspend_reason": merchantProfile.SuspendReason.String,
			"suspend_until":  merchantProfile.SuspendUntil.Time,
		})
		return
	}

	// 解析自动打烊时间
	var autoCloseAt pgtype.Timestamptz
	if req.AutoCloseAt != "" && *req.IsOpen {
		t, err := time.Parse(time.RFC3339, req.AutoCloseAt)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid auto_close_at format, use RFC3339")))
			return
		}
		if t.Before(time.Now()) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("auto_close_at must be in the future")))
			return
		}
		autoCloseAt = pgtype.Timestamptz{Time: t, Valid: true}
	}

	// 更新营业状态
	_, err = server.store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{
		ID:          merchant.ID,
		IsOpen:      *req.IsOpen,
		AutoCloseAt: autoCloseAt,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构建响应消息
	message := "店铺已打烊"
	if *req.IsOpen {
		message = "店铺已开始营业"
		if autoCloseAt.Valid {
			message = fmt.Sprintf("店铺已开始营业，将于 %s 自动打烊", autoCloseAt.Time.Format("15:04"))
		}
	}

	resp := merchantStatusResponse{
		IsOpen:  *req.IsOpen,
		Message: message,
	}
	if autoCloseAt.Valid {
		resp.AutoCloseAt = &autoCloseAt.Time
	}

	ctx.JSON(http.StatusOK, resp)
}

// getMerchantOpenStatus godoc
// @Summary 获取商户营业状态
// @Description 获取当前商户的开店/打烊状态
// @Tags 商户
// @Produce json
// @Success 200 {object} merchantStatusResponse "营业状态"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/me/status [get]
// @Security BearerAuth
func (server *Server) getMerchantOpenStatus(ctx *gin.Context) {
	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取营业状态
	status, err := server.store.GetMerchantIsOpen(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	message := "店铺已打烊"
	if status.IsOpen {
		message = "店铺营业中"
		if status.AutoCloseAt.Valid {
			message = fmt.Sprintf("店铺营业中，将于 %s 自动打烊", status.AutoCloseAt.Time.Format("15:04"))
		}
	}

	resp := merchantStatusResponse{
		IsOpen:  status.IsOpen,
		Message: message,
	}
	if status.AutoCloseAt.Valid {
		resp.AutoCloseAt = &status.AutoCloseAt.Time
	}

	ctx.JSON(http.StatusOK, resp)
}

// ==================== 商户营业时间管理 ====================

type businessHourItem struct {
	DayOfWeek int32  `json:"day_of_week" binding:"min=0,max=6"`   // 0=周日, 1=周一, ..., 6=周六
	OpenTime  string `json:"open_time" binding:"required,len=5"`  // HH:MM 格式
	CloseTime string `json:"close_time" binding:"required,len=5"` // HH:MM 格式
	IsClosed  bool   `json:"is_closed"`                           // 是否休息
}

type setBusinessHoursRequest struct {
	Hours []businessHourItem `json:"hours" binding:"required,min=1,max=7,dive"` // 一周的营业时间
}

type businessHourResponse struct {
	ID        int64  `json:"id"`
	DayOfWeek int32  `json:"day_of_week"`
	DayName   string `json:"day_name"`
	OpenTime  string `json:"open_time"`
	CloseTime string `json:"close_time"`
	IsClosed  bool   `json:"is_closed"`
}

type businessHoursListResponse struct {
	Hours []businessHourResponse `json:"hours"`
}

// getDayName 获取星期名称
func getDayName(dayOfWeek int32) string {
	days := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	if dayOfWeek >= 0 && dayOfWeek < 7 {
		return days[dayOfWeek]
	}
	return "未知"
}

// parseTimeString 解析 HH:MM 格式的时间字符串
func parseTimeString(s string) (pgtype.Time, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return pgtype.Time{}, fmt.Errorf("invalid time format, expected HH:MM")
	}
	// 转换为微秒（从午夜开始）
	microseconds := int64(t.Hour()*3600+t.Minute()*60) * 1000000
	return pgtype.Time{
		Microseconds: microseconds,
		Valid:        true,
	}, nil
}

// formatTimeFromPgtype 将 pgtype.Time 格式化为 HH:MM
func formatTimeFromPgtype(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	// Microseconds 是从午夜开始的微秒数
	totalSeconds := t.Microseconds / 1000000
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}

// setMerchantBusinessHours godoc
// @Summary 设置商户营业时间
// @Description 设置商户每周的营业时间
// @Tags 商户
// @Accept json
// @Produce json
// @Param request body setBusinessHoursRequest true "营业时间列表"
// @Success 200 {object} businessHoursListResponse "设置后的营业时间"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/me/business-hours [put]
// @Security BearerAuth
func (server *Server) setMerchantBusinessHours(ctx *gin.Context) {
	var req setBusinessHoursRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证没有重复的星期
	daySet := make(map[int32]bool)
	for _, h := range req.Hours {
		if daySet[h.DayOfWeek] {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("duplicate day_of_week: %d", h.DayOfWeek)))
			return
		}
		daySet[h.DayOfWeek] = true
	}

	// 预先解析所有时间，避免事务中途失败
	hoursInput := make([]db.BusinessHourInput, 0, len(req.Hours))
	for _, h := range req.Hours {
		openTime, err := parseTimeString(h.OpenTime)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid open_time for day %d: %v", h.DayOfWeek, err)))
			return
		}
		closeTime, err := parseTimeString(h.CloseTime)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid close_time for day %d: %v", h.DayOfWeek, err)))
			return
		}
		hoursInput = append(hoursInput, db.BusinessHourInput{
			DayOfWeek: h.DayOfWeek,
			OpenTime:  openTime,
			CloseTime: closeTime,
			IsClosed:  h.IsClosed,
		})
	}

	// 使用事务设置营业时间（原子操作）
	result, err := server.store.SetBusinessHoursTx(ctx, db.SetBusinessHoursTxParams{
		MerchantID: merchant.ID,
		Hours:      hoursInput,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构建响应
	var results []businessHourResponse
	for _, bh := range result.Hours {
		results = append(results, businessHourResponse{
			ID:        bh.ID,
			DayOfWeek: bh.DayOfWeek,
			DayName:   getDayName(bh.DayOfWeek),
			OpenTime:  formatTimeFromPgtype(bh.OpenTime),
			CloseTime: formatTimeFromPgtype(bh.CloseTime),
			IsClosed:  bh.IsClosed,
		})
	}

	ctx.JSON(http.StatusOK, businessHoursListResponse{Hours: results})
}

// getMerchantBusinessHours godoc
// @Summary 获取商户营业时间
// @Description 获取当前商户每周的营业时间
// @Tags 商户
// @Produce json
// @Success 200 {object} businessHoursListResponse "营业时间列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/me/business-hours [get]
// @Security BearerAuth
func (server *Server) getMerchantBusinessHours(ctx *gin.Context) {
	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取营业时间列表
	hours, err := server.store.ListMerchantBusinessHours(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var results []businessHourResponse
	for _, h := range hours {
		results = append(results, businessHourResponse{
			ID:        h.ID,
			DayOfWeek: h.DayOfWeek,
			DayName:   getDayName(h.DayOfWeek),
			OpenTime:  formatTimeFromPgtype(h.OpenTime),
			CloseTime: formatTimeFromPgtype(h.CloseTime),
			IsClosed:  h.IsClosed,
		})
	}

	ctx.JSON(http.StatusOK, businessHoursListResponse{Hours: results})
}

// ==================== 餐厅优惠活动 API ====================
//
// 📌 前端开发注意：商户优惠活动的管理入口分布在不同模块
//
// 1. 配送费优惠（满X元减配送费）
//    - 管理接口在 delivery_fee.go
//    - POST   /v1/delivery-fee/merchants/:merchant_id/promotions  创建
//    - GET    /v1/delivery-fee/merchants/:merchant_id/promotions  列表
//    - DELETE /v1/delivery-fee/merchants/:merchant_id/promotions/:id  删除
//
// 2. 满减活动、优惠券等
//    - 管理接口在 discount.go / voucher.go（待实现或已有）
//
// 下方 getMerchantPromotions 是聚合展示接口，用于 C 端用户查看商户所有优惠

type promotionItem struct {
	Type        string `json:"type"`        // delivery_fee_return, discount, voucher
	Title       string `json:"title"`       // 优惠标题
	Description string `json:"description"` // 优惠描述
	MinAmount   int64  `json:"min_amount"`  // 起点金额（分）
	Value       int64  `json:"value"`       // 优惠金额或比例
	ValidUntil  string `json:"valid_until"` // 有效期
}

type merchantPromotionsResponse struct {
	MerchantID       int64           `json:"merchant_id"`
	DeliveryFeeRules []promotionItem `json:"delivery_fee_rules"` // 满返运费
	DiscountRules    []promotionItem `json:"discount_rules"`     // 满减活动
	Vouchers         []promotionItem `json:"vouchers"`           // 可领优惠券
}

// getMerchantPromotions godoc
// @Summary 获取商户优惠活动
// @Description 获取商户所有活跃的优惠活动（满返运费、满减、可领优惠券）
// @Tags 商户
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {object} merchantPromotionsResponse "优惠活动列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/{id}/promotions [get]
func (server *Server) getMerchantPromotions(ctx *gin.Context) {
	merchantID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid merchant id")))
		return
	}

	// 检查商户是否存在
	_, err = server.store.GetMerchant(ctx, merchantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := merchantPromotionsResponse{
		MerchantID:       merchantID,
		DeliveryFeeRules: []promotionItem{},
		DiscountRules:    []promotionItem{},
		Vouchers:         []promotionItem{},
	}

	// 获取满返运费规则
	deliveryPromos, err := server.store.ListActiveDeliveryPromotionsByMerchant(ctx, merchantID)
	if err == nil {
		for _, promo := range deliveryPromos {
			response.DeliveryFeeRules = append(response.DeliveryFeeRules, promotionItem{
				Type:        "delivery_fee_return",
				Title:       fmt.Sprintf("满%d返运费", promo.MinOrderAmount/100),
				Description: fmt.Sprintf("订单满%d元，返还运费", promo.MinOrderAmount/100),
				MinAmount:   promo.MinOrderAmount,
				Value:       0, // 全额返还
				ValidUntil:  promo.ValidUntil.Format("2006-01-02"),
			})
		}
	}

	// 获取满减规则
	discounts, err := server.store.ListActiveDiscountRules(ctx, merchantID)
	if err == nil {
		for _, d := range discounts {
			response.DiscountRules = append(response.DiscountRules, promotionItem{
				Type:        "discount",
				Title:       fmt.Sprintf("满%d减%d", d.MinOrderAmount/100, d.DiscountAmount/100),
				Description: fmt.Sprintf("订单满%d元，立减%d元", d.MinOrderAmount/100, d.DiscountAmount/100),
				MinAmount:   d.MinOrderAmount,
				Value:       d.DiscountAmount,
				ValidUntil:  d.ValidUntil.Format("2006-01-02"),
			})
		}
	}

	// 获取可领优惠券
	vouchers, err := server.store.ListActiveVouchers(ctx, db.ListActiveVouchersParams{
		MerchantID: merchantID,
		Limit:      20,
		Offset:     0,
	})
	if err == nil {
		for _, v := range vouchers {
			remaining := v.TotalQuantity - v.ClaimedQuantity
			if remaining > 0 {
				response.Vouchers = append(response.Vouchers, promotionItem{
					Type:        "voucher",
					Title:       v.Name,
					Description: fmt.Sprintf("满%d可用，减%d元", v.MinOrderAmount/100, v.Amount/100),
					MinAmount:   v.MinOrderAmount,
					Value:       v.Amount,
					ValidUntil:  v.ValidUntil.Format("2006-01-02"),
				})
			}
		}
	}

	ctx.JSON(http.StatusOK, response)
}
