package api

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type stubMapClient struct {
	reverseResult *maps.ReverseGeocodeResult
	reverseErr    error
}

func (s stubMapClient) GetBicyclingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, errors.New("not implemented")
}

func (s stubMapClient) GetWalkingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, errors.New("not implemented")
}

func (s stubMapClient) GetDrivingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, errors.New("not implemented")
}

func (s stubMapClient) GetDistanceMatrix(ctx context.Context, froms, tos []maps.Location, mode string) (*maps.DistanceMatrixResult, error) {
	return nil, errors.New("not implemented")
}

func (s stubMapClient) Geocode(ctx context.Context, address string) (*maps.GeocodeResult, error) {
	return nil, errors.New("not implemented")
}

func (s stubMapClient) ReverseGeocode(ctx context.Context, location maps.Location) (*maps.ReverseGeocodeResult, error) {
	return s.reverseResult, s.reverseErr
}

func TestMatchRegionID_UsesClosestWhenNoMapClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := &Server{store: store}

	lat := 39.9
	lon := 116.4
	region := db.Region{ID: 100, Name: "朝阳区"}

	store.EXPECT().
		GetClosestRegion(gomock.Any(), db.GetClosestRegionParams{Lat: lat, Lon: lon}).
		Times(1).
		Return(region, nil)

	regionID, err := server.matchRegionID(context.Background(), lat, lon)
	require.NoError(t, err)
	require.Equal(t, region.ID, regionID)
}

func TestMatchRegionID_UsesAdcodeWhenAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	mapClient := stubMapClient{reverseResult: &maps.ReverseGeocodeResult{Adcode: "110105"}}
	server := &Server{store: store, mapClient: mapClient}

	lat := 39.9
	lon := 116.4
	region := db.Region{ID: 200, Code: "110105"}

	store.EXPECT().
		GetRegionByCode(gomock.Any(), "110105").
		Times(1).
		Return(region, nil)

	regionID, err := server.matchRegionID(context.Background(), lat, lon)
	require.NoError(t, err)
	require.Equal(t, region.ID, regionID)
}

func TestMatchRegionID_UsesDistrictFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	mapClient := stubMapClient{reverseResult: &maps.ReverseGeocodeResult{Adcode: "100000", City: "北京市", District: "朝阳区"}}
	server := &Server{store: store, mapClient: mapClient}

	lat := 39.9
	lon := 116.4
	cityRegion := db.Region{ID: 10, Name: "北京市", Level: 2}
	districtRegion := db.Region{ID: 20, Name: "朝阳区", Level: 3}

	store.EXPECT().
		GetRegionByCode(gomock.Any(), "100000").
		Times(1).
		Return(db.Region{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetRegionByNameAndLevel(gomock.Any(), db.GetRegionByNameAndLevelParams{Name: "北京市", Level: 2}).
		Times(1).
		Return(cityRegion, nil)
	store.EXPECT().
		GetRegionByNameAndParent(gomock.Any(), db.GetRegionByNameAndParentParams{Name: "朝阳区", ParentID: pgtype.Int8{Int64: cityRegion.ID, Valid: true}}).
		Times(1).
		Return(districtRegion, nil)

	regionID, err := server.matchRegionID(context.Background(), lat, lon)
	require.NoError(t, err)
	require.Equal(t, districtRegion.ID, regionID)
}

func TestMatchRegionID_UsesCountyLevelFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	mapClient := stubMapClient{reverseResult: &maps.ReverseGeocodeResult{Adcode: "100000", City: "衡水市", District: "景县"}}
	server := &Server{store: store, mapClient: mapClient}

	lat := 37.0611534
	lon := 115.0554199
	countyRegion := db.Region{ID: 30, Name: "景县", Level: 4}

	store.EXPECT().
		GetRegionByCode(gomock.Any(), "100000").
		Times(1).
		Return(db.Region{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetRegionByNameAndLevel(gomock.Any(), db.GetRegionByNameAndLevelParams{Name: "衡水市", Level: 2}).
		Times(1).
		Return(db.Region{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetRegionByNameAndLevel(gomock.Any(), db.GetRegionByNameAndLevelParams{Name: "景县", Level: 3}).
		Times(1).
		Return(db.Region{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetRegionByNameAndLevel(gomock.Any(), db.GetRegionByNameAndLevelParams{Name: "景县", Level: 4}).
		Times(1).
		Return(countyRegion, nil)

	regionID, err := server.matchRegionID(context.Background(), lat, lon)
	require.NoError(t, err)
	require.Equal(t, countyRegion.ID, regionID)
}
