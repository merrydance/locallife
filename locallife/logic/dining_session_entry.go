package logic

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	DiningSessionEntryActionOpen     = "open_session"
	DiningSessionEntryActionResume   = "resume_session"
	DiningSessionEntryActionTransfer = "transfer_session"
	DiningSessionEntryActionBlocked  = "blocked"
)

type DiningSessionEntryInput struct {
	UserID     int64
	MerchantID *int64
	TableNo    *string
	TableID    *int64
	Now        time.Time
}

type DiningSessionEntryResult struct {
	Merchant                  db.Merchant
	Table                     db.Table
	Precheck                  DiningSessionPrecheckResult
	Action                    string
	BlockedReason             *string
	ActiveSession             *db.DiningSession
	ActiveBillingGroup        *db.BillingGroup
	TransferSession           *db.DiningSession
	TransferBillingGroup      *db.BillingGroup
	TransferSourceTable       *db.Table
	RequiresTableCode         bool
	TransferRequiresTableCode bool
	CanOrder                  bool
	CanTransfer               bool
}

type DiningSessionMenuResult struct {
	Session      db.DiningSession
	BillingGroup db.BillingGroup
	Merchant     db.Merchant
	Table        db.Table
}

func ResolveDiningSessionEntry(ctx context.Context, store db.Store, input DiningSessionEntryInput) (DiningSessionEntryResult, error) {
	var result DiningSessionEntryResult

	table, merchant, err := resolveDiningEntryTarget(ctx, store, input)
	if err != nil {
		return result, err
	}
	result.Table = table
	result.Merchant = merchant

	if merchant.Status != "approved" {
		return blockedDiningSessionEntryResult(result, "商户暂未开放堂食服务"), nil
	}
	if !merchant.IsOpen {
		return blockedDiningSessionEntryResult(result, "商户当前暂停营业"), nil
	}
	if table.Status == "disabled" {
		return blockedDiningSessionEntryResult(result, "当前桌台暂不可用"), nil
	}

	precheck, err := PrecheckDiningSession(ctx, store, DiningSessionPrecheckInput{
		UserID:  input.UserID,
		TableID: table.ID,
		Now:     input.Now,
	})
	if err != nil {
		var reqErr *RequestError
		if errors.As(err, &reqErr) && reqErr.Status == http.StatusConflict {
			return blockedDiningSessionEntryResult(result, reqErr.Error()), nil
		}
		return result, err
	}
	result.Precheck = precheck
	result.RequiresTableCode = precheck.Reservation == nil

	if currentSession, currentBillingGroup, currentAccess, err := resolveCurrentTableSession(ctx, store, table, precheck, input.UserID); err != nil {
		return result, err
	} else if currentSession != nil {
		if currentAccess {
			result.Action = DiningSessionEntryActionResume
			result.ActiveSession = currentSession
			result.ActiveBillingGroup = currentBillingGroup
			result.CanOrder = true
			return result, nil
		}
		return blockedDiningSessionEntryResult(result, "该桌台正有人用餐（已有活动会话）"), nil
	}

	if transferSession, transferBillingGroup, transferTable, err := resolveTransferCandidate(ctx, store, merchant.ID, table.ID, input.UserID); err != nil {
		return result, err
	} else if transferSession != nil {
		result.Action = DiningSessionEntryActionTransfer
		result.TransferSession = transferSession
		result.TransferBillingGroup = transferBillingGroup
		result.TransferSourceTable = transferTable
		result.TransferRequiresTableCode = !transferSession.ReservationID.Valid
		result.CanOrder = true
		result.CanTransfer = true
		return result, nil
	}

	result.Action = DiningSessionEntryActionOpen
	result.CanOrder = true
	return result, nil
}

func ResolveDiningSessionMenu(ctx context.Context, store db.Store, sessionID int64, userID int64) (DiningSessionMenuResult, error) {
	var result DiningSessionMenuResult

	session, err := store.GetDiningSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("dining session not found"))
		}
		return result, err
	}
	if session.Status != "open" {
		return result, NewRequestError(http.StatusConflict, errors.New("dining session is not open"))
	}

	isCustomerOwner, linkedReservation, err := resolveDiningSessionCustomerOwnership(ctx, store, session, userID)
	if err != nil {
		return result, err
	}

	billingGroup, err := store.GetDefaultBillingGroupBySession(ctx, session.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusConflict, errors.New("default billing group not found"))
		}
		return result, err
	}

	if !isCustomerOwner {
		if linkedReservation != nil {
			isDisallowedActor, err := isReservationCustomerAccessDisallowedActor(ctx, store, session, *linkedReservation, userID)
			if err != nil {
				return result, err
			}
			if isDisallowedActor {
				return result, NewRequestError(http.StatusForbidden, errors.New("dining session does not belong to you"))
			}
		}
		if _, err := store.GetActiveBillingGroupMember(ctx, db.GetActiveBillingGroupMemberParams{
			BillingGroupID: billingGroup.ID,
			UserID:         userID,
		}); err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return result, NewRequestError(http.StatusForbidden, errors.New("dining session does not belong to you"))
			}
			return result, err
		}
	}

	table, err := store.GetTable(ctx, session.TableID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("table not found"))
		}
		return result, err
	}

	merchant, err := store.GetMerchant(ctx, session.MerchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("merchant not found"))
		}
		return result, err
	}

	result.Session = session
	result.BillingGroup = billingGroup
	result.Table = table
	result.Merchant = merchant
	return result, nil
}

func resolveDiningEntryTarget(ctx context.Context, store db.Store, input DiningSessionEntryInput) (db.Table, db.Merchant, error) {
	if input.TableID != nil {
		table, err := store.GetTable(ctx, *input.TableID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return db.Table{}, db.Merchant{}, NewRequestError(http.StatusNotFound, errors.New("table not found"))
			}
			return db.Table{}, db.Merchant{}, err
		}
		merchant, err := store.GetMerchant(ctx, table.MerchantID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return db.Table{}, db.Merchant{}, NewRequestError(http.StatusNotFound, errors.New("merchant not found"))
			}
			return db.Table{}, db.Merchant{}, err
		}
		return table, merchant, nil
	}

	if input.MerchantID == nil || input.TableNo == nil || strings.TrimSpace(*input.TableNo) == "" {
		return db.Table{}, db.Merchant{}, NewRequestError(http.StatusBadRequest, errors.New("merchant_id and table_no are required"))
	}

	merchant, err := store.GetMerchant(ctx, *input.MerchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.Table{}, db.Merchant{}, NewRequestError(http.StatusNotFound, errors.New("merchant not found"))
		}
		return db.Table{}, db.Merchant{}, err
	}

	table, err := store.GetTableByMerchantAndNo(ctx, db.GetTableByMerchantAndNoParams{
		MerchantID: *input.MerchantID,
		TableNo:    strings.TrimSpace(*input.TableNo),
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.Table{}, db.Merchant{}, NewRequestError(http.StatusNotFound, errors.New("table not found"))
		}
		return db.Table{}, db.Merchant{}, err
	}

	return table, merchant, nil
}

func blockedDiningSessionEntryResult(result DiningSessionEntryResult, reason string) DiningSessionEntryResult {
	result.Action = DiningSessionEntryActionBlocked
	result.CanOrder = false
	result.CanTransfer = false
	result.BlockedReason = &reason
	return result
}

func resolveCurrentTableSession(ctx context.Context, store db.Store, table db.Table, precheck DiningSessionPrecheckResult, userID int64) (*db.DiningSession, *db.BillingGroup, bool, error) {
	if precheck.Reservation != nil {
		session, err := store.GetActiveDiningSessionByReservation(ctx, pgtype.Int8{Int64: precheck.Reservation.ID, Valid: true})
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return nil, nil, false, nil
			}
			return nil, nil, false, err
		}
		billingGroup, accessible, err := resolveSessionAccessibility(ctx, store, session, userID)
		if err != nil {
			return nil, nil, false, err
		}
		return &session, billingGroup, accessible, nil
	}

	session, err := store.GetActiveDiningSessionByTable(ctx, table.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, nil, false, nil
		}
		return nil, nil, false, err
	}
	if session.Status != "open" {
		return nil, nil, false, nil
	}
	billingGroup, accessible, err := resolveSessionAccessibility(ctx, store, session, userID)
	if err != nil {
		return nil, nil, false, err
	}
	return &session, billingGroup, accessible, nil
}

func resolveTransferCandidate(ctx context.Context, store db.Store, merchantID, currentTableID, userID int64) (*db.DiningSession, *db.BillingGroup, *db.Table, error) {
	sessions, err := store.ListDiningSessionsByUser(ctx, db.ListDiningSessionsByUserParams{
		UserID: userID,
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	for _, session := range sessions {
		if session.Status != "open" || session.MerchantID != merchantID || session.TableID == currentTableID {
			continue
		}
		if isOwner, _, err := resolveDiningSessionCustomerOwnership(ctx, store, session, userID); err != nil {
			return nil, nil, nil, err
		} else if !isOwner {
			continue
		}
		billingGroup, err := GetOrCreateDefaultBillingGroup(ctx, store, session, userID)
		if err != nil {
			return nil, nil, nil, err
		}
		table, err := store.GetTable(ctx, session.TableID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				continue
			}
			return nil, nil, nil, err
		}
		return &session, &billingGroup, &table, nil
	}

	return nil, nil, nil, nil
}

func resolveSessionAccessibility(ctx context.Context, store db.Store, session db.DiningSession, userID int64) (*db.BillingGroup, bool, error) {
	isCustomerOwner, linkedReservation, err := resolveDiningSessionCustomerOwnership(ctx, store, session, userID)
	if err != nil {
		return nil, false, err
	}

	billingGroup, err := store.GetDefaultBillingGroupBySession(ctx, session.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			if isCustomerOwner {
				bg, createErr := GetOrCreateDefaultBillingGroup(ctx, store, session, userID)
				if createErr != nil {
					return nil, false, createErr
				}
				return &bg, true, nil
			}
			return nil, false, nil
		}
		return nil, false, err
	}

	if isCustomerOwner {
		return &billingGroup, true, nil
	}
	if linkedReservation != nil {
		isDisallowedActor, err := isReservationCustomerAccessDisallowedActor(ctx, store, session, *linkedReservation, userID)
		if err != nil {
			return nil, false, err
		}
		if isDisallowedActor {
			return &billingGroup, false, nil
		}
	}
	if _, err := store.GetActiveBillingGroupMember(ctx, db.GetActiveBillingGroupMemberParams{
		BillingGroupID: billingGroup.ID,
		UserID:         userID,
	}); err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return &billingGroup, false, nil
		}
		return nil, false, err
	}
	return &billingGroup, true, nil
}
