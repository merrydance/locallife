package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	mockdb "github.com/merrydance/locallife/db/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGenerateAppBindCodeRedisUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/app-bind/code", nil)
	require.NoError(t, err)
	ctx.Request = req

	server.generateAppBindCode(ctx)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "绑定码服务暂不可用", resp.Error)
}

func TestVerifyAppBindCodeRedisUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/app-bind/verify", nil)
	require.NoError(t, err)
	ctx.Request = req

	server.verifyAppBindCode(ctx)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "绑定码服务暂不可用", resp.Error)
}
