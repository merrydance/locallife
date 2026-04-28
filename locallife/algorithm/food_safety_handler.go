package algorithm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
)

type FoodSafetyReportInput struct {
	ReporterUserID int64
	MerchantID     int64
	Order          db.Order
}

// FoodSafetyHandler 食安事件处理器
type FoodSafetyHandler struct {
	store db.Store
	wsHub WebSocketHub // WebSocket通知
}

// NewFoodSafetyHandler 创建食安事件处理器
func NewFoodSafetyHandler(store db.Store, wsHub WebSocketHub) *FoodSafetyHandler {
	return &FoodSafetyHandler{
		store: store,
		wsHub: wsHub,
	}
}

// EvaluateFoodSafetyReport 评估食安举报
// 返回是否应该熔断、是否恶作剧、熔断时长等
func (fsh *FoodSafetyHandler) EvaluateFoodSafetyReport(
	ctx context.Context,
	input FoodSafetyReportInput,
) (*FoodSafetyCheckResult, error) {
	// Step 1: 检查商户最近1小时的食安举报
	var (
		reports []db.FoodSafetyIncident
		err     error
	)
	recentRows, queryErr := fsh.store.GetMerchantRecentFoodSafetyReports(ctx, input.MerchantID)
	err = queryErr
	reports = make([]db.FoodSafetyIncident, 0, len(recentRows))
	for _, row := range recentRows {
		reports = append(reports, db.FoodSafetyIncident{ID: row.ID, OrderID: row.OrderID, UserID: row.UserID})
	}
	if err != nil {
		return nil, err
	}

	reporterIDs := make([]int64, 0, len(reports)+1)
	reporterIDs = append(reporterIDs, input.ReporterUserID)
	for _, report := range reports {
		reporterIDs = append(reporterIDs, report.UserID)
	}
	reporterIDs = UniqueInt64(reporterIDs)

	// Step 2: 未达到3个不同用户 -> 仅记录和通知
	if len(reporterIDs) < FoodSafetyReportsIn1Hour {
		return &FoodSafetyCheckResult{
			ShouldCircuitBreak: false,
			IsMalicious:        false,
			ReasonCode:         "insufficient-reports",
			Message:            "食安举报未达到熔断阈值，仅记录",
			DurationHours:      0,
		}, nil
	}

	// Step 3: 达到3个，检查是否恶作剧
	isMalicious, err := fsh.checkMaliciousPattern(ctx, reporterIDs, input.Order, reports)
	if err != nil {
		return nil, err
	}

	if isMalicious {
		// 恶作剧
		return &FoodSafetyCheckResult{
			ShouldCircuitBreak: false,
			IsMalicious:        true,
			ReasonCode:         "malicious-coordinated-reports",
			Message:            "检测到恶意协同举报，暂不熔断",
			DurationHours:      0,
		}, nil
	}

	// Step 4: 真实举报 -> 熔断48小时
	return &FoodSafetyCheckResult{
		ShouldCircuitBreak: true,
		IsMalicious:        false,
		ReasonCode:         "3-distinct-customer-food-safety-reports-in-1h",
		Message:            "1小时内同商户出现3名不同顾客的真实食安举报，立即熔断",
		DurationHours:      48,
	}, nil
}

// checkMaliciousPattern 检查是否恶作剧
// 恶作剧特征：
// 1. 注册时间都不超过一周 + 只点了这一次外卖
// 2. 相同设备（通过已检测的欺诈模式判断）
// 3. 相同收货地址
func (fsh *FoodSafetyHandler) checkMaliciousPattern(
	ctx context.Context,
	reporterIDs []int64,
	currentOrder db.Order,
	priorReports []db.FoodSafetyIncident,
) (bool, error) {
	if len(reporterIDs) < 2 {
		return false, nil
	}

	// 检查1: 都是新用户且首单
	allNewUsersFirstOrder := true
	for _, uid := range reporterIDs {
		totalOrders, err := fsh.store.CountUserOrders(ctx, uid)
		if err != nil {
			return false, err
		}
		if totalOrders > 1 {
			allNewUsersFirstOrder = false
			break
		}
	}

	if allNewUsersFirstOrder {
		return true, nil
	}

	// 检查2: 当前参与举报用户是否存在共享设备
	sharedDevice, err := fsh.hasSharedReporterDevice(ctx, reporterIDs)
	if err != nil {
		return false, err
	}
	if sharedDevice {
		return true, nil
	}

	// 检查3: 当前事件簇是否存在共享收货地址
	sharedAddress, err := fsh.hasSharedReporterAddress(ctx, currentOrder, priorReports)
	if err != nil {
		return false, err
	}
	if sharedAddress {
		return true, nil
	}

	return false, nil
}

func (fsh *FoodSafetyHandler) hasSharedReporterDevice(ctx context.Context, reporterIDs []int64) (bool, error) {
	deviceUsers := make(map[string]map[int64]struct{})

	for _, uid := range reporterIDs {
		devices, err := fsh.store.GetDevicesByUserID(ctx, uid)
		if err != nil {
			return false, err
		}
		for _, device := range devices {
			deviceKey := strings.TrimSpace(device.DeviceID)
			if device.DeviceFingerprint.Valid && strings.TrimSpace(device.DeviceFingerprint.String) != "" {
				deviceKey = strings.TrimSpace(device.DeviceFingerprint.String)
			}
			if deviceKey == "" {
				continue
			}
			if _, ok := deviceUsers[deviceKey]; !ok {
				deviceUsers[deviceKey] = make(map[int64]struct{})
			}
			deviceUsers[deviceKey][uid] = struct{}{}
		}
	}

	for _, users := range deviceUsers {
		if len(users) >= 2 {
			return true, nil
		}
	}

	return false, nil
}

func (fsh *FoodSafetyHandler) hasSharedReporterAddress(ctx context.Context, currentOrder db.Order, priorReports []db.FoodSafetyIncident) (bool, error) {
	addressUsers := make(map[int64]map[int64]struct{})

	if currentOrder.AddressID.Valid {
		addressUsers[currentOrder.AddressID.Int64] = map[int64]struct{}{
			currentOrder.UserID: {},
		}
	}

	for _, report := range priorReports {
		order, err := fsh.store.GetOrder(ctx, report.OrderID)
		if err != nil {
			return false, err
		}
		if !order.AddressID.Valid {
			continue
		}
		if _, ok := addressUsers[order.AddressID.Int64]; !ok {
			addressUsers[order.AddressID.Int64] = make(map[int64]struct{})
		}
		addressUsers[order.AddressID.Int64][order.UserID] = struct{}{}
	}

	for _, users := range addressUsers {
		if len(users) >= 2 {
			return true, nil
		}
	}

	return false, nil
}

// CircuitBreakMerchant 熔断商户
func (fsh *FoodSafetyHandler) CircuitBreakMerchant(
	ctx context.Context,
	merchantID int64,
	reason string,
	durationHours int,
) error {
	suspendUntil := time.Now().Add(time.Duration(durationHours) * time.Hour)

	err := fsh.store.SuspendMerchant(ctx, db.SuspendMerchantParams{
		MerchantID: merchantID,
		SuspendReason: pgtype.Text{
			String: reason,
			Valid:  true,
		},
		SuspendUntil: pgtype.Timestamptz{
			Time:  suspendUntil,
			Valid: true,
		},
	})
	if err != nil {
		return err
	}

	fsh.NotifyMerchantCircuitBreak(merchantID)

	// 取消未来预定并处理退款
	// 1. 先获取需要退款的预订列表
	reservationsToRefund, err := fsh.store.ListMerchantFutureReservationsForRefund(ctx, merchantID)
	if err == nil && len(reservationsToRefund) > 0 {
		// 记录需要退款的预订信息（用于财务系统处理）
		for _, res := range reservationsToRefund {
			refundAmount := res.DepositAmount + res.PrepaidAmount
			if refundAmount > 0 {
				// 通知用户预订被取消
				go fsh.sendNotification(
					"customer",
					"预订取消通知",
					fmt.Sprintf("由于商户原因，您的预订（%s）已取消，%d分将原路退回",
						res.ReservationDate.Time.Format("2006-01-02"),
						refundAmount),
					res.UserID,
				)
			}
		}
	}

	// 2. 批量取消所有未来预订
	cancelReason := fmt.Sprintf("商户熔断：%s", reason)
	_, _ = fsh.store.CancelMerchantFutureReservations(ctx, db.CancelMerchantFutureReservationsParams{
		MerchantID:   merchantID,
		CancelReason: pgtype.Text{String: cancelReason, Valid: true},
	})

	return nil
}

// NotifyMerchantCircuitBreak 在熔断完成后向商户发送告警。
func (fsh *FoodSafetyHandler) NotifyMerchantCircuitBreak(merchantID int64) {
	fsh.sendNotification(
		"merchant",
		"食安熔断警告",
		"您的店铺因食品安全问题被熔断，请立即整改",
		merchantID,
	)
}

// sendNotification 发送WebSocket通知（复用ClaimAutoApproval的逻辑）
func (fsh *FoodSafetyHandler) sendNotification(entityType, title, message string, entityID int64) {
	if fsh.wsHub == nil {
		return // 静默失败，不阻塞业务
	}

	notificationData := map[string]interface{}{
		"title":     title,
		"message":   message,
		"timestamp": time.Now().Unix(),
	}

	dataBytes, err := json.Marshal(notificationData)
	if err != nil {
		return // 序列化失败，静默跳过
	}

	msg := websocket.Message{
		Type:      "behavior_alert",
		Data:      dataBytes,
		Timestamp: time.Now(),
	}

	switch entityType {
	case "merchant":
		fsh.wsHub.SendToMerchant(entityID, msg)
	case "rider":
		fsh.wsHub.SendToRider(entityID, msg)
	}
}
