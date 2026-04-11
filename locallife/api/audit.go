package api

import (
	"github.com/gin-gonic/gin"
)

func (server *Server) writeAuditLog(ctx *gin.Context, input AuditLogInput) {
	if server.auditWriter == nil {
		return
	}
	server.auditWriter.Write(ctx, input)
}
