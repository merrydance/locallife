/**
 * 商户统计类型定义
 * 完全对齐后端 api/merchant_stats.go 中的响应结构
 */

// ==================== 概览统计 ====================

/**
 * 商户概览响应 - 对应 merchantOverviewResponse
 */
export interface OverviewResponse {
  total_days: number;       // 统计天数
  total_orders: number;     // 总订单数 (int32)
  total_sales: number;      // 总销售额 (分, int64)
  total_commission: number; // 总佣金 (分, int64)
  avg_daily_sales: number;  // 日均销售额 (分, int64)
}

// ==================== 日报统计 ====================

/**
 * 日报统计行 - 对应 dailyStatRow
 */
export interface DailyStatRow {
  date: string;           // 日期 YYYY-MM-DD
  order_count: number;    // 订单数 (int32)
  total_sales: number;    // 销售额 (分, int64)
  commission: number;     // 佣金 (分, int64)
  takeout_orders: number; // 外卖订单数 (int32)
  dine_in_orders: number; // 堂食订单数 (int32)
}

// ==================== 菜品排行 ====================

/**
 * 菜品销量排行 - 对应 topSellingDishRow
 */
export interface TopDishRow {
  dish_id: number;      // 菜品ID (int64)
  dish_name: string;    // 菜品名称
  dish_price: number;   // 菜品价格 (分, int64)
  total_sold: number;   // 销量 (int32)
  total_revenue: number;// 营收 (分, int64)
}

// ==================== 时段分析 ====================

/**
 * 时段统计 - 对应 merchantHourlyStatsRow
 */
export interface HourlyStatsRow {
  hour: number;           // 小时 0-23 (int32)
  order_count: number;    // 订单数 (int32)
  avg_order_amount: number; // 平均订单金额 (分, int64)
}

// ==================== 订单来源 ====================

/**
 * 订单来源统计 - 对应 merchantOrderSourceStatsRow
 */
export interface OrderSourceStatsRow {
  order_type: string;   // 订单类型: takeout/dine_in/pickup
  order_count: number;  // 订单数 (int32)
  total_sales: number;  // 销售额 (分, int64)
}

// ==================== 复购率分析 ====================

/**
 * 复购率统计 - 对应 merchantRepurchaseRateResponse
 */
export interface RepurchaseRateResponse {
  total_users: number;        // 总顾客数 (int32)
  repeat_users: number;       // 复购顾客数 (int32)
  repurchase_rate: number;    // 复购率 (百分比, float64)
  avg_orders_per_user: number;// 人均订单数 (float64)
}

// ==================== 分类统计 ====================

/**
 * 分类销售统计 - 对应 dishCategoryStatsRow
 */
export interface CategoryStatsRow {
  category_name: string;  // 分类名称
  order_count: number;    // 订单数 (int32)
  total_sales: number;    // 销售额 (分, int64)
  total_quantity: number; // 销量 (int32)
}

// ==================== 顾客统计 ====================

/**
 * 顾客消费统计 - 对应 customerStatRow
 */
export interface CustomerStatRow {
  user_id: number;        // 用户ID (int64)
  full_name: string;      // 姓名
  phone: string;          // 电话
  avatar_url: string;     // 头像URL
  total_orders: number;   // 总订单数 (int32)
  total_amount: number;   // 总消费 (分, int64)
  avg_order_amount: number; // 平均订单金额 (分, int64)
  first_order_at: string; // 首次下单时间 (RFC3339)
  last_order_at: string;  // 最后下单时间 (RFC3339)
}

/**
 * 顾客喜好菜品 - 对应 favoriteDishRow
 */
export interface FavoriteDishRow {
  dish_id: number;      // 菜品ID (int64)
  dish_name: string;    // 菜品名称
  order_count: number;  // 下单次数 (int32)
  total_quantity: number; // 总数量 (int32)
}

/**
 * 顾客详情响应 - 对应 customerDetailResponse
 */
export interface CustomerDetailResponse extends CustomerStatRow {
  favorite_dishes: FavoriteDishRow[];
}

/**
 * 顾客列表响应
 */
export interface CustomerListResponse {
  data: CustomerStatRow[];
  total_count: number;
  total: number;
  page_id: number;
  page_size: number;
  page: number;
  limit: number;
}

// ==================== 财务统计 ====================

/**
 * 财务概览响应 - 对应 financeOverviewResponse
 */
export interface FinanceOverviewResponse {
  completed_orders?: number;    // 已完成订单数
  pending_orders?: number;      // 待结算订单数
  promotion_orders?: number;    // 满返订单数
  total_gmv?: number;           // 总GMV (分)
  total_income?: number;        // 商户总收入 (分)
  net_income?: number;          // 净收入 (分)
  pending_income?: number;      // 待结算收入 (分)
  total_platform_fee?: number;  // 平台服务费 (分)
  total_operator_fee?: number;  // 运营商服务费 (分)
  total_service_fee?: number;   // 总服务费 (分)
  total_promotion_exp?: number; // 满返支出 (分)
}

// ==================== 工具类型 ====================

/**
 * 日期范围参数
 */
export interface DateRangeParams {
  start_date: string;
  end_date: string;
}

/**
 * 订单类型标签映射
 */
export const ORDER_TYPE_LABELS: Record<string, string> = {
  takeout: "外卖配送",
  dine_in: "堂食点餐",
  pickup: "到店自取",
  reservation: "预订",
};
