package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

// 中国经纬度范围常量
const (
	minLongitude = 73.0  // 中国最西端
	maxLongitude = 135.0 // 中国最东端
	minLatitude  = 3.0   // 中国最南端
	maxLatitude  = 54.0  // 中国最北端
)

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

// resolveMerchantForUser returns the merchant currently associated with the user.
// The association may come from owner_user_id or active merchant_staff records.
func (server *Server) resolveMerchantForUser(ctx *gin.Context, userID int64) (db.Merchant, error) {
	return server.getMerchantFromContextOrStore(ctx, userID)
}

type merchantVersionConflictResponse struct {
	Error          string `json:"error"`
	CurrentVersion int32  `json:"current_version"`
	YourVersion    int32  `json:"your_version"`
}

type merchantSuspendedResponse struct {
	Error         string      `json:"error"`
	SuspendReason string      `json:"suspend_reason"`
	SuspendUntil  interface{} `json:"suspend_until"`
}

type merchantHasOrderedResponse struct {
	HasOrdered bool `json:"has_ordered"`
}

// uploadMerchantImage godoc
// @Summary [Deprecated] 上传商户图片
// @Description **已下线**。请改用媒体上传三步流程：POST /v1/media/upload-sessions → 直传 OSS → POST /v1/media/complete，然后将 media_asset_id 提交至商户接口。
// @Tags 商户
// @Produce json
// @Success 410 {object} ErrorResponse "接口已停用"
// @Router /v1/merchants/images/upload [post]
// @Security BearerAuth
func (server *Server) uploadMerchantImage(ctx *gin.Context) {
	ctx.JSON(http.StatusGone, errorResponse(errors.New(
		"此接口已停用。请改用媒体上传接口：POST /v1/media/upload-sessions",
	)))
}

// ==================== 商户管理 ====================

type merchantResponse struct {
	ID                int64     `json:"id"`
	OwnerUserID       int64     `json:"owner_user_id"`
	RegionID          int64     `json:"region_id"`
	Name              string    `json:"name"`
	Description       *string   `json:"description,omitempty"`
	LogoAssetID       *int64    `json:"-"`
	LogoURL           string    `json:"logo_url,omitempty"`
	Phone             string    `json:"phone"`
	Address           string    `json:"address"`
	Latitude          *string   `json:"latitude,omitempty"`
	Longitude         *string   `json:"longitude,omitempty"`
	Status            string    `json:"status"`
	IsOpen            bool      `json:"is_open"`
	Version           int32     `json:"version"`
	GroupID           *int64    `json:"group_id,omitempty"`
	BrandID           *int64    `json:"brand_id,omitempty"`
	StorefrontImages  *[]string `json:"storefront_images,omitempty"`
	EnvironmentImages *[]string `json:"environment_images,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (server *Server) newMerchantResponse(merchant db.Merchant) (merchantResponse, error) {
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
	resp.LogoAssetID = int64PtrFromPgInt8(merchant.LogoMediaAssetID)
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

	if merchant.GroupID.Valid {
		resp.GroupID = &merchant.GroupID.Int64
	}
	if merchant.BrandID.Valid {
		resp.BrandID = &merchant.BrandID.Int64
	}

	if merchant.StorefrontImages != nil {
		images, err := decodeStoredMerchantApplicationImageList(merchant.ID, "storefront_images", merchant.StorefrontImages)
		if err != nil {
			return merchantResponse{}, err
		}
		for i, img := range images {
			images[i] = server.resolvePublicUploadURLForClient(img)
		}
		resp.StorefrontImages = &images
	}
	if merchant.EnvironmentImages != nil {
		images, err := decodeStoredMerchantApplicationImageList(merchant.ID, "environment_images", merchant.EnvironmentImages)
		if err != nil {
			return merchantResponse{}, err
		}
		for i, img := range images {
			images[i] = server.resolvePublicUploadURLForClient(img)
		}
		resp.EnvironmentImages = &images
	}

	return resp, nil
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

	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if writeMerchantSelectionError(ctx, err) {
			return
		}
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp, err := server.newMerchantResponse(merchant)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	resp.LogoURL = server.publicImageURL(ctx, resp.LogoAssetID, media.VariantCard)
	ctx.JSON(http.StatusOK, resp)
}

// listMyMerchants godoc
// @Summary 获取当前用户的所有商户
// @Description 获取当前用户拥有的所有商户列表（用于多店铺切换）
// @Tags 商户
// @Accept json
// @Produce json
// @Success 200 {array} merchantResponse "商户列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/my [get]
// @Security BearerAuth
func (server *Server) listMyMerchants(ctx *gin.Context) {
	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	merchants, err := server.listAccessibleMerchants(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换为响应格式
	responses := make([]merchantResponse, len(merchants))
	for i, m := range merchants {
		resp, err := server.newMerchantResponse(m)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		responses[i] = resp
	}

	logoAssetIDs := make([]int64, 0, len(responses))
	for _, r := range responses {
		if r.LogoAssetID != nil {
			logoAssetIDs = append(logoAssetIDs, *r.LogoAssetID)
		}
	}
	if len(logoAssetIDs) > 0 {
		logoURLs := server.batchPublicImageURLs(ctx, logoAssetIDs, media.VariantCard)
		for i := range responses {
			if responses[i].LogoAssetID != nil {
				responses[i].LogoURL = logoURLs[*responses[i].LogoAssetID]
			}
		}
	}

	ctx.JSON(http.StatusOK, responses)
}

type updateMerchantRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=2,max=50"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	LogoAssetID *int64  `json:"logo_asset_id" binding:"omitempty,min=1"`
	ClearLogo   bool    `json:"clear_logo"`
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
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if writeMerchantSelectionError(ctx, err) {
			return
		}
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// ✅ P1-2: 检查版本号，防止并发更新冲突
	if merchant.Version != req.Version {
		ctx.JSON(http.StatusConflict, merchantVersionConflictResponse{
			Error:          "merchant has been modified by another request",
			CurrentVersion: merchant.Version,
			YourVersion:    req.Version,
		})
		return
	}

	if req.ClearLogo && req.LogoAssetID != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("clear_logo and logo_asset_id cannot be set together")))
		return
	}

	var updatedMerchant db.Merchant
	if req.ClearLogo {
		updatedMerchant, err = server.store.ClearMerchantLogo(ctx, db.ClearMerchantLogoParams{
			ID:      merchant.ID,
			Version: req.Version,
		})
	} else {
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
		if req.LogoAssetID != nil {
			arg.LogoMediaAssetID = pgtype.Int8{Int64: *req.LogoAssetID, Valid: true}
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
					ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("latitude must be between %.1f and %.1f", minLatitude, maxLatitude)))
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
					ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("longitude must be between %.1f and %.1f", minLongitude, maxLongitude)))
					return
				}
				arg.Longitude = lng
			} else {
				ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid longitude: %w", err)))
				return
			}
		}

		updatedMerchant, err = server.store.UpdateMerchant(ctx, arg)
	}
	if err != nil {
		// 检查是否是乐观锁冲突（没有返回结果 = version不匹配）
		if isNotFoundError(err) {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("merchant has been modified, please refresh and try again")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp, err := server.newMerchantResponse(updatedMerchant)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	resp.LogoURL = server.publicImageURL(ctx, resp.LogoAssetID, media.VariantCard)
	ctx.JSON(http.StatusOK, resp)
}

// ==================== 商户门头照/环境照更新（已通过审核后） ====================

type updateCurrentMerchantShopImagesRequest struct {
	StorefrontImages  []string `json:"storefront_images"`
	EnvironmentImages []string `json:"environment_images"`
}

type updateCurrentMerchantShopImagesResponse struct {
	StorefrontImages  []string `json:"storefront_images"`
	EnvironmentImages []string `json:"environment_images"`
}

// updateCurrentMerchantShopImages godoc
// @Summary 更新商户门头照和环境照
// @Description 允许已审核通过的商户更新店铺外观图片（门头照最多3张，环境照最多5张）
// @Tags 商户
// @Accept json
// @Produce json
// @Param request body updateCurrentMerchantShopImagesRequest true "图片URL数组"
// @Success 200 {object} updateCurrentMerchantShopImagesResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/merchants/me/shop-images [patch]
// @Security BearerAuth
func (server *Server) updateCurrentMerchantShopImages(ctx *gin.Context) {
	var req updateCurrentMerchantShopImagesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if len(req.StorefrontImages) > 3 {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrTooManyStorefrontPhotos))
		return
	}
	if len(req.EnvironmentImages) > 5 {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrTooManyAmbientPhotos))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant context not found")))
		return
	}

	arg := db.UpdateMerchantShopImagesParams{
		ID: merchant.ID,
	}
	if req.StorefrontImages != nil {
		for i, img := range req.StorefrontImages {
			req.StorefrontImages[i] = normalizeImageURLForStorage(img)
		}
		jsonData, err := json.Marshal(req.StorefrontImages)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		arg.StorefrontImages = jsonData
	}
	if req.EnvironmentImages != nil {
		for i, img := range req.EnvironmentImages {
			req.EnvironmentImages[i] = normalizeImageURLForStorage(img)
		}
		jsonData, err := json.Marshal(req.EnvironmentImages)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		arg.EnvironmentImages = jsonData
	}

	updatedMerchant, err := server.store.UpdateMerchantShopImages(ctx, arg)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := updateCurrentMerchantShopImagesResponse{}
	if len(updatedMerchant.StorefrontImages) > 0 {
		images, decodeErr := decodeStoredMerchantApplicationImageList(updatedMerchant.ID, "storefront_images", updatedMerchant.StorefrontImages)
		if decodeErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, decodeErr))
			return
		}
		for i, img := range images {
			images[i] = server.resolvePublicUploadURLForClient(img)
		}
		resp.StorefrontImages = images
	}
	if len(updatedMerchant.EnvironmentImages) > 0 {
		images, decodeErr := decodeStoredMerchantApplicationImageList(updatedMerchant.ID, "environment_images", updatedMerchant.EnvironmentImages)
		if decodeErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, decodeErr))
			return
		}
		for i, img := range images {
			images[i] = server.resolvePublicUploadURLForClient(img)
		}
		resp.EnvironmentImages = images
	}

	ctx.JSON(http.StatusOK, resp)
}

// ==================== 商户营业时间管理 ====================

type businessHourItem struct {
	DayOfWeek   int32   `json:"day_of_week" binding:"min=0,max=6"`   // 0=周日, 1=周一, ..., 6=周六
	OpenTime    string  `json:"open_time" binding:"required,len=5"`  // HH:MM 格式
	CloseTime   string  `json:"close_time" binding:"required,len=5"` // HH:MM 格式
	IsClosed    bool    `json:"is_closed"`                           // 是否休息
	SpecialDate *string `json:"special_date,omitempty"`              // YYYY-MM-DD 格式
}

type setBusinessHoursRequest struct {
	Hours                   []businessHourItem `json:"hours" binding:"required,min=0,max=50,dive"` // 营业时间
	AutoOpenByBusinessHours bool               `json:"auto_open_by_business_hours"`
}

type businessHourResponse struct {
	ID          int64  `json:"id"`
	DayOfWeek   int32  `json:"day_of_week"`
	DayName     string `json:"day_name"`
	OpenTime    string `json:"open_time"`
	CloseTime   string `json:"close_time"`
	IsClosed    bool   `json:"is_closed"`
	SpecialDate string `json:"special_date,omitempty"` // YYYY-MM-DD
}

type businessHoursListResponse struct {
	Hours                   []businessHourResponse `json:"hours"`
	AutoOpenByBusinessHours bool                   `json:"auto_open_by_business_hours"`
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
	if s == "" {
		return pgtype.Time{}, nil
	}
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

// parseDateString 解析 YYYY-MM-DD 格式的日期字符串
func parseDateString(s string) (pgtype.Date, error) {
	if s == "" {
		return pgtype.Date{Valid: false}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{Valid: false}, fmt.Errorf("invalid date format, expected YYYY-MM-DD")
	}
	return pgtype.Date{
		Time:  t,
		Valid: true,
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
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if writeMerchantSelectionError(ctx, err) {
			return
		}
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 允许同一天有多个时段（移除重复星期校验）
	// 但特殊日期如果设置了，DayOfWeek 应该与日期一致

	// 预先解析所有时间，避免事务中途失败
	hoursInput := make([]db.BusinessHourInput, 0, len(req.Hours))
	for _, h := range req.Hours {
		openTime, err := parseTimeString(h.OpenTime)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid open_time: %v", err)))
			return
		}
		closeTime, err := parseTimeString(h.CloseTime)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid close_time: %v", err)))
			return
		}
		if !h.IsClosed && openTime.Microseconds >= closeTime.Microseconds {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("open_time must be earlier than close_time")))
			return
		}
		specialDate, err := parseDateString(util.ValueOrDefault(h.SpecialDate, ""))
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid special_date: %v", err)))
			return
		}

		hoursInput = append(hoursInput, db.BusinessHourInput{
			DayOfWeek:   h.DayOfWeek,
			OpenTime:    openTime,
			CloseTime:   closeTime,
			IsClosed:    h.IsClosed,
			SpecialDate: specialDate,
		})
	}

	// 使用事务设置营业时间（原子操作）
	result, err := server.store.SetBusinessHoursTx(ctx, db.SetBusinessHoursTxParams{
		MerchantID:              merchant.ID,
		Hours:                   hoursInput,
		AutoOpenByBusinessHours: req.AutoOpenByBusinessHours,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构建响应
	var results []businessHourResponse
	for _, bh := range result.Hours {
		dateStr := ""
		if bh.SpecialDate.Valid {
			dateStr = bh.SpecialDate.Time.Format("2006-01-02")
		}
		results = append(results, businessHourResponse{
			ID:          bh.ID,
			DayOfWeek:   bh.DayOfWeek,
			DayName:     getDayName(bh.DayOfWeek),
			OpenTime:    formatTimeFromPgtype(bh.OpenTime),
			CloseTime:   formatTimeFromPgtype(bh.CloseTime),
			IsClosed:    bh.IsClosed,
			SpecialDate: dateStr,
		})
	}

	ctx.JSON(http.StatusOK, businessHoursListResponse{
		Hours:                   results,
		AutoOpenByBusinessHours: result.AutoOpenByBusinessHours,
	})
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
	merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
	if err != nil {
		if writeMerchantSelectionError(ctx, err) {
			return
		}
		if isNotFoundError(err) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取营业时间列表 (包括特殊日期)
	hours, err := server.store.ListMerchantBusinessHoursAll(ctx, merchant.ID)
	if err != nil {
		// 如果 ListMerchantBusinessHoursAll 不存在，退而求其次
		hours, err = server.store.ListMerchantBusinessHours(ctx, merchant.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	var results []businessHourResponse
	for _, h := range hours {
		dateStr := ""
		if h.SpecialDate.Valid {
			dateStr = h.SpecialDate.Time.Format("2006-01-02")
		}

		results = append(results, businessHourResponse{
			ID:          h.ID,
			DayOfWeek:   h.DayOfWeek,
			DayName:     getDayName(h.DayOfWeek),
			OpenTime:    formatTimeFromPgtype(h.OpenTime),
			CloseTime:   formatTimeFromPgtype(h.CloseTime),
			IsClosed:    h.IsClosed,
			SpecialDate: dateStr,
		})
	}

	ctx.JSON(http.StatusOK, businessHoursListResponse{
		Hours:                   results,
		AutoOpenByBusinessHours: merchant.AutoOpenByBusinessHours,
	})
}

// ==================== 餐厅优惠活动 API ====================
//
// 📌 前端开发注意：商户优惠活动的管理入口分布在不同模块
//
// 1. 代取费优惠（满X元减代取费）
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
	Type        string `json:"type"`         // delivery_fee_return, discount, voucher, recharge
	Title       string `json:"title"`        // 优惠标题
	Description string `json:"description"`  // 优惠描述
	MinAmount   int64  `json:"min_amount"`   // 起点金额（分）
	Value       int64  `json:"value"`        // 优惠金额或比例
	BonusAmount int64  `json:"bonus_amount"` // 赠送金额(充值活动用)
	ValidUntil  string `json:"valid_until"`  // 有效期
	RuleID      int64  `json:"rule_id"`      // 规则ID（充值活动用）
}

type merchantPromotionsResponse struct {
	MerchantID       int64           `json:"merchant_id"`
	DeliveryFeeRules []promotionItem `json:"delivery_fee_rules"` // 满返运费
	DiscountRules    []promotionItem `json:"discount_rules"`     // 满减活动
	Vouchers         []promotionItem `json:"vouchers"`           // 可领优惠券
	RechargeRules    []promotionItem `json:"recharge_rules"`     // 充值活动
}

// getMerchantPromotions godoc
// @Summary 获取商户优惠活动
// @Description 获取商户所有活跃的优惠活动（满返运费、满减、可领优惠券、充值活动）
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
		if isNotFoundError(err) {
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
		RechargeRules:    []promotionItem{},
	}

	// 获取满返运费规则
	deliveryPromos, err := server.store.ListActiveDeliveryPromotionsByMerchant(ctx, merchantID)
	if err == nil {
		for _, promo := range deliveryPromos {
			minAmountYuan := fenToYuanString(promo.MinOrderAmount, 0)
			response.DeliveryFeeRules = append(response.DeliveryFeeRules, promotionItem{
				Type:        "delivery_fee_return",
				Title:       fmt.Sprintf("满%s返运费", minAmountYuan),
				Description: fmt.Sprintf("订单满%s元，返还运费", minAmountYuan),
				MinAmount:   promo.MinOrderAmount,
				Value:       promo.DiscountAmount, // 实际返还金额
				ValidUntil:  promo.ValidUntil.Format("2006-01-02"),
			})
		}
	}

	// 获取满减规则
	discounts, err := server.store.ListActiveDiscountRules(ctx, merchantID)
	if err == nil {
		for _, d := range discounts {
			minAmountYuan := fenToYuanString(d.MinOrderAmount, 0)
			discountYuan := fenToYuanString(d.DiscountAmount, 0)
			response.DiscountRules = append(response.DiscountRules, promotionItem{
				Type:        "discount",
				Title:       fmt.Sprintf("满%s减%s", minAmountYuan, discountYuan),
				Description: fmt.Sprintf("订单满%s元，立减%s元", minAmountYuan, discountYuan),
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
				minAmountYuan := fenToYuanString(v.MinOrderAmount, 0)
				voucherAmountYuan := fenToYuanString(v.Amount, 0)
				response.Vouchers = append(response.Vouchers, promotionItem{
					Type:        "voucher",
					Title:       v.Name,
					Description: fmt.Sprintf("满%s可用，减%s元", minAmountYuan, voucherAmountYuan),
					MinAmount:   v.MinOrderAmount,
					Value:       v.Amount,
					ValidUntil:  v.ValidUntil.Format("2006-01-02"),
					RuleID:      v.ID, // 用于领券API
				})
			}
		}
	}

	// 获取充值活动
	rechargeRules, err := server.store.ListActiveRechargeRules(ctx, merchantID)
	if err == nil {
		for _, r := range rechargeRules {
			total := r.RechargeAmount + r.BonusAmount
			rechargeYuan := fenToYuanString(r.RechargeAmount, 0)
			bonusYuan := fenToYuanString(r.BonusAmount, 0)
			totalYuan := fenToYuanString(total, 0)
			response.RechargeRules = append(response.RechargeRules, promotionItem{
				Type:        "recharge",
				Title:       fmt.Sprintf("充%s送%s", rechargeYuan, bonusYuan),
				Description: fmt.Sprintf("充值%s元，赠送%s元，到账%s元", rechargeYuan, bonusYuan, totalYuan),
				MinAmount:   r.RechargeAmount,
				Value:       r.BonusAmount,
				BonusAmount: r.BonusAmount,
				ValidUntil:  r.ValidUntil.Format("2006-01-02"),
				RuleID:      r.ID,
			})
		}
	}

	ctx.JSON(http.StatusOK, response)
}

// ==================== 消费者端商户详情 ====================

// publicMerchantDetailRequest 公开商户详情请求
type publicMerchantDetailRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// publicMerchantDetailResponse 公开商户详情响应（消费者端）
type publicMerchantDetailResponse struct {
	ID                      int64                     `json:"id"`
	Name                    string                    `json:"name"`
	Description             *string                   `json:"description,omitempty"`
	LogoAssetID             *int64                    `json:"-"`
	LogoURL                 string                    `json:"logo_url,omitempty"`
	CoverImage              *string                   `json:"cover_image,omitempty"` // 门头照/招牌图
	Phone                   string                    `json:"phone"`
	Address                 string                    `json:"address"`
	Latitude                float64                   `json:"latitude"`
	Longitude               float64                   `json:"longitude"`
	RegionID                int64                     `json:"region_id"`
	IsOpen                  bool                      `json:"is_open"`
	IsOrderingSuspended     bool                      `json:"is_ordering_suspended"`
	Tags                    []string                  `json:"tags"`                                 // 商户标签（如：快餐、川菜）
	SystemLabels            []string                  `json:"system_labels,omitempty"`              // 商户系统标签（如：无明厨亮灶）
	MonthlySales            int32                     `json:"monthly_sales"`                        // 近30天订单量
	AvgPrepMinutes          int32                     `json:"avg_prep_minutes"`                     // 平均出餐时间（分钟）
	BusinessLicenseImageURL *string                   `json:"business_license_image_url,omitempty"` // 营业执照
	FoodPermitURL           *string                   `json:"food_permit_url,omitempty"`            // 食品经营许可证
	BusinessHours           []businessHourItem        `json:"business_hours,omitempty"`             // 营业时间
	DiscountRules           []publicDiscountRule      `json:"discount_rules,omitempty"`             // 满减规则
	Vouchers                []publicVoucher           `json:"vouchers,omitempty"`                   // 代金券
	DeliveryPromotions      []publicDeliveryPromotion `json:"delivery_promotions,omitempty"`        // 代取费优惠
}

type publicDiscountRule struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	MinOrderAmount int64  `json:"min_order_amount"`
	DiscountAmount int64  `json:"discount_amount"`
}

type publicVoucher struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Amount         int64  `json:"amount"`
	MinOrderAmount int64  `json:"min_order_amount"`
}

type publicDeliveryPromotion struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	MinOrderAmount int64  `json:"min_order_amount"`
	DiscountAmount int64  `json:"discount_amount"`
}

// NOTE: businessHourItem is already defined at line ~961, reusing it here

// 店铺二维码直达页需要在进件审核态保持可访问，但“可在顾客搜索/列表中被发现”仍由搜索 SQL 的 active 过滤控制。
// 这里仅定义直达店铺详情页及其附属接口的可访问状态，不应被复用为 discoverability 规则。

func isMerchantStatusPublicStorefrontAccessible(status string) bool {
	switch status {
	case "approved", "pending_bindbank", "bindbank_submitted", "active":
		return true
	default:
		return false
	}
}

func isMerchantPublicStorefrontOrderable(merchant db.Merchant) bool {
	return merchant.Status == "active" && merchant.IsOpen
}

func (server *Server) loadPublicStorefrontMerchant(ctx *gin.Context, merchantID int64) (db.Merchant, bool) {
	merchant, err := server.store.GetMerchant(ctx, merchantID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return db.Merchant{}, false
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant: %w", err)))
		return db.Merchant{}, false
	}

	if !isMerchantStatusPublicStorefrontAccessible(merchant.Status) {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant is not available")))
		return db.Merchant{}, false
	}

	return merchant, true
}

// getPublicMerchantDetail godoc
// @Summary 获取商户详情（消费者端）
// @Description 需登录访问（消费者端），获取商户详细信息
// @Tags 公开接口
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {object} publicMerchantDetailResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "商户不存在或未上线"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/public/merchants/{id} [get]
func (server *Server) getPublicMerchantDetail(ctx *gin.Context) {
	var req publicMerchantDetailRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// lite=true：Feed 卡片专用，跳过资质图片 presign 和 GetMerchantApplicationDraft，
	// 只返回促销/出餐时间等首页需要展示的字段，减少不必要的 DB 查询和签名运算。
	liteMode := ctx.Query("lite") == "true"

	merchant, ok := server.loadPublicStorefrontMerchant(ctx, req.ID)
	if !ok {
		return
	}

	// 构建响应
	resp := publicMerchantDetailResponse{
		ID:       merchant.ID,
		Name:     merchant.Name,
		Phone:    merchant.Phone,
		Address:  merchant.Address,
		RegionID: merchant.RegionID,
		IsOpen:   isMerchantPublicStorefrontOrderable(merchant),
		Tags:     []string{},
	}

	// 处理可空字段
	if merchant.Description.Valid {
		resp.Description = &merchant.Description.String
	}
	resp.LogoAssetID = int64PtrFromPgInt8(merchant.LogoMediaAssetID)
	resp.LogoURL = server.publicImageURL(ctx, resp.LogoAssetID, media.VariantCard)
	if merchant.Latitude.Valid {
		lat, _ := parseNumericToFloat(merchant.Latitude)
		resp.Latitude = lat
	}
	if merchant.Longitude.Valid {
		lng, _ := parseNumericToFloat(merchant.Longitude)
		resp.Longitude = lng
	}

	// 解析 application_data 获取证照信息（lite 模式跳过）
	if !liteMode && merchant.ApplicationData != nil {
		var appData map[string]interface{}
		if err := json.Unmarshal(merchant.ApplicationData, &appData); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("decode merchant %d application_data: %w", merchant.ID, err)))
			return
		}
		if v, ok := appData["business_license_media_asset_id"].(float64); ok && v > 0 {
			id := int64(v)
			url := server.publicImageURL(ctx, &id, media.VariantOriginal)
			resp.BusinessLicenseImageURL = &url
		}
		if v, ok := appData["food_permit_media_asset_id"].(float64); ok && v > 0 {
			id := int64(v)
			url := server.publicImageURL(ctx, &id, media.VariantOriginal)
			resp.FoodPermitURL = &url
		}
	}

	if len(merchant.StorefrontImages) > 0 {
		storefrontImages, decodeErr := decodeStoredMerchantApplicationImageList(merchant.ID, "storefront_images", merchant.StorefrontImages)
		if decodeErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, decodeErr))
			return
		}
		if len(storefrontImages) > 0 {
			url := server.resolvePublicUploadURLForClient(storefrontImages[0])
			if url != "" {
				resp.CoverImage = &url
			}
		}
	}

	// 兼容迁移前尚未回填的商户：live 图为 NULL 时才回退最新申请图，空数组表示商户已明确清空。
	if resp.CoverImage == nil && merchant.StorefrontImages == nil && !liteMode {
		application, appErr := server.store.GetLatestApprovedMerchantApplicationByUser(ctx, merchant.OwnerUserID)
		if appErr == nil && len(application.StorefrontImages) > 0 {
			storefrontImages, decodeErr := decodeStoredMerchantApplicationImageList(application.ID, "storefront_images", application.StorefrontImages)
			if decodeErr != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, decodeErr))
				return
			}
			if len(storefrontImages) > 0 {
				url := server.resolvePublicUploadURLForClient(storefrontImages[0])
				if url != "" {
					resp.CoverImage = &url
				}
			}
		}
	}

	// 获取商户标签
	tags, err := server.store.ListMerchantTags(ctx, merchant.ID)
	if err == nil && len(tags) > 0 {
		resp.Tags = make([]string, len(tags))
		for i, tag := range tags {
			resp.Tags[i] = tag.Name
		}
	}
	systemLabels, err := server.store.ListMerchantSystemLabels(ctx, merchant.ID)
	if err == nil && len(systemLabels) > 0 {
		resp.SystemLabels = make([]string, len(systemLabels))
		for i, label := range systemLabels {
			resp.SystemLabels[i] = label.Name
		}
	}

	// 获取商户 profile（订单量）
	profile, err := server.store.GetMerchantProfile(ctx, merchant.ID)
	if err == nil {
		resp.MonthlySales = profile.CompletedOrders // 使用已完成订单数
		resp.IsOrderingSuspended = profile.IsTakeoutSuspended
	} else {
		resp.MonthlySales = 0
	}

	// 获取平均出餐时间。
	// 查询成功但结果为 0 时同样回退默认值，避免前端拿到 0 后不展示。
	avgPrepMinutes, err := server.store.GetMerchantAvgPrepMinutes(ctx, merchant.ID)
	if err != nil || avgPrepMinutes <= 0 {
		resp.AvgPrepMinutes = DefaultAvgPrepareTimeMinutes
	} else {
		resp.AvgPrepMinutes = avgPrepMinutes
	}

	// 获取营业时间（Feed 卡片用不到，lite 模式跳过）
	if !liteMode {
		hours, err := server.store.ListMerchantBusinessHours(ctx, merchant.ID)
		if err == nil && len(hours) > 0 {
			resp.BusinessHours = make([]businessHourItem, len(hours))
			for i, h := range hours {
				resp.BusinessHours[i] = businessHourItem{
					DayOfWeek: int32(h.DayOfWeek),
					OpenTime:  formatTimeForResponse(h.OpenTime),
					CloseTime: formatTimeForResponse(h.CloseTime),
					IsClosed:  h.IsClosed,
				}
			}
		}
	}

	// 获取满减规则
	discountRules, err := server.store.ListMerchantActiveDiscountRules(ctx, merchant.ID)
	if err == nil {
		for _, r := range discountRules {
			resp.DiscountRules = append(resp.DiscountRules, publicDiscountRule{
				ID:             r.ID,
				Name:           r.Name,
				MinOrderAmount: r.MinOrderAmount,
				DiscountAmount: r.DiscountAmount,
			})
		}
	}

	// 获取代金券
	vouchers, err := server.store.ListMerchantActiveVouchers(ctx, merchant.ID)
	if err == nil {
		for _, v := range vouchers {
			resp.Vouchers = append(resp.Vouchers, publicVoucher{
				ID:             v.ID,
				Name:           v.Name,
				Amount:         v.Amount,
				MinOrderAmount: v.MinOrderAmount,
			})
		}
	}

	// 获取代取费优惠
	deliveryPromotions, err := server.store.ListMerchantActiveDeliveryPromotions(ctx, merchant.ID)
	if err == nil {
		for _, p := range deliveryPromotions {
			resp.DeliveryPromotions = append(resp.DeliveryPromotions, publicDeliveryPromotion{
				ID:             p.ID,
				Name:           p.Name,
				MinOrderAmount: p.MinOrderAmount,
				DiscountAmount: p.DiscountAmount,
			})
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// formatTimeForResponse 格式化时间为 HH:MM 字符串
func formatTimeForResponse(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	// pgtype.Time 存储的是微秒数
	totalSeconds := t.Microseconds / 1000000
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}

// ==================== 消费者端菜品列表 ====================

type publicDishCategoryItem struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int16  `json:"sort_order"`
}

type publicDishItem struct {
	ID                  int64                `json:"id"`
	Name                string               `json:"name"`
	Description         string               `json:"description,omitempty"`
	Price               int64                `json:"price"`
	MemberPrice         *int64               `json:"member_price,omitempty"`
	ImageAssetID        *int64               `json:"-"`
	ImageURL            string               `json:"image_url,omitempty"`
	CategoryID          int64                `json:"category_id"`
	CategoryName        string               `json:"category_name"`
	MonthlySales        int32                `json:"monthly_sales"`
	PrepareTime         int16                `json:"prepare_time"`
	Tags                []string             `json:"tags"`
	CustomizationGroups []customizationGroup `json:"customization_groups,omitempty"`
}

type publicMerchantDishesResponse struct {
	Categories []publicDishCategoryItem `json:"categories"`
	Dishes     []publicDishItem         `json:"dishes"`
}

// getPublicMerchantDishes godoc
// @Summary 获取商户菜品列表（消费者端）
// @Description 需登录访问（消费者端），获取商户所有在线菜品及分类
// @Tags 公开接口
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {object} publicMerchantDishesResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/public/merchants/{id}/dishes [get]
func (server *Server) getPublicMerchantDishes(ctx *gin.Context) {
	var req publicMerchantDetailRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if _, ok := server.loadPublicStorefrontMerchant(ctx, req.ID); !ok {
		return
	}

	dishes, err := server.store.GetMerchantDishesWithCategory(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 提取分类
	categoryMap := make(map[int64]publicDishCategoryItem)
	var dishList []publicDishItem

	for _, d := range dishes {
		// 添加分类
		if _, exists := categoryMap[d.CategoryID]; !exists {
			categoryMap[d.CategoryID] = publicDishCategoryItem{
				ID:        d.CategoryID,
				Name:      d.CategoryName,
				SortOrder: d.CategorySortOrder,
			}
		}

		// 解析标签
		var tags []string
		if d.Tags != nil {
			if err := parseJSON(d.Tags, &tags); err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("decode public dish %d tags: %w", d.ID, err)))
				return
			}
		}
		if tags == nil {
			tags = []string{}
		}

		// 解析月销量
		var monthlySales int32
		if d.MonthlySales != nil {
			switch v := d.MonthlySales.(type) {
			case int32:
				monthlySales = v
			case int64:
				monthlySales = int32(v)
			case float64:
				monthlySales = int32(v)
			}
		}

		dish := publicDishItem{
			ID:                  d.ID,
			Name:                d.Name,
			Price:               d.Price,
			CategoryID:          d.CategoryID,
			CategoryName:        d.CategoryName,
			MonthlySales:        monthlySales,
			PrepareTime:         d.PrepareTime,
			Tags:                tags,
			CustomizationGroups: []customizationGroup{},
		}

		if d.CustomizationGroups != nil {
			if err := parseJSON(d.CustomizationGroups, &dish.CustomizationGroups); err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("decode public dish %d customization_groups: %w", d.ID, err)))
				return
			}
		}
		if dish.CustomizationGroups == nil {
			dish.CustomizationGroups = []customizationGroup{}
		}

		if d.Description.Valid {
			dish.Description = d.Description.String
		}
		if d.ImageMediaAssetID.Valid {
			v := d.ImageMediaAssetID.Int64
			dish.ImageAssetID = &v
		}
		if d.MemberPrice.Valid {
			dish.MemberPrice = &d.MemberPrice.Int64
		}

		dishList = append(dishList, dish)
	}

	// 批量解析菜品图片 URL
	dishAssetIDs := make([]int64, 0, len(dishList))
	for _, d := range dishList {
		if d.ImageAssetID != nil {
			dishAssetIDs = append(dishAssetIDs, *d.ImageAssetID)
		}
	}
	if len(dishAssetIDs) > 0 {
		imgURLs := server.batchPublicImageURLs(ctx, dishAssetIDs, media.VariantCard)
		for i := range dishList {
			if dishList[i].ImageAssetID != nil {
				dishList[i].ImageURL = imgURLs[*dishList[i].ImageAssetID]
			}
		}
	}

	// 构建分类列表
	var categories []publicDishCategoryItem
	for _, c := range categoryMap {
		categories = append(categories, c)
	}

	ctx.JSON(http.StatusOK, publicMerchantDishesResponse{
		Categories: categories,
		Dishes:     dishList,
	})
}

// ==================== 消费者端套餐列表 ====================

type comboDishItem struct {
	DishID   int64  `json:"dish_id"`
	DishName string `json:"dish_name"`
	Quantity int16  `json:"quantity"`
}

type publicComboItem struct {
	ID            int64           `json:"id"`
	Name          string          `json:"name"`
	Description   string          `json:"description,omitempty"`
	ImageAssetID  *int64          `json:"-"`
	ImageURL      string          `json:"image_url,omitempty"`
	ComboPrice    int64           `json:"combo_price"`
	OriginalPrice int64           `json:"original_price"`
	Dishes        []comboDishItem `json:"dishes"`
	Tags          []string        `json:"tags"`
	DishImageURLs []string        `json:"dish_image_urls,omitempty"` // 子菜品图片 CDN URL 列表
}

type publicMerchantCombosResponse struct {
	Combos []publicComboItem `json:"combos"`
}

// getPublicMerchantCombos godoc
// @Summary 获取商户套餐列表（消费者端）
// @Description 需登录访问（消费者端），获取商户所有在线套餐
// @Tags 公开接口
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {object} publicMerchantCombosResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/public/merchants/{id}/combos [get]
func (server *Server) getPublicMerchantCombos(ctx *gin.Context) {
	var req publicMerchantDetailRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if _, ok := server.loadPublicStorefrontMerchant(ctx, req.ID); !ok {
		return
	}

	combos, err := server.store.GetMerchantOnlineCombos(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var comboList []publicComboItem
	for _, c := range combos {
		combo := publicComboItem{
			ID:            c.ID,
			Name:          c.Name,
			ComboPrice:    c.ComboPrice,
			OriginalPrice: c.OriginalPrice,
			Dishes:        []comboDishItem{},
			Tags:          []string{},
		}

		if c.Description.Valid {
			combo.Description = c.Description.String
		}
		if c.ImageMediaAssetID.Valid {
			v := c.ImageMediaAssetID.Int64
			combo.ImageAssetID = &v
		}

		// 解析菜品
		if c.Dishes != nil {
			var dishes []comboDishItem
			if err := json.Unmarshal(c.Dishes, &dishes); err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("decode public combo %d dishes: %w", c.ID, err)))
				return
			}
			combo.Dishes = dishes
		}

		// 解析标签
		if c.Tags != nil {
			var tags []string
			if tagBytes, ok := c.Tags.([]byte); ok {
				if err := json.Unmarshal(tagBytes, &tags); err != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("decode public combo %d tags: %w", c.ID, err)))
					return
				}
				combo.Tags = tags
			} else {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("decode public combo %d tags: unexpected type %T", c.ID, c.Tags)))
				return
			}
		}

		comboList = append(comboList, combo)
	}

	// 批量填充图片
	server.enrichPublicComboListImages(ctx, comboList)

	ctx.JSON(http.StatusOK, publicMerchantCombosResponse{
		Combos: comboList,
	})
}

// ==================== 消费者端包间列表 ====================

type publicRoomItem struct {
	ID                  int64    `json:"id"`
	Name                string   `json:"name"`
	Capacity            int16    `json:"capacity"`
	MinimumSpend        *int64   `json:"minimum_spend,omitempty"`
	Description         string   `json:"description,omitempty"`
	PrimaryImageAssetID int64    `json:"-"`
	ImageURL            string   `json:"image_url,omitempty"`
	MonthlySales        int64    `json:"monthly_sales"`
	Status              string   `json:"status"`
	Tags                []string `json:"tags"`
}

type publicMerchantRoomsResponse struct {
	Rooms []publicRoomItem `json:"rooms"`
}

// getPublicMerchantRooms godoc
// @Summary 获取商户包间列表（消费者端）
// @Description 需登录访问（消费者端），获取商户所有包间信息，帮助消费者决策
// @Tags 公开接口
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {object} publicMerchantRoomsResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/public/merchants/{id}/rooms [get]
func (server *Server) getPublicMerchantRooms(ctx *gin.Context) {
	var req publicMerchantDetailRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if _, ok := server.loadPublicStorefrontMerchant(ctx, req.ID); !ok {
		return
	}

	rooms, err := server.store.ListMerchantRoomsForCustomer(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var roomList []publicRoomItem
	for _, r := range rooms {
		room := publicRoomItem{
			ID:           r.ID,
			Name:         r.TableNo,
			Capacity:     r.Capacity,
			MonthlySales: r.MonthlyReservations,
			Status:       r.Status,
			Tags:         []string{},
		}

		if r.Description.Valid {
			room.Description = r.Description.String
		}
		if r.MinimumSpend.Valid {
			room.MinimumSpend = &r.MinimumSpend.Int64
		}
		if r.PrimaryImageAssetID != nil {
			if v, ok := r.PrimaryImageAssetID.(int64); ok {
				room.PrimaryImageAssetID = v
			}
		}

		// 获取包间标签
		tags, err := server.store.ListTableTags(ctx, r.ID)
		if err == nil {
			for _, t := range tags {
				room.Tags = append(room.Tags, t.TagName)
			}
		}

		roomList = append(roomList, room)
	}

	server.enrichPublicRoomImageURLs(ctx, roomList)
	ctx.JSON(http.StatusOK, publicMerchantRoomsResponse{
		Rooms: roomList,
	})
}

// getPublicMerchantHasOrdered godoc
// @Summary 查询当前用户是否曾在该商户成功下单
// @Tags Merchant
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {object} map[string]bool "has_ordered"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/public/merchants/{id}/has-ordered [get]
func (server *Server) getPublicMerchantHasOrdered(ctx *gin.Context) {
	var req publicMerchantDetailRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	hasOrdered, err := server.store.HasUserOrderedFromMerchant(ctx, db.HasUserOrderedFromMerchantParams{
		UserID:     authPayload.UserID,
		MerchantID: req.ID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, merchantHasOrderedResponse{HasOrdered: hasOrdered})
}

// enrichPublicComboListImages godoc
func (server *Server) enrichPublicComboListImages(ctx context.Context, combos []publicComboItem) {
	if len(combos) == 0 {
		return
	}

	comboIDs := make([]int64, 0, len(combos))
	for _, c := range combos {
		comboIDs = append(comboIDs, c.ID)
	}

	// 批量查询成员图片
	memberImages, err := server.store.GetComboMemberImagesByCombos(ctx, comboIDs)
	if err != nil {
		log.Error().Err(err).Msg("enrichPublicComboListImages: failed to get images")
		return
	}

	// 按 combo_id 组织成员图片 asset IDs
	imgMap := make(map[int64][]int64)
	for _, row := range memberImages {
		if row.ImageMediaAssetID.Valid {
			imgMap[row.ComboID] = append(imgMap[row.ComboID], row.ImageMediaAssetID.Int64)
		}
	}

	// 收集所有需要解析的 asset ID（套餐自身图 + 成员图）
	allAssetIDs := make([]int64, 0, len(combos)+len(memberImages))
	for _, c := range combos {
		if c.ImageAssetID != nil {
			allAssetIDs = append(allAssetIDs, *c.ImageAssetID)
		}
	}
	for _, ids := range imgMap {
		allAssetIDs = append(allAssetIDs, ids...)
	}
	imgURLs := server.batchPublicImageURLs(ctx, allAssetIDs, media.VariantCard)

	// 回填
	for i := range combos {
		if combos[i].ImageAssetID != nil {
			combos[i].ImageURL = imgURLs[*combos[i].ImageAssetID]
		}
		if imgs, ok := imgMap[combos[i].ID]; ok {
			urls := make([]string, 0, len(imgs))
			for _, id := range imgs {
				if u, ok2 := imgURLs[id]; ok2 {
					urls = append(urls, u)
				}
			}
			combos[i].DishImageURLs = urls
		}
	}
}
