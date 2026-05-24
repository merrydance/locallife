/**
 * 骑手基础管理接口重构 (Task 3.1)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：骑手信息、上下线管理、位置上报
 */

import { request } from '../utils/request'
import { resolveCurrentRegionId } from '../utils/current-region'

// ==================== 数据类型定义 ====================

/** 骑手状态枚举 */
export type RiderStatus = 'pending' | 'approved' | 'active' | 'suspended' | 'rejected'

/** 在线状态枚举 */
export type OnlineStatus = 'offline' | 'online' | 'delivering'

// ==================== 骑手基础信息相关类型 ====================

/** 骑手响应 - 基于swagger api.riderResponse */
export interface RiderResponse {
    id: number
    user_id: number
    region_id: number
    real_name: string
    phone: string
    status: RiderStatus
    is_online: boolean
    current_latitude?: number
    current_longitude?: number
    location_updated_at?: string
    deposit_amount: number
    frozen_deposit: number
    credit_score: number
    total_orders: number
    total_earnings: number
    online_duration: number
    created_at: string
}

/** 骑手状态响应 - 基于swagger api.riderStatusResponse */
export interface RiderStatusResponse {
    is_online: boolean
    online_status: OnlineStatus
    status: RiderStatus
    current_region_id: number
    required_deposit: number
    current_latitude?: number
    current_longitude?: number
    location_updated_at?: string
    active_deliveries: number
    can_go_online: boolean
    can_go_offline: boolean
    online_block_reason?: string
    settlement_account?: BaofuSettlementReadiness
}

export interface BaofuSettlementReadiness {
    state: string
    label: string
    payment_ready: boolean
}

// ==================== 位置管理相关类型 ====================

/** 位置点 - 基于swagger api.locationPoint */
export interface LocationPoint {
    latitude: number
    longitude: number
    recorded_at: string
    accuracy?: number
    speed?: number
    heading?: number
}

/** 更新位置请求 - 基于swagger api.updateLocationRequest */
export interface UpdateLocationRequest extends Record<string, unknown> {
    region_id: number
    locations: LocationPoint[]
}

/** 位置更新响应 */
export interface LocationUpdateResponse {
    count: number
    latitude: number
    longitude: number
    message: string
}

// ==================== 骑手基础管理服务类 ====================

/**
 * 骑手基础管理服务
 * 提供骑手信息查询、状态管理、位置上报等功能
 */
export class RiderBasicManagementService {
    /**
     * 获取当前骑手信息
     */
    async getRiderInfo(): Promise<RiderResponse> {
        return request({
            url: '/v1/rider/me',
            method: 'GET'
        })
    }

    /**
     * 获取骑手状态
     */
    async getRiderStatus(): Promise<RiderStatusResponse> {
        return request({
            url: '/v1/rider/status',
            method: 'GET'
        })
    }

    /**
     * 骑手上线
     */
    async syncCurrentRegion(regionId: number): Promise<RiderResponse> {
        return request({
            url: '/v1/rider/current-region',
            method: 'PATCH',
            data: { region_id: regionId }
        })
    }

    async goOnline(regionId: number): Promise<RiderResponse> {
        return request({
            url: '/v1/rider/online',
            method: 'POST',
            data: { region_id: regionId }
        })
    }

    /**
     * 骑手下线
     */
    async goOffline(): Promise<RiderResponse> {
        return request({
            url: '/v1/rider/offline',
            method: 'POST'
        })
    }

    /**
     * 更新骑手位置
     * @param locationData 位置数据
     */
    async updateLocation(locationData: UpdateLocationRequest): Promise<LocationUpdateResponse> {
        return request({
            url: '/v1/rider/location',
            method: 'POST',
            data: locationData
        })
    }

}

// ==================== 位置管理服务类 ====================

/**
 * 位置管理服务
 * 提供位置上报、轨迹管理等功能
 */
export class LocationManagementService {
    /**
     * 批量上报位置点
     * @param locations 位置点数组
     */
    async batchUpdateLocation(regionId: number, locations: LocationPoint[]): Promise<LocationUpdateResponse> {
        return request({
            url: '/v1/rider/location',
            method: 'POST',
            data: { region_id: regionId, locations }
        })
    }

    /**
     * 单点位置上报
     * @param location 单个位置点
     */
    async updateSingleLocation(location: LocationPoint): Promise<LocationUpdateResponse> {
        const regionId = await resolveCurrentRegionId()
        return this.batchUpdateLocation(regionId, [location])
    }

    /**
     * 获取当前位置
     */
    async getCurrentLocation(): Promise<{
        latitude: number
        longitude: number
        updated_at: string
    }> {
        const riderInfo = await new RiderBasicManagementService().getRiderInfo()
        return {
            latitude: riderInfo.current_latitude || 0,
            longitude: riderInfo.current_longitude || 0,
            updated_at: riderInfo.location_updated_at || ''
        }
    }
}

// ==================== 数据适配器 ====================

/**
 * 骑手基础管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class RiderBasicManagementAdapter {
    /**
     * 适配骑手响应数据
     */
    static adaptRiderResponse(data: RiderResponse): {
        id: number
        userId: number
        realName: string
        phone: string
        status: RiderStatus
        isOnline: boolean
        currentLatitude?: number
        currentLongitude?: number
        locationUpdatedAt?: string
        depositAmount: number
        frozenDeposit: number
        creditScore: number
        totalOrders: number
        totalEarnings: number
        onlineDuration: number
        createdAt: string
    } {
        return {
            id: data.id,
            userId: data.user_id,
            realName: data.real_name,
            phone: data.phone,
            status: data.status,
            isOnline: data.is_online,
            currentLatitude: data.current_latitude,
            currentLongitude: data.current_longitude,
            locationUpdatedAt: data.location_updated_at,
            depositAmount: data.deposit_amount,
            frozenDeposit: data.frozen_deposit,
            creditScore: data.credit_score,
            totalOrders: data.total_orders,
            totalEarnings: data.total_earnings,
            onlineDuration: data.online_duration,
            createdAt: data.created_at
        }
    }

    /**
     * 适配骑手状态响应数据
     */
    static adaptRiderStatusResponse(data: RiderStatusResponse): {
        isOnline: boolean
        onlineStatus: OnlineStatus
        status: RiderStatus
        currentLatitude?: number
        currentLongitude?: number
        locationUpdatedAt?: string
        activeDeliveries: number
        canGoOnline: boolean
        canGoOffline: boolean
        onlineBlockReason?: string
        settlementAccount?: BaofuSettlementReadiness
    } {
        return {
            isOnline: data.is_online,
            onlineStatus: data.online_status,
            status: data.status,
            currentLatitude: data.current_latitude,
            currentLongitude: data.current_longitude,
            locationUpdatedAt: data.location_updated_at,
            activeDeliveries: data.active_deliveries,
            canGoOnline: data.can_go_online,
            canGoOffline: data.can_go_offline,
            onlineBlockReason: data.online_block_reason,
            settlementAccount: data.settlement_account
        }
    }

    /**
     * 适配位置点数据
     */
    static adaptLocationPoint(data: {
        latitude: number
        longitude: number
        recordedAt: string
        accuracy?: number
        speed?: number
        heading?: number
    }): LocationPoint {
        return {
            latitude: data.latitude,
            longitude: data.longitude,
            recorded_at: data.recordedAt,
            accuracy: data.accuracy,
            speed: data.speed,
            heading: data.heading
        }
    }

}

// ==================== 导出服务实例 ====================

export const riderBasicManagementService = new RiderBasicManagementService()
export const locationManagementService = new LocationManagementService()

// ==================== 便捷函数 ====================

/**
 * 获取骑手工作台数据
 */
export async function getRiderDashboard(): Promise<{
    riderInfo: RiderResponse
    riderStatus: RiderStatusResponse
    todayStats: {
        onlineDuration: number
        completedOrders: number
        earnings: number
    }
}> {
    const [riderInfo, riderStatus] = await Promise.all([
        riderBasicManagementService.getRiderInfo(),
        riderBasicManagementService.getRiderStatus()
    ])

    // 今日统计数据需要根据实际接口调整
    const todayStats = {
        onlineDuration: riderInfo.online_duration,
        completedOrders: riderInfo.total_orders,
        earnings: riderInfo.total_earnings
    }

    return {
        riderInfo,
        riderStatus,
        todayStats
    }
}

/**
 * 智能上下线管理
 * @param action 操作类型
 */
export async function smartOnlineManagement(action: 'online' | 'offline'): Promise<{
    success: boolean
    message: string
    riderInfo?: RiderResponse
}> {
    try {
        const status = await riderBasicManagementService.getRiderStatus()

        if (action === 'online') {
            if (!status.can_go_online) {
                return {
                    success: false,
                    message: status.online_block_reason || '当前无法上线'
                }
            }

            const regionId = await resolveCurrentRegionId()
            const riderInfo = await riderBasicManagementService.goOnline(regionId)
            return {
                success: true,
                message: '上线成功',
                riderInfo
            }
        } else {
            if (!status.can_go_offline) {
                return {
                    success: false,
                    message: status.active_deliveries > 0 ? '有代取中的订单，无法下线' : '当前无法下线'
                }
            }

            const riderInfo = await riderBasicManagementService.goOffline()
            return {
                success: true,
                message: '下线成功',
                riderInfo
            }
        }
    } catch (error: unknown) {
        return {
            success: false,
            message: error instanceof Error ? error.message : `${action === 'online' ? '上线' : '下线'}失败`
        }
    }
}

/**
 * 位置上报管理器
 */
export class LocationReportManager {
    private reportInterval: number = 30000 // 30秒上报一次
    private intervalId: ReturnType<typeof setInterval> | null = null
    private lastLocation: LocationPoint | null = null

    /**
     * 开始自动位置上报
     * @param interval 上报间隔（毫秒）
     */
    startAutoReport(interval: number = 30000): void {
        this.reportInterval = interval
        this.stopAutoReport() // 先停止之前的定时器

        this.intervalId = setInterval(async () => {
            try {
                await this.reportCurrentLocation()
            } catch (error) {
                console.error('位置上报失败:', error)
            }
        }, this.reportInterval)
    }

    /**
     * 停止自动位置上报
     */
    stopAutoReport(): void {
        if (this.intervalId) {
            clearInterval(this.intervalId)
            this.intervalId = null
        }
    }

    /**
     * 上报当前位置
     */
    async reportCurrentLocation(): Promise<LocationUpdateResponse | null> {
        try {
            // 获取当前位置（这里需要调用微信小程序的位置API）
            const location = await this.getCurrentPosition()

            if (location) {
                const result = await locationManagementService.updateSingleLocation(location)
                this.lastLocation = location
                return result
            }
        } catch (error) {
            console.error('获取位置失败:', error)
        }
        return null
    }

    /**
     * 获取当前GPS位置
     */
    private async getCurrentPosition(): Promise<LocationPoint | null> {
        return new Promise((resolve) => {
            // 微信小程序获取位置
            wx.getLocation({
                type: 'gcj02',
                success: (res) => {
                    const heading = (res as unknown as { heading?: number }).heading
                    resolve({
                        latitude: res.latitude,
                        longitude: res.longitude,
                        recorded_at: new Date().toISOString(),
                        accuracy: res.accuracy,
                        speed: res.speed,
                        heading
                    })
                },
                fail: () => {
                    resolve(null)
                }
            })
        })
    }

    /**
     * 获取最后上报的位置
     */
    getLastLocation(): LocationPoint | null {
        return this.lastLocation
    }
}

/**
 * 格式化骑手状态显示
 * @param status 骑手状态
 */
export function formatRiderStatus(status: RiderStatus): string {
    const statusMap: Record<RiderStatus, string> = {
        pending: '待审核',
        approved: '已通过',
        active: '正常',
        suspended: '已暂停',
        rejected: '已拒绝'
    }
    return statusMap[status] || status
}

/**
 * 格式化在线状态显示
 * @param onlineStatus 在线状态
 */
export function formatOnlineStatus(onlineStatus: OnlineStatus): string {
    const statusMap: Record<OnlineStatus, string> = {
        offline: '离线',
        online: '在线',
        delivering: '代取中'
    }
    return statusMap[onlineStatus] || onlineStatus
}

/**
 * 计算在线时长显示
 * @param duration 在线时长（秒）
 */
export function formatOnlineDuration(duration: number): string {
    const hours = Math.floor(duration / 3600)
    const minutes = Math.floor((duration % 3600) / 60)

    if (hours > 0) {
        return `${hours}小时${minutes}分钟`
    } else {
        return `${minutes}分钟`
    }
}

/**
 * 格式化收入显示
 * @param amount 金额（分）
 * @param showUnit 是否显示单位
 */
export function formatEarnings(amount: number, showUnit: boolean = true): string {
    const yuan = (amount / 100).toFixed(2)
    return showUnit ? `¥${yuan}` : yuan
}

/**
 * 验证位置数据
 * @param location 位置数据
 */
export function validateLocationPoint(location: LocationPoint): { valid: boolean, message?: string } {
    if (!location.latitude || !location.longitude) {
        return { valid: false, message: '经纬度不能为空' }
    }

    if (location.latitude < -90 || location.latitude > 90) {
        return { valid: false, message: '纬度范围应在-90到90之间' }
    }

    if (location.longitude < -180 || location.longitude > 180) {
        return { valid: false, message: '经度范围应在-180到180之间' }
    }

    if (location.accuracy && (location.accuracy < 0 || location.accuracy > 1000)) {
        return { valid: false, message: 'GPS精度应在0到1000米之间' }
    }

    return { valid: true }
}
