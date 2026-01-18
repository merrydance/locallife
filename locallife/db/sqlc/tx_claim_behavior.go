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
	EvidenceURLs       []string
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
			OrderID:        arg.OrderID,
			UserID:         arg.UserID,
			ClaimType:      arg.ClaimType,
			Description:    arg.Description,
			EvidenceUrls:   arg.EvidenceURLs,
			ClaimAmount:    arg.ClaimAmount,
			Status:         arg.Status,
			IsMalicious:    false,
			LookbackResult: arg.LookbackResult,
			CreatedAt:      time.Now(),
			ApprovalType:   pgtype.Text{String: arg.ApprovalType, Valid: arg.ApprovalType != ""},
			AutoApprovalReason: pgtype.Text{String: arg.AutoApprovalReason, Valid: arg.AutoApprovalReason != ""},
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

		evidenceItems := buildBehaviorEvidencePayloads(arg)
		for _, item := range evidenceItems {
			_, err = q.CreateBehaviorEvidence(ctx, CreateBehaviorEvidenceParams{
				DecisionID:   decision.ID,
				EvidenceType: item.EvidenceType,
				Payload:      item.Payload,
			})
			if err != nil {
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

type behaviorEvidencePayload struct {
	EvidenceType string
	Payload      []byte
}

func buildBehaviorEvidencePayloads(arg CreateClaimWithBehaviorTxParams) []behaviorEvidencePayload {
	items := make([]behaviorEvidencePayload, 0, 6)

	if len(arg.EvidenceURLs) > 0 {
		payload, _ := json.Marshal(map[string]any{"urls": arg.EvidenceURLs})
		items = append(items, behaviorEvidencePayload{EvidenceType: "image", Payload: payload})
	}
	if arg.DeviceID != "" {
		payload, _ := json.Marshal(map[string]any{"device_id": arg.DeviceID})
		items = append(items, behaviorEvidencePayload{EvidenceType: "device", Payload: payload})
	}
	if arg.DeviceFingerprint != "" {
		payload, _ := json.Marshal(map[string]any{"device_fingerprint": arg.DeviceFingerprint})
		items = append(items, behaviorEvidencePayload{EvidenceType: "device", Payload: payload})
	}
	if arg.IPAddress != "" {
		payload, _ := json.Marshal(map[string]any{"ip": arg.IPAddress})
		items = append(items, behaviorEvidencePayload{EvidenceType: "ip", Payload: payload})
	}
	if arg.UserAgent != "" {
		payload, _ := json.Marshal(map[string]any{"user_agent": arg.UserAgent})
		items = append(items, behaviorEvidencePayload{EvidenceType: "user_agent", Payload: payload})
	}
	if arg.AddressID != nil {
		payload, _ := json.Marshal(map[string]any{"address_id": *arg.AddressID})
		items = append(items, behaviorEvidencePayload{EvidenceType: "address", Payload: payload})
	}

	return items
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
