package media

import "errors"

var (
	// ErrObjectNotFound 表示对象在存储后端不存在（StatObject、CreatePrivateDownloadURL 可能返回）。
	ErrObjectNotFound = errors.New("media: object not found in storage")

	// ErrUploadSessionNotFound 表示 upload_id 对应的上传会话不存在或已过期被清理。
	ErrUploadSessionNotFound = errors.New("media: upload session not found")

	// ErrUploadSessionExpired 表示上传会话存在但 POST Policy 已过期。
	ErrUploadSessionExpired = errors.New("media: upload session expired")

	// ErrUploadNotConfirmed 表示客户端尚未完成实际上传（complete 阶段 StatObject 失败）。
	ErrUploadNotConfirmed = errors.New("media: object not yet uploaded to storage")

	// ErrAssetNotFound 表示 media_assets 记录不存在。
	ErrAssetNotFound = errors.New("media: asset not found")

	// ErrAssetDeleted 表示媒体资产已被标记为 deleted。
	ErrAssetDeleted = errors.New("media: asset has been deleted")

	// ErrUnauthorized 表示当前用户无权访问该媒体资产（私有资产鉴权失败）。
	ErrUnauthorized = errors.New("media: unauthorized access to asset")

	// ErrInvalidCategory 表示 media_category 不在已知枚举中。
	ErrInvalidCategory = errors.New("media: invalid media category")

	// ErrFileTooLarge 表示文件大小超出配置上限。
	ErrFileTooLarge = errors.New("media: file size exceeds limit")

	// ErrUnsupportedContentType 表示 Content-Type 对于该 category 不被允许。
	ErrUnsupportedContentType = errors.New("media: unsupported content type for this category")
)
