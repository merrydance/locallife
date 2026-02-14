import { request } from '../utils/request'

export interface RiderInfo {
    id: number
    user_id: number
    real_name: string
    phone: string
    deposit_amount: number
    frozen_deposit: number
    status: 'pending' | 'approved' | 'active' | 'suspended' | 'rejected'
    is_online: boolean
    credit_score: number
    total_orders: number
    total_earnings: number
    online_duration: number
    current_longitude?: number
    current_latitude?: number
}

export interface RiderStatus {
    status: string
    is_online: boolean
    online_status: 'offline' | 'online' | 'delivering'
    active_deliveries: number
    can_go_online: boolean
    can_go_offline: boolean
    online_block_reason?: string
}

export class RiderService {
    static async getMe(): Promise<RiderInfo> {
        return await request({ url: '/v1/rider/me', method: 'GET' })
    }

    static async getStatus(): Promise<RiderStatus> {
        return await request({ url: '/v1/rider/status', method: 'GET' })
    }

    static async goOnline(): Promise<RiderInfo> {
        return await request({ url: '/v1/rider/online', method: 'POST' })
    }

    static async goOffline(): Promise<RiderInfo> {
        return await request({ url: '/v1/rider/offline', method: 'POST' })
    }

    static async updateLocation(locations: Array<{
        longitude: number
        latitude: number
        recorded_at: string
        delivery_id?: number
    }>): Promise<unknown> {
        return await request({
            url: '/v1/rider/location',
            method: 'POST',
            data: { locations }
        })
    }
    static async request<T = unknown>(
        url: string,
        method: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE',
        data?: unknown
    ): Promise<T> {
        return await request<T>({ url, method, data })
    }

    /**
     * 上报异常
     */
    static async reportException(orderId: number, data: {
        exception_type: string
        description: string
    }): Promise<unknown> {
        return await request({
            url: `/v1/rider/orders/${orderId}/exception`,
            method: 'POST',
            data
        })
    }
}

export default RiderService
