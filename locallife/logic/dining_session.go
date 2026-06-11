package logic

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
)

// OpenDiningSessionInput holds parameters for opening a dining session.
type OpenDiningSessionInput struct {
	UserID              int64
	TableID             int64
	ReservationID       *int64
	TableCode           *string
	Now                 time.Time
	CheckInEarlyMinutes int
	CheckInLateMinutes  int
}

// OpenDiningSessionResult returns the session creation outcome.
type OpenDiningSessionResult struct {
	Session       db.DiningSession
	BillingGroup  db.BillingGroup
	CartID        *int64
	ImportedItems int
	Existing      bool
}

// TransferDiningSessionTableInput holds parameters for transferring a dining session.
type TransferDiningSessionTableInput struct {
	SessionID int64
	ToTableID int64
	UserID    int64
	TableCode *string
	Reason    *string
	Now       time.Time
}

// TransferDiningSessionTableResult returns transfer details.
type TransferDiningSessionTableResult struct {
	Session   db.DiningSession
	FromTable db.Table
	ToTable   db.Table
	SameTable bool
}

// CheckoutDiningSessionInput holds parameters for closing a dining session.
type CheckoutDiningSessionInput struct {
	SessionID int64
	UserID    int64
}

// CheckoutDiningSessionResult returns closure details.
type CheckoutDiningSessionResult struct {
	Session  db.DiningSession
	Merchant db.Merchant
}

func resolveDiningSessionCustomerOwnership(ctx context.Context, store db.Store, session db.DiningSession, userID int64) (bool, *db.TableReservation, error) {
	if !session.ReservationID.Valid {
		return session.UserID == userID, nil, nil
	}

	reservation, err := store.GetTableReservation(ctx, session.ReservationID.Int64)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return false, nil, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
		}
		return false, nil, err
	}

	return isCustomerOwnedReservation(reservation, userID), &reservation, nil
}

// GetOrCreateDefaultBillingGroup returns the default billing group and ensures membership.
func GetOrCreateDefaultBillingGroup(ctx context.Context, store db.Store, session db.DiningSession, userID int64) (db.BillingGroup, error) {
	billingGroup, err := store.GetDefaultBillingGroupBySession(ctx, session.ID)
	if err != nil {
		if !errors.Is(err, db.ErrRecordNotFound) {
			return db.BillingGroup{}, err
		}

		billingGroup, err = store.CreateBillingGroup(ctx, db.CreateBillingGroupParams{
			DiningSessionID: session.ID,
			Status:          "open",
			IsDefault:       true,
			TotalAmount:     0,
			PaidAmount:      0,
		})
		if err != nil {
			if db.ErrorCode(err) != db.UniqueViolation {
				return db.BillingGroup{}, err
			}
			billingGroup, err = store.GetDefaultBillingGroupBySession(ctx, session.ID)
			if err != nil {
				return db.BillingGroup{}, err
			}
		}
	}

	role := "member"
	if session.UserID == userID {
		role = "owner"
	}
	shouldEnsureMembership := true
	if session.ReservationID.Valid {
		isCustomerOwner, linkedReservation, err := resolveDiningSessionCustomerOwnership(ctx, store, session, userID)
		if err != nil {
			return db.BillingGroup{}, err
		}
		if isCustomerOwner {
			role = "owner"
		} else if linkedReservation != nil {
			isDisallowedActor, err := isReservationCustomerAccessDisallowedActor(ctx, store, session, *linkedReservation, userID)
			if err != nil {
				return db.BillingGroup{}, err
			}
			if isDisallowedActor {
				shouldEnsureMembership = false
			} else {
				role = "member"
			}
		} else {
			role = "member"
		}
	}
	if !shouldEnsureMembership {
		return billingGroup, nil
	}
	if _, err := store.GetActiveBillingGroupMember(ctx, db.GetActiveBillingGroupMemberParams{
		BillingGroupID: billingGroup.ID,
		UserID:         userID,
	}); err != nil {
		if !errors.Is(err, db.ErrRecordNotFound) {
			return db.BillingGroup{}, err
		}
		if _, err := store.CreateBillingGroupMember(ctx, db.CreateBillingGroupMemberParams{
			BillingGroupID: billingGroup.ID,
			UserID:         userID,
			Role:           role,
		}); err != nil && db.ErrorCode(err) != db.UniqueViolation {
			return db.BillingGroup{}, err
		}
	}

	return billingGroup, nil
}

// FindActiveReservationForTable finds an active reservation for a table that conflicts with now.
func FindActiveReservationForTable(ctx context.Context, store db.Store, tableID int64, now time.Time) (*db.TableReservation, error) {
	date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	reservations, err := store.ListReservationsByTableAndDate(ctx, db.ListReservationsByTableAndDateParams{
		TableID: tableID,
		ReservationDate: pgtype.Date{
			Time:  date,
			Valid: true,
		},
	})
	if err != nil {
		return nil, err
	}

	for _, r := range reservations {
		if r.Status != "pending" && r.Status != "paid" && r.Status != "confirmed" && r.Status != "checked_in" {
			continue
		}
		if !r.ReservationTime.Valid {
			continue
		}

		resStart := util.CombineDateAndTime(r.ReservationDate.Time, r.ReservationTime.Microseconds)
		if !util.IsConflictWithReservation(now, resStart) {
			continue
		}

		res := r
		return &res, nil
	}

	return nil, nil
}

// OpenDiningSession validates and opens a dining session or returns an existing one.
func OpenDiningSession(ctx context.Context, store db.Store, input OpenDiningSessionInput) (OpenDiningSessionResult, error) {
	var result OpenDiningSessionResult

	table, err := store.GetTable(ctx, input.TableID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("table not found"))
		}
		return result, err
	}

	isMerchant := false
	if m, err := store.GetMerchant(ctx, table.MerchantID); err == nil && m.OwnerUserID == input.UserID {
		isMerchant = true
	} else if err == nil {
		if hasAccess, err := store.CheckUserHasMerchantAccess(ctx, db.CheckUserHasMerchantAccessParams{
			MerchantID: table.MerchantID,
			UserID:     input.UserID,
		}); err == nil && hasAccess {
			isMerchant = true
		}
	}

	activeReservation, err := FindActiveReservationForTable(ctx, store, table.ID, input.Now)
	if err != nil {
		return result, err
	}

	var reservation *db.TableReservation
	if input.ReservationID != nil {
		res, err := store.GetTableReservation(ctx, *input.ReservationID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return result, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
			}
			return result, err
		}
		reservation = &res
	} else if activeReservation != nil && isCustomerOwnedReservation(*activeReservation, input.UserID) {
		reservation = activeReservation
	}

	if activeReservation != nil && (reservation == nil || activeReservation.ID != reservation.ID) {
		if !isCustomerOwnedReservation(*activeReservation, input.UserID) && !isMerchant {
			return result, NewRequestError(http.StatusConflict, errors.New("该桌位已被预订，暂时不可用"))
		}
	}

	if reservation != nil {
		if !isCustomerOwnedReservation(*reservation, input.UserID) && !isMerchant {
			return result, NewRequestError(http.StatusForbidden, errors.New("reservation does not belong to you"))
		}
		if reservation.TableID != input.TableID {
			return result, NewRequestError(http.StatusBadRequest, errors.New("table does not match reservation"))
		}
		if reservation.Status != "paid" && reservation.Status != "confirmed" && reservation.Status != "checked_in" {
			return result, NewRequestError(http.StatusConflict, errors.New("reservation is not ready for dining"))
		}

		if !isMerchant {
			scheduledAt := util.CombineDateAndTime(reservation.ReservationDate.Time, reservation.ReservationTime.Microseconds)
			earlyLimit := scheduledAt.Add(-time.Duration(input.CheckInEarlyMinutes) * time.Minute)
			lateLimit := scheduledAt.Add(time.Duration(input.CheckInLateMinutes) * time.Minute)
			if input.Now.Before(earlyLimit) {
				return result, NewRequestError(http.StatusConflict, errors.New("too early to check in for reservation"))
			}
			if input.Now.After(lateLimit) {
				return result, NewRequestError(http.StatusConflict, errors.New("reservation check-in window has passed"))
			}
		}
	}

	if reservation == nil && isMerchant {
		return result, NewRequestError(http.StatusForbidden, errors.New("商户不能代客开台，请让客人扫码入座"))
	}

	if reservation == nil && !isMerchant {
		if !table.AccessCodeHash.Valid || strings.TrimSpace(table.AccessCodeHash.String) == "" {
			return result, NewRequestError(http.StatusConflict, errors.New("table access code is not configured"))
		}
		if input.TableCode == nil || strings.TrimSpace(*input.TableCode) == "" {
			return result, NewRequestError(http.StatusBadRequest, errors.New("table access code is required"))
		}
		if err := util.CheckPassword(*input.TableCode, table.AccessCodeHash.String); err != nil {
			return result, NewRequestError(http.StatusForbidden, errors.New("invalid table access code"))
		}
	}

	if reservation != nil {
		if existing, err := store.GetActiveDiningSessionByReservation(ctx, pgtype.Int8{Int64: reservation.ID, Valid: true}); err == nil {
			billingGroup, err := GetOrCreateDefaultBillingGroup(ctx, store, existing, input.UserID)
			if err != nil {
				return result, err
			}
			result.Session = existing
			result.BillingGroup = billingGroup
			result.Existing = true
			return result, nil
		} else if !errors.Is(err, db.ErrRecordNotFound) {
			return result, err
		}
	}

	if existing, err := store.GetActiveDiningSessionByTable(ctx, input.TableID); err == nil {
		if existing.UserID == input.UserID && !existing.ActiveOrderID.Valid {
			_, _ = store.CloseDiningSessionTx(ctx, db.CloseDiningSessionTxParams{
				ID:         existing.ID,
				MerchantID: existing.MerchantID,
			})
		} else {
			if reservation != nil {
				if !existing.ReservationID.Valid || existing.ReservationID.Int64 != reservation.ID {
					return result, NewRequestError(http.StatusConflict, errors.New("该桌台正有人用餐（已有活动会话）"))
				}
			} else if existing.ReservationID.Valid {
				return result, NewRequestError(http.StatusConflict, errors.New("该桌台正有人用餐（已有活动会话）"))
			}

			billingGroup, err := GetOrCreateDefaultBillingGroup(ctx, store, existing, input.UserID)
			if err != nil {
				return result, err
			}
			result.Session = existing
			result.BillingGroup = billingGroup
			result.Existing = true
			return result, nil
		}
	} else if !errors.Is(err, db.ErrRecordNotFound) {
		return result, err
	}

	resID := pgtype.Int8{Valid: false}
	if reservation != nil {
		resID = pgtype.Int8{Int64: reservation.ID, Valid: true}
	}

	var activateOrder *db.ActivateOrderInput
	if reservation != nil {
		order, err := store.GetLatestOrderByReservation(ctx, resID)
		if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			return result, err
		}
		if err == nil {
			if order.Status != db.OrderStatusPaid {
				return result, NewRequestError(http.StatusConflict, errors.New("reservation order is not paid"))
			}

			newFulfillment := order.FulfillmentStatus
			if order.FulfillmentStatus == db.FulfillmentStatusScheduled {
				newFulfillment = db.FulfillmentStatusPendingKitchen
			}

			activateOrder = &db.ActivateOrderInput{
				OrderID:              order.ID,
				OldStatus:            db.OrderStatusPaid,
				Status:               order.Status,
				NewFulfillmentStatus: pgtype.Text{String: newFulfillment, Valid: true},
			}
		}
	}

	txResult, err := store.OpenDiningSessionTx(ctx, db.OpenDiningSessionTxParams{
		TableID:                       table.ID,
		MerchantID:                    table.MerchantID,
		UserID:                        input.UserID,
		ReservationID:                 resID,
		ImportReservationItems:        reservation != nil,
		SkipDefaultBillingGroupMember: reservation != nil && !isCustomerOwnedReservation(*reservation, input.UserID),
		ActivateOrder:                 activateOrder,
	})
	if err != nil {
		return result, err
	}

	result.Session = txResult.Session
	if txResult.ActivatedOrder != nil {
		result.Session.ActiveOrderID = pgtype.Int8{Int64: txResult.ActivatedOrder.ID, Valid: true}
	}
	result.BillingGroup = txResult.BillingGroup
	result.CartID = txResult.CartID
	result.ImportedItems = txResult.ImportedItems

	return result, nil
}

// TransferDiningSessionTable validates and transfers a dining session to another table.
func TransferDiningSessionTable(ctx context.Context, store db.Store, input TransferDiningSessionTableInput) (TransferDiningSessionTableResult, error) {
	var result TransferDiningSessionTableResult

	session, err := store.GetDiningSession(ctx, input.SessionID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("dining session not found"))
		}
		return result, err
	}
	if session.Status != "open" {
		return result, NewRequestError(http.StatusConflict, errors.New("dining session is not open"))
	}

	isOwner, linkedReservation, err := resolveDiningSessionCustomerOwnership(ctx, store, session, input.UserID)
	if err != nil {
		return result, err
	}
	isMerchant := false
	if m, err := store.GetMerchant(ctx, session.MerchantID); err == nil && m.OwnerUserID == input.UserID {
		isMerchant = true
	} else if err == nil {
		if hasAccess, err := store.CheckUserHasMerchantAccess(ctx, db.CheckUserHasMerchantAccessParams{
			MerchantID: session.MerchantID,
			UserID:     input.UserID,
		}); err == nil && hasAccess {
			isMerchant = true
		}
	}

	if !isOwner && !isMerchant {
		return result, NewRequestError(http.StatusForbidden, errors.New("not authorized to transfer dining session"))
	}

	if input.ToTableID == session.TableID {
		fromTable, err := store.GetTable(ctx, session.TableID)
		if err != nil {
			return result, err
		}
		result.Session = session
		result.FromTable = fromTable
		result.ToTable = fromTable
		result.SameTable = true
		return result, nil
	}

	toTable, err := store.GetTable(ctx, input.ToTableID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("table not found"))
		}
		return result, err
	}
	if toTable.MerchantID != session.MerchantID {
		return result, NewRequestError(http.StatusBadRequest, errors.New("table does not belong to session merchant"))
	}
	if toTable.Status == "disabled" {
		return result, NewRequestError(http.StatusConflict, errors.New("target table is disabled"))
	}

	if linkedReservation != nil && !isMerchant && !isCustomerOwnedReservation(*linkedReservation, input.UserID) {
		return result, NewRequestError(http.StatusForbidden, errors.New("reservation does not belong to you"))
	}

	activeReservation, err := FindActiveReservationForTable(ctx, store, toTable.ID, input.Now)
	if err != nil {
		return result, err
	}
	if activeReservation != nil {
		if !session.ReservationID.Valid || activeReservation.ID != session.ReservationID.Int64 {
			return result, NewRequestError(http.StatusConflict, errors.New("目标桌位已有其他时段的预约且不可用"))
		}
	}

	if !isMerchant && !session.ReservationID.Valid {
		if input.TableCode == nil || strings.TrimSpace(*input.TableCode) == "" {
			return result, NewRequestError(http.StatusBadRequest, errors.New("table access code is required"))
		}
		if !toTable.AccessCodeHash.Valid || strings.TrimSpace(toTable.AccessCodeHash.String) == "" {
			return result, NewRequestError(http.StatusConflict, errors.New("table access code is not configured"))
		}
		if err := util.CheckPassword(*input.TableCode, toTable.AccessCodeHash.String); err != nil {
			return result, NewRequestError(http.StatusForbidden, errors.New("invalid table access code"))
		}
	}

	var reason pgtype.Text
	if input.Reason != nil {
		trimmed := strings.TrimSpace(*input.Reason)
		if trimmed != "" {
			reason = pgtype.Text{String: trimmed, Valid: true}
		}
	}

	transferResult, err := store.TransferDiningSessionTableTx(ctx, db.TransferDiningSessionTableTxParams{
		SessionID:      session.ID,
		ToTableID:      input.ToTableID,
		OperatorUserID: input.UserID,
		Reason:         reason,
	})
	if err != nil {
		switch {
		case errors.Is(err, db.ErrDiningSessionNotFound):
			return result, NewRequestError(http.StatusNotFound, errors.New("找不到就餐会话"))
		case errors.Is(err, db.ErrDiningSessionNotOpen):
			return result, NewRequestError(http.StatusConflict, errors.New("就餐会话未开启"))
		case errors.Is(err, db.ErrTargetTableDisabled):
			return result, NewRequestError(http.StatusConflict, errors.New("目标桌位已禁用"))
		case errors.Is(err, db.ErrTargetTableOccupied):
			return result, NewRequestError(http.StatusConflict, errors.New("目标桌台正有人用餐，请选择其他桌位"))
		case errors.Is(err, db.ErrTargetTableReserved):
			return result, NewRequestError(http.StatusConflict, errors.New("目标桌台已被预约"))
		case errors.Is(err, db.ErrReservationMismatch):
			return result, NewRequestError(http.StatusConflict, errors.New("预约记录不匹配"))
		default:
			return result, err
		}
	}

	result.Session = transferResult.Session
	result.FromTable = transferResult.FromTable
	result.ToTable = transferResult.ToTable
	return result, nil
}

// CheckoutDiningSession closes a dining session.
func CheckoutDiningSession(ctx context.Context, store db.Store, input CheckoutDiningSessionInput) (CheckoutDiningSessionResult, error) {
	var result CheckoutDiningSessionResult

	merchant, err := resolveMerchantForUser(ctx, store, input.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusForbidden, errors.New("您不是商户，无法操作"))
		}
		return result, err
	}

	closeResult, err := store.CloseDiningSessionTx(ctx, db.CloseDiningSessionTxParams{
		ID:         input.SessionID,
		MerchantID: merchant.ID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("找不到指定的就餐会话"))
		}
		return result, err
	}

	result.Session = closeResult.Session
	result.Merchant = merchant
	return result, nil
}
