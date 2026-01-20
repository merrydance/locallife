import { getMyVouchers, getMyAvailableVouchers, claimVoucher, UserVoucherResponse, VoucherResponse } from '../../../api/personal'
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

const getDatasetId = (event: WechatMiniprogram.CustomEvent): number | null => {
  const dataset = event.currentTarget.dataset as { id?: string | number }
  const id = dataset.id
  const numericId = typeof id === 'number' ? id : Number(id)
  return Number.isFinite(numericId) ? numericId : null
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
        coupons = response.vouchers.map((v: VoucherResponse) => ({
          id: v.id,
          merchant_name: v.merchant_name || (v.merchant_id === 0 ? '平台通用' : `商户${v.merchant_id}`),
          name: v.name,
          threshold: v.min_order_amount,
          thresholdDisplay: formatPriceNoSymbol(v.min_order_amount || 0),
          discount: v.amount,
          discountDisplay: formatPriceNoSymbol(v.amount || 0),
          end_date: v.valid_until?.split('T')[0] || '',
          can_claim: true
        }))
      } else {
        // 获取我的优惠券
        const response = await getMyVouchers()
        coupons = response.vouchers.map((v: UserVoucherResponse) => ({
          id: v.id,
          merchant_name: v.merchant_name || '平台通用',
          name: v.name,
          threshold: v.min_order_amount,
          thresholdDisplay: formatPriceNoSymbol(v.min_order_amount || 0),
          discount: v.amount,
          discountDisplay: formatPriceNoSymbol(v.amount || 0),
          end_date: v.expires_at?.split('T')[0] || '',
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

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: 'AVAILABLE' | 'MY' }>) {
    this.setData({ activeTab: e.detail.value })
    this.loadCoupons()
  },

  async onClaimCoupon(e: WechatMiniprogram.CustomEvent) {
    const id = getDatasetId(e)
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
