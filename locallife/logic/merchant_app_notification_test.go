package logic

import (
	"testing"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestBuildMerchantNewOrderNotification(t *testing.T) {
	order := db.Order{ID: 88, OrderNo: "ORD202604280001", TotalAmount: 3500}

	payload := BuildMerchantNewOrderNotification(order, "春风小馆")
	again := BuildMerchantNewOrderNotification(order, "春风小馆")

	require.Equal(t, "merchant:new_order:88", payload.MessageID)
	require.Equal(t, payload.MessageID, again.MessageID)
	require.Equal(t, MerchantNotificationEventNewOrder, payload.Event)
	require.Equal(t, int64(88), payload.OrderID)
	require.Equal(t, "ORD202604280001", payload.OrderNo)
	require.Equal(t, "新订单", payload.Title)
	require.Contains(t, payload.Content, "ORD202604280001")
	require.Equal(t, int64(3500), payload.Amount)
	require.Equal(t, "春风小馆", payload.ShopName)
}

func TestBuildMerchantNewOrderNotification_DefaultShopName(t *testing.T) {
	payload := BuildMerchantNewOrderNotification(db.Order{ID: 9, OrderNo: "ORD9"}, " ")

	require.Equal(t, "商户", payload.ShopName)
}
