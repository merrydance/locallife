package api

// 注意：本文件下的 /v1/regions* API 当前作为“后备能力”保留。
// 线上联调阶段（及可能的生产形态）前端可能直接调用腾讯 LBS 接口获取区域/行政区划数据。
// 为避免切换/降级成本，这里不删除实现，仅保留以便未来切回或灾备使用。

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// regionResponse 地区响应结构
type regionResponse struct {
	ID        int64   `json:"id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Level     int16   `json:"level"`
	ParentID  *int64  `json:"parent_id,omitempty"`
	Longitude *string `json:"longitude,omitempty"`
	Latitude  *string `json:"latitude,omitempty"`
}

func newRegionResponse(region db.Region) regionResponse {
	resp := regionResponse{
		ID:    region.ID,
		Code:  region.Code,
		Name:  region.Name,
		Level: region.Level,
	}

	if region.ParentID.Valid {
		resp.ParentID = &region.ParentID.Int64
	}

	if region.Longitude.Valid {
		lng := fmt.Sprintf("%v", region.Longitude)
		resp.Longitude = &lng
	}

	if region.Latitude.Valid {
		lat := fmt.Sprintf("%v", region.Latitude)
		resp.Latitude = &lat
	}

	return resp
}

// getRegion godoc
// @Summary 获取区域详情
// @Description 根据ID获取区域详细信息
// @Tags 区域管理
// @Accept json
// @Produce json
// @Param id path int true "区域ID"
// @Success 200 {object} regionResponse "区域详情"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "区域不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/regions/{id} [get]
func (server *Server) getRegion(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	region, err := server.store.GetRegion(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("区域不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRegionResponse(region))
}

type listRegionsRequest struct {
	Level    *int16 `form:"level" binding:"omitempty,min=1,max=4"`
	ParentID *int64 `form:"parent_id" binding:"omitempty,min=1"`
	PageID   int32  `form:"page_id" binding:"required,min=1"`
	PageSize int32  `form:"page_size" binding:"required,min=5,max=100"`
}

// listRegions godoc
// @Summary 获取区域列表
// @Description 分页获取区域列表，支持按层级和上级区域筛选
// @Tags 区域管理
// @Accept json
// @Produce json
// @Param level query int false "区域层级（1=省 2=市 3=区县）" minimum(1) maximum(4)
// @Param parent_id query int false "上级区域ID" minimum(1)
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(100)
// @Success 200 {array} regionResponse "区域列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/regions [get]
func (server *Server) listRegions(ctx *gin.Context) {
	var req listRegionsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	arg := db.ListRegionsParams{
		Limit:  req.PageSize,
		Offset: (req.PageID - 1) * req.PageSize,
	}

	if req.Level != nil {
		arg.Level = pgtype.Int2{
			Int16: *req.Level,
			Valid: true,
		}
	}

	if req.ParentID != nil {
		arg.ParentID = pgtype.Int8{
			Int64: *req.ParentID,
			Valid: true,
		}
	}

	regions, err := server.store.ListRegions(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	responses := make([]regionResponse, len(regions))
	for i, region := range regions {
		responses[i] = newRegionResponse(region)
	}

	ctx.JSON(http.StatusOK, responses)
}

// listRegionChildren godoc
// @Summary 获取下级区域列表
// @Description 获取指定区域的所有直接下级区域
// @Tags 区域管理
// @Accept json
// @Produce json
// @Param id path int true "上级区域ID"
// @Success 200 {array} regionResponse "下级区域列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/regions/{id}/children [get]
func (server *Server) listRegionChildren(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	regions, err := server.store.ListRegionChildren(ctx, pgtype.Int8{
		Int64: id,
		Valid: true,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	responses := make([]regionResponse, len(regions))
	for i, region := range regions {
		responses[i] = newRegionResponse(region)
	}

	ctx.JSON(http.StatusOK, responses)
}

type searchRegionsRequest struct {
	Query string `form:"q" binding:"required,min=1,max=100"`
}

// searchRegions godoc
// @Summary 搜索区域
// @Description 按名称模糊搜索区域
// @Tags 区域管理
// @Accept json
// @Produce json
// @Param q query string true "搜索关键词" minLength(1) maxLength(100)
// @Success 200 {array} regionResponse "搜索结果（最多100条）"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/regions/search [get]
func (server *Server) searchRegions(ctx *gin.Context) {
	var req searchRegionsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	regions, err := server.store.SearchRegionsByName(ctx, db.SearchRegionsByNameParams{
		Column1: pgtype.Text{String: req.Query, Valid: true},
		Limit:   100, // 限制返回前100个结果
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	responses := make([]regionResponse, len(regions))
	for i, region := range regions {
		responses[i] = newRegionResponse(region)
	}

	ctx.JSON(http.StatusOK, responses)
}

// ==================== 区域可用性检查 ====================

type availableRegionResponse struct {
	ID         int64  `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Level      int16  `json:"level"`
	ParentID   *int64 `json:"parent_id,omitempty"`
	ParentName string `json:"parent_name,omitempty"`
}

type regionAvailabilityResponse struct {
	RegionID    int64  `json:"region_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	IsAvailable bool   `json:"is_available"`
	Reason      string `json:"reason,omitempty"` // 不可用原因
}

// listAvailableRegions godoc
// @Summary 获取可申请的区县列表
// @Description 返回当前尚未被运营商绑定的区县列表，供新运营商选择
// @Tags 区域管理
// @Accept json
// @Produce json
// @Param parent_id query int false "上级区域ID（可选，用于过滤指定城市下的区县）" minimum(1)
// @Param level query int false "区域层级" minimum(1) maximum(4)
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(100)
// @Success 200 {object} map[string]interface{} "可申请的区县列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/regions/available [get]
func (server *Server) listAvailableRegions(ctx *gin.Context) {
	var req listRegionsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	offset := (req.PageID - 1) * req.PageSize

	// 使用优化的 SQL 查询，一次性获取未被运营商占用的区域
	var parentID pgtype.Int8
	var level pgtype.Int2

	if req.ParentID != nil {
		parentID = pgtype.Int8{Int64: *req.ParentID, Valid: true}
	}

	if req.Level != nil {
		level = pgtype.Int2{Int16: *req.Level, Valid: true}
	}
	// 注：如果没有指定 level，SQL 查询默认返回 level=3 的区县

	regions, err := server.store.ListAvailableRegions(ctx, db.ListAvailableRegionsParams{
		Limit:    req.PageSize,
		Offset:   offset,
		ParentID: parentID,
		Level:    level,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应格式
	availableRegions := make([]availableRegionResponse, len(regions))
	for i, region := range regions {
		resp := availableRegionResponse{
			ID:    region.ID,
			Code:  region.Code,
			Name:  region.Name,
			Level: region.Level,
		}
		if region.ParentID.Valid {
			resp.ParentID = &region.ParentID.Int64
		}
		if region.ParentName.Valid {
			resp.ParentName = region.ParentName.String
		}
		availableRegions[i] = resp
	}

	ctx.JSON(http.StatusOK, gin.H{
		"regions":   availableRegions,
		"total":     len(availableRegions),
		"page_id":   req.PageID,
		"page_size": req.PageSize,
	})
}

// checkRegionAvailability godoc
// @Summary 检查区县是否可申请
// @Description 检查指定区县是否已被其他运营商绑定
// @Tags 区域管理
// @Accept json
// @Produce json
// @Param id path int true "区域ID"
// @Success 200 {object} regionAvailabilityResponse "区域可用性信息"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "区域不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/regions/{id}/check [get]
func (server *Server) checkRegionAvailability(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取区域信息
	region, err := server.store.GetRegion(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("区域不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := regionAvailabilityResponse{
		RegionID: region.ID,
		Code:     region.Code,
		Name:     region.Name,
	}

	// 检查是否已有运营商绑定
	operator, err := server.store.GetOperatorByRegion(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 没有运营商，可用
			response.IsAvailable = true
		} else {
			// 数据库查询失败，返回500
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	} else {
		// 已有运营商绑定
		response.IsAvailable = false
		response.Reason = fmt.Sprintf("该区域已有运营商（%s）运营，不可申请", operator.Name)
	}

	ctx.JSON(http.StatusOK, response)
}
