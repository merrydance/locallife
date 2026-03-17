package api

import (
	"context"
	"encoding/json"
	"sync"
	"time"

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

const (
	auditWorkerCount = 4
	auditQueueSize   = 1024
)

type auditJob struct {
	action string
	params db.CreateAuditLogParams
}

type DBAuditWriter struct {
	store db.Store
	jobs  chan auditJob
	wg    sync.WaitGroup
}

func NewDBAuditWriter(store db.Store) AuditWriter {
	w := &DBAuditWriter{
		store: store,
		jobs:  make(chan auditJob, auditQueueSize),
	}
	for i := 0; i < auditWorkerCount; i++ {
		w.wg.Add(1)
		go w.workerLoop()
	}
	return w
}

func (w *DBAuditWriter) workerLoop() {
	defer w.wg.Done()
	for job := range w.jobs {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		if _, err := w.store.CreateAuditLog(ctx, job.params); err != nil {
			log.Warn().Err(err).Str("action", job.action).Msg("failed to write audit log")
		}
		cancel()
	}
}

// Close drains the jobs channel and waits for all workers to finish.
// Must be called during server shutdown to avoid losing tail audit entries.
func (w *DBAuditWriter) Close() {
	close(w.jobs)
	w.wg.Wait()
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

	params := db.CreateAuditLogParams{
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
	}

	select {
	case w.jobs <- auditJob{action: input.Action, params: params}:
	default:
		auditLogDroppedTotal.Inc()
		log.Warn().Str("action", input.Action).Msg("audit log queue full, dropping write")
	}
}
