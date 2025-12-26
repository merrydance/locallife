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

// NotificationDistributor 通知分发器接口
// 用于解耦 algorithm 包和 worker 包，避免循环依赖
type NotificationDistributor interface {
	// SendUserNotification 发送用户通知（站内通知 + 可选的微信消息）
	// notificationType: system/food_safety/order 等
	// relatedType: claim/order/merchant 等
	SendUserNotification(ctx context.Context, userID int64, notificationType, title, content string, relatedType string, relatedID int64) error
}

// ClaimAutoApproval 索赔自动审核器
// 设计理念：
// 1. 所有索赔都赔付，不存在不赔的情况
// 2. 通过行为回溯标定用户是谁，而不是决定赔不赔
// 3. 问题用户照赔，但平台垫付（退还骑手/商户），然后拒绝服务
type ClaimAutoApproval struct {
	store                   db.Store
	lookbackChecker         *LookbackChecker
	notificationDistributor NotificationDistributor // 通知分发器（可选，用于发送用户通知）
	wsHub                   WebSocketHub            // WebSocket通知（必需，用于商户/骑手实时推送）
}

// WebSocketHub WebSocket通知接口
type WebSocketHub interface {
	SendToMerchant(merchantID int64, msg websocket.Message)
	SendToRider(riderID int64, msg websocket.Message)
}

// NewClaimAutoApproval 创建索赔自动审核器
func NewClaimAutoApproval(store db.Store, wsHub WebSocketHub) *ClaimAutoApproval {
	return &ClaimAutoApproval{
		store:                   store,
		lookbackChecker:         NewLookbackChecker(store),
		notificationDistributor: nil,
		wsHub:                   wsHub,
	}
}

// SetNotificationDistributor 设置通知分发器（可选）
func (caa *ClaimAutoApproval) SetNotificationDistributor(distributor NotificationDistributor) {
	caa.notificationDistributor = distributor
}

// EvaluateClaim 评估索赔申请（新设计）
// 核心逻辑：
// 1. 食安 → 人工审核
// 2. 其他类型 → 检查用户行为 → 决定是否秒赔/需证据/平台垫付
func (caa *ClaimAutoApproval) EvaluateClaim(
	ctx context.Context,
	userID int64,
	orderID int64,
	claimAmount int64,
	deliveryFee int64,
	claimType string,
	hasEvidence bool, // 是否提交了证据照片
) (*Decision, error) {
	// Step 1: 食安索赔 → 人工审核
	if claimType == ClaimTypeFoodSafety {
		return &Decision{
			Type:               ApprovalTypeManual,
			Approved:           false, // 需要人工审核后才赔付
			Amount:             0,
			Reason:             "食安索赔需人工审核",
			CompensationSource: CompensationSourceMerchant,
			NeedsReview:        true,
			ReviewMessage:      "食安索赔需要人工审核，退全款+医药费另议",
		}, nil
	}

	// Step 2: 确定赔付金额和来源
	compensationAmount := claimAmount
	compensationSource := CompensationSourceMerchant

	switch claimType {
	case ClaimTypeTimeout:
		// 超时只赔运费，从骑手押金扣
		compensationAmount = deliveryFee
		compensationSource = CompensationSourceRider
	case ClaimTypeDamage:
		// 餐损赔全额，从骑手押金扣
		compensationSource = CompensationSourceRider
	case ClaimTypeForeignObject:
		// 异物赔全额，商户退款
		compensationSource = CompensationSourceMerchant
	}

	// Step 3: 检查用户行为（近3个月外卖订单和索赔比例）
	behaviorResult, err := caa.CheckUserClaimBehavior(ctx, userID)
	if err != nil {
		// 查询失败，降级为正常秒赔
		return &Decision{
			Type:               ApprovalTypeInstant,
			Approved:           true,
			Amount:             compensationAmount,
			Reason:             "行为检查失败，降级秒赔",
			BehaviorStatus:     ClaimBehaviorNormal,
			CompensationSource: compensationSource,
		}, nil
	}

	// Step 4: 根据行为状态决定处理方式
	decision := &Decision{
		Approved:           true, // 新设计：永远赔付
		Amount:             compensationAmount,
		BehaviorStatus:     behaviorResult.Status,
		CompensationSource: compensationSource,
	}

	switch behaviorResult.Status {
	case ClaimBehaviorNormal:
		// 正常用户：秒赔
		decision.Type = ApprovalTypeInstant
		decision.Reason = "正常用户秒赔"

	case ClaimBehaviorWarned:
		// 首次触发警告（5单3索赔）：秒赔 + 警告
		decision.Type = ApprovalTypeInstant
		decision.Reason = "首次警告，本次秒赔"
		decision.Warning = fmt.Sprintf(
			"您近3个月%d笔外卖订单中已索赔%d次，下次索赔需提交证据照片",
			behaviorResult.TakeoutOrders, behaviorResult.ClaimCount+1)
		decision.ShouldWarn = true
		// 记录警告
		go caa.recordUserWarning(ctx, userID, decision.Warning)

	case ClaimBehaviorEvidenceRequired:
		// 已被警告过：需要证据
		if !hasEvidence {
			// 未提交证据，要求提交
			decision.Type = "evidence-required"
			decision.Approved = false // 暂不赔付，等证据
			decision.Amount = 0
			decision.Reason = "需要提交证据"
			decision.NeedsEvidence = true
			decision.Warning = "您已被警告，请提交证据照片后重新提交索赔"
		} else {
			// 已提交证据，秒赔
			decision.Type = ApprovalTypeInstant
			decision.Reason = "已提交证据，秒赔"
		}

	case ClaimBehaviorPlatformPay:
		// 问题用户：照赔，但平台垫付
		decision.Type = "platform-pay"
		decision.Reason = "问题用户，平台垫付"
		decision.CompensationSource = CompensationSourcePlatform
		decision.Warning = fmt.Sprintf(
			"您的索赔行为异常（近3个月%d单索赔%d次），本次由平台垫付。如继续异常行为，将被拒绝服务。",
			behaviorResult.TakeoutOrders, behaviorResult.ClaimCount+1)
		// 记录平台垫付
		go caa.recordPlatformPay(ctx, userID, decision.Warning)

	case ClaimBehaviorRejectService:
		// 拒绝服务用户：照赔 + 平台垫付 + 拒绝后续服务
		decision.Type = "platform-pay"
		decision.Reason = "拒绝服务用户，平台垫付"
		decision.CompensationSource = CompensationSourcePlatform
		decision.Warning = "您的账号因索赔行为异常已被限制服务，本次索赔由平台垫付。"
		// 触发拒绝服务流程
		go caa.triggerRejectService(ctx, userID)
	}

	return decision, nil
}

// CheckUserClaimBehavior 检查用户索赔行为
// 核心逻辑：近3个月5单3索赔触发警告
func (caa *ClaimAutoApproval) CheckUserClaimBehavior(ctx context.Context, userID int64) (*ClaimBehaviorResult, error) {
	// 查询用户行为统计
	stats, err := caa.store.GetUserBehaviorStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := &ClaimBehaviorResult{
		RecentMonths:  3,
		TakeoutOrders: int(stats.TakeoutOrders90d),
		ClaimCount:    int(stats.Claims90d),
		WarningCount:  int(stats.WarningCount),
	}

	// 计算索赔比例
	if result.TakeoutOrders > 0 {
		result.ClaimRatio = float64(result.ClaimCount) / float64(result.TakeoutOrders)
	}

	// 判定状态
	// 1. 已被要求提交证据
	if stats.RequiresEvidence {
		result.Status = ClaimBehaviorEvidenceRequired
		result.NeedsEvidence = true
		result.Message = "已被警告，需要提交证据"
		return result, nil
	}

	// 2. 平台垫付次数>=2次 → 拒绝服务
	if stats.PlatformPayCount >= 2 {
		result.Status = ClaimBehaviorRejectService
		result.RejectService = true
		result.Message = "多次平台垫付，拒绝服务"
		return result, nil
	}

	// 3. 已有警告+继续触发条件 → 需要证据或平台垫付
	if stats.WarningCount > 0 {
		// 已被警告，需要证据
		result.Status = ClaimBehaviorEvidenceRequired
		result.NeedsEvidence = true
		result.Message = "已被警告，需要提交证据"
		return result, nil
	}

	// 4. 检查是否触发首次警告（5单3索赔）
	// 注意：ClaimCount是历史索赔数，当前这次还没算进去，所以用+1来判断
	if result.TakeoutOrders <= ClaimWarningOrderCount && result.ClaimCount+1 >= ClaimWarningClaimCount {
		result.Status = ClaimBehaviorWarned
		result.ShouldWarn = true
		result.Message = fmt.Sprintf(
			"近3个月%d笔外卖订单已索赔%d次，触发警告",
			result.TakeoutOrders, result.ClaimCount+1)
		return result, nil
	}

	// 5. 订单较多但索赔比例异常高（>60%且>=3次）
	if result.ClaimRatio >= ClaimWarningRatio && result.ClaimCount+1 >= ClaimWarningClaimCount {
		result.Status = ClaimBehaviorWarned
		result.ShouldWarn = true
		result.Message = fmt.Sprintf(
			"索赔比例%.0f%%异常高，触发警告",
			result.ClaimRatio*100)
		return result, nil
	}

	// 6. 正常用户
	result.Status = ClaimBehaviorNormal
	result.Message = "正常用户"
	return result, nil
}

// recordUserWarning 记录用户警告
func (caa *ClaimAutoApproval) recordUserWarning(ctx context.Context, userID int64, reason string) {
	// 先检查是否已有警告记录
	_, err := caa.store.GetUserClaimWarningStatus(ctx, userID)
	if err != nil {
		// 不存在，创建新记录
		_, _ = caa.store.CreateUserClaimWarning(ctx, db.CreateUserClaimWarningParams{
			UserID:           userID,
			LastWarningReason: pgtype.Text{String: reason, Valid: true},
			RequiresEvidence: true, // 首次警告后，下次就需要证据
		})
	} else {
		// 已存在，增加警告次数
		_ = caa.store.IncrementUserClaimWarning(ctx, db.IncrementUserClaimWarningParams{
			UserID:           userID,
			LastWarningReason: pgtype.Text{String: reason, Valid: true},
			RequiresEvidence: true,
		})
	}
}

// recordPlatformPay 记录平台垫付
func (caa *ClaimAutoApproval) recordPlatformPay(ctx context.Context, userID int64, reason string) {
	_ = caa.store.IncrementUserPlatformPayCount(ctx, db.IncrementUserPlatformPayCountParams{
		UserID:           userID,
		LastWarningReason: pgtype.Text{String: reason, Valid: true},
	})
}

// triggerRejectService 触发拒绝服务
func (caa *ClaimAutoApproval) triggerRejectService(ctx context.Context, userID int64) {
	// 更新用户信任分到70以下
	_ = caa.store.UpdateUserTrustScore(ctx, db.UpdateUserTrustScoreParams{
		UserID:     userID,
		Role:       EntityTypeCustomer,
		TrustScore: TrustScoreRejectService - 1, // 69分，低于70
	})
	
	// 记录信用分变更
	_, _ = caa.store.CreateTrustScoreChange(ctx, db.CreateTrustScoreChangeParams{
		EntityType:        EntityTypeCustomer,
		EntityID:          userID,
		OldScore:          TrustScoreMax, // 假设原来是满分
		NewScore:          TrustScoreRejectService - 1,
		ScoreChange:       int16(TrustScoreRejectService - 1 - TrustScoreMax),
		ReasonType:        "reject-service",
		ReasonDescription: "索赔行为异常，拒绝服务",
		IsAuto:            true,
	})
	
	// 发送通知给用户（站内通知）
	// C端用户使用站内通知系统，用户打开小程序时可看到
	// 微信订阅消息需要用户授权，由前端根据通知内容决定是否触发
	if caa.notificationDistributor != nil {
		_ = caa.notificationDistributor.SendUserNotification(
			ctx,
			userID,
			"system",                                                    // notificationType
			"账户状态变更通知",                                                 // title
			"由于您的账户存在异常索赔行为，服务已受到限制。如有疑问请联系客服。", // content
			"user",                                                      // relatedType
			userID,                                                      // relatedID
		)
	}
}

// CheckRiderDamageHistory 检查骑手餐损历史（异步）
// 触发条件：骑手被索赔餐损时异步调用
// 功能：检查7天内餐损次数，达到阈值则扣信用分并通知
// 注意：押金扣款已在 CreateClaimWithDecision 中即时执行，此处只处理信用分
func (caa *ClaimAutoApproval) CheckRiderDamageHistory(
	ctx context.Context,
	riderID int64,
) error {
	// 查询骑手最近7天的餐损索赔
	startTime := time.Now().Add(-Recent7Days)
	claims, err := caa.store.ListRiderClaims(ctx, db.ListRiderClaimsParams{
		RiderID:   pgtype.Int8{Int64: riderID, Valid: true},
		CreatedAt: startTime,
	})
	if err != nil {
		return err
	}

	// 过滤餐损索赔
	damageCount := 0
	for _, claim := range claims {
		if claim.ClaimType == ClaimTypeDamage {
			damageCount++
		}
	}

	// 达到3次：触发信用分扣分和警告通知
	if damageCount >= DamageIncidentsIn7Days {
		// 1. 扣骑手信用分
		riderProfile, err := caa.store.GetRiderProfileForUpdate(ctx, riderID)
		if err != nil {
			return err
		}

		newScore := ClampInt16(
			riderProfile.TrustScore+ScoreDamage3Times,
			TrustScoreMin,
			TrustScoreMax,
		)

		err = caa.store.UpdateRiderTrustScore(ctx, db.UpdateRiderTrustScoreParams{
			RiderID:    riderID,
			TrustScore: newScore,
		})
		if err != nil {
			return err
		}

		// 2. 记录信用分变更
		_, err = caa.store.CreateTrustScoreChange(ctx, db.CreateTrustScoreChangeParams{
			EntityType:        EntityTypeRider,
			EntityID:          riderID,
			OldScore:          riderProfile.TrustScore,
			NewScore:          newScore,
			ScoreChange:       ScoreDamage3Times,
			ReasonType:        "damage-3-times-in-7d",
			ReasonDescription: fmt.Sprintf("一周内发生%d次餐损", damageCount),
			IsAuto:            true,
		})
		if err != nil {
			return err
		}

		// 3. 更新骑手profile统计
		err = caa.store.IncrementRiderDamageIncident(ctx, riderID)
		if err != nil {
			return err
		}

		// 4. 发送通知给骑手（信用分扣分警告）
		go caa.sendNotification("rider", "餐损索赔警告",
			fmt.Sprintf("您近7天内发生%d次餐损索赔，信用分-%d，请注意配送安全", damageCount, -ScoreDamage3Times), riderID)
		
		// 注意：押金扣款已在 CreateClaimWithDecision 中即时执行，此处不重复扣款
	}

	return nil
}

// CreateClaimWithDecision 根据决策创建索赔记录
func (caa *ClaimAutoApproval) CreateClaimWithDecision(
	ctx context.Context,
	orderID int64,
	userID int64,
	claimType string,
	description string,
	evidenceURLs []string,
	claimAmount int64,
	decision *Decision,
) (*db.Claim, error) {
	// 序列化回溯结果
	var lookbackJSON []byte
	if decision.LookbackData != nil {
		var err error
		lookbackJSON, err = json.Marshal(decision.LookbackData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal lookback result: %w", err)
		}
	}

	// 获取用户当前信用分作为快照
	profile, err := caa.store.GetUserProfile(ctx, db.GetUserProfileParams{
		UserID: userID,
		Role:   EntityTypeCustomer,
	})
	if err != nil {
		// 如果获取失败，使用默认值
		profile = db.UserProfile{TrustScore: int16(TrustScoreMax)}
	}

	// 确定状态（新设计）
	status := ClaimStatusPending
	var approvedAmount *int64

	switch decision.Type {
	case ApprovalTypeInstant, ApprovalTypeAuto, "platform-pay":
		// 秒赔、自动通过、平台垫付都是自动批准
		status = ClaimStatusAutoApproved
		approvedAmount = &claimAmount
	case ApprovalTypeManual:
		// 人工审核（食安）
		status = ClaimStatusManualReview
	case "evidence-required":
		// 需要证据，暂不批准
		status = ClaimStatusPending
	default:
		if decision.Approved {
			status = ClaimStatusAutoApproved
			approvedAmount = &claimAmount
		}
	}

	// 创建索赔记录
	params := db.CreateClaimParams{
		OrderID:        orderID,
		UserID:         userID,
		ClaimType:      claimType,
		Description:    description,
		EvidenceUrls:   evidenceURLs,
		ClaimAmount:    claimAmount,
		Status:         status,
		IsMalicious:    false,
		LookbackResult: lookbackJSON,
		CreatedAt:      time.Now(),
	}

	if approvedAmount != nil {
		params.ApprovedAmount = pgtype.Int8{Int64: *approvedAmount, Valid: true}
	}
	params.ApprovalType = pgtype.Text{String: decision.Type, Valid: true}
	params.TrustScoreSnapshot = pgtype.Int2{Int16: profile.TrustScore, Valid: true}
	params.AutoApprovalReason = pgtype.Text{String: decision.Reason, Valid: true}

	claim, err := caa.store.CreateClaim(ctx, params)

	if err != nil {
		return nil, fmt.Errorf("failed to create claim: %w", err)
	}

	// 更新用户profile的索赔统计
	err = caa.store.IncrementUserClaimCount(ctx, db.IncrementUserClaimCountParams{
		UserID: userID,
		Role:   EntityTypeCustomer,
	})
	if err != nil {
		return nil, err
	}

	// ========================================
	// 骑手押金扣款（餐损/超时）
	// ========================================
	// 如果赔付来源是骑手押金，且已批准，则即时扣款
	if decision.Approved && decision.CompensationSource == CompensationSourceRider && decision.Amount > 0 {
		// 获取骑手ID
		order, orderErr := caa.store.GetOrder(ctx, orderID)
		if orderErr == nil && order.OrderType == "takeout" {
			delivery, deliveryErr := caa.store.GetDeliveryByOrderID(ctx, orderID)
			if deliveryErr == nil && delivery.RiderID.Valid {
				riderID := delivery.RiderID.Int64
				
				// 执行押金扣款并退款给用户（异步，不阻塞API响应）
				// 使用原子事务：骑手押金扣款 → 用户余额入账
				go func() {
					deductCtx := context.Background()
					result, deductErr := caa.store.DeductRiderDepositAndRefundTx(deductCtx, db.DeductRiderDepositAndRefundTxParams{
						RiderID:   riderID,
						UserID:    userID,
						ClaimID:   claim.ID,
						Amount:    decision.Amount,
						ClaimType: claimType,
					})
					if deductErr != nil {
						// 押金不足或其他错误
						// 记录日志，后续由定时任务处理欠款或暂停接单
						fmt.Printf("Failed to deduct rider deposit and refund: riderID=%d, userID=%d, amount=%d, err=%v\n",
							riderID, userID, decision.Amount, deductErr)
					} else {
						// 发送通知给骑手
						caa.sendNotification("rider", "押金扣款通知",
							fmt.Sprintf("您有一笔%s索赔，押金已扣款%d分（索赔ID: %d）", claimType, decision.Amount, claim.ID),
							riderID)
						// 发送通知给用户（余额到账）
						if caa.notificationDistributor != nil {
							_ = caa.notificationDistributor.SendUserNotification(
								deductCtx,
								userID,
								"order",
								"索赔退款到账",
								fmt.Sprintf("您的%s索赔已处理完成，%d分已退还至您的账户余额（当前余额: %d分）", claimType, decision.Amount, result.UserBalance.Balance),
								"claim",
								claim.ID,
							)
						}
					}
				}()
			}
		}
	}

	// 餐损/食安赔付后的事后处理
	if (claimType == ClaimTypeDamage || claimType == ClaimTypeFoodSafety) && decision.NeedsReview {
		// 发现可疑模式，异步进行信用分处理
		// 使用Worker任务异步执行（如果未配置worker则降级为goroutine）
		// 接入方法：在API层通过caa.SetTaskDistributor(server.taskDistributor)设置
		// 然后通过worker.NewHandleSuspiciousPatternTask分发任务
		// 当前降级：直接用goroutine（生产环境建议使用Worker）
		go caa.handleSuspiciousPattern(ctx, userID, claim.ID, claimType, decision.LookbackData)
	}

	return &claim, nil
}

// handleSuspiciousPattern 处理可疑的餐损/食安索赔模式
// 已经赔付，但根据信用分和模式进行事后处罚
func (caa *ClaimAutoApproval) handleSuspiciousPattern(
	ctx context.Context,
	userID int64,
	claimID int64,
	claimType string,
	lookback *LookbackResult,
) {
	calculator := NewTrustScoreCalculator(caa.store, caa.wsHub)

	// 根据索赔频率和模式扣分
	var scoreChange int16
	var reason string
	var warningMessage string

	if lookback != nil && lookback.ClaimsFound >= 5 {
		// 高频索赔（5次以上）
		scoreChange = ScoreThirdMaliciousClaim // -50
		reason = "高频索赔处罚"
		warningMessage = fmt.Sprintf("您最近%s内已索赔%d次，被系统判定为恶意索赔风险，信用分-50",
			lookback.Period, lookback.ClaimsFound)
	} else if lookback != nil && lookback.ClaimsFound >= 3 {
		// 频繁索赔（3次以上）
		scoreChange = ScoreFirstMaliciousClaim // -30
		reason = "频繁索赔警告"
		warningMessage = fmt.Sprintf("您最近%s内5笔订单中已索赔%d次，系统判定有恶意索赔风险，信用分-30。继续索赔将影响您的账号使用。",
			lookback.Period, lookback.ClaimsFound)
	} else {
		// 可疑但次数不多，轻微扣分
		scoreChange = -15
		reason = "可疑模式提醒"
		warningMessage = "系统检测到可疑索赔模式，信用分-15，请注意"
	}

	relatedType := "claim"
	err := calculator.UpdateTrustScore(
		ctx,
		EntityTypeCustomer,
		userID,
		scoreChange,
		reason,
		fmt.Sprintf("%s索赔模式异常（索赔ID: %d）", claimType, claimID),
		&relatedType,
		&claimID,
	)

	if err != nil {
		// 记录错误但不影响业务
		fmt.Printf("Failed to update trust score for suspicious pattern: %v\n", err)
		return
	}

	// 发送用户通知
	// 实现方式：通过站内通知系统发送，用户打开小程序时可查看
	// 注意：
	// 1. C端用户不使用WebSocket，使用站内通知系统
	// 2. 微信订阅消息需要用户提前授权模板ID，属于运营配置层面
	// 3. 如需发送微信订阅消息，可在 NotificationDistributor 中扩展实现
	if caa.notificationDistributor != nil {
		_ = caa.notificationDistributor.SendUserNotification(
			ctx,
			userID,
			"system",       // notificationType
			"信用分变动提醒",    // title
			warningMessage, // content
			"claim",        // relatedType
			claimID,        // relatedID
		)
	}

	// 如果信用分降到阈值以下，触发措施
	// ≤600: 禁止发布评价
	// ≤450: 提醒商家（到店消费时）
	// ≤300: 完全拉黑（已在trust_score_calculator.go中自动触发）
}

// sendNotification 发送WebSocket通知
func (caa *ClaimAutoApproval) sendNotification(entityType, title, message string, entityID int64) {
	if caa.wsHub == nil {
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
		caa.wsHub.SendToMerchant(entityID, msg)
	case "rider":
		caa.wsHub.SendToRider(entityID, msg)
		// 注：用户通知需要通过其他渠道（小程序模板消息）
	}
}
