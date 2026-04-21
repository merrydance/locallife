package api

const (
	// WebSocket Event Types (WebSocket 事件类型)
	// 用于实时推送消息给商户端
	EventReservationUpdate     = "reservation_update"       // 预订状态变更
	EventTableStatusChange     = "table_status_change"      // 桌台状态变更
	EventTableTransfer         = "table_transfer"           // 换桌/转台
	EventMerchantUserRiskAlert = "merchant_user_risk_alert" // 高风险用户到店提醒
	EventSessionClosed         = "session_closed"           // 就餐会话结束

	// Reservation Statuses (预约状态)
	// 这些值与数据库 CHECK 约束保持一致
	ReservationStatusPending   = "pending"
	ReservationStatusPaid      = "paid"
	ReservationStatusConfirmed = "confirmed"
	ReservationStatusCheckedIn = "checked_in"
	ReservationStatusCompleted = "completed"
	ReservationStatusCancelled = "cancelled"
	ReservationStatusExpired   = "expired"
	ReservationStatusNoShow    = "no_show"

	// Table Statuses (餐桌状态)
	TableStatusAvailable = "available"
	TableStatusOccupied  = "occupied"
	TableStatusReserved  = "reserved"
	TableStatusDisabled  = "disabled"

	// Dining Session Statuses (就餐会话状态)
	DiningSessionStatusOpen   = "open"
	DiningSessionStatusClosed = "closed"

	// Stats Constants (统计相关常量)
	// 用于查询全部历史数据的日期范围边界
	StatsStartYear = 2020
	StatsEndYear   = 2099

	// Operator Revenue Sharing Constants (运营商分成相关常量)
	// 运营商从平台佣金中可分得的比例
	// 业务规则：平台抽佣 5%，运营商获取其中 3 个点，即 3/5 = 60%
	OperatorRevenueShareRatio = 0.6

	// Rate Limit Constants (速率限制相关常量)
	// P1-019 修复：催单频率限制
	UrgeOrderRateLimitWindowSeconds = 300 // 5分钟窗口
	UrgeOrderRateLimitMaxCount      = 3   // 窗口内最多3次

	// Geofencing Constants (地理围栏相关常量)
	// P1-003 修复：骑手抢单最大距离（米）
	MaxGrabOrderDistanceMeters = 5000 // 5公里
	// P1-005 修复：配送确认半径（米）
	DeliveryConfirmRadiusMeters = 500 // 500米
	// P1-005 修复：配送确认定位最大时效（秒）
	DeliveryConfirmLocationMaxAgeSec = 120 // 2分钟

	// Recovery dispute constants (追偿争议相关常量)
	// P1-010 修复：追偿争议有效期窗口
	RecoveryDisputeWindowDays = 7 // 索赔后7天内可发起追偿争议

	// Reservation Constants (预订相关常量)
	// P1-023 修复：签到时间窗口
	ReservationCheckInEarlyMinutes = 30 // 可提前签到时间（分钟）
	ReservationCheckInLateMinutes  = 30 // 迟到容忍时间（分钟）

	// Cart Constants (购物车相关常量)
	// P1-016 修复：单品最大数量
	CartItemMaxQuantity = 99
)
