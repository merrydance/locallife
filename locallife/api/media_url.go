package api

import (
	"context"
	"path/filepath"

	"github.com/merrydance/locallife/media"
)

// mediaAssetLocalPath 在本地存储模式下，根据 media_asset ID 返回文件的本地绝对路径。
// 用于将已通过媒体上传服务上传的图片文件提供给 OCR 等本地处理逻辑使用。
// 若非本地存储模式（FILE_STORAGE_PROVIDER=oss）或资产不存在，返回空字符串。
func (server *Server) mediaAssetLocalPath(ctx context.Context, assetID int64) string {
	if server.config.FileStorageProvider != "local" {
		return ""
	}
	asset, err := server.store.GetMediaAssetByID(ctx, assetID)
	if err != nil {
		return ""
	}
	return filepath.Join("uploads/dev", filepath.FromSlash(asset.ObjectKey))
}

// batchPublicImageURLs 接收一组 media_asset ID，一次性查询 media_assets 表，
// 返回 assetID → CDN URL 的映射。未找到的 ID 不在返回 map 中。
// 失败时静默返回空 map，不影响主请求。
func (server *Server) batchPublicImageURLs(ctx context.Context, assetIDs []int64, variant media.Variant) map[int64]string {
	result := make(map[int64]string, len(assetIDs))
	if len(assetIDs) == 0 {
		return result
	}

	// 去重
	seen := make(map[int64]struct{}, len(assetIDs))
	dedupIDs := assetIDs[:0:len(assetIDs)]
	dedupIDs = dedupIDs[:0]
	for _, id := range assetIDs {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			dedupIDs = append(dedupIDs, id)
		}
	}

	assets, err := server.store.ListMediaAssetsByIDs(ctx, dedupIDs)
	if err != nil {
		return result
	}

	for _, a := range assets {
		if a.Visibility != string(media.VisibilityPublic) || a.ModerationStatus != "approved" {
			continue
		}
		result[a.ID] = server.mediaResolver.PublicURL(a.ObjectKey, variant)
	}
	return result
}

// publicImageURL 针对单个 asset ID 解析 CDN URL。
// 用于 create/update 等单项响应不值得批量查询的场景。
func (server *Server) publicImageURL(ctx context.Context, assetID *int64, variant media.Variant) string {
	if assetID == nil {
		return ""
	}
	m, err := server.store.GetMediaAssetByID(ctx, *assetID)
	if err != nil {
		return ""
	}
	if m.Visibility != string(media.VisibilityPublic) || m.ModerationStatus != "approved" {
		return ""
	}
	return server.mediaResolver.PublicURL(m.ObjectKey, variant)
}

// enrichCartImageURLs 为购物车商品批量填充 ImageURL 字段。
// 单次 DB 调用覆盖所有商品；未找到 asset 的商品 ImageURL 保持空字符串。
func (server *Server) enrichCartImageURLs(ctx context.Context, items []cartItemResponse) {
	assetIDs := make([]int64, 0, len(items))
	for _, item := range items {
		if item.ImageAssetID != nil {
			assetIDs = append(assetIDs, *item.ImageAssetID)
		}
	}
	imgURLs := server.batchPublicImageURLs(ctx, assetIDs, media.VariantCard)
	for i := range items {
		if items[i].ImageAssetID != nil {
			items[i].ImageURL = imgURLs[*items[i].ImageAssetID]
		}
	}
}

// enrichSearchDishURLs 为搜索菜品结果批量填充 ImageURL 和 MerchantLogoURL。
func (server *Server) enrichSearchDishURLs(ctx context.Context, dishes []searchDishResponse) {
	assetIDs := make([]int64, 0, len(dishes)*2)
	for _, d := range dishes {
		if d.ImageAssetID != nil {
			assetIDs = append(assetIDs, *d.ImageAssetID)
		}
		if d.MerchantLogoAssetID != nil {
			assetIDs = append(assetIDs, *d.MerchantLogoAssetID)
		}
	}
	imgURLs := server.batchPublicImageURLs(ctx, assetIDs, media.VariantCard)
	for i := range dishes {
		if dishes[i].ImageAssetID != nil {
			dishes[i].ImageURL = imgURLs[*dishes[i].ImageAssetID]
		}
		if dishes[i].MerchantLogoAssetID != nil {
			dishes[i].MerchantLogoURL = imgURLs[*dishes[i].MerchantLogoAssetID]
		}
	}
}

// enrichSearchMerchantURLs 为搜索商户结果批量填充 LogoURL。
func (server *Server) enrichSearchMerchantURLs(ctx context.Context, merchants []searchMerchantResponse) {
	assetIDs := make([]int64, 0, len(merchants))
	for _, m := range merchants {
		if m.LogoAssetID != nil {
			assetIDs = append(assetIDs, *m.LogoAssetID)
		}
	}
	imgURLs := server.batchPublicImageURLs(ctx, assetIDs, media.VariantCard)
	for i := range merchants {
		if merchants[i].LogoAssetID != nil {
			merchants[i].LogoURL = imgURLs[*merchants[i].LogoAssetID]
		}
	}
}

// enrichSearchRoomURLs 为搜索包间结果批量填充 MerchantLogoURL 和 ImageURL。
func (server *Server) enrichSearchRoomURLs(ctx context.Context, rooms []searchRoomResponse) {
	assetIDs := make([]int64, 0, len(rooms)*2)
	for _, r := range rooms {
		if r.MerchantLogoAssetID != nil {
			assetIDs = append(assetIDs, *r.MerchantLogoAssetID)
		}
		if r.PrimaryImageAssetID != nil {
			assetIDs = append(assetIDs, *r.PrimaryImageAssetID)
		}
	}
	imgURLs := server.batchPublicImageURLs(ctx, assetIDs, media.VariantCard)
	for i := range rooms {
		if rooms[i].MerchantLogoAssetID != nil {
			rooms[i].MerchantLogoURL = imgURLs[*rooms[i].MerchantLogoAssetID]
		}
		if rooms[i].PrimaryImageAssetID != nil {
			rooms[i].ImageURL = imgURLs[*rooms[i].PrimaryImageAssetID]
		}
	}
}

// enrichPublicRoomImageURLs 为公开包间列表批量填充 ImageURL。
func (server *Server) enrichPublicRoomImageURLs(ctx context.Context, rooms []publicRoomItem) {
	assetIDs := make([]int64, 0, len(rooms))
	for _, r := range rooms {
		if r.PrimaryImageAssetID != 0 {
			assetIDs = append(assetIDs, r.PrimaryImageAssetID)
		}
	}
	if len(assetIDs) == 0 {
		return
	}
	imgURLs := server.batchPublicImageURLs(ctx, assetIDs, media.VariantCard)
	for i := range rooms {
		if rooms[i].PrimaryImageAssetID != 0 {
			rooms[i].ImageURL = imgURLs[rooms[i].PrimaryImageAssetID]
		}
	}
}

// enrichSearchComboURLs 为搜索套餐结果批量填充 ImageURL 和 MerchantLogoURL。
func (server *Server) enrichSearchComboURLs(ctx context.Context, combos []searchComboResponse) {
	assetIDs := make([]int64, 0, len(combos)*2)
	for _, c := range combos {
		if c.ImageAssetID != nil {
			assetIDs = append(assetIDs, *c.ImageAssetID)
		}
		if c.MerchantLogoAssetID != nil {
			assetIDs = append(assetIDs, *c.MerchantLogoAssetID)
		}
	}
	imgURLs := server.batchPublicImageURLs(ctx, assetIDs, media.VariantCard)
	for i := range combos {
		if combos[i].ImageAssetID != nil {
			combos[i].ImageURL = imgURLs[*combos[i].ImageAssetID]
		}
		if combos[i].MerchantLogoAssetID != nil {
			combos[i].MerchantLogoURL = imgURLs[*combos[i].MerchantLogoAssetID]
		}
	}
}
