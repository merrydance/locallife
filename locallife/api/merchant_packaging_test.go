package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type merchantPackagingSettingsTestResponse struct {
	MerchantID           int64    `json:"merchant_id"`
	Enabled              bool     `json:"enabled"`
	Required             bool     `json:"required"`
	ApplicableOrderTypes []string `json:"applicable_order_types"`
	DefaultOptionID      *int64   `json:"default_option_id"`
}

type merchantPackagingOptionTestResponse struct {
	ID          int64  `json:"id"`
	MerchantID  int64  `json:"merchant_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int64  `json:"price"`
	IsEnabled   bool   `json:"is_enabled"`
	SortOrder   int16  `json:"sort_order"`
}

type merchantPackagingOptionListTestResponse struct {
	Options    []merchantPackagingOptionTestResponse `json:"options"`
	Total      int                                   `json:"total"`
	Page       int                                   `json:"page"`
	Limit      int                                   `json:"limit"`
	TotalPages int                                   `json:"total_pages"`
}

func TestMerchantPackagingOwnerCanUpsertSettings(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		UpsertMerchantPackagingSettingsTx(gomock.Any(), db.UpsertMerchantPackagingSettingsParams{
			MerchantID:           merchant.ID,
			Enabled:              true,
			Required:             true,
			ApplicableOrderTypes: []string{db.OrderTypeTakeout},
			DefaultOptionID:      pgtype.Int8{},
		}).
		Times(1).
		Return(db.MerchantPackagingSetting{
			ID:                   1,
			MerchantID:           merchant.ID,
			Enabled:              true,
			Required:             true,
			ApplicableOrderTypes: []string{db.OrderTypeTakeout},
		}, nil)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodPut, "/v1/merchant/packaging-settings", map[string]any{
		"enabled":                true,
		"required":               true,
		"applicable_order_types": []string{db.OrderTypeTakeout},
	}, owner.ID)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp merchantPackagingSettingsTestResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, merchant.ID, resp.MerchantID)
	require.True(t, resp.Enabled)
	require.True(t, resp.Required)
	require.Equal(t, []string{db.OrderTypeTakeout}, resp.ApplicableOrderTypes)
	require.Nil(t, resp.DefaultOptionID)
}

func TestMerchantPackagingManagerCanUpsertSettings(t *testing.T) {
	owner, _ := randomUser(t)
	manager, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleStaffMerchant(store, manager.ID, merchant)
	store.EXPECT().
		GetUserMerchantRole(gomock.Any(), db.GetUserMerchantRoleParams{
			MerchantID: merchant.ID,
			UserID:     manager.ID,
		}).
		Times(1).
		Return(db.MerchantStaffRoleManager, nil)
	store.EXPECT().
		UpsertMerchantPackagingSettingsTx(gomock.Any(), db.UpsertMerchantPackagingSettingsParams{
			MerchantID:           merchant.ID,
			Enabled:              false,
			Required:             false,
			ApplicableOrderTypes: []string{db.OrderTypeTakeout, db.OrderTypeTakeaway},
			DefaultOptionID:      pgtype.Int8{},
		}).
		Times(1).
		Return(db.MerchantPackagingSetting{
			ID:                   2,
			MerchantID:           merchant.ID,
			Enabled:              false,
			Required:             false,
			ApplicableOrderTypes: []string{db.OrderTypeTakeout, db.OrderTypeTakeaway},
		}, nil)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodPut, "/v1/merchant/packaging-settings", map[string]any{
		"enabled":  false,
		"required": false,
	}, manager.ID)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp merchantPackagingSettingsTestResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, merchant.ID, resp.MerchantID)
	require.False(t, resp.Enabled)
	require.False(t, resp.Required)
	require.Equal(t, []string{db.OrderTypeTakeout, db.OrderTypeTakeaway}, resp.ApplicableOrderTypes)
}

func TestMerchantPackagingGetSettingsReturnsDefaultWhenMissing(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetMerchantPackagingSettings(gomock.Any(), merchant.ID).
		Times(1).
		Return(db.MerchantPackagingSetting{}, db.ErrRecordNotFound)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodGet, "/v1/merchant/packaging-settings", nil, owner.ID)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp merchantPackagingSettingsTestResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, merchant.ID, resp.MerchantID)
	require.False(t, resp.Enabled)
	require.True(t, resp.Required)
	require.Equal(t, []string{db.OrderTypeTakeout, db.OrderTypeTakeaway}, resp.ApplicableOrderTypes)
	require.Nil(t, resp.DefaultOptionID)
}

func TestMerchantPackagingRejectsUnavailableDefaultOption(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	defaultOptionID := int64(2001)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		UpsertMerchantPackagingSettingsTx(gomock.Any(), db.UpsertMerchantPackagingSettingsParams{
			MerchantID:           merchant.ID,
			Enabled:              true,
			Required:             true,
			ApplicableOrderTypes: []string{db.OrderTypeTakeout, db.OrderTypeTakeaway},
			DefaultOptionID:      pgtype.Int8{Int64: defaultOptionID, Valid: true},
		}).
		Times(1).
		Return(db.MerchantPackagingSetting{}, db.ErrMerchantPackagingDefaultOptionUnavailable)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodPut, "/v1/merchant/packaging-settings", map[string]any{
		"enabled":           true,
		"required":          true,
		"default_option_id": defaultOptionID,
	}, owner.ID)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestMerchantPackagingNonMerchantDenied(t *testing.T) {
	user, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveNoAccessibleMerchants(store, user.ID)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodPut, "/v1/merchant/packaging-settings", map[string]any{
		"enabled":  true,
		"required": true,
	}, user.ID)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestMerchantPackagingCashierDenied(t *testing.T) {
	owner, _ := randomUser(t)
	cashier, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleStaffMerchant(store, cashier.ID, merchant)
	store.EXPECT().
		GetUserMerchantRole(gomock.Any(), db.GetUserMerchantRoleParams{
			MerchantID: merchant.ID,
			UserID:     cashier.ID,
		}).
		Times(1).
		Return(db.MerchantStaffRoleCashier, nil)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodPut, "/v1/merchant/packaging-settings", map[string]any{
		"enabled":  true,
		"required": true,
	}, cashier.ID)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestMerchantPackagingRejectsInvalidOrderType(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodPut, "/v1/merchant/packaging-settings", map[string]any{
		"enabled":                true,
		"required":               true,
		"applicable_order_types": []string{db.OrderTypeDineIn},
	}, owner.ID)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestMerchantPackagingListOptionsReturnsPaginationMetadata(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		ListMerchantPackagingOptions(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.MerchantPackagingOption{
			{
				ID:          1001,
				MerchantID:  merchant.ID,
				Name:        "普通餐盒",
				Description: pgtype.Text{String: "环保纸盒", Valid: true},
				Price:       100,
				IsEnabled:   true,
				SortOrder:   0,
			},
			{
				ID:         1002,
				MerchantID: merchant.ID,
				Name:       "保温袋",
				Price:      200,
				IsEnabled:  true,
				SortOrder:  1,
			},
		}, nil)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodGet, "/v1/merchant/packaging-options", nil, owner.ID)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp merchantPackagingOptionListTestResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Options, 2)
	require.Equal(t, 2, resp.Total)
	require.Equal(t, 1, resp.Page)
	require.Equal(t, 2, resp.Limit)
	require.Equal(t, 1, resp.TotalPages)
}

func TestMerchantPackagingRejectsInvalidOptionIDWithChineseError(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodDelete, "/v1/merchant/packaging-options/not-a-number", nil, owner.ID)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "包装方式ID无效")
}

func TestMerchantPackagingForeignOptionUpdateDenied(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		UpdateMerchantPackagingOptionTx(gomock.Any(), db.UpdateMerchantPackagingOptionParams{
			ID:          99,
			MerchantID:  merchant.ID,
			Name:        "普通餐盒",
			Description: pgtype.Text{String: "环保纸盒", Valid: true},
			Price:       100,
			IsEnabled:   true,
			SortOrder:   0,
		}).
		Times(1).
		Return(db.MerchantPackagingOption{}, db.ErrRecordNotFound)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodPut, "/v1/merchant/packaging-options/99", map[string]any{
		"name":        "普通餐盒",
		"description": "环保纸盒",
		"price":       100,
		"is_enabled":  true,
		"sort_order":  0,
	}, owner.ID)

	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestMerchantPackagingDisablingDefaultOptionClearsDefaultInTransaction(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	optionID := int64(1001)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		UpdateMerchantPackagingOptionTx(gomock.Any(), db.UpdateMerchantPackagingOptionParams{
			ID:          optionID,
			MerchantID:  merchant.ID,
			Name:        "普通餐盒",
			Description: pgtype.Text{},
			Price:       100,
			IsEnabled:   false,
			SortOrder:   0,
		}).
		Times(1).
		Return(db.MerchantPackagingOption{
			ID:         optionID,
			MerchantID: merchant.ID,
			Name:       "普通餐盒",
			Price:      100,
			IsEnabled:  false,
			SortOrder:  0,
		}, nil)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodPut, "/v1/merchant/packaging-options/1001", map[string]any{
		"name":       "普通餐盒",
		"price":      100,
		"is_enabled": false,
		"sort_order": 0,
	}, owner.ID)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestMerchantPackagingDuplicateActiveOptionNameRejected(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		CreateMerchantPackagingOption(gomock.Any(), db.CreateMerchantPackagingOptionParams{
			MerchantID:  merchant.ID,
			Name:        "普通餐盒",
			Description: pgtype.Text{},
			Price:       100,
			IsEnabled:   true,
			SortOrder:   0,
		}).
		Times(1).
		Return(db.MerchantPackagingOption{}, &pgconn.PgError{
			Code:           "23505",
			ConstraintName: "uq_merchant_packaging_options_name_active",
		})
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodPost, "/v1/merchant/packaging-options", map[string]any{
		"name":       "普通餐盒",
		"price":      100,
		"is_enabled": true,
		"sort_order": 0,
	}, owner.ID)

	require.Equal(t, http.StatusConflict, recorder.Code)
}

func TestMerchantPackagingDeletingDefaultOptionClearsDefaultInTransaction(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	optionID := int64(1001)
	deletedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		SoftDeleteMerchantPackagingOptionTx(gomock.Any(), db.SoftDeleteMerchantPackagingOptionParams{
			ID:         optionID,
			MerchantID: merchant.ID,
		}).
		Times(1).
		Return(db.MerchantPackagingOption{
			ID:          optionID,
			MerchantID:  merchant.ID,
			Name:        "普通餐盒",
			Price:       100,
			IsEnabled:   false,
			DeletedAt:   deletedAt,
			Description: pgtype.Text{},
		}, nil)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodDelete, "/v1/merchant/packaging-options/1001", nil, owner.ID)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestMerchantPackagingSoftDeleteIsIdempotentForOwnedOption(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	deletedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		SoftDeleteMerchantPackagingOptionTx(gomock.Any(), db.SoftDeleteMerchantPackagingOptionParams{
			ID:         1001,
			MerchantID: merchant.ID,
		}).
		Times(1).
		Return(db.MerchantPackagingOption{
			ID:          1001,
			MerchantID:  merchant.ID,
			Name:        "普通餐盒",
			Price:       100,
			IsEnabled:   false,
			DeletedAt:   deletedAt,
			Description: pgtype.Text{},
		}, nil)
	server := newTestServer(t, store)

	recorder := performMerchantPackagingRequest(t, server, http.MethodDelete, "/v1/merchant/packaging-options/1001", nil, owner.ID)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp merchantPackagingOptionTestResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(1001), resp.ID)
	require.Equal(t, merchant.ID, resp.MerchantID)
	require.False(t, resp.IsEnabled)
}

func performMerchantPackagingRequest(t *testing.T, server *Server, method, path string, body any, userID int64) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	}

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(method, path, reader)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, userID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	return recorder
}
