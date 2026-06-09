package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type ApproveGroupApplicationTxParams struct {
	ApplicationID  int64
	ReviewerUserID int64
}

type ApproveGroupApplicationTxResult struct {
	Application MerchantGroupApplication
	Group       MerchantGroup
}

func (store *SQLStore) ApproveGroupApplicationTx(ctx context.Context, arg ApproveGroupApplicationTxParams) (ApproveGroupApplicationTxResult, error) {
	var result ApproveGroupApplicationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		app, err := q.GetGroupApplicationForUpdate(ctx, arg.ApplicationID)
		if err != nil {
			return err
		}
		if app.Status != "submitted" {
			return ErrGroupApplicationReviewConflict
		}

		reviewedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		app, err = q.ReviewSubmittedGroupApplication(ctx, ReviewSubmittedGroupApplicationParams{
			ID:           app.ID,
			Status:       "approved",
			RejectReason: pgtype.Text{Valid: false},
			ReviewedBy:   pgtype.Int8{Int64: arg.ReviewerUserID, Valid: true},
			ReviewedAt:   reviewedAt,
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrGroupApplicationReviewConflict
			}
			return err
		}

		group, err := q.CreateMerchantGroup(ctx, CreateMerchantGroupParams{
			Name:                app.GroupName,
			OwnerUserID:         app.ApplicantUserID,
			ContactPhone:        pgtype.Text{String: app.ContactPhone, Valid: app.ContactPhone != ""},
			LicenseNumber:       app.LicenseNumber,
			LicenseMediaAssetID: app.LicenseMediaAssetID,
			Address:             app.Address,
			RegionID:            app.RegionID,
			ApplicationData:     app.ApplicationData,
		})
		if err != nil {
			return err
		}

		_, err = q.CreateGroupMember(ctx, CreateGroupMemberParams{
			GroupID:   group.ID,
			UserID:    app.ApplicantUserID,
			Role:      "owner",
			InvitedBy: pgtype.Int8{Valid: false},
		})
		if err != nil {
			return err
		}

		meta, _ := json.Marshal(map[string]any{
			"application_id": app.ID,
			"group_id":       group.ID,
		})
		_, err = q.CreateGroupAuditLog(ctx, CreateGroupAuditLogParams{
			GroupID:     pgtype.Int8{Int64: group.ID, Valid: true},
			ActorUserID: pgtype.Int8{Int64: arg.ReviewerUserID, Valid: true},
			Action:      "group_application_approved",
			TargetType:  "group_application",
			TargetID:    pgtype.Int8{Int64: app.ID, Valid: true},
			Metadata:    meta,
		})
		if err != nil {
			return err
		}

		result.Application = app
		result.Group = group
		return nil
	})

	return result, err
}

type ApproveGroupJoinRequestTxParams struct {
	RequestID      int64
	GroupID        int64
	ReviewerUserID int64
	BrandID        pgtype.Int8
}

type ApproveGroupJoinRequestTxResult struct {
	Request MerchantGroupJoinRequest
}

type CreateGroupJoinRequestTxParams struct {
	GroupID         int64
	MerchantID      int64
	ApplicantUserID int64
	Reason          pgtype.Text
}

type CreateGroupJoinRequestTxResult struct {
	Request MerchantGroupJoinRequest
}

func (store *SQLStore) CreateGroupJoinRequestTx(ctx context.Context, arg CreateGroupJoinRequestTxParams) (CreateGroupJoinRequestTxResult, error) {
	var result CreateGroupJoinRequestTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		affiliation, err := q.GetMerchantGroupAffiliationForUpdate(ctx, arg.MerchantID)
		if err != nil {
			return err
		}
		if affiliation.GroupID.Valid {
			return ErrMerchantAlreadyJoinedGroup
		}

		req, err := q.CreateGroupJoinRequest(ctx, CreateGroupJoinRequestParams{
			GroupID:         arg.GroupID,
			MerchantID:      arg.MerchantID,
			ApplicantUserID: arg.ApplicantUserID,
			Reason:          arg.Reason,
		})
		if err != nil {
			return err
		}

		meta, _ := json.Marshal(map[string]any{
			"merchant_id": req.MerchantID,
			"group_id":    req.GroupID,
			"request_id":  req.ID,
		})
		_, err = q.CreateGroupAuditLog(ctx, CreateGroupAuditLogParams{
			GroupID:     pgtype.Int8{Int64: req.GroupID, Valid: true},
			ActorUserID: pgtype.Int8{Int64: arg.ApplicantUserID, Valid: true},
			Action:      "group_join_request_created",
			TargetType:  "merchant",
			TargetID:    pgtype.Int8{Int64: req.MerchantID, Valid: true},
			Metadata:    meta,
		})
		if err != nil {
			return err
		}

		result.Request = req
		return nil
	})

	return result, err
}

func (store *SQLStore) ApproveGroupJoinRequestTx(ctx context.Context, arg ApproveGroupJoinRequestTxParams) (ApproveGroupJoinRequestTxResult, error) {
	var result ApproveGroupJoinRequestTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		req, err := q.GetGroupJoinRequestForUpdate(ctx, arg.RequestID)
		if err != nil {
			return err
		}
		if req.GroupID != arg.GroupID {
			return ErrGroupJoinRequestGroupMismatch
		}
		if req.Status != "pending" {
			return ErrGroupJoinRequestReviewConflict
		}

		affiliation, err := q.GetMerchantGroupAffiliationForUpdate(ctx, req.MerchantID)
		if err != nil {
			return err
		}
		if affiliation.GroupID.Valid {
			return ErrMerchantAlreadyJoinedGroup
		}

		reviewedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		req, err = q.ApprovePendingGroupJoinRequest(ctx, ApprovePendingGroupJoinRequestParams{
			ID:         req.ID,
			ReviewedBy: pgtype.Int8{Int64: arg.ReviewerUserID, Valid: true},
			ReviewedAt: reviewedAt,
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrGroupJoinRequestReviewConflict
			}
			return err
		}

		affected, err := q.AttachMerchantToGroupIfUnassigned(ctx, AttachMerchantToGroupIfUnassignedParams{
			ID:      req.MerchantID,
			GroupID: pgtype.Int8{Int64: arg.GroupID, Valid: true},
			BrandID: arg.BrandID,
		})
		if err != nil {
			return err
		}
		if affected != 1 {
			return ErrMerchantAlreadyJoinedGroup
		}

		meta, _ := json.Marshal(map[string]any{
			"request_id":  req.ID,
			"merchant_id": req.MerchantID,
		})
		_, err = q.CreateGroupAuditLog(ctx, CreateGroupAuditLogParams{
			GroupID:     pgtype.Int8{Int64: arg.GroupID, Valid: true},
			ActorUserID: pgtype.Int8{Int64: arg.ReviewerUserID, Valid: true},
			Action:      "group_join_request_approved",
			TargetType:  "merchant",
			TargetID:    pgtype.Int8{Int64: req.MerchantID, Valid: true},
			Metadata:    meta,
		})
		if err != nil {
			return err
		}

		result.Request = req
		return nil
	})

	return result, err
}

type RejectGroupJoinRequestTxParams struct {
	RequestID      int64
	GroupID        int64
	ReviewerUserID int64
	Reason         pgtype.Text
}

type RejectGroupJoinRequestTxResult struct {
	Request MerchantGroupJoinRequest
}

func (store *SQLStore) RejectGroupJoinRequestTx(ctx context.Context, arg RejectGroupJoinRequestTxParams) (RejectGroupJoinRequestTxResult, error) {
	var result RejectGroupJoinRequestTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		req, err := q.GetGroupJoinRequestForUpdate(ctx, arg.RequestID)
		if err != nil {
			return err
		}
		if req.GroupID != arg.GroupID {
			return ErrGroupJoinRequestGroupMismatch
		}
		if req.Status != "pending" {
			return ErrGroupJoinRequestReviewConflict
		}

		reviewedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		req, err = q.RejectPendingGroupJoinRequest(ctx, RejectPendingGroupJoinRequestParams{
			ID:         req.ID,
			ReviewedBy: pgtype.Int8{Int64: arg.ReviewerUserID, Valid: true},
			ReviewedAt: reviewedAt,
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrGroupJoinRequestReviewConflict
			}
			return err
		}

		meta, _ := json.Marshal(map[string]any{
			"request_id": req.ID,
			"reason":     nullableTextForAudit(arg.Reason),
		})
		_, err = q.CreateGroupAuditLog(ctx, CreateGroupAuditLogParams{
			GroupID:     pgtype.Int8{Int64: arg.GroupID, Valid: true},
			ActorUserID: pgtype.Int8{Int64: arg.ReviewerUserID, Valid: true},
			Action:      "group_join_request_rejected",
			TargetType:  "merchant",
			TargetID:    pgtype.Int8{Int64: req.MerchantID, Valid: true},
			Metadata:    meta,
		})
		if err != nil {
			return err
		}

		result.Request = req
		return nil
	})

	return result, err
}

type CancelGroupJoinRequestTxParams struct {
	RequestID       int64
	GroupID         int64
	ApplicantUserID int64
}

type CancelGroupJoinRequestTxResult struct {
	Request MerchantGroupJoinRequest
}

func (store *SQLStore) CancelGroupJoinRequestTx(ctx context.Context, arg CancelGroupJoinRequestTxParams) (CancelGroupJoinRequestTxResult, error) {
	var result CancelGroupJoinRequestTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		req, err := q.GetGroupJoinRequestForUpdate(ctx, arg.RequestID)
		if err != nil {
			return err
		}
		if req.GroupID != arg.GroupID {
			return ErrGroupJoinRequestGroupMismatch
		}
		if req.ApplicantUserID != arg.ApplicantUserID {
			return ErrGroupJoinRequestApplicantMismatch
		}
		if req.Status != "pending" {
			return ErrGroupJoinRequestReviewConflict
		}

		reviewedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		req, err = q.CancelPendingGroupJoinRequest(ctx, CancelPendingGroupJoinRequestParams{
			ID:         req.ID,
			ReviewedBy: pgtype.Int8{Int64: arg.ApplicantUserID, Valid: true},
			ReviewedAt: reviewedAt,
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrGroupJoinRequestReviewConflict
			}
			return err
		}

		meta, _ := json.Marshal(map[string]any{
			"request_id": req.ID,
		})
		_, err = q.CreateGroupAuditLog(ctx, CreateGroupAuditLogParams{
			GroupID:     pgtype.Int8{Int64: arg.GroupID, Valid: true},
			ActorUserID: pgtype.Int8{Int64: arg.ApplicantUserID, Valid: true},
			Action:      "group_join_request_cancelled",
			TargetType:  "merchant",
			TargetID:    pgtype.Int8{Int64: req.MerchantID, Valid: true},
			Metadata:    meta,
		})
		if err != nil {
			return err
		}

		result.Request = req
		return nil
	})

	return result, err
}

func nullableTextForAudit(value pgtype.Text) any {
	if value.Valid {
		return value.String
	}
	return nil
}
