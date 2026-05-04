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

	"github.com/jackc/pgx/v5/pgtype"
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

func TestUpdateMerchantOpenStatus_RequireBaofuAccountWhenOpen_MissingBinding(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          90,
		OwnerUserID: user.ID,
		RegionID:    1,
		Status:      "active",
		Name:        "商户C",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantProfile(gomock.Any(), merchant.ID).Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{
		MerchantID: merchant.ID,
		SubMchID:   "190000090",
		Status:     "active",
	}, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   merchant.ID,
	}).Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
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
	require.Contains(t, recorder.Body.String(), "商户结算账户未开通")
	require.NotContains(t, recorder.Body.String(), "contract")
	require.NotContains(t, recorder.Body.String(), "sharing")
}

func TestUpdateMerchantOpenStatus_RequireBaofuAccountWhenOpen_WechatChannelPending(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          91,
		OwnerUserID: user.ID,
		RegionID:    1,
		Status:      "active",
		Name:        "商户D",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantProfile(gomock.Any(), merchant.ID).Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{
		MerchantID: merchant.ID,
		SubMchID:   "190000091",
		Status:     "active",
	}, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   merchant.ID,
	}).Return(db.BaofuAccountBinding{
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      merchant.ID,
		AccountType:  db.BaofuAccountTypeBusiness,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "CM91", Valid: true},
		SharingMerID: pgtype.Text{String: "CM91", Valid: true},
	}, nil)
	store.EXPECT().GetBaofuMerchantReportByOwner(gomock.Any(), db.GetBaofuMerchantReportByOwnerParams{
		OwnerType:  db.BaofuAccountOwnerTypeMerchant,
		OwnerID:    merchant.ID,
		ReportType: db.BaofuMerchantReportTypeWechat,
	}).Return(db.BaofuMerchantReport{}, db.ErrRecordNotFound)
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
	require.Contains(t, recorder.Body.String(), "微信支付通道待开通")
	require.NotContains(t, recorder.Body.String(), "CM91")
}

func TestGetMerchantOpenStatus_IncludesBaofuSettlementReadiness(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          92,
		OwnerUserID: user.ID,
		RegionID:    1,
		Status:      "active",
		Name:        "商户E",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetMerchantIsOpen(gomock.Any(), merchant.ID).
		Return(db.GetMerchantIsOpenRow{ID: merchant.ID, IsOpen: false}, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   merchant.ID,
	}).Return(db.BaofuAccountBinding{
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      merchant.ID,
		AccountType:  db.BaofuAccountTypeBusiness,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "CM92", Valid: true},
		SharingMerID: pgtype.Text{String: "CM92", Valid: true},
	}, nil)
	store.EXPECT().GetBaofuMerchantReportByOwner(gomock.Any(), db.GetBaofuMerchantReportByOwnerParams{
		OwnerType:  db.BaofuAccountOwnerTypeMerchant,
		OwnerID:    merchant.ID,
		ReportType: db.BaofuMerchantReportTypeWechat,
	}).Return(db.BaofuMerchantReport{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchants/me/status", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp merchantStatusResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.False(t, resp.IsOpen)
	require.NotNil(t, resp.SettlementAccount)
	require.Equal(t, "wechat_channel_pending", resp.SettlementAccount.State)
	require.Equal(t, "微信支付通道待开通", resp.SettlementAccount.Label)
	require.False(t, resp.SettlementAccount.PaymentReady)
	require.NotContains(t, recorder.Body.String(), "CM92")
	require.NotContains(t, recorder.Body.String(), "contract")
	require.NotContains(t, recorder.Body.String(), "sharing")
}
