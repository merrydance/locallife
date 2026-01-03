package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

// ==================== Boss 认领码生成 ====================

type generateBossBindCodeResponse struct {
	BindCode  string `json:"bind_code"`
	ExpiresAt string `json:"expires_at"`
}

// generateBossBindCode 生成 Boss 认领码
// @Summary 生成 Boss 认领码
// @Description 商户老板生成认领码，让 Boss 扫码认领店铺
// @Tags Boss管理
// @Produce json
// @Success 200 {object} generateBossBindCodeResponse
// @Router /v1/merchant/boss-bind-code [post]
func (server *Server) generateBossBindCode(ctx *gin.Context) {
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant not found")))
		return
	}

	// 检查是否 owner
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	if merchant.OwnerUserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("only owner can generate boss bind code")))
		return
	}

	// 检查现有认领码是否有效
	if merchant.BossBindCode.Valid && merchant.BossBindCodeExpiresAt.Valid {
		if merchant.BossBindCodeExpiresAt.Time.After(time.Now()) {
			// 现有码仍有效，直接返回
			ctx.JSON(http.StatusOK, generateBossBindCodeResponse{
				BindCode:  merchant.BossBindCode.String,
				ExpiresAt: merchant.BossBindCodeExpiresAt.Time.Format(time.RFC3339),
			})
			return
		}
	}

	// 生成新认领码
	codeBytes := make([]byte, 8)
	if _, err := rand.Read(codeBytes); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	code := hex.EncodeToString(codeBytes)
	expiresAt := time.Now().Add(24 * time.Hour)

	// 更新商户
	_, err := server.store.UpdateMerchantBossBindCode(ctx, db.UpdateMerchantBossBindCodeParams{
		ID:                    merchant.ID,
		BossBindCode:          pgtype.Text{String: code, Valid: true},
		BossBindCodeExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, generateBossBindCodeResponse{
		BindCode:  code,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	})
}

// ==================== Boss 认领店铺 ====================

type claimBossRequest struct {
	BindCode string `json:"bind_code" binding:"required"`
}

type claimBossResponse struct {
	Message      string `json:"message"`
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
}

// claimBoss 认领店铺
// @Summary Boss 认领店铺
// @Description Boss 扫码或输入认领码认领店铺
// @Tags Boss管理
// @Accept json
// @Produce json
// @Param request body claimBossRequest true "认领请求"
// @Success 200 {object} claimBossResponse
// @Router /v1/claim-boss [post]
func (server *Server) claimBoss(ctx *gin.Context) {
	var req claimBossRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 查找商户
	merchant, err := server.store.GetMerchantByBossBindCode(ctx, pgtype.Text{String: req.BindCode, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的认领码")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查认领码是否过期
	if !merchant.BossBindCodeExpiresAt.Valid || merchant.BossBindCodeExpiresAt.Time.Before(time.Now()) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("认领码已过期")))
		return
	}

	// 检查是否已经认领
	existingBoss, err := server.store.GetMerchantBoss(ctx, db.GetMerchantBossParams{
		UserID:     authPayload.UserID,
		MerchantID: merchant.ID,
	})
	if err == nil && existingBoss.ID > 0 {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("您已经认领过该店铺")))
		return
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 创建 Boss 关系
	_, err = server.store.CreateMerchantBoss(ctx, db.CreateMerchantBossParams{
		UserID:     authPayload.UserID,
		MerchantID: merchant.ID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 添加 merchant_boss 角色到 user_roles
	server.store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          authPayload.UserID,
		Role:            "merchant_boss",
		RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
	})

	ctx.JSON(http.StatusOK, claimBossResponse{
		Message:      "认领成功",
		MerchantID:   merchant.ID,
		MerchantName: merchant.Name,
	})
}

// ==================== Boss 店铺列表 ====================

type bossMerchantResponse struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	LogoURL string `json:"logo_url,omitempty"`
	Address string `json:"address"`
	Phone   string `json:"phone"`
	Status  string `json:"status"`
}

// listBossMerchants Boss 获取关联店铺列表
// @Summary Boss 获取关联店铺
// @Description 获取当前 Boss 认领的所有店铺
// @Tags Boss管理
// @Produce json
// @Success 200 {array} bossMerchantResponse
// @Router /v1/boss/merchants [get]
func (server *Server) listBossMerchants(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	merchants, err := server.store.ListMerchantsByBoss(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]bossMerchantResponse, 0, len(merchants))
	for _, m := range merchants {
		logoURL := ""
		if m.LogoUrl.Valid {
			logoURL = m.LogoUrl.String
		}
		result = append(result, bossMerchantResponse{
			ID:      m.ID,
			Name:    m.Name,
			LogoURL: logoURL,
			Address: m.Address,
			Phone:   m.Phone,
			Status:  m.Status,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"merchants": result})
}

// ==================== Boss 查看店铺 Bosses 列表 ====================

type merchantBossResponse struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	FullName  string `json:"full_name"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// listMerchantBosses 获取店铺的所有 Boss
// @Summary 获取店铺的 Boss 列表
// @Description 商户老板查看认领该店铺的所有 Boss
// @Tags Boss管理
// @Produce json
// @Success 200 {array} merchantBossResponse
// @Router /v1/merchant/bosses [get]
func (server *Server) listMerchantBosses(ctx *gin.Context) {
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant not found")))
		return
	}

	bosses, err := server.store.ListBossesByMerchant(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]merchantBossResponse, 0, len(bosses))
	for _, b := range bosses {
		avatarURL := ""
		if b.AvatarUrl.Valid {
			avatarURL = b.AvatarUrl.String
		}
		phone := ""
		if b.Phone.Valid {
			phone = b.Phone.String
		}
		result = append(result, merchantBossResponse{
			ID:        b.ID,
			UserID:    b.UserID,
			FullName:  b.FullName,
			AvatarURL: avatarURL,
			Phone:     phone,
			Status:    b.Status,
			CreatedAt: b.CreatedAt.Format(time.RFC3339),
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"bosses": result})
}

// ==================== Boss 移除 ====================

type removeBossRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// removeBoss 移除 Boss
// @Summary 移除 Boss
// @Description 商户老板移除已认领的 Boss
// @Tags Boss管理
// @Produce json
// @Param id path int true "Boss ID"
// @Success 200 {object} gin.H
// @Router /v1/merchant/bosses/{id} [delete]
func (server *Server) removeBoss(ctx *gin.Context) {
	var req removeBossRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant not found")))
		return
	}

	// 检查是否 owner
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	if merchant.OwnerUserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("only owner can remove boss")))
		return
	}

	// 软删除
	_, err := server.store.UpdateMerchantBossStatus(ctx, db.UpdateMerchantBossStatusParams{
		ID:     req.ID,
		Status: "disabled",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Boss 已移除"})
}
