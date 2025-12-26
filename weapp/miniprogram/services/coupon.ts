/**
 * 优惠券服务
 * 使用真实后端API
 */

import {
  voucherManagementService,
  VoucherResponse
} from '../api/marketing-membership'

export interface Coupon {
  id: string
  name: string
  type: 'DISCOUNT' | 'AMOUNT'
  value: number
  minAmount: number
  startTime: string
  endTime: string
  status: 'AVAILABLE' | 'USED' | 'EXPIRED'
  description?: string
  code?: string
}

export class CouponService {
  private static instance: CouponService

  private constructor() { }

  static getInstance(): CouponService {
    if (!CouponService.instance) {
      CouponService.instance = new CouponService()
    }
    return CouponService.instance
  }

  /**
   * 获取商户可用优惠券
   */
  async getAvailableCoupons(merchantId: string): Promise<Coupon[]> {
    try {
      const merchantIdNum = parseInt(merchantId)
      if (isNaN(merchantIdNum)) {
        return []
      }

      const vouchers = await voucherManagementService.listActiveVouchers(merchantIdNum)
      return this.convertVouchersToCoupons(vouchers)
    } catch (error) {
      console.error('获取优惠券失败:', error)
      return []
    }
  }

  /**
   * 获取用户优惠券
   * 注意：后端暂无用户优惠券列表接口，返回空数组
   */
  async getUserCoupons(): Promise<Coupon[]> {
    // TODO: 后端需要实现 GET /v1/customers/vouchers 接口
    console.warn('getUserCoupons: 后端暂无用户优惠券列表接口')
    return []
  }

  /**
   * 领取优惠券
   * 注意：后端暂无领取优惠券接口
   */
  async claimCoupon(couponId: string): Promise<boolean> {
    // TODO: 后端需要实现 POST /v1/vouchers/{id}/claim 接口
    console.warn('claimCoupon: 后端暂无领取优惠券接口')
    return false
  }

  /**
   * 计算优惠金额
   */
  calculateDiscount(amount: number, coupon: Coupon): number {
    if (amount < coupon.minAmount) return 0

    if (coupon.type === 'AMOUNT') {
      return coupon.value
    } else if (coupon.type === 'DISCOUNT') {
      // value is percentage, e.g., 85 for 8.5折
      const discount = Math.floor(amount * (100 - coupon.value) / 100)
      return discount
    }
    return 0
  }

  /**
   * 转换后端优惠券格式为前端格式
   */
  private convertVouchersToCoupons(vouchers: VoucherResponse[]): Coupon[] {
    const now = new Date()

    return vouchers.map(voucher => {
      const validFrom = new Date(voucher.valid_from)
      const validUntil = new Date(voucher.valid_until)

      let status: 'AVAILABLE' | 'USED' | 'EXPIRED' = 'AVAILABLE'
      if (now > validUntil) {
        status = 'EXPIRED'
      } else if (voucher.claimed_quantity >= voucher.total_quantity) {
        status = 'USED' // 已领完
      }

      return {
        id: String(voucher.id),
        name: voucher.name,
        type: 'AMOUNT' as const, // 后端目前只支持金额类型
        value: voucher.amount,
        minAmount: voucher.min_order_amount || 0,
        startTime: voucher.valid_from,
        endTime: voucher.valid_until,
        status,
        description: voucher.description,
        code: voucher.code
      }
    })
  }
}

export default CouponService.getInstance()
