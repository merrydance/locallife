package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type behaviorPayoutActionDetail struct {
	Action     string `json:"action"`
	ClaimID    int64  `json:"claim_id"`
	UserID     int64  `json:"user_id"`
	Amount     int64  `json:"amount"`
	SourceType string `json:"source_type"`
	SourceID   int64  `json:"source_id"`
	Remark     string `json:"remark"`
}

type behaviorRecoveryActionDetail struct {
	Action         string    `json:"action"`
	ClaimID        int64     `json:"claim_id"`
	RecoveryID     int64     `json:"recovery_id"`
	TargetEntity   string    `json:"target_entity"`
	TargetID       int64     `json:"target_id,omitempty"`
	RecoveryBasis  string    `json:"recovery_basis,omitempty"`
	RecoveryAmount int64     `json:"recovery_amount"`
	DueAt          time.Time `json:"due_at"`
	Remark         string    `json:"remark"`
}

type behaviorRestrictionActionDetail struct {
	Action            string `json:"action"`
	ClaimID           int64  `json:"claim_id"`
	UserID            int64  `json:"user_id"`
	DecisionMode      string `json:"decision_mode"`
	RestrictionReason string `json:"restriction_reason,omitempty"`
	Remark            string `json:"remark"`
}

type behaviorNotifyActionDetail struct {
	Action           string `json:"action"`
	ClaimID          int64  `json:"claim_id"`
	TargetEntity     string `json:"target_entity"`
	TargetID         int64  `json:"target_id,omitempty"`
	RecipientUserID  int64  `json:"recipient_user_id,omitempty"`
	NotificationType string `json:"notification_type"`
	Title            string `json:"title"`
	Content          string `json:"content"`
	RelatedType      string `json:"related_type"`
	RelatedID        int64  `json:"related_id"`
	Remark           string `json:"remark"`
}

type behaviorDecisionFactSnapshot struct {
	OrderID              int64                       `json:"order_id"`
	ClaimType            string                      `json:"claim_type"`
	ClaimAmount          int64                       `json:"claim_amount"`
	ResponsibleParty     string                      `json:"responsible_party"`
	CompensationSource   string                      `json:"compensation_source"`
	DecisionMode         string                      `json:"decision_mode"`
	ResponsibilityDomain string                      `json:"responsibility_domain"`
	PayoutMode           string                      `json:"payout_mode"`
	RecoveryTarget       string                      `json:"recovery_target,omitempty"`
	RecoveryAmount       int64                       `json:"recovery_amount,omitempty"`
	ApprovedAmount       int64                       `json:"approved_amount,omitempty"`
	Associations         behaviorAssociationPayload  `json:"associations"`
	ResponsibilityFacts  behaviorResponsibilityFacts `json:"responsibility_facts"`
}

type behaviorTraceMetricPayload struct {
	CompletedOrders          int32 `json:"completed_orders,omitempty"`
	ClaimAttempts            int32 `json:"claim_attempts,omitempty"`
	EffectiveClaims          int32 `json:"effective_claims,omitempty"`
	EffectiveLiabilityClaims int32 `json:"effective_liability_claims,omitempty"`
	MaliciousConfirmedClaims int32 `json:"malicious_confirmed_claims,omitempty"`
	MerchantRecoveredClaims  int32 `json:"merchant_recovered_claims,omitempty"`
	RiderRecoveredClaims     int32 `json:"rider_recovered_claims,omitempty"`
	DistinctDevices          int32 `json:"distinct_devices,omitempty"`
	DistinctAddresses        int32 `json:"distinct_addresses,omitempty"`
}

type behaviorEmptyAssociationPayload struct{}

type behaviorAssociationPayload struct {
	HasDeviceFingerprint    bool  `json:"has_device_fingerprint,omitempty"`
	HasAddressID            bool  `json:"has_address_id,omitempty"`
	DistinctDevices         int32 `json:"distinct_devices,omitempty"`
	DistinctAddresses       int32 `json:"distinct_addresses,omitempty"`
	SharedDeviceUsers       int32 `json:"shared_device_users,omitempty"`
	SharedDeviceOtherUsers  int32 `json:"shared_device_other_users,omitempty"`
	SharedAddressUsers      int32 `json:"shared_address_users,omitempty"`
	SharedAddressOtherUsers int32 `json:"shared_address_other_users,omitempty"`
	SharedAddressOrders     int64 `json:"shared_address_orders,omitempty"`
}

type behaviorResponsibilityFacts struct {
	OrderType            string   `json:"order_type,omitempty"`
	OrderStatus          string   `json:"order_status,omitempty"`
	FulfillmentStatus    string   `json:"fulfillment_status,omitempty"`
	AddressPresent       bool     `json:"address_present,omitempty"`
	DeliveryExists       bool     `json:"delivery_exists,omitempty"`
	RiderAssigned        bool     `json:"rider_assigned,omitempty"`
	CourierAccepted      bool     `json:"courier_accepted,omitempty"`
	PickupConfirmed      bool     `json:"pickup_confirmed,omitempty"`
	DeliveryCompleted    bool     `json:"delivery_completed,omitempty"`
	DeliveryStatus       string   `json:"delivery_status,omitempty"`
	MissingCriticalFacts []string `json:"missing_critical_facts,omitempty"`
}

type behaviorDecisionGraphHits struct {
	HitCodes                []string `json:"hit_codes,omitempty"`
	SharedDeviceUsers       int32    `json:"shared_device_users,omitempty"`
	SharedDeviceOtherUsers  int32    `json:"shared_device_other_users,omitempty"`
	SharedAddressUsers      int32    `json:"shared_address_users,omitempty"`
	SharedAddressOtherUsers int32    `json:"shared_address_other_users,omitempty"`
}

type claimRecoveryEventPayload struct {
	RecoveryTarget string `json:"recovery_target,omitempty"`
	RecoveryBasis  string `json:"recovery_basis,omitempty"`
	RecoveryAmount int64  `json:"recovery_amount"`
}

const (
	behaviorDecisionRepeatRiskThreshold = int32(3)
)

type behaviorDecisionScoreBreakdown struct {
	Version           string                      `json:"version"`
	UserRisk          behaviorDecisionScoreDetail `json:"user_risk"`
	MerchantLiability behaviorDecisionScoreDetail `json:"merchant_liability"`
	RiderLiability    behaviorDecisionScoreDetail `json:"rider_liability"`
	Confidence        behaviorDecisionScoreDetail `json:"confidence"`
}

type behaviorDecisionScoreDetail struct {
	Score   int32                    `json:"score"`
	Signals []behaviorDecisionSignal `json:"signals,omitempty"`
}

type behaviorDecisionSignal struct {
	Code   string `json:"code"`
	Weight int32  `json:"weight"`
	Count  int32  `json:"count,omitempty"`
	Active bool   `json:"active"`
}

type behaviorDecisionResolution struct {
	ResponsibleParty     string
	CompensationSource   string
	ApprovalType         string
	AutoApprovalReason   string
	TraceSummary         string
	DecisionMode         string
	ResponsibilityDomain string
	PayoutMode           string
	FallbackReason       string
	RestrictionReason    string
	CreateRecovery       bool
	RecoveryTarget       string
	RecoveryAmount       int64
}

type CreateClaimWithBehaviorTxParams struct {
	OrderID            int64
	UserID             int64
	ClaimType          string
	Description        string
	ClaimAmount        int64
	Status             string
	ApprovalType       string
	ApprovedAmount     *int64
	AutoApprovalReason string
	LookbackResult     []byte
	DecisionVersion    string
	ReasonCodes        []string
	ResponsibleParty   string
	CompensationSource string
	TraceSummary       string
	DeviceID           string
	DeviceFingerprint  string
	DeviceType         string
	IPAddress          string
	UserAgent          string
	AddressID          *int64
	CreateRecovery     bool
	RecoveryTarget     string
	RecoveryAmount     int64
	RecoveryDueAt      *time.Time
	DecisionSnapshot   []byte
	SkipActionCreation bool
}

type CreateClaimWithBehaviorTxResult struct {
	Claim              Claim
	BehaviorDecision   BehaviorDecision
	PayoutAction       *BehaviorAction
	RecoveryAction     *BehaviorAction
	RestrictionAction  *BehaviorAction
	NotificationAction *BehaviorAction
}

type CreateClaimCompensationTxParams struct {
	ClaimID int64
}

type CreateClaimCompensationTxResult struct {
	Claim              Claim
	BehaviorDecision   BehaviorDecision
	PayoutAction       *BehaviorAction
	RecoveryAction     *BehaviorAction
	RestrictionAction  *BehaviorAction
	NotificationAction *BehaviorAction
}

type FinalizeClaimCompensationAfterPayoutTxParams struct {
	ClaimID int64
}

type FinalizeClaimCompensationAfterPayoutTxResult struct {
	Claim              Claim
	BehaviorDecision   BehaviorDecision
	RecoveryAction     *BehaviorAction
	RestrictionAction  *BehaviorAction
	NotificationAction *BehaviorAction
}

func (store *SQLStore) CreateClaimWithBehaviorTx(ctx context.Context, arg CreateClaimWithBehaviorTxParams) (CreateClaimWithBehaviorTxResult, error) {
	var result CreateClaimWithBehaviorTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		now := time.Now()
		order, err := q.GetOrder(ctx, arg.OrderID)
		if err != nil {
			return err
		}

		if arg.DeviceID != "" || arg.DeviceFingerprint != "" {
			_, err := q.UpsertUserDevice(ctx, UpsertUserDeviceParams{
				UserID:            arg.UserID,
				DeviceID:          arg.DeviceID,
				DeviceFingerprint: pgtype.Text{String: arg.DeviceFingerprint, Valid: arg.DeviceFingerprint != ""},
				DeviceType:        arg.DeviceType,
			})
			if err != nil {
				return err
			}
		}

		var riderID pgtype.Int8
		var delivery *Delivery
		if order.OrderType == "takeout" {
			loadedDelivery, err := q.GetDeliveryByOrderID(ctx, order.ID)
			if err == nil {
				delivery = &loadedDelivery
			}
			if delivery != nil && delivery.RiderID.Valid {
				riderID = pgtype.Int8{Int64: delivery.RiderID.Int64, Valid: true}
			}
		}

		decisionResolution := deriveBehaviorDecisionResolution(arg)
		addressID := resolveBehaviorAddressID(order, arg.AddressID)
		associationPayload, graphHits, err := loadUserAssociationPayload(ctx, q, arg.UserID, arg.DeviceFingerprint, addressID)
		if err != nil {
			return err
		}
		responsibilityFacts, _ := buildResponsibilityFacts(order, delivery, decisionResolution.ResponsibilityDomain)

		userScoreSummary, err := loadBehaviorEffectSummary(ctx, q, "user", arg.UserID, 30, now)
		if err != nil {
			return err
		}

		decisionResolution = promoteBehaviorUserRestricted(
			arg.Status,
			arg.ApprovedAmount,
			decisionResolution,
			associationPayload,
			userScoreSummary,
		)
		if err := validateClaimCompensationTargets(decisionResolution, riderID); err != nil {
			return err
		}
		scoreBreakdownJSON := []byte(`{"version":"claims_rules_v1"}`)

		claimParams := CreateClaimParams{
			OrderID:            arg.OrderID,
			UserID:             arg.UserID,
			ClaimType:          arg.ClaimType,
			Description:        arg.Description,
			ClaimAmount:        arg.ClaimAmount,
			Status:             arg.Status,
			IsMalicious:        false,
			LookbackResult:     arg.LookbackResult,
			CreatedAt:          now,
			ApprovalType:       pgtype.Text{String: decisionResolution.ApprovalType, Valid: decisionResolution.ApprovalType != ""},
			AutoApprovalReason: pgtype.Text{String: decisionResolution.AutoApprovalReason, Valid: decisionResolution.AutoApprovalReason != ""},
			DecisionVersion:    pgtype.Text{String: arg.DecisionVersion, Valid: arg.DecisionVersion != ""},
			DecisionReason:     pgtype.Text{String: decisionResolution.TraceSummary, Valid: decisionResolution.TraceSummary != ""},
		}
		if arg.ApprovedAmount != nil {
			claimParams.ApprovedAmount = pgtype.Int8{Int64: *arg.ApprovedAmount, Valid: true}
		}

		claim, err := q.CreateClaim(ctx, claimParams)
		if err != nil {
			return err
		}

		graphHitsJSON, err := json.Marshal(graphHits)
		if err != nil {
			return err
		}
		factSnapshot, err := json.Marshal(behaviorDecisionFactSnapshot{
			OrderID:              arg.OrderID,
			ClaimType:            arg.ClaimType,
			ClaimAmount:          arg.ClaimAmount,
			ResponsibleParty:     decisionResolution.ResponsibleParty,
			CompensationSource:   decisionResolution.CompensationSource,
			DecisionMode:         decisionResolution.DecisionMode,
			ResponsibilityDomain: decisionResolution.ResponsibilityDomain,
			PayoutMode:           decisionResolution.PayoutMode,
			RecoveryTarget:       decisionResolution.RecoveryTarget,
			RecoveryAmount:       decisionResolution.RecoveryAmount,
			ApprovedAmount:       approvedAmountValue(arg.ApprovedAmount),
			Associations:         associationPayload,
			ResponsibilityFacts:  responsibilityFacts,
		})
		if err != nil {
			return err
		}

		decision, err := q.CreateBehaviorDecision(ctx, CreateBehaviorDecisionParams{
			OrderID:                pgtype.Int8{Int64: arg.OrderID, Valid: true},
			ReservationID:          pgtype.Int8{},
			ClaimID:                pgtype.Int8{Int64: claim.ID, Valid: true},
			UserID:                 pgtype.Int8{Int64: arg.UserID, Valid: true},
			MerchantID:             pgtype.Int8{Int64: order.MerchantID, Valid: true},
			RiderID:                riderID,
			DecisionVersion:        arg.DecisionVersion,
			ReasonCodes:            normalizeBehaviorReasonCodes(arg.ReasonCodes, decisionResolution),
			ResponsibleParty:       decisionResolution.ResponsibleParty,
			CompensationSource:     decisionResolution.CompensationSource,
			DecisionStatus:         "decided",
			TraceSummary:           pgtype.Text{String: decisionResolution.TraceSummary, Valid: decisionResolution.TraceSummary != ""},
			DecisionMode:           pgtype.Text{String: decisionResolution.DecisionMode, Valid: decisionResolution.DecisionMode != ""},
			ResponsibilityDomain:   pgtype.Text{String: decisionResolution.ResponsibilityDomain, Valid: decisionResolution.ResponsibilityDomain != ""},
			PayoutMode:             pgtype.Text{String: decisionResolution.PayoutMode, Valid: decisionResolution.PayoutMode != ""},
			ConfidenceScore:        pgtype.Int4{},
			UserRiskScore:          pgtype.Int4{},
			MerchantLiabilityScore: pgtype.Int4{},
			RiderLiabilityScore:    pgtype.Int4{},
			FallbackReason:         pgtype.Text{String: decisionResolution.FallbackReason, Valid: decisionResolution.FallbackReason != ""},
			RestrictionReason:      pgtype.Text{String: decisionResolution.RestrictionReason, Valid: decisionResolution.RestrictionReason != ""},
			LiabilityShares:        []byte(`{}`),
			ScoreBreakdown:         scoreBreakdownJSON,
			GraphHits:              graphHitsJSON,
			FactSnapshot:           factSnapshot,
		})
		if err != nil {
			return err
		}

		if err := createDecisionEffects(ctx, q, decision.ID, arg.UserID, order.MerchantID, riderID, decisionResolution.DecisionMode, arg.ApprovedAmount); err != nil {
			return err
		}
		if err := q.UpdateBehaviorDecisionProfileEffectApplied(ctx, UpdateBehaviorDecisionProfileEffectAppliedParams{
			ID:                   decision.ID,
			ProfileEffectApplied: true,
		}); err != nil {
			return err
		}

		stats, err := q.GetUserClaimWindowStats(ctx, arg.UserID)
		if err != nil {
			return err
		}
		userNet7d, err := loadBehaviorEffectSummary(ctx, q, "user", arg.UserID, 7, now)
		if err != nil {
			return err
		}
		userNet30d, err := loadBehaviorEffectSummary(ctx, q, "user", arg.UserID, 30, now)
		if err != nil {
			return err
		}
		if err := createUserTraceSnapshots(ctx, q, decision.ID, arg.UserID, 7, stats.TakeoutOrders7d, stats.Claims7d, associationPayload, graphHits.HitCodes, userNet7d); err != nil {
			return err
		}
		if err := createUserTraceSnapshots(ctx, q, decision.ID, arg.UserID, 30, stats.TakeoutOrders30d, stats.Claims30d, associationPayload, graphHits.HitCodes, userNet30d); err != nil {
			return err
		}

		merchantNet7d, err := loadBehaviorEffectSummary(ctx, q, "merchant", order.MerchantID, 7, now)
		if err != nil {
			return err
		}
		merchantNet30d, err := loadBehaviorEffectSummary(ctx, q, "merchant", order.MerchantID, 30, now)
		if err != nil {
			return err
		}
		if err := createEntityTraceSnapshots(ctx, q, decision.ID, order.MerchantID, "merchant", 7, graphHits.HitCodes, merchantNet7d, now); err != nil {
			return err
		}
		if err := createEntityTraceSnapshots(ctx, q, decision.ID, order.MerchantID, "merchant", 30, graphHits.HitCodes, merchantNet30d, now); err != nil {
			return err
		}
		if riderID.Valid {
			riderNet7d, err := loadBehaviorEffectSummary(ctx, q, "rider", riderID.Int64, 7, now)
			if err != nil {
				return err
			}
			riderNet30d, err := loadBehaviorEffectSummary(ctx, q, "rider", riderID.Int64, 30, now)
			if err != nil {
				return err
			}
			if err := createEntityTraceSnapshots(ctx, q, decision.ID, riderID.Int64, "rider", 7, graphHits.HitCodes, riderNet7d, now); err != nil {
				return err
			}
			if err := createEntityTraceSnapshots(ctx, q, decision.ID, riderID.Int64, "rider", 30, graphHits.HitCodes, riderNet30d, now); err != nil {
				return err
			}
		}

		result.Claim = claim
		result.BehaviorDecision = decision

		if !arg.SkipActionCreation {
			artifacts, err := ensureClaimCompensationArtifacts(ctx, q, claimCompensationArtifactParams{
				Claim:              claim,
				BehaviorDecision:   decision,
				Order:              order,
				RiderID:            riderID,
				ApprovedAmount:     arg.ApprovedAmount,
				DecisionResolution: decisionResolution,
				DecisionSnapshot:   arg.DecisionSnapshot,
				RecoveryDueAt:      arg.RecoveryDueAt,
			})
			if err != nil {
				return err
			}
			result.PayoutAction = artifacts.PayoutAction
			result.RecoveryAction = artifacts.RecoveryAction
			result.RestrictionAction = artifacts.RestrictionAction
			result.NotificationAction = artifacts.NotificationAction
		}

		return nil
	})

	return result, err
}

func (store *SQLStore) CreateClaimCompensationTx(ctx context.Context, arg CreateClaimCompensationTxParams) (CreateClaimCompensationTxResult, error) {
	var result CreateClaimCompensationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		claim, err := q.GetClaimForUpdate(ctx, arg.ClaimID)
		if err != nil {
			return err
		}

		if !claim.ApprovedAmount.Valid || claim.ApprovedAmount.Int64 <= 0 {
			return ErrClaimCompensationNotEligible
		}

		if claim.Status == ClaimStatusRejected || claim.Status == ClaimStatusWithdrawn || claim.Status == ClaimStatusPending {
			return ErrClaimCompensationNotEligible
		}

		if !claim.PaidAt.Valid && claim.Status != ClaimStatusApproved && claim.Status != ClaimStatusWaitingCustomerConfirmation {
			return ErrClaimCompensationNotEligible
		}

		decision, err := q.GetLatestBehaviorDecisionByClaimID(ctx, pgtype.Int8{Int64: arg.ClaimID, Valid: true})
		if err != nil {
			return err
		}

		order, err := q.GetOrder(ctx, claim.OrderID)
		if err != nil {
			return err
		}

		var riderID pgtype.Int8
		if order.OrderType == "takeout" {
			delivery, err := q.GetDeliveryByOrderID(ctx, order.ID)
			if err == nil && delivery.RiderID.Valid {
				riderID = pgtype.Int8{Int64: delivery.RiderID.Int64, Valid: true}
			}
		}

		actions, err := q.ListBehaviorActionsByDecision(ctx, decision.ID)
		if err != nil {
			return err
		}

		var recovery *ClaimRecovery
		existingRecovery, err := q.GetClaimRecoveryByClaimID(ctx, claim.ID)
		if err == nil {
			recovery = &existingRecovery
		} else if err != ErrRecordNotFound {
			return err
		}

		if claim.PaidAt.Valid || claim.Status == ClaimStatusApproved {
			decisionResolution := persistedBehaviorDecisionResolution(claim, decision, recovery)
			result.Claim = claim
			result.BehaviorDecision = decision
			result.PayoutAction = findBehaviorAction(actions, "payout", "user")
			result.RecoveryAction = findBehaviorAction(actions, "recovery", decisionResolution.RecoveryTarget)
			return nil
		}

		artifacts, err := ensureClaimCompensationStartArtifacts(ctx, q, claimCompensationArtifactParams{
			Claim:              claim,
			BehaviorDecision:   decision,
			Order:              order,
			RiderID:            riderID,
			ApprovedAmount:     approvedClaimAmountPointer(claim),
			DecisionResolution: persistedBehaviorDecisionResolution(claim, decision, recovery),
			DecisionSnapshot:   decisionSnapshotForCompensation(decision),
			ExistingActions:    actions,
			ExistingRecovery:   recovery,
		})
		if err != nil {
			return err
		}

		if claim.Status != ClaimStatusApproved && !claim.PaidAt.Valid && claim.ApprovedAmount.Valid && claim.ApprovedAmount.Int64 > 0 {
			if err := q.UpdateClaimStatus(ctx, UpdateClaimStatusParams{
				ID:     claim.ID,
				Status: ClaimStatusApproved,
			}); err != nil {
				return err
			}
			claim.Status = ClaimStatusApproved
		}

		result.Claim = claim
		result.BehaviorDecision = decision
		result.PayoutAction = artifacts.PayoutAction
		result.RecoveryAction = artifacts.RecoveryAction
		result.RestrictionAction = artifacts.RestrictionAction
		result.NotificationAction = artifacts.NotificationAction
		return nil
	})

	return result, err
}

func (store *SQLStore) FinalizeClaimCompensationAfterPayoutTx(ctx context.Context, arg FinalizeClaimCompensationAfterPayoutTxParams) (FinalizeClaimCompensationAfterPayoutTxResult, error) {
	var result FinalizeClaimCompensationAfterPayoutTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		claim, err := q.GetClaimForUpdate(ctx, arg.ClaimID)
		if err != nil {
			return err
		}
		if !claim.PaidAt.Valid {
			return ErrClaimCompensationNotEligible
		}

		decision, err := q.GetLatestBehaviorDecisionByClaimID(ctx, pgtype.Int8{Int64: arg.ClaimID, Valid: true})
		if err != nil {
			return err
		}

		order, err := q.GetOrder(ctx, claim.OrderID)
		if err != nil {
			return err
		}

		var riderID pgtype.Int8
		if order.OrderType == "takeout" {
			delivery, err := q.GetDeliveryByOrderID(ctx, order.ID)
			if err == nil && delivery.RiderID.Valid {
				riderID = pgtype.Int8{Int64: delivery.RiderID.Int64, Valid: true}
			}
		}

		actions, err := q.ListBehaviorActionsByDecision(ctx, decision.ID)
		if err != nil {
			return err
		}

		var recovery *ClaimRecovery
		existingRecovery, err := q.GetClaimRecoveryByClaimID(ctx, claim.ID)
		if err == nil {
			recovery = &existingRecovery
		} else if err != ErrRecordNotFound {
			return err
		}

		artifacts, err := ensureClaimCompensationPostPayoutArtifacts(ctx, q, claimCompensationArtifactParams{
			Claim:              claim,
			BehaviorDecision:   decision,
			Order:              order,
			RiderID:            riderID,
			ApprovedAmount:     approvedClaimAmountPointer(claim),
			DecisionResolution: persistedBehaviorDecisionResolution(claim, decision, recovery),
			ExistingActions:    actions,
			ExistingRecovery:   recovery,
		})
		if err != nil {
			return err
		}

		result.Claim = claim
		result.BehaviorDecision = decision
		result.RecoveryAction = recoveryActionFromArtifactsOrExisting(artifacts, actions, persistedBehaviorDecisionResolution(claim, decision, recovery))
		result.RestrictionAction = artifacts.RestrictionAction
		result.NotificationAction = artifacts.NotificationAction
		return nil
	})

	return result, err
}

func IsClaimCompensationNotEligible(err error) bool {
	return errors.Is(err, ErrClaimCompensationNotEligible)
}

type claimCompensationArtifactParams struct {
	Claim              Claim
	BehaviorDecision   BehaviorDecision
	Order              Order
	RiderID            pgtype.Int8
	ApprovedAmount     *int64
	DecisionResolution behaviorDecisionResolution
	DecisionSnapshot   []byte
	RecoveryDueAt      *time.Time
	ExistingActions    []BehaviorAction
	ExistingRecovery   *ClaimRecovery
}

type claimCompensationArtifacts struct {
	PayoutAction       *BehaviorAction
	RecoveryAction     *BehaviorAction
	RestrictionAction  *BehaviorAction
	NotificationAction *BehaviorAction
}

func ensureClaimCompensationArtifacts(ctx context.Context, q *Queries, arg claimCompensationArtifactParams) (claimCompensationArtifacts, error) {
	artifacts := claimCompensationArtifacts{
		PayoutAction:       findBehaviorAction(arg.ExistingActions, "payout", "user"),
		RecoveryAction:     findBehaviorAction(arg.ExistingActions, "recovery", arg.DecisionResolution.RecoveryTarget),
		RestrictionAction:  findBehaviorAction(arg.ExistingActions, "block", "user"),
		NotificationAction: findBehaviorAction(arg.ExistingActions, "notify", notificationTargetEntity(arg.DecisionResolution)),
	}

	if err := validateClaimCompensationTargets(arg.DecisionResolution, arg.RiderID); err != nil {
		return artifacts, err
	}

	if artifacts.PayoutAction == nil && arg.ApprovedAmount != nil && *arg.ApprovedAmount > 0 {
		detail, err := json.Marshal(behaviorPayoutActionDetail{
			Action:     "platform_payout",
			ClaimID:    arg.Claim.ID,
			UserID:     arg.Claim.UserID,
			Amount:     *arg.ApprovedAmount,
			SourceType: "platform",
			SourceID:   0,
			Remark:     "platform payout",
		})
		if err != nil {
			return artifacts, err
		}
		createdAction, err := q.CreateBehaviorAction(ctx, CreateBehaviorActionParams{
			DecisionID:   arg.BehaviorDecision.ID,
			ActionType:   "payout",
			TargetEntity: "user",
			Status:       "created",
			Detail:       detail,
		})
		if err != nil {
			return artifacts, err
		}
		artifacts.PayoutAction = &createdAction
	}

	if arg.DecisionResolution.CreateRecovery {
		recovery := arg.ExistingRecovery
		if recovery == nil {
			dueAt := time.Now().Add(24 * time.Hour)
			if arg.RecoveryDueAt != nil {
				dueAt = *arg.RecoveryDueAt
			}
			createdRecovery, err := q.CreateClaimRecovery(ctx, CreateClaimRecoveryParams{
				ClaimID:          arg.Claim.ID,
				OrderID:          arg.Claim.OrderID,
				DecisionID:       pgtype.Int8{Int64: arg.BehaviorDecision.ID, Valid: true},
				ResponsibleParty: arg.DecisionResolution.ResponsibleParty,
				RecoveryTarget:   pgtype.Text{String: arg.DecisionResolution.RecoveryTarget, Valid: arg.DecisionResolution.RecoveryTarget != ""},
				RecoveryAmount:   arg.DecisionResolution.RecoveryAmount,
				Status:           "pending",
				DueAt:            dueAt,
				DecisionSnapshot: arg.DecisionSnapshot,
				RecoveryBasis:    pgtype.Text{String: recoveryBasisFromDecisionMode(arg.DecisionResolution.DecisionMode), Valid: recoveryBasisFromDecisionMode(arg.DecisionResolution.DecisionMode) != ""},
			})
			if err != nil {
				return artifacts, err
			}
			recovery = &createdRecovery
			recoveryEventPayload, err := json.Marshal(claimRecoveryEventPayload{
				RecoveryTarget: arg.DecisionResolution.RecoveryTarget,
				RecoveryBasis:  recoveryBasisFromDecisionMode(arg.DecisionResolution.DecisionMode),
				RecoveryAmount: arg.DecisionResolution.RecoveryAmount,
			})
			if err != nil {
				return artifacts, err
			}
			if _, err := q.CreateClaimRecoveryEvent(ctx, CreateClaimRecoveryEventParams{
				RecoveryID: recovery.ID,
				DecisionID: pgtype.Int8{Int64: arg.BehaviorDecision.ID, Valid: true},
				EventType:  ClaimRecoveryEventTypeCreated,
				Payload:    recoveryEventPayload,
			}); err != nil {
				return artifacts, err
			}
		}

		if artifacts.RecoveryAction == nil && recovery != nil {
			targetID := int64(0)
			if arg.DecisionResolution.RecoveryTarget == "merchant" {
				targetID = arg.Order.MerchantID
			}
			if arg.DecisionResolution.RecoveryTarget == "rider" && arg.RiderID.Valid {
				targetID = arg.RiderID.Int64
			}
			recoveryActionDetail, err := json.Marshal(behaviorRecoveryActionDetail{
				Action:         "claim_recovery",
				ClaimID:        arg.Claim.ID,
				RecoveryID:     recovery.ID,
				TargetEntity:   arg.DecisionResolution.RecoveryTarget,
				TargetID:       targetID,
				RecoveryBasis:  recoveryBasisFromDecisionMode(arg.DecisionResolution.DecisionMode),
				RecoveryAmount: recovery.RecoveryAmount,
				DueAt:          recovery.DueAt,
				Remark:         "claim recovery created",
			})
			if err != nil {
				return artifacts, err
			}
			createdAction, err := q.CreateBehaviorAction(ctx, CreateBehaviorActionParams{
				DecisionID:   arg.BehaviorDecision.ID,
				ActionType:   "recovery",
				TargetEntity: arg.DecisionResolution.RecoveryTarget,
				Status:       "created",
				Detail:       recoveryActionDetail,
			})
			if err != nil {
				return artifacts, err
			}
			artifacts.RecoveryAction = &createdAction
		}
	}

	if artifacts.RestrictionAction == nil && arg.DecisionResolution.DecisionMode == BehaviorDecisionModeUserRestricted {
		restrictionDetail, err := json.Marshal(behaviorRestrictionActionDetail{
			Action:            "apply_user_restriction",
			ClaimID:           arg.Claim.ID,
			UserID:            arg.Claim.UserID,
			DecisionMode:      arg.DecisionResolution.DecisionMode,
			RestrictionReason: arg.DecisionResolution.RestrictionReason,
			Remark:            "user restricted action created",
		})
		if err != nil {
			return artifacts, err
		}
		createdAction, err := q.CreateBehaviorAction(ctx, CreateBehaviorActionParams{
			DecisionID:   arg.BehaviorDecision.ID,
			ActionType:   "block",
			TargetEntity: "user",
			Status:       "created",
			Detail:       restrictionDetail,
		})
		if err != nil {
			return artifacts, err
		}
		artifacts.RestrictionAction = &createdAction
	}

	if artifacts.NotificationAction == nil {
		notificationAction, err := createBehaviorNotificationAction(ctx, q, arg.BehaviorDecision.ID, arg.Claim, arg.Claim.UserID, arg.Order, arg.RiderID, arg.DecisionResolution)
		if err != nil {
			return artifacts, err
		}
		artifacts.NotificationAction = notificationAction
	}

	return artifacts, nil
}

func ensureClaimCompensationStartArtifacts(ctx context.Context, q *Queries, arg claimCompensationArtifactParams) (claimCompensationArtifacts, error) {
	artifacts := claimCompensationArtifacts{
		PayoutAction:   findBehaviorAction(arg.ExistingActions, "payout", "user"),
		RecoveryAction: findBehaviorAction(arg.ExistingActions, "recovery", arg.DecisionResolution.RecoveryTarget),
	}

	if err := validateClaimCompensationTargets(arg.DecisionResolution, arg.RiderID); err != nil {
		return artifacts, err
	}

	if artifacts.PayoutAction == nil && arg.ApprovedAmount != nil && *arg.ApprovedAmount > 0 {
		detail, err := json.Marshal(behaviorPayoutActionDetail{
			Action:     "platform_payout",
			ClaimID:    arg.Claim.ID,
			UserID:     arg.Claim.UserID,
			Amount:     *arg.ApprovedAmount,
			SourceType: "platform",
			SourceID:   0,
			Remark:     "platform payout",
		})
		if err != nil {
			return artifacts, err
		}
		createdAction, err := q.CreateBehaviorAction(ctx, CreateBehaviorActionParams{
			DecisionID:   arg.BehaviorDecision.ID,
			ActionType:   "payout",
			TargetEntity: "user",
			Status:       "created",
			Detail:       detail,
		})
		if err != nil {
			return artifacts, err
		}
		artifacts.PayoutAction = &createdAction
	}

	if arg.DecisionResolution.CreateRecovery {
		recovery := arg.ExistingRecovery
		if recovery == nil {
			dueAt := time.Now().Add(24 * time.Hour)
			if arg.RecoveryDueAt != nil {
				dueAt = *arg.RecoveryDueAt
			}
			createdRecovery, err := q.CreateClaimRecovery(ctx, CreateClaimRecoveryParams{
				ClaimID:          arg.Claim.ID,
				OrderID:          arg.Claim.OrderID,
				DecisionID:       pgtype.Int8{Int64: arg.BehaviorDecision.ID, Valid: true},
				ResponsibleParty: arg.DecisionResolution.ResponsibleParty,
				RecoveryTarget:   pgtype.Text{String: arg.DecisionResolution.RecoveryTarget, Valid: arg.DecisionResolution.RecoveryTarget != ""},
				RecoveryAmount:   arg.DecisionResolution.RecoveryAmount,
				Status:           "pending",
				DueAt:            dueAt,
				DecisionSnapshot: arg.DecisionSnapshot,
				RecoveryBasis:    pgtype.Text{String: recoveryBasisFromDecisionMode(arg.DecisionResolution.DecisionMode), Valid: recoveryBasisFromDecisionMode(arg.DecisionResolution.DecisionMode) != ""},
			})
			if err != nil {
				return artifacts, err
			}
			recovery = &createdRecovery
			recoveryEventPayload, err := json.Marshal(claimRecoveryEventPayload{
				RecoveryTarget: arg.DecisionResolution.RecoveryTarget,
				RecoveryBasis:  recoveryBasisFromDecisionMode(arg.DecisionResolution.DecisionMode),
				RecoveryAmount: arg.DecisionResolution.RecoveryAmount,
			})
			if err != nil {
				return artifacts, err
			}
			if _, err := q.CreateClaimRecoveryEvent(ctx, CreateClaimRecoveryEventParams{
				RecoveryID: recovery.ID,
				DecisionID: pgtype.Int8{Int64: arg.BehaviorDecision.ID, Valid: true},
				EventType:  ClaimRecoveryEventTypeCreated,
				Payload:    recoveryEventPayload,
			}); err != nil {
				return artifacts, err
			}
		}

		if artifacts.RecoveryAction == nil && recovery != nil {
			targetID := int64(0)
			if arg.DecisionResolution.RecoveryTarget == "merchant" {
				targetID = arg.Order.MerchantID
			}
			if arg.DecisionResolution.RecoveryTarget == "rider" && arg.RiderID.Valid {
				targetID = arg.RiderID.Int64
			}
			recoveryActionDetail, err := json.Marshal(behaviorRecoveryActionDetail{
				Action:         "claim_recovery",
				ClaimID:        arg.Claim.ID,
				RecoveryID:     recovery.ID,
				TargetEntity:   arg.DecisionResolution.RecoveryTarget,
				TargetID:       targetID,
				RecoveryBasis:  recoveryBasisFromDecisionMode(arg.DecisionResolution.DecisionMode),
				RecoveryAmount: recovery.RecoveryAmount,
				DueAt:          recovery.DueAt,
				Remark:         "claim recovery created",
			})
			if err != nil {
				return artifacts, err
			}
			createdAction, err := q.CreateBehaviorAction(ctx, CreateBehaviorActionParams{
				DecisionID:   arg.BehaviorDecision.ID,
				ActionType:   "recovery",
				TargetEntity: arg.DecisionResolution.RecoveryTarget,
				Status:       "created",
				Detail:       recoveryActionDetail,
			})
			if err != nil {
				return artifacts, err
			}
			artifacts.RecoveryAction = &createdAction
		}
	}

	return artifacts, nil
}

func ensureClaimCompensationPostPayoutArtifacts(ctx context.Context, q *Queries, arg claimCompensationArtifactParams) (claimCompensationArtifacts, error) {
	artifacts := claimCompensationArtifacts{
		RestrictionAction:  findBehaviorAction(arg.ExistingActions, "block", "user"),
		NotificationAction: findBehaviorAction(arg.ExistingActions, "notify", notificationTargetEntity(arg.DecisionResolution)),
	}

	if err := validateClaimCompensationTargets(arg.DecisionResolution, arg.RiderID); err != nil {
		return artifacts, err
	}

	if artifacts.RestrictionAction == nil && arg.DecisionResolution.DecisionMode == BehaviorDecisionModeUserRestricted {
		restrictionDetail, err := json.Marshal(behaviorRestrictionActionDetail{
			Action:            "apply_user_restriction",
			ClaimID:           arg.Claim.ID,
			UserID:            arg.Claim.UserID,
			DecisionMode:      arg.DecisionResolution.DecisionMode,
			RestrictionReason: arg.DecisionResolution.RestrictionReason,
			Remark:            "user restricted action created after payout",
		})
		if err != nil {
			return artifacts, err
		}
		createdAction, err := q.CreateBehaviorAction(ctx, CreateBehaviorActionParams{
			DecisionID:   arg.BehaviorDecision.ID,
			ActionType:   "block",
			TargetEntity: "user",
			Status:       "created",
			Detail:       restrictionDetail,
		})
		if err != nil {
			return artifacts, err
		}
		artifacts.RestrictionAction = &createdAction
	}

	if artifacts.NotificationAction == nil {
		notificationAction, err := createBehaviorNotificationAction(ctx, q, arg.BehaviorDecision.ID, arg.Claim, arg.Claim.UserID, arg.Order, arg.RiderID, arg.DecisionResolution)
		if err != nil {
			return artifacts, err
		}
		artifacts.NotificationAction = notificationAction
	}

	return artifacts, nil
}

func recoveryActionFromArtifactsOrExisting(artifacts claimCompensationArtifacts, actions []BehaviorAction, resolution behaviorDecisionResolution) *BehaviorAction {
	if artifacts.RecoveryAction != nil {
		return artifacts.RecoveryAction
	}
	return findBehaviorAction(actions, "recovery", resolution.RecoveryTarget)
}

func persistedBehaviorDecisionResolution(claim Claim, decision BehaviorDecision, existingRecovery *ClaimRecovery) behaviorDecisionResolution {
	resolution := behaviorDecisionResolution{
		ResponsibleParty:   decision.ResponsibleParty,
		CompensationSource: decision.CompensationSource,
		TraceSummary:       persistedBehaviorDecisionTraceSummary(decision),
	}
	if decision.DecisionMode.Valid {
		resolution.DecisionMode = decision.DecisionMode.String
	}
	if decision.ResponsibilityDomain.Valid {
		resolution.ResponsibilityDomain = decision.ResponsibilityDomain.String
	}
	if decision.PayoutMode.Valid {
		resolution.PayoutMode = decision.PayoutMode.String
	}
	if decision.FallbackReason.Valid {
		resolution.FallbackReason = decision.FallbackReason.String
	}
	if decision.RestrictionReason.Valid {
		resolution.RestrictionReason = decision.RestrictionReason.String
	}

	if existingRecovery != nil {
		resolution.CreateRecovery = true
		resolution.RecoveryTarget = existingRecovery.RecoveryTarget.String
		resolution.RecoveryAmount = existingRecovery.RecoveryAmount
		return resolution
	}

	switch resolution.DecisionMode {
	case BehaviorDecisionModeMerchantRecovery:
		resolution.CreateRecovery = true
		resolution.RecoveryTarget = "merchant"
	case BehaviorDecisionModeRiderRecovery:
		resolution.CreateRecovery = true
		resolution.RecoveryTarget = "rider"
	}

	if len(decision.FactSnapshot) > 0 {
		var factSnapshot behaviorDecisionFactSnapshot
		if err := json.Unmarshal(decision.FactSnapshot, &factSnapshot); err == nil {
			if factSnapshot.RecoveryTarget != "" {
				resolution.RecoveryTarget = factSnapshot.RecoveryTarget
			}
			if factSnapshot.RecoveryAmount > 0 {
				resolution.RecoveryAmount = factSnapshot.RecoveryAmount
			}
		}
	}

	if resolution.CreateRecovery && resolution.RecoveryAmount <= 0 {
		if approvedAmount := approvedClaimAmountPointer(claim); approvedAmount != nil {
			resolution.RecoveryAmount = *approvedAmount
		}
	}

	return resolution
}

func persistedBehaviorDecisionTraceSummary(decision BehaviorDecision) string {
	if decision.TraceSummary.Valid && decision.TraceSummary.String != "" {
		return decision.TraceSummary.String
	}
	if decision.FallbackReason.Valid && decision.FallbackReason.String != "" {
		return decision.FallbackReason.String
	}
	if decision.RestrictionReason.Valid && decision.RestrictionReason.String != "" {
		return decision.RestrictionReason.String
	}
	return ""
}

func approvedClaimAmountPointer(claim Claim) *int64 {
	if !claim.ApprovedAmount.Valid || claim.ApprovedAmount.Int64 <= 0 {
		return nil
	}
	amount := claim.ApprovedAmount.Int64
	return &amount
}

func decisionSnapshotForCompensation(decision BehaviorDecision) []byte {
	if len(decision.FactSnapshot) > 0 {
		return decision.FactSnapshot
	}
	return []byte(`{}`)
}

func findBehaviorAction(actions []BehaviorAction, actionType string, targetEntity string) *BehaviorAction {
	for _, action := range actions {
		if action.ActionType != actionType {
			continue
		}
		if targetEntity != "" && action.TargetEntity != targetEntity {
			continue
		}
		copied := action
		return &copied
	}
	return nil
}

func validateClaimCompensationTargets(decisionResolution behaviorDecisionResolution, riderID pgtype.Int8) error {
	requiresConcreteRider := decisionResolution.DecisionMode == BehaviorDecisionModeRiderRecovery ||
		(decisionResolution.CreateRecovery && decisionResolution.RecoveryTarget == "rider")
	if requiresConcreteRider && !riderID.Valid {
		return ErrClaimResponsibleRiderMissing
	}
	return nil
}

func notificationTargetEntity(decisionResolution behaviorDecisionResolution) string {
	switch decisionResolution.DecisionMode {
	case BehaviorDecisionModeMerchantRecovery:
		return "merchant"
	case BehaviorDecisionModeRiderRecovery:
		return "rider"
	case BehaviorDecisionModeUserRestricted:
		return "user"
	default:
		return ""
	}
}

func createBehaviorNotificationAction(ctx context.Context, q *Queries, decisionID int64, claim Claim, userID int64, order Order, riderID pgtype.Int8, decisionResolution behaviorDecisionResolution) (*BehaviorAction, error) {
	var (
		actionName      string
		targetEntity    string
		targetID        int64
		recipientUserID int64
		title           string
		content         string
	)

	switch decisionResolution.DecisionMode {
	case BehaviorDecisionModeMerchantRecovery:
		merchant, err := q.GetMerchant(ctx, order.MerchantID)
		if err != nil {
			return nil, err
		}
		actionName = "notify_responsible_party"
		targetEntity = "merchant"
		targetID = order.MerchantID
		recipientUserID = merchant.OwnerUserID
		title = "异常订单判责通知"
		content = buildBehaviorRecoveryNotificationContent(order.OrderNo, claim.ClaimType, approvedClaimAmount(claim), decisionResolution.RecoveryAmount, decisionResolution.TraceSummary)
	case BehaviorDecisionModeRiderRecovery:
		if !riderID.Valid {
			return nil, nil
		}
		rider, err := q.GetRider(ctx, riderID.Int64)
		if err != nil {
			return nil, err
		}
		actionName = "notify_responsible_party"
		targetEntity = "rider"
		targetID = riderID.Int64
		recipientUserID = rider.UserID
		title = "异常订单判责通知"
		content = buildBehaviorRecoveryNotificationContent(order.OrderNo, claim.ClaimType, approvedClaimAmount(claim), decisionResolution.RecoveryAmount, decisionResolution.TraceSummary)
	case BehaviorDecisionModeUserRestricted:
		actionName = "notify_user_restriction"
		targetEntity = "user"
		targetID = userID
		recipientUserID = userID
		title = "账户状态变更通知"
		content = "由于您的账户存在异常索赔行为，服务已受到限制。如有疑问请联系客服。"
	default:
		return nil, nil
	}

	detail, err := json.Marshal(behaviorNotifyActionDetail{
		Action:           actionName,
		ClaimID:          claim.ID,
		TargetEntity:     targetEntity,
		TargetID:         targetID,
		RecipientUserID:  recipientUserID,
		NotificationType: "system",
		Title:            title,
		Content:          content,
		RelatedType:      "claim",
		RelatedID:        claim.ID,
		Remark:           "notification action created",
	})
	if err != nil {
		return nil, err
	}

	action, err := q.CreateBehaviorAction(ctx, CreateBehaviorActionParams{
		DecisionID:   decisionID,
		ActionType:   "notify",
		TargetEntity: targetEntity,
		Status:       "created",
		Detail:       detail,
	})
	if err != nil {
		return nil, err
	}

	return &action, nil
}

func buildBehaviorRecoveryNotificationContent(orderNo string, claimType string, approvedAmount int64, recoveryAmount int64, traceSummary string) string {
	content := fmt.Sprintf(
		"订单%s的%s异常索赔已判定由您承担。平台已向用户先行赔付%s，并已生成%s追偿单，请尽快处理。",
		orderNo,
		behaviorClaimTypeLabel(claimType),
		formatBehaviorClaimAmount(approvedAmount),
		formatBehaviorClaimAmount(recoveryAmount),
	)
	if traceSummary != "" {
		content += " 判责依据：" + traceSummary + "。"
	}
	return content
}

func approvedClaimAmount(claim Claim) int64 {
	if claim.ApprovedAmount.Valid {
		return claim.ApprovedAmount.Int64
	}
	return claim.ClaimAmount
}

func behaviorClaimTypeLabel(claimType string) string {
	switch claimType {
	case "foreign-object":
		return "异物"
	case "damage":
		return "餐损"
	case "timeout":
		return "超时"
	default:
		return "异常订单"
	}
}

func formatBehaviorClaimAmount(amount int64) string {
	return fmt.Sprintf("%.2f元", float64(amount)/100)
}

func createUserTraceSnapshots(ctx context.Context, q *Queries, decisionID int64, userID int64, windowDays int32, totalOrders int32, totalClaims int32, associations behaviorAssociationPayload, associationHits []string, netSummary GetBehaviorEffectSummaryRow) error {
	rawPayload, err := json.Marshal(behaviorTraceMetricPayload{
		CompletedOrders:   totalOrders,
		ClaimAttempts:     totalClaims,
		DistinctDevices:   associations.DistinctDevices,
		DistinctAddresses: associations.DistinctAddresses,
	})
	if err != nil {
		return err
	}
	associationPayload, err := json.Marshal(associations)
	if err != nil {
		return err
	}
	if err := createTraceSnapshot(ctx, q, decisionID, userID, "user", windowDays, totalOrders, totalClaims, associationHits, BehaviorSnapshotScopeRaw, rawPayload, associationPayload); err != nil {
		return err
	}

	netPayload, err := json.Marshal(behaviorTraceMetricPayload{
		CompletedOrders:          totalOrders,
		ClaimAttempts:            int32Value(netSummary.ClaimAttempts),
		EffectiveClaims:          int32Value(netSummary.EffectiveClaims),
		MaliciousConfirmedClaims: int32Value(netSummary.MaliciousConfirmedClaims),
		MerchantRecoveredClaims:  int32Value(netSummary.MerchantRecoveredClaims),
		RiderRecoveredClaims:     int32Value(netSummary.RiderRecoveredClaims),
		DistinctDevices:          associations.DistinctDevices,
		DistinctAddresses:        associations.DistinctAddresses,
	})
	if err != nil {
		return err
	}
	return createTraceSnapshot(ctx, q, decisionID, userID, "user", windowDays, totalOrders, userNetAbnormalCount(netSummary), associationHits, BehaviorSnapshotScopeNet, netPayload, associationPayload)
}

func createEntityTraceSnapshots(ctx context.Context, q *Queries, decisionID int64, entityID int64, actorType string, windowDays int32, associationHits []string, netSummary GetBehaviorEffectSummaryRow, now time.Time) error {
	summary, err := loadAbnormalStatsSummary(ctx, q, actorType, entityID, windowDays, now)
	if err != nil {
		return err
	}
	payload := behaviorTraceMetricPayload{CompletedOrders: summary.TotalOrders}
	if actorType == "merchant" || actorType == "rider" {
		payload.EffectiveLiabilityClaims = summary.AbnormalClaims
	}
	metricPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	associationPayload, err := json.Marshal(behaviorEmptyAssociationPayload{})
	if err != nil {
		return err
	}
	if err := createTraceSnapshot(ctx, q, decisionID, entityID, actorType, windowDays, summary.TotalOrders, summary.AbnormalClaims, associationHits, BehaviorSnapshotScopeRaw, metricPayload, associationPayload); err != nil {
		return err
	}

	netPayload, err := json.Marshal(behaviorTraceMetricPayload{
		CompletedOrders:          summary.TotalOrders,
		EffectiveLiabilityClaims: int32Value(netSummary.EffectiveLiabilityClaims),
	})
	if err != nil {
		return err
	}
	return createTraceSnapshot(ctx, q, decisionID, entityID, actorType, windowDays, summary.TotalOrders, int32Value(netSummary.EffectiveLiabilityClaims), associationHits, BehaviorSnapshotScopeNet, netPayload, associationPayload)
}

func createTraceSnapshot(ctx context.Context, q *Queries, decisionID int64, actorID int64, actorType string, windowDays int32, totalOrders int32, totalClaims int32, associationHits []string, statsScope string, metricPayload []byte, associationPayload []byte) error {
	if associationHits == nil {
		associationHits = []string{}
	}

	abnormalRate := 0.0
	if totalOrders > 0 {
		abnormalRate = float64(totalClaims) / float64(totalOrders)
	}

	rateNumeric := pgtype.Numeric{Valid: true}
	if !math.IsNaN(abnormalRate) {
		scaled := int64(math.Round(abnormalRate * 10000))
		rateNumeric.Int = big.NewInt(scaled)
		rateNumeric.Exp = -4
	}

	_, err := q.CreateBehaviorTraceSnapshot(ctx, CreateBehaviorTraceSnapshotParams{
		DecisionID:         decisionID,
		WindowDays:         windowDays,
		AbnormalCount:      totalClaims,
		TotalCount:         totalOrders,
		AbnormalRate:       rateNumeric,
		AssociationHits:    associationHits,
		ActorType:          pgtype.Text{String: actorType, Valid: actorType != ""},
		ActorID:            pgtype.Int8{Int64: actorID, Valid: actorID > 0},
		WindowKey:          pgtype.Text{String: windowKeyFromDays(windowDays), Valid: true},
		StatsScope:         pgtype.Text{String: statsScope, Valid: true},
		MetricPayload:      metricPayload,
		AssociationPayload: associationPayload,
		SnapshotVersion:    BehaviorSnapshotVersionV2,
	})
	return err
}

func loadBehaviorEffectSummary(ctx context.Context, q *Queries, entityType string, entityID int64, windowDays int32, now time.Time) (GetBehaviorEffectSummaryRow, error) {
	startAt := now.AddDate(0, 0, -int(windowDays))
	return q.GetBehaviorEffectSummary(ctx, GetBehaviorEffectSummaryParams{
		EntityType: entityType,
		EntityID:   entityID,
		StartAt:    startAt,
		EndAt:      now,
	})
}

func loadUserAssociationPayload(ctx context.Context, q *Queries, userID int64, deviceFingerprint string, addressID pgtype.Int8) (behaviorAssociationPayload, behaviorDecisionGraphHits, error) {
	association := behaviorAssociationPayload{
		HasDeviceFingerprint: deviceFingerprint != "",
		HasAddressID:         addressID.Valid,
	}
	graphHits := behaviorDecisionGraphHits{}

	devices, err := q.GetDevicesByUserID(ctx, userID)
	if err != nil {
		return association, graphHits, err
	}
	deviceSet := make(map[string]struct{}, len(devices))
	for _, device := range devices {
		if device.DeviceID != "" {
			deviceSet[device.DeviceID] = struct{}{}
		}
	}
	association.DistinctDevices = int32(len(deviceSet))

	addresses, err := q.ListUserAddresses(ctx, userID)
	if err != nil {
		return association, graphHits, err
	}
	association.DistinctAddresses = int32(len(addresses))

	if deviceFingerprint != "" {
		userIDs, err := q.GetUsersByDeviceFingerprint(ctx, pgtype.Text{String: deviceFingerprint, Valid: true})
		if err != nil {
			return association, graphHits, err
		}
		association.SharedDeviceUsers = int32(len(userIDs))
		association.SharedDeviceOtherUsers = countOtherUsers(userIDs, userID)
		graphHits.SharedDeviceUsers = association.SharedDeviceUsers
		graphHits.SharedDeviceOtherUsers = association.SharedDeviceOtherUsers
		if association.SharedDeviceOtherUsers > 0 {
			graphHits.HitCodes = append(graphHits.HitCodes, "shared_device_fingerprint")
		}
	}

	if addressID.Valid {
		rows, err := q.GetUsersByAddressID(ctx, addressID.Int64)
		if err != nil {
			return association, graphHits, err
		}
		association.SharedAddressUsers = int32(len(rows))
		graphHits.SharedAddressUsers = association.SharedAddressUsers
		for _, row := range rows {
			association.SharedAddressOrders += row.OrderCount
			if row.UserID != userID {
				association.SharedAddressOtherUsers++
			}
		}
		graphHits.SharedAddressOtherUsers = association.SharedAddressOtherUsers
		if association.SharedAddressOtherUsers > 0 {
			graphHits.HitCodes = append(graphHits.HitCodes, "shared_address_id")
		}
	}

	return association, graphHits, nil
}

func buildResponsibilityFacts(order Order, delivery *Delivery, responsibilityDomain string) (behaviorResponsibilityFacts, []string) {
	facts := behaviorResponsibilityFacts{
		OrderType:         order.OrderType,
		OrderStatus:       order.Status,
		FulfillmentStatus: order.FulfillmentStatus,
		AddressPresent:    order.AddressID.Valid,
	}
	if delivery != nil {
		facts.DeliveryExists = true
		facts.RiderAssigned = delivery.RiderID.Valid
		facts.CourierAccepted = order.CourierAcceptAt.Valid || delivery.AssignedAt.Valid
		facts.PickupConfirmed = order.PickedAt.Valid || delivery.PickedAt.Valid || isPickupConfirmedByStatus(order.Status, delivery.Status)
		facts.DeliveryCompleted = delivery.DeliveredAt.Valid || delivery.CompletedAt.Valid
		facts.DeliveryStatus = delivery.Status
	}

	missingFacts := make([]string, 0)
	if responsibilityDomain == BehaviorResponsibilityDomainRider {
		if order.OrderType != "takeout" {
			missingFacts = append(missingFacts, "not_takeout_order")
		}
		if delivery == nil {
			missingFacts = append(missingFacts, "missing_delivery_chain")
		} else {
			if !facts.RiderAssigned {
				missingFacts = append(missingFacts, "missing_rider_assignment")
			}
			if !facts.PickupConfirmed {
				missingFacts = append(missingFacts, "missing_pickup_confirmation")
			}
			if facts.DeliveryStatus == "" {
				missingFacts = append(missingFacts, "missing_delivery_status")
			}
		}
	}
	facts.MissingCriticalFacts = missingFacts
	return facts, missingFacts
}

func resolveBehaviorAddressID(order Order, explicitAddressID *int64) pgtype.Int8 {
	if explicitAddressID != nil {
		return pgtype.Int8{Int64: *explicitAddressID, Valid: true}
	}
	return order.AddressID
}

func countOtherUsers(userIDs []int64, targetUserID int64) int32 {
	otherUsers := int32(0)
	for _, candidateUserID := range userIDs {
		if candidateUserID != targetUserID {
			otherUsers++
		}
	}
	return otherUsers
}

func userNetAbnormalCount(summary GetBehaviorEffectSummaryRow) int32 {
	return int32Value(summary.EffectiveClaims)
}

func int32Value(value int64) int32 {
	return int32(value)
}

func isPickupConfirmedByStatus(orderStatus string, deliveryStatus string) bool {
	if orderStatus == OrderStatusPicked || orderStatus == OrderStatusDelivering || orderStatus == OrderStatusRiderDelivered || orderStatus == OrderStatusUserDelivered || orderStatus == OrderStatusCompleted {
		return true
	}
	return deliveryStatus == "picked" || deliveryStatus == "delivering" || deliveryStatus == "delivered" || deliveryStatus == "completed"
}

func loadAbnormalStatsSummary(ctx context.Context, q *Queries, entityType string, entityID int64, windowDays int32, now time.Time) (GetAbnormalStatsSummaryRow, error) {
	startDate := pgtype.Date{Time: now.AddDate(0, 0, -int(windowDays)), Valid: true}
	endDate := pgtype.Date{Time: now, Valid: true}
	return q.GetAbnormalStatsSummary(ctx, GetAbnormalStatsSummaryParams{
		EntityType: entityType,
		EntityID:   entityID,
		StatDate:   startDate,
		StatDate_2: endDate,
	})
}

func createDecisionEffects(ctx context.Context, q *Queries, decisionID int64, userID int64, merchantID int64, riderID pgtype.Int8, decisionMode string, approvedAmount *int64) error {
	if _, err := q.CreateBehaviorDecisionEffect(ctx, CreateBehaviorDecisionEffectParams{
		DecisionID: decisionID,
		EntityType: "user",
		EntityID:   userID,
		MetricKey:  "claim_attempts",
		DeltaValue: 1,
		Status:     BehaviorDecisionEffectStatusApplied,
		Note:       pgtype.Text{String: "phase1 dual-write claim attempt", Valid: true},
	}); err != nil {
		return err
	}

	if approvedAmount != nil && *approvedAmount > 0 {
		if _, err := q.CreateBehaviorDecisionEffect(ctx, CreateBehaviorDecisionEffectParams{
			DecisionID: decisionID,
			EntityType: "user",
			EntityID:   userID,
			MetricKey:  "effective_claims",
			DeltaValue: 1,
			Status:     BehaviorDecisionEffectStatusApplied,
			Note:       pgtype.Text{String: "phase1 dual-write effective claim", Valid: true},
		}); err != nil {
			return err
		}
	}

	switch decisionMode {
	case BehaviorDecisionModeMerchantRecovery:
		if _, err := q.CreateBehaviorDecisionEffect(ctx, CreateBehaviorDecisionEffectParams{
			DecisionID: decisionID,
			EntityType: "user",
			EntityID:   userID,
			MetricKey:  "merchant_recovered_claims",
			DeltaValue: 1,
			Status:     BehaviorDecisionEffectStatusApplied,
			Note:       pgtype.Text{String: "phase1 dual-write merchant recovery", Valid: true},
		}); err != nil {
			return err
		}
		if _, err := q.CreateBehaviorDecisionEffect(ctx, CreateBehaviorDecisionEffectParams{
			DecisionID: decisionID,
			EntityType: "merchant",
			EntityID:   merchantID,
			MetricKey:  "effective_liability_claims",
			DeltaValue: 1,
			Status:     BehaviorDecisionEffectStatusApplied,
			Note:       pgtype.Text{String: "phase1 dual-write merchant liability", Valid: true},
		}); err != nil {
			return err
		}
	case BehaviorDecisionModeRiderRecovery:
		if _, err := q.CreateBehaviorDecisionEffect(ctx, CreateBehaviorDecisionEffectParams{
			DecisionID: decisionID,
			EntityType: "user",
			EntityID:   userID,
			MetricKey:  "rider_recovered_claims",
			DeltaValue: 1,
			Status:     BehaviorDecisionEffectStatusApplied,
			Note:       pgtype.Text{String: "phase1 dual-write rider recovery", Valid: true},
		}); err != nil {
			return err
		}
		if riderID.Valid {
			if _, err := q.CreateBehaviorDecisionEffect(ctx, CreateBehaviorDecisionEffectParams{
				DecisionID: decisionID,
				EntityType: "rider",
				EntityID:   riderID.Int64,
				MetricKey:  "effective_liability_claims",
				DeltaValue: 1,
				Status:     BehaviorDecisionEffectStatusApplied,
				Note:       pgtype.Text{String: "phase1 dual-write rider liability", Valid: true},
			}); err != nil {
				return err
			}
		}
	case BehaviorDecisionModeUserRestricted:
		if _, err := q.CreateBehaviorDecisionEffect(ctx, CreateBehaviorDecisionEffectParams{
			DecisionID: decisionID,
			EntityType: "user",
			EntityID:   userID,
			MetricKey:  "malicious_confirmed_claims",
			DeltaValue: 1,
			Status:     BehaviorDecisionEffectStatusApplied,
			Note:       pgtype.Text{String: "phase1 dual-write user restricted", Valid: true},
		}); err != nil {
			return err
		}
	}

	return nil
}

func deriveBehaviorDecisionMode(responsibleParty string, recoveryTarget string, createRecovery bool) string {
	if responsibleParty == "user" {
		return BehaviorDecisionModeUserRestricted
	}
	if createRecovery && recoveryTarget == "merchant" {
		return BehaviorDecisionModeMerchantRecovery
	}
	if createRecovery && recoveryTarget == "rider" {
		return BehaviorDecisionModeRiderRecovery
	}
	if responsibleParty == "merchant" {
		return BehaviorDecisionModeMerchantRecovery
	}
	if responsibleParty == "rider" {
		return BehaviorDecisionModeRiderRecovery
	}
	return ""
}

func deriveBehaviorDecisionResolution(arg CreateClaimWithBehaviorTxParams) behaviorDecisionResolution {
	decisionMode := deriveBehaviorDecisionMode(arg.ResponsibleParty, arg.RecoveryTarget, arg.CreateRecovery)
	approvalType := normalizeBehaviorApprovalType(arg.ApprovalType)
	return behaviorDecisionResolution{
		ResponsibleParty:     arg.ResponsibleParty,
		CompensationSource:   arg.CompensationSource,
		ApprovalType:         approvalType,
		AutoApprovalReason:   arg.AutoApprovalReason,
		TraceSummary:         arg.TraceSummary,
		DecisionMode:         decisionMode,
		ResponsibilityDomain: deriveBehaviorResponsibilityDomain(decisionMode),
		PayoutMode:           deriveBehaviorPayoutMode(arg.Status, approvalType, arg.ApprovedAmount),
		FallbackReason:       deriveBehaviorFallbackReason(decisionMode),
		RestrictionReason:    deriveBehaviorRestrictionReason(decisionMode),
		CreateRecovery:       arg.CreateRecovery,
		RecoveryTarget:       arg.RecoveryTarget,
		RecoveryAmount:       arg.RecoveryAmount,
	}
}

func promoteBehaviorUserRestricted(status string, approvedAmount *int64, resolution behaviorDecisionResolution, associations behaviorAssociationPayload, userSummary GetBehaviorEffectSummaryRow) behaviorDecisionResolution {
	if resolution.DecisionMode == BehaviorDecisionModeUserRestricted {
		return resolution
	}

	if _, shouldPromote := shouldPromoteBehaviorUserRestricted(associations, userSummary); !shouldPromote {
		return resolution
	}

	message := "您的账号因索赔行为异常已被限制服务；若确认继续索赔，平台将先行赔付并停止后续服务。"
	resolution.ResponsibleParty = "user"
	resolution.CompensationSource = "platform"
	resolution.ApprovalType = "auto"
	resolution.AutoApprovalReason = message
	resolution.TraceSummary = message
	resolution.DecisionMode = BehaviorDecisionModeUserRestricted
	resolution.ResponsibilityDomain = BehaviorResponsibilityDomainUser
	resolution.PayoutMode = deriveBehaviorPayoutMode(status, resolution.ApprovalType, approvedAmount)
	resolution.FallbackReason = ""
	resolution.RestrictionReason = deriveBehaviorRestrictionReason(resolution.DecisionMode)
	resolution.CreateRecovery = false
	resolution.RecoveryTarget = ""
	resolution.RecoveryAmount = 0

	return resolution
}

func normalizeBehaviorApprovalType(approvalType string) string {
	return approvalType
}

func shouldPromoteBehaviorUserRestricted(associations behaviorAssociationPayload, userSummary GetBehaviorEffectSummaryRow) (string, bool) {
	return strongBehaviorUserRiskConfirmationReason(associations, userSummary)
}

func strongBehaviorUserRiskConfirmationReason(associations behaviorAssociationPayload, userSummary GetBehaviorEffectSummaryRow) (string, bool) {
	if userSummary.MaliciousConfirmedClaims > 0 {
		return "historical_malicious_confirmed", true
	}
	if (associations.SharedDeviceOtherUsers > 0 || associations.SharedAddressOtherUsers > 0) && userNetAbnormalCount(userSummary) >= behaviorDecisionRepeatRiskThreshold {
		return "shared_graph_repeat_claim_pattern", true
	}
	return "", false
}

func normalizeBehaviorReasonCodes(reasonCodes []string, resolution behaviorDecisionResolution) []string {
	normalized := make([]string, 0, len(reasonCodes)+3)
	appendUniqueReasonCode := func(code string) {
		if code == "" {
			return
		}
		for _, existing := range normalized {
			if existing == code {
				return
			}
		}
		normalized = append(normalized, code)
	}

	appendUniqueReasonCode(resolution.DecisionMode)
	appendUniqueReasonCode(resolution.FallbackReason)
	appendUniqueReasonCode(resolution.RestrictionReason)

	for _, code := range reasonCodes {
		if code == "" {
			continue
		}
		if isBehaviorDecisionModeCode(code) && code != resolution.DecisionMode {
			continue
		}
		appendUniqueReasonCode(code)
	}

	return normalized
}

func isBehaviorDecisionModeCode(code string) bool {
	switch code {
	case BehaviorDecisionModeMerchantRecovery, BehaviorDecisionModeRiderRecovery, BehaviorDecisionModeUserRestricted:
		return true
	default:
		return false
	}
}

func deriveBehaviorResponsibilityDomain(decisionMode string) string {
	switch decisionMode {
	case BehaviorDecisionModeMerchantRecovery:
		return BehaviorResponsibilityDomainMerchant
	case BehaviorDecisionModeRiderRecovery:
		return BehaviorResponsibilityDomainRider
	case BehaviorDecisionModeUserRestricted:
		return BehaviorResponsibilityDomainUser
	default:
		return ""
	}
}

func deriveBehaviorPayoutMode(status string, approvalType string, approvedAmount *int64) string {
	if status == "rejected" || approvedAmount == nil || *approvedAmount <= 0 {
		return BehaviorPayoutModeRejected
	}
	if approvalType == "instant" {
		return BehaviorPayoutModeInstantPaid
	}
	return BehaviorPayoutModeLimitedPaid
}

func deriveBehaviorFallbackReason(_ string) string {
	return ""
}

func deriveBehaviorRestrictionReason(decisionMode string) string {
	if decisionMode == BehaviorDecisionModeUserRestricted {
		return "confirmed_high_user_risk"
	}
	return ""
}

func recoveryBasisFromDecisionMode(decisionMode string) string {
	switch decisionMode {
	case BehaviorDecisionModeMerchantRecovery:
		return ClaimRecoveryBasisMerchantRecovery
	case BehaviorDecisionModeRiderRecovery:
		return ClaimRecoveryBasisRiderRecovery
	default:
		return ""
	}
}

func windowKeyFromDays(windowDays int32) string {
	switch windowDays {
	case 7:
		return BehaviorSnapshotWindowKey7d
	case 30:
		return BehaviorSnapshotWindowKey30d
	default:
		return ""
	}
}

func approvedAmountValue(approvedAmount *int64) int64 {
	if approvedAmount == nil {
		return 0
	}
	return *approvedAmount
}
