package media

import (
	"fmt"
	"strings"
)

// Variant 是图片处理规格。
type Variant string

const (
	// VariantOriginal 不添加任何图片处理参数，返回原图。
	VariantOriginal Variant = "original"
	// VariantThumb 缩略图（列表页小图）。
	VariantThumb Variant = "thumb"
	// VariantCard 卡片图（菜品卡片/商户卡片）。
	VariantCard Variant = "card"
	// VariantDetail 详情大图。
	VariantDetail Variant = "detail"
)

// ResolverConfig 是 URLResolver 需要的配置子集，从 util.Config 中提取。
type ResolverConfig struct {
	CDNPublicBaseURL string // 公共 CDN 根地址，例如 https://cdn.example.com
	ThumbWidth       int    // 缩略图宽度，像素
	CardWidth        int    // 卡片图宽度，像素
	DetailWidth      int    // 详情图宽度，像素
}

// URLResolver 根据 media_asset 记录构建最终访问 URL。
//
// 公共资产：CDN 地址 + 可选图片处理参数（OSS 图片处理 style 参数）。
// 私有资产：必须先调用 ObjectStorage.CreatePrivateDownloadURL，URLResolver 不处理私有资产，
//
//	由上层服务（RegistryService）负责签名后返回调用方。
type URLResolver struct {
	cfg     ResolverConfig
	storage ObjectStorage
}

// NewURLResolver 创建 URLResolver。
func NewURLResolver(cfg ResolverConfig, storage ObjectStorage) *URLResolver {
	return &URLResolver{cfg: cfg, storage: storage}
}

// PublicURL 返回公共资产的 CDN 访问地址，附带图片处理参数。
// objectKey 是 media_assets.object_key 列的值。
func (r *URLResolver) PublicURL(objectKey string, v Variant) string {
	base := strings.TrimRight(r.cfg.CDNPublicBaseURL, "/")
	key := strings.TrimLeft(objectKey, "/")
	url := fmt.Sprintf("%s/%s", base, key)

	param := r.ossProcessParam(v)
	if param != "" {
		url += "?" + param
	}
	return url
}

// ossProcessParam 返回阿里云 OSS 图片处理的 x-oss-process 参数值。
// 当 storage 指向本地开发环境时返回空字符串（本地不支持图片处理）。
func (r *URLResolver) ossProcessParam(v Variant) string {
	// LocalStorage 不支持图片处理
	if _, ok := r.storage.(*LocalStorage); ok {
		return ""
	}
	switch v {
	case VariantThumb:
		return fmt.Sprintf("image/resize,w_%d,m_lfit/format,webp", r.cfg.ThumbWidth)
	case VariantCard:
		return fmt.Sprintf("image/resize,w_%d,m_lfit/format,webp", r.cfg.CardWidth)
	case VariantDetail:
		return fmt.Sprintf("image/resize,w_%d,m_lfit/format,webp", r.cfg.DetailWidth)
	default:
		return ""
	}
}
