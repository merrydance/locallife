import dayjs from 'dayjs'
import { generateAppBindCode } from '../../../api/auth'
import { getMerchantComplaintSummary } from '../../../api/merchant-complaints'
import {
  getMyMerchantOpenStatus,
  getMyMerchantProfile,
  type MerchantOperatorResponse,
  updateMyMerchantOpenStatus
} from '../../../api/merchant'
import {
  buildMerchantApplymentStatusView,
  getMerchantApplymentStatus
} from '../../../api/merchant-applyment'
import { MerchantStatsService } from '../../../api/merchant-stats'
import { MerchantOrderManagementService } from '../../../api/order-management'
import {
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
import { getErrorUserMessage } from '../../../utils/user-facing'

interface OverviewMetric {
  id: string
  label: string
  value: string
  note: string
}

interface DashboardIconConfig {
  name: string
  color: string
  size: string
}

interface DashboardBadgeConfig {
  count: string
  maxCount: number
}

interface DashboardEntryDefinition {
  id: string
  title: string
  icon: DashboardIconConfig
  path: string
  badgeKey?: 'orders' | 'complaints'
}

interface DashboardEntryView extends DashboardEntryDefinition {
  badgeText: string
  badgeProps: DashboardBadgeConfig | null
}

interface DashboardSectionDefinition {
  id: string
  title: string
  items: DashboardEntryDefinition[]
}

interface DashboardSectionView {
  id: string
  title: string
  items: DashboardEntryView[]
}

type DashboardRequestResult<T> =
  | { ok: true, value: T }
  | { ok: false, error: unknown }

const EMPTY_MERCHANT: MerchantOperatorResponse = {
  id: 0,
  owner_user_id: 0,
  region_id: 0,
  name: '',
  description: '',
  logo_url: '',
  phone: '',
  address: '',
  latitude: '',
  longitude: '',
  status: '',
  is_open: true,
  version: 0,
  created_at: '',
  updated_at: ''
}

const SKELETON_ROWS = [
  { width: '100%', height: '360rpx' },
  { width: '100%', height: '920rpx', marginTop: '24rpx' }
]

const GRID_GUTTER = 16
const DASHBOARD_STALE_MS = 60 * 1000

const getErrorMessage = getErrorUserMessage

function hasTrustedDashboardData(data: {
  lastRefreshAt: number
}) {
  return data.lastRefreshAt > 0
}

function shouldAutoRefreshDashboard(data: {
  lastRefreshAt: number
}) {
  return !data.lastRefreshAt || Date.now() - data.lastRefreshAt >= DASHBOARD_STALE_MS
}

function createIcon(name: string, color: string): DashboardIconConfig {
  return {
    name,
    color,
    size: '40rpx'
  }
}

function formatAppBindRemaining(seconds: number) {
  if (seconds <= 0) {
    return '绑定码已过期，请重新生成'
  }

  const minutes = Math.floor(seconds / 60)
  const remainderSeconds = seconds % 60
  return `请在 ${String(minutes).padStart(2, '0')}:${String(remainderSeconds).padStart(2, '0')} 内在商户端 App 输入此绑定码`
}

async function captureDashboardRequest<T>(request: Promise<T>): Promise<DashboardRequestResult<T>> {
  try {
    return { ok: true, value: await request }
  } catch (error) {
    return { ok: false, error }
  }
}

function isDashboardRequestOk<T>(result: DashboardRequestResult<T>): result is { ok: true, value: T } {
  return result.ok
}

const DASHBOARD_SECTIONS: DashboardSectionDefinition[] = [
  {
    id: 'operations',
    title: '经营',
    items: [
      { id: 'orders', title: '订单管理', icon: createIcon('cart', 'var(--td-brand-color)'), path: '/pages/merchant/orders/list/index', badgeKey: 'orders' },
      { id: 'kitchen', title: '后厨看板', icon: createIcon('task', 'var(--td-brand-color)'), path: '/pages/merchant/kitchen/index' },
      { id: 'reservations', title: '预订管理', icon: createIcon('calendar-event', 'var(--td-brand-color)'), path: '/pages/merchant/reservations/index' }
    ]
  },
  {
    id: 'store',
    title: '店铺信息',
    items: [
      { id: 'profile', title: '门店资料', icon: createIcon('shop', 'var(--td-success-color)'), path: '/pages/merchant/settings/profile/index' },
      { id: 'business-hours', title: '营业时间', icon: createIcon('calendar-1', 'var(--td-success-color)'), path: '/pages/merchant/settings/business-hours/index' },
      { id: 'staff', title: '员工管理', icon: createIcon('usergroup', 'var(--td-success-color)'), path: '/pages/merchant/staff/index' },
      { id: 'dishes', title: '菜品管理', icon: createIcon('fork', 'var(--td-success-color)'), path: '/pages/merchant/dishes/index' },
      { id: 'tables', title: '桌台房间', icon: createIcon('table', 'var(--td-success-color)'), path: '/pages/merchant/tables/index' },
      { id: 'combos', title: '套餐管理', icon: createIcon('combination', 'var(--td-success-color)'), path: '/pages/merchant/combos/index' },
      { id: 'inventory', title: '库存管理', icon: createIcon('system-storage', 'var(--td-success-color)'), path: '/pages/merchant/inventory/index' },
      { id: 'display-config', title: '后厨协同', icon: createIcon('setting', 'var(--td-success-color)'), path: '/pages/merchant/settings/display-config/index' },
      { id: 'printers', title: '打印设备', icon: createIcon('print', 'var(--td-success-color)'), path: '/pages/merchant/printers/index' },
      { id: 'bind-app', title: '绑定商户端App', icon: createIcon('setting', 'var(--td-success-color)'), path: '' },
      { id: 'group-join', title: '申请加入集团', icon: createIcon('cooperate', 'var(--td-success-color)'), path: '/pages/merchant/group/join/index' }
    ]
  },
  {
    id: 'growth',
    title: '会员与营销',
    items: [
      { id: 'membership', title: '叠加规则', icon: createIcon('cardmembership', 'var(--td-warning-color)'), path: '/pages/merchant/settings/membership/index' },
      { id: 'members', title: '会员列表', icon: createIcon('usergroup', 'var(--td-warning-color)'), path: '/pages/merchant/settings/members/index' },
      { id: 'recharge-rules', title: '充值规则', icon: createIcon('saving-pot', 'var(--td-warning-color)'), path: '/pages/merchant/settings/recharge-rules/index' },
      { id: 'discount-rules', title: '满减活动', icon: createIcon('discount', 'var(--td-warning-color)'), path: '/pages/merchant/discount-rules/index' },
      { id: 'delivery-promotions', title: '配送活动', icon: createIcon('vehicle', 'var(--td-warning-color)'), path: '/pages/merchant/delivery-promotions/index' },
      { id: 'vouchers', title: '代金券', icon: createIcon('coupon', 'var(--td-warning-color)'), path: '/pages/merchant/vouchers/index' }
    ]
  },
  // {
  //   id: 'service',
  //   title: '售后服务',
  //   items: [
  //     { id: 'complaints', title: '投诉处理', icon: createIcon('chat-message', 'var(--td-error-color)'), path: '/pages/merchant/complaints/index', badgeKey: 'complaints' },
  //     { id: 'reviews', title: '评价管理', icon: createIcon('star', 'var(--td-error-color)'), path: '/pages/merchant/reviews/index' },
  //     { id: 'claims', title: '索赔处理', icon: createIcon('fact-check', 'var(--td-error-color)'), path: '/pages/merchant/claims/index' }
  //   ]
  // },
  {
    id: 'finance',
    title: '财务',
    items: [
      { id: 'finance', title: '资金账户', icon: createIcon('wallet', 'var(--td-brand-color)'), path: '/pages/merchant/finance/index' },
      { id: 'settlement-account', title: '微信提现卡', icon: createIcon('creditcard', 'var(--td-brand-color)'), path: '/pages/merchant/finance/settlement-account/index' },
      // { id: 'finance-analysis', title: '经营分析', icon: createIcon('chart-bar', 'var(--td-brand-color)'), path: '/pages/merchant/finance/bills/index' },
      { id: 'application', title: '主体资料', icon: createIcon('personal-information', 'var(--td-brand-color)'), path: '/pages/merchant/settings/application/index' },
      { id: 'applyment', title: '收付通进件', icon: createIcon('creditcard-add', 'var(--td-brand-color)'), path: '/pages/merchant/settings/applyment/index' }
    ]
  }
]

const DEVICE_MANAGE_ENTRY_IDS = new Set(['display-config', 'printers', 'bind-app'])

function formatMoney(fen: number | null | undefined) {
  if (typeof fen !== 'number' || !Number.isFinite(fen)) return '--'
  return `¥${(fen / 100).toFixed(0)}`
}

function formatCount(value: number | null | undefined) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '--'
  return `${value}`
}

function buildOverviewMetrics(params: {
  monthlyOrders: number | null
  monthlySales: number | null
  complaintBacklog: number | null
}): OverviewMetric[] {
  return [
    {
      id: 'orders',
      label: '本月订单',
      value: formatCount(params.monthlyOrders),
      note: '已完成订单'
    },
    {
      id: 'sales',
      label: '本月成交额',
      value: formatMoney(params.monthlySales),
      note: '扣折后金额'
    },
    {
      id: 'complaints',
      label: '投诉待办',
      value: formatCount(params.complaintBacklog),
      note: '当前待跟进'
    }
  ]
}

function toBadgeText(value: number | null | undefined) {
  if (typeof value !== 'number' || value <= 0) return ''
  return value > 99 ? '99+' : `${value}`
}

function buildSections(params: {
  pendingOrders: number | null
  pendingComplaints: number | null
  canManageDeviceSettings: boolean
  canManageMerchantApplyment: boolean
}): DashboardSectionView[] {
  return DASHBOARD_SECTIONS.map((section) => ({
    id: section.id,
    title: section.title,
    items: section.items.filter((item) => {
      const passesDeviceGate = params.canManageDeviceSettings || !DEVICE_MANAGE_ENTRY_IDS.has(item.id)
      const passesApplymentGate = params.canManageMerchantApplyment || item.id !== 'applyment'
      return passesDeviceGate && passesApplymentGate
    }).map((item) => {
      let badgeText = ''
      if (item.badgeKey === 'orders') {
        badgeText = toBadgeText(params.pendingOrders)
      }
      if (item.badgeKey === 'complaints') {
        badgeText = toBadgeText(params.pendingComplaints)
      }

      return {
        ...item,
        badgeText,
        badgeProps: badgeText
          ? {
              count: badgeText,
              maxCount: 99
            }
          : null
      }
    })
  })).filter((section) => section.items.length > 0)
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
    monthRangeLabel: '',
    lastRefreshAt: 0,
    activeMerchant: EMPTY_MERCHANT,
    gridGutter: GRID_GUTTER,
    monthlyOrdersValue: null as number | null,
    monthlySalesValue: null as number | null,
    complaintBacklogValue: null as number | null,
    pendingOrdersValue: null as number | null,
    pendingComplaintsValue: null as number | null,
    canManageDeviceSettings: false,
    canManageMerchantApplyment: false,
    appBindPopupVisible: false,
    appBindLoading: false,
    appBindError: false,
    appBindErrorMessage: '',
    appBindCode: '',
    appBindRemainingSeconds: 0,
    appBindRemainingLabel: formatAppBindRemaining(0),
    overviewMetrics: buildOverviewMetrics({
      monthlyOrders: null,
      monthlySales: null,
      complaintBacklog: null
    }) as OverviewMetric[],
    sections: buildSections({
      pendingOrders: null,
      pendingComplaints: null,
      canManageDeviceSettings: false,
      canManageMerchantApplyment: false
    }) as DashboardSectionView[],
    skeletonRows: SKELETON_ROWS
  },

  appBindCountdownTimer: 0 as number,

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage()
  },

  onUnload() {
    this.clearAppBindCountdown()
  },

  async onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage || this.data.initialLoading) {
      return
    }

    if (!shouldAutoRefreshDashboard(this.data)) {
      return
    }

    await this.loadDashboard({ silent: true })
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
    try {
      const deviceAccess = await getRecentMerchantDeviceAccess()
      canManageDeviceSettings = canUseMerchantDeviceManagementFallback(grantedRoles, deviceAccess)
    } catch (err) {
      logger.warn('Merchant dashboard device access probe failed', err)
      canManageDeviceSettings = canUseMerchantDeviceManagementFallback(grantedRoles)
    }

    this.setData({ canManageDeviceSettings })

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
        overviewResult,
        orderSummaryResult,
        complaintSummaryResult
      ] = await Promise.all([
        captureDashboardRequest(getMyMerchantProfile()),
        captureDashboardRequest(getMyMerchantOpenStatus()),
        captureDashboardRequest(MerchantStatsService.getOverview({
          start_date: monthStart,
          end_date: monthEnd
        })),
        captureDashboardRequest(MerchantOrderManagementService.getOrderSummary()),
        captureDashboardRequest(getMerchantComplaintSummary())
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
      const pendingOrders = isDashboardRequestOk(orderSummaryResult)
        ? (orderSummaryResult.value.paid_count || 0)
        : this.data.pendingOrdersValue
      const complaintBacklog = isDashboardRequestOk(complaintSummaryResult)
        ? (complaintSummaryResult.value.pending_response || 0) + (complaintSummaryResult.value.processing || 0)
        : this.data.complaintBacklogValue
      const monthlyOrdersValue = isDashboardRequestOk(overviewResult)
        ? overviewResult.value.total_orders
        : this.data.monthlyOrdersValue
      const monthlySalesValue = isDashboardRequestOk(overviewResult)
        ? overviewResult.value.total_sales
        : this.data.monthlySalesValue

      const partialFailure = [
        openStatusResult,
        overviewResult,
        orderSummaryResult,
        complaintSummaryResult
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
        monthlyOrdersValue,
        monthlySalesValue,
        complaintBacklogValue: complaintBacklog,
        pendingOrdersValue: pendingOrders,
        pendingComplaintsValue: complaintBacklog,
        overviewMetrics: buildOverviewMetrics({
          monthlyOrders: monthlyOrdersValue,
          monthlySales: monthlySalesValue,
          complaintBacklog
        }),
        sections: buildSections({
          pendingOrders,
          pendingComplaints: complaintBacklog,
          canManageDeviceSettings: this.data.canManageDeviceSettings,
          canManageMerchantApplyment: this.data.canManageMerchantApplyment
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

  onManualRefresh() {
    if (this.data.accessDenied || this.data.accessErrorMessage || !this.data.accessReady) return
    this.loadDashboard({ silent: true }).catch((err) => logger.error('Merchant dashboard manual refresh failed', err))
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
      const response = await generateAppBindCode()
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
      const applyment = await getMerchantApplymentStatus()
      const applymentStatusView = buildMerchantApplymentStatusView(applyment)

      if (applymentStatusView.isOpened) {
        return true
      }

      const content = applymentStatusView.blockReason || applymentStatusView.guideText

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
        wx.navigateTo({ url: '/pages/merchant/settings/applyment/index' })
      }

      return false
    } catch (err) {
      logger.warn('Merchant dashboard precheck applyment status failed', err)
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
      const response = await updateMyMerchantOpenStatus(nextIsOpen)
      this.setData({
        isOpen: response.is_open,
        lastRefreshAt: Date.now(),
        refreshErrorMessage: ''
      })
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
