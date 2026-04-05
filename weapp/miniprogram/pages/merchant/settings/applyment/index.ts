import {
  ApplymentStatusResponse,
  getMerchantApplymentStatus,
  merchantBindBank
} from '../../../../api/merchant-finance'
import type { ApplymentBindBankPayload } from '../../../../api/applyment-bank'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'

const APPLYMENT_AUTO_REFRESH_WINDOW_MS = 60 * 1000

const EMPTY_APPLYMENT: ApplymentStatusResponse = {
  status: '',
  status_desc: ''
}

function hasExistingApplyment(status?: string) {
  return Boolean(status && status !== 'not_applied' && status !== 'pending')
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

function canEditApplyment(status?: string) {
  return !status || status === 'not_applied' || status === 'pending' || status === 'rejected' || status === 'rejected_sign'
}

function resolveCanSubmitApplyment(data: ApplymentStatusResponse) {
  if (typeof data.can_submit === 'boolean') {
    return data.can_submit
  }
  return canEditApplyment(data.status)
}

function isCompletedApplyment(status?: string) {
  return status === 'finish' || status === 'active'
}

function getApplymentStatusText(status?: string) {
  switch (status) {
    case 'not_applied':
    case 'pending':
      return '尚未提交进件资料'
    case 'submitted':
      return '已提交'
    case 'bindbank_submitted':
      return '进件审核中'
    case 'auditing':
      return '审核中'
    case 'to_be_signed':
      return '待签约'
    case 'signing':
      return '签约中'
    case 'finish':
    case 'active':
      return '已开通'
    case 'rejected':
      return '已拒绝'
    case 'rejected_sign':
      return '签约被拒绝'
    case 'frozen':
      return '已冻结'
    default:
      return '状态更新中'
  }
}

function getApplymentStatusTheme(status?: string) {
  switch (status) {
    case 'finish':
    case 'active':
      return 'success'
    case 'rejected':
    case 'rejected_sign':
    case 'frozen':
      return 'danger'
    case 'to_be_signed':
    case 'signing':
      return 'primary'
    default:
      return 'warning'
  }
}

function getApplymentActionLabel(status?: string) {
  if (!hasExistingApplyment(status)) {
    return '填写进件资料'
  }
  if (status === 'rejected' || status === 'rejected_sign') {
    return '重新提交资料'
  }
  return '填写进件资料'
}

function getApplymentActionHint(status?: string) {
  if (!status || status === 'not_applied' || status === 'pending' || status === 'rejected' || status === 'rejected_sign') {
    return ''
  }
  if (status === 'submitted' || status === 'auditing' || status === 'bindbank_submitted') {
    return '当前资料正在审核中，暂不支持重复提交。若状态长时间未更新，可点击“刷新状态”重新拉取结果。'
  }
  if (status === 'to_be_signed' || status === 'signing') {
    return '当前已进入微信签约环节，请先完成签约，再点击“刷新状态”查看最新结果。'
  }
  if (status === 'finish' || status === 'active') {
    return '当前账户已开通，无需重复提交进件资料。余额、提现和结算结果请前往资金账户查看。'
  }
  return '当前状态暂不支持重新提交资料，请先刷新状态。'
}

function getApplymentActionHintFromResponse(data: ApplymentStatusResponse) {
  if (data.block_reason) {
    return data.block_reason
  }
  return getApplymentActionHint(data.status)
}

function getApplymentNoticeTheme(status?: string) {
  if (status === 'rejected' || status === 'rejected_sign') {
    return 'hint-card-error'
  }
  if (status === 'to_be_signed' || status === 'signing') {
    return 'hint-card-warn'
  }
  return 'hint-card-info'
}

function getApplymentNoticeThemeFromResponse(data: ApplymentStatusResponse) {
  if (data.block_reason && data.can_submit === false && data.status !== 'to_be_signed' && data.status !== 'signing') {
    return 'hint-card-warn'
  }
  return getApplymentNoticeTheme(data.status)
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    loading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loadingApplyment: true,
    lastLoadedAt: 0,
    applymentLoaded: false,
    applymentStatus: EMPTY_APPLYMENT as ApplymentStatusResponse | null,
    hasApplyment: false,
    canEditCurrentApplyment: true,
    applymentActionLabel: '填写进件资料',
    applymentActionHint: '',
    pageNoticeThemeClass: 'hint-card-info',
    bindBankDraft: null as ApplymentBindBankPayload | null,
    showBindForm: false,
    submittingBind: false,
    refreshingStatus: false,
    allowCompletedView: false
  },

  async onLoad(options: Record<string, string>) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({
      navBarHeight,
      allowCompletedView: options.allowCompletedView === '1'
    })

    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })
    if (accessResult.status !== 'granted') {
      this.setData({ loading: false, loadingApplyment: false })
      return
    }

    this.loadApplyment()
  },

  onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      loading: true,
      loadingApplyment: true,
      initialError: false,
      initialErrorMessage: ''
    })
    this.onLoad({ allowCompletedView: this.data.allowCompletedView ? '1' : '0' })
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (!this.data.loading && !this.data.submittingBind && shouldAutoRefresh(this.data.lastLoadedAt, APPLYMENT_AUTO_REFRESH_WINDOW_MS)) {
      this.loadApplyment(true)
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadApplyment(this.data.applymentLoaded, true)
  },

  async loadApplyment(silent = false, force = false) {
    const hasExistingData = this.data.applymentLoaded || this.data.lastLoadedAt > 0
    if (!force && hasExistingData && !shouldAutoRefresh(this.data.lastLoadedAt, APPLYMENT_AUTO_REFRESH_WINDOW_MS)) {
      wx.stopPullDownRefresh()
      return
    }

    if (!silent) {
      this.setData({ loading: true, initialError: false, initialErrorMessage: '', refreshErrorMessage: '' })
    } else if (hasExistingData) {
      this.setData({ refreshErrorMessage: '' })
    }
    this.setData({ loadingApplyment: true })

    try {
      const data = await getMerchantApplymentStatus()
      const status = data.status || ''
      if (isCompletedApplyment(status) && !this.data.allowCompletedView) {
        wx.redirectTo({ url: '/pages/merchant/settings/applyment/completed/index' })
        return
      }

      const exists = hasExistingApplyment(status)
      const canEdit = resolveCanSubmitApplyment(data)

      this.setData({
        applymentStatus: data,
        hasApplyment: exists,
        applymentLoaded: true,
        loading: false,
        loadingApplyment: false,
        canEditCurrentApplyment: canEdit,
        applymentActionLabel: getApplymentActionLabel(status),
        applymentActionHint: getApplymentActionHintFromResponse(data),
        pageNoticeThemeClass: getApplymentNoticeThemeFromResponse(data),
        bindBankDraft: canEdit ? this.data.bindBankDraft : null,
        showBindForm: canEdit ? this.data.showBindForm : false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (error: unknown) {
      logger.error('Load merchant applyment page failed', error, 'merchant-applyment-page')
      const message = getErrorMessage(error, '进件状态加载失败，请稍后重试')
      if (!silent || !hasExistingData) {
        this.setData({
          loading: false,
          loadingApplyment: false,
          applymentLoaded: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: ''
        })
      } else {
        this.setData({
          loading: false,
          loadingApplyment: false,
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      }
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onShowBindForm() {
    if (!this.data.canEditCurrentApplyment) {
      wx.showToast({ title: this.data.applymentActionHint || '当前状态暂不支持重提资料', icon: 'none' })
      return
    }
    this.setData({ showBindForm: true })
  },

  onHideBindForm() {
    this.setData({ showBindForm: false })
  },

  onBindDraftChange(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    this.setData({ bindBankDraft: e.detail })
  },

  async onSubmitBindBank(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    if (this.data.submittingBind) return

    this.setData({ submittingBind: true })
    wx.showLoading({ title: '提交中...' })

    try {
      await merchantBindBank(e.detail)

      this.setData({
        bindBankDraft: null,
        showBindForm: false
      })
      await this.loadApplyment(true, true)
    } catch (error) {
      logger.error('Submit merchant applyment bind bank failed', error, 'merchant-applyment-page')
      wx.showToast({ title: getErrorMessage(error, '提交进件资料失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ submittingBind: false })
    }
  },

  onCopySignUrl() {
    const signUrl = this.data.applymentStatus?.sign_url
    if (!signUrl) return

    wx.setClipboardData({
      data: signUrl,
      success: () => {
        wx.showToast({ title: '签约链接已复制', icon: 'success' })
      }
    })
  },

  async onRefreshStatus() {
    if (this.data.refreshingStatus || this.data.loadingApplyment) return

    this.setData({ refreshingStatus: true })
    try {
      await this.loadApplyment(true, true)
    } finally {
      this.setData({ refreshingStatus: false })
    }
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.onRefreshStatus()
  },

  onGoFinance() {
    wx.navigateTo({ url: '/pages/merchant/finance/index' })
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadApplyment()
  },

  getApplymentStatusText(status?: string) {
    return getApplymentStatusText(status)
  },

  getApplymentStatusTheme(status?: string) {
    return getApplymentStatusTheme(status)
  }
})