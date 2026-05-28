package api

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type securityRejectionInput struct {
	ActorUserID int64
	ActorRole   string
	Action      string
	TargetType  string
	TargetID    int64
	MerchantID  int64
	GroupID     int64
	Reason      string
	Audit       bool
	Metadata    map[string]any
}

func (server *Server) logSecurityRejection(ctx *gin.Context, input securityRejectionInput) {
	log.Warn().
		Str("request_id", GetRequestID(ctx)).
		Str("path", ctx.Request.URL.Path).
		Str("method", ctx.Request.Method).
		Int64("actor_user_id", input.ActorUserID).
		Str("actor_role", input.ActorRole).
		Str("action", input.Action).
		Str("target_type", input.TargetType).
		Int64("target_id", input.TargetID).
		Int64("merchant_id", input.MerchantID).
		Int64("group_id", input.GroupID).
		Str("reason", input.Reason).
		Msg("security request rejected")

	if !input.Audit {
		return
	}

	metadata := map[string]any{
		"reason":      input.Reason,
		"path":        ctx.Request.URL.Path,
		"method":      ctx.Request.Method,
		"merchant_id": input.MerchantID,
		"group_id":    input.GroupID,
	}
	for key, value := range input.Metadata {
		metadata[key] = value
	}

	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: input.ActorUserID,
		ActorRole:   input.ActorRole,
		Action:      input.Action,
		TargetType:  input.TargetType,
		TargetID:    optionalAuditID(input.TargetID),
		Metadata:    metadata,
	})
}

func optionalAuditID(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}
