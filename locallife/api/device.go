package api

import (
	"errors"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 注册打印机 ====================

type createPrinterRequest struct {
	PrinterName      string `json:"printer_name" binding:"required,max=100"`
	PrinterSN        string `json:"printer_sn" binding:"required,max=100"`
	PrinterKey       string `json:"printer_key" binding:"required,max=100"`
	PrinterType      string `json:"printer_type" binding:"required,oneof=feieyun yilianyun other"`
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
	PrintTakeout     bool   `json:"print_takeout"`
	PrintDineIn      bool   `json:"print_dine_in"`
	PrintReservation bool   `json:"print_reservation"`
	IsActive         bool   `json:"is_active"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

// createPrinter 注册打印机设备
// @Summary 注册打印机
// @Description 商户注册新的云打印机设备，支持飞鹅云和易联云
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

	// 获取认证信息
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取商户信息
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
	if !errors.Is(err, pgx.ErrNoRows) {
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

	// 创建打印机
	printer, err := server.store.CreateCloudPrinter(ctx, db.CreateCloudPrinterParams{
		MerchantID:       merchant.ID,
		PrinterName:      req.PrinterName,
		PrinterSn:        req.PrinterSN,
		PrinterKey:       req.PrinterKey,
		PrinterType:      req.PrinterType,
		PrintTakeout:     printTakeout,
		PrintDineIn:      printDineIn,
		PrintReservation: printReservation,
	})
	if err != nil {
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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

	ctx.JSON(http.StatusOK, gin.H{
		"printers": result,
		"total":    len(result),
	})
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取打印机
	printer, err := server.store.GetCloudPrinter(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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

// ==================== 更新打印机 ====================

type updatePrinterRequest struct {
	PrinterName      string `json:"printer_name" binding:"omitempty,max=100"`
	PrinterKey       string `json:"printer_key" binding:"omitempty,max=100"`
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取打印机验证归属
	printer, err := server.store.GetCloudPrinter(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取打印机验证归属
	printer, err := server.store.GetCloudPrinter(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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

	// 删除打印机
	err = server.store.DeleteCloudPrinter(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "printer deleted successfully",
	})
}

// ==================== 测试打印机 ====================

type testPrinterRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// testPrinter 测试打印机
// @Summary 测试打印机
// @Description 向打印机发送测试打印命令（功能尚未接入云打印服务）
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取打印机验证归属
	printer, err := server.store.GetCloudPrinter(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
	// 生产环境需要根据 printer_type 调用对应的云打印服务 API:
	// - feieyun: 飞鹅云打印 API
	// - yilianyun: 易联云打印 API
	ctx.JSON(http.StatusNotImplemented, gin.H{
		"error":        "云打印测试功能尚未接入，请联系技术支持",
		"printer_name": printer.PrinterName,
		"printer_sn":   printer.PrinterSn,
		"printer_type": printer.PrinterType,
	})
}

// ==================== 订单展示配置 ====================

type getDisplayConfigResponse struct {
	ID               int64  `json:"id"`
	MerchantID       int64  `json:"merchant_id"`
	EnablePrint      bool   `json:"enable_print"`
	PrintTakeout     bool   `json:"print_takeout"`
	PrintDineIn      bool   `json:"print_dine_in"`
	PrintReservation bool   `json:"print_reservation"`
	EnableVoice      bool   `json:"enable_voice"`
	VoiceTakeout     bool   `json:"voice_takeout"`
	VoiceDineIn      bool   `json:"voice_dine_in"`
	EnableKDS        bool   `json:"enable_kds"`
	KdsURL           string `json:"kds_url,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

// getDisplayConfig 获取订单展示配置
// @Summary 获取订单展示配置
// @Description 商户获取订单展示配置，包括打印、语音播报、KDS等设置
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取配置
	config, err := server.store.GetOrderDisplayConfigByMerchant(ctx, merchant.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// 返回默认配置
			ctx.JSON(http.StatusOK, getDisplayConfigResponse{
				MerchantID:       merchant.ID,
				EnablePrint:      true,
				PrintTakeout:     true,
				PrintDineIn:      true,
				PrintReservation: true,
				EnableVoice:      false,
				VoiceTakeout:     true,
				VoiceDineIn:      true,
				EnableKDS:        false,
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
		ID:               config.ID,
		MerchantID:       config.MerchantID,
		EnablePrint:      config.EnablePrint,
		PrintTakeout:     config.PrintTakeout,
		PrintDineIn:      config.PrintDineIn,
		PrintReservation: config.PrintReservation,
		EnableVoice:      config.EnableVoice,
		VoiceTakeout:     config.VoiceTakeout,
		VoiceDineIn:      config.VoiceDineIn,
		EnableKDS:        config.EnableKds,
		KdsURL:           kdsURL,
		CreatedAt:        config.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        updatedAt,
	}

	ctx.JSON(http.StatusOK, resp)
}

// ==================== 更新订单展示配置 ====================

type updateDisplayConfigRequest struct {
	EnablePrint      *bool   `json:"enable_print"`
	PrintTakeout     *bool   `json:"print_takeout"`
	PrintDineIn      *bool   `json:"print_dine_in"`
	PrintReservation *bool   `json:"print_reservation"`
	EnableVoice      *bool   `json:"enable_voice"`
	VoiceTakeout     *bool   `json:"voice_takeout"`
	VoiceDineIn      *bool   `json:"voice_dine_in"`
	EnableKDS        *bool   `json:"enable_kds"`
	KdsURL           *string `json:"kds_url" binding:"omitempty,url,max=500"`
}

// updateDisplayConfig 更新订单展示配置
// @Summary 更新订单展示配置
// @Description 商户更新订单展示配置，包括打印、语音播报、KDS等设置
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body updateDisplayConfigRequest true "配置更新内容"
// @Success 200 {object} getDisplayConfigResponse "更新成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "商户不存在"
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否已有配置
	_, err = server.store.GetOrderDisplayConfigByMerchant(ctx, merchant.ID)
	configNotFound := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !configNotFound {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var config db.OrderDisplayConfig
	if configNotFound {
		// 创建新配置
		createParams := db.CreateOrderDisplayConfigParams{
			MerchantID:       merchant.ID,
			EnablePrint:      true,
			PrintTakeout:     true,
			PrintDineIn:      true,
			PrintReservation: true,
			EnableVoice:      false,
			VoiceTakeout:     true,
			VoiceDineIn:      true,
			EnableKds:        false,
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
		if req.EnableVoice != nil {
			createParams.EnableVoice = *req.EnableVoice
		}
		if req.VoiceTakeout != nil {
			createParams.VoiceTakeout = *req.VoiceTakeout
		}
		if req.VoiceDineIn != nil {
			createParams.VoiceDineIn = *req.VoiceDineIn
		}
		if req.EnableKDS != nil {
			createParams.EnableKds = *req.EnableKDS
		}
		if req.KdsURL != nil {
			createParams.KdsUrl = pgtype.Text{String: *req.KdsURL, Valid: true}
		}

		config, err = server.store.CreateOrderDisplayConfig(ctx, createParams)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	} else {
		// 更新现有配置
		updateParams := db.UpdateOrderDisplayConfigParams{
			MerchantID: merchant.ID,
		}

		if req.EnablePrint != nil {
			updateParams.EnablePrint = pgtype.Bool{Bool: *req.EnablePrint, Valid: true}
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
		if req.EnableVoice != nil {
			updateParams.EnableVoice = pgtype.Bool{Bool: *req.EnableVoice, Valid: true}
		}
		if req.VoiceTakeout != nil {
			updateParams.VoiceTakeout = pgtype.Bool{Bool: *req.VoiceTakeout, Valid: true}
		}
		if req.VoiceDineIn != nil {
			updateParams.VoiceDineIn = pgtype.Bool{Bool: *req.VoiceDineIn, Valid: true}
		}
		if req.EnableKDS != nil {
			updateParams.EnableKds = pgtype.Bool{Bool: *req.EnableKDS, Valid: true}
		}
		if req.KdsURL != nil {
			updateParams.KdsUrl = pgtype.Text{String: *req.KdsURL, Valid: true}
		}

		config, err = server.store.UpdateOrderDisplayConfig(ctx, updateParams)
		if err != nil {
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
		ID:               config.ID,
		MerchantID:       config.MerchantID,
		EnablePrint:      config.EnablePrint,
		PrintTakeout:     config.PrintTakeout,
		PrintDineIn:      config.PrintDineIn,
		PrintReservation: config.PrintReservation,
		EnableVoice:      config.EnableVoice,
		VoiceTakeout:     config.VoiceTakeout,
		VoiceDineIn:      config.VoiceDineIn,
		EnableKDS:        config.EnableKds,
		KdsURL:           kdsURL,
		CreatedAt:        config.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        updatedAt,
	}

	ctx.JSON(http.StatusOK, resp)
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
		PrintTakeout:     printer.PrintTakeout,
		PrintDineIn:      printer.PrintDineIn,
		PrintReservation: printer.PrintReservation,
		IsActive:         printer.IsActive,
		CreatedAt:        printer.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        updatedAt,
	}
}
