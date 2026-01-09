/**
 * 优惠券系统接口
 * 包含优惠券列表、领取、我的优惠券等功能
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

export type CouponStatus = 'available' | 'used' | 'expired'

/**
 * 优惠券定义
 */
export interface Coupon {
    id: number
    merchant_id: number
    merchant_name: string
    title: string
    type: 'discount' | 'amount' // 折扣券 或 满减券
    value: number // 折扣率(85=8.5折) 或 减免金额(分)
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
     * GET /v1/coupons
     */
    static async getAvailableCoupons(params: CouponListParams): Promise<{ coupons: Coupon[], total: number }> {
        return await request({
            url: '/v1/coupons',
            method: 'GET',
            data: params
        })
    }

    /**
     * 领取优惠券
     * POST /v1/coupons/:id/claim
     */
    static async claimCoupon(id: number): Promise<UserCoupon> {
        return await request({
            url: `/v1/coupons/${id}/claim`,
            method: 'POST'
        })
    }

    /**
     * 获取我的优惠券列表
     * GET /v1/user/coupons
     */
    static async getMyCoupons(params: MyCouponParams): Promise<{ coupons: UserCoupon[], total: number }> {
        return await request({
            url: '/v1/user/coupons',
            method: 'GET',
            data: params
        })
    }
}

export default CouponService
