package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestResolvePackagingRequirement(t *testing.T) {
	const userID int64 = 7
	const merchantID int64 = 41
	const cartID int64 = 77
	const optionID int64 = 1001

	testCases := []struct {
		name       string
		input      ResolvePackagingInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, requirement PackagingRequirement, err error)
	}{
		{
			name: "NonApplicableOrderTypeReturnsNoRequirement",
			input: ResolvePackagingInput{
				MerchantID:        merchantID,
				OrderType:         db.OrderTypeDineIn,
				PackagingOptionID: packagingInt64Ptr(optionID),
			},
			buildStubs: func(store *mockdb.MockStore) {},
			check: func(t *testing.T, requirement PackagingRequirement, err error) {
				require.NoError(t, err)
				require.False(t, requirement.Enabled)
				require.False(t, requirement.Applicable)
				require.Nil(t, requirement.SelectedOption)
				require.Empty(t, requirement.Options)
			},
		},
		{
			name: "MissingSettingsMeansDisabled",
			input: ResolvePackagingInput{
				MerchantID: merchantID,
				OrderType:  db.OrderTypeTakeout,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, requirement PackagingRequirement, err error) {
				require.NoError(t, err)
				require.False(t, requirement.Enabled)
				require.False(t, requirement.Applicable)
				require.Nil(t, requirement.SelectedOption)
			},
		},
		{
			name: "DisabledSettingsIgnoreSelectedOption",
			input: ResolvePackagingInput{
				MerchantID:        merchantID,
				OrderType:         db.OrderTypeTakeout,
				PackagingOptionID: packagingInt64Ptr(optionID),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{
						MerchantID:           merchantID,
						Enabled:              false,
						Required:             true,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout},
					}, nil)
			},
			check: func(t *testing.T, requirement PackagingRequirement, err error) {
				require.NoError(t, err)
				require.False(t, requirement.Enabled)
				require.True(t, requirement.Required)
				require.True(t, requirement.Applicable)
				require.Nil(t, requirement.SelectedOption)
				require.Empty(t, requirement.Options)
			},
		},
		{
			name: "DisabledSettingsIgnoreCartSelection",
			input: ResolvePackagingInput{
				UserID:     userID,
				MerchantID: merchantID,
				OrderType:  db.OrderTypeTakeout,
				CartID:     packagingInt64Ptr(cartID),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{
						MerchantID:           merchantID,
						Enabled:              false,
						Required:             true,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout},
					}, nil)
			},
			check: func(t *testing.T, requirement PackagingRequirement, err error) {
				require.NoError(t, err)
				require.False(t, requirement.Enabled)
				require.True(t, requirement.Required)
				require.True(t, requirement.Applicable)
				require.Nil(t, requirement.SelectedOption)
				require.Empty(t, requirement.Options)
			},
		},
		{
			name: "EnabledRequiredMissingOptionFails",
			input: ResolvePackagingInput{
				MerchantID: merchantID,
				OrderType:  db.OrderTypeTakeout,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{
						MerchantID:           merchantID,
						Enabled:              true,
						Required:             true,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout},
					}, nil)
				store.EXPECT().
					ListEnabledMerchantPackagingOptions(gomock.Any(), merchantID).
					Times(1).
					Return([]db.MerchantPackagingOption{enabledPackagingOption(optionID, merchantID)}, nil)
			},
			check: func(t *testing.T, _ PackagingRequirement, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "请先选择包装方式", reqErr.Err.Error())
			},
		},
		{
			name: "EnabledOptionalMissingOptionReturnsOptions",
			input: ResolvePackagingInput{
				MerchantID: merchantID,
				OrderType:  db.OrderTypeTakeout,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{
						MerchantID:           merchantID,
						Enabled:              true,
						Required:             false,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout},
					}, nil)
				store.EXPECT().
					ListEnabledMerchantPackagingOptions(gomock.Any(), merchantID).
					Times(1).
					Return([]db.MerchantPackagingOption{enabledPackagingOption(optionID, merchantID)}, nil)
			},
			check: func(t *testing.T, requirement PackagingRequirement, err error) {
				require.NoError(t, err)
				require.True(t, requirement.Enabled)
				require.False(t, requirement.Required)
				require.True(t, requirement.Applicable)
				require.Len(t, requirement.Options, 1)
				require.Nil(t, requirement.SelectedOption)
			},
		},
		{
			name: "EmptyApplicableOrderTypesUsesDefaultTakeoutTakeaway",
			input: ResolvePackagingInput{
				MerchantID:        merchantID,
				OrderType:         db.OrderTypeTakeout,
				PackagingOptionID: packagingInt64Ptr(optionID),
			},
			buildStubs: func(store *mockdb.MockStore) {
				option := enabledPackagingOption(optionID, merchantID)
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{
						MerchantID:           merchantID,
						Enabled:              true,
						Required:             true,
						ApplicableOrderTypes: []string{},
					}, nil)
				store.EXPECT().
					ListEnabledMerchantPackagingOptions(gomock.Any(), merchantID).
					Times(1).
					Return([]db.MerchantPackagingOption{option}, nil)
				store.EXPECT().
					GetMerchantPackagingOption(gomock.Any(), db.GetMerchantPackagingOptionParams{
						ID:         optionID,
						MerchantID: merchantID,
					}).
					Times(1).
					Return(option, nil)
			},
			check: func(t *testing.T, requirement PackagingRequirement, err error) {
				require.NoError(t, err)
				require.True(t, requirement.Enabled)
				require.True(t, requirement.Applicable)
				require.NotNil(t, requirement.SelectedOption)
			},
		},
		{
			name: "SelectedOptionFromAnotherMerchantFails",
			input: ResolvePackagingInput{
				MerchantID:        merchantID,
				OrderType:         db.OrderTypeTakeout,
				PackagingOptionID: packagingInt64Ptr(optionID),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{
						MerchantID:           merchantID,
						Enabled:              true,
						Required:             true,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout},
					}, nil)
				store.EXPECT().
					ListEnabledMerchantPackagingOptions(gomock.Any(), merchantID).
					Times(1).
					Return([]db.MerchantPackagingOption{}, nil)
				store.EXPECT().
					GetMerchantPackagingOption(gomock.Any(), db.GetMerchantPackagingOptionParams{
						ID:         optionID,
						MerchantID: merchantID,
					}).
					Times(1).
					Return(db.MerchantPackagingOption{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ PackagingRequirement, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "包装方式不可用", reqErr.Err.Error())
			},
		},
		{
			name: "CartFromAnotherUserFails",
			input: ResolvePackagingInput{
				UserID:     userID,
				MerchantID: merchantID,
				OrderType:  db.OrderTypeTakeout,
				CartID:     packagingInt64Ptr(cartID),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{
						MerchantID:           merchantID,
						Enabled:              true,
						Required:             true,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout},
					}, nil)
				store.EXPECT().
					GetCart(gomock.Any(), cartID).
					Times(1).
					Return(db.Cart{
						ID:         cartID,
						UserID:     userID + 1,
						MerchantID: merchantID,
						OrderType:  db.OrderTypeTakeout,
					}, nil)
			},
			check: func(t *testing.T, _ PackagingRequirement, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "购物车不属于你", reqErr.Err.Error())
			},
		},
		{
			name: "CartMerchantMismatchFailsConflict",
			input: ResolvePackagingInput{
				UserID:     userID,
				MerchantID: merchantID,
				OrderType:  db.OrderTypeTakeout,
				CartID:     packagingInt64Ptr(cartID),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{
						MerchantID:           merchantID,
						Enabled:              true,
						Required:             true,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout},
					}, nil)
				store.EXPECT().
					GetCart(gomock.Any(), cartID).
					Times(1).
					Return(db.Cart{
						ID:         cartID,
						UserID:     userID,
						MerchantID: merchantID + 1,
						OrderType:  db.OrderTypeTakeout,
					}, nil)
			},
			check: func(t *testing.T, _ PackagingRequirement, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "购物车状态已变化，请刷新后重试", reqErr.Err.Error())
			},
		},
		{
			name: "DisabledSelectedOptionFails",
			input: ResolvePackagingInput{
				MerchantID:        merchantID,
				OrderType:         db.OrderTypeTakeout,
				PackagingOptionID: packagingInt64Ptr(optionID),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{
						MerchantID:           merchantID,
						Enabled:              true,
						Required:             true,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout},
					}, nil)
				store.EXPECT().
					ListEnabledMerchantPackagingOptions(gomock.Any(), merchantID).
					Times(1).
					Return([]db.MerchantPackagingOption{}, nil)
				store.EXPECT().
					GetMerchantPackagingOption(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantPackagingOption{
						ID:         optionID,
						MerchantID: merchantID,
						Name:       "普通餐盒",
						Price:      100,
						IsEnabled:  false,
					}, nil)
			},
			check: func(t *testing.T, _ PackagingRequirement, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "包装方式不可用", reqErr.Err.Error())
			},
		},
		{
			name: "DeletedSelectedOptionFails",
			input: ResolvePackagingInput{
				MerchantID:        merchantID,
				OrderType:         db.OrderTypeTakeout,
				PackagingOptionID: packagingInt64Ptr(optionID),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{
						MerchantID:           merchantID,
						Enabled:              true,
						Required:             true,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout},
					}, nil)
				store.EXPECT().
					ListEnabledMerchantPackagingOptions(gomock.Any(), merchantID).
					Times(1).
					Return([]db.MerchantPackagingOption{}, nil)
				store.EXPECT().
					GetMerchantPackagingOption(gomock.Any(), gomock.Any()).
					Times(1).
					Return(deletedPackagingOption(optionID, merchantID), nil)
			},
			check: func(t *testing.T, _ PackagingRequirement, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "包装方式不可用", reqErr.Err.Error())
			},
		},
		{
			name: "CartBackedSelectedOptionBuildsRequirement",
			input: ResolvePackagingInput{
				UserID:     userID,
				MerchantID: merchantID,
				OrderType:  db.OrderTypeTakeout,
				CartID:     packagingInt64Ptr(cartID),
			},
			buildStubs: func(store *mockdb.MockStore) {
				option := enabledPackagingOption(optionID, merchantID)
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{
						MerchantID:           merchantID,
						Enabled:              true,
						Required:             true,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout},
					}, nil)
				store.EXPECT().
					GetCart(gomock.Any(), cartID).
					Times(1).
					Return(db.Cart{
						ID:         cartID,
						UserID:     userID,
						MerchantID: merchantID,
						OrderType:  db.OrderTypeTakeout,
					}, nil)
				store.EXPECT().
					GetCartPackagingSelection(gomock.Any(), cartID).
					Times(1).
					Return(db.CartPackagingSelection{
						CartID:            cartID,
						PackagingOptionID: pgtype.Int8{Int64: optionID, Valid: true},
						SelectionVersion:  3,
					}, nil)
				store.EXPECT().
					ListEnabledMerchantPackagingOptions(gomock.Any(), merchantID).
					Times(1).
					Return([]db.MerchantPackagingOption{option}, nil)
				store.EXPECT().
					GetMerchantPackagingOption(gomock.Any(), db.GetMerchantPackagingOptionParams{
						ID:         optionID,
						MerchantID: merchantID,
					}).
					Times(1).
					Return(option, nil)
			},
			check: func(t *testing.T, requirement PackagingRequirement, err error) {
				require.NoError(t, err)
				require.True(t, requirement.Enabled)
				require.True(t, requirement.Required)
				require.True(t, requirement.Applicable)
				require.Len(t, requirement.Options, 1)
				require.NotNil(t, requirement.SelectedOption)
				require.Equal(t, optionID, requirement.SelectedOption.ID)
			},
		},
		{
			name: "ValidSelectedOptionBuildsRequirement",
			input: ResolvePackagingInput{
				MerchantID:        merchantID,
				OrderType:         db.OrderTypeTakeaway,
				PackagingOptionID: packagingInt64Ptr(optionID),
			},
			buildStubs: func(store *mockdb.MockStore) {
				option := enabledPackagingOption(optionID, merchantID)
				store.EXPECT().
					GetMerchantPackagingSettings(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingSetting{
						MerchantID:           merchantID,
						Enabled:              true,
						Required:             true,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout, db.OrderTypeTakeaway},
					}, nil)
				store.EXPECT().
					ListEnabledMerchantPackagingOptions(gomock.Any(), merchantID).
					Times(1).
					Return([]db.MerchantPackagingOption{option}, nil)
				store.EXPECT().
					GetMerchantPackagingOption(gomock.Any(), db.GetMerchantPackagingOptionParams{
						ID:         optionID,
						MerchantID: merchantID,
					}).
					Times(1).
					Return(option, nil)
			},
			check: func(t *testing.T, requirement PackagingRequirement, err error) {
				require.NoError(t, err)
				require.True(t, requirement.Enabled)
				require.True(t, requirement.Required)
				require.True(t, requirement.Applicable)
				require.Len(t, requirement.Options, 1)
				require.NotNil(t, requirement.SelectedOption)
				require.Equal(t, optionID, requirement.SelectedOption.ID)
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

			service := NewPackagingService(store)
			requirement, err := service.ResolvePackagingRequirement(context.Background(), tc.input)
			tc.check(t, requirement, err)
		})
	}
}

func TestBuildOrderPackagingSnapshot(t *testing.T) {
	optionID := int64(1001)
	snapshot := BuildOrderPackagingSnapshot(enabledPackagingOption(optionID, 41))

	require.NotNil(t, snapshot.PackagingOptionID)
	require.Equal(t, optionID, *snapshot.PackagingOptionID)
	require.Equal(t, "普通餐盒", snapshot.Name)
	require.Equal(t, int64(100), snapshot.UnitPrice)
	require.Equal(t, int16(1), snapshot.Quantity)
	require.Equal(t, int64(100), snapshot.Subtotal)
}

func TestResolveCartPackagingStateIncludesOptionsSelectedOptionAndVersion(t *testing.T) {
	const userID int64 = 7
	const merchantID int64 = 41
	const cartID int64 = 77
	const optionID int64 = 1001

	cart := db.Cart{
		ID:         cartID,
		UserID:     userID,
		MerchantID: merchantID,
		OrderType:  db.OrderTypeTakeout,
	}
	option := enabledPackagingOption(optionID, merchantID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantPackagingSettings(gomock.Any(), merchantID).
		Times(1).
		Return(db.MerchantPackagingSetting{
			MerchantID:           merchantID,
			Enabled:              true,
			Required:             true,
			ApplicableOrderTypes: []string{db.OrderTypeTakeout},
		}, nil)
	store.EXPECT().
		ListEnabledMerchantPackagingOptions(gomock.Any(), merchantID).
		Times(1).
		Return([]db.MerchantPackagingOption{option}, nil)
	store.EXPECT().
		GetCartPackagingSelection(gomock.Any(), cartID).
		Times(1).
		Return(db.CartPackagingSelection{
			CartID:            cartID,
			PackagingOptionID: pgtype.Int8{Int64: optionID, Valid: true},
			SelectionVersion:  3,
		}, nil)

	service := NewPackagingService(store)
	state, err := service.ResolveCartPackagingState(context.Background(), ResolveCartPackagingStateInput{
		UserID:     userID,
		MerchantID: merchantID,
		OrderType:  db.OrderTypeTakeout,
		Cart:       &cart,
	})

	require.NoError(t, err)
	require.True(t, state.Enabled)
	require.True(t, state.Required)
	require.True(t, state.Applicable)
	require.NotNil(t, state.SelectedOptionID)
	require.Equal(t, optionID, *state.SelectedOptionID)
	require.Equal(t, int64(3), state.SelectionVersion)
	require.Len(t, state.Options, 1)
	require.Equal(t, optionID, state.Options[0].ID)
}

func TestResolveCartPackagingStateMissingSelectionReportsVersionZero(t *testing.T) {
	const userID int64 = 7
	const merchantID int64 = 41
	const cartID int64 = 77
	const optionID int64 = 1001

	cart := db.Cart{
		ID:         cartID,
		UserID:     userID,
		MerchantID: merchantID,
		OrderType:  db.OrderTypeTakeout,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantPackagingSettings(gomock.Any(), merchantID).
		Times(1).
		Return(db.MerchantPackagingSetting{
			MerchantID:           merchantID,
			Enabled:              true,
			Required:             false,
			ApplicableOrderTypes: []string{db.OrderTypeTakeout},
		}, nil)
	store.EXPECT().
		ListEnabledMerchantPackagingOptions(gomock.Any(), merchantID).
		Times(1).
		Return([]db.MerchantPackagingOption{enabledPackagingOption(optionID, merchantID)}, nil)
	store.EXPECT().
		GetCartPackagingSelection(gomock.Any(), cartID).
		Times(1).
		Return(db.CartPackagingSelection{}, db.ErrRecordNotFound)

	service := NewPackagingService(store)
	state, err := service.ResolveCartPackagingState(context.Background(), ResolveCartPackagingStateInput{
		UserID:     userID,
		MerchantID: merchantID,
		OrderType:  db.OrderTypeTakeout,
		Cart:       &cart,
	})

	require.NoError(t, err)
	require.True(t, state.Enabled)
	require.False(t, state.Required)
	require.True(t, state.Applicable)
	require.Nil(t, state.SelectedOptionID)
	require.Equal(t, int64(0), state.SelectionVersion)
	require.Len(t, state.Options, 1)
}

func enabledPackagingOption(id, merchantID int64) db.MerchantPackagingOption {
	return db.MerchantPackagingOption{
		ID:          id,
		MerchantID:  merchantID,
		Name:        "普通餐盒",
		Description: pgtype.Text{String: "环保纸盒", Valid: true},
		Price:       100,
		IsEnabled:   true,
	}
}

func deletedPackagingOption(id, merchantID int64) db.MerchantPackagingOption {
	option := enabledPackagingOption(id, merchantID)
	option.DeletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return option
}

func packagingInt64Ptr(value int64) *int64 {
	return &value
}
