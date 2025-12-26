package algorithm

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDetectDeviceReuse(t *testing.T) {
	testCases := []struct {
		name              string
		deviceFingerprint string
		buildStubs        func(store *mockdb.MockStore)
		checkResult       func(t *testing.T, result *FraudDetectionResult, err error)
	}{
		{
			name:              "EmptyDeviceFingerprint",
			deviceFingerprint: "",
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用任何方法
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.False(t, result.IsFraud)
				require.Equal(t, FraudPatternDeviceReuse, result.PatternType)
				require.Equal(t, 0, result.Confidence)
			},
		},
		{
			name:              "LessThan3Users",
			deviceFingerprint: "device-12345",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUsersByDeviceID(gomock.Any(), gomock.Eq("device-12345")).
					Times(1).
					Return([]int64{1, 2}, nil)
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.False(t, result.IsFraud)
			},
		},
		{
			name:              "3Users_LessThan3Claims",
			deviceFingerprint: "device-fraud",
			buildStubs: func(store *mockdb.MockStore) {
				userIDs := []int64{1, 2, 3}

				store.EXPECT().
					GetUsersByDeviceID(gomock.Any(), gomock.Eq("device-fraud")).
					Times(1).
					Return(userIDs, nil)

				store.EXPECT().
					CountRecentClaimsByUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(2), nil) // 只有2次索赔
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.False(t, result.IsFraud)
			},
		},
		{
			name:              "FraudDetected_3Users3Claims",
			deviceFingerprint: "device-fraud-confirmed",
			buildStubs: func(store *mockdb.MockStore) {
				userIDs := []int64{1, 2, 3}

				store.EXPECT().
					GetUsersByDeviceID(gomock.Any(), gomock.Eq("device-fraud-confirmed")).
					Times(1).
					Return(userIDs, nil)

				store.EXPECT().
					CountRecentClaimsByUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(3), nil)

				store.EXPECT().
					GetUsersWithRecentClaims(gomock.Any(), gomock.Any()).
					Times(1).
					Return(userIDs, nil)

				// Mock ListUserClaimsInPeriod for each user
				store.EXPECT().
					ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).
					Times(3).
					Return([]db.Claim{
						{ID: 1, OrderID: 100, CreatedAt: time.Now()},
					}, nil)

				// CreateFraudPattern will auto-confirm (matchCount=3 >= HighMatchCount=2)
				// So we need to mock HandleConfirmedFraud calls
				store.EXPECT().
					CreateFraudPattern(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.FraudPattern{
						ID:             1,
						IsConfirmed:    true,
						RelatedUserIds: userIDs,
					}, nil)

				// Mock HandleConfirmedFraud calls
				store.EXPECT().
					GetFraudPattern(gomock.Any(), int64(1)).
					Times(1).
					Return(db.FraudPattern{
						ID:             1,
						IsConfirmed:    true,
						RelatedUserIds: userIDs,
					}, nil)

				// Mock BlacklistUser: 3 times in HandleConfirmedFraud + 3 times in checkThresholds
				store.EXPECT().
					BlacklistUser(gomock.Any(), gomock.Any()).
					Times(6).
					Return(nil)

				// Mock UpdateTrustScore calls (GetUserProfileForUpdate + UpdateUserTrustScore + CreateTrustScoreChange for each user)
				// 新的 100 分制，扣 100 分后变成 0 分，会触发 blacklist
				store.EXPECT().
					GetUserProfileForUpdate(gomock.Any(), gomock.Any()).
					Times(3).
					Return(db.UserProfile{TrustScore: 100}, nil)

				store.EXPECT().
					UpdateUserTrustScore(gomock.Any(), gomock.Any()).
					Times(3).
					Return(nil)

				store.EXPECT().
					CreateTrustScoreChange(gomock.Any(), gomock.Any()).
					Times(3).
					Return(db.TrustScoreChange{}, nil)

				// Mock 欺诈处理相关调用
				store.EXPECT().
					GetClaimsByFraudPattern(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				store.EXPECT().
					SumClaimAmountsByMerchant(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				store.EXPECT().
					SumClaimAmountsByRider(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				// Mock ConfirmFraudPattern
				store.EXPECT().
					ConfirmFraudPattern(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.True(t, result.IsFraud)
				require.Equal(t, FraudPatternDeviceReuse, result.PatternType)
				require.Greater(t, result.Confidence, 0)
				require.Len(t, result.RelatedUserIDs, 3)
			},
		},
		{
			name:              "ErrorGettingUsers",
			deviceFingerprint: "device-error",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUsersByDeviceID(gomock.Any(), gomock.Eq("device-error")).
					Times(1).
					Return(nil, sql.ErrConnDone)
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.Error(t, err)
				require.Nil(t, result)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			detector := NewFraudDetector(store, nil)
			result, err := detector.DetectDeviceReuse(context.Background(), tc.deviceFingerprint)

			tc.checkResult(t, result, err)
		})
	}
}

func TestDetectAddressCluster(t *testing.T) {
	testCases := []struct {
		name        string
		addressID   int64
		buildStubs  func(store *mockdb.MockStore)
		checkResult func(t *testing.T, result *FraudDetectionResult, err error)
	}{
		{
			name:      "ZeroAddressID",
			addressID: 0,
			buildStubs: func(store *mockdb.MockStore) {
				// 不应该调用任何方法
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.False(t, result.IsFraud)
				require.Equal(t, FraudPatternAddressCluster, result.PatternType)
			},
		},
		{
			name:      "LessThan3Users",
			addressID: 123,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUsersByAddressID(gomock.Any(), gomock.Eq(int64(123))).
					Times(1).
					Return([]db.GetUsersByAddressIDRow{
						{UserID: 1, OrderCount: 5},
						{UserID: 2, OrderCount: 3},
					}, nil)
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.False(t, result.IsFraud)
			},
		},
		{
			name:      "FraudDetected_3Users3Claims",
			addressID: 456,
			buildStubs: func(store *mockdb.MockStore) {
				userRows := []db.GetUsersByAddressIDRow{
					{UserID: 1, OrderCount: 5},
					{UserID: 2, OrderCount: 3},
					{UserID: 3, OrderCount: 2},
				}
				userIDs := []int64{1, 2, 3}

				store.EXPECT().
					GetUsersByAddressID(gomock.Any(), gomock.Eq(int64(456))).
					Times(1).
					Return(userRows, nil)

				store.EXPECT().
					CountRecentClaimsByUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(6), nil)

				store.EXPECT().
					GetUsersWithRecentClaims(gomock.Any(), gomock.Any()).
					Times(1).
					Return(userIDs, nil)

				store.EXPECT().
					ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).
					Times(3).
					Return([]db.Claim{
						{ID: 1, OrderID: 100, CreatedAt: time.Now()},
						{ID: 2, OrderID: 101, CreatedAt: time.Now()},
					}, nil)

				// CreateFraudPattern will auto-confirm (6 claims >= 5)
				// So we need to mock HandleConfirmedFraud calls
				store.EXPECT().
					CreateFraudPattern(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.FraudPattern{
						ID:             2,
						IsConfirmed:    true,
						RelatedUserIds: userIDs,
					}, nil)

				// Mock HandleConfirmedFraud calls
				store.EXPECT().
					GetFraudPattern(gomock.Any(), int64(2)).
					Times(1).
					Return(db.FraudPattern{
						ID:             2,
						IsConfirmed:    true,
						RelatedUserIds: userIDs,
					}, nil)

				// Mock BlacklistUser: 3 times in HandleConfirmedFraud + 3 times in checkThresholds
				store.EXPECT().
					BlacklistUser(gomock.Any(), gomock.Any()).
					Times(6).
					Return(nil)

				// Mock UpdateTrustScore calls
				store.EXPECT().
					GetUserProfileForUpdate(gomock.Any(), gomock.Any()).
					Times(3).
					Return(db.UserProfile{TrustScore: 100}, nil)

				store.EXPECT().
					UpdateUserTrustScore(gomock.Any(), gomock.Any()).
					Times(3).
					Return(nil)

				store.EXPECT().
					CreateTrustScoreChange(gomock.Any(), gomock.Any()).
					Times(3).
					Return(db.TrustScoreChange{}, nil)

				// Mock 欺诈处理相关调用
				store.EXPECT().
					GetClaimsByFraudPattern(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				store.EXPECT().
					SumClaimAmountsByMerchant(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				store.EXPECT().
					SumClaimAmountsByRider(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				// Mock ConfirmFraudPattern
				store.EXPECT().
					ConfirmFraudPattern(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.True(t, result.IsFraud)
				require.Equal(t, FraudPatternAddressCluster, result.PatternType)
				require.Greater(t, result.Confidence, 0)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			detector := NewFraudDetector(store, nil)
			result, err := detector.DetectAddressCluster(context.Background(), tc.addressID)

			tc.checkResult(t, result, err)
		})
	}
}

func TestCheckUserForFraud(t *testing.T) {
	testCases := []struct {
		name              string
		userID            int64
		deviceFingerprint string
		addressID         int64
		buildStubs        func(store *mockdb.MockStore)
		checkResult       func(t *testing.T, result *FraudDetectionResult, err error)
	}{
		{
			name:              "NoFraud",
			userID:            1,
			deviceFingerprint: "device-clean",
			addressID:         100,
			buildStubs: func(store *mockdb.MockStore) {
				// Device check - no fraud
				store.EXPECT().
					GetUsersByDeviceID(gomock.Any(), gomock.Eq("device-clean")).
					Times(1).
					Return([]int64{1}, nil)

				// Address check - no fraud
				store.EXPECT().
					GetUsersByAddressID(gomock.Any(), gomock.Eq(int64(100))).
					Times(1).
					Return([]db.GetUsersByAddressIDRow{
						{UserID: 1, OrderCount: 5},
					}, nil)
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.False(t, result.IsFraud)
			},
		},
		{
			name:              "DeviceFraudDetected",
			userID:            1,
			deviceFingerprint: "device-fraud",
			addressID:         100,
			buildStubs: func(store *mockdb.MockStore) {
				userIDs := []int64{1, 2, 3}

				// Device check - fraud detected
				store.EXPECT().
					GetUsersByDeviceID(gomock.Any(), gomock.Eq("device-fraud")).
					Times(1).
					Return(userIDs, nil)

				store.EXPECT().
					CountRecentClaimsByUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(3), nil)

				store.EXPECT().
					GetUsersWithRecentClaims(gomock.Any(), gomock.Any()).
					Times(1).
					Return(userIDs, nil)

				store.EXPECT().
					ListUserClaimsInPeriod(gomock.Any(), gomock.Any()).
					Times(3).
					Return([]db.Claim{
						{ID: 1, OrderID: 100, CreatedAt: time.Now()},
					}, nil)

				// CreateFraudPattern will auto-confirm (matchCount=3 >= HighMatchCount=2)
				store.EXPECT().
					CreateFraudPattern(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.FraudPattern{
						ID:             1,
						IsConfirmed:    true,
						RelatedUserIds: userIDs,
					}, nil)

				// Mock HandleConfirmedFraud calls
				store.EXPECT().
					GetFraudPattern(gomock.Any(), int64(1)).
					Times(1).
					Return(db.FraudPattern{
						ID:             1,
						IsConfirmed:    true,
						RelatedUserIds: userIDs,
					}, nil)

				store.EXPECT().
					BlacklistUser(gomock.Any(), gomock.Any()).
					Times(6).
					Return(nil)

				store.EXPECT().
					GetUserProfileForUpdate(gomock.Any(), gomock.Any()).
					Times(3).
					Return(db.UserProfile{TrustScore: 100}, nil)

				store.EXPECT().
					UpdateUserTrustScore(gomock.Any(), gomock.Any()).
					Times(3).
					Return(nil)

				store.EXPECT().
					CreateTrustScoreChange(gomock.Any(), gomock.Any()).
					Times(3).
					Return(db.TrustScoreChange{}, nil)

				// Mock 欺诈处理相关调用
				store.EXPECT().
					GetClaimsByFraudPattern(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				store.EXPECT().
					SumClaimAmountsByMerchant(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				store.EXPECT().
					SumClaimAmountsByRider(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				store.EXPECT().
					ConfirmFraudPattern(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.True(t, result.IsFraud)
				require.Equal(t, FraudPatternDeviceReuse, result.PatternType)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			detector := NewFraudDetector(store, nil)
			result, err := detector.CheckUserForFraud(context.Background(), tc.userID, tc.deviceFingerprint, tc.addressID)

			tc.checkResult(t, result, err)
		})
	}
}

func TestCreateFraudPattern(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	detector := NewFraudDetector(store, nil)

	t.Run("AutoConfirm_HighMatchCount", func(t *testing.T) {
		store.EXPECT().
			CreateFraudPattern(gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx context.Context, arg db.CreateFraudPatternParams) (db.FraudPattern, error) {
				// 验证自动确认逻辑
				require.True(t, arg.IsConfirmed)
				require.Equal(t, int16(6), arg.MatchCount)
				return db.FraudPattern{ID: 1, IsConfirmed: true}, nil
			})

		store.EXPECT().
			GetFraudPattern(gomock.Any(), gomock.Eq(int64(1))).
			Times(1).
			Return(db.FraudPattern{
				ID:             1,
				IsConfirmed:    true,
				RelatedUserIds: []int64{1, 2, 3},
			}, nil)

		// Mock BlacklistUser: 3 in HandleConfirmedFraud + 3 in checkThresholds (score drops from 100 to 0)
		store.EXPECT().
			BlacklistUser(gomock.Any(), gomock.Any()).
			Times(6).
			Return(nil)

		// Mock GetUserProfileForUpdate and UpdateUserTrustScore for UpdateTrustScore calls
		store.EXPECT().
			GetUserProfileForUpdate(gomock.Any(), gomock.Any()).
			Times(3).
			Return(db.UserProfile{
				UserID:     1,
				Role:       EntityTypeCustomer,
				TrustScore: 100,
			}, nil)

		store.EXPECT().
			UpdateUserTrustScore(gomock.Any(), gomock.Any()).
			Times(3).
			Return(nil)

		store.EXPECT().
			CreateTrustScoreChange(gomock.Any(), gomock.Any()).
			Times(3).
			Return(db.TrustScoreChange{}, nil)

		// Mock 欺诈处理相关调用
		store.EXPECT().
			GetClaimsByFraudPattern(gomock.Any(), gomock.Any()).
			AnyTimes().
			Return(nil, nil)

		store.EXPECT().
			SumClaimAmountsByMerchant(gomock.Any(), gomock.Any()).
			AnyTimes().
			Return(nil, nil)

		store.EXPECT().
			SumClaimAmountsByRider(gomock.Any(), gomock.Any()).
			AnyTimes().
			Return(nil, nil)

		store.EXPECT().
			ConfirmFraudPattern(gomock.Any(), gomock.Any()).
			Times(1).
			Return(nil)

		pattern, err := detector.CreateFraudPattern(
			context.Background(),
			FraudPatternDeviceReuse,
			[]int64{1, 2, 3},
			[]int64{100, 101},
			[]int64{1, 2, 3},
			[]string{"device-123"},
			nil,
			6, // High match count
			"Test fraud pattern",
		)

		require.NoError(t, err)
		require.NotNil(t, pattern)
		require.True(t, pattern.IsConfirmed)
	})

	t.Run("NoAutoConfirm_LowMatchCount", func(t *testing.T) {
		store.EXPECT().
			CreateFraudPattern(gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx context.Context, arg db.CreateFraudPatternParams) (db.FraudPattern, error) {
				// 验证不会自动确认 (matchCount=1 < HighMatchCount=2 and claimCount=1 < 5)
				require.False(t, arg.IsConfirmed)
				require.Equal(t, int16(1), arg.MatchCount)
				return db.FraudPattern{ID: 2, IsConfirmed: false}, nil
			})

		pattern, err := detector.CreateFraudPattern(
			context.Background(),
			FraudPatternDeviceReuse,
			[]int64{1, 2, 3},
			[]int64{100},
			[]int64{1},
			[]string{"device-456"},
			nil,
			1, // Low match count (< HighMatchCount=2)
			"Test fraud pattern",
		)

		require.NoError(t, err)
		require.NotNil(t, pattern)
		require.False(t, pattern.IsConfirmed)
	})
}

func TestDetectCoordinatedClaims(t *testing.T) {
	testCases := []struct {
		name        string
		claimID     int64
		buildStubs  func(store *mockdb.MockStore)
		checkResult func(t *testing.T, result *FraudDetectionResult, err error)
	}{
		{
			name:    "NoFraud_LessThan2ClaimsInWindow",
			claimID: 1,
			buildStubs: func(store *mockdb.MockStore) {
				// Mock GetClaim
				store.EXPECT().
					GetClaim(gomock.Any(), int64(1)).
					Times(1).
					Return(db.Claim{
						ID:        1,
						OrderID:   100,
						UserID:    1,
						CreatedAt: time.Now(),
					}, nil)

				// Mock GetOrder for current claim
				store.EXPECT().
					GetOrder(gomock.Any(), int64(100)).
					Times(1).
					Return(db.Order{MerchantID: 10}, nil)

				// Mock ListClaimsByTimeWindow - 返回少于2条
				store.EXPECT().
					ListClaimsByTimeWindow(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListClaimsByTimeWindowRow{
						{ID: 2, OrderID: 101, UserID: 2, CreatedAt: time.Now()},
					}, nil)

				// Mock GetOrder for the claim in window (different merchant)
				store.EXPECT().
					GetOrder(gomock.Any(), int64(101)).
					Times(1).
					Return(db.Order{MerchantID: 99}, nil) // 不同商户
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.False(t, result.IsFraud)
				require.Equal(t, FraudPatternCoordinatedClaims, result.PatternType)
			},
		},
		{
			name:    "NoFraud_LessThan3DistinctUsers",
			claimID: 1,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaim(gomock.Any(), int64(1)).
					Times(1).
					Return(db.Claim{
						ID:        1,
						OrderID:   100,
						UserID:    1,
						CreatedAt: time.Now(),
					}, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), int64(100)).
					Times(1).
					Return(db.Order{MerchantID: 10}, nil)

				store.EXPECT().
					ListClaimsByTimeWindow(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListClaimsByTimeWindowRow{
						{ID: 2, OrderID: 101, UserID: 2, CreatedAt: time.Now()},
						{ID: 3, OrderID: 102, UserID: 2, CreatedAt: time.Now()}, // 同一用户
					}, nil)

				// Mock GetOrder for each claim
				store.EXPECT().
					GetOrder(gomock.Any(), int64(101)).
					Times(1).
					Return(db.Order{MerchantID: 10}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), int64(102)).
					Times(1).
					Return(db.Order{MerchantID: 10}, nil)
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.False(t, result.IsFraud)
				require.Equal(t, 2, result.Confidence) // 只有2个不同用户
			},
		},
		{
			name:    "MerchantSuspect_3IndependentUsers",
			claimID: 1,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaim(gomock.Any(), int64(1)).
					Times(1).
					Return(db.Claim{
						ID:        1,
						OrderID:   100,
						UserID:    1,
						CreatedAt: time.Now(),
					}, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), int64(100)).
					Times(1).
					Return(db.Order{MerchantID: 10}, nil)

				store.EXPECT().
					ListClaimsByTimeWindow(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListClaimsByTimeWindowRow{
						{ID: 2, OrderID: 101, UserID: 2, CreatedAt: time.Now()},
						{ID: 3, OrderID: 102, UserID: 3, CreatedAt: time.Now()},
						{ID: 4, OrderID: 103, UserID: 4, CreatedAt: time.Now()},
					}, nil)

				// Mock GetOrder for each claim - 同一商户
				store.EXPECT().
					GetOrder(gomock.Any(), int64(101)).
					Times(1).
					Return(db.Order{MerchantID: 10}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), int64(102)).
					Times(1).
					Return(db.Order{MerchantID: 10}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), int64(103)).
					Times(1).
					Return(db.Order{MerchantID: 10}, nil)

				// checkUserAssociation: 没有关联
				// 检查1: 没有已确认欺诈模式
				store.EXPECT().
					GetFraudPatternsByUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, nil)

				// 检查2: 没有共享设备 (4个用户，每个用户不同设备)
				store.EXPECT().
					GetDevicesByUserID(gomock.Any(), gomock.Any()).
					Times(4).
					DoAndReturn(func(ctx context.Context, uid int64) ([]db.UserDevice, error) {
						return []db.UserDevice{{DeviceID: "unique-device-" + string(rune(uid))}}, nil
					})

				// 检查3: 没有共享地址 (4个用户，每个用户不同地址)
				store.EXPECT().
					ListUserRecentOrders(gomock.Any(), gomock.Any()).
					Times(4).
					DoAndReturn(func(ctx context.Context, arg db.ListUserRecentOrdersParams) ([]db.ListUserRecentOrdersRow, error) {
						return []db.ListUserRecentOrdersRow{
							{AddressID: pgtype.Int8{Int64: arg.UserID * 100, Valid: true}}, // 每个用户不同地址
						}, nil
					})

				// 检查4: 不全是新用户首单 (有订单历史)
				store.EXPECT().
					GetUserProfile(gomock.Any(), gomock.Any()).
					Times(1). // 只要有一个用户TotalOrders > 1就返回
					Return(db.UserProfile{TotalOrders: 10}, nil)
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.False(t, result.IsFraud)              // 不是欺诈
				require.True(t, result.MerchantSuspect)        // 商户可疑
				require.Equal(t, int64(10), result.SuspectMerchantID)
				require.Contains(t, result.Description, "需调查商户而非用户")
			},
		},
		{
			name:    "FraudDetected_3UsersSharedDevice",
			claimID: 1,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetClaim(gomock.Any(), int64(1)).
					Times(1).
					Return(db.Claim{
						ID:        1,
						OrderID:   100,
						UserID:    1,
						CreatedAt: time.Now(),
					}, nil)

				store.EXPECT().
					GetOrder(gomock.Any(), int64(100)).
					Times(1).
					Return(db.Order{MerchantID: 10}, nil)

				store.EXPECT().
					ListClaimsByTimeWindow(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListClaimsByTimeWindowRow{
						{ID: 2, OrderID: 101, UserID: 2, CreatedAt: time.Now()},
						{ID: 3, OrderID: 102, UserID: 3, CreatedAt: time.Now()},
					}, nil)

				// 同一商户
				store.EXPECT().
					GetOrder(gomock.Any(), int64(101)).
					Times(1).
					Return(db.Order{MerchantID: 10}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), int64(102)).
					Times(1).
					Return(db.Order{MerchantID: 10}, nil)

				// checkUserAssociation: 有共享设备
				store.EXPECT().
					GetFraudPatternsByUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, nil)

				// 共享设备: 用户1和用户2共享设备 (一旦发现共享就返回)
				store.EXPECT().
					GetDevicesByUserID(gomock.Any(), gomock.Any()).
					Times(3). // 检查3个用户
					DoAndReturn(func(ctx context.Context, uid int64) ([]db.UserDevice, error) {
						if uid == 1 || uid == 2 {
							return []db.UserDevice{{DeviceID: "shared-device-123"}}, nil
						}
						return []db.UserDevice{{DeviceID: "device-3"}}, nil
					})

				// 获取订单地址 (收集相关数据用)
				store.EXPECT().
					GetOrder(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(db.Order{MerchantID: 10}, nil)

				// CreateFraudPattern (matchCount=3 >= HighMatchCount=2, 会自动确认)
				store.EXPECT().
					CreateFraudPattern(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.FraudPattern{ID: 1, IsConfirmed: true, RelatedUserIds: []int64{1, 2, 3}}, nil)

				// HandleConfirmedFraud: 需要 mock 相关调用
				store.EXPECT().
					GetFraudPattern(gomock.Any(), int64(1)).
					Times(1).
					Return(db.FraudPattern{ID: 1, IsConfirmed: true, RelatedUserIds: []int64{1, 2, 3}}, nil)

				// 拉黑用户
				store.EXPECT().
					BlacklistUser(gomock.Any(), gomock.Any()).
					Times(6). // 3 in HandleConfirmedFraud + 3 in checkThresholds
					Return(nil)

				// 更新信用分
				store.EXPECT().
					GetUserProfileForUpdate(gomock.Any(), gomock.Any()).
					Times(3).
					Return(db.UserProfile{TrustScore: 100}, nil)

				store.EXPECT().
					UpdateUserTrustScore(gomock.Any(), gomock.Any()).
					Times(3).
					Return(nil)

				store.EXPECT().
					CreateTrustScoreChange(gomock.Any(), gomock.Any()).
					Times(3).
					Return(db.TrustScoreChange{}, nil)

				// 欺诈处理相关
				store.EXPECT().
					GetClaimsByFraudPattern(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				store.EXPECT().
					SumClaimAmountsByMerchant(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				store.EXPECT().
					SumClaimAmountsByRider(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(nil, nil)

				store.EXPECT().
					ConfirmFraudPattern(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResult: func(t *testing.T, result *FraudDetectionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.True(t, result.IsFraud)
				require.False(t, result.MerchantSuspect)
				require.Equal(t, FraudPatternCoordinatedClaims, result.PatternType)
				require.Greater(t, result.Confidence, 0)
				require.Contains(t, result.Description, "关联用户")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			detector := NewFraudDetector(store, nil)
			result, err := detector.DetectCoordinatedClaims(context.Background(), tc.claimID)
			tc.checkResult(t, result, err)
		})
	}
}
