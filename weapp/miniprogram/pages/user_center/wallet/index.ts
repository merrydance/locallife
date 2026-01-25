import { formatPriceNoSymbol } from '../../../utils/util'
import { getPayments, PaymentOrder } from '../../../api/payment-refund'
import MembershipService from '../../../api/membership'

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

Page({
  data: {
    balance: 0,
    balanceDisplay: '0.00',
    totalRecharged: 0,
    totalRechargedDisplay: '0.00',
    memberships: [] as MembershipDisplay[],
    transactions: [] as TransactionDisplay[],
    loading: false,
    navBarHeight: 88
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
    try {
      const [membershipRes, paymentRes] = await Promise.all([
        MembershipService.listMyMemberships(1, 50),
        getPayments({ page: 1, page_size: 10 })
      ])

      // 1. Process Memberships
      const memberships: MembershipDisplay[] = (membershipRes.memberships || []).map(m => {
        const date = new Date(m.created_at)
        const dateStr = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
        return {
          id: m.id,
          merchant_id: m.merchant_id,
          merchant_name: m.merchant_name || '商户会员卡',
          logo_url: m.logo_url || '',
          balance_display: formatPriceNoSymbol(m.balance || 0),
          created_at_date: dateStr
        }
      })

      const totalBalance = (membershipRes.memberships || []).reduce((sum, m) => sum + (m.balance || 0), 0)
      const totalRecharged = (membershipRes.memberships || []).reduce((sum, m) => sum + (m.total_recharged || 0), 0)

      // 2. Process Transactions (from payments)
      const transactions: TransactionDisplay[] = (paymentRes.payment_orders || []).map((p: PaymentOrder) => {
        const isRefund = p.status === 'refunded'
        const amount = isRefund ? p.amount : -p.amount
        
        let statusName = '已完成'
        let statusTheme: any = 'success'
        if (p.status === 'pending') { statusName = '待支付'; statusTheme = 'warning' }
        else if (p.status === 'closed') { statusName = '已关闭'; statusTheme = 'default' }
        else if (p.status === 'refunded') { statusName = '已退款'; statusTheme = 'primary' }

        const date = new Date(p.paid_at || p.created_at)
        const timeStr = `${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`

        return {
          id: String(p.id),
          type: isRefund ? 'REFUND' : 'PAYMENT',
          amount,
          amountDisplay: (amount > 0 ? '+' : '') + formatPriceNoSymbol(Math.abs(amount)),
          title: p.business_type === 'reservation' ? '预订消费' : '订单消费',
          time: timeStr,
          status: p.status,
          statusName,
          statusTheme
        }
      })

      this.setData({
        balance: totalBalance,
        balanceDisplay: formatPriceNoSymbol(totalBalance),
        totalRecharged,
        totalRechargedDisplay: formatPriceNoSymbol(totalRecharged),
        memberships,
        transactions,
        loading: false
      })
    } catch (error) {
      console.error('Failed to load wallet data:', error)
      this.setData({ loading: false })
      wx.showToast({ title: '加载失败', icon: 'error' })
    }
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

  onTopUp(e: any) {
    const id = e.currentTarget.dataset.id
    if (id) {
       // Target specific membership recharge
       const m = this.data.memberships.find(item => item.id === id)
       if (m) {
         wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${m.merchant_id}&activeTab=recharge` })
         return
       }
    }
    
    if (this.data.memberships.length === 0) {
      wx.showModal({
        title: '去充值',
        content: '您目前还没有会员卡，去喜欢的餐厅看看吧',
        confirmText: '去逛逛',
        success: (res) => {
          if (res.confirm) wx.switchTab({ url: '/pages/takeout/index' })
        }
      })
      return
    }

    // General selection
    const names = this.data.memberships.map(m => m.merchant_name)
    wx.showActionSheet({
      itemList: names.slice(0, 6), // Wechat limit is 6
      success: (res) => {
        const m = this.data.memberships[res.tapIndex]
        wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${m.merchant_id}&activeTab=recharge` })
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
    wx.navigateTo({ url: '/pages/user_center/coupons/index' })
  },

  onShowBill() {
    wx.showToast({ title: '完整账单生成中', icon: 'none' })
  },

  onManageMembership() {
    wx.navigateTo({ url: '/pages/user_center/membership/index' })
  },

  onViewMembership(e: any) {
    const id = e.currentTarget.dataset.id
    const m = this.data.memberships.find(item => item.id === id)
    if (m) {
      wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${m.merchant_id}` })
    }
  },

  onTransactionDetail(e: any) {
    const item = e.currentTarget.dataset.item
    if (item.type === 'REFUND') {
      wx.navigateTo({ url: `/pages/user_center/refund-detail/index?id=${item.id}` })
    } else {
      wx.navigateTo({ url: `/pages/user_center/payment-detail/index?id=${item.id}` })
    }
  }
})
