package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
)

type createYilianyunAuthorizationSessionRequest struct {
	PrinterName string `json:"printer_name" binding:"omitempty,max=100"`
	PrinterRole string `json:"printer_role" binding:"omitempty,oneof=front kitchen"`
}

type yilianyunAuthorizationSessionResponse struct {
	SessionID    int64     `json:"session_id"`
	State        string    `json:"state"`
	AuthorizeURL string    `json:"authorize_url"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type yilianyunAuthorizationCallbackRequest struct {
	Code  string `form:"code" binding:"required"`
	State string `form:"state" binding:"required"`
}

type authorizeScannedYilianyunPrinterRequest struct {
	MachineCode      string `json:"machine_code" binding:"required,max=100"`
	QRKey            string `json:"qr_key" binding:"omitempty,max=255"`
	MSign            string `json:"msign" binding:"omitempty,max=255"`
	PrinterName      string `json:"printer_name" binding:"omitempty,max=100"`
	PrinterRole      string `json:"printer_role" binding:"omitempty,oneof=front kitchen"`
	PrintTakeout     *bool  `json:"print_takeout"`
	PrintDineIn      *bool  `json:"print_dine_in"`
	PrintReservation *bool  `json:"print_reservation"`
}

type yilianyunAuthorizationResponse struct {
	AuthorizationID  int64            `json:"authorization_id"`
	MerchantID       int64            `json:"merchant_id"`
	ProviderType     string           `json:"provider_type"`
	MachineCode      string           `json:"machine_code"`
	Status           string           `json:"status"`
	AccessExpiresAt  time.Time        `json:"access_expires_at"`
	RefreshExpiresAt time.Time        `json:"refresh_expires_at"`
	Printer          *printerResponse `json:"printer,omitempty"`
}

func (server *Server) cloudPrinterAuthorizationService() *logic.CloudPrinterAuthorizationService {
	return logic.NewCloudPrinterAuthorizationService(
		server.store,
		server.yilianyunOAuthClient,
		server.dataEncryptor,
		logic.CloudPrinterAuthorizationServiceConfig{},
	)
}

// createYilianyunAuthorizationSession 创建易联云开放应用授权会话
// @Summary 创建易联云授权会话
// @Description 商户创建一次性易联云开放应用授权 state，并返回 provider authorize_url
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body createYilianyunAuthorizationSessionRequest true "授权会话参数"
// @Success 201 {object} yilianyunAuthorizationSessionResponse "成功创建授权会话"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无商户设备管理权限"
// @Failure 503 {object} ErrorResponse "授权服务不可用"
// @Router /v1/merchant/devices/yilianyun/authorization-sessions [post]
func (server *Server) createYilianyunAuthorizationSession(ctx *gin.Context) {
	var req createYilianyunAuthorizationSessionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant context missing")))
		return
	}

	result, err := server.cloudPrinterAuthorizationService().CreateYilianyunAuthorizationSession(ctx, logic.CreateYilianyunAuthorizationSessionInput{
		MerchantID:  merchant.ID,
		CreatedBy:   authPayload.UserID,
		PrinterName: req.PrinterName,
		PrinterRole: req.PrinterRole,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusCreated, yilianyunAuthorizationSessionResponse{
		SessionID:    result.SessionID,
		State:        result.State,
		AuthorizeURL: result.AuthorizeURL,
		ExpiresAt:    result.ExpiresAt,
	})
}

// handleYilianyunAuthorizationCallback 处理易联云开放应用授权回调
// @Summary 易联云授权回调
// @Description 易联云开放平台 redirect_uri 回调；仅依赖一次性 state，不要求 Bearer
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Param code query string true "授权码"
// @Param state query string true "LocalLife 一次性授权 state"
// @Success 200 {object} yilianyunAuthorizationResponse "授权完成"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "授权会话不存在或过期"
// @Failure 409 {object} ErrorResponse "授权会话已处理"
// @Failure 502 {object} ErrorResponse "易联云授权失败"
// @Router /v1/merchant/devices/yilianyun/auth/callback [get]
func (server *Server) handleYilianyunAuthorizationCallback(ctx *gin.Context) {
	var req yilianyunAuthorizationCallbackRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	result, err := server.cloudPrinterAuthorizationService().CompleteYilianyunAuthorizationCode(ctx, logic.CompleteYilianyunAuthorizationCodeInput{
		Code:  req.Code,
		State: req.State,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusOK, newYilianyunAuthorizationResponse(result))
}

// authorizeScannedYilianyunPrinter 易联云机器码/快速授权
// @Summary 易联云机器码授权
// @Description 商户提交机器码 machine_code 和终端密钥 msign；后续有真实二维码样本后也可提交 qr_key，完成易联云打印机授权
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body authorizeScannedYilianyunPrinterRequest true "机器码授权参数"
// @Success 200 {object} yilianyunAuthorizationResponse "授权完成"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无商户设备管理权限"
// @Failure 409 {object} ErrorResponse "设备已被其他商户绑定"
// @Failure 502 {object} ErrorResponse "易联云授权失败"
// @Router /v1/merchant/devices/yilianyun/scan-authorizations [post]
func (server *Server) authorizeScannedYilianyunPrinter(ctx *gin.Context) {
	var req authorizeScannedYilianyunPrinterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant context missing")))
		return
	}
	result, err := server.cloudPrinterAuthorizationService().AuthorizeScannedYilianyunPrinter(ctx, logic.AuthorizeScannedYilianyunPrinterInput{
		MerchantID:       merchant.ID,
		MachineCode:      req.MachineCode,
		QRKey:            req.QRKey,
		MSign:            req.MSign,
		PrinterName:      req.PrinterName,
		PrinterRole:      req.PrinterRole,
		PrintTakeout:     req.PrintTakeout,
		PrintDineIn:      req.PrintDineIn,
		PrintReservation: req.PrintReservation,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusOK, newYilianyunAuthorizationResponse(result))
}

func newYilianyunAuthorizationResponse(result logic.YilianyunAuthorizationResult) yilianyunAuthorizationResponse {
	resp := yilianyunAuthorizationResponse{
		AuthorizationID:  result.AuthorizationID,
		MerchantID:       result.MerchantID,
		ProviderType:     result.ProviderType,
		MachineCode:      result.MachineCode,
		Status:           result.Status,
		AccessExpiresAt:  result.AccessExpiresAt,
		RefreshExpiresAt: result.RefreshExpiresAt,
	}
	if result.Printer.ID > 0 {
		resp.Printer = &printerResponse{
			ID:               result.Printer.ID,
			MerchantID:       result.Printer.MerchantID,
			PrinterName:      result.Printer.PrinterName,
			PrinterSN:        result.Printer.PrinterSN,
			PrinterType:      result.Printer.PrinterType,
			PrinterRole:      result.Printer.PrinterRole,
			PrintTakeout:     result.Printer.PrintTakeout,
			PrintDineIn:      result.Printer.PrintDineIn,
			PrintReservation: result.Printer.PrintReservation,
			IsActive:         result.Printer.IsActive,
			CreatedAt:        result.Printer.CreatedAt.Format(time.RFC3339),
		}
		if result.Printer.UpdatedAt.Valid {
			resp.Printer.UpdatedAt = result.Printer.UpdatedAt.Time.Format(time.RFC3339)
		}
	}
	return resp
}
