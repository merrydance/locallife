import {
  buildMerchantApplymentWorkflowView,
  fetchMerchantApplymentWorkflowView,
  type MerchantApplymentTaskIntent,
  type MerchantApplymentWorkflowSecondaryTask
} from '../../../../services/merchant-applyment-workflow'
import {
  buildMerchantSettlementAccountView,
  getMerchantSettlementAccount
} from '../../../../api/merchant-settlement-account'
import {
  ensureMerchantApplymentAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../../utils/console-access'
import Toast from '../../../../miniprogram_npm/tdesign-miniprogram/toast/index'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

const APPLYMENT_AUTO_REFRESH_WINDOW_MS = 60 * 1000
const APPLYMENT_FORCE_REFRESH_STORAGE_KEY = 'merchantApplymentShouldRefresh'
const SUBMIT_PAGE_PATH = '/pages/merchant/settings/applyment/submit/index'
const SETTLEMENT_ACCOUNT_PAGE_PATH = '/pages/merchant/settings/applyment/settlement-account/index'
const EMPTY_WORKFLOW_VIEW = buildMerchantApplymentWorkflowView(null)
const EMPTY_SETTLEMENT_ACCOUNT_VIEW = buildMerchantSettlementAccountView(null)
const EMPTY_DISPLAY_STATUS_ITEMS: Array<{ label: string, value: string }> = []
const TOAST_SELECTOR = '#t-toast'

const getErrorMessage = getErrorUserMessage

let applymentRequestPending = false
let settlementRequestPending = false

interface ApplymentOpenedSummaryView {
  title: string
  description: string
  subMchId: string
  settlementActionText: string
  financeHint: string
  merchantAssistantHint: string
}

const EMPTY_OPENED_SUMMARY_VIEW: ApplymentOpenedSummaryView = {
  title: '',
  description: '',
  subMchId: '-',
  settlementActionText: '查看/修改结算账户',
  financeHint: '',
  merchantAssistantHint: ''
}

function showResultToast(context: WechatMiniprogram.Page.TrivialInstance, message: string, theme: 'success' | 'warning' | 'error') {
  Toast({
    context,
    selector: TOAST_SELECTOR,
    message,
    theme,
    direction: 'column',
    duration: 1800
  })
}

function copyText(context: WechatMiniprogram.Page.TrivialInstance, data: string, successTitle: string) {
  const trimmed = String(data || '').trim()
  if (!trimmed) {
    return
  }

  wx.setClipboardData({
    data: trimmed,
    success: () => {
      showResultToast(context, successTitle, 'success')
    }
  })
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
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

function buildDisplayStatusItems(workflowView: typeof EMPTY_WORKFLOW_VIEW) {
  return workflowView.statusItems.filter((item) => item.label !== '当前阶段' && item.label !== '状态说明')
}

function buildApplymentOpenedSummaryView(workflowView: typeof EMPTY_WORKFLOW_VIEW): ApplymentOpenedSummaryView {
  if (workflowView.currentStage !== 'opened') {
    return { ...EMPTY_OPENED_SUMMARY_VIEW }
  }

  return {
    title: '微信支付已开通',
    description: '可使用平台收款能力。',
    subMchId: workflowView.statusView.subMchId || '-',
    settlementActionText: '查看/修改结算账户',
    financeHint: '订单流水和平台内结算记录可在财务页查看。',
    merchantAssistantHint: '商户余额、提现和商户平台操作请前往微信支付商户平台或微信支付商家助手处理。'
  }
}

function resolveWorkflowStagePath(workflowView: typeof EMPTY_WORKFLOW_VIEW) {
  if (workflowView.currentTask.type === 'submit_material' || workflowView.currentTask.type === 'resubmit_after_reject') {
    return SUBMIT_PAGE_PATH
  }

  if (workflowView.currentStage === 'action_required' && workflowView.primaryActionIntent === 'navigate' && workflowView.primaryActionPath) {
    return workflowView.primaryActionPath
  }

  return ''
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
    workflowView: { ...EMPTY_WORKFLOW_VIEW },
    openedSummary: { ...EMPTY_OPENED_SUMMARY_VIEW },
    settlementAccountView: { ...EMPTY_SETTLEMENT_ACCOUNT_VIEW },
    displayStatusItems: EMPTY_DISPLAY_STATUS_ITEMS,
    settlementLoading: false,
    settlementLoaded: false,
    settlementErrorMessage: '',
    refreshingStatus: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
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

    if (this.data.workflowView.reentryPolicy === 'force_refresh_on_show') {
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
      workflowView: { ...EMPTY_WORKFLOW_VIEW },
      openedSummary: { ...EMPTY_OPENED_SUMMARY_VIEW },
      settlementAccountView: { ...EMPTY_SETTLEMENT_ACCOUNT_VIEW },
      displayStatusItems: EMPTY_DISPLAY_STATUS_ITEMS,
      settlementLoading: false,
      settlementLoaded: false,
      settlementErrorMessage: '',
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
      const workflowView = await fetchMerchantApplymentWorkflowView()
      const stagePath = resolveWorkflowStagePath(workflowView)

      if (stagePath) {
        wx.redirectTo({ url: stagePath })
        return
      }

      const isOpened = workflowView.currentStage === 'opened'
      this.setData({
        workflowView,
        openedSummary: buildApplymentOpenedSummaryView(workflowView),
        displayStatusItems: buildDisplayStatusItems(workflowView),
        statusLoaded: true,
        loadingApplyment: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        ...(isOpened
          ? {}
          : {
              openedSummary: { ...EMPTY_OPENED_SUMMARY_VIEW },
              settlementAccountView: { ...EMPTY_SETTLEMENT_ACCOUNT_VIEW },
              settlementLoading: false,
              settlementLoaded: false,
              settlementErrorMessage: ''
            }),
        lastLoadedAt: Date.now()
      })

      if (isOpened) {
        void this.loadSettlementAccount({ force: true, silent: true })
      }
    } catch (error: unknown) {
      logger.error('Load merchant applyment page failed', error, 'merchant-applyment-page')
      const message = getErrorMessage(error, '开户状态加载失败，请稍后重试')

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

  async loadSettlementAccount(options?: { force?: boolean, silent?: boolean }) {
    const { force = false, silent = false } = options || {}
    if (!this.hasApplymentAccess() || this.data.workflowView.currentStage !== 'opened') {
      return
    }

    if (settlementRequestPending) {
      return
    }

    if (!force && this.data.settlementLoaded) {
      return
    }

    settlementRequestPending = true
    const hasTrustedData = this.data.settlementLoaded

    this.setData({
      settlementLoading: true,
      ...(hasTrustedData || silent ? { settlementErrorMessage: '' } : {})
    })

    try {
      const response = await getMerchantSettlementAccount()
      this.setData({
        settlementAccountView: buildMerchantSettlementAccountView(response),
        settlementLoading: false,
        settlementLoaded: true,
        settlementErrorMessage: ''
      })
    } catch (error: unknown) {
      logger.error('Load merchant settlement account failed', error, 'merchant-applyment-page')
      const message = getErrorMessage(error, '结算账户加载失败，请稍后重试')
      this.setData({
        settlementLoading: false,
        settlementErrorMessage: hasTrustedData ? `${message}，当前已保留上次同步结果` : message
      })
    } finally {
      settlementRequestPending = false
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

  navigateByIntent(intent: MerchantApplymentTaskIntent, path: string) {
    if (intent === 'refresh') {
      void this.onRefreshStatus()
      return
    }

    if (intent !== 'navigate' || !path) {
      return
    }

    wx.navigateTo({ url: path })
  },

  onOpenPrimaryAction() {
    this.navigateByIntent(this.data.workflowView.primaryActionIntent, this.data.workflowView.primaryActionPath)
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

    this.navigateByIntent(intent, path)
  },

  onRetrySettlementAccount() {
    void this.loadSettlementAccount({ force: true })
  },

  onOpenSettlementAccountPage() {
    wx.navigateTo({ url: SETTLEMENT_ACCOUNT_PAGE_PATH })
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
