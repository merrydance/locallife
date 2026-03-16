package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

// =============================================================================
// Search API Handlers
// 搜索功能 - 菜品搜索和商户搜索
// =============================================================================

// ========================= Request/Response Types ============================

type searchDishesRequest struct {
	Keyword       string   `form:"keyword" binding:"omitempty,max=100"`   // 可选：为空时返回全部
	MerchantID    *int64   `form:"merchant_id" binding:"omitempty,min=1"` // 可选：在特定商户内搜索
	RegionID      *int64   `form:"region_id" binding:"omitempty,min=1"`   // 可选：区域过滤
	PageID        int32    `form:"page_id" binding:"required,min=1"`
	PageSize      int32    `form:"page_size" binding:"required,min=1,max=50"`
	UserLatitude  *float64 `form:"user_latitude" binding:"omitempty"`  // 用户当前纬度
	UserLongitude *float64 `form:"user_longitude" binding:"omitempty"` // 用户当前经度
	TagID         *int64   `form:"tag_id" binding:"omitempty"`         // 可选：标签ID过滤
}

type searchMerchantsRequest struct {
	Keyword       string   `form:"keyword" binding:"omitempty,max=100"` // 可选：为空时返回全部
	RegionID      *int64   `form:"region_id" binding:"omitempty,min=1"` // 可选：区域过滤
	TagID         *int64   `form:"tag_id" binding:"omitempty,min=1"`    // 可选：标签（菜系）ID 过滤
	PageID        int32    `form:"page_id" binding:"required,min=1"`
	PageSize      int32    `form:"page_size" binding:"required,min=1,max=50"`
	UserLatitude  *float64 `form:"user_latitude" binding:"omitempty"`  // 用户当前纬度
	UserLongitude *float64 `form:"user_longitude" binding:"omitempty"` // 用户当前经度
}

type searchDishListResponse struct {
	Dishes   []searchDishResponse `json:"dishes"`
	Total    int64                `json:"total"`
	PageID   int32                `json:"page_id"`
	PageSize int32                `json:"page_size"`
}

type searchMerchantListResponse struct {
	Merchants []searchMerchantResponse `json:"merchants"`
	Total     int64                    `json:"total"`
	PageID    int32                    `json:"page_id"`
	PageSize  int32                    `json:"page_size"`
}

type searchComboListResponse struct {
	Combos   []searchComboResponse `json:"combos"`
	Total    int64                 `json:"total"`
	PageID   int32                 `json:"page_id"`
	PageSize int32                 `json:"page_size"`
}

type searchRoomListResponse struct {
	Rooms    []searchRoomResponse `json:"rooms"`
	Total    int64                `json:"total"`
	PageID   int32                `json:"page_id"`
	PageSize int32                `json:"page_size"`
}

type searchHistoryListResponse struct {
	History []db.ListSearchHistoryRow `json:"history"`
}

type searchKeywordsListResponse struct {
	Keywords []db.GetPopularKeywordsRow `json:"keywords"`
}

type searchSuggestionsListResponse struct {
	Suggestions []searchSuggestionItem `json:"suggestions"`
}

type searchCategoryItem struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	MerchantCount int32  `json:"merchant_count"`
}

type searchCategoriesListResponse struct {
	Categories []searchCategoryItem `json:"categories"`
}

type searchDishResponse struct {
	ID             int64   `json:"id"`
	MerchantID     int64   `json:"merchant_id"`
	CategoryID     *int64  `json:"category_id,omitempty"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	ImageURL       string  `json:"image_url"`
	Price          int64   `json:"price"`                  // Cents
	OriginalPrice  int64   `json:"original_price"`         // Cents
	MemberPrice    int64   `json:"member_price,omitempty"` // Cents
	IsAvailable    bool    `json:"is_available"`
	IsOnline       bool    `json:"is_online"`
	SortOrder      int16   `json:"sort_order"`
	MonthlySales   int32   `json:"monthly_sales"`
	RepurchaseRate float64 `json:"repurchase_rate"`
	// New fields for home feed
	MerchantName          string               `json:"merchant_name,omitempty"`
	MerchantLogo          string               `json:"merchant_logo,omitempty"`
	MerchantIsOpen        *bool                `json:"merchant_is_open,omitempty"`
	Distance              int                  `json:"distance"`                // Meters
	EstimatedDeliveryTime int                  `json:"estimated_delivery_time"` // Seconds
	EstimatedDeliveryFee  int                  `json:"estimated_delivery_fee"`  // Cents
	Attributes            []string             `json:"attributes,omitempty"`
	CustomizationGroups   []customizationGroup `json:"customization_groups,omitempty"`
}

type searchMerchantResponse struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	Address              string    `json:"address,omitempty"`
	Latitude             float64   `json:"-"`
	Longitude            float64   `json:"-"`
	Phone                string    `json:"phone,omitempty"`
	LogoURL              string    `json:"logo_url"`
	CoverImage           string    `json:"cover_image,omitempty"` // 门头照（首张），作为列表卡片封面
	Status               string    `json:"status"`
	IsOpen               bool      `json:"is_open"`
	RegionID             int64     `json:"region_id"`                        // 区域ID，用于运费计算
	TotalOrders          int32     `json:"total_orders,omitempty"`           // 总销量
	Distance             *int      `json:"distance,omitempty"`               // 距离（米），需要传入用户位置
	EstimatedDeliveryFee *int64    `json:"estimated_delivery_fee,omitempty"` // 预估配送费（分），需要传入用户位置
	Tags                 []string  `json:"tags,omitempty"`
	CreatedAt            time.Time `json:"created_at"`      // 入驻时间，前端用于判断新店
	Label                string    `json:"label,omitempty"` // 推荐 或 热销
}

type searchComboResponse struct {
	ID                    int64    `json:"id"`
	MerchantID            int64    `json:"merchant_id"`
	Name                  string   `json:"name"`
	Description           string   `json:"description"`
	ImageURL              string   `json:"image_url"`
	OriginalPrice         int64    `json:"original_price"`  // Cents
	ComboPrice            int64    `json:"combo_price"`     // Cents
	SavingsPercent        int      `json:"savings_percent"` // 节省百分比
	MonthlySales          int32    `json:"monthly_sales"`
	MerchantName          string   `json:"merchant_name"`
	MerchantLogo          string   `json:"merchant_logo"`
	MerchantIsOpen        bool     `json:"merchant_is_open"`
	Distance              int      `json:"distance"`
	EstimatedDeliveryFee  *int64   `json:"estimated_delivery_fee,omitempty"`
	EstimatedDeliveryTime int      `json:"estimated_delivery_time"`
	Tags                  []string `json:"tags,omitempty"`
}

func resolveUserLocation(ctx *gin.Context, reqLat, reqLng *float64) (*float64, *float64) {
	if reqLat != nil && reqLng != nil {
		return reqLat, reqLng
	}

	latHeader := ctx.GetHeader("X-User-Latitude")
	lngHeader := ctx.GetHeader("X-User-Longitude")
	if latHeader == "" || lngHeader == "" {
		return nil, nil
	}

	lat, err := strconv.ParseFloat(latHeader, 64)
	if err != nil {
		return nil, nil
	}
	lng, err := strconv.ParseFloat(lngHeader, 64)
	if err != nil {
		return nil, nil
	}

	return &lat, &lng
}

func resolveRegionID(ctx *gin.Context, server *Server, reqRegionID *int64, userLat, userLng *float64) (pgtype.Int8, error) {
	if reqRegionID != nil {
		return pgtype.Int8{Int64: *reqRegionID, Valid: true}, nil
	}

	if userLat != nil && userLng != nil {
		regionID, err := server.matchRegionID(ctx.Request.Context(), *userLat, *userLng)
		if err != nil {
			return pgtype.Int8{}, err
		}
		return pgtype.Int8{Int64: regionID, Valid: true}, nil
	}

	return pgtype.Int8{}, errors.New("region_id is required")
}

func isRegionUnavailableError(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// searchDishes godoc
// ... (comments remain same)
// @Security BearerAuth
func (server *Server) searchDishes(ctx *gin.Context) {
	var req searchDishesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	offset := pageOffset(req.PageID, req.PageSize)

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

		ctx.JSON(http.StatusOK, searchDishListResponse{
			Dishes:   response,
			Total:    total,
			PageID:   req.PageID,
			PageSize: req.PageSize,
		})
		return
	}

	// 准备位置参数（用于排序）
	resolvedLat, resolvedLng := resolveUserLocation(ctx, req.UserLatitude, req.UserLongitude)
	var userLat, userLng float64
	if resolvedLat != nil && resolvedLng != nil {
		userLat = *resolvedLat
		userLng = *resolvedLng
	}

	// 准备TagID
	var tagIDVal int64
	if req.TagID != nil {
		tagIDVal = *req.TagID
	}

	// 准备RegionID（全局搜索必须）
	regionID, err := resolveRegionID(ctx, server, req.RegionID, resolvedLat, resolvedLng)
	if err != nil {
		if isRegionUnavailableError(err) {
			ctx.JSON(http.StatusOK, searchDishListResponse{
				Dishes:   []searchDishResponse{},
				PageID:   req.PageID,
				PageSize: req.PageSize,
			})
			return
		}
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 全局搜索 - 使用高效的单次数据库查询（仅搜索已批准商户的上架菜品）
	dishes, err := server.store.SearchDishesGlobal(ctx, db.SearchDishesGlobalParams{
		Column1:  pgtype.Text{String: req.Keyword, Valid: true},
		Limit:    req.PageSize,
		Offset:   int32(offset),
		Column4:  userLat,
		Column5:  userLng,
		TagID:    pgtype.Int8{Int64: tagIDVal, Valid: req.TagID != nil},
		RegionID: regionID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取总数用于分页
	total, err := server.store.CountSearchDishesGlobal(ctx, db.CountSearchDishesGlobalParams{
		Column1:  pgtype.Text{String: req.Keyword, Valid: true},
		TagID:    pgtype.Int8{Int64: tagIDVal, Valid: req.TagID != nil},
		RegionID: regionID,
	})
	if err != nil {
		total = int64(len(dishes))
	}

	// 若用户提供位置，批量拉取路网距离覆盖直线距离
	routeDistances := server.getRouteDistancesByMerchant(ctx, dishesToMerchantLocs(dishes), resolvedLat, resolvedLng)

	response := make([]searchDishResponse, len(dishes))
	for i, dish := range dishes {
		distanceMeters := int(dish.Distance)
		if dist, ok := routeDistances[dish.MerchantID]; ok && dist > 0 {
			distanceMeters = dist
		}

		response[i] = newSearchDishResponseFromGlobalRow(dish, distanceMeters)

		// 使用路网距离计算配送费
		feeResult, err := server.calculateDeliveryFeeInternal(ctx, dish.MerchantRegionID, dish.MerchantID, int32(distanceMeters), 0)
		if err != nil {
			log.Error().Err(err).
				Int64("region_id", dish.MerchantRegionID).
				Int64("merchant_id", dish.MerchantID).
				Msg("calculateDeliveryFeeInternal failed in searchDishes")
		} else if feeResult != nil && !feeResult.DeliverySuspended {
			response[i].EstimatedDeliveryFee = int(feeResult.FinalFee)

			log.Debug().
				Int64("dish_id", dish.ID).
				Int64("region_id", dish.MerchantRegionID).
				Int64("merchant_id", dish.MerchantID).
				Int("distance", distanceMeters).
				Int64("final_fee", feeResult.FinalFee).
				Msg("searchDishes fee calculation details")
		}
	}

	// 后台记录搜索历史 + 热词
	if req.Keyword != "" {
		authPayload, ok := ctx.Get(authorizationPayloadKey)
		if ok {
			if p, ok2 := authPayload.(*token.Payload); ok2 {
				server.recordSearchKeyword(p.UserID, req.Keyword, "dish")
			}
		}
	}

	ctx.JSON(http.StatusOK, searchDishListResponse{
		Dishes:   response,
		Total:    total,
		PageID:   req.PageID,
		PageSize: req.PageSize,
	})
}

// searchMerchants godoc
// @Summary 搜索商户
// @Description 根据关键词搜索商户，可传入用户位置计算距离和预估运费
// @Tags Search
// @Accept json
// @Produce json
// @Param keyword query string true "搜索关键词"
// @Param region_id query int false "区域ID"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Param user_latitude query number false "用户当前纬度（用于计算距离和运费）"
// @Param user_longitude query number false "用户当前经度（用于计算距离和运费）"
// @Success 200 {object} searchMerchantListResponse "搜索结果"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/search/merchants [get]
func (server *Server) searchMerchants(ctx *gin.Context) {
	var req searchMerchantsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	offset := pageOffset(req.PageID, req.PageSize)

	// 准备位置参数（用于排序）
	resolvedLat, resolvedLng := resolveUserLocation(ctx, req.UserLatitude, req.UserLongitude)
	var userLat, userLng float64
	if resolvedLat != nil && resolvedLng != nil {
		userLat = *resolvedLat
		userLng = *resolvedLng
	}

	merchantRegionID, err := resolveRegionID(ctx, server, req.RegionID, resolvedLat, resolvedLng)
	if err != nil {
		if isRegionUnavailableError(err) {
			ctx.JSON(http.StatusOK, searchMerchantListResponse{
				Merchants: []searchMerchantResponse{},
				PageID:    req.PageID,
				PageSize:  req.PageSize,
			})
			return
		}
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var response []searchMerchantResponse
	var total int64

	if req.TagID != nil {
		// 按菜系品类过滤搜索
		merchants, err := server.store.SearchMerchantsByTag(ctx, db.SearchMerchantsByTagParams{
			TagID:    *req.TagID,
			RegionID: merchantRegionID,
			UserLat:  userLat,
			UserLng:  userLng,
			Offset:   int32(offset),
			Limit:    req.PageSize,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		totalCount, err := server.store.CountSearchMerchantsByTag(ctx, db.CountSearchMerchantsByTagParams{
			TagID:    *req.TagID,
			RegionID: merchantRegionID,
		})
		if err != nil {
			totalCount = int64(len(merchants))
		}
		total = totalCount
		response = make([]searchMerchantResponse, len(merchants))
		repurchaseRates := make([]float64, len(merchants))
		orderCounts := make([]int32, len(merchants))
		for i, m := range merchants {
			response[i] = newSearchMerchantResponseFromTagRow(m)
			repurchaseRates[i] = m.AvgRepurchaseRate
			orderCounts[i] = m.TotalOrders
		}
		assignMerchantLabels(response, repurchaseRates, orderCounts)
	} else {
		// 普通关键词搜索
		merchants, err := server.store.SearchMerchants(ctx, db.SearchMerchantsParams{
			Offset:   int32(offset),
			Limit:    req.PageSize,
			Column3:  req.Keyword,
			Column4:  userLat,
			Column5:  userLng,
			RegionID: merchantRegionID,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		totalCount, err := server.store.CountSearchMerchants(ctx, db.CountSearchMerchantsParams{
			Column1:  pgtype.Text{String: req.Keyword, Valid: true},
			RegionID: merchantRegionID,
		})
		if err != nil {
			totalCount = int64(len(merchants))
		}
		total = totalCount
		response = make([]searchMerchantResponse, len(merchants))
		repurchaseRates := make([]float64, len(merchants))
		orderCounts := make([]int32, len(merchants))
		for i, merchant := range merchants {
			response[i] = newSearchMerchantResponseFromRow(merchant)
			repurchaseRates[i] = merchant.AvgRepurchaseRate
			orderCounts[i] = merchant.TotalOrders
		}
		assignMerchantLabels(response, repurchaseRates, orderCounts)
	}

	// 如果用户提供了位置，计算精确距离（展示用）和运费
	if resolvedLat != nil && resolvedLng != nil && server.mapClient != nil {
		server.calculateSearchMerchantDistancesAndFees(ctx, response, *resolvedLat, *resolvedLng)
	}

	// 后台记录搜索历史 + 热词
	if req.Keyword != "" {
		authPayload, ok := ctx.Get(authorizationPayloadKey)
		if ok {
			if p, ok2 := authPayload.(*token.Payload); ok2 {
				server.recordSearchKeyword(p.UserID, req.Keyword, "merchant")
			}
		}
	}

	ctx.JSON(http.StatusOK, searchMerchantListResponse{
		Merchants: response,
		Total:     total,
		PageID:    req.PageID,
		PageSize:  req.PageSize,
	})
}

// searchCombos godoc
// @Summary 搜索套餐
// @Description 搜索套餐，消费者端使用，只返回上架且商户状态正常的套餐
// @Tags Search
// @Accept json
// @Produce json
// @Param keyword query string false "搜索关键词"
// @Param region_id query int false "区域ID"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Param user_latitude query number false "用户当前纬度"
// @Param user_longitude query number false "用户当前经度"
// @Success 200 {object} searchComboListResponse "搜索结果"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/search/combos [get]
func (server *Server) searchCombos(ctx *gin.Context) {
	var req searchDishesRequest // Reuse same request struct as params are identical
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	offset := pageOffset(req.PageID, req.PageSize)

	// 准备位置参数
	resolvedLat, resolvedLng := resolveUserLocation(ctx, req.UserLatitude, req.UserLongitude)
	var userLat, userLng float64
	if resolvedLat != nil && resolvedLng != nil {
		userLat = *resolvedLat
		userLng = *resolvedLng
	}

	// 准备RegionID（全局搜索必须）
	comboRegionID, err := resolveRegionID(ctx, server, req.RegionID, resolvedLat, resolvedLng)
	if err != nil {
		if isRegionUnavailableError(err) {
			ctx.JSON(http.StatusOK, searchComboListResponse{
				Combos:   []searchComboResponse{},
				PageID:   req.PageID,
				PageSize: req.PageSize,
			})
			return
		}
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 执行搜索
	combos, err := server.store.SearchCombosGlobal(ctx, db.SearchCombosGlobalParams{
		Column1:  req.Keyword,
		Limit:    req.PageSize,
		Offset:   offset,
		Column4:  userLat,
		Column5:  userLng,
		RegionID: comboRegionID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取总数
	total, err := server.store.CountSearchCombosGlobal(ctx, db.CountSearchCombosGlobalParams{
		Column1:  req.Keyword,
		RegionID: comboRegionID,
	})
	if err != nil {
		total = int64(len(combos))
	}

	// 若用户提供位置，批量拉取路网距离覆盖直线距离
	routeDistances := server.getRouteDistancesByMerchant(ctx, combosToMerchantLocs(combos), resolvedLat, resolvedLng)

	response := make([]searchComboResponse, len(combos))
	for i, row := range combos {
		// Calculate Savings (using int64 cents)
		originalPrice := int64(row.OriginalPrice)
		comboPrice := int64(row.ComboPrice)
		savings := 0
		if originalPrice > comboPrice {
			savings = int((float64(originalPrice) - float64(comboPrice)) / float64(originalPrice) * 100)
		}

		// Prefer combo image; fall back to first dish image if missing
		imageURL := row.ImageUrl.String
		if imageURL == "" {
			imageURL = row.FallbackImageUrl.String
		}

		// Delivery Fee & Time Calculation
		var estimatedFee *int64
		deliveryTime := 1800 // Default 30 mins
		distanceMeters := int(row.Distance)
		if dist, ok := routeDistances[row.MerchantID]; ok && dist > 0 {
			distanceMeters = dist
		}
		if distanceMeters > 0 {
			// Time: 15 mins prep + travel (12km/h)
			travelTime := int(float64(distanceMeters) / 3.33)
			deliveryTime = 900 + travelTime

			// Fee: Use standard internal calculator logic
			feeResult, err := server.calculateDeliveryFeeInternal(ctx, row.MerchantRegionID, row.MerchantID, int32(distanceMeters), 0)
			if err != nil {
				// Log error but don't fail the request. Return nil fee.
				log.Error().Err(err).
					Int64("region_id", row.MerchantRegionID).
					Int64("merchant_id", row.MerchantID).
					Int("distance", distanceMeters).
					Msg("calculateDeliveryFeeInternal failed in searchCombos")
			} else if feeResult != nil && !feeResult.DeliverySuspended {
				log.Info().
					Int64("merchant_id", row.MerchantID).
					Int64("region_id", row.MerchantRegionID).
					Int64("final_fee", feeResult.FinalFee).
					Msg("Delivery fee calculated successfully") // DEBUG LOG
				estimatedFee = &feeResult.FinalFee
			} else {
				log.Warn().
					Int64("merchant_id", row.MerchantID).
					Bool("fee_result_nil", feeResult == nil).
					Interface("fee_result", feeResult).
					Msg("Delivery fee calculation returned no valid fee")
			}
		}

		response[i] = searchComboResponse{
			ID:                    row.ID,
			MerchantID:            row.MerchantID,
			Name:                  row.Name,
			Description:           row.Description.String,
			ImageURL:              normalizeUploadURLForClient(imageURL),
			OriginalPrice:         originalPrice,
			ComboPrice:            comboPrice,
			SavingsPercent:        savings,
			MonthlySales:          row.MonthlySales,
			MerchantName:          row.MerchantName,
			MerchantLogo:          normalizeUploadURLForClient(row.MerchantLogo.String),
			MerchantIsOpen:        row.MerchantIsOpen,
			Distance:              distanceMeters,
			EstimatedDeliveryFee:  estimatedFee,
			EstimatedDeliveryTime: deliveryTime,
		}

		if row.Tags != nil {
			_ = parseJSON(row.Tags, &response[i].Tags)
		}
	}

	ctx.JSON(http.StatusOK, searchComboListResponse{
		Combos:   response,
		Total:    total,
		PageID:   req.PageID,
		PageSize: req.PageSize,
	})
}

// Helper for simple Dish (Merchant Search)
func newSearchDishResponseFromDish(dish db.Dish) searchDishResponse {
	resp := searchDishResponse{
		ID:            dish.ID,
		MerchantID:    dish.MerchantID,
		Name:          dish.Name,
		Description:   dish.Description.String,
		ImageURL:      normalizeUploadURLForClient(dish.ImageUrl.String),
		Price:         int64(dish.Price),
		OriginalPrice: int64(dish.Price),
		IsAvailable:   dish.IsAvailable,
		IsOnline:      dish.IsOnline,
		SortOrder:     dish.SortOrder,
		MonthlySales:  dish.MonthlySales,
	}
	if dish.RepurchaseRate.Valid {
		val, _ := dish.RepurchaseRate.Float64Value()
		resp.RepurchaseRate = val.Float64
	}
	if dish.CategoryID.Valid {
		resp.CategoryID = &dish.CategoryID.Int64
	}
	if dish.MemberPrice.Valid {
		resp.MemberPrice = int64(dish.MemberPrice.Int64)
	}
	// Merchant info is empty for merchant-specific search
	return resp
}

// Helper for Global Search Row
func newSearchDishResponseFromGlobalRow(row db.SearchDishesGlobalRow, distanceMeters int) searchDishResponse {
	// Calculate estimated delivery time (PrepareTime + Distance/Speed)
	// Assume average speed 200m/min (12km/h) for simple estimation if real routing not available
	deliveryTimeSeconds := int(row.PrepareTime) * 60
	if distanceMeters > 0 {
		travelTimeSeconds := int(float64(distanceMeters) / 3.33) // 3.33 m/s = 12 km/h
		deliveryTimeSeconds += travelTimeSeconds
	}

	resp := searchDishResponse{
		ID:            row.ID,
		MerchantID:    row.MerchantID,
		Name:          row.Name,
		Description:   row.Description.String,
		ImageURL:      normalizeUploadURLForClient(row.ImageUrl.String),
		Price:         int64(row.Price),
		OriginalPrice: int64(row.Price),
		IsAvailable:   row.IsAvailable,
		IsOnline:      row.IsOnline,
		SortOrder:     row.SortOrder,
		MonthlySales:  row.MonthlySales,
		// Enriched Fields
		MerchantName:   row.MerchantName,
		MerchantLogo:   normalizeUploadURLForClient(row.MerchantLogo.String),
		MerchantIsOpen: &row.MerchantIsOpen,
		Distance:       distanceMeters,

		EstimatedDeliveryTime: deliveryTimeSeconds,
		EstimatedDeliveryFee:  0, // Caller overwrites with calculateDeliveryFeeInternal result; 0 = fee unknown (fallback)
	}

	if row.RepurchaseRate.Valid {
		val, _ := row.RepurchaseRate.Float64Value()
		resp.RepurchaseRate = val.Float64
	}
	if row.CategoryID.Valid {
		resp.CategoryID = &row.CategoryID.Int64
	}
	if row.MemberPrice.Valid {
		resp.MemberPrice = int64(row.MemberPrice.Int64)
	}

	// 解析 Tags 和 CustomizationGroups
	if row.Tags != nil {
		_ = parseJSON(row.Tags, &resp.Attributes)
	}
	if row.CustomizationGroups != nil {
		_ = parseJSON(row.CustomizationGroups, &resp.CustomizationGroups)
	}

	return resp
}

// assignMerchantLabels 对商户列表中复购率最高的标注"推荐"，销量最高（非推荐）的标注"热销"
func assignMerchantLabels(merchants []searchMerchantResponse, repurchaseRates []float64, totalOrders []int32) {
	if len(merchants) == 0 {
		return
	}
	// 找复购率最高的商户
	recommendIdx := -1
	maxRate := 0.0
	for i, r := range repurchaseRates {
		if r > maxRate {
			maxRate = r
			recommendIdx = i
		}
	}
	if recommendIdx >= 0 && maxRate > 0 {
		merchants[recommendIdx].Label = "推荐"
	}
	// 找销量最高的商户（排除推荐商户）
	hotIdx := -1
	maxOrders := int32(0)
	for i, o := range totalOrders {
		if i == recommendIdx {
			continue
		}
		if o > maxOrders {
			maxOrders = o
			hotIdx = i
		}
	}
	if hotIdx >= 0 && maxOrders > 0 {
		merchants[hotIdx].Label = "热销"
	}
}

func newSearchMerchantResponseFromTagRow(merchant db.SearchMerchantsByTagRow) searchMerchantResponse {
	resp := searchMerchantResponse{
		ID:          merchant.ID,
		Name:        merchant.Name,
		Description: merchant.Description.String,
		LogoURL:     normalizeUploadURLForClient(merchant.LogoUrl.String),
		Status:      merchant.Status,
		IsOpen:      merchant.IsOpen,
		RegionID:    merchant.RegionID,
		TotalOrders: merchant.TotalOrders,
		CreatedAt:   merchant.CreatedAt,
	}
	if cover := extractCoverImageFromStorefrontImages(merchant.StorefrontImages); cover != "" {
		resp.CoverImage = cover
	}
	if merchant.Tags != nil {
		if tagsBytes, ok := merchant.Tags.([]byte); ok {
			var tags []string
			if err := json.Unmarshal(tagsBytes, &tags); err == nil {
				resp.Tags = tags
			}
		} else if tagsStrs, ok := merchant.Tags.([]interface{}); ok {
			for _, t := range tagsStrs {
				if s, ok := t.(string); ok {
					resp.Tags = append(resp.Tags, s)
				}
			}
		}
	}
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
		LogoURL:     normalizeUploadURLForClient(merchant.LogoUrl.String),
		Status:      merchant.Status,
		IsOpen:      merchant.IsOpen,
		RegionID:    merchant.RegionID, // 添加区域ID用于运费计算
		TotalOrders: merchant.TotalOrders,
		CreatedAt:   merchant.CreatedAt,
	}
	if cover := extractCoverImageFromStorefrontImages(merchant.StorefrontImages); cover != "" {
		resp.CoverImage = cover
	}

	// 处理标签
	if merchant.Tags != nil {
		if tagsBytes, ok := merchant.Tags.([]byte); ok {
			var tags []string
			if err := json.Unmarshal(tagsBytes, &tags); err == nil {
				resp.Tags = tags
			}
		} else if tagsStrs, ok := merchant.Tags.([]interface{}); ok {
			// 如果 sqlc 返回了解构后的数组
			for _, t := range tagsStrs {
				if s, ok := t.(string); ok {
					resp.Tags = append(resp.Tags, s)
				}
			}
		}
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
	RegionID        *int64   `form:"region_id" binding:"omitempty,min=1"`            // 区域ID
	MinCapacity     *int16   `form:"min_capacity" binding:"omitempty,min=1,max=100"` // 最小容纳人数
	MaxCapacity     *int16   `form:"max_capacity" binding:"omitempty,min=1,max=100"` // 最大容纳人数
	MaxMinimumSpend *int64   `form:"max_minimum_spend" binding:"omitempty,min=0"`    // 最大低消（分）
	MinMinimumSpend *int64   `form:"min_minimum_spend" binding:"omitempty,min=0"`    // 最小低消（分）
	ReservationDate string   `form:"reservation_date" binding:"required"`            // 预订日期 YYYY-MM-DD
	ReservationTime string   `form:"reservation_time" binding:"required"`            // 预订时段 HH:MM
	TagID           *int64   `form:"tag_id" binding:"omitempty,min=1"`               // 菜系/标签ID
	PageID          int32    `form:"page_id" binding:"required,min=1"`               // 页码
	PageSize        int32    `form:"page_size" binding:"required,min=1,max=50"`      // 每页数量
	UserLatitude    *float64 `form:"user_latitude" binding:"omitempty"`              // 用户当前纬度
	UserLongitude   *float64 `form:"user_longitude" binding:"omitempty"`             // 用户当前经度
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
// @Param min_minimum_spend query int false "最小低消（分）"
// @Param max_minimum_spend query int false "最大低消（分）"
// @Param reservation_date query string true "预订日期（YYYY-MM-DD）"
// @Param reservation_time query string true "预订时段（HH:MM）"
// @Param tag_id query int false "菜系/标签ID"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Success 200 {object} searchRoomListResponse "搜索结果"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
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

	offset := pageOffset(req.PageID, req.PageSize)

	// 构建查询参数
	// 准备RegionID（必须）
	resolvedLat, resolvedLng := resolveUserLocation(ctx, req.UserLatitude, req.UserLongitude)
	regionID, err := resolveRegionID(ctx, server, req.RegionID, resolvedLat, resolvedLng)
	if err != nil {
		if isRegionUnavailableError(err) {
			ctx.JSON(http.StatusOK, searchRoomListResponse{
				Rooms:    []searchRoomResponse{},
				PageID:   req.PageID,
				PageSize: req.PageSize,
			})
			return
		}
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var minCapacity, maxCapacity pgtype.Int2
	if req.MinCapacity != nil {
		minCapacity = pgtype.Int2{Int16: *req.MinCapacity, Valid: true}
	}
	if req.MaxCapacity != nil {
		maxCapacity = pgtype.Int2{Int16: *req.MaxCapacity, Valid: true}
	}

	var minMinimumSpend, maxMinimumSpend pgtype.Int8
	if req.MinMinimumSpend != nil {
		minMinimumSpend = pgtype.Int8{Int64: *req.MinMinimumSpend, Valid: true}
	}
	if req.MaxMinimumSpend != nil {
		maxMinimumSpend = pgtype.Int8{Int64: *req.MaxMinimumSpend, Valid: true}
	}

	var rooms []searchRoomResponse
	var total int64

	// 如果指定了标签（菜系），使用按标签搜索
	if req.TagID != nil {
		searchResults, err := server.store.SearchRoomsByMerchantTag(ctx, db.SearchRoomsByMerchantTagParams{
			TagID:           *req.TagID,
			RegionID:        regionID.Int64,
			MinCapacity:     minCapacity,
			MaxCapacity:     maxCapacity,
			MinMinimumSpend: minMinimumSpend,
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
			rooms[i] = newSearchRoomResponseFromTag(r)
		}
		total = int64(len(searchResults))
	} else {
		// 不按标签搜索，使用带图片的查询
		searchResults, err := server.store.SearchRoomsWithImage(ctx, db.SearchRoomsWithImageParams{
			RegionID:        regionID.Int64,
			MinCapacity:     minCapacity,
			MaxCapacity:     maxCapacity,
			MinMinimumSpend: minMinimumSpend,
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
			RegionID:        regionID.Int64,
			MinCapacity:     minCapacity,
			MaxCapacity:     maxCapacity,
			MinMinimumSpend: minMinimumSpend,
			MaxMinimumSpend: maxMinimumSpend,
			ReservationDate: reservationDate,
			ReservationTime: reservationTime,
		})
		if err != nil {
			total = int64(len(rooms))
		}
	}

	ctx.JSON(http.StatusOK, searchRoomListResponse{
		Rooms:    rooms,
		Total:    total,
		PageID:   req.PageID,
		PageSize: req.PageSize,
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
	t, err := parseISODate(dateStr, "")
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

// getRouteDistancesByMerchant 使用路网距离批量计算商户到用户的距离（骑行模式）
func (server *Server) getRouteDistancesByMerchant(ctx *gin.Context, merchantLocs map[int64]maps.Location, userLat, userLng *float64) map[int64]int {
	if server.mapClient == nil || userLat == nil || userLng == nil || len(merchantLocs) == 0 {
		return nil
	}

	merchantIDs := make([]int64, 0, len(merchantLocs))
	locs := make([]maps.Location, 0, len(merchantLocs))
	for id, loc := range merchantLocs {
		if loc.Lat == 0 && loc.Lng == 0 {
			continue
		}
		merchantIDs = append(merchantIDs, id)
		locs = append(locs, loc)
	}

	if len(locs) == 0 {
		return nil
	}

	userLoc := []maps.Location{{Lat: *userLat, Lng: *userLng}}
	result, err := server.mapClient.GetDistanceMatrix(ctx, locs, userLoc, "bicycling")
	if err != nil {
		return nil
	}

	distances := make(map[int64]int, len(locs))
	for i, row := range result.Rows {
		if i >= len(merchantIDs) {
			break
		}
		if len(row.Elements) > 0 {
			distances[merchantIDs[i]] = row.Elements[0].Distance
		}
	}

	return distances
}

// dishesToMerchantLocs 提取菜品结果中的商户经纬度
func dishesToMerchantLocs(rows []db.SearchDishesGlobalRow) map[int64]maps.Location {
	locs := make(map[int64]maps.Location, len(rows))
	for _, r := range rows {
		if r.MerchantLatitude.Valid && r.MerchantLongitude.Valid {
			lat, _ := r.MerchantLatitude.Float64Value()
			lng, _ := r.MerchantLongitude.Float64Value()
			locs[r.MerchantID] = maps.Location{Lat: lat.Float64, Lng: lng.Float64}
		}
	}
	return locs
}

// combosToMerchantLocs 提取套餐结果中的商户经纬度
func combosToMerchantLocs(rows []db.SearchCombosGlobalRow) map[int64]maps.Location {
	locs := make(map[int64]maps.Location, len(rows))
	for _, r := range rows {
		if r.MerchantLatitude.Valid && r.MerchantLongitude.Valid {
			lat, _ := r.MerchantLatitude.Float64Value()
			lng, _ := r.MerchantLongitude.Float64Value()
			locs[r.MerchantID] = maps.Location{Lat: lat.Float64, Lng: lng.Float64}
		}
	}
	return locs
}

// =============================================================================
// 搜索历史 & 热门关键词 & 搜索建议 API
// =============================================================================

// ── 搜索历史 ─────────────────────────────────────────────────────────────────

type listSearchHistoryRequest struct {
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=50"`
}

// listSearchHistory godoc
// @Summary 获取搜索历史
// @Tags Search
// @Success 200 {object} searchHistoryListResponse "搜索历史列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/search/history [get]
func (server *Server) listSearchHistory(ctx *gin.Context) {
	var req listSearchHistoryRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	limit := req.Limit
	if limit == 0 {
		limit = 10
	}

	payload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	history, err := server.store.ListSearchHistory(ctx, db.ListSearchHistoryParams{
		UserID: payload.UserID,
		Limit:  limit,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, searchHistoryListResponse{History: history})
}

type deleteSearchHistoryRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// deleteSearchHistory godoc
// @Summary 删除单条搜索历史
// @Tags Search
// @Success 200 {object} successMessageResponse "删除成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/search/history/{id} [delete]
func (server *Server) deleteSearchHistory(ctx *gin.Context) {
	var req deleteSearchHistoryRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	payload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	if err := server.store.DeleteSearchHistory(ctx, db.DeleteSearchHistoryParams{
		ID:     req.ID,
		UserID: payload.UserID,
	}); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, successMessage("ok"))
}

// clearSearchHistory godoc
// @Summary 清除全部搜索历史
// @Tags Search
// @Success 200 {object} successMessageResponse "清除成功"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/search/history [delete]
func (server *Server) clearSearchHistory(ctx *gin.Context) {
	payload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	if err := server.store.ClearSearchHistory(ctx, payload.UserID); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, successMessage("ok"))
}

// ── 热门搜索 ─────────────────────────────────────────────────────────────────

type getPopularKeywordsRequest struct {
	Type  string `form:"type" binding:"omitempty,oneof=dish merchant combo room"`
	Limit int32  `form:"limit" binding:"omitempty,min=1,max=20"`
}

// getPopularKeywords godoc
// @Summary 获取热门搜索关键词
// @Tags Search
// @Success 200 {object} searchKeywordsListResponse "热门关键词列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/search/popular [get]
func (server *Server) getPopularKeywords(ctx *gin.Context) {
	var req getPopularKeywordsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	ktype := req.Type
	if ktype == "" {
		ktype = "dish"
	}
	limit := req.Limit
	if limit == 0 {
		limit = 10
	}

	keywords, err := server.store.GetPopularKeywords(ctx, db.GetPopularKeywordsParams{
		Type:  ktype,
		Limit: limit,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, searchKeywordsListResponse{Keywords: keywords})
}

// ── 搜索建议 ─────────────────────────────────────────────────────────────────

type getSearchSuggestionsRequest struct {
	Keyword string `form:"keyword" binding:"required,min=1,max=50"`
	Type    string `form:"type" binding:"omitempty,oneof=dish merchant"`
	Limit   int32  `form:"limit" binding:"omitempty,min=1,max=10"`
}

type searchSuggestionItem struct {
	Keyword string `json:"keyword"`
	Type    string `json:"type"`
}

// getSearchSuggestions godoc
// @Summary 实时搜索建议（前缀匹配）
// @Tags Search
// @Success 200 {object} searchSuggestionsListResponse "搜索建议列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Security BearerAuth
// @Router /v1/search/suggestions [get]
func (server *Server) getSearchSuggestions(ctx *gin.Context) {
	var req getSearchSuggestionsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	limit := req.Limit
	if limit == 0 {
		limit = 8
	}

	suggestions := []searchSuggestionItem{}

	// 从搜索历史中前缀匹配（个性化建议优先）
	payload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	history, _ := server.store.ListSearchHistory(ctx, db.ListSearchHistoryParams{
		UserID: payload.UserID,
		Limit:  50,
	})
	seen := make(map[string]bool)
	prefix := req.Keyword
	for _, h := range history {
		if len(suggestions) >= int(limit) {
			break
		}
		if len(h.Keyword) >= len(prefix) && h.Keyword[:len(prefix)] == prefix {
			if !seen[h.Keyword] {
				suggestions = append(suggestions, searchSuggestionItem{Keyword: h.Keyword, Type: h.Type})
				seen[h.Keyword] = true
			}
		}
	}

	// 从热门关键词补充
	if len(suggestions) < int(limit) {
		ktype := req.Type
		if ktype == "" {
			ktype = "dish"
		}
		popular, _ := server.store.GetPopularKeywords(ctx, db.GetPopularKeywordsParams{
			Type:  ktype,
			Limit: 20,
		})
		for _, p := range popular {
			if len(suggestions) >= int(limit) {
				break
			}
			if len(p.Keyword) >= len(prefix) && p.Keyword[:len(prefix)] == prefix {
				if !seen[p.Keyword] {
					suggestions = append(suggestions, searchSuggestionItem{Keyword: p.Keyword, Type: p.Type})
					seen[p.Keyword] = true
				}
			}
		}
	}

	ctx.JSON(http.StatusOK, searchSuggestionsListResponse{Suggestions: suggestions})
}

// ── 辅助：统一搜索入口（记录历史 + 热词）────────────────────────────────────

// recordSearchKeyword 在后台 goroutine 中记录搜索历史和更新热词计数，不阻塞请求
func (server *Server) recordSearchKeyword(userID int64, keyword, ktype string) {
	if keyword == "" {
		return
	}
	if gin.Mode() == gin.TestMode {
		return
	}
	go func() {
		ctx := context.Background()
		// 记录用户历史（upsert）
		_, _ = server.store.UpsertSearchHistory(ctx, db.UpsertSearchHistoryParams{
			UserID:  userID,
			Keyword: keyword,
			Type:    ktype,
		})
		// 更新热词计数
		_ = server.store.IncrementPopularKeyword(ctx, db.IncrementPopularKeywordParams{
			Keyword: keyword,
			Type:    ktype,
		})
	}()
}

// searchCategories godoc
// @Summary 获取区域活跃菜系品类
// @Description 按用户位置或区域ID，返回有商户覆盖的菜系品类列表及商户数量，用于首页品类网格动态渲染
// @Tags Search
// @Produce json
// @Param region_id query int false "区域ID（可选，优先于坐标）"
// @Param user_latitude query number false "用户当前纬度"
// @Param user_longitude query number false "用户当前经度"
// @Success 200 {object} searchCategoriesListResponse "品类列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/search/categories [get]
func (server *Server) searchCategories(ctx *gin.Context) {
	type searchCategoriesRequest struct {
		RegionID      *int64   `form:"region_id" binding:"omitempty,min=1"`
		UserLatitude  *float64 `form:"user_latitude" binding:"omitempty"`
		UserLongitude *float64 `form:"user_longitude" binding:"omitempty"`
	}

	var req searchCategoriesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	resolvedLat, resolvedLng := resolveUserLocation(ctx, req.UserLatitude, req.UserLongitude)
	regionID, err := resolveRegionID(ctx, server, req.RegionID, resolvedLat, resolvedLng)
	if err != nil {
		if isRegionUnavailableError(err) {
			ctx.JSON(http.StatusOK, searchCategoriesListResponse{Categories: []searchCategoryItem{}})
			return
		}
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	id := regionID.Int64

	categories, err := server.store.GetActiveCategoriesByRegion(ctx, id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]searchCategoryItem, len(categories))
	for i, c := range categories {
		result[i] = searchCategoryItem{
			ID:            c.ID,
			Name:          c.Name,
			MerchantCount: c.MerchantCount,
		}
	}

	ctx.JSON(http.StatusOK, searchCategoriesListResponse{
		Categories: result,
	})
}

// extractCoverImageFromStorefrontImages 从 merchant_applications.storefront_images JSONB 字段提取第一张门头照 URL。
func extractCoverImageFromStorefrontImages(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var images []string
	if err := json.Unmarshal(data, &images); err != nil || len(images) == 0 {
		return ""
	}
	return normalizeUploadURLForClient(images[0])
}
