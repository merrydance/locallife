import {
  buildMerchantApplymentStatusView,
  DEFAULT_MERCHANT_APPLYMENT_STATUS_VIEW,
  type ApplymentStatusResponse,
  getMerchantApplymentStatus
} from '../../../../api/merchant-applyment'
import {
  ensureMerchantApplymentAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../../utils/console-access'
import { saveApplymentQRCodePosterToAlbum } from '../../../../utils/applyment-qrcode'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

const APPLYMENT_AUTO_REFRESH_WINDOW_MS = 60 * 1000
const APPLYMENT_FORCE_REFRESH_STORAGE_KEY = 'merchantApplymentShouldRefresh'

const EMPTY_APPLYMENT: ApplymentStatusResponse = {
  status: '',
  status_desc: ''
}

interface ApplymentQRCodeDialogState {
  visible: boolean
  title: string
  description: string
  hint: string
  value: string
  saving: boolean
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

function getMiniProgramErrorMessage(error: unknown): string {
  if (typeof error === 'string') {
    return error
  }
  if (error && typeof error === 'object' && 'errMsg' in error && typeof error.errMsg === 'string') {
    return error.errMsg
  }
  if (error instanceof Error) {
    return error.message
  }
  return ''
}

function isPermissionDeniedError(error: unknown): boolean {
  const message = getMiniProgramErrorMessage(error)
  return message.includes('auth deny') || message.includes('auth denied')
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
    refreshingStatus: false,
    qrCodeDialog: {
      visible: false,
      title: '签约二维码',
      description: '',
      hint: '',
      value: '',
      saving: false
    } as ApplymentQRCodeDialogState
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
      this.data.loadingApplyment
    ) {
      return
    }

    const shouldForceRefresh = wx.getStorageSync(APPLYMENT_FORCE_REFRESH_STORAGE_KEY) === '1'
    if (shouldForceRefresh) {
      wx.removeStorageSync(APPLYMENT_FORCE_REFRESH_STORAGE_KEY)
      void this.loadApplyment({ silent: this.data.statusLoaded, force: true })
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
      refreshingStatus: false,
      qrCodeDialog: {
        visible: false,
        title: '签约二维码',
        description: '',
        hint: '',
        value: '',
        saving: false
      }
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
        qrCodeDialog: {
          ...this.data.qrCodeDialog,
          visible: false,
          saving: false
        }
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

  onOpenSubmitPage() {
    if (!this.data.applymentView.canSubmitOpenInfo) {
      wx.showToast({
        title: this.data.applymentView.blockReason || this.data.applymentView.guideText || '当前状态暂不支持重新提交',
        icon: 'none'
      })
      return
    }

    wx.navigateTo({ url: '/pages/merchant/settings/applyment/submit/index' })
  },

  openQRCodeDialog(payload: Omit<ApplymentQRCodeDialogState, 'visible' | 'saving'>) {
    if (!payload.value) {
      return
    }

    this.setData({
      qrCodeDialog: {
        ...payload,
        visible: true,
        saving: false
      }
    })
  },

  onShowSignQRCode() {
    this.openQRCodeDialog({
      title: '微信支付签约二维码',
      description: '请使用超级管理员微信扫码完成签约确认。',
      hint: '如果当前手机就是签约微信，可先保存二维码到相册，再尝试通过微信扫一扫从相册识别。',
      value: this.data.applymentView.signURL
    })
  },

  onShowLegalValidationQRCode() {
    this.openQRCodeDialog({
      title: '法人验证二维码',
      description: '请使用法人微信扫码完成验证。',
      hint: '如果当前手机就是法人验证微信，可先保存二维码到相册，再尝试通过微信扫一扫从相册识别。',
      value: this.data.applymentView.legalValidationURL
    })
  },

  onCloseQRCodeDialog() {
    if (this.data.qrCodeDialog.saving) {
      return
    }

    this.setData({
      'qrCodeDialog.visible': false,
      'qrCodeDialog.saving': false
    })
  },

  async onSaveQRCodeToAlbum() {
    if (this.data.qrCodeDialog.saving || !this.data.qrCodeDialog.value) {
      return
    }

    this.setData({ 'qrCodeDialog.saving': true })
    wx.showLoading({ title: '保存中...' })

    try {
      await saveApplymentQRCodePosterToAlbum({
        page: this,
        canvasSelector: '#applymentQrcodePosterCanvas',
        value: this.data.qrCodeDialog.value,
        title: this.data.qrCodeDialog.title,
        subtitle: this.data.qrCodeDialog.description
      })
      wx.showToast({ title: '二维码已保存到相册', icon: 'success' })
    } catch (error: unknown) {
      if (isPermissionDeniedError(error)) {
        wx.showModal({
          title: '需要相册权限',
          content: '请在设置中开启“保存到相册”权限后重试。',
          confirmText: '去设置',
          success: (result) => {
            if (result.confirm) {
              wx.openSetting()
            }
          }
        })
      } else {
        wx.showToast({
          title: getErrorMessage(error, '保存二维码失败，请稍后重试'),
          icon: 'none'
        })
      }
    } finally {
      wx.hideLoading()
      this.setData({ 'qrCodeDialog.saving': false })
    }
  },

  onCopyValidationAccountNumber() {
    copyText(this.data.applymentView.accountValidation?.destinationAccountNumber || '', '收款卡号已复制')
  },

  onCopyValidationRemark() {
    copyText(this.data.applymentView.accountValidation?.remark || '', '汇款备注已复制')
  },

  onGoSettlementAccount() {
    wx.navigateTo({ url: '/pages/merchant/finance/settlement-account/index' })
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