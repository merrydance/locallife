/**
 * 订单相关API接口
 * 基于swagger.json中的订单管理接口
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 订单状态枚举 */
export type OrderStatus =
  | 'pending'     // 待支付
  | 'paid'        // 已支付
  | 'preparing'   // 制作中
  | 'ready'       // 待配送/待取餐
  | 'delivering'  // 配送中
  | 'completed'   // 已完成
  | 'cancelled'   // 已取消

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
  image_url: string       // 商品图片URL
  quantity: number
  unit_price: number      // 单价（分）
  subtotal: number        // 小计金额（分，含定制化加价）
  customizations?: OrderCustomizationItem[]
}

/** 订单商品定制化 - 对齐swagger api.orderCustomizationItem */
export interface OrderCustomizationItem {
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
  merchant_name: string
  status: OrderStatus
  order_type: OrderType
  payment_method?: 'wechat' | 'balance'
  items: OrderItemResponse[]
  subtotal: number              // 商品小计（分，不含配送费）
  total_amount: number          // 订单总金额（分）
  delivery_fee: number
  delivery_fee_discount: number
  discount_amount: number
  notes?: string
  created_at: string
  updated_at: string
  paid_at?: string
  completed_at?: string
  cancelled_at?: string
  cancel_reason?: string
  // 配送相关
  address_id?: number
  delivery_distance?: number
  // 堂食相关
  table_id?: number
  // 预定相关
  reservation_id?: number
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
  use_balance?: boolean         // 是否使用会员余额支付
  user_voucher_id?: number      // 用户优惠券ID
}

/** 订单商品请求 - 对齐 api.orderItemRequest */
export interface OrderItemRequest {
  combo_id?: number             // 套餐ID
  customizations?: OrderCustomizationItem[]  // 定制化选项
  dish_id?: number              // 菜品ID
  quantity: number              // 数量
}

/** 订单列表查询参数 */
export interface ListOrdersParams extends Record<string, unknown> {
  page_id: number
  page_size: number
  status?: OrderStatus
}

/** 订单计算参数 */
export interface CalculateOrderParams extends Record<string, unknown> {
  merchant_id: number
  items: Array<{
    dish_id: number
    quantity: number
    customizations?: Array<{
      group_id: number
      option_id: number
    }>
    combo_id?: number
  }>
  address_id?: number
  voucher_id?: number
  use_membership_discount?: boolean
  order_type: OrderType
}

/** 订单计算结果 - 对齐 api.calculateCartResponse */
/** 订单计算结果 - 对齐 api.orderCalculationResponse */
export interface OrderCalculationResponse {
  delivery_fee?: number             // 配送费（分）
  delivery_fee_discount?: number    // 配送费优惠（分）
  discount_amount?: number          // 满减优惠（分）
  items?: CalculatedItemResponse[]  // 商品明细
  promotions?: PromotionApplied[]   // 优惠明细
  subtotal?: number                 // 商品小计（分）
  total_amount?: number             // 最终应付金额（分）
}

/** 计算后的商品项 */
export interface CalculatedItemResponse {
  dish_id: number
  name: string
  quantity: number
  unit_price: number
  subtotal: number
}

/** 已应用的优惠 */
export interface PromotionApplied {
  name: string
  discount_amount: number
}

/** 取消订单请求 */
export interface CancelOrderRequest extends Record<string, unknown> {
  reason: string
}

/** 催单请求 */
export interface UrgeOrderRequest extends Record<string, unknown> {
  message?: string
}

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
  amount?: number             // 优惠金额（分）
  title?: string              // 优惠名称
  type?: string               // 优惠类型：discount, delivery_fee_return, voucher
}

// ==================== API接口函数 ====================

/**
 * 获取订单列表
 * @param params 查询参数
 */
export async function getOrderList(params: ListOrdersParams): Promise<OrderResponse[]> {
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
export async function cancelOrder(orderId: number, cancelData: CancelOrderRequest): Promise<void> {
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
export async function confirmOrder(orderId: number): Promise<void> {
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

// ==================== 便捷方法 ====================

/**
 * 获取指定状态的订单
 * @param status 订单状态
 * @param pageSize 每页数量
 */
export async function getOrdersByStatus(status: OrderStatus, pageSize: number = 10): Promise<OrderResponse[]> {
  return getOrderList({
    page_id: 1,
    page_size: pageSize,
    status
  })
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
  const statuses: OrderStatus[] = ['paid', 'preparing', 'ready', 'delivering']
  const results = await Promise.all(
    statuses.map(status => getOrdersByStatus(status, 20))
  )
  return results.reduce((acc, curr) => acc.concat(curr), [])
}

/**
 * 获取历史订单（已完成或已取消）
 */
export async function getHistoryOrders(): Promise<OrderResponse[]> {
  const statuses: OrderStatus[] = ['completed', 'cancelled']
  const results = await Promise.all(
    statuses.map(status => getOrdersByStatus(status, 20))
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
    voucher_id?: number
    use_membership_discount?: boolean
    notes?: string
    table_id?: number
    guest_count?: number
    reservation_id?: number
  } = {}
): Promise<OrderResponse> {
  // 这里需要先获取购物车数据，然后转换为订单格式
  // 实际实现时需要调用购物车API
  console.log('Creating order from cart for merchant:', merchantId, 'type:', orderType, 'options:', options)
  throw new Error('需要先实现购物车到订单的转换逻辑')
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

