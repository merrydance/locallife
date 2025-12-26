package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

// =============================================================================
// Scan Table API - 扫码点餐
// =============================================================================

// scanTableRequest 扫码请求
type scanTableRequest struct {
	MerchantID int64  `form:"merchant_id" binding:"required,min=1"`
	TableNo    string `form:"table_no" binding:"required,max=50"`
}

// scanTableMerchantInfo 商户信息（扫码返回）
type scanTableMerchantInfo struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	LogoUrl     *string `json:"logo_url,omitempty"`
	Phone       string  `json:"phone"`
	Address     string  `json:"address"`
	Status      string  `json:"status"`
}

// scanTableTableInfo 桌台信息
type scanTableTableInfo struct {
	ID           int64   `json:"id"`
	TableNo      string  `json:"table_no"`
	TableType    string  `json:"table_type"`
	Capacity     int16   `json:"capacity"`
	Description  *string `json:"description,omitempty"`
	MinimumSpend *int64  `json:"minimum_spend,omitempty"`
	Status       string  `json:"status"`
}

// scanTableDishInfo 菜品信息
type scanTableDishInfo struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	ImageUrl     *string `json:"image_url,omitempty"`
	Price        int64   `json:"price"`
	MemberPrice  *int64  `json:"member_price,omitempty"`
	CategoryID   int64   `json:"category_id"`
	CategoryName string  `json:"category_name"`
	IsAvailable  bool    `json:"is_available"`
	SortOrder    int32   `json:"sort_order"`
}

// scanTableCategoryInfo 分类信息
type scanTableCategoryInfo struct {
	ID        int64               `json:"id"`
	Name      string              `json:"name"`
	SortOrder int32               `json:"sort_order"`
	Dishes    []scanTableDishInfo `json:"dishes"`
}

// scanTableComboInfo 套餐信息
type scanTableComboInfo struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	Description   *string `json:"description,omitempty"`
	ImageUrl      *string `json:"image_url,omitempty"`
	Price         int64   `json:"price"`
	OriginalPrice *int64  `json:"original_price,omitempty"`
	IsAvailable   bool    `json:"is_available"`
}

// scanTablePromotionInfo 满返/满减优惠信息
type scanTablePromotionInfo struct {
	Type        string `json:"type"` // delivery_return / discount
	MinAmount   int64  `json:"min_amount"`
	ReturnValue int64  `json:"return_value"` // 满返金额 或 满减金额
	Description string `json:"description"`
}

// scanTableResponse 扫码返回结果
type scanTableResponse struct {
	Merchant   scanTableMerchantInfo    `json:"merchant"`
	Table      scanTableTableInfo       `json:"table"`
	Categories []scanTableCategoryInfo  `json:"categories"`
	Combos     []scanTableComboInfo     `json:"combos"`
	Promotions []scanTablePromotionInfo `json:"promotions"`
}

// scanTable godoc
// @Summary 扫码点餐
// @Description 扫描桌台二维码获取商户信息、桌台信息、菜单和优惠活动。用于堂食场景，顾客扫码后可查看菜单并下单。
// @Tags 扫码点餐
// @Accept json
// @Produce json
// @Param merchant_id query int true "商户ID" minimum(1)
// @Param table_no query string true "桌台编号" maxLength(50)
// @Success 200 {object} scanTableResponse "扫码成功，返回完整菜单信息"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 404 {object} ErrorResponse "商户或桌台不存在"
// @Failure 503 {object} ErrorResponse "商户未营业或桌台已停用"
// @Router /v1/scan/table [get]
func (server *Server) scanTable(ctx *gin.Context) {
	var req scanTableRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 1. 获取商户信息
	merchant, err := server.store.GetMerchant(ctx, req.MerchantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("商户不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查商户状态
	if merchant.Status != "approved" {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("商户未营业")))
		return
	}

	// 2. 获取桌台信息
	table, err := server.store.GetTableByMerchantAndNo(ctx, db.GetTableByMerchantAndNoParams{
		MerchantID: req.MerchantID,
		TableNo:    req.TableNo,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("桌台不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查桌台状态
	if table.Status == "disabled" {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("桌台已停用")))
		return
	}

	// 3. 获取菜品分类
	categories, err := server.store.ListDishCategories(ctx, req.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 4. 获取所有上架菜品
	dishes, err := server.store.ListDishesForMenu(ctx, req.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构建分类 -> 菜品映射
	categoryMap := make(map[int64]*scanTableCategoryInfo)
	var categoryList []scanTableCategoryInfo
	for _, cat := range categories {
		catInfo := scanTableCategoryInfo{
			ID:        cat.ID,
			Name:      cat.Name,
			SortOrder: int32(cat.SortOrder),
			Dishes:    []scanTableDishInfo{},
		}
		categoryList = append(categoryList, catInfo)
		categoryMap[cat.ID] = &categoryList[len(categoryList)-1]
	}

	// 将菜品分配到分类
	for _, dish := range dishes {
		var categoryID int64
		if dish.CategoryID.Valid {
			categoryID = dish.CategoryID.Int64
		}

		dishInfo := scanTableDishInfo{
			ID:          dish.ID,
			Name:        dish.Name,
			Price:       dish.Price,
			CategoryID:  categoryID,
			IsAvailable: dish.IsAvailable,
			SortOrder:   int32(dish.SortOrder),
		}
		if dish.Description.Valid {
			dishInfo.Description = &dish.Description.String
		}
		if dish.ImageUrl.Valid {
			img := normalizeUploadURLForClient(dish.ImageUrl.String)
			dishInfo.ImageUrl = &img
		}
		if dish.MemberPrice.Valid {
			dishInfo.MemberPrice = &dish.MemberPrice.Int64
		}

		// 找到分类名称
		for _, cat := range categories {
			if cat.ID == categoryID {
				dishInfo.CategoryName = cat.Name
				break
			}
		}

		if cat, ok := categoryMap[categoryID]; ok {
			cat.Dishes = append(cat.Dishes, dishInfo)
		}
	}

	// 5. 获取上架套餐
	combos, err := server.store.ListOnlineCombosByMerchant(ctx, req.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var comboList []scanTableComboInfo
	for _, combo := range combos {
		comboInfo := scanTableComboInfo{
			ID:          combo.ID,
			Name:        combo.Name,
			Price:       combo.Price,
			IsAvailable: combo.IsOnline,
		}
		if combo.Description.Valid {
			comboInfo.Description = &combo.Description.String
		}
		if combo.ImageUrl.Valid {
			img := normalizeUploadURLForClient(combo.ImageUrl.String)
			comboInfo.ImageUrl = &img
		}
		if combo.OriginalPrice > 0 {
			comboInfo.OriginalPrice = &combo.OriginalPrice
		}
		comboList = append(comboList, comboInfo)
	}

	// 6. 获取优惠活动（满返运费 + 满减）
	var promotions []scanTablePromotionInfo

	// 6.1 满返运费（商户配送优惠）- 使用已过滤有效期和活动状态的查询
	deliveryPromotions, err := server.store.ListActiveDeliveryPromotionsByMerchant(ctx, req.MerchantID)
	if err == nil {
		for _, dp := range deliveryPromotions {
			promotions = append(promotions, scanTablePromotionInfo{
				Type:        "delivery_return",
				MinAmount:   dp.MinOrderAmount,
				ReturnValue: dp.DiscountAmount,
				Description: formatDeliveryReturnDesc(dp.MinOrderAmount, dp.DiscountAmount),
			})
		}
	}

	// 6.2 满减规则
	discountRules, err := server.store.ListActiveDiscountRules(ctx, req.MerchantID)
	if err == nil {
		for _, dr := range discountRules {
			promotions = append(promotions, scanTablePromotionInfo{
				Type:        "discount",
				MinAmount:   dr.MinOrderAmount,
				ReturnValue: dr.DiscountAmount,
				Description: formatDiscountDesc(dr.MinOrderAmount, dr.DiscountAmount),
			})
		}
	}

	// 构建响应
	response := scanTableResponse{
		Merchant: scanTableMerchantInfo{
			ID:     merchant.ID,
			Name:   merchant.Name,
			Phone:  merchant.Phone,
			Status: merchant.Status,
		},
		Table: scanTableTableInfo{
			ID:        table.ID,
			TableNo:   table.TableNo,
			TableType: table.TableType,
			Capacity:  table.Capacity,
			Status:    table.Status,
		},
		Categories: categoryList,
		Combos:     comboList,
		Promotions: promotions,
	}

	// 填充可选字段
	if merchant.Description.Valid {
		response.Merchant.Description = &merchant.Description.String
	}
	if merchant.LogoUrl.Valid {
		logo := normalizeUploadURLForClient(merchant.LogoUrl.String)
		response.Merchant.LogoUrl = &logo
	}
	response.Merchant.Address = merchant.Address

	if table.Description.Valid {
		response.Table.Description = &table.Description.String
	}
	if table.MinimumSpend.Valid {
		response.Table.MinimumSpend = &table.MinimumSpend.Int64
	}

	ctx.JSON(http.StatusOK, response)
}

// formatDeliveryReturnDesc 格式化满返描述
func formatDeliveryReturnDesc(minAmount, returnAmount int64) string {
	return "满" + strconv.FormatFloat(float64(minAmount)/100, 'f', 0, 64) +
		"元返" + strconv.FormatFloat(float64(returnAmount)/100, 'f', 0, 64) + "元运费"
}

// formatDiscountDesc 格式化满减描述
func formatDiscountDesc(minAmount, discountValue int64) string {
	return "满" + strconv.FormatFloat(float64(minAmount)/100, 'f', 0, 64) +
		"元减" + strconv.FormatFloat(float64(discountValue)/100, 'f', 0, 64) + "元"
}

// generateTableQRCodeResponse 生成二维码响应
type generateTableQRCodeResponse struct {
	QrCodeUrl  string `json:"qr_code_url" example:"https://api.example.com/v1/scan/table?merchant_id=1&table_no=T01"`
	TableNo    string `json:"table_no" example:"T01"`
	MerchantID int64  `json:"merchant_id" example:"1"`
}

// generateTableQRCode godoc
// @Summary 生成桌台二维码
// @Description 为指定桌台生成扫码点餐二维码URL。仅桌台所属商户可调用。
// @Tags 桌台管理
// @Accept json
// @Produce json
// @Param id path int true "桌台ID" minimum(1)
// @Success 200 {object} generateTableQRCodeResponse "生成成功"
// @Failure 400 {object} ErrorResponse "无效的桌台ID"
// @Failure 403 {object} ErrorResponse "非商户或桌台不属于该商户"
// @Failure 404 {object} ErrorResponse "桌台不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/tables/{id}/qrcode [get]
func (server *Server) generateTableQRCode(ctx *gin.Context) {
	tableID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的桌台ID")))
		return
	}

	// 获取桌台
	table, err := server.store.GetTable(ctx, tableID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("桌台不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证是否为商户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("您不是商户")))
		return
	}

	if table.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("该桌台不属于您的商户")))
		return
	}

	// 生成扫码URL（实际生产中需要配置基础URL）
	// 格式: https://your-domain.com/v1/scan/table?merchant_id=xxx&table_no=xxx
	baseURL := server.config.WebBaseURL
	if baseURL == "" {
		baseURL = "https://api.example.com"
	}
	scanURL := baseURL + "/v1/scan/table?merchant_id=" + strconv.FormatInt(merchant.ID, 10) + "&table_no=" + table.TableNo

	// 更新桌台的二维码URL
	_, err = server.store.UpdateTable(ctx, db.UpdateTableParams{
		ID:        tableID,
		QrCodeUrl: pgtype.Text{String: scanURL, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, generateTableQRCodeResponse{
		QrCodeUrl:  scanURL,
		TableNo:    table.TableNo,
		MerchantID: merchant.ID,
	})
}
