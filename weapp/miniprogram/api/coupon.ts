/**
 * 优惠券系统接口
 * 包含优惠券列表、领取、我的优惠券等功能
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

export type CouponStatus = 'available' | 'used' | 'expired'

interface BackendVoucher {
    id: number
    merchant_id: number
    merchant_name?: string
    name: string
    amount: number
    min_order_amount: number
    valid_from?: string
    valid_until?: string
    total_quantity?: number
    claimed_quantity?: number
}

interface BackendUserVoucher {
    id: number
    voucher_id: number
    merchant_id: number
    merchant_name?: string
    name: string
    amount: number
    min_order_amount: number
    status: CouponStatus
    obtained_at?: string
    expires_at?: string
    used_at?: string
    order_id?: number
}

/**
 * 优惠券定义
 */
export interface Coupon {
    id: number
    merchant_id: number
    merchant_name: string
    title: string
    type: 'discount' | 'amount' // 当前后端仅返回 amount
    value: number // 金额(分)
    min_spend: number // 最低消费金额(分)
    start_time: string
    end_time: string
    total_count: number
    claimed_count: number
    is_claimed?: boolean // 当前用户是否已领取（列表查询时返回）
}

/**
 * 用户优惠券
 */
export interface UserCoupon {
    id: number // 用户优惠券记录ID
    coupon_id: number
    merchant_id: number
    merchant_name: string
    title: string
    type: 'discount' | 'amount'
    value: number
    min_spend: number
    start_time: string
    end_time: string
    status: CouponStatus
    used_at?: string
    order_id?: number
}

function normalizeVoucherToCoupon(item: BackendVoucher): Coupon {
    return {
        id: item.id,
        merchant_id: item.merchant_id,
        merchant_name: item.merchant_name || '商家',
        title: item.name,
        type: 'amount',
        value: item.amount,
        min_spend: item.min_order_amount,
        start_time: item.valid_from || '',
        end_time: item.valid_until || '',
        total_count: item.total_quantity || 0,
        claimed_count: item.claimed_quantity || 0,
        is_claimed: false
    }
}

function normalizeUserVoucherToCoupon(item: BackendUserVoucher): Coupon {
    return {
        id: item.voucher_id,
        merchant_id: item.merchant_id,
        merchant_name: item.merchant_name || '商家',
        title: item.name,
        type: 'amount',
        value: item.amount,
        min_spend: item.min_order_amount,
        start_time: item.obtained_at || '',
        end_time: item.expires_at || '',
        total_count: 0,
        claimed_count: 0,
        is_claimed: true
    }
}

function normalizeUserVoucher(item: BackendUserVoucher): UserCoupon {
    return {
        id: item.id,
        coupon_id: item.voucher_id,
        merchant_id: item.merchant_id,
        merchant_name: item.merchant_name || '商家',
        title: item.name,
        type: 'amount',
        value: item.amount,
        min_spend: item.min_order_amount,
        start_time: item.obtained_at || '',
        end_time: item.expires_at || '',
        status: item.status,
        used_at: item.used_at,
        order_id: item.order_id
    }
}

/**
 * 优惠券列表查询参数
 */
export interface CouponListParams {
    merchant_id?: number
    page_id: number
    page_size: number
}

/**
 * 我的优惠券查询参数
 */
export interface MyCouponParams {
    status?: CouponStatus
    page_id: number
    page_size: number
}

// ==================== 优惠券服务 ====================

export class CouponService {

    /**
     * 获取可领取的优惠券列表
     * 优先使用商户可领券接口；无 merchant_id 时回退为“我的可用券”列表
     */
    static async getAvailableCoupons(params: CouponListParams): Promise<{ coupons: Coupon[], total: number }> {
        if (params.merchant_id) {
            const res = await request<{ vouchers: BackendVoucher[], total?: number, total_count?: number }>({
                url: `/v1/merchants/${params.merchant_id}/vouchers/active`,
                method: 'GET',
                data: params
            })

            const vouchers = Array.isArray(res?.vouchers) ? res.vouchers : []
            const coupons = vouchers.map(normalizeVoucherToCoupon)
            const total = typeof res?.total === 'number'
                ? res.total
                : (typeof res?.total_count === 'number' ? res.total_count : coupons.length)

            return { coupons, total }
        }

        const res = await request<{ vouchers: BackendUserVoucher[], total?: number, total_count?: number }>({
            url: '/v1/vouchers/available',
            method: 'GET',
            data: params
        })

        const vouchers = Array.isArray(res?.vouchers) ? res.vouchers : []
        const coupons = vouchers.map(normalizeUserVoucherToCoupon)
        const total = typeof res?.total === 'number'
            ? res.total
            : (typeof res?.total_count === 'number' ? res.total_count : coupons.length)

        return { coupons, total }
    }

    /**
     * 领取优惠券
     * POST /v1/vouchers/:id/claim
     */
    static async claimCoupon(id: number): Promise<UserCoupon> {
        const res = await request<BackendUserVoucher>({
            url: `/v1/vouchers/${id}/claim`,
            method: 'POST'
        })
        return normalizeUserVoucher(res)
    }

    /**
     * 获取我的优惠券列表
     * GET /v1/vouchers/me
     */
    static async getMyCoupons(params: MyCouponParams): Promise<{ coupons: UserCoupon[], total: number }> {
        const res = await request<{ vouchers: BackendUserVoucher[], total?: number, total_count?: number }>({
            url: '/v1/vouchers/me',
            method: 'GET',
            data: params
        })

        const vouchers = Array.isArray(res?.vouchers) ? res.vouchers : []
        const coupons = vouchers.map(normalizeUserVoucher)
        const total = typeof res?.total === 'number'
            ? res.total
            : (typeof res?.total_count === 'number' ? res.total_count : coupons.length)

        return { coupons, total }
    }
}

export default CouponService
