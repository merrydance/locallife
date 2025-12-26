package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

// createRandomClaim 创建一个随机的索赔记录
// 有效的 claim_type: 'foreign-object', 'damage', 'delay', 'quality', 'missing-item', 'other'
func createRandomClaim(t *testing.T, userID, orderID int64) Claim {
	arg := CreateClaimParams{
		OrderID:     orderID,
		UserID:      userID,
		ClaimType:   "quality",
		ClaimAmount: 5000,
		Description: "食品有问题",
		Status:      "pending",
	}

	claim, err := testStore.CreateClaim(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, claim.ID)

	// 更新状态为已批准以便测试申诉
	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(),
		"UPDATE claims SET status = 'approved', approved_amount = $1 WHERE id = $2",
		claim.ClaimAmount, claim.ID)
	require.NoError(t, err)

	return claim
}

// createRandomAppeal 创建一个随机的申诉记录
func createRandomAppeal(t *testing.T, claimID, appellantID int64, appellantType string, regionID int64) Appeal {
	arg := CreateAppealParams{
		ClaimID:       claimID,
		AppellantType: appellantType,
		AppellantID:   appellantID,
		Reason:        "我方不存在问题",
		EvidenceUrls:  []string{"https://example.com/evidence1.jpg"},
		RegionID:      regionID,
	}

	appeal, err := testStore.CreateAppeal(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, appeal.ID)

	return appeal
}

// ==================== CreateAppeal Tests ====================

func TestCreateAppeal(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	appeal := createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	require.Equal(t, claim.ID, appeal.ClaimID)
	require.Equal(t, "merchant", appeal.AppellantType)
	require.Equal(t, merchant.ID, appeal.AppellantID)
	require.Equal(t, "pending", appeal.Status)
	require.NotEmpty(t, appeal.Reason)
	require.Len(t, appeal.EvidenceUrls, 1)
}

func TestCreateAppeal_DuplicateClaimShouldFail(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)

	// 第一次创建
	createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 重复创建应该失败（唯一约束）
	_, err := testStore.CreateAppeal(context.Background(), CreateAppealParams{
		ClaimID:       claim.ID,
		AppellantType: "merchant",
		AppellantID:   merchant.ID,
		Reason:        "再次申诉",
		RegionID:      merchant.RegionID,
	})
	require.Error(t, err)
}

// ==================== GetAppeal Tests ====================

func TestGetAppeal(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	created := createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	got, err := testStore.GetAppeal(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, created.ClaimID, got.ClaimID)
	require.Equal(t, created.Reason, got.Reason)
}

func TestGetAppeal_NotFound(t *testing.T) {
	_, err := testStore.GetAppeal(context.Background(), 99999999)
	require.Error(t, err)
}

func TestGetAppealByClaim(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	created := createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	got, err := testStore.GetAppealByClaim(context.Background(), claim.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}

// ==================== CheckAppealExists Tests ====================

func TestCheckAppealExists(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)

	// 未申诉前
	exists, err := testStore.CheckAppealExists(context.Background(), claim.ID)
	require.NoError(t, err)
	require.False(t, exists)

	// 申诉后
	createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	exists, err = testStore.CheckAppealExists(context.Background(), claim.ID)
	require.NoError(t, err)
	require.True(t, exists)
}

// ==================== ListMerchantAppeals Tests ====================

func TestListMerchantAppealsForMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建5个申诉
	for i := 0; i < 5; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
	}

	appeals, err := testStore.ListMerchantAppealsForMerchant(context.Background(), ListMerchantAppealsForMerchantParams{
		AppellantID: merchant.ID,
		Limit:       10,
		Offset:      0,
	})
	require.NoError(t, err)
	require.Len(t, appeals, 5)
}

func TestCountMerchantAppealsForMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建3个申诉
	for i := 0; i < 3; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
	}

	count, err := testStore.CountMerchantAppealsForMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

// ==================== ReviewAppeal Tests ====================

func TestReviewAppeal_Approve(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	operator := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	appeal := createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 审核通过
	reviewed, err := testStore.ReviewAppeal(context.Background(), ReviewAppealParams{
		ID:                 appeal.ID,
		Status:             "approved",
		ReviewerID:         pgtype.Int8{Int64: operator.ID, Valid: true},
		ReviewNotes:        pgtype.Text{String: "证据充分，准予申诉", Valid: true},
		CompensationAmount: pgtype.Int8{Int64: 3000, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "approved", reviewed.Status)
	require.True(t, reviewed.ReviewerID.Valid)
	require.Equal(t, operator.ID, reviewed.ReviewerID.Int64)
	require.True(t, reviewed.CompensationAmount.Valid)
	require.Equal(t, int64(3000), reviewed.CompensationAmount.Int64)
	require.True(t, reviewed.ReviewedAt.Valid)
	require.True(t, reviewed.CompensatedAt.Valid)
}

func TestReviewAppeal_Reject(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	operator := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	appeal := createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 审核拒绝
	reviewed, err := testStore.ReviewAppeal(context.Background(), ReviewAppealParams{
		ID:          appeal.ID,
		Status:      "rejected",
		ReviewerID:  pgtype.Int8{Int64: operator.ID, Valid: true},
		ReviewNotes: pgtype.Text{String: "证据不足", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "rejected", reviewed.Status)
	require.False(t, reviewed.CompensationAmount.Valid) // 拒绝时没有赔偿
	require.False(t, reviewed.CompensatedAt.Valid)
}

func TestReviewAppeal_AlreadyReviewed(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	operator := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	appeal := createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 第一次审核
	_, err := testStore.ReviewAppeal(context.Background(), ReviewAppealParams{
		ID:         appeal.ID,
		Status:     "approved",
		ReviewerID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 再次审核应该失败（只有 pending 状态才能审核）
	_, err = testStore.ReviewAppeal(context.Background(), ReviewAppealParams{
		ID:         appeal.ID,
		Status:     "rejected",
		ReviewerID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.Error(t, err)
}

// ==================== Operator Appeal Tests ====================

func TestListOperatorAppeals(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建3个申诉
	for i := 0; i < 3; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
	}

	// 按区域查询
	appeals, err := testStore.ListOperatorAppeals(context.Background(), ListOperatorAppealsParams{
		RegionID: merchant.RegionID,
		Column2:  "", // 不筛选状态
		Limit:    10,
		Offset:   0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(appeals), 3)
}

func TestListOperatorAppeals_FilterByStatus(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	operator := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	appeal := createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 审核通过
	_, err := testStore.ReviewAppeal(context.Background(), ReviewAppealParams{
		ID:                 appeal.ID,
		Status:             "approved",
		ReviewerID:         pgtype.Int8{Int64: operator.ID, Valid: true},
		ReviewNotes:        pgtype.Text{String: "测试通过", Valid: true},
		CompensationAmount: pgtype.Int8{Int64: 1000, Valid: true},
	})
	require.NoError(t, err)

	// 筛选 pending 状态（应该不包含刚审核的申诉）
	appeals, err := testStore.ListOperatorAppeals(context.Background(), ListOperatorAppealsParams{
		RegionID: merchant.RegionID,
		Column2:  "pending",
		Limit:    100,
		Offset:   0,
	})
	require.NoError(t, err)
	for _, a := range appeals {
		require.NotEqual(t, appeal.ID, a.ID)
	}
}

func TestCountOperatorAppeals(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 记录初始数量
	initialCount, err := testStore.CountOperatorAppeals(context.Background(), CountOperatorAppealsParams{
		RegionID: merchant.RegionID,
		Column2:  "",
	})
	require.NoError(t, err)

	// 创建2个申诉
	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
	}

	count, err := testStore.CountOperatorAppeals(context.Background(), CountOperatorAppealsParams{
		RegionID: merchant.RegionID,
		Column2:  "",
	})
	require.NoError(t, err)
	require.Equal(t, initialCount+2, count)
}

// ==================== Rider Appeal Tests ====================

func TestListRiderAppeals(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	riderUser := createRandomUser(t)
	rider := createRandomRiderWithUser(t, riderUser.ID)

	// 创建骑手申诉
	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomAppeal(t, claim.ID, rider.ID, "rider", merchant.RegionID)
	}

	appeals, err := testStore.ListRiderAppeals(context.Background(), ListRiderAppealsParams{
		AppellantID: rider.ID,
		Limit:       10,
		Offset:      0,
	})
	require.NoError(t, err)
	require.Len(t, appeals, 2)

	for _, a := range appeals {
		require.Equal(t, "rider", a.AppellantType)
		require.Equal(t, rider.ID, a.AppellantID)
	}
}

func TestCountRiderAppeals(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	riderUser := createRandomUser(t)
	rider := createRandomRiderWithUser(t, riderUser.ID)

	// 创建3个骑手申诉
	for i := 0; i < 3; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomAppeal(t, claim.ID, rider.ID, "rider", merchant.RegionID)
	}

	count, err := testStore.CountRiderAppeals(context.Background(), rider.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

// ==================== GetClaimForAppeal Tests ====================

func TestGetClaimForAppeal(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)

	// 获取用于申诉的索赔信息
	claimInfo, err := testStore.GetClaimForAppeal(context.Background(), claim.ID)
	require.NoError(t, err)
	require.Equal(t, claim.ID, claimInfo.ID)
	require.Equal(t, merchant.ID, claimInfo.MerchantID)
	require.Equal(t, merchant.RegionID, claimInfo.RegionID)
}

func TestGetClaimForAppeal_NotApproved(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())

	// 创建未审核的索赔
	arg := CreateClaimParams{
		OrderID:     order.ID,
		UserID:      user.ID,
		ClaimType:   "quality",
		ClaimAmount: 5000,
		Description: "测试",
		Status:      "pending", // 未审核
	}
	claim, err := testStore.CreateClaim(context.Background(), arg)
	require.NoError(t, err)

	// 未审核的索赔应该查不到
	_, err = testStore.GetClaimForAppeal(context.Background(), claim.ID)
	require.Error(t, err) // 应该返回ErrNoRows
}

// ==================== GetMerchantAppealDetail Tests ====================

func TestGetMerchantAppealDetail(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	appeal := createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 获取商户申诉详情
	detail, err := testStore.GetMerchantAppealDetail(context.Background(), GetMerchantAppealDetailParams{
		ID:          appeal.ID,
		AppellantID: merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, appeal.ID, detail.ID)
	require.Equal(t, claim.ID, detail.ClaimID)
	require.Equal(t, order.OrderNo, detail.OrderNo)
}

func TestGetMerchantAppealDetail_NotOwned(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	appeal := createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 用其他商户ID查询，应该查不到
	_, err := testStore.GetMerchantAppealDetail(context.Background(), GetMerchantAppealDetailParams{
		ID:          appeal.ID,
		AppellantID: merchant.ID + 999, // 不存在的商户
	})
	require.Error(t, err)
}

// ==================== GetRiderAppealDetail Tests ====================

func TestGetRiderAppealDetail(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	riderUser := createRandomUser(t)
	rider := createRandomRiderWithUser(t, riderUser.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	appeal := createRandomAppeal(t, claim.ID, rider.ID, "rider", merchant.RegionID)

	// 获取骑手申诉详情
	detail, err := testStore.GetRiderAppealDetail(context.Background(), GetRiderAppealDetailParams{
		ID:          appeal.ID,
		AppellantID: rider.ID,
	})
	require.NoError(t, err)
	require.Equal(t, appeal.ID, detail.ID)
	require.Equal(t, "rider", detail.AppellantType)
}

// ==================== GetOperatorAppealDetail Tests ====================

func TestGetOperatorAppealDetail(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	appeal := createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 获取运营商视角的申诉详情
	detail, err := testStore.GetOperatorAppealDetail(context.Background(), GetOperatorAppealDetailParams{
		ID:       appeal.ID,
		RegionID: merchant.RegionID,
	})
	require.NoError(t, err)
	require.Equal(t, appeal.ID, detail.ID)
	require.Equal(t, merchant.ID, detail.MerchantID)
	require.NotEmpty(t, detail.MerchantName)
}

func TestGetOperatorAppealDetail_WrongRegion(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	appeal := createRandomAppeal(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 用错误的区域ID查询，应该查不到
	_, err := testStore.GetOperatorAppealDetail(context.Background(), GetOperatorAppealDetailParams{
		ID:       appeal.ID,
		RegionID: merchant.RegionID + 999,
	})
	require.Error(t, err)
}

// ==================== Claim Helper Functions ====================

// 查看 claim.sql 中的创建函数
// 有效的 claim_type: 'foreign-object', 'damage', 'delay', 'quality', 'missing-item', 'other'
func TestCreateClaim(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())

	arg := CreateClaimParams{
		OrderID:     order.ID,
		UserID:      user.ID,
		ClaimType:   "quality",
		ClaimAmount: 5000,
		Description: "食品安全问题",
		Status:      "pending",
	}

	claim, err := testStore.CreateClaim(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, claim.ID)
	require.Equal(t, order.ID, claim.OrderID)
	require.Equal(t, user.ID, claim.UserID)
	require.Equal(t, "quality", claim.ClaimType)
	require.Equal(t, int64(5000), claim.ClaimAmount)
	require.Equal(t, "pending", claim.Status)
}

func TestGetClaim(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())

	created, err := testStore.CreateClaim(context.Background(), CreateClaimParams{
		OrderID:     order.ID,
		UserID:      user.ID,
		ClaimType:   "damage",
		ClaimAmount: 3000,
		Status:      "pending",
	})
	require.NoError(t, err)

	got, err := testStore.GetClaim(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, created.ClaimType, got.ClaimType)
}
