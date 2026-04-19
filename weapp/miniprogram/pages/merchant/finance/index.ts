import {
  createMerchantWithdraw,
  getMerchantAccountBalance,
  getMerchantWithdrawal,
  getMerchantWithdrawStatusView,
  listMerchantWithdrawals,
  type MerchantAccountBalanceResponse,
  type MerchantWithdrawItem
} from '../../../api/merchant-finance-account'
import {
  fetchMerchantPaymentReadiness,
  type MerchantPaymentReadinessView
} from '../../../services/merchant-payment'
import {
  canManageMerchantApplyment,
  ensureMerchantConsoleAccess
} from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorDebugMessage, getErrorUserMessage } from '../../../utils/user-facing'

type InputChangeDetail = {
  value: string
}

const EMPTY_BALANCE: MerchantAccountBalanceResponse = {
  sub_mch_id: '',
  available_amount: 0,
  pending_amount: 0,
  withdrawable_amount: 0,
  account_status: '',
  status_desc: ''
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    canManageMerchantApplyment: false,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    loading: false,
    loadedOnce: false,
    refreshErrorMessage: '',
    readinessSummaryTitle: '',
    balanceStatusDesc: '',
    readinessActionText: '',
    readinessActionPath: '',
    notConfigured: false,
    balance: EMPTY_BALANCE as MerchantAccountBalanceResponse,
    paymentReadiness: null as MerchantPaymentReadinessView | null,
    withdrawAmountYuan: '',
    withdrawRemark: '',
    isWithdrawDialogVisible: false,
    withdrawSyncingId: 0,
    withdrawals: [] as MerchantWithdrawItem[],
    submitting: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage()
  },

  onPullDownRefresh() {
    if (!this.hasAccess()) {
      wx.stopPullDownRefresh()
      return
    }

    void this.loadData()
  },

  async bootstrapPage() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      canManageMerchantApplyment: false,
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      loading: false,
      loadedOnce: false,
      refreshErrorMessage: ''
    })

    const accessResult = await ensureMerchantConsoleAccess()
    const roles = accessResult.status === 'granted' ? accessResult.user?.roles || [] : []

    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : '',
      canManageMerchantApplyment: canManageMerchantApplyment(roles)
    })

    if (accessResult.status !== 'granted') {
      this.setData({ initialLoading: false })
      return
    }

    await this.loadData()
  },

  hasAccess() {
    return this.data.accessReady && !this.data.accessDenied && !this.data.accessErrorMessage
  },

  async loadData() {
    const hadTrustedData = this.data.loadedOnce

    this.setData({
      loading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: ''
    })

    try {
      const paymentReadiness = await fetchMerchantPaymentReadiness()

      if (!paymentReadiness.canOpenWithdraw) {
        this.setData({
          initialLoading: false,
          initialError: false,
          initialErrorMessage: '',
          loading: false,
          loadedOnce: true,
          paymentReadiness,
          notConfigured: true,
          readinessSummaryTitle: paymentReadiness.summaryTitle,
          balanceStatusDesc: paymentReadiness.summaryDescription,
          readinessActionText: paymentReadiness.actionText,
          readinessActionPath: paymentReadiness.actionPath,
          balance: EMPTY_BALANCE,
          withdrawals: [],
          refreshErrorMessage: ''
        })

        return
      }

      const [balance, records] = await Promise.all([
        getMerchantAccountBalance(),
        listMerchantWithdrawals(1, 20)
      ])

      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        loading: false,
        loadedOnce: true,
        paymentReadiness,
        balance,
        notConfigured: false,
        readinessSummaryTitle: paymentReadiness.summaryTitle,
        balanceStatusDesc: paymentReadiness.summaryDescription,
        readinessActionText: paymentReadiness.actionText,
        readinessActionPath: paymentReadiness.actionPath,
        withdrawals: records.withdrawals || [],
        refreshErrorMessage: ''
      })
    } catch (error) {
      const debugMessage = getErrorDebugMessage(error)
      if (debugMessage.includes('404')) {
        this.setData({
          initialLoading: false,
          initialError: false,
          initialErrorMessage: '',
          loading: false,
          loadedOnce: true,
          paymentReadiness: null,
          notConfigured: true,
          readinessSummaryTitle: '先完成收付通开户',
          balanceStatusDesc: '暂未查询到收付通账户，请先完成收付通进件和签约。',
          readinessActionText: '去完成开户',
          readinessActionPath: '/pages/merchant/settings/applyment/index',
          balance: EMPTY_BALANCE,
          withdrawals: [],
          refreshErrorMessage: ''
        })
        return
      }

      const message = getErrorUserMessage(error, '资金账户加载失败，请稍后重试')
      logger.error('Load merchant finance account failed', error, 'merchant-finance-home')

      if (hadTrustedData) {
        this.setData({
          initialLoading: false,
          initialError: false,
          initialErrorMessage: '',
          loading: false,
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
        return
      }

      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: message,
        loading: false,
        refreshErrorMessage: ''
      })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onRetryAccess() {
    void this.bootstrapPage()
  },

  onRetry() {
    if (!this.hasAccess()) {
      void this.bootstrapPage()
      return
    }

    void this.loadData()
  },

  onGoPaymentSetup() {
    const actionPath = this.data.readinessActionPath || '/pages/merchant/settings/applyment/index'
    wx.navigateTo({ url: actionPath })
  },

  onWithdrawAmountChange(e: WechatMiniprogram.CustomEvent<InputChangeDetail>) {
    this.setData({ withdrawAmountYuan: e.detail.value })
  },

  onWithdrawRemarkChange(e: WechatMiniprogram.CustomEvent<InputChangeDetail>) {
    this.setData({ withdrawRemark: e.detail.value })
  },

  onOpenWithdrawDialog() {
    if (this.data.notConfigured || !this.data.canManageMerchantApplyment) {
      return
    }

    this.setData({ isWithdrawDialogVisible: true })
  },

  onCloseWithdrawDialog() {
    if (this.data.submitting) {
      return
    }

    this.setData({ isWithdrawDialogVisible: false })
  },

  async onSubmitWithdraw() {
    if (!this.data.canManageMerchantApplyment) {
      wx.showToast({ title: '提现仅支持老板账号发起', icon: 'none' })
      return
    }

    if (this.data.submitting || this.data.notConfigured) {
      return
    }

    const amountYuan = Number(this.data.withdrawAmountYuan)
    if (!Number.isFinite(amountYuan) || amountYuan < 1) {
      wx.showToast({ title: '提现金额至少1元', icon: 'none' })
      return
    }

    if (!this.data.withdrawRemark.trim()) {
      wx.showToast({ title: '请输入提现备注', icon: 'none' })
      return
    }

    const amount = Math.round(amountYuan * 100)
    if (amount > this.data.balance.withdrawable_amount) {
      wx.showToast({ title: '超过可提现余额', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '提交中...' })

    try {
      const result = await createMerchantWithdraw({
        amount,
        remark: this.data.withdrawRemark.trim()
      })

      this.upsertWithdrawal(result.withdrawal)
      this.setData({ withdrawAmountYuan: '', withdrawRemark: '', isWithdrawDialogVisible: false })
      await this.loadData()
      wx.showModal({
        title: '提现申请已提交',
        content: this.getWithdrawCreatedMessage(result.withdrawal),
        showCancel: false,
        confirmText: '知道了'
      })
    } catch (error) {
      logger.error('Submit merchant withdraw failed', error, 'merchant-finance-home')
      wx.showToast({
        title: getErrorUserMessage(error, '提现申请失败，请稍后重试'),
        icon: 'none'
      })
    } finally {
      wx.hideLoading()
      this.setData({ submitting: false })
    }
  },

  async onRefreshWithdrawal(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id || this.data.withdrawSyncingId === id) {
      return
    }

    this.setData({ withdrawSyncingId: id })
    try {
      const record = await getMerchantWithdrawal(id)
      this.upsertWithdrawal(record)
      wx.showToast({ title: `状态已同步为${this.getStatusText(record.status)}`, icon: 'none' })
    } catch (error) {
      logger.error('Refresh merchant withdrawal failed', error, 'merchant-finance-home')
      wx.showToast({ title: getErrorUserMessage(error, '同步提现状态失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ withdrawSyncingId: 0 })
    }
  },

  upsertWithdrawal(withdrawal: MerchantWithdrawItem) {
    const next = [withdrawal, ...this.data.withdrawals.filter((item) => item.id !== withdrawal.id)]
    this.setData({ withdrawals: next.slice(0, 20) })
  },

  getWithdrawCreatedMessage(withdrawal: MerchantWithdrawItem): string {
    const parts = [`状态：${this.getStatusText(withdrawal.status)}`]
    if (withdrawal.out_request_no) {
      parts.push(`请求单号：${withdrawal.out_request_no}`)
    }
    if (withdrawal.withdraw_id) {
      parts.push(`微信提现单号：${withdrawal.withdraw_id}`)
    }
    if (withdrawal.reason) {
      parts.push(`原因：${withdrawal.reason}`)
    }
    parts.push('可在本页继续同步最新状态。')
    return parts.join('\n')
  },

  formatAmount(fen: number) {
    return (fen / 100).toFixed(2)
  },

  formatDateTime(value?: string) {
    if (!value) return '暂无'
    return value.replace('T', ' ').slice(0, 16)
  },

  getStatusText(status: string) {
    return getMerchantWithdrawStatusView(status).text
  },

  getStatusTheme(status: string) {
    return getMerchantWithdrawStatusView(status).theme
  },

  getWithdrawPanelTheme() {
    if (this.data.notConfigured) {
      return 'warning'
    }

    if (!this.data.canManageMerchantApplyment) {
      return 'default'
    }

    return 'success'
  },

  getWithdrawPanelText() {
    if (this.data.notConfigured) {
      return '未开通'
    }

    if (!this.data.canManageMerchantApplyment) {
      return '仅查看'
    }

    return '可提现'
  },

  canSubmitWithdraw() {
    return !this.data.notConfigured && this.data.canManageMerchantApplyment
  }
})