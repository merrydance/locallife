import { formatPriceNoSymbol } from '../../../utils/util'
import { getPaymentLedger, PaymentLedgerEntry } from '../../../api/payment'
import ConsumerProfileAdapter from '../../../adapters/consumer-profile'
import MembershipService from '../../../api/membership'
import Navigation from '../../../utils/navigation'
import { getPaymentLedgerStatusView, isPaymentLedgerEntryTerminal } from '../../../utils/payment-ledger-view'
import { MEMBERSHIP_RECHARGE_PAUSED_MESSAGE } from '../../../utils/membership-recharge-pause'

interface MembershipDisplay {
  id: number
  merchant_id: number
  merchant_name: string
  logo_url: string
  balance_display: string
  created_at_date: string
}

interface TransactionDisplay {
  id: string
  type: 'PAYMENT' | 'REFUND' | 'TOPUP'
  amount: number
  amountDisplay: string
  title: string
  time: string
  status: string
  statusName: string
  statusTheme: 'primary' | 'success' | 'warning' | 'error' | 'default'
}

interface BusinessTitlePair {
  payment: string
  refund: string
}

const businessTitleMap: Record<string, BusinessTitlePair> = {
  order: { payment: '订单消费', refund: '订单退款' },
  reservation: { payment: '预订消费', refund: '预订退款' },
  reservation_addon: { payment: '预订补差', refund: '预订退款' },
  membership_recharge: { payment: '会员充值', refund: '会员退款' },
  rider_deposit: { payment: '押金支付', refund: '押金退款' },
  claim_recovery: { payment: '追偿支付', refund: '追偿退款' }
}

function formatTransactionTime(timeText: string): string {
  const date = new Date(timeText)
  return `${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

function mapTransactionDisplay(entry: PaymentLedgerEntry): TransactionDisplay {
  const isRefund = entry.entry_type === 'refund'
  const amount = isRefund ? entry.amount : -entry.amount
  const titleConfig = businessTitleMap[entry.business_type] || { payment: '支付记录', refund: '退款记录' }
  const statusView = getPaymentLedgerStatusView(entry)

  return {
    id: String(isRefund ? entry.refund_order_id || entry.id : entry.payment_order_id),
    type: isRefund ? 'REFUND' : 'PAYMENT',
    amount,
    amountDisplay: `${amount > 0 ? '+' : '-'}${formatPriceNoSymbol(Math.abs(amount))}`,
    title: isRefund ? titleConfig.refund : titleConfig.payment,
    time: formatTransactionTime(entry.occurred_at || entry.created_at),
    status: entry.status,
    statusName: statusView.statusName,
    statusTheme: statusView.statusTheme
  }
}

type MembershipEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      id?: number
      item?: {
        type?: 'REFUND' | 'PAYMENT' | 'TOPUP'
        id?: string
      }
    }
  }
}

Page({
  data: {
    balance: 0,
    balanceDisplay: '0.00',
    totalRecharged: 0,
    totalRechargedDisplay: '0.00',
    memberships: [] as MembershipDisplay[],
    transactions: [] as TransactionDisplay[],
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    error: null as string | null
  },

  onLoad() {
    this.setData({ loading: true })
    this.initData()
  },

  onShow() {
    // Refresh data when returning to page
    if (!this.data.loading) {
       this.initData()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async initData() {
    if (this.data.loading && !this.data.initialLoading) return
    
    this.setData({ loading: true, error: null })
    try {
      const [membershipRes, paymentRes] = await Promise.all([
        MembershipService.listMyMemberships(1, 50),
        getPaymentLedger({ page_id: 1, page_size: 10 })
      ])

      // 1. Process Memberships
      const memberships: MembershipDisplay[] = (membershipRes.memberships || []).map((m) => {
        return ConsumerProfileAdapter.toWalletMembershipViewModel(m)
      })

      const totalBalance = (membershipRes.memberships || []).reduce((sum, m) => sum + (m.balance || 0), 0)
      const totalRecharged = (membershipRes.memberships || []).reduce((sum, m) => sum + (m.total_recharged || 0), 0)

      const transactions: TransactionDisplay[] = (paymentRes.entries || [])
        .filter((entry) => isPaymentLedgerEntryTerminal(entry))
        .map((entry) => mapTransactionDisplay(entry))

      this.setData({
        balance: totalBalance,
        balanceDisplay: formatPriceNoSymbol(totalBalance),
        totalRecharged,
        totalRechargedDisplay: formatPriceNoSymbol(totalRecharged),
        memberships,
        transactions,
        loading: false,
        initialLoading: false
      })
    } catch (error) {
      console.error('Failed to load wallet data:', error)
      this.setData({ 
        loading: false, 
        initialLoading: false,
        error: '加载钱包数据失败'
      })
    }
  },

  onRetry() {
    this.initData()
  },

  onShowAssetDetail() {
    const cardCount = this.data.memberships.length
    wx.showModal({
      title: '资产概览',
      content: `您当前在 ${cardCount} 个商户拥有会员余额，总计 ¥${this.data.balanceDisplay}。`,
      showCancel: false,
      confirmText: '我知道了'
    })
  },

  onTopUp(e: MembershipEvent) {
    const id = e.currentTarget.dataset.id
    if (id) {
       const m = this.data.memberships.find((item) => item.id === id)
       if (m) {
         this.showRechargePaused(m)
         return
       }
    }
    
    if (this.data.memberships.length === 0) {
      wx.showModal({
        title: '暂无会员卡',
        content: '您目前还没有会员卡，去喜欢的餐厅看看吧',
        confirmText: '去逛逛',
        success: (res) => {
          if (res.confirm) Navigation.toTakeoutHome()
        }
      })
      return
    }

    // General selection
    const names = this.data.memberships.map((m) => m.merchant_name)
    wx.showActionSheet({
      itemList: names.slice(0, 6), // Wechat limit is 6
      success: (res) => {
        const m = this.data.memberships[res.tapIndex]
        this.showRechargePaused(m)
      }
    })
  },

  showRechargePaused(membership?: MembershipDisplay) {
    wx.showModal({
      title: '线上充值已暂停',
      content: MEMBERSHIP_RECHARGE_PAUSED_MESSAGE,
      cancelText: '我知道了',
      confirmText: membership ? '查看商户' : '我知道了',
      showCancel: !!membership,
      success: (res) => {
        if (res.confirm && membership) {
          Navigation.toRestaurantDetail(membership.merchant_id)
        }
      }
    })
  },

  onWithdraw() {
    wx.showModal({
      title: '暂未开放',
      content: '提现功能正在安全评估中，敬请期待。',
      showCancel: false,
      confirmText: '我知道了'
    })
  },

  onGoToCoupons() {
    Navigation.toCoupons()
  },

  onShowBill() {
    wx.showToast({ title: '完整账单生成中', icon: 'none' })
  },

  onManageMembership() {
    Navigation.toMembership()
  },

  onViewMembership(e: MembershipEvent) {
    const id = e.currentTarget.dataset.id
    const m = this.data.memberships.find((item) => item.id === id)
    if (m) {
      Navigation.toRestaurantDetail(m.merchant_id)
    }
  },

  onTransactionDetail(e: MembershipEvent) {
    const item = e.currentTarget.dataset.item
    if (!item?.id || !item.type) {
      return
    }
    if (item.type === 'REFUND') {
      wx.navigateTo({ url: `/pages/user_center/refund-detail/index?id=${item.id}` })
    } else {
      wx.navigateTo({ url: `/pages/user_center/payment-detail/index?id=${item.id}` })
    }
  }
})
