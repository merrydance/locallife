package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type ApproveGroupApplicationTxParams struct {
	ApplicationID int64
	ReviewerUserID int64
}

type ApproveGroupApplicationTxResult struct {
	Application MerchantGroupApplication
	Group       MerchantGroup
}

func (store *SQLStore) ApproveGroupApplicationTx(ctx context.Context, arg ApproveGroupApplicationTxParams) (ApproveGroupApplicationTxResult, error) {
	var result ApproveGroupApplicationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		app, err := q.GetGroupApplication(ctx, arg.ApplicationID)
		if err != nil {
			return err
		}
		if app.Status != "submitted" {
			return errors.New("application status is not submitted")
		}

		group, err := q.CreateMerchantGroup(ctx, CreateMerchantGroupParams{
			Name:            app.GroupName,
			OwnerUserID:     app.ApplicantUserID,
			ContactPhone:    pgtype.Text{String: app.ContactPhone, Valid: app.ContactPhone != ""},
			LicenseNumber:   app.LicenseNumber,
			LicenseImageUrl: app.LicenseImageUrl,
			Address:         app.Address,
			RegionID:        app.RegionID,
			ApplicationData: app.ApplicationData,
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

		reviewedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		app, err = q.ReviewGroupApplication(ctx, ReviewGroupApplicationParams{
			ID:           app.ID,
			Status:       "approved",
			RejectReason: pgtype.Text{Valid: false},
			ReviewedBy:   pgtype.Int8{Int64: arg.ReviewerUserID, Valid: true},
			ReviewedAt:   reviewedAt,
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

func (store *SQLStore) ApproveGroupJoinRequestTx(ctx context.Context, arg ApproveGroupJoinRequestTxParams) (ApproveGroupJoinRequestTxResult, error) {
	var result ApproveGroupJoinRequestTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		req, err := q.GetGroupJoinRequest(ctx, arg.RequestID)
		if err != nil {
			return err
		}
		if req.GroupID != arg.GroupID {
			return errors.New("group mismatch")
		}
		if req.Status != "pending" {
			return errors.New("request status is not pending")
		}

		reviewedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		req, err = q.UpdateGroupJoinRequestStatus(ctx, UpdateGroupJoinRequestStatusParams{
			ID:         req.ID,
			Status:     "approved",
			ReviewedBy: pgtype.Int8{Int64: arg.ReviewerUserID, Valid: true},
			ReviewedAt: reviewedAt,
		})
		if err != nil {
			return err
		}

		if err := q.UpdateMerchantGroupBinding(ctx, UpdateMerchantGroupBindingParams{
			ID:      req.MerchantID,
			GroupID: pgtype.Int8{Int64: arg.GroupID, Valid: true},
			BrandID: arg.BrandID,
		}); err != nil {
			return err
		}

		meta, _ := json.Marshal(map[string]any{
			"request_id": req.ID,
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
