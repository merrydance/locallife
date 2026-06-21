import dayjs from '../_main_shared/miniprogram_npm/dayjs/index'
import { type BaofuSettlementAccountView } from '../_main_shared/api/baofu-account-view'
import {
  canManageMerchantPackaging,
  canManageMerchantApplyment,
  canUseMerchantDeviceManagementFallback,
  ensureMerchantConsoleAccess,
  getMerchantConsoleAccessErrorMessage,
  getRecentMerchantDeviceAccess,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import {
  fetchMerchantDashboardOrderSummary,
  fetchMerchantDashboardOverview
} from '../_services/merchant-dashboard'
import {
  fetchMerchantStorefrontOpenStatus,
  fetchMerchantStorefrontProfile,
  updateMerchantStorefrontOpenStatus
} from '../_services/merchant-open-status'
import {
  fetchMerchantBaofuSettlementAccountView
} from '../_services/merchant-baofu-settlement-account'
import {
  createMerchantAppBindCode
} from '../_services/merchant-app-bind'
import {
  buildOverviewMetrics,
  buildMerchantBusinessStateView,
  buildSections,
  captureDashboardRequest,
  EMPTY_MERCHANT,
  formatAppBindRemaining,
  getErrorMessage,
  GRID_GUTTER,
  hasTrustedDashboardData,
  isDashboardRequestOk,
  type DashboardSectionView,
  type OverviewMetric,
  shouldAutoRefreshDashboard,
  SKELETON_ROWS
} from '../_utils/merchant-dashboard-view'
import {
  MERCHANT_SETTLEMENT_ACCOUNT_PAGE_PATH
} from '../_utils/merchant-finance-entry-view'
import { wsManager, WSMessageType } from '../_main_shared/utils/websocket'

const SETTLEMENT_ACCOUNT_PAGE_PATH = MERCHANT_SETTLEMENT_ACCOUNT_PAGE_PATH

interface MerchantStatusChangePayload {
  merchant_id?: number
  is_open?: boolean
  auto_close_at?: string
  source?: string
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    isPageSyncing: false,
    openStatusSubmitting: false,
    isOpen: true,
    businessStateTitle: '营业中',
    businessStateHint: '顾客当前可以正常下单。',
    monthRangeLabel: '',
    lastRefreshAt: 0,
    activeMerchant: EMPTY_MERCHANT,
    settlementAccountView: null as BaofuSettlementAccountView | null,
    gridGutter: GRID_GUTTER,
    monthlyOrdersValue: null as number | null,
    monthlySalesValue: null as number | null,
    pendingOrdersValue: null as number | null,
    canManageDeviceSettings: false,
    canManageMerchantApplyment: false,
    canManagePackagingSettings: false,
    appBindPopupVisible: false,
    appBindLoading: false,
    appBindError: false,
    appBindErrorMessage: '',
    appBindCode: '',
    appBindRemainingSeconds: 0,
    appBindRemainingLabel: formatAppBindRemaining(0),
    overviewMetrics: buildOverviewMetrics({
      monthlyOrders: null,
      monthlySales: null
    }) as OverviewMetric[],
    sections: buildSections({
      pendingOrders: null,
      canManageDeviceSettings: false,
      canManageMerchantApplyment: false,
      canManagePackagingSettings: false
    }) as DashboardSectionView[],
    skeletonRows: SKELETON_ROWS,
    _wsListeners: [] as Array<() => void>
  },

  appBindCountdownTimer: 0 as number,

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage()
  },

  onUnload() {
    this.clearAppBindCountdown()
    this.cleanupWebSocketListeners()
    wsManager.disconnect()
  },

  async onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage || this.data.initialLoading) {
      return
    }

    this.initWebSocketListeners()

    if (!shouldAutoRefreshDashboard(this.data)) {
      return
    }

    await this.loadDashboard({ silent: true })
  },

  cleanupWebSocketListeners() {
    if (this.data._wsListeners?.length) {
      this.data._wsListeners.forEach((unsubscribe) => unsubscribe())
      this.data._wsListeners = []
    }
  },

  initWebSocketListeners() {
    this.cleanupWebSocketListeners()
    wsManager.connect()

    const statusChangeSub = wsManager.on(WSMessageType.MERCHANT_STATUS_CHANGE, (data) => {
      const payload = typeof data === 'object' && data !== null
        ? (data as MerchantStatusChangePayload)
        : {}

      if (payload.merchant_id !== this.data.activeMerchant.id) {
        return
      }

      this.loadDashboard({ silent: true }).catch((err) => logger.error('Merchant dashboard status change refresh failed', err))
    })

    const blockedSub = wsManager.on(WSMessageType.CONNECTION_BLOCKED, (payload) => {
      const message = typeof payload === 'object' && payload !== null && 'message' in payload
        ? String((payload as { message?: unknown }).message || '')
        : ''
      if (!message) {
        return
      }
      this.setData({ refreshErrorMessage: message })
    })

    this.data._wsListeners = [statusChangeSub, blockedSub]
  },

  async bootstrapPage() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: ''
    })

    const accessResult = await ensureMerchantConsoleAccess()
    if (!isMerchantConsoleAccessGranted(accessResult)) {
      this.setData({
        accessReady: true,
        accessDenied: isMerchantConsoleAccessDenied(accessResult),
        accessErrorMessage: getMerchantConsoleAccessErrorMessage(accessResult),
        initialLoading: false
      })
      return
    }

    this.setData({
      accessReady: true,
      accessDenied: false,
      accessErrorMessage: '',
      canManageMerchantApplyment: canManageMerchantApplyment(accessResult.user?.roles || [])
    })

    const grantedRoles = accessResult.user?.roles || []
    let canManageDeviceSettings = false
    let canManagePackagingSettings = false
    try {
      const deviceAccess = await getRecentMerchantDeviceAccess()
      canManageDeviceSettings = canUseMerchantDeviceManagementFallback(grantedRoles, deviceAccess)
      canManagePackagingSettings = canManageMerchantPackaging(grantedRoles, deviceAccess)
    } catch (err) {
      logger.warn('Merchant dashboard device access probe failed', err)
      canManageDeviceSettings = canUseMerchantDeviceManagementFallback(grantedRoles)
      canManagePackagingSettings = canManageMerchantPackaging(grantedRoles)
    }

    this.setData({ canManageDeviceSettings, canManagePackagingSettings })

    this.initWebSocketListeners()
    await this.loadDashboard()
  },

  async loadDashboard(options: { silent?: boolean } = {}) {
    const { silent = false } = options
    if (this.data.isPageSyncing) return
    const trustedDataAvailable = hasTrustedDashboardData(this.data)

    const monthStart = dayjs().startOf('month').format('YYYY-MM-DD')
    const monthEnd = dayjs().endOf('month').format('YYYY-MM-DD')

    this.setData({
      isPageSyncing: true,
      ...(silent
        ? { refreshErrorMessage: '' }
        : {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          })
    })

    try {
      const [
        profileResult,
        openStatusResult,
        settlementAccountResult,
        overviewResult,
        orderSummaryResult
      ] = await Promise.all([
        captureDashboardRequest(fetchMerchantStorefrontProfile()),
        captureDashboardRequest(fetchMerchantStorefrontOpenStatus()),
        this.data.canManageMerchantApplyment
          ? captureDashboardRequest(fetchMerchantBaofuSettlementAccountView())
          : Promise.resolve({ ok: true as const, value: null as BaofuSettlementAccountView | null }),
        captureDashboardRequest(fetchMerchantDashboardOverview(monthStart, monthEnd)),
        captureDashboardRequest(fetchMerchantDashboardOrderSummary())
      ] as const)

      if (!isDashboardRequestOk(profileResult)) {
        if (trustedDataAvailable) {
          this.setData({
            initialLoading: false,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: '页面同步失败，当前保留上次结果'
          })
          return
        }
        throw profileResult.error
      }

      const profile = profileResult.value
      const isOpen = isDashboardRequestOk(openStatusResult)
        ? openStatusResult.value.is_open
        : trustedDataAvailable
          ? this.data.isOpen
          : profile.is_open
      const settlementAccountView = settlementAccountResult.ok
        ? settlementAccountResult.value
        : trustedDataAvailable
          ? this.data.settlementAccountView
          : null
      const businessStateView = buildMerchantBusinessStateView({
        merchantStatus: profile.status,
        isOpen,
        settlementAccountView
      })
      const pendingOrders = isDashboardRequestOk(orderSummaryResult)
        ? (orderSummaryResult.value.paid_count || 0)
        : this.data.pendingOrdersValue
      const monthlyOrdersValue = isDashboardRequestOk(overviewResult)
        ? overviewResult.value.total_orders
        : this.data.monthlyOrdersValue
      const monthlySalesValue = isDashboardRequestOk(overviewResult)
        ? overviewResult.value.total_sales
        : this.data.monthlySalesValue

      const partialFailure = [
        openStatusResult,
        settlementAccountResult,
        overviewResult,
        orderSummaryResult
      ].some((result) => !result.ok)

      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: partialFailure
          ? (trustedDataAvailable ? '部分数据同步失败，当前保留上次结果' : '部分数据同步失败，未获取到的数据暂未显示')
          : '',
        monthRangeLabel: `${dayjs(monthStart).format('MM.DD')} - ${dayjs(monthEnd).format('MM.DD')}`,
        activeMerchant: profile,
        isOpen,
        businessStateTitle: businessStateView.title,
        businessStateHint: businessStateView.hint,
        settlementAccountView,
        monthlyOrdersValue,
        monthlySalesValue,
        pendingOrdersValue: pendingOrders,
        overviewMetrics: buildOverviewMetrics({
          monthlyOrders: monthlyOrdersValue,
          monthlySales: monthlySalesValue
        }),
        sections: buildSections({
          pendingOrders,
          canManageDeviceSettings: this.data.canManageDeviceSettings,
          canManageMerchantApplyment: this.data.canManageMerchantApplyment,
          canManagePackagingSettings: this.data.canManagePackagingSettings
        })
      })
    } catch (err) {
      logger.error('Merchant dashboard refresh failed', err)
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: '商户首页加载失败，请重试'
      })
    } finally {
      this.setData({
        isPageSyncing: false,
        lastRefreshAt: Date.now()
      })
    }
  },

  onRetryAccess() {
    this.bootstrapPage()
  },

  onRetry() {
    this.loadDashboard().catch((err) => logger.error('Merchant dashboard retry failed', err))
  },

  clearAppBindCountdown() {
    if (this.appBindCountdownTimer) {
      clearInterval(this.appBindCountdownTimer)
      this.appBindCountdownTimer = 0
    }
  },

  startAppBindCountdown(expiresIn: number) {
    this.clearAppBindCountdown()

    const tick = () => {
      const nextSeconds = Math.max(0, this.data.appBindRemainingSeconds - 1)
      this.setData({
        appBindRemainingSeconds: nextSeconds,
        appBindRemainingLabel: formatAppBindRemaining(nextSeconds)
      })
      if (nextSeconds <= 0) {
        this.clearAppBindCountdown()
      }
    }

    this.setData({
      appBindRemainingSeconds: expiresIn,
      appBindRemainingLabel: formatAppBindRemaining(expiresIn)
    })

    this.appBindCountdownTimer = setInterval(tick, 1000) as unknown as number
  },

  async openAppBindPopup() {
    this.setData({
      appBindPopupVisible: true,
      appBindLoading: true,
      appBindError: false,
      appBindErrorMessage: '',
      appBindCode: '',
      appBindRemainingSeconds: 0,
      appBindRemainingLabel: formatAppBindRemaining(0)
    })

    try {
      const response = await createMerchantAppBindCode()
      this.setData({
        appBindCode: response.code,
        appBindLoading: false,
        appBindError: false,
        appBindErrorMessage: ''
      })
      this.startAppBindCountdown(response.expires_in)
    } catch (error) {
      logger.error('Generate app bind code failed', error, 'merchant-dashboard-app-bind')
      this.clearAppBindCountdown()
      this.setData({
        appBindLoading: false,
        appBindError: true,
        appBindErrorMessage: getErrorMessage(error, '生成绑定码失败，请稍后重试'),
        appBindCode: '',
        appBindRemainingSeconds: 0,
        appBindRemainingLabel: formatAppBindRemaining(0)
      })
    }
  },

  onCloseAppBindPopup() {
    this.clearAppBindCountdown()
    this.setData({ appBindPopupVisible: false })
  },

  onRetryAppBindCode() {
    this.openAppBindPopup().catch((error) => logger.error('Retry app bind code failed', error, 'merchant-dashboard-app-bind'))
  },

  onCopyAppBindCode() {
    if (!this.data.appBindCode) {
      return
    }

    wx.setClipboardData({
      data: this.data.appBindCode,
      success: () => {
        wx.showToast({ title: '绑定码已复制', icon: 'success' })
      }
    })
  },

  async ensureCanResumeBusiness() {
    if (!this.data.canManageMerchantApplyment) {
      return true
    }

    try {
      const settlementAccountView = await fetchMerchantBaofuSettlementAccountView()

      if (settlementAccountView.paymentReady) {
        return true
      }

      const content = settlementAccountView.statusDesc || settlementAccountView.nextActionText || '结算账户仍在处理，请稍后再试'

      const result = await new Promise<boolean>((resolve) => {
        wx.showModal({
          title: '暂时无法恢复营业',
          content,
          confirmText: '去处理',
          cancelText: '知道了',
          success: (modalResult) => resolve(!!modalResult.confirm),
          fail: () => resolve(false)
        })
      })

      if (result) {
        wx.navigateTo({ url: SETTLEMENT_ACCOUNT_PAGE_PATH })
      }

      return false
    } catch (err) {
      logger.warn('Merchant dashboard precheck baofu settlement account failed', err)
      return true
    }
  },

  async onOpenStatusSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    if (this.data.accessDenied || this.data.accessErrorMessage || !this.data.accessReady || this.data.openStatusSubmitting) {
      return
    }

    const nextIsOpen = Boolean(e.detail?.value)
    if (nextIsOpen === this.data.isOpen) {
      return
    }

    if (nextIsOpen) {
      const canResumeBusiness = await this.ensureCanResumeBusiness()
      if (!canResumeBusiness) {
        return
      }
    }

    const confirmed = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: nextIsOpen ? '恢复营业' : '立即打烊',
        content: nextIsOpen
          ? '确认恢复营业后，顾客将可以继续下单。'
          : '确认立即打烊后，顾客将暂时无法继续下单。',
        confirmText: nextIsOpen ? '确认营业' : '确认打烊',
        cancelText: '再想想',
        success: (result) => resolve(!!result.confirm),
        fail: () => resolve(false)
      })
    })

    if (!confirmed) {
      return
    }

    this.setData({ openStatusSubmitting: true })

    try {
      const response = await updateMerchantStorefrontOpenStatus(nextIsOpen)
      const businessStateView = buildMerchantBusinessStateView({
        merchantStatus: this.data.activeMerchant.status,
        isOpen: response.is_open,
        settlementAccountView: this.data.settlementAccountView
      })
      this.setData({
        isOpen: response.is_open,
        businessStateTitle: businessStateView.title,
        businessStateHint: businessStateView.hint,
        lastRefreshAt: Date.now(),
        refreshErrorMessage: ''
      })
      this.initWebSocketListeners()
    } catch (err) {
      logger.error('Merchant dashboard update open status failed', err)
      wx.showToast({
        title: getErrorMessage(err, nextIsOpen ? '恢复营业失败，请稍后重试' : '打烊失败，请稍后重试'),
        icon: 'none'
      })
    } finally {
      this.setData({ openStatusSubmitting: false })
    }
  },

  onTapEntry(e: WechatMiniprogram.TouchEvent) {
    if (this.data.accessDenied || this.data.accessErrorMessage || !this.data.accessReady) return
    const { path, id } = e.currentTarget.dataset as { path?: string, id?: string }
    if (id === 'bind-app') {
      this.openAppBindPopup().catch((error) => logger.error('Open app bind popup failed', error, 'merchant-dashboard-app-bind'))
      return
    }
    if (!path) return
    wx.navigateTo({ url: path })
  }
})
