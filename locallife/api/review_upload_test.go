package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	mockwechat "github.com/merrydance/locallife/wechat/mock"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestUploadReviewImageAPI_Gone verifies the old upload endpoint returns 410 Gone.
// The endpoint has been replaced by the media upload flow (POST /v1/media/upload-sessions).
func TestUploadReviewImageAPI_Gone(t *testing.T) {
	user, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wechatClient := mockwechat.NewMockWechatClient(ctrl)
	server := newTestServerWithWechat(t, store, wechatClient)

	request, err := http.NewRequest(http.MethodPost, "/v1/reviews/images/upload", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusGone, recorder.Code)
}
