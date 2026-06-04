package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/rs/zerolog/log"
)

func (server *Server) deleteRiderApplicationDocumentByType(ctx *gin.Context, documentType riderApplicationDocumentType) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	switch documentType {
	case riderApplicationDocumentIDCardFront, riderApplicationDocumentIDCardBack, riderApplicationDocumentHealthCert:
	default:
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid document type")))
		return
	}

	app, err := server.store.GetRiderApplicationByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider application by user: %w", err)))
		return
	}

	app, err = server.ensureEditableRiderApplication(app)
	if err != nil {
		if errors.Is(err, ErrApplicationNotDraft) {
			log.Warn().
				Int64("application_id", app.ID).
				Int64("user_id", authPayload.UserID).
				Str("status", app.Status).
				Str("document_type", string(documentType)).
				Bool("front_asset_bound", app.IDCardFrontMediaAssetID.Valid).
				Bool("back_asset_bound", app.IDCardBackMediaAssetID.Valid).
				Bool("health_asset_bound", app.HealthCertMediaAssetID.Valid).
				Int64("front_asset_id", app.IDCardFrontMediaAssetID.Int64).
				Int64("back_asset_id", app.IDCardBackMediaAssetID.Int64).
				Int64("health_asset_id", app.HealthCertMediaAssetID.Int64).
				Msg("reject deleting rider application document because application is not draft")
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrApplicationNotDraft))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("ensure rider application editable: %w", err)))
		return
	}

	var (
		updated db.RiderApplication
		assetID int64
	)

	switch documentType {
	case riderApplicationDocumentIDCardFront:
		if app.IDCardFrontMediaAssetID.Valid {
			assetID = app.IDCardFrontMediaAssetID.Int64
		}
		updated, err = server.store.ClearRiderApplicationIDCardFront(ctx, app.ID)
	case riderApplicationDocumentIDCardBack:
		if app.IDCardBackMediaAssetID.Valid {
			assetID = app.IDCardBackMediaAssetID.Int64
		}
		updated, err = server.store.ClearRiderApplicationIDCardBack(ctx, app.ID)
	default:
		if app.HealthCertMediaAssetID.Valid {
			assetID = app.HealthCertMediaAssetID.Int64
		}
		updated, err = server.store.ClearRiderApplicationHealthCert(ctx, app.ID)
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("clear rider application document: %w", err)))
		return
	}

	log.Info().
		Int64("application_id", updated.ID).
		Int64("user_id", authPayload.UserID).
		Str("document_type", string(documentType)).
		Int64("deleted_asset_id", assetID).
		Bool("front_asset_bound", updated.IDCardFrontMediaAssetID.Valid).
		Bool("back_asset_bound", updated.IDCardBackMediaAssetID.Valid).
		Bool("health_asset_bound", updated.HealthCertMediaAssetID.Valid).
		Int64("front_asset_id", updated.IDCardFrontMediaAssetID.Int64).
		Int64("back_asset_id", updated.IDCardBackMediaAssetID.Int64).
		Int64("health_asset_id", updated.HealthCertMediaAssetID.Int64).
		Msg("rider application document cleared")

	if assetID > 0 {
		if err := server.mediaRegistry.SoftDelete(ctx, assetID, authPayload.UserID); err != nil {
			log.Warn().Err(err).Int64("asset_id", assetID).Str("document_type", string(documentType)).Msg("delete rider application document: soft delete media failed")
		}
	}

	ctx.JSON(http.StatusOK, server.newRiderApplicationResponse(ctx, updated))
}

// deleteRiderApplicationDocument godoc
// @Summary 删除骑手申请证照
// @Description 删除骑手草稿中的单个证照绑定，并清空对应 OCR 结果。支持证照类型：id_card_front、id_card_back、health_cert。
// @Tags 骑手申请
// @Produce json
// @Param document_type path string true "证照类型: id_card_front|id_card_back|health_cert"
// @Success 200 {object} riderApplicationResponse "删除成功"
// @Failure 400 {object} ErrorResponse "参数错误或状态不允许修改"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application/documents/{document_type} [delete]
// @Security BearerAuth
func (server *Server) deleteRiderApplicationDocument(ctx *gin.Context) {
	documentType := riderApplicationDocumentType(strings.TrimSpace(ctx.Param("document_type")))
	server.deleteRiderApplicationDocumentByType(ctx, documentType)
}

// deleteRiderApplicationHealthCert godoc
// @Summary 删除骑手申请健康证
// @Description 删除骑手草稿中的健康证绑定，并清空对应 OCR 结果。
// @Tags 骑手申请
// @Produce json
// @Success 200 {object} riderApplicationResponse "删除成功"
// @Failure 400 {object} ErrorResponse "状态不允许修改"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "申请不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/application/health-cert [delete]
// @Security BearerAuth
func (server *Server) deleteRiderApplicationHealthCert(ctx *gin.Context) {
	server.deleteRiderApplicationDocumentByType(ctx, riderApplicationDocumentHealthCert)
}
