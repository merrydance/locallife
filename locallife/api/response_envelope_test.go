package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponseEnvelopeMiddleware_SanitizesRawServiceUnavailableMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ResponseEnvelopeMiddleware())
	router.GET("/test", func(ctx *gin.Context) {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
	})

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/test", nil)
	require.NoError(t, err)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, "微信支付服务暂不可用，请稍后重试", resp.Message)
	require.Empty(t, resp.Data)
}

func TestResponseEnvelopeMiddleware_PreservesSafeServiceUnavailableMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ResponseEnvelopeMiddleware())
	router.GET("/test", func(ctx *gin.Context) {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("绑定码服务暂不可用")))
	})

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/test", nil)
	require.NoError(t, err)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, "绑定码服务暂不可用", resp.Message)
}
