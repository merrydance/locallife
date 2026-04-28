package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/worker"
)

type repairProfitSharingReceiverLifecycleRequest struct {
	OwnerType    string `json:"owner_type" binding:"required,oneof=operator rider"`
	OwnerID      int64  `json:"owner_id" binding:"required,min=1"`
	DesiredState string `json:"desired_state" binding:"required,oneof=present absent"`
}

type profitSharingReceiverLifecycleTargetResponse struct {
	ID           int64  `json:"id"`
	OwnerType    string `json:"owner_type"`
	OwnerID      int64  `json:"owner_id"`
	DesiredState string `json:"desired_state"`
	SyncStatus   string `json:"sync_status"`
	AttemptCount int32  `json:"attempt_count"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type repairProfitSharingReceiverLifecycleResponse struct {
	Target   profitSharingReceiverLifecycleTargetResponse `json:"target"`
	Enqueued bool                                         `json:"enqueued"`
}

func newProfitSharingReceiverLifecycleTargetResponse(target db.ProfitSharingReceiverTarget) profitSharingReceiverLifecycleTargetResponse {
	return profitSharingReceiverLifecycleTargetResponse{
		ID:           target.ID,
		OwnerType:    target.OwnerType,
		OwnerID:      target.OwnerID,
		DesiredState: target.DesiredState,
		SyncStatus:   target.SyncStatus,
		AttemptCount: target.AttemptCount,
		CreatedAt:    target.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    target.UpdatedAt.Format(time.RFC3339),
	}
}

// repairProfitSharingReceiverLifecycle writes an owner-scoped receiver target intent and immediately enqueues its worker.
// @Summary 手工修复分账接收方生命周期
// @Description 平台按 owner 维度重放分账接收方同步，不使用 payment_order 上下文，也不在 API 请求内直接调用微信
// @Tags Platform
// @Accept json
// @Produce json
// @Param request body repairProfitSharingReceiverLifecycleRequest true "接收方生命周期修复请求"
// @Security BearerAuth
// @Success 202 {object} repairProfitSharingReceiverLifecycleResponse "已写入修复意图并入队"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 404 {object} errorRes "owner不存在"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/profit-sharing/receiver-lifecycle/repair [post]
func (server *Server) repairProfitSharingReceiverLifecycle(ctx *gin.Context) {
	var req repairProfitSharingReceiverLifecycleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	target, err := server.requestProfitSharingReceiverLifecycleTarget(ctx, req)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("receiver owner not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if server.taskDistributor == nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("profit sharing receiver target task distributor not configured")))
		return
	}
	if err := server.taskDistributor.DistributeTaskProcessProfitSharingReceiverTarget(
		ctx,
		&worker.ProfitSharingReceiverTargetPayload{TargetID: target.ID},
		asynq.MaxRetry(3),
		asynq.Queue(worker.QueueCritical),
	); err != nil {
		server.writeProfitSharingReceiverLifecycleRepairAudit(ctx, target, req, false)
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("enqueue profit sharing receiver target repair: %w", err)))
		return
	}

	server.writeProfitSharingReceiverLifecycleRepairAudit(ctx, target, req, true)
	ctx.JSON(http.StatusAccepted, repairProfitSharingReceiverLifecycleResponse{
		Target:   newProfitSharingReceiverLifecycleTargetResponse(target),
		Enqueued: true,
	})
}

func (server *Server) writeProfitSharingReceiverLifecycleRepairAudit(ctx *gin.Context, target db.ProfitSharingReceiverTarget, req repairProfitSharingReceiverLifecycleRequest, enqueued bool) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "profit_sharing_receiver_lifecycle_repair_requested",
		TargetType:  "profit_sharing_receiver_target",
		TargetID:    &target.ID,
		Metadata: map[string]any{
			"owner_type":    req.OwnerType,
			"owner_id":      req.OwnerID,
			"desired_state": req.DesiredState,
			"sync_status":   target.SyncStatus,
			"attempt_count": target.AttemptCount,
			"enqueued":      enqueued,
		},
	})
}

func (server *Server) requestProfitSharingReceiverLifecycleTarget(ctx *gin.Context, req repairProfitSharingReceiverLifecycleRequest) (db.ProfitSharingReceiverTarget, error) {
	service := server.buildProfitSharingReceiverLifecycleService()
	switch req.OwnerType {
	case db.ProfitSharingReceiverOwnerTypeOperator:
		operator, err := server.store.GetOperator(ctx, req.OwnerID)
		if err != nil {
			return db.ProfitSharingReceiverTarget{}, err
		}
		if req.DesiredState == db.ProfitSharingReceiverDesiredStatePresent {
			return service.RequestOperatorReceiverPresent(ctx, operator)
		}
		return service.RequestOperatorReceiverAbsent(ctx, operator)
	case db.ProfitSharingReceiverOwnerTypeRider:
		rider, err := server.store.GetRider(ctx, req.OwnerID)
		if err != nil {
			return db.ProfitSharingReceiverTarget{}, err
		}
		if req.DesiredState == db.ProfitSharingReceiverDesiredStatePresent {
			return service.RequestRiderReceiverPresent(ctx, rider)
		}
		return service.RequestRiderReceiverAbsent(ctx, rider)
	default:
		return db.ProfitSharingReceiverTarget{}, logic.NewRequestError(http.StatusBadRequest, errors.New("unsupported receiver owner type"))
	}
}
