package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
)

// AuditLogInput represents a single audit log record to be written.
// It is kept as an API-layer struct so business handlers/middlewares don't depend
// on the DB schema types.
type AuditLogInput struct {
	ActorUserID int64
	ActorRole   string
	Action      string
	TargetType  string
	TargetID    *int64
	RegionID    *int64
	Metadata    map[string]any
}

// AuditWriter writes audit logs. Implementations must be best-effort and must not
// block or fail the main request.
type AuditWriter interface {
	Write(ctx *gin.Context, input AuditLogInput)
}

type NoopAuditWriter struct{}

func (w NoopAuditWriter) Write(ctx *gin.Context, input AuditLogInput) {}

func NewNoopAuditWriter() AuditWriter {
	return NoopAuditWriter{}
}

type DBAuditWriter struct {
	store db.Store
}

func NewDBAuditWriter(store db.Store) AuditWriter {
	return &DBAuditWriter{store: store}
}

func (w *DBAuditWriter) Write(ctx *gin.Context, input AuditLogInput) {
	metadataBytes, err := json.Marshal(input.Metadata)
	if err != nil {
		log.Warn().Err(err).Str("action", input.Action).Msg("failed to marshal audit metadata")
		metadataBytes = nil
	}

	var targetID pgtype.Int8
	if input.TargetID != nil {
		targetID = pgtype.Int8{Int64: *input.TargetID, Valid: true}
	}

	var regionID pgtype.Int8
	if input.RegionID != nil {
		regionID = pgtype.Int8{Int64: *input.RegionID, Valid: true}
	}

	requestID := GetRequestID(ctx)
	clientIP := ctx.ClientIP()
	userAgent := ctx.Request.UserAgent()

	_, err = w.store.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorUserID: pgtype.Int8{Int64: input.ActorUserID, Valid: true},
		ActorRole:   input.ActorRole,
		Action:      input.Action,
		TargetType:  input.TargetType,
		TargetID:    targetID,
		RegionID:    regionID,
		RequestID:   pgtype.Text{String: requestID, Valid: requestID != ""},
		TraceID:     pgtype.Text{},
		ClientIp:    pgtype.Text{String: clientIP, Valid: clientIP != ""},
		UserAgent:   pgtype.Text{String: userAgent, Valid: userAgent != ""},
		Metadata:    metadataBytes,
	})
	if err != nil {
		log.Warn().Err(err).Str("action", input.Action).Msg("failed to write audit log")
	}
}
