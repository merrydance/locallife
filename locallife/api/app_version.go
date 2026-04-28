package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/logic"
)

type getLatestAppVersionRequest struct {
	Platform    string `form:"platform" binding:"required" example:"android"`
	Channel     string `form:"channel" binding:"required" example:"merchant_app"`
	PackageName string `form:"package_name" binding:"required" example:"com.merrydance.locallife.merchant"`
	VersionCode int32  `form:"version_code" binding:"required,min=1" example:"1"`
	VersionName string `form:"version_name" example:"1.0.0"`
}

type getLatestAppVersionResponse struct {
	HasUpdate     bool       `json:"has_update" example:"true"`
	VersionCode   int32      `json:"version_code" example:"2"`
	VersionName   string     `json:"version_name" example:"1.0.1"`
	DownloadURL   string     `json:"download_url" example:"https://example.com/merchant-app-1.0.1.apk"`
	Changelog     string     `json:"changelog" example:"修复蓝牙连接稳定性"`
	IsForce       bool       `json:"is_force" example:"false"`
	PublishedAt   *time.Time `json:"published_at,omitempty" example:"2026-04-12T12:00:00Z"`
	FileSizeBytes *int64     `json:"file_size_bytes,omitempty" example:"48392120"`
	Sha256        string     `json:"sha256,omitempty" example:"3a6eb0790f39ac87c94f3856b2dd2c5d110e6811602261a9a923d3bb23adc8b7"`
}

// getLatestAppVersion godoc
// @Summary 查询 App 最新版本
// @Description 按平台、渠道、包名和当前版本码查询是否存在可升级版本；无更新时返回 200 和 has_update=false
// @Tags app-version
// @Produce json
// @Param platform query string true "平台" Enums(android)
// @Param channel query string true "渠道" Enums(merchant_app)
// @Param package_name query string true "包名"
// @Param version_code query int true "当前版本码" minimum(1)
// @Param version_name query string false "当前展示版本"
// @Success 200 {object} getLatestAppVersionResponse
// @Failure 400 {object} errorMessage
// @Failure 500 {object} errorMessage
// @Router /v1/app/version/latest [get]
func (server *Server) getLatestAppVersion(ctx *gin.Context) {
	var req getLatestAppVersionRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	result, err := logic.GetLatestAppVersion(ctx, server.store, logic.AppVersionLatestInput{
		Platform:    req.Platform,
		Channel:     req.Channel,
		PackageName: req.PackageName,
		VersionCode: req.VersionCode,
		VersionName: req.VersionName,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, getLatestAppVersionResponse{
		HasUpdate:     result.HasUpdate,
		VersionCode:   result.VersionCode,
		VersionName:   result.VersionName,
		DownloadURL:   result.DownloadURL,
		Changelog:     result.Changelog,
		IsForce:       result.IsForce,
		PublishedAt:   result.PublishedAt,
		FileSizeBytes: result.FileSizeBytes,
		Sha256:        result.Sha256,
	})
}
