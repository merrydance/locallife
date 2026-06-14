package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
)

const (
	currentTableQRCodeFilenameSuffix = "_qrcode_v2.png"
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
	LogoAssetID *int64  `json:"logo_asset_id,omitempty"`
	LogoURL     string  `json:"logo_url,omitempty"`
	Phone       string  `json:"phone,omitempty"`
	Address     string  `json:"address,omitempty"`
	Status      string  `json:"status"`
	IsOpen      bool    `json:"is_open"`
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
	ID                  int64                `json:"id"`
	Name                string               `json:"name"`
	Description         *string              `json:"description,omitempty"`
	ImageAssetID        *int64               `json:"image_asset_id,omitempty"`
	ImageURL            string               `json:"image_url,omitempty"`
	Price               int64                `json:"price"`
	OriginalPrice       int64                `json:"original_price"`
	MemberPrice         *int64               `json:"member_price,omitempty"`
	CategoryID          int64                `json:"category_id"`
	CategoryName        string               `json:"category_name"`
	IsAvailable         bool                 `json:"is_available"`
	SortOrder           int32                `json:"sort_order"`
	CustomizationGroups []customizationGroup `json:"customization_groups,omitempty"`
	Tags                []string             `json:"tags,omitempty"`
	MerchantID          int64                `json:"merchant_id"`
	IsOnline            bool                 `json:"is_online"`
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
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Description   *string  `json:"description,omitempty"`
	ImageAssetID  *int64   `json:"image_asset_id,omitempty"`
	ImageURL      string   `json:"image_url,omitempty"`
	Price         int64    `json:"price"`
	OriginalPrice *int64   `json:"original_price,omitempty"`
	IsAvailable   bool     `json:"is_available"`
	Tags          []string `json:"tags,omitempty"`
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
// @Security BearerAuth
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
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查商户状态
	if merchant.Status != "approved" {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(ErrMerchantServiceUnavailable))
		return
	}

	// P1-022 修复：检查商户实时营业状态
	if !merchant.IsOpen {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(ErrMerchantServiceUnavailable))
		return
	}

	// 2. 获取桌台信息
	table, err := server.store.GetTableByMerchantAndNo(ctx, db.GetTableByMerchantAndNoParams{
		MerchantID: req.MerchantID,
		TableNo:    req.TableNo,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrTableNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查桌台状态
	if table.Status == "disabled" {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(ErrTableDisabled))
		return
	}

	response, err := server.buildScanTableResponse(ctx, merchant, table)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, response)
}

func (server *Server) buildScanTableResponse(ctx context.Context, merchant db.Merchant, table db.Table) (scanTableResponse, error) {
	categories, err := server.store.ListDishCategories(ctx, merchant.ID)
	if err != nil {
		return scanTableResponse{}, err
	}

	dishes, err := server.store.ListDishesForMenu(ctx, merchant.ID)
	if err != nil {
		return scanTableResponse{}, err
	}

	categoryMap := make(map[int64]int)
	categoryNameMap := make(map[int64]string)
	categoryList := make([]scanTableCategoryInfo, 0, len(categories)+1)
	const uncategorizedCategoryID int64 = -1
	const uncategorizedCategoryName = "其他"
	var dishAssetIDs []int64

	for _, cat := range categories {
		catInfo := scanTableCategoryInfo{
			ID:        cat.ID,
			Name:      cat.Name,
			SortOrder: int32(cat.SortOrder),
			Dishes:    []scanTableDishInfo{},
		}
		categoryList = append(categoryList, catInfo)
		categoryMap[cat.ID] = len(categoryList) - 1
		categoryNameMap[cat.ID] = cat.Name
	}

	ensureUncategorizedCategory := func() int {
		if idx, ok := categoryMap[uncategorizedCategoryID]; ok {
			return idx
		}

		categoryList = append(categoryList, scanTableCategoryInfo{
			ID:        uncategorizedCategoryID,
			Name:      uncategorizedCategoryName,
			SortOrder: 9999,
			Dishes:    []scanTableDishInfo{},
		})
		idx := len(categoryList) - 1
		categoryMap[uncategorizedCategoryID] = idx
		categoryNameMap[uncategorizedCategoryID] = uncategorizedCategoryName
		return idx
	}

	for _, dish := range dishes {
		var categoryID int64
		if dish.CategoryID.Valid {
			categoryID = dish.CategoryID.Int64
		}

		dishInfo := scanTableDishInfo{
			ID:            dish.ID,
			MerchantID:    merchant.ID,
			IsOnline:      true,
			Name:          dish.Name,
			Price:         dish.Price,
			OriginalPrice: dish.Price,
			CategoryID:    categoryID,
			IsAvailable:   dish.IsAvailable,
			SortOrder:     int32(dish.SortOrder),
		}
		if dish.Description.Valid {
			dishInfo.Description = &dish.Description.String
		}
		dishInfo.ImageAssetID = int64PtrFromPgInt8(dish.ImageMediaAssetID)
		if dishInfo.ImageAssetID != nil {
			dishAssetIDs = append(dishAssetIDs, *dishInfo.ImageAssetID)
		}
		if dish.MemberPrice.Valid {
			dishInfo.MemberPrice = &dish.MemberPrice.Int64
		}
		if dish.Tags != nil {
			if err := parseJSON(dish.Tags, &dishInfo.Tags); err != nil {
				return scanTableResponse{}, fmt.Errorf("decode scan table dish %d tags: %w", dish.ID, err)
			}
		}
		if dish.CustomizationGroups != nil {
			if err := parseJSON(dish.CustomizationGroups, &dishInfo.CustomizationGroups); err != nil {
				return scanTableResponse{}, fmt.Errorf("decode scan table dish %d customization_groups: %w", dish.ID, err)
			}
		}

		if categoryName, ok := categoryNameMap[categoryID]; ok {
			dishInfo.CategoryName = categoryName
		}

		if categoryIndex, ok := categoryMap[categoryID]; ok {
			categoryList[categoryIndex].Dishes = append(categoryList[categoryIndex].Dishes, dishInfo)
		} else {
			fallbackIndex := ensureUncategorizedCategory()
			dishInfo.CategoryID = uncategorizedCategoryID
			dishInfo.CategoryName = uncategorizedCategoryName
			categoryList[fallbackIndex].Dishes = append(categoryList[fallbackIndex].Dishes, dishInfo)
		}
	}

	combos, err := server.store.ListOnlineCombosByMerchant(ctx, merchant.ID)
	if err != nil {
		return scanTableResponse{}, err
	}

	var comboAssetIDs []int64
	comboList := make([]scanTableComboInfo, 0, len(combos))
	for _, combo := range combos {
		comboInfo := scanTableComboInfo{
			ID:          combo.ID,
			Name:        combo.Name,
			Price:       combo.Price,
			IsAvailable: combo.IsOnline,
		}
		if combo.Tags != nil {
			if err := parseJSON(combo.Tags, &comboInfo.Tags); err != nil {
				return scanTableResponse{}, fmt.Errorf("decode scan table combo %d tags: %w", combo.ID, err)
			}
		}
		if combo.Description.Valid {
			comboInfo.Description = &combo.Description.String
		}
		comboInfo.ImageAssetID = int64PtrFromPgInt8(combo.ImageMediaAssetID)
		if comboInfo.ImageAssetID != nil {
			comboAssetIDs = append(comboAssetIDs, *comboInfo.ImageAssetID)
		}
		if combo.OriginalPrice > 0 {
			comboInfo.OriginalPrice = &combo.OriginalPrice
		}
		comboList = append(comboList, comboInfo)
	}

	var promotions []scanTablePromotionInfo
	if deliveryPromotions, err := server.store.ListActiveDeliveryPromotionsByMerchant(ctx, merchant.ID); err == nil {
		for _, dp := range deliveryPromotions {
			promotions = append(promotions, scanTablePromotionInfo{
				Type:        "delivery_return",
				MinAmount:   dp.MinOrderAmount,
				ReturnValue: dp.DiscountAmount,
				Description: formatDeliveryReturnDesc(dp.MinOrderAmount, dp.DiscountAmount),
			})
		}
	}
	if discountRules, err := server.store.ListActiveDiscountRules(ctx, merchant.ID); err == nil {
		for _, dr := range discountRules {
			promotions = append(promotions, scanTablePromotionInfo{
				Type:        "discount",
				MinAmount:   dr.MinOrderAmount,
				ReturnValue: dr.DiscountAmount,
				Description: formatDiscountDesc(dr.MinOrderAmount, dr.DiscountAmount),
			})
		}
	}

	response := scanTableResponse{
		Merchant: scanTableMerchantInfo{
			ID:      merchant.ID,
			Name:    merchant.Name,
			Phone:   merchant.Phone,
			Address: merchant.Address,
			Status:  merchant.Status,
			IsOpen:  merchant.IsOpen,
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

	if merchant.Description.Valid {
		response.Merchant.Description = &merchant.Description.String
	}
	response.Merchant.LogoAssetID = int64PtrFromPgInt8(merchant.LogoMediaAssetID)
	response.Merchant.LogoURL = server.publicImageURL(ctx, response.Merchant.LogoAssetID, media.VariantCard)
	if table.Description.Valid {
		response.Table.Description = &table.Description.String
	}
	if table.MinimumSpend.Valid {
		response.Table.MinimumSpend = &table.MinimumSpend.Int64
	}

	dishImageURLs := server.batchPublicImageURLs(ctx, dishAssetIDs, media.VariantCard)
	comboImageURLs := server.batchPublicImageURLs(ctx, comboAssetIDs, media.VariantCard)
	for i := range response.Categories {
		for j := range response.Categories[i].Dishes {
			if response.Categories[i].Dishes[j].ImageAssetID != nil {
				response.Categories[i].Dishes[j].ImageURL = dishImageURLs[*response.Categories[i].Dishes[j].ImageAssetID]
			}
			dishID := response.Categories[i].Dishes[j].ID
			dishWithCust, err := server.store.GetDishWithCustomizations(ctx, dishID)
			if err == nil {
				var groups []customizationGroup
				if err := parseJSON(dishWithCust.CustomizationGroups, &groups); err == nil {
					response.Categories[i].Dishes[j].CustomizationGroups = groups
				}
			}
		}
	}
	for i := range response.Combos {
		if response.Combos[i].ImageAssetID != nil {
			response.Combos[i].ImageURL = comboImageURLs[*response.Combos[i].ImageAssetID]
		}
	}

	return response, nil
}

// formatDeliveryReturnDesc 格式化满返描述
func formatDeliveryReturnDesc(minAmount, returnAmount int64) string {
	return "满" + fenToYuanString(minAmount, 0) +
		"元返" + fenToYuanString(returnAmount, 0) + "元运费"
}

// formatDiscountDesc 格式化满减描述
func formatDiscountDesc(minAmount, discountValue int64) string {
	return "满" + fenToYuanString(minAmount, 0) +
		"元减" + fenToYuanString(discountValue, 0) + "元"
}

// generateTableQRCodeResponse 生成二维码响应
type generateTableQRCodeResponse struct {
	QrCodeUrl  string `json:"qr_code_url" example:"https://cdn.example.com/qrcodes/m1_t123.png"`
	TableNo    string `json:"table_no" example:"T01"`
	MerchantID int64  `json:"merchant_id" example:"1"`
}

func isCurrentTableQRCodePath(path string) bool {
	return strings.HasSuffix(path, currentTableQRCodeFilenameSuffix)
}

func buildTableQRCodeObjectKey(merchantID, tableID int64, checksum string) string {
	shortChecksum := checksum
	if len(shortChecksum) > 12 {
		shortChecksum = shortChecksum[:12]
	}
	filename := fmt.Sprintf("qrcode_m%d_t%d_%s%s", merchantID, tableID, shortChecksum, currentTableQRCodeFilenameSuffix)
	return fmt.Sprintf("merchant/table/%d/qrcodes/%s", merchantID, filename)
}

func (server *Server) storeTableQRCode(ctx context.Context, uploaderID int64, merchantID, tableID int64, pngData []byte) (string, error) {
	if server.mediaStorage == nil {
		return "", fmt.Errorf("media storage is not initialized")
	}

	checksumBytes := sha256.Sum256(pngData)
	checksum := hex.EncodeToString(checksumBytes[:])
	objectKey := buildTableQRCodeObjectKey(merchantID, tableID, checksum)

	if err := server.mediaStorage.PutObject(ctx, server.mediaStorage.PublicBucket(), objectKey, "image/png", bytes.NewReader(pngData), int64(len(pngData))); err != nil {
		return "", err
	}

	asset, err := server.store.CreateMediaAsset(ctx, db.CreateMediaAssetParams{
		ObjectKey:      objectKey,
		Visibility:     string(media.VisibilityPublic),
		MediaCategory:  string(media.CategoryTableImage),
		MimeType:       "image/png",
		FileSize:       int64(len(pngData)),
		ChecksumSha256: checksum,
		UploadedBy:     uploaderID,
		SourceClient:   "server",
	})
	if err != nil {
		return "", err
	}

	if asset.ModerationStatus != "approved" {
		if _, err := server.store.SetMediaAssetModerationStatus(ctx, db.SetMediaAssetModerationStatusParams{
			ID:               asset.ID,
			ModerationStatus: "approved",
		}); err != nil {
			return "", err
		}
	}

	return server.mediaResolver.PublicURL(objectKey, media.VariantOriginal), nil
}

// generateTableQRCode godoc
// @Summary 生成桌台二维码
// @Description 为指定桌台生成微信小程序码。扫码后跳转到堂食菜单页面。仅桌台所属商户可调用。
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
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrInvalidTableID))
		return
	}

	// 获取桌台
	table, err := server.store.GetTable(ctx, tableID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrTableNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证是否为商户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(ErrNotMerchant))
		return
	}

	if table.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(ErrTableNotOwned))
		return
	}

	// 如果已有当前版本二维码，直接返回；旧版带文字二维码会在这里自动刷新为新版本。
	if table.QrCodeUrl.Valid && table.QrCodeUrl.String != "" && isCurrentTableQRCodePath(table.QrCodeUrl.String) {
		ctx.JSON(http.StatusOK, generateTableQRCodeResponse{
			QrCodeUrl:  table.QrCodeUrl.String,
			TableNo:    table.TableNo,
			MerchantID: merchant.ID,
		})
		return
	}

	// 调用微信API生成小程序码
	// scene参数只允许：数字、英文字母、下划线、减号，最大32字符
	// 格式: m_商户ID-t_桌号
	scene := "m_" + strconv.FormatInt(merchant.ID, 10) + "-t_" + table.TableNo
	if len(scene) > 32 {
		// 如果超长，使用桌台ID
		scene = "tid_" + strconv.FormatInt(tableID, 10)
	}

	checkPath := false
	wxaReq := &wechat.WXACodeRequest{
		Scene:      scene,
		Page:       "pages/dine-in/menu/menu", // 堂食菜单页面
		CheckPath:  &checkPath,                // 跳过页面验证 (开发时使用)
		EnvVersion: "develop",                 // 开发版 (正式发布后改为 release)
		Width:      430,
	}

	pngData, err := server.wechatClient.GetWXACodeUnlimited(ctx, wxaReq)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("生成小程序码失败: %w", err)))
		return
	}

	qrCodeURL, err := server.storeTableQRCode(ctx, authPayload.UserID, merchant.ID, tableID, pngData)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("保存二维码图片失败: %w", err)))
		return
	}

	// 更新桌台的二维码URL
	_, err = server.store.UpdateTable(ctx, db.UpdateTableParams{
		ID:        tableID,
		QrCodeUrl: pgtype.Text{String: qrCodeURL, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, generateTableQRCodeResponse{
		QrCodeUrl:  qrCodeURL,
		TableNo:    table.TableNo,
		MerchantID: merchant.ID,
	})
}
