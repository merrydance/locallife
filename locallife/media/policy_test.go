package media

import (
"errors"
"testing"
"time"

"github.com/stretchr/testify/require"
)

func TestPolicy_Lookup(t *testing.T) {
p := Policy{}

t.Run("known public category", func(t *testing.T) {
vis, prefix, err := p.Lookup(CategoryDishImage)
require.NoError(t, err)
require.Equal(t, VisibilityPublic, vis)
require.Equal(t, "merchant/dish", prefix)
})

t.Run("known private category", func(t *testing.T) {
vis, prefix, err := p.Lookup(CategoryBusinessLicense)
require.NoError(t, err)
require.Equal(t, VisibilityPrivate, vis)
require.Equal(t, "merchant/license/business", prefix)
})

t.Run("unknown category returns ErrInvalidCategory", func(t *testing.T) {
_, _, err := p.Lookup("nonexistent_category")
require.ErrorIs(t, err, ErrInvalidCategory)
})
}

func TestPolicy_AllCategoriesAreRegistered(t *testing.T) {
p := Policy{}
known := []Category{
CategoryMerchantLogo, CategoryDishImage, CategoryComboImage,
CategoryTableImage, CategoryReviewImage, CategoryAvatar,
CategoryBusinessLicense, CategoryFoodPermit, CategoryIDCardFront,
CategoryIDCardBack, CategoryHealthCert, CategoryGroupLicense,
CategorySafetyReportImage,
}
for _, cat := range known {
_, _, err := p.Lookup(cat)
require.NoError(t, err, "category %s should be registered", cat)
}
}

func TestPolicy_VisibilityMapping(t *testing.T) {
p := Policy{}

publicCats := []Category{
CategoryMerchantLogo, CategoryDishImage, CategoryComboImage,
CategoryTableImage, CategoryReviewImage, CategoryAvatar,
}
privateCats := []Category{
CategoryBusinessLicense, CategoryFoodPermit, CategoryIDCardFront,
CategoryIDCardBack, CategoryHealthCert, CategoryGroupLicense,
CategorySafetyReportImage,
}

for _, cat := range publicCats {
vis, _, err := p.Lookup(cat)
require.NoError(t, err)
require.Equal(t, VisibilityPublic, vis, "category %s should be public", cat)
}
for _, cat := range privateCats {
vis, _, err := p.Lookup(cat)
require.NoError(t, err)
require.Equal(t, VisibilityPrivate, vis, "category %s should be private", cat)
}
}

func TestPolicy_IsAllowedContentType(t *testing.T) {
p := Policy{}

cases := []struct {
name        string
cat         Category
contentType string
wantErr     error
}{
{"jpeg allowed", CategoryDishImage, "image/jpeg", nil},
{"png allowed", CategoryDishImage, "image/png", nil},
{"webp allowed", CategoryDishImage, "image/webp", nil},
{"heic allowed", CategoryDishImage, "image/heic", nil},
{"heif allowed", CategoryDishImage, "image/heif", nil},
{"case insensitive", CategoryDishImage, "Image/JPEG", nil},
{"with params stripped", CategoryDishImage, "image/jpeg; charset=utf-8", nil},
{"pdf rejected", CategoryDishImage, "application/pdf", ErrUnsupportedContentType},
{"text rejected", CategoryDishImage, "text/plain", ErrUnsupportedContentType},
{"unknown category", "bad_cat", "image/jpeg", ErrInvalidCategory},
}

for _, tc := range cases {
t.Run(tc.name, func(t *testing.T) {
err := p.IsAllowedContentType(tc.cat, tc.contentType)
if tc.wantErr == nil {
require.NoError(t, err)
} else {
require.True(t, errors.Is(err, tc.wantErr), "got %v, want %v", err, tc.wantErr)
}
})
}
}

func TestPolicy_ObjectKey(t *testing.T) {
p := Policy{}
fixedTime := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)

t.Run("generates correct key", func(t *testing.T) {
key, err := p.ObjectKey(CategoryDishImage, 42, "up_abc123", "jpeg", fixedTime)
require.NoError(t, err)
require.Equal(t, "merchant/dish/42/20260318/up_abc123.jpeg", key)
})

t.Run("ext without dot is normalized", func(t *testing.T) {
key, err := p.ObjectKey(CategoryDishImage, 1, "up_xyz", "png", fixedTime)
require.NoError(t, err)
require.Contains(t, key, ".png")
})

t.Run("ext with dot is preserved", func(t *testing.T) {
key, err := p.ObjectKey(CategoryDishImage, 1, "up_xyz", ".jpg", fixedTime)
require.NoError(t, err)
require.Contains(t, key, ".jpg")
})

t.Run("empty ext produces key without dot", func(t *testing.T) {
key, err := p.ObjectKey(CategoryDishImage, 1, "up_xyz", "", fixedTime)
require.NoError(t, err)
require.Equal(t, "merchant/dish/1/20260318/up_xyz", key)
})

t.Run("unknown category returns error", func(t *testing.T) {
_, err := p.ObjectKey("bad_cat", 1, "up_xyz", "jpeg", fixedTime)
require.ErrorIs(t, err, ErrInvalidCategory)
})

t.Run("private category uses correct prefix", func(t *testing.T) {
key, err := p.ObjectKey(CategoryBusinessLicense, 99, "up_lic", "jpg", fixedTime)
require.NoError(t, err)
require.Equal(t, "merchant/license/business/99/20260318/up_lic.jpg", key)
})
}

func TestPolicy_BucketFor(t *testing.T) {
p := Policy{}
ls := NewLocalStorage("http://localhost:8080", "/tmp/test")

t.Run("public category returns public bucket", func(t *testing.T) {
bucket, err := p.BucketFor(CategoryDishImage, ls)
require.NoError(t, err)
require.Equal(t, ls.PublicBucket(), bucket)
})

t.Run("private category returns private bucket", func(t *testing.T) {
bucket, err := p.BucketFor(CategoryBusinessLicense, ls)
require.NoError(t, err)
require.Equal(t, ls.PrivateBucket(), bucket)
})

t.Run("unknown category returns error", func(t *testing.T) {
_, err := p.BucketFor("unknown", ls)
require.ErrorIs(t, err, ErrInvalidCategory)
})
}
