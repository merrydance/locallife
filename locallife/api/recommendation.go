package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/token"
	"github.com/rs/zerolog/log"
)

// numericToFloat64 将pgtype.Numeric转换为float64
func numericToFloat64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	val, _ := n.Float64Value()
	return val.Float64
}

// ==================== 行为埋点 ====================

type trackBehaviorRequest struct {
	BehaviorType string `json:"behavior_type" binding:"required,oneof=view detail cart purchase"`
	DishID       *int64 `json:"dish_id" binding:"omitempty,min=1"`
	ComboID      *int64 `json:"combo_id" binding:"omitempty,min=1"`
	MerchantID   *int64 `json:"merchant_id" binding:"omitempty,min=1"`
	Duration     *int32 `json:"duration" binding:"omitempty,min=0,max=86400"` // 停留时长（秒），最大24小时
}

// trackBehavior godoc
// @Summary 上报用户行为埋点
// @Description 记录用户浏览、详情、加购、购买等行为数据，用于个性化推荐
// @Tags 推荐引擎
// @Accept json
// @Produce json
// @Param request body trackBehaviorRequest true "行为数据"
// @Success 200 {object} map[string]interface{} "记录成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/behaviors/track [post]
func (server *Server) trackBehavior(ctx *gin.Context) {
	var req trackBehaviorRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证至少有一个关联对象
	if req.DishID == nil && req.ComboID == nil && req.MerchantID == nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("must provide at least one of: dish_id, combo_id, merchant_id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 记录行为
	var dishID, comboID, merchantID pgtype.Int8
	if req.DishID != nil {
		dishID = pgtype.Int8{Int64: *req.DishID, Valid: true}
	}
	if req.ComboID != nil {
		comboID = pgtype.Int8{Int64: *req.ComboID, Valid: true}
	}
	if req.MerchantID != nil {
		merchantID = pgtype.Int8{Int64: *req.MerchantID, Valid: true}
	}

	var duration pgtype.Int4
	if req.Duration != nil {
		duration = pgtype.Int4{Int32: *req.Duration, Valid: true}
	}

	behavior, err := server.store.TrackBehavior(ctx, db.TrackBehaviorParams{
		UserID:       authPayload.UserID,
		BehaviorType: req.BehaviorType,
		DishID:       dishID,
		ComboID:      comboID,
		MerchantID:   merchantID,
		Duration:     duration,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"id":         behavior.ID,
		"tracked_at": behavior.CreatedAt,
	})
}

// ==================== 推荐接口 ====================

type recommendRequest struct {
	Limit         int32    `form:"limit" binding:"omitempty,min=1,max=50"`
	UserLatitude  *float64 `form:"user_latitude" binding:"omitempty"`  // 用户当前纬度
	UserLongitude *float64 `form:"user_longitude" binding:"omitempty"` // 用户当前经度
}

type recommendDishesResponse struct {
	Dishes    []dishSummary `json:"dishes"`
	Algorithm string        `json:"algorithm"`
	ExpiredAt string        `json:"expired_at"`
}

type recommendDishesAPIResponse struct {
	Code    int                     `json:"code" example:"0"`
	Message string                  `json:"message" example:"ok"`
	Data    recommendDishesResponse `json:"data"`
}

type dishSummary struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Price       int64   `json:"price"`        // 原价（分）
	MemberPrice *int64  `json:"member_price"` // 会员价（分），null表示无会员价
	ImageURL    *string `json:"image_url,omitempty"`
	MerchantID  int64   `json:"merchant_id"`
	IsAvailable bool    `json:"is_available"`

	// 商户信息
	MerchantName      string  `json:"merchant_name"`
	MerchantLogo      *string `json:"merchant_logo,omitempty"`
	MerchantLatitude  float64 `json:"merchant_latitude"`
	MerchantLongitude float64 `json:"merchant_longitude"`
	MerchantRegionID  int64   `json:"merchant_region_id"` // 用于运费计算

	// 销量与标签
	MonthlySales int32    `json:"monthly_sales"` // 近30天销量
	Tags         []string `json:"tags"`          // 菜品标签

	// 距离与运费（需要用户位置）
	Distance              *int   `json:"distance,omitempty"`                // 距离（米）
	EstimatedDeliveryTime *int   `json:"estimated_delivery_time,omitempty"` // 预估配送时间（秒）
	EstimatedDeliveryFee  *int64 `json:"estimated_delivery_fee,omitempty"`  // 预估配送费（分）
}

// recommendDishes godoc
// @Summary 推荐菜品
// @Description 基于用户行为和偏好，使用EE算法推荐个性化菜品列表。返回完整的菜品信息包括月销量、会员价、标签、商户信息、距离和运费估算。
// @Tags 推荐引擎
// @Accept json
// @Produce json
// @Param limit query int false "返回数量" default(20) minimum(1) maximum(50)
// @Param user_latitude query number false "用户当前纬度（用于计算距离和运费）"
// @Param user_longitude query number false "用户当前经度（用于计算距离和运费）"
// @Success 200 {object} recommendDishesAPIResponse "推荐菜品列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/recommendations/dishes [get]
func (server *Server) recommendDishes(ctx *gin.Context) {
	var req recommendRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 默认推荐20个
	if req.Limit == 0 {
		req.Limit = 20
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 生成新的推荐
	recommender := algorithm.NewPersonalizedRecommender(server.store)
	config := algorithm.DefaultPersonalizedConfig()

	// 获取用户偏好判断是否为新用户
	preferences, err := server.store.GetUserPreferences(ctx, authPayload.UserID)
	if err == nil && preferences.PurchaseFrequency < 3 {
		config = algorithm.NewUserPersonalizedConfig() // 新用户使用不同配置
	}

	// 生成推荐
	dishIDs, err := recommender.RecommendDishes(ctx, authPayload.UserID, config, int(req.Limit))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 保存推荐结果到数据库（5分钟过期）
	expiredAt := time.Now().Add(5 * time.Minute)
	_, _ = server.store.SaveRecommendations(ctx, db.SaveRecommendationsParams{
		UserID:    authPayload.UserID,
		DishIds:   dishIDs,
		Algorithm: "ee-algorithm",
		ExpiredAt: expiredAt,
	})

	// 使用新的丰富查询获取菜品详情
	dishes := make([]dishSummary, 0)
	if len(dishIDs) > 0 {
		dishRows, err := server.store.GetDishesWithMerchantByIDs(ctx, dishIDs)
		if err == nil {
			dishes = make([]dishSummary, len(dishRows))
			for i, d := range dishRows {
				var imgURL, merchantLogo *string
				if d.ImageUrl.Valid {
					img := normalizeUploadURLForClient(d.ImageUrl.String)
					imgURL = &img
				}
				if d.MerchantLogo.Valid {
					logo := normalizeUploadURLForClient(d.MerchantLogo.String)
					merchantLogo = &logo
				}

				var memberPrice *int64
				if d.MemberPrice.Valid {
					memberPrice = &d.MemberPrice.Int64
				}

				// 获取菜品标签
				tags := []string{}
				dishTags, err := server.store.ListDishTags(ctx, d.ID)
				if err == nil {
					for _, t := range dishTags {
						tags = append(tags, t.Name)
					}
				}

				dishes[i] = dishSummary{
					ID:                d.ID,
					Name:              d.Name,
					Price:             d.Price,
					MemberPrice:       memberPrice,
					ImageURL:          imgURL,
					MerchantID:        d.MerchantID,
					IsAvailable:       d.IsAvailable,
					MerchantName:      d.MerchantName,
					MerchantLogo:      merchantLogo,
					MerchantLatitude:  numericToFloat64(d.MerchantLatitude),
					MerchantLongitude: numericToFloat64(d.MerchantLongitude),
					MerchantRegionID:  d.MerchantRegionID,
					MonthlySales:      d.MonthlySales,
					Tags:              tags,
				}
			}

			// 如果用户提供了位置，计算距离和运费
			if req.UserLatitude != nil && req.UserLongitude != nil && server.mapClient != nil {
				server.calculateDishDistancesAndFees(ctx, dishes, *req.UserLatitude, *req.UserLongitude)
			}
		}
	}

	ctx.JSON(http.StatusOK, recommendDishesAPIResponse{
		Code:    0,
		Message: "ok",
		Data: recommendDishesResponse{
			Dishes:    dishes,
			Algorithm: "ee-algorithm",
			ExpiredAt: expiredAt.Format(time.RFC3339),
		},
	})
}

type comboSummary struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	OriginalPrice  int64   `json:"original_price"`  // 原价（分）
	ComboPrice     int64   `json:"combo_price"`     // 套餐价（分）
	SavingsPercent float64 `json:"savings_percent"` // 优惠百分比
	ImageURL       *string `json:"image_url,omitempty"`
	MerchantID     int64   `json:"merchant_id"`

	// 商户信息
	MerchantName      string  `json:"merchant_name"`
	MerchantLogo      *string `json:"merchant_logo,omitempty"`
	MerchantLatitude  float64 `json:"merchant_latitude"`
	MerchantLongitude float64 `json:"merchant_longitude"`
	MerchantRegionID  int64   `json:"merchant_region_id"`

	// 销量与标签
	MonthlySales int32    `json:"monthly_sales"` // 近30天销量
	Tags         []string `json:"tags"`          // 套餐标签

	// 距离与运费（需要用户位置）
	Distance             *int   `json:"distance,omitempty"`               // 距离（米）
	EstimatedDeliveryFee *int64 `json:"estimated_delivery_fee,omitempty"` // 预估配送费（分）
}

type recommendCombosResponse struct {
	Combos    []comboSummary `json:"combos"`
	Algorithm string         `json:"algorithm"`
	ExpiredAt string         `json:"expired_at"`
}

// recommendCombos godoc
// @Summary 推荐套餐
// @Description 基于用户行为和偏好，使用EE算法推荐个性化套餐列表。返回完整的套餐信息包括月销量、优惠百分比、标签、商户信息、距离和运费估算。
// @Tags 推荐引擎
// @Accept json
// @Produce json
// @Param limit query int false "返回数量" default(10) minimum(1) maximum(50)
// @Param user_latitude query number false "用户当前纬度（用于计算距离和运费）"
// @Param user_longitude query number false "用户当前经度（用于计算距离和运费）"
// @Success 200 {object} recommendCombosResponse "推荐套餐列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/recommendations/combos [get]
func (server *Server) recommendCombos(ctx *gin.Context) {
	var req recommendRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.Limit == 0 {
		req.Limit = 10
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 生成套餐推荐
	recommender := algorithm.NewPersonalizedRecommender(server.store)
	config := algorithm.DefaultPersonalizedConfig()
	comboIDs, err := recommender.RecommendCombos(ctx, authPayload.UserID, config, int(req.Limit))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 批量查询套餐详情（使用新的丰富查询）
	combos := make([]comboSummary, 0)
	if len(comboIDs) > 0 {
		comboRows, err := server.store.GetCombosWithMerchantByIDs(ctx, comboIDs)
		if err == nil {
			combos = make([]comboSummary, len(comboRows))
			for i, c := range comboRows {
				var imgURL, merchantLogo *string
				if c.ImageUrl.Valid {
					img := normalizeUploadURLForClient(c.ImageUrl.String)
					imgURL = &img
				}
				if c.MerchantLogo.Valid {
					logo := normalizeUploadURLForClient(c.MerchantLogo.String)
					merchantLogo = &logo
				}

				// 计算优惠百分比
				var savingsPercent float64
				if c.OriginalPrice > 0 {
					savingsPercent = float64(c.OriginalPrice-c.ComboPrice) / float64(c.OriginalPrice) * 100
				}

				// 获取套餐标签
				tags := []string{}
				comboTags, err := server.store.ListComboTags(ctx, c.ID)
				if err == nil {
					for _, t := range comboTags {
						tags = append(tags, t.Name)
					}
				}

				combos[i] = comboSummary{
					ID:                c.ID,
					Name:              c.Name,
					OriginalPrice:     c.OriginalPrice,
					ComboPrice:        c.ComboPrice,
					SavingsPercent:    savingsPercent,
					ImageURL:          imgURL,
					MerchantID:        c.MerchantID,
					MerchantName:      c.MerchantName,
					MerchantLogo:      merchantLogo,
					MerchantLatitude:  numericToFloat64(c.MerchantLatitude),
					MerchantLongitude: numericToFloat64(c.MerchantLongitude),
					MerchantRegionID:  c.MerchantRegionID,
					MonthlySales:      c.MonthlySales,
					Tags:              tags,
				}
			}

			// 如果用户提供了位置，计算距离和运费
			if req.UserLatitude != nil && req.UserLongitude != nil && server.mapClient != nil {
				server.calculateComboDistancesAndFees(ctx, combos, *req.UserLatitude, *req.UserLongitude)
			}
		}
	}

	ctx.JSON(http.StatusOK, recommendCombosResponse{
		Combos:    combos,
		Algorithm: "ee-algorithm",
		ExpiredAt: time.Now().Add(5 * time.Minute).Format(time.RFC3339),
	})
}

type merchantSummary struct {
	ID                   int64    `json:"id"`
	Name                 string   `json:"name"`
	Description          *string  `json:"description,omitempty"`
	LogoURL              *string  `json:"logo_url,omitempty"`
	Address              string   `json:"address"`
	Latitude             float64  `json:"latitude"`
	Longitude            float64  `json:"longitude"`
	RegionID             int64    `json:"region_id"`                        // 区域ID，用于运费计算
	MonthlySales         int32    `json:"monthly_sales"`                    // 近30天订单量
	Tags                 []string `json:"tags"`                             // 商户标签
	Distance             *int     `json:"distance,omitempty"`               // 距离（米），需要传入用户位置
	EstimatedDeliveryFee *int64   `json:"estimated_delivery_fee,omitempty"` // 预估配送费（分），需要传入用户位置
}

type recommendMerchantsResponse struct {
	Merchants []merchantSummary `json:"merchants"`
	Algorithm string            `json:"algorithm"`
	ExpiredAt string            `json:"expired_at"`
}

// recommendMerchants godoc
// @Summary 推荐商户
// @Description 基于用户行为和偏好，使用EE算法推荐个性化商户列表。返回完整的商户信息包括月订单量、标签、距离和运费估算。
// @Tags 推荐引擎
// @Accept json
// @Produce json
// @Param limit query int false "返回数量" default(10) minimum(1) maximum(50)
// @Param user_latitude query number false "用户当前纬度（用于计算距离和运费）"
// @Param user_longitude query number false "用户当前经度（用于计算距离和运费）"
// @Success 200 {object} recommendMerchantsResponse "推荐商户列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/recommendations/merchants [get]
func (server *Server) recommendMerchants(ctx *gin.Context) {
	var req recommendRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.Limit == 0 {
		req.Limit = 10
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 生成商户推荐
	recommender := algorithm.NewPersonalizedRecommender(server.store)
	config := algorithm.DefaultPersonalizedConfig()
	merchantIDs, err := recommender.RecommendMerchants(ctx, authPayload.UserID, config, int(req.Limit))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 批量查询商户详情（使用新的丰富查询）
	merchants := make([]merchantSummary, 0)
	if len(merchantIDs) > 0 {
		merchantRows, err := server.store.GetMerchantsWithStatsByIDs(ctx, merchantIDs)
		if err == nil {
			merchants = make([]merchantSummary, len(merchantRows))
			for i, m := range merchantRows {
				var desc, logoURL *string
				if m.Description.Valid {
					desc = &m.Description.String
				}
				if m.LogoUrl.Valid {
					logo := normalizeUploadURLForClient(m.LogoUrl.String)
					logoURL = &logo
				}

				// 获取商户标签
				tags := []string{}
				merchantTags, err := server.store.ListMerchantTags(ctx, m.ID)
				if err == nil {
					for _, t := range merchantTags {
						tags = append(tags, t.Name)
					}
				}

				merchants[i] = merchantSummary{
					ID:           m.ID,
					Name:         m.Name,
					Description:  desc,
					LogoURL:      logoURL,
					Address:      m.Address,
					Latitude:     numericToFloat64(m.Latitude),
					Longitude:    numericToFloat64(m.Longitude),
					RegionID:     m.RegionID,
					MonthlySales: m.MonthlyOrders,
					Tags:         tags,
				}
			}

			// 如果用户提供了位置，计算距离和运费
			if req.UserLatitude != nil && req.UserLongitude != nil && server.mapClient != nil {
				server.calculateMerchantDistancesAndFees(ctx, merchants, *req.UserLatitude, *req.UserLongitude)
			}
		}
	}

	ctx.JSON(http.StatusOK, recommendMerchantsResponse{
		Merchants: merchants,
		Algorithm: "ee-algorithm",
		ExpiredAt: time.Now().Add(5 * time.Minute).Format(time.RFC3339),
	})
}

// ==================== 推荐配置管理（运营商）====================

type getRecommendationConfigResponse struct {
	RegionID          int64   `json:"region_id"`
	ExploitationRatio float64 `json:"exploitation_ratio"`
	ExplorationRatio  float64 `json:"exploration_ratio"`
	RandomRatio       float64 `json:"random_ratio"`
	AutoAdjust        bool    `json:"auto_adjust"`
	UpdatedAt         string  `json:"updated_at"`
}

type updateRecommendationConfigRequest struct {
	ExploitationRatio *float64 `json:"exploitation_ratio" binding:"omitempty,min=0,max=1"`
	ExplorationRatio  *float64 `json:"exploration_ratio" binding:"omitempty,min=0,max=1"`
	RandomRatio       *float64 `json:"random_ratio" binding:"omitempty,min=0,max=1"`
	AutoAdjust        *bool    `json:"auto_adjust"`
}

// getRecommendationConfig godoc
// @Summary 获取区域推荐配置
// @Description 获取指定区域的EE算法推荐配置，如果不存在则返回默认配置
// @Tags 推荐配置管理
// @Accept json
// @Produce json
// @Param id path int true "区域ID"
// @Success 200 {object} getRecommendationConfigResponse "推荐配置"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限（非该区域运营商）"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/regions/{id}/recommendation-config [get]
func (server *Server) getRecommendationConfig(ctx *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 查询配置（如果不存在则返回默认值）
	config, err := server.store.GetRecommendationConfig(ctx, uri.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// 返回默认配置
			ctx.JSON(http.StatusOK, getRecommendationConfigResponse{
				RegionID:          uri.ID,
				ExploitationRatio: 0.60,
				ExplorationRatio:  0.30,
				RandomRatio:       0.10,
				AutoAdjust:        false,
				UpdatedAt:         time.Now().Format(time.RFC3339),
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, getRecommendationConfigResponse{
		RegionID:          config.RegionID,
		ExploitationRatio: numericToFloat64(config.ExploitationRatio),
		ExplorationRatio:  numericToFloat64(config.ExplorationRatio),
		RandomRatio:       numericToFloat64(config.RandomRatio),
		AutoAdjust:        config.AutoAdjust,
		UpdatedAt:         config.UpdatedAt.Format(time.RFC3339),
	})
}

// updateRecommendationConfig godoc
// @Summary 更新区域推荐配置
// @Description 更新指定区域的EE算法推荐配置，比例之和必须等于1.0
// @Tags 推荐配置管理
// @Accept json
// @Produce json
// @Param id path int true "区域ID"
// @Param request body updateRecommendationConfigRequest true "配置更新"
// @Success 200 {object} getRecommendationConfigResponse "更新后的配置"
// @Failure 400 {object} ErrorResponse "参数错误（如比例之和不为1）"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限（非该区域运营商）"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/regions/{id}/recommendation-config [patch]
func (server *Server) updateRecommendationConfig(ctx *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateRecommendationConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证比例之和是否为1
	if req.ExploitationRatio != nil && req.ExplorationRatio != nil && req.RandomRatio != nil {
		sum := *req.ExploitationRatio + *req.ExplorationRatio + *req.RandomRatio
		if sum < 0.99 || sum > 1.01 { // 允许浮点误差
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("sum of ratios must equal 1.0")))
			return
		}
	}

	// 获取现有配置或使用默认值
	existingConfig, err := server.store.GetRecommendationConfig(ctx, uri.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 应用更新
	exploitationRatio := 0.60
	explorationRatio := 0.30
	randomRatio := 0.10
	autoAdjust := false

	if err == nil {
		// 使用现有配置
		exploitationRatio = numericToFloat64(existingConfig.ExploitationRatio)
		explorationRatio = numericToFloat64(existingConfig.ExplorationRatio)
		randomRatio = numericToFloat64(existingConfig.RandomRatio)
		autoAdjust = existingConfig.AutoAdjust
	}

	if req.ExploitationRatio != nil {
		exploitationRatio = *req.ExploitationRatio
	}
	if req.ExplorationRatio != nil {
		explorationRatio = *req.ExplorationRatio
	}
	if req.RandomRatio != nil {
		randomRatio = *req.RandomRatio
	}
	if req.AutoAdjust != nil {
		autoAdjust = *req.AutoAdjust
	}

	// Upsert配置
	config, err := server.store.UpsertRecommendationConfig(ctx, db.UpsertRecommendationConfigParams{
		RegionID:          uri.ID,
		ExploitationRatio: numericFromFloat(exploitationRatio),
		ExplorationRatio:  numericFromFloat(explorationRatio),
		RandomRatio:       numericFromFloat(randomRatio),
		AutoAdjust:        autoAdjust,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, getRecommendationConfigResponse{
		RegionID:          config.RegionID,
		ExploitationRatio: numericToFloat64(config.ExploitationRatio),
		ExplorationRatio:  numericToFloat64(config.ExplorationRatio),
		RandomRatio:       numericToFloat64(config.RandomRatio),
		AutoAdjust:        config.AutoAdjust,
		UpdatedAt:         config.UpdatedAt.Format(time.RFC3339),
	})
}

// ==================== 包间探索 ====================

type exploreRoomsRequest struct {
	RegionID        *int64   `form:"region_id" binding:"omitempty,min=1"`            // 区域ID
	MinCapacity     *int16   `form:"min_capacity" binding:"omitempty,min=1,max=100"` // 最小容纳人数
	MaxCapacity     *int16   `form:"max_capacity" binding:"omitempty,min=1,max=100"` // 最大容纳人数
	MaxMinimumSpend *int64   `form:"max_minimum_spend" binding:"omitempty,min=0"`    // 最大低消（分）
	PageID          int32    `form:"page_id" binding:"required,min=1"`               // 页码
	PageSize        int32    `form:"page_size" binding:"required,min=1,max=50"`      // 每页数量
	UserLatitude    *float64 `form:"user_latitude" binding:"omitempty"`              // 用户当前纬度
	UserLongitude   *float64 `form:"user_longitude" binding:"omitempty"`             // 用户当前经度
}

type exploreRoomItem struct {
	ID                   int64   `json:"id"`
	MerchantID           int64   `json:"merchant_id"`
	TableNo              string  `json:"table_no"`
	Capacity             int16   `json:"capacity"`
	Description          *string `json:"description,omitempty"`
	MinimumSpend         *int64  `json:"minimum_spend,omitempty"` // 分
	Status               string  `json:"status"`
	MerchantName         string  `json:"merchant_name"`
	MerchantLogo         *string `json:"merchant_logo,omitempty"`
	MerchantAddress      string  `json:"merchant_address"`
	MerchantPhone        *string `json:"merchant_phone,omitempty"`
	MerchantLatitude     float64 `json:"merchant_latitude"`
	MerchantLongitude    float64 `json:"merchant_longitude"`
	PrimaryImage         *string `json:"primary_image,omitempty"`
	MonthlyReservations  int     `json:"monthly_reservations"`             // 近30天预订量
	Distance             *int    `json:"distance,omitempty"`               // 距离（米），需要传入用户位置
	EstimatedDeliveryFee *int64  `json:"estimated_delivery_fee,omitempty"` // 预估配送费（分），需要传入用户位置
}

type exploreRoomsResponse struct {
	Rooms    []exploreRoomItem `json:"rooms"`
	Total    int64             `json:"total"`
	PageID   int32             `json:"page_id"`
	PageSize int32             `json:"page_size"`
}

// exploreRooms godoc
// @Summary 探索附近包间
// @Description 浏览本地区域的可用包间，无需指定预订日期时段。支持按区域、人数、低消等筛选。返回包间主图和近30天预订量。
// @Tags 推荐引擎
// @Accept json
// @Produce json
// @Param region_id query int false "区域ID"
// @Param min_capacity query int false "最小容纳人数"
// @Param max_capacity query int false "最大容纳人数"
// @Param max_minimum_spend query int false "最大低消（分）"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(1) maximum(50)
// @Param user_latitude query number false "用户当前纬度（用于计算距离）"
// @Param user_longitude query number false "用户当前经度（用于计算距离）"
// @Success 200 {object} exploreRoomsResponse "包间列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/recommendations/rooms [get]
func (server *Server) exploreRooms(ctx *gin.Context) {
	var req exploreRoomsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
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

	// 查询包间
	rows, err := server.store.ExploreNearbyRooms(ctx, db.ExploreNearbyRoomsParams{
		RegionID:        regionID,
		MinCapacity:     minCapacity,
		MaxCapacity:     maxCapacity,
		MaxMinimumSpend: maxMinimumSpend,
		PageOffset:      offset,
		PageSize:        req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询总数
	total, err := server.store.CountExploreNearbyRooms(ctx, db.CountExploreNearbyRoomsParams{
		RegionID:        regionID,
		MinCapacity:     minCapacity,
		MaxCapacity:     maxCapacity,
		MaxMinimumSpend: maxMinimumSpend,
	})
	if err != nil {
		total = int64(len(rows))
	}

	// 转换响应
	rooms := make([]exploreRoomItem, len(rows))
	for i, r := range rows {
		rooms[i] = exploreRoomItem{
			ID:                  r.ID,
			MerchantID:          r.MerchantID,
			TableNo:             r.TableNo,
			Capacity:            r.Capacity,
			Status:              r.Status,
			MerchantName:        r.MerchantName,
			MerchantAddress:     r.MerchantAddress,
			MonthlyReservations: int(r.MonthlyReservations),
		}

		if r.Description.Valid {
			rooms[i].Description = &r.Description.String
		}
		if r.MinimumSpend.Valid {
			rooms[i].MinimumSpend = &r.MinimumSpend.Int64
		}
		if r.MerchantLogo.Valid {
			logo := normalizeUploadURLForClient(r.MerchantLogo.String)
			rooms[i].MerchantLogo = &logo
		}
		if r.MerchantPhone != "" {
			phone := r.MerchantPhone
			rooms[i].MerchantPhone = &phone
		}
		if r.PrimaryImage != "" {
			primaryImage := normalizeUploadURLForClient(r.PrimaryImage)
			rooms[i].PrimaryImage = &primaryImage
		}
		if r.MerchantLatitude.Valid {
			lat, _ := r.MerchantLatitude.Float64Value()
			rooms[i].MerchantLatitude = lat.Float64
		}
		if r.MerchantLongitude.Valid {
			lng, _ := r.MerchantLongitude.Float64Value()
			rooms[i].MerchantLongitude = lng.Float64
		}
	}

	// 如果用户提供了位置，计算距离
	if req.UserLatitude != nil && req.UserLongitude != nil && server.mapClient != nil {
		server.calculateRoomDistances(ctx, rooms, *req.UserLatitude, *req.UserLongitude)
	}

	ctx.JSON(http.StatusOK, exploreRoomsResponse{
		Rooms:    rooms,
		Total:    total,
		PageID:   req.PageID,
		PageSize: req.PageSize,
	})
}

// ==================== 距离和运费计算辅助函数 ====================

// calculateDishDistancesAndFees 批量计算菜品商户到用户的距离和预估运费
func (server *Server) calculateDishDistancesAndFees(ctx *gin.Context, dishes []dishSummary, userLat, userLng float64) {
	if len(dishes) == 0 || server.mapClient == nil {
		return
	}

	// 构建商户位置列表（去重）
	merchantDistances := make(map[int64]int)   // merchantID -> distance
	merchantDurations := make(map[int64]int)   // merchantID -> duration (seconds)
	merchantRegionIDs := make(map[int64]int64) // merchantID -> regionID
	var merchantLocs []maps.Location
	var merchantIDs []int64

	for _, d := range dishes {
		if _, exists := merchantDistances[d.MerchantID]; !exists && (d.MerchantLatitude != 0 || d.MerchantLongitude != 0) {
			merchantLocs = append(merchantLocs, maps.Location{
				Lat: d.MerchantLatitude,
				Lng: d.MerchantLongitude,
			})
			merchantIDs = append(merchantIDs, d.MerchantID)
			merchantDistances[d.MerchantID] = -1 // 标记为待计算
			merchantDurations[d.MerchantID] = -1
			merchantRegionIDs[d.MerchantID] = d.MerchantRegionID
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
		log.Warn().Err(err).Msg("failed to calculate dish distances via Tencent Map API")
		return
	}

	// 更新商户距离和时间映射
	for i, row := range result.Rows {
		if i >= len(merchantIDs) {
			break
		}
		if len(row.Elements) > 0 {
			merchantDistances[merchantIDs[i]] = row.Elements[0].Distance
			merchantDurations[merchantIDs[i]] = row.Elements[0].Duration
		}
	}

	// 为每个菜品计算距离、时间和运费（使用菜品实际价格）
	for i := range dishes {
		dist, ok := merchantDistances[dishes[i].MerchantID]
		if !ok || dist < 0 {
			continue
		}
		dishes[i].Distance = &dist

		// 设置配送时间
		if dur, ok := merchantDurations[dishes[i].MerchantID]; ok && dur > 0 {
			dishes[i].EstimatedDeliveryTime = &dur
		}

		// 使用菜品的实际价格计算运费
		// 如果有会员价则使用会员价，否则使用原价
		// 价格为0时只是跳过货值加价，其他系数（时段、天气、距离）仍正常计算
		orderAmount := dishes[i].Price
		if dishes[i].MemberPrice != nil && *dishes[i].MemberPrice > 0 {
			orderAmount = *dishes[i].MemberPrice
		}

		regionID := merchantRegionIDs[dishes[i].MerchantID]
		feeResult, err := server.calculateDeliveryFeeInternal(ctx, regionID, dishes[i].MerchantID, int32(dist), orderAmount)
		if err == nil && feeResult != nil && !feeResult.DeliverySuspended {
			dishes[i].EstimatedDeliveryFee = &feeResult.FinalFee
		}
	}
}

// calculateComboDistancesAndFees 批量计算套餐商户到用户的距离和预估运费
func (server *Server) calculateComboDistancesAndFees(ctx *gin.Context, combos []comboSummary, userLat, userLng float64) {
	if len(combos) == 0 || server.mapClient == nil {
		return
	}

	// 构建商户位置列表（去重）
	merchantDistances := make(map[int64]int)
	merchantRegionIDs := make(map[int64]int64)
	var merchantLocs []maps.Location
	var merchantIDs []int64

	for _, c := range combos {
		if _, exists := merchantDistances[c.MerchantID]; !exists && (c.MerchantLatitude != 0 || c.MerchantLongitude != 0) {
			merchantLocs = append(merchantLocs, maps.Location{
				Lat: c.MerchantLatitude,
				Lng: c.MerchantLongitude,
			})
			merchantIDs = append(merchantIDs, c.MerchantID)
			merchantDistances[c.MerchantID] = -1
			merchantRegionIDs[c.MerchantID] = c.MerchantRegionID
		}
	}

	if len(merchantLocs) == 0 {
		return
	}

	userLoc := []maps.Location{{Lat: userLat, Lng: userLng}}
	result, err := server.mapClient.GetDistanceMatrix(ctx, merchantLocs, userLoc, "bicycling")
	if err != nil {
		return
	}

	// 更新商户距离映射
	for i, row := range result.Rows {
		if i >= len(merchantIDs) {
			break
		}
		if len(row.Elements) > 0 {
			merchantDistances[merchantIDs[i]] = row.Elements[0].Distance
		}
	}

	// 为每个套餐计算距离和运费（使用套餐实际价格）
	for i := range combos {
		dist, ok := merchantDistances[combos[i].MerchantID]
		if !ok || dist < 0 {
			continue
		}
		combos[i].Distance = &dist

		// 使用套餐的实际售价计算运费
		// 价格为0时只是跳过货值加价，其他系数（时段、天气、距离）仍正常计算
		orderAmount := combos[i].ComboPrice

		regionID := merchantRegionIDs[combos[i].MerchantID]
		feeResult, err := server.calculateDeliveryFeeInternal(ctx, regionID, combos[i].MerchantID, int32(dist), orderAmount)
		if err == nil && feeResult != nil && !feeResult.DeliverySuspended {
			combos[i].EstimatedDeliveryFee = &feeResult.FinalFee
		}
	}
}

// calculateMerchantDistancesAndFees 批量计算商户到用户的距离和预估运费
func (server *Server) calculateMerchantDistancesAndFees(ctx *gin.Context, merchants []merchantSummary, userLat, userLng float64) {
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

// calculateRoomDistances 批量计算包间商户到用户的距离
func (server *Server) calculateRoomDistances(ctx *gin.Context, rooms []exploreRoomItem, userLat, userLng float64) {
	if len(rooms) == 0 || server.mapClient == nil {
		return
	}

	// 构建商户位置列表（去重，因为多个包间可能属于同一商户）
	merchantDistances := make(map[int64]int) // merchantID -> distance
	var merchantLocs []maps.Location
	var merchantIDs []int64

	for _, r := range rooms {
		if _, exists := merchantDistances[r.MerchantID]; !exists && (r.MerchantLatitude != 0 || r.MerchantLongitude != 0) {
			merchantLocs = append(merchantLocs, maps.Location{
				Lat: r.MerchantLatitude,
				Lng: r.MerchantLongitude,
			})
			merchantIDs = append(merchantIDs, r.MerchantID)
			merchantDistances[r.MerchantID] = -1 // 标记为待计算
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

	// 更新距离映射
	for i, row := range result.Rows {
		if i >= len(merchantIDs) {
			break
		}
		if len(row.Elements) > 0 {
			merchantDistances[merchantIDs[i]] = row.Elements[0].Distance
		}
	}

	// 填充到每个包间
	for i := range rooms {
		if dist, ok := merchantDistances[rooms[i].MerchantID]; ok && dist >= 0 {
			rooms[i].Distance = &dist
		}
	}
}
