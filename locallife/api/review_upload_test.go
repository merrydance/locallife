package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	"github.com/merrydance/locallife/token"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/wechat"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUploadReviewImageAPI(t *testing.T) {
	user, _ := randomUser(t)
	uploadDir := filepath.Join("uploads", "reviews", fmt.Sprintf("%d", user.ID))

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(wechatClient *mockwechat.MockWechatClient)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(wechatClient *mockwechat.MockWechatClient) {
				wechatClient.EXPECT().
					ImgSecCheck(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response uploadImageResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.NotEmpty(t, response.ImageURL)
				require.Contains(t, response.ImageURL, "uploads")
				require.Contains(t, response.ImageURL, "reviews")
				require.Contains(t, response.ImageURL, fmt.Sprintf("/%d/", user.ID))

				// Best-effort cleanup (tests run from package dir)
				_ = os.Remove(strings.TrimPrefix(response.ImageURL, "/"))
				_ = os.RemoveAll(filepath.Join("uploads", "reviews", fmt.Sprintf("%d", user.ID)))
			},
		},
		{
			name: "RiskyContent",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(wechatClient *mockwechat.MockWechatClient) {
				wechatClient.EXPECT().
					ImgSecCheck(gomock.Any(), gomock.Any()).
					Times(1).
					Return(wechat.ErrRiskyContent)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "WechatError",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(wechatClient *mockwechat.MockWechatClient) {
				wechatClient.EXPECT().
					ImgSecCheck(gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("wechat unavailable"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadGateway, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(wechatClient *mockwechat.MockWechatClient) {
				wechatClient.EXPECT().
					ImgSecCheck(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			// Ensure a clean slate so we can assert "no file written" reliably.
			_ = os.RemoveAll(uploadDir)
			defer func() {
				_ = os.RemoveAll(uploadDir)
			}()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			wechatClient := mockwechat.NewMockWechatClient(ctrl)
			tc.buildStubs(wechatClient)

			server := newTestServerWithWechat(t, store, wechatClient)

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			part, err := writer.CreateFormFile("image", "test.jpg")
			require.NoError(t, err)
			_, err = part.Write([]byte("fake image data"))
			require.NoError(t, err)
			require.NoError(t, writer.Close())

			url := "/v1/reviews/images/upload"
			request, err := http.NewRequest(http.MethodPost, url, body)
			require.NoError(t, err)
			request.Header.Set("Content-Type", writer.FormDataContentType())

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)

			// For non-success cases, ensure nothing was persisted on disk.
			if tc.name != "OK" {
				entries, err := os.ReadDir(uploadDir)
				if err == nil {
					require.Len(t, entries, 0)
				} else {
					require.True(t, os.IsNotExist(err))
				}
			}

			tc.checkResponse(t, recorder)
		})
	}
}
