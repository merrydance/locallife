package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// stubStorage 是 ObjectStorage 的内联测试桩。
type stubStorage struct {
	statErr      error
	statMeta     ObjectMetadata
	directResult DirectUploadResult
	privateURL   string
	privateErr   error
	readData     []byte
	readErr      error
}

func (s *stubStorage) CreateDirectUpload(_ context.Context, req DirectUploadRequest) (DirectUploadResult, error) {
	if s.directResult.UploadHost == "" {
		return DirectUploadResult{UploadHost: "http://test-bucket.oss.example.com", FormFields: map[string]string{"key": req.ObjectKey}}, nil
	}
	return s.directResult, nil
}

func (s *stubStorage) StatObject(_ context.Context, _, _ string) (ObjectMetadata, error) {
	return s.statMeta, s.statErr
}

func (s *stubStorage) ReadObject(_ context.Context, _, _ string) (io.ReadCloser, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	return io.NopCloser(bytes.NewReader(s.readData)), nil
}

func (s *stubStorage) CreatePrivateDownloadURL(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return s.privateURL, s.privateErr
}

func (s *stubStorage) DeleteObject(_ context.Context, _, _ string) error { return nil }
func (s *stubStorage) PutObject(_ context.Context, _, _, _ string, _ io.Reader, _ int64) error {
	return nil
}
func (s *stubStorage) PublicBucket() string  { return "test-public" }
func (s *stubStorage) PrivateBucket() string { return "test-private" }

// ==================== CreateUploadSession ====================

func TestRegistry_CreateUploadSession_New(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	storage := &stubStorage{}
	reg := NewRegistry(store, storage)

	const userID int64 = 1
	req := UploadSessionRequest{
		UserID:         userID,
		BusinessType:   "merchant",
		Category:       CategoryDishImage,
		ContentType:    "image/jpeg",
		ContentLength:  500_000,
		ChecksumSha256: "abc123",
		ExpireIn:       30 * time.Minute,
	}

	// 无已有 pending 会话
	store.EXPECT().
		GetPendingUploadSessionByIdempotencyKey(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.MediaUploadSession{}, errors.New("not found"))

	// 创建新会话
	createdSession := db.MediaUploadSession{
		ID:            "up_test-uuid",
		UserID:        userID,
		MediaCategory: string(CategoryDishImage),
		Status:        "pending",
		ExpireAt:      time.Now().Add(30 * time.Minute),
	}
	store.EXPECT().
		CreateUploadSession(gomock.Any(), gomock.Any()).
		Times(1).
		Return(createdSession, nil)

	result, err := reg.CreateUploadSession(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "up_test-uuid", result.Session.ID)
	require.NotEmpty(t, result.UploadResult.UploadHost)
}

func TestRegistry_CreateUploadSession_Idempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	storage := &stubStorage{}
	reg := NewRegistry(store, storage)

	existingSession := db.MediaUploadSession{
		ID:            "up_existing",
		UserID:        1,
		MediaCategory: string(CategoryDishImage),
		ObjectKey:     "dish/1/up_existing/photo.jpg",
		Status:        "pending",
		ExpireAt:      time.Now().Add(30 * time.Minute),
	}

	// 返回已有 pending 会话
	store.EXPECT().
		GetPendingUploadSessionByIdempotencyKey(gomock.Any(), gomock.Any()).
		Times(1).
		Return(existingSession, nil)

	// 不应再创建新会话（Times 0）
	store.EXPECT().
		CreateUploadSession(gomock.Any(), gomock.Any()).
		Times(0)

	result, err := reg.CreateUploadSession(context.Background(), UploadSessionRequest{
		UserID:        1,
		Category:      CategoryDishImage,
		ContentType:   "image/jpeg",
		ContentLength: 100_000,
		ExpireIn:      30 * time.Minute,
	})
	require.NoError(t, err)
	require.Equal(t, "up_existing", result.Session.ID)
}

func TestRegistry_CreateUploadSession_FileTooLarge(t *testing.T) {
	reg := NewRegistry(nil, &stubStorage{})

	_, err := reg.CreateUploadSession(context.Background(), UploadSessionRequest{
		UserID:        1,
		Category:      CategoryDishImage,
		ContentType:   "image/jpeg",
		ContentLength: 20_000_000, // 20MB
		MaxFileBytes:  10_000_000, // 10MB 限制
	})
	require.ErrorIs(t, err, ErrFileTooLarge)
}

func TestRegistry_CreateUploadSession_InvalidContentType(t *testing.T) {
	reg := NewRegistry(nil, &stubStorage{})

	_, err := reg.CreateUploadSession(context.Background(), UploadSessionRequest{
		UserID:        1,
		Category:      CategoryDishImage,
		ContentType:   "application/pdf", // 不允许
		ContentLength: 500_000,
	})
	require.ErrorIs(t, err, ErrUnsupportedContentType)
}

// ==================== CompleteUpload ====================

func TestRegistry_CompleteUpload_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	storage := &stubStorage{
		statMeta: ObjectMetadata{ContentType: "image/jpeg", Size: 500_000},
	}
	reg := NewRegistry(store, storage)

	const uploadID = "up_abc"
	const userID int64 = 1
	objectKey := "dish/1/up_abc/photo.jpg"

	session := db.MediaUploadSession{
		ID:            uploadID,
		UserID:        userID,
		MediaCategory: string(CategoryDishImage),
		Visibility:    string(VisibilityPublic),
		ObjectKey:     objectKey,
		ContentType:   "image/jpeg",
		Status:        "pending",
		ExpireAt:      time.Now().Add(time.Hour),
	}
	store.EXPECT().
		GetUploadSession(gomock.Any(), uploadID).
		Times(1).
		Return(session, nil)

	createdAsset := db.MediaAsset{
		ID:            42,
		ObjectKey:     objectKey,
		Visibility:    "public",
		MediaCategory: string(CategoryDishImage),
	}
	store.EXPECT().
		CreateMediaAsset(gomock.Any(), gomock.Any()).
		Times(1).
		Return(createdAsset, nil)

	store.EXPECT().
		CompleteUploadSession(gomock.Any(), gomock.Any()).
		Times(1).
		Return(session, nil)

	result, err := reg.CompleteUpload(context.Background(), CompleteRequest{
		UploadID:  uploadID,
		ObjectKey: objectKey,
		UserID:    userID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(42), result.Asset.ID)
}

func TestRegistry_CompleteUpload_SessionNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reg := NewRegistry(store, &stubStorage{})

	store.EXPECT().
		GetUploadSession(gomock.Any(), "up_missing").
		Times(1).
		Return(db.MediaUploadSession{}, errors.New("not found"))

	_, err := reg.CompleteUpload(context.Background(), CompleteRequest{
		UploadID: "up_missing",
		UserID:   1,
	})
	require.ErrorIs(t, err, ErrUploadSessionNotFound)
}

func TestRegistry_CompleteUpload_WrongUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reg := NewRegistry(store, &stubStorage{})

	session := db.MediaUploadSession{
		ID:       "up_abc",
		UserID:   99, // 属于用户 99
		Status:   "pending",
		ExpireAt: time.Now().Add(time.Hour),
	}
	store.EXPECT().
		GetUploadSession(gomock.Any(), "up_abc").
		Times(1).
		Return(session, nil)

	_, err := reg.CompleteUpload(context.Background(), CompleteRequest{
		UploadID: "up_abc",
		UserID:   1, // 用户 1 试图完成
	})
	require.ErrorIs(t, err, ErrUnauthorized)
}

func TestRegistry_CompleteUpload_ObjectKeyMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reg := NewRegistry(store, &stubStorage{})

	session := db.MediaUploadSession{
		ID:        "up_abc",
		UserID:    1,
		ObjectKey: "dish/1/up_abc/photo.jpg",
		Status:    "pending",
		ExpireAt:  time.Now().Add(time.Hour),
	}
	store.EXPECT().
		GetUploadSession(gomock.Any(), "up_abc").
		Times(1).
		Return(session, nil)

	_, err := reg.CompleteUpload(context.Background(), CompleteRequest{
		UploadID:  "up_abc",
		ObjectKey: "dish/1/up_abc/malicious.jpg", // 不一致
		UserID:    1,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "object_key mismatch")
}

func TestRegistry_CompleteUpload_Expired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reg := NewRegistry(store, &stubStorage{})

	session := db.MediaUploadSession{
		ID:       "up_abc",
		UserID:   1,
		Status:   "pending",
		ExpireAt: time.Now().Add(-time.Minute), // 已过期
	}
	store.EXPECT().
		GetUploadSession(gomock.Any(), "up_abc").
		Times(1).
		Return(session, nil)

	_, err := reg.CompleteUpload(context.Background(), CompleteRequest{
		UploadID: "up_abc",
		UserID:   1,
	})
	require.ErrorIs(t, err, ErrUploadSessionExpired)
}

func TestRegistry_CompleteUpload_ObjectNotYetUploaded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	storage := &stubStorage{statErr: ErrObjectNotFound}
	reg := NewRegistry(store, storage)

	session := db.MediaUploadSession{
		ID:            "up_abc",
		UserID:        1,
		MediaCategory: string(CategoryDishImage),
		ObjectKey:     "dish/1/up_abc/photo.jpg",
		Status:        "pending",
		ExpireAt:      time.Now().Add(time.Hour),
	}
	store.EXPECT().
		GetUploadSession(gomock.Any(), "up_abc").
		Times(1).
		Return(session, nil)

	_, err := reg.CompleteUpload(context.Background(), CompleteRequest{
		UploadID: "up_abc",
		UserID:   1,
	})
	require.ErrorIs(t, err, ErrUploadNotConfirmed)
}

func TestRegistry_CompleteUpload_Idempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reg := NewRegistry(store, &stubStorage{})

	const assetID int64 = 77
	existingAsset := db.MediaAsset{ID: assetID, ObjectKey: "dish/1/up_abc/photo.jpg"}

	// 会话已 completed
	session := db.MediaUploadSession{
		ID:           "up_abc",
		UserID:       1,
		Status:       "completed",
		MediaAssetID: pgtype.Int8{Int64: assetID, Valid: true},
		ExpireAt:     time.Now().Add(time.Hour),
	}
	store.EXPECT().
		GetUploadSession(gomock.Any(), "up_abc").
		Times(1).
		Return(session, nil)

	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), assetID).
		Times(1).
		Return(existingAsset, nil)

	result, err := reg.CompleteUpload(context.Background(), CompleteRequest{
		UploadID: "up_abc",
		UserID:   1,
	})
	require.NoError(t, err)
	require.Equal(t, assetID, result.Asset.ID)
}

// ==================== GetAsset ====================

func TestRegistry_GetAsset_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reg := NewRegistry(store, &stubStorage{})

	asset := db.MediaAsset{ID: 1, ObjectKey: "dish/1/photo.jpg", UploadStatus: "confirmed"}
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), int64(1)).
		Times(1).
		Return(asset, nil)

	got, err := reg.GetAsset(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), got.ID)
}

func TestRegistry_GetAsset_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reg := NewRegistry(store, &stubStorage{})

	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), int64(99)).
		Times(1).
		Return(db.MediaAsset{}, errors.New("not found"))

	_, err := reg.GetAsset(context.Background(), 99)
	require.ErrorIs(t, err, ErrAssetNotFound)
}

func TestRegistry_GetAsset_Deleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reg := NewRegistry(store, &stubStorage{})

	asset := db.MediaAsset{ID: 5, UploadStatus: "deleted"}
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), int64(5)).
		Times(1).
		Return(asset, nil)

	_, err := reg.GetAsset(context.Background(), 5)
	require.ErrorIs(t, err, ErrAssetDeleted)
}

// ==================== SoftDelete ====================

func TestRegistry_SoftDelete_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reg := NewRegistry(store, &stubStorage{})

	const userID int64 = 1
	asset := db.MediaAsset{ID: 10, UploadedBy: userID}
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), int64(10)).
		Times(1).
		Return(asset, nil)

	store.EXPECT().
		SoftDeleteMediaAsset(gomock.Any(), int64(10)).
		Times(1).
		Return(asset, nil)

	err := reg.SoftDelete(context.Background(), 10, userID)
	require.NoError(t, err)
}

func TestRegistry_SoftDelete_WrongOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reg := NewRegistry(store, &stubStorage{})

	asset := db.MediaAsset{ID: 10, UploadedBy: 99}
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), int64(10)).
		Times(1).
		Return(asset, nil)

	err := reg.SoftDelete(context.Background(), 10, 1) // 用户 1 删除用户 99 的资产
	require.ErrorIs(t, err, ErrUnauthorized)
}

// ==================== CreatePrivateAccessURL ====================

func TestRegistry_CreatePrivateAccessURL_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	storage := &stubStorage{
		privateURL: "https://private-bucket.oss.example.com/path/file.jpg?signature=xxx",
	}
	reg := NewRegistry(store, storage)

	asset := db.MediaAsset{ID: 20, ObjectKey: "rider/idcard/front.jpg", Visibility: string(VisibilityPrivate), UploadStatus: "confirmed"}
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), int64(20)).
		Times(1).
		Return(asset, nil)

	url, err := reg.CreatePrivateAccessURL(context.Background(), 20, 5*time.Minute)
	require.NoError(t, err)
	require.Contains(t, url, "signature")
}

func TestRegistry_CreatePrivateAccessURL_PublicAsset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	reg := NewRegistry(store, &stubStorage{})

	asset := db.MediaAsset{ID: 21, Visibility: string(VisibilityPublic), UploadStatus: "confirmed"}
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), int64(21)).
		Times(1).
		Return(asset, nil)

	_, err := reg.CreatePrivateAccessURL(context.Background(), 21, 5*time.Minute)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not private")
}

func TestRegistry_ReadMediaAsset_LocalOrRemoteStorage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	storage := &stubStorage{readData: []byte("binary-image-data")}
	reg := NewRegistry(store, storage)

	asset := db.MediaAsset{ID: 30, ObjectKey: "merchant/license/test.jpg", Visibility: string(VisibilityPrivate), UploadStatus: "confirmed", MimeType: "image/jpeg"}
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), int64(30)).
		Times(1).
		Return(asset, nil)

	data, contentType, err := reg.ReadMediaAsset(context.Background(), 30)
	require.NoError(t, err)
	require.Equal(t, []byte("binary-image-data"), data)
	require.Equal(t, "image/jpeg", contentType)
}

func TestRegistry_ReadMediaAsset_TooLarge(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	storage := &stubStorage{readData: make([]byte, maxDownloadBytes+1)}
	reg := NewRegistry(store, storage)

	asset := db.MediaAsset{ID: 31, ObjectKey: "merchant/license/huge.jpg", Visibility: string(VisibilityPublic), UploadStatus: "confirmed", MimeType: "image/jpeg"}
	store.EXPECT().
		GetMediaAssetByID(gomock.Any(), int64(31)).
		Times(1).
		Return(asset, nil)

	_, _, err := reg.ReadMediaAsset(context.Background(), 31)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds max download size")
}
