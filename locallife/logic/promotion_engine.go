package logic

import (
	"context"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

// PriceCalculationResult describes the result of promotion calculation.
type PriceCalculationResult struct {
	Subtotal            int64              `json:"subtotal"`
	DeliveryFee         int64              `json:"delivery_fee"`
	DeliveryFeeDiscount int64              `json:"delivery_fee_discount"`
	VoucherDiscount     int64              `json:"voucher_discount"`
	MerchantDiscount    int64              `json:"merchant_discount"`
	TotalAmount         int64              `json:"total_amount"`
	AppliedPromotions   []AppliedPromotion `json:"applied_promotions"`
	SuggestedVoucher    *SuggestedVoucher  `json:"suggested_voucher,omitempty"`
	PaymentAssessment   PaymentAssessment  `json:"payment_assessment"`
}

// AppliedPromotion represents an applied promotion detail.
type AppliedPromotion struct {
	Title  string `json:"title"`
	Amount int64  `json:"amount"`
	Type   string `json:"type"` // merchant, voucher, delivery
}

// SuggestedVoucher suggests a voucher for preview only.
type SuggestedVoucher struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Amount int64  `json:"amount"`
}

// PaymentAssessment provides a quick balance assessment.
type PaymentAssessment struct {
	IsBalancePayable bool   `json:"is_balance_payable"`
	UsableBalance    int64  `json:"usable_balance"`
	PrincipalPart    int64  `json:"principal_part"`
	BonusPart        int64  `json:"bonus_part"`
	PaymentHint      string `json:"payment_hint"`
}

// OrderContext describes the order context for promotion calculation.
type OrderContext struct {
	MerchantID          int64
	UserID              int64
	OrderType           string
	Subtotal            int64
	VoucherID           *int64
	DeliveryFee         int64
	DeliveryFeeDiscount int64
}

// PromotionEngine encapsulates promotion calculation logic.
type PromotionEngine struct {
	store db.Store
	now   func() time.Time
}

// NewPromotionEngine creates a promotion engine.
func NewPromotionEngine(store db.Store) *PromotionEngine {
	return &PromotionEngine{store: store, now: time.Now}
}

// CalculateFinalPrice calculates final price with active promotions.
func (engine *PromotionEngine) CalculateFinalPrice(ctx context.Context, opt OrderContext) (*PriceCalculationResult, error) {
	res := &PriceCalculationResult{
		Subtotal:            opt.Subtotal,
		DeliveryFee:         opt.DeliveryFee,
		DeliveryFeeDiscount: opt.DeliveryFeeDiscount,
		AppliedPromotions:   []AppliedPromotion{},
	}

	hasExclusivePromo := false

	// 1) Merchant discount rules
	activeRules, err := engine.store.ListActiveDiscountRules(ctx, opt.MerchantID)
	if err != nil {
		log.Error().Err(err).Int64("merchant_id", opt.MerchantID).Msg("failed to list discount rules")
	}

	selectedRules := pickBestRulesByStackingGroup(activeRules)
	for _, rule := range selectedRules {
		if !isRuleMatch(rule, opt, engine.now()) {
			continue
		}
		res.MerchantDiscount += rule.DiscountAmount
		res.AppliedPromotions = append(res.AppliedPromotions, AppliedPromotion{
			Title:  rule.Name,
			Amount: rule.DiscountAmount,
			Type:   "merchant",
		})
		if !rule.CanStackWithVoucher {
			hasExclusivePromo = true
		}
	}

	// 2) Delivery fee promotion (already computed in caller)
	if res.DeliveryFeeDiscount > 0 {
		res.AppliedPromotions = append(res.AppliedPromotions, AppliedPromotion{
			Title:  "代取费减免",
			Amount: res.DeliveryFeeDiscount,
			Type:   "delivery",
		})
	}

	// 3) Voucher
	if opt.VoucherID != nil && !hasExclusivePromo {
		voucher, err := engine.store.GetUserVoucher(ctx, *opt.VoucherID)
		if err == nil && isVoucherValid(voucher, opt, engine.now()) {
			res.VoucherDiscount = voucher.Amount
			res.AppliedPromotions = append(res.AppliedPromotions, AppliedPromotion{
				Title:  voucher.Name,
				Amount: voucher.Amount,
				Type:   "voucher",
			})
		}
	} else if opt.VoucherID == nil && !hasExclusivePromo {
		if suggested, err := suggestBestVoucher(ctx, engine.store, opt); err == nil {
			res.SuggestedVoucher = suggested
		}
	}

	// Final total
	res.TotalAmount = res.Subtotal + res.DeliveryFee - res.DeliveryFeeDiscount - res.VoucherDiscount - res.MerchantDiscount
	if res.TotalAmount < 0 {
		res.TotalAmount = 0
	}

	// 4) Balance assessment (fallback: treat all as principal if split not available)
	assessment := PaymentAssessment{}
	membership, err := engine.store.GetMembershipByMerchantAndUser(ctx, db.GetMembershipByMerchantAndUserParams{
		MerchantID: opt.MerchantID,
		UserID:     opt.UserID,
	})
	if err == nil {
		principal := membership.Balance
		bonus := int64(0)
		assessment.PrincipalPart = principal
		assessment.BonusPart = bonus
	}

	curatePaymentBalance(opt, res, &assessment)
	res.PaymentAssessment = assessment
	return res, nil
}

func pickBestRulesByStackingGroup(rules []db.DiscountRule) []db.DiscountRule {
	selected := make(map[string]db.DiscountRule)
	for _, rule := range rules {
		group := "default"
		if rule.StackingGroup.Valid && rule.StackingGroup.String != "" {
			group = rule.StackingGroup.String
		}
		current, exists := selected[group]
		if !exists || rule.DiscountAmount > current.DiscountAmount {
			selected[group] = rule
		}
	}
	res := make([]db.DiscountRule, 0, len(selected))
	for _, rule := range selected {
		res = append(res, rule)
	}
	return res
}

func isRuleMatch(rule db.DiscountRule, opt OrderContext, now time.Time) bool {
	if opt.Subtotal < rule.MinOrderAmount {
		return false
	}
	if now.Before(rule.ValidFrom) || now.After(rule.ValidUntil) {
		return false
	}
	return true
}

func isVoucherValid(v db.GetUserVoucherRow, opt OrderContext, now time.Time) bool {
	if v.Status != "unused" {
		return false
	}
	if now.After(v.ExpiresAt) {
		return false
	}
	if v.MerchantID != opt.MerchantID && v.MerchantID != 0 {
		return false
	}
	if opt.Subtotal < v.MinOrderAmount {
		return false
	}
	if len(v.AllowedOrderTypes) > 0 && !containsString(v.AllowedOrderTypes, opt.OrderType) {
		return false
	}
	return true
}

func suggestBestVoucher(ctx context.Context, store db.Store, opt OrderContext) (*SuggestedVoucher, error) {
	vouchers, err := store.ListUserAvailableVouchersForMerchant(ctx, db.ListUserAvailableVouchersForMerchantParams{
		UserID:         opt.UserID,
		MerchantID:     opt.MerchantID,
		MinOrderAmount: opt.Subtotal,
	})
	if err != nil || len(vouchers) == 0 {
		return nil, err
	}
	for _, candidate := range vouchers {
		if len(candidate.AllowedOrderTypes) == 0 || containsString(candidate.AllowedOrderTypes, opt.OrderType) {
			return &SuggestedVoucher{ID: candidate.ID, Name: candidate.Name, Amount: candidate.Amount}, nil
		}
	}
	return nil, nil
}

func containsString(arr []string, target string) bool {
	for _, v := range arr {
		if v == target {
			return true
		}
	}
	return false
}

func curatePaymentBalance(opt OrderContext, res *PriceCalculationResult, assessment *PaymentAssessment) {
	if opt.OrderType == "takeout" && assessment.BonusPart > 0 {
		netDeliveryFee := res.DeliveryFee - res.DeliveryFeeDiscount
		if netDeliveryFee < 0 {
			netDeliveryFee = 0
		}
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

	if opt.OrderType == "reservation" && assessment.BonusPart > 0 {
		assessment.BonusPart = 0
		assessment.PaymentHint = "说明：包间预定定金需使用本金支付，暂不支持赠额抵扣"
	}

	assessment.UsableBalance = assessment.PrincipalPart + assessment.BonusPart
	assessment.IsBalancePayable = assessment.UsableBalance >= res.TotalAmount

	if !assessment.IsBalancePayable && assessment.UsableBalance > 0 {
		if assessment.PaymentHint == "" {
			assessment.PaymentHint = "可用余额不足支付本单"
		}
	}
}
