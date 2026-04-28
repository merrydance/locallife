package logic

import (
	"fmt"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
)

const MerchantNotificationEventNewOrder = "new_order"

const MerchantAppNotificationEventNewOrder = MerchantNotificationEventNewOrder

type MerchantNewOrderNotificationPayload struct {
	MessageID string `json:"message_id"`
	Event     string `json:"event"`
	OrderID   int64  `json:"order_id"`
	OrderNo   string `json:"order_no"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Amount    int64  `json:"amount"`
	ShopName  string `json:"shop_name"`
}

type MerchantAppNotificationPayload = MerchantNewOrderNotificationPayload

func BuildMerchantNewOrderNotification(order db.Order, shopName string) MerchantNewOrderNotificationPayload {
	shopName = strings.TrimSpace(shopName)
	if shopName == "" {
		shopName = "商户"
	}

	return MerchantNewOrderNotificationPayload{
		MessageID: fmt.Sprintf("merchant:new_order:%d", order.ID),
		Event:     MerchantNotificationEventNewOrder,
		OrderID:   order.ID,
		OrderNo:   order.OrderNo,
		Title:     "新订单",
		Content:   fmt.Sprintf("您有一笔新订单 %s，请及时处理", order.OrderNo),
		Amount:    order.TotalAmount,
		ShopName:  shopName,
	}
}

func BuildMerchantAppNewOrderNotification(order db.Order, shopName string) MerchantAppNotificationPayload {
	return BuildMerchantNewOrderNotification(order, shopName)
}
