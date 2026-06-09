import dayjs from '../_main_shared/miniprogram_npm/dayjs/index'
import {
  formatReservationStatus,
  getMerchantReservationActionState,
  getReservationStatusTheme,
  isReservationPendingPayment,
  type MerchantReservationActionKey,
  type MerchantReservationFilterStatus,
  type ReservationResponse,
  type ReservationStatusTheme
} from '../_main_shared/api/reservation'

export type ReservationWorkbenchTab = 'all' | 'paid' | 'confirmed' | 'checked_in' | 'completed' | 'exception'
export type ReservationMutationKey = MerchantReservationActionKey
export type ReservationPrimaryActionKey = ReservationMutationKey | ''

export interface ReservationCardView extends ReservationResponse {
  statusLabel: string
  statusTheme: ReservationStatusTheme
  titleText: string
  subtitleText: string
  sourceLabel: string
  paymentLabel: string
  itemPreview: string
  cookingStartedNotice: string
  canEdit: boolean
  canCancel: boolean
  canConfirm: boolean
  canCheckIn: boolean
  canStartCooking: boolean
  canNoShow: boolean
  canComplete: boolean
  primaryActionKey: ReservationPrimaryActionKey
  primaryActionLabel: string
  showMoreActions: boolean
}

export interface ReservationWorkbenchTabOption {
  key: ReservationWorkbenchTab
  label: string
}

export interface RefreshPageOptions {
  showLoading?: boolean
  preserveList?: boolean
}

export interface LoadReservationListOptions {
  showLoading?: boolean
  preserveCurrent?: boolean
}

export interface ReservationActionSheetItem {
  label: string
  key: MerchantReservationActionKey | 'edit' | 'cancel_reason'
  reason?: string
  color?: 'default' | 'danger'
}

export type ReservationActionSheetMode = 'actions' | 'cancel_reasons' | ''

export const PAGE_AUTO_REFRESH_WINDOW_MS = 60 * 1000
export const RESERVATION_CANCEL_REASONS = ['顾客临时取消', '桌台冲突需改约', '商户暂停营业', '信息填写有误', '其他原因']
export const DEFAULT_TAB_OPTIONS: ReservationWorkbenchTabOption[] = [
  { key: 'all', label: '全部' },
  { key: 'paid', label: '待确认' },
  { key: 'confirmed', label: '已确认' },
  { key: 'checked_in', label: '已到店' },
  { key: 'completed', label: '已完成' },
  { key: 'exception', label: '异常' }
]

function formatReservationSource(source?: string): string {
  switch (source) {
    case 'online':
      return '线上预订'
    case 'phone':
      return '电话预订'
    case 'walkin':
      return '到店预订'
    case 'merchant':
      return '店内代录'
    default:
      return '未标记来源'
  }
}

function formatPaymentMode(reservation: ReservationResponse): string {
  const prepaidAmount = reservation.prepaid_amount || 0
  const depositAmount = reservation.deposit_amount || 0
  const hasPaidRecord = Boolean(reservation.paid_at)

  if (prepaidAmount > 0) return '全款预付'
  if (depositAmount > 0) return '定金预订'
  if (reservation.source === 'merchant' || reservation.source === 'phone' || reservation.source === 'walkin') {
    return '到店结算'
  }
  if (reservation.payment_mode === 'full') {
    return hasPaidRecord ? '已全额支付' : '线上预订'
  }
  if (reservation.payment_mode === 'deposit') {
    if (hasPaidRecord) return '已支付定金'
    if (isReservationPendingPayment(reservation.status)) return '待支付定金'
    return '未见支付记录'
  }
  return hasPaidRecord ? '已支付' : '未见支付记录'
}

function formatReservationItems(reservation: ReservationResponse): string {
  if (!reservation.items?.length) return '未预点菜'
  return reservation.items
    .slice(0, 2)
    .map((item) => `${item.name || (item.type === 'combo' ? '套餐' : '菜品')} x${item.quantity}`)
    .join('，')
}

function formatCookingStartedNotice(value?: string): string {
  if (!value) return ''
  const startedAt = dayjs(value)
  if (!startedAt.isValid()) return '已通知后厨起菜'
  return `已于 ${startedAt.format('HH:mm')} 通知后厨起菜`
}

export function buildReservationCard(reservation: ReservationResponse): ReservationCardView {
  const actionState = getMerchantReservationActionState(reservation)
  const tableLabel = reservation.table_no || '未分配桌台'

  return {
    ...reservation,
    statusLabel: formatReservationStatus(reservation.status),
    statusTheme: getReservationStatusTheme(reservation.status),
    titleText: `${reservation.reservation_time} · ${tableLabel}`,
    subtitleText: `${reservation.contact_name} · ${reservation.contact_phone}`,
    sourceLabel: formatReservationSource(reservation.source),
    paymentLabel: formatPaymentMode(reservation),
    itemPreview: formatReservationItems(reservation),
    cookingStartedNotice: formatCookingStartedNotice(reservation.cooking_started_at),
    ...actionState
  }
}

export function getReservationActionDialogConfig(
  actionKey: ReservationMutationKey,
  contact?: string,
  cancelReason?: string
): { title: string, content: string, confirmText: string, confirmTheme: 'primary' | 'danger' } {
  switch (actionKey) {
    case 'confirm':
      return { title: '确认预订', content: `确认 ${contact || '该顾客'} 的预订后，状态会进入“已确认”，但不会占用桌台；实际占用需在到店后开台并进入“就餐中”。`, confirmText: '确认预订', confirmTheme: 'primary' }
    case 'check_in':
      return { title: '登记顾客到店', content: `${contact || '该顾客'} 到店后，预订会进入“已到店”状态，可继续安排入座或完成预订。`, confirmText: '确认登记', confirmTheme: 'primary' }
    case 'start_cooking':
      return { title: '通知后厨起菜', content: `确认通知后厨为${contact || '该顾客'}备菜后，厨房看板会同步显示起菜状态。`, confirmText: '立即通知', confirmTheme: 'primary' }
    case 'complete':
      return { title: '完成预订', content: `确认 ${contact || '该顾客'} 已离店并完成就餐后，若当前桌台仍关联该预订，桌台会释放回空闲状态。`, confirmText: '确认完成', confirmTheme: 'primary' }
    case 'no_show':
      return { title: '标记未到店', content: `确认 ${contact || '该顾客'} 未到店后，预订会进入“未到店”状态且不可直接恢复。`, confirmText: '确认标记', confirmTheme: 'danger' }
    case 'cancel':
      return { title: '取消预订', content: `将按“${cancelReason || '其他原因'}”取消该预订。若已支付，系统会按当前退款策略处理。`, confirmText: '确认取消', confirmTheme: 'danger' }
    default:
      return { title: '确认操作', content: '请确认是否继续执行当前操作。', confirmText: '确认', confirmTheme: 'primary' }
  }
}

export function getReservationActionLoadingText(actionKey: ReservationMutationKey): string {
  switch (actionKey) {
    case 'confirm': return '正在确认预订...'
    case 'check_in': return '正在登记到店...'
    case 'start_cooking': return '正在通知后厨...'
    case 'complete': return '正在完成预订...'
    case 'no_show': return '正在标记未到店...'
    case 'cancel': return '正在取消预订...'
    default: return '处理中...'
  }
}

export function getReservationActionSuccessText(actionKey: ReservationMutationKey): string {
  switch (actionKey) {
    case 'confirm': return '预订已确认'
    case 'check_in': return '已登记到店'
    case 'start_cooking': return '已通知后厨起菜'
    case 'complete': return '预订已完成'
    case 'no_show': return '已标记未到店'
    case 'cancel': return '预订已取消'
    default: return '操作已完成'
  }
}

export function buildListSummaryText(currentTab: ReservationWorkbenchTab, total: number) {
  if (currentTab === 'all') return `当前共 ${total} 条预订`

  const labelMap: Record<ReservationWorkbenchTab, string> = {
    all: '全部预订',
    paid: '待确认预订',
    confirmed: '已确认预订',
    checked_in: '已到店预订',
    completed: '已完成预订',
    exception: '异常预订'
  }
  return `${labelMap[currentTab]}共 ${total} 条`
}

export function getReservationListStatusFilter(tab: ReservationWorkbenchTab): MerchantReservationFilterStatus | undefined {
  if (tab === 'all') return undefined
  return tab
}

export function buildReservationWorkbenchResetPatch(date: string) {
  return {
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    pageSyncing: false,
    lastLoadedAt: 0,
    date,
    datePickerVisible: false,
    currentTab: 'all' as ReservationWorkbenchTab,
    tabOptions: DEFAULT_TAB_OPTIONS,
    listAvailable: false,
    listInitialError: false,
    listInitialErrorMessage: '',
    listRefreshErrorMessage: '',
    listLoading: false,
    reservations: [] as ReservationCardView[],
    listPage: 1,
    listPageSize: 20,
    listHasMore: true,
    listTotal: 0,
    listSummaryText: '当前共 0 条预订',
    actionSubmittingKey: '',
    confirmDialogVisible: false,
    confirmDialogTitle: '',
    confirmDialogContent: '',
    confirmDialogConfirmText: '确认',
    confirmDialogConfirmTheme: 'primary' as const,
    confirmDialogSubmitting: false,
    confirmDialogAction: '' as ReservationMutationKey | '',
    confirmDialogReservationId: 0,
    confirmDialogContact: '',
    confirmDialogCancelReason: '',
    actionSheetVisible: false,
    actionSheetMode: '' as ReservationActionSheetMode,
    actionSheetDescription: '',
    actionSheetItems: [] as ReservationActionSheetItem[],
    actionSheetReservationId: 0,
    actionSheetReservationContact: ''
  }
}

export function buildReservationEditUrl(date: string, id?: number) {
  const baseUrl = '/pages/merchant/reservations/edit/index'
  return id ? `${baseUrl}?id=${id}` : `${baseUrl}?date=${date}`
}

export function buildReservationMoreActionItems(reservation: ReservationCardView): ReservationActionSheetItem[] {
  const optionDefs: Array<{ label: string, key: ReservationMutationKey | 'edit' }> = []
  if (reservation.canEdit) optionDefs.push({ label: '编辑预订', key: 'edit' })
  if (reservation.canConfirm && reservation.primaryActionKey !== 'confirm') optionDefs.push({ label: '确认预订', key: 'confirm' })
  if (reservation.canCheckIn && reservation.primaryActionKey !== 'check_in') optionDefs.push({ label: '登记到店', key: 'check_in' })
  if (reservation.canStartCooking && reservation.primaryActionKey !== 'start_cooking') optionDefs.push({ label: '通知后厨起菜', key: 'start_cooking' })
  if (reservation.canComplete && reservation.primaryActionKey !== 'complete') optionDefs.push({ label: '完成预订', key: 'complete' })
  if (reservation.canNoShow) optionDefs.push({ label: '标记未到店', key: 'no_show' })
  if (reservation.canCancel) optionDefs.push({ label: '取消预订', key: 'cancel' })

  return optionDefs.map((item) => ({
    label: item.label,
    key: item.key,
    color: item.key === 'cancel' || item.key === 'no_show' ? 'danger' : 'default'
  }))
}
