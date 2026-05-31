package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func createClaimRecoveryDecisionForTest(t *testing.T, claimID, orderID int64) BehaviorDecision {
	t.Helper()

	ctx := context.Background()
	claim, err := testStore.GetClaim(ctx, claimID)
	require.NoError(t, err)
	order, err := testStore.GetOrder(ctx, orderID)
	require.NoError(t, err)

	decision, err := testStore.CreateBehaviorDecision(ctx, CreateBehaviorDecisionParams{
		UserID:                 pgtype.Int8{Int64: claim.UserID, Valid: true},
		MerchantID:             pgtype.Int8{Int64: order.MerchantID, Valid: true},
		RiderID:                pgtype.Int8{},
		DecisionVersion:        "claim-recovery-test",
		ReasonCodes:            []string{"test_claim_recovery"},
		ResponsibleParty:       "merchant",
		CompensationSource:     "merchant",
		DecisionStatus:         "decided",
		TraceSummary:           pgtype.Text{String: "claim recovery test decision", Valid: true},
		OrderID:                pgtype.Int8{Int64: orderID, Valid: true},
		ReservationID:          pgtype.Int8{},
		ClaimID:                pgtype.Int8{Int64: claimID, Valid: true},
		DecisionMode:           pgtype.Text{String: BehaviorDecisionModeMerchantRecovery, Valid: true},
		ResponsibilityDomain:   pgtype.Text{String: BehaviorResponsibilityDomainMerchant, Valid: true},
		PayoutMode:             pgtype.Text{String: BehaviorPayoutModeInstantPaid, Valid: true},
		ConfidenceScore:        pgtype.Int4{},
		UserRiskScore:          pgtype.Int4{},
		MerchantLiabilityScore: pgtype.Int4{},
		RiderLiabilityScore:    pgtype.Int4{},
		FallbackReason:         pgtype.Text{},
		RestrictionReason:      pgtype.Text{},
		LiabilityShares:        []byte(`{}`),
		ScoreBreakdown:         []byte(`{}`),
		GraphHits:              []byte(`{}`),
		FactSnapshot:           []byte(`{}`),
		SupersedesDecisionID:   pgtype.Int8{},
		OverturnedByDecisionID: pgtype.Int8{},
	})
	require.NoError(t, err)

	return decision
}

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

	// 更新状态为已批准以便测试追偿争议
	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(),
		"UPDATE claims SET status = 'approved', approved_amount = $1 WHERE id = $2",
		claim.ClaimAmount, claim.ID)
	require.NoError(t, err)

	return claim
}

// createRandomRecoveryDispute 创建一个随机的追偿争议记录
func createRandomRecoveryDispute(t *testing.T, claimID, appellantID int64, appellantType string, regionID int64) RecoveryDispute {
	arg := CreateRecoveryDisputeParams{
		ClaimID:       claimID,
		AppellantType: appellantType,
		AppellantID:   appellantID,
		Reason:        "我方不存在问题",
		RegionID:      regionID,
	}

	recoveryDispute, err := testStore.CreateRecoveryDispute(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, recoveryDispute.ID)

	return recoveryDispute
}

func createRandomClaimRecovery(t *testing.T, claimID, orderID int64, status string) ClaimRecovery {
	decision := createClaimRecoveryDecisionForTest(t, claimID, orderID)
	return createClaimRecoveryWithTargetAndDecision(t, claimID, orderID, status, "merchant", decision.ID)
}

func createClaimRecoveryWithTarget(t *testing.T, claimID, orderID int64, status, recoveryTarget string) ClaimRecovery {
	decision := createClaimRecoveryDecisionForTest(t, claimID, orderID)
	return createClaimRecoveryWithTargetAndDecision(t, claimID, orderID, status, recoveryTarget, decision.ID)
}

func createClaimRecoveryWithTargetAndDecision(t *testing.T, claimID, orderID int64, status, recoveryTarget string, decisionID int64) ClaimRecovery {
	responsibleParty := recoveryTarget
	recoveryBasis := ClaimRecoveryBasisMerchantRecovery
	if recoveryTarget == "rider" {
		recoveryBasis = ClaimRecoveryBasisRiderRecovery
	}

	arg := CreateClaimRecoveryParams{
		ClaimID:          claimID,
		OrderID:          orderID,
		DecisionID:       pgtype.Int8{Int64: decisionID, Valid: true},
		ResponsibleParty: responsibleParty,
		RecoveryTarget:   pgtype.Text{String: recoveryTarget, Valid: true},
		RecoveryAmount:   3000,
		Status:           status,
		DueAt:            time.Now().Add(24 * time.Hour),
		DecisionSnapshot: []byte(`{"source":"test"}`),
		RecoveryBasis:    pgtype.Text{String: recoveryBasis, Valid: true},
	}

	recovery, err := testStore.CreateClaimRecovery(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, recovery.ID)

	return recovery
}

func requireClaimByID(t *testing.T, claims []ListMerchantClaimsForMerchantRow, claimID int64) ListMerchantClaimsForMerchantRow {
	t.Helper()
	for _, claim := range claims {
		if claim.ID == claimID {
			return claim
		}
	}
	require.Failf(t, "claim not found", "claim_id=%d len=%d", claimID, len(claims))
	return ListMerchantClaimsForMerchantRow{}
}

func requireRiderClaimByID(t *testing.T, claims []ListRiderClaimsForRiderRow, claimID int64) ListRiderClaimsForRiderRow {
	t.Helper()
	for _, claim := range claims {
		if claim.ID == claimID {
			return claim
		}
	}
	require.Failf(t, "claim not found", "claim_id=%d len=%d", claimID, len(claims))
	return ListRiderClaimsForRiderRow{}
}

// ==================== CreateRecoveryDispute Tests ====================

func TestCreateRecoveryDispute(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	require.Equal(t, claim.ID, recoveryDispute.ClaimID)
	require.Equal(t, "merchant", recoveryDispute.AppellantType)
	require.Equal(t, merchant.ID, recoveryDispute.AppellantID)
	require.Equal(t, "submitted", recoveryDispute.Status)
	require.NotEmpty(t, recoveryDispute.Reason)
}

func TestCreateRecoveryDispute_DuplicateClaimShouldFail(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)

	// 第一次创建
	createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 重复创建应该失败（唯一约束）
	_, err := testStore.CreateRecoveryDispute(context.Background(), CreateRecoveryDisputeParams{
		ClaimID:       claim.ID,
		AppellantType: "merchant",
		AppellantID:   merchant.ID,
		Reason:        "再次发起追偿争议",
		RegionID:      merchant.RegionID,
	})
	require.Error(t, err)
}

// ==================== GetRecoveryDispute Tests ====================

func TestGetRecoveryDispute(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	created := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	got, err := testStore.GetRecoveryDispute(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, created.ClaimID, got.ClaimID)
	require.Equal(t, created.Reason, got.Reason)
}

func TestGetRecoveryDispute_NotFound(t *testing.T) {
	_, err := testStore.GetRecoveryDispute(context.Background(), 99999999)
	require.Error(t, err)
}

func TestGetRecoveryDisputeByClaim(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	created := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	got, err := testStore.GetRecoveryDisputeByClaim(context.Background(), GetRecoveryDisputeByClaimParams{
		ClaimID:       claim.ID,
		AppellantType: "merchant",
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}

// ==================== CheckRecoveryDisputeExists Tests ====================

func TestCheckRecoveryDisputeExists(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)

	// 未发起追偿争议前
	exists, err := testStore.CheckRecoveryDisputeExists(context.Background(), CheckRecoveryDisputeExistsParams{
		ClaimID:       claim.ID,
		AppellantType: "merchant",
	})
	require.NoError(t, err)
	require.False(t, exists)

	// 发起追偿争议后
	createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	exists, err = testStore.CheckRecoveryDisputeExists(context.Background(), CheckRecoveryDisputeExistsParams{
		ClaimID:       claim.ID,
		AppellantType: "merchant",
	})
	require.NoError(t, err)
	require.True(t, exists)
}

func TestCreateRecoveryDisputeWithRecoveryTx_WritesDisputedRecoveryEvent(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recovery := createRandomClaimRecovery(t, claim.ID, order.ID, "pending")

	result, err := testStore.CreateRecoveryDisputeWithRecoveryTx(context.Background(), CreateRecoveryDisputeWithRecoveryTxParams{
		ClaimID:        claim.ID,
		RecoveryTarget: "merchant",
		AppellantType:  "merchant",
		AppellantID:    merchant.ID,
		Reason:         "商户发起追偿争议",
		RegionID:       merchant.RegionID,
	})
	require.NoError(t, err)
	require.NotZero(t, result.RecoveryDispute.ID)

	updatedRecovery, err := testStore.GetClaimRecoveryByClaimIDAndTarget(context.Background(), GetClaimRecoveryByClaimIDAndTargetParams{
		ClaimID:        claim.ID,
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "disputed", updatedRecovery.Status)

	recoveryEvents, err := testStore.ListClaimRecoveryEventsByRecovery(context.Background(), recovery.ID)
	require.NoError(t, err)
	require.Len(t, recoveryEvents, 1)
	require.Equal(t, ClaimRecoveryEventTypeDisputed, recoveryEvents[0].EventType)
}

func TestReviewRecoveryDisputeWithCompensationTx_ApprovedWaivesClaimRecovery(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
	createRandomClaimRecovery(t, claim.ID, order.ID, "disputed")

	result, err := testStore.ReviewRecoveryDisputeWithCompensationTx(context.Background(), ReviewRecoveryDisputeWithCompensationTxParams{
		ID:                 recoveryDispute.ID,
		Status:             "approved",
		ReviewerID:         pgtype.Int8{},
		ReviewNotes:        pgtype.Text{String: "系统自动复核通过", Valid: true},
		CompensationAmount: pgtype.Int8{},
	})
	require.NoError(t, err)
	require.Equal(t, "approved", result.RecoveryDispute.Status)
	require.NotNil(t, result.ReleaseAction)
	require.Equal(t, "release", result.ReleaseAction.ActionType)
	require.Equal(t, "merchant", result.ReleaseAction.TargetEntity)

	updatedRecovery, err := testStore.GetClaimRecoveryByClaimIDAndTarget(context.Background(), GetClaimRecoveryByClaimIDAndTargetParams{
		ClaimID:        claim.ID,
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "waived", updatedRecovery.Status)
	recoveryEvents, err := testStore.ListClaimRecoveryEventsByRecovery(context.Background(), updatedRecovery.ID)
	require.NoError(t, err)
	require.Len(t, recoveryEvents, 1)
	require.Equal(t, ClaimRecoveryEventTypeWaived, recoveryEvents[0].EventType)
}

// ==================== ListMerchantRecoveryDisputes Tests ====================

func TestListMerchantRecoveryDisputesForMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建5个追偿争议
	for i := 0; i < 5; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
	}

	recoveryDisputes, err := testStore.ListMerchantRecoveryDisputesForMerchant(context.Background(), ListMerchantRecoveryDisputesForMerchantParams{
		AppellantID: merchant.ID,
		Status:      pgtype.Text{},
		Limit:       10,
		Offset:      0,
	})
	require.NoError(t, err)
	require.Len(t, recoveryDisputes, 5)
}

func TestListMerchantRecoveryDisputesForMerchant_FilterByStatus(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
		_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
			"UPDATE recovery_disputes SET status = 'approved' WHERE id = $1",
			recoveryDispute.ID,
		)
		require.NoError(t, err)
	}

	pendingUser := createRandomUser(t)
	pendingOrder := createCompletedOrderForStats(t, pendingUser.ID, merchant.ID, 10000, "takeout", time.Now())
	pendingClaim := createRandomClaim(t, pendingUser.ID, pendingOrder.ID)
	createRandomRecoveryDispute(t, pendingClaim.ID, merchant.ID, "merchant", merchant.RegionID)

	recoveryDisputes, err := testStore.ListMerchantRecoveryDisputesForMerchant(context.Background(), ListMerchantRecoveryDisputesForMerchantParams{
		AppellantID: merchant.ID,
		Status:      pgtype.Text{String: "submitted", Valid: true},
		Limit:       10,
		Offset:      0,
	})
	require.NoError(t, err)
	require.Len(t, recoveryDisputes, 1)
	require.Equal(t, "submitted", recoveryDisputes[0].Status)
}

func TestListMerchantRecoveryDisputesForMerchant_UsesIDTieBreaker(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)
	var recoveryDisputeIDs []int64

	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
		recoveryDisputeIDs = append(recoveryDisputeIDs, recoveryDispute.ID)
	}

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		"UPDATE recovery_disputes SET created_at = $1 WHERE id = ANY($2)",
		tiedCreatedAt,
		recoveryDisputeIDs,
	)
	require.NoError(t, err)

	recoveryDisputes, err := testStore.ListMerchantRecoveryDisputesForMerchant(context.Background(), ListMerchantRecoveryDisputesForMerchantParams{
		AppellantID: merchant.ID,
		Status:      pgtype.Text{},
		Limit:       2,
		Offset:      0,
	})
	require.NoError(t, err)
	require.Len(t, recoveryDisputes, 2)
	require.Equal(t, recoveryDisputeIDs[1], recoveryDisputes[0].ID)
	require.Equal(t, recoveryDisputeIDs[0], recoveryDisputes[1].ID)
}

func TestCountMerchantRecoveryDisputesForMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建3个追偿争议
	for i := 0; i < 3; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
	}

	count, err := testStore.CountMerchantRecoveryDisputesForMerchant(context.Background(), CountMerchantRecoveryDisputesForMerchantParams{
		AppellantID: merchant.ID,
		Status:      pgtype.Text{},
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func TestCountMerchantRecoveryDisputesForMerchant_FilterByStatus(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
		_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
			"UPDATE recovery_disputes SET status = 'rejected' WHERE id = $1",
			recoveryDispute.ID,
		)
		require.NoError(t, err)
	}

	pendingUser := createRandomUser(t)
	pendingOrder := createCompletedOrderForStats(t, pendingUser.ID, merchant.ID, 10000, "takeout", time.Now())
	pendingClaim := createRandomClaim(t, pendingUser.ID, pendingOrder.ID)
	createRandomRecoveryDispute(t, pendingClaim.ID, merchant.ID, "merchant", merchant.RegionID)

	count, err := testStore.CountMerchantRecoveryDisputesForMerchant(context.Background(), CountMerchantRecoveryDisputesForMerchantParams{
		AppellantID: merchant.ID,
		Status:      pgtype.Text{String: "rejected", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}

func TestListMerchantClaimsForMerchant_FilterByBucket(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	pendingUser := createRandomUser(t)
	pendingOrder := createCompletedOrderForStats(t, pendingUser.ID, merchant.ID, 10000, "takeout", time.Now())
	pendingClaim := createRandomClaim(t, pendingUser.ID, pendingOrder.ID)
	createRandomClaimRecovery(t, pendingClaim.ID, pendingOrder.ID, "pending")

	disputedUser := createRandomUser(t)
	disputedOrder := createCompletedOrderForStats(t, disputedUser.ID, merchant.ID, 10000, "takeout", time.Now())
	disputedClaim := createRandomClaim(t, disputedUser.ID, disputedOrder.ID)
	createRandomRecoveryDispute(t, disputedClaim.ID, merchant.ID, "merchant", merchant.RegionID)
	createRandomClaimRecovery(t, disputedClaim.ID, disputedOrder.ID, "disputed")

	closedUser := createRandomUser(t)
	closedOrder := createCompletedOrderForStats(t, closedUser.ID, merchant.ID, 10000, "takeout", time.Now())
	closedClaim := createRandomClaim(t, closedUser.ID, closedOrder.ID)
	createRandomClaimRecovery(t, closedClaim.ID, closedOrder.ID, "waived")

	claims, err := testStore.ListMerchantClaimsForMerchant(context.Background(), ListMerchantClaimsForMerchantParams{
		MerchantID: merchant.ID,
		Bucket:     pgtype.Text{String: "disputed", Valid: true},
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.Equal(t, disputedClaim.ID, claims[0].ID)
	require.Equal(t, "disputed", claims[0].RecoveryStatus)
}

func TestListMerchantClaimsForMerchant_UsesMerchantRecoveryTarget(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	decision := createClaimRecoveryDecisionForTest(t, claim.ID, order.ID)

	merchantRecovery := createClaimRecoveryWithTargetAndDecision(t, claim.ID, order.ID, "pending", "merchant", decision.ID)
	_ = createClaimRecoveryWithTargetAndDecision(t, claim.ID, order.ID, "paid", "rider", decision.ID)

	claims, err := testStore.ListMerchantClaimsForMerchant(context.Background(), ListMerchantClaimsForMerchantParams{
		MerchantID: merchant.ID,
		Bucket:     pgtype.Text{},
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	got := requireClaimByID(t, claims, claim.ID)
	require.Equal(t, merchantRecovery.ID, got.RecoveryID)
	require.Equal(t, merchantRecovery.Status, got.RecoveryStatus)

	detail, err := testStore.GetMerchantClaimDetailForMerchant(context.Background(), GetMerchantClaimDetailForMerchantParams{
		ID:         claim.ID,
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, merchantRecovery.ID, detail.RecoveryID)
	require.Equal(t, merchantRecovery.Status, detail.RecoveryStatus)
}

func TestListRiderClaimsForRider_UsesRiderRecoveryTarget(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	riderUser := createRandomUser(t)
	rider := createRandomRiderWithUser(t, riderUser.ID)
	user := createRandomUser(t)
	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	delivery := createRandomDeliveryWithOrder(t, order.ID)
	_, err := testStore.AssignDelivery(context.Background(), AssignDeliveryParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)
	claim := createRandomClaim(t, user.ID, order.ID)
	decision := createClaimRecoveryDecisionForTest(t, claim.ID, order.ID)

	riderRecovery := createClaimRecoveryWithTargetAndDecision(t, claim.ID, order.ID, "pending", "rider", decision.ID)
	_ = createClaimRecoveryWithTargetAndDecision(t, claim.ID, order.ID, "paid", "merchant", decision.ID)

	claims, err := testStore.ListRiderClaimsForRider(context.Background(), ListRiderClaimsForRiderParams{
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		Bucket:  pgtype.Text{},
		Limit:   10,
		Offset:  0,
	})
	require.NoError(t, err)
	got := requireRiderClaimByID(t, claims, claim.ID)
	require.Equal(t, riderRecovery.ID, got.RecoveryID)
	require.Equal(t, riderRecovery.Status, got.RecoveryStatus)

	detail, err := testStore.GetRiderClaimDetailForRider(context.Background(), GetRiderClaimDetailForRiderParams{
		ID:      claim.ID,
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, riderRecovery.ID, detail.RecoveryID)
	require.Equal(t, riderRecovery.Status, detail.RecoveryStatus)
}

func TestListMerchantClaimsForMerchant_UsesIDTieBreaker(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)
	var claimIDs []int64

	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomClaimRecovery(t, claim.ID, order.ID, "pending")
		claimIDs = append(claimIDs, claim.ID)
	}

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		"UPDATE claims SET created_at = $1 WHERE id = ANY($2)",
		tiedCreatedAt,
		claimIDs,
	)
	require.NoError(t, err)

	claims, err := testStore.ListMerchantClaimsForMerchant(context.Background(), ListMerchantClaimsForMerchantParams{
		MerchantID: merchant.ID,
		Bucket:     pgtype.Text{},
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, claims, 2)
	require.Equal(t, claimIDs[1], claims[0].ID)
	require.Equal(t, claimIDs[0], claims[1].ID)
}

func TestCountMerchantClaimsForMerchant_FilterByBucket(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomClaimRecovery(t, claim.ID, order.ID, "paid")
	}

	pendingUser := createRandomUser(t)
	pendingOrder := createCompletedOrderForStats(t, pendingUser.ID, merchant.ID, 10000, "takeout", time.Now())
	pendingClaim := createRandomClaim(t, pendingUser.ID, pendingOrder.ID)
	createRandomClaimRecovery(t, pendingClaim.ID, pendingOrder.ID, "pending")

	count, err := testStore.CountMerchantClaimsForMerchant(context.Background(), CountMerchantClaimsForMerchantParams{
		MerchantID: merchant.ID,
		Bucket:     pgtype.Text{String: "closed", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}

// ==================== ReviewRecoveryDispute Tests ====================

func TestReviewRecoveryDispute_Approve(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	operator := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 审核通过
	reviewed, err := testStore.ReviewRecoveryDispute(context.Background(), ReviewRecoveryDisputeParams{
		ID:                 recoveryDispute.ID,
		Status:             "approved",
		ReviewerID:         pgtype.Int8{Int64: operator.ID, Valid: true},
		ReviewNotes:        pgtype.Text{String: "证据充分，准予追偿争议", Valid: true},
		CompensationAmount: pgtype.Int8{Int64: 3000, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "approved", reviewed.Status)
	require.True(t, reviewed.ReviewerID.Valid)
	require.Equal(t, operator.ID, reviewed.ReviewerID.Int64)
	require.True(t, reviewed.CompensationAmount.Valid)
	require.Equal(t, int64(3000), reviewed.CompensationAmount.Int64)
	require.True(t, reviewed.ReviewedAt.Valid)
	require.False(t, reviewed.CompensatedAt.Valid)
}

func TestReviewRecoveryDispute_Reject(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	operator := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 审核拒绝
	reviewed, err := testStore.ReviewRecoveryDispute(context.Background(), ReviewRecoveryDisputeParams{
		ID:          recoveryDispute.ID,
		Status:      "rejected",
		ReviewerID:  pgtype.Int8{Int64: operator.ID, Valid: true},
		ReviewNotes: pgtype.Text{String: "证据不足", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "rejected", reviewed.Status)
	require.False(t, reviewed.CompensationAmount.Valid) // 拒绝时没有赔偿
	require.False(t, reviewed.CompensatedAt.Valid)
}

func TestReviewRecoveryDispute_AlreadyReviewed(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	operator := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 第一次审核
	_, err := testStore.ReviewRecoveryDispute(context.Background(), ReviewRecoveryDisputeParams{
		ID:         recoveryDispute.ID,
		Status:     "approved",
		ReviewerID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 再次审核应该失败（只有 submitted 状态才能审核）
	_, err = testStore.ReviewRecoveryDispute(context.Background(), ReviewRecoveryDisputeParams{
		ID:         recoveryDispute.ID,
		Status:     "rejected",
		ReviewerID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.Error(t, err)
}

// ==================== Operator Recovery Dispute Tests ====================

func TestListOperatorRecoveryDisputes(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建3个追偿争议
	for i := 0; i < 3; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
	}

	// 按区域查询
	recoveryDisputes, err := testStore.ListOperatorRecoveryDisputes(context.Background(), ListOperatorRecoveryDisputesParams{
		RegionID: merchant.RegionID,
		Column2:  "", // 不筛选状态
		Limit:    10,
		Offset:   0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(recoveryDisputes), 3)
}

func TestListOperatorRecoveryDisputes_FilterByStatus(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	operator := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 审核通过
	_, err := testStore.ReviewRecoveryDispute(context.Background(), ReviewRecoveryDisputeParams{
		ID:                 recoveryDispute.ID,
		Status:             "approved",
		ReviewerID:         pgtype.Int8{Int64: operator.ID, Valid: true},
		ReviewNotes:        pgtype.Text{String: "测试通过", Valid: true},
		CompensationAmount: pgtype.Int8{Int64: 1000, Valid: true},
	})
	require.NoError(t, err)

	// 筛选 submitted 状态（应该不包含刚审核的追偿争议）
	recoveryDisputes, err := testStore.ListOperatorRecoveryDisputes(context.Background(), ListOperatorRecoveryDisputesParams{
		RegionID: merchant.RegionID,
		Column2:  "submitted",
		Limit:    100,
		Offset:   0,
	})
	require.NoError(t, err)
	for _, a := range recoveryDisputes {
		require.NotEqual(t, recoveryDispute.ID, a.ID)
	}
}

func TestListOperatorRecoveryDisputes_UsesIDTieBreaker(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)
	var recoveryDisputeIDs []int64

	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
		recoveryDisputeIDs = append(recoveryDisputeIDs, recoveryDispute.ID)
	}

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		"UPDATE recovery_disputes SET created_at = $1 WHERE id = ANY($2)",
		tiedCreatedAt,
		recoveryDisputeIDs,
	)
	require.NoError(t, err)

	recoveryDisputes, err := testStore.ListOperatorRecoveryDisputes(context.Background(), ListOperatorRecoveryDisputesParams{
		RegionID: merchant.RegionID,
		Column2:  "submitted",
		Limit:    2,
		Offset:   0,
	})
	require.NoError(t, err)
	require.Len(t, recoveryDisputes, 2)
	require.Equal(t, recoveryDisputeIDs[1], recoveryDisputes[0].ID)
	require.Equal(t, recoveryDisputeIDs[0], recoveryDisputes[1].ID)
}

func TestCountOperatorRecoveryDisputes(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 记录初始数量
	initialCount, err := testStore.CountOperatorRecoveryDisputes(context.Background(), CountOperatorRecoveryDisputesParams{
		RegionID: merchant.RegionID,
		Column2:  "",
	})
	require.NoError(t, err)

	// 创建2个追偿争议
	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)
	}

	count, err := testStore.CountOperatorRecoveryDisputes(context.Background(), CountOperatorRecoveryDisputesParams{
		RegionID: merchant.RegionID,
		Column2:  "",
	})
	require.NoError(t, err)
	require.Equal(t, initialCount+2, count)
}

// ==================== Rider Recovery Dispute Tests ====================

func TestListRiderRecoveryDisputes(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	riderUser := createRandomUser(t)
	rider := createRandomRiderWithUser(t, riderUser.ID)

	// 创建骑手追偿争议
	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomRecoveryDispute(t, claim.ID, rider.ID, "rider", merchant.RegionID)
	}

	recoveryDisputes, err := testStore.ListRiderRecoveryDisputes(context.Background(), ListRiderRecoveryDisputesParams{
		AppellantID: rider.ID,
		Limit:       10,
		Offset:      0,
	})
	require.NoError(t, err)
	require.Len(t, recoveryDisputes, 2)

	for _, a := range recoveryDisputes {
		require.Equal(t, "rider", a.AppellantType)
		require.Equal(t, rider.ID, a.AppellantID)
	}
}

func TestListRiderRecoveryDisputes_UsesIDTieBreaker(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	riderUser := createRandomUser(t)
	rider := createRandomRiderWithUser(t, riderUser.ID)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)
	var recoveryDisputeIDs []int64

	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		recoveryDispute := createRandomRecoveryDispute(t, claim.ID, rider.ID, "rider", merchant.RegionID)
		recoveryDisputeIDs = append(recoveryDisputeIDs, recoveryDispute.ID)
	}

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		"UPDATE recovery_disputes SET created_at = $1 WHERE id = ANY($2)",
		tiedCreatedAt,
		recoveryDisputeIDs,
	)
	require.NoError(t, err)

	recoveryDisputes, err := testStore.ListRiderRecoveryDisputes(context.Background(), ListRiderRecoveryDisputesParams{
		AppellantID: rider.ID,
		Limit:       2,
		Offset:      0,
	})
	require.NoError(t, err)
	require.Len(t, recoveryDisputes, 2)
	require.Equal(t, recoveryDisputeIDs[1], recoveryDisputes[0].ID)
	require.Equal(t, recoveryDisputeIDs[0], recoveryDisputes[1].ID)
}

func TestCountRiderRecoveryDisputes(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	riderUser := createRandomUser(t)
	rider := createRandomRiderWithUser(t, riderUser.ID)

	// 创建3个骑手追偿争议
	for i := 0; i < 3; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomRecoveryDispute(t, claim.ID, rider.ID, "rider", merchant.RegionID)
	}

	count, err := testStore.CountRiderRecoveryDisputes(context.Background(), CountRiderRecoveryDisputesParams{
		AppellantID: rider.ID,
		Status:      pgtype.Text{},
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func TestListRiderClaimsForRider_UsesIDTieBreaker(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	riderUser := createRandomUser(t)
	rider := createRandomRiderWithUser(t, riderUser.ID)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)
	var claimIDs []int64

	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		delivery := createRandomDeliveryWithOrder(t, order.ID)
		_, err := testStore.AssignDelivery(context.Background(), AssignDeliveryParams{
			ID:      delivery.ID,
			RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		})
		require.NoError(t, err)

		claim := createRandomClaim(t, user.ID, order.ID)
		createRandomClaimRecovery(t, claim.ID, order.ID, "pending")
		claimIDs = append(claimIDs, claim.ID)
	}

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		"UPDATE claims SET created_at = $1 WHERE id = ANY($2)",
		tiedCreatedAt,
		claimIDs,
	)
	require.NoError(t, err)

	claims, err := testStore.ListRiderClaimsForRider(context.Background(), ListRiderClaimsForRiderParams{
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		Limit:   2,
		Offset:  0,
		Bucket:  pgtype.Text{},
	})
	require.NoError(t, err)
	require.Len(t, claims, 2)
	require.Equal(t, claimIDs[1], claims[0].ID)
	require.Equal(t, claimIDs[0], claims[1].ID)
}

// ==================== GetMerchantRecoveryDisputeDetail Tests ====================

func TestGetMerchantRecoveryDisputeDetail(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 获取商户追偿争议详情
	detail, err := testStore.GetMerchantRecoveryDisputeDetail(context.Background(), GetMerchantRecoveryDisputeDetailParams{
		ID:          recoveryDispute.ID,
		AppellantID: merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, recoveryDispute.ID, detail.ID)
	require.Equal(t, claim.ID, detail.ClaimID)
	require.Equal(t, order.OrderNo, detail.OrderNo)
}

func TestGetMerchantRecoveryDisputeDetail_NotOwned(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 用其他商户ID查询，应该查不到
	_, err := testStore.GetMerchantRecoveryDisputeDetail(context.Background(), GetMerchantRecoveryDisputeDetailParams{
		ID:          recoveryDispute.ID,
		AppellantID: merchant.ID + 999, // 不存在的商户
	})
	require.Error(t, err)
}

// ==================== GetRiderRecoveryDisputeDetail Tests ====================

func TestGetRiderRecoveryDisputeDetail(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	riderUser := createRandomUser(t)
	rider := createRandomRiderWithUser(t, riderUser.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recoveryDispute := createRandomRecoveryDispute(t, claim.ID, rider.ID, "rider", merchant.RegionID)

	// 获取骑手追偿争议详情
	detail, err := testStore.GetRiderRecoveryDisputeDetail(context.Background(), GetRiderRecoveryDisputeDetailParams{
		ID:          recoveryDispute.ID,
		AppellantID: rider.ID,
	})
	require.NoError(t, err)
	require.Equal(t, recoveryDispute.ID, detail.ID)
	require.Equal(t, "rider", detail.AppellantType)
}

// ==================== GetOperatorRecoveryDisputeDetail Tests ====================

func TestGetOperatorRecoveryDisputeDetail(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 获取运营商视角的追偿争议详情
	detail, err := testStore.GetOperatorRecoveryDisputeDetail(context.Background(), GetOperatorRecoveryDisputeDetailParams{
		ID:       recoveryDispute.ID,
		RegionID: merchant.RegionID,
	})
	require.NoError(t, err)
	require.Equal(t, recoveryDispute.ID, detail.ID)
	require.Equal(t, merchant.ID, detail.MerchantID)
	require.NotEmpty(t, detail.MerchantName)
}

func TestGetOperatorRecoveryDisputeDetail_WrongRegion(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recoveryDispute := createRandomRecoveryDispute(t, claim.ID, merchant.ID, "merchant", merchant.RegionID)

	// 用错误的区域ID查询，应该查不到
	_, err := testStore.GetOperatorRecoveryDisputeDetail(context.Background(), GetOperatorRecoveryDisputeDetailParams{
		ID:       recoveryDispute.ID,
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
