package media

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // OSS POST Policy 规范要求使用 SHA1，无替代选项
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

// OSSStorage 是基于阿里云 OSS 的 ObjectStorage 实现（使用 alibabacloud-oss-go-sdk-v2）。
//
// 直传凭证：使用 OSS POST Policy（HMAC-SHA1 签名，OSS 协议要求）。
// 服务端 API：HeadObject / DeleteObject 使用 v2 SDK 客户端（V4 签名）。
// 私有下载：使用 v2 SDK Presign 接口生成短期签名 URL。
type OSSStorage struct {
	client          *oss.Client
	publicBucket    string
	privateBucket   string
	endpoint        string // 完整 endpoint，如 https://oss-cn-hangzhou.aliyuncs.com
	accessKeyID     string
	accessKeySecret string
}

// NewOSSStorage 创建 OSSStorage 实例。
//
// endpoint 格式：https://oss-cn-hangzhou.aliyuncs.com（或不带 scheme 的域名）。
// region 格式：cn-hangzhou（v2 SDK V4 签名必填）。
func NewOSSStorage(endpoint, region, accessKeyID, accessKeySecret, publicBucket, privateBucket string) (*OSSStorage, error) {
	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, accessKeySecret)).
		WithRegion(region).
		WithEndpoint(endpoint)

	client := oss.NewClient(cfg)

	return &OSSStorage{
		client:          client,
		publicBucket:    publicBucket,
		privateBucket:   privateBucket,
		endpoint:        endpoint,
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
	}, nil
}

func (s *OSSStorage) PublicBucket() string  { return s.publicBucket }
func (s *OSSStorage) PrivateBucket() string { return s.privateBucket }

// CreateDirectUpload 生成 OSS POST Policy 直传凭证。
//
// 客户端发 multipart/form-data POST 到 UploadHost，附带 FormFields 中所有字段。
// OSS POST Policy 规范固定使用 HMAC-SHA1 签名，与 SDK 版本无关。
func (s *OSSStorage) CreateDirectUpload(_ context.Context, req DirectUploadRequest) (DirectUploadResult, error) {
	expireAt := time.Now().Add(req.ExpireIn)

	// 构造 Policy 条件
	policy := ossPostPolicy{
		Expiration: expireAt.UTC().Format("2006-01-02T15:04:05Z"),
		Conditions: []interface{}{
			// 限制 object key 必须以指定前缀开头
			[]interface{}{"starts-with", "$key", ossKeyDir(req.ObjectKey)},
			// 限制 content-type 前缀（如 "image/"）
			[]interface{}{"starts-with", "$Content-Type", contentTypePrefix(req.ContentType)},
			// 限制文件大小：0 ~ ContentLength+1
			[]interface{}{"content-length-range", 0, req.ContentLength + 1},
			// 上传成功后 OSS 返回 200
			map[string]interface{}{"success_action_status": "200"},
		},
	}

	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return DirectUploadResult{}, fmt.Errorf("media: marshal oss policy: %w", err)
	}
	policyB64 := base64.StdEncoding.EncodeToString(policyJSON)

	// HMAC-SHA1 签名（OSS POST Policy 规范固定要求 SHA1）
	h := hmac.New(sha1.New, []byte(s.accessKeySecret)) //nolint:gosec
	h.Write([]byte(policyB64))
	sig := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// 构造 POST 上传地址：https://{bucket}.{endpoint_host}
	uploadHost := fmt.Sprintf("https://%s.%s", req.Bucket, stripScheme(s.endpoint))

	formFields := map[string]string{
		"key":                   req.ObjectKey,
		"OSSAccessKeyId":        s.accessKeyID,
		"policy":                policyB64,
		"Signature":             sig,
		"Content-Type":          req.ContentType,
		"success_action_status": "200",
	}

	return DirectUploadResult{
		UploadHost: uploadHost,
		FormFields: formFields,
	}, nil
}

// StatObject 通过 HeadObject 确认对象存在并读取元信息。
// complete 阶段用于验证客户端确实已完成上传。
func (s *OSSStorage) StatObject(ctx context.Context, bucket, objectKey string) (ObjectMetadata, error) {
	result, err := s.client.HeadObject(ctx, &oss.HeadObjectRequest{
		Bucket: oss.Ptr(bucket),
		Key:    oss.Ptr(objectKey),
	})
	if err != nil {
		var serviceErr *oss.ServiceError
		if errors.As(err, &serviceErr) && serviceErr.StatusCode == 404 {
			return ObjectMetadata{}, ErrObjectNotFound
		}
		return ObjectMetadata{}, fmt.Errorf("media: stat object %s: %w", objectKey, err)
	}

	meta := ObjectMetadata{
		Size: result.ContentLength,
	}
	if result.ContentType != nil {
		meta.ContentType = *result.ContentType
	}
	if result.ETag != nil {
		meta.ETag = *result.ETag
	}
	return meta, nil
}

// CreatePrivateDownloadURL 为私有桶对象签发短期 GET 签名 URL（V4 签名）。
func (s *OSSStorage) CreatePrivateDownloadURL(ctx context.Context, bucket, objectKey string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	result, err := s.client.Presign(ctx, &oss.GetObjectRequest{
		Bucket: oss.Ptr(bucket),
		Key:    oss.Ptr(objectKey),
	}, func(o *oss.PresignOptions) {
		o.Expiration = time.Now().Add(ttl)
	})
	if err != nil {
		return "", fmt.Errorf("media: presign private url for %s: %w", objectKey, err)
	}
	return result.URL, nil
}

// DeleteObject 删除 OSS 对象。通常由异步任务调用。
func (s *OSSStorage) DeleteObject(ctx context.Context, bucket, objectKey string) error {
	_, err := s.client.DeleteObject(ctx, &oss.DeleteObjectRequest{
		Bucket: oss.Ptr(bucket),
		Key:    oss.Ptr(objectKey),
	})
	if err != nil {
		return fmt.Errorf("media: delete object %s: %w", objectKey, err)
	}
	return nil
}

// ---- 内部辅助类型与函数 ----

type ossPostPolicy struct {
	Expiration string        `json:"expiration"`
	Conditions []interface{} `json:"conditions"`
}

// ossKeyDir 提取 object key 的目录前缀（最后一个 / 前的部分 + /）。
// 用于 Policy 中的 starts-with 条件，防止客户端上传到任意路径。
func ossKeyDir(objectKey string) string {
	for i := len(objectKey) - 1; i >= 0; i-- {
		if objectKey[i] == '/' {
			return objectKey[:i+1]
		}
	}
	return ""
}

// contentTypePrefix 取 MIME 主类型，如 "image/jpeg" → "image/"。
func contentTypePrefix(ct string) string {
	for i, c := range ct {
		if c == '/' {
			return ct[:i+1]
		}
	}
	return ct
}

// stripScheme 去除 endpoint 中的 https:// 或 http:// 前缀。
func stripScheme(endpoint string) string {
	for _, prefix := range []string{"https://", "http://"} {
		if len(endpoint) > len(prefix) && endpoint[:len(prefix)] == prefix {
			return endpoint[len(prefix):]
		}
	}
	return endpoint
}
