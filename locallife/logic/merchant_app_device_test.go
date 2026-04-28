package logic

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRegisterMerchantAppDevice(t *testing.T) {
	input := MerchantAppDeviceRegisterInput{
		MerchantID:  10,
		UserID:      20,
		DeviceID:    " device-1 ",
		PushToken:   " token-1 ",
		Platform:    " Android ",
		Provider:    " XiaoMi ",
		DeviceModel: " Redmi K70 ",
		OSVersion:   " Android 15 ",
		AppVersion:  " 1.0.0 ",
	}

	testCases := []struct {
		name       string
		input      MerchantAppDeviceRegisterInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result MerchantAppDeviceResult, err error)
	}{
		{
			name:  "Success",
			input: input,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					RegisterMerchantAppDeviceTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.RegisterMerchantAppDeviceParams) (db.MerchantAppDevice, error) {
						require.Equal(t, int64(10), arg.MerchantID)
						require.Equal(t, int64(20), arg.UserID)
						require.Equal(t, "device-1", arg.DeviceID)
						require.Equal(t, "token-1", arg.PushToken)
						require.Equal(t, db.MerchantAppDevicePlatformAndroid, arg.Platform)
						require.Equal(t, db.MerchantAppDeviceProviderXiaomi, arg.Provider)
						require.Equal(t, pgtype.Text{String: "Redmi K70", Valid: true}, arg.DeviceModel)
						require.Equal(t, pgtype.Text{String: "Android 15", Valid: true}, arg.OsVersion)
						require.Equal(t, pgtype.Text{String: "1.0.0", Valid: true}, arg.AppVersion)
						return db.MerchantAppDevice{MerchantID: arg.MerchantID, UserID: arg.UserID, DeviceID: arg.DeviceID, Provider: arg.Provider}, nil
					})
			},
			check: func(t *testing.T, result MerchantAppDeviceResult, err error) {
				require.NoError(t, err)
				require.True(t, result.Registered)
				require.Equal(t, "device-1", result.DeviceID)
				require.Equal(t, db.MerchantAppDeviceProviderXiaomi, result.Device.Provider)
			},
		},
		{
			name:  "DefaultUnknownProvider",
			input: MerchantAppDeviceRegisterInput{MerchantID: 10, UserID: 20, DeviceID: "device-1", PushToken: "token-1", Platform: "android"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					RegisterMerchantAppDeviceTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.RegisterMerchantAppDeviceParams) (db.MerchantAppDevice, error) {
						require.Equal(t, db.MerchantAppDeviceProviderUnknown, arg.Provider)
						return db.MerchantAppDevice{DeviceID: arg.DeviceID, Provider: arg.Provider}, nil
					})
			},
			check: func(t *testing.T, result MerchantAppDeviceResult, err error) {
				require.NoError(t, err)
				require.Equal(t, db.MerchantAppDeviceProviderUnknown, result.Device.Provider)
			},
		},
		{
			name:       "MissingDeviceID",
			input:      MerchantAppDeviceRegisterInput{MerchantID: 10, UserID: 20, PushToken: "token-1", Platform: "android"},
			buildStubs: func(store *mockdb.MockStore) {},
			check: func(t *testing.T, _ MerchantAppDeviceResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "device_id is required", reqErr.Err.Error())
			},
		},
		{
			name:       "UnsupportedProvider",
			input:      MerchantAppDeviceRegisterInput{MerchantID: 10, UserID: 20, DeviceID: "device-1", PushToken: "token-1", Platform: "android", Provider: "jpush"},
			buildStubs: func(store *mockdb.MockStore) {},
			check: func(t *testing.T, _ MerchantAppDeviceResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "unsupported provider", reqErr.Err.Error())
			},
		},
		{
			name:       "UnsupportedPlatform",
			input:      MerchantAppDeviceRegisterInput{MerchantID: 10, UserID: 20, DeviceID: "device-1", PushToken: "token-1", Platform: "ios"},
			buildStubs: func(store *mockdb.MockStore) {},
			check: func(t *testing.T, _ MerchantAppDeviceResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "unsupported platform", reqErr.Err.Error())
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			result, err := RegisterMerchantAppDevice(context.Background(), store, tc.input)
			tc.check(t, result, err)
		})
	}
}

func TestHeartbeatMerchantAppDevice(t *testing.T) {
	input := MerchantAppDeviceHeartbeatInput{
		MerchantID:  10,
		UserID:      20,
		DeviceID:    " device-1 ",
		Provider:    " vivo ",
		PushToken:   " token-2 ",
		DeviceModel: " X100 ",
		OSVersion:   " Android 15 ",
		AppVersion:  " 1.0.1 ",
	}

	testCases := []struct {
		name       string
		input      MerchantAppDeviceHeartbeatInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result MerchantAppDeviceResult, err error)
	}{
		{
			name:  "Success",
			input: input,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateMerchantAppDeviceHeartbeatTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateMerchantAppDeviceHeartbeatParams) (db.MerchantAppDevice, error) {
						require.Equal(t, int64(10), arg.MerchantID)
						require.Equal(t, "device-1", arg.DeviceID)
						require.Equal(t, pgtype.Text{String: db.MerchantAppDeviceProviderVivo, Valid: true}, arg.Provider)
						require.Equal(t, pgtype.Text{String: "token-2", Valid: true}, arg.PushToken)
						require.Equal(t, pgtype.Text{String: "X100", Valid: true}, arg.DeviceModel)
						return db.MerchantAppDevice{MerchantID: arg.MerchantID, DeviceID: arg.DeviceID, Provider: arg.Provider.String}, nil
					})
			},
			check: func(t *testing.T, result MerchantAppDeviceResult, err error) {
				require.NoError(t, err)
				require.True(t, result.Heartbeat)
				require.Equal(t, "device-1", result.DeviceID)
			},
		},
		{
			name:  "MetadataOnlyHeartbeat",
			input: MerchantAppDeviceHeartbeatInput{MerchantID: 10, UserID: 20, DeviceID: "device-1"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateMerchantAppDeviceHeartbeatTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateMerchantAppDeviceHeartbeatParams) (db.MerchantAppDevice, error) {
						require.False(t, arg.Provider.Valid)
						require.False(t, arg.PushToken.Valid)
						return db.MerchantAppDevice{DeviceID: arg.DeviceID}, nil
					})
			},
			check: func(t *testing.T, result MerchantAppDeviceResult, err error) {
				require.NoError(t, err)
				require.True(t, result.Heartbeat)
			},
		},
		{
			name:  "NotFound",
			input: MerchantAppDeviceHeartbeatInput{MerchantID: 10, UserID: 20, DeviceID: "device-1"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateMerchantAppDeviceHeartbeatTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantAppDevice{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ MerchantAppDeviceResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "merchant app device not found", reqErr.Err.Error())
			},
		},
		{
			name:       "UnsupportedProvider",
			input:      MerchantAppDeviceHeartbeatInput{MerchantID: 10, UserID: 20, DeviceID: "device-1", Provider: "jpush"},
			buildStubs: func(store *mockdb.MockStore) {},
			check: func(t *testing.T, _ MerchantAppDeviceResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "unsupported provider", reqErr.Err.Error())
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			result, err := HeartbeatMerchantAppDevice(context.Background(), store, tc.input)
			tc.check(t, result, err)
		})
	}
}

func TestUnregisterMerchantAppDevice(t *testing.T) {
	testCases := []struct {
		name       string
		input      MerchantAppDeviceUnregisterInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result MerchantAppDeviceResult, err error)
	}{
		{
			name:  "Success",
			input: MerchantAppDeviceUnregisterInput{MerchantID: 10, UserID: 20, DeviceID: " device-1 "},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UnregisterMerchantAppDevice(gomock.Any(), db.UnregisterMerchantAppDeviceParams{MerchantID: 10, DeviceID: "device-1"}).
					Times(1).
					Return(int64(1), nil)
			},
			check: func(t *testing.T, result MerchantAppDeviceResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "device-1", result.DeviceID)
				require.True(t, result.Unregistered)
			},
		},
		{
			name:  "IdempotentMissingDevice",
			input: MerchantAppDeviceUnregisterInput{MerchantID: 10, UserID: 20, DeviceID: "device-1"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UnregisterMerchantAppDevice(gomock.Any(), db.UnregisterMerchantAppDeviceParams{MerchantID: 10, DeviceID: "device-1"}).
					Times(1).
					Return(int64(0), nil)
			},
			check: func(t *testing.T, result MerchantAppDeviceResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "device-1", result.DeviceID)
				require.False(t, result.Unregistered)
			},
		},
		{
			name:       "MissingUserID",
			input:      MerchantAppDeviceUnregisterInput{MerchantID: 10, DeviceID: "device-1"},
			buildStubs: func(store *mockdb.MockStore) {},
			check: func(t *testing.T, _ MerchantAppDeviceResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "user_id is required", reqErr.Err.Error())
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			result, err := UnregisterMerchantAppDevice(context.Background(), store, tc.input)
			tc.check(t, result, err)
		})
	}
}
