package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUpdateMerchantOpenStatus_RequireApplymentWhenOpen_NoPaymentConfig(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          88,
		OwnerUserID: user.ID,
		RegionID:    1,
		Status:      "active",
		Name:        "商户A",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantProfile(gomock.Any(), merchant.ID).Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{}, db.ErrRecordNotFound)
	store.EXPECT().UpdateMerchantIsOpen(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)

	body, err := json.Marshal(map[string]any{"is_open": true})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPatch, "/v1/merchants/me/status", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "普通服务商")
	require.Contains(t, recorder.Body.String(), "完成微信支付进件")
	require.Contains(t, recorder.Body.String(), "结算账户")
}

func TestUpdateMerchantOpenStatus_RequireApplymentWhenOpen_InactivePaymentConfig(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          89,
		OwnerUserID: user.ID,
		RegionID:    1,
		Status:      "active",
		Name:        "商户B",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantProfile(gomock.Any(), merchant.ID).Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{
		MerchantID: merchant.ID,
		SubMchID:   "",
		Status:     "pending",
	}, nil)
	store.EXPECT().UpdateMerchantIsOpen(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)

	body, err := json.Marshal(map[string]any{"is_open": true})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPatch, "/v1/merchants/me/status", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "普通服务商特约商户未激活")
	require.NotContains(t, recorder.Body.String(), "开户意愿授权")
	require.Contains(t, recorder.Body.String(), "微信支付进件")
	require.Contains(t, recorder.Body.String(), "结算账户")
}
