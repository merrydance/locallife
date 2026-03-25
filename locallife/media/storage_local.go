package media

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// LocalStorage 是仅供开发环境使用的 ObjectStorage 实现。
//
// 工作原理：
//   - CreateDirectUpload 返回后端自身的 /v1/media/_devupload 地址作为上传目标。
//   - 后端在 api/server.go 中（仅 FILE_STORAGE_PROVIDER=local 时）注册该路由，
//     接收文件并保存到本地 uploads/dev/ 目录。
//   - StatObject、DeleteObject 直接操作本地文件系统。
//   - CreatePrivateDownloadURL 返回本地开发访问 URL，由 /dev/uploads/* 路由直接读取文件。
//
// 注意：
//   - 此实现绝对不会出现在生产配置中（FILE_STORAGE_PROVIDER=oss 时不会创建此实例）。
//   - 客户端代码与生产路径完全相同（upload-sessions → POST → complete），无需感知差异。
type LocalStorage struct {
	serverBaseURL string // 后端服务基地址，如 http://localhost:8080
	baseDir       string // 本地存储根目录，如 ./uploads/dev
}

// NewLocalStorage 创建 LocalStorage 实例。
// serverBaseURL 是后端服务地址（用于生成直传目标 URL）。
// baseDir 是本地文件存储目录。
func NewLocalStorage(serverBaseURL, baseDir string) *LocalStorage {
	return &LocalStorage{
		serverBaseURL: serverBaseURL,
		baseDir:       baseDir,
	}
}

func (s *LocalStorage) PublicBucket() string  { return "local-public" }
func (s *LocalStorage) PrivateBucket() string { return "local-private" }

// CreateDirectUpload 返回后端自身的 devupload 路由作为直传目标。
// FormFields 包含 key（objectKey）和一个简单令牌，后端路由验证此令牌防止意外公开使用。
func (s *LocalStorage) CreateDirectUpload(_ context.Context, req DirectUploadRequest) (DirectUploadResult, error) {
	return DirectUploadResult{
		UploadHost: s.serverBaseURL + "/v1/media/_devupload",
		FormFields: map[string]string{
			"key":         req.ObjectKey,
			"_dev_bucket": req.Bucket,
		},
	}, nil
}

// StatObject 检查本地文件是否存在并读取元信息。
func (s *LocalStorage) StatObject(_ context.Context, _ string, objectKey string) (ObjectMetadata, error) {
	localPath := filepath.Join(s.baseDir, filepath.FromSlash(objectKey))
	info, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ObjectMetadata{}, ErrObjectNotFound
		}
		return ObjectMetadata{}, fmt.Errorf("media: stat local file %s: %w", localPath, err)
	}
	return ObjectMetadata{
		Size: info.Size(),
	}, nil
}

// ReadObject 直接打开本地对象文件并返回可读流。
func (s *LocalStorage) ReadObject(_ context.Context, _ string, objectKey string) (io.ReadCloser, error) {
	localPath := filepath.Join(s.baseDir, filepath.FromSlash(objectKey))
	file, err := os.Open(localPath) //nolint:gosec // objectKey is generated server-side
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("media: open local object %s: %w", localPath, err)
	}
	return file, nil
}

// CreatePrivateDownloadURL 返回本地开发访问 URL。
// 本地模式下 TTL 仅用于保持与生产签名接口一致的调用约定；实际不做过期校验。
func (s *LocalStorage) CreatePrivateDownloadURL(_ context.Context, _ string, objectKey string, _ time.Duration) (string, error) {
	return fmt.Sprintf("%s/dev/uploads/%s", s.serverBaseURL, objectKey), nil
}

// DeleteObject 删除本地文件。
func (s *LocalStorage) DeleteObject(_ context.Context, _ string, objectKey string) error {
	localPath := filepath.Join(s.baseDir, filepath.FromSlash(objectKey))
	if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("media: delete local file %s: %w", localPath, err)
	}
	return nil
}

// PutObject 将服务端生成的对象直接写入本地开发存储。
func (s *LocalStorage) PutObject(_ context.Context, _ string, objectKey string, _ string, body io.Reader, _ int64) error {
	destPath := filepath.Join(s.baseDir, filepath.FromSlash(objectKey))
	if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
		return fmt.Errorf("media: create local object dir %s: %w", filepath.Dir(destPath), err)
	}

	dest, err := os.Create(destPath) //nolint:gosec // objectKey is generated server-side
	if err != nil {
		return fmt.Errorf("media: create local object %s: %w", destPath, err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, body); err != nil {
		return fmt.Errorf("media: write local object %s: %w", destPath, err)
	}
	return nil
}

// DevUploadHandler 返回一个 http.HandlerFunc，接收开发环境的直传文件请求。
// 只应在 FILE_STORAGE_PROVIDER=local 时注册到路由，注册路径为 POST /v1/media/_devupload。
func (s *LocalStorage) DevUploadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "bad multipart", http.StatusBadRequest)
			return
		}

		objectKey := r.FormValue("key")
		if objectKey == "" {
			http.Error(w, "missing key", http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		destPath := filepath.Join(s.baseDir, filepath.FromSlash(objectKey))
		if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		dest, err := os.Create(destPath) //nolint:gosec // path is constructed from server-side objectKey, not user input
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer dest.Close()

		buf := make([]byte, 32*1024)
		for {
			n, rerr := file.Read(buf)
			if n > 0 {
				if _, werr := dest.Write(buf[:n]); werr != nil {
					http.Error(w, "write error", http.StatusInternalServerError)
					return
				}
			}
			if rerr != nil {
				break
			}
		}

		w.WriteHeader(http.StatusOK)
	}
}
