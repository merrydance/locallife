package api

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

const (
	miniProgramMediaCheckScene   = 2
	mediaModerationPrivateURLTTL = 15 * time.Minute
)

type miniProgramMediaCheckXML struct {
	XMLName xml.Name `xml:"xml"`
	AppID   string   `xml:"AppID"`
	TraceID string   `xml:"trace_id"`
	Result  struct {
		Suggest string `xml:"suggest"`
		Label   string `xml:"label"`
	} `xml:"result"`
	IsRisky string `xml:"isrisky"`
	MsgType string `xml:"MsgType"`
	Event   string `xml:"Event"`
}

func (server *Server) publicVariantsForAsset(asset db.MediaAsset) map[string]string {
	if asset.Visibility != string(media.VisibilityPublic) || asset.ModerationStatus != "approved" {
		return nil
	}

	return map[string]string{
		"thumb":    server.mediaResolver.PublicURL(asset.ObjectKey, media.VariantThumb),
		"card":     server.mediaResolver.PublicURL(asset.ObjectKey, media.VariantCard),
		"detail":   server.mediaResolver.PublicURL(asset.ObjectKey, media.VariantDetail),
		"original": server.mediaResolver.PublicURL(asset.ObjectKey, media.VariantOriginal),
	}
}

func (server *Server) triggerMediaModeration(ctx *gin.Context, asset *db.MediaAsset, uploaderID int64) error {
	if asset == nil {
		return nil
	}
	if asset.ModerationStatus != "pending" || asset.ModerationTraceID.Valid {
		log.Info().
			Int64("media_id", asset.ID).
			Str("object_key", asset.ObjectKey).
			Str("moderation_status", asset.ModerationStatus).
			Str("moderation_trace_id", asset.ModerationTraceID.String).
			Msg("media moderation skipped because asset is already processed or queued")
		return nil
	}
	if !strings.HasPrefix(asset.MimeType, "image/") {
		log.Info().
			Int64("media_id", asset.ID).
			Str("object_key", asset.ObjectKey).
			Str("mime_type", asset.MimeType).
			Msg("media moderation skipped because asset is not an image")
		return nil
	}
	if server.config.Environment == "development" && (server.wechatClient == nil || server.config.WechatMiniAppID == "" || server.config.WechatMiniAppSecret == "") {
		log.Warn().
			Int64("media_id", asset.ID).
			Str("object_key", asset.ObjectKey).
			Str("environment", server.config.Environment).
			Bool("wechat_client_configured", server.wechatClient != nil).
			Bool("wechat_app_id_configured", server.config.WechatMiniAppID != "").
			Bool("wechat_app_secret_configured", server.config.WechatMiniAppSecret != "").
			Msg("media moderation auto-approved in development because wechat moderation is not configured")
		updated, err := server.store.SetMediaAssetModerationStatus(ctx, db.SetMediaAssetModerationStatusParams{
			ID:               asset.ID,
			ModerationStatus: "approved",
		})
		if err != nil {
			return fmt.Errorf("auto approve media moderation in development: %w", err)
		}
		*asset = updated
		log.Info().
			Int64("media_id", asset.ID).
			Str("moderation_status", asset.ModerationStatus).
			Msg("media moderation development auto-approval persisted")
		return nil
	}
	if server.wechatClient == nil || server.config.WechatMiniAppID == "" || server.config.WechatMiniAppSecret == "" {
		log.Warn().
			Int64("media_id", asset.ID).
			Str("object_key", asset.ObjectKey).
			Str("environment", server.config.Environment).
			Bool("wechat_client_configured", server.wechatClient != nil).
			Bool("wechat_app_id_configured", server.config.WechatMiniAppID != "").
			Bool("wechat_app_secret_configured", server.config.WechatMiniAppSecret != "").
			Msg("media moderation skipped because wechat moderation is not configured")
		return nil
	}

	user, err := server.store.GetUser(ctx, uploaderID)
	if err != nil {
		return fmt.Errorf("load uploader for media moderation: %w", err)
	}
	if user.WechatOpenid == "" {
		log.Error().
			Int64("media_id", asset.ID).
			Int64("uploader_id", uploaderID).
			Str("object_key", asset.ObjectKey).
			Msg("media moderation failed because uploader wechat_openid is missing")
		return fmt.Errorf("user %d missing wechat_openid for media moderation", uploaderID)
	}

	mediaURL, err := server.mediaModerationSourceURL(ctx, *asset)
	if err != nil {
		return err
	}

	log.Info().
		Int64("media_id", asset.ID).
		Int64("uploader_id", uploaderID).
		Str("object_key", asset.ObjectKey).
		Str("mime_type", asset.MimeType).
		Str("visibility", asset.Visibility).
		Str("source_client", asset.SourceClient).
		Msg("requesting async media moderation")

	result, err := server.wechatClient.MediaCheckAsync(ctx, wechat.MediaCheckAsyncRequest{
		MediaURL:  mediaURL,
		MediaType: wechat.SecCheckMediaTypeImage,
		Version:   2,
		OpenID:    user.WechatOpenid,
		Scene:     miniProgramMediaCheckScene,
	})
	if err != nil {
		log.Error().
			Err(err).
			Int64("media_id", asset.ID).
			Int64("uploader_id", uploaderID).
			Str("object_key", asset.ObjectKey).
			Msg("async media moderation request failed")
		return fmt.Errorf("request wechat media moderation: %w", err)
	}

	log.Info().
		Int64("media_id", asset.ID).
		Int64("uploader_id", uploaderID).
		Str("object_key", asset.ObjectKey).
		Str("trace_id", result.TraceID).
		Msg("async media moderation request accepted")

	updated, err := server.store.SetMediaAssetModerationTraceID(ctx, db.SetMediaAssetModerationTraceIDParams{
		ID:                asset.ID,
		ModerationTraceID: pgtype.Text{String: result.TraceID, Valid: true},
	})
	if err != nil {
		log.Error().
			Err(err).
			Int64("media_id", asset.ID).
			Str("object_key", asset.ObjectKey).
			Str("trace_id", result.TraceID).
			Msg("persist media moderation trace id failed")
		return fmt.Errorf("persist media moderation trace id: %w", err)
	}
	*asset = updated
	log.Info().
		Int64("media_id", asset.ID).
		Str("object_key", asset.ObjectKey).
		Str("trace_id", result.TraceID).
		Msg("async media moderation trace id persisted")
	return nil
}

func (server *Server) mediaModerationSourceURL(ctx *gin.Context, asset db.MediaAsset) (string, error) {
	if asset.Visibility == string(media.VisibilityPublic) {
		return server.mediaResolver.PublicURL(asset.ObjectKey, media.VariantOriginal), nil
	}

	url, err := server.mediaRegistry.CreatePrivateAccessURL(ctx, asset.ID, mediaModerationPrivateURLTTL)
	if err != nil {
		return "", fmt.Errorf("create private access url for media moderation: %w", err)
	}
	return url, nil
}

func (server *Server) verifyMiniProgramMediaCheckWebhook(ctx *gin.Context) {
	if !server.verifyMiniProgramMessageSignature(ctx.Query("signature"), ctx.Query("timestamp"), ctx.Query("nonce")) {
		ctx.String(http.StatusUnauthorized, "invalid signature")
		return
	}
	ctx.String(http.StatusOK, ctx.Query("echostr"))
}

func (server *Server) handleMiniProgramMediaCheckNotify(ctx *gin.Context) {
	if !server.verifyMiniProgramMessageSignature(ctx.Query("signature"), ctx.Query("timestamp"), ctx.Query("nonce")) {
		ctx.String(http.StatusUnauthorized, "invalid signature")
		return
	}

	body, status, err := readWebhookBody(ctx)
	if err != nil {
		ctx.String(status, "read body failed")
		return
	}

	var payload miniProgramMediaCheckXML
	if err := xml.Unmarshal(body, &payload); err != nil {
		ctx.String(http.StatusBadRequest, "invalid xml")
		return
	}
	if payload.AppID != "" && server.config.WechatMiniAppID != "" && payload.AppID != server.config.WechatMiniAppID {
		ctx.String(http.StatusBadRequest, "appid mismatch")
		return
	}
	if payload.TraceID == "" {
		ctx.String(http.StatusBadRequest, "missing trace_id")
		return
	}

	moderationStatus := mapMediaModerationStatus(payload)
	log.Info().
		Str("trace_id", payload.TraceID).
		Str("appid", payload.AppID).
		Str("event", payload.Event).
		Str("msg_type", payload.MsgType).
		Str("suggest", payload.Result.Suggest).
		Str("label", payload.Result.Label).
		Str("is_risky", payload.IsRisky).
		Str("mapped_status", moderationStatus).
		Msg("media moderation callback received")
	asset, err := server.store.SetMediaAssetModerationStatusByTraceID(ctx, db.SetMediaAssetModerationStatusByTraceIDParams{
		ModerationTraceID: pgtype.Text{String: payload.TraceID, Valid: true},
		ModerationStatus:  moderationStatus,
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("trace_id", payload.TraceID).
			Str("suggest", payload.Result.Suggest).
			Str("label", payload.Result.Label).
			Str("mapped_status", moderationStatus).
			Msg("update media moderation status by trace id failed")
		ctx.String(http.StatusNotFound, "media asset not found")
		return
	}

	log.Info().
		Int64("media_id", asset.ID).
		Str("trace_id", payload.TraceID).
		Str("object_key", asset.ObjectKey).
		Str("moderation_status", moderationStatus).
		Str("label", payload.Result.Label).
		Msg("media moderation callback processed")
	if err := server.processPendingOCRJobsForMediaModeration(ctx, asset); err != nil {
		log.Error().
			Err(err).
			Int64("media_id", asset.ID).
			Str("trace_id", payload.TraceID).
			Str("moderation_status", moderationStatus).
			Msg("process pending ocr jobs for media moderation failed")
		ctx.String(http.StatusInternalServerError, "ocr moderation linkage failed")
		return
	}
	ctx.String(http.StatusOK, "success")
}

func mapMediaModerationStatus(payload miniProgramMediaCheckXML) string {
	suggest := strings.ToLower(strings.TrimSpace(payload.Result.Suggest))
	if suggest == "" {
		switch strings.TrimSpace(payload.IsRisky) {
		case "0", "false":
			suggest = "pass"
		case "1", "true":
			suggest = "risky"
		}
	}

	switch suggest {
	case "pass":
		return "approved"
	case "review":
		return "quarantined"
	case "risky":
		return "rejected"
	default:
		return "quarantined"
	}
}

func (server *Server) verifyMiniProgramMessageSignature(signature, timestamp, nonce string) bool {
	if server.config.WechatMiniAppMessageToken == "" {
		return true
	}
	if signature == "" || timestamp == "" || nonce == "" {
		return false
	}

	parts := []string{server.config.WechatMiniAppMessageToken, timestamp, nonce}
	sort.Strings(parts)
	hash := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(hash[:]) == signature
}
