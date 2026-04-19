package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrFoodSafetyCaseAlreadyResolved            = errors.New("food safety case is already resolved")
	ErrFoodSafetyCaseMissingInvestigationReport = errors.New("food safety case requires an investigation report before resolution")
)

const foodSafetyCircuitBreakReasonPrefix = "同商户1小时内多名顾客食安举报触发熔断"
const foodSafetyPausedOrderStatusHint = "商户因食安事件暂停履约，请等待商家或平台联系"

const (
	foodSafetyCaseOpenConstraintName     = "uq_food_safety_cases_merchant_open"
	foodSafetyIncidentOpenConstraintName = "uq_food_safety_incidents_order_user_open"
)

type ReportFoodSafetyIncidentTxParams struct {
	CreateFoodSafetyIncidentParams CreateFoodSafetyIncidentParams
	ProductKey                     string
	ProductLabel                   string
	ShouldCircuitBreak             bool
	CircuitBreakReason             string
	CircuitBreakDurationHours      int
}

type ReportFoodSafetyIncidentTxResult struct {
	Incident               FoodSafetyIncident
	Case                   *FoodSafetyCase
	AffectedReservations   []TableReservation
	AffectedTakeoutOrders  []Order
	OpenedNewCase          bool
	ReusedExistingIncident bool
}

type ResolveFoodSafetyCaseTxParams struct {
	CaseID                      int64
	RegionID                    int64
	InvestigationReport         string
	MerchantRectificationReport string
	Resolution                  string
}

type ResolveFoodSafetyCaseTxResult struct {
	Case FoodSafetyCase
}

func (store *SQLStore) ReportFoodSafetyIncidentTx(ctx context.Context, arg ReportFoodSafetyIncidentTxParams) (ReportFoodSafetyIncidentTxResult, error) {
	var result ReportFoodSafetyIncidentTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		createArg := arg.CreateFoodSafetyIncidentParams
		createArg.PrimaryProductKey = strings.TrimSpace(arg.ProductKey)
		createArg.PrimaryProductLabel = strings.TrimSpace(arg.ProductLabel)

		existingIncident, err := q.GetOpenFoodSafetyIncidentByOrderAndUser(ctx, GetOpenFoodSafetyIncidentByOrderAndUserParams{
			OrderID: createArg.OrderID,
			UserID:  createArg.UserID,
		})
		if err == nil {
			result.Incident = foodSafetyIncidentFromOpenRow(existingIncident)
			result.ReusedExistingIncident = true
			if existingIncident.CaseID.Valid {
				caseRecord, caseErr := q.GetFoodSafetyCase(ctx, existingIncident.CaseID.Int64)
				if caseErr != nil && !errors.Is(caseErr, ErrRecordNotFound) {
					return fmt.Errorf("get existing food safety case: %w", caseErr)
				}
				if caseErr == nil {
					result.Case = &caseRecord
				}
			}
			return nil
		}
		if !errors.Is(err, ErrRecordNotFound) {
			return fmt.Errorf("get open food safety incident by order and user: %w", err)
		}

		incident, err := q.CreateFoodSafetyIncident(ctx, createArg)
		if err != nil {
			if isFoodSafetyUniqueConstraint(err, foodSafetyIncidentOpenConstraintName) {
				existingIncident, getExistingErr := q.GetOpenFoodSafetyIncidentByOrderAndUser(ctx, GetOpenFoodSafetyIncidentByOrderAndUserParams{
					OrderID: createArg.OrderID,
					UserID:  createArg.UserID,
				})
				if getExistingErr != nil {
					return fmt.Errorf("get duplicated food safety incident after unique violation: %w", getExistingErr)
				}
				result.Incident = foodSafetyIncidentFromOpenRow(existingIncident)
				result.ReusedExistingIncident = true
				if existingIncident.CaseID.Valid {
					caseRecord, caseErr := q.GetFoodSafetyCase(ctx, existingIncident.CaseID.Int64)
					if caseErr != nil && !errors.Is(caseErr, ErrRecordNotFound) {
						return fmt.Errorf("get duplicated food safety case: %w", caseErr)
					}
					if caseErr == nil {
						result.Case = &caseRecord
					}
				}
				return nil
			}
			return fmt.Errorf("create food safety incident: %w", err)
		}
		result.Incident = incident

		if !arg.ShouldCircuitBreak {
			return nil
		}

		merchant, err := q.GetMerchant(ctx, createArg.MerchantID)
		if err != nil {
			return fmt.Errorf("get merchant for food safety case: %w", err)
		}

		caseRecord, err := q.GetOpenFoodSafetyCaseByMerchant(ctx, merchant.ID)
		if err != nil {
			if !errors.Is(err, ErrRecordNotFound) {
				return fmt.Errorf("get open food safety case: %w", err)
			}

			result.OpenedNewCase = true
			caseRecord, err = q.CreateFoodSafetyCase(ctx, CreateFoodSafetyCaseParams{
				MerchantID:          merchant.ID,
				RegionID:            merchant.RegionID,
				PrimaryProductKey:   createArg.PrimaryProductKey,
				PrimaryProductLabel: createArg.PrimaryProductLabel,
				Status:              "merchant-suspended",
				TriggerReason:       arg.CircuitBreakReason,
				SuspendedAt:         time.Now(),
			})
			if err != nil {
				if isFoodSafetyUniqueConstraint(err, foodSafetyCaseOpenConstraintName) {
					result.OpenedNewCase = false
					caseRecord, err = q.GetOpenFoodSafetyCaseByMerchant(ctx, merchant.ID)
					if err != nil {
						return fmt.Errorf("get open food safety case after unique violation: %w", err)
					}
				} else {
					return fmt.Errorf("create food safety case: %w", err)
				}
			}
		}
		result.Case = &caseRecord

		incidentIDs, err := collectFoodSafetyClusterIncidentIDs(ctx, q, merchant.ID)
		if err != nil {
			return err
		}
		incidentIDs = appendUniqueFoodSafetyIncidentID(incidentIDs, incident.ID)

		if _, err := q.LinkFoodSafetyIncidentsToCase(ctx, LinkFoodSafetyIncidentsToCaseParams{
			IncidentIds: incidentIDs,
			CaseID:      pgtype.Int8{Int64: caseRecord.ID, Valid: true},
			Status:      "merchant-suspended",
		}); err != nil {
			return fmt.Errorf("link incidents to food safety case: %w", err)
		}

		suspendUntil := time.Now().Add(time.Duration(arg.CircuitBreakDurationHours) * time.Hour)
		if err := q.SuspendMerchant(ctx, SuspendMerchantParams{
			MerchantID: merchant.ID,
			SuspendReason: pgtype.Text{
				String: arg.CircuitBreakReason,
				Valid:  strings.TrimSpace(arg.CircuitBreakReason) != "",
			},
			SuspendUntil: pgtype.Timestamptz{Time: suspendUntil, Valid: true},
		}); err != nil {
			return fmt.Errorf("suspend merchant for food safety case: %w", err)
		}

		if err := q.SuspendMerchantTakeout(ctx, SuspendMerchantTakeoutParams{
			MerchantID: merchant.ID,
			TakeoutSuspendReason: pgtype.Text{
				String: arg.CircuitBreakReason,
				Valid:  strings.TrimSpace(arg.CircuitBreakReason) != "",
			},
			TakeoutSuspendUntil: pgtype.Timestamptz{Time: suspendUntil, Valid: true},
		}); err != nil {
			return fmt.Errorf("suspend merchant takeout for food safety case: %w", err)
		}

		if _, err := q.UpdateMerchantStatus(ctx, UpdateMerchantStatusParams{
			ID:     merchant.ID,
			Status: "suspended",
		}); err != nil {
			return fmt.Errorf("update merchant status to suspended for food safety case: %w", err)
		}

		if !result.OpenedNewCase {
			return nil
		}

		result.AffectedReservations, err = q.ListMerchantFutureReservationsForFoodSafetyAlert(ctx, merchant.ID)
		if err != nil {
			return fmt.Errorf("list merchant future reservations for food safety alert: %w", err)
		}

		result.AffectedTakeoutOrders, err = q.ListMerchantActiveTakeoutOrdersForFoodSafety(ctx, merchant.ID)
		if err != nil {
			return fmt.Errorf("list merchant active takeout orders for food safety: %w", err)
		}

		for _, activeOrder := range result.AffectedTakeoutOrders {
			if _, err := q.UpdateOrderFoodSafetyPauseState(ctx, UpdateOrderFoodSafetyPauseStateParams{
				ID:             activeOrder.ID,
				ExceptionState: pgtype.Text{String: OrderExceptionStateFoodSafetyPaused, Valid: true},
				StatusHint:     pgtype.Text{String: foodSafetyPausedOrderStatusHint, Valid: true},
				ClaimChannel:   pgtype.Text{String: ClaimChannelFoodSafety, Valid: true},
			}); err != nil {
				return fmt.Errorf("pause active takeout order for food safety: %w", err)
			}

			if _, err := q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
				OrderID:      activeOrder.ID,
				FromStatus:   pgtype.Text{String: activeOrder.Status, Valid: true},
				ToStatus:     activeOrder.Status,
				OperatorType: pgtype.Text{String: "system", Valid: true},
				Notes:        pgtype.Text{String: "食安事件触发暂停履约", Valid: true},
			}); err != nil {
				return fmt.Errorf("create food safety pause order log: %w", err)
			}
		}

		return nil
	})

	return result, err
}

func (store *SQLStore) ResolveFoodSafetyCaseTx(ctx context.Context, arg ResolveFoodSafetyCaseTxParams) (ResolveFoodSafetyCaseTxResult, error) {
	var result ResolveFoodSafetyCaseTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		caseRecord, err := q.GetFoodSafetyCaseForUpdate(ctx, arg.CaseID)
		if err != nil {
			return fmt.Errorf("get food safety case for update: %w", err)
		}
		if caseRecord.RegionID != arg.RegionID {
			return fmt.Errorf("food safety case %d does not belong to region %d", arg.CaseID, arg.RegionID)
		}
		if caseRecord.Status == "resolved" {
			return ErrFoodSafetyCaseAlreadyResolved
		}

		investigationReport := strings.TrimSpace(arg.InvestigationReport)
		if investigationReport == "" {
			investigationReport = strings.TrimSpace(caseRecord.InvestigationReport.String)
		}
		if investigationReport == "" {
			return ErrFoodSafetyCaseMissingInvestigationReport
		}

		resolvedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		result.Case, err = q.ResolveFoodSafetyCase(ctx, ResolveFoodSafetyCaseParams{
			ID: arg.CaseID,
			InvestigationReport: pgtype.Text{
				String: investigationReport,
				Valid:  true,
			},
			MerchantRectificationReport: pgtype.Text{
				String: strings.TrimSpace(arg.MerchantRectificationReport),
				Valid:  true,
			},
			Resolution: pgtype.Text{
				String: strings.TrimSpace(arg.Resolution),
				Valid:  true,
			},
			ResolvedAt: resolvedAt,
		})
		if err != nil {
			return fmt.Errorf("resolve food safety case: %w", err)
		}

		if err := q.ResolveFoodSafetyIncidentsByCase(ctx, ResolveFoodSafetyIncidentsByCaseParams{
			CaseID: pgtype.Int8{Int64: arg.CaseID, Valid: true},
			InvestigationReport: pgtype.Text{
				String: investigationReport,
				Valid:  true,
			},
			Resolution: pgtype.Text{
				String: strings.TrimSpace(arg.Resolution),
				Valid:  true,
			},
			ResolvedAt: resolvedAt,
		}); err != nil {
			return fmt.Errorf("resolve food safety incidents by case: %w", err)
		}

		merchant, err := q.GetMerchant(ctx, caseRecord.MerchantID)
		if err != nil {
			return fmt.Errorf("get merchant after food safety resolution: %w", err)
		}

		merchantProfile, err := q.GetMerchantProfile(ctx, caseRecord.MerchantID)
		if err != nil {
			return fmt.Errorf("get merchant profile after food safety resolution: %w", err)
		}

		releasedMerchantSuspension := false
		if merchantProfile.IsSuspended && isFoodSafetySuspendReasonText(merchantProfile.SuspendReason) {
			if err := q.UnsuspendMerchant(ctx, caseRecord.MerchantID); err != nil {
				return fmt.Errorf("unsuspend merchant after food safety resolution: %w", err)
			}
			releasedMerchantSuspension = true
		}

		if merchantProfile.IsTakeoutSuspended && isFoodSafetySuspendReasonText(merchantProfile.TakeoutSuspendReason) {
			if err := q.UnsuspendMerchantTakeout(ctx, caseRecord.MerchantID); err != nil {
				return fmt.Errorf("unsuspend merchant takeout after food safety resolution: %w", err)
			}

			if _, err := q.ClearMerchantFoodSafetyPausedOrders(ctx, caseRecord.MerchantID); err != nil {
				return fmt.Errorf("clear paused takeout orders after food safety resolution: %w", err)
			}
		}

		if releasedMerchantSuspension && merchant.Status == "suspended" {
			if _, err := q.UpdateMerchantStatus(ctx, UpdateMerchantStatusParams{
				ID:     caseRecord.MerchantID,
				Status: "active",
			}); err != nil {
				return fmt.Errorf("update merchant status to active after food safety resolution: %w", err)
			}
		}

		return nil
	})

	return result, err
}

func collectFoodSafetyClusterIncidentIDs(ctx context.Context, q *Queries, merchantID int64) ([]int64, error) {
	incidents, err := q.GetMerchantRecentFoodSafetyReports(ctx, merchantID)
	if err != nil {
		return nil, fmt.Errorf("list recent food safety incidents for cluster: %w", err)
	}

	ids := make([]int64, 0, len(incidents))
	for _, incident := range incidents {
		ids = appendUniqueFoodSafetyIncidentID(ids, incident.ID)
	}

	return ids, nil
}

func appendUniqueFoodSafetyIncidentID(ids []int64, candidate int64) []int64 {
	for _, id := range ids {
		if id == candidate {
			return ids
		}
	}
	return append(ids, candidate)
}

func foodSafetyIncidentFromOpenRow(row GetOpenFoodSafetyIncidentByOrderAndUserRow) FoodSafetyIncident {
	return FoodSafetyIncident{
		ID:                  row.ID,
		OrderID:             row.OrderID,
		MerchantID:          row.MerchantID,
		UserID:              row.UserID,
		IncidentType:        row.IncidentType,
		Description:         row.Description,
		OrderSnapshot:       row.OrderSnapshot,
		MerchantSnapshot:    row.MerchantSnapshot,
		RiderSnapshot:       row.RiderSnapshot,
		Status:              row.Status,
		InvestigationReport: row.InvestigationReport,
		Resolution:          row.Resolution,
		PrimaryProductKey:   row.PrimaryProductKey,
		PrimaryProductLabel: row.PrimaryProductLabel,
		CaseID:              row.CaseID,
		CreatedAt:           row.CreatedAt,
		ResolvedAt:          row.ResolvedAt,
	}
}

func IsFoodSafetySuspendReason(reason string) bool {
	trimmedReason := strings.TrimSpace(reason)
	return strings.HasPrefix(trimmedReason, foodSafetyCircuitBreakReasonPrefix) ||
		strings.HasPrefix(trimmedReason, "同商户同产品食安举报触发熔断")
}

func isFoodSafetySuspendReasonText(reason pgtype.Text) bool {
	if !reason.Valid {
		return false
	}
	return IsFoodSafetySuspendReason(reason.String)
}

func isFoodSafetyUniqueConstraint(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != UniqueViolation {
		return false
	}
	return strings.Contains(pgErr.ConstraintName, constraintName)
}
