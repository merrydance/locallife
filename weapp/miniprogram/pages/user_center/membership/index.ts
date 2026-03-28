import MembershipService, { Membership } from '../../../api/membership'
import ConsumerProfileAdapter from '../../../adapters/consumer-profile'
import { getPublicRechargeRules, rechargeMembership, RechargeRuleResponse } from '../../../api/personal'
import { invokeWechatPay, PaymentCancelledError } from '../../../api/payment'
import { formatPriceNoSymbol } from '../../../utils/util'
import { ErrorHandler } from '../../../utils/error-handler'
import Navigation from '../../../utils/navigation'

const DEFAULT_RECHARGE_AMOUNTS = [5000, 10000, 20000]

type RechargeOption = {
  label: string
  amount: number
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

function buildRechargeOptions(rules: RechargeRuleResponse[]): RechargeOption[] {
  if (rules.length > 0) {
    return rules
      .slice()
      .sort((left, right) => left.recharge_amount - right.recharge_amount)
      .slice(0, 6)
      .map((rule) => {
        const rechargeDisplay = formatPriceNoSymbol(rule.recharge_amount)
        const bonusDisplay = formatPriceNoSymbol(rule.bonus_amount)
        const totalDisplay = formatPriceNoSymbol(rule.recharge_amount + rule.bonus_amount)
        return {
          label: rule.bonus_amount > 0
            ? `充¥${rechargeDisplay}送¥${bonusDisplay} 到账¥${totalDisplay}`
            : `充值 ¥${rechargeDisplay}`,
          amount: rule.recharge_amount
        }
      })
  }

  return DEFAULT_RECHARGE_AMOUNTS.map((amount) => ({
    label: `充值 ¥${formatPriceNoSymbol(amount)}`,
    amount
  }))
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
    hasMore: true,
    rechargingMembershipId: 0
  },

  _pendingAutoRechargeMembershipId: 0,

  onLoad(options?: { membershipId?: string, autoRecharge?: string }) {
    const membershipId = Number(options?.membershipId || 0)
    const shouldAutoRecharge = options?.autoRecharge === '1'
    this._pendingAutoRechargeMembershipId = shouldAutoRecharge && membershipId > 0 ? membershipId : 0
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

  async chooseRechargeAmount(options: RechargeOption[]): Promise<RechargeOption | null> {
    return new Promise((resolve) => {
      wx.showActionSheet({
        itemList: options.map((item) => item.label),
        success: ({ tapIndex }) => {
          resolve(options[tapIndex] || null)
        },
        fail: () => {
          resolve(null)
        }
      })
    })
  },

  async tryAutoRecharge(memberships: MembershipDisplay[]) {
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

    await this.startRecharge(item)
  },

  async startRecharge(item: MembershipDisplay) {
    this.setData({ rechargingMembershipId: item.id })
    wx.showLoading({ title: '加载方案...' })

    try {
      const rules = await getPublicRechargeRules(item.merchantId).catch(() => [] as RechargeRuleResponse[])
      wx.hideLoading()

      const selectedOption = await this.chooseRechargeAmount(buildRechargeOptions(rules))
      if (!selectedOption) {
        return
      }

      wx.showLoading({ title: '发起支付...' })
      const rechargeResult = await rechargeMembership({
        membership_id: item.id,
        payment_method: 'wechat',
        recharge_amount: selectedOption.amount
      })

      if (!rechargeResult.pay_params) {
        throw new Error('支付参数缺失')
      }

      wx.hideLoading()
      await invokeWechatPay(rechargeResult.pay_params)
      wx.showToast({ title: '支付成功', icon: 'success' })

      setTimeout(() => {
        void this.loadMemberships(true)
      }, 1200)
    } catch (error) {
      wx.hideLoading()
      if (error instanceof PaymentCancelledError) {
        wx.showToast({ title: '已取消支付', icon: 'none' })
        return
      }

      ErrorHandler.handle(error, 'Membership.recharge')
    } finally {
      this.setData({ rechargingMembershipId: 0 })
    }
  },

  async onRechargeTap(e: WechatMiniprogram.BaseEvent) {
    const item = e.currentTarget.dataset.item as MembershipDisplay | undefined
    if (!item) return
    await this.startRecharge(item)
  },

  onGoHome() {
    Navigation.toTakeoutHome()
  }
})
