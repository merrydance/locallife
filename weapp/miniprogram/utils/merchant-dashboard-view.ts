import { getErrorUserMessage } from './user-facing'
import type { MerchantConsoleProfile } from '../services/merchant-console'

export interface OverviewMetric {
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

export interface DashboardSectionView {
  id: string
  title: string
  items: DashboardEntryView[]
}

export type DashboardRequestResult<T> =
  | { ok: true, value: T }
  | { ok: false, error: unknown }

export const EMPTY_MERCHANT: MerchantConsoleProfile = {
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

export const SKELETON_ROWS = [
  { width: '100%', height: '360rpx' },
  { width: '100%', height: '920rpx', marginTop: '24rpx' }
]

export const GRID_GUTTER = 16
export const DASHBOARD_STALE_MS = 60 * 1000
export const getErrorMessage = getErrorUserMessage

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
  {
    id: 'finance',
    title: '财务',
    items: [
      { id: 'finance', title: '资金账户', icon: createIcon('wallet', 'var(--td-brand-color)'), path: '/pages/merchant/finance/index' },
      { id: 'settlement-account', title: '微信提现卡', icon: createIcon('creditcard', 'var(--td-brand-color)'), path: '/pages/merchant/finance/settlement-account/index' },
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

function toBadgeText(value: number | null | undefined) {
  if (typeof value !== 'number' || value <= 0) return ''
  return value > 99 ? '99+' : `${value}`
}

export function hasTrustedDashboardData(data: { lastRefreshAt: number }) {
  return data.lastRefreshAt > 0
}

export function shouldAutoRefreshDashboard(data: { lastRefreshAt: number }) {
  return !data.lastRefreshAt || Date.now() - data.lastRefreshAt >= DASHBOARD_STALE_MS
}

export function formatAppBindRemaining(seconds: number) {
  if (seconds <= 0) {
    return '绑定码已过期，请重新生成'
  }

  const minutes = Math.floor(seconds / 60)
  const remainderSeconds = seconds % 60
  return `请在 ${String(minutes).padStart(2, '0')}:${String(remainderSeconds).padStart(2, '0')} 内在商户端 App 输入此绑定码`
}

export async function captureDashboardRequest<T>(request: Promise<T>): Promise<DashboardRequestResult<T>> {
  try {
    return { ok: true, value: await request }
  } catch (error) {
    return { ok: false, error }
  }
}

export function isDashboardRequestOk<T>(result: DashboardRequestResult<T>): result is { ok: true, value: T } {
  return result.ok
}

export function buildOverviewMetrics(params: {
  monthlyOrders: number | null
  monthlySales: number | null
  complaintBacklog: number | null
}): OverviewMetric[] {
  return [
    { id: 'orders', label: '本月订单', value: formatCount(params.monthlyOrders), note: '已完成订单' },
    { id: 'sales', label: '本月成交额', value: formatMoney(params.monthlySales), note: '扣折后金额' },
    { id: 'complaints', label: '投诉待办', value: formatCount(params.complaintBacklog), note: '当前待跟进' }
  ]
}

export function buildSections(params: {
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
        badgeProps: badgeText ? { count: badgeText, maxCount: 99 } : null
      }
    })
  })).filter((section) => section.items.length > 0)
}