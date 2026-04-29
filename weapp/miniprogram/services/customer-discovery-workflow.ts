import { getUserCarts } from '../api/cart'
import CouponService, { type Coupon } from '../api/coupon'
import {
  getPublicMerchantCombos,
  getPublicMerchantDetail,
  getPublicMerchantDishes,
  type PublicCombo,
  type PublicDish,
  type PublicDishCategory,
  type PublicMerchantDetail
} from '../api/merchant'
import { getPublicMerchantRooms, type PublicRoom } from '../api/room'

export type CustomerCoupon = Coupon
export type CustomerMerchantDetail = PublicMerchantDetail
export type CustomerDish = PublicDish
export type CustomerDishCategory = PublicDishCategory
export type CustomerCombo = PublicCombo
export type CustomerRoom = PublicRoom

export interface CustomerMerchantCouponView {
  id: number
  title: string
  valueDisplay: string
  minSpendDisplay: string
  statusText: string
  claimable: boolean
}

export async function loadCustomerMerchantDetail(merchantId: number): Promise<CustomerMerchantDetail> {
  return getPublicMerchantDetail(merchantId)
}

export async function loadCustomerMerchantDishes(merchantId: number) {
  return getPublicMerchantDishes(merchantId)
}

export async function loadCustomerMerchantCombos(merchantId: number) {
  return getPublicMerchantCombos(merchantId)
}

export async function loadCustomerMerchantRooms(merchantId: number) {
  return getPublicMerchantRooms(merchantId)
}

export async function loadCustomerMerchantCoupons(merchantId: number) {
  return CouponService.getAvailableCoupons({
    merchant_id: merchantId,
    page_id: 1,
    page_size: 3
  })
}

export async function loadCustomerMerchantCouponViews(merchantId: number): Promise<{ coupons: CustomerMerchantCouponView[], error: string }> {
  try {
    const result = await loadCustomerMerchantCoupons(merchantId)
    return {
      coupons: result.coupons.map(mapCustomerMerchantCouponView),
      error: ''
    }
  } catch (error) {
    console.error('加载优惠券失败:', error)
    return { coupons: [], error: '优惠券刷新失败' }
  }
}

export function markCustomerCouponClaimed(coupons: CustomerMerchantCouponView[], couponId: number): CustomerMerchantCouponView[] {
  return coupons.map((coupon) => coupon.id === couponId
    ? { ...coupon, claimable: false, statusText: '已领取' }
    : coupon)
}

function mapCustomerMerchantCouponView(coupon: Coupon): CustomerMerchantCouponView {
  const remaining = coupon.total_count > 0 ? coupon.total_count - coupon.claimed_count : 1
  const claimable = !coupon.is_claimed && remaining > 0
  return {
    id: coupon.id,
    title: coupon.title,
    valueDisplay: formatFen(coupon.value),
    minSpendDisplay: formatFen(coupon.min_spend),
    statusText: coupon.is_claimed ? '已领取' : claimable ? '领取' : '已领完',
    claimable
  }
}

function formatFen(value: number): string {
  return (value / 100).toFixed(2)
}

export async function claimCustomerCoupon(couponId: number) {
  return CouponService.claimCoupon(couponId)
}

export async function loadTakeoutCartSummary() {
  return getUserCarts('takeout')
}
