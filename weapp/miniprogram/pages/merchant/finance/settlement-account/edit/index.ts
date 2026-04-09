import {
  getMerchantSettlementAccount,
  getSettlementAccountStatusView,
  modifyMerchantSettlementAccount,
  type MerchantSettlementAccountResponse
} from '../../../../../api/merchant-settlement-account'
import type { ApplymentBindBankPayload } from '../../../../../api/applyment-bank'
import { ensureMerchantApplymentAccess } from '../../../../../utils/console-access'
import { logger } from '../../../../../utils/logger'
import { getStableBarHeights } from '../../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../../utils/user-facing'

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessDeniedMessage: '',
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    submitting: false,
    canEdit: false,
    settlementAccount: null as MerchantSettlementAccountResponse | null,
    settlementStatusDesc: '',
    bindBankDraft: null as ApplymentBindBankPayload | null
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
      accessDeniedMessage: '',
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      submitting: false,
      canEdit: false,
      bindBankDraft: null
    })

    const accessResult = await ensureMerchantApplymentAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessDeniedMessage: accessResult.status === 'denied' ? accessResult.message : '',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
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
    this.setData({ initialLoading: true, initialError: false, initialErrorMessage: '' })
    try {
      const settlementAccount = await getMerchantSettlementAccount()
      const statusView = getSettlementAccountStatusView(settlementAccount.account_status, settlementAccount.status_desc)
      this.setData({
        settlementAccount,
        settlementStatusDesc: statusView.statusDesc,
        canEdit: statusView.isActive,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: ''
      })
    } catch (error) {
      logger.error('Load settlement account edit page failed', error, 'merchant-settlement-account-edit')
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(error, '提现银行卡编辑页加载失败，请稍后重试')
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

  onBindDraftChange(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    this.setData({ bindBankDraft: e.detail })
  },

  onCancelEdit() {
    if (this.data.submitting) {
      return
    }

    wx.navigateBack()
  },

  async onSubmitForm(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    if (this.data.submitting || !this.data.canEdit) {
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '提交中...' })

    try {
      const payload = e.detail
      const result = await modifyMerchantSettlementAccount({
        account_type: payload.account_type,
        account_bank: payload.account_bank,
        bank_name: payload.bank_name,
        bank_branch_id: payload.bank_branch_id,
        account_number: payload.account_number,
        account_name: payload.account_name
      })

      wx.redirectTo({
        url: `/pages/merchant/finance/settlement-account/index?application_no=${encodeURIComponent(result.application_no)}`
      })
    } catch (error) {
      logger.error('Submit settlement account edit failed', error, 'merchant-settlement-account-edit')
      wx.showToast({
        title: getErrorUserMessage(error, '更换银行卡提交失败，请稍后重试'),
        icon: 'none'
      })
    } finally {
      wx.hideLoading()
      this.setData({ submitting: false })
    }
  },

  onGoApplyment() {
    wx.navigateTo({ url: '/pages/merchant/settings/applyment/index' })
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
  }
})