/**
 * 商户订单管理接口
 * 基于swagger.json完全重构，仅保留后端支持的接口
 */

import { request } from '../utils/request'

// ==================== 订单数据类型定义 ====================

/**
 * 订单响应 - 完全对齐 api.orderResponse
 */
export interface OrderResponse {
    id: number                                   // 订单ID
    order_no: string                             // 订单编号
    order_type: 'takeout' | 'dine_in' | 'takeaway' | 'reservation'  // 订单类型
    status: 'pending' | 'paid' | 'preparing' | 'ready' | 'delivering' | 'completed' | 'cancelled'  // 订单状态
    user_id: number                              // 用户ID
    merchant_id: number                          // 商户ID
    merchant_name: string                        // 商户名称
    table_id?: number                            // 桌台ID（堂食订单）
    address_id?: number                          // 地址ID（外卖订单）
    reservation_id?: number                      // 预定ID（预定订单）
    items: OrderItemResponse[]                   // 订单商品列表
    subtotal: number                             // 商品小计（分）
    delivery_fee: number                         // 配送费（分）
    delivery_fee_discount: number                // 配送费优惠（分）
    discount_amount: number                      // 优惠金额（分）
    total_amount: number                         // 订单总金额（分）
    payment_method: 'wechat' | 'balance'         // 支付方式
    notes?: string                               // 订单备注
    delivery_distance?: number                   // 配送距离（米）
    created_at: string                           // 创建时间
    paid_at?: string                             // 支付时间
    completed_at?: string                        // 完成时间
    cancelled_at?: string                        // 取消时间
    cancel_reason?: string                       // 取消原因
    updated_at: string                           // 更新时间
}

/**
 * 订单商品项响应 - 对齐 api.orderItemResponse
 */
export interface OrderItemResponse {
    combo_id?: number                            // 套餐ID（套餐订单时有值）
    customizations?: OrderItemCustomization[]    // 定制化选项列表
    dish_id?: number                             // 菜品ID（菜品订单时有值）
    id: number                                   // 订单明细ID
    image_url: string                            // 商品图片URL
    name: string                                 // 商品名称
    quantity: number                             // 数量
    subtotal: number                             // 小计金额（分，含定制化加价）
    unit_price: number                           // 单价（分）
}

/**
 * 订单商品定制化 - 完全对齐 api.orderItemCustomization
 */
export interface OrderItemCustomization {
    group_name: string                           // 定制分组名称
    option_name: string                          // 定制选项名称
    price_adjustment: number                     // 价格调整（分）
}

/**
 * 订单统计响应 - 对齐 db.GetOrderStatsRow
 */
export interface OrderStatsResponse {
    total_orders: number                         // 总订单数
    total_revenue: number                        // 总营收（分）
    avg_order_value: number                      // 平均订单价值（分）
    completed_orders: number                     // 已完成订单数
    cancelled_orders: number                     // 已取消订单数
    completion_rate: number                      // 完成率
}

/**
 * 拒单请求 - 对齐 api.rejectOrderRequest
 */
export interface RejectOrderRequest extends Record<string, unknown> {
    reason: string                               // 拒单原因（必填）
}

// ==================== KDS后厨数据类型定义 ====================

/**
 * 后厨订单响应 - 对齐 api.kitchenOrderResponse
 */
export interface KitchenOrderResponse {
    created_at: string                           // 创建时间
    estimated_ready_at?: string                  // 预计出餐时间
    id: number                                   // 订单ID
    is_urged: boolean                            // 是否催单
    items: KitchenOrderItem[]                    // 订单商品列表
    notes?: string                               // 订单备注
    order_no: string                             // 订单编号
    order_type: string                           // 订单类型
    paid_at?: string                             // 支付时间
    pickup_number?: string                       // 取餐号（打包自取）
    status: string                               // 订单状态
    table_no?: string                            // 桌台号（堂食订单）
    table_number?: string                        // 桌台号（前端兼容）
    waiting_minutes: number                      // 等待时间（分钟）
    preparing_started_at?: string                // 开始制作时间
    ready_at?: string                            // 制作完成时间
    customer_name?: string                       // 顾客姓名
    estimated_time?: number                      // 预计制作时间（分钟）
}

/**
 * 后厨订单商品项 - 对齐 api.kitchenOrderItem
 */
export interface KitchenOrderItem {
    customizations?: OrderItemCustomization[]    // 定制化选项
    id: number                                   // 商品项ID
    image_url: string                            // 商品图片URL
    name: string                                 // 商品名称
    prepare_time: number                         // 预估制作时间（分钟）
    quantity: number                             // 数量
}

/**
 * 后厨统计信息 - 对齐 api.kitchenStats
 */
export interface KitchenStats {
    avg_prepare_time?: number                    // 平均出餐时间（分钟）
    completed_today_count?: number               // 今日完成订单数
    new_count?: number                           // 新订单数
    preparing_count?: number                     // 制作中订单数
    ready_count?: number                         // 待取餐数
    total_pending?: number                       // 待处理总数 (兼容)
    avg_preparation_time?: number                // 平均制作时间 (兼容)
    orders_behind_schedule?: number              // 超时订单数 (兼容)
}

/**
 * 后厨订单列表响应 - 完全对齐 api.kitchenOrdersResponse
 */
export interface KitchenOrdersResponse {
    new_orders: KitchenOrderResponse[]           // 新订单列表
    preparing_orders: KitchenOrderResponse[]     // 制作中订单列表
    ready_orders: KitchenOrderResponse[]         // 待取餐订单列表
    stats?: KitchenStats                         // 统计信息
}

// ==================== 商户订单管理服务 ====================

/**
 * 商户订单管理服务
 * 基于swagger.json完全重构，仅包含后端支持的接口
 */
export class MerchantOrderManagementService {

    /**
     * 获取商户订单列表
     * GET /v1/merchant/orders
     */
    static async getOrderList(params: {
        page_id: number                            // 页码（必填）
        page_size: number                          // 每页数量（必填，5-50）
        status?: 'pending' | 'paid' | 'preparing' | 'ready' | 'delivering' | 'completed' | 'cancelled'  // 状态筛选
    }): Promise<OrderResponse[]> {
        return await request({
            url: '/v1/merchant/orders',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取订单统计
     * GET /v1/merchant/orders/stats
     */
    static async getOrderStats(params: {
        start_date: string                         // 开始日期（YYYY-MM-DD，必填）
        end_date: string                           // 结束日期（YYYY-MM-DD，必填）
    }): Promise<OrderStatsResponse> {
        return await request({
            url: '/v1/merchant/orders/stats',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取订单详情
     * GET /v1/merchant/orders/{id}
     */
    static async getOrderDetail(orderId: number): Promise<OrderResponse> {
        return await request({
            url: `/v1/merchant/orders/${orderId}`,
            method: 'GET'
        })
    }

    /**
     * 商户接单
     * POST /v1/merchant/orders/{id}/accept
     */
    static async acceptOrder(orderId: number): Promise<OrderResponse> {
        return await request({
            url: `/v1/merchant/orders/${orderId}/accept`,
            method: 'POST'
        })
    }

    /**
     * 商户拒单
     * POST /v1/merchant/orders/{id}/reject
     */
    static async rejectOrder(orderId: number, data: RejectOrderRequest): Promise<OrderResponse> {
        return await request({
            url: `/v1/merchant/orders/${orderId}/reject`,
            method: 'POST',
            data
        })
    }

    /**
     * 标记订单准备完成
     * POST /v1/merchant/orders/{id}/ready
     */
    static async markOrderReady(orderId: number): Promise<OrderResponse> {
        return await request({
            url: `/v1/merchant/orders/${orderId}/ready`,
            method: 'POST'
        })
    }

    /**
     * 完成订单（堂食/打包自取）
     * POST /v1/merchant/orders/{id}/complete
     */
    static async completeOrder(orderId: number): Promise<OrderResponse> {
        return await request({
            url: `/v1/merchant/orders/${orderId}/complete`,
            method: 'POST'
        })
    }
}

// ==================== KDS后厨管理服务 ====================

/**
 * KDS后厨管理服务
 * 基于swagger.json完全重构，仅包含后端支持的接口
 */
export class KitchenDisplayService {

    /**
     * 获取厨房订单列表
     * GET /v1/kitchen/orders
     */
    static async getKitchenOrders(): Promise<KitchenOrdersResponse> {
        return await request({
            url: '/v1/kitchen/orders',
            method: 'GET'
        })
    }

    /**
     * 获取厨房订单详情
     * GET /v1/kitchen/orders/{id}
     */
    static async getKitchenOrderDetail(orderId: number): Promise<KitchenOrderResponse> {
        return await request({
            url: `/v1/kitchen/orders/${orderId}`,
            method: 'GET'
        })
    }

    /**
     * 开始制作订单
     * POST /v1/kitchen/orders/{id}/preparing
     */
    static async startPreparing(orderId: number): Promise<KitchenOrderResponse> {
        return await request({
            url: `/v1/kitchen/orders/${orderId}/preparing`,
            method: 'POST'
        })
    }

    /**
     * 标记订单制作完成
     * POST /v1/kitchen/orders/{id}/ready
     */
    static async markKitchenOrderReady(orderId: number): Promise<KitchenOrderResponse> {
        return await request({
            url: `/v1/kitchen/orders/${orderId}/ready`,
            method: 'POST'
        })
    }
}

// ==================== 订单管理适配器 ====================

/**
 * 订单管理数据适配器
 * 处理前端展示数据和后端API数据之间的转换
 */
export class OrderManagementAdapter {

    /**
     * 格式化订单状态显示文本
     */
    static formatOrderStatus(status: string): string {
        const statusMap: Record<string, string> = {
            'pending': '待支付',
            'paid': '已支付',
            'preparing': '制作中',
            'ready': '待配送/待取餐',
            'delivering': '配送中',
            'completed': '已完成',
            'cancelled': '已取消'
        }
        return statusMap[status] || status
    }

    /**
     * 格式化订单类型显示文本
     */
    static formatOrderType(orderType: string): string {
        const typeMap: Record<string, string> = {
            'takeout': '外卖',
            'dine_in': '堂食',
            'takeaway': '打包自取',
            'reservation': '预定点菜'
        }
        return typeMap[orderType] || orderType
    }

    /**
     * 格式化支付方式显示文本
     */
    static formatPaymentMethod(paymentMethod: string): string {
        const methodMap: Record<string, string> = {
            'wechat': '微信支付',
            'balance': '余额支付'
        }
        return methodMap[paymentMethod] || paymentMethod
    }

    /**
     * 计算订单实际支付金额
     */
    static calculateActualAmount(order: OrderResponse): number {
        return order.subtotal + order.delivery_fee - order.delivery_fee_discount - order.discount_amount
    }

    /**
     * 格式化金额显示（分转元）
     */
    static formatAmount(amountInCents: number): string {
        return (amountInCents / 100).toFixed(2)
    }

    /**
     * 格式化距离显示
     */
    static formatDistance(distanceInMeters?: number): string {
        if (!distanceInMeters) return '--'

        if (distanceInMeters < 1000) {
            return `${distanceInMeters}m`
        } else {
            return `${(distanceInMeters / 1000).toFixed(1)}km`
        }
    }

    /**
     * 判断订单是否可以接单
     */
    static canAcceptOrder(order: OrderResponse): boolean {
        return order.status === 'paid'
    }

    /**
     * 判断订单是否可以拒单
     */
    static canRejectOrder(order: OrderResponse): boolean {
        return ['paid', 'preparing'].includes(order.status)
    }

    /**
     * 判断订单是否可以标记为准备完成
     */
    static canMarkReady(order: OrderResponse): boolean {
        return order.status === 'preparing'
    }

    /**
     * 判断订单是否可以完成
     */
    static canCompleteOrder(order: OrderResponse): boolean {
        return order.status === 'ready' && ['dine_in', 'takeaway'].includes(order.order_type)
    }

    /**
     * 获取订单状态对应的颜色
     */
    static getStatusColor(status: string): string {
        const colorMap: Record<string, string> = {
            'pending': '#f39c12',      // 橙色
            'paid': '#3498db',         // 蓝色
            'preparing': '#e74c3c',    // 红色
            'ready': '#f39c12',        // 橙色
            'delivering': '#9b59b6',   // 紫色
            'completed': '#27ae60',    // 绿色
            'cancelled': '#95a5a6'     // 灰色
        }
        return colorMap[status] || '#95a5a6'
    }

    /**
     * 计算订单制作时长（分钟）
     */
    static calculatePreparationTime(order: KitchenOrderResponse): number | null {
        if (!order.preparing_started_at || !order.ready_at) {
            return null
        }

        const startTime = new Date(order.preparing_started_at)
        const endTime = new Date(order.ready_at)

        return Math.round((endTime.getTime() - startTime.getTime()) / (1000 * 60))
    }

    /**
     * 判断订单是否超时
     */
    static isOrderOverdue(order: KitchenOrderResponse): boolean {
        if (!order.preparing_started_at || order.ready_at || !order.estimated_time) {
            return false
        }

        const startTime = new Date(order.preparing_started_at)
        const now = new Date()
        const elapsedMinutes = (now.getTime() - startTime.getTime()) / (1000 * 60)

        return elapsedMinutes > order.estimated_time
    }

    /**
     * 获取订单剩余制作时间（分钟）
     */
    static getRemainingTime(order: KitchenOrderResponse): number {
        if (!order.preparing_started_at || order.ready_at || !order.estimated_time) {
            return 0
        }

        const startTime = new Date(order.preparing_started_at)
        const now = new Date()
        const elapsedMinutes = (now.getTime() - startTime.getTime()) / (1000 * 60)

        return Math.max(0, order.estimated_time - elapsedMinutes)
    }
}

// ==================== 导出默认服务 ====================

export default MerchantOrderManagementService