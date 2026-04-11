package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const localDevUploadMaxBytes int64 = 64 << 20

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

type rootedReadCloser struct {
	file *os.File
	root *os.Root
}

func (r *rootedReadCloser) Read(p []byte) (int, error) {
	return r.file.Read(p)
}

func (r *rootedReadCloser) Close() error {
	fileErr := r.file.Close()
	rootErr := r.root.Close()
	if fileErr != nil {
		return fileErr
	}
	return rootErr
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

func (s *LocalStorage) openRoot() (*os.Root, error) {
	return os.OpenRoot(s.baseDir)
}

func mkdirAllInRoot(root *os.Root, dir string) error {
	cleaned := path.Clean(strings.ReplaceAll(dir, "\\", "/"))
	if cleaned == "." || cleaned == "" {
		return nil
	}

	parts := strings.Split(cleaned, "/")
	current := ""
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if current == "" {
			current = part
		} else {
			current = path.Join(current, part)
		}
		if err := root.Mkdir(current, 0750); err != nil && !os.IsExist(err) {
			return err
		}
	}

	return nil
}

func normalizeLocalObjectKey(objectKey string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(objectKey, "\\", "/"))
	if trimmed == "" {
		return "", fmt.Errorf("empty object key")
	}
	if strings.HasPrefix(trimmed, "/") {
		return "", fmt.Errorf("absolute object key is not allowed")
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("invalid object key")
	}
	return cleaned, nil
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
	relPath, err := normalizeLocalObjectKey(objectKey)
	if err != nil {
		return ObjectMetadata{}, fmt.Errorf("media: normalize local object key: %w", err)
	}
	root, err := s.openRoot()
	if err != nil {
		return ObjectMetadata{}, fmt.Errorf("media: open local storage root: %w", err)
	}
	defer root.Close()

	localPath := filepath.FromSlash(relPath)
	info, err := root.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ObjectMetadata{}, ErrObjectNotFound
		}
		return ObjectMetadata{}, fmt.Errorf("media: stat local file %s: %w", relPath, err)
	}
	return ObjectMetadata{
		Size: info.Size(),
	}, nil
}

// ReadObject 直接打开本地对象文件并返回可读流。
func (s *LocalStorage) ReadObject(_ context.Context, _ string, objectKey string) (io.ReadCloser, error) {
	relPath, err := normalizeLocalObjectKey(objectKey)
	if err != nil {
		return nil, fmt.Errorf("media: normalize local object key: %w", err)
	}
	root, err := s.openRoot()
	if err != nil {
		return nil, fmt.Errorf("media: open local storage root: %w", err)
	}
	localPath := filepath.FromSlash(relPath)
	file, err := root.Open(localPath)
	if err != nil {
		_ = root.Close()
		if os.IsNotExist(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("media: open local object %s: %w", relPath, err)
	}
	return &rootedReadCloser{file: file, root: root}, nil
}

// CreatePrivateDownloadURL 返回本地开发访问 URL。
// 本地模式下 TTL 仅用于保持与生产签名接口一致的调用约定；实际不做过期校验。
func (s *LocalStorage) CreatePrivateDownloadURL(_ context.Context, _ string, objectKey string, _ time.Duration) (string, error) {
	return fmt.Sprintf("%s/dev/uploads/%s", s.serverBaseURL, objectKey), nil
}

// DeleteObject 删除本地文件。
func (s *LocalStorage) DeleteObject(_ context.Context, _ string, objectKey string) error {
	relPath, err := normalizeLocalObjectKey(objectKey)
	if err != nil {
		return fmt.Errorf("media: normalize local object key: %w", err)
	}
	root, err := s.openRoot()
	if err != nil {
		return fmt.Errorf("media: open local storage root: %w", err)
	}
	defer root.Close()

	localPath := filepath.FromSlash(relPath)
	if err := root.Remove(localPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("media: delete local file %s: %w", relPath, err)
	}
	return nil
}

// PutObject 将服务端生成的对象直接写入本地开发存储。
func (s *LocalStorage) PutObject(_ context.Context, _ string, objectKey string, _ string, body io.Reader, _ int64) error {
	relPath, err := normalizeLocalObjectKey(objectKey)
	if err != nil {
		return fmt.Errorf("media: normalize local object key: %w", err)
	}
	root, err := s.openRoot()
	if err != nil {
		return fmt.Errorf("media: open local storage root: %w", err)
	}
	defer root.Close()

	destPath := filepath.FromSlash(relPath)
	if err := mkdirAllInRoot(root, filepath.ToSlash(filepath.Dir(destPath))); err != nil {
		return fmt.Errorf("media: create local object dir %s: %w", filepath.Dir(relPath), err)
	}

	dest, err := root.Create(destPath)
	if err != nil {
		return fmt.Errorf("media: create local object %s: %w", relPath, err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, body); err != nil {
		return fmt.Errorf("media: write local object %s: %w", relPath, err)
	}
	return nil
}

// DevUploadHandler 返回一个 http.HandlerFunc，接收开发环境的直传文件请求。
// 只应在 FILE_STORAGE_PROVIDER=local 时注册到路由，注册路径为 POST /v1/media/_devupload。
func (s *LocalStorage) DevUploadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, localDevUploadMaxBytes)
		reader, err := r.MultipartReader()
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "bad multipart", http.StatusBadRequest)
			return
		}

		objectKey := ""
		sawFile := false

		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, "bad multipart", http.StatusBadRequest)
				return
			}

			switch part.FormName() {
			case "key":
				keyBytes, err := io.ReadAll(io.LimitReader(part, 4096))
				_ = part.Close()
				if err != nil {
					http.Error(w, "bad multipart", http.StatusBadRequest)
					return
				}
				objectKey, err = normalizeLocalObjectKey(string(keyBytes))
				if err != nil {
					http.Error(w, "invalid key", http.StatusBadRequest)
					return
				}
			case "file":
				if objectKey == "" {
					_ = part.Close()
					http.Error(w, "missing key", http.StatusBadRequest)
					return
				}
				root, err := s.openRoot()
				if err != nil {
					_ = part.Close()
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}

				destPath := filepath.FromSlash(objectKey)
				if err := mkdirAllInRoot(root, filepath.ToSlash(filepath.Dir(destPath))); err != nil {
					_ = part.Close()
					_ = root.Close()
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}

				dest, err := root.Create(destPath)
				if err != nil {
					_ = part.Close()
					_ = root.Close()
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}

				if _, err := io.Copy(dest, part); err != nil {
					var maxBytesErr *http.MaxBytesError
					_ = part.Close()
					_ = dest.Close()
					_ = root.Close()
					if errors.As(err, &maxBytesErr) {
						http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
						return
					}
					http.Error(w, "write error", http.StatusInternalServerError)
					return
				}
				_ = part.Close()
				if err := dest.Close(); err != nil {
					_ = root.Close()
					http.Error(w, "write error", http.StatusInternalServerError)
					return
				}
				if err := root.Close(); err != nil {
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				sawFile = true
			default:
				_ = part.Close()
			}
		}

		if objectKey == "" {
			http.Error(w, "missing key", http.StatusBadRequest)
			return
		}
		if !sawFile {
			http.Error(w, "missing file field", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
