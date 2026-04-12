import {
  buildMerchantApplymentStatusView,
  DEFAULT_MERCHANT_APPLYMENT_STATUS_VIEW,
  type ApplymentStatusResponse,
  getMerchantApplymentStatus,
  merchantBindBank
} from '../../../../api/merchant-applyment'
import type { ApplymentBindBankDraftPayload, ApplymentBindBankPayload } from '../../../../api/applyment-bank'
import {
  ensureMerchantApplymentAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../../utils/console-access'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

const APPLYMENT_AUTO_REFRESH_WINDOW_MS = 60 * 1000

const EMPTY_APPLYMENT: ApplymentStatusResponse = {
  status: '',
  status_desc: ''
}

const getErrorMessage = getErrorUserMessage

let applymentRequestPending = false

function copyText(data: string, successTitle: string) {
  const trimmed = String(data || '').trim()
  if (!trimmed) {
    return
  }

  wx.setClipboardData({
    data: trimmed,
    success: () => {
      wx.showToast({ title: successTitle, icon: 'success' })
    }
  })
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
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
    loadingApplyment: false,
    lastLoadedAt: 0,
    statusLoaded: false,
    applymentStatus: EMPTY_APPLYMENT as ApplymentStatusResponse | null,
    applymentView: { ...DEFAULT_MERCHANT_APPLYMENT_STATUS_VIEW },
    bindBankDraft: null as ApplymentBindBankDraftPayload | null,
    showBindForm: false,
    submittingBind: false,
    refreshingStatus: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage()
  },

  onShow() {
    if (
      !this.data.accessReady ||
      this.data.accessDenied ||
      this.data.accessErrorMessage ||
      this.data.initialLoading ||
      this.data.submittingBind ||
      this.data.showBindForm
    ) {
      return
    }

    if (!shouldAutoRefresh(this.data.lastLoadedAt, APPLYMENT_AUTO_REFRESH_WINDOW_MS)) {
      return
    }

    void this.loadApplyment({ silent: this.data.statusLoaded })
  },

  onPullDownRefresh() {
    if (!this.hasApplymentAccess()) {
      wx.stopPullDownRefresh()
      return
    }

    void this.loadApplyment({
      silent: this.data.statusLoaded,
      force: true
    })
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
      loadingApplyment: false,
      lastLoadedAt: 0,
      statusLoaded: false,
      applymentStatus: EMPTY_APPLYMENT,
      applymentView: { ...DEFAULT_MERCHANT_APPLYMENT_STATUS_VIEW },
      bindBankDraft: null,
      showBindForm: false,
      submittingBind: false,
      refreshingStatus: false
    })

    const accessResult = await ensureMerchantApplymentAccess()
    if (!isMerchantConsoleAccessGranted(accessResult)) {
      this.setData({
        accessReady: true,
        accessDenied: isMerchantConsoleAccessDenied(accessResult),
        accessDeniedMessage: accessResult.status === 'denied' ? accessResult.message : '',
        accessErrorMessage: getMerchantConsoleAccessErrorMessage(accessResult),
        initialLoading: false,
        loadingApplyment: false
      })
      return
    }

    this.setData({
      accessReady: true,
      accessDenied: false,
      accessDeniedMessage: '',
      accessErrorMessage: ''
    })

    await this.loadApplyment({ force: true })
  },

  hasApplymentAccess() {
    return this.data.accessReady && !this.data.accessDenied && !this.data.accessErrorMessage
  },

  async loadApplyment(options?: { silent?: boolean, force?: boolean }) {
    const { silent = false, force = false } = options || {}
    if (!this.hasApplymentAccess()) {
      wx.stopPullDownRefresh()
      return
    }

    if (applymentRequestPending) {
      wx.stopPullDownRefresh()
      return
    }

    const hasTrustedData = this.data.statusLoaded
    if (!force && hasTrustedData && !shouldAutoRefresh(this.data.lastLoadedAt, APPLYMENT_AUTO_REFRESH_WINDOW_MS)) {
      wx.stopPullDownRefresh()
      return
    }

    applymentRequestPending = true

    this.setData({
      loadingApplyment: true,
      ...(hasTrustedData || silent
        ? { refreshErrorMessage: '' }
        : {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          })
    })

    try {
      const applymentStatus = await getMerchantApplymentStatus()
      const applymentView = buildMerchantApplymentStatusView(applymentStatus)

      this.setData({
        applymentStatus,
        applymentView,
        statusLoaded: true,
        loadingApplyment: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now(),
        showBindForm: applymentView.canSubmitOpenInfo ? this.data.showBindForm : false,
        bindBankDraft: applymentView.canSubmitOpenInfo ? this.data.bindBankDraft : null
      })
    } catch (error: unknown) {
      logger.error('Load merchant applyment page failed', error, 'merchant-applyment-page')
      const message = getErrorMessage(error, '进件状态加载失败，请稍后重试')

      if (!hasTrustedData) {
        this.setData({
          loadingApplyment: false,
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: '',
          statusLoaded: false
        })
      } else {
        this.setData({
          loadingApplyment: false,
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      }
    } finally {
      applymentRequestPending = false
      wx.stopPullDownRefresh()
    }
  },

  onRetryAccess() {
    void this.bootstrapPage()
  },

  onRetry() {
    if (!this.hasApplymentAccess()) {
      void this.bootstrapPage()
      return
    }

    void this.loadApplyment({ force: true })
  },

  onRetryRefresh() {
    if (!this.hasApplymentAccess()) {
      wx.stopPullDownRefresh()
      return
    }

    void this.loadApplyment({
      silent: true,
      force: true
    })
  },

  onShowBindForm() {
    if (!this.data.applymentView.canSubmitOpenInfo) {
      wx.showToast({
        title: this.data.applymentView.blockReason || this.data.applymentView.guideText || '当前状态暂不支持重新提交',
        icon: 'none'
      })
      return
    }

    this.setData({ showBindForm: true })
  },

  onHideBindForm() {
    this.setData({ showBindForm: false })
  },

  onBindDraftChange(e: WechatMiniprogram.CustomEvent<ApplymentBindBankDraftPayload>) {
    this.setData({ bindBankDraft: e.detail })
  },

  async onSubmitBindBank(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    if (this.data.submittingBind) {
      return
    }

    this.setData({ submittingBind: true })
    wx.showLoading({ title: '提交中...' })

    try {
      await merchantBindBank(e.detail)
      this.setData({
        bindBankDraft: null,
        showBindForm: false
      })
      await this.loadApplyment({
        silent: true,
        force: true
      })
      wx.showToast({ title: '进件资料已提交', icon: 'success' })
    } catch (error: unknown) {
      logger.error('Submit merchant applyment bind bank failed', error, 'merchant-applyment-page')
      wx.showToast({ title: getErrorMessage(error, '提交进件资料失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ submittingBind: false })
    }
  },

  onCopySignUrl() {
    const signURL = this.data.applymentView.signURL
    copyText(signURL, '签约链接已复制')
  },

  onCopyLegalValidationUrl() {
    copyText(this.data.applymentView.legalValidationURL, '验证链接已复制')
  },

  onCopyValidationAccountNumber() {
    copyText(this.data.applymentView.accountValidation?.destinationAccountNumber || '', '收款卡号已复制')
  },

  onCopyValidationRemark() {
    copyText(this.data.applymentView.accountValidation?.remark || '', '汇款备注已复制')
  },

  async onRefreshStatus() {
    if (this.data.refreshingStatus || this.data.loadingApplyment) {
      return
    }

    this.setData({ refreshingStatus: true })
    try {
      await this.loadApplyment({
        silent: true,
        force: true
      })
    } finally {
      this.setData({ refreshingStatus: false })
    }
  }
})