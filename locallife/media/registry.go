package media

import (
	"context"
	"fmt"
	"io"
	"mime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// UploadSessionRequest 是申请上传会话的参数。
type UploadSessionRequest struct {
	UserID         int64
	BusinessType   string
	Category       Category
	ContentType    string
	ContentLength  int64
	ChecksumSha256 string
	SourceClient   string // web / weapp / operator-web
	ExpireIn       time.Duration
	MaxFileBytes   int64 // 来自配置的上限，0 表示不限
}

// UploadSessionResult 是创建上传会话后返回给 API 层的结果。
type UploadSessionResult struct {
	Session      db.MediaUploadSession
	UploadResult DirectUploadResult
}

// CompleteRequest 是确认上传完成的参数。
type CompleteRequest struct {
	UploadID  string
	ObjectKey string // 客户端回传，必须与会话中记录的一致
	ETag      string // OSS 返回的 ETag（可选，用于额外校验）
	UserID    int64  // 当前操作用户，用于归属校验
}

// CompleteResult 是确认完成后的结果。
type CompleteResult struct {
	Asset db.MediaAsset
}

// Registry 负责上传会话生命周期管理和 media_assets 持久化。
//
// 依赖：
//   - db.Store：sqlc 生成的查询接口
//   - ObjectStorage：校验对象是否确实已传到 OSS
//   - Policy：获取 category 元数据
type Registry struct {
	store   db.Store
	storage ObjectStorage
	policy  Policy
}

// NewRegistry 创建 Registry 实例。
func NewRegistry(store db.Store, storage ObjectStorage) *Registry {
	return &Registry{
		store:   store,
		storage: storage,
		policy:  Policy{},
	}
}

// CreateUploadSession 申请一个新的直传会话。
//
// 幂等语义：若同一 user + category + checksum 已有 pending 会话，则直接复用，
// 避免对同一文件重复申请。
func (r *Registry) CreateUploadSession(ctx context.Context, req UploadSessionRequest) (UploadSessionResult, error) {
	if err := r.policy.IsAllowedContentType(req.Category, req.ContentType); err != nil {
		return UploadSessionResult{}, err
	}
	if req.MaxFileBytes > 0 && req.ContentLength > req.MaxFileBytes {
		return UploadSessionResult{}, fmt.Errorf("%w: %d > %d",
			ErrFileTooLarge, req.ContentLength, req.MaxFileBytes)
	}

	visibility, _, err := r.policy.Lookup(req.Category)
	if err != nil {
		return UploadSessionResult{}, err
	}

	// 幂等：复用已有的 pending 会话
	if existing, iErr := r.store.GetPendingUploadSessionByIdempotencyKey(ctx, db.GetPendingUploadSessionByIdempotencyKeyParams{
		UserID:         req.UserID,
		MediaCategory:  string(req.Category),
		ChecksumSha256: req.ChecksumSha256,
	}); iErr == nil {
		// 复用时重新生成直传凭证（policy 可能已过期）
		uploadResult, uErr := r.createDirectUpload(ctx, req, existing.ObjectKey)
		if uErr != nil {
			return UploadSessionResult{}, uErr
		}
		return UploadSessionResult{Session: existing, UploadResult: uploadResult}, nil
	}

	// 生成 upload_id（"up_" + UUID v4）
	uploadID := "up_" + uuid.NewString()

	// 生成 object key
	ext := extFromContentType(req.ContentType)
	objectKey, err := r.policy.ObjectKey(req.Category, req.UserID, uploadID, ext, time.Now())
	if err != nil {
		return UploadSessionResult{}, err
	}

	// 确定 bucket
	bucket, err := r.policy.BucketFor(req.Category, r.storage)
	if err != nil {
		return UploadSessionResult{}, err
	}

	expireIn := req.ExpireIn
	if expireIn <= 0 {
		expireIn = 30 * time.Minute
	}
	expireAt := time.Now().Add(expireIn)

	// 创建数据库会话记录
	session, err := r.store.CreateUploadSession(ctx, db.CreateUploadSessionParams{
		ID:             uploadID,
		UserID:         req.UserID,
		BusinessType:   req.BusinessType,
		MediaCategory:  string(req.Category),
		Visibility:     string(visibility),
		ObjectKey:      objectKey,
		ChecksumSha256: req.ChecksumSha256,
		ContentType:    req.ContentType,
		ContentLength:  req.ContentLength,
		ExpireAt:       expireAt,
	})
	if err != nil {
		return UploadSessionResult{}, fmt.Errorf("media: create upload session: %w", err)
	}

	uploadResult, err := r.createDirectUpload(ctx, req, objectKey)
	if err != nil {
		return UploadSessionResult{}, err
	}

	_ = bucket // bucket 已通过 policy.BucketFor 确认，objectKey 已包含路径信息
	return UploadSessionResult{Session: session, UploadResult: uploadResult}, nil
}

// createDirectUpload 向 ObjectStorage 申请直传凭证。
func (r *Registry) createDirectUpload(ctx context.Context, req UploadSessionRequest, objectKey string) (DirectUploadResult, error) {
	bucket, err := r.policy.BucketFor(req.Category, r.storage)
	if err != nil {
		return DirectUploadResult{}, err
	}
	expireIn := req.ExpireIn
	if expireIn <= 0 {
		expireIn = 30 * time.Minute
	}
	return r.storage.CreateDirectUpload(ctx, DirectUploadRequest{
		Bucket:        bucket,
		ObjectKey:     objectKey,
		ContentType:   req.ContentType,
		ContentLength: req.ContentLength,
		ExpireIn:      expireIn,
	})
}

// CompleteUpload 确认客户端已上传完成，持久化 media_asset 记录。
//
// 幂等语义：若会话已 completed，直接返回对应 media_asset。
func (r *Registry) CompleteUpload(ctx context.Context, req CompleteRequest) (CompleteResult, error) {
	session, err := r.store.GetUploadSession(ctx, req.UploadID)
	if err != nil {
		return CompleteResult{}, ErrUploadSessionNotFound
	}

	// 归属校验
	if session.UserID != req.UserID {
		return CompleteResult{}, ErrUnauthorized
	}

	// object_key 一致性校验
	if req.ObjectKey != "" && session.ObjectKey != req.ObjectKey {
		return CompleteResult{}, fmt.Errorf("media: object_key mismatch: got %s, expected %s",
			req.ObjectKey, session.ObjectKey)
	}

	// 幂等：已完成的直接返回
	if session.Status == "completed" && session.MediaAssetID.Valid {
		asset, aErr := r.store.GetMediaAssetByID(ctx, session.MediaAssetID.Int64)
		if aErr == nil {
			return CompleteResult{Asset: asset}, nil
		}
	}

	// 会话过期检查
	if session.Status == "expired" || time.Now().After(session.ExpireAt) {
		return CompleteResult{}, ErrUploadSessionExpired
	}

	// 校验对象确实已传到 OSS
	bucket, err := r.policy.BucketFor(Category(session.MediaCategory), r.storage)
	if err != nil {
		return CompleteResult{}, err
	}
	meta, err := r.storage.StatObject(ctx, bucket, session.ObjectKey)
	if err != nil {
		return CompleteResult{}, ErrUploadNotConfirmed
	}

	// 写入 media_assets
	asset, err := r.store.CreateMediaAsset(ctx, db.CreateMediaAssetParams{
		ObjectKey:      session.ObjectKey,
		Visibility:     session.Visibility,
		MediaCategory:  session.MediaCategory,
		MimeType:       session.ContentType,
		FileSize:       meta.Size,
		ChecksumSha256: session.ChecksumSha256,
		UploadedBy:     session.UserID,
		SourceClient:   session.BusinessType,
	})
	if err != nil {
		// 唯一键冲突（同 object_key 重复）视为幂等成功
		if existing, kErr := r.store.GetMediaAssetByObjectKey(ctx, session.ObjectKey); kErr == nil {
			asset = existing
		} else {
			return CompleteResult{}, fmt.Errorf("media: create media asset: %w", err)
		}
	}

	// 更新上传会话状态
	_, _ = r.store.CompleteUploadSession(ctx, db.CompleteUploadSessionParams{
		ID:           session.ID,
		MediaAssetID: pgtype.Int8{Int64: asset.ID, Valid: true},
	})

	return CompleteResult{Asset: asset}, nil
}

// GetAsset 按 media_asset_id 查询媒体资产。
func (r *Registry) GetAsset(ctx context.Context, id int64) (db.MediaAsset, error) {
	asset, err := r.store.GetMediaAssetByID(ctx, id)
	if err != nil {
		return db.MediaAsset{}, ErrAssetNotFound
	}
	if asset.UploadStatus == "deleted" {
		return db.MediaAsset{}, ErrAssetDeleted
	}
	return asset, nil
}

// SoftDelete 软删除媒体资产。
func (r *Registry) SoftDelete(ctx context.Context, id, callerUserID int64) error {
	asset, err := r.store.GetMediaAssetByID(ctx, id)
	if err != nil {
		return ErrAssetNotFound
	}
	if asset.UploadedBy != callerUserID {
		return ErrUnauthorized
	}
	_, err = r.store.SoftDeleteMediaAsset(ctx, id)
	if err != nil {
		return fmt.Errorf("media: soft delete asset %d: %w", id, err)
	}
	return nil
}

// CreatePrivateAccessURL 为私有媒体资产签发短期下载地址。
// 调用方负责业务鉴权，此方法只负责签名。
func (r *Registry) CreatePrivateAccessURL(ctx context.Context, id int64, ttl time.Duration) (string, error) {
	asset, err := r.GetAsset(ctx, id)
	if err != nil {
		return "", err
	}
	if asset.Visibility != string(VisibilityPrivate) {
		return "", fmt.Errorf("media: asset %d is not private", id)
	}
	return r.storage.CreatePrivateDownloadURL(ctx, r.storage.PrivateBucket(), asset.ObjectKey, ttl)
}

// extFromContentType 从 MIME 类型推断常见扩展名。
func extFromContentType(ct string) string {
	exts, err := mime.ExtensionsByType(ct)
	if err != nil || len(exts) == 0 {
		// 对常见图片类型提供回退
		switch ct {
		case "image/jpeg":
			return ".jpg"
		case "image/png":
			return ".png"
		case "image/webp":
			return ".webp"
		case "image/heic":
			return ".heic"
		}
		return ""
	}
	// mime 库返回的扩展名可能包含多个，优先选择短的常见扩展
	for _, e := range exts {
		if e == ".jpg" || e == ".jpeg" || e == ".png" || e == ".webp" {
			return e
		}
	}
	return exts[0]
}

// maxDownloadBytes 是服务端内部读取媒体对象允许读取的最大字节数（10 MB）。
// 超过此限制视为异常，拒绝继续读取以防止 OOM。
const maxDownloadBytes = 10 << 20

// ReadMediaAsset 以服务端内部方式读取媒体资产字节流，不通过公开 URL 回读。
// 返回数据库中记录的 mime_type 与对象字节流，供 OCR 等内部处理场景使用。
func (r *Registry) ReadMediaAsset(ctx context.Context, assetID int64) ([]byte, string, error) {
	asset, err := r.GetAsset(ctx, assetID)
	if err != nil {
		return nil, "", err
	}

	bucket := r.storage.PublicBucket()
	if asset.Visibility == string(VisibilityPrivate) {
		bucket = r.storage.PrivateBucket()
	}

	body, err := r.storage.ReadObject(ctx, bucket, asset.ObjectKey)
	if err != nil {
		return nil, "", fmt.Errorf("read media asset %d: %w", assetID, err)
	}
	defer body.Close()

	data, err := io.ReadAll(io.LimitReader(body, maxDownloadBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read media asset %d body: %w", assetID, err)
	}
	if int64(len(data)) > maxDownloadBytes {
		return nil, "", fmt.Errorf("media asset %d exceeds max download size (%d bytes)", assetID, maxDownloadBytes)
	}

	return data, asset.MimeType, nil
}

// DownloadObject 下载指定媒体资产的原始内容。
// 返回推荐文件名（object_key 末段）和文件字节流。
// 主要用于需要将媒体二进制数据转发给第三方（如微信进件图片上传）的场景。
func (r *Registry) DownloadObject(ctx context.Context, assetID int64) (filename string, data []byte, err error) {
	asset, err := r.store.GetMediaAssetByID(ctx, assetID)
	if err != nil {
		return "", nil, fmt.Errorf("get media asset %d: %w", assetID, err)
	}

	name := asset.ObjectKey
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	data, _, err = r.ReadMediaAsset(ctx, assetID)
	if err != nil {
		return "", nil, err
	}
	return name, data, nil
}
