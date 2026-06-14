import { request } from '../../../../utils/request'

export interface RiderInfo {
    id: number
    user_id: number
    region_id: number
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
    location_updated_at?: string
    created_at: string
}

export interface RiderStatus {
    status: string
    is_online: boolean
    online_status: 'offline' | 'online' | 'delivering'
    active_deliveries: number
    current_region_id: number
    required_deposit: number
    current_longitude?: number
    current_latitude?: number
    location_updated_at?: string
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

export interface RiderDepositBalance {
    current_region_id: number
    required_deposit: number
    total_deposit: number
    frozen_deposit: number
    delivery_frozen_deposit?: number
    withdrawal_processing_amount?: number
    available_deposit: number
}

export interface RiderDepositRecord {
    id: number
    rider_id: number
    amount: number
    type: string
    balance_after: number
    remark?: string
    created_at: string
}

export interface RiderDepositListResponse {
    deposits: RiderDepositRecord[]
    total: number
    page_id: number
    page_size: number
}

export interface RiderDepositPayResponse {
    payment_order_id?: number
    out_trade_no?: string
    amount?: number
    expires_at?: string
    pay_params?: WechatMiniprogram.RequestPaymentOption
}

export interface RiderWithdrawRefundItem {
    refund_order_id: number
    payment_order_id: number
    out_refund_no: string
    amount: number
    status: string
}

export interface RiderWithdrawResponse {
    status: string
    requested_amount: number
    accepted_amount: number
    refunds: RiderWithdrawRefundItem[]
}

export interface RiderWithdrawOptions {
    idempotencyKey: string
}

export interface RiderWithdrawalStatusRefundItem {
    refund_order_id: number
    payment_order_id: number
    out_refund_no: string
    refund_id?: string
    amount: number
    status: string
    status_text: string
    created_at: string
    refunded_at?: string
    source_payment_amount: number
    out_trade_no: string
}

export interface RiderWithdrawalStatusResponse {
    status: string
    status_text: string
    message: string
    requested_refund_order_ids: number[]
    accepted_amount: number
    processing_amount: number
    success_amount: number
    failed_amount: number
    refunds: RiderWithdrawalStatusRefundItem[]
}

export class RiderService {
    static async getMe(): Promise<RiderInfo> {
        return await request({ url: '/v1/rider/me', method: 'GET' })
    }

    static async getStatus(): Promise<RiderStatus> {
        return await request({ url: '/v1/rider/status', method: 'GET' })
    }

    static async syncCurrentRegion(regionId: number): Promise<RiderInfo> {
        return await request({
            url: '/v1/rider/current-region',
            method: 'PATCH',
            data: { region_id: regionId }
        })
    }

    static async goOnline(regionId: number): Promise<RiderInfo> {
        return await request({
            url: '/v1/rider/online',
            method: 'POST',
            data: { region_id: regionId }
        })
    }

    static async goOffline(): Promise<RiderInfo> {
        return await request({ url: '/v1/rider/offline', method: 'POST' })
    }

    static async getDepositBalance(): Promise<RiderDepositBalance> {
        return await request({ url: '/v1/rider/deposit', method: 'GET' })
    }

    static async listDepositRecords(params: { page: number, limit: number }): Promise<RiderDepositListResponse> {
        return await request({
            url: '/v1/rider/deposits',
            method: 'GET',
            data: params
        })
    }

    static async rechargeDeposit(data: { amount: number, remark?: string }): Promise<RiderDepositPayResponse> {
        return await request({
            url: '/v1/rider/deposit',
            method: 'POST',
            data
        })
    }

    static async withdrawDeposit(data: { amount: number, remark?: string }, options: RiderWithdrawOptions): Promise<RiderWithdrawResponse> {
        if (!options?.idempotencyKey) {
            throw new Error('缺少押金提现请求幂等键')
        }

        return await request({
            url: '/v1/rider/withdraw',
            method: 'POST',
            data,
            header: {
                'Idempotency-Key': options.idempotencyKey
            }
        })
    }

    static async getWithdrawalStatus(refundOrderIds: number[]): Promise<RiderWithdrawalStatusResponse> {
        return await request({
            url: '/v1/rider/withdrawals/status',
            method: 'GET',
            data: { refund_order_ids: refundOrderIds.join(',') }
        })
    }

    static async updateLocation(regionId: number, locations: Array<{
        longitude: number
        latitude: number
        recorded_at: string
        delivery_id?: number
        source?: string
        accuracy?: number
        speed?: number
        heading?: number
    }>): Promise<unknown> {
        return await request({
            url: '/v1/rider/location',
            method: 'POST',
            data: { region_id: regionId, locations }
        })
    }
    static async request<T = unknown>(
        url: string,
        method: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE',
        data?: unknown
    ): Promise<T> {
        return await request<T>({ url, method, data })
    }
}

export default RiderService
