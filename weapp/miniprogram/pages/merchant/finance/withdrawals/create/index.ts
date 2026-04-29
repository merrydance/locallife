import { getMerchantAccountBalance } from '../../../../../api/merchant-finance'
import {
  buildAccountBalanceView,
  getMerchantFinanceUserMessage,
  submitMerchantWithdrawAndWait,
  type MerchantAccountBalanceView
} from '../../../../../services/merchant-finance-workflow'
import {
  ensureMerchantConsoleAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../../../utils/console-access'
import { logger } from '../../../../../utils/logger'
import { getStableBarHeights } from '../../../../../utils/responsive'

const DETAIL_PAGE_PATH = '/pages/merchant/finance/withdrawals/detail/index'

function parseYuanToFen(value: string): number {
  const text = String(value || '').trim()
  if (!/^\d+(\.\d{0,2})?$/.test(text)) {
    return 0
  }
  return Math.round(Number(text) * 100)
}

function getInputValue(detail: unknown): string {
  if (typeof detail === 'string') {
    return detail
  }
  if (detail && typeof detail === 'object' && 'value' in detail) {
    return String((detail as { value?: unknown }).value || '')
  }
  return ''
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    submitting: false,
    amountText: '',
    remark: '',
    formErrorMessage: '',
    submitResultMessage: '',
    balanceView: null as MerchantAccountBalanceView | null
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async bootstrapPage() {
    this.setData({ accessReady: false, accessDenied: false, accessErrorMessage: '', initialLoading: true, initialError: false, initialErrorMessage: '' })
    const accessResult = await ensureMerchantConsoleAccess()
    if (!isMerchantConsoleAccessGranted(accessResult)) {
      this.setData({ accessReady: true, accessDenied: isMerchantConsoleAccessDenied(accessResult), accessErrorMessage: getMerchantConsoleAccessErrorMessage(accessResult), initialLoading: false })
      return
    }
    this.setData({ accessReady: true, accessDenied: false, accessErrorMessage: '' })
    await this.loadBalance()
  },

  async loadBalance() {
    try {
      const balanceView = buildAccountBalanceView(await getMerchantAccountBalance())
      this.setData({ initialLoading: false, initialError: false, initialErrorMessage: '', balanceView })
    } catch (error) {
      logger.warn('Merchant withdrawal balance load failed', error)
      this.setData({ initialLoading: false, initialError: true, initialErrorMessage: getMerchantFinanceUserMessage(error, '账户余额加载失败，请稍后重试') })
    }
  },

  onRetryAccess() { void this.bootstrapPage() },
  onRetry() { void this.loadBalance() },

  onAmountChange(event: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({ amountText: getInputValue(event.detail), formErrorMessage: '', submitResultMessage: '' })
  },

  onRemarkChange(event: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({ remark: getInputValue(event.detail), formErrorMessage: '', submitResultMessage: '' })
  },

  async onSubmit() {
    if (this.data.submitting) return
    const amount = parseYuanToFen(this.data.amountText)
    const balance = this.data.balanceView
    if (!balance?.isActive) {
      this.setData({ formErrorMessage: balance?.statusDesc || '收付通账户未激活，暂不可提现' })
      return
    }
    if (amount <= 0) {
      this.setData({ formErrorMessage: '请输入有效提现金额' })
      return
    }
    if (amount > balance.withdrawableAmount.amount) {
      this.setData({ formErrorMessage: '提现金额不能超过可提现余额' })
      return
    }

    this.setData({ submitting: true, formErrorMessage: '', submitResultMessage: '提现申请已提交，正在同步后端结果...' })
    try {
      const result = await submitMerchantWithdrawAndWait({ amount, remark: this.data.remark || '商户提现' })
      const message = result.timedOut ? '提现已受理，最新状态可在详情页刷新查看' : `提现${result.withdrawal.status.text}`
      this.setData({ submitting: false, submitResultMessage: message })
      wx.redirectTo({ url: `${DETAIL_PAGE_PATH}?id=${result.withdrawal.id}` })
    } catch (error) {
      logger.warn('Merchant withdrawal submit failed', error)
      this.setData({ submitting: false, submitResultMessage: '', formErrorMessage: getMerchantFinanceUserMessage(error, '提现提交失败，请稍后重试') })
    }
  }
})