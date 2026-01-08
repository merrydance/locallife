import { getMyVouchers, getMyAvailableVouchers, claimVoucher, UserVoucherResponse } from '../../../api/personal'
import { formatPriceNoSymbol } from '../../../utils/util'

interface CouponDisplay {
  id: number
  merchant_name: string
  name: string
  threshold: number
  thresholdDisplay: string
  discount: number
  discountDisplay: string
  end_date: string
  can_claim?: boolean
  status?: string
}

Page({
  data: {
    activeTab: 'AVAILABLE' as 'AVAILABLE' | 'MY',
    coupons: [] as CouponDisplay[],
    navBarHeight: 88,
    loading: false
  },

  onLoad() {
    this.loadCoupons()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadCoupons() {
    this.setData({ loading: true })

    try {
      const { activeTab } = this.data
      let coupons: CouponDisplay[] = []

      if (activeTab === 'AVAILABLE') {
        // 获取可领取的优惠券
        const response = await getMyAvailableVouchers()
        coupons = response.vouchers.map(v => ({
          id: v.id,
          merchant_name: v.merchant_name || '平台通用',
          name: v.name,
          threshold: v.min_order_amount,
          thresholdDisplay: formatPriceNoSymbol(v.min_order_amount || 0),
          discount: v.discount_amount,
          discountDisplay: formatPriceNoSymbol(v.discount_amount || 0),
          end_date: v.end_time?.split('T')[0] || '',
          can_claim: true
        }))
      } else {
        // 获取我的优惠券
        const response = await getMyVouchers()
        coupons = response.vouchers.map((v: UserVoucherResponse) => ({
          id: v.id,
          merchant_name: v.merchant_name || '平台通用',
          name: v.voucher_name,
          threshold: v.min_order_amount,
          thresholdDisplay: formatPriceNoSymbol(v.min_order_amount || 0),
          discount: v.discount_amount,
          discountDisplay: formatPriceNoSymbol(v.discount_amount || 0),
          end_date: v.end_time?.split('T')[0] || '',
          status: v.status
        }))
      }

      this.setData({
        coupons,
        loading: false
      })
    } catch (error) {
      console.error('加载优惠券失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false, coupons: [] })
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ activeTab: e.detail.value })
    this.loadCoupons()
  },

  async onClaimCoupon(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    if (!id) return

    try {
      await claimVoucher(Number(id))
      wx.showToast({ title: '领取成功', icon: 'success' })
      this.loadCoupons()
    } catch (error) {
      console.error('领取优惠券失败:', error)
      wx.showToast({ title: '领取失败', icon: 'error' })
    }
  }
})
