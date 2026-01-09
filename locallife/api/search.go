package api

import (
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// =============================================================================
// Search API Handlers
// 搜索功能 - 菜品搜索和商户搜索
// =============================================================================

// ========================= Request/Response Types ============================

type searchDishesRequest struct {
	Keyword       string   `form:"keyword" binding:"omitempty,max=100"`   // 可选：为空时返回全部
	MerchantID    *int64   `form:"merchant_id" binding:"omitempty,min=1"` // 可选：在特定商户内搜索
	PageID        int32    `form:"page_id" binding:"required,min=1"`
	PageSize      int32    `form:"page_size" binding:"required,min=1,max=50"`
	UserLatitude  *float64 `form:"user_latitude" binding:"omitempty"`  // 用户当前纬度
	UserLongitude *float64 `form:"user_longitude" binding:"omitempty"` // 用户当前经度
}

type searchMerchantsRequest struct {
	Keyword       string   `form:"keyword" binding:"omitempty,max=100"` // 可选：为空时返回全部
	PageID        int32    `form:"page_id" binding:"required,min=1"`
	PageSize      int32    `form:"page_size" binding:"required,min=1,max=50"`
	UserLatitude  *float64 `form:"user_latitude" binding:"omitempty"`  // 用户当前纬度
	UserLongitude *float64 `form:"user_longitude" binding:"omitempty"` // 用户当前经度
}

type searchDishResponse struct {
	ID             int64   `json:"id"`
	MerchantID     int64   `json:"merchant_id"`
	CategoryID     *int64  `json:"category_id,omitempty"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	ImageURL       string  `json:"image_url"`
	Price          float64 `json:"price"`
	MemberPrice    float64 `json:"member_price,omitempty"`
	IsAvailable    bool    `json:"is_available"`
	IsOnline       bool    `json:"is_online"`
	SortOrder      int16   `json:"sort_order"`
	MonthlySales   int32   `json:"monthly_sales"`
	RepurchaseRate float64 `json:"repurchase_rate"`
	// New fields for home feed
	MerchantName          string `json:"merchant_name,omitempty"`
	MerchantLogo          string `json:"merchant_logo,omitempty"`
	MerchantIsOpen        *bool  `json:"merchant_is_open,omitempty"`
	Distance              int    `json:"distance"`                // Meters
	EstimatedDeliveryTime int    `json:"estimated_delivery_time"` // Seconds
}

type searchMerchantResponse struct {
	ID                   int64   `json:"id"`
	Name                 string  `json:"name"`
	Description          string  `json:"description"`
	Address              string  `json:"address"`
	Latitude             float64 `json:"latitude"`
	Longitude            float64 `json:"longitude"`
	Phone                string  `json:"phone"`
	LogoURL              string  `json:"logo_url"`
	Status               string  `json:"status"`
	RegionID             int64   `json:"region_id"`                        // 区域ID，用于运费计算
	TotalOrders          int32   `json:"total_orders,omitempty"`           // 总销量
	Distance             *int    `json:"distance,omitempty"`               // 距离（米），需要传入用户位置
	EstimatedDeliveryFee *int64  `json:"estimated_delivery_fee,omitempty"` // 预估配送费（分），需要传入用户位置
}

// searchDishes godoc
// ... (comments remain same)
func (server *Server) searchDishes(ctx *gin.Context) {
	var req searchDishesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	offset := (req.PageID - 1) * req.PageSize

	// 如果指定了商户ID，在特定商户内搜索 (This part uses old query structure, keeping simple response or need upgrade too?
	// The requirement is mostly for Global Search (Home Feed). Keeping Merchant Search as is for now but we need to match response type.)
	if req.MerchantID != nil {
		dishes, err := server.store.SearchDishesByName(ctx, db.SearchDishesByNameParams{
			MerchantID: *req.MerchantID,
			Column2:    pgtype.Text{String: req.Keyword, Valid: true},
			Limit:      req.PageSize,
			Offset:     offset,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 获取总数用于分页
		total, err := server.store.CountSearchDishesByName(ctx, db.CountSearchDishesByNameParams{
			MerchantID: *req.MerchantID,
			Column2:    pgtype.Text{String: req.Keyword, Valid: true},
		})
		if err != nil {
			total = int64(len(dishes))
		}

		// Convert simple Dish to Enriched Response (with empty merchant info as it's implicit)
		response := make([]searchDishResponse, len(dishes))
		for i, dish := range dishes {
			response[i] = newSearchDishResponseFromDish(dish)
		}

		ctx.JSON(http.StatusOK, gin.H{
			"dishes":    response,
			"total":     total,
			"page_id":   req.PageID,
			"page_size": req.PageSize,
		})
		return
	}

	// 准备位置参数（用于排序）
	var userLat, userLng float64
	if req.UserLatitude != nil && req.UserLongitude != nil {
		userLat = *req.UserLatitude
		userLng = *req.UserLongitude
	}

	// 全局搜索 - 使用高效的单次数据库查询（仅搜索已批准商户的上架菜品）
	dishes, err := server.store.SearchDishesGlobal(ctx, db.SearchDishesGlobalParams{
		Column1: pgtype.Text{String: req.Keyword, Valid: true},
		Limit:   req.PageSize,
		Offset:  int32(offset),
		Column4: userLat,
		Column5: userLng,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取总数用于分页
	total, err := server.store.CountSearchDishesGlobal(ctx, pgtype.Text{String: req.Keyword, Valid: true})
	if err != nil {
		total = int64(len(dishes))
	}

	response := make([]searchDishResponse, len(dishes))
	for i, dish := range dishes {
		response[i] = newSearchDishResponseFromGlobalRow(dish)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"dishes":    response,
		"total":     total,
		"page_id":   req.PageID,
		"page_size": req.PageSize,
	})
}

// searchMerchants godoc
// @Summary 搜索商户
// @Description 根据关键词搜索商户，可传入用户位置计算距离和预估运费
// @Tags Search
// @Accept json
// @Produce json
// @Param keyword query string true "搜索关键词"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Param user_latitude query number false "用户当前纬度（用于计算距离和运费）"
// @Param user_longitude query number false "用户当前经度（用于计算距离和运费）"
// @Success 200 {object} map[string]interface{} "搜索结果"
// @Failure 400 {object} map[string]string "请求参数错误"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /v1/search/merchants [get]
func (server *Server) searchMerchants(ctx *gin.Context) {
	var req searchMerchantsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	offset := (req.PageID - 1) * req.PageSize

	// 准备位置参数（用于排序）
	var userLat, userLng float64
	if req.UserLatitude != nil && req.UserLongitude != nil {
		userLat = *req.UserLatitude
		userLng = *req.UserLongitude
	}

	merchants, err := server.store.SearchMerchants(ctx, db.SearchMerchantsParams{
		Offset:  int32(offset),
		Limit:   req.PageSize,
		Column3: req.Keyword,
		Column4: userLat,
		Column5: userLng,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取总数用于分页
	total, err := server.store.CountSearchMerchants(ctx, pgtype.Text{String: req.Keyword, Valid: true})
	if err != nil {
		total = int64(len(merchants))
	}

	response := make([]searchMerchantResponse, len(merchants))
	for i, merchant := range merchants {
		response[i] = newSearchMerchantResponseFromRow(merchant)
	}

	// 如果用户提供了位置，计算精确距离（展示用）和运费
	if req.UserLatitude != nil && req.UserLongitude != nil && server.mapClient != nil {
		server.calculateSearchMerchantDistancesAndFees(ctx, response, *req.UserLatitude, *req.UserLongitude)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"merchants": response,
		"total":     total,
		"page_id":   req.PageID,
		"page_size": req.PageSize,
	})
}

// Helper for simple Dish (Merchant Search)
func newSearchDishResponseFromDish(dish db.Dish) searchDishResponse {
	resp := searchDishResponse{
		ID:           dish.ID,
		MerchantID:   dish.MerchantID,
		Name:         dish.Name,
		Description:  dish.Description.String,
		ImageURL:     normalizeUploadURLForClient(dish.ImageUrl.String),
		Price:        float64(dish.Price) / 100,
		IsAvailable:  dish.IsAvailable,
		IsOnline:     dish.IsOnline,
		SortOrder:    dish.SortOrder,
		MonthlySales: dish.MonthlySales,
	}
	if dish.RepurchaseRate.Valid {
		val, _ := dish.RepurchaseRate.Float64Value()
		resp.RepurchaseRate = val.Float64
	}
	if dish.CategoryID.Valid {
		resp.CategoryID = &dish.CategoryID.Int64
	}
	if dish.MemberPrice.Valid {
		resp.MemberPrice = float64(dish.MemberPrice.Int64) / 100
	}
	// Merchant info is empty for merchant-specific search
	return resp
}

// Helper for Global Search Row
func newSearchDishResponseFromGlobalRow(row db.SearchDishesGlobalRow) searchDishResponse {
	// Calculate estimated delivery time (PrepareTime + Distance/Speed)
	// Assume average speed 200m/min (12km/h) for simple estimation if real routing not available
	deliveryTimeSeconds := int(row.PrepareTime) * 60
	if row.Distance > 0 {
		travelTimeSeconds := int(row.Distance / 3.33) // 3.33 m/s = 12 km/h
		deliveryTimeSeconds += travelTimeSeconds
	}

	resp := searchDishResponse{
		ID:           row.ID,
		MerchantID:   row.MerchantID,
		Name:         row.Name,
		Description:  row.Description.String,
		ImageURL:     normalizeUploadURLForClient(row.ImageUrl.String),
		Price:        float64(row.Price) / 100,
		IsAvailable:  row.IsAvailable,
		IsOnline:     row.IsOnline,
		SortOrder:    row.SortOrder,
		MonthlySales: row.MonthlySales,
		// Enriched Fields
		MerchantName:          row.MerchantName,
		MerchantLogo:          normalizeUploadURLForClient(row.MerchantLogo.String),
		MerchantIsOpen:        &row.MerchantIsOpen,
		Distance:              int(row.Distance),
		EstimatedDeliveryTime: deliveryTimeSeconds,
	}

	if row.RepurchaseRate.Valid {
		val, _ := row.RepurchaseRate.Float64Value()
		resp.RepurchaseRate = val.Float64
	}
	if row.CategoryID.Valid {
		resp.CategoryID = &row.CategoryID.Int64
	}
	if row.MemberPrice.Valid {
		resp.MemberPrice = float64(row.MemberPrice.Int64) / 100
	}
	return resp
}

func newSearchMerchantResponse(merchant db.Merchant) searchMerchantResponse {
	resp := searchMerchantResponse{
		ID:          merchant.ID,
		Name:        merchant.Name,
		Description: merchant.Description.String,
		Address:     merchant.Address,
		Phone:       merchant.Phone,
		LogoURL:     normalizeUploadURLForClient(merchant.LogoUrl.String),
		Status:      merchant.Status,
		RegionID:    merchant.RegionID, // 添加区域ID用于运费计算
	}
	// 转换 pgtype.Numeric 到 float64
	if merchant.Latitude.Valid {
		lat, _ := merchant.Latitude.Float64Value()
		resp.Latitude = lat.Float64
	}
	if merchant.Longitude.Valid {
		lng, _ := merchant.Longitude.Float64Value()
		resp.Longitude = lng.Float64
	}
	return resp
}

func newSearchMerchantResponseFromRow(merchant db.SearchMerchantsRow) searchMerchantResponse {
	resp := searchMerchantResponse{
		ID:          merchant.ID,
		Name:        merchant.Name,
		Description: merchant.Description.String,
		Address:     merchant.Address,
		Phone:       merchant.Phone,
		LogoURL:     normalizeUploadURLForClient(merchant.LogoUrl.String),
		Status:      merchant.Status,
		RegionID:    merchant.RegionID, // 添加区域ID用于运费计算
		TotalOrders: merchant.TotalOrders,
	}
	// 转换 pgtype.Numeric 到 float64
	if merchant.Latitude.Valid {
		lat, _ := merchant.Latitude.Float64Value()
		resp.Latitude = lat.Float64
	}
	if merchant.Longitude.Valid {
		lng, _ := merchant.Longitude.Float64Value()
		resp.Longitude = lng.Float64
	}
	return resp
}

// =============================================================================
// Room Search API
// 包间搜索功能 - 按日期、时段、人数、菜系等条件搜索可用包间
// =============================================================================

type searchRoomsRequest struct {
	RegionID        *int64 `form:"region_id" binding:"omitempty,min=1"`            // 区域ID
	MinCapacity     *int16 `form:"min_capacity" binding:"omitempty,min=1,max=100"` // 最小容纳人数
	MaxCapacity     *int16 `form:"max_capacity" binding:"omitempty,min=1,max=100"` // 最大容纳人数
	MaxMinimumSpend *int64 `form:"max_minimum_spend" binding:"omitempty,min=0"`    // 最大低消（分）
	ReservationDate string `form:"reservation_date" binding:"required"`            // 预订日期 YYYY-MM-DD
	ReservationTime string `form:"reservation_time" binding:"required"`            // 预订时段 HH:MM
	TagID           *int64 `form:"tag_id" binding:"omitempty,min=1"`               // 菜系/标签ID
	PageID          int32  `form:"page_id" binding:"required,min=1"`               // 页码
	PageSize        int32  `form:"page_size" binding:"required,min=1,max=50"`      // 每页数量
}

type searchRoomResponse struct {
	ID                int64   `json:"id"`
	MerchantID        int64   `json:"merchant_id"`
	TableNo           string  `json:"table_no"`
	Capacity          int16   `json:"capacity"`
	Description       *string `json:"description,omitempty"`
	MinimumSpend      *int64  `json:"minimum_spend,omitempty"` // 分
	Status            string  `json:"status"`
	MerchantName      string  `json:"merchant_name"`
	MerchantLogo      *string `json:"merchant_logo,omitempty"`
	MerchantAddress   string  `json:"merchant_address"`
	MerchantLatitude  float64 `json:"merchant_latitude"`
	MerchantLongitude float64 `json:"merchant_longitude"`
	PrimaryImage      *string `json:"primary_image,omitempty"` // 包间主图
}

// searchRooms godoc
// @Summary 搜索包间
// @Description 根据日期、时段、人数、菜系等条件搜索可用包间
// @Tags Search
// @Accept json
// @Produce json
// @Param region_id query int false "区域ID"
// @Param min_capacity query int false "最小容纳人数"
// @Param max_capacity query int false "最大容纳人数"
// @Param max_minimum_spend query int false "最大低消（分）"
// @Param reservation_date query string true "预订日期（YYYY-MM-DD）"
// @Param reservation_time query string true "预订时段（HH:MM）"
// @Param tag_id query int false "菜系/标签ID"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Success 200 {object} map[string]interface{} "搜索结果"
// @Failure 400 {object} map[string]string "请求参数错误"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /v1/search/rooms [get]
func (server *Server) searchRooms(ctx *gin.Context) {
	var req searchRoomsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 解析日期和时间
	reservationDate, err := parseDate(req.ReservationDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	reservationTime, err := parseTime(req.ReservationTime)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	offset := (req.PageID - 1) * req.PageSize

	// 构建查询参数
	var regionID pgtype.Int8
	if req.RegionID != nil {
		regionID = pgtype.Int8{Int64: *req.RegionID, Valid: true}
	}

	var minCapacity, maxCapacity pgtype.Int2
	if req.MinCapacity != nil {
		minCapacity = pgtype.Int2{Int16: *req.MinCapacity, Valid: true}
	}
	if req.MaxCapacity != nil {
		maxCapacity = pgtype.Int2{Int16: *req.MaxCapacity, Valid: true}
	}

	var maxMinimumSpend pgtype.Int8
	if req.MaxMinimumSpend != nil {
		maxMinimumSpend = pgtype.Int8{Int64: *req.MaxMinimumSpend, Valid: true}
	}

	var rooms []searchRoomResponse
	var total int64

	// 如果指定了标签（菜系），使用按标签搜索
	if req.TagID != nil {
		searchResults, err := server.store.SearchRoomsByMerchantTag(ctx, db.SearchRoomsByMerchantTagParams{
			TagID:           *req.TagID,
			RegionID:        regionID,
			MinCapacity:     minCapacity,
			MaxCapacity:     maxCapacity,
			ReservationDate: reservationDate,
			ReservationTime: reservationTime,
			PageOffset:      offset,
			PageSize:        req.PageSize,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		rooms = make([]searchRoomResponse, len(searchResults))
		for i, r := range searchResults {
			rooms[i] = newSearchRoomResponseFromTag(r)
		}
		total = int64(len(searchResults))
	} else {
		// 不按标签搜索，使用带图片的查询
		searchResults, err := server.store.SearchRoomsWithImage(ctx, db.SearchRoomsWithImageParams{
			RegionID:        regionID,
			MinCapacity:     minCapacity,
			MaxCapacity:     maxCapacity,
			MaxMinimumSpend: maxMinimumSpend,
			ReservationDate: reservationDate,
			ReservationTime: reservationTime,
			PageOffset:      offset,
			PageSize:        req.PageSize,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		rooms = make([]searchRoomResponse, len(searchResults))
		for i, r := range searchResults {
			rooms[i] = newSearchRoomResponseWithImage(r)
		}

		// 获取总数
		total, err = server.store.CountSearchRooms(ctx, db.CountSearchRoomsParams{
			RegionID:        regionID,
			MinCapacity:     minCapacity,
			MaxCapacity:     maxCapacity,
			MaxMinimumSpend: maxMinimumSpend,
			ReservationDate: reservationDate,
			ReservationTime: reservationTime,
		})
		if err != nil {
			total = int64(len(rooms))
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"rooms":     rooms,
		"total":     total,
		"page_id":   req.PageID,
		"page_size": req.PageSize,
	})
}

func newSearchRoomResponseFromTag(r db.SearchRoomsByMerchantTagRow) searchRoomResponse {
	resp := searchRoomResponse{
		ID:              r.ID,
		MerchantID:      r.MerchantID,
		TableNo:         r.TableNo,
		Capacity:        r.Capacity,
		Status:          r.Status,
		MerchantName:    r.MerchantName,
		MerchantAddress: r.MerchantAddress,
	}

	if r.Description.Valid {
		resp.Description = &r.Description.String
	}
	if r.MinimumSpend.Valid {
		resp.MinimumSpend = &r.MinimumSpend.Int64
	}
	if r.MerchantLogo.Valid {
		logo := normalizeUploadURLForClient(r.MerchantLogo.String)
		resp.MerchantLogo = &logo
	}
	if r.MerchantLatitude.Valid {
		lat, _ := r.MerchantLatitude.Float64Value()
		resp.MerchantLatitude = lat.Float64
	}
	if r.MerchantLongitude.Valid {
		lng, _ := r.MerchantLongitude.Float64Value()
		resp.MerchantLongitude = lng.Float64
	}

	return resp
}

func newSearchRoomResponseWithImage(r db.SearchRoomsWithImageRow) searchRoomResponse {
	resp := searchRoomResponse{
		ID:              r.ID,
		MerchantID:      r.MerchantID,
		TableNo:         r.TableNo,
		Capacity:        r.Capacity,
		Status:          r.Status,
		MerchantName:    r.MerchantName,
		MerchantAddress: r.MerchantAddress,
	}

	if r.Description.Valid {
		resp.Description = &r.Description.String
	}
	if r.MinimumSpend.Valid {
		resp.MinimumSpend = &r.MinimumSpend.Int64
	}
	if r.MerchantLogo.Valid {
		logo := normalizeUploadURLForClient(r.MerchantLogo.String)
		resp.MerchantLogo = &logo
	}
	if r.PrimaryImage != "" {
		primaryImage := normalizeUploadURLForClient(r.PrimaryImage)
		resp.PrimaryImage = &primaryImage
	}
	if r.MerchantLatitude.Valid {
		lat, _ := r.MerchantLatitude.Float64Value()
		resp.MerchantLatitude = lat.Float64
	}
	if r.MerchantLongitude.Valid {
		lng, _ := r.MerchantLongitude.Float64Value()
		resp.MerchantLongitude = lng.Float64
	}

	return resp
}

// parseDate 解析日期字符串 (YYYY-MM-DD)
func parseDate(dateStr string) (pgtype.Date, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

// parseTime 解析时间字符串 (HH:MM 或 HH:MM:SS)
func parseTime(timeStr string) (pgtype.Time, error) {
	var t time.Time
	var err error

	// 尝试不同的时间格式
	if len(timeStr) == 5 { // HH:MM
		t, err = time.Parse("15:04", timeStr)
	} else { // HH:MM:SS
		t, err = time.Parse("15:04:05", timeStr)
	}

	if err != nil {
		return pgtype.Time{}, err
	}

	// 转换为微秒数
	microseconds := int64(t.Hour())*3600000000 + int64(t.Minute())*60000000 + int64(t.Second())*1000000
	return pgtype.Time{Microseconds: microseconds, Valid: true}, nil
}

// ==================== 距离和运费计算辅助函数 ====================

// calculateSearchMerchantDistancesAndFees 批量计算搜索结果中商户到用户的距离和预估运费
func (server *Server) calculateSearchMerchantDistancesAndFees(ctx *gin.Context, merchants []searchMerchantResponse, userLat, userLng float64) {
	if len(merchants) == 0 || server.mapClient == nil {
		return
	}

	// 构建商户位置列表
	var merchantLocs []maps.Location
	var validIndices []int // 记录有有效经纬度的商户索引

	for i, m := range merchants {
		if m.Latitude != 0 || m.Longitude != 0 {
			merchantLocs = append(merchantLocs, maps.Location{
				Lat: m.Latitude,
				Lng: m.Longitude,
			})
			validIndices = append(validIndices, i)
		}
	}

	if len(merchantLocs) == 0 {
		return
	}

	// 用户位置
	userLoc := []maps.Location{{Lat: userLat, Lng: userLng}}

	// 批量计算距离（骑行模式）
	result, err := server.mapClient.GetDistanceMatrix(ctx, merchantLocs, userLoc, "bicycling")
	if err != nil {
		return
	}

	// 填充距离和运费
	for i, row := range result.Rows {
		if i >= len(validIndices) {
			break
		}
		idx := validIndices[i]
		if len(row.Elements) > 0 {
			distance := row.Elements[0].Distance
			merchants[idx].Distance = &distance

			// 商户列表没有具体订单金额，传0表示只计算基础运费+距离费，不含货值加价
			// 时段系数和天气系数仍正常参与计算
			regionID := merchants[idx].RegionID
			merchantID := merchants[idx].ID
			feeResult, err := server.calculateDeliveryFeeInternal(ctx, regionID, merchantID, int32(distance), 0)
			if err == nil && feeResult != nil && !feeResult.DeliverySuspended {
				merchants[idx].EstimatedDeliveryFee = &feeResult.FinalFee
			}
		}
	}
}
