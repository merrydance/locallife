/**
 * 财务管理类型定义
 * 完全对齐后端 api/merchant_finance.go 中的响应结构
 */

// ==================== 财务概览 ====================

/**
 * 财务概览响应 - 对应 financeOverviewResponse
 */
export interface FinanceOverviewResponse {
  // 订单统计
  completed_orders: number;   // 已完成订单数 (int64)
  pending_orders: number;     // 待结算订单数 (int64)

  // 金额统计（分）
  total_gmv: number;          // 总交易额 (int64)
  total_income: number;       // 商户净收入 (int64)
  total_platform_fee: number; // 平台服务费 (int64)
  total_operator_fee: number; // 运营商服务费 (int64)
  total_service_fee: number;  // 总服务费（平台+运营商）(int64)
  pending_income: number;     // 待结算收入 (int64)

  // 满返支出统计
  promotion_orders: number;    // 满返订单数 (int64)
  total_promotion_exp: number; // 满返支出总额 (int64)

  // 汇总
  net_income: number;          // 净收入 = 商户收入 - 满返支出 (int64)
}

// ==================== 订单收入明细 ====================

/**
 * 财务订单项 - 对应 financeOrderItem
 */
export interface FinanceOrderItem {
  id: number;                    // ID (int64)
  payment_order_id: number;      // 支付订单ID (int64)
  order_id?: number;             // 业务订单ID (int64, 可选)
  order_source: string;          // 订单来源
  total_amount: number;          // 订单金额 (分, int64)
  platform_commission: number;   // 平台佣金 (分, int64)
  operator_commission: number;   // 运营商佣金 (分, int64)
  merchant_amount: number;       // 商户到账 (分, int64)
  status: string;                // 状态
  created_at: string;            // 创建时间 (RFC3339)
  finished_at?: string;          // 完成时间 (RFC3339, 可选)
}

/**
 * 财务订单列表响应
 */
export interface FinanceOrdersResponse {
  orders: FinanceOrderItem[];
  total: number;
  total_count: number;
  page: number;
  page_id: number;
  page_size: number;
  limit: number;
  total_pages: number;
}

// ==================== 服务费明细 ====================

/**
 * 服务费项 - 对应 serviceFeeItem
 */
export interface ServiceFeeItem {
  date: string;           // 日期 YYYY-MM-DD
  order_source: string;   // 订单来源
  order_count: number;    // 订单数 (int64)
  total_amount: number;   // 订单金额 (分, int64)
  platform_fee: number;   // 平台服务费 (分, int64)
  operator_fee: number;   // 运营商服务费 (分, int64)
  total_fee: number;      // 总服务费 (分, int64)
}

/**
 * 服务费明细响应
 */
export interface ServiceFeesResponse {
  details: ServiceFeeItem[];
  total_platform_fee: number;
  total_operator_fee: number;
  total_service_fee: number;
}

// ==================== 满返支出明细 ====================

/**
 * 满返支出项 - 对应 promotionExpenseItem
 */
export interface PromotionExpenseItem {
  id: number;                     // ID (int64)
  order_no: string;               // 订单编号
  order_type: string;             // 订单类型
  subtotal: number;               // 商品小计 (分, int64)
  delivery_fee: number;           // 配送费 (分, int64)
  delivery_fee_discount: number;  // 配送费优惠 (分, int64)
  total_amount: number;           // 订单总额 (分, int64)
  created_at: string;             // 创建时间 (RFC3339)
  completed_at?: string;          // 完成时间 (RFC3339, 可选)
}

/**
 * 满返支出列表响应
 */
export interface PromotionExpensesResponse {
  orders: PromotionExpenseItem[];
  total: number;
  total_count: number;
  page: number;
  page_id: number;
  page_size: number;
  limit: number;
  total_pages: number;
  total_promo_orders: number;
  total_promo_amount: number;
}

// ==================== 每日财务汇总 ====================

/**
 * 每日财务项 - 对应 dailyFinanceItem
 */
export interface DailyFinanceItem {
  date: string;           // 日期 YYYY-MM-DD
  order_count: number;    // 订单数 (int64)
  total_gmv: number;      // 总交易额 (分, int64)
  merchant_income: number;// 商户收入 (分, int64)
  total_fee: number;      // 总服务费 (分, int64)
}

/**
 * 每日财务响应
 */
export interface DailyFinanceResponse {
  daily_stats: DailyFinanceItem[];
}

// ==================== 结算记录 ====================

/**
 * 结算状态
 */
export type SettlementStatus = 'pending' | 'processing' | 'finished' | 'failed';

/**
 * 结算项 - 对应 merchantSettlementItem
 */
export interface SettlementItem {
  id: number;                    // ID (int64)
  payment_order_id: number;      // 支付订单ID (int64)
  order_source: string;          // 订单来源
  total_amount: number;          // 订单金额 (分, int64)
  platform_commission: number;   // 平台佣金 (分, int64)
  operator_commission: number;   // 运营商佣金 (分, int64)
  merchant_amount: number;       // 商户到账 (分, int64)
  out_order_no: string;          // 外部订单号
  sharing_order_id?: string;     // 分账订单号 (可选)
  status: string;                // 状态
  created_at: string;            // 创建时间 (RFC3339)
  finished_at?: string;          // 完成时间 (RFC3339, 可选)
}

/**
 * 结算列表响应
 */
export interface SettlementsResponse {
  settlements: SettlementItem[];
  total: number;
  total_count: number;
  page: number;
  page_id: number;
  page_size: number;
  limit: number;
  total_pages: number;
  total_amount: number;
  total_merchant_amount: number;
  total_platform_fee: number;
  total_operator_fee: number;
}

// ==================== 工具常量 ====================

/**
 * 结算状态标签
 */
export const SETTLEMENT_STATUS_LABELS: Record<string, string> = {
  pending: "待处理",
  processing: "处理中",
  finished: "已完成",
  failed: "失败",
};

/**
 * 结算状态颜色
 */
export const SETTLEMENT_STATUS_COLORS: Record<string, string> = {
  pending: "bg-amber-100 text-amber-700",
  processing: "bg-blue-100 text-blue-700",
  finished: "bg-emerald-100 text-emerald-700",
  failed: "bg-rose-100 text-rose-700",
};

/**
 * 订单来源标签
 */
export const ORDER_SOURCE_LABELS: Record<string, string> = {
  takeout: "外卖",
  dine_in: "堂食",
  pickup: "自取",
  reservation: "预订",
};
