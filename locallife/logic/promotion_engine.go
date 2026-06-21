package logic

import (
	"context"
	"errors"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

// PriceCalculationResult describes the result of promotion calculation.
type PriceCalculationResult struct {
	Subtotal            int64              `json:"subtotal"`
	PackagingFee        int64              `json:"packaging_fee"`
	DeliveryFee         int64              `json:"delivery_fee"`
	DeliveryFeeDiscount int64              `json:"delivery_fee_discount"`
	VoucherDiscount     int64              `json:"voucher_discount"`
	MerchantDiscount    int64              `json:"merchant_discount"`
	TotalAmount         int64              `json:"total_amount"`
	AppliedPromotions   []AppliedPromotion `json:"applied_promotions"`
	SuggestedVoucher    *SuggestedVoucher  `json:"suggested_voucher,omitempty"`
	LadderPromotions    []LadderPromotion  `json:"ladder_promotions,omitempty"`
	VoucherTrials       []VoucherTrial     `json:"voucher_trials,omitempty"`
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

// LadderPromotion describes available ladder discount preview.
type LadderPromotion struct {
	RuleID      int64  `json:"rule_id"`
	Name        string `json:"name"`
	Threshold   int64  `json:"threshold"`
	Discount    int64  `json:"discount"`
	CurrentHit  bool   `json:"current_hit"`
	MissingNeed int64  `json:"missing_need"`
}

// VoucherTrial describes voucher simulation result.
type VoucherTrial struct {
	VoucherID    int64  `json:"voucher_id"`
	VoucherName  string `json:"voucher_name"`
	Amount       int64  `json:"amount"`
	TrialPayable int64  `json:"trial_payable"`
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
	PackagingFee        int64
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
		PackagingFee:        opt.PackagingFee,
		DeliveryFee:         opt.DeliveryFee,
		DeliveryFeeDiscount: opt.DeliveryFeeDiscount,
		AppliedPromotions:   []AppliedPromotion{},
		LadderPromotions:    []LadderPromotion{},
		VoucherTrials:       []VoucherTrial{},
	}

	hasExclusivePromo := false
	hasMembershipExclusivePromo := false

	// 1) Merchant discount rules
	activeRules, err := engine.store.ListActiveDiscountRules(ctx, opt.MerchantID)
	if err != nil {
		log.Error().Err(err).Int64("merchant_id", opt.MerchantID).Msg("failed to list discount rules")
	}
	res.LadderPromotions = buildLadderPromotions(activeRules, opt, engine.now())

	selectedRules := pickBestMatchingRulesByStackingGroup(activeRules, opt, engine.now())
	for _, rule := range selectedRules {
		res.MerchantDiscount += rule.DiscountAmount
		res.AppliedPromotions = append(res.AppliedPromotions, AppliedPromotion{
			Title:  rule.Name,
			Amount: rule.DiscountAmount,
			Type:   "merchant",
		})
		if !rule.CanStackWithVoucher {
			hasExclusivePromo = true
		}
		if !rule.CanStackWithMembership {
			hasMembershipExclusivePromo = true
		}
	}

	availableVouchers, err := listAvailableVouchers(ctx, engine.store, opt)
	if err != nil {
		log.Warn().Err(err).Int64("merchant_id", opt.MerchantID).Int64("user_id", opt.UserID).Msg("failed to list available vouchers for trials")
	}
	res.VoucherTrials = buildVoucherTrials(availableVouchers, opt, res)

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
		if suggested := suggestBestVoucherFromList(availableVouchers, opt); suggested != nil {
			res.SuggestedVoucher = suggested
		}
	}

	// Final total
	res.TotalAmount = res.Subtotal + res.PackagingFee + res.DeliveryFee - res.DeliveryFeeDiscount - res.VoucherDiscount - res.MerchantDiscount
	if res.TotalAmount < 0 {
		res.TotalAmount = 0
	}

	// 4) Balance assessment
	assessment := PaymentAssessment{}
	membership, err := engine.store.GetMembershipByMerchantAndUser(ctx, db.GetMembershipByMerchantAndUserParams{
		MerchantID: opt.MerchantID,
		UserID:     opt.UserID,
	})
	if err == nil {
		settings := loadMembershipSettings(ctx, engine.store, opt.MerchantID)
		applyMembershipSettings(opt, res, &assessment, membership, settings, hasMembershipExclusivePromo)
	}

	curatePaymentBalance(opt, res, &assessment)
	res.PaymentAssessment = assessment
	return res, nil
}

func pickBestRulesByStackingGroup(rules []db.DiscountRule) []db.DiscountRule {
	exclusiveCandidate := map[string]db.DiscountRule{}
	selected := make(map[string]db.DiscountRule)
	for _, rule := range rules {
		group := "default"
		if rule.StackingGroup.Valid && rule.StackingGroup.String != "" {
			group = rule.StackingGroup.String
		}
		if isExclusiveStackingGroup(group) {
			current, exists := exclusiveCandidate[group]
			if !exists || rule.DiscountAmount > current.DiscountAmount {
				exclusiveCandidate[group] = rule
			}
			continue
		}
		current, exists := selected[group]
		if !exists || rule.DiscountAmount > current.DiscountAmount {
			selected[group] = rule
		}
	}
	if len(exclusiveCandidate) > 0 {
		best := db.DiscountRule{}
		hasBest := false
		for _, candidate := range exclusiveCandidate {
			if !hasBest || candidate.DiscountAmount > best.DiscountAmount {
				best = candidate
				hasBest = true
			}
		}
		if hasBest {
			return []db.DiscountRule{best}
		}
	}
	res := make([]db.DiscountRule, 0, len(selected))
	for _, rule := range selected {
		res = append(res, rule)
	}
	return res
}

func pickBestMatchingRulesByStackingGroup(rules []db.DiscountRule, opt OrderContext, now time.Time) []db.DiscountRule {
	matching := make([]db.DiscountRule, 0, len(rules))
	for _, rule := range rules {
		if !isRuleMatch(rule, opt, now) {
			continue
		}
		matching = append(matching, rule)
	}

	return pickBestRulesByStackingGroup(matching)
}

func isExclusiveStackingGroup(group string) bool {
	return group == "exclusive" || group == "mutex"
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
	if v.VoucherTemplateBlockReason != "" {
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

func listAvailableVouchers(ctx context.Context, store db.Store, opt OrderContext) ([]db.ListUserAvailableVouchersForMerchantRow, error) {
	return store.ListUserAvailableVouchersForMerchant(ctx, db.ListUserAvailableVouchersForMerchantParams{
		UserID:         opt.UserID,
		MerchantID:     opt.MerchantID,
		MinOrderAmount: opt.Subtotal,
	})
}

func suggestBestVoucherFromList(vouchers []db.ListUserAvailableVouchersForMerchantRow, opt OrderContext) *SuggestedVoucher {
	if len(vouchers) == 0 {
		return nil
	}
	for _, candidate := range vouchers {
		if len(candidate.AllowedOrderTypes) == 0 || containsString(candidate.AllowedOrderTypes, opt.OrderType) {
			return &SuggestedVoucher{ID: candidate.ID, Name: candidate.Name, Amount: candidate.Amount}
		}
	}
	return nil
}

func buildLadderPromotions(rules []db.DiscountRule, opt OrderContext, now time.Time) []LadderPromotion {
	res := make([]LadderPromotion, 0)
	for _, rule := range rules {
		if now.Before(rule.ValidFrom) || now.After(rule.ValidUntil) {
			continue
		}
		if rule.MinOrderAmount <= 0 {
			continue
		}
		item := LadderPromotion{
			RuleID:    rule.ID,
			Name:      rule.Name,
			Threshold: rule.MinOrderAmount,
			Discount:  rule.DiscountAmount,
		}
		if opt.Subtotal >= rule.MinOrderAmount {
			item.CurrentHit = true
		} else {
			item.MissingNeed = rule.MinOrderAmount - opt.Subtotal
		}
		res = append(res, item)
	}
	return res
}

func buildVoucherTrials(vouchers []db.ListUserAvailableVouchersForMerchantRow, opt OrderContext, calc *PriceCalculationResult) []VoucherTrial {
	if len(vouchers) == 0 {
		return []VoucherTrial{}
	}
	base := calc.Subtotal + calc.PackagingFee + calc.DeliveryFee - calc.DeliveryFeeDiscount - calc.MerchantDiscount
	if base < 0 {
		base = 0
	}
	trials := make([]VoucherTrial, 0, len(vouchers))
	for _, voucher := range vouchers {
		if len(voucher.AllowedOrderTypes) > 0 && !containsString(voucher.AllowedOrderTypes, opt.OrderType) {
			continue
		}
		trialPayable := base - voucher.Amount
		if trialPayable < 0 {
			trialPayable = 0
		}
		trials = append(trials, VoucherTrial{
			VoucherID:    voucher.ID,
			VoucherName:  voucher.Name,
			Amount:       voucher.Amount,
			TrialPayable: trialPayable,
		})
	}
	return trials
}

func containsString(arr []string, target string) bool {
	for _, v := range arr {
		if v == target {
			return true
		}
	}
	return false
}

func loadMembershipSettings(ctx context.Context, store db.Store, merchantID int64) MembershipSettingsResult {
	settings := defaultMembershipSettings(merchantID)

	model, err := store.GetMerchantMembershipSettings(ctx, merchantID)
	if err == nil {
		return settingsResultFromModel(model)
	}
	if !errors.Is(err, db.ErrRecordNotFound) {
		log.Warn().Err(err).Int64("merchant_id", merchantID).Msg("failed to load membership settings")
	}

	return settings
}

func applyMembershipSettings(
	opt OrderContext,
	res *PriceCalculationResult,
	assessment *PaymentAssessment,
	membership db.MerchantMembership,
	settings MembershipSettingsResult,
	hasMembershipExclusivePromo bool,
) {
	principal := membership.PrincipalBalance
	bonus := membership.BonusBalance
	if !IsMembershipBalanceSupportedOrderType(opt.OrderType) {
		principal = 0
		bonus = 0
	} else {
		if !containsString(sanitizeMembershipUsableScenes(settings.BalanceUsableScenes), opt.OrderType) {
			principal = 0
		}
		if !containsString(sanitizeMembershipUsableScenes(settings.BonusUsableScenes), opt.OrderType) {
			bonus = 0
		}
	}

	if res.MerchantDiscount > 0 && hasMembershipExclusivePromo {
		principal = 0
		bonus = 0
		assessment.PaymentHint = "会员余额不可与当前营销优惠叠加"
	}

	if res.VoucherDiscount > 0 && !settings.AllowWithVoucher {
		principal = 0
		bonus = 0
		assessment.PaymentHint = "会员余额不可与优惠券叠加"
	}
	if res.MerchantDiscount > 0 && !settings.AllowWithDiscount {
		principal = 0
		bonus = 0
		if assessment.PaymentHint == "" {
			assessment.PaymentHint = "会员余额不可与满减优惠叠加"
		}
	}

	if settings.MaxDeductionPercent <= 0 {
		principal = 0
		bonus = 0
		if assessment.PaymentHint == "" {
			assessment.PaymentHint = "会员余额暂不可用于抵扣"
		}
	} else if settings.MaxDeductionPercent < 100 {
		cap := res.TotalAmount * int64(settings.MaxDeductionPercent) / 100
		principal, bonus = applyBalanceCap(principal, bonus, cap)
	}

	assessment.PrincipalPart = principal
	assessment.BonusPart = bonus
}

func applyBalanceCap(principal, bonus, cap int64) (int64, int64) {
	if cap <= 0 {
		return 0, 0
	}
	if principal+bonus <= cap {
		return principal, bonus
	}
	if bonus >= cap {
		return 0, cap
	}

	remaining := cap - bonus
	if remaining < 0 {
		remaining = 0
	}
	if principal > remaining {
		principal = remaining
	}
	return principal, bonus
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
				assessment.PaymentHint = "支付提示：您的赠送金额暂不可抵扣代取费"
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
