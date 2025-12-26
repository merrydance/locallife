package algorithm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProfitSharingCalculator_Takeout(t *testing.T) {
	// 外卖订单分账测试
	// 用户支付：¥88（8800分）
	// 配送费：¥8（800分）
	// 可分账金额：¥80（8000分）
	// 平台(2%)：¥1.60（160分）
	// 运营商(3%)：¥2.40（240分）
	// 商户(95%)：¥75.60（7600分）
	// 骑手：¥8（800分）

	calculator := NewDefaultCalculator()
	result := calculator.Calculate(ProfitSharingInput{
		TotalAmount: 8800,
		DeliveryFee: 800,
		OrderSource: "takeout",
	})

	require.True(t, result.IsValid)
	require.Empty(t, result.Error)

	require.Equal(t, int64(8800), result.TotalAmount)
	require.Equal(t, int64(800), result.DeliveryFee)
	require.Equal(t, int64(8000), result.DistributableAmount)

	require.Equal(t, int64(800), result.RiderAmount)     // 骑手拿配送费
	require.Equal(t, int64(160), result.PlatformAmount)  // 8000 * 2% = 160
	require.Equal(t, int64(240), result.OperatorAmount)  // 8000 * 3% = 240
	require.Equal(t, int64(7600), result.MerchantAmount) // 8000 - 160 - 240 = 7600

	// 验证分账总和等于支付总额
	sum := result.RiderAmount + result.PlatformAmount + result.OperatorAmount + result.MerchantAmount
	require.Equal(t, result.TotalAmount, sum)
}

func TestProfitSharingCalculator_DineIn(t *testing.T) {
	// 堂食订单分账测试
	// 用户支付：¥100（10000分）
	// 无配送费
	// 平台(0%)：¥0
	// 运营商(0%)：¥0
	// 商户(100%)：¥100（10000分）

	calculator := NewDefaultCalculator()
	result := calculator.Calculate(ProfitSharingInput{
		TotalAmount: 10000,
		DeliveryFee: 0,
		OrderSource: "dine_in",
	})

	require.True(t, result.IsValid)
	require.Empty(t, result.Error)

	require.Equal(t, int64(10000), result.TotalAmount)
	require.Equal(t, int64(0), result.DeliveryFee)
	require.Equal(t, int64(10000), result.DistributableAmount)

	require.Equal(t, int64(0), result.RiderAmount)
	require.Equal(t, int64(0), result.PlatformAmount)
	require.Equal(t, int64(0), result.OperatorAmount)
	require.Equal(t, int64(10000), result.MerchantAmount) // 商户拿全部

	// 验证分账总和
	sum := result.RiderAmount + result.PlatformAmount + result.OperatorAmount + result.MerchantAmount
	require.Equal(t, result.TotalAmount, sum)
}

func TestProfitSharingCalculator_Takeaway(t *testing.T) {
	// 打包自提订单测试（与堂食相同，无服务费）
	calculator := NewDefaultCalculator()
	result := calculator.Calculate(ProfitSharingInput{
		TotalAmount: 5000,
		DeliveryFee: 0,
		OrderSource: "takeaway",
	})

	require.True(t, result.IsValid)
	require.Equal(t, int64(5000), result.MerchantAmount)
	require.Equal(t, int64(0), result.PlatformAmount)
	require.Equal(t, int64(0), result.OperatorAmount)
}

func TestProfitSharingCalculator_Reservation(t *testing.T) {
	// 预定订单测试（按外卖规则但无配送）
	calculator := NewDefaultCalculator()
	result := calculator.Calculate(ProfitSharingInput{
		TotalAmount: 20000,
		DeliveryFee: 0,
		OrderSource: "reservation",
	})

	require.True(t, result.IsValid)
	require.Equal(t, int64(20000), result.DistributableAmount)

	require.Equal(t, int64(0), result.RiderAmount)        // 无骑手
	require.Equal(t, int64(400), result.PlatformAmount)   // 20000 * 2%
	require.Equal(t, int64(600), result.OperatorAmount)   // 20000 * 3%
	require.Equal(t, int64(19000), result.MerchantAmount) // 20000 - 400 - 600
}

func TestProfitSharingCalculator_InvalidInput(t *testing.T) {
	calculator := NewDefaultCalculator()

	// 测试负金额
	result := calculator.Calculate(ProfitSharingInput{
		TotalAmount: -100,
		DeliveryFee: 0,
		OrderSource: "takeout",
	})
	require.False(t, result.IsValid)
	require.Contains(t, result.Error, "大于0")

	// 测试配送费大于总额
	result = calculator.Calculate(ProfitSharingInput{
		TotalAmount: 1000,
		DeliveryFee: 2000,
		OrderSource: "takeout",
	})
	require.False(t, result.IsValid)
	require.Contains(t, result.Error, "配送费不能大于")

	// 测试负配送费
	result = calculator.Calculate(ProfitSharingInput{
		TotalAmount: 1000,
		DeliveryFee: -100,
		OrderSource: "takeout",
	})
	require.False(t, result.IsValid)
	require.Contains(t, result.Error, "不能为负")
}

func TestProfitSharingCalculator_Combined(t *testing.T) {
	// 合单分账测试：用户同时从3个商户购买
	calculator := NewDefaultCalculator()

	input := CombinedProfitSharingInput{
		SubOrders: []SubOrderInput{
			{
				MerchantID:  1,
				OrderID:     101,
				Amount:      1500, // ¥15 奶茶
				DeliveryFee: 300,  // ¥3 配送费
				OrderSource: "takeout",
			},
			{
				MerchantID:  2,
				OrderID:     102,
				Amount:      2500, // ¥25 炸鸡
				DeliveryFee: 300,  // ¥3 配送费
				OrderSource: "takeout",
			},
			{
				MerchantID:  3,
				OrderID:     103,
				Amount:      1800, // ¥18 甜品
				DeliveryFee: 200,  // ¥2 配送费
				OrderSource: "takeout",
			},
		},
	}

	result := calculator.CalculateCombined(input)

	require.True(t, result.IsValid)
	require.Len(t, result.SubResults, 3)

	// 验证合计
	require.Equal(t, int64(5800), result.TotalAmount) // 1500+2500+1800
	require.Equal(t, int64(800), result.TotalRider)   // 300+300+200 配送费总和

	// 验证各商户分账
	// 商户1：可分账1200，平台24，运营商36，商户1140
	require.Equal(t, int64(1140), result.SubResults[0].MerchantAmount)

	// 商户2：可分账2200，平台44，运营商66，商户2090
	require.Equal(t, int64(2090), result.SubResults[1].MerchantAmount)

	// 商户3：可分账1600，平台32，运营商48，商户1520
	require.Equal(t, int64(1520), result.SubResults[2].MerchantAmount)

	// 验证合计金额
	totalSum := result.TotalRider + result.TotalPlatform + result.TotalOperator + result.TotalMerchant
	require.Equal(t, result.TotalAmount, totalSum)
}

func TestCalculateTakeoutProfitSharing(t *testing.T) {
	result, err := CalculateTakeoutProfitSharing(10000, 500)
	require.NoError(t, err)
	require.True(t, result.IsValid)

	require.Equal(t, int64(500), result.RiderAmount)
	require.Equal(t, int64(9500), result.DistributableAmount)
	require.Equal(t, int64(190), result.PlatformAmount)  // 9500 * 2%
	require.Equal(t, int64(285), result.OperatorAmount)  // 9500 * 3%
	require.Equal(t, int64(9025), result.MerchantAmount) // 9500 - 190 - 285
}

func TestCalculateDineInProfitSharing(t *testing.T) {
	result, err := CalculateDineInProfitSharing(10000)
	require.NoError(t, err)
	require.True(t, result.IsValid)

	require.Equal(t, int64(0), result.RiderAmount)
	require.Equal(t, int64(0), result.PlatformAmount)
	require.Equal(t, int64(0), result.OperatorAmount)
	require.Equal(t, int64(10000), result.MerchantAmount)
}

func TestProfitSharingCalculator_SmallAmount(t *testing.T) {
	// 测试小金额分账（确保向下取整保护商户利益）
	calculator := NewDefaultCalculator()
	result := calculator.Calculate(ProfitSharingInput{
		TotalAmount: 100, // ¥1
		DeliveryFee: 0,
		OrderSource: "takeout",
	})

	require.True(t, result.IsValid)
	require.Equal(t, int64(2), result.PlatformAmount)  // 100 * 2% = 2
	require.Equal(t, int64(3), result.OperatorAmount)  // 100 * 3% = 3
	require.Equal(t, int64(95), result.MerchantAmount) // 100 - 2 - 3 = 95

	// 验证总和
	sum := result.RiderAmount + result.PlatformAmount + result.OperatorAmount + result.MerchantAmount
	require.Equal(t, result.TotalAmount, sum)
}

func TestProfitSharingCalculator_OnlyDeliveryFee(t *testing.T) {
	// 极端情况：只有配送费（商品免费）
	calculator := NewDefaultCalculator()
	result := calculator.Calculate(ProfitSharingInput{
		TotalAmount: 500, // 全部是配送费
		DeliveryFee: 500,
		OrderSource: "takeout",
	})

	require.True(t, result.IsValid)
	require.Equal(t, int64(500), result.RiderAmount) // 骑手拿全部
	require.Equal(t, int64(0), result.DistributableAmount)
	require.Equal(t, int64(0), result.PlatformAmount)
	require.Equal(t, int64(0), result.OperatorAmount)
	require.Equal(t, int64(0), result.MerchantAmount)
}
