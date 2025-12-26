package algorithm

import (
	"errors"
	"math"
)

// ProfitSharingConfig 分账配置
type ProfitSharingConfig struct {
	PlatformRate int  // 平台分账比例（百分比），默认2%
	OperatorRate int  // 运营商分账比例（百分比），默认3%
	EnableRider  bool // 是否启用骑手分账（外卖订单）
}

// DefaultProfitSharingConfig 默认分账配置
var DefaultProfitSharingConfig = ProfitSharingConfig{
	PlatformRate: 2,
	OperatorRate: 3,
	EnableRider:  true,
}

// DineInProfitSharingConfig 堂食/自提分账配置（无运营商分成）
var DineInProfitSharingConfig = ProfitSharingConfig{
	PlatformRate: 0,
	OperatorRate: 0,
	EnableRider:  false,
}

// ProfitSharingInput 分账计算输入
type ProfitSharingInput struct {
	TotalAmount int64  // 用户实际支付金额（分）
	DeliveryFee int64  // 配送费（分）
	OrderSource string // 订单来源：takeout（外卖）、dine_in（堂食）、takeaway（打包自提）
}

// ProfitSharingResult 分账计算结果
type ProfitSharingResult struct {
	// 输入值
	TotalAmount int64 // 用户支付总额
	DeliveryFee int64 // 配送费

	// 分账比例
	PlatformRate int // 平台比例
	OperatorRate int // 运营商比例

	// 计算中间值
	DistributableAmount int64 // 可分账金额 = TotalAmount - DeliveryFee

	// 分账结果
	RiderAmount    int64 // 骑手收入 = 配送费
	PlatformAmount int64 // 平台收入 = DistributableAmount * PlatformRate%
	OperatorAmount int64 // 运营商收入 = DistributableAmount * OperatorRate%
	MerchantAmount int64 // 商户收入 = DistributableAmount - PlatformAmount - OperatorAmount

	// 验证
	IsValid bool   // 分账结果是否有效
	Error   string // 错误信息
}

// ProfitSharingCalculator 分账计算器
type ProfitSharingCalculator struct {
	config ProfitSharingConfig
}

// NewProfitSharingCalculator 创建分账计算器
func NewProfitSharingCalculator(config ProfitSharingConfig) *ProfitSharingCalculator {
	return &ProfitSharingCalculator{config: config}
}

// NewDefaultCalculator 创建默认配置的分账计算器
func NewDefaultCalculator() *ProfitSharingCalculator {
	return NewProfitSharingCalculator(DefaultProfitSharingConfig)
}

// Calculate 计算分账
func (c *ProfitSharingCalculator) Calculate(input ProfitSharingInput) ProfitSharingResult {
	result := ProfitSharingResult{
		TotalAmount:  input.TotalAmount,
		DeliveryFee:  input.DeliveryFee,
		PlatformRate: c.config.PlatformRate,
		OperatorRate: c.config.OperatorRate,
	}

	// 输入校验
	if input.TotalAmount <= 0 {
		result.Error = "支付金额必须大于0"
		return result
	}

	if input.DeliveryFee < 0 {
		result.Error = "配送费不能为负数"
		return result
	}

	if input.DeliveryFee > input.TotalAmount {
		result.Error = "配送费不能大于支付总额"
		return result
	}

	// 根据订单来源调整配置
	config := c.getConfigByOrderSource(input.OrderSource)
	result.PlatformRate = config.PlatformRate
	result.OperatorRate = config.OperatorRate

	// 1. 骑手先拿配送费（仅外卖订单）
	if config.EnableRider && input.DeliveryFee > 0 {
		result.RiderAmount = input.DeliveryFee
	}

	// 2. 计算可分账金额（扣除配送费后）
	result.DistributableAmount = input.TotalAmount - input.DeliveryFee

	// 3. 计算平台分成
	result.PlatformAmount = c.calculateShare(result.DistributableAmount, config.PlatformRate)

	// 4. 计算运营商分成
	result.OperatorAmount = c.calculateShare(result.DistributableAmount, config.OperatorRate)

	// 5. 商户收入 = 可分账金额 - 平台分成 - 运营商分成
	result.MerchantAmount = result.DistributableAmount - result.PlatformAmount - result.OperatorAmount

	// 验证分账结果
	result.IsValid = c.validate(&result)

	return result
}

// getConfigByOrderSource 根据订单来源获取分账配置
func (c *ProfitSharingCalculator) getConfigByOrderSource(orderSource string) ProfitSharingConfig {
	switch orderSource {
	case "takeout": // 外卖
		return ProfitSharingConfig{
			PlatformRate: c.config.PlatformRate,
			OperatorRate: c.config.OperatorRate,
			EnableRider:  true,
		}
	case "dine_in", "takeaway": // 堂食、打包自提
		return ProfitSharingConfig{
			PlatformRate: 0,
			OperatorRate: 0,
			EnableRider:  false,
		}
	case "reservation": // 预定（按外卖规则）
		return ProfitSharingConfig{
			PlatformRate: c.config.PlatformRate,
			OperatorRate: c.config.OperatorRate,
			EnableRider:  false, // 预定不涉及配送
		}
	default:
		return c.config
	}
}

// calculateShare 计算分成金额（向下取整，确保商户利益）
func (c *ProfitSharingCalculator) calculateShare(amount int64, rate int) int64 {
	if rate <= 0 || amount <= 0 {
		return 0
	}
	return int64(math.Floor(float64(amount) * float64(rate) / 100))
}

// validate 验证分账结果
func (c *ProfitSharingCalculator) validate(result *ProfitSharingResult) bool {
	// 分账金额之和必须等于支付总额
	sum := result.RiderAmount + result.PlatformAmount + result.OperatorAmount + result.MerchantAmount
	if sum != result.TotalAmount {
		result.Error = "分账金额之和不等于支付总额"
		return false
	}

	// 所有金额必须非负
	if result.RiderAmount < 0 || result.PlatformAmount < 0 ||
		result.OperatorAmount < 0 || result.MerchantAmount < 0 {
		result.Error = "分账金额不能为负数"
		return false
	}

	return true
}

// ==================== 批量分账计算（用于合单支付）====================

// CombinedProfitSharingInput 合单分账输入
type CombinedProfitSharingInput struct {
	SubOrders []SubOrderInput // 子订单列表
}

// SubOrderInput 子订单分账输入
type SubOrderInput struct {
	MerchantID  int64  // 商户ID
	OrderID     int64  // 订单ID
	Amount      int64  // 子单金额（用户支付）
	DeliveryFee int64  // 配送费
	OrderSource string // 订单来源
	RiderID     *int64 // 骑手ID（可选）
}

// CombinedProfitSharingResult 合单分账结果
type CombinedProfitSharingResult struct {
	TotalAmount   int64            // 合计支付金额
	SubResults    []SubOrderResult // 各子单分账结果
	TotalRider    int64            // 合计骑手收入
	TotalPlatform int64            // 合计平台收入
	TotalOperator int64            // 合计运营商收入
	TotalMerchant int64            // 合计商户收入
	IsValid       bool             // 是否全部有效
	Errors        []string         // 错误列表
}

// SubOrderResult 子订单分账结果
type SubOrderResult struct {
	MerchantID int64
	OrderID    int64
	RiderID    *int64
	ProfitSharingResult
}

// CalculateCombined 计算合单分账
func (c *ProfitSharingCalculator) CalculateCombined(input CombinedProfitSharingInput) CombinedProfitSharingResult {
	result := CombinedProfitSharingResult{
		SubResults: make([]SubOrderResult, 0, len(input.SubOrders)),
		IsValid:    true,
	}

	for _, subOrder := range input.SubOrders {
		// 计算单个子订单的分账
		subInput := ProfitSharingInput{
			TotalAmount: subOrder.Amount,
			DeliveryFee: subOrder.DeliveryFee,
			OrderSource: subOrder.OrderSource,
		}

		subResult := c.Calculate(subInput)

		subOrderResult := SubOrderResult{
			MerchantID:          subOrder.MerchantID,
			OrderID:             subOrder.OrderID,
			RiderID:             subOrder.RiderID,
			ProfitSharingResult: subResult,
		}

		result.SubResults = append(result.SubResults, subOrderResult)

		// 累计金额
		result.TotalAmount += subOrder.Amount
		result.TotalRider += subResult.RiderAmount
		result.TotalPlatform += subResult.PlatformAmount
		result.TotalOperator += subResult.OperatorAmount
		result.TotalMerchant += subResult.MerchantAmount

		// 检查是否有错误
		if !subResult.IsValid {
			result.IsValid = false
			result.Errors = append(result.Errors, subResult.Error)
		}
	}

	return result
}

// ==================== 工具函数 ====================

// CalculateProfitSharing 快捷计算函数
func CalculateProfitSharing(totalAmount, deliveryFee int64, orderSource string) (ProfitSharingResult, error) {
	calculator := NewDefaultCalculator()
	result := calculator.Calculate(ProfitSharingInput{
		TotalAmount: totalAmount,
		DeliveryFee: deliveryFee,
		OrderSource: orderSource,
	})

	if !result.IsValid {
		return result, errors.New(result.Error)
	}

	return result, nil
}

// CalculateTakeoutProfitSharing 计算外卖订单分账
func CalculateTakeoutProfitSharing(totalAmount, deliveryFee int64) (ProfitSharingResult, error) {
	return CalculateProfitSharing(totalAmount, deliveryFee, "takeout")
}

// CalculateDineInProfitSharing 计算堂食订单分账（无服务费）
func CalculateDineInProfitSharing(totalAmount int64) (ProfitSharingResult, error) {
	return CalculateProfitSharing(totalAmount, 0, "dine_in")
}
