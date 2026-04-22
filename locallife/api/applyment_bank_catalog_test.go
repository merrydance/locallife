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

func TestListApplymentBanks_EcommerceClientUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/applyment/banks?account_type=ACCOUNT_TYPE_PRIVATE", nil)
	require.NoError(t, err)
	ctx.Request = request

	server.listApplymentBanks(ctx)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var response ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "wechat ecommerce client is not configured", response.Error)
}
