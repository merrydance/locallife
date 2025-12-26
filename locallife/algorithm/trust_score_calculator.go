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

// TrustScoreCalculator 信用分计算器
type TrustScoreCalculator struct {
	store db.Store
	wsHub WebSocketHub // WebSocket通知
}

// NewTrustScoreCalculator 创建信用分计算器
func NewTrustScoreCalculator(store db.Store, wsHub WebSocketHub) *TrustScoreCalculator {
	return &TrustScoreCalculator{
		store: store,
		wsHub: wsHub,
	}
}

// UpdateTrustScore 更新信用分
func (tsc *TrustScoreCalculator) UpdateTrustScore(
	ctx context.Context,
	entityType string,
	entityID int64,
	scoreChange int16,
	reasonType string,
	reasonDescription string,
	relatedType *string,
	relatedID *int64,
) error {
	var oldScore int16
	var newScore int16

	switch entityType {
	case EntityTypeCustomer:
		profile, err := tsc.store.GetUserProfileForUpdate(ctx, db.GetUserProfileForUpdateParams{
			UserID: entityID,
			Role:   EntityTypeCustomer,
		})
		if err != nil {
			return fmt.Errorf("failed to get user profile: %w", err)
		}

		oldScore = profile.TrustScore
		newScore = ClampInt16(oldScore+scoreChange, TrustScoreMin, TrustScoreMax)

		err = tsc.store.UpdateUserTrustScore(ctx, db.UpdateUserTrustScoreParams{
			UserID:     entityID,
			Role:       EntityTypeCustomer,
			TrustScore: newScore,
		})
		if err != nil {
			return err
		}

	case EntityTypeMerchant:
		profile, err := tsc.store.GetMerchantProfileForUpdate(ctx, entityID)
		if err != nil {
			return fmt.Errorf("failed to get merchant profile: %w", err)
		}

		oldScore = profile.TrustScore
		newScore = ClampInt16(oldScore+scoreChange, TrustScoreMin, TrustScoreMax)

		err = tsc.store.UpdateMerchantTrustScore(ctx, db.UpdateMerchantTrustScoreParams{
			MerchantID: entityID,
			TrustScore: newScore,
		})
		if err != nil {
			return err
		}

	case EntityTypeRider:
		profile, err := tsc.store.GetRiderProfileForUpdate(ctx, entityID)
		if err != nil {
			return fmt.Errorf("failed to get rider profile: %w", err)
		}

		oldScore = profile.TrustScore
		newScore = ClampInt16(oldScore+scoreChange, TrustScoreMin, TrustScoreMax)

		err = tsc.store.UpdateRiderTrustScore(ctx, db.UpdateRiderTrustScoreParams{
			RiderID:    entityID,
			TrustScore: newScore,
		})
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("invalid entity type: %s", entityType)
	}

	// 记录信用分变更
	changeParams := db.CreateTrustScoreChangeParams{
		EntityType:        entityType,
		EntityID:          entityID,
		OldScore:          oldScore,
		NewScore:          newScore,
		ScoreChange:       scoreChange,
		ReasonType:        reasonType,
		ReasonDescription: reasonDescription,
		IsAuto:            true,
		CreatedAt:         time.Now(),
	}

	if relatedType != nil {
		changeParams.RelatedType = pgtype.Text{String: *relatedType, Valid: true}
	}
	if relatedID != nil {
		changeParams.RelatedID = pgtype.Int8{Int64: *relatedID, Valid: true}
	}

	_, err := tsc.store.CreateTrustScoreChange(ctx, changeParams)
	if err != nil {
		return fmt.Errorf("failed to create trust score change: %w", err)
	}

	// 检查阈值触发
	err = tsc.checkThresholds(ctx, entityType, entityID, newScore)
	if err != nil {
		return err
	}

	return nil
}

// checkThresholds 检查信用分阈值触发
// 新版本：使用 100 分制，70 分以下拒绝服务
// 注意：骑手不再使用信用分系统，改用高值单资格机制激励
func (tsc *TrustScoreCalculator) checkThresholds(
	ctx context.Context,
	entityType string,
	entityID int64,
	newScore int16,
) error {
	switch entityType {
	case EntityTypeCustomer:
		if newScore < TrustScoreRejectService {
			// 禁止下单（信用分低于70）
			err := tsc.store.BlacklistUser(ctx, db.BlacklistUserParams{
				UserID: entityID,
				Role:   EntityTypeCustomer,
				BlacklistReason: pgtype.Text{
					String: fmt.Sprintf("信用分降至%d分（低于%d分）", newScore, TrustScoreRejectService),
					Valid:  true,
				},
			})
			if err != nil {
				return err
			}
			// 发送封禁通知
			go tsc.sendNotification("customer", "账号已被限制",
				fmt.Sprintf("您的信用分已降至%d分，下单功能已被限制。请改善行为以恢复信用", newScore), entityID)
		} else if newScore < TrustScoreWarning {
			// 发送警告通知（信用分低于85）
			go tsc.sendNotification("customer", "信用分警告",
				fmt.Sprintf("您的信用分已降至%d分，请注意您的行为。继续违规可能导致账号受限", newScore), entityID)
		}

	case EntityTypeMerchant:
		// 商户的信用分机制主要用于食安和异物问题
		// 异物追踪由 MerchantForeignObjectTracker 独立处理
		// 食安问题需要人工审核恢复
		if newScore < TrustScoreRejectService {
			// 直接封禁停业（可申请恢复）
			err := tsc.store.SuspendMerchant(ctx, db.SuspendMerchantParams{
				MerchantID: entityID,
				SuspendReason: pgtype.Text{
					String: fmt.Sprintf("信用分降至%d分（低于%d分），已封禁停业，可在线申请恢复", newScore, TrustScoreRejectService),
					Valid:  true,
				},
			})
			if err != nil {
				return err
			}
			// 发送通知（包含申请恢复入口）
			go tsc.sendNotification("merchant", "店铺已封禁",
				fmt.Sprintf("您的信用分已降至%d分，店铺已被封禁。可在商家后台提交恢复申请并承诺改善", newScore), entityID)
		}

	case EntityTypeRider:
		// 骑手使用「高值单资格积分」机制（premium_score）
		// 积分规则：完成普通单+1，完成高值单-3，超时-5，餐损-10
		// 积分≥0才能接高值单，积分可为负
		// 骑手赚不到钱自己会离开，不需要额外的惩罚机制
	}

	return nil
}

// DecrementMaliciousClaim 恶意索赔扣分（累进制）
// 首次-30，第二次-40，第三次-50，第五次-200
func (tsc *TrustScoreCalculator) DecrementMaliciousClaim(
	ctx context.Context,
	userID int64,
	claimID int64,
) error {
	// 获取用户当前恶意索赔次数
	profile, err := tsc.store.GetUserProfileForUpdate(ctx, db.GetUserProfileForUpdateParams{
		UserID: userID,
		Role:   EntityTypeCustomer,
	})
	if err != nil {
		return err
	}

	maliciousCount := profile.MaliciousClaims + 1

	var scoreChange int16
	var description string

	switch maliciousCount {
	case 1:
		scoreChange = ScoreFirstMaliciousClaim
		description = "首次恶意索赔"
	case 2:
		scoreChange = ScoreSecondMaliciousClaim
		description = "第二次恶意索赔"
	case 3:
		scoreChange = ScoreThirdMaliciousClaim
		description = "第三次恶意索赔"
	case 5:
		scoreChange = ScoreFifthMaliciousClaim
		description = "第五次恶意索赔，严重违规"
	default:
		scoreChange = ScoreMaliciousClaim
		description = fmt.Sprintf("第%d次恶意索赔", maliciousCount)
	}

	// 更新信用分
	relatedType := "claim"
	err = tsc.UpdateTrustScore(
		ctx,
		EntityTypeCustomer,
		userID,
		scoreChange,
		"malicious-claim",
		description,
		&relatedType,
		&claimID,
	)
	if err != nil {
		return err
	}

	// 更新恶意索赔计数
	err = tsc.store.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		UserID: userID,
		Role:   EntityTypeCustomer,
		MaliciousClaims: pgtype.Int4{
			Int32: maliciousCount,
			Valid: true,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// ProcessRecoveryRequest 处理恢复申请（商户/骑手）
// 救济机制：系统自动给一次机会，第二次再犯永久封禁
func (tsc *TrustScoreCalculator) ProcessRecoveryRequest(
	ctx context.Context,
	entityType string,
	entityID int64,
	commitmentMessage string, // 改善承诺
) error {
	// 检查恢复次数
	changes, err := tsc.store.ListEntityTrustScoreChanges(ctx, db.ListEntityTrustScoreChangesParams{
		EntityType: entityType,
		EntityID:   entityID,
		Limit:      100,
		Offset:     0,
	})
	if err != nil {
		return fmt.Errorf("failed to get trust score changes: %w", err)
	}

	// 统计已恢复次数
	recoveryCount := 0
	for _, change := range changes {
		if change.ReasonType == "recovery-granted" {
			recoveryCount++
		}
	}

	if recoveryCount >= MaxRecoveryAttempts {
		// 已用完恢复机会，永久封禁
		return fmt.Errorf("已超过最大恢复次数（%d次），永久封禁", MaxRecoveryAttempts)
	}

	// 自动批准恢复，恢复到阈值+10分
	var recoverScore int16
	var profileUpdateErr error

	switch entityType {
	case EntityTypeMerchant:
		recoverScore = TrustScoreSuspendMerchant + 10 // 恢复到410分

		// 取消停业
		profileUpdateErr = tsc.store.UpdateMerchantProfile(ctx, db.UpdateMerchantProfileParams{
			MerchantID:  entityID,
			TrustScore:  pgtype.Int2{Int16: recoverScore, Valid: true},
			IsSuspended: pgtype.Bool{Bool: false, Valid: true},
		})

	case EntityTypeRider:
		recoverScore = TrustScoreSuspendRider + 10 // 恢复到360分

		// 取消暂停
		profileUpdateErr = tsc.store.UpdateRiderProfile(ctx, db.UpdateRiderProfileParams{
			RiderID:     entityID,
			TrustScore:  pgtype.Int2{Int16: recoverScore, Valid: true},
			IsSuspended: pgtype.Bool{Bool: false, Valid: true},
		})

	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}

	if profileUpdateErr != nil {
		return fmt.Errorf("failed to update profile: %w", profileUpdateErr)
	}

	// 记录恢复日志
	relatedType := "recovery"
	oldScore := int16(0) // 封禁时的分数
	switch entityType {
	case EntityTypeMerchant:
		oldScore = TrustScoreSuspendMerchant
	case EntityTypeRider:
		oldScore = TrustScoreSuspendRider
	}

	_, err = tsc.store.CreateTrustScoreChange(ctx, db.CreateTrustScoreChangeParams{
		EntityType:        entityType,
		EntityID:          entityID,
		OldScore:          oldScore,
		NewScore:          recoverScore,
		ScoreChange:       recoverScore - oldScore,
		ReasonType:        "recovery-granted",
		ReasonDescription: fmt.Sprintf("恢复申请已批准（第%d次机会），承诺：%s", recoveryCount+1, commitmentMessage),
		RelatedType:       pgtype.Text{String: relatedType, Valid: true},
		IsAuto:            true,
		CreatedAt:         time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to create recovery log: %w", err)
	}

	// 发送恢复通知
	message := fmt.Sprintf("恢复申请已批准，信用分恢复至%d。这是您的第%d次恢复机会，请遵守规则。", recoverScore, recoveryCount+1)
	switch entityType {
	case EntityTypeMerchant:
		go tsc.sendNotification("merchant", "店铺已恢复", message, entityID)
	case EntityTypeRider:
		go tsc.sendNotification("rider", "接单权限已恢复", message, entityID)
	}

	return nil
}

// sendNotification 发送WebSocket通知
func (tsc *TrustScoreCalculator) sendNotification(entityType, title, message string, entityID int64) {
	if tsc.wsHub == nil {
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
		tsc.wsHub.SendToMerchant(entityID, msg)
	case "rider":
		tsc.wsHub.SendToRider(entityID, msg)
		// 注：用户通知需要通过其他渠道（小程序模板消息）
	}
}
