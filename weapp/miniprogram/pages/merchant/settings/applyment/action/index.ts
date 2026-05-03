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
import Toast, { hideToast } from '../../../../../miniprogram_npm/tdesign-miniprogram/toast/index'
import { logger } from '../../../../../utils/logger'
import { getStableBarHeights } from '../../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../../utils/user-facing'

const HOME_PAGE_PATH = '/pages/merchant/settings/applyment/index'
const EMPTY_WORKFLOW_VIEW = buildMerchantApplymentWorkflowView(null)
const TOAST_SELECTOR = '#t-toast'

const getErrorMessage = getErrorUserMessage

let workflowRequestPending = false

interface ApplymentActionStepView {
  title: string
  description: string
}

interface ApplymentActionTaskView {
  eyebrow: string
  title: string
  description: string
  statusNote: string
  qrTitle: string
  saveQRCodeLabel: string
  steps: ApplymentActionStepView[]
}

const EMPTY_ACTION_TASK_VIEW: ApplymentActionTaskView = {
  eyebrow: '微信侧待办',
  title: '处理微信待办',
  description: '请按微信支付页面提示完成当前待办。',
  statusNote: '完成后回到本页，系统会自动刷新开户状态。',
  qrTitle: '待办二维码',
  saveQRCodeLabel: '保存二维码',
  steps: [
    { title: '处理微信待办', description: '按页面提示完成签约、法人验证或账户验证。' },
    { title: '回到开户页', description: '完成后回到本页或开户首页查看最新状态。' }
  ]
}

function normalizePreferredTaskType(task?: string): MerchantApplymentTaskType | '' {
  switch (String(task || '').trim()) {
    case 'sign_agreement':
    case 'legal_validation':
    case 'bank_transfer_validation':
      return String(task) as MerchantApplymentTaskType
    default:
      return ''
  }
}

function showActionToast(
  context: WechatMiniprogram.Page.TrivialInstance,
  message: string,
  theme: 'loading' | 'success' | 'warning' | 'error',
  options?: { duration?: number, preventScrollThrough?: boolean }
) {
  Toast({
    context,
    selector: TOAST_SELECTOR,
    message,
    theme,
    direction: 'column',
    duration: options?.duration,
    preventScrollThrough: options?.preventScrollThrough
  })
}

function hideActionToast(context: WechatMiniprogram.Page.TrivialInstance) {
  hideToast({ context, selector: TOAST_SELECTOR })
}

function copyText(context: WechatMiniprogram.Page.TrivialInstance, data: string, successTitle: string) {
  const trimmed = String(data || '').trim()
  if (!trimmed) {
    return
  }

  wx.setClipboardData({
    data: trimmed,
    success: () => {
      showActionToast(context, successTitle, 'success', { duration: 1800 })
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

function buildApplymentActionTaskView(workflowView: typeof EMPTY_WORKFLOW_VIEW): ApplymentActionTaskView {
  switch (workflowView.currentTask.type) {
    case 'sign_agreement':
      return {
        eyebrow: '微信侧待办',
        title: '完成微信支付签约',
        description: '保存二维码后用微信扫一扫，从相册识别二维码，并按微信支付提示完成商户签约。',
        statusNote: '签约完成后回到本页，系统会自动刷新开户状态。',
        qrTitle: '签约二维码',
        saveQRCodeLabel: '保存签约二维码',
        steps: [
          { title: '保存二维码', description: '将签约二维码保存到手机相册。' },
          { title: '微信识别处理', description: '退出小程序，用微信扫一扫从相册识别二维码并完成签约。' },
          { title: '返回查看状态', description: '完成后回到本页，系统会自动同步最新开户状态。' }
        ]
      }
    case 'legal_validation':
      return {
        eyebrow: '微信侧待办',
        title: '完成法人扫码验证',
        description: '保存二维码后用法人微信扫一扫，从相册识别二维码，并按微信支付提示完成法人验证。',
        statusNote: '法人验证完成后回到本页，系统会自动刷新开户状态。',
        qrTitle: '法人验证二维码',
        saveQRCodeLabel: '保存法人验证二维码',
        steps: [
          { title: '保存二维码', description: '将法人验证二维码保存到手机相册。' },
          { title: '法人微信识别', description: '请法人用微信扫一扫从相册识别二维码并完成验证。' },
          { title: '返回查看状态', description: '完成后回到本页，系统会自动同步最新开户状态。' }
        ]
      }
    case 'bank_transfer_validation':
      return {
        eyebrow: '微信侧待办',
        title: '完成账户验证',
        description: '请按微信支付提供的收款账户、金额和备注完成汇款验证。',
        statusNote: '汇款完成后回到本页，系统会自动刷新开户状态。',
        qrTitle: '',
        saveQRCodeLabel: '',
        steps: [
          { title: '核对汇款信息', description: '确认付款户名、付款卡号、收款卡号、金额和备注。' },
          { title: '完成验证汇款', description: '使用进件结算账户按微信要求汇款。' },
          { title: '返回查看状态', description: '完成后回到本页，系统会自动同步最新开户状态。' }
        ]
      }
    default:
      return {
        ...EMPTY_ACTION_TASK_VIEW,
        title: workflowView.currentTask.title || EMPTY_ACTION_TASK_VIEW.title,
        description: workflowView.currentTask.description || EMPTY_ACTION_TASK_VIEW.description
      }
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
    loadingWorkflow: false,
    workflowLoaded: false,
    preferredTaskType: '',
    workflowView: { ...EMPTY_WORKFLOW_VIEW },
    actionTaskView: { ...EMPTY_ACTION_TASK_VIEW },
    refreshingStatus: false,
    savingQRCode: false,
    albumPermissionDialogVisible: false
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
      actionTaskView: { ...EMPTY_ACTION_TASK_VIEW },
      refreshingStatus: false,
      savingQRCode: false,
      albumPermissionDialogVisible: false
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
        actionTaskView: buildApplymentActionTaskView(workflowView),
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

  onCloseAlbumPermissionDialog() {
    this.setData({ albumPermissionDialogVisible: false })
  },

  onConfirmAlbumPermissionDialog() {
    this.setData({ albumPermissionDialogVisible: false })
    wx.openSetting()
  },

  onTapSecondaryTask(e: WechatMiniprogram.TouchEvent) {
    const dataset = e.currentTarget.dataset as Partial<MerchantApplymentWorkflowSecondaryTask>
    const intent = String(dataset.actionIntent || 'none') as MerchantApplymentTaskIntent
    const taskType = String(dataset.type || '')
    const value = String(dataset.value || '')
    const path = String(dataset.actionPath || '')

    if (intent === 'inline' && value) {
      copyText(this, value, resolveCopySuccessTitle(taskType))
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
    showActionToast(this, '保存中...', 'loading', { duration: 0, preventScrollThrough: true })

    try {
      await saveApplymentQRCodePosterToAlbum({
        page: this,
        canvasSelector: '#applymentActionPosterCanvas',
        value: this.data.workflowView.currentTaskQRCodeValue,
        title: this.data.actionTaskView.title,
        subtitle: this.data.actionTaskView.description
      })
      hideActionToast(this)
      showActionToast(this, '二维码已保存，请退出小程序后用微信扫一扫从相册识别', 'success', { duration: 2600 })
    } catch (error: unknown) {
      hideActionToast(this)
      if (isPermissionDeniedError(error)) {
        this.setData({ albumPermissionDialogVisible: true })
      } else {
        showActionToast(this, getErrorMessage(error, '保存二维码失败，请稍后重试'), 'warning', { duration: 2000 })
      }
    } finally {
      this.setData({ savingQRCode: false })
    }
  }
})
