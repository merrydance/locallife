package algorithm

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// FraudDetector 团伙欺诈检测器
type FraudDetector struct {
	store db.Store
	wsHub WebSocketHub // WebSocket通知
}

// NewFraudDetector 创建团伙欺诈检测器
func NewFraudDetector(store db.Store, wsHub WebSocketHub) *FraudDetector {
	return &FraudDetector{
		store: store,
		wsHub: wsHub,
	}
}

// DetectDeviceReuse 检测设备复用
// 同一设备3个账号7天内3次索赔
func (fd *FraudDetector) DetectDeviceReuse(
	ctx context.Context,
	deviceFingerprint string,
) (*FraudDetectionResult, error) {
	if deviceFingerprint == "" {
		return &FraudDetectionResult{
			IsFraud:     false,
			PatternType: FraudPatternDeviceReuse,
			Confidence:  0,
		}, nil
	}

	// 1. 查询使用该设备的所有用户
	userIDs, err := fd.store.GetUsersByDeviceID(ctx, deviceFingerprint)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by device: %w", err)
	}

	// 需要至少3个不同用户
	if len(userIDs) < 3 {
		return &FraudDetectionResult{
			IsFraud:     false,
			PatternType: FraudPatternDeviceReuse,
			Confidence:  0,
		}, nil
	}

	// 2. 查询这些用户最近7天的索赔记录
	lookbackDays := int32(7)
	totalClaims, err := fd.store.CountRecentClaimsByUsers(ctx, db.CountRecentClaimsByUsersParams{
		Column1: userIDs,
		Column2: lookbackDays,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count recent claims: %w", err)
	}

	// 3. 如果 >=3个用户 且 >=3次索赔，创建欺诈模式记录
	if len(userIDs) >= 3 && totalClaims >= 3 {
		// 获取有索赔记录的用户
		usersWithClaims, err := fd.store.GetUsersWithRecentClaims(ctx, db.GetUsersWithRecentClaimsParams{
			Column1: userIDs,
			Column2: lookbackDays,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get users with claims: %w", err)
		}

		// 需要查询实际的索赔和订单ID
		// 为简化，我们从所有用户的所有索赔中提取
		var claimIDs []int64
		var orderIDs []int64
		sevenDaysAgo := time.Now().AddDate(0, 0, -7)

		for _, userID := range usersWithClaims {
			// 查询用户最近7天的索赔
			claims, err := fd.store.ListUserClaimsInPeriod(ctx, db.ListUserClaimsInPeriodParams{
				UserID:    userID,
				CreatedAt: sevenDaysAgo,
			})
			if err != nil {
				continue
			}

			for _, claim := range claims {
				claimIDs = append(claimIDs, claim.ID)
				orderIDs = append(orderIDs, claim.OrderID)
			}
		}

		// 创建欺诈模式记录
		description := fmt.Sprintf(
			"设备复用检测：设备 %s 被 %d 个用户使用，最近7天发起了 %d 次索赔",
			deviceFingerprint, len(userIDs), totalClaims,
		)
		_, err = fd.CreateFraudPattern(
			ctx,
			FraudPatternDeviceReuse,
			userIDs,
			orderIDs,
			claimIDs,
			[]string{deviceFingerprint},
			nil,
			int(totalClaims),
			description,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create fraud pattern: %w", err)
		}

		// 根据匹配数量计算置信度（返回匹配数）
		confidence := len(userIDs) + int(totalClaims)

		return &FraudDetectionResult{
			IsFraud:         true,
			PatternType:     FraudPatternDeviceReuse,
			Confidence:      confidence,
			RelatedUserIDs:  userIDs,
			RelatedClaimIDs: claimIDs,
			Description:     description,
			ShouldBlock:     len(userIDs) >= 5 || totalClaims >= 10,
		}, nil
	}

	return &FraudDetectionResult{
		IsFraud:     false,
		PatternType: FraudPatternDeviceReuse,
		Confidence:  0,
	}, nil
}

// DetectAddressCluster 检测地址聚类
// 同一地址3个账号7天内3次索赔
func (fd *FraudDetector) DetectAddressCluster(
	ctx context.Context,
	addressID int64,
) (*FraudDetectionResult, error) {
	if addressID == 0 {
		return &FraudDetectionResult{
			IsFraud:     false,
			PatternType: FraudPatternAddressCluster,
			Confidence:  0,
		}, nil
	}

	// 1. 查询使用该地址的所有用户
	userRows, err := fd.store.GetUsersByAddressID(ctx, addressID)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by address: %w", err)
	}

	// 需要至少3个不同用户
	if len(userRows) < 3 {
		return &FraudDetectionResult{
			IsFraud:     false,
			PatternType: FraudPatternAddressCluster,
			Confidence:  0,
		}, nil
	}

	// 提取用户ID
	userIDs := make([]int64, len(userRows))
	for i, row := range userRows {
		userIDs[i] = row.UserID
	}

	// 2. 查询这些用户最近7天的索赔记录
	lookbackDays := int32(7)
	totalClaims, err := fd.store.CountRecentClaimsByUsers(ctx, db.CountRecentClaimsByUsersParams{
		Column1: userIDs,
		Column2: lookbackDays,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count recent claims: %w", err)
	}

	// 3. 如果 >=3个用户 且 >=3次索赔，创建欺诈模式记录
	if len(userIDs) >= 3 && totalClaims >= 3 {
		// 获取有索赔记录的用户
		usersWithClaims, err := fd.store.GetUsersWithRecentClaims(ctx, db.GetUsersWithRecentClaimsParams{
			Column1: userIDs,
			Column2: lookbackDays,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get users with claims: %w", err)
		}

		// 需要查询实际的索赔和订单ID
		var claimIDs []int64
		var orderIDs []int64
		sevenDaysAgo := time.Now().AddDate(0, 0, -7)

		for _, userID := range usersWithClaims {
			// 查询用户最近7天的索赔
			claims, err := fd.store.ListUserClaimsInPeriod(ctx, db.ListUserClaimsInPeriodParams{
				UserID:    userID,
				CreatedAt: sevenDaysAgo,
			})
			if err != nil {
				continue
			}

			for _, claim := range claims {
				claimIDs = append(claimIDs, claim.ID)
				orderIDs = append(orderIDs, claim.OrderID)
			}
		}

		// 创建欺诈模式记录
		description := fmt.Sprintf(
			"地址聚类检测：地址ID %d 被 %d 个用户使用，最近7天发起了 %d 次索赔",
			addressID, len(userIDs), totalClaims,
		)
		_, err = fd.CreateFraudPattern(
			ctx,
			FraudPatternAddressCluster,
			userIDs,
			orderIDs,
			claimIDs,
			nil,
			[]int64{addressID},
			int(totalClaims),
			description,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create fraud pattern: %w", err)
		}

		// 根据匹配数量计算置信度（返回匹配数）
		confidence := len(userIDs) + int(totalClaims)

		return &FraudDetectionResult{
			IsFraud:         true,
			PatternType:     FraudPatternAddressCluster,
			Confidence:      confidence,
			RelatedUserIDs:  userIDs,
			RelatedClaimIDs: claimIDs,
			Description:     description,
			ShouldBlock:     len(userIDs) >= 5 || totalClaims >= 10,
		}, nil
	}

	return &FraudDetectionResult{
		IsFraud:     false,
		PatternType: FraudPatternAddressCluster,
		Confidence:  0,
	}, nil
}

// DetectCoordinatedClaims 检测协同索赔
// 条件: 1小时内3+用户对同一商户索赔 且 这些用户共享设备或地址
// 注意: 如果3+毫无关联的用户都投诉同一商户，那是商户问题，不是欺诈
func (fd *FraudDetector) DetectCoordinatedClaims(
	ctx context.Context,
	claimID int64,
) (*FraudDetectionResult, error) {
	// 获取当前索赔
	claim, err := fd.store.GetClaim(ctx, claimID)
	if err != nil {
		return nil, err
	}

	// 获取当前索赔对应订单的商户ID
	order, err := fd.store.GetOrder(ctx, claim.OrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order for claim: %w", err)
	}
	currentMerchantID := order.MerchantID

	// 时间窗口：前后1小时
	windowStart := claim.CreatedAt.Add(-1 * time.Hour)
	windowEnd := claim.CreatedAt.Add(1 * time.Hour)

	// 查询同一时间窗口内的其他索赔
	claimsInWindow, err := fd.store.ListClaimsByTimeWindow(ctx, db.ListClaimsByTimeWindowParams{
		CreatedAt:   windowStart,
		CreatedAt_2: windowEnd,
		ID:          claimID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query claims in time window: %w", err)
	}

	// 过滤出同一商户的索赔（需要查询每个索赔对应的订单）
	var sameMerchantClaims []db.ListClaimsByTimeWindowRow
	for _, c := range claimsInWindow {
		claimOrder, err := fd.store.GetOrder(ctx, c.OrderID)
		if err != nil {
			continue
		}
		if claimOrder.MerchantID == currentMerchantID {
			sameMerchantClaims = append(sameMerchantClaims, c)
		}
	}

	if len(sameMerchantClaims) < 2 {
		// 同一商户的索赔不足，不构成协同欺诈
		return &FraudDetectionResult{
			IsFraud:     false,
			PatternType: FraudPatternCoordinatedClaims,
			Confidence:  0,
		}, nil
	}

	// 统计不同用户
	userSet := make(map[int64]struct{})
	userSet[claim.UserID] = struct{}{}
	for _, c := range sameMerchantClaims {
		userSet[c.UserID] = struct{}{}
	}

	var relatedUserIDs []int64
	for userID := range userSet {
		relatedUserIDs = append(relatedUserIDs, userID)
	}
	distinctUsers := len(relatedUserIDs)

	// 不足3个用户，不构成协同欺诈
	if distinctUsers < 3 {
		return &FraudDetectionResult{
			IsFraud:     false,
			PatternType: FraudPatternCoordinatedClaims,
			Confidence:  distinctUsers,
		}, nil
	}

	// ⚠️ 关键检查：用户之间是否有关联（共享设备或地址）
	// 如果没有关联，说明是商户问题而非用户欺诈
	hasAssociation, associationType, err := fd.checkUserAssociation(ctx, relatedUserIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to check user association: %w", err)
	}

	if !hasAssociation {
		// 用户之间没有关联 → 这是商户问题，需要调查商户
		return &FraudDetectionResult{
			IsFraud:           false,
			PatternType:       FraudPatternCoordinatedClaims,
			Confidence:        0,
			Description:       fmt.Sprintf("1小时内%d个独立用户投诉同一商户，需调查商户而非用户", distinctUsers),
			MerchantSuspect:   true, // 标记商户可疑
			RelatedUserIDs:    relatedUserIDs,
			SuspectMerchantID: currentMerchantID,
		}, nil
	}

	// 用户有关联 → 欺诈嫌疑
	// 收集相关数据
	var relatedClaimIDs []int64
	var relatedOrderIDs []int64
	addressSet := make(map[int64]struct{})

	relatedClaimIDs = append(relatedClaimIDs, claimID)
	for _, c := range sameMerchantClaims {
		relatedClaimIDs = append(relatedClaimIDs, c.ID)
		relatedOrderIDs = append(relatedOrderIDs, c.OrderID)
	}

	// 获取收货地址
	for _, orderID := range relatedOrderIDs {
		orderData, err := fd.store.GetOrder(ctx, orderID)
		if err == nil && orderData.AddressID.Valid {
			addressSet[orderData.AddressID.Int64] = struct{}{}
		}
	}

	var addressIDs []int64
	for addrID := range addressSet {
		addressIDs = append(addressIDs, addrID)
	}

	// 创建欺诈模式记录
	description := fmt.Sprintf(
		"协同索赔检测：1小时内 %d 个关联用户（%s）对商户 %d 发起了 %d 次索赔",
		distinctUsers, associationType, currentMerchantID, len(relatedClaimIDs),
	)

	_, err = fd.CreateFraudPattern(
		ctx,
		FraudPatternCoordinatedClaims,
		relatedUserIDs,
		relatedOrderIDs,
		relatedClaimIDs,
		nil,
		addressIDs,
		len(relatedClaimIDs),
		description,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create fraud pattern: %w", err)
	}

	return &FraudDetectionResult{
		IsFraud:         true,
		PatternType:     FraudPatternCoordinatedClaims,
		Confidence:      distinctUsers + len(relatedClaimIDs),
		RelatedUserIDs:  relatedUserIDs,
		RelatedClaimIDs: relatedClaimIDs,
		Description:     description,
		ShouldBlock:     distinctUsers >= 5 || len(relatedClaimIDs) >= 10,
	}, nil
}

// checkUserAssociation 检查用户之间是否有关联
// 关联类型: 共享设备、共享地址、已存在欺诈记录
// 返回: 是否有关联, 关联类型描述, 错误
func (fd *FraudDetector) checkUserAssociation(
	ctx context.Context,
	userIDs []int64,
) (bool, string, error) {
	if len(userIDs) < 2 {
		return false, "", nil
	}

	// 检查1: 是否存在已确认的欺诈模式（这些用户已被标记为团伙）
	patterns, err := fd.store.GetFraudPatternsByUsers(ctx, userIDs)
	if err == nil && len(patterns) > 0 {
		for _, pattern := range patterns {
			if pattern.IsConfirmed {
				return true, fmt.Sprintf("已确认欺诈团伙-%s", pattern.PatternType), nil
			}
		}
	}

	// 检查2: 共享设备
	// 查询每个用户的设备指纹
	deviceMap := make(map[string][]int64) // device -> user_ids
	for _, uid := range userIDs {
		devices, err := fd.store.GetDevicesByUserID(ctx, uid)
		if err != nil {
			continue
		}
		for _, device := range devices {
			deviceMap[device.DeviceID] = append(deviceMap[device.DeviceID], uid)
		}
	}
	// 检查是否有设备被多个用户共享
	for device, users := range deviceMap {
		if len(users) >= 2 {
			return true, fmt.Sprintf("共享设备:%s", device[:min(8, len(device))]), nil
		}
	}

	// 检查3: 共享地址
	// 查询每个用户最近订单的收货地址
	addressMap := make(map[int64][]int64) // address_id -> user_ids
	for _, uid := range userIDs {
		orders, err := fd.store.ListUserRecentOrders(ctx, db.ListUserRecentOrdersParams{
			UserID: uid,
			Limit:  5, // 查最近5单
		})
		if err != nil || len(orders) == 0 {
			continue
		}
		for _, order := range orders {
			if order.AddressID.Valid {
				addressMap[order.AddressID.Int64] = append(addressMap[order.AddressID.Int64], uid)
			}
		}
	}
	// 检查是否有地址被多个用户共享
	for addrID, users := range addressMap {
		if len(users) >= 2 {
			return true, fmt.Sprintf("共享地址ID:%d", addrID), nil
		}
	}

	// 检查4: 都是新用户且首单（可疑的批量注册）
	allNewUsersFirstOrder := true
	for _, uid := range userIDs {
		profile, err := fd.store.GetUserProfile(ctx, db.GetUserProfileParams{
			UserID: uid,
			Role:   EntityTypeCustomer,
		})
		if err != nil {
			continue
		}
		if profile.TotalOrders > 1 {
			allNewUsersFirstOrder = false
			break
		}
	}
	if allNewUsersFirstOrder && len(userIDs) >= 3 {
		return true, "批量新用户首单", nil
	}

	// 没有发现关联
	return false, "", nil
}

// CreateFraudPattern 创建欺诈模式记录
func (fd *FraudDetector) CreateFraudPattern(
	ctx context.Context,
	patternType string,
	relatedUserIDs []int64,
	relatedOrderIDs []int64,
	relatedClaimIDs []int64,
	deviceFingerprints []string,
	addressIDs []int64,
	matchCount int,
	description string,
) (*db.FraudPattern, error) {
	// 判断是否应该自动确认
	isConfirmed := matchCount >= HighMatchCount || len(relatedClaimIDs) >= 5

	pattern, err := fd.store.CreateFraudPattern(ctx, db.CreateFraudPatternParams{
		PatternType:        patternType,
		RelatedUserIds:     relatedUserIDs,
		RelatedOrderIds:    relatedOrderIDs,
		RelatedClaimIds:    relatedClaimIDs,
		DeviceFingerprints: deviceFingerprints,
		AddressIds:         addressIDs,
		PatternDescription: pgtype.Text{String: description, Valid: true},
		MatchCount:         int16(matchCount),
		IsConfirmed:        isConfirmed,
		DetectedAt:         time.Now(),
	})
	if err != nil {
		return nil, err
	}

	// 如果自动确认，立即执行惩罚
	if isConfirmed {
		err = fd.HandleConfirmedFraud(ctx, pattern.ID)
		if err != nil {
			return nil, err
		}
	}

	return &pattern, nil
}

// HandleConfirmedFraud 处理已确认的欺诈
func (fd *FraudDetector) HandleConfirmedFraud(
	ctx context.Context,
	fraudPatternID int64,
) error {
	pattern, err := fd.store.GetFraudPattern(ctx, fraudPatternID)
	if err != nil {
		return err
	}

	if !pattern.IsConfirmed {
		return fmt.Errorf("fraud pattern %d is not confirmed", fraudPatternID)
	}

	// Step 1: 拉黑所有涉及的用户
	for _, userID := range pattern.RelatedUserIds {
		err := fd.store.BlacklistUser(ctx, db.BlacklistUserParams{
			UserID: userID,
			Role:   EntityTypeCustomer,
			BlacklistReason: pgtype.Text{
				String: fmt.Sprintf("确认为欺诈团伙成员（模式ID: %d）", fraudPatternID),
				Valid:  true,
			},
		})
		if err != nil {
			// 继续处理其他用户
			continue
		}

		// 扣除信用分
		calculator := NewTrustScoreCalculator(fd.store, fd.wsHub)
		relatedType := "fraud-pattern"
		err = calculator.UpdateTrustScore(
			ctx,
			EntityTypeCustomer,
			userID,
			ScoreMaliciousClaim, // -100分
			"confirmed-fraud-pattern",
			fmt.Sprintf("确认为欺诈团伙成员（模式ID: %d）", fraudPatternID),
			&relatedType,
			&fraudPatternID,
		)
		if err != nil {
			continue
		}
	}

	// Step 2: 查找受损商户和骑手，返还损失
	// 使用新的查询获取相关索赔详情
	_, err = fd.store.GetClaimsByFraudPattern(ctx, pattern.RelatedClaimIds)
	if err != nil {
		// 非致命错误，继续处理
	}

	// 统计商户损失并记录（实际返还通过财务系统处理）
	merchantLosses, _ := fd.store.SumClaimAmountsByMerchant(ctx, pattern.RelatedClaimIds)
	var totalMerchantRecovery int64
	for _, loss := range merchantLosses {
		totalMerchantRecovery += loss.TotalLoss
	}

	// 统计骑手损失（餐损类型）
	riderLosses, _ := fd.store.SumClaimAmountsByRider(ctx, pattern.RelatedClaimIds)
	var totalRiderRecovery int64
	for _, loss := range riderLosses {
		totalRiderRecovery += loss.TotalLoss
	}

	// 构建返还记录描述
	actionDescription := fmt.Sprintf("block_users;merchant_refund:%d;rider_refund:%d",
		totalMerchantRecovery, totalRiderRecovery)

	// Step 3: 更新欺诈模式记录（包含返还金额信息）
	err = fd.store.ConfirmFraudPattern(ctx, db.ConfirmFraudPatternParams{
		ID:          fraudPatternID,
		ActionTaken: pgtype.Text{String: actionDescription, Valid: true},
	})
	if err != nil {
		return err
	}

	// 发送通知给相关用户（封禁通知）
	// 注：用户通知需要通过微信模板消息
	// 实际生产环境应集成微信模板消息服务

	return nil
}

// CheckUserForFraud 综合检查用户是否存在欺诈行为
// 在用户提交索赔时调用
func (fd *FraudDetector) CheckUserForFraud(
	ctx context.Context,
	userID int64,
	deviceFingerprint string,
	addressID int64,
) (*FraudDetectionResult, error) {
	// 检查设备复用
	if deviceFingerprint != "" {
		result, err := fd.DetectDeviceReuse(ctx, deviceFingerprint)
		if err == nil && result.IsFraud {
			return result, nil
		}
	}

	// 检查地址聚类
	if addressID > 0 {
		result, err := fd.DetectAddressCluster(ctx, addressID)
		if err == nil && result.IsFraud {
			return result, nil
		}
	}

	return &FraudDetectionResult{
		IsFraud:    false,
		Confidence: 0,
	}, nil
}
