/**
 * 订单相关API接口
 * 基于swagger.json中的订单管理接口
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 订单状态枚举（业务状态） */
export type OrderStatus =
  | 'pending'     // 待支付
  | 'paid'        // 已支付
  | 'preparing'   // 制作中
  | 'ready'       // 待配送/待取餐
  | 'courier_accepted' // 骑手已接单
  | 'picked'      // 已取餐
  | 'delivering'  // 配送中
  | 'rider_delivered' // 骑手已送达
  | 'user_delivered'  // 用户已确认
  | 'completed'   // 已完成
  | 'cancelled'   // 已取消

/** 履约状态枚举（厨房/出餐流转） */
export type FulfillmentStatus =
  | 'scheduled'
  | 'pending_kitchen'
  | 'preparing'
  | 'ready'
  | 'completed'
  | 'cancelled'

/** 订单类型枚举 */
export type OrderType =
  | 'takeout'     // 外卖
  | 'dine_in'     // 堂食
  | 'takeaway'    // 打包自取
  | 'reservation' // 预定点菜

/** 订单商品项响应 - 对齐swagger api.orderItemResponse */
export interface OrderItemResponse {
  id: number
  dish_id?: number        // 菜品ID (菜品订单时有值)
  combo_id?: number       // 套餐ID (套餐订单时有值)
  name: string            // 商品名称
  image_url?: string      // 商品图片URL
  quantity: number
  unit_price: number      // 单价（分）
  subtotal: number        // 小计金额（分，含定制化加价）
  customizations?: OrderCustomizationItem[]
}

/** 订单商品定制化 - 对齐swagger api.orderCustomizationItem */
export interface OrderCustomizationItem {
  group_id?: number       // 定制分组ID
  option_id?: number      // 定制选项ID
  tag_id?: number         // 标签ID
  extra_price: number     // 额外价格（分）
  name: string            // 定制项名称
  value: string           // 定制项取值
}

/** 订单响应 - 对齐swagger api.orderResponse */
export interface OrderResponse {
  id: number
  order_no: string
  user_id: number
  merchant_id: number
  merchant_name?: string
  merchant_phone?: string          // 商户电话
  status: OrderStatus
  status_hint?: string
  badges?: Array<{ text: string, type?: string, locale?: string }> | string[]
  actions?: string[]
  dispatch_order_id?: number
  flow_id?: number
  pickup_code?: string
  pickup_code_masked?: string
  exception_state?: string
  claim_channel?: string
  overtime?: boolean
  fulfillment_status: FulfillmentStatus
  order_type: OrderType
  payment_method?: 'wechat' | 'balance'
  items?: OrderItemResponse[]
  subtotal: number              // 商品小计（分，不含配送费）
  total_amount: number          // 订单总金额（分）
  delivery_fee: number
  delivery_fee_discount: number
  discount_amount: number
  delivery_eta_minutes?: number      // 预计送达总时长（分钟）
  estimated_delivery_at?: string     // 预计送达时间（ISO字符串）
  notes?: string
  created_at: string
  updated_at?: string
  paid_at?: string
  prep_start_at?: string
  ready_at?: string
  courier_accept_at?: string
  picked_at?: string
  rider_delivered_at?: string
  user_delivered_at?: string
  auto_user_delivered_at?: string
  completed_at?: string
  cancelled_at?: string
  cancel_reason?: string
  // 配送相关
  address_id?: number
  delivery_distance?: number
  delivery_contact_name?: string   // 配送联系人
  delivery_contact_phone?: string  // 配送联系电话
  delivery_address?: string        // 配送地址
  // 堂食相关
  table_id?: number
  // 预定相关
  reservation_id?: number
  replaced_by_order_id?: number
  // 微信支付交易号，用于拉起小程序确认收货组件
  wechat_transaction_id?: string
}

/** 计算后的应付金额（便捷属性，total_amount - discount_amount） */
export function getPayableAmount(order: OrderResponse): number {
  return order.total_amount - order.discount_amount
}

/** 创建订单请求 - 对齐 api.createOrderRequest */
export interface CreateOrderRequest extends Record<string, unknown> {
  address_id?: number           // 配送地址ID（外卖订单必填）
  items: OrderItemRequest[]     // 订单商品列表
  merchant_id: number           // 商户ID
  notes?: string                // 订单备注
  order_type: OrderType         // 订单类型
  reservation_id?: number       // 预订ID（预定点菜时必填）
  table_id?: number             // 桌台ID（堂食订单必填）
  billing_group_id?: number     // 账单组ID（堂食可选）
  use_balance?: boolean         // 是否使用会员余额支付
  user_voucher_id?: number      // 用户优惠券ID
  delivery_fee?: number         // 前端计算的配送费（分）
  delivery_fee_discount?: number// 前端计算的配送费优惠（分）
  delivery_distance?: number    // 前端计算的配送距离（米）
}

/** 订单商品请求 - 对齐 api.orderItemRequest */
export interface OrderItemRequest {
  combo_id?: number             // 套餐ID
  customizations?: Record<string, number | string>  // 定制化选项：{group_id: option_id}
  dish_id?: number              // 菜品ID
  quantity: number              // 数量
}

/** 订单列表查询参数 */
export interface ListOrdersParams extends Record<string, unknown> {
  page_id: number
  page_size: number
  status?: OrderStatus
  order_type?: OrderType
  reservation_id?: number
  fulfillment_status?: FulfillmentStatus
}

/** 订单列表响应 - 对齐 api.listOrdersResponse */
export interface ListOrdersResponse {
  orders: OrderResponse[]
  total: number
  page_id: number
  page_size: number
}

/** 订单计算参数 */
export interface CalculateOrderParams extends Record<string, unknown> {
  merchant_id: number
  order_type: OrderType
  latitude?: number
  longitude?: number
  address_id?: number
  user_voucher_id?: number
  voucher_code?: string
}

/** 订单计算结果 - 对齐 api.calculateCartResponse */
/** 订单计算结果 - 对齐 api.orderCalculationResponse */
export interface OrderCalculationResponse {
  delivery_fee: number              // 配送费（分）
  delivery_fee_discount: number     // 配送费优惠（分）
  discount_amount: number           // 满减优惠（分）
  items: CalculatedItemResponse[]   // 商品明细
  promotions?: PromotionApplied[]   // 优惠明细
  subtotal: number                  // 商品小计（分）
  total_amount: number              // 最终应付金额（分）
}

/** 计算后的商品项 */
export interface CalculatedItemResponse {
  dish_id?: number
  combo_id?: number
  name: string
  quantity: number
  unit_price: number
  subtotal: number
}

/** 已应用的优惠 */
export interface PromotionApplied {
  title: string
  amount: number
  type: string
}

/** 取消订单请求 */
export interface CancelOrderRequest extends Record<string, unknown> {
  reason: string
}

/** 催单请求 */
export interface UrgeOrderRequest extends Record<string, unknown> {
  message?: string
}

/** 替换订单请求体（后端保持向前兼容，使用 Record 以适配变更） */
export type ReplaceOrderRequest = Record<string, unknown>

/** 拒单请求体 - 对齐 api.rejectOrderBody */
export interface RejectOrderBody extends Record<string, unknown> {
  reason: string              // 拒单原因（2-200字符，必填）
}

/** 订单计算商品项 - 对齐 api.orderCalculationItem */
export interface OrderCalculationItem {
  dish_id?: number            // 菜品ID
  combo_id?: number           // 套餐ID
  name: string                // 商品名称
  quantity: number            // 数量
  unit_price: number          // 单价（分）
  subtotal: number            // 小计（分）
}

/** 订单优惠项 - 对齐 api.orderPromotion */
export interface OrderPromotion {
  amount: number              // 优惠金额（分）
  title: string               // 优惠名称
  type: string                // 优惠类型：discount, delivery_fee_return, voucher
}

// ==================== API接口函数 ====================

/**
 * 获取订单列表
 * @param params 查询参数
 */
export async function getOrderList(params: ListOrdersParams): Promise<ListOrdersResponse> {
  return request({
    url: '/v1/orders',
    method: 'GET',
    data: params
  })
}

/**
 * 获取订单详情
 * @param orderId 订单ID
 */
export async function getOrderDetail(orderId: number): Promise<OrderResponse> {
  return request({
    url: `/v1/orders/${orderId}`,
    method: 'GET'
  })
}

/**
 * 创建订单
 * @param orderData 订单数据
 */
export async function createOrder(orderData: CreateOrderRequest): Promise<OrderResponse> {
  return request({
    url: '/v1/orders',
    method: 'POST',
    data: orderData
  })
}

/**
 * 计算订单金额
 * @param params 计算参数
 */
export async function calculateOrder(params: CalculateOrderParams): Promise<OrderCalculationResponse> {
  return request({
    url: '/v1/orders/calculate',
    method: 'GET',
    data: params
  })
}

/**
 * 取消订单
 * @param orderId 订单ID
 * @param cancelData 取消原因
 */
export async function cancelOrder(orderId: number, cancelData: CancelOrderRequest): Promise<OrderResponse> {
  return request({
    url: `/v1/orders/${orderId}/cancel`,
    method: 'POST',
    data: cancelData
  })
}

/**
 * 确认订单（用户确认收货）
 * @param orderId 订单ID
 */
export async function confirmOrder(orderId: number): Promise<OrderResponse> {
  return request({
    url: `/v1/orders/${orderId}/confirm`,
    method: 'POST'
  })
}

/**
 * 催单
 * @param orderId 订单ID
 * @param urgeData 催单信息
 */
export async function urgeOrder(orderId: number, urgeData: UrgeOrderRequest = {}): Promise<void> {
  return request({
    url: `/v1/orders/${orderId}/urge`,
    method: 'POST',
    data: urgeData
  })
}

/**
 * 替换订单（生成新订单，旧订单标记为已被替换）
 */
export async function replaceOrder(orderId: number, data: ReplaceOrderRequest = {}): Promise<OrderResponse> {
  return request({
    url: `/v1/orders/${orderId}/replace`,
    method: 'POST',
    data
  })
}

// ==================== 便捷方法 ====================

/**
 * 获取指定状态的订单
 * @param status 订单状态
 * @param pageSize 每页数量
 */
export async function getOrdersByStatus(status: OrderStatus, pageSize: number = 10): Promise<OrderResponse[]> {
  const response = await getOrderList({
    page_id: 1,
    page_size: pageSize,
    status
  })
  return response.orders
}

/**
 * 获取待支付订单
 */
export async function getPendingOrders(): Promise<OrderResponse[]> {
  return getOrdersByStatus('pending')
}

/**
 * 获取进行中的订单（已支付但未完成）
 */
export async function getActiveOrders(): Promise<OrderResponse[]> {
  const statuses: OrderStatus[] = [
    'paid',
    'preparing',
    'ready',
    'courier_accepted',
    'picked',
    'delivering',
    'rider_delivered'
  ]
  const results = await Promise.all(
    statuses.map((status) => getOrdersByStatus(status, 20))
  )
  return results.reduce((acc, curr) => acc.concat(curr), [])
}

/**
 * 获取历史订单（已完成或已取消）
 */
export async function getHistoryOrders(): Promise<OrderResponse[]> {
  const statuses: OrderStatus[] = ['user_delivered', 'completed', 'cancelled']
  const results = await Promise.all(
    statuses.map((status) => getOrdersByStatus(status, 20))
  )
  return results.reduce((acc, curr) => acc.concat(curr), [])
}

/**
 * 从购物车创建订单
 * @param merchantId 商户ID
 * @param orderType 订单类型
 * @param options 其他选项
 */
export async function createOrderFromCart(
  merchantId: number,
  orderType: OrderType,
  options: {
    address_id?: number
    use_balance?: boolean
    user_voucher_id?: number
    notes?: string
    table_id?: number
    reservation_id?: number
    // 前端计算透传字段
    delivery_fee?: number
    delivery_fee_discount?: number
    delivery_distance?: number
  } = {}
): Promise<OrderResponse> {
  const { getCart } = await import('./cart')
  
  // 1. 获取对应商户和类型的购物车数据
  const cart = await getCart({
    merchant_id: merchantId,
    order_type: orderType,
    table_id: options.table_id,
    reservation_id: options.reservation_id
  })

  if (!cart || cart.items.length === 0) {
    throw new Error('购物车为空，无法创建订单')
  }

  // 2. 将购物车项转换为订单项
  const items: OrderItemRequest[] = cart.items.map((item) => ({
    dish_id: item.dish_id,
    combo_id: item.combo_id,
    quantity: item.quantity,
    customizations: item.customizations as Record<string, number | string>
  }))

  // 3. 提交创建订单
  return createOrder({
    merchant_id: merchantId,
    order_type: orderType,
    items,
    address_id: options.address_id,
    use_balance: options.use_balance,
    user_voucher_id: options.user_voucher_id,
    notes: options.notes,
    table_id: options.table_id,
    reservation_id: options.reservation_id,
    delivery_fee: options.delivery_fee,
    delivery_fee_discount: options.delivery_fee_discount,
    delivery_distance: options.delivery_distance
  })
}

// ==================== 兼容性别名 ====================

/** @deprecated 使用 getOrderList 替代 */
export const getOrders = getOrderList

/** @deprecated 使用 OrderResponse 替代 */
export type OrderDTO = OrderResponse

/** @deprecated 使用 OrderItemResponse 替代 */
export type OrderItemDTO = OrderItemResponse

/** @deprecated 使用 calculateOrder 替代 */
export const previewOrder = calculateOrder

/** @deprecated 使用 CreateOrderRequest 替代 */
export type CreateOrderData = CreateOrderRequest

