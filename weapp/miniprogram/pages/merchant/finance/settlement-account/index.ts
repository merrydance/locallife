import {
  getMerchantSettlementApplication,
  getSettlementVerifyResultText,
  getSettlementVerifyResultTheme,
  type MerchantSettlementAccountResponse,
  type SettlementApplicationResponse
} from '../../../../api/merchant-settlement-account'
import {
  fetchMerchantPaymentReadiness,
  type MerchantPaymentReadinessView
} from '../../../../services/merchant-payment'
import {
  canManageMerchantApplyment,
  ensureMerchantConsoleAccess
} from '../../../../utils/console-access'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type SettlementAccountPageOptions = {
  application_no?: string
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
    refreshErrorMessage: '',
    loading: false,
    refreshingApplication: false,
    paymentReadiness: null as MerchantPaymentReadinessView | null,
    settlementAccount: null as MerchantSettlementAccountResponse | null,
    settlementStatusDesc: '',
    applicationNo: '',
    applicationLoading: false,
    applicationError: false,
    applicationErrorMessage: '',
    applicationResult: null as SettlementApplicationResponse | null
  },

  async onLoad(options: SettlementAccountPageOptions) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight, applicationNo: String(options.application_no || '') })
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
    this.setData({ loading: true, initialError: false, initialErrorMessage: '', refreshErrorMessage: '' })
    try {
      const paymentReadiness = await fetchMerchantPaymentReadiness()
      const settlementAccount = paymentReadiness.settlementAccount
      const nextApplicationNo = String(this.data.applicationNo || settlementAccount?.latest_application_no || '')

      this.setData({
        paymentReadiness,
        settlementAccount,
        settlementStatusDesc: paymentReadiness.summaryDescription,
        applicationNo: nextApplicationNo,
        applicationResult: nextApplicationNo ? this.data.applicationResult : null,
        applicationLoading: false,
        applicationError: false,
        applicationErrorMessage: '',
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        loading: false
      })

      if (nextApplicationNo) {
        await this.loadApplicationResult(nextApplicationNo)
      }
    } catch (error) {
      logger.error('Load merchant settlement account failed', error, 'merchant-settlement-account')
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(error, '结算账户页面加载失败，请稍后重试'),
        loading: false
      })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  async loadApplicationResult(applicationNo: string) {
    if (!applicationNo) {
      return
    }

    this.setData({ applicationLoading: true, applicationError: false, applicationErrorMessage: '' })
    try {
      const applicationResult = await getMerchantSettlementApplication(applicationNo)
      this.setData({
        applicationResult,
        applicationNo,
        applicationLoading: false,
        applicationError: false,
        applicationErrorMessage: ''
      })
    } catch (error) {
      logger.error('Load settlement application result failed', error, 'merchant-settlement-account')
      this.setData({
        applicationResult: null,
        applicationLoading: false,
        applicationError: true,
        applicationErrorMessage: getErrorUserMessage(error, '结算账户申请结果加载失败，请稍后重试')
      })
    }
  },

  onRetryAccess() {
    void this.bootstrapPage()
  },

  onRetry() {
    void this.loadData()
  },

  onGoEditSettlementAccount() {
    if (!this.data.canManageMerchantApplyment) {
      wx.showToast({ title: '更换银行卡仅支持老板账号操作', icon: 'none' })
      return
    }

    if (!this.data.paymentReadiness?.canEditSettlementAccount) {
      wx.showToast({ title: '当前账户尚未激活，暂不可更换银行卡', icon: 'none' })
      return
    }

    wx.navigateTo({ url: '/pages/merchant/finance/settlement-account/edit/index' })
  },

  async onRefreshApplication() {
    if (!this.data.applicationNo || this.data.refreshingApplication) {
      return
    }

    this.setData({ refreshingApplication: true })
    try {
      await this.loadApplicationResult(this.data.applicationNo)
    } finally {
      this.setData({ refreshingApplication: false })
    }
  },

  onGoApplyment() {
    if (!this.data.canManageMerchantApplyment) {
      wx.showToast({ title: '收付通进件仅支持老板账号维护', icon: 'none' })
      return
    }

    wx.navigateTo({
      url: this.data.paymentReadiness?.actionPath || '/pages/merchant/settings/applyment/index'
    })
  },

  formatAccountNumber(value?: string) {
    const rawValue = String(value || '').replace(/\s+/g, '')
    if (!rawValue) {
      return '-'
    }

    if (rawValue.includes('*')) {
      return rawValue.replace(/(.{4})/g, '$1 ').trim()
    }

    if (rawValue.length <= 8) {
      return rawValue
    }

    return `${rawValue.slice(0, 4)} **** **** ${rawValue.slice(-4)}`.replace(/\s+/g, ' ').trim()
  },

  getAccountTypeText(accountType?: string) {
    return accountType === 'ACCOUNT_TYPE_PRIVATE' ? '对私账户' : '对公账户'
  },

  getVerifyResultText(result?: string) {
    return getSettlementVerifyResultText(result)
  },

  getVerifyResultTheme(result?: string) {
    return getSettlementVerifyResultTheme(result)
  }
})