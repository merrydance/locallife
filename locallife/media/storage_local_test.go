package media

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLocalStorage_Buckets(t *testing.T) {
	ls := NewLocalStorage("http://localhost:8080", "/tmp/test")
	require.Equal(t, "local-public", ls.PublicBucket())
	require.Equal(t, "local-private", ls.PrivateBucket())
}

func TestLocalStorage_CreateDirectUpload(t *testing.T) {
	ls := NewLocalStorage("http://localhost:8080", "/tmp/test")
	req := DirectUploadRequest{
		Bucket:      "local-public",
		ObjectKey:   "merchant/dish/1/20260318/up_abc.jpeg",
		ContentType: "image/jpeg",
	}

	result, err := ls.CreateDirectUpload(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:8080/v1/media/_devupload", result.UploadHost)
	require.Equal(t, req.ObjectKey, result.FormFields["key"])
	require.Equal(t, req.Bucket, result.FormFields["_dev_bucket"])
}

func TestLocalStorage_StatObject(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage("http://localhost:8080", dir)

	objectKey := "merchant/dish/1/file.jpg"
	localPath := filepath.Join(dir, filepath.FromSlash(objectKey))
	require.NoError(t, os.MkdirAll(filepath.Dir(localPath), 0750))
	require.NoError(t, os.WriteFile(localPath, []byte("fake image content"), 0600))

	t.Run("existing file returns metadata", func(t *testing.T) {
		meta, err := ls.StatObject(context.Background(), "local-public", objectKey)
		require.NoError(t, err)
		require.Equal(t, int64(len("fake image content")), meta.Size)
	})

	t.Run("missing file returns ErrObjectNotFound", func(t *testing.T) {
		_, err := ls.StatObject(context.Background(), "local-public", "nonexistent/path.jpg")
		require.ErrorIs(t, err, ErrObjectNotFound)
	})
}

func TestLocalStorage_DeleteObject(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage("http://localhost:8080", dir)

	objectKey := "user/review/42/photo.jpg"
	localPath := filepath.Join(dir, filepath.FromSlash(objectKey))
	require.NoError(t, os.MkdirAll(filepath.Dir(localPath), 0750))
	require.NoError(t, os.WriteFile(localPath, []byte("data"), 0600))

	t.Run("deletes existing file", func(t *testing.T) {
		err := ls.DeleteObject(context.Background(), "local-public", objectKey)
		require.NoError(t, err)
		_, statErr := os.Stat(localPath)
		require.True(t, os.IsNotExist(statErr))
	})

	t.Run("deleting nonexistent file is idempotent", func(t *testing.T) {
		err := ls.DeleteObject(context.Background(), "local-public", "does/not/exist.jpg")
		require.NoError(t, err)
	})
}

func TestLocalStorage_CreatePrivateDownloadURL(t *testing.T) {
	ls := NewLocalStorage("http://localhost:8080", "/tmp/test")
	url, err := ls.CreatePrivateDownloadURL(context.Background(), "local-private", "id_card/front/1/file.jpg", 5*time.Minute)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:8080/uploads/id_card/front/1/file.jpg", url)
}

func TestLocalStorage_DevUploadHandler(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStorage("http://localhost:8080", dir)
	handler := ls.DevUploadHandler()

	t.Run("valid upload stores file", func(t *testing.T) {
		objectKey := "merchant/dish/1/20260318/up_test.jpg"
		body, contentType := buildMultipartBody(t, objectKey, "local-public", []byte("image data"))

		req := httptest.NewRequest(http.MethodPost, "/v1/media/_devupload", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		savedPath := filepath.Join(dir, filepath.FromSlash(objectKey))
		data, err := os.ReadFile(savedPath)
		require.NoError(t, err)
		require.Equal(t, []byte("image data"), data)
	})

	t.Run("missing key returns 400", func(t *testing.T) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", "test.jpg")
		_, _ = fw.Write([]byte("data"))
		mw.Close()

		req := httptest.NewRequest(http.MethodPost, "/v1/media/_devupload", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing file field returns 400", func(t *testing.T) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("key", "some/path.jpg")
		_ = mw.WriteField("_dev_bucket", "local-public")
		mw.Close()

		req := httptest.NewRequest(http.MethodPost, "/v1/media/_devupload", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// buildMultipartBody constructs a multipart/form-data body with key, _dev_bucket, and file fields.
func buildMultipartBody(t *testing.T, objectKey, bucket string, fileContent []byte) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("key", objectKey)
	_ = mw.WriteField("_dev_bucket", bucket)
	fw, err := mw.CreateFormFile("file", "upload.jpg")
	require.NoError(t, err)
	_, err = fw.Write(fileContent)
	require.NoError(t, err)
	mw.Close()
	return &buf, mw.FormDataContentType()
}
