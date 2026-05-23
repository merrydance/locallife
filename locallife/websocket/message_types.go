package websocket

// 消息类型常量
const (
	MessageTypeNotification         = "notification"
	MessageTypePing                 = "ping"
	MessageTypePong                 = "pong"
	MessageTypeAlert                = "alert"
	MessageTypeMerchantStatusChange = "merchant_status_change"

	// 配送相关消息类型
	MessageTypeDeliveryPoolNew    = "delivery_pool_new"    // 配送池新增订单
	MessageTypeDeliveryPoolGone   = "delivery_pool_gone"   // 配送池订单被抢/移除
	MessageTypeDeliveryStatusSync = "delivery_status_sync" // 配送状态同步
)

// 通知目标类型
const (
	EntityRider    = "rider"
	EntityMerchant = "merchant"
	EntityPlatform = "platform"
)
