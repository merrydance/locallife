package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 单元测试（无需网络，只验证签名逻辑与辅助函数）
// ---------------------------------------------------------------------------

func TestOSSKeyDir(t *testing.T) {
	cases := []struct {
		objectKey string
		want      string
	}{
		{"merchant/dish/1/20260318/up_abc.jpeg", "merchant/dish/1/20260318/"},
		{"a/b/c.jpg", "a/b/"},
		{"nodir.jpg", ""},
		{"", ""},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, ossKeyDir(tc.objectKey), "objectKey=%q", tc.objectKey)
	}
}

func TestContentTypePrefix(t *testing.T) {
	assert.Equal(t, "image/", contentTypePrefix("image/jpeg"))
	assert.Equal(t, "image/", contentTypePrefix("image/png"))
	assert.Equal(t, "video/", contentTypePrefix("video/mp4"))
	assert.Equal(t, "application/", contentTypePrefix("application/pdf"))
	assert.Equal(t, "", contentTypePrefix(""))
}

func TestStripScheme(t *testing.T) {
	assert.Equal(t, "oss-cn-hangzhou.aliyuncs.com", stripScheme("https://oss-cn-hangzhou.aliyuncs.com"))
	assert.Equal(t, "oss-cn-hangzhou.aliyuncs.com", stripScheme("http://oss-cn-hangzhou.aliyuncs.com"))
	assert.Equal(t, "oss-cn-hangzhou.aliyuncs.com", stripScheme("oss-cn-hangzhou.aliyuncs.com"))
	assert.Equal(t, "", stripScheme(""))
}

func TestOSSStorage_CreateDirectUpload_FormFields(t *testing.T) {
	// 使用假凭证——测试只验证本地签名逻辑，不发起任何网络请求
	s, err := NewOSSStorage(
		"https://oss-cn-hangzhou.aliyuncs.com",
		"cn-hangzhou",
		"TEST_ACCESS_KEY_ID",
		"TEST_ACCESS_KEY_SECRET",
		"test-public-bucket",
		"test-private-bucket",
	)
	require.NoError(t, err)

	req := DirectUploadRequest{
		Bucket:        s.PublicBucket(),
		ObjectKey:     "merchant/dish/1/20260318/up_abc.jpeg",
		ContentType:   "image/jpeg",
		ContentLength: 102400,
		ExpireIn:      15 * time.Minute,
	}

	result, err := s.CreateDirectUpload(context.Background(), req)
	require.NoError(t, err)

	// 验证上传地址格式
	assert.Equal(t, "https://test-public-bucket.oss-cn-hangzhou.aliyuncs.com", result.UploadHost)

	// 验证必要的表单字段存在且非空
	fields := result.FormFields
	assert.Equal(t, req.ObjectKey, fields["key"])
	assert.Equal(t, "TEST_ACCESS_KEY_ID", fields["OSSAccessKeyId"])
	assert.NotEmpty(t, fields["policy"], "policy 字段不应为空")
	assert.NotEmpty(t, fields["Signature"], "Signature 字段不应为空")
	assert.Equal(t, "image/jpeg", fields["Content-Type"])
	assert.Equal(t, "200", fields["success_action_status"])
}

func TestOSSStorage_CreateDirectUpload_UploadHostScheme(t *testing.T) {
	// 验证当 endpoint 无 scheme 时仍能正确构造上传地址
	s, err := NewOSSStorage(
		"oss-cn-beijing.aliyuncs.com", // 无 https:// 前缀
		"cn-beijing",
		"AK", "SK",
		"bucket", "private-bucket",
	)
	require.NoError(t, err)

	result, err := s.CreateDirectUpload(context.Background(), DirectUploadRequest{
		Bucket:        "bucket",
		ObjectKey:     "dir/file.jpg",
		ContentType:   "image/jpeg",
		ContentLength: 1,
		ExpireIn:      time.Minute,
	})
	require.NoError(t, err)
	assert.Equal(t, "https://bucket.oss-cn-beijing.aliyuncs.com", result.UploadHost)
}

// ---------------------------------------------------------------------------
// 集成测试（需要真实 OSS 环境，通过 OSS_ACCESS_KEY_ID 环境变量启用）
// ---------------------------------------------------------------------------

// newOSSStorageFromEnv 从环境变量读取 OSS 配置，若未设置则跳过测试。
func newOSSStorageFromEnv(t *testing.T) *OSSStorage {
	t.Helper()
	akID := os.Getenv("OSS_ACCESS_KEY_ID")
	if akID == "" {
		t.Skip("跳过 OSS 集成测试：未设置 OSS_ACCESS_KEY_ID 环境变量")
	}

	endpoint := os.Getenv("OSS_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://oss-cn-hangzhou.aliyuncs.com"
	}
	region := os.Getenv("OSS_REGION")
	if region == "" {
		region = "cn-beijing"
	}
	publicBucket := os.Getenv("OSS_PUBLIC_BUCKET")
	privateBucket := os.Getenv("OSS_PRIVATE_BUCKET")
	if publicBucket == "" || privateBucket == "" {
		t.Skip("跳过 OSS 集成测试：未设置 OSS_PUBLIC_BUCKET / OSS_PRIVATE_BUCKET")
	}

	s, err := NewOSSStorage(endpoint, region, akID, os.Getenv("OSS_ACCESS_KEY_SECRET"), publicBucket, privateBucket)
	require.NoError(t, err)
	return s
}

// testObjectKey 生成带时间戳的唯一测试对象 key，隔离测试产生的脏数据。
func testObjectKey(suffix string) string {
	return fmt.Sprintf("_test/integration/%d/%s", time.Now().UnixNano(), suffix)
}

// uploadViaPostPolicy 用 OSS POST Policy 表单字段真实上传一个小文件。
// 返回 HTTP Status Code，方便断言。
func uploadViaPostPolicy(t *testing.T, result DirectUploadResult, objectKey, contentType string, body []byte) int {
	t.Helper()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// 先写所有表单字段（key 必须在 file 字段之前）
	for fieldName, fieldVal := range result.FormFields {
		if fieldName == "key" {
			// key 已在下面写
			continue
		}
		fw, err := mw.CreateFormField(fieldName)
		require.NoError(t, err)
		_, err = fw.Write([]byte(fieldVal))
		require.NoError(t, err)
	}
	// key 字段
	fw, err := mw.CreateFormField("key")
	require.NoError(t, err)
	_, err = fw.Write([]byte(objectKey))
	require.NoError(t, err)

	// file 字段：显式设置 Content-Type 让 OSS POST Policy 条件匹配
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="test.jpg"`)
	h.Set("Content-Type", contentType)
	fw, err = mw.CreatePart(h)
	require.NoError(t, err)
	_, err = fw.Write(body)
	require.NoError(t, err)

	require.NoError(t, mw.Close())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, result.UploadHost, &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Logf("OSS POST Policy 响应 %d: %s", resp.StatusCode, string(body))
	}
	return resp.StatusCode
}

// TestOSSStorage_StatObject_NotFound 验证不存在的 key 返回 ErrObjectNotFound。
func TestOSSStorage_StatObject_NotFound(t *testing.T) {
	s := newOSSStorageFromEnv(t)
	ctx := context.Background()

	_, err := s.StatObject(ctx, s.PublicBucket(), "_test/nonexistent/should_not_exist_ever.jpg")
	require.ErrorIs(t, err, ErrObjectNotFound)
}

// TestOSSStorage_FullRoundTrip_PublicBucket 全流程集成测试（公开桶）：
//  1. CreateDirectUpload → 生成 POST Policy 表单字段
//  2. 实际 HTTP POST 上传小图片到 OSS
//  3. StatObject → 验证 Size/ContentType/ETag
//  4. DeleteObject → 清理，再 StatObject 确认已删除
func TestOSSStorage_FullRoundTrip_PublicBucket(t *testing.T) {
	s := newOSSStorageFromEnv(t)
	ctx := context.Background()

	objectKey := testObjectKey("public/test.jpg")
	const contentType = "image/jpeg"

	// 最小合法 JPEG（2×2 像素，SOI+EOI 标记）
	minimalJPEG := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
	}

	// 1. 生成直传凭证
	uploadResult, err := s.CreateDirectUpload(ctx, DirectUploadRequest{
		Bucket:        s.PublicBucket(),
		ObjectKey:     objectKey,
		ContentType:   contentType,
		ContentLength: int64(len(minimalJPEG)),
		ExpireIn:      5 * time.Minute,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, uploadResult.UploadHost)
	assert.Contains(t, uploadResult.UploadHost, s.PublicBucket())

	// 2. 实际上传
	statusCode := uploadViaPostPolicy(t, uploadResult, objectKey, contentType, minimalJPEG)
	assert.Equal(t, http.StatusOK, statusCode, "OSS POST Policy 直传应返回 200")

	// 3. StatObject 验证
	meta, err := s.StatObject(ctx, s.PublicBucket(), objectKey)
	require.NoError(t, err)
	assert.Equal(t, int64(len(minimalJPEG)), meta.Size)
	assert.Contains(t, strings.ToLower(meta.ContentType), "image")
	assert.NotEmpty(t, meta.ETag)

	// 4. DeleteObject + 验证已删除（defer 确保清理，即使前面断言失败）
	t.Cleanup(func() {
		_ = s.DeleteObject(ctx, s.PublicBucket(), objectKey)
	})
	require.NoError(t, s.DeleteObject(ctx, s.PublicBucket(), objectKey))
	_, err = s.StatObject(ctx, s.PublicBucket(), objectKey)
	require.ErrorIs(t, err, ErrObjectNotFound, "删除后 StatObject 应返回 ErrObjectNotFound")
}

// TestOSSStorage_FullRoundTrip_PrivateBucket 私有桶全流程：上传 → Presign → 验证 URL 格式 → 删除。
func TestOSSStorage_FullRoundTrip_PrivateBucket(t *testing.T) {
	s := newOSSStorageFromEnv(t)
	ctx := context.Background()

	objectKey := testObjectKey("private/test.jpg")
	const contentType = "image/jpeg"
	minimalJPEG := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
	}

	// 1. 上传到私有桶
	uploadResult, err := s.CreateDirectUpload(ctx, DirectUploadRequest{
		Bucket:        s.PrivateBucket(),
		ObjectKey:     objectKey,
		ContentType:   contentType,
		ContentLength: int64(len(minimalJPEG)),
		ExpireIn:      5 * time.Minute,
	})
	require.NoError(t, err)

	statusCode := uploadViaPostPolicy(t, uploadResult, objectKey, contentType, minimalJPEG)
	assert.Equal(t, http.StatusOK, statusCode)

	t.Cleanup(func() { _ = s.DeleteObject(ctx, s.PrivateBucket(), objectKey) })

	// 2. HeadObject 验证
	meta, err := s.StatObject(ctx, s.PrivateBucket(), objectKey)
	require.NoError(t, err)
	assert.Equal(t, int64(len(minimalJPEG)), meta.Size)

	// 3. CreatePrivateDownloadURL → Presign URL 格式校验
	ttl := 5 * time.Minute
	presignedURL, err := s.CreatePrivateDownloadURL(ctx, s.PrivateBucket(), objectKey, ttl)
	require.NoError(t, err)
	assert.NotEmpty(t, presignedURL)
	// URL 必须是 HTTPS 且包含对象 key
	assert.True(t, strings.HasPrefix(presignedURL, "https://"), "presigned URL 应以 https:// 开头")
	assert.Contains(t, presignedURL, s.PrivateBucket(), "presigned URL 应包含 bucket 名称")

	// 4. Presign URL 可以用匿名 HTTP GET 访问（验证签名有效）
	getResp, err := http.Get(presignedURL) //nolint:noctx
	require.NoError(t, err)
	defer getResp.Body.Close()
	assert.Equal(t, http.StatusOK, getResp.StatusCode, "Presign URL 应可匿名 GET 访问")
}

// TestOSSStorage_DeleteObject_Idempotent 删除不存在的对象不应报错（OSS 行为是幂等的）。
func TestOSSStorage_DeleteObject_Idempotent(t *testing.T) {
	s := newOSSStorageFromEnv(t)
	ctx := context.Background()

	err := s.DeleteObject(ctx, s.PublicBucket(), "_test/nonexistent/ghost_file.jpg")
	// OSS DeleteObject 对不存在的 key 返回 204，SDK 不应报错
	require.NoError(t, err)
}
