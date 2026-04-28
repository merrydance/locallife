import type { ApplymentAccountType, ApplymentBindBankPayload } from '../../../../../api/applyment-bank'
import {
  buildMerchantSettlementAccountView,
  buildMerchantSettlementApplicationView,
  getMerchantSettlementAccount,
  getMerchantSettlementApplication,
  modifyMerchantSettlementAccount,
  type MerchantSettlementAccountInfo
} from '../../../../../api/merchant-settlement-account'
import {
  ensureMerchantApplymentAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../../../utils/console-access'
import { logger } from '../../../../../utils/logger'
import { getStableBarHeights } from '../../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../../utils/user-facing'

const APPLYMENT_FORCE_REFRESH_STORAGE_KEY = 'merchantApplymentShouldRefresh'
const EMPTY_ACCOUNT_VIEW = buildMerchantSettlementAccountView(null)
const EMPTY_APPLICATION_VIEW = buildMerchantSettlementApplicationView(null)

let accountRequestPending = false
let applicationRequestPending = false

function normalizeAccountType(value?: string): ApplymentAccountType {
  return value === 'ACCOUNT_TYPE_BUSINESS' ? 'ACCOUNT_TYPE_BUSINESS' : 'ACCOUNT_TYPE_PRIVATE'
}

function buildBankDraft(account?: MerchantSettlementAccountInfo) {
  const accountType = normalizeAccountType(account?.account_type)
  return {
    account_type: accountType,
    account_bank: account?.account_bank || '',
    account_bank_code: 0,
    bank_alias: '',
    bank_alias_code: '',
    need_bank_branch: Boolean(account?.bank_name || account?.bank_branch_id),
    bank_address_code: '',
    bank_branch_id: account?.bank_branch_id || '',
    bank_name: account?.bank_name || '',
    account_number: '',
    account_name: ''
  }
}

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
    refreshErrorMessage: '',
    accountLoading: false,
    accountLoaded: false,
    submitting: false,
    applicationLoading: false,
    applicationErrorMessage: '',
    settlementAccountView: { ...EMPTY_ACCOUNT_VIEW },
    settlementApplicationView: { ...EMPTY_APPLICATION_VIEW },
    currentAccountItems: [] as Array<{ label: string, value: string }>,
    bankDraft: buildBankDraft(),
    canEditSettlementAccount: false,
    applicationNo: ''
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    if (!this.hasAccess()) {
      wx.stopPullDownRefresh()
      return
    }
    if (this.data.applicationNo) {
      void this.loadSettlementApplication(this.data.applicationNo)
      return
    }
    void this.loadSettlementAccount({ force: true })
  },

  onShow() {
    if (
      !this.hasAccess() ||
      this.data.initialLoading ||
      this.data.submitting ||
      this.data.applicationLoading ||
      !this.data.applicationNo
    ) {
      return
    }
    void this.loadSettlementApplication(this.data.applicationNo)
  },

  hasAccess() {
    return this.data.accessReady && !this.data.accessDenied && !this.data.accessErrorMessage
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
      refreshErrorMessage: '',
      accountLoaded: false,
      applicationErrorMessage: '',
      settlementAccountView: { ...EMPTY_ACCOUNT_VIEW },
      settlementApplicationView: { ...EMPTY_APPLICATION_VIEW },
      currentAccountItems: [],
      bankDraft: buildBankDraft(),
      canEditSettlementAccount: false,
      applicationNo: ''
    })

    const accessResult = await ensureMerchantApplymentAccess()
    if (!isMerchantConsoleAccessGranted(accessResult)) {
      this.setData({
        accessReady: true,
        accessDenied: isMerchantConsoleAccessDenied(accessResult),
        accessDeniedMessage: accessResult.status === 'denied' ? accessResult.message : '',
        accessErrorMessage: getMerchantConsoleAccessErrorMessage(accessResult),
        initialLoading: false
      })
      return
    }

    this.setData({
      accessReady: true,
      accessDenied: false,
      accessDeniedMessage: '',
      accessErrorMessage: ''
    })
    await this.loadSettlementAccount({ force: true })
  },

  async loadSettlementAccount(options?: { force?: boolean }) {
    const { force = false } = options || {}
    if (!this.hasAccess()) {
      wx.stopPullDownRefresh()
      return
    }
    if (accountRequestPending || (!force && !this.data.initialLoading)) {
      wx.stopPullDownRefresh()
      return
    }

    accountRequestPending = true
    const hasTrustedData = this.data.accountLoaded
    this.setData({
      accountLoading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: ''
    })

    try {
      const response = await getMerchantSettlementAccount()
      const accountView = buildMerchantSettlementAccountView(response)
      this.setData({
        settlementAccountView: accountView,
        currentAccountItems: accountView.items.filter((item) => item.label !== '最新申请单'),
        bankDraft: buildBankDraft(response.account),
        canEditSettlementAccount: accountView.canEditSettlementAccount,
        initialLoading: false,
        accountLoaded: true,
        accountLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (error: unknown) {
      logger.error('Load settlement account modify page failed', error, 'merchant-settlement-account-page')
      const message = getErrorUserMessage(error, '结算账户加载失败，请稍后重试')
      if (hasTrustedData) {
        this.setData({
          accountLoading: false,
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      } else {
        this.setData({
          initialLoading: false,
          accountLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      }
    } finally {
      accountRequestPending = false
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    void this.loadSettlementAccount({ force: true })
  },

  onRetryAccess() {
    void this.bootstrapPage()
  },

  onBack() {
    wx.navigateBack()
  },

  async loadSettlementApplication(applicationNo: string) {
    const normalizedApplicationNo = String(applicationNo || '').trim()
    if (!normalizedApplicationNo || applicationRequestPending) {
      return
    }

    applicationRequestPending = true
    this.setData({
      applicationLoading: true,
      applicationErrorMessage: '',
      settlementApplicationView: buildMerchantSettlementApplicationView(null, normalizedApplicationNo)
    })

    try {
      const response = await getMerchantSettlementApplication(normalizedApplicationNo)
      this.setData({
        settlementApplicationView: buildMerchantSettlementApplicationView(response, normalizedApplicationNo),
        applicationLoading: false,
        applicationErrorMessage: ''
      })
    } catch (error: unknown) {
      logger.error('Load settlement account application failed', error, 'merchant-settlement-account-page')
      this.setData({
        applicationLoading: false,
        applicationErrorMessage: getErrorUserMessage(error, '修改申请状态加载失败，请稍后重试')
      })
    } finally {
      applicationRequestPending = false
      wx.stopPullDownRefresh()
    }
  },

  onRetryApplication() {
    void this.loadSettlementApplication(this.data.applicationNo)
  },

  async onSubmitSettlementAccount(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    if (this.data.submitting || !this.data.canEditSettlementAccount || this.data.applicationNo) {
      return
    }

    const payload = e.detail
    this.setData({ submitting: true, applicationErrorMessage: '' })

    try {
      const response = await modifyMerchantSettlementAccount({
        account_type: payload.account_type,
        account_bank: payload.account_bank,
        need_bank_branch: payload.need_bank_branch,
        bank_name: payload.bank_name,
        bank_branch_id: payload.bank_branch_id,
        account_number: payload.account_number,
        account_name: payload.account_name
      })
      const applicationNo = response.application_no
      wx.setStorageSync(APPLYMENT_FORCE_REFRESH_STORAGE_KEY, '1')
      this.setData({
        submitting: false,
        applicationNo,
        settlementApplicationView: buildMerchantSettlementApplicationView(null, applicationNo)
      })
      wx.showToast({ title: '修改申请已提交', icon: 'success' })
      void this.loadSettlementApplication(applicationNo)
    } catch (error: unknown) {
      logger.error('Submit settlement account modify failed', error, 'merchant-settlement-account-page')
      this.setData({ submitting: false })
      wx.showToast({
        title: getErrorUserMessage(error, '结算账户修改提交失败，请稍后重试'),
        icon: 'none'
      })
    }
  }
})