package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestValidateMerchantForOrder(t *testing.T) {
	merchantID := int64(101)

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, merchant db.Merchant, err error)
	}{
		{
			name: "NotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchantID).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ db.Merchant, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "merchant not found", reqErr.Err.Error())
			},
		},
		{
			name: "Inactive",
			buildStubs: func(store *mockdb.MockStore) {
				merchant := db.Merchant{ID: merchantID, Status: "pending", IsOpen: true}
				store.EXPECT().
					GetMerchant(gomock.Any(), merchantID).
					Times(1).
					Return(merchant, nil)
			},
			check: func(t *testing.T, _ db.Merchant, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "merchant is not active", reqErr.Err.Error())
			},
		},
		{
			name: "Closed",
			buildStubs: func(store *mockdb.MockStore) {
				merchant := db.Merchant{ID: merchantID, Status: "active", IsOpen: false}
				store.EXPECT().
					GetMerchant(gomock.Any(), merchantID).
					Times(1).
					Return(merchant, nil)
			},
			check: func(t *testing.T, _ db.Merchant, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "商户已打烊，暂时无法接单", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				merchant := db.Merchant{ID: merchantID, Status: "active", IsOpen: true}
				store.EXPECT().
					GetMerchant(gomock.Any(), merchantID).
					Times(1).
					Return(merchant, nil)
			},
			check: func(t *testing.T, merchant db.Merchant, err error) {
				require.NoError(t, err)
				require.Equal(t, merchantID, merchant.ID)
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

			merchant, err := ValidateMerchantForOrder(context.Background(), store, merchantID)
			tc.check(t, merchant, err)
		})
	}
}

func TestGetTakeoutSuspension(t *testing.T) {
	merchantID := int64(202)
	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, info *TakeoutSuspensionInfo, err error)
	}{
		{
			name: "ProfileNotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantProfile(gomock.Any(), merchantID).
					Times(1).
					Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, info *TakeoutSuspensionInfo, err error) {
				require.NoError(t, err)
				require.Nil(t, info)
			},
		},
		{
			name: "ProfileError",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantProfile(gomock.Any(), merchantID).
					Times(1).
					Return(db.GetMerchantProfileRow{}, errors.New("db timeout"))
			},
			check: func(t *testing.T, info *TakeoutSuspensionInfo, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "db timeout")
				require.Nil(t, info)
			},
		},
		{
			name: "NotSuspended",
			buildStubs: func(store *mockdb.MockStore) {
				profile := db.GetMerchantProfileRow{MerchantID: merchantID, IsTakeoutSuspended: false}
				store.EXPECT().
					GetMerchantProfile(gomock.Any(), merchantID).
					Times(1).
					Return(profile, nil)
			},
			check: func(t *testing.T, info *TakeoutSuspensionInfo, err error) {
				require.NoError(t, err)
				require.Nil(t, info)
			},
		},
		{
			name: "Suspended",
			buildStubs: func(store *mockdb.MockStore) {
				profile := db.GetMerchantProfileRow{
					MerchantID:           merchantID,
					IsTakeoutSuspended:   true,
					TakeoutSuspendReason: pgtype.Text{String: "policy", Valid: true},
					TakeoutSuspendUntil:  pgtype.Timestamptz{Time: now.Add(2 * time.Hour), Valid: true},
				}
				store.EXPECT().
					GetMerchantProfile(gomock.Any(), merchantID).
					Times(1).
					Return(profile, nil)
			},
			check: func(t *testing.T, info *TakeoutSuspensionInfo, err error) {
				require.NoError(t, err)
				require.NotNil(t, info)
				require.Equal(t, "policy", info.Reason)
				require.Equal(t, now.Add(2*time.Hour), info.Until)
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

			info, err := GetTakeoutSuspension(context.Background(), store, merchantID)
			tc.check(t, info, err)
		})
	}
}

func TestGetRiderSuspension(t *testing.T) {
	riderID := int64(303)
	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, info *RiderSuspensionInfo, err error)
	}{
		{
			name: "ProfileNotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderProfile(gomock.Any(), riderID).
					Times(1).
					Return(db.RiderProfile{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, info *RiderSuspensionInfo, err error) {
				require.NoError(t, err)
				require.Nil(t, info)
			},
		},
		{
			name: "ProfileError",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderProfile(gomock.Any(), riderID).
					Times(1).
					Return(db.RiderProfile{}, errors.New("db timeout"))
			},
			check: func(t *testing.T, info *RiderSuspensionInfo, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "db timeout")
				require.Nil(t, info)
			},
		},
		{
			name: "NotSuspended",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderProfile(gomock.Any(), riderID).
					Times(1).
					Return(db.RiderProfile{RiderID: riderID, IsSuspended: false}, nil)
			},
			check: func(t *testing.T, info *RiderSuspensionInfo, err error) {
				require.NoError(t, err)
				require.Nil(t, info)
			},
		},
		{
			name: "Suspended",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderProfile(gomock.Any(), riderID).
					Times(1).
					Return(db.RiderProfile{
						RiderID:       riderID,
						IsSuspended:   true,
						SuspendReason: pgtype.Text{String: "claim recovery overdue", Valid: true},
						SuspendUntil:  pgtype.Timestamptz{Time: now.Add(2 * time.Hour), Valid: true},
					}, nil)
			},
			check: func(t *testing.T, info *RiderSuspensionInfo, err error) {
				require.NoError(t, err)
				require.NotNil(t, info)
				require.Equal(t, "claim recovery overdue", info.Reason)
				require.Equal(t, now.Add(2*time.Hour), info.Until)
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

			info, err := GetRiderSuspension(context.Background(), store, riderID)
			tc.check(t, info, err)
		})
	}
}

func TestValidateTableOwnership(t *testing.T) {
	merchantID := int64(301)
	tableID := int64(401)

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, err error)
	}{
		{
			name: "TableNotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), tableID).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "table not found", reqErr.Err.Error())
			},
		},
		{
			name: "TableMismatch",
			buildStubs: func(store *mockdb.MockStore) {
				table := db.Table{ID: tableID, MerchantID: merchantID + 1}
				store.EXPECT().
					GetTable(gomock.Any(), tableID).
					Times(1).
					Return(table, nil)
			},
			check: func(t *testing.T, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "table does not belong to this merchant", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				table := db.Table{ID: tableID, MerchantID: merchantID}
				store.EXPECT().
					GetTable(gomock.Any(), tableID).
					Times(1).
					Return(table, nil)
			},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
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

			err := ValidateTableOwnership(context.Background(), store, merchantID, tableID)
			tc.check(t, err)
		})
	}
}

func TestCheckTakeoutBlocklist(t *testing.T) {
	userID := int64(501)

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, blocked bool, err error)
	}{
		{
			name: "NotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetActiveBehaviorBlocklist(gomock.Any(), db.GetActiveBehaviorBlocklistParams{
						EntityType: "user",
						EntityID:   userID,
					}).
					Times(1).
					Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, blocked bool, err error) {
				require.NoError(t, err)
				require.False(t, blocked)
			},
		},
		{
			name: "StoreError",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.BehaviorBlocklist{}, errors.New("boom"))
			},
			check: func(t *testing.T, blocked bool, err error) {
				require.Error(t, err)
				require.False(t, blocked)
			},
		},
		{
			name: "ActiveBlock",
			buildStubs: func(store *mockdb.MockStore) {
				block := db.BehaviorBlocklist{
					ID:         10,
					EntityType: "user",
					EntityID:   userID,
					BlockUntil: pgtype.Timestamptz{Time: time.Now().Add(2 * time.Hour), Valid: true},
				}
				store.EXPECT().
					GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).
					Times(1).
					Return(block, nil)
			},
			check: func(t *testing.T, blocked bool, err error) {
				require.NoError(t, err)
				require.True(t, blocked)
			},
		},
		{
			name: "ExpiredBlock",
			buildStubs: func(store *mockdb.MockStore) {
				block := db.BehaviorBlocklist{
					ID:         11,
					EntityType: "user",
					EntityID:   userID,
					BlockUntil: pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Minute), Valid: true},
				}
				store.EXPECT().
					GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).
					Times(1).
					Return(block, nil)
				store.EXPECT().
					UpdateBehaviorBlocklistStatus(gomock.Any(), db.UpdateBehaviorBlocklistStatusParams{
						ID:     block.ID,
						Status: "expired",
					}).
					Times(1).
					Return(nil)
			},
			check: func(t *testing.T, blocked bool, err error) {
				require.NoError(t, err)
				require.False(t, blocked)
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

			blocked, err := CheckTakeoutBlocklist(context.Background(), store, userID)
			tc.check(t, blocked, err)
		})
	}
}
