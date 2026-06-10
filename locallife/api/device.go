package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/merrydance/locallife/cloudprint"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	orderDisplayConfigPrintAutoAcceptConstraint = "order_display_configs_print_auto_accept_check"
	orderDisplayConfigPrintAutoAcceptMessage    = "关闭打印后不能启用自动接单，请刷新设置后重试"
	printerTypeFeieyun                          = string(cloudprint.ProviderFeieyun)
	printerTypeYilianyun                        = string(cloudprint.ProviderYilianyun)
	printerTypeShangpeng                        = string(cloudprint.ProviderShangpeng)
)

// ==================== 注册打印机 ====================

type createPrinterRequest struct {
	PrinterName      string `json:"printer_name" binding:"required,max=100"`
	PrinterSN        string `json:"printer_sn" binding:"required,max=100"`
	PrinterKey       string `json:"printer_key" binding:"required,max=100"`
	PrinterType      string `json:"printer_type" binding:"required,oneof=feieyun shangpeng"`
	PrinterRole      string `json:"printer_role" binding:"omitempty,oneof=front kitchen"`
	PrintTakeout     *bool  `json:"print_takeout"`
	PrintDineIn      *bool  `json:"print_dine_in"`
	PrintReservation *bool  `json:"print_reservation"`
}

type printerResponse struct {
	ID               int64  `json:"id"`
	MerchantID       int64  `json:"merchant_id"`
	PrinterName      string `json:"printer_name"`
	PrinterSN        string `json:"printer_sn"`
	PrinterType      string `json:"printer_type"`
	PrinterRole      string `json:"printer_role"`
	PrintTakeout     bool   `json:"print_takeout"`
	PrintDineIn      bool   `json:"print_dine_in"`
	PrintReservation bool   `json:"print_reservation"`
	IsActive         bool   `json:"is_active"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

type printerListResponse struct {
	Printers interface{} `json:"printers"`
	Total    int         `json:"total"`
}

type printerTestNotImplementedResponse struct {
	Error       string `json:"error"`
	PrinterName string `json:"printer_name"`
	PrinterSN   string `json:"printer_sn"`
	PrinterType string `json:"printer_type"`
}

type printerLiveStatusResponse struct {
	PrinterID      int64     `json:"printer_id"`
	PrinterName    string    `json:"printer_name"`
	PrinterSN      string    `json:"printer_sn"`
	PrinterType    string    `json:"printer_type"`
	ProviderStatus string    `json:"provider_status"`
	Online         bool      `json:"online"`
	Working        bool      `json:"working"`
	Model          *string   `json:"model,omitempty"`
	PrintLogo      *bool     `json:"print_logo,omitempty"`
	ScanSwitch     *bool     `json:"scan_switch,omitempty"`
	CheckedAt      time.Time `json:"checked_at"`
	InfoStatus     *string   `json:"info_status,omitempty"`
}

// createPrinter 注册打印机设备
// @Summary 注册打印机
// @Description 商户注册新的云打印机设备，支持飞鹅云和商鹏云；易联云需通过授权流程绑定
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body createPrinterRequest true "打印机信息"
// @Success 201 {object} printerResponse "成功注册打印机"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
// @Failure 409 {object} map[string]interface{} "打印机序列号已存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/devices [post]
func (server *Server) createPrinter(ctx *gin.Context) {
	var req createPrinterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if !isSupportedCloudPrinterProviderType(req.PrinterType) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("unsupported printer type")))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查打印机序列号是否已存在
	_, err = server.store.GetCloudPrinterBySN(ctx, req.PrinterSN)
	if err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("printer serial number already registered")))
		return
	}
	if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 设置默认值
	printTakeout := true
	if req.PrintTakeout != nil {
		printTakeout = *req.PrintTakeout
	}
	printDineIn := true
	if req.PrintDineIn != nil {
		printDineIn = *req.PrintDineIn
	}
	printReservation := true
	if req.PrintReservation != nil {
		printReservation = *req.PrintReservation
	}
	printerRole := req.PrinterRole
	if printerRole == "" {
		printerRole = "front"
	}

	printerProvider, registeredRemotely := server.cloudPrinterProvider(req.PrinterType)
	if req.PrinterType != printerTypeFeieyun && !registeredRemotely {
		ctx.JSON(http.StatusNotImplemented, errorResponse(errors.New("current printer type or environment does not support remote registration")))
		return
	}
	if registeredRemotely {
		if err := printerProvider.AddPrinter(ctx, cloudprint.AddPrinterInput{
			SN:       req.PrinterSN,
			Key:      req.PrinterKey,
			Name:     req.PrinterName,
			Business: cloudPrinterProviderBusinessKey(merchant.ID),
		}); err != nil {
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "cloud printer provider unavailable", "cloud printer add failed"))
			return
		}
	}

	// 创建打印机
	printer, err := server.store.CreateCloudPrinter(ctx, db.CreateCloudPrinterParams{
		MerchantID:       merchant.ID,
		PrinterName:      req.PrinterName,
		PrinterSn:        req.PrinterSN,
		PrinterKey:       req.PrinterKey,
		PrinterType:      req.PrinterType,
		PrinterRole:      printerRole,
		PrintTakeout:     printTakeout,
		PrintDineIn:      printDineIn,
		PrintReservation: printReservation,
	})
	if err != nil {
		if registeredRemotely {
			server.recordPrinterReconciliationJob(ctx, printerReconciliationJobRecordInput{
				MerchantID:    merchant.ID,
				PrinterName:   req.PrinterName,
				PrinterSN:     req.PrinterSN,
				PrinterType:   req.PrinterType,
				DesiredAction: db.CloudPrinterReconciliationActionRemove,
				SourceAction:  db.CloudPrinterReconciliationSourceCreate,
				FailureReason: buildPrinterStateDriftFailureReason(err),
				LastError:     err.Error(),
			})
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := toPrinterResponse(printer)
	ctx.JSON(http.StatusCreated, resp)
}

// ==================== 获取打印机列表 ====================

type listPrintersRequest struct {
	OnlyActive *bool `form:"only_active"`
}

// listPrinters 获取商户打印机列表
// @Summary 获取打印机列表
// @Description 商户获取已注册的打印机设备列表
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param only_active query bool false "是否只返回启用的打印机"
// @Success 200 {object} map[string]interface{} "成功返回打印机列表"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/devices [get]
func (server *Server) listPrinters(ctx *gin.Context) {
	var req listPrintersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 根据参数查询
	var printers []db.CloudPrinter
	if req.OnlyActive != nil && *req.OnlyActive {
		printers, err = server.store.ListActiveCloudPrintersByMerchant(ctx, merchant.ID)
	} else {
		printers, err = server.store.ListCloudPrintersByMerchant(ctx, merchant.ID)
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result := make([]printerResponse, len(printers))
	for i, printer := range printers {
		result[i] = toPrinterResponse(printer)
	}

	ctx.JSON(http.StatusOK, printerListResponse{Printers: result, Total: len(result)})
}

// ==================== 获取单个打印机 ====================

type getPrinterRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getPrinter 获取单个打印机详情
// @Summary 获取打印机详情
// @Description 商户获取指定打印机的详细信息
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "打印机ID"
// @Success 200 {object} printerResponse "成功返回打印机详情"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "打印机不属于该商户"
// @Failure 404 {object} map[string]interface{} "打印机不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/devices/{id} [get]
func (server *Server) getPrinter(ctx *gin.Context) {
	var req getPrinterRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取打印机
	printer, err := server.store.GetCloudPrinter(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("printer not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证归属
	if printer.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("printer does not belong to this merchant")))
		return
	}

	resp := toPrinterResponse(printer)
	ctx.JSON(http.StatusOK, resp)
}

// getPrinterLiveStatus 获取打印机实时状态
// @Summary 获取打印机实时状态
// @Description 向云打印平台查询打印机在线状态与基础能力信息；易联云授权型打印机暂不支持此实时查询
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "打印机ID"
// @Success 200 {object} printerLiveStatusResponse "成功返回实时状态"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "打印机不属于该商户"
// @Failure 404 {object} map[string]interface{} "打印机不存在"
// @Failure 501 {object} map[string]interface{} "当前环境不支持查询"
// @Failure 502 {object} map[string]interface{} "云打印平台调用失败"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/devices/{id}/status [get]
func (server *Server) getPrinterLiveStatus(ctx *gin.Context) {
	var req getPrinterRequest
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

	printer, err := server.store.GetCloudPrinter(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("printer not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if printer.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("printer does not belong to this merchant")))
		return
	}
	printerProvider, ok := server.cloudPrinterProvider(printer.PrinterType)
	if !ok {
		ctx.JSON(http.StatusNotImplemented, errorResponse(errors.New("current printer type or environment does not support live status query")))
		return
	}

	var providerStatus string
	var info cloudprint.PrinterInfo
	if printer.PrinterType == printerTypeShangpeng {
		info, err = printerProvider.GetPrinterInfo(ctx, printer.PrinterSn)
		if err != nil {
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "cloud printer status unavailable", "cloud printer info query failed"))
			return
		}
		providerStatus = info.Status
	} else {
		providerStatus, err = printerProvider.QueryPrinterStatus(ctx, printer.PrinterSn)
		if err != nil {
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "cloud printer status unavailable", "cloud printer status query failed"))
			return
		}
		info, err = printerProvider.GetPrinterInfo(ctx, printer.PrinterSn)
		if err != nil {
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "cloud printer status unavailable", "cloud printer info query failed"))
			return
		}
	}

	online, working := interpretCloudPrinterStatus(printer.PrinterType, providerStatus)
	resp := printerLiveStatusResponse{
		PrinterID:      printer.ID,
		PrinterName:    printer.PrinterName,
		PrinterSN:      printer.PrinterSn,
		PrinterType:    printer.PrinterType,
		ProviderStatus: providerStatus,
		Online:         online,
		Working:        working,
		PrintLogo:      info.PrintLogo,
		ScanSwitch:     info.ScanSwitch,
		CheckedAt:      time.Now(),
	}
	if info.Model != "" {
		model := info.Model
		resp.Model = &model
	}
	if info.Status != "" {
		status := info.Status
		resp.InfoStatus = &status
	}

	ctx.JSON(http.StatusOK, resp)
}

// ==================== 更新打印机 ====================

type updatePrinterRequest struct {
	PrinterName      string `json:"printer_name" binding:"omitempty,max=100"`
	PrinterKey       string `json:"printer_key" binding:"omitempty,max=100"`
	PrinterRole      string `json:"printer_role" binding:"omitempty,oneof=front kitchen"`
	PrintTakeout     *bool  `json:"print_takeout"`
	PrintDineIn      *bool  `json:"print_dine_in"`
	PrintReservation *bool  `json:"print_reservation"`
	IsActive         *bool  `json:"is_active"`
}

// updatePrinter 更新打印机配置
// @Summary 更新打印机配置
// @Description 商户更新打印机的配置信息
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "打印机ID"
// @Param request body updatePrinterRequest true "更新内容"
// @Success 200 {object} printerResponse "更新成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "打印机不属于该商户"
// @Failure 404 {object} map[string]interface{} "打印机不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/devices/{id} [put]
func (server *Server) updatePrinter(ctx *gin.Context) {
	var uriReq getPrinterRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updatePrinterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取打印机验证归属
	printer, err := server.store.GetCloudPrinter(ctx, uriReq.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("printer not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if printer.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("printer does not belong to this merchant")))
		return
	}

	// 构建更新参数
	updateParams := db.UpdateCloudPrinterParams{
		ID: uriReq.ID,
	}

	if req.PrinterName != "" {
		updateParams.PrinterName = pgtype.Text{String: req.PrinterName, Valid: true}
	}
	if req.PrinterKey != "" {
		updateParams.PrinterKey = pgtype.Text{String: req.PrinterKey, Valid: true}
	}
	if req.PrinterRole != "" {
		updateParams.PrinterRole = pgtype.Text{String: req.PrinterRole, Valid: true}
	}
	if req.PrintTakeout != nil {
		updateParams.PrintTakeout = pgtype.Bool{Bool: *req.PrintTakeout, Valid: true}
	}
	if req.PrintDineIn != nil {
		updateParams.PrintDineIn = pgtype.Bool{Bool: *req.PrintDineIn, Valid: true}
	}
	if req.PrintReservation != nil {
		updateParams.PrintReservation = pgtype.Bool{Bool: *req.PrintReservation, Valid: true}
	}
	if req.IsActive != nil {
		updateParams.IsActive = pgtype.Bool{Bool: *req.IsActive, Valid: true}
	}

	// 更新打印机
	updatedPrinter, err := server.store.UpdateCloudPrinter(ctx, updateParams)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := toPrinterResponse(updatedPrinter)
	ctx.JSON(http.StatusOK, resp)
}

// ==================== 删除打印机 ====================

// deletePrinter 删除打印机
// @Summary 删除打印机
// @Description 商户删除已注册的打印机设备
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "打印机ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "打印机不属于该商户"
// @Failure 404 {object} map[string]interface{} "打印机不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/devices/{id} [delete]
func (server *Server) deletePrinter(ctx *gin.Context) {
	var req getPrinterRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取打印机验证归属
	printer, err := server.store.GetCloudPrinter(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("printer not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if printer.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("printer does not belong to this merchant")))
		return
	}

	printerProvider, removedRemotely := server.cloudPrinterProvider(printer.PrinterType)
	if printer.PrinterType != printerTypeFeieyun && !removedRemotely {
		ctx.JSON(http.StatusNotImplemented, errorResponse(errors.New("current printer type or environment does not support remote deletion")))
		return
	}
	if removedRemotely {
		if err := printerProvider.RemovePrinter(ctx, cloudprint.RemovePrinterInput{
			SN:       printer.PrinterSn,
			Business: cloudPrinterProviderBusinessKey(merchant.ID),
		}); err != nil {
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "cloud printer provider unavailable", "cloud printer remove failed"))
			return
		}
	}

	// 删除打印机
	err = server.store.DeleteCloudPrinter(ctx, req.ID)
	if err != nil {
		if removedRemotely {
			server.recordPrinterReconciliationJob(ctx, printerReconciliationJobRecordInput{
				MerchantID:    merchant.ID,
				PrinterID:     pgtype.Int8{Int64: printer.ID, Valid: true},
				PrinterName:   printer.PrinterName,
				PrinterSN:     printer.PrinterSn,
				PrinterKey:    pgtype.Text{String: printer.PrinterKey, Valid: printer.PrinterKey != ""},
				PrinterType:   printer.PrinterType,
				DesiredAction: db.CloudPrinterReconciliationActionRegister,
				SourceAction:  db.CloudPrinterReconciliationSourceDelete,
				FailureReason: buildPrinterStateDriftFailureReason(err),
				LastError:     err.Error(),
			})
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, successMessage("printer deleted successfully"))
}

// ==================== 测试打印机 ====================

type testPrinterRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// testPrinter 测试打印机
// @Summary 测试打印机
// @Description 向飞鹅云打印机发送在线测试打印命令；未配置云打印客户端时返回暂不支持
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "打印机ID"
// @Success 200 {object} map[string]interface{} "测试命令已发送"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 403 {object} map[string]interface{} "打印机不属于该商户"
// @Failure 404 {object} map[string]interface{} "打印机不存在"
// @Failure 501 {object} map[string]interface{} "功能尚未实现"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/devices/{id}/test [post]
func (server *Server) testPrinter(ctx *gin.Context) {
	var req testPrinterRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取打印机验证归属
	printer, err := server.store.GetCloudPrinter(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("printer not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if printer.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("printer does not belong to this merchant")))
		return
	}

	// 注意：云打印测试功能尚未接入实际服务
	if printer.PrinterType != printerTypeFeieyun || server.printerClient == nil {
		ctx.JSON(http.StatusNotImplemented, printerTestNotImplementedResponse{
			Error:       "当前打印机类型或环境配置暂不支持在线测试打印",
			PrinterName: printer.PrinterName,
			PrinterSN:   printer.PrinterSn,
			PrinterType: printer.PrinterType,
		})
		return
	}

	content := "<CB><B>打印测试</B></CB><BR>乐客来福<BR>设备：" + printer.PrinterName + "<BR>时间：" + time.Now().Format("2006-01-02 15:04:05") + "<BR><CUT>"
	orderID, err := server.printerClient.Print(ctx, cloudprint.PrintInput{
		SN:      printer.PrinterSn,
		Content: content,
		Copies:  1,
	})
	if err != nil {
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "cloud printer provider unavailable", "cloud printer test print failed"))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":         "test print sent",
		"vendor_order_id": orderID,
	})
}

func interpretFeieyunPrinterStatus(status string) (bool, bool) {
	normalized := strings.TrimSpace(status)
	if normalized == "" {
		return false, false
	}
	online := strings.Contains(normalized, "在线")
	working := online && strings.Contains(normalized, "正常") && !strings.Contains(normalized, "不正常")
	return online, working
}

func interpretCloudPrinterStatus(providerType string, status string) (bool, bool) {
	switch providerType {
	case printerTypeFeieyun:
		return interpretFeieyunPrinterStatus(status)
	case printerTypeShangpeng:
		return interpretNormalizedCloudPrinterStatus(status)
	default:
		return false, false
	}
}

func interpretNormalizedCloudPrinterStatus(status string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online":
		return true, true
	case "abnormal", "busy":
		return true, false
	default:
		return false, false
	}
}

func isSupportedCloudPrinterProviderType(providerType string) bool {
	switch strings.TrimSpace(providerType) {
	case printerTypeFeieyun, printerTypeShangpeng:
		return true
	default:
		return false
	}
}

func cloudPrinterProviderBusinessKey(merchantID int64) string {
	return strconv.FormatInt(merchantID, 10)
}

func (server *Server) cloudPrinterProvider(providerType string) (cloudprint.Client, bool) {
	if server.cloudPrinterManager != nil {
		if provider, ok := server.cloudPrinterManager.Provider(providerType); ok && provider != nil {
			return provider, true
		}
	}
	if providerType == printerTypeFeieyun && server.printerClient != nil {
		return server.printerClient, true
	}
	return nil, false
}

// ==================== 订单展示配置 ====================

type getDisplayConfigResponse struct {
	ID                   int64  `json:"id"`
	MerchantID           int64  `json:"merchant_id"`
	EnablePrint          bool   `json:"enable_print"`
	PrintTakeout         bool   `json:"print_takeout"`
	PrintDineIn          bool   `json:"print_dine_in"`
	PrintReservation     bool   `json:"print_reservation"`
	PrintDispatchMode    string `json:"print_dispatch_mode"`
	PrintTriggerMode     string `json:"print_trigger_mode"`
	AutoAcceptPaidOrders bool   `json:"auto_accept_paid_orders"`
	// 旧客户端兼容响应字段；语音播报已在小程序下线，不再作为可配置能力。
	EnableVoice bool `json:"enable_voice"`
	// 旧客户端兼容响应字段；语音播报已在小程序下线，不再作为可配置能力。
	VoiceTakeout bool `json:"voice_takeout"`
	// 旧客户端兼容响应字段；语音播报已在小程序下线，不再作为可配置能力。
	VoiceDineIn bool   `json:"voice_dine_in"`
	EnableKDS   bool   `json:"enable_kds"`
	KdsURL      string `json:"kds_url,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// getDisplayConfig 获取订单展示配置
// @Summary 获取订单展示配置
// @Description 商户获取订单展示配置，包括打印、自动接单、KDS以及兼容旧客户端的语音字段
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} getDisplayConfigResponse "成功返回配置"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/display-config [get]
func (server *Server) getDisplayConfig(ctx *gin.Context) {
	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取配置
	config, err := server.store.GetOrderDisplayConfigByMerchant(ctx, merchant.ID)
	if err != nil {
		if isNotFoundError(err) {
			// 返回默认配置
			ctx.JSON(http.StatusOK, getDisplayConfigResponse{
				MerchantID:           merchant.ID,
				EnablePrint:          true,
				PrintTakeout:         true,
				PrintDineIn:          true,
				PrintReservation:     true,
				PrintDispatchMode:    "single_full",
				PrintTriggerMode:     "accepted",
				AutoAcceptPaidOrders: false,
				EnableVoice:          false,
				VoiceTakeout:         true,
				VoiceDineIn:          true,
				EnableKDS:            false,
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	kdsURL := ""
	if config.KdsUrl.Valid {
		kdsURL = config.KdsUrl.String
	}

	updatedAt := ""
	if config.UpdatedAt.Valid {
		updatedAt = config.UpdatedAt.Time.Format(time.RFC3339)
	}

	resp := getDisplayConfigResponse{
		ID:                   config.ID,
		MerchantID:           config.MerchantID,
		EnablePrint:          config.EnablePrint,
		PrintTakeout:         config.PrintTakeout,
		PrintDineIn:          config.PrintDineIn,
		PrintReservation:     config.PrintReservation,
		PrintDispatchMode:    config.PrintDispatchMode,
		PrintTriggerMode:     config.PrintTriggerMode,
		AutoAcceptPaidOrders: config.AutoAcceptPaidOrders,
		EnableVoice:          config.EnableVoice,
		VoiceTakeout:         config.VoiceTakeout,
		VoiceDineIn:          config.VoiceDineIn,
		EnableKDS:            config.EnableKds,
		KdsURL:               kdsURL,
		CreatedAt:            config.CreatedAt.Format(time.RFC3339),
		UpdatedAt:            updatedAt,
	}

	ctx.JSON(http.StatusOK, resp)
}

// ==================== 更新订单展示配置 ====================

type updateDisplayConfigRequest struct {
	EnablePrint          *bool   `json:"enable_print"`
	PrintTakeout         *bool   `json:"print_takeout"`
	PrintDineIn          *bool   `json:"print_dine_in"`
	PrintReservation     *bool   `json:"print_reservation"`
	PrintDispatchMode    *string `json:"print_dispatch_mode" binding:"omitempty,oneof=single_full split"`
	PrintTriggerMode     *string `json:"print_trigger_mode" binding:"omitempty,oneof=accepted ready manual"`
	AutoAcceptPaidOrders *bool   `json:"auto_accept_paid_orders"`
	// Deprecated/no-op: 旧客户端兼容请求字段，后端接受但忽略，不再更新语音配置。
	EnableVoice *bool `json:"enable_voice"`
	// Deprecated/no-op: 旧客户端兼容请求字段，后端接受但忽略，不再更新语音配置。
	VoiceTakeout *bool `json:"voice_takeout"`
	// Deprecated/no-op: 旧客户端兼容请求字段，后端接受但忽略，不再更新语音配置。
	VoiceDineIn *bool   `json:"voice_dine_in"`
	EnableKDS   *bool   `json:"enable_kds"`
	KdsURL      *string `json:"kds_url" binding:"omitempty,url,max=500"`
}

// updateDisplayConfig 更新订单展示配置
// @Summary 更新订单展示配置
// @Description 商户更新订单展示配置，包括打印、自动接单、KDS等设置；关闭打印时自动接单会同步关闭；语音字段仅兼容旧客户端请求并保持为 no-op
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body updateDisplayConfigRequest true "配置更新内容"
// @Success 200 {object} getDisplayConfigResponse "更新成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
// @Failure 409 {object} map[string]interface{} "配置状态冲突"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /v1/merchant/display-config [put]
func (server *Server) updateDisplayConfig(ctx *gin.Context) {
	var req updateDisplayConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否已有配置
	existingConfig, err := server.store.GetOrderDisplayConfigByMerchant(ctx, merchant.ID)
	configNotFound := isNotFoundError(err)
	if err != nil && !configNotFound {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var config db.OrderDisplayConfig
	if configNotFound {
		// 创建新配置
		createParams := db.CreateOrderDisplayConfigParams{
			MerchantID:           merchant.ID,
			EnablePrint:          true,
			PrintTakeout:         true,
			PrintDineIn:          true,
			PrintReservation:     true,
			PrintDispatchMode:    "single_full",
			PrintTriggerMode:     "accepted",
			AutoAcceptPaidOrders: false,
			EnableVoice:          false,
			VoiceTakeout:         true,
			VoiceDineIn:          true,
			EnableKds:            false,
		}

		// 应用请求中的值
		if req.EnablePrint != nil {
			createParams.EnablePrint = *req.EnablePrint
		}
		if req.PrintTakeout != nil {
			createParams.PrintTakeout = *req.PrintTakeout
		}
		if req.PrintDineIn != nil {
			createParams.PrintDineIn = *req.PrintDineIn
		}
		if req.PrintReservation != nil {
			createParams.PrintReservation = *req.PrintReservation
		}
		if req.PrintDispatchMode != nil {
			createParams.PrintDispatchMode = *req.PrintDispatchMode
		}
		if req.PrintTriggerMode != nil {
			createParams.PrintTriggerMode = *req.PrintTriggerMode
		}
		if req.AutoAcceptPaidOrders != nil {
			createParams.AutoAcceptPaidOrders = *req.AutoAcceptPaidOrders
		}
		if req.EnableKDS != nil {
			createParams.EnableKds = *req.EnableKDS
		}
		if req.KdsURL != nil {
			createParams.KdsUrl = pgtype.Text{String: *req.KdsURL, Valid: true}
		}
		if !createParams.EnablePrint {
			createParams.AutoAcceptPaidOrders = false
		}

		config, err = server.store.CreateOrderDisplayConfig(ctx, createParams)
		if err != nil {
			if isOrderDisplayConfigPrintAutoAcceptConstraintError(err) {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New(orderDisplayConfigPrintAutoAcceptMessage)))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	} else {
		// 更新现有配置
		updateParams := db.UpdateOrderDisplayConfigParams{
			MerchantID: merchant.ID,
		}
		effectiveEnablePrint := existingConfig.EnablePrint

		if req.EnablePrint != nil {
			updateParams.EnablePrint = pgtype.Bool{Bool: *req.EnablePrint, Valid: true}
			effectiveEnablePrint = *req.EnablePrint
		}
		if req.PrintTakeout != nil {
			updateParams.PrintTakeout = pgtype.Bool{Bool: *req.PrintTakeout, Valid: true}
		}
		if req.PrintDineIn != nil {
			updateParams.PrintDineIn = pgtype.Bool{Bool: *req.PrintDineIn, Valid: true}
		}
		if req.PrintReservation != nil {
			updateParams.PrintReservation = pgtype.Bool{Bool: *req.PrintReservation, Valid: true}
		}
		if req.PrintDispatchMode != nil {
			updateParams.PrintDispatchMode = pgtype.Text{String: *req.PrintDispatchMode, Valid: true}
		}
		if req.PrintTriggerMode != nil {
			updateParams.PrintTriggerMode = pgtype.Text{String: *req.PrintTriggerMode, Valid: true}
		}
		if req.AutoAcceptPaidOrders != nil {
			updateParams.AutoAcceptPaidOrders = pgtype.Bool{Bool: *req.AutoAcceptPaidOrders, Valid: true}
		}
		if req.EnableKDS != nil {
			updateParams.EnableKds = pgtype.Bool{Bool: *req.EnableKDS, Valid: true}
		}
		if req.KdsURL != nil {
			updateParams.KdsUrl = pgtype.Text{String: *req.KdsURL, Valid: true}
		}
		if !effectiveEnablePrint {
			updateParams.AutoAcceptPaidOrders = pgtype.Bool{Bool: false, Valid: true}
		}

		config, err = server.store.UpdateOrderDisplayConfig(ctx, updateParams)
		if err != nil {
			if isOrderDisplayConfigPrintAutoAcceptConstraintError(err) {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New(orderDisplayConfigPrintAutoAcceptMessage)))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	kdsURL := ""
	if config.KdsUrl.Valid {
		kdsURL = config.KdsUrl.String
	}

	updatedAt := ""
	if config.UpdatedAt.Valid {
		updatedAt = config.UpdatedAt.Time.Format(time.RFC3339)
	}

	resp := getDisplayConfigResponse{
		ID:                   config.ID,
		MerchantID:           config.MerchantID,
		EnablePrint:          config.EnablePrint,
		PrintTakeout:         config.PrintTakeout,
		PrintDineIn:          config.PrintDineIn,
		PrintReservation:     config.PrintReservation,
		PrintDispatchMode:    config.PrintDispatchMode,
		PrintTriggerMode:     config.PrintTriggerMode,
		AutoAcceptPaidOrders: config.AutoAcceptPaidOrders,
		EnableVoice:          config.EnableVoice,
		VoiceTakeout:         config.VoiceTakeout,
		VoiceDineIn:          config.VoiceDineIn,
		EnableKDS:            config.EnableKds,
		KdsURL:               kdsURL,
		CreatedAt:            config.CreatedAt.Format(time.RFC3339),
		UpdatedAt:            updatedAt,
	}

	ctx.JSON(http.StatusOK, resp)
}

func isOrderDisplayConfigPrintAutoAcceptConstraintError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23514" &&
		pgErr.ConstraintName == orderDisplayConfigPrintAutoAcceptConstraint
}

// ==================== 辅助函数 ====================

func toPrinterResponse(printer db.CloudPrinter) printerResponse {
	updatedAt := ""
	if printer.UpdatedAt.Valid {
		updatedAt = printer.UpdatedAt.Time.Format(time.RFC3339)
	}

	return printerResponse{
		ID:               printer.ID,
		MerchantID:       printer.MerchantID,
		PrinterName:      printer.PrinterName,
		PrinterSN:        printer.PrinterSn,
		PrinterType:      printer.PrinterType,
		PrinterRole:      printer.PrinterRole,
		PrintTakeout:     printer.PrintTakeout,
		PrintDineIn:      printer.PrintDineIn,
		PrintReservation: printer.PrintReservation,
		IsActive:         printer.IsActive,
		CreatedAt:        printer.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        updatedAt,
	}
}
