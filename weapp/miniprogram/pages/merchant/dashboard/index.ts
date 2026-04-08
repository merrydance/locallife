import dayjs from 'dayjs'
import { getMerchantComplaintSummary } from '../../../api/merchant-complaints'
import {
  getMyMerchantOpenStatus,
  getMyMerchantProfile,
  type MerchantOperatorResponse
} from '../../../api/merchant'
import { MerchantStatsService } from '../../../api/merchant-stats'
import { MerchantOrderManagementService } from '../../../api/order-management'
import {
  ensureMerchantConsoleAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { isSettledFulfilled, isSettledRejected, settleAll } from '../../../utils/promise'
import { getStableBarHeights } from '../../../utils/responsive'

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

function createIcon(name: string, color: string): DashboardIconConfig {
  return {
    name,
    color,
    size: '40rpx'
  }
}

const DASHBOARD_SECTIONS: DashboardSectionDefinition[] = [
  {
    id: 'operations',
    title: '经营',
    items: [
      { id: 'orders', title: '订单管理', icon: createIcon('order-list', 'var(--td-brand-color)'), path: '/pages/merchant/orders/list/index', badgeKey: 'orders' },
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
      { id: 'display-config', title: '展示配置', icon: createIcon('setting', 'var(--td-success-color)'), path: '/pages/merchant/settings/display-config/index' },
      { id: 'printers', title: '打印设备', icon: createIcon('print', 'var(--td-success-color)'), path: '/pages/merchant/printers/index' },
      { id: 'group-join', title: '申请加入集团', icon: createIcon('cooperate', 'var(--td-success-color)'), path: '/pages/merchant/group/join/index' }
    ]
  },
  {
    id: 'growth',
    title: '会员与营销',
    items: [
      { id: 'membership', title: '优惠规则', icon: createIcon('cardmembership', 'var(--td-warning-color)'), path: '/pages/merchant/settings/membership/index' },
      { id: 'members', title: '会员列表', icon: createIcon('usergroup', 'var(--td-warning-color)'), path: '/pages/merchant/settings/members/index' },
      { id: 'recharge-rules', title: '充值规则', icon: createIcon('saving-pot', 'var(--td-warning-color)'), path: '/pages/merchant/settings/recharge-rules/index' },
      { id: 'discount-rules', title: '满减活动', icon: createIcon('discount', 'var(--td-warning-color)'), path: '/pages/merchant/discount-rules/index' },
      { id: 'delivery-promotions', title: '配送活动', icon: createIcon('vehicle', 'var(--td-warning-color)'), path: '/pages/merchant/delivery-promotions/index' },
      { id: 'vouchers', title: '代金券', icon: createIcon('coupon', 'var(--td-warning-color)'), path: '/pages/merchant/vouchers/index' }
    ]
  },
  {
    id: 'service',
    title: '售后服务',
    items: [
      { id: 'complaints', title: '投诉处理', icon: createIcon('chat-message', 'var(--td-error-color)'), path: '/pages/merchant/complaints/index', badgeKey: 'complaints' },
      { id: 'reviews', title: '评价管理', icon: createIcon('star', 'var(--td-error-color)'), path: '/pages/merchant/reviews/index' },
      { id: 'claims', title: '索赔处理', icon: createIcon('fact-check', 'var(--td-error-color)'), path: '/pages/merchant/claims/index' }
    ]
  },
  {
    id: 'finance',
    title: '财务',
    items: [
      { id: 'finance', title: '财务中心', icon: createIcon('wallet', 'var(--td-brand-color)'), path: '/pages/merchant/finance/index' },
      { id: 'stats', title: '经营统计', icon: createIcon('chart-bar', 'var(--td-brand-color)'), path: '/pages/merchant/stats/index' },
      { id: 'application', title: '主体资料', icon: createIcon('personal-information', 'var(--td-brand-color)'), path: '/pages/merchant/settings/application/index' },
      { id: 'applyment', title: '收款进件', icon: createIcon('creditcard-add', 'var(--td-brand-color)'), path: '/pages/merchant/settings/applyment/index' }
    ]
  }
]

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
}): DashboardSectionView[] {
  return DASHBOARD_SECTIONS.map((section) => ({
    id: section.id,
    title: section.title,
    items: section.items.map((item) => {
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
  }))
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
    isOpen: true,
    monthRangeLabel: '',
    activeMerchant: EMPTY_MERCHANT,
    gridGutter: GRID_GUTTER,
    overviewMetrics: buildOverviewMetrics({
      monthlyOrders: null,
      monthlySales: null,
      complaintBacklog: null
    }) as OverviewMetric[],
    sections: buildSections({
      pendingOrders: null,
      pendingComplaints: null
    }) as DashboardSectionView[],
    skeletonRows: SKELETON_ROWS
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage()
  },

  async onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage || this.data.initialLoading) {
      return
    }

    await this.loadDashboard({ silent: true })
  },

  async onPullDownRefresh() {
    try {
      if (this.data.accessReady && !this.data.accessDenied && !this.data.accessErrorMessage) {
        await this.loadDashboard()
      }
    } finally {
      wx.stopPullDownRefresh()
    }
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
      accessErrorMessage: ''
    })

    await this.loadDashboard()
  },

  async loadDashboard(options: { silent?: boolean } = {}) {
    const { silent = false } = options
    if (this.data.isPageSyncing) return

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
      ] = await settleAll([
        getMyMerchantProfile(),
        getMyMerchantOpenStatus(),
        MerchantStatsService.getOverview({
          start_date: monthStart,
          end_date: monthEnd
        }),
        MerchantOrderManagementService.getOrderSummary(),
        getMerchantComplaintSummary()
      ] as const)

      if (!isSettledFulfilled(profileResult)) {
        throw profileResult.reason
      }

      const profile = profileResult.value
      const isOpen = isSettledFulfilled(openStatusResult) ? openStatusResult.value.is_open : profile.is_open
      const pendingOrders = isSettledFulfilled(orderSummaryResult)
        ? (orderSummaryResult.value.pending_count || 0) + (orderSummaryResult.value.paid_count || 0)
        : null
      const complaintBacklog = isSettledFulfilled(complaintSummaryResult)
        ? (complaintSummaryResult.value.pending_response || 0) + (complaintSummaryResult.value.processing || 0)
        : null

      const partialFailure = [
        openStatusResult,
        overviewResult,
        orderSummaryResult,
        complaintSummaryResult
      ].some((result) => isSettledRejected(result))

      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: partialFailure ? '部分数据同步失败，当前保留可用结果' : '',
        monthRangeLabel: `${dayjs(monthStart).format('MM.DD')} - ${dayjs(monthEnd).format('MM.DD')}`,
        activeMerchant: profile,
        isOpen,
        overviewMetrics: buildOverviewMetrics({
          monthlyOrders: isSettledFulfilled(overviewResult) ? overviewResult.value.total_orders : null,
          monthlySales: isSettledFulfilled(overviewResult) ? overviewResult.value.total_sales : null,
          complaintBacklog
        }),
        sections: buildSections({
          pendingOrders,
          pendingComplaints: complaintBacklog
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
      this.setData({ isPageSyncing: false })
    }
  },

  onRetryAccess() {
    this.bootstrapPage()
  },

  onRetry() {
    this.loadDashboard().catch((err) => logger.error('Merchant dashboard retry failed', err))
  },

  onTapEntry(e: WechatMiniprogram.TouchEvent) {
    if (this.data.accessDenied || this.data.accessErrorMessage || !this.data.accessReady) return
    const { path } = e.currentTarget.dataset as { path?: string }
    if (!path) return
    wx.navigateTo({ url: path })
  }
})
