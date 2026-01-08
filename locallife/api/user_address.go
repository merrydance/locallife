package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

// userAddressResponse 用户地址响应结构
type userAddressResponse struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"user_id"`
	RegionID      int64  `json:"region_id"`
	RegionName    string `json:"region_name,omitempty"`
	DetailAddress string `json:"detail_address"`
	ContactName   string `json:"contact_name"`
	ContactPhone  string `json:"contact_phone"`
	Longitude     string `json:"longitude"`
	Latitude      string `json:"latitude"`
	IsDefault     bool   `json:"is_default"`
	CreatedAt     string `json:"created_at"`
}

func newUserAddressResponse(address db.UserAddress) userAddressResponse {
	// 正确格式化经纬度为小数字符串
	lon, _ := parseNumericToFloat(address.Longitude)
	lat, _ := parseNumericToFloat(address.Latitude)

	return userAddressResponse{
		ID:            address.ID,
		UserID:        address.UserID,
		RegionID:      address.RegionID,
		DetailAddress: address.DetailAddress,
		ContactName:   address.ContactName,
		ContactPhone:  address.ContactPhone,
		Longitude:     strconv.FormatFloat(lon, 'f', -1, 64),
		Latitude:      strconv.FormatFloat(lat, 'f', -1, 64),
		IsDefault:     address.IsDefault,
		CreatedAt:     address.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

type createUserAddressRequest struct {
	RegionID      *int64 `json:"region_id" binding:"omitempty,min=1" example:"1"`
	DetailAddress string `json:"detail_address" binding:"required,min=1,max=200" example:"深圳市南山区科苑路15号"`
	ContactName   string `json:"contact_name" binding:"required,min=1,max=50" example:"张三"`
	ContactPhone  string `json:"contact_phone" binding:"required,min=11,max=11" example:"13800138000"`
	Longitude     string `json:"longitude" binding:"required" example:"113.946874"`
	Latitude      string `json:"latitude" binding:"required" example:"22.528499"`
	IsDefault     bool   `json:"is_default" example:"false"`
}

// createUserAddress godoc
// @Summary 创建用户地址
// @Description 为当前用户创建新的收货地址。若未提供 region_id，后端将根据经纬度自动匹配。
// @Tags 用户地址
// @Accept json
// @Produce json
// @Param request body createUserAddressRequest true "创建地址请求"
// @Success 200 {object} userAddressResponse "创建成功的地址"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/addresses [post]
// @Security BearerAuth
func (server *Server) createUserAddress(ctx *gin.Context) {
	var req createUserAddressRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	var regionID int64
	if req.RegionID != nil {
		regionID = *req.RegionID
		// 验证 region 是否存在
		_, err := server.store.GetRegion(ctx, regionID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("region not found")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	} else {
		// 自动匹配 region_id
		lat, _ := strconv.ParseFloat(req.Latitude, 64)
		lon, _ := strconv.ParseFloat(req.Longitude, 64)
		var err error
		regionID, err = server.matchRegionID(ctx, lat, lon)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("failed to match region: %w", err)))
			return
		}
	}

	// 创建经纬度Numeric类型
	var longitude pgtype.Numeric
	if err := longitude.Scan(req.Longitude); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid longitude format: %w", err)))
		return
	}

	var latitude pgtype.Numeric
	if err := latitude.Scan(req.Latitude); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid latitude format: %w", err)))
		return
	}

	arg := db.CreateUserAddressParams{
		UserID:        authPayload.UserID,
		RegionID:      regionID,
		DetailAddress: req.DetailAddress,
		ContactName:   req.ContactName,
		ContactPhone:  req.ContactPhone,
		Longitude:     longitude,
		Latitude:      latitude,
		IsDefault:     req.IsDefault,
	}

	// 如果设置为默认地址，先清除其他默认地址
	if req.IsDefault {
		err := server.store.SetDefaultAddress(ctx, authPayload.UserID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("failed to clear default addresses: %w", err)))
			return
		}
	}

	address, err := server.store.CreateUserAddress(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newUserAddressResponse(address))
}

// getUserAddress godoc
// @Summary 获取用户地址
// @Description 根据ID获取指定地址
// @Tags 用户地址
// @Accept json
// @Produce json
// @Param id path int true "地址ID"
// @Success 200 {object} userAddressResponse "地址信息"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "禁止访问"
// @Failure 404 {object} ErrorResponse "地址不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/addresses/{id} [get]
// @Security BearerAuth
func (server *Server) getUserAddress(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	address, err := server.store.GetUserAddress(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证地址属于当前用户
	if address.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(fmt.Errorf("access denied")))
		return
	}

	ctx.JSON(http.StatusOK, newUserAddressResponse(address))
}

// listUserAddresses godoc
// @Summary 获取用户地址列表
// @Description 获取当前用户的所有地址
// @Tags 用户地址
// @Accept json
// @Produce json
// @Success 200 {array} userAddressResponse "地址列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/addresses [get]
// @Security BearerAuth
func (server *Server) listUserAddresses(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	addresses, err := server.store.ListUserAddresses(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	responses := make([]userAddressResponse, len(addresses))
	for i, address := range addresses {
		responses[i] = newUserAddressResponse(address)
	}

	ctx.JSON(http.StatusOK, responses)
}

type updateUserAddressRequest struct {
	RegionID      *int64  `json:"region_id" binding:"omitempty,min=1" example:"1"`
	DetailAddress *string `json:"detail_address" binding:"omitempty,min=1,max=200" example:"深圳市南山区科苑路15号"`
	ContactName   *string `json:"contact_name" binding:"omitempty,min=1,max=50" example:"张三"`
	ContactPhone  *string `json:"contact_phone" binding:"omitempty,min=11,max=11" example:"13800138000"`
	Longitude     *string `json:"longitude" example:"113.946874"`
	Latitude      *string `json:"latitude" example:"22.528499"`
}

// updateUserAddress godoc
// @Summary 更新用户地址
// @Description 更新指定地址的信息。若更新了经纬度但未提供 region_id，后端将自动重新匹配。
// @Tags 用户地址
// @Accept json
// @Produce json
// @Param id path int true "地址ID"
// @Param request body updateUserAddressRequest true "更新地址请求"
// @Success 200 {object} userAddressResponse "更新后的地址"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "禁止访问"
// @Failure 404 {object} ErrorResponse "地址不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/addresses/{id} [patch]
// @Security BearerAuth
func (server *Server) updateUserAddress(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateUserAddressRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证地址存在且属于当前用户
	existingAddress, err := server.store.GetUserAddress(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if existingAddress.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(fmt.Errorf("access denied")))
		return
	}

	arg := db.UpdateUserAddressParams{
		ID:     id,
		UserID: authPayload.UserID,
	}

	if req.RegionID != nil {
		arg.RegionID = pgtype.Int8{
			Int64: *req.RegionID,
			Valid: true,
		}
	}

	if req.DetailAddress != nil {
		arg.DetailAddress = pgtype.Text{
			String: *req.DetailAddress,
			Valid:  true,
		}
	}

	if req.ContactName != nil {
		arg.ContactName = pgtype.Text{
			String: *req.ContactName,
			Valid:  true,
		}
	}

	if req.ContactPhone != nil {
		arg.ContactPhone = pgtype.Text{
			String: *req.ContactPhone,
			Valid:  true,
		}
	}

	// 处理经纬度更新和自动匹配 region_id
	var lat, lon float64
	var latValid, lonValid bool

	if req.Longitude != nil {
		var lng pgtype.Numeric
		if err := lng.Scan(*req.Longitude); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid longitude format: %w", err)))
			return
		}
		arg.Longitude = lng
		lon, _ = strconv.ParseFloat(*req.Longitude, 64)
		lonValid = true
	} else {
		lon, _ = parseNumericToFloat(existingAddress.Longitude)
		lonValid = true
	}

	if req.Latitude != nil {
		var l pgtype.Numeric
		if err := l.Scan(*req.Latitude); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid latitude format: %w", err)))
			return
		}
		arg.Latitude = l
		lat, _ = strconv.ParseFloat(*req.Latitude, 64)
		latValid = true
	} else {
		lat, _ = parseNumericToFloat(existingAddress.Latitude)
		latValid = true
	}

	// 如果更新了经纬度但没有提供 region_id，则自动匹配
	if (req.Longitude != nil || req.Latitude != nil) && req.RegionID == nil {
		if latValid && lonValid {
			regionID, err := server.matchRegionID(ctx, lat, lon)
			if err == nil {
				arg.RegionID = pgtype.Int8{Int64: regionID, Valid: true}
			}
		}
	}

	address, err := server.store.UpdateUserAddress(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newUserAddressResponse(address))
}

// setDefaultAddress godoc
// @Summary 设置默认地址
// @Description 将指定地址设为默认收货地址
// @Tags 用户地址
// @Accept json
// @Produce json
// @Param id path int true "地址ID"
// @Success 200 {object} userAddressResponse "设置后的默认地址"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "禁止访问"
// @Failure 404 {object} ErrorResponse "地址不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/addresses/{id}/default [patch]
// @Security BearerAuth
func (server *Server) setDefaultAddress(ctx *gin.Context) {
	idStr := ctx.Param("id")
	addressID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid address id: %w", err)))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证地址存在且属于当前用户
	existingAddress, err := server.store.GetUserAddress(ctx, addressID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if existingAddress.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(fmt.Errorf("access denied")))
		return
	}

	// 先清除所有默认地址
	err = server.store.SetDefaultAddress(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("failed to clear default addresses: %w", err)))
		return
	}

	// 设置新的默认地址
	address, err := server.store.SetAddressAsDefault(ctx, db.SetAddressAsDefaultParams{
		ID:     addressID,
		UserID: authPayload.UserID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newUserAddressResponse(address))
}

// deleteUserAddress godoc
// @Summary 删除用户地址
// @Description 删除指定地址
// @Tags 用户地址
// @Accept json
// @Produce json
// @Param id path int true "地址ID"
// @Success 204 "删除成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "禁止访问"
// @Failure 404 {object} ErrorResponse "地址不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/addresses/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteUserAddress(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证地址存在且属于当前用户
	existingAddress, err := server.store.GetUserAddress(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if existingAddress.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(fmt.Errorf("access denied")))
		return
	}

	err = server.store.DeleteUserAddress(ctx, db.DeleteUserAddressParams{
		ID:     id,
		UserID: authPayload.UserID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.Status(http.StatusNoContent)
}
