package media

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestResolver(storage ObjectStorage) *URLResolver {
	return NewURLResolver(ResolverConfig{
		CDNPublicBaseURL: "https://cdn.example.com",
		ThumbWidth:       200,
		CardWidth:        400,
		DetailWidth:      960,
	}, storage)
}

func TestURLResolver_PublicURL_WithOSSStorage(t *testing.T) {
	// OSSStorage を直接生成するのは重いので、LocalStorage の逆（非 Local）として
	// ダミーの OSSStorage を構築する代わりに、nil ポインタでない *OSSStorage を
	// 渡せないため、ここではシンプルに LocalStorage の有無で分岐する ossProcessParam
	// の挙動を間接的に検証する。
	// use a non-LocalStorage to exercise the OSS branch via ossProcessParam.
	// We can't easily instantiate OSSStorage without credentials, so we rely on
	// the fact that ossProcessParam checks `if _, ok := r.storage.(*LocalStorage)`.
	// A nil ObjectStorage will panic; use LocalStorage for the "no-param" path,
	// and validate the OSS variant format via direct ossProcessParam tests.

	ls := NewLocalStorage("http://localhost:8080", "/tmp")
	r := newTestResolver(ls)

	t.Run("local storage — original has no process param", func(t *testing.T) {
		url := r.PublicURL("merchant/dish/1/file.jpg", VariantOriginal)
		require.Equal(t, "https://cdn.example.com/merchant/dish/1/file.jpg", url)
	})

	t.Run("local storage — all variants have no process param", func(t *testing.T) {
		for _, v := range []Variant{VariantThumb, VariantCard, VariantDetail} {
			url := r.PublicURL("merchant/dish/1/file.jpg", v)
			require.False(t, strings.Contains(url, "?"),
				"LocalStorage should not append query params for variant %s", v)
		}
	})

	t.Run("leading slash in objectKey is handled", func(t *testing.T) {
		url := r.PublicURL("/merchant/dish/1/file.jpg", VariantOriginal)
		require.Equal(t, "https://cdn.example.com/merchant/dish/1/file.jpg", url)
		// path portion should not have consecutive slashes
		path := strings.TrimPrefix(url, "https://cdn.example.com")
		require.False(t, strings.Contains(path, "//"), "path should not have double slash")
	})

	t.Run("trailing slash in CDN base is trimmed", func(t *testing.T) {
		r2 := NewURLResolver(ResolverConfig{CDNPublicBaseURL: "https://cdn.example.com/"}, ls)
		url := r2.PublicURL("a/b/c.jpg", VariantOriginal)
		require.Equal(t, "https://cdn.example.com/a/b/c.jpg", url)
	})
}

func TestURLResolver_ossProcessParam(t *testing.T) {
	ls := NewLocalStorage("http://localhost:8080", "/tmp")
	r := newTestResolver(ls)

	// LocalStorage → always empty
	for _, v := range []Variant{VariantThumb, VariantCard, VariantDetail, VariantOriginal} {
		param := r.ossProcessParam(v)
		require.Empty(t, param, "LocalStorage should produce empty param for variant %s", v)
	}
}

func TestURLResolver_ossProcessParam_OSSBranch(t *testing.T) {
	// To test the OSS branch we build a URLResolver with a non-*LocalStorage.
	// We can use the LocalStorage bucket names but wrap it in an interface-only
	// holder to break the type assertion, triggering the OSS code path.
	r := &URLResolver{
		cfg: ResolverConfig{
			ThumbWidth:  200,
			CardWidth:   400,
			DetailWidth: 960,
		},
		storage: &ossStorageStub{},
	}

	t.Run("thumb variant", func(t *testing.T) {
		param := r.ossProcessParam(VariantThumb)
		require.Equal(t, "image/resize,w_200,m_lfit/format,webp", param)
	})

	t.Run("card variant", func(t *testing.T) {
		param := r.ossProcessParam(VariantCard)
		require.Equal(t, "image/resize,w_400,m_lfit/format,webp", param)
	})

	t.Run("detail variant", func(t *testing.T) {
		param := r.ossProcessParam(VariantDetail)
		require.Equal(t, "image/resize,w_960,m_lfit/format,webp", param)
	})

	t.Run("original variant produces empty param", func(t *testing.T) {
		param := r.ossProcessParam(VariantOriginal)
		require.Empty(t, param)
	})

	t.Run("PublicURL with card variant appends query string", func(t *testing.T) {
		r2 := NewURLResolver(ResolverConfig{
			CDNPublicBaseURL: "https://cdn.example.com",
			CardWidth:        400,
		}, &ossStorageStub{})
		url := r2.PublicURL("merchant/dish/1/f.jpg", VariantCard)
		require.Equal(t,
			"https://cdn.example.com/merchant/dish/1/f.jpg?image/resize,w_400,m_lfit/format,webp",
			url)
	})
}

// ossStorageStub is a minimal ObjectStorage that is NOT *LocalStorage,
// used to exercise the OSS code path in ossProcessParam.
type ossStorageStub struct{}

func (ossStorageStub) CreateDirectUpload(_ context.Context, _ DirectUploadRequest) (DirectUploadResult, error) {
	return DirectUploadResult{}, nil
}
func (ossStorageStub) StatObject(_ context.Context, _, _ string) (ObjectMetadata, error) {
	return ObjectMetadata{}, nil
}
func (ossStorageStub) CreatePrivateDownloadURL(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return "", nil
}
func (ossStorageStub) DeleteObject(_ context.Context, _, _ string) error { return nil }
func (ossStorageStub) PublicBucket() string                              { return "pub" }
func (ossStorageStub) PrivateBucket() string                             { return "priv" }
