/**
 * 优惠券系统接口
 * 包含优惠券列表、领取、我的优惠券等功能
 */

import { request } from '../utils/request'
import { normalizePaginatedResult, type PaginatedListResult, type PaginationEnvelope } from './types'

// ==================== 数据类型定义 ====================

export type CouponStatus = 'available' | 'used' | 'expired'

interface BackendVoucher {
    id: number
    merchant_id: number
    code?: string
    merchant_name?: string
    name: string
    description?: string
    amount: number
    min_order_amount: number
    valid_from?: string
    valid_until?: string
    total_quantity?: number
    claimed_quantity?: number
    used_quantity?: number
    is_active?: boolean
    status_code?: MerchantVoucherStatusCode
    status_label?: string
    status_theme?: MerchantVoucherStatusTheme
    allowed_order_types?: string[]
    is_claimed?: boolean
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

export interface MerchantVoucher {
    id: number
    merchant_id: number
    code: string
    name: string
    description: string
    amount: number
    min_order_amount: number
    total_quantity: number
    claimed_quantity: number
    used_quantity: number
    valid_from: string
    valid_until: string
    is_active: boolean
    status_code: MerchantVoucherStatusCode
    status_label: string
    status_theme: MerchantVoucherStatusTheme
    allowed_order_types: string[]
}

export type MerchantVoucherStatusTheme = 'success' | 'warning' | 'danger' | 'default'
export type MerchantVoucherStatusCode = 'inactive' | 'expired' | 'scheduled' | 'depleted' | 'active'

export interface MerchantVoucherStatusView {
    label: string
    theme: MerchantVoucherStatusTheme
    code: MerchantVoucherStatusCode
}

export function buildMerchantVoucherStatusView(
    voucher: Pick<MerchantVoucher, 'is_active' | 'valid_from' | 'valid_until' | 'total_quantity' | 'claimed_quantity'>,
    now: Date = new Date()
): MerchantVoucherStatusView {
    const voucherWithStatus = voucher as Partial<Pick<MerchantVoucher, 'status_code' | 'status_label' | 'status_theme'>>

    if (voucherWithStatus.status_code && voucherWithStatus.status_label && voucherWithStatus.status_theme) {
        return {
            label: voucherWithStatus.status_label,
            theme: voucherWithStatus.status_theme,
            code: voucherWithStatus.status_code
        }
    }

    const validUntil = new Date(voucher.valid_until)
    const validFrom = new Date(voucher.valid_from)
    const remainingQuantity = Math.max(voucher.total_quantity - voucher.claimed_quantity, 0)

    if (!voucher.is_active) {
        return { label: '已停用', theme: 'default', code: 'inactive' }
    }

    if (now > validUntil) {
        return { label: '已过期', theme: 'danger', code: 'expired' }
    }

    if (now < validFrom) {
        return { label: '未开始', theme: 'warning', code: 'scheduled' }
    }

    if (remainingQuantity <= 0) {
        return { label: '已领完', theme: 'warning', code: 'depleted' }
    }

    return { label: '发放中', theme: 'success', code: 'active' }
}

export interface MerchantVoucherListResult extends PaginatedListResult<MerchantVoucher> {
    vouchers: MerchantVoucher[]
}

export interface CreateMerchantVoucherParams {
    code?: string
    name: string
    description?: string
    amount: number
    min_order_amount: number
    total_quantity: number
    valid_from: string
    valid_until: string
    allowed_order_types?: string[]
}

export interface UpdateMerchantVoucherParams {
    name?: string
    description?: string
    amount?: number
    min_order_amount?: number
    total_quantity?: number
    valid_from?: string
    valid_until?: string
    is_active?: boolean
    allowed_order_types?: string[]
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

export interface CouponListResult extends PaginatedListResult<Coupon> {
    coupons: Coupon[]
}

export interface UserCouponListResult extends PaginatedListResult<UserCoupon> {
    coupons: UserCoupon[]
}

type VoucherListResponse<T> = PaginationEnvelope & {
    vouchers?: T[]
    total_count?: number
}

function buildCouponListResult<T>(coupons: T[], response: VoucherListResponse<unknown>, params: { page_id: number, page_size: number }) {
    const normalized = normalizePaginatedResult(coupons, {
        ...response,
        total: typeof response?.total === 'number' ? response.total : response?.total_count
    }, {
        page: params.page_id,
        pageSize: params.page_size
    })

    return {
        ...normalized,
        coupons
    }
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
        is_claimed: item.is_claimed === true
    }
}

function normalizeMerchantVoucher(item: BackendVoucher): MerchantVoucher {
    const normalizedVoucher = {
        id: item.id,
        merchant_id: item.merchant_id,
        code: item.code || '',
        name: item.name,
        description: item.description || '',
        amount: item.amount,
        min_order_amount: item.min_order_amount,
        total_quantity: item.total_quantity || 0,
        claimed_quantity: item.claimed_quantity || 0,
        used_quantity: item.used_quantity || 0,
        valid_from: item.valid_from || '',
        valid_until: item.valid_until || '',
        is_active: typeof item.is_active === 'boolean' ? item.is_active : true,
        status_code: item.status_code || 'active',
        status_label: item.status_label || '',
        status_theme: item.status_theme || 'success',
        allowed_order_types: Array.isArray(item.allowed_order_types) ? item.allowed_order_types : []
    } as MerchantVoucher

    const statusView = buildMerchantVoucherStatusView(normalizedVoucher)

    return {
        ...normalizedVoucher,
        status_code: statusView.code,
        status_label: statusView.label,
        status_theme: statusView.theme
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
    static async getAvailableCoupons(params: CouponListParams): Promise<CouponListResult> {
        if (params.merchant_id) {
            const res = await request<VoucherListResponse<BackendVoucher>>({
                url: `/v1/merchants/${params.merchant_id}/vouchers/active`,
                method: 'GET',
                data: params
            })

            const vouchers = Array.isArray(res?.vouchers) ? res.vouchers : []
            const coupons = vouchers.map(normalizeVoucherToCoupon)

            return buildCouponListResult(coupons, res, params)
        }

        const res = await request<VoucherListResponse<BackendUserVoucher>>({
            url: '/v1/vouchers/available',
            method: 'GET',
            data: params
        })

        const vouchers = Array.isArray(res?.vouchers) ? res.vouchers : []
        const coupons = vouchers.map(normalizeUserVoucherToCoupon)

        return buildCouponListResult(coupons, res, params)
    }

    /**
     * 获取我的可用优惠券列表
     * GET /v1/vouchers/available
     */
    static async getMyAvailableCoupons(params: MyCouponParams): Promise<UserCouponListResult> {
        const res = await request<VoucherListResponse<BackendUserVoucher>>({
            url: '/v1/vouchers/available',
            method: 'GET',
            data: params
        })

        const vouchers = Array.isArray(res?.vouchers) ? res.vouchers : []
        const coupons = vouchers.map(normalizeUserVoucher)

        return buildCouponListResult(coupons, res, params)
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
    static async getMyCoupons(params: MyCouponParams): Promise<UserCouponListResult> {
        const res = await request<VoucherListResponse<BackendUserVoucher>>({
            url: '/v1/vouchers/me',
            method: 'GET',
            data: params
        })

        const vouchers = Array.isArray(res?.vouchers) ? res.vouchers : []
        const coupons = vouchers.map(normalizeUserVoucher)

        return buildCouponListResult(coupons, res, params)
    }
}

export class MerchantVoucherService {
    static async listMerchantVouchers(merchantId: number, pageId: number = 1, pageSize: number = 20): Promise<MerchantVoucherListResult> {
        const res = await request<VoucherListResponse<BackendVoucher>>({
            url: `/v1/merchants/${merchantId}/vouchers`,
            method: 'GET',
            data: { page_id: pageId, page_size: pageSize }
        })

        const vouchers = Array.isArray(res?.vouchers) ? res.vouchers.map(normalizeMerchantVoucher) : []
        const normalized = normalizePaginatedResult(vouchers, res, { page: pageId, pageSize })

        return {
            ...normalized,
            vouchers
        }
    }

    static async createMerchantVoucher(merchantId: number, data: CreateMerchantVoucherParams): Promise<MerchantVoucher> {
        const res = await request<BackendVoucher>({
            url: `/v1/merchants/${merchantId}/vouchers`,
            method: 'POST',
            data
        })

        return normalizeMerchantVoucher(res)
    }

    static async updateMerchantVoucher(merchantId: number, voucherId: number, data: UpdateMerchantVoucherParams): Promise<MerchantVoucher> {
        const res = await request<BackendVoucher>({
            url: `/v1/merchants/${merchantId}/vouchers/${voucherId}`,
            method: 'PATCH',
            data
        })

        return normalizeMerchantVoucher(res)
    }

    static async deleteMerchantVoucher(merchantId: number, voucherId: number): Promise<void> {
        await request({
            url: `/v1/merchants/${merchantId}/vouchers/${voucherId}`,
            method: 'DELETE'
        })
    }
}

export default CouponService
