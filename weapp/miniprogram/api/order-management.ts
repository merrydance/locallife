/**
 * 商户订单管理接口
 * 基于swagger.json完全重构，仅保留后端支持的接口
 */

import { request } from '../utils/request'
import { canMerchantMarkOrderReady } from '../utils/merchant-order-action-view'

export const MERCHANT_REJECT_REASON_OPTIONS = [
    '门店临时打烊',
    '商品已售罄',
    '代取资源不足',
    '订单信息异常'
] as const

// ==================== 订单数据类型定义 ====================

export interface MerchantOrderFeeBreakdown {
    food_amount: number
    merchant_discount_amount: number
    voucher_discount_amount: number
    food_payable_amount: number
    delivery_fee_amount: number
    delivery_fee_discount_amount: number
    delivery_payable_amount: number
    customer_payable_amount: number
    platform_service_fee_amount: number
    payment_channel_fee_amount: number
    merchant_receivable_amount: number
    rider_gross_amount?: number
    rider_payment_fee_amount?: number
    rider_net_earnings_amount?: number
}

/**
 * 订单响应 - 完全对齐 api.orderResponse
 */
export interface OrderResponse {
    id: number                                   // 订单ID
    order_no: string                             // 订单编号
    order_type: 'takeout' | 'dine_in' | 'takeaway' | 'reservation'  // 订单类型
    status: 'pending' | 'paid' | 'preparing' | 'ready' | 'courier_accepted' | 'picked' | 'delivering' | 'rider_delivered' | 'user_delivered' | 'completed' | 'cancelled'  // 订单状态
    user_id: number                              // 用户ID
    merchant_id: number                          // 商户ID
    merchant_name: string                        // 商户名称
    table_id?: number                            // 桌台ID（堂食订单）
    address_id?: number                          // 地址ID（外卖订单）
    reservation_id?: number                      // 预定ID（预定订单）
    items: OrderItemResponse[]                   // 订单商品列表
    subtotal: number                             // 商品小计（分）
    delivery_fee: number                         // 代取费（分）
    delivery_fee_discount: number                // 代取费优惠（分）
    discount_amount: number                      // 优惠金额（分）
    total_amount: number                         // 订单总金额（分）
    fee_breakdown?: MerchantOrderFeeBreakdown     // 商户视角费用清单（金额单位：分）
    fulfillment_status?: 'scheduled' | 'pending_kitchen' | 'preparing' | 'ready' | 'completed' | 'cancelled'
    payment_method?: 'wechat' | 'balance'        // 支付方式
    notes?: string                               // 订单备注
    status_hint?: string
    actions?: string[]
    can_mark_ready?: boolean
    exception_state?: string
    claim_channel?: string
    overtime?: boolean
    delivery_eta_minutes?: number
    estimated_delivery_at?: string
    pickup_code?: string
    pickup_code_masked?: string
    merchant_phone?: string
    delivery_contact_name?: string
    delivery_contact_phone?: string
    delivery_address?: string
    delivery_distance?: number                   // 代取距离（米）
    created_at: string                           // 创建时间
    paid_at?: string                             // 支付时间
    prep_start_at?: string
    ready_at?: string
    courier_accept_at?: string
    picked_at?: string
    rider_delivered_at?: string
    user_delivered_at?: string
    auto_user_delivered_at?: string
    completed_at?: string                        // 完成时间
    cancelled_at?: string                        // 取消时间
    cancel_reason?: string                       // 取消原因
    updated_at: string                           // 更新时间
    badges?: Array<{
        text?: string
        type?: string
        locale?: string
    }>
}

export type MerchantVisibleOrderStatus = Exclude<OrderResponse['status'], 'pending'>
export type MerchantOrderStatusFilter = '' | MerchantVisibleOrderStatus

export function normalizeMerchantVisibleOrderStatusFilter(status?: OrderResponse['status'] | MerchantOrderStatusFilter): MerchantOrderStatusFilter {
    if (!status) {
        return ''
    }
    if (status === 'pending') {
        return 'paid'
    }
    return status
}

/**
 * 订单商品项响应 - 对齐 api.orderItemResponse
 */
export interface OrderItemResponse {
    combo_id?: number                            // 套餐ID（套餐订单时有值）
    customizations?: OrderItemCustomization[]    // 定制化选项列表
    dish_id?: number                             // 菜品ID（菜品订单时有值）
    id: number                                   // 订单明细ID
    name: string                                 // 商品名称
    quantity: number                             // 数量
    specs_text: string                           // 用户下单时选择的规格摘要，无规格为空字符串
    subtotal: number                             // 小计金额（分，含定制化加价）
    unit_price: number                           // 单价（分）
}

/**
 * 订单商品定制化 - 完全对齐 api.orderItemCustomization
 */
export interface OrderItemCustomization {
    group_id?: number                            // 定制分组ID
    option_id?: number                           // 定制选项ID
    tag_id?: number                              // 标签ID
    name: string                                 // 定制分组名称
    value: string                                // 定制选项名称
    extra_price: number                          // 价格调整（分）
}

/**
 * 订单统计响应 - 对齐 db.GetOrderStatsRow
 */
export interface OrderStatsResponse {
    pending_count: number                        // 待处理订单数
    paid_count: number                           // 已支付订单数
    preparing_count: number                      // 制作中订单数
    ready_count: number                          // 待取餐/待代取订单数
    delivering_count: number                     // 代取中订单数
    completed_count: number                      // 已完成订单数（含用户确认收货）
    cancelled_count: number                      // 已取消订单数
}

export interface MerchantOrderListResult {
    orders: OrderResponse[]
    total: number
    page_id: number
    page_size: number
}

export interface MerchantOrderSummaryResponse {
    total: number
    pending_count: number
    paid_count: number
    preparing_count: number
    ready_count: number
    courier_accepted_count: number
    picked_count: number
    delivering_count: number
    rider_delivered_count: number
    user_delivered_count: number
    completed_count: number
    cancelled_count: number
}

export interface MerchantOrderPrintJobResponse {
    id: number
    order_id: number
    printer_id: number
    printer_name: string
    status: string
    vendor_order_id?: string
    error_message?: string
    printed_at?: string
    created_at: string
}

export interface MerchantOrderPrintJobsResult {
    order_id: number
    items: MerchantOrderPrintJobResponse[]
}

export interface MerchantOrderPrintJobStatusResponse {
    print_log_id: number
    order_id: number
    printer_id: number
    printer_name: string
    local_status: string
    vendor_order_id?: string
    cloud_query_available: boolean
    cloud_printed?: boolean
    checked_at: string
}

export interface RetryMerchantOrderPrintJobResponse {
    message: string
    order_id: number
    print_log_id: number
    trigger: string
}

export interface PrintMerchantOrderResponse {
    message: string
    order_id: number
    trigger: string
}

export interface MerchantPrintAnomalyItem {
    print_log_id: number
    order_id: number
    order_no: string
    order_type: OrderResponse['order_type'] | string
    printer_id: number
    printer_name: string
    local_status: string
    error_message?: string
    vendor_order_id?: string
    last_attempt_at: string
    can_retry: boolean
    retry_hint?: string
}

export interface MerchantPrintAnomaliesResult {
    items: MerchantPrintAnomalyItem[]
    total: number
    page_id: number
    page_size: number
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
    pickup_code?: string                         // 取餐码
    pickup_number?: string                       // 取餐码兼容别名
    status: string                               // 订单状态
    order_status?: OrderResponse['status'] | string
    fulfillment_status?: OrderResponse['fulfillment_status'] | string
    kitchen_status?: string
    can_mark_ready?: boolean
    status_hint?: string
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
    category_name?: string                        // 商品分类名称
    customizations?: OrderItemCustomization[]    // 定制化选项
    id: number                                   // 商品项ID
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
        status?: MerchantVisibleOrderStatus        // 状态筛选，商户侧不可请求 pending
        order_type?: OrderResponse['order_type']   // 订单类型筛选
    }): Promise<MerchantOrderListResult> {
        const response = await request<OrderResponse[] | MerchantOrderListResult | { orders?: OrderResponse[], total?: number, page_id?: number, page_size?: number }>({
            url: '/v1/merchant/orders',
            method: 'GET',
            data: params
        })

        if (Array.isArray(response)) {
            return {
                orders: response,
                total: response.length,
                page_id: params.page_id,
                page_size: params.page_size
            }
        }

        return {
            orders: response.orders || [],
            total: response.total || 0,
            page_id: response.page_id || params.page_id,
            page_size: response.page_size || params.page_size
        }
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

    static async getOrderSummary(): Promise<MerchantOrderSummaryResponse> {
        return await request({
            url: '/v1/merchant/orders/summary',
            method: 'GET'
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

    /**
     * 获取订单打印任务列表
     * GET /v1/merchant/orders/{id}/print-jobs
     */
    static async listOrderPrintJobs(orderId: number): Promise<MerchantOrderPrintJobsResult> {
        return await request({
            url: `/v1/merchant/orders/${orderId}/print-jobs`,
            method: 'GET'
        })
    }

    /**
     * 重试订单打印任务
     * POST /v1/merchant/orders/{id}/print-jobs/{print_log_id}/retry
     */
    static async retryOrderPrintJob(orderId: number, printLogId: number): Promise<RetryMerchantOrderPrintJobResponse> {
        return await request({
            url: `/v1/merchant/orders/${orderId}/print-jobs/${printLogId}/retry`,
            method: 'POST'
        })
    }

    /**
     * 查询订单打印任务的云端执行状态
     * GET /v1/merchant/orders/{id}/print-jobs/{print_log_id}/status
     */
    static async getOrderPrintJobStatus(orderId: number, printLogId: number): Promise<MerchantOrderPrintJobStatusResponse> {
        return await request({
            url: `/v1/merchant/orders/${orderId}/print-jobs/${printLogId}/status`,
            method: 'GET'
        })
    }

    /**
     * 手动创建订单打印任务
     * POST /v1/merchant/orders/{id}/print-jobs
     */
    static async printOrder(orderId: number): Promise<PrintMerchantOrderResponse> {
        return await request({
            url: `/v1/merchant/orders/${orderId}/print-jobs`,
            method: 'POST'
        })
    }

    /**
     * 获取商户打印异常列表
     * GET /v1/merchant/orders/print-anomalies
     */
    static async listPrintAnomalies(params: {
        page_id: number
        page_size: number
        status?: 'failed' | 'pending'
    }): Promise<MerchantPrintAnomaliesResult> {
        return await request({
            url: '/v1/merchant/orders/print-anomalies',
            method: 'GET',
            data: params
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

    static isTerminalOrderStatus(status: OrderResponse['status'] | string): boolean {
        return status === 'completed' || status === 'cancelled'
    }

    /**
     * 格式化订单状态显示文本
     */
    static formatOrderStatus(status: string): string {
        const statusMap: Record<string, string> = {
            'pending': '待支付',
            'paid': '待接单',
            'preparing': '制作中',
            'ready': '待交付',
            'courier_accepted': '骑手已接单',
            'picked': '骑手已取餐',
            'delivering': '代取中',
            'rider_delivered': '骑手已送达',
            'user_delivered': '用户已确认',
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

    static getCustomerPayableAmount(order: OrderResponse): number {
        return order.fee_breakdown?.customer_payable_amount ?? order.total_amount
    }

    static getMerchantReceivableAmount(order: OrderResponse): number | null {
        return typeof order.fee_breakdown?.merchant_receivable_amount === 'number'
            ? order.fee_breakdown.merchant_receivable_amount
            : null
    }

    /**
     * @deprecated 商户侧金额真值来自后端 fee_breakdown；请使用 getCustomerPayableAmount。
     */
    static calculateActualAmount(order: OrderResponse): number {
        return this.getCustomerPayableAmount(order)
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

    static getMerchantOrderStatusHint(order: OrderResponse): string {
        switch (order.status) {
            case 'paid':
                return '顾客已支付，建议尽快接单或拒单处理'
            case 'preparing':
                return '商户正在制作中，可在出餐后标记完成'
            case 'ready':
                return order.order_type === 'takeout' ? '等待骑手取餐或系统分代取力' : '等待顾客取餐或到店核销'
            case 'courier_accepted':
                return '骑手已接单，正在到店取餐'
            case 'picked':
                return '骑手已取餐，订单即将代取'
            case 'delivering':
                return '代取途中，请关注异常和超时情况'
            case 'rider_delivered':
                return '骑手已送达，等待顾客确认'
            case 'user_delivered':
                return '顾客已确认收货，系统即将完成订单'
            case 'completed':
                return '订单已完成履约'
            case 'cancelled':
                return order.cancel_reason || '订单已取消'
            default:
                return ''
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
        return order.status === 'paid'
    }

    /**
     * 判断订单是否可以标记为准备完成
     */
    static canMarkReady(order: OrderResponse): boolean {
        return canMerchantMarkOrderReady(order)
    }

    /**
     * 判断订单是否可以完成
     */
    static canCompleteOrder(order: OrderResponse): boolean {
        return order.status === 'ready' && ['dine_in', 'takeaway'].includes(order.order_type)
    }

    static shouldShowPassiveState(order: OrderResponse): boolean {
        return !this.canAcceptOrder(order)
            && !this.canRejectOrder(order)
            && !this.canMarkReady(order)
            && !this.canCompleteOrder(order)
            && !this.isTerminalOrderStatus(order.status)
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
            'courier_accepted': '#2563eb',
            'picked': '#0f766e',
            'delivering': '#0284c7',
            'rider_delivered': '#0d9488',
            'user_delivered': '#16a34a',
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
