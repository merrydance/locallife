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
)
