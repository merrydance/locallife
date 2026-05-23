/**
 * 配送任务管理接口重构 (Task 3.2)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：任务获取、配送流程、任务历史
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 配送状态枚举 */
export type DeliveryStatus = 'assigned' | 'picked_up' | 'delivering' | 'delivered' | 'completed' | 'cancelled'

// ==================== 推荐订单相关类型 ====================

/** 推荐订单响应 - 基于swagger api.recommendedOrderResponse */
export interface RecommendedOrderResponse {
    order_id: number
    merchant_id: number
    delivery_fee: number
    pickup_latitude: number
    pickup_longitude: number
    delivery_latitude: number
    delivery_longitude: number
    distance: number
    distance_to_pickup: number
    real_distance: number
    real_duration: number
    estimated_minutes: number
    expires_at: string
    total_score: number
    distance_score: number
    profit_score: number
    route_score: number
    urgency_score: number
}

/** 推荐订单查询参数 */
export interface RecommendOrdersParams extends Record<string, unknown> {
    latitude: number
    longitude: number
}

// ==================== 配送任务相关类型 ====================

/** 配送响应 - 基于swagger api.deliveryResponse */
export interface DeliveryResponse {
    id: number
    order_id: number
    rider_id: number
    status: DeliveryStatus
    delivery_fee: number
    rider_earnings: number
    rider_gross_amount?: number
    rider_payment_fee?: number
    rider_net_earnings?: number
    profit_sharing_order_id?: number
    profit_sharing_status?: string
    distance: number
    pickup_address: string
    pickup_contact: string
    pickup_phone: string
    pickup_latitude: number
    pickup_longitude: number
    delivery_address: string
    delivery_contact: string
    delivery_phone: string
    delivery_latitude: number
    delivery_longitude: number
    estimated_pickup_at: string
    estimated_delivery_at: string
    created_at: string
    assigned_at?: string
    picked_at?: string
    delivered_at?: string
    completed_at?: string
}

/** 配送历史查询参数 */
export interface DeliveryHistoryParams extends Record<string, unknown> {
    page_id: number
    page_size: number
    start_date?: string
    end_date?: string
    status?: DeliveryStatus
}

/** 配送操作请求 */
export interface DeliveryActionRequest extends Record<string, unknown> {
    latitude?: number
    longitude?: number
    notes?: string
    photos?: string[]
}

// ==================== 配送任务管理服务类 ====================

/**
 * 配送任务管理服务
 * 提供任务获取、配送流程管理等功能
 */
export class DeliveryTaskManagementService {
    /**
     * 获取推荐订单列表
     * @param params 查询参数
     */
    async getRecommendedOrders(params: RecommendOrdersParams): Promise<RecommendedOrderResponse[]> {
        return request({
            url: '/v1/delivery/recommend',
            method: 'GET',
            data: params
        })
    }

    /**
     * 抢单
     * @param orderId 订单ID
     */
    async grabOrder(orderId: number): Promise<DeliveryResponse> {
        return request({
            url: `/v1/delivery/grab/${orderId}`,
            method: 'POST'
        })
    }

    /**
     * 获取当前活跃配送任务
     */
    async getActiveDeliveries(): Promise<DeliveryResponse[]> {
        return request({
            url: '/v1/delivery/active',
            method: 'GET'
        })
    }

    /**
     * 获取配送历史
     * @param params 查询参数
     */
    async getDeliveryHistory(params: DeliveryHistoryParams): Promise<{
        deliveries: DeliveryResponse[]
        total: number
        page_id: number
        page_size: number
        has_more: boolean
    }> {
        return request({
            url: '/v1/delivery/history',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取配送任务详情
     * @param deliveryId 配送任务ID
     */
    async getDeliveryDetail(deliveryId: number): Promise<DeliveryResponse> {
        return request({
            url: `/v1/delivery/${deliveryId}`,
            method: 'GET'
        })
    }

    /**
     * 开始取餐
     * @param deliveryId 配送任务ID
     * @param actionData 操作数据
     */
    async startPickup(deliveryId: number, actionData?: DeliveryActionRequest): Promise<DeliveryResponse> {
        return request({
            url: `/v1/delivery/${deliveryId}/start-pickup`,
            method: 'POST',
            data: actionData || {}
        })
    }

    /**
     * 确认取餐完成
     * @param deliveryId 配送任务ID
     * @param actionData 操作数据
     */
    async confirmPickup(deliveryId: number, actionData?: DeliveryActionRequest): Promise<DeliveryResponse> {
        return request({
            url: `/v1/delivery/${deliveryId}/confirm-pickup`,
            method: 'POST',
            data: actionData || {}
        })
    }

    /**
     * 开始配送
     * @param deliveryId 配送任务ID
     * @param actionData 操作数据
     */
    async startDelivery(deliveryId: number, actionData?: DeliveryActionRequest): Promise<DeliveryResponse> {
        return request({
            url: `/v1/delivery/${deliveryId}/start-delivery`,
            method: 'POST',
            data: actionData || {}
        })
    }

    /**
     * 确认送达
     * @param deliveryId 配送任务ID
     * @param actionData 操作数据
     */
    async confirmDelivery(deliveryId: number, actionData?: DeliveryActionRequest): Promise<DeliveryResponse> {
        return request({
            url: `/v1/delivery/${deliveryId}/confirm-delivery`,
            method: 'POST',
            data: actionData || {}
        })
    }
}

// ==================== 配送流程管理服务类 ====================

/**
 * 配送流程管理服务
 * 提供配送流程的状态管理和操作指导
 */
export class DeliveryProcessService {
    /**
     * 获取配送流程的下一步操作
     * @param delivery 配送任务信息
     */
    getNextAction(delivery: DeliveryResponse): {
        action: string
        actionText: string
        canExecute: boolean
        reason?: string
    } {
        switch (delivery.status) {
            case 'assigned':
                return {
                    action: 'start_pickup',
                    actionText: '开始取餐',
                    canExecute: true
                }
            case 'picked_up':
                return {
                    action: 'confirm_pickup',
                    actionText: '确认取餐',
                    canExecute: true
                }
            case 'delivering':
                return {
                    action: 'start_delivery',
                    actionText: '开始配送',
                    canExecute: true
                }
            case 'delivered':
                return {
                    action: 'confirm_delivery',
                    actionText: '确认送达',
                    canExecute: true
                }
            case 'completed':
                return {
                    action: 'none',
                    actionText: '已完成',
                    canExecute: false,
                    reason: '配送任务已完成'
                }
            case 'cancelled':
                return {
                    action: 'none',
                    actionText: '已取消',
                    canExecute: false,
                    reason: '配送任务已取消'
                }
            default:
                return {
                    action: 'unknown',
                    actionText: '未知状态',
                    canExecute: false,
                    reason: '未知的配送状态'
                }
        }
    }

    /**
     * 执行配送流程操作
     * @param delivery 配送任务信息
     * @param actionData 操作数据
     */
    async executeDeliveryAction(
        delivery: DeliveryResponse,
        actionData?: DeliveryActionRequest
    ): Promise<{ success: boolean, message: string, updatedDelivery?: DeliveryResponse }> {
        const nextAction = this.getNextAction(delivery)

        if (!nextAction.canExecute) {
            return {
                success: false,
                message: nextAction.reason || '无法执行操作'
            }
        }

        try {
            const service = new DeliveryTaskManagementService()
            let updatedDelivery: DeliveryResponse

            switch (nextAction.action) {
                case 'start_pickup':
                    updatedDelivery = await service.startPickup(delivery.id, actionData)
                    break
                case 'confirm_pickup':
                    updatedDelivery = await service.confirmPickup(delivery.id, actionData)
                    break
                case 'start_delivery':
                    updatedDelivery = await service.startDelivery(delivery.id, actionData)
                    break
                case 'confirm_delivery':
                    updatedDelivery = await service.confirmDelivery(delivery.id, actionData)
                    break
                default:
                    return {
                        success: false,
                        message: '未知的操作类型'
                    }
            }

            return {
                success: true,
                message: `${nextAction.actionText}成功`,
                updatedDelivery
            }
        } catch (error: unknown) {
            return {
                success: false,
                message: error instanceof Error ? error.message : `${nextAction.actionText}失败`
            }
        }
    }
}

// ==================== 数据适配器 ====================

// ==================== 导出服务实例 ====================

export const deliveryTaskManagementService = new DeliveryTaskManagementService()
export const deliveryProcessService = new DeliveryProcessService()

// ==================== 便捷函数 ====================

/**
 * 获取骑手配送工作台数据
 * @param latitude 当前纬度
 * @param longitude 当前经度
 */
export async function getRiderDeliveryDashboard(latitude: number, longitude: number): Promise<{
    recommendedOrders: RecommendedOrderResponse[]
    activeDeliveries: DeliveryResponse[]
    todayStats: {
        completedDeliveries: number
        totalEarnings: number
        totalDistance: number
        avgDeliveryTime: number
    }
}> {
    const [recommendedOrders, activeDeliveries] = await Promise.all([
        deliveryTaskManagementService.getRecommendedOrders({ latitude, longitude }),
        deliveryTaskManagementService.getActiveDeliveries()
    ])

    // 今日统计数据需要根据实际接口调整
    const todayStats = {
        completedDeliveries: 0,
        totalEarnings: 0,
        totalDistance: 0,
        avgDeliveryTime: 0
    }

    return {
        recommendedOrders,
        activeDeliveries,
        todayStats
    }
}

/**
 * 格式化距离显示
 * @param distance 距离（米）
 */
export function formatDistance(distance: number): string {
    if (distance < 1000) {
        return `${distance}m`
    } else {
        return `${(distance / 1000).toFixed(1)}km`
    }
}

/**
 * 格式化配送费显示
 * @param fee 配送费（分）
 * @param showUnit 是否显示单位
 */
export function formatDeliveryFee(fee: number, showUnit: boolean = true): string {
    const yuan = (fee / 100).toFixed(2)
    return showUnit ? `¥${yuan}` : yuan
}
