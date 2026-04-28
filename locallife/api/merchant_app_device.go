package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
)

type registerMerchantAppDeviceRequest struct {
	DeviceID    string `json:"device_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	PushToken   string `json:"push_token" binding:"required" example:"vendor-token"`
	Platform    string `json:"platform" binding:"required" example:"android"`
	Provider    string `json:"provider,omitempty" example:"xiaomi"`
	DeviceModel string `json:"device_model,omitempty" example:"Redmi K70"`
	OSVersion   string `json:"os_version,omitempty" example:"Android 15"`
	AppVersion  string `json:"app_version,omitempty" example:"1.0.0"`
}

type heartbeatMerchantAppDeviceRequest struct {
	DeviceID    string `json:"device_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	Provider    string `json:"provider,omitempty" example:"xiaomi"`
	PushToken   string `json:"push_token,omitempty" example:"vendor-token"`
	DeviceModel string `json:"device_model,omitempty" example:"Redmi K70"`
	OSVersion   string `json:"os_version,omitempty" example:"Android 15"`
	AppVersion  string `json:"app_version,omitempty" example:"1.0.0"`
}

type registerMerchantAppDeviceResponse struct {
	DeviceID     string `json:"device_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Provider     string `json:"provider" example:"xiaomi"`
	Registered   bool   `json:"registered" example:"true"`
	MerchantID   int64  `json:"merchant_id" example:"2001"`
	MerchantName string `json:"merchant_name" example:"示例门店"`
}

type heartbeatMerchantAppDeviceResponse struct {
	DeviceID  string `json:"device_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Heartbeat bool   `json:"heartbeat" example:"true"`
}

type unregisterMerchantAppDeviceResponse struct {
	DeviceID     string `json:"device_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Unregistered bool   `json:"unregistered" example:"true"`
}

// registerMerchantAppDevice godoc
// @Summary 注册商户 App 原生推送设备
// @Description 将当前登录商户、登录用户、设备 ID 与厂商原生推送 token 建立幂等绑定
// @Tags merchant-app-device
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body registerMerchantAppDeviceRequest true "设备注册请求"
// @Success 200 {object} registerMerchantAppDeviceResponse
// @Failure 400 {object} errorMessage
// @Failure 401 {object} errorMessage
// @Failure 403 {object} errorMessage
// @Failure 500 {object} errorMessage
// @Router /v1/merchant/device/register [post]
func (server *Server) registerMerchantAppDevice(ctx *gin.Context) {
	var req registerMerchantAppDeviceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant context missing")))
		return
	}

	result, err := logic.RegisterMerchantAppDevice(ctx, server.store, logic.MerchantAppDeviceRegisterInput{
		MerchantID:  merchant.ID,
		UserID:      authPayload.UserID,
		DeviceID:    req.DeviceID,
		PushToken:   req.PushToken,
		Platform:    req.Platform,
		Provider:    req.Provider,
		DeviceModel: req.DeviceModel,
		OSVersion:   req.OSVersion,
		AppVersion:  req.AppVersion,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, registerMerchantAppDeviceResponse{
		DeviceID:     result.DeviceID,
		Provider:     result.Device.Provider,
		Registered:   result.Registered,
		MerchantID:   merchant.ID,
		MerchantName: merchant.Name,
	})
}

// heartbeatMerchantAppDevice godoc
// @Summary 上报商户 App 设备心跳
// @Description 更新当前登录商户设备的活跃时间、App 版本、设备信息和可选推送 token
// @Tags merchant-app-device
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body heartbeatMerchantAppDeviceRequest true "设备心跳请求"
// @Success 200 {object} heartbeatMerchantAppDeviceResponse
// @Failure 400 {object} errorMessage
// @Failure 401 {object} errorMessage
// @Failure 403 {object} errorMessage
// @Failure 404 {object} errorMessage
// @Failure 500 {object} errorMessage
// @Router /v1/merchant/device/heartbeat [put]
func (server *Server) heartbeatMerchantAppDevice(ctx *gin.Context) {
	var req heartbeatMerchantAppDeviceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant context missing")))
		return
	}

	result, err := logic.HeartbeatMerchantAppDevice(ctx, server.store, logic.MerchantAppDeviceHeartbeatInput{
		MerchantID:  merchant.ID,
		UserID:      authPayload.UserID,
		DeviceID:    req.DeviceID,
		Provider:    req.Provider,
		PushToken:   req.PushToken,
		DeviceModel: req.DeviceModel,
		OSVersion:   req.OSVersion,
		AppVersion:  req.AppVersion,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, heartbeatMerchantAppDeviceResponse{DeviceID: result.DeviceID, Heartbeat: result.Heartbeat})
}

// unregisterMerchantAppDevice godoc
// @Summary 解绑商户 App 原生推送设备
// @Description 登出、设备失效或切换商户时失效当前登录商户下的设备推送绑定
// @Tags merchant-app-device
// @Produce json
// @Security BearerAuth
// @Param device_id path string true "设备 ID"
// @Success 200 {object} unregisterMerchantAppDeviceResponse
// @Failure 400 {object} errorMessage
// @Failure 401 {object} errorMessage
// @Failure 403 {object} errorMessage
// @Failure 500 {object} errorMessage
// @Router /v1/merchant/device/{device_id} [delete]
func (server *Server) unregisterMerchantAppDevice(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant context missing")))
		return
	}

	result, err := logic.UnregisterMerchantAppDevice(ctx, server.store, logic.MerchantAppDeviceUnregisterInput{
		MerchantID: merchant.ID,
		UserID:     authPayload.UserID,
		DeviceID:   ctx.Param("device_id"),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, unregisterMerchantAppDeviceResponse{DeviceID: result.DeviceID, Unregistered: result.Unregistered})
}
