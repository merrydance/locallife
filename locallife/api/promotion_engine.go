package api

import (
	"context"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

/**
 * 核心设计原则：
 * 1. 禁止硬编码：所有阈值、金额、类型限制必须来源于数据库。
 * 2. 真实计算：余额拆分必须基于交易流水的真实汇总，而非固定比例。
 * 3. 动态引擎：优惠逻辑应遍历规则列表执行，而非 hardcode 的 if-else。
 */

// PriceCalculationResult 统一的价格计算结果模型
type PriceCalculationResult struct {
	Subtotal            int64              `json:"subtotal"`
	DeliveryFee         int64              `json:"delivery_fee"`
	DeliveryFeeDiscount int64              `json:"delivery_fee_discount"`
	VoucherDiscount     int64              `json:"voucher_discount"`
	MerchantDiscount    int64              `json:"merchant_discount"`
	TotalAmount         int64              `json:"total_amount"`
	AppliedPromotions   []AppliedPromotion `json:"applied_promotions"`
	PaymentAssessment   PaymentAssessment  `json:"payment_assessment"`
}

type AppliedPromotion struct {
	Title  string `json:"title"`
	Amount int64  `json:"amount"`
}

type PaymentAssessment struct {
	IsBalancePayable bool   `json:"is_balance_payable"`
	UsableBalance    int64  `json:"usable_balance"`
	PrincipalPart    int64  `json:"principal_part"`
	BonusPart        int64  `json:"bonus_part"`
	PaymentHint      string `json:"payment_hint"`
}

type OrderContext struct {
	MerchantID  int64
	UserID      int64
	OrderType   string
	Subtotal    int64
	VoucherID   *int64
	DeliveryFee int64
	Distance    int32
}

func (server *Server) CalculateFinalPrice(ctx context.Context, opt OrderContext) (*PriceCalculationResult, error) {
	res := &PriceCalculationResult{
		Subtotal:          opt.Subtotal,
		DeliveryFee:       opt.DeliveryFee,
		AppliedPromotions: []AppliedPromotion{}, // 显式初始化，避免返回 null
	}

	// 1. 获取商户所有激活的满减规则
	activeRules, err := server.store.ListActiveDiscountRules(ctx, opt.MerchantID)
	if err != nil {
		log.Error().Err(err).Int64("merchant_id", opt.MerchantID).Msg("Failed to list discount rules")
	}

	hasExclusivePromo := false

	// --- 1. 执行满减优惠 ---
	for _, rule := range activeRules {
		if server.isRuleMatch(rule, opt) {
			res.MerchantDiscount += rule.DiscountAmount
			res.AppliedPromotions = append(res.AppliedPromotions, AppliedPromotion{
				Title:  rule.Name,
				Amount: rule.DiscountAmount,
			})
			// 目前 DiscountRule 表通过 can_stack_with_voucher 控制叠加性
			if !rule.CanStackWithVoucher {
				hasExclusivePromo = true
			}
		}
	}

	// --- 2. 配送费优惠 ---
	// 不需要 gin.Context 断言，因为底层已更新为支持标准 context.Context
	feeRes, _ := server.calculateDeliveryFeeInternal(ctx, 0, opt.MerchantID, opt.Distance, opt.Subtotal)
	if feeRes != nil {
		res.DeliveryFeeDiscount = feeRes.PromotionDiscount
		if res.DeliveryFeeDiscount > 0 {
			res.AppliedPromotions = append(res.AppliedPromotions, AppliedPromotion{
				Title:  "配送费减免",
				Amount: res.DeliveryFeeDiscount,
			})
		}
	}

	// --- 3. 优惠券 ---
	if opt.VoucherID != nil && !hasExclusivePromo {
		voucher, err := server.store.GetUserVoucher(ctx, *opt.VoucherID)
		if err == nil && server.isVoucherValid(voucher, opt) {
			res.VoucherDiscount = voucher.Amount
			res.AppliedPromotions = append(res.AppliedPromotions, AppliedPromotion{
				Title:  voucher.Name,
				Amount: voucher.Amount,
			})
		}
	}

	// 最终计算
	res.TotalAmount = res.Subtotal + res.DeliveryFee - res.DeliveryFeeDiscount - res.VoucherDiscount - res.MerchantDiscount
	if res.TotalAmount < 0 {
		res.TotalAmount = 0
	}

	// --- 4. 支付能力评估 ---
	membership, err := server.store.GetMembershipByMerchantAndUser(ctx, db.GetMembershipByMerchantAndUserParams{
		MerchantID: opt.MerchantID,
		UserID:     opt.UserID,
	})

	principal := int64(0)
	bonus := int64(0)
	if err == nil {
		// 实效逻辑：目前 DB 虽然只有一个 Balance，但我们可以定义业务规则。
		// 在进阶版本中，这里应该查询 membership_transactions 来分类汇总余额。
		// 暂时将所有余额视为本金（Principal），未来扩展 Bonus 字段后可直接修改此逻辑。
		principal = membership.Balance
	}

	assessment := PaymentAssessment{
		PrincipalPart: principal,
		BonusPart:     bonus,
	}

	server.curatePaymentBalance(opt, res, &assessment)

	res.PaymentAssessment = assessment
	return res, nil
}

func (server *Server) isRuleMatch(rule db.DiscountRule, opt OrderContext) bool {
	// 1. 金额门槛校验
	if opt.Subtotal < rule.MinOrderAmount {
		return false
	}
	// 2. 有效期校验 (ListActiveDiscountRules 已经过滤，此处为双重保险)
	now := time.Now()
	if now.Before(rule.ValidFrom) || now.After(rule.ValidUntil) {
		return false
	}
	return true
}

func (server *Server) isVoucherValid(v db.GetUserVoucherRow, opt OrderContext) bool {
	// 1. 状态校验
	if v.Status != "unused" {
		return false
	}
	// 2. 过期校验
	if time.Now().After(v.ExpiresAt) {
		return false
	}
	// 3. 商户限制
	if v.MerchantID != opt.MerchantID && v.MerchantID != 0 {
		return false
	}
	// 4. 金额门槛
	if opt.Subtotal < v.MinOrderAmount {
		return false
	}
	// 5. 订单类型限制
	if len(v.AllowedOrderTypes) > 0 && !containsString(v.AllowedOrderTypes, opt.OrderType) {
		return false
	}
	return true
}

func (server *Server) curatePaymentBalance(opt OrderContext, res *PriceCalculationResult, assessment *PaymentAssessment) {
	// 核心业务规则：赠额用途约束
	// 规则 1：赠额原则上不支付配送费（外卖场景）
	if opt.OrderType == OrderTypeTakeout && assessment.BonusPart > 0 {
		netDeliveryFee := res.DeliveryFee - res.DeliveryFeeDiscount
		if netDeliveryFee < 0 {
			netDeliveryFee = 0
		}

		// 计算可由赠额支付的理论最大值（即非配送费部分）
		maxBonusCoverage := res.TotalAmount - netDeliveryFee
		if maxBonusCoverage < 0 {
			maxBonusCoverage = 0
		}

		if assessment.BonusPart > maxBonusCoverage {
			originalBonus := assessment.BonusPart
			assessment.BonusPart = maxBonusCoverage
			if assessment.BonusPart < originalBonus {
				assessment.PaymentHint = "支付提示：您的赠送金额暂不可抵扣配送费"
			}
		}
	}

	// 规则 2：预定押金通常仅限本金支付
	if opt.OrderType == OrderTypeReservation && assessment.BonusPart > 0 {
		assessment.BonusPart = 0
		assessment.PaymentHint = "说明：包间预定定金需使用本金支付，暂不支持赠额抵扣"
	}

	assessment.UsableBalance = assessment.PrincipalPart + assessment.BonusPart
	assessment.IsBalancePayable = assessment.UsableBalance >= res.TotalAmount

	if !assessment.IsBalancePayable && (assessment.PrincipalPart+assessment.BonusPart) > 0 {
		if assessment.PaymentHint == "" {
			assessment.PaymentHint = "可用余额不足支付本单"
		}
	}
}
