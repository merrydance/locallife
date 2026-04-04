import {
  ApplymentStatusResponse,
  getMerchantApplymentStatus,
  merchantBindBank
} from '../../../../api/merchant-finance'
import type { ApplymentBindBankPayload } from '../../../../api/applyment-bank'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

const EMPTY_APPLYMENT: ApplymentStatusResponse = {
  status: '',
  status_desc: ''
}

function hasExistingApplyment(status?: string) {
  return Boolean(status && status !== 'not_applied' && status !== 'pending')
}

function canEditApplyment(status?: string) {
  return !status || status === 'not_applied' || status === 'pending' || status === 'rejected' || status === 'rejected_sign'
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

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    initialError: false,
    initialErrorMessage: '',
    loadingApplyment: true,
    applymentLoaded: false,
    applymentStatus: EMPTY_APPLYMENT as ApplymentStatusResponse | null,
    hasApplyment: false,
    canEditCurrentApplyment: true,
    applymentActionLabel: '填写进件资料',
    applymentActionHint: '',
    showBindForm: false,
    submittingBind: false,
    refreshingStatus: false
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadApplyment()
  },

  onShow() {
    if (!this.data.loading && !this.data.submittingBind) {
      this.loadApplyment(true)
    }
  },

  onPullDownRefresh() {
    this.loadApplyment()
  },

  async loadApplyment(silent = false) {
    if (!silent) {
      this.setData({ loading: true, initialError: false, initialErrorMessage: '' })
    }
    this.setData({ loadingApplyment: true })

    try {
      const data = await getMerchantApplymentStatus()
      const status = data.status || ''
      const exists = hasExistingApplyment(status)
      const canEdit = canEditApplyment(status)

      this.setData({
        applymentStatus: data,
        hasApplyment: exists,
        applymentLoaded: true,
        loading: false,
        loadingApplyment: false,
        canEditCurrentApplyment: canEdit,
        applymentActionLabel: getApplymentActionLabel(status),
        applymentActionHint: getApplymentActionHint(status),
        showBindForm: canEdit ? this.data.showBindForm : false,
        initialError: false,
        initialErrorMessage: ''
      })
    } catch (error: unknown) {
      logger.error('Load merchant applyment page failed', error, 'merchant-applyment-page')
      const message = getErrorMessage(error, '进件状态加载失败，请稍后重试')
      if (!silent || !this.data.applymentLoaded) {
        this.setData({
          loading: false,
          loadingApplyment: false,
          applymentLoaded: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else {
        this.setData({ loading: false, loadingApplyment: false })
        wx.showToast({ title: message, icon: 'none' })
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

  async onSubmitBindBank(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    if (this.data.submittingBind) return

    this.setData({ submittingBind: true })
    wx.showLoading({ title: '提交中...' })

    try {
      await merchantBindBank(e.detail)

      this.setData({
        showBindForm: false
      })
      await this.loadApplyment(true)
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
      await this.loadApplyment(true)
    } finally {
      this.setData({ refreshingStatus: false })
    }
  },

  onGoFinance() {
    wx.navigateTo({ url: '/pages/merchant/finance/index' })
  },

  onRetry() {
    this.loadApplyment()
  },

  getApplymentStatusText(status: string): string {
    const map: Record<string, string> = {
      submitted: '已提交',
      bindbank_submitted: '进件审核中',
      auditing: '审核中',
      to_be_signed: '待签约',
      signing: '签约中',
      finish: '已开通',
      active: '已开通',
      rejected: '已拒绝'
    }
    return map[status] || status || '未开始'
  },

  getApplymentStatusTheme(status: string): string {
    switch (status) {
      case 'finish':
      case 'active':
        return 'success'
      case 'rejected':
        return 'danger'
      case 'to_be_signed':
      case 'signing':
        return 'primary'
      default:
        return 'warning'
    }
  },

  getProgressCurrent(status?: string) {
    if (!status) return 0
    if (status === 'submitted' || status === 'bindbank_submitted' || status === 'auditing') return 1
    if (status === 'to_be_signed' || status === 'signing') return 2
    if (status === 'finish' || status === 'active') return 3
    if (status === 'rejected') return 2
    return 0
  }
})