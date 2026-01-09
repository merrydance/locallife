/**
 * 骑手配送管理接口
 * 基于swagger.json完全重构，包含配送任务、异常处理、财务管理等
 */

import { request } from '../utils/request'

// ==================== 配送任务数据类型定义 ====================

/**
 * 配送任务响应
 */
export interface DeliveryTaskResponse {
    delivery_id: number                          // 配送ID
    order_id: number                             // 订单ID
    order_no: string                             // 订单编号
    merchant_id: number                          // 商户ID
    merchant_name: string                        // 商户名称
    merchant_address: string                     // 商户地址
    merchant_latitude?: number                   // 商户纬度
    merchant_longitude?: number                  // 商户经度
    customer_name: string                        // 顾客姓名
    customer_phone: string                       // 顾客电话
    customer_address: string                     // 顾客地址
    customer_latitude?: number                   // 顾客纬度
    customer_longitude?: number                  // 顾客经度
    delivery_fee: number                         // 配送费（分）
    distance: number                             // 配送距离（米）
    estimated_time: number                       // 预计配送时间（分钟）
    status: string                               // 配送状态
    pickup_time?: string                         // 取餐时间
    delivery_time?: string                       // 送达时间
    created_at: string                           // 创建时间
}

/**
 * 推荐配送任务响应
 */
export interface RecommendedDeliveryResponse {
    tasks: DeliveryTaskResponse[]                // 推荐任务列表
    total: number                                // 总数
}

/**
 * 配送历史响应
 */
export interface DeliveryHistoryResponse {
    deliveries: DeliveryTaskResponse[]           // 配送历史列表
    total: number                                // 总数
    total_earnings: number                       // 总收入（分）
}

/**
 * 骑手位置响应
 */
export interface RiderLocationResponse {
    latitude: number                             // 纬度
    longitude: number                            // 经度
    updated_at: string                           // 更新时间
}

// ==================== 骑手信息数据类型定义 ====================

/**
 * 骑手信息响应
 */
export interface RiderInfoResponse {
    id: number                                   // 骑手ID
    name: string                                 // 骑手姓名
    phone: string                                // 手机号
    status: 'online' | 'offline' | 'busy'        // 在线状态
    score: number                                // 信用分
    total_deliveries: number                     // 总配送单数
    success_rate: number                         // 成功率
    avg_rating: number                           // 平均评分
    balance: number                              // 余额（分）
    created_at: string                           // 注册时间
}

/**
 * 骑手状态响应 - 对齐 api.riderStatusResponse
 */
export interface RiderStatusResponse {
    active_deliveries: number                    // 当前配送中订单数量
    can_go_offline: boolean                      // 是否可以下线
    can_go_online: boolean                       // 是否可以上线
    current_latitude: number                     // 当前纬度
    current_longitude: number                    // 当前经度
    is_online: boolean                           // 是否在线
    location_updated_at: string                  // 位置更新时间
    online_block_reason?: string                 // 不能上线的原因
    online_status: string                        // 在线状态描述
    status: string                               // 账号状态
}

/**
 * 骑手信用分响应
 */
export interface RiderScoreResponse {
    score: number                                // 当前信用分
    level: string                                // 信用等级
    total_deliveries: number                     // 总配送单数
    success_rate: number                         // 成功率
    complaint_count: number                      // 投诉次数
}

/**
 * 信用分历史响应
 */
export interface ScoreHistoryResponse {
    id: number                                   // 记录ID
    score_change: number                         // 分数变化
    reason: string                               // 变化原因
    created_at: string                           // 创建时间
}

// ==================== 异常处理数据类型定义 ====================

/**
 * 异常上报请求
 */
export interface ReportExceptionRequest extends Record<string, unknown> {
    exception_type: string                       // 异常类型
    description: string                          // 异常描述
    images?: string[]                            // 图片证据
}

/**
 * 延迟上报请求
 */
export interface ReportDelayRequest extends Record<string, unknown> {
    delay_reason: string                         // 延迟原因
    estimated_delay: number                      // 预计延迟时间（分钟）
}

// ==================== 财务管理数据类型定义 ====================

/**
 * 保证金响应 - 对齐 api.depositResponse
 */
export interface DepositResponse {
    amount: number                               // 金额（分）
    balance_after: number                        // 操作后余额（分）
    created_at: string                           // 创建时间
    id: number                                   // 记录ID
    remark?: string                              // 备注
    rider_id: number                             // 骑手ID
    type: string                                 // 类型
}

/**
 * 保证金记录响应
 */
export interface DepositRecordResponse {
    id: number                                   // 记录ID
    type: 'deposit' | 'refund' | 'deduction'     // 类型
    amount: number                               // 金额（分）
    reason?: string                              // 原因
    created_at: string                           // 创建时间
}

/**
 * 提现请求 - 对齐 api.withdrawRequest
 */
export interface WithdrawRequest extends Record<string, unknown> {
    amount: number                               // 提现金额（分）
    remark?: string                              // 备注
}

// ==================== 配送任务管理服务 ====================

/**
 * 配送任务管理服务
 */
export class DeliveryTaskService {

    /**
     * 获取推荐配送任务
     * GET /v1/delivery/recommend
     */
    static async getRecommendedTasks(): Promise<RecommendedDeliveryResponse> {
        return await request({
            url: '/v1/delivery/recommend',
            method: 'GET'
        })
    }

    /**
     * 抢单
     * POST /v1/delivery/grab/:order_id
     */
    static async grabOrder(orderId: number): Promise<DeliveryTaskResponse> {
        return await request({
            url: `/v1/delivery/grab/${orderId}`,
            method: 'POST'
        })
    }

    /**
     * 获取当前配送任务
     * GET /v1/delivery/active
     */
    static async getActiveTasks(): Promise<DeliveryTaskResponse[]> {
        return await request({
            url: '/v1/delivery/active',
            method: 'GET'
        })
    }

    /**
     * 获取配送历史
     * GET /v1/delivery/history
     */
    static async getDeliveryHistory(params: {
        page_id: number                            // 页码
        page_size: number                          // 每页数量
        start_date?: string                        // 开始日期
        end_date?: string                          // 结束日期
    }): Promise<DeliveryHistoryResponse> {
        return await request({
            url: '/v1/delivery/history',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取订单详情
     * GET /v1/delivery/order/:order_id
     */
    static async getOrderDetail(orderId: number): Promise<DeliveryTaskResponse> {
        return await request({
            url: `/v1/delivery/order/${orderId}`,
            method: 'GET'
        })
    }

    /**
     * 开始取餐
     * POST /v1/delivery/:delivery_id/start-pickup
     */
    static async startPickup(deliveryId: number): Promise<DeliveryTaskResponse> {
        return await request({
            url: `/v1/delivery/${deliveryId}/start-pickup`,
            method: 'POST'
        })
    }

    /**
     * 确认取餐
     * POST /v1/delivery/:delivery_id/confirm-pickup
     */
    static async confirmPickup(deliveryId: number): Promise<DeliveryTaskResponse> {
        return await request({
            url: `/v1/delivery/${deliveryId}/confirm-pickup`,
            method: 'POST'
        })
    }

    /**
     * 开始配送
     * POST /v1/delivery/:delivery_id/start-delivery
     */
    static async startDelivery(deliveryId: number): Promise<DeliveryTaskResponse> {
        return await request({
            url: `/v1/delivery/${deliveryId}/start-delivery`,
            method: 'POST'
        })
    }

    /**
     * 确认送达
     * POST /v1/delivery/:delivery_id/confirm-delivery
     */
    static async confirmDelivery(deliveryId: number): Promise<DeliveryTaskResponse> {
        return await request({
            url: `/v1/delivery/${deliveryId}/confirm-delivery`,
            method: 'POST'
        })
    }

    /**
     * 获取骑手位置
     * GET /v1/delivery/:delivery_id/rider-location
     */
    static async getRiderLocation(deliveryId: number): Promise<RiderLocationResponse> {
        return await request({
            url: `/v1/delivery/${deliveryId}/rider-location`,
            method: 'GET'
        })
    }
}

// ==================== 骑手信息管理服务 ====================

/**
 * 骑手信息管理服务
 */
export class RiderInfoService {

    /**
     * 获取骑手信息
     * GET /v1/rider/me
     */
    static async getRiderInfo(): Promise<RiderInfoResponse> {
        return await request({
            url: '/v1/rider/me',
            method: 'GET'
        })
    }

    /**
     * 获取骑手状态
     * GET /v1/rider/status
     */
    static async getRiderStatus(): Promise<RiderStatusResponse> {
        return await request({
            url: '/v1/rider/status',
            method: 'GET'
        })
    }

    /**
     * 上线
     * POST /v1/rider/online
     */
    static async goOnline(): Promise<void> {
        return await request({
            url: '/v1/rider/online',
            method: 'POST'
        })
    }

    /**
     * 下线
     * POST /v1/rider/offline
     */
    static async goOffline(): Promise<void> {
        return await request({
            url: '/v1/rider/offline',
            method: 'POST'
        })
    }

    /**
     * 上报位置
     * POST /v1/rider/location
     */
    static async reportLocation(data: {
        latitude: number                           // 纬度
        longitude: number                          // 经度
    }): Promise<void> {
        return await request({
            url: '/v1/rider/location',
            method: 'POST',
            data
        })
    }

    /**
     * 获取信用分
     * GET /v1/rider/score
     */
    static async getScore(): Promise<RiderScoreResponse> {
        return await request({
            url: '/v1/rider/score',
            method: 'GET'
        })
    }

    /**
     * 获取信用分历史
     * GET /v1/rider/score/history
     */
    static async getScoreHistory(params: {
        page_id: number                            // 页码
        page_size: number                          // 每页数量
    }): Promise<ScoreHistoryResponse[]> {
        return await request({
            url: '/v1/rider/score/history',
            method: 'GET',
            data: params
        })
    }
}

// ==================== 异常处理服务 ====================

/**
 * 异常处理服务
 */
export class ExceptionHandlingService {

    /**
     * 上报异常
     * POST /rider/orders/{id}/exception
     */
    static async reportException(orderId: number, data: ReportExceptionRequest): Promise<void> {
        return await request({
            url: `/rider/orders/${orderId}/exception`,
            method: 'POST',
            data
        })
    }

    /**
     * 上报延迟
     * POST /rider/orders/{id}/delay
     */
    static async reportDelay(orderId: number, data: ReportDelayRequest): Promise<void> {
        return await request({
            url: `/rider/orders/${orderId}/delay`,
            method: 'POST',
            data
        })
    }
}

// ==================== 财务管理服务 ====================

/**
 * 财务管理服务
 */
export class RiderFinanceService {

    /**
     * 获取保证金信息
     * GET /v1/rider/deposit
     */
    static async getDeposit(): Promise<DepositResponse> {
        return await request({
            url: '/v1/rider/deposit',
            method: 'GET'
        })
    }

    /**
     * 获取保证金记录
     * GET /v1/rider/deposits
     */
    static async getDepositRecords(params: {
        page_id: number                            // 页码
        page_size: number                          // 每页数量
    }): Promise<DepositRecordResponse[]> {
        return await request({
            url: '/v1/rider/deposits',
            method: 'GET',
            data: params
        })
    }

    /**
     * 提现
     * POST /v1/rider/withdraw
     */
    static async withdraw(data: WithdrawRequest): Promise<void> {
        return await request({
            url: '/v1/rider/withdraw',
            method: 'POST',
            data
        })
    }
}

// ==================== 配送管理适配器 ====================

/**
 * 配送管理数据适配器
 */
export class DeliveryAdapter {

    /**
     * 格式化金额显示（分转元）
     */
    static formatAmount(amountInCents: number): string {
        return (amountInCents / 100).toFixed(2)
    }

    /**
     * 格式化距离显示（统一中文格式：米/公里）
     */
    static formatDistance(distanceInMeters: number): string {
        if (distanceInMeters < 1000) {
            return `${Math.round(distanceInMeters)}米`
        } else {
            return `${(distanceInMeters / 1000).toFixed(1)}公里`
        }
    }

    /**
     * 格式化配送状态
     */
    static formatDeliveryStatus(status: string): string {
        const statusMap: Record<string, string> = {
            'pending': '待接单',
            'accepted': '已接单',
            'picking_up': '取餐中',
            'picked_up': '已取餐',
            'delivering': '配送中',
            'delivered': '已送达',
            'cancelled': '已取消'
        }
        return statusMap[status] || status
    }

    /**
     * 获取配送状态颜色
     */
    static getStatusColor(status: string): string {
        const colorMap: Record<string, string> = {
            'pending': '#f39c12',
            'accepted': '#3498db',
            'picking_up': '#e74c3c',
            'picked_up': '#9b59b6',
            'delivering': '#e67e22',
            'delivered': '#27ae60',
            'cancelled': '#95a5a6'
        }
        return colorMap[status] || '#95a5a6'
    }

    /**
     * 格式化骑手状态
     */
    static formatRiderStatus(status: string): string {
        const statusMap: Record<string, string> = {
            'online': '在线',
            'offline': '离线',
            'busy': '忙碌中'
        }
        return statusMap[status] || status
    }

    /**
     * 获取骑手状态颜色
     */
    static getRiderStatusColor(status: string): string {
        const colorMap: Record<string, string> = {
            'online': '#52c41a',
            'offline': '#999',
            'busy': '#fa8c16'
        }
        return colorMap[status] || '#999'
    }

    /**
     * 计算预计送达时间
     */
    static calculateEstimatedArrival(createdAt: string, estimatedTime: number): string {
        const created = new Date(createdAt)
        const arrival = new Date(created.getTime() + estimatedTime * 60 * 1000)
        const hours = ('0' + arrival.getHours()).slice(-2)
        const minutes = ('0' + arrival.getMinutes()).slice(-2)
        return `${hours}:${minutes}`
    }
}

// ==================== 导出默认服务 ====================

export default {
    DeliveryTaskService,
    RiderInfoService,
    ExceptionHandlingService,
    RiderFinanceService,
    DeliveryAdapter
}
