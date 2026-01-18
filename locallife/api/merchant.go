package api

import (
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
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newMerchantResponse(merchant))
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

	merchants, err := server.store.ListMerchantsByOwner(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换为响应格式
	responses := make([]merchantResponse, len(merchants))
	for i, m := range merchants {
		responses[i] = newMerchantResponse(m)
	}

	ctx.JSON(http.StatusOK, responses)
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
		if isNotFoundError(err) {
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
		if isNotFoundError(err) {
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
		if isNotFoundError(err) {
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
		if isNotFoundError(err) {
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
		if isNotFoundError(err) {
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
		if isNotFoundError(err) {
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
	LogoURL                 *string                   `json:"logo_url,omitempty"`
	CoverImage              *string                   `json:"cover_image,omitempty"` // 门头照/招牌图
	Phone                   string                    `json:"phone"`
	Address                 string                    `json:"address"`
	Latitude                float64                   `json:"latitude"`
	Longitude               float64                   `json:"longitude"`
	RegionID                int64                     `json:"region_id"`
	IsOpen                  bool                      `json:"is_open"`
	Tags                    []string                  `json:"tags"`                                 // 商户标签（如：快餐、川菜）
	MonthlySales            int32                     `json:"monthly_sales"`                        // 近30天订单量
	AvgPrepMinutes          int32                     `json:"avg_prep_minutes"`                     // 平均出餐时间（分钟）
	BusinessLicenseImageURL *string                   `json:"business_license_image_url,omitempty"` // 营业执照
	FoodPermitURL           *string                   `json:"food_permit_url,omitempty"`            // 食品经营许可证
	BusinessHours           []businessHourItem        `json:"business_hours,omitempty"`             // 营业时间
	DiscountRules           []publicDiscountRule      `json:"discount_rules,omitempty"`             // 满减规则
	Vouchers                []publicVoucher           `json:"vouchers,omitempty"`                   // 代金券
	DeliveryPromotions      []publicDeliveryPromotion `json:"delivery_promotions,omitempty"`        // 配送费优惠
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

	// 获取商户基本信息
	merchant, err := server.store.GetMerchant(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant: %w", err)))
		return
	}

	// 只返回已批准的商户
	if merchant.Status != "approved" && merchant.Status != "active" {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant is not available")))
		return
	}

	// 构建响应
	resp := publicMerchantDetailResponse{
		ID:       merchant.ID,
		Name:     merchant.Name,
		Phone:    merchant.Phone,
		Address:  merchant.Address,
		RegionID: merchant.RegionID,
		IsOpen:   merchant.IsOpen,
		Tags:     []string{},
	}

	// 处理可空字段
	if merchant.Description.Valid {
		resp.Description = &merchant.Description.String
	}
	if merchant.LogoUrl.Valid {
		logo := normalizeUploadURLForClient(merchant.LogoUrl.String)
		resp.LogoURL = &logo
	}
	if merchant.Latitude.Valid {
		lat, _ := parseNumericToFloat(merchant.Latitude)
		resp.Latitude = lat
	}
	if merchant.Longitude.Valid {
		lng, _ := parseNumericToFloat(merchant.Longitude)
		resp.Longitude = lng
	}

	// 解析 application_data 获取证照信息和门头照
	if merchant.ApplicationData != nil {
		var appData map[string]interface{}
		if err := json.Unmarshal(merchant.ApplicationData, &appData); err == nil {
			if licenseURL, ok := appData["business_license_image_url"].(string); ok && licenseURL != "" {
				normalized := normalizeUploadURLForClient(licenseURL)
				resp.BusinessLicenseImageURL = &normalized
			}
			if permitURL, ok := appData["food_permit_url"].(string); ok && permitURL != "" {
				normalized := normalizeUploadURLForClient(permitURL)
				resp.FoodPermitURL = &normalized
			}
			// 门头照数组（取第一张作为封面图）
			if storefrontImages, ok := appData["storefront_images"].([]interface{}); ok && len(storefrontImages) > 0 {
				if firstImage, ok := storefrontImages[0].(string); ok && firstImage != "" {
					normalized := normalizeUploadURLForClient(firstImage)
					resp.CoverImage = &normalized
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

	// 获取商户 profile（订单量）
	profile, err := server.store.GetMerchantProfile(ctx, merchant.ID)
	if err == nil {
		resp.MonthlySales = profile.CompletedOrders // 使用已完成订单数
	} else {
		resp.MonthlySales = 0
	}

	// 获取平均出餐时间
	avgPrepMinutes, err := server.store.GetMerchantAvgPrepMinutes(ctx, merchant.ID)
	if err == nil {
		resp.AvgPrepMinutes = avgPrepMinutes
	} else {
		resp.AvgPrepMinutes = 15 // 默认 15 分钟
	}

	// 获取营业时间
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

	// 获取配送费优惠
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
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Price        int64    `json:"price"`
	MemberPrice  *int64   `json:"member_price,omitempty"`
	ImageURL     string   `json:"image_url,omitempty"`
	CategoryID   int64    `json:"category_id"`
	CategoryName string   `json:"category_name"`
	MonthlySales int32    `json:"monthly_sales"`
	PrepareTime  int16    `json:"prepare_time"`
	Tags         []string `json:"tags"`
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
			if tagBytes, ok := d.Tags.([]byte); ok {
				json.Unmarshal(tagBytes, &tags)
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
			ID:           d.ID,
			Name:         d.Name,
			Price:        d.Price,
			CategoryID:   d.CategoryID,
			CategoryName: d.CategoryName,
			MonthlySales: monthlySales,
			PrepareTime:  d.PrepareTime,
			Tags:         tags,
		}

		if d.Description.Valid {
			dish.Description = d.Description.String
		}
		if d.ImageUrl.Valid {
			dish.ImageURL = normalizeUploadURLForClient(d.ImageUrl.String)
		}
		if d.MemberPrice.Valid {
			dish.MemberPrice = &d.MemberPrice.Int64
		}

		dishList = append(dishList, dish)
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
	ImageURL      string          `json:"image_url,omitempty"`
	ComboPrice    int64           `json:"combo_price"`
	OriginalPrice int64           `json:"original_price"`
	Dishes        []comboDishItem `json:"dishes"`
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
		}

		if c.Description.Valid {
			combo.Description = c.Description.String
		}
		if c.ImageUrl.Valid {
			combo.ImageURL = normalizeUploadURLForClient(c.ImageUrl.String)
		}

		// 解析菜品
		if c.Dishes != nil {
			var dishes []comboDishItem
			if err := json.Unmarshal(c.Dishes, &dishes); err == nil {
				combo.Dishes = dishes
			}
		}

		comboList = append(comboList, combo)
	}

	ctx.JSON(http.StatusOK, publicMerchantCombosResponse{
		Combos: comboList,
	})
}

// ==================== 消费者端包间列表 ====================

type publicRoomItem struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Capacity     int16    `json:"capacity"`
	MinimumSpend *int64   `json:"minimum_spend,omitempty"`
	Description  string   `json:"description,omitempty"`
	PrimaryImage string   `json:"primary_image,omitempty"` // 统一字段名：包间主图
	MonthlySales int64    `json:"monthly_sales"`
	Status       string   `json:"status"`
	Tags         []string `json:"tags"`
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
		if r.PrimaryImage != "" {
			room.PrimaryImage = normalizeUploadURLForClient(r.PrimaryImage)
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

	ctx.JSON(http.StatusOK, publicMerchantRoomsResponse{
		Rooms: roomList,
	})
}
