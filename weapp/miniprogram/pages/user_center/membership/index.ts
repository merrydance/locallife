import MembershipService, { Membership } from './_main_shared/api/membership'
import ConsumerProfileAdapter from './_main_shared/adapters/consumer-profile'
import { ErrorHandler } from '../../../utils/error-handler'
import Navigation from '../../../utils/navigation'

function showMembershipRechargePausedMessage() {
  wx.showModal({
    title: '线上充值已暂停',
    content: '会员线上充值已暂停，请联系商户线下充值后入账。',
    showCancel: false,
    confirmText: '我知道了'
  })
}

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

  _pendingAutoRechargeMembershipId: 0,

  onLoad(options?: { membershipId?: string, autoRecharge?: string }) {
    const membershipId = Number(options?.membershipId || 0)
    const shouldAutoRecharge = options?.autoRecharge === '1'
    this._pendingAutoRechargeMembershipId = shouldAutoRecharge && membershipId > 0 ? membershipId : 0
    this.loadMemberships(true)
  },

  onShow() {
    if (!this.data.initialLoading && !this.data.loading) {
      this.loadMemberships(true)
    }
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
      
      const newMemberships: MembershipDisplay[] = res.memberships.map((m: Membership) => ConsumerProfileAdapter.toMembershipCardViewModel(m))

      const memberships = reset ? newMemberships : [...this.data.memberships, ...newMemberships]
      
      this.setData({
        memberships,
        page: page + 1,
        hasMore: memberships.length < res.total,
        loading: false,
        initialLoading: false
      })

      if (reset) {
        void this.tryAutoRecharge(memberships)
      }

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
    const item = this.data.memberships.find((m) => m.id === id)
    if (item) {
       Navigation.toRestaurantDetail(item.merchantId)
    }
  },

  tryAutoRecharge(memberships: MembershipDisplay[]) {
    if (!this._pendingAutoRechargeMembershipId) {
      return
    }

    const targetMembershipId = this._pendingAutoRechargeMembershipId
    this._pendingAutoRechargeMembershipId = 0

    const item = memberships.find((membership) => membership.id === targetMembershipId)
    if (!item) {
      wx.showToast({ title: '未找到对应会员卡', icon: 'none' })
      return
    }

    this.showRechargePaused(item)
  },

  showRechargePaused(_item: MembershipDisplay) {
    showMembershipRechargePausedMessage()
  },

  onRechargeTap(e: WechatMiniprogram.BaseEvent) {
    const item = e.currentTarget.dataset.item as MembershipDisplay | undefined
    if (!item) return
    this.showRechargePaused(item)
  },

  onGoHome() {
    Navigation.toTakeoutHome()
  }
})
