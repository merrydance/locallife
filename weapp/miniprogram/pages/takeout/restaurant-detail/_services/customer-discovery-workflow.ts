import { getUserCarts } from '../../../../api/cart'
import CouponService from '../_main_shared/api/coupon'
import {
  getPublicMerchantCombos,
  getPublicMerchantDetail,
  getPublicMerchantDishes,
  type PublicCombo,
  type PublicDish,
  type PublicDishCategory,
  type PublicMerchantDetail
} from '../../../../api/merchant'
import { getPublicMerchantRooms, type PublicRoom } from '../_main_shared/api/room'

export type CustomerMerchantDetail = PublicMerchantDetail
export type CustomerDish = PublicDish
export type CustomerDishCategory = PublicDishCategory
export type CustomerCombo = PublicCombo
export type CustomerRoom = PublicRoom
export type CustomerCoupon = NonNullable<PublicMerchantDetail['vouchers']>[number]

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

export function buildCustomerMerchantCouponViews(
  vouchers?: PublicMerchantDetail['vouchers']
): CustomerMerchantCouponView[] {
  if (!vouchers || vouchers.length === 0) {
    return []
  }

  return vouchers.slice(0, 3).map(mapCustomerMerchantCouponView)
}

export function markCustomerCouponClaimed(coupons: CustomerMerchantCouponView[], couponId: number): CustomerMerchantCouponView[] {
  return coupons.map((coupon) => coupon.id === couponId
    ? { ...coupon, claimable: false, statusText: '已领取' }
    : coupon)
}

function mapCustomerMerchantCouponView(coupon: CustomerCoupon): CustomerMerchantCouponView {
  return {
    id: coupon.id,
    title: coupon.name,
    valueDisplay: formatFen(coupon.amount),
    minSpendDisplay: formatFen(coupon.min_order_amount),
    statusText: '领取',
    claimable: true
  }
}

function formatFen(value: number): string {
  return (value / 100).toFixed(2)
}

export async function claimCustomerCoupon(couponId: number) {
  return CouponService.claimCoupon(couponId)
}

export async function loadTakeoutCartSummary(orderType: 'takeout' | 'takeaway' = 'takeout') {
  return getUserCarts(orderType)
}
