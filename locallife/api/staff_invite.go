package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const merchantStaffInviteCodeTTL = 24 * time.Hour
const merchantStaffInviteCodeGenerationAttempts = 5

type generateInviteCodeResponse struct {
	InviteCode string `json:"invite_code"`
	ExpiresAt  string `json:"expires_at"`
}

// generateInviteCode gets the current active staff invite code, or creates one when none is active.
// @Summary 获取或生成员工邀请码
// @Description 商户老板或店长获取当前有效员工邀请码；没有有效邀请码时生成一个新码
// @Tags 员工管理
// @Produce json
// @Success 200 {object} generateInviteCodeResponse "邀请码"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/staff/invite-code [post]
// @Security BearerAuth
func (server *Server) generateInviteCode(ctx *gin.Context) {
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant not found in context")))
		return
	}

	if writeActiveInviteCodeResponse(ctx, merchant, time.Now()) {
		return
	}

	existingCode := ""
	if merchant.BindCode.Valid {
		existingCode = merchant.BindCode.String
	}
	server.createInviteCodeWhenInactive(ctx, merchant.ID, existingCode)
}

// rotateInviteCode forces a new staff invite code and immediately invalidates the old code.
// @Summary 轮换员工邀请码
// @Description 商户老板或店长强制生成新的员工邀请码；旧邀请码立即失效
// @Tags 员工管理
// @Produce json
// @Success 200 {object} generateInviteCodeResponse "新邀请码"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/staff/invite-code/rotate [post]
// @Security BearerAuth
func (server *Server) rotateInviteCode(ctx *gin.Context) {
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant not found in context")))
		return
	}

	existingCode := ""
	if merchant.BindCode.Valid {
		existingCode = merchant.BindCode.String
	}
	server.createAndStoreInviteCode(ctx, merchant.ID, existingCode)
}

// revokeInviteCode clears the current staff invite code so previously shared codes stop working.
// @Summary 停用员工邀请码
// @Description 商户老板或店长停用当前员工邀请码；旧邀请码立即失效
// @Tags 员工管理
// @Produce json
// @Success 200 {object} successMessageResponse "邀请码已停用"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchant/staff/invite-code/revoke [post]
// @Security BearerAuth
func (server *Server) revokeInviteCode(ctx *gin.Context) {
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant not found in context")))
		return
	}

	_, err := server.store.UpdateMerchantBindCode(ctx, db.UpdateMerchantBindCodeParams{
		ID:                merchant.ID,
		BindCode:          pgtype.Text{},
		BindCodeExpiresAt: pgtype.Timestamptz{},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, successMessage("invite code revoked"))
}

func (server *Server) createAndStoreInviteCode(ctx *gin.Context, merchantID int64, excludedInviteCode string) {
	for attempt := 0; attempt < merchantStaffInviteCodeGenerationAttempts; attempt++ {
		nextCode, err := generateMerchantStaffInviteCode()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if excludedInviteCode != "" && nextCode == excludedInviteCode {
			continue
		}

		expiresAt := time.Now().Add(merchantStaffInviteCodeTTL)
		_, err = server.store.UpdateMerchantBindCode(ctx, db.UpdateMerchantBindCodeParams{
			ID:                merchantID,
			BindCode:          pgtype.Text{String: nextCode, Valid: true},
			BindCodeExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		})
		if err != nil {
			if db.ErrorCode(err) == db.UniqueViolation {
				continue
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		ctx.JSON(http.StatusOK, generateInviteCodeResponse{
			InviteCode: nextCode,
			ExpiresAt:  expiresAt.Format(time.RFC3339),
		})
		return
	}

	ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("failed to generate distinct invite code")))
}

func (server *Server) createInviteCodeWhenInactive(ctx *gin.Context, merchantID int64, excludedInviteCode string) {
	for attempt := 0; attempt < merchantStaffInviteCodeGenerationAttempts; attempt++ {
		nextCode, err := generateMerchantStaffInviteCode()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if excludedInviteCode != "" && nextCode == excludedInviteCode {
			continue
		}

		expiresAt := time.Now().Add(merchantStaffInviteCodeTTL)
		_, err = server.store.CreateMerchantBindCodeWhenInactive(ctx, db.CreateMerchantBindCodeWhenInactiveParams{
			ID:                merchantID,
			BindCode:          pgtype.Text{String: nextCode, Valid: true},
			BindCodeExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		})
		if err != nil {
			if db.ErrorCode(err) == db.UniqueViolation {
				continue
			}
			if errors.Is(err, db.ErrRecordNotFound) {
				currentMerchant, getErr := server.store.GetMerchant(ctx, merchantID)
				if getErr != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, getErr))
					return
				}
				if writeActiveInviteCodeResponse(ctx, currentMerchant, time.Now()) {
					return
				}
				continue
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		ctx.JSON(http.StatusOK, generateInviteCodeResponse{
			InviteCode: nextCode,
			ExpiresAt:  expiresAt.Format(time.RFC3339),
		})
		return
	}

	ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("failed to get or generate active invite code")))
}

func writeActiveInviteCodeResponse(ctx *gin.Context, merchant db.Merchant, now time.Time) bool {
	if merchant.BindCode.Valid && merchant.BindCode.String != "" &&
		merchant.BindCodeExpiresAt.Valid && merchant.BindCodeExpiresAt.Time.After(now) {
		ctx.JSON(http.StatusOK, generateInviteCodeResponse{
			InviteCode: merchant.BindCode.String,
			ExpiresAt:  merchant.BindCodeExpiresAt.Time.Format(time.RFC3339),
		})
		return true
	}
	return false
}

func generateMerchantStaffInviteCode() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
