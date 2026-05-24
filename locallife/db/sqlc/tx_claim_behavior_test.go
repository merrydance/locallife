package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createTakeoutOrderForClaimBehavior(t *testing.T, userID int64, merchantID int64, addressID *int64) Order {
	t.Helper()

	addressArg := pgtype.Int8{}
	if addressID != nil {
		addressArg = pgtype.Int8{Int64: *addressID, Valid: true}
	}

	order, err := testStore.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:             util.RandomString(20),
		UserID:              userID,
		MerchantID:          merchantID,
		OrderType:           "takeout",
		AddressID:           addressArg,
		DeliveryFee:         0,
		Subtotal:            8800,
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         8800,
		Status:              OrderStatusPending,
	})
	require.NoError(t, err)

	return order
}

func createAssignedDeliveryForClaimBehavior(t *testing.T, orderID int64, riderID int64) Delivery {
	t.Helper()

	delivery, err := testStore.CreateDelivery(context.Background(), CreateDeliveryParams{
		OrderID:           orderID,
		PickupAddress:     "北京市朝阳区商家取餐点",
		PickupLongitude:   numericFromFloat(116.404),
		PickupLatitude:    numericFromFloat(39.915),
		PickupContact:     pgtype.Text{String: "商家", Valid: true},
		PickupPhone:       pgtype.Text{String: "13800138000", Valid: true},
		DeliveryAddress:   "北京市朝阳区用户地址",
		DeliveryLongitude: numericFromFloat(116.410),
		DeliveryLatitude:  numericFromFloat(39.920),
		DeliveryContact:   pgtype.Text{String: "用户", Valid: true},
		DeliveryPhone:     pgtype.Text{String: "13900139000", Valid: true},
		Distance:          2600,
		DeliveryFee:       600,
	})
	require.NoError(t, err)

	assigned, err := testStore.AssignDelivery(context.Background(), AssignDeliveryParams{
		ID:      delivery.ID,
		RiderID: pgtype.Int8{Int64: riderID, Valid: true},
	})
	require.NoError(t, err)

	return assigned
}

func seedMerchantLiabilityHistoryForClaimBehavior(t *testing.T, merchantID int64, delta int64) {
	t.Helper()

	ctx := context.Background()
	user := createRandomUser(t)
	order := createTakeoutOrderForClaimBehavior(t, user.ID, merchantID, nil)
	decision, err := testStore.CreateBehaviorDecision(ctx, CreateBehaviorDecisionParams{
		UserID:                 pgtype.Int8{Int64: user.ID, Valid: true},
		MerchantID:             pgtype.Int8{Int64: merchantID, Valid: true},
		RiderID:                pgtype.Int8{},
		DecisionVersion:        "behavior-history-test",
		ReasonCodes:            []string{"seed_merchant_liability"},
		ResponsibleParty:       "merchant",
		CompensationSource:     "merchant",
		DecisionStatus:         "decided",
		TraceSummary:           pgtype.Text{String: "seed merchant liability history", Valid: true},
		OrderID:                pgtype.Int8{Int64: order.ID, Valid: true},
		ReservationID:          pgtype.Int8{},
		ClaimID:                pgtype.Int8{},
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

	_, err = testStore.CreateBehaviorDecisionEffect(ctx, CreateBehaviorDecisionEffectParams{
		DecisionID: decision.ID,
		EntityType: "merchant",
		EntityID:   merchantID,
		MetricKey:  "effective_liability_claims",
		DeltaValue: delta,
		Status:     BehaviorDecisionEffectStatusApplied,
		Note:       pgtype.Text{String: "seed merchant liability history", Valid: true},
	})
	require.NoError(t, err)
}

func seedUserMaliciousHistoryForClaimBehavior(t *testing.T, userID int64, delta int64) {
	t.Helper()

	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	order := createTakeoutOrderForClaimBehavior(t, userID, merchant.ID, nil)
	decision, err := testStore.CreateBehaviorDecision(ctx, CreateBehaviorDecisionParams{
		UserID:                 pgtype.Int8{Int64: userID, Valid: true},
		MerchantID:             pgtype.Int8{Int64: merchant.ID, Valid: true},
		RiderID:                pgtype.Int8{},
		DecisionVersion:        "behavior-history-test",
		ReasonCodes:            []string{"seed_user_restricted"},
		ResponsibleParty:       "user",
		CompensationSource:     "platform",
		DecisionStatus:         "decided",
		TraceSummary:           pgtype.Text{String: "seed user malicious history", Valid: true},
		OrderID:                pgtype.Int8{Int64: order.ID, Valid: true},
		ReservationID:          pgtype.Int8{},
		ClaimID:                pgtype.Int8{},
		DecisionMode:           pgtype.Text{String: BehaviorDecisionModeUserRestricted, Valid: true},
		ResponsibilityDomain:   pgtype.Text{String: BehaviorResponsibilityDomainUser, Valid: true},
		PayoutMode:             pgtype.Text{String: BehaviorPayoutModeLimitedPaid, Valid: true},
		ConfidenceScore:        pgtype.Int4{},
		UserRiskScore:          pgtype.Int4{},
		MerchantLiabilityScore: pgtype.Int4{},
		RiderLiabilityScore:    pgtype.Int4{},
		FallbackReason:         pgtype.Text{},
		RestrictionReason:      pgtype.Text{String: "confirmed_high_user_risk", Valid: true},
		LiabilityShares:        []byte(`{}`),
		ScoreBreakdown:         []byte(`{}`),
		GraphHits:              []byte(`{}`),
		FactSnapshot:           []byte(`{}`),
		SupersedesDecisionID:   pgtype.Int8{},
		OverturnedByDecisionID: pgtype.Int8{},
	})
	require.NoError(t, err)

	_, err = testStore.CreateBehaviorDecisionEffect(ctx, CreateBehaviorDecisionEffectParams{
		DecisionID: decision.ID,
		EntityType: "user",
		EntityID:   userID,
		MetricKey:  "malicious_confirmed_claims",
		DeltaValue: delta,
		Status:     BehaviorDecisionEffectStatusApplied,
		Note:       pgtype.Text{String: "seed user malicious history", Valid: true},
	})
	require.NoError(t, err)
}

func decodeFactSnapshot(t *testing.T, raw []byte) behaviorDecisionFactSnapshot {
	t.Helper()

	var snapshot behaviorDecisionFactSnapshot
	require.NoError(t, json.Unmarshal(raw, &snapshot))
	return snapshot
}

func decodeTraceMetricPayload(t *testing.T, raw []byte) behaviorTraceMetricPayload {
	t.Helper()

	var payload behaviorTraceMetricPayload
	require.NoError(t, json.Unmarshal(raw, &payload))
	return payload
}

func decodeAssociationPayload(t *testing.T, raw []byte) behaviorAssociationPayload {
	t.Helper()

	var payload behaviorAssociationPayload
	require.NoError(t, json.Unmarshal(raw, &payload))
	return payload
}

func decodeGraphHits(t *testing.T, raw []byte) behaviorDecisionGraphHits {
	t.Helper()

	var payload behaviorDecisionGraphHits
	require.NoError(t, json.Unmarshal(raw, &payload))
	return payload
}

func decodeScoreBreakdown(t *testing.T, raw []byte) behaviorDecisionScoreBreakdown {
	t.Helper()

	var payload behaviorDecisionScoreBreakdown
	require.NoError(t, json.Unmarshal(raw, &payload))
	return payload
}

func snapshotKey(snapshot BehaviorTraceSnapshot) string {
	return snapshot.ActorType.String + ":" + snapshot.WindowKey.String + ":" + snapshot.StatsScope.String
}

func effectKey(effect BehaviorDecisionEffect) string {
	return effect.EntityType + ":" + effect.MetricKey
}

func actionKey(action BehaviorAction) string {
	return action.ActionType + ":" + action.TargetEntity
}

func TestCreateClaimWithBehaviorTx_MerchantRecoveryWritesV2Artifacts(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	address := createRandomUserAddress(t, user)
	order := createTakeoutOrderForClaimBehavior(t, user.ID, merchant.ID, &address.ID)
	sharedFingerprint := "merchant-phase2-" + util.RandomString(8)
	otherUser := createRandomUser(t)
	_, err := testStore.UpsertUserDevice(ctx, UpsertUserDeviceParams{
		UserID:            otherUser.ID,
		DeviceID:          util.RandomString(12),
		DeviceFingerprint: pgtype.Text{String: sharedFingerprint, Valid: true},
		DeviceType:        "ios",
	})
	require.NoError(t, err)
	createTakeoutOrderForClaimBehavior(t, otherUser.ID, merchant.ID, &address.ID)
	seedMerchantLiabilityHistoryForClaimBehavior(t, merchant.ID, 2)

	approvedAmount := int64(4200)
	recoveryDueAt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	decisionSnapshot := []byte(`{"source":"merchant-recovery-test"}`)

	result, err := testStore.CreateClaimWithBehaviorTx(ctx, CreateClaimWithBehaviorTxParams{
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "foreign-object",
		Description:        "发现异物",
		ClaimAmount:        approvedAmount,
		Status:             "auto-approved",
		ApprovalType:       "instant",
		ApprovedAmount:     &approvedAmount,
		AutoApprovalReason: "phase1 merchant recovery test",
		DecisionVersion:    "behavior-v2-test",
		ReasonCodes:        []string{"merchant_foreign_object"},
		ResponsibleParty:   "merchant",
		CompensationSource: "merchant",
		TraceSummary:       "merchant recovery dual write",
		DeviceID:           util.RandomString(12),
		DeviceFingerprint:  sharedFingerprint,
		DeviceType:         "ios",
		AddressID:          &address.ID,
		CreateRecovery:     true,
		RecoveryTarget:     "merchant",
		RecoveryAmount:     approvedAmount,
		RecoveryDueAt:      &recoveryDueAt,
		DecisionSnapshot:   decisionSnapshot,
	})
	require.NoError(t, err)
	require.NotZero(t, result.Claim.ID)
	require.NotNil(t, result.PayoutAction)

	decision := result.BehaviorDecision
	persistedDecision, err := testStore.GetBehaviorDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.True(t, decision.ClaimID.Valid)
	require.Equal(t, result.Claim.ID, decision.ClaimID.Int64)
	require.Equal(t, BehaviorDecisionModeMerchantRecovery, decision.DecisionMode.String)
	require.Equal(t, BehaviorResponsibilityDomainMerchant, decision.ResponsibilityDomain.String)
	require.Equal(t, BehaviorPayoutModeInstantPaid, decision.PayoutMode.String)
	require.False(t, decision.ConfidenceScore.Valid)
	require.False(t, decision.UserRiskScore.Valid)
	require.False(t, decision.MerchantLiabilityScore.Valid)
	require.False(t, decision.RiderLiabilityScore.Valid)
	require.Equal(t, BehaviorEffectiveStatusEffective, persistedDecision.EffectiveStatus)
	require.True(t, persistedDecision.ProfileEffectApplied)
	require.JSONEq(t, `{}`, string(decision.LiabilityShares))

	scoreBreakdown := decodeScoreBreakdown(t, decision.ScoreBreakdown)
	require.Equal(t, "claims_rules_v1", scoreBreakdown.Version)

	graphHits := decodeGraphHits(t, decision.GraphHits)
	require.Equal(t, int32(2), graphHits.SharedDeviceUsers)
	require.Equal(t, int32(1), graphHits.SharedDeviceOtherUsers)
	require.Equal(t, int32(2), graphHits.SharedAddressUsers)
	require.Equal(t, int32(1), graphHits.SharedAddressOtherUsers)
	require.Contains(t, graphHits.HitCodes, "shared_device_fingerprint")
	require.Contains(t, graphHits.HitCodes, "shared_address_id")

	factSnapshot := decodeFactSnapshot(t, decision.FactSnapshot)
	require.Equal(t, order.ID, factSnapshot.OrderID)
	require.Equal(t, "foreign-object", factSnapshot.ClaimType)
	require.Equal(t, approvedAmount, factSnapshot.ClaimAmount)
	require.Equal(t, "merchant", factSnapshot.ResponsibleParty)
	require.Equal(t, "merchant", factSnapshot.CompensationSource)
	require.Equal(t, BehaviorDecisionModeMerchantRecovery, factSnapshot.DecisionMode)
	require.Equal(t, BehaviorResponsibilityDomainMerchant, factSnapshot.ResponsibilityDomain)
	require.Equal(t, BehaviorPayoutModeInstantPaid, factSnapshot.PayoutMode)
	require.Equal(t, "merchant", factSnapshot.RecoveryTarget)
	require.Equal(t, approvedAmount, factSnapshot.RecoveryAmount)
	require.Equal(t, approvedAmount, factSnapshot.ApprovedAmount)
	require.Equal(t, int32(1), factSnapshot.Associations.DistinctDevices)
	require.Equal(t, int32(1), factSnapshot.Associations.DistinctAddresses)
	require.Equal(t, int32(2), factSnapshot.Associations.SharedDeviceUsers)
	require.Equal(t, int32(2), factSnapshot.Associations.SharedAddressUsers)
	require.Equal(t, "takeout", factSnapshot.ResponsibilityFacts.OrderType)
	require.Empty(t, factSnapshot.ResponsibilityFacts.MissingCriticalFacts)

	actions, err := testStore.ListBehaviorActionsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.NotNil(t, result.PayoutAction)
	require.Nil(t, result.RecoveryAction)
	require.Nil(t, result.NotificationAction)
	actionByKey := make(map[string]BehaviorAction, len(actions))
	for _, action := range actions {
		actionByKey[actionKey(action)] = action
	}
	require.Equal(t, result.PayoutAction.ID, actionByKey["payout:user"].ID)
	require.NotContains(t, actionByKey, "recovery:merchant")
	require.NotContains(t, actionByKey, "notify:merchant")

	var payoutDetail behaviorPayoutActionDetail
	require.NoError(t, json.Unmarshal(actionByKey["payout:user"].Detail, &payoutDetail))
	require.Equal(t, "platform_payout", payoutDetail.Action)
	require.Equal(t, approvedAmount, payoutDetail.Amount)
	require.Equal(t, result.Claim.ID, payoutDetail.ClaimID)
	require.Equal(t, user.ID, payoutDetail.UserID)

	snapshots, err := testStore.ListBehaviorTraceSnapshotsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, snapshots, 8)

	snapshotByKey := make(map[string]BehaviorTraceSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		snapshotByKey[snapshotKey(snapshot)] = snapshot
		require.Equal(t, BehaviorSnapshotVersionV2, snapshot.SnapshotVersion.String)
	}

	user7Raw, ok := snapshotByKey["user:7d:raw"]
	require.True(t, ok)
	require.Equal(t, user.ID, user7Raw.ActorID.Int64)
	require.GreaterOrEqual(t, user7Raw.TotalCount, int32(1))
	require.GreaterOrEqual(t, user7Raw.AbnormalCount, int32(1))
	user7RawPayload := decodeTraceMetricPayload(t, user7Raw.MetricPayload)
	require.Equal(t, user7Raw.TotalCount, user7RawPayload.CompletedOrders)
	require.Equal(t, user7Raw.AbnormalCount, user7RawPayload.ClaimAttempts)
	user7Association := decodeAssociationPayload(t, user7Raw.AssociationPayload)
	require.Equal(t, int32(1), user7Association.DistinctDevices)
	require.Equal(t, int32(1), user7Association.DistinctAddresses)
	require.Equal(t, int32(1), user7Association.SharedDeviceOtherUsers)
	require.Equal(t, int32(1), user7Association.SharedAddressOtherUsers)
	require.Contains(t, user7Raw.AssociationHits, "shared_device_fingerprint")
	require.Contains(t, user7Raw.AssociationHits, "shared_address_id")

	user7Net, ok := snapshotByKey["user:7d:net_effective"]
	require.True(t, ok)
	user7NetPayload := decodeTraceMetricPayload(t, user7Net.MetricPayload)
	require.Equal(t, int32(1), user7NetPayload.ClaimAttempts)
	require.Equal(t, int32(1), user7NetPayload.EffectiveClaims)
	require.Equal(t, int32(1), user7NetPayload.MerchantRecoveredClaims)
	require.Equal(t, int32(1), user7NetPayload.DistinctDevices)
	require.Equal(t, int32(1), user7NetPayload.DistinctAddresses)

	user30Raw, ok := snapshotByKey["user:30d:raw"]
	require.True(t, ok)
	require.Equal(t, user.ID, user30Raw.ActorID.Int64)

	user30Net, ok := snapshotByKey["user:30d:net_effective"]
	require.True(t, ok)
	require.Equal(t, user.ID, user30Net.ActorID.Int64)

	merchant7Raw, ok := snapshotByKey["merchant:7d:raw"]
	require.True(t, ok)
	require.Equal(t, merchant.ID, merchant7Raw.ActorID.Int64)
	merchant7RawPayload := decodeTraceMetricPayload(t, merchant7Raw.MetricPayload)
	require.Equal(t, merchant7Raw.AbnormalCount, merchant7RawPayload.EffectiveLiabilityClaims)

	merchant7Net, ok := snapshotByKey["merchant:7d:net_effective"]
	require.True(t, ok)
	merchant7NetPayload := decodeTraceMetricPayload(t, merchant7Net.MetricPayload)
	require.Equal(t, int32(3), merchant7NetPayload.EffectiveLiabilityClaims)

	merchant30Raw, ok := snapshotByKey["merchant:30d:raw"]
	require.True(t, ok)
	require.Equal(t, merchant.ID, merchant30Raw.ActorID.Int64)

	merchant30Net, ok := snapshotByKey["merchant:30d:net_effective"]
	require.True(t, ok)
	require.Equal(t, merchant.ID, merchant30Net.ActorID.Int64)

	effects, err := testStore.ListBehaviorDecisionEffectsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, effects, 4)

	effectByKey := make(map[string]BehaviorDecisionEffect, len(effects))
	for _, effect := range effects {
		effectByKey[effectKey(effect)] = effect
		require.Equal(t, BehaviorDecisionEffectStatusApplied, effect.Status)
	}

	require.Equal(t, int64(1), effectByKey["user:claim_attempts"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["user:effective_claims"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["user:merchant_recovered_claims"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["merchant:effective_liability_claims"].DeltaValue)

	recovery, err := testStore.GetClaimRecoveryByClaimID(ctx, result.Claim.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)
	require.Zero(t, recovery.ID)
}

func TestCreateClaimWithBehaviorTx_MerchantRecoveryPersistsDeterministicResponsibility(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	address := createRandomUserAddress(t, user)
	order := createTakeoutOrderForClaimBehavior(t, user.ID, merchant.ID, &address.ID)

	approvedAmount := int64(4200)
	recoveryDueAt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)

	result, err := testStore.CreateClaimWithBehaviorTx(ctx, CreateClaimWithBehaviorTxParams{
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "foreign-object",
		Description:        "发现异物",
		ClaimAmount:        approvedAmount,
		Status:             "auto-approved",
		ApprovalType:       "instant",
		ApprovedAmount:     &approvedAmount,
		AutoApprovalReason: "phase2 merchant fallback test",
		DecisionVersion:    "behavior-v2-test",
		ReasonCodes:        []string{"merchant_foreign_object"},
		ResponsibleParty:   "merchant",
		CompensationSource: "merchant",
		TraceSummary:       "销售侧异常索赔默认由商户承担责任",
		DeviceID:           util.RandomString(12),
		DeviceFingerprint:  "merchant-fallback-" + util.RandomString(8),
		DeviceType:         "ios",
		AddressID:          &address.ID,
		CreateRecovery:     true,
		RecoveryTarget:     "merchant",
		RecoveryAmount:     approvedAmount,
		RecoveryDueAt:      &recoveryDueAt,
		DecisionSnapshot:   []byte(`{"source":"merchant-fallback-test"}`),
	})
	require.NoError(t, err)

	decision := result.BehaviorDecision
	require.Equal(t, BehaviorDecisionModeMerchantRecovery, decision.DecisionMode.String)
	require.Equal(t, BehaviorResponsibilityDomainMerchant, decision.ResponsibilityDomain.String)
	require.Equal(t, "merchant", decision.CompensationSource)
	require.Equal(t, "merchant", decision.ResponsibleParty)
	require.False(t, decision.FallbackReason.Valid)
	require.Equal(t, "instant", result.Claim.ApprovalType.String)
	require.Equal(t, "phase2 merchant fallback test", result.Claim.AutoApprovalReason.String)
	require.False(t, decision.MerchantLiabilityScore.Valid)

	scoreBreakdown := decodeScoreBreakdown(t, decision.ScoreBreakdown)
	require.Equal(t, "claims_rules_v1", scoreBreakdown.Version)

	factSnapshot := decodeFactSnapshot(t, decision.FactSnapshot)
	require.Equal(t, BehaviorDecisionModeMerchantRecovery, factSnapshot.DecisionMode)
	require.Equal(t, BehaviorResponsibilityDomainMerchant, factSnapshot.ResponsibilityDomain)
	require.Equal(t, "merchant", factSnapshot.RecoveryTarget)
	require.Equal(t, approvedAmount, factSnapshot.RecoveryAmount)

	effects, err := testStore.ListBehaviorDecisionEffectsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, effects, 4)

	effectByKey := make(map[string]BehaviorDecisionEffect, len(effects))
	for _, effect := range effects {
		effectByKey[effectKey(effect)] = effect
	}

	require.Equal(t, int64(1), effectByKey["user:claim_attempts"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["user:effective_claims"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["user:merchant_recovered_claims"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["merchant:effective_liability_claims"].DeltaValue)

	recovery, err := testStore.GetClaimRecoveryByClaimID(ctx, result.Claim.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)
	require.Zero(t, recovery.ID)

	require.Equal(t, "销售侧异常索赔默认由商户承担责任", decision.TraceSummary.String)
}

func TestCreateClaimWithBehaviorTx_RiderRecoveryPersistsWithoutPickupConfirmation(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	rider := createOnlineRider(t)
	address := createRandomUserAddress(t, user)
	order := createTakeoutOrderForClaimBehavior(t, user.ID, merchant.ID, &address.ID)
	deviceFingerprint := "rider-phase2-" + util.RandomString(8)
	createAssignedDeliveryForClaimBehavior(t, order.ID, rider.ID)

	approvedAmount := int64(3600)
	recoveryDueAt := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)

	result, err := testStore.CreateClaimWithBehaviorTx(ctx, CreateClaimWithBehaviorTxParams{
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "damage",
		Description:        "骑手代取损坏",
		ClaimAmount:        approvedAmount,
		Status:             "auto-approved",
		ApprovalType:       "instant",
		ApprovedAmount:     &approvedAmount,
		AutoApprovalReason: "phase1 rider recovery test",
		DecisionVersion:    "behavior-v2-test",
		ReasonCodes:        []string{"rider_damage_after_pickup"},
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		TraceSummary:       "rider recovery dual write",
		DeviceID:           util.RandomString(12),
		DeviceFingerprint:  deviceFingerprint,
		DeviceType:         "android",
		AddressID:          &address.ID,
		CreateRecovery:     true,
		RecoveryTarget:     "rider",
		RecoveryAmount:     approvedAmount,
		RecoveryDueAt:      &recoveryDueAt,
		DecisionSnapshot:   []byte(`{"source":"rider-recovery-test"}`),
	})
	require.NoError(t, err)

	decision := result.BehaviorDecision
	persistedDecision, err := testStore.GetBehaviorDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.True(t, decision.RiderID.Valid)
	require.Equal(t, rider.ID, decision.RiderID.Int64)
	require.Equal(t, BehaviorDecisionModeRiderRecovery, decision.DecisionMode.String)
	require.Equal(t, BehaviorResponsibilityDomainRider, decision.ResponsibilityDomain.String)
	require.Equal(t, BehaviorPayoutModeInstantPaid, decision.PayoutMode.String)
	require.Equal(t, "rider", decision.CompensationSource)
	require.Equal(t, "rider", decision.ResponsibleParty)
	require.False(t, decision.FallbackReason.Valid)
	require.False(t, decision.ConfidenceScore.Valid)
	require.False(t, decision.RiderLiabilityScore.Valid)
	require.True(t, persistedDecision.ProfileEffectApplied)
	require.Equal(t, "instant", result.Claim.ApprovalType.String)
	require.Equal(t, "phase1 rider recovery test", result.Claim.AutoApprovalReason.String)

	scoreBreakdown := decodeScoreBreakdown(t, decision.ScoreBreakdown)
	require.Equal(t, "claims_rules_v1", scoreBreakdown.Version)

	factSnapshot := decodeFactSnapshot(t, decision.FactSnapshot)
	require.Equal(t, "rider", factSnapshot.RecoveryTarget)
	require.Equal(t, approvedAmount, factSnapshot.RecoveryAmount)
	require.Equal(t, BehaviorDecisionModeRiderRecovery, factSnapshot.DecisionMode)
	require.Equal(t, BehaviorResponsibilityDomainRider, factSnapshot.ResponsibilityDomain)
	require.True(t, factSnapshot.ResponsibilityFacts.DeliveryExists)
	require.True(t, factSnapshot.ResponsibilityFacts.RiderAssigned)
	require.False(t, factSnapshot.ResponsibilityFacts.PickupConfirmed)
	require.Contains(t, factSnapshot.ResponsibilityFacts.MissingCriticalFacts, "missing_pickup_confirmation")

	snapshots, err := testStore.ListBehaviorTraceSnapshotsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, snapshots, 12)

	snapshotByKey := make(map[string]BehaviorTraceSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		snapshotByKey[snapshotKey(snapshot)] = snapshot
	}

	rider7Raw, ok := snapshotByKey["rider:7d:raw"]
	require.True(t, ok)
	require.Equal(t, rider.ID, rider7Raw.ActorID.Int64)
	require.Equal(t, BehaviorSnapshotVersionV2, rider7Raw.SnapshotVersion.String)
	rider7RawPayload := decodeTraceMetricPayload(t, rider7Raw.MetricPayload)
	require.Equal(t, rider7Raw.AbnormalCount, rider7RawPayload.EffectiveLiabilityClaims)

	rider7Net, ok := snapshotByKey["rider:7d:net_effective"]
	require.True(t, ok)
	rider7NetPayload := decodeTraceMetricPayload(t, rider7Net.MetricPayload)
	require.Equal(t, int32(1), rider7NetPayload.EffectiveLiabilityClaims)

	rider30Raw, ok := snapshotByKey["rider:30d:raw"]
	require.True(t, ok)
	require.Equal(t, rider.ID, rider30Raw.ActorID.Int64)

	rider30Net, ok := snapshotByKey["rider:30d:net_effective"]
	require.True(t, ok)
	require.Equal(t, rider.ID, rider30Net.ActorID.Int64)

	effects, err := testStore.ListBehaviorDecisionEffectsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, effects, 4)

	effectByKey := make(map[string]BehaviorDecisionEffect, len(effects))
	for _, effect := range effects {
		effectByKey[effectKey(effect)] = effect
	}

	require.Equal(t, int64(1), effectByKey["user:claim_attempts"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["user:effective_claims"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["user:rider_recovered_claims"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["rider:effective_liability_claims"].DeltaValue)

	recovery, err := testStore.GetClaimRecoveryByClaimID(ctx, result.Claim.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)
	require.Zero(t, recovery.ID)

	require.Equal(t, "rider recovery dual write", decision.TraceSummary.String)
}

func TestCreateClaimWithBehaviorTx_RiderRecoveryWritesRiderArtifacts(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	rider := createOnlineRider(t)
	address := createRandomUserAddress(t, user)
	order := createTakeoutOrderForClaimBehavior(t, user.ID, merchant.ID, &address.ID)
	deviceFingerprint := "rider-phase2-" + util.RandomString(8)
	createAssignedDeliveryForClaimBehavior(t, order.ID, rider.ID)
	_, err := testStore.UpdateOrderStatus(ctx, UpdateOrderStatusParams{
		Status:            OrderStatusPicked,
		FulfillmentStatus: pgtype.Text{},
		ID:                order.ID,
		ExpectedStatus:    OrderStatusPending,
	})
	require.NoError(t, err)

	approvedAmount := int64(3600)
	recoveryDueAt := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)

	result, err := testStore.CreateClaimWithBehaviorTx(ctx, CreateClaimWithBehaviorTxParams{
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "damage",
		Description:        "骑手代取损坏",
		ClaimAmount:        approvedAmount,
		Status:             "auto-approved",
		ApprovalType:       "instant",
		ApprovedAmount:     &approvedAmount,
		AutoApprovalReason: "phase1 rider recovery test",
		DecisionVersion:    "behavior-v2-test",
		ReasonCodes:        []string{"rider_damage_after_pickup"},
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		TraceSummary:       "rider recovery dual write",
		DeviceID:           util.RandomString(12),
		DeviceFingerprint:  deviceFingerprint,
		DeviceType:         "android",
		AddressID:          &address.ID,
		CreateRecovery:     true,
		RecoveryTarget:     "rider",
		RecoveryAmount:     approvedAmount,
		RecoveryDueAt:      &recoveryDueAt,
		DecisionSnapshot:   []byte(`{"source":"rider-recovery-test"}`),
	})
	require.NoError(t, err)

	decision := result.BehaviorDecision
	persistedDecision, err := testStore.GetBehaviorDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.True(t, decision.RiderID.Valid)
	require.Equal(t, rider.ID, decision.RiderID.Int64)
	require.Equal(t, BehaviorDecisionModeRiderRecovery, decision.DecisionMode.String)
	require.Equal(t, BehaviorResponsibilityDomainRider, decision.ResponsibilityDomain.String)
	require.Equal(t, BehaviorPayoutModeInstantPaid, decision.PayoutMode.String)
	require.False(t, decision.ConfidenceScore.Valid)
	require.False(t, decision.RiderLiabilityScore.Valid)
	require.True(t, persistedDecision.ProfileEffectApplied)

	scoreBreakdown := decodeScoreBreakdown(t, decision.ScoreBreakdown)
	require.Equal(t, "claims_rules_v1", scoreBreakdown.Version)

	factSnapshot := decodeFactSnapshot(t, decision.FactSnapshot)
	require.Equal(t, "rider", factSnapshot.RecoveryTarget)
	require.Equal(t, BehaviorDecisionModeRiderRecovery, factSnapshot.DecisionMode)
	require.Equal(t, BehaviorResponsibilityDomainRider, factSnapshot.ResponsibilityDomain)
	require.True(t, factSnapshot.ResponsibilityFacts.DeliveryExists)
	require.True(t, factSnapshot.ResponsibilityFacts.RiderAssigned)
	require.True(t, factSnapshot.ResponsibilityFacts.PickupConfirmed)
	require.Empty(t, factSnapshot.ResponsibilityFacts.MissingCriticalFacts)

	snapshots, err := testStore.ListBehaviorTraceSnapshotsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, snapshots, 12)

	snapshotByKey := make(map[string]BehaviorTraceSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		snapshotByKey[snapshotKey(snapshot)] = snapshot
	}

	rider7Raw, ok := snapshotByKey["rider:7d:raw"]
	require.True(t, ok)
	require.Equal(t, rider.ID, rider7Raw.ActorID.Int64)
	require.Equal(t, BehaviorSnapshotVersionV2, rider7Raw.SnapshotVersion.String)
	rider7RawPayload := decodeTraceMetricPayload(t, rider7Raw.MetricPayload)
	require.Equal(t, rider7Raw.AbnormalCount, rider7RawPayload.EffectiveLiabilityClaims)

	rider7Net, ok := snapshotByKey["rider:7d:net_effective"]
	require.True(t, ok)
	rider7NetPayload := decodeTraceMetricPayload(t, rider7Net.MetricPayload)
	require.Equal(t, int32(1), rider7NetPayload.EffectiveLiabilityClaims)

	user7Raw, ok := snapshotByKey["user:7d:raw"]
	require.True(t, ok)
	require.Equal(t, user.ID, user7Raw.ActorID.Int64)
	user7RawPayload := decodeTraceMetricPayload(t, user7Raw.MetricPayload)
	require.Equal(t, user7Raw.AbnormalCount, user7RawPayload.ClaimAttempts)

	merchant7Raw, ok := snapshotByKey["merchant:7d:raw"]
	require.True(t, ok)
	require.Equal(t, merchant.ID, merchant7Raw.ActorID.Int64)
	merchant7RawPayload := decodeTraceMetricPayload(t, merchant7Raw.MetricPayload)
	require.Equal(t, merchant7Raw.AbnormalCount, merchant7RawPayload.EffectiveLiabilityClaims)

	effects, err := testStore.ListBehaviorDecisionEffectsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, effects, 4)

	effectByKey := make(map[string]BehaviorDecisionEffect, len(effects))
	for _, effect := range effects {
		effectByKey[effectKey(effect)] = effect
	}

	require.Equal(t, int64(1), effectByKey["user:claim_attempts"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["user:effective_claims"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["user:rider_recovered_claims"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["rider:effective_liability_claims"].DeltaValue)

	recovery, err := testStore.GetClaimRecoveryByClaimID(ctx, result.Claim.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)
	require.Zero(t, recovery.ID)

	actions, err := testStore.ListBehaviorActionsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.NotNil(t, result.PayoutAction)
	require.Nil(t, result.RecoveryAction)
	require.Nil(t, result.NotificationAction)
	actionByKey := make(map[string]BehaviorAction, len(actions))
	for _, action := range actions {
		actionByKey[actionKey(action)] = action
	}
	require.Equal(t, result.PayoutAction.ID, actionByKey["payout:user"].ID)
	require.NotContains(t, actionByKey, "recovery:rider")
	require.NotContains(t, actionByKey, "notify:rider")

	recovery, err = testStore.GetClaimRecoveryByClaimID(ctx, result.Claim.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)
	require.Zero(t, recovery.ID)
}

func TestCreateClaimWithBehaviorTx_PromotesUserRestrictedFromHighRiskSignals(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	address := createRandomUserAddress(t, user)
	order := createTakeoutOrderForClaimBehavior(t, user.ID, merchant.ID, &address.ID)
	deviceFingerprint := "user-risk-promotion-" + util.RandomString(8)
	otherUser := createRandomUser(t)

	_, err := testStore.UpsertUserDevice(ctx, UpsertUserDeviceParams{
		UserID:            otherUser.ID,
		DeviceID:          util.RandomString(12),
		DeviceFingerprint: pgtype.Text{String: deviceFingerprint, Valid: true},
		DeviceType:        "ios",
	})
	require.NoError(t, err)
	createTakeoutOrderForClaimBehavior(t, otherUser.ID, merchant.ID, &address.ID)
	seedUserMaliciousHistoryForClaimBehavior(t, user.ID, 2)

	approvedAmount := int64(2600)
	recoveryDueAt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)

	result, err := testStore.CreateClaimWithBehaviorTx(ctx, CreateClaimWithBehaviorTxParams{
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "foreign-object",
		Description:        "异物索赔但用户风险已确认",
		ClaimAmount:        approvedAmount,
		Status:             "auto-approved",
		ApprovalType:       "instant",
		ApprovedAmount:     &approvedAmount,
		AutoApprovalReason: "merchant candidate before user risk override",
		DecisionVersion:    "behavior-v2-test",
		ReasonCodes:        []string{"merchant_recovery_candidate"},
		ResponsibleParty:   "merchant",
		CompensationSource: "merchant",
		TraceSummary:       "merchant recovery candidate",
		DeviceID:           util.RandomString(12),
		DeviceFingerprint:  deviceFingerprint,
		DeviceType:         "ios",
		AddressID:          &address.ID,
		CreateRecovery:     true,
		RecoveryTarget:     "merchant",
		RecoveryAmount:     approvedAmount,
		RecoveryDueAt:      &recoveryDueAt,
		DecisionSnapshot:   []byte(`{"source":"user-risk-promotion-test"}`),
	})
	require.NoError(t, err)

	decision := result.BehaviorDecision
	require.Equal(t, BehaviorDecisionModeUserRestricted, decision.DecisionMode.String)
	require.Equal(t, BehaviorResponsibilityDomainUser, decision.ResponsibilityDomain.String)
	require.Equal(t, "user", decision.ResponsibleParty)
	require.Equal(t, "platform", decision.CompensationSource)
	require.Equal(t, "confirmed_high_user_risk", decision.RestrictionReason.String)
	require.False(t, decision.FallbackReason.Valid)
	require.False(t, decision.UserRiskScore.Valid)
	require.False(t, decision.ConfidenceScore.Valid)
	require.Contains(t, decision.ReasonCodes, BehaviorDecisionModeUserRestricted)
	require.Contains(t, decision.ReasonCodes, "confirmed_high_user_risk")
	require.NotContains(t, decision.ReasonCodes, BehaviorDecisionModeMerchantRecovery)
	require.NotNil(t, result.RestrictionAction)

	scoreBreakdown := decodeScoreBreakdown(t, decision.ScoreBreakdown)
	require.Equal(t, "claims_rules_v1", scoreBreakdown.Version)

	factSnapshot := decodeFactSnapshot(t, decision.FactSnapshot)
	require.Equal(t, BehaviorDecisionModeUserRestricted, factSnapshot.DecisionMode)
	require.Equal(t, BehaviorResponsibilityDomainUser, factSnapshot.ResponsibilityDomain)
	require.Equal(t, "user", factSnapshot.ResponsibleParty)
	require.Empty(t, factSnapshot.RecoveryTarget)
	require.Zero(t, factSnapshot.RecoveryAmount)

	effects, err := testStore.ListBehaviorDecisionEffectsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, effects, 3)

	effectByKey := make(map[string]BehaviorDecisionEffect, len(effects))
	for _, effect := range effects {
		effectByKey[effectKey(effect)] = effect
	}

	require.Equal(t, int64(1), effectByKey["user:malicious_confirmed_claims"].DeltaValue)

	recovery, err := testStore.GetClaimRecoveryByClaimID(ctx, result.Claim.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)
	require.Zero(t, recovery.ID)

	actions, err := testStore.ListBehaviorActionsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, actions, 3)
	require.NotNil(t, result.NotificationAction)
	actionByKey := make(map[string]BehaviorAction, len(actions))
	for _, action := range actions {
		actionByKey[actionKey(action)] = action
	}
	var restrictionActionDetail behaviorRestrictionActionDetail
	require.NoError(t, json.Unmarshal(actionByKey["block:user"].Detail, &restrictionActionDetail))
	require.Equal(t, "apply_user_restriction", restrictionActionDetail.Action)
	require.Equal(t, result.Claim.ID, restrictionActionDetail.ClaimID)
	require.Equal(t, user.ID, restrictionActionDetail.UserID)
	require.Equal(t, BehaviorDecisionModeUserRestricted, restrictionActionDetail.DecisionMode)
	require.Equal(t, "confirmed_high_user_risk", restrictionActionDetail.RestrictionReason)

	var restrictedNotifyDetail behaviorNotifyActionDetail
	require.NoError(t, json.Unmarshal(actionByKey["notify:user"].Detail, &restrictedNotifyDetail))
	require.Equal(t, "notify_user_restriction", restrictedNotifyDetail.Action)
	require.Equal(t, result.Claim.ID, restrictedNotifyDetail.ClaimID)
	require.Equal(t, user.ID, restrictedNotifyDetail.TargetID)
	require.Equal(t, user.ID, restrictedNotifyDetail.RecipientUserID)
	require.Equal(t, "账户状态变更通知", restrictedNotifyDetail.Title)
}

func TestCreateClaimWithBehaviorTx_DoesNotPromoteUserRestrictedWithoutStrongConfirmation(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	address := createRandomUserAddress(t, user)
	order := createTakeoutOrderForClaimBehavior(t, user.ID, merchant.ID, &address.ID)
	deviceFingerprint := "user-risk-guard-" + util.RandomString(8)
	otherUser := createRandomUser(t)

	_, err := testStore.UpsertUserDevice(ctx, UpsertUserDeviceParams{
		UserID:            otherUser.ID,
		DeviceID:          util.RandomString(12),
		DeviceFingerprint: pgtype.Text{String: deviceFingerprint, Valid: true},
		DeviceType:        "ios",
	})
	require.NoError(t, err)
	createTakeoutOrderForClaimBehavior(t, otherUser.ID, merchant.ID, &address.ID)
	seedMerchantLiabilityHistoryForClaimBehavior(t, merchant.ID, 2)

	approvedAmount := int64(2600)
	recoveryDueAt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)

	result, err := testStore.CreateClaimWithBehaviorTx(ctx, CreateClaimWithBehaviorTxParams{
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "foreign-object",
		Description:        "异物索赔且商户责任稳定",
		ClaimAmount:        approvedAmount,
		Status:             "auto-approved",
		ApprovalType:       "instant",
		ApprovedAmount:     &approvedAmount,
		AutoApprovalReason: "merchant candidate should remain merchant recovery",
		DecisionVersion:    "behavior-v2-test",
		ReasonCodes:        []string{"merchant_recovery_candidate"},
		ResponsibleParty:   "merchant",
		CompensationSource: "merchant",
		TraceSummary:       "merchant recovery candidate",
		DeviceID:           util.RandomString(12),
		DeviceFingerprint:  deviceFingerprint,
		DeviceType:         "ios",
		AddressID:          &address.ID,
		CreateRecovery:     true,
		RecoveryTarget:     "merchant",
		RecoveryAmount:     approvedAmount,
		RecoveryDueAt:      &recoveryDueAt,
		DecisionSnapshot:   []byte(`{"source":"user-risk-guard-test"}`),
	})
	require.NoError(t, err)

	decision := result.BehaviorDecision
	require.Equal(t, BehaviorDecisionModeMerchantRecovery, decision.DecisionMode.String)
	require.Equal(t, BehaviorResponsibilityDomainMerchant, decision.ResponsibilityDomain.String)
	require.Equal(t, "merchant", decision.ResponsibleParty)
	require.False(t, decision.RestrictionReason.Valid)
	require.False(t, decision.UserRiskScore.Valid)
	require.Contains(t, decision.ReasonCodes, BehaviorDecisionModeMerchantRecovery)
	require.NotContains(t, decision.ReasonCodes, BehaviorDecisionModeUserRestricted)
	require.Nil(t, result.RecoveryAction)

	recovery, err := testStore.GetClaimRecoveryByClaimID(ctx, result.Claim.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)
	require.Zero(t, recovery.ID)
}

func TestCreateClaimWithBehaviorTx_UserRestrictedPersistsFormalDecision(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	address := createRandomUserAddress(t, user)
	order := createTakeoutOrderForClaimBehavior(t, user.ID, merchant.ID, &address.ID)

	approvedAmount := int64(2400)

	result, err := testStore.CreateClaimWithBehaviorTx(ctx, CreateClaimWithBehaviorTxParams{
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "damage",
		Description:        "用户索赔行为异常但本次仍赔付",
		ClaimAmount:        approvedAmount,
		Status:             "auto-approved",
		ApprovalType:       "auto",
		ApprovedAmount:     &approvedAmount,
		AutoApprovalReason: "用户限制服务，平台先行赔付",
		DecisionVersion:    "behavior-v2-test",
		ReasonCodes:        []string{"user_restricted"},
		ResponsibleParty:   "user",
		CompensationSource: "platform",
		TraceSummary:       "您的账号因索赔行为异常已被限制服务；若确认继续索赔，平台将先行赔付并停止后续服务。",
		DeviceID:           util.RandomString(12),
		DeviceFingerprint:  "user-restricted-" + util.RandomString(8),
		DeviceType:         "ios",
		AddressID:          &address.ID,
		CreateRecovery:     false,
	})
	require.NoError(t, err)

	persistedClaim, err := testStore.GetClaim(ctx, result.Claim.ID)
	require.NoError(t, err)
	require.Equal(t, "auto", persistedClaim.ApprovalType.String)

	decision := result.BehaviorDecision
	require.Equal(t, BehaviorDecisionModeUserRestricted, decision.DecisionMode.String)
	require.Equal(t, BehaviorResponsibilityDomainUser, decision.ResponsibilityDomain.String)
	require.Equal(t, BehaviorPayoutModeLimitedPaid, decision.PayoutMode.String)
	require.Equal(t, "confirmed_high_user_risk", decision.RestrictionReason.String)
	require.False(t, decision.UserRiskScore.Valid)
	require.False(t, decision.ConfidenceScore.Valid)
	require.Equal(t, "platform", decision.CompensationSource)
	require.Equal(t, "user", decision.ResponsibleParty)
	require.False(t, decision.FallbackReason.Valid)

	scoreBreakdown := decodeScoreBreakdown(t, decision.ScoreBreakdown)
	require.Equal(t, "claims_rules_v1", scoreBreakdown.Version)
	require.NotNil(t, result.RestrictionAction)

	factSnapshot := decodeFactSnapshot(t, decision.FactSnapshot)
	require.Equal(t, BehaviorDecisionModeUserRestricted, factSnapshot.DecisionMode)
	require.Equal(t, BehaviorResponsibilityDomainUser, factSnapshot.ResponsibilityDomain)
	require.Equal(t, "user", factSnapshot.ResponsibleParty)
	require.Equal(t, "platform", factSnapshot.CompensationSource)

	effects, err := testStore.ListBehaviorDecisionEffectsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, effects, 3)

	effectByKey := make(map[string]BehaviorDecisionEffect, len(effects))
	for _, effect := range effects {
		effectByKey[effectKey(effect)] = effect
	}

	require.Equal(t, int64(1), effectByKey["user:claim_attempts"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["user:effective_claims"].DeltaValue)
	require.Equal(t, int64(1), effectByKey["user:malicious_confirmed_claims"].DeltaValue)

	actions, err := testStore.ListBehaviorActionsByDecision(ctx, decision.ID)
	require.NoError(t, err)
	require.Len(t, actions, 3)
	require.NotNil(t, result.NotificationAction)
	actionByKey := make(map[string]BehaviorAction, len(actions))
	for _, action := range actions {
		actionByKey[actionKey(action)] = action
	}
	var restrictionActionDetail behaviorRestrictionActionDetail
	require.NoError(t, json.Unmarshal(actionByKey["block:user"].Detail, &restrictionActionDetail))
	require.Equal(t, "apply_user_restriction", restrictionActionDetail.Action)
	require.Equal(t, result.Claim.ID, restrictionActionDetail.ClaimID)
	require.Equal(t, user.ID, restrictionActionDetail.UserID)
	require.Equal(t, "confirmed_high_user_risk", restrictionActionDetail.RestrictionReason)

	var persistedRestrictionNotify behaviorNotifyActionDetail
	require.NoError(t, json.Unmarshal(actionByKey["notify:user"].Detail, &persistedRestrictionNotify))
	require.Equal(t, "notify_user_restriction", persistedRestrictionNotify.Action)
	require.Equal(t, user.ID, persistedRestrictionNotify.RecipientUserID)

	snapshots, err := testStore.ListBehaviorTraceSnapshotsByDecision(ctx, decision.ID)
	require.NoError(t, err)

	snapshotByKey := make(map[string]BehaviorTraceSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		snapshotByKey[snapshotKey(snapshot)] = snapshot
	}

	user7Net, ok := snapshotByKey["user:7d:net_effective"]
	require.True(t, ok)
	user7NetPayload := decodeTraceMetricPayload(t, user7Net.MetricPayload)
	require.Equal(t, int32(1), user7NetPayload.ClaimAttempts)
	require.Equal(t, int32(1), user7NetPayload.EffectiveClaims)
	require.Equal(t, int32(1), user7NetPayload.MaliciousConfirmedClaims)

	recovery, err := testStore.GetClaimRecoveryByClaimID(ctx, result.Claim.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)
	require.Zero(t, recovery.ID)
}

func TestCreateClaimCompensationTx_DefersMerchantRecoveryArtifactsUntilPayoutComplete(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	address := createRandomUserAddress(t, user)
	order := createTakeoutOrderForClaimBehavior(t, user.ID, merchant.ID, &address.ID)
	sharedFingerprint := "deferred-merchant-" + util.RandomString(8)
	otherUser := createRandomUser(t)
	_, err := testStore.UpsertUserDevice(ctx, UpsertUserDeviceParams{
		UserID:            otherUser.ID,
		DeviceID:          util.RandomString(12),
		DeviceFingerprint: pgtype.Text{String: sharedFingerprint, Valid: true},
		DeviceType:        "ios",
	})
	require.NoError(t, err)
	createTakeoutOrderForClaimBehavior(t, otherUser.ID, merchant.ID, &address.ID)
	seedMerchantLiabilityHistoryForClaimBehavior(t, merchant.ID, 2)

	approvedAmount := int64(2600)
	recoveryDueAt := time.Now().Add(36 * time.Hour).UTC().Truncate(time.Second)
	submitResult, err := testStore.CreateClaimWithBehaviorTx(ctx, CreateClaimWithBehaviorTxParams{
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "foreign-object",
		Description:        "先判责后赔付",
		ClaimAmount:        approvedAmount,
		Status:             ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:       "auto",
		ApprovedAmount:     &approvedAmount,
		AutoApprovalReason: "商户责任，等待进入赔付阶段",
		DecisionVersion:    "behavior-v2-test",
		ReasonCodes:        []string{"merchant_recovery"},
		ResponsibleParty:   "merchant",
		CompensationSource: "merchant",
		TraceSummary:       "商户责任成立，待进入赔付阶段",
		DeviceID:           util.RandomString(12),
		DeviceFingerprint:  sharedFingerprint,
		DeviceType:         "ios",
		AddressID:          &address.ID,
		CreateRecovery:     true,
		RecoveryTarget:     "merchant",
		RecoveryAmount:     approvedAmount,
		RecoveryDueAt:      &recoveryDueAt,
		DecisionSnapshot:   []byte(`{"phase":"submit"}`),
		SkipActionCreation: true,
	})
	require.NoError(t, err)
	require.Nil(t, submitResult.PayoutAction)
	require.Nil(t, submitResult.RecoveryAction)
	require.Nil(t, submitResult.NotificationAction)

	_, err = testStore.GetClaimRecoveryByClaimID(ctx, submitResult.Claim.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)

	compensationResult, err := testStore.CreateClaimCompensationTx(ctx, CreateClaimCompensationTxParams{ClaimID: submitResult.Claim.ID})
	require.NoError(t, err)
	require.NotNil(t, compensationResult.PayoutAction)
	require.Nil(t, compensationResult.RecoveryAction)
	require.Nil(t, compensationResult.NotificationAction)
	require.Nil(t, compensationResult.RestrictionAction)

	_, err = testStore.GetClaimRecoveryByClaimID(ctx, submitResult.Claim.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)

	actions, err := testStore.ListBehaviorActionsByDecision(ctx, submitResult.BehaviorDecision.ID)
	require.NoError(t, err)
	require.Len(t, actions, 1)

	actionByKey := make(map[string]BehaviorAction, len(actions))
	for _, action := range actions {
		actionByKey[actionKey(action)] = action
	}
	require.Contains(t, actionByKey, "payout:user")
	require.NotContains(t, actionByKey, "recovery:merchant")
	require.NotContains(t, actionByKey, "notify:merchant")

	err = testStore.MarkClaimPaid(ctx, MarkClaimPaidParams{
		ID:     submitResult.Claim.ID,
		PaidAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)
	paidClaim, err := testStore.GetClaim(ctx, submitResult.Claim.ID)
	require.NoError(t, err)
	require.True(t, paidClaim.PaidAt.Valid)

	finalizeResult, err := testStore.FinalizeClaimCompensationAfterPayoutTx(ctx, FinalizeClaimCompensationAfterPayoutTxParams{ClaimID: submitResult.Claim.ID})
	require.NoError(t, err)
	require.NotNil(t, finalizeResult.RecoveryAction)
	require.NotNil(t, finalizeResult.NotificationAction)
	require.Nil(t, finalizeResult.RestrictionAction)

	recovery, err := testStore.GetClaimRecoveryByClaimID(ctx, submitResult.Claim.ID)
	require.NoError(t, err)
	require.Equal(t, "merchant", recovery.RecoveryTarget.String)
	require.Equal(t, approvedAmount, recovery.RecoveryAmount)
	require.Equal(t, submitResult.BehaviorDecision.ID, recovery.DecisionID.Int64)
	require.WithinDuration(t, recoveryDueAt, recovery.DueAt, time.Second)
	recoveryEvents, err := testStore.ListClaimRecoveryEventsByRecovery(ctx, recovery.ID)
	require.NoError(t, err)
	require.Len(t, recoveryEvents, 2)
	require.Equal(t, ClaimRecoveryEventTypeCreated, recoveryEvents[0].EventType)
	require.Equal(t, ClaimRecoveryEventTypePayable, recoveryEvents[1].EventType)

	secondResult, err := testStore.CreateClaimCompensationTx(ctx, CreateClaimCompensationTxParams{ClaimID: submitResult.Claim.ID})
	require.NoError(t, err)
	require.Equal(t, compensationResult.PayoutAction.ID, secondResult.PayoutAction.ID)
	require.NotNil(t, secondResult.RecoveryAction)
	require.Equal(t, finalizeResult.RecoveryAction.ID, secondResult.RecoveryAction.ID)
	require.Nil(t, secondResult.NotificationAction)

	actions, err = testStore.ListBehaviorActionsByDecision(ctx, submitResult.BehaviorDecision.ID)
	require.NoError(t, err)
	require.Len(t, actions, 3)
	actionByKey = make(map[string]BehaviorAction, len(actions))
	for _, action := range actions {
		actionByKey[actionKey(action)] = action
	}
	require.Equal(t, finalizeResult.RecoveryAction.ID, actionByKey["recovery:merchant"].ID)
	require.Equal(t, finalizeResult.NotificationAction.ID, actionByKey["notify:merchant"].ID)
}

func TestCreateClaimCompensationTx_IsIdempotentForUserRestrictedArtifacts(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	address := createRandomUserAddress(t, user)
	order := createTakeoutOrderForClaimBehavior(t, user.ID, merchant.ID, &address.ID)

	approvedAmount := int64(1900)
	submitResult, err := testStore.CreateClaimWithBehaviorTx(ctx, CreateClaimWithBehaviorTxParams{
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "damage",
		Description:        "高风险用户需要进入补偿阶段",
		ClaimAmount:        approvedAmount,
		Status:             ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:       "auto",
		ApprovedAmount:     &approvedAmount,
		AutoApprovalReason: "高风险用户平台先行赔付",
		DecisionVersion:    "behavior-v2-test",
		ReasonCodes:        []string{"user_restricted"},
		ResponsibleParty:   "user",
		CompensationSource: "platform",
		TraceSummary:       "您的账号因索赔行为异常已被限制服务；若确认继续索赔，平台将先行赔付并停止后续服务。",
		DeviceID:           util.RandomString(12),
		DeviceFingerprint:  "deferred-user-" + util.RandomString(8),
		DeviceType:         "ios",
		AddressID:          &address.ID,
		CreateRecovery:     false,
		SkipActionCreation: true,
	})
	require.NoError(t, err)

	firstResult, err := testStore.CreateClaimCompensationTx(ctx, CreateClaimCompensationTxParams{ClaimID: submitResult.Claim.ID})
	require.NoError(t, err)
	require.NotNil(t, firstResult.PayoutAction)
	require.Nil(t, firstResult.RestrictionAction)
	require.Nil(t, firstResult.NotificationAction)
	require.Nil(t, firstResult.RecoveryAction)

	secondResult, err := testStore.CreateClaimCompensationTx(ctx, CreateClaimCompensationTxParams{ClaimID: submitResult.Claim.ID})
	require.NoError(t, err)
	require.Equal(t, firstResult.PayoutAction.ID, secondResult.PayoutAction.ID)
	require.Nil(t, secondResult.RestrictionAction)
	require.Nil(t, secondResult.NotificationAction)

	actions, err := testStore.ListBehaviorActionsByDecision(ctx, submitResult.BehaviorDecision.ID)
	require.NoError(t, err)
	require.Len(t, actions, 1)

	actionByKey := make(map[string]BehaviorAction, len(actions))
	for _, action := range actions {
		actionByKey[actionKey(action)] = action
	}
	require.Contains(t, actionByKey, "payout:user")
	require.NotContains(t, actionByKey, "block:user")
	require.NotContains(t, actionByKey, "notify:user")

	err = testStore.MarkClaimPaid(ctx, MarkClaimPaidParams{
		ID:     submitResult.Claim.ID,
		PaidAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)
	paidClaim, err := testStore.GetClaim(ctx, submitResult.Claim.ID)
	require.NoError(t, err)
	require.True(t, paidClaim.PaidAt.Valid)

	finalizeResult, err := testStore.FinalizeClaimCompensationAfterPayoutTx(ctx, FinalizeClaimCompensationAfterPayoutTxParams{ClaimID: submitResult.Claim.ID})
	require.NoError(t, err)
	require.NotNil(t, finalizeResult.RestrictionAction)
	require.NotNil(t, finalizeResult.NotificationAction)

	secondFinalizeResult, err := testStore.FinalizeClaimCompensationAfterPayoutTx(ctx, FinalizeClaimCompensationAfterPayoutTxParams{ClaimID: submitResult.Claim.ID})
	require.NoError(t, err)
	require.Equal(t, finalizeResult.RestrictionAction.ID, secondFinalizeResult.RestrictionAction.ID)
	require.Equal(t, finalizeResult.NotificationAction.ID, secondFinalizeResult.NotificationAction.ID)

	actions, err = testStore.ListBehaviorActionsByDecision(ctx, submitResult.BehaviorDecision.ID)
	require.NoError(t, err)
	require.Len(t, actions, 3)

	actionByKey = make(map[string]BehaviorAction, len(actions))
	for _, action := range actions {
		actionByKey[actionKey(action)] = action
	}
	require.Contains(t, actionByKey, "block:user")
	require.Contains(t, actionByKey, "notify:user")

	var restrictionDetail behaviorRestrictionActionDetail
	require.NoError(t, json.Unmarshal(actionByKey["block:user"].Detail, &restrictionDetail))
	require.Equal(t, submitResult.Claim.ID, restrictionDetail.ClaimID)
	require.Equal(t, user.ID, restrictionDetail.UserID)
}

func TestCreateClaimCompensationTx_RejectsLegacyAutoApprovedStatus(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	address := createRandomUserAddress(t, user)
	order := createTakeoutOrderForClaimBehavior(t, user.ID, merchant.ID, &address.ID)

	approvedAmount := int64(1500)
	legacyResult, err := testStore.CreateClaimWithBehaviorTx(ctx, CreateClaimWithBehaviorTxParams{
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "damage",
		Description:        "遗留 auto-approved 记录",
		ClaimAmount:        approvedAmount,
		Status:             ClaimStatusAutoApproved,
		ApprovalType:       "auto",
		ApprovedAmount:     &approvedAmount,
		AutoApprovalReason: "legacy processing row",
		DecisionVersion:    "behavior-v2-test",
		ReasonCodes:        []string{"user_restricted"},
		ResponsibleParty:   "user",
		CompensationSource: "platform",
		TraceSummary:       "legacy processing row",
		DeviceID:           util.RandomString(12),
		DeviceFingerprint:  "legacy-auto-approved-" + util.RandomString(8),
		DeviceType:         "ios",
		AddressID:          &address.ID,
		CreateRecovery:     false,
		SkipActionCreation: true,
	})
	require.NoError(t, err)

	_, err = testStore.CreateClaimCompensationTx(ctx, CreateClaimCompensationTxParams{ClaimID: legacyResult.Claim.ID})
	require.ErrorIs(t, err, ErrClaimCompensationNotEligible)
}

func TestCreateClaimWithBehaviorTx_RejectsRiderRecoveryWithoutConcreteRider(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	address := createRandomUserAddress(t, user)
	order := createTakeoutOrderForClaimBehavior(t, user.ID, merchant.ID, &address.ID)

	approvedAmount := int64(1800)
	_, err := testStore.CreateClaimWithBehaviorTx(ctx, CreateClaimWithBehaviorTxParams{
		OrderID:            order.ID,
		UserID:             user.ID,
		ClaimType:          "damage",
		Description:        "缺少具体骑手主体时不应进入骑手追偿",
		ClaimAmount:        approvedAmount,
		Status:             ClaimStatusWaitingCustomerConfirmation,
		ApprovalType:       "instant",
		ApprovedAmount:     &approvedAmount,
		AutoApprovalReason: "服务侧异常索赔默认由骑手承担责任",
		DecisionVersion:    "behavior-v2-test",
		ReasonCodes:        []string{"rider_recovery"},
		ResponsibleParty:   "rider",
		CompensationSource: "rider",
		TraceSummary:       "服务侧异常索赔默认由骑手承担责任",
		DeviceID:           util.RandomString(12),
		DeviceFingerprint:  "missing-rider-" + util.RandomString(8),
		DeviceType:         "ios",
		AddressID:          &address.ID,
		CreateRecovery:     true,
		RecoveryTarget:     "rider",
		RecoveryAmount:     approvedAmount,
		DecisionSnapshot:   []byte(`{"phase":"submit"}`),
		SkipActionCreation: true,
	})
	require.ErrorIs(t, err, ErrClaimResponsibleRiderMissing)
}
