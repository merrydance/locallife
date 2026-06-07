package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateTagAPIStoresIcon(t *testing.T) {
	admin, _ := randomUser(t)
	icon := "🍜"
	tag := db.Tag{
		ID:        21,
		Name:      "面条",
		Type:      "merchant",
		SortOrder: 7,
		Status:    "active",
		Icon:      pgtype.Text{String: icon, Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		CreateTag(gomock.Any(), db.CreateTagParams{
			Name:      tag.Name,
			Type:      tag.Type,
			SortOrder: tag.SortOrder,
			Status:    tag.Status,
			Icon:      pgtype.Text{String: icon, Valid: true},
		}).
		Times(1).
		Return(tag, nil)

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{
		"name":       tag.Name,
		"type":       tag.Type,
		"sort_order": tag.SortOrder,
		"icon":       icon,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/tags", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	var response tagDetailResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, icon, response.Icon)
}

func TestCreateTagAPIRejectsBlankNameAfterTrim(t *testing.T) {
	admin, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().CreateTag(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{
		"name": "   ",
		"type": "merchant",
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/tags", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUpdateTagAPIUpdatesIcon(t *testing.T) {
	admin, _ := randomUser(t)
	icon := "🥘"
	tag := db.Tag{
		ID:        21,
		Name:      "砂锅",
		Type:      "merchant",
		SortOrder: 7,
		Status:    "active",
		Icon:      pgtype.Text{String: icon, Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().
		UpdateTag(gomock.Any(), db.UpdateTagParams{
			ID:   tag.ID,
			Name: pgtype.Text{String: tag.Name, Valid: true},
			Icon: pgtype.Text{String: icon, Valid: true},
		}).
		Times(1).
		Return(tag, nil)

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{
		"name": tag.Name,
		"icon": icon,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPatch, "/v1/tags/21", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response tagDetailResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, tag.Name, response.Name)
	require.Equal(t, icon, response.Icon)
}

func TestUpdateTagAPIRejectsBlankNameAfterTrim(t *testing.T) {
	admin, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectAdminRoleForPlatformEntity(t, store, admin)
	store.EXPECT().UpdateTag(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{
		"name": "   ",
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPatch, "/v1/tags/21", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestSetMerchantTagsAPIUsesTransactionalReplacement(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	firstTag := db.Tag{ID: 11, Name: "中餐", Type: "merchant", SortOrder: 1, Status: "active"}
	secondTag := db.Tag{ID: 12, Name: "快餐", Type: "merchant", SortOrder: 2, Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetTag(gomock.Any(), firstTag.ID).
		Times(1).
		Return(firstTag, nil)
	store.EXPECT().
		GetTag(gomock.Any(), secondTag.ID).
		Times(1).
		Return(secondTag, nil)
	store.EXPECT().
		SetMerchantTagsTx(gomock.Any(), db.SetMerchantTagsTxParams{
			MerchantID: merchant.ID,
			TagIDs:     []int64{firstTag.ID, secondTag.ID},
		}).
		Times(1).
		Return(db.SetMerchantTagsTxResult{Tags: []db.Tag{firstTag, secondTag}}, nil)
	store.EXPECT().ClearMerchantTags(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().AddMerchantTag(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{"tag_ids": []int64{firstTag.ID, secondTag.ID}})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPut, "/v1/merchants/me/tags", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response merchantTagsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Len(t, response.Tags, 2)
	require.Equal(t, firstTag.ID, response.Tags[0].ID)
	require.Equal(t, secondTag.ID, response.Tags[1].ID)
}

func TestSetMerchantTagsAPIRejectsDuplicateTagIDs(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	tag := db.Tag{ID: 11, Name: "中餐", Type: "merchant", SortOrder: 1, Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetTag(gomock.Any(), tag.ID).
		Times(1).
		Return(tag, nil)
	store.EXPECT().SetMerchantTagsTx(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().ClearMerchantTags(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().AddMerchantTag(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{"tag_ids": []int64{tag.ID, tag.ID}})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPut, "/v1/merchants/me/tags", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "duplicated")
}
