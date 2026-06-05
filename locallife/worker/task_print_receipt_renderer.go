package worker

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

type printSettlementBill struct {
	breakdown          logic.MerchantOrderFeeBreakdown
	profitSharingOrder db.ProfitSharingOrder
}

func buildReceiptForProvider(
	providerType string,
	order db.GetOrderWithDetailsRow,
	items []db.ListOrderItemsWithDishByOrderRow,
	user db.User,
	slip string,
	settlementBill *printSettlementBill,
) string {
	switch providerType {
	case printerTypeShangpeng:
		return buildShangpengReceipt(order, items, user, slip, settlementBill)
	default:
		return buildFeieReceipt(order, items, user, slip, settlementBill)
	}
}

func buildFeieReceipt(order db.GetOrderWithDetailsRow, items []db.ListOrderItemsWithDishByOrderRow, user db.User, slip string, settlementBill *printSettlementBill) string {
	var builder strings.Builder
	title := "乐客来福"
	if order.PickupCode.Valid && order.PickupCode.String != "" {
		title = order.PickupCode.String + "# 乐客来福"
	}

	builder.WriteString("<CB><B>" + title + "</B></CB><BR>")
	if slip == printSlipKitchen {
		builder.WriteString("<C>后厨单</C><BR>")
	} else {
		builder.WriteString("<C>前台出单</C><BR>")
	}
	builder.WriteString("订单号：" + order.OrderNo + "<BR>")
	builder.WriteString("下单时间：" + order.CreatedAt.Format("2006-01-02 15:04:05") + "<BR>")
	builder.WriteString("类型：" + orderTypeLabel(order.OrderType) + "<BR>")
	builder.WriteString("--------------------------------<BR>")

	for _, item := range items {
		builder.WriteString(formatPrintItemLine(item.Name, item.Quantity, item.Subtotal))
		builder.WriteString("<BR>")
	}

	builder.WriteString("--------------------------------<BR>")
	builder.WriteString("菜品小计：" + fenToYuan(order.Subtotal) + "<BR>")
	if order.DiscountAmount > 0 {
		builder.WriteString("优惠：-" + fenToYuan(order.DiscountAmount) + "<BR>")
	}
	if order.VoucherAmount > 0 {
		builder.WriteString("券抵扣：-" + fenToYuan(order.VoucherAmount) + "<BR>")
	}
	if slip == printSlipFull && settlementBill != nil {
		writeFeieSettlementBill(&builder, settlementBill)
	}

	if order.Notes.Valid && order.Notes.String != "" {
		builder.WriteString("备注：" + order.Notes.String + "<BR>")
	}

	if slip == printSlipFull {
		if customerName := resolvePrintCustomerName(order, user); customerName != "" {
			builder.WriteString("顾客：" + customerName + "<BR>")
		}
		if order.OrderType == db.OrderTypeTakeout && strings.TrimSpace(order.DeliveryAddress) != "" {
			builder.WriteString("地址：" + order.DeliveryAddress + "<BR>")
		}
	}

	builder.WriteString(ticketCodeBlock(order.OrderNo))
	builder.WriteString("<CUT>")
	return builder.String()
}

func writeFeieSettlementBill(builder *strings.Builder, settlementBill *printSettlementBill) {
	breakdown := settlementBill.breakdown
	profitSharingOrder := settlementBill.profitSharingOrder

	builder.WriteString("--------------------------------<BR>")
	builder.WriteString("用户实付：" + fenToYuan(breakdown.CustomerPayableAmount) + "<BR>")
	builder.WriteString("商户账单<BR>")
	builder.WriteString("菜品合计：" + fenToYuan(breakdown.FoodPayableAmount) + "<BR>")
	builder.WriteString(formatPrintDeductionLine("平台服务费", breakdown.PlatformServiceFeeAmount))
	builder.WriteString(formatPrintDeductionLine("支付通道费", breakdown.PaymentChannelFeeAmount))
	builder.WriteString("<BOLD>商户实收：" + fenToYuan(breakdown.MerchantReceivableAmount) + "</BOLD><BR>")

	if profitSharingOrder.RiderGrossAmount > 0 || profitSharingOrder.RiderPaymentFee > 0 || profitSharingOrder.RiderAmount > 0 {
		builder.WriteString("骑手账单<BR>")
		builder.WriteString("代取费：" + fenToYuan(profitSharingOrder.RiderGrossAmount) + "<BR>")
		builder.WriteString(formatPrintDeductionLine("支付通道费", profitSharingOrder.RiderPaymentFee))
		builder.WriteString("骑手实收：" + fenToYuan(profitSharingOrder.RiderAmount) + "<BR>")
	}
}

func buildShangpengReceipt(order db.GetOrderWithDetailsRow, items []db.ListOrderItemsWithDishByOrderRow, user db.User, slip string, settlementBill *printSettlementBill) string {
	var builder strings.Builder
	title := "乐客来福"
	if order.PickupCode.Valid && order.PickupCode.String != "" {
		title = order.PickupCode.String + "# 乐客来福"
	}

	writeReceiptLine(&builder, title)
	if slip == printSlipKitchen {
		writeReceiptLine(&builder, "后厨单")
	} else {
		writeReceiptLine(&builder, "前台出单")
	}
	writeReceiptLine(&builder, "订单号："+order.OrderNo)
	writeReceiptLine(&builder, "下单时间："+order.CreatedAt.Format("2006-01-02 15:04:05"))
	writeReceiptLine(&builder, "类型："+orderTypeLabel(order.OrderType))
	writeReceiptLine(&builder, "--------------------------------")

	for _, item := range items {
		writeReceiptLine(&builder, formatPrintItemLine(item.Name, item.Quantity, item.Subtotal))
	}

	writeReceiptLine(&builder, "--------------------------------")
	writeReceiptLine(&builder, "菜品小计："+fenToYuan(order.Subtotal))
	if order.DiscountAmount > 0 {
		writeReceiptLine(&builder, "优惠：-"+fenToYuan(order.DiscountAmount))
	}
	if order.VoucherAmount > 0 {
		writeReceiptLine(&builder, "券抵扣：-"+fenToYuan(order.VoucherAmount))
	}
	if slip == printSlipFull && settlementBill != nil {
		writePlainSettlementBill(&builder, settlementBill)
	}

	if order.Notes.Valid && order.Notes.String != "" {
		writeReceiptLine(&builder, "备注："+order.Notes.String)
	}

	if slip == printSlipFull {
		if customerName := resolvePrintCustomerName(order, user); customerName != "" {
			writeReceiptLine(&builder, "顾客："+customerName)
		}
		if order.OrderType == db.OrderTypeTakeout && strings.TrimSpace(order.DeliveryAddress) != "" {
			writeReceiptLine(&builder, "地址："+order.DeliveryAddress)
		}
	}

	writeReceiptLine(&builder, "")
	writeReceiptLine(&builder, "取餐码："+order.OrderNo)
	return builder.String()
}

func writePlainSettlementBill(builder *strings.Builder, settlementBill *printSettlementBill) {
	breakdown := settlementBill.breakdown
	profitSharingOrder := settlementBill.profitSharingOrder

	writeReceiptLine(builder, "--------------------------------")
	writeReceiptLine(builder, "用户实付："+fenToYuan(breakdown.CustomerPayableAmount))
	writeReceiptLine(builder, "商户账单")
	writeReceiptLine(builder, "菜品合计："+fenToYuan(breakdown.FoodPayableAmount))
	writeReceiptLine(builder, "- 平台服务费：-"+fenToYuan(breakdown.PlatformServiceFeeAmount))
	writeReceiptLine(builder, "- 支付通道费：-"+fenToYuan(breakdown.PaymentChannelFeeAmount))
	writeReceiptLine(builder, "商户实收："+fenToYuan(breakdown.MerchantReceivableAmount))

	if profitSharingOrder.RiderGrossAmount > 0 || profitSharingOrder.RiderPaymentFee > 0 || profitSharingOrder.RiderAmount > 0 {
		writeReceiptLine(builder, "骑手账单")
		writeReceiptLine(builder, "代取费："+fenToYuan(profitSharingOrder.RiderGrossAmount))
		writeReceiptLine(builder, "- 支付通道费：-"+fenToYuan(profitSharingOrder.RiderPaymentFee))
		writeReceiptLine(builder, "骑手实收："+fenToYuan(profitSharingOrder.RiderAmount))
	}
}

func writeReceiptLine(builder *strings.Builder, value string) {
	builder.WriteString(value)
	builder.WriteString("\n")
}

func resolvePrintCustomerName(order db.GetOrderWithDetailsRow, user db.User) string {
	if name := strings.TrimSpace(order.DeliveryContactName); name != "" {
		return name
	}
	return strings.TrimSpace(user.FullName)
}

func formatPrintItemLine(name string, quantity int16, subtotal int64) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		trimmed = "未命名商品"
	}
	return fmt.Sprintf("%s x%d  %s", trimmed, quantity, fenToYuan(subtotal))
}

func formatPrintDeductionLine(label string, amount int64) string {
	return "- " + label + "：-" + fenToYuan(amount) + "<BR>"
}

func fenToYuan(amount int64) string {
	return fmt.Sprintf("%.2f", float64(amount)/100)
}

func orderTypeLabel(orderType string) string {
	switch orderType {
	case db.OrderTypeTakeout:
		return "外卖"
	case "takeaway":
		return "自取"
	case "dine_in":
		return "堂食"
	case db.OrderTypeReservation:
		return "预订"
	default:
		return orderType
	}
}

func ticketCodeBlock(orderNo string) string {
	upper := strings.ToUpper(orderNo)
	if canUseFeieBarcode(upper) {
		return "<BR><BC128_A>" + upper + "</BC128_A><BR>"
	}
	return "<BR><QR>" + orderNo + "</QR><BR>"
}

func canUseFeieBarcode(value string) bool {
	if len(value) == 0 || len(value) > 14 {
		return false
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'A' || ch > 'Z') {
			return false
		}
	}
	return true
}

func newPrintProviderOriginID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("po%030x", time.Now().UnixNano())
}
