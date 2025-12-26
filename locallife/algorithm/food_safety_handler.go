package algorithm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
)

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
	userID int64,
	merchantID int64,
	evidence []string,
) (*FoodSafetyCheckResult, error) {
	// Step 1: 获取用户信用分
	userProfile, err := fsh.store.GetUserProfile(ctx, db.GetUserProfileParams{
		UserID: userID,
		Role:   EntityTypeCustomer,
	})
	if err != nil {
		return nil, err
	}

	// Step 2: 高信用用户 + 有证据 = 立即熔断24小时
	if userProfile.TrustScore >= TrustScoreFoodSafetyReport && len(evidence) > 0 {
		return &FoodSafetyCheckResult{
			ShouldCircuitBreak: true,
			IsMalicious:        false,
			ReasonCode:         "high-trust-user-report-with-evidence",
			Message:            "高信用用户举报且有证据，立即熔断",
			DurationHours:      24,
		}, nil
	}

	// Step 3: 检查商户最近1小时的食安举报
	reports, err := fsh.store.GetMerchantRecentFoodSafetyReports(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	// Step 4: 未达到3个 -> 仅记录和通知
	if len(reports) < FoodSafetyReportsIn1Hour-1 { // -1因为当前这次还没创建
		return &FoodSafetyCheckResult{
			ShouldCircuitBreak: false,
			IsMalicious:        false,
			ReasonCode:         "insufficient-reports",
			Message:            "食安举报未达到熔断阈值，仅记录",
			DurationHours:      0,
		}, nil
	}

	// Step 5: 达到3个，检查是否恶作剧
	// 需要加上当前用户
	reporterIDs := make([]int64, 0, len(reports)+1)
	reporterIDs = append(reporterIDs, userID)
	for _, report := range reports {
		reporterIDs = append(reporterIDs, report.UserID)
	}

	isMalicious, err := fsh.checkMaliciousPattern(ctx, reporterIDs)
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

	// Step 6: 真实举报 -> 熔断48小时
	return &FoodSafetyCheckResult{
		ShouldCircuitBreak: true,
		IsMalicious:        false,
		ReasonCode:         "3-food-safety-reports-in-1h",
		Message:            "1小时内3次真实食安举报，立即熔断",
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
) (bool, error) {
	if len(reporterIDs) < 2 {
		return false, nil
	}

	// 检查1: 都是新用户且首单
	allNewUsersFirstOrder := true
	for _, uid := range reporterIDs {
		profile, err := fsh.store.GetUserProfile(ctx, db.GetUserProfileParams{
			UserID: uid,
			Role:   EntityTypeCustomer,
		})
		if err != nil {
			// 如果查不到profile，说明可能还没创建，跳过
			continue
		}

		// 检查是否新用户：只点了这一次外卖
		if profile.TotalOrders > 1 {
			allNewUsersFirstOrder = false
			break
		}
	}

	if allNewUsersFirstOrder {
		return true, nil
	}

	// 检查2: 是否存在同一设备被多个用户使用的已确认欺诈模式
	// 查询这些用户是否已经被标记为欺诈关联用户
	patterns, err := fsh.store.GetFraudPatternsByUsers(ctx, reporterIDs)
	if err == nil {
		// 如果用户存在已确认的欺诈模式，判定为恶作剧
		for _, pattern := range patterns {
			if pattern.IsConfirmed && (pattern.PatternType == FraudPatternDeviceReuse || pattern.PatternType == FraudPatternAddressCluster) {
				return true, nil
			}
		}
	}

	// 检查3: 相同收货地址
	// 获取每个用户最近一次订单的地址
	addressMap := make(map[int64]int) // address_id -> 用户数
	for _, uid := range reporterIDs {
		// 查询用户最近订单的地址
		orders, err := fsh.store.ListUserRecentOrders(ctx, db.ListUserRecentOrdersParams{
			UserID: uid,
			Limit:  1,
		})
		if err != nil || len(orders) == 0 {
			continue
		}
		if orders[0].AddressID.Valid {
			addressMap[orders[0].AddressID.Int64]++
		}
	}
	// 如果同一地址被>=2个用户使用，判定为恶作剧
	for _, count := range addressMap {
		if count >= 2 {
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

	// 发送通知给商户
	fsh.sendNotification(
		"merchant",
		"食安熔断警告",
		"您的店铺因食品安全问题被熔断，请立即整改",
		merchantID,
	)

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
		Type:      "trust_score_alert",
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
