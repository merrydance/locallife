package db

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

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
}

type CreateClaimWithBehaviorTxResult struct {
	Claim            Claim
	BehaviorDecision BehaviorDecision
}

func (store *SQLStore) CreateClaimWithBehaviorTx(ctx context.Context, arg CreateClaimWithBehaviorTxParams) (CreateClaimWithBehaviorTxResult, error) {
	var result CreateClaimWithBehaviorTxResult

	err := store.execTx(ctx, func(q *Queries) error {
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
		if order.OrderType == "takeout" {
			delivery, err := q.GetDeliveryByOrderID(ctx, order.ID)
			if err == nil && delivery.RiderID.Valid {
				riderID = pgtype.Int8{Int64: delivery.RiderID.Int64, Valid: true}
			}
		}

		claimParams := CreateClaimParams{
			OrderID:            arg.OrderID,
			UserID:             arg.UserID,
			ClaimType:          arg.ClaimType,
			Description:        arg.Description,
			ClaimAmount:        arg.ClaimAmount,
			Status:             arg.Status,
			IsMalicious:        false,
			LookbackResult:     arg.LookbackResult,
			CreatedAt:          time.Now(),
			ApprovalType:       pgtype.Text{String: arg.ApprovalType, Valid: arg.ApprovalType != ""},
			AutoApprovalReason: pgtype.Text{String: arg.AutoApprovalReason, Valid: arg.AutoApprovalReason != ""},
			DecisionVersion:    pgtype.Text{String: arg.DecisionVersion, Valid: arg.DecisionVersion != ""},
			DecisionReason:     pgtype.Text{String: arg.TraceSummary, Valid: arg.TraceSummary != ""},
		}
		if arg.ApprovedAmount != nil {
			claimParams.ApprovedAmount = pgtype.Int8{Int64: *arg.ApprovedAmount, Valid: true}
		}

		claim, err := q.CreateClaim(ctx, claimParams)
		if err != nil {
			return err
		}

		decision, err := q.CreateBehaviorDecision(ctx, CreateBehaviorDecisionParams{
			OrderID:            arg.OrderID,
			UserID:             pgtype.Int8{Int64: arg.UserID, Valid: true},
			MerchantID:         pgtype.Int8{Int64: order.MerchantID, Valid: true},
			RiderID:            riderID,
			DecisionVersion:    arg.DecisionVersion,
			ReasonCodes:        arg.ReasonCodes,
			ResponsibleParty:   arg.ResponsibleParty,
			CompensationSource: arg.CompensationSource,
			DecisionStatus:     "decided",
			TraceSummary:       pgtype.Text{String: arg.TraceSummary, Valid: arg.TraceSummary != ""},
		})
		if err != nil {
			return err
		}

		// 记录行为动作（赔付动作）
		if arg.ApprovedAmount != nil && *arg.ApprovedAmount > 0 {
			detail, _ := json.Marshal(map[string]any{
				"action":   "platform_payout",
				"claim_id": claim.ID,
				"amount":   *arg.ApprovedAmount,
			})
			if _, err := q.CreateBehaviorAction(ctx, CreateBehaviorActionParams{
				DecisionID:   decision.ID,
				ActionType:   "refund",
				TargetEntity: "user",
				Status:       "created",
				Detail:       detail,
			}); err != nil {
				return err
			}
		}

		stats, err := q.GetUserClaimWindowStats(ctx, arg.UserID)
		if err != nil {
			return err
		}

		if err := createTraceSnapshot(ctx, q, decision.ID, 7, stats.TakeoutOrders7d, stats.Claims7d); err != nil {
			return err
		}
		if err := createTraceSnapshot(ctx, q, decision.ID, 30, stats.TakeoutOrders30d, stats.Claims30d); err != nil {
			return err
		}

		result.Claim = claim
		result.BehaviorDecision = decision
		return nil
	})

	return result, err
}
func createTraceSnapshot(ctx context.Context, q *Queries, decisionID int64, windowDays int32, totalOrders int32, totalClaims int32) error {
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
		DecisionID:      decisionID,
		WindowDays:      windowDays,
		AbnormalCount:   totalClaims,
		TotalCount:      totalOrders,
		AbnormalRate:    rateNumeric,
		AssociationHits: []string{},
	})
	return err
}
