package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 测试数据生成 ====================

func randomTable(merchantID int64) db.Table {
	return db.Table{
		ID:           util.RandomInt(1, 1000),
		MerchantID:   merchantID,
		TableNo:      fmt.Sprintf("T%d", util.RandomInt(1, 100)),
		TableType:    "table",
		Capacity:     int16(util.RandomInt(2, 10)),
		Description:  pgtype.Text{String: util.RandomString(20), Valid: true},
		MinimumSpend: pgtype.Int8{Int64: util.RandomInt(0, 50000), Valid: true},
		QrCodeUrl:    pgtype.Text{String: "https://example.com/qr.png", Valid: true},
		Status:       "available",
		CreatedAt:    time.Now(),
	}
}

func randomRoom(merchantID int64) db.Table {
	return db.Table{
		ID:           util.RandomInt(1, 1000),
		MerchantID:   merchantID,
		TableNo:      fmt.Sprintf("R%d", util.RandomInt(1, 20)),
		TableType:    "room",
		Capacity:     int16(util.RandomInt(4, 20)),
		Description:  pgtype.Text{String: "豪华包间", Valid: true},
		MinimumSpend: pgtype.Int8{Int64: util.RandomInt(30000, 100000), Valid: true},
		QrCodeUrl:    pgtype.Text{String: "https://example.com/room-qr.png", Valid: true},
		Status:       "available",
		CreatedAt:    time.Now(),
	}
}

func randomTableImageMediaAsset(id, ownerID int64, objectKey string) db.MediaAsset {
	asset := randomMediaAsset(id, ownerID, string(media.VisibilityPublic), objectKey)
	asset.MediaCategory = string(media.CategoryTableImage)
	asset.UploadStatus = "confirmed"
	asset.ModerationStatus = "approved"
	return asset
}

func tableToListTablesByMerchantRow(table db.Table, primaryImageAssetID int64) db.ListTablesByMerchantRow {
	return db.ListTablesByMerchantRow{
		ID:                   table.ID,
		MerchantID:           table.MerchantID,
		TableNo:              table.TableNo,
		TableType:            table.TableType,
		Capacity:             table.Capacity,
		Description:          table.Description,
		MinimumSpend:         table.MinimumSpend,
		QrCodeUrl:            table.QrCodeUrl,
		Status:               table.Status,
		CurrentReservationID: table.CurrentReservationID,
		CreatedAt:            table.CreatedAt,
		UpdatedAt:            table.UpdatedAt,
		AccessCodeHash:       table.AccessCodeHash,
		PrimaryImageAssetID:  primaryImageAssetID,
	}
}

func tableToListTablesByMerchantAndTypeRow(table db.Table, primaryImageAssetID int64) db.ListTablesByMerchantAndTypeRow {
	return db.ListTablesByMerchantAndTypeRow{
		ID:                   table.ID,
		MerchantID:           table.MerchantID,
		TableNo:              table.TableNo,
		TableType:            table.TableType,
		Capacity:             table.Capacity,
		Description:          table.Description,
		MinimumSpend:         table.MinimumSpend,
		QrCodeUrl:            table.QrCodeUrl,
		Status:               table.Status,
		CurrentReservationID: table.CurrentReservationID,
		CreatedAt:            table.CreatedAt,
		UpdatedAt:            table.UpdatedAt,
		AccessCodeHash:       table.AccessCodeHash,
		PrimaryImageAssetID:  primaryImageAssetID,
	}
}

func TestNewTableResponse_RewritesLegacyQRCodeURLInOSSMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server, _ := newTestServerForMedia(t, store)
	server.config.FileStorageProvider = "oss"

	table := randomTable(util.RandomInt(1, 1000))
	table.QrCodeUrl = pgtype.Text{String: "uploads/public/merchants/12/qrcodes/qrcode_m12_t3_labeled.png", Valid: true}

	resp := server.newTableResponse(table)
	require.NotNil(t, resp.QrCodeUrl)
	require.Contains(t, *resp.QrCodeUrl, "cdn.test.example.com/uploads/public/merchants/12/qrcodes/qrcode_m12_t3_labeled.png")
}

// ==================== 创建桌台测试 ====================

func TestCreateTableAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)
	tagID := util.RandomInt(1000, 2000)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"table_no":      table.TableNo,
				"table_type":    table.TableType,
				"capacity":      table.Capacity,
				"description":   table.Description.String,
				"minimum_spend": table.MinimumSpend.Int64,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)

				store.EXPECT().
					CreateTable(gomock.Any(), gomock.Any()).
					Times(1).
					Return(table, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				requireBodyMatchTable(t, recorder.Body, table)
			},
		},
		{
			name: "OKWithTags",
			body: gin.H{
				"table_no":   table.TableNo,
				"table_type": table.TableType,
				"capacity":   table.Capacity,
				"tag_ids":    []int64{tagID},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetTag(gomock.Any(), gomock.Eq(tagID)).
					Times(1).
					Return(db.Tag{ID: tagID, Type: "table"}, nil)

				store.EXPECT().
					CreateTableTx(gomock.Any(), gomock.AssignableToTypeOf(db.CreateTableTxParams{})).
					DoAndReturn(func(_ context.Context, arg db.CreateTableTxParams) (db.CreateTableTxResult, error) {
						require.Equal(t, table.TableNo, arg.Table.TableNo)
						require.Equal(t, table.TableType, arg.Table.TableType)
						require.Equal(t, table.Capacity, arg.Table.Capacity)
						require.Equal(t, []int64{tagID}, arg.TagIDs)
						return db.CreateTableTxResult{
							Table: table,
							Tags:  []db.TableTag{{TableID: table.ID, TagID: tagID}},
						}, nil
					}).
					Times(1)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				requireBodyMatchTable(t, recorder.Body, table)
			},
		},
		{
			name: "NoAuthorization",
			body: gin.H{
				"table_no":   table.TableNo,
				"table_type": table.TableType,
				"capacity":   table.Capacity,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No authorization
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "MerchantNotFound",
			body: gin.H{
				"table_no":   table.TableNo,
				"table_type": table.TableType,
				"capacity":   table.Capacity,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "CashierForbidden",
			body: gin.H{
				"table_no":   table.TableNo,
				"table_type": table.TableType,
				"capacity":   table.Capacity,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				staffMerchant := merchant
				staffMerchant.OwnerUserID = user.ID + 100

				expectResolveSingleStaffMerchant(store, user.ID, staffMerchant)

				store.EXPECT().
					GetUserMerchantRole(gomock.Any(), db.GetUserMerchantRoleParams{
						MerchantID: merchant.ID,
						UserID:     user.ID,
					}).
					Times(1).
					Return("cashier", nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "DuplicateTableNo",
			body: gin.H{
				"table_no":   table.TableNo,
				"table_type": table.TableType,
				"capacity":   table.Capacity,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTableByMerchantAndNo(gomock.Any(), gomock.Any()).
					Times(1).
					Return(table, nil) // 已存在
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "InvalidTableType",
			body: gin.H{
				"table_no":   table.TableNo,
				"table_type": "invalid_type",
				"capacity":   table.Capacity,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidCapacity",
			body: gin.H{
				"table_no":   table.TableNo,
				"table_type": table.TableType,
				"capacity":   0, // Invalid
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/tables"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestCreateTableAPIRejectsInvalidTagIDsBeforeCreate(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)
	missingTagID := util.RandomInt(1000, 2000)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetTableByMerchantAndNo(gomock.Any(), db.GetTableByMerchantAndNoParams{
			MerchantID: merchant.ID,
			TableNo:    table.TableNo,
		}).
		Times(1).
		Return(db.Table{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetTag(gomock.Any(), gomock.Eq(missingTagID)).
		Times(1).
		Return(db.Tag{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body := gin.H{
		"table_no":   table.TableNo,
		"table_type": table.TableType,
		"capacity":   table.Capacity,
		"tag_ids":    []int64{missingTagID},
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/tables", bytes.NewReader(data))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestCreateTableAPIRejectsDuplicateTagIDsBeforeCreate(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)
	tagID := util.RandomInt(1000, 2000)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetTableByMerchantAndNo(gomock.Any(), db.GetTableByMerchantAndNoParams{
			MerchantID: merchant.ID,
			TableNo:    table.TableNo,
		}).
		Times(1).
		Return(db.Table{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetTag(gomock.Any(), gomock.Eq(tagID)).
		Times(1).
		Return(db.Tag{ID: tagID, Type: "table"}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body := gin.H{
		"table_no":   table.TableNo,
		"table_type": table.TableType,
		"capacity":   table.Capacity,
		"tag_ids":    []int64{tagID, tagID},
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/tables", bytes.NewReader(data))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

// ==================== 获取桌台详情测试 ====================

func TestGetTableAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)

	testCases := []struct {
		name          string
		tableID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					ListTableTags(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return([]db.ListTableTagsRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchTable(t, recorder.Body, table)
			},
		},
		{
			name:    "NotFound",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "InvalidID",
			tableID: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/tables/%d", tc.tableID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 列出桌台测试 ====================

func TestListTablesAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	n := 5
	tables := make([]db.Table, n)
	tableRows := make([]db.ListTablesByMerchantRow, n)
	tableTypeRows := make([]db.ListTablesByMerchantAndTypeRow, n)
	for i := 0; i < n; i++ {
		tables[i] = randomTable(merchant.ID)
		tableRows[i] = tableToListTablesByMerchantRow(tables[i], 0)
		tableTypeRows[i] = tableToListTablesByMerchantAndTypeRow(tables[i], 0)
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListTablesByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(tableRows, nil)

				store.EXPECT().
					ListTableTags(gomock.Any(), gomock.Any()).
					Times(len(tableRows)).
					Return([]db.ListTableTagsRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "WithTableType",
			query: "table_type=room",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					ListTablesByMerchantAndType(gomock.Any(), gomock.Any()).
					Times(1).
					Return(tableTypeRows, nil)

				store.EXPECT().
					ListTableTags(gomock.Any(), gomock.Any()).
					Times(len(tableTypeRows)).
					Return([]db.ListTableTagsRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "NotMerchant",
			query: "",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/tables?" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListTablesAPI_IncludesImageURL(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)
	assetID := int64(42)
	rows := []db.ListTablesByMerchantRow{tableToListTablesByMerchantRow(table, assetID)}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		ListTablesByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
		Times(1).
		Return(rows, nil)
	store.EXPECT().
		ListMediaAssetsByIDs(gomock.Any(), gomock.Eq([]int64{assetID})).
		Times(1).
		Return([]db.ListMediaAssetsByIDsRow{approvedAssetRow(assetID, "merchant/table/cover.jpg")}, nil)
	store.EXPECT().
		ListTableTags(gomock.Any(), gomock.Eq(table.ID)).
		Times(1).
		Return([]db.ListTableTagsRow{}, nil)

	server, _ := newTestServerForMedia(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/tables", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code    int                `json:"code"`
		Message string             `json:"message"`
		Data    listTablesResponse `json:"data"`
	}
	err = json.Unmarshal(recorder.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Data.Tables, 1)
	require.NotEmpty(t, resp.Data.Tables[0].ImageURL)
	require.Contains(t, resp.Data.Tables[0].ImageURL, "cdn.test.example.com")
	require.Contains(t, resp.Data.Tables[0].ImageURL, "merchant/table/cover.jpg")
}

// ==================== 更新桌台测试 ====================

func TestUpdateTableAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)

	newTableNo := "T99"
	newCapacity := int16(8)
	tagID := util.RandomInt(1000, 2000)

	testCases := []struct {
		name          string
		tableID       int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			body: gin.H{
				"table_no": newTableNo,
				"capacity": newCapacity,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				updatedTable := table
				updatedTable.TableNo = newTableNo
				updatedTable.Capacity = newCapacity
				updatedTable.QrCodeUrl = pgtype.Text{}

				store.EXPECT().
					UpdateTableTx(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateTableTxParams{})).
					DoAndReturn(func(_ context.Context, arg db.UpdateTableTxParams) (db.UpdateTableTxResult, error) {
						require.True(t, arg.RequireNoFutureReservations)
						require.Nil(t, arg.TagIDs)
						require.True(t, arg.Table.TableNo.Valid)
						require.Equal(t, newTableNo, arg.Table.TableNo.String)
						require.False(t, arg.Table.QrCodeUrl.Valid)
						return db.UpdateTableTxResult{Table: updatedTable}, nil
					}).
					Times(1)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OKWithTags",
			tableID: table.ID,
			body: gin.H{
				"capacity": newCapacity,
				"tag_ids":  []int64{tagID},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetTag(gomock.Any(), gomock.Eq(tagID)).
					Times(1).
					Return(db.Tag{ID: tagID, Type: "table"}, nil)

				updatedTable := table
				updatedTable.Capacity = newCapacity

				store.EXPECT().
					UpdateTableTx(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateTableTxParams{})).
					DoAndReturn(func(_ context.Context, arg db.UpdateTableTxParams) (db.UpdateTableTxResult, error) {
						require.True(t, arg.RequireNoFutureReservations)
						require.Equal(t, table.ID, arg.Table.ID)
						require.True(t, arg.Table.Capacity.Valid)
						require.Equal(t, newCapacity, arg.Table.Capacity.Int16)
						require.NotNil(t, arg.TagIDs)
						require.Equal(t, []int64{tagID}, *arg.TagIDs)
						return db.UpdateTableTxResult{
							Table: updatedTable,
							Tags:  []db.TableTag{{TableID: table.ID, TagID: tagID}},
						}, nil
					}).
					Times(1)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OKClearsTags",
			tableID: table.ID,
			body: gin.H{
				"tag_ids": []int64{},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					UpdateTableTx(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateTableTxParams{})).
					DoAndReturn(func(_ context.Context, arg db.UpdateTableTxParams) (db.UpdateTableTxResult, error) {
						require.False(t, arg.RequireNoFutureReservations)
						require.Equal(t, table.ID, arg.Table.ID)
						require.NotNil(t, arg.TagIDs)
						require.Empty(t, *arg.TagIDs)
						return db.UpdateTableTxResult{
							Table: table,
							Tags:  []db.TableTag{},
						}, nil
					}).
					Times(1)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "NoopFulfillmentFieldStillUsesTransaction",
			tableID: table.ID,
			body: gin.H{
				"table_no":    table.TableNo,
				"description": "updated description only",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				updatedTable := table
				updatedTable.Description = pgtype.Text{String: "updated description only", Valid: true}

				store.EXPECT().
					UpdateTableTx(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateTableTxParams{})).
					DoAndReturn(func(_ context.Context, arg db.UpdateTableTxParams) (db.UpdateTableTxResult, error) {
						require.True(t, arg.RequireNoFutureReservations)
						require.Nil(t, arg.TagIDs)
						require.True(t, arg.Table.TableNo.Valid)
						require.Equal(t, table.TableNo, arg.Table.TableNo.String)
						require.True(t, arg.Table.Description.Valid)
						require.Equal(t, "updated description only", arg.Table.Description.String)
						require.False(t, arg.Table.QrCodeUrl.Valid)
						return db.UpdateTableTxResult{Table: updatedTable}, nil
					}).
					Times(1)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "KeepsQRCodeWhenTableNoUnchanged",
			tableID: table.ID,
			body: gin.H{
				"capacity": newCapacity,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				updatedTable := table
				updatedTable.Capacity = newCapacity

				store.EXPECT().
					UpdateTableTx(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateTableTxParams{})).
					DoAndReturn(func(_ context.Context, arg db.UpdateTableTxParams) (db.UpdateTableTxResult, error) {
						require.True(t, arg.RequireNoFutureReservations)
						require.Nil(t, arg.TagIDs)
						require.False(t, arg.Table.QrCodeUrl.Valid)
						return db.UpdateTableTxResult{Table: updatedTable}, nil
					}).
					Times(1)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "TableNotFound",
			tableID: table.ID,
			body: gin.H{
				"table_no": newTableNo,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "ForbiddenNotOwner",
			tableID: table.ID,
			body: gin.H{
				"table_no": newTableNo,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				// 桌台属于另一个商户
				otherTable := table
				otherTable.MerchantID = merchant.ID + 1

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(otherTable, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/tables/%d", tc.tableID)
			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateTableAPIRejectsNonTableTagBeforeMutating(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)
	nonTableTagID := util.RandomInt(1000, 2000)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetTable(gomock.Any(), gomock.Eq(table.ID)).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		GetTag(gomock.Any(), gomock.Eq(nonTableTagID)).
		Times(1).
		Return(db.Tag{ID: nonTableTagID, Type: "dish"}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body := gin.H{
		"capacity": table.Capacity + 1,
		"tag_ids":  []int64{nonTableTagID},
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/tables/%d", table.ID)
	request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUpdateTableAPIRejectsInvalidTagIDBeforeMutating(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetTable(gomock.Any(), gomock.Eq(table.ID)).
		Times(1).
		Return(table, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body := gin.H{
		"capacity": table.Capacity + 1,
		"tag_ids":  []int64{0},
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/tables/%d", table.ID)
	request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUpdateTableAPIRejectsFulfillmentChangeWithFutureReservations(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetTable(gomock.Any(), gomock.Eq(table.ID)).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		UpdateTableTx(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateTableTxParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateTableTxParams) (db.UpdateTableTxResult, error) {
			require.True(t, arg.RequireNoFutureReservations)
			require.True(t, arg.Table.Capacity.Valid)
			return db.UpdateTableTxResult{}, db.ErrTableHasFutureReservations
		}).
		Times(1)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body := gin.H{
		"capacity": table.Capacity - 1,
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/tables/%d", table.ID)
	request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusConflict, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "cannot update table fulfillment fields with future reservations", resp.Message)
}

func TestUpdateTableAPIRejectsDeletedTableAfterPrecheck(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetTable(gomock.Any(), gomock.Eq(table.ID)).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		UpdateTableTx(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateTableTxParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateTableTxParams) (db.UpdateTableTxResult, error) {
			require.True(t, arg.RequireNoFutureReservations)
			require.True(t, arg.Table.Capacity.Valid)
			return db.UpdateTableTxResult{}, fmt.Errorf("lock table: %w", db.ErrRecordNotFound)
		}).
		Times(1)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body := gin.H{
		"capacity": table.Capacity - 1,
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/tables/%d", table.ID)
	request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestUpdateTableAPIDescriptionUpdateRejectsDeletedTableAfterPrecheck(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetTable(gomock.Any(), gomock.Eq(table.ID)).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		UpdateTable(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateTableParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateTableParams) (db.Table, error) {
			require.Equal(t, table.ID, arg.ID)
			require.True(t, arg.Description.Valid)
			return db.Table{}, fmt.Errorf("update table: %w", db.ErrRecordNotFound)
		}).
		Times(1)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body := gin.H{
		"description": "updated description",
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/tables/%d", table.ID)
	request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

// ==================== 删除桌台测试 ====================

func TestDeleteTableAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)

	testCases := []struct {
		name          string
		tableID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					DeleteTableTx(gomock.Any(), gomock.Eq(db.DeleteTableParams{
						TableID: table.ID,
					})).
					Times(1).
					Return(db.DeleteTableResult{TableID: table.ID}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "TableHasActiveReservation",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				tableWithReservation := table
				tableWithReservation.CurrentReservationID = pgtype.Int8{Int64: 123, Valid: true}

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(tableWithReservation, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name:    "TableHasFutureReservations",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					DeleteTableTx(gomock.Any(), gomock.Eq(db.DeleteTableParams{
						TableID: table.ID,
					})).
					Times(1).
					Return(db.DeleteTableResult{}, db.ErrTableHasFutureReservations)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name:    "TableBecomesActiveReservationDuringDelete",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					DeleteTableTx(gomock.Any(), gomock.Eq(db.DeleteTableParams{
						TableID: table.ID,
					})).
					Times(1).
					Return(db.DeleteTableResult{}, db.ErrTableHasActiveReservation)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name:    "TableDeletedDuringDeleteTx",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					DeleteTableTx(gomock.Any(), gomock.Eq(db.DeleteTableParams{
						TableID: table.ID,
					})).
					Times(1).
					Return(db.DeleteTableResult{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "NotFound",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/tables/%d", tc.tableID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== TestUpdateTableStatusAPI ====================

func TestUpdateTableStatusAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)

	testCases := []struct {
		name          string
		tableID       int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			body: gin.H{
				"status": "occupied",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				updatedTable := table
				updatedTable.Status = "occupied"

				store.EXPECT().
					UpdateTableStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(updatedTable, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "CashierAllowed",
			tableID: table.ID,
			body: gin.H{
				"status": "occupied",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				staffMerchant := merchant
				staffMerchant.OwnerUserID = user.ID + 100

				expectResolveSingleStaffMerchant(store, user.ID, staffMerchant)

				store.EXPECT().
					GetUserMerchantRole(gomock.Any(), db.GetUserMerchantRoleParams{
						MerchantID: merchant.ID,
						UserID:     user.ID,
					}).
					Times(1).
					Return("cashier", nil)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				updatedTable := table
				updatedTable.Status = "occupied"

				store.EXPECT().
					UpdateTableStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(updatedTable, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "AlreadyDisabledStillUsesTransactionForDisabledRequest",
			tableID: table.ID,
			body: gin.H{
				"status": db.TableStatusDisabled,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				disabledTable := table
				disabledTable.Status = db.TableStatusDisabled

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(disabledTable, nil)

				store.EXPECT().
					UpdateTableStatusTx(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateTableStatusTxParams{})).
					DoAndReturn(func(_ context.Context, arg db.UpdateTableStatusTxParams) (db.Table, error) {
						require.True(t, arg.RequireNoFutureReservations)
						require.Equal(t, table.ID, arg.ID)
						require.Equal(t, db.TableStatusDisabled, arg.Status)
						return disabledTable, nil
					}).
					Times(1)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "ReleaseTableFailsWhenReservationCleanupFails",
			tableID: table.ID,
			body: gin.H{
				"status": "available",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				tableWithReservation := table
				tableWithReservation.CurrentReservationID = pgtype.Int8{Int64: 88, Valid: true}

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(tableWithReservation, nil)

				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(db.DiningSession{}, db.ErrRecordNotFound)

				store.EXPECT().
					UpdateReservationToCompleted(gomock.Any(), gomock.Eq(int64(88))).
					Times(1).
					Return(db.TableReservation{}, fmt.Errorf("update reservation failed"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, "internal server error", resp.Message)
			},
		},
		{
			name:    "InvalidStatus",
			tableID: table.ID,
			body: gin.H{
				"status": "invalid_status",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "TableNotFound",
			tableID: table.ID,
			body: gin.H{
				"status": "occupied",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "ForbiddenNotOwner",
			tableID: table.ID,
			body: gin.H{
				"status": "occupied",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				otherTable := table
				otherTable.MerchantID = merchant.ID + 1 // 不同商户

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(otherTable, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/tables/%d/status", tc.tableID)
			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateTableStatusAPIRejectsDisableWithFutureReservations(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetTable(gomock.Any(), gomock.Eq(table.ID)).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		UpdateTableStatusTx(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateTableStatusTxParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateTableStatusTxParams) (db.Table, error) {
			require.True(t, arg.RequireNoFutureReservations)
			require.Equal(t, table.ID, arg.ID)
			require.Equal(t, db.TableStatusDisabled, arg.Status)
			return db.Table{}, db.ErrTableHasFutureReservations
		}).
		Times(1)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body := gin.H{
		"status": db.TableStatusDisabled,
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/tables/%d/status", table.ID)
	request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusConflict, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "cannot disable table with future reservations", resp.Message)
}

func TestUpdateTableStatusAPIRejectsDeletedTableAfterPrecheck(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetTable(gomock.Any(), gomock.Eq(table.ID)).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		UpdateTableStatusTx(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateTableStatusTxParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateTableStatusTxParams) (db.Table, error) {
			require.True(t, arg.RequireNoFutureReservations)
			require.Equal(t, table.ID, arg.ID)
			require.Equal(t, db.TableStatusDisabled, arg.Status)
			return db.Table{}, fmt.Errorf("lock table: %w", db.ErrRecordNotFound)
		}).
		Times(1)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body := gin.H{
		"status": db.TableStatusDisabled,
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/tables/%d/status", table.ID)
	request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

// ==================== TestAddTableTagAPI ====================

func TestAddTableTagAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)
	tagID := util.RandomInt(1, 100)

	testCases := []struct {
		name          string
		tableID       int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			body: gin.H{
				"tag_id": tagID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetTag(gomock.Any(), gomock.Eq(tagID)).
					Times(1).
					Return(db.Tag{ID: tagID, Type: "table"}, nil)

				store.EXPECT().
					AddTableTag(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.TableTag{
						ID:      1,
						TableID: table.ID,
						TagID:   tagID,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "TableNotFound",
			tableID: table.ID,
			body: gin.H{
				"tag_id": tagID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "InvalidTagID",
			tableID: table.ID,
			body: gin.H{
				"tag_id": 0,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/tables/%d/tags", tc.tableID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			request.Header.Set("Content-Type", "application/json")
			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== TestRemoveTableTagAPI ====================

func TestRemoveTableTagAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)
	tagID := util.RandomInt(1, 100)

	testCases := []struct {
		name          string
		tableID       int64
		tagID         int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			tagID:   tagID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					RemoveTableTag(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "TableNotFound",
			tableID: table.ID,
			tagID:   tagID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "InvalidTagID",
			tableID: table.ID,
			tagID:   0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/tables/%d/tags/%d", tc.tableID, tc.tagID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== TestListTableTagsAPI ====================

func TestListTableTagsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)

	testCases := []struct {
		name          string
		tableID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					ListTableTags(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return([]db.ListTableTagsRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "InvalidTableID",
			tableID: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/tables/%d/tags", tc.tableID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== TestListAvailableRoomsAPI ====================

func TestListAvailableRoomsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	testCases := []struct {
		name          string
		merchantID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchant.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListAvailableRoomsForCustomer(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return([]db.ListAvailableRoomsForCustomerRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:       "InvalidMerchantID",
			merchantID: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchants/%d/rooms", tc.merchantID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== TestGetRoomDetailAPI ====================

func TestGetRoomDetailAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	room := randomTable(merchant.ID)
	room.TableType = "room"

	testCases := []struct {
		name          string
		roomID        int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "OK",
			roomID: room.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRoomDetailForCustomer(gomock.Any(), gomock.Eq(room.ID)).
					Times(1).
					Return(db.GetRoomDetailForCustomerRow{
						ID:           room.ID,
						MerchantID:   room.MerchantID,
						TableNo:      room.TableNo,
						Capacity:     room.Capacity,
						Status:       room.Status,
						MerchantName: "Test Merchant",
					}, nil)

				store.EXPECT().
					ListTableTags(gomock.Any(), gomock.Eq(room.ID)).
					Times(1).
					Return([]db.ListTableTagsRow{}, nil)

				store.EXPECT().
					ListTableImages(gomock.Any(), gomock.Eq(room.ID)).
					Times(1).
					Return([]db.TableImage{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:   "RoomNotFound",
			roomID: room.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRoomDetailForCustomer(gomock.Any(), gomock.Eq(room.ID)).
					Times(1).
					Return(db.GetRoomDetailForCustomerRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/rooms/%d", tc.roomID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== TestGetRoomAvailabilityAPI ====================

func TestGetRoomAvailabilityAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	room := randomTable(merchant.ID)
	room.TableType = "room"

	testCases := []struct {
		name          string
		roomID        int64
		date          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "OK",
			roomID: room.ID,
			date:   "2025-01-15",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(room.ID)).
					Times(1).
					Return(room, nil)

				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.TableReservation{}, nil)

				// 商户营业时间：午餐 11:00-13:00，晚餐 17:00-20:00（30分钟间隔将产生 12 个 time slots）
				store.EXPECT().
					ListMerchantBusinessHours(gomock.Any(), gomock.Eq(room.MerchantID)).
					Times(1).
					Return([]db.MerchantBusinessHour{
						{
							MerchantID:  room.MerchantID,
							DayOfWeek:   3, // 2025-01-15 是周三
							OpenTime:    pgtype.Time{Microseconds: 11 * 60 * 60 * 1_000_000, Valid: true},
							CloseTime:   pgtype.Time{Microseconds: 13 * 60 * 60 * 1_000_000, Valid: true},
							IsClosed:    false,
							SpecialDate: pgtype.Date{Valid: false},
						},
						{
							MerchantID:  room.MerchantID,
							DayOfWeek:   3,
							OpenTime:    pgtype.Time{Microseconds: 17 * 60 * 60 * 1_000_000, Valid: true},
							CloseTime:   pgtype.Time{Microseconds: 20 * 60 * 60 * 1_000_000, Valid: true},
							IsClosed:    false,
							SpecialDate: pgtype.Date{Valid: false},
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				// 验证返回的时间段数量 (午餐 11:00-13:00 + 晚餐 17:00-20:00, 每30分钟；按可用“开始时间”生成，不包含关店时刻)
				data, err := io.ReadAll(recorder.Body)
				require.NoError(t, err)
				var resp roomAvailabilityResponse
				requireUnmarshalAPIResponseData(t, data, &resp)
				require.Equal(t, 10, len(resp.TimeSlots)) // 11:00-13:00 共4个，17:00-20:00 共6个
				require.Equal(t, "2025-01-15", resp.Date)
			},
		},
		{
			name:   "RoomNotFound",
			roomID: room.ID,
			date:   "2025-01-15",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(room.ID)).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:   "InvalidDateFormat",
			roomID: room.ID,
			date:   "2025/01/15", // 错误格式
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:   "MissingDate",
			roomID: room.ID,
			date:   "",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/rooms/%d/availability", tc.roomID)
			if tc.date != "" {
				url = fmt.Sprintf("%s?date=%s", url, tc.date)
			}
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 辅助函数 ====================

func requireBodyMatchTable(t *testing.T, body *bytes.Buffer, table db.Table) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var gotTable tableResponse
	requireUnmarshalAPIResponseData(t, data, &gotTable)

	require.Equal(t, table.ID, gotTable.ID)
	require.Equal(t, table.MerchantID, gotTable.MerchantID)
	require.Equal(t, table.TableNo, gotTable.TableNo)
	require.Equal(t, table.TableType, gotTable.TableType)
	require.Equal(t, table.Capacity, gotTable.Capacity)
	require.Equal(t, table.Status, gotTable.Status)
}

// ==================== 包间图片管理测试 ====================

func TestAddTableImageAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomRoom(merchant.ID)

	testCases := []struct {
		name          string
		tableID       int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			body: gin.H{
				"media_asset_id": 1,
				"sort_order":     1,
				"is_primary":     true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomTableImageMediaAsset(1, user.ID, "merchant/table/1/table_detail.jpg")
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), asset.ID).
					Times(1).
					Return(asset, nil)

				// 因为 is_primary=true，需要先清除其他主图
				store.EXPECT().
					SetPrimaryTableImage(gomock.Any(), table.ID).
					Times(1).
					Return(nil)

				store.EXPECT().
					AddTableImage(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.TableImage{
						ID:           1,
						TableID:      table.ID,
						MediaAssetID: pgtype.Int8{Int64: asset.ID, Valid: true},
						SortOrder:    1,
						IsPrimary:    true,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				body := recorder.Body.String()
				require.Contains(t, body, "image_url")
				require.Contains(t, body, "merchant/table/1/table_detail.jpg")
			},
		},
		{
			name:    "OK_NotPrimary",
			tableID: table.ID,
			body: gin.H{
				"media_asset_id": 2,
				"sort_order":     2,
				"is_primary":     false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomTableImageMediaAsset(2, user.ID, "merchant/table/2/table_detail.jpg")
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), asset.ID).
					Times(1).
					Return(asset, nil)

				// 不是主图，不调用 SetPrimaryTableImage

				store.EXPECT().
					AddTableImage(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.TableImage{
						ID:           2,
						TableID:      table.ID,
						MediaAssetID: pgtype.Int8{Int64: asset.ID, Valid: true},
						SortOrder:    2,
						IsPrimary:    false,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				require.Contains(t, recorder.Body.String(), "merchant/table/2/table_detail.jpg")
			},
		},
		{
			name:    "OK_UploadedByMerchantStaff",
			tableID: table.ID,
			body: gin.H{
				"media_asset_id": 3,
				"sort_order":     3,
				"is_primary":     false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				uploaderID := user.ID + 1000
				asset := randomTableImageMediaAsset(3, uploaderID, "merchant/table/3/staff_table_detail.jpg")
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), asset.ID).
					Times(1).
					Return(asset, nil)

				store.EXPECT().
					GetUserMerchantRole(gomock.Any(), db.GetUserMerchantRoleParams{
						MerchantID: merchant.ID,
						UserID:     uploaderID,
					}).
					Times(1).
					Return("manager", nil)

				store.EXPECT().
					AddTableImage(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.TableImage{
						ID:           3,
						TableID:      table.ID,
						MediaAssetID: pgtype.Int8{Int64: asset.ID, Valid: true},
						SortOrder:    3,
						IsPrimary:    false,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				require.Contains(t, recorder.Body.String(), "merchant/table/3/staff_table_detail.jpg")
			},
		},
		{
			name:    "RejectWrongMerchantMediaAsset",
			tableID: table.ID,
			body: gin.H{
				"media_asset_id": 4,
				"is_primary":     false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				uploaderID := user.ID + 2000
				asset := randomTableImageMediaAsset(4, uploaderID, "merchant/table/4/other_merchant.jpg")
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), asset.ID).
					Times(1).
					Return(asset, nil)

				store.EXPECT().
					GetUserMerchantRole(gomock.Any(), db.GetUserMerchantRoleParams{
						MerchantID: merchant.ID,
						UserID:     uploaderID,
					}).
					Times(1).
					Return("", db.ErrRecordNotFound)

				store.EXPECT().
					AddTableImage(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "RejectPendingStaffMediaAsset",
			tableID: table.ID,
			body: gin.H{
				"media_asset_id": 5,
				"is_primary":     false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				uploaderID := user.ID + 3000
				asset := randomTableImageMediaAsset(5, uploaderID, "merchant/table/5/pending_staff.jpg")
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), asset.ID).
					Times(1).
					Return(asset, nil)

				store.EXPECT().
					GetUserMerchantRole(gomock.Any(), db.GetUserMerchantRoleParams{
						MerchantID: merchant.ID,
						UserID:     uploaderID,
					}).
					Times(1).
					Return("pending", nil)

				store.EXPECT().
					AddTableImage(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "RejectWrongMediaCategoryBeforePrimaryReset",
			tableID: table.ID,
			body: gin.H{
				"media_asset_id": 6,
				"is_primary":     true,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomMediaAsset(6, user.ID, "public", "merchant/dish/6/not_table.jpg")
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), asset.ID).
					Times(1).
					Return(asset, nil)

				store.EXPECT().
					SetPrimaryTableImage(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					AddTableImage(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "RejectUnconfirmedMediaAsset",
			tableID: table.ID,
			body: gin.H{
				"media_asset_id": 7,
				"is_primary":     false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomTableImageMediaAsset(7, user.ID, "merchant/table/7/uploaded_only.jpg")
				asset.UploadStatus = "uploaded"
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), asset.ID).
					Times(1).
					Return(asset, nil)

				store.EXPECT().
					AddTableImage(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "RejectUnapprovedMediaAsset",
			tableID: table.ID,
			body: gin.H{
				"media_asset_id": 8,
				"is_primary":     false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomTableImageMediaAsset(8, user.ID, "merchant/table/8/pending_moderation.jpg")
				asset.ModerationStatus = "pending"
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), asset.ID).
					Times(1).
					Return(asset, nil)

				store.EXPECT().
					AddTableImage(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "RejectPrivateMediaAsset",
			tableID: table.ID,
			body: gin.H{
				"media_asset_id": 9,
				"is_primary":     false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomTableImageMediaAsset(9, user.ID, "id_card/front/9/private.jpg")
				asset.Visibility = string(media.VisibilityPrivate)
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), asset.ID).
					Times(1).
					Return(asset, nil)

				store.EXPECT().
					AddTableImage(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "RejectMissingMediaAsset",
			tableID: table.ID,
			body: gin.H{
				"media_asset_id": 10,
				"is_primary":     false,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), int64(10)).
					Times(1).
					Return(db.MediaAsset{}, db.ErrRecordNotFound)

				store.EXPECT().
					AddTableImage(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "NotMerchant",
			tableID: table.ID,
			body: gin.H{
				"media_asset_id": 1,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "TableNotFound",
			tableID: 99999,
			body: gin.H{
				"media_asset_id": 1,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "InvalidMediaAssetID",
			tableID: table.ID,
			body: gin.H{
				"media_asset_id": 0,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/tables/%d/images", tc.tableID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListTableImagesAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomRoom(merchant.ID)

	testCases := []struct {
		name          string
		tableID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				assetRows := []db.ListMediaAssetsByIDsRow{
					{
						ID:               11,
						ObjectKey:        "merchant/table/11/table_1.jpg",
						Visibility:       "public",
						ModerationStatus: "approved",
					},
					{
						ID:               12,
						ObjectKey:        "merchant/table/12/table_2.jpg",
						Visibility:       "public",
						ModerationStatus: "approved",
					},
				}
				store.EXPECT().
					ListTableImages(gomock.Any(), table.ID).
					Times(1).
					Return([]db.TableImage{
						{ID: 1, TableID: table.ID, MediaAssetID: pgtype.Int8{Int64: 11, Valid: true}, IsPrimary: true},
						{ID: 2, TableID: table.ID, MediaAssetID: pgtype.Int8{Int64: 12, Valid: true}, IsPrimary: false},
					}, nil)
				store.EXPECT().
					ListMediaAssetsByIDs(gomock.Any(), gomock.Any()).
					Times(1).
					Return(assetRows, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				body := recorder.Body.String()
				require.Contains(t, body, "merchant/table/11/table_1.jpg")
				require.Contains(t, body, "merchant/table/12/table_2.jpg")
			},
		},
		{
			name:    "InvalidTableID",
			tableID: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "InternalError",
			tableID: table.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(table.ID)).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					ListTableImages(gomock.Any(), table.ID).
					Times(1).
					Return(nil, errors.New("internal error"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/tables/%d/images", tc.tableID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestDeleteTableImageAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomRoom(merchant.ID)

	testCases := []struct {
		name          string
		tableID       int64
		imageID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			imageID: 1,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					DeleteTableImage(gomock.Any(), db.DeleteTableImageParams{
						TableID: table.ID,
						ID:      int64(1),
					}).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "NotMerchant",
			tableID: table.ID,
			imageID: 1,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "TableNotFound",
			tableID: 99999,
			imageID: 1,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "ImageNotFound",
			tableID: table.ID,
			imageID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					DeleteTableImage(gomock.Any(), db.DeleteTableImageParams{
						TableID: table.ID,
						ID:      int64(99999),
					}).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/tables/%d/images/%d", tc.tableID, tc.imageID)
			request, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestSetTableImagePrimaryAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomRoom(merchant.ID)

	testCases := []struct {
		name          string
		tableID       int64
		imageID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			tableID: table.ID,
			imageID: 1,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				asset := randomMediaAsset(1, user.ID, "public", "merchant/table/1/primary.jpg")
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					SetTableImagePrimaryTx(gomock.Any(), db.SetTableImagePrimaryTxParams{
						TableID: table.ID,
						ImageID: int64(1),
					}).
					Times(1).
					Return(db.TableImage{
						ID:           1,
						TableID:      table.ID,
						MediaAssetID: pgtype.Int8{Int64: asset.ID, Valid: true},
						IsPrimary:    true,
					}, nil)

				store.EXPECT().
					GetMediaAssetByID(gomock.Any(), asset.ID).
					Times(1).
					Return(asset, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				require.Contains(t, recorder.Body.String(), "merchant/table/1/primary.jpg")
			},
		},
		{
			name:    "NotMerchant",
			tableID: table.ID,
			imageID: 1,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, user.ID)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "ImageNotFound",
			tableID: table.ID,
			imageID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					SetTableImagePrimaryTx(gomock.Any(), db.SetTableImagePrimaryTxParams{
						TableID: table.ID,
						ImageID: int64(99999),
					}).
					Times(1).
					Return(db.TableImage{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/tables/%d/images/%d/primary", tc.tableID, tc.imageID)
			request, err := http.NewRequest(http.MethodPut, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 商户所有包间列表测试 ====================

func TestListMerchantRoomsForCustomerAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchantID := int64(123)

	testCases := []struct {
		name          string
		merchantID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "OK",
			merchantID: merchantID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListMerchantRoomsForCustomer(gomock.Any(), merchantID).
					Times(1).
					Return([]db.ListMerchantRoomsForCustomerRow{
						{
							ID:                  1,
							MerchantID:          merchantID,
							TableNo:             "R01",
							Capacity:            8,
							Status:              "available",
							MonthlyReservations: 15,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:       "InvalidMerchantID",
			merchantID: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchants/%d/rooms/all", tc.merchantID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestGetRoomDetailAPI_WithImages 回归测试（Phase 5.2）：
// 当包间存在关联图片资产时，GET /v1/rooms/{id} 响应中应包含 CDN image_urls。
func TestGetRoomDetailAPI_WithImages(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	room := randomTable(merchant.ID)

	const imageAssetID int64 = 77
	tableImage := db.TableImage{
		TableID:      room.ID,
		MediaAssetID: pgtype.Int8{Int64: imageAssetID, Valid: true},
	}
	imageAsset := db.ListMediaAssetsByIDsRow{
		ID:               imageAssetID,
		ObjectKey:        "table/room/77/room_photo.jpg",
		Visibility:       "public",
		ModerationStatus: "approved",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRoomDetailForCustomer(gomock.Any(), gomock.Eq(room.ID)).
		Times(1).
		Return(db.GetRoomDetailForCustomerRow{
			ID:           room.ID,
			MerchantID:   room.MerchantID,
			TableNo:      room.TableNo,
			Capacity:     room.Capacity,
			Status:       room.Status,
			MerchantName: "Test Merchant",
		}, nil)
	store.EXPECT().
		ListTableTags(gomock.Any(), gomock.Eq(room.ID)).
		Times(1).Return([]db.ListTableTagsRow{}, nil)
	store.EXPECT().
		ListTableImages(gomock.Any(), gomock.Eq(room.ID)).
		Times(1).Return([]db.TableImage{tableImage}, nil)
	store.EXPECT().
		ListMediaAssetsByIDs(gomock.Any(), gomock.Any()).
		Times(1).Return([]db.ListMediaAssetsByIDsRow{imageAsset}, nil)

	server, _ := newTestServerForMedia(t, store)

	url := fmt.Sprintf("/v1/rooms/%d", room.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp roomDetailResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.ImageURLs, 1, "包间应有 1 条 image_url")
	require.Contains(t, resp.ImageURLs[0], "https://cdn.test.example.com", "image_url 应指向 CDN 域名")
	require.Contains(t, resp.ImageURLs[0], imageAsset.ObjectKey, "image_url 应包含 object key")
}
