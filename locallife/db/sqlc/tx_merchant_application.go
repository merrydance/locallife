package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type onboardingReviewRunSummary struct {
	RunID         int64    `json:"run_id"`
	Stage         string   `json:"stage"`
	Outcome       string   `json:"outcome"`
	ReasonCode    string   `json:"reason_code"`
	ReasonMessage string   `json:"reason_message,omitempty"`
	RuleHits      []string `json:"rule_hits,omitempty"`
	OCRJobRefs    []int64  `json:"ocr_job_refs,omitempty"`
	CreatedAt     string   `json:"created_at"`
}

// ApproveMerchantApplicationTxParams contains input parameters for approving merchant application
type ApproveMerchantApplicationTxParams struct {
	ApplicationID     int64
	UserID            int64
	MerchantName      string
	Phone             string
	Address           string
	Latitude          pgtype.Numeric
	Longitude         pgtype.Numeric
	RegionID          int64
	AppData           []byte // JSON 格式的申请数据
	StorefrontImages  []byte
	EnvironmentImages []byte
}

// ApproveMerchantApplicationTxResult contains the result of approval transaction
type ApproveMerchantApplicationTxResult struct {
	Application MerchantApplication
	Merchant    Merchant
	UserRole    UserRole
}

// ApproveMerchantApplicationTx approves a merchant application, creates merchant record,
// assigns merchant owner role, and ensures the owner is present in merchant_staff.
// This ensures atomicity: if any step fails, all changes are rolled back.
func (store *SQLStore) ApproveMerchantApplicationTx(ctx context.Context, arg ApproveMerchantApplicationTxParams) (ApproveMerchantApplicationTxResult, error) {
	var result ApproveMerchantApplicationTxResult

	if arg.RegionID <= 0 {
		return result, fmt.Errorf("invalid region id: %d", arg.RegionID)
	}

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// Step 1: 更新申请状态为 approved
		result.Application, err = q.ApproveMerchantApplication(ctx, ApproveMerchantApplicationParams{
			ID:     arg.ApplicationID,
			UserID: arg.UserID,
		})
		if err != nil {
			return fmt.Errorf("approve application: %w", err)
		}

		// Step 2: 创建或更新商户记录
		existingMerchant, err := q.GetMerchantOwnedByUser(ctx, arg.UserID)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				// 创建新商户
				result.Merchant, err = q.CreateMerchant(ctx, CreateMerchantParams{
					OwnerUserID:       arg.UserID,
					Name:              arg.MerchantName,
					Description:       pgtype.Text{},
					Phone:             arg.Phone,
					Address:           arg.Address,
					Latitude:          arg.Latitude,
					Longitude:         arg.Longitude,
					Status:            "approved",
					ApplicationData:   arg.AppData,
					RegionID:          arg.RegionID,
					StorefrontImages:  arg.StorefrontImages,
					EnvironmentImages: arg.EnvironmentImages,
				})
			} else {
				return fmt.Errorf("get existing merchant: %w", err)
			}
		} else {
			// 更新现有商户
			result.Merchant, err = q.UpdateMerchant(ctx, UpdateMerchantParams{
				ID:        existingMerchant.ID,
				Version:   existingMerchant.Version,
				Name:      pgtype.Text{String: arg.MerchantName, Valid: true},
				Phone:     pgtype.Text{String: arg.Phone, Valid: true},
				Address:   pgtype.Text{String: arg.Address, Valid: true},
				Latitude:  arg.Latitude,
				Longitude: arg.Longitude,
				RegionID:  pgtype.Int8{Int64: arg.RegionID, Valid: true},
			})
			if err == nil {
				// 仅在非 active/suspended 状态下推进到 approved，避免降级
				if existingMerchant.Status != "active" && existingMerchant.Status != "suspended" {
					result.Merchant, err = q.UpdateMerchantStatus(ctx, UpdateMerchantStatusParams{
						ID:     existingMerchant.ID,
						Status: "approved",
					})
				}
			}
			if err == nil {
				result.Merchant, err = q.UpdateMerchantShopImages(ctx, UpdateMerchantShopImagesParams{
					ID:                result.Merchant.ID,
					StorefrontImages:  arg.StorefrontImages,
					EnvironmentImages: arg.EnvironmentImages,
				})
			}
		}
		if err != nil {
			return fmt.Errorf("create/update merchant: %w", err)
		}

		normalizedMerchantName := normalizeWantedMerchantNameForDB(result.Merchant.Name)
		if normalizedMerchantName != "" {
			if err := q.MarkActiveWantedMerchantMatchedByMerchant(ctx, MarkActiveWantedMerchantMatchedByMerchantParams{
				RegionID:          arg.RegionID,
				NormalizedName:    normalizedMerchantName,
				MatchedMerchantID: pgtype.Int8{Int64: result.Merchant.ID, Valid: true},
			}); err != nil {
				return fmt.Errorf("mark wanted merchant matched: %w", err)
			}
		}

		// Step 2.5: 入驻后默认设为打烊状态，商户需手动开店
		result.Merchant, err = q.UpdateMerchantIsOpen(ctx, UpdateMerchantIsOpenParams{
			ID:     result.Merchant.ID,
			IsOpen: false,
		})
		if err != nil {
			return fmt.Errorf("set merchant closed: %w", err)
		}

		catalog, err := LoadMerchantSystemLabelCatalog(ctx, q)
		if err != nil {
			return fmt.Errorf("load merchant system label catalog: %w", err)
		}
		if err := ReconcileMerchantSystemLabels(ctx, q, result.Merchant.ID, catalog, MerchantSystemLabelSourceReconciler); err != nil {
			return fmt.Errorf("reconcile merchant system labels: %w", err)
		}

		// Step 3: 确保老板在 merchant_staff 中有 owner 记录。
		staff, err := q.GetMerchantStaff(ctx, GetMerchantStaffParams{
			MerchantID: result.Merchant.ID,
			UserID:     arg.UserID,
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				_, err = q.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
					MerchantID: result.Merchant.ID,
					UserID:     arg.UserID,
					Role:       MerchantStaffRoleOwner,
					Status:     MerchantStaffStatusActive,
					InvitedBy:  pgtype.Int8{},
				})
			} else {
				return fmt.Errorf("get merchant staff: %w", err)
			}
		} else if staff.Role != MerchantStaffRoleOwner || staff.Status != MerchantStaffStatusActive {
			_, err = q.UpdateMerchantStaffRole(ctx, UpdateMerchantStaffRoleParams{
				ID:   staff.ID,
				Role: MerchantStaffRoleOwner,
			})
		}
		if err != nil {
			return fmt.Errorf("ensure merchant owner staff: %w", err)
		}

		// Step 4: 创建或更新用户商户老板角色。
		// 检查是否已有该角色
		roles, err := q.ListUserRoles(ctx, arg.UserID)
		if err != nil {
			return fmt.Errorf("list user roles: %w", err)
		}

		hasMerchantRole := false
		for _, r := range roles {
			if r.Role == UserRoleMerchantOwner {
				hasMerchantRole = true
				// 如果角色已存在但关联实体 ID 不对，或者状态不是 active，可以在这里更新
				// 但目前 CreateUserRole 足够，如果已存在则跳过
				result.UserRole = r
				break
			}
		}

		if !hasMerchantRole {
			result.UserRole, err = q.CreateUserRole(ctx, CreateUserRoleParams{
				UserID:          arg.UserID,
				Role:            UserRoleMerchantOwner,
				Status:          "active",
				RelatedEntityID: pgtype.Int8{Int64: result.Merchant.ID, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create user role: %w", err)
			}
		}

		return nil
	})

	return result, err
}

// ResetMerchantApplicationTxParams contains input parameters for resetting merchant application
type ResetMerchantApplicationTxParams struct {
	ApplicationID int64
	UserID        int64
}

// ResetMerchantApplicationTxResult contains the result of reset transaction
type ResetMerchantApplicationTxResult struct {
	Application         MerchantApplication
	Merchant            Merchant
	CancelledReviewRuns []OnboardingReviewRun
}

// ResetMerchantApplicationTx resets a merchant application to draft status
// and sets the associated merchant status to pending (if exists).
func (store *SQLStore) ResetMerchantApplicationTx(ctx context.Context, arg ResetMerchantApplicationTxParams) (ResetMerchantApplicationTxResult, error) {
	var result ResetMerchantApplicationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// Step 1: 重置申请状态为 draft
		result.Application, err = q.ResetMerchantApplicationToDraft(ctx, arg.ApplicationID)
		if err != nil {
			return fmt.Errorf("reset application: %w", err)
		}

		// Step 2: 明确编辑/重置会使当前排队审核过期，避免旧 worker 之后重试漂移。
		result.CancelledReviewRuns, err = q.CancelActiveMerchantOnboardingReviewRunsForApplication(ctx, CancelActiveMerchantOnboardingReviewRunsForApplicationParams{
			Outcome:               pgtype.Text{String: OnboardingReviewOutcomeNeedsResubmit, Valid: true},
			ReasonCode:            pgtype.Text{String: OnboardingReviewReasonSupersededByEdit, Valid: true},
			ReasonMessage:         pgtype.Text{String: OnboardingReviewReasonMessageSupersededByEdit, Valid: true},
			MerchantApplicationID: pgtype.Int8{Int64: arg.ApplicationID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("cancel active merchant onboarding review runs: %w", err)
		}
		if len(result.CancelledReviewRuns) > 0 {
			latestRun := latestOnboardingReviewRun(result.CancelledReviewRuns)
			summaryJSON, err := onboardingReviewRunSummaryJSON(latestRun)
			if err != nil {
				return fmt.Errorf("build superseded onboarding review summary: %w", err)
			}
			result.Application, err = q.UpdateMerchantApplicationReviewSummary(ctx, UpdateMerchantApplicationReviewSummaryParams{
				ID:            result.Application.ID,
				ReviewSummary: summaryJSON,
			})
			if err != nil {
				return fmt.Errorf("update merchant application review summary: %w", err)
			}
		}

		// Step 3: 如果商户记录已存在，仅在非 active/approved 状态下改为 pending
		merchant, err := q.GetMerchantByOwner(ctx, arg.UserID)
		if err == nil {
			if merchant.Status != "active" && merchant.Status != "approved" {
				result.Merchant, err = q.UpdateMerchantStatus(ctx, UpdateMerchantStatusParams{
					ID:     merchant.ID,
					Status: "pending",
				})
				if err != nil {
					return fmt.Errorf("update merchant status: %w", err)
				}
			}
		} else if !errors.Is(err, ErrRecordNotFound) {
			return fmt.Errorf("get merchant: %w", err)
		}

		return nil
	})

	return result, err
}

func latestOnboardingReviewRun(runs []OnboardingReviewRun) OnboardingReviewRun {
	latest := runs[0]
	for _, run := range runs[1:] {
		if run.CreatedAt.After(latest.CreatedAt) || (run.CreatedAt.Equal(latest.CreatedAt) && run.ID > latest.ID) {
			latest = run
		}
	}
	return latest
}

func onboardingReviewRunSummaryJSON(run OnboardingReviewRun) ([]byte, error) {
	summary := onboardingReviewRunSummary{
		RunID:         run.ID,
		Stage:         run.Stage,
		Outcome:       run.Outcome.String,
		ReasonCode:    run.ReasonCode.String,
		ReasonMessage: run.ReasonMessage.String,
		RuleHits:      append([]string(nil), run.RuleHits...),
		OCRJobRefs:    append([]int64(nil), run.OcrJobRefs...),
		CreatedAt:     run.CreatedAt.UTC().Format(time.RFC3339),
	}
	return json.Marshal(summary)
}
