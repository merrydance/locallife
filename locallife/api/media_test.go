package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== helpers ====================

func randomUploadSession(userID int64, category, visibility, objectKey string, expired bool) db.MediaUploadSession {
	expireAt := time.Now().Add(30 * time.Minute)
	if expired {
		expireAt = time.Now().Add(-1 * time.Minute)
	}
	return db.MediaUploadSession{
		ID:             "up_test_" + fmt.Sprintf("%d", userID),
		UserID:         userID,
		BusinessType:   "merchant",
		MediaCategory:  category,
		Visibility:     visibility,
		ObjectKey:      objectKey,
		ChecksumSha256: "a" + fmt.Sprintf("%063d", 0), // 64-char hex
		ContentType:    "image/jpeg",
		ContentLength:  1024,
		Status:         "pending",
		ExpireAt:       expireAt,
	}
}

func randomMediaAsset(id, ownerID int64, visibility, objectKey string) db.MediaAsset {
	return db.MediaAsset{
		ID:               id,
		ObjectKey:        objectKey,
		Visibility:       visibility,
		MediaCategory:    "dish",
		MimeType:         "image/jpeg",
		FileSize:         1024,
		UploadStatus:     "confirmed",
		ModerationStatus: "approved",
		UploadedBy:       ownerID,
	}
}

func marshalBody(t *testing.T, body interface{}) *bytes.Buffer {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	return bytes.NewBuffer(data)
}

// writeLocalFile creates a file in tempDir at the given objectKey path.
func writeLocalFile(t *testing.T, tempDir, objectKey string) {
	t.Helper()
	localPath := filepath.Join(tempDir, filepath.FromSlash(objectKey))
	require.NoError(t, os.MkdirAll(filepath.Dir(localPath), 0750))
	require.NoError(t, os.WriteFile(localPath, []byte("fake image data"), 0600))
}

// ==================== POST /v1/media/upload-sessions ====================

func TestCreateMediaUploadSessionAPI(t *testing.T) {
	user, _ := randomUser(t)
	validReq := createUploadSessionRequest{
		BusinessType:   "merchant",
		MediaCategory:  "dish",
		ContentType:    "image/jpeg",
		ContentLength:  2048,
		ChecksumSha256: fmt.Sprintf("%064x", 1),
	}

	testCases := []struct {
		name          string
		body          interface{}
		setupAuth     func(t *testing.T, req *http.Request, server *Server)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "OK new session",
			body: validReq,
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPendingUploadSessionByIdempotencyKey(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MediaUploadSession{}, db.ErrRecordNotFound)
				store.EXPECT().
					CreateUploadSession(gomock.Any(), gomock.Any()).
					Times(1).
					Return(randomUploadSession(user.ID, "dish", "public", "merchant/dish/1/20260318/up_test.jpeg", false), nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, rec.Code)
				var resp uploadSessionResponse
				requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
				require.NotEmpty(t, resp.UploadID)
				require.NotEmpty(t, resp.UploadHost)
			},
		},
		{
			name: "OK idempotent existing session",
			body: validReq,
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				existing := randomUploadSession(user.ID, "dish", "public", "merchant/dish/1/20260318/up_existing.jpeg", false)
				store.EXPECT().
					GetPendingUploadSessionByIdempotencyKey(gomock.Any(), gomock.Any()).
					Times(1).
					Return(existing, nil)
				// No CreateUploadSession call — reuses existing
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, rec.Code)
			},
		},
		{
			name: "BadRequest missing field",
			body: map[string]interface{}{
				"business_type": "merchant",
				// missing media_category, content_type, content_length, checksum_sha256
			},
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			name: "BadRequest invalid media category",
			body: createUploadSessionRequest{
				BusinessType:   "merchant",
				MediaCategory:  "nonexistent_category",
				ContentType:    "image/jpeg",
				ContentLength:  1024,
				ChecksumSha256: fmt.Sprintf("%064x", 2),
			},
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			// Policy validation (Lookup/IsAllowedContentType) runs before DB call
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			name: "BadRequest unsupported content type",
			body: createUploadSessionRequest{
				BusinessType:   "merchant",
				MediaCategory:  "dish",
				ContentType:    "application/pdf",
				ContentLength:  1024,
				ChecksumSha256: fmt.Sprintf("%064x", 3),
			},
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			// IsAllowedContentType runs before DB call
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			name: "Unauthorized no token",
			body: validReq,
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				// no auth
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, rec.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server, _ := newTestServerForMedia(t, store)

			rec := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodPost, "/v1/media/upload-sessions", marshalBody(t, tc.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, req, server)

			server.router.ServeHTTP(rec, req)
			tc.checkResponse(t, rec)
		})
	}
}

// ==================== POST /v1/media/complete ====================

func TestCompleteMediaUploadAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	objectKey := "merchant/dish/1/20260318/up_complete.jpg"

	validReq := completeUploadRequest{
		UploadID:  "up_test_complete",
		ObjectKey: objectKey,
	}

	testCases := []struct {
		name          string
		body          interface{}
		setupAuth     func(t *testing.T, req *http.Request, server *Server)
		buildStubs    func(store *mockdb.MockStore, tempDir string)
		checkResponse func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "OK new public asset",
			body: validReq,
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, tempDir string) {
				session := randomUploadSession(user.ID, "dish", "public", objectKey, false)
				session.ID = validReq.UploadID

				writeLocalFile(t, tempDir, objectKey)

				store.EXPECT().GetUploadSession(gomock.Any(), validReq.UploadID).Times(1).Return(session, nil)
				// CreateMediaAsset succeeds directly → GetMediaAssetByObjectKey NOT called
				store.EXPECT().CreateMediaAsset(gomock.Any(), gomock.Any()).Times(1).Return(randomMediaAsset(10, user.ID, "public", objectKey), nil)
				store.EXPECT().CompleteUploadSession(gomock.Any(), gomock.Any()).Times(1).Return(session, nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, rec.Code)
				var resp completeUploadResponse
				requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
				require.EqualValues(t, 10, resp.MediaID)
				require.NotEmpty(t, resp.Variants["thumb"])
				require.NotEmpty(t, resp.Variants["card"])
			},
		},
		{
			name: "OK idempotent (session already completed)",
			body: validReq,
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, tempDir string) {
				session := randomUploadSession(user.ID, "dish", "public", objectKey, false)
				session.ID = validReq.UploadID
				session.Status = "completed" // triggers idempotency path in Registry
				session.MediaAssetID = pgtype.Int8{Int64: 10, Valid: true}

				store.EXPECT().GetUploadSession(gomock.Any(), validReq.UploadID).Times(1).Return(session, nil)
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(10)).Times(1).Return(randomMediaAsset(10, user.ID, "public", objectKey), nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, rec.Code)
				var resp completeUploadResponse
				requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
				require.EqualValues(t, 10, resp.MediaID)
			},
		},
		{
			name: "NotFound session not found",
			body: validReq,
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, tempDir string) {
				store.EXPECT().GetUploadSession(gomock.Any(), validReq.UploadID).Times(1).Return(db.MediaUploadSession{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, rec.Code)
			},
		},
		{
			name: "Gone session expired",
			body: validReq,
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, tempDir string) {
				session := randomUploadSession(user.ID, "dish", "public", objectKey, true /* expired */)
				session.ID = validReq.UploadID
				store.EXPECT().GetUploadSession(gomock.Any(), validReq.UploadID).Times(1).Return(session, nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusGone, rec.Code)
			},
		},
		{
			name: "Forbidden wrong user",
			body: validReq,
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, tempDir string) {
				session := randomUploadSession(user.ID, "dish", "public", objectKey, false)
				session.ID = validReq.UploadID
				store.EXPECT().GetUploadSession(gomock.Any(), validReq.UploadID).Times(1).Return(session, nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, rec.Code)
			},
		},
		{
			name: "UnprocessableEntity file not uploaded",
			body: validReq,
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, tempDir string) {
				session := randomUploadSession(user.ID, "dish", "public", objectKey, false)
				session.ID = validReq.UploadID
				store.EXPECT().GetUploadSession(gomock.Any(), validReq.UploadID).Times(1).Return(session, nil)
				// No file created in tempDir → StatObject returns ErrObjectNotFound
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
			},
		},
		{
			name:       "Unauthorized no token",
			body:       validReq,
			setupAuth:  func(t *testing.T, req *http.Request, server *Server) {},
			buildStubs: func(store *mockdb.MockStore, tempDir string) {},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, rec.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)

			server, tempDir := newTestServerForMedia(t, store)
			tc.buildStubs(store, tempDir)

			rec := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodPost, "/v1/media/complete", marshalBody(t, tc.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, req, server)

			server.router.ServeHTTP(rec, req)
			tc.checkResponse(t, rec)
		})
	}
}

// ==================== POST /v1/media/private-access ====================

func TestGetMediaPrivateAccessAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	objectKey := "id_card/front/1/20260318/up_priv.jpg"

	testCases := []struct {
		name          string
		body          interface{}
		setupAuth     func(t *testing.T, req *http.Request, server *Server)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "OK owner access",
			body: privateAccessRequest{MediaID: 5},
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomMediaAsset(5, user.ID, "private", objectKey)
				// GetAsset called twice: once in handler, once in CreatePrivateAccessURL
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(5)).Times(2).Return(asset, nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, rec.Code)
				var resp privateAccessResponse
				requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
				require.NotEmpty(t, resp.DownloadURL)
				require.True(t, resp.ExpireAt.After(time.Now()))
			},
		},
		{
			name: "Forbidden other user non-admin",
			body: privateAccessRequest{MediaID: 5},
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomMediaAsset(5, user.ID, "private", objectKey)
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(5)).Times(1).Return(asset, nil)
				// hasActiveRole → ListUserRoles returns no admin role
				store.EXPECT().ListUserRoles(gomock.Any(), otherUser.ID).Times(1).Return([]db.UserRole{}, nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, rec.Code)
			},
		},
		{
			name: "Forbidden admin access to id card asset",
			body: privateAccessRequest{MediaID: 6},
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomMediaAsset(6, user.ID, "private", objectKey)
				asset.MediaCategory = string(media.CategoryIDCardFront)
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(6)).Times(1).Return(asset, nil)
				store.EXPECT().ListUserRoles(gomock.Any(), otherUser.ID).Times(0)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, rec.Code)
			},
		},
		{
			name: "Admin access allowed for non-id private asset",
			body: privateAccessRequest{MediaID: 7},
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomMediaAsset(7, user.ID, "private", "merchant/license/business/1/20260318/license.jpg")
				asset.MediaCategory = string(media.CategoryGroupLicense)
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(7)).Times(2).Return(asset, nil)
				store.EXPECT().ListUserRoles(gomock.Any(), otherUser.ID).Times(1).Return([]db.UserRole{{Role: RoleAdmin, Status: "active"}}, nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, rec.Code)
				var resp privateAccessResponse
				requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
				require.NotEmpty(t, resp.DownloadURL)
			},
		},
		{
			name: "NotFound asset not found",
			body: privateAccessRequest{MediaID: 9999},
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(9999)).Times(1).Return(db.MediaAsset{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, rec.Code)
			},
		},
		{
			name:       "Unauthorized no token",
			body:       privateAccessRequest{MediaID: 5},
			setupAuth:  func(t *testing.T, req *http.Request, server *Server) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, rec.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server, _ := newTestServerForMedia(t, store)

			rec := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodPost, "/v1/media/private-access", marshalBody(t, tc.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, req, server)

			server.router.ServeHTTP(rec, req)
			tc.checkResponse(t, rec)
		})
	}
}

// ==================== DELETE /v1/media/{id} ====================

func TestDeleteMediaAssetAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	objectKey := "merchant/logo/1/20260318/up_del.jpg"

	testCases := []struct {
		name          string
		mediaID       string
		setupAuth     func(t *testing.T, req *http.Request, server *Server)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:    "OK deleted",
			mediaID: "7",
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomMediaAsset(7, user.ID, "public", objectKey)
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(7)).Times(1).Return(asset, nil)
				store.EXPECT().SoftDeleteMediaAsset(gomock.Any(), int64(7)).Times(1).Return(asset, nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNoContent, rec.Code)
			},
		},
		{
			name:    "Forbidden other user",
			mediaID: "7",
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomMediaAsset(7, user.ID, "public", objectKey)
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(7)).Times(1).Return(asset, nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, rec.Code)
			},
		},
		{
			name:    "NotFound",
			mediaID: "9999",
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(9999)).Times(1).Return(db.MediaAsset{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, rec.Code)
			},
		},
		{
			name:    "BadRequest invalid id",
			mediaID: "abc",
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
		{
			name:       "Unauthorized",
			mediaID:    "7",
			setupAuth:  func(t *testing.T, req *http.Request, server *Server) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, rec.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server, _ := newTestServerForMedia(t, store)

			rec := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodDelete, "/v1/media/"+tc.mediaID, nil)
			require.NoError(t, err)
			tc.setupAuth(t, req, server)

			server.router.ServeHTTP(rec, req)
			tc.checkResponse(t, rec)
		})
	}
}

// ==================== GET /v1/media/{id} ====================

func TestGetMediaAssetAPI(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)

	testCases := []struct {
		name          string
		mediaID       string
		setupAuth     func(t *testing.T, req *http.Request, server *Server)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:    "OK public asset — CDN URLs returned",
			mediaID: "1",
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomMediaAsset(1, user.ID, "public", "merchant/dish/1/20260318/file.jpg")
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(1)).Times(1).Return(asset, nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, rec.Code)
				var resp mediaAssetResponse
				requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
				require.Equal(t, int64(1), resp.ID)
				require.NotEmpty(t, resp.Variants["thumb"])
				require.NotEmpty(t, resp.Variants["original"])
			},
		},
		{
			name:    "OK private asset (own) — no URLs",
			mediaID: "2",
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomMediaAsset(2, user.ID, "private", "id_card/front/1/file.jpg")
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(2)).Times(1).Return(asset, nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, rec.Code)
				var resp mediaAssetResponse
				requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
				require.Empty(t, resp.Variants)
			},
		},
		{
			name:    "Forbidden — private asset other user",
			mediaID: "3",
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomMediaAsset(3, user.ID, "private", "id_card/front/1/file.jpg")
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(3)).Times(1).Return(asset, nil)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, rec.Code)
			},
		},
		{
			name:    "NotFound",
			mediaID: "9999",
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetMediaAssetByID(gomock.Any(), int64(9999)).Times(1).Return(db.MediaAsset{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, rec.Code)
			},
		},
		{
			name:    "BadRequest invalid id",
			mediaID: "bad",
			setupAuth: func(t *testing.T, req *http.Request, server *Server) {
				addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, rec.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server, _ := newTestServerForMedia(t, store)

			rec := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/v1/media/"+tc.mediaID, nil)
			require.NoError(t, err)
			tc.setupAuth(t, req, server)

			server.router.ServeHTTP(rec, req)
			tc.checkResponse(t, rec)
		})
	}
}
