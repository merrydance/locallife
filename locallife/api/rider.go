package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

// ==================== 骑手申请 ====================

type applyRiderRequest struct {
	RealName string `json:"real_name" binding:"required,min=2,max=50"`
	IDCardNo string `json:"id_card_no" binding:"required,len=18,validIDCard"`
	Phone    string `json:"phone" binding:"required,validPhone"`
}

type riderResponse struct {
	ID                int64      `json:"id"`
	UserID            int64      `json:"user_id"`
	RealName          string     `json:"real_name"`
	Phone             string     `json:"phone"`
	DepositAmount     int64      `json:"deposit_amount"`
	FrozenDeposit     int64      `json:"frozen_deposit"`
	Status            string     `json:"status"`
	IsOnline          bool       `json:"is_online"`
	CreditScore       int16      `json:"credit_score"`
	TotalOrders       int32      `json:"total_orders"`
	TotalEarnings     int64      `json:"total_earnings"`
	OnlineDuration    int32      `json:"online_duration"`
	CurrentLongitude  *float64   `json:"current_longitude,omitempty"`
	CurrentLatitude   *float64   `json:"current_latitude,omitempty"`
	LocationUpdatedAt *time.Time `json:"location_updated_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

func newRiderResponse(rider db.Rider) riderResponse {
	resp := riderResponse{
		ID:             rider.ID,
		UserID:         rider.UserID,
		RealName:       rider.RealName,
		Phone:          rider.Phone,
		DepositAmount:  rider.DepositAmount,
		FrozenDeposit:  rider.FrozenDeposit,
		Status:         rider.Status,
		IsOnline:       rider.IsOnline,
		CreditScore:    rider.CreditScore,
		TotalOrders:    rider.TotalOrders,
		TotalEarnings:  rider.TotalEarnings,
		OnlineDuration: rider.OnlineDuration,
		CreatedAt:      rider.CreatedAt,
	}

	if rider.CurrentLongitude.Valid {
		lng, _ := rider.CurrentLongitude.Float64Value()
		resp.CurrentLongitude = &lng.Float64
	}
	if rider.CurrentLatitude.Valid {
		lat, _ := rider.CurrentLatitude.Float64Value()
		resp.CurrentLatitude = &lat.Float64
	}
	if rider.LocationUpdatedAt.Valid {
		resp.LocationUpdatedAt = &rider.LocationUpdatedAt.Time
	}

	return resp
}

// applyRider godoc
// @Summary 骑手入驻申请
// @Description 用户提交骑手入驻申请，需要提供真实姓名、身份证号和手机号。申请后状态为pending，等待管理员审核。
// @Tags 骑手
// @Accept json
// @Produce json
// @Param request body applyRiderRequest true "骑手申请信息"
// @Success 201 {object} riderResponse "申请成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 409 {object} ErrorResponse "重复申请"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/apply [post]
// @Security BearerAuth
func (server *Server) applyRider(ctx *gin.Context) {
	var req applyRiderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 检查是否已申请
	_, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("您已申请成为骑手")))
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	arg := db.CreateRiderParams{
		UserID:   authPayload.UserID,
		RealName: req.RealName,
		IDCardNo: req.IDCardNo,
		Phone:    req.Phone,
	}

	rider, err := server.store.CreateRider(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newRiderResponse(rider))
}

// getRiderMe godoc
// @Summary 获取当前骑手信息
// @Description 获取当前登录用户的骑手信息，包括状态、押金、统计数据等
// @Tags 骑手
// @Accept json
// @Produce json
// @Success 200 {object} riderResponse "骑手信息"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/me [get]
// @Security BearerAuth
func (server *Server) getRiderMe(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderResponse(rider))
}

// ==================== 骑手审核（管理员） ====================

type approveRiderRequest struct {
	ID int64 `uri:"rider_id" binding:"required,min=1"`
}

// approveRider godoc
// @Summary 审核通过骑手申请（管理员）
// @Description 管理员审核通过骑手入驻申请，骑手状态从pending变为pending_bindbank，需要完成微信支付开户才能接单
// @Tags 骑手管理
// @Accept json
// @Produce json
// @Param rider_id path int true "骑手ID"
// @Success 200 {object} riderResponse "审核成功"
// @Failure 400 {object} ErrorResponse "状态不允许审核"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "骑手不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/admin/riders/{rider_id}/approve [post]
// @Security BearerAuth
func (server *Server) approveRider(ctx *gin.Context) {
	var req approveRiderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rider, err := server.store.GetRider(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("骑手不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if rider.Status != "pending" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("该骑手不是待审核状态")))
		return
	}

	// 审核通过后设置为 pending_bindbank 状态
	// 骑手需要完成微信支付开户后才能正常接单
	updated, err := server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
		ID:     req.ID,
		Status: "pending_bindbank",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderResponse(updated))
}

// rejectRider godoc
// @Summary 拒绝骑手申请（管理员）
// @Description 管理员拒绝骑手入驻申请，骑手状态从pending变为rejected
// @Tags 骑手管理
// @Accept json
// @Produce json
// @Param rider_id path int true "骑手ID"
// @Success 200 {object} riderResponse "拒绝成功"
// @Failure 400 {object} ErrorResponse "状态不允许审核"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "骑手不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/admin/riders/{rider_id}/reject [post]
// @Security BearerAuth
func (server *Server) rejectRider(ctx *gin.Context) {
	var req approveRiderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rider, err := server.store.GetRider(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("骑手不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if rider.Status != "pending" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("该骑手不是待审核状态")))
		return
	}

	updated, err := server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
		ID:     req.ID,
		Status: "rejected",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderResponse(updated))
}

// ==================== 押金管理 ====================

// 押金常量定义
const (
	// MinDepositAmount 最小充值金额：100分 = 1元
	MinDepositAmount = int64(100)
	// MaxDepositAmount 最大充值金额：1000000分 = 10000元
	MaxDepositAmount = int64(1000000)
	// MinWithdrawAmount 最小提现金额：100分 = 1元
	MinWithdrawAmount = int64(100)
	// MaxWithdrawAmount 单次最大提现金额：5000000分 = 50000元
	MaxWithdrawAmount = int64(5000000)
	// MinOnlineDeposit 上线所需最低押金：10000分 = 100元
	MinOnlineDeposit = int64(10000)
)

type depositRequest struct {
	Amount int64  `json:"amount" binding:"required,min=100,max=1000000"`
	Remark string `json:"remark" binding:"max=200"`
}

type withdrawRequest struct {
	Amount int64  `json:"amount" binding:"required,min=100,max=5000000"`
	Remark string `json:"remark" binding:"max=200"`
}

type depositResponse struct {
	ID           int64     `json:"id"`
	RiderID      int64     `json:"rider_id"`
	Amount       int64     `json:"amount"`
	Type         string    `json:"type"`
	BalanceAfter int64     `json:"balance_after"`
	Remark       string    `json:"remark,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// depositBalanceResponse 押金余额响应
type depositBalanceResponse struct {
	TotalDeposit     int64 `json:"total_deposit"`     // 总押金
	FrozenDeposit    int64 `json:"frozen_deposit"`    // 冻结押金
	AvailableDeposit int64 `json:"available_deposit"` // 可用押金
}

type paginationRequest struct {
	Page  int32 `form:"page" binding:"min=0"`
	Limit int32 `form:"limit" binding:"min=0,max=100"`
}

func newDepositResponse(d db.RiderDeposit) depositResponse {
	resp := depositResponse{
		ID:           d.ID,
		RiderID:      d.RiderID,
		Amount:       d.Amount,
		Type:         d.Type,
		BalanceAfter: d.BalanceAfter,
		CreatedAt:    d.CreatedAt,
	}
	if d.Remark.Valid {
		resp.Remark = d.Remark.String
	}
	return resp
}

// depositRider godoc
// @Summary 骑手押金充值
// @Description 创建骑手押金充值支付订单，返回微信支付参数
// @Tags 骑手
// @Accept json
// @Produce json
// @Param request body depositRequest true "充值金额（单位：分）"
// @Success 200 {object} object "微信支付参数"
// @Failure 400 {object} ErrorResponse "参数错误或骑手未激活"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/deposit [post]
// @Security BearerAuth
func (server *Server) depositRider(ctx *gin.Context) {
	var req depositRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if rider.Status != "approved" && rider.Status != "active" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("您的骑手账号尚未激活")))
		return
	}

	// 创建支付订单
	// 骑手押金充值使用单独的 business_type，回调时根据 user_id 找到对应骑手
	outTradeNo := generateOutTradeNo()
	expiresAt := time.Now().Add(30 * time.Minute)

	paymentOrder, err := server.store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		UserID:       authPayload.UserID,
		PaymentType:  PaymentTypeMiniProgram,
		BusinessType: "rider_deposit", // 骑手押金充值
		Amount:       req.Amount,
		OutTradeNo:   outTradeNo,
		ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构建响应
	resp := map[string]interface{}{
		"payment_order_id": paymentOrder.ID,
		"out_trade_no":     outTradeNo,
		"amount":           req.Amount,
		"expires_at":       expiresAt,
	}

	// 调用微信支付 API 创建预支付订单
	if server.paymentClient != nil {
		user, err := server.store.GetUser(ctx, authPayload.UserID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get user: %w", err)))
			return
		}

		wxResp, payParams, err := server.paymentClient.CreateJSAPIOrder(ctx, &wechat.JSAPIOrderRequest{
			OutTradeNo:    outTradeNo,
			Description:   "骑手押金充值",
			TotalAmount:   req.Amount,
			OpenID:        user.WechatOpenid,
			ExpireTime:    expiresAt,
			Attach:        fmt.Sprintf("deposit:rider_%d", authPayload.UserID), // 押金充值标识
			PayerClientIP: ctx.ClientIP(),                                      // 用户终端IP
		})
		if err != nil {
			server.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("wechat pay: %w", err)))
			return
		}

		// 更新 prepay_id
		server.store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
			ID:       paymentOrder.ID,
			PrepayID: pgtype.Text{String: wxResp.PrepayID, Valid: true},
		})

		// 返回支付参数
		resp["pay_params"] = map[string]string{
			"timeStamp": payParams.TimeStamp,
			"nonceStr":  payParams.NonceStr,
			"package":   payParams.Package,
			"signType":  payParams.SignType,
			"paySign":   payParams.PaySign,
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// withdrawRider godoc
// @Summary 骑手押金提现
// @Description 从骑手账户提取押金到微信零钱，需要确保没有进行中的配送订单。最小提现金额1元，单次最大提现金额50000元。
// @Tags 骑手
// @Accept json
// @Produce json
// @Param request body withdrawRequest true "提现金额（单位：分）"
// @Success 200 {object} depositResponse "提现成功"
// @Failure 400 {object} ErrorResponse "余额不足、有进行中订单或账号未激活"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/withdraw [post]
// @Security BearerAuth
func (server *Server) withdrawRider(ctx *gin.Context) {
	var req withdrawRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查骑手状态：只有 active 状态才能提现
	if rider.Status != "active" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("您的骑手账号尚未激活，无法提现")))
		return
	}

	// 检查可用余额
	availableBalance := rider.DepositAmount - rider.FrozenDeposit
	if req.Amount > availableBalance {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("可用余额不足")))
		return
	}

	// 检查是否有进行中的订单
	activeDeliveries, err := server.store.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if len(activeDeliveries) > 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("您有进行中的配送订单，无法提现")))
		return
	}

	// 使用事务执行提现操作：锁定骑手行 + 验证余额 + 更新余额 + 创建流水
	result, err := server.store.WithdrawDepositTx(ctx, db.WithdrawDepositTxParams{
		RiderID: rider.ID,
		Amount:  req.Amount,
		Remark:  req.Remark,
	})
	if err != nil {
		// 区分余额不足错误
		if err.Error() == "可用余额不足" {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 调用微信提现 API（商家转账到零钱）
	if server.paymentClient != nil {
		user, err := server.store.GetUser(ctx, authPayload.UserID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get user: %w", err)))
			return
		}

		// 生成批次单号
		outBatchNo := fmt.Sprintf("WD%s%d", time.Now().Format("20060102150405"), result.DepositLog.ID)

		_, err = server.paymentClient.CreateTransfer(ctx, &wechat.TransferRequest{
			OutBatchNo:     outBatchNo,
			BatchName:      "骑手押金提现",
			BatchRemark:    req.Remark,
			TransferAmount: req.Amount,
			OpenID:         user.WechatOpenid,
			UserName:       rider.RealName,
			TransferRemark: "骑手押金提现",
		})
		if err != nil {
			// 提现失败，使用事务回滚押金变更
			rollbackErr := server.store.RollbackWithdrawTx(ctx, db.RollbackWithdrawTxParams{
				RiderID: rider.ID,
				Amount:  req.Amount,
			})
			if rollbackErr != nil {
				// 记录回滚失败日志，需要人工介入
				log.Error().
					Err(err).
					Int64("rider_id", rider.ID).
					Int64("amount", req.Amount).
					Err(rollbackErr).
					Msg("withdraw rollback failed, need manual intervention")
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("提现失败且回滚失败，请联系客服处理")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("提现失败: %w", err)))
			return
		}
	}

	ctx.JSON(http.StatusOK, newDepositResponse(result.DepositLog))
}

// getRiderDepositBalance godoc
// @Summary 获取押金余额
// @Description 获取当前骑手的押金余额信息，包括总押金、冻结押金和可用押金
// @Tags 骑手
// @Accept json
// @Produce json
// @Success 200 {object} depositBalanceResponse "押金余额信息"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/deposit [get]
// @Security BearerAuth
func (server *Server) getRiderDepositBalance(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := depositBalanceResponse{
		TotalDeposit:     rider.DepositAmount,
		FrozenDeposit:    rider.FrozenDeposit,
		AvailableDeposit: rider.DepositAmount - rider.FrozenDeposit,
	}

	ctx.JSON(http.StatusOK, response)
}

// listRiderDeposits godoc
// @Summary 查询押金流水
// @Description 分页查询当前骑手的押金变动流水记录，包括充值、提现、冻结、解冻、扣款等
// @Tags 骑手
// @Accept json
// @Produce json
// @Param page query int false "页码" minimum(1) default(1)
// @Param limit query int false "每页数量" minimum(1) maximum(100) default(20)
// @Success 200 {array} depositResponse "押金流水列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/deposits [get]
// @Security BearerAuth
func (server *Server) listRiderDeposits(ctx *gin.Context) {
	var req paginationRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 处理分页默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	deposits, err := server.store.ListRiderDeposits(ctx, db.ListRiderDepositsParams{
		RiderID: rider.ID,
		Limit:   req.Limit,
		Offset:  (req.Page - 1) * req.Limit,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 确保返回空数组而非 null
	response := make([]depositResponse, 0, len(deposits))
	for _, d := range deposits {
		response = append(response, newDepositResponse(d))
	}

	ctx.JSON(http.StatusOK, response)
}

// ==================== 上下线管理 ====================

// riderStatusResponse 骑手状态响应
type riderStatusResponse struct {
	Status            string     `json:"status"`            // 账号状态：pending/approved/active/suspended/rejected
	IsOnline          bool       `json:"is_online"`         // 是否在线
	OnlineStatus      string     `json:"online_status"`     // 在线状态描述：offline/online/delivering
	ActiveDeliveries  int        `json:"active_deliveries"` // 当前配送中订单数量
	CurrentLongitude  *float64   `json:"current_longitude,omitempty"`
	CurrentLatitude   *float64   `json:"current_latitude,omitempty"`
	LocationUpdatedAt *time.Time `json:"location_updated_at,omitempty"`
	CanGoOnline       bool       `json:"can_go_online"`                 // 是否可以上线
	CanGoOffline      bool       `json:"can_go_offline"`                // 是否可以下线
	OnlineBlockReason string     `json:"online_block_reason,omitempty"` // 不能上线的原因
}

// getRiderStatus godoc
// @Summary 获取骑手当前状态
// @Description 获取骑手当前在线状态、位置信息、配送状态等
// @Tags 骑手
// @Accept json
// @Produce json
// @Success 200 {object} riderStatusResponse "骑手状态"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/status [get]
// @Security BearerAuth
func (server *Server) getRiderStatus(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取活跃配送数量
	activeDeliveries, err := server.store.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := riderStatusResponse{
		Status:           rider.Status,
		IsOnline:         rider.IsOnline,
		ActiveDeliveries: len(activeDeliveries),
	}

	// 确定在线状态描述
	if !rider.IsOnline {
		resp.OnlineStatus = "offline"
	} else if len(activeDeliveries) > 0 {
		resp.OnlineStatus = "delivering"
	} else {
		resp.OnlineStatus = "online"
	}

	// 位置信息
	if rider.CurrentLongitude.Valid {
		lng, _ := rider.CurrentLongitude.Float64Value()
		resp.CurrentLongitude = &lng.Float64
	}
	if rider.CurrentLatitude.Valid {
		lat, _ := rider.CurrentLatitude.Float64Value()
		resp.CurrentLatitude = &lat.Float64
	}
	if rider.LocationUpdatedAt.Valid {
		resp.LocationUpdatedAt = &rider.LocationUpdatedAt.Time
	}

	// 判断是否可以上线/下线
	availableDeposit := rider.DepositAmount - rider.FrozenDeposit
	if rider.Status != "active" {
		resp.CanGoOnline = false
		resp.OnlineBlockReason = "账号未激活"
	} else if availableDeposit < MinOnlineDeposit {
		resp.CanGoOnline = false
		resp.OnlineBlockReason = fmt.Sprintf("押金不足，需要至少%d元", MinOnlineDeposit/100)
	} else {
		resp.CanGoOnline = true
	}

	// 有活跃配送时不能下线
	resp.CanGoOffline = rider.IsOnline && len(activeDeliveries) == 0

	ctx.JSON(http.StatusOK, resp)
}

// goOnline godoc
// @Summary 骑手上线
// @Description 设置骑手状态为在线，开始接受订单。需要骑手状态为active且押金充足
// @Tags 骑手
// @Accept json
// @Produce json
// @Success 200 {object} riderResponse "上线成功"
// @Failure 400 {object} ErrorResponse "账号未激活或押金不足"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/online [post]
// @Security BearerAuth
func (server *Server) goOnline(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if rider.Status != "active" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("您的骑手账号尚未激活")))
		return
	}

	// 检查押金余额
	if rider.DepositAmount-rider.FrozenDeposit < MinOnlineDeposit {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("押金余额不足，请先充值")))
		return
	}

	if rider.IsOnline {
		ctx.JSON(http.StatusOK, newRiderResponse(rider))
		return
	}

	updated, err := server.store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{
		ID:       rider.ID,
		IsOnline: true,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderResponse(updated))
}

// goOffline godoc
// @Summary 骑手下线
// @Description 设置骑手状态为离线，停止接单。如果有进行中的配送订单则无法下线
// @Tags 骑手
// @Accept json
// @Produce json
// @Success 200 {object} riderResponse "下线成功"
// @Failure 400 {object} ErrorResponse "有进行中订单"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/offline [post]
// @Security BearerAuth
func (server *Server) goOffline(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否有进行中的订单
	activeDeliveries, err := server.store.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if len(activeDeliveries) > 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("您有进行中的配送订单，无法下线")))
		return
	}

	if !rider.IsOnline {
		ctx.JSON(http.StatusOK, newRiderResponse(rider))
		return
	}

	updated, err := server.store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{
		ID:       rider.ID,
		IsOnline: false,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderResponse(updated))
}

// ==================== 位置上报 ====================

type updateLocationRequest struct {
	Locations []locationPoint `json:"locations" binding:"required,min=1,max=100,dive"`
}

type locationPoint struct {
	Longitude  float64   `json:"longitude" binding:"required,gte=-180,lte=180"`
	Latitude   float64   `json:"latitude" binding:"required,gte=-90,lte=90"`
	Accuracy   *float64  `json:"accuracy,omitempty" binding:"omitempty,gte=0,lte=1000"` // GPS精度(米)，0-1000
	Speed      *float64  `json:"speed,omitempty" binding:"omitempty,gte=0,lte=200"`     // 速度(m/s)，0-200(约720km/h)
	Heading    *float64  `json:"heading,omitempty" binding:"omitempty,gte=0,lte=360"`   // 航向角(度)，0-360
	RecordedAt time.Time `json:"recorded_at" binding:"required"`
}

// updateRiderLocation godoc
// @Summary 更新骑手位置
// @Description 批量上报骑手GPS位置点，仅在线状态可调用
// @Tags 骑手
// @Accept json
// @Produce json
// @Param request body updateLocationRequest true "位置点数组"
// @Success 200 {object} object{message=string,count=int,longitude=number,latitude=number} "上报成功"
// @Failure 400 {object} ErrorResponse "参数错误或不在线"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/location [post]
// @Security BearerAuth
func (server *Server) updateRiderLocation(ctx *gin.Context) {
	var req updateLocationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 时间验证：不允许超过5分钟的未来时间，不允许超过1小时的历史时间
	now := time.Now()
	maxFuture := now.Add(5 * time.Minute)
	maxPast := now.Add(-1 * time.Hour)
	for _, loc := range req.Locations {
		if loc.RecordedAt.After(maxFuture) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("位置记录时间不能超过当前时间5分钟")))
			return
		}
		if loc.RecordedAt.Before(maxPast) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("位置记录时间不能早于1小时前")))
			return
		}
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if !rider.IsOnline {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("您当前不在线")))
		return
	}

	// 获取当前活跃订单
	var activeDeliveryID *int64
	activeDeliveries, err := server.store.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	if err == nil && len(activeDeliveries) > 0 {
		activeDeliveryID = &activeDeliveries[0].ID
	}

	// 批量插入位置记录
	var locations []db.BatchCreateRiderLocationsParams
	var latestLocation locationPoint

	for _, loc := range req.Locations {
		param := db.BatchCreateRiderLocationsParams{
			RiderID:    rider.ID,
			Longitude:  numericFromFloat(loc.Longitude),
			Latitude:   numericFromFloat(loc.Latitude),
			RecordedAt: loc.RecordedAt,
		}

		if activeDeliveryID != nil {
			param.DeliveryID = pgtype.Int8{Int64: *activeDeliveryID, Valid: true}
		}
		if loc.Accuracy != nil {
			param.Accuracy = numericFromFloat(*loc.Accuracy)
		}
		if loc.Speed != nil {
			param.Speed = numericFromFloat(*loc.Speed)
		}
		if loc.Heading != nil {
			param.Heading = numericFromFloat(*loc.Heading)
		}

		locations = append(locations, param)

		// 记录最新位置
		if loc.RecordedAt.After(latestLocation.RecordedAt) {
			latestLocation = loc
		}
	}

	// 批量插入
	_, err = server.store.BatchCreateRiderLocations(ctx, locations)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 更新骑手最新位置
	_, err = server.store.UpdateRiderLocation(ctx, db.UpdateRiderLocationParams{
		ID:               rider.ID,
		CurrentLongitude: numericFromFloat(latestLocation.Longitude),
		CurrentLatitude:  numericFromFloat(latestLocation.Latitude),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":   "位置更新成功",
		"count":     len(locations),
		"longitude": latestLocation.Longitude,
		"latitude":  latestLocation.Latitude,
	})
}

// ==================== 管理员接口 ====================

type listRidersRequest struct {
	Status string `form:"status"`
	Page   int32  `form:"page" binding:"min=1"`
	Limit  int32  `form:"limit" binding:"min=1,max=100"`
}

// listRiders godoc
// @Summary 获取骑手列表（管理员）
// @Description 管理员或运营商分页获取骑手列表，支持状态筛选
// @Tags 骑手管理
// @Accept json
// @Produce json
// @Param status query string false "筛选状态" Enums(pending, approved, active, suspended, rejected)
// @Param page query int false "页码" minimum(1) default(1)
// @Param limit query int false "每页数量" minimum(1) maximum(100) default(20)
// @Success 200 {array} riderResponse "骑手列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/admin/riders [get]
// @Security BearerAuth
func (server *Server) listRiders(ctx *gin.Context) {
	var req listRidersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	if req.Status == "" {
		req.Status = "pending"
	}

	riders, err := server.store.ListRidersByStatus(ctx, db.ListRidersByStatusParams{
		Status: req.Status,
		Limit:  req.Limit,
		Offset: (req.Page - 1) * req.Limit,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var response []riderResponse
	for _, r := range riders {
		response = append(response, newRiderResponse(r))
	}

	ctx.JSON(http.StatusOK, response)
}

// ==================== 辅助函数 ====================

func numericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	n.Scan(f)
	return n
}

// ==================== 延时申报与异常上报 ====================

type riderOrderIDRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type delayReportRequest struct {
	Reason          string `json:"reason" binding:"required,min=5,max=500"`           // 延时原因
	ExpectedMinutes int    `json:"expected_minutes" binding:"required,min=5,max=120"` // 预计延迟分钟数
}

type exceptionReportRequest struct {
	ExceptionType string   `json:"exception_type" binding:"required,oneof=customer_unreachable merchant_not_ready weather_issue road_blocked other"` // 异常类型
	Description   string   `json:"description" binding:"required,min=5,max=500"`                                                                     // 异常描述
	EvidenceURLs  []string `json:"evidence_urls"`                                                                                                    // 证据图片URL
}

type delayReportResponse struct {
	OrderID         int64     `json:"order_id"`
	Reason          string    `json:"reason"`
	ExpectedMinutes int       `json:"expected_minutes"`
	ReportedAt      time.Time `json:"reported_at"`
	Status          string    `json:"status"` // pending, acknowledged
}

type exceptionReportResponse struct {
	ID            int64     `json:"id"`
	OrderID       int64     `json:"order_id"`
	ExceptionType string    `json:"exception_type"`
	Description   string    `json:"description"`
	EvidenceURLs  []string  `json:"evidence_urls"`
	Status        string    `json:"status"` // pending, resolved, dismissed
	ReportedAt    time.Time `json:"reported_at"`
}

// reportDelay godoc
// @Summary 骑手延时申报
// @Description 骑手申报订单配送将延迟，通知顾客和商户
// @Tags riders
// @Accept json
// @Produce json
// @Param id path int true "订单ID"
// @Param request body delayReportRequest true "延时申报信息"
// @Success 200 {object} delayReportResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "非该订单骑手"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse
// @Router /rider/orders/{id}/delay [post]
// @Security BearerAuth
func (server *Server) reportDelay(ctx *gin.Context) {
	var uriReq riderOrderIDRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req delayReportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取当前骑手
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, errorResponse(fmt.Errorf("未找到骑手信息")))
		return
	}

	// 获取订单的配送信息
	delivery, err := server.store.GetDeliveryByOrderID(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("配送记录不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证是否是该骑手的订单
	if !delivery.RiderID.Valid || delivery.RiderID.Int64 != rider.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(fmt.Errorf("您不是该订单的配送骑手")))
		return
	}

	// 检查订单状态是否允许延时申报
	order, err := server.store.GetOrder(ctx, uriReq.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if order.Status != "delivering" && order.Status != "picked_up" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("当前订单状态不允许延时申报")))
		return
	}

	// 记录延时申报日志
	_, err = server.store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
		OrderID:      order.ID,
		FromStatus:   pgtype.Text{String: order.Status, Valid: true},
		ToStatus:     order.Status, // 状态不变
		OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
		OperatorType: pgtype.Text{String: "rider", Valid: true},
		Notes:        pgtype.Text{String: fmt.Sprintf("延时申报: %s, 预计延迟%d分钟", req.Reason, req.ExpectedMinutes), Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 发送通知给顾客
	go server.sendDelayNotification(order.UserID, order.ID, req.Reason, req.ExpectedMinutes)

	// 发送通知给商户
	merchant, _ := server.store.GetMerchant(ctx, order.MerchantID)
	if merchant.OwnerUserID > 0 {
		go server.sendDelayNotificationToMerchant(merchant.OwnerUserID, order.ID, req.Reason, req.ExpectedMinutes)
	}

	response := delayReportResponse{
		OrderID:         order.ID,
		Reason:          req.Reason,
		ExpectedMinutes: req.ExpectedMinutes,
		ReportedAt:      time.Now(),
		Status:          "acknowledged",
	}

	ctx.JSON(http.StatusOK, response)
}

// reportException godoc
// @Summary 骑手异常上报
// @Description 骑手上报配送过程中遇到的异常情况
// @Tags riders
// @Accept json
// @Produce json
// @Param id path int true "订单ID"
// @Param request body exceptionReportRequest true "异常上报信息"
// @Success 200 {object} exceptionReportResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "非该订单骑手"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse
// @Router /rider/orders/{id}/exception [post]
// @Security BearerAuth
func (server *Server) reportException(ctx *gin.Context) {
	var uriReq riderOrderIDRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req exceptionReportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取当前骑手
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, errorResponse(fmt.Errorf("未找到骑手信息")))
		return
	}

	// 获取订单的配送信息
	delivery, err := server.store.GetDeliveryByOrderID(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(fmt.Errorf("配送记录不存在")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证是否是该骑手的订单
	if !delivery.RiderID.Valid || delivery.RiderID.Int64 != rider.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(fmt.Errorf("您不是该订单的配送骑手")))
		return
	}

	// 获取订单
	order, err := server.store.GetOrder(ctx, uriReq.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构建异常描述
	exceptionDesc := fmt.Sprintf("异常类型: %s, 描述: %s", getExceptionTypeName(req.ExceptionType), req.Description)
	if len(req.EvidenceURLs) > 0 {
		exceptionDesc += fmt.Sprintf(", 证据: %v", req.EvidenceURLs)
	}

	// 记录异常日志
	_, err = server.store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
		OrderID:      order.ID,
		FromStatus:   pgtype.Text{String: order.Status, Valid: true},
		ToStatus:     order.Status, // 状态不变
		OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
		OperatorType: pgtype.Text{String: "rider", Valid: true},
		Notes:        pgtype.Text{String: exceptionDesc, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 根据异常类型处理
	switch req.ExceptionType {
	case "customer_unreachable":
		// 联系不上顾客 - 通知平台客服
		go server.notifyPlatformSupport(order.ID, req.ExceptionType, req.Description)
	case "merchant_not_ready":
		// 商户未出餐 - 通知商户
		merchant, _ := server.store.GetMerchant(ctx, order.MerchantID)
		if merchant.OwnerUserID > 0 {
			go server.notifyMerchantException(merchant.OwnerUserID, order.ID, req.Description)
		}
	}

	response := exceptionReportResponse{
		ID:            time.Now().UnixNano(), // 临时ID
		OrderID:       order.ID,
		ExceptionType: req.ExceptionType,
		Description:   req.Description,
		EvidenceURLs:  req.EvidenceURLs,
		Status:        "pending",
		ReportedAt:    time.Now(),
	}

	ctx.JSON(http.StatusOK, response)
}

// 辅助函数：获取异常类型名称
func getExceptionTypeName(exceptionType string) string {
	names := map[string]string{
		"customer_unreachable": "联系不上顾客",
		"merchant_not_ready":   "商户未出餐",
		"weather_issue":        "恶劣天气",
		"road_blocked":         "道路受阻",
		"other":                "其他",
	}
	if name, ok := names[exceptionType]; ok {
		return name
	}
	return exceptionType
}

// 辅助函数：发送延时通知给顾客
func (server *Server) sendDelayNotification(userID int64, orderID int64, reason string, minutes int) {
	_ = server.SendNotification(context.Background(), SendNotificationParams{
		UserID:      userID,
		Type:        "delivery_delay",
		Title:       "配送延迟提醒",
		Content:     fmt.Sprintf("您的订单(#%d)配送将延迟约%d分钟", orderID, minutes),
		RelatedType: "order",
		RelatedID:   orderID,
		ExtraData: map[string]any{
			"reason":  reason,
			"minutes": minutes,
		},
	})
}

// 辅助函数：发送延时通知给商户
func (server *Server) sendDelayNotificationToMerchant(userID int64, orderID int64, reason string, minutes int) {
	_ = server.SendNotification(context.Background(), SendNotificationParams{
		UserID:      userID,
		Type:        "delivery_delay",
		Title:       "配送延迟通知",
		Content:     fmt.Sprintf("订单(#%d)配送将延迟约%d分钟，原因：%s", orderID, minutes, reason),
		RelatedType: "order",
		RelatedID:   orderID,
		ExtraData: map[string]any{
			"reason":  reason,
			"minutes": minutes,
		},
	})
}

// 辅助函数：通知平台客服（运营商）
// 业务逻辑：配送异常时通知该订单所在区域的运营商
func (server *Server) notifyPlatformSupport(orderID int64, exceptionType, description string) {
	ctx := context.Background()

	// 1. 获取订单
	order, err := server.store.GetOrder(ctx, orderID)
	if err != nil {
		log.Error().Err(err).Int64("order_id", orderID).Msg("notifyPlatformSupport: failed to get order")
		return
	}

	// 2. 获取商户以确定区域
	merchant, err := server.store.GetMerchant(ctx, order.MerchantID)
	if err != nil {
		log.Error().Err(err).Int64("merchant_id", order.MerchantID).Msg("notifyPlatformSupport: failed to get merchant")
		return
	}

	// 3. 获取该区域的运营商
	operator, err := server.store.GetOperatorByRegion(ctx, merchant.RegionID)
	if err != nil {
		// 该区域没有运营商，记录日志但不阻塞
		log.Warn().
			Int64("order_id", orderID).
			Int64("region_id", merchant.RegionID).
			Str("exception_type", exceptionType).
			Msg("notifyPlatformSupport: no operator found for region, skipping notification")
		return
	}

	// 4. 向运营商发送通知
	exceptionTitle := getExceptionTypeName(exceptionType)
	err = server.SendNotification(ctx, SendNotificationParams{
		UserID:      operator.UserID,
		Type:        "delivery_exception",
		Title:       fmt.Sprintf("配送异常: %s", exceptionTitle),
		Content:     fmt.Sprintf("订单(#%d)发生配送异常[%s]: %s，请及时处理", orderID, exceptionTitle, description),
		RelatedType: "order",
		RelatedID:   orderID,
		ExtraData: map[string]any{
			"exception_type": exceptionType,
			"description":    description,
			"merchant_id":    merchant.ID,
			"merchant_name":  merchant.Name,
		},
	})
	if err != nil {
		log.Error().Err(err).Int64("order_id", orderID).Msg("notifyPlatformSupport: failed to send notification")
	}
}

// 辅助函数：通知商户异常
func (server *Server) notifyMerchantException(userID int64, orderID int64, description string) {
	_ = server.SendNotification(context.Background(), SendNotificationParams{
		UserID:      userID,
		Type:        "delivery_exception",
		Title:       "配送异常通知",
		Content:     fmt.Sprintf("订单(#%d)配送异常：%s", orderID, description),
		RelatedType: "order",
		RelatedID:   orderID,
		ExtraData: map[string]any{
			"description": description,
		},
	})
}

// ==================== 高值单资格积分 ====================

// 高值单资格积分响应
type riderPremiumScoreResponse struct {
	RiderID               int64  `json:"rider_id"`                 // 骑手ID
	RealName              string `json:"real_name"`                // 真实姓名
	PremiumScore          int16  `json:"premium_score"`            // 高值单资格积分（可为负）
	CanAcceptPremiumOrder bool   `json:"can_accept_premium_order"` // 是否可以接高值单（积分≥0）
}

// getRiderPremiumScore godoc
// @Summary 获取高值单资格积分
// @Description 获取当前骑手的高值单资格积分及接单资格状态。积分规则：完成普通单+1，完成高值单-3，超时-5，餐损-10。初始积分为0，积分≥0时可以接高值单。
// @Tags 骑手-高值单
// @Accept json
// @Produce json
// @Success 200 {object} riderPremiumScoreResponse "成功返回积分信息"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "非骑手用户或骑手档案不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/rider/score [get]
// @Security BearerAuth
func (server *Server) getRiderPremiumScore(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取骑手信息
	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取高值单资格积分信息
	scoreInfo, err := server.store.GetRiderPremiumScoreWithProfile(ctx, rider.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("骑手档案不存在，请联系管理员")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, riderPremiumScoreResponse{
		RiderID:               scoreInfo.RiderID,
		RealName:              scoreInfo.RealName,
		PremiumScore:          scoreInfo.PremiumScore,
		CanAcceptPremiumOrder: scoreInfo.CanAcceptPremiumOrder,
	})
}

// 高值单资格积分历史记录请求
type listRiderPremiumScoreHistoryRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=5,max=50"`
}

// 高值单资格积分历史记录响应
type premiumScoreLogItem struct {
	ID                int64   `json:"id"`
	ChangeAmount      int16   `json:"change_amount"`       // 变更量（正数为增加，负数为减少）
	OldScore          int16   `json:"old_score"`           // 变更前积分
	NewScore          int16   `json:"new_score"`           // 变更后积分
	ChangeType        string  `json:"change_type"`         // 变更类型：normal_order/premium_order/adjustment
	ChangeTypeName    string  `json:"change_type_name"`    // 变更类型中文名
	RelatedOrderID    *int64  `json:"related_order_id"`    // 关联订单ID
	RelatedDeliveryID *int64  `json:"related_delivery_id"` // 关联配送单ID
	Remark            *string `json:"remark"`              // 备注
	CreatedAt         string  `json:"created_at"`          // 变更时间
}

type listRiderPremiumScoreHistoryResponse struct {
	CurrentScore int16                 `json:"current_score"` // 当前积分
	Total        int64                 `json:"total"`         // 总记录数
	Logs         []premiumScoreLogItem `json:"logs"`          // 历史记录
}

// getChangeTypeName 获取变更类型中文名
func getChangeTypeName(changeType string) string {
	switch changeType {
	case "normal_order":
		return "完成普通单"
	case "premium_order":
		return "完成高值单"
	case "timeout":
		return "超时扣分"
	case "damage":
		return "餐损扣分"
	case "adjustment":
		return "人工调整"
	default:
		return changeType
	}
}

// listRiderPremiumScoreHistory godoc
// @Summary 获取高值单资格积分历史
// @Description 分页查询骑手的高值单资格积分变更历史记录
// @Tags 骑手-高值单
// @Accept json
// @Produce json
// @Param page_id query int true "页码" minimum(1) default(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50) default(20)
// @Success 200 {object} listRiderPremiumScoreHistoryResponse "成功返回积分历史"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "非骑手用户"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/rider/score/history [get]
// @Security BearerAuth
func (server *Server) listRiderPremiumScoreHistory(ctx *gin.Context) {
	var req listRiderPremiumScoreHistoryRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取骑手信息
	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("您还不是骑手")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取当前积分
	currentScore, err := server.store.GetRiderPremiumScore(ctx, rider.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// 如果没有rider_profiles记录，返回默认值0
			currentScore = 0
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	// 获取总数
	total, err := server.store.CountRiderPremiumScoreLogs(ctx, rider.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取历史记录
	logs, err := server.store.ListRiderPremiumScoreLogs(ctx, db.ListRiderPremiumScoreLogsParams{
		RiderID: rider.ID,
		Limit:   req.PageSize,
		Offset:  (req.PageID - 1) * req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应格式
	items := make([]premiumScoreLogItem, len(logs))
	for i, log := range logs {
		item := premiumScoreLogItem{
			ID:             log.ID,
			ChangeAmount:   log.ChangeAmount,
			OldScore:       log.OldScore,
			NewScore:       log.NewScore,
			ChangeType:     log.ChangeType,
			ChangeTypeName: getChangeTypeName(log.ChangeType),
			CreatedAt:      log.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if log.RelatedOrderID.Valid {
			item.RelatedOrderID = &log.RelatedOrderID.Int64
		}
		if log.RelatedDeliveryID.Valid {
			item.RelatedDeliveryID = &log.RelatedDeliveryID.Int64
		}
		if log.Remark.Valid {
			item.Remark = &log.Remark.String
		}
		items[i] = item
	}

	ctx.JSON(http.StatusOK, listRiderPremiumScoreHistoryResponse{
		CurrentScore: currentScore,
		Total:        total,
		Logs:         items,
	})
}
