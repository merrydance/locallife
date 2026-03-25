package media

import (
	"fmt"
	"strings"
	"time"
)

// Category 是媒体资产的业务分类，决定存储桶、object key 前缀和允许的 Content-Type。
type Category string

const (
	CategoryMerchantLogo      Category = "logo"
	CategoryDishImage         Category = "dish"
	CategoryComboImage        Category = "combo"
	CategoryTableImage        Category = "table"
	CategoryReviewImage       Category = "review"
	CategoryAvatar            Category = "avatar"
	CategoryBusinessLicense   Category = "business_license"
	CategoryFoodPermit        Category = "food_permit"
	CategoryIDCardFront       Category = "id_card_front"
	CategoryIDCardBack        Category = "id_card_back"
	CategoryHealthCert        Category = "health_cert"
	CategoryGroupLicense      Category = "group_license"
	CategorySafetyReportImage Category = "safety_report"
)

// Visibility 对应 media_assets.visibility 列。
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

// categoryMeta 包含一个 Category 的完整策略。
type categoryMeta struct {
	Visibility   Visibility
	KeyPrefix    string   // object key 目录前缀，不含末尾斜杠
	AllowedTypes []string // 允许的 Content-Type（精确匹配）
}

// registry 是全局 category 策略表。
var registry = map[Category]categoryMeta{
	CategoryMerchantLogo:      {VisibilityPublic, "merchant/logo", imageTypes},
	CategoryDishImage:         {VisibilityPublic, "merchant/dish", imageTypes},
	CategoryComboImage:        {VisibilityPublic, "merchant/combo", imageTypes},
	CategoryTableImage:        {VisibilityPublic, "merchant/table", imageTypes},
	CategoryReviewImage:       {VisibilityPublic, "user/review", imageTypes},
	CategoryAvatar:            {VisibilityPublic, "user/avatar", imageTypes},
	CategoryBusinessLicense:   {VisibilityPublic, "merchant/license/business", imageTypes},
	CategoryFoodPermit:        {VisibilityPublic, "merchant/license/food", imageTypes},
	CategoryIDCardFront:       {VisibilityPrivate, "id_card/front", imageTypes},
	CategoryIDCardBack:        {VisibilityPrivate, "id_card/back", imageTypes},
	CategoryHealthCert:        {VisibilityPrivate, "rider/health_cert", imageTypes},
	CategoryGroupLicense:      {VisibilityPrivate, "group/license", imageTypes},
	CategorySafetyReportImage: {VisibilityPrivate, "operator/safety", imageTypes},
}

var imageTypes = []string{
	"image/jpeg",
	"image/png",
	"image/webp",
	"image/heic",
	"image/heif",
}

// Policy 提供 Category 相关的策略查询。
type Policy struct{}

// Lookup 返回 category 的元数据，如果 category 未知则返回 ErrInvalidCategory。
func (Policy) Lookup(cat Category) (Visibility, string, error) {
	m, ok := registry[cat]
	if !ok {
		return "", "", fmt.Errorf("%w: %s", ErrInvalidCategory, cat)
	}
	return m.Visibility, m.KeyPrefix, nil
}

// IsAllowedContentType 检查给定 Content-Type 是否被该 category 允许。
func (Policy) IsAllowedContentType(cat Category, contentType string) error {
	m, ok := registry[cat]
	if !ok {
		return fmt.Errorf("%w: %s", ErrInvalidCategory, cat)
	}
	// 只匹配主类型/子类型，忽略参数（如 "; charset=utf-8"）
	ct := strings.SplitN(contentType, ";", 2)[0]
	ct = strings.TrimSpace(ct)
	for _, allowed := range m.AllowedTypes {
		if strings.EqualFold(ct, allowed) {
			return nil
		}
	}
	return fmt.Errorf("%w: %s for category %s", ErrUnsupportedContentType, contentType, cat)
}

// ObjectKey 生成标准化的 object key。
// 格式：{prefix}/{userID}/{date}/{uploadID}.{ext}
// 示例：merchant/dish/42/20260318/up_01J….jpeg
func (Policy) ObjectKey(cat Category, userID int64, uploadID, ext string, t time.Time) (string, error) {
	m, ok := registry[cat]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrInvalidCategory, cat)
	}
	date := t.UTC().Format("20060102")
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("%s/%d/%s/%s%s", m.KeyPrefix, userID, date, uploadID, ext), nil
}

// BucketFor 返回 category 应使用的存储桶（公共或私有）。
func (p Policy) BucketFor(cat Category, storage ObjectStorage) (string, error) {
	m, ok := registry[cat]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrInvalidCategory, cat)
	}
	if m.Visibility == VisibilityPublic {
		return storage.PublicBucket(), nil
	}
	return storage.PrivateBucket(), nil
}
