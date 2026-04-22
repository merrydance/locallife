import {
  buildMerchantApplymentWorkflowView,
  fetchMerchantApplymentWorkflowView,
  type MerchantApplymentTaskIntent,
  type MerchantApplymentTaskType,
  type MerchantApplymentWorkflowSecondaryTask
} from '../../../../../services/merchant-applyment-workflow'
import {
  ensureMerchantApplymentAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../../../utils/console-access'
import { saveApplymentQRCodePosterToAlbum } from '../../../../../utils/applyment-qrcode'
import { logger } from '../../../../../utils/logger'
import { getStableBarHeights } from '../../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../../utils/user-facing'

const HOME_PAGE_PATH = '/pages/merchant/settings/applyment/index'
const EMPTY_WORKFLOW_VIEW = buildMerchantApplymentWorkflowView(null)

const getErrorMessage = getErrorUserMessage

let workflowRequestPending = false

function normalizePreferredTaskType(task?: string): MerchantApplymentTaskType | '' {
  switch (String(task || '').trim()) {
    case 'sign_agreement':
    case 'merchant_confirmation':
    case 'legal_validation':
    case 'bank_transfer_validation':
      return String(task) as MerchantApplymentTaskType
    default:
      return ''
  }
}

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

function resolveCopySuccessTitle(taskType: string) {
  switch (taskType) {
    case 'copy_validation_account':
      return '收款卡号已复制'
    case 'copy_validation_remark':
      return '汇款备注已复制'
    default:
      return '内容已复制'
  }
}

function goBackHome() {
  wx.redirectTo({ url: HOME_PAGE_PATH })
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
    loadingWorkflow: false,
    workflowLoaded: false,
    preferredTaskType: '',
    workflowView: { ...EMPTY_WORKFLOW_VIEW },
    refreshingStatus: false,
    savingQRCode: false
  },

  async onLoad(query: Record<string, string>) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({
      navBarHeight,
      preferredTaskType: normalizePreferredTaskType(query.task)
    })
    await this.bootstrapPage()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onShow() {
    if (
      !this.data.accessReady ||
      this.data.accessDenied ||
      this.data.accessErrorMessage ||
      this.data.initialLoading ||
      this.data.loadingWorkflow ||
      !this.data.workflowLoaded
    ) {
      return
    }

    if (this.data.workflowView.reentryPolicy === 'force_refresh_on_show') {
      void this.loadWorkflow({ silent: true, force: true })
    }
  },

  onPullDownRefresh() {
    if (!this.hasApplymentAccess()) {
      wx.stopPullDownRefresh()
      return
    }

    void this.loadWorkflow({ silent: this.data.workflowLoaded, force: true })
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
      loadingWorkflow: false,
      workflowLoaded: false,
      workflowView: { ...EMPTY_WORKFLOW_VIEW },
      refreshingStatus: false,
      savingQRCode: false
    })

    const accessResult = await ensureMerchantApplymentAccess()
    if (!isMerchantConsoleAccessGranted(accessResult)) {
      this.setData({
        accessReady: true,
        accessDenied: isMerchantConsoleAccessDenied(accessResult),
        accessDeniedMessage: accessResult.status === 'denied' ? accessResult.message : '',
        accessErrorMessage: getMerchantConsoleAccessErrorMessage(accessResult),
        initialLoading: false,
        loadingWorkflow: false
      })
      return
    }

    this.setData({
      accessReady: true,
      accessDenied: false,
      accessDeniedMessage: '',
      accessErrorMessage: ''
    })

    await this.loadWorkflow({ force: true })
  },

  hasApplymentAccess() {
    return this.data.accessReady && !this.data.accessDenied && !this.data.accessErrorMessage
  },

  async loadWorkflow(options?: { silent?: boolean, force?: boolean }) {
    const { silent = false } = options || {}
    if (!this.hasApplymentAccess()) {
      wx.stopPullDownRefresh()
      return
    }

    if (workflowRequestPending) {
      wx.stopPullDownRefresh()
      return
    }

    workflowRequestPending = true

    this.setData({
      loadingWorkflow: true,
      ...(this.data.workflowLoaded || silent
        ? { refreshErrorMessage: '' }
        : {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          })
    })

    try {
      const preferredTaskType = normalizePreferredTaskType(this.data.preferredTaskType)
      const workflowView = await fetchMerchantApplymentWorkflowView(preferredTaskType || undefined)

      if (workflowView.currentStage !== 'action_required') {
        goBackHome()
        return
      }

      this.setData({
        workflowView,
        workflowLoaded: true,
        loadingWorkflow: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (error: unknown) {
      logger.error('Load merchant applyment action page failed', error, 'merchant-applyment-action-page')
      const message = getErrorMessage(error, '开户待办加载失败，请稍后重试')

      if (!this.data.workflowLoaded) {
        this.setData({
          loadingWorkflow: false,
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: '',
          workflowLoaded: false
        })
      } else {
        this.setData({
          loadingWorkflow: false,
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      }
    } finally {
      workflowRequestPending = false
      wx.stopPullDownRefresh()
    }
  },

  onRetryAccess() {
    void this.bootstrapPage()
  },

  onRetry() {
    void this.bootstrapPage()
  },

  onBackHome() {
    goBackHome()
  },

  onTapSecondaryTask(e: WechatMiniprogram.TouchEvent) {
    const dataset = e.currentTarget.dataset as Partial<MerchantApplymentWorkflowSecondaryTask>
    const intent = String(dataset.actionIntent || 'none') as MerchantApplymentTaskIntent
    const taskType = String(dataset.type || '')
    const value = String(dataset.value || '')
    const path = String(dataset.actionPath || '')

    if (intent === 'inline' && value) {
      copyText(value, resolveCopySuccessTitle(taskType))
      return
    }

    if (intent === 'navigate' && path) {
      wx.redirectTo({ url: path })
    }
  },

  async onRefreshStatus() {
    if (this.data.refreshingStatus || this.data.loadingWorkflow) {
      return
    }

    this.setData({ refreshingStatus: true })
    try {
      await this.loadWorkflow({ silent: true, force: true })
    } finally {
      this.setData({ refreshingStatus: false })
    }
  },

  async onSaveQRCodeToAlbum() {
    if (this.data.savingQRCode || !this.data.workflowView.currentTaskQRCodeValue) {
      return
    }

    this.setData({ savingQRCode: true })
    wx.showLoading({ title: '保存中...' })

    try {
      await saveApplymentQRCodePosterToAlbum({
        page: this,
        canvasSelector: '#applymentActionPosterCanvas',
        value: this.data.workflowView.currentTaskQRCodeValue,
        title: this.data.workflowView.currentTask.title,
        subtitle: this.data.workflowView.currentTask.description
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
        wx.showToast({ title: getErrorMessage(error, '保存二维码失败，请稍后重试'), icon: 'none' })
      }
    } finally {
      wx.hideLoading()
      this.setData({ savingQRCode: false })
    }
  }
})