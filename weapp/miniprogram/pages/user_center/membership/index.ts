import MembershipService, { Membership } from '../../../api/membership'
import { formatPriceNoSymbol } from '../../../utils/util'
import { ErrorHandler } from '../../../utils/error-handler'

interface MembershipDisplay {
  id: number
  merchantId: number
  merchantName: string
  logoUrl: string
  balanceDisplay: string
  totalRechargedDisplay: string
  totalConsumedDisplay: string
}

Page({
  data: {
    memberships: [] as MembershipDisplay[],
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    error: null as string | null,
    page: 1,
    pageSize: 10,
    hasMore: true
  },

  onLoad() {
    this.loadMemberships(true)
  },

  onShow() {
    // Refresh if needed, or just keep loaded
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadMemberships(reset = false) {
    if (this.data.loading && !this.data.initialLoading) return
    if (!reset && !this.data.hasMore) return

    this.setData({ loading: true, error: null })

    try {
      const page = reset ? 1 : this.data.page
      const res = await MembershipService.listMyMemberships(page, this.data.pageSize)
      
      const newMemberships: MembershipDisplay[] = res.memberships.map((m: Membership) => ({
        id: m.id,
        merchantId: m.merchant_id,
        merchantName: m.merchant_name || '商户',
        logoUrl: m.logo_url || '/assets/icons/shop.svg', // Default icon if missing
        balanceDisplay: formatPriceNoSymbol(m.balance || 0),
        totalRechargedDisplay: formatPriceNoSymbol(m.total_recharged || 0),
        totalConsumedDisplay: formatPriceNoSymbol(m.total_consumed || 0)
      }))

      const memberships = reset ? newMemberships : [...this.data.memberships, ...newMemberships]
      
      this.setData({
        memberships,
        page: page + 1,
        hasMore: memberships.length < res.total,
        loading: false,
        initialLoading: false
      })

    } catch (error) {
      ErrorHandler.handle(error, 'Membership.list')
      this.setData({ 
        loading: false,
        initialLoading: false,
        error: '加载会员卡失败'
      })
      if (reset) this.setData({ memberships: [] })
    }
  },

  onRetry() {
    this.loadMemberships(true)
  },

  onReachBottom() {
    this.loadMemberships()
  },

  onPullDownRefresh() {
    this.loadMemberships(true).then(() => {
      wx.stopPullDownRefresh()
    })
  },

  onCardTap(e: WechatMiniprogram.BaseEvent) {
    const id = e.currentTarget.dataset.id
    // Future: Go to membership detail or records
    const item = this.data.memberships.find(m => m.id === id)
    if (item) {
       // Navigate to merchant detail for now, or just show toast
       wx.navigateTo({
           url: `/pages/takeout/restaurant-detail/index?id=${item.merchantId}`
       })
    }
  },

  onRechargeTap(e: WechatMiniprogram.BaseEvent) {
    const item = e.currentTarget.dataset.item
    // Placeholder for Recharge Flow
    wx.showToast({
        title: '充值功能即将上线',
        icon: 'none'
    })
    // In future: navigate to /pages/membership/recharge?id=...
  },

  onGoHome() {
    wx.switchTab({ url: '/pages/takeout/index' })
  }
})
