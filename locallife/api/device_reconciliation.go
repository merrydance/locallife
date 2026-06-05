package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/rs/zerolog/log"
)

type printerReconciliationJobRecordInput struct {
	MerchantID    int64
	PrinterID     pgtype.Int8
	PrinterName   string
	PrinterSN     string
	PrinterKey    pgtype.Text
	PrinterType   string
	DesiredAction string
	SourceAction  string
	FailureReason string
	LastError     string
}

type listPrinterReconciliationJobsRequest struct {
	Status string `form:"status" binding:"omitempty,oneof=pending resolved"`
}

type retryPrinterReconciliationJobRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type printerReconciliationJobResponse struct {
	ID            int64      `json:"id"`
	PrinterID     *int64     `json:"printer_id,omitempty"`
	PrinterName   string     `json:"printer_name"`
	PrinterSN     string     `json:"printer_sn"`
	PrinterType   string     `json:"printer_type"`
	DesiredAction string     `json:"desired_action"`
	SourceAction  string     `json:"source_action"`
	Status        string     `json:"status"`
	FailureReason string     `json:"failure_reason"`
	LastError     string     `json:"last_error"`
	RetryCount    int32      `json:"retry_count"`
	CanRetry      bool       `json:"can_retry"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
}

type printerReconciliationJobListResponse struct {
	Jobs  []printerReconciliationJobResponse `json:"jobs"`
	Total int                                `json:"total"`
}

func buildPrinterStateDriftFailureReason(localErr error) string {
	return fmt.Sprintf("local change failed after remote change succeeded: %v", localErr)
}

func (server *Server) recordPrinterReconciliationJob(ctx *gin.Context, input printerReconciliationJobRecordInput) {
	if _, err := server.store.UpsertCloudPrinterReconciliationJob(ctx, db.UpsertCloudPrinterReconciliationJobParams{
		MerchantID:    input.MerchantID,
		PrinterID:     input.PrinterID,
		PrinterName:   input.PrinterName,
		PrinterSn:     input.PrinterSN,
		PrinterKey:    input.PrinterKey,
		PrinterType:   input.PrinterType,
		DesiredAction: input.DesiredAction,
		SourceAction:  input.SourceAction,
		FailureReason: input.FailureReason,
		LastError:     input.LastError,
	}); err != nil {
		log.Error().Err(err).Int64("merchant_id", input.MerchantID).Str("printer_sn", input.PrinterSN).Msg("persist printer reconciliation job failed")
	}
}

func toPrinterReconciliationJobResponse(job db.CloudPrinterReconciliationJob) printerReconciliationJobResponse {
	resp := printerReconciliationJobResponse{
		ID:            job.ID,
		PrinterName:   job.PrinterName,
		PrinterSN:     job.PrinterSn,
		PrinterType:   job.PrinterType,
		DesiredAction: job.DesiredAction,
		SourceAction:  job.SourceAction,
		Status:        job.Status,
		FailureReason: job.FailureReason,
		LastError:     job.LastError,
		RetryCount:    job.RetryCount,
		CanRetry:      job.Status == db.CloudPrinterReconciliationStatusPending,
		CreatedAt:     job.CreatedAt,
		UpdatedAt:     job.UpdatedAt,
	}
	if job.PrinterID.Valid {
		printerID := job.PrinterID.Int64
		resp.PrinterID = &printerID
	}
	if job.ResolvedAt.Valid {
		resolvedAt := job.ResolvedAt.Time
		resp.ResolvedAt = &resolvedAt
	}
	return resp
}

// listPrinterReconciliationJobs 获取打印机异常对账任务
// @Summary 获取打印机异常对账任务
// @Description 商户查看云打印机远端补偿失败后待处理的对账任务，可选查看已解决任务
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param status query string false "任务状态，默认 pending" Enums(pending,resolved)
// @Success 200 {object} printerReconciliationJobListResponse "成功返回任务列表"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/devices/reconciliation-jobs [get]
func (server *Server) listPrinterReconciliationJobs(ctx *gin.Context) {
	var req listPrinterReconciliationJobsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	status := pgtype.Text{String: db.CloudPrinterReconciliationStatusPending, Valid: true}
	if req.Status != "" {
		status = pgtype.Text{String: req.Status, Valid: true}
	}

	jobs, err := server.store.ListCloudPrinterReconciliationJobsByMerchant(ctx, db.ListCloudPrinterReconciliationJobsByMerchantParams{
		MerchantID: merchant.ID,
		Status:     status,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]printerReconciliationJobResponse, len(jobs))
	for i := range jobs {
		resp[i] = toPrinterReconciliationJobResponse(jobs[i])
	}

	ctx.JSON(http.StatusOK, printerReconciliationJobListResponse{Jobs: resp, Total: len(resp)})
}

// retryPrinterReconciliationJob 重试打印机异常对账任务
// @Summary 重试打印机异常对账任务
// @Description 商户对远端补偿失败的打印机任务发起重试，成功后将任务标记为 resolved
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "对账任务ID"
// @Success 200 {object} printerReconciliationJobResponse "重试结果"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "任务不属于该商户"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 501 {object} map[string]interface{} "当前环境不支持重试"
// @Failure 502 {object} map[string]interface{} "云打印平台调用失败"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/devices/reconciliation-jobs/{id}/retry [post]
func (server *Server) retryPrinterReconciliationJob(ctx *gin.Context) {
	var req retryPrinterReconciliationJobRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	job, err := server.store.GetCloudPrinterReconciliationJob(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reconciliation job not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if job.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("reconciliation job does not belong to this merchant")))
		return
	}
	if job.Status == db.CloudPrinterReconciliationStatusResolved {
		ctx.JSON(http.StatusOK, toPrinterReconciliationJobResponse(job))
		return
	}
	printerProvider, ok := server.cloudPrinterProvider(job.PrinterType)
	if !ok {
		ctx.JSON(http.StatusNotImplemented, errorResponse(errors.New("current printer type or environment does not support reconciliation retry")))
		return
	}

	switch job.DesiredAction {
	case db.CloudPrinterReconciliationActionRemove:
		err = printerProvider.RemovePrinter(ctx, cloudprint.RemovePrinterInput{
			SN:       job.PrinterSn,
			Business: cloudPrinterProviderBusinessKey(merchant.ID),
		})
	case db.CloudPrinterReconciliationActionRegister:
		if !job.PrinterKey.Valid || job.PrinterKey.String == "" {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("printer key missing for register reconciliation")))
			return
		}
		err = printerProvider.AddPrinter(ctx, cloudprint.AddPrinterInput{
			SN:       job.PrinterSn,
			Key:      job.PrinterKey.String,
			Name:     job.PrinterName,
			Business: cloudPrinterProviderBusinessKey(merchant.ID),
		})
	default:
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("unsupported reconciliation action")))
		return
	}
	if err != nil {
		updatedJob, updateErr := server.store.FailCloudPrinterReconciliationJobRetry(ctx, db.FailCloudPrinterReconciliationJobRetryParams{
			ID:        job.ID,
			LastError: err.Error(),
		})
		if updateErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, updateErr))
			return
		}
		ctx.JSON(http.StatusBadGateway, gin.H{
			"code":    CodeBadGateway,
			"message": http.StatusText(http.StatusBadGateway),
			"data": gin.H{
				"error": errorResponse(err),
				"job":   toPrinterReconciliationJobResponse(updatedJob),
			},
		})
		return
	}

	resolvedJob, err := server.store.ResolveCloudPrinterReconciliationJob(ctx, job.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, toPrinterReconciliationJobResponse(resolvedJob))
}
