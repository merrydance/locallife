package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetRegionAPI(t *testing.T) {
	region := randomRegion()

	testCases := []struct {
		name          string
		url           string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			url:  fmt.Sprintf("/v1/regions/%d", region.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(region, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchRegion(t, recorder.Body, region)
			},
		},
		{
			name: "NotFound",
			url:  fmt.Sprintf("/v1/regions/%d", region.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(db.Region{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "InternalError",
			url:  fmt.Sprintf("/v1/regions/%d", region.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(db.Region{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "ZeroID_NotFound",
			url:  "/v1/regions/0",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(int64(0))).
					Times(1).
					Return(db.Region{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "InvalidID_BadRequest",
			url:  "/v1/regions/abc",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Any()).
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

			request, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListRegionsAPI(t *testing.T) {
	n := 5
	regions := make([]db.Region, n)
	for i := 0; i < n; i++ {
		regions[i] = randomRegion()
	}

	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=5",
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.ListRegionsParams{
					Limit:  5,
					Offset: 0,
				}
				store.EXPECT().
					ListRegions(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(regions, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchRegions(t, recorder.Body, regions)
			},
		},
		{
			name:  "OK_WithLevel",
			query: "?level=2&page_id=1&page_size=5",
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.ListRegionsParams{
					Limit:  5,
					Offset: 0,
					Level:  pgtype.Int2{Int16: 2, Valid: true},
				}
				store.EXPECT().
					ListRegions(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(regions, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_WithParentID",
			query: "?parent_id=123&page_id=1&page_size=5",
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.ListRegionsParams{
					Limit:    5,
					Offset:   0,
					ParentID: pgtype.Int8{Int64: 123, Valid: true},
				}
				store.EXPECT().
					ListRegions(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(regions, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidPageID",
			query: "?page_id=0&page_size=5",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListRegions(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidPageSize",
			query: "?page_id=1&page_size=200",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListRegions(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InternalError",
			query: "?page_id=1&page_size=5",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListRegions(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Region{}, sql.ErrConnDone)
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

			url := "/v1/regions" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListRegionChildrenAPI(t *testing.T) {
	parentRegion := randomRegion()
	childRegions := make([]db.Region, 3)
	for i := 0; i < 3; i++ {
		childRegions[i] = randomRegion()
		childRegions[i].ParentID = pgtype.Int8{Int64: parentRegion.ID, Valid: true}
	}

	testCases := []struct {
		name          string
		url           string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			url:  fmt.Sprintf("/v1/regions/%d/children", parentRegion.ID),
			buildStubs: func(store *mockdb.MockStore) {
				arg := pgtype.Int8{Int64: parentRegion.ID, Valid: true}
				store.EXPECT().
					ListRegionChildren(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(childRegions, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchRegions(t, recorder.Body, childRegions)
			},
		},
		{
			name: "EmptyChildren",
			url:  fmt.Sprintf("/v1/regions/%d/children", parentRegion.ID),
			buildStubs: func(store *mockdb.MockStore) {
				arg := pgtype.Int8{Int64: parentRegion.ID, Valid: true}
				store.EXPECT().
					ListRegionChildren(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return([]db.Region{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var regions []regionResponse
				err := json.NewDecoder(recorder.Body).Decode(&regions)
				require.NoError(t, err)
				require.Empty(t, regions)
			},
		},
		{
			name: "InternalError",
			url:  fmt.Sprintf("/v1/regions/%d/children", parentRegion.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListRegionChildren(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Region{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "ZeroID_OK",
			url:  "/v1/regions/0/children",
			buildStubs: func(store *mockdb.MockStore) {
				// ID为0也会被解析，但这是边界情况
				arg := pgtype.Int8{Int64: 0, Valid: true}
				store.EXPECT().
					ListRegionChildren(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return([]db.Region{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "InvalidID_BadRequest",
			url:  "/v1/regions/abc/children",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListRegionChildren(gomock.Any(), gomock.Any()).
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

			request, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestSearchRegionsAPI(t *testing.T) {
	regions := make([]db.Region, 3)
	for i := 0; i < 3; i++ {
		regions[i] = randomRegion()
	}

	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?q=test",
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.SearchRegionsByNameParams{
					Column1: pgtype.Text{String: "test", Valid: true},
					Limit:   100,
				}
				store.EXPECT().
					SearchRegionsByName(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(regions, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchRegions(t, recorder.Body, regions)
			},
		},
		{
			name:  "MissingQuery",
			query: "",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					SearchRegionsByName(gomock.Any(), gomock.Any()).
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

			url := "/v1/regions/search" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func randomRegion() db.Region {
	return db.Region{
		ID:    util.RandomInt(1, 1000),
		Code:  util.RandomString(6),
		Name:  util.RandomString(10),
		Level: int16(util.RandomInt(1, 4)),
		ParentID: pgtype.Int8{
			Int64: 0,
			Valid: false,
		},
		Longitude: pgtype.Numeric{
			Valid: false,
		},
		Latitude: pgtype.Numeric{
			Valid: false,
		},
	}
}

func requireBodyMatchRegion(t *testing.T, body *bytes.Buffer, region db.Region) {
	var gotRegion regionResponse
	err := json.NewDecoder(body).Decode(&gotRegion)
	require.NoError(t, err)
	require.Equal(t, region.ID, gotRegion.ID)
	require.Equal(t, region.Code, gotRegion.Code)
	require.Equal(t, region.Name, gotRegion.Name)
}

func requireBodyMatchRegions(t *testing.T, body *bytes.Buffer, regions []db.Region) {
	var gotRegions []regionResponse
	err := json.NewDecoder(body).Decode(&gotRegions)
	require.NoError(t, err)
	require.Equal(t, len(regions), len(gotRegions))
	for i, region := range regions {
		require.Equal(t, region.ID, gotRegions[i].ID)
		require.Equal(t, region.Code, gotRegions[i].Code)
		require.Equal(t, region.Name, gotRegions[i].Name)
	}
}

// ==================== 新增测试：listAvailableRegions ====================

func TestListAvailableRegionsAPI(t *testing.T) {
	parentRegion := randomRegion()
	parentRegion.Level = 2 // 市级

	availableRegions := make([]db.ListAvailableRegionsRow, 3)
	for i := 0; i < 3; i++ {
		region := randomRegion()
		region.Level = 3 // 区县级
		region.ParentID = pgtype.Int8{Int64: parentRegion.ID, Valid: true}
		availableRegions[i] = db.ListAvailableRegionsRow{
			ID:         region.ID,
			Code:       region.Code,
			Name:       region.Name,
			Level:      region.Level,
			ParentID:   region.ParentID,
			ParentName: pgtype.Text{String: parentRegion.Name, Valid: true},
		}
	}

	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.ListAvailableRegionsParams{
					Limit:  10,
					Offset: 0,
				}
				store.EXPECT().
					ListAvailableRegions(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(availableRegions, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response map[string]interface{}
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				regions, ok := response["regions"].([]interface{})
				require.True(t, ok)
				require.Equal(t, 3, len(regions))
			},
		},
		{
			name:  "OK_WithParentID",
			query: fmt.Sprintf("?page_id=1&page_size=10&parent_id=%d", parentRegion.ID),
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.ListAvailableRegionsParams{
					Limit:    10,
					Offset:   0,
					ParentID: pgtype.Int8{Int64: parentRegion.ID, Valid: true},
				}
				store.EXPECT().
					ListAvailableRegions(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(availableRegions, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "OK_WithLevel",
			query: "?page_id=1&page_size=10&level=3",
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.ListAvailableRegionsParams{
					Limit:  10,
					Offset: 0,
					Level:  pgtype.Int2{Int16: 3, Valid: true},
				}
				store.EXPECT().
					ListAvailableRegions(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(availableRegions, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "EmptyResult",
			query: "?page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListAvailableRegions(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListAvailableRegionsRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response map[string]interface{}
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				regions, ok := response["regions"].([]interface{})
				require.True(t, ok)
				require.Empty(t, regions)
			},
		},
		{
			name:  "InvalidPageID",
			query: "?page_id=0&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListAvailableRegions(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidPageSize",
			query: "?page_id=1&page_size=200",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListAvailableRegions(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidLevel",
			query: "?page_id=1&page_size=10&level=5",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListAvailableRegions(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InternalError",
			query: "?page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListAvailableRegions(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListAvailableRegionsRow{}, sql.ErrConnDone)
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

			url := "/v1/regions/available" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 新增测试：checkRegionAvailability ====================

func TestCheckRegionAvailabilityAPI(t *testing.T) {
	region := randomRegion()
	region.Level = 3 // 区县级

	operator := db.Operator{
		ID:       util.RandomInt(1, 1000),
		Name:     "测试运营商",
		RegionID: region.ID,
	}

	testCases := []struct {
		name          string
		url           string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK_Available",
			url:  fmt.Sprintf("/v1/regions/%d/check", region.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(region, nil)
				store.EXPECT().
					GetOperatorByRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(db.Operator{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response regionAvailabilityResponse
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, region.ID, response.RegionID)
				require.True(t, response.IsAvailable)
				require.Empty(t, response.Reason)
			},
		},
		{
			name: "OK_Unavailable",
			url:  fmt.Sprintf("/v1/regions/%d/check", region.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(region, nil)
				store.EXPECT().
					GetOperatorByRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(operator, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response regionAvailabilityResponse
				err := json.NewDecoder(recorder.Body).Decode(&response)
				require.NoError(t, err)
				require.Equal(t, region.ID, response.RegionID)
				require.False(t, response.IsAvailable)
				require.Contains(t, response.Reason, operator.Name)
			},
		},
		{
			name: "RegionNotFound",
			url:  fmt.Sprintf("/v1/regions/%d/check", region.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(db.Region{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "ZeroID_NotFound",
			url:  "/v1/regions/0/check",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(int64(0))).
					Times(1).
					Return(db.Region{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "InvalidID_BadRequest",
			url:  "/v1/regions/abc/check",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					GetOperatorByRegion(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "GetRegion_InternalError",
			url:  fmt.Sprintf("/v1/regions/%d/check", region.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(db.Region{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "GetOperatorByRegion_InternalError",
			url:  fmt.Sprintf("/v1/regions/%d/check", region.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(region, nil)
				store.EXPECT().
					GetOperatorByRegion(gomock.Any(), gomock.Eq(region.ID)).
					Times(1).
					Return(db.Operator{}, sql.ErrConnDone)
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

			request, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
