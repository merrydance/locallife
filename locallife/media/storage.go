package media

import (
	"context"
	"io"
	"time"
)

// DirectUploadRequest 是申请直传凭证的参数。
type DirectUploadRequest struct {
	Bucket        string
	ObjectKey     string
	ContentType   string
	ContentLength int64
	ExpireIn      time.Duration
}

// DirectUploadResult 是返回给客户端的直传信息。
// 客户端使用 UploadHost + FormFields 向 OSS 发起 multipart/form-data POST。
type DirectUploadResult struct {
	UploadHost string            // POST 目标地址
	FormFields map[string]string // 需要附带的表单字段（policy、signature 等）
}

// ObjectMetadata 是对象的元信息。
type ObjectMetadata struct {
	ContentType string
	Size        int64
	ETag        string
}

// ObjectStorage 定义与对象存储后端的交互接口。
// 生产实现：OSSStorage（阿里云 OSS）
// 开发实现：LocalStorage（后端本地接收，仅开发环境）
type ObjectStorage interface {
	// CreateDirectUpload 生成客户端直传凭证（POST Policy 或等价机制）。
	CreateDirectUpload(ctx context.Context, req DirectUploadRequest) (DirectUploadResult, error)

	// StatObject 确认对象是否存在并读取元信息。
	// complete 阶段用于验证客户端确实已上传到 OSS。
	StatObject(ctx context.Context, bucket, objectKey string) (ObjectMetadata, error)

	// CreatePrivateDownloadURL 为私有桶对象签发短期访问地址。
	CreatePrivateDownloadURL(ctx context.Context, bucket, objectKey string, ttl time.Duration) (string, error)

	// DeleteObject 删除指定对象。通常由异步任务调用。
	DeleteObject(ctx context.Context, bucket, objectKey string) error

	// PutObject 由服务端直接写入对象存储，适用于二维码等后端生成文件。
	PutObject(ctx context.Context, bucket, objectKey string, contentType string, body io.Reader, contentLength int64) error

	// PublicBucket 返回公共桶名称。
	PublicBucket() string

	// PrivateBucket 返回私有桶名称。
	PrivateBucket() string
}
