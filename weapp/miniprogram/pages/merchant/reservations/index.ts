import dayjs from 'dayjs'
import Toast, { hideToast } from '../../../miniprogram_npm/tdesign-miniprogram/toast'
import { getStableBarHeights } from '../../../utils/responsive'
import {
  cancelReservation,
  checkInReservation,
  completeReservationByMerchant,
  confirmReservationByMerchant,
  formatReservationStatus,
  getMerchantReservationActionState,
  getReservationStatusTheme,
  isReservationPendingPayment,
  markReservationNoShow,
  MerchantReservationActionKey,
  MerchantReservationFilterStatus,
  ReservationResponse,
  ReservationService,
  ReservationStatusTheme,
  startCookingReservation
} from '../../../api/reservation'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import {
  ensureMerchantConsoleAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../utils/console-access'

type ReservationWorkbenchTab = 'all' | 'paid' | 'confirmed' | 'checked_in' | 'completed' | 'exception'
type ReservationMutationKey = MerchantReservationActionKey
type ReservationPrimaryActionKey = ReservationMutationKey | ''

interface ReservationCardView extends ReservationResponse {
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

interface ReservationWorkbenchTabOption {
  key: ReservationWorkbenchTab
  label: string
}

interface RefreshPageOptions {
  showLoading?: boolean
  preserveList?: boolean
}

interface LoadReservationListOptions {
  showLoading?: boolean
  preserveCurrent?: boolean
}

interface ReservationActionSheetItem {
  label: string
  key: MerchantReservationActionKey | 'edit' | 'cancel_reason'
  reason?: string
  color?: 'default' | 'danger'
}

type ReservationActionSheetMode = 'actions' | 'cancel_reasons' | ''

const PAGE_AUTO_REFRESH_WINDOW_MS = 60 * 1000
const RESERVATION_CANCEL_REASONS = ['顾客临时取消', '桌台冲突需改约', '商户暂停营业', '信息填写有误', '其他原因']
const DEFAULT_TAB_OPTIONS: ReservationWorkbenchTabOption[] = [
  { key: 'all', label: '全部' },
  { key: 'paid', label: '待确认' },
  { key: 'confirmed', label: '已确认' },
  { key: 'checked_in', label: '已到店' },
  { key: 'completed', label: '已完成' },
  { key: 'exception', label: '异常' }
]

const getErrorMessage = getErrorUserMessage

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

function buildReservationCard(reservation: ReservationResponse): ReservationCardView {
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

function getReservationActionDialogConfig(
  actionKey: ReservationMutationKey,
  contact?: string,
  cancelReason?: string
): { title: string, content: string, confirmText: string, confirmTheme: 'primary' | 'danger' } {
  switch (actionKey) {
    case 'confirm':
      return {
        title: '确认预订',
        content: `确认 ${contact || '该顾客'} 的预订后，状态会进入“已确认”。`,
        confirmText: '确认预订',
        confirmTheme: 'primary'
      }
    case 'check_in':
      return {
        title: '登记顾客到店',
        content: `${contact || '该顾客'} 到店后，预订会进入“已到店”状态，可继续安排入座或完成预订。`,
        confirmText: '确认登记',
        confirmTheme: 'primary'
      }
    case 'start_cooking':
      return {
        title: '通知后厨起菜',
        content: `确认通知后厨为${contact || '该顾客'}备菜后，厨房看板会同步显示起菜状态。`,
        confirmText: '立即通知',
        confirmTheme: 'primary'
      }
    case 'complete':
      return {
        title: '完成预订',
        content: `确认 ${contact || '该顾客'} 已离店并完成就餐后，桌台会释放回空闲状态。`,
        confirmText: '确认完成',
        confirmTheme: 'primary'
      }
    case 'no_show':
      return {
        title: '标记未到店',
        content: `确认 ${contact || '该顾客'} 未到店后，预订会进入“未到店”状态且不可直接恢复。`,
        confirmText: '确认标记',
        confirmTheme: 'danger'
      }
    case 'cancel':
      return {
        title: '取消预订',
        content: `将按“${cancelReason || '其他原因'}”取消该预订。若已支付，系统会按当前退款策略处理。`,
        confirmText: '确认取消',
        confirmTheme: 'danger'
      }
    default:
      return {
        title: '确认操作',
        content: '请确认是否继续执行当前操作。',
        confirmText: '确认',
        confirmTheme: 'primary'
      }
  }
}

function getReservationActionLoadingText(actionKey: ReservationMutationKey): string {
  switch (actionKey) {
    case 'confirm':
      return '正在确认预订...'
    case 'check_in':
      return '正在登记到店...'
    case 'start_cooking':
      return '正在通知后厨...'
    case 'complete':
      return '正在完成预订...'
    case 'no_show':
      return '正在标记未到店...'
    case 'cancel':
      return '正在取消预订...'
    default:
      return '处理中...'
  }
}

function getReservationActionSuccessText(actionKey: ReservationMutationKey): string {
  switch (actionKey) {
    case 'confirm':
      return '预订已确认'
    case 'check_in':
      return '已登记到店'
    case 'start_cooking':
      return '已通知后厨起菜'
    case 'complete':
      return '预订已完成'
    case 'no_show':
      return '已标记未到店'
    case 'cancel':
      return '预订已取消'
    default:
      return '操作已完成'
  }
}

function buildListSummaryText(currentTab: ReservationWorkbenchTab, total: number) {
  if (currentTab === 'all') {
    return `当前共 ${total} 条预订`
  }

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

function getReservationListStatusFilter(tab: ReservationWorkbenchTab): MerchantReservationFilterStatus | undefined {
  if (tab === 'all') return undefined
  return tab
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
    pageSyncing: false,
    lastLoadedAt: 0,
    date: '',
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
    confirmDialogConfirmTheme: 'primary',
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
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({
      navBarHeight,
      date: dayjs().format('YYYY-MM-DD')
    })
    await this.initializePage()
  },

  async onShow() {
    if (
      !this.data.accessReady ||
      this.data.accessDenied ||
      this.data.accessErrorMessage ||
      this.data.initialLoading ||
      this.data.pageSyncing ||
      this.data.listLoading
    ) {
      return
    }

    if (!this.data.lastLoadedAt || Date.now() - this.data.lastLoadedAt >= PAGE_AUTO_REFRESH_WINDOW_MS) {
      await this.refreshPage({
        showLoading: false,
        preserveList: this.data.reservations.length > 0
      })
    }
  },

  async initializePage() {
    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: isMerchantConsoleAccessDenied(accessResult),
      accessErrorMessage: getMerchantConsoleAccessErrorMessage(accessResult)
    })

    if (!isMerchantConsoleAccessGranted(accessResult)) {
      this.setData({ initialLoading: false, pageSyncing: false })
      wx.stopPullDownRefresh()
      return
    }

    await this.refreshPage({ showLoading: true, preserveList: false })
  },

  async refreshPage(options?: RefreshPageOptions) {
    if (this.data.pageSyncing) return

    const showLoading = options?.showLoading !== false
    const preserveList = !!options?.preserveList
    const hasTrustedData = this.data.listAvailable

    this.setData({
      pageSyncing: true,
      ...(showLoading && !hasTrustedData
        ? {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: ''
          }
        : {})
    })

    const listOk = await this.loadReservationList(true, {
      showLoading,
      preserveCurrent: preserveList
    })

    this.setData({
      pageSyncing: false,
      initialLoading: false,
      ...(listOk
        ? {
            initialError: false,
            initialErrorMessage: '',
            lastLoadedAt: Date.now()
          }
        : !hasTrustedData
          ? {
              initialError: true,
              initialErrorMessage: '预订列表加载失败，请重试'
            }
          : {})
    })

    wx.stopPullDownRefresh()
  },

  async loadReservationList(reset = false, options?: LoadReservationListOptions) {
    if (this.data.listLoading) return false
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return false
    if (!reset && !this.data.listHasMore) return false

    const showLoading = options?.showLoading !== false
    const hasExistingReservations = this.data.reservations.length > 0
    const preserveCurrent = !!options?.preserveCurrent && reset && hasExistingReservations
    const shouldToggleLoading = !reset || showLoading || !preserveCurrent
    const page = reset ? 1 : this.data.listPage

    this.setData({
      ...(shouldToggleLoading ? { listLoading: true } : {}),
      ...(showLoading
        ? {
            listInitialError: false,
            listInitialErrorMessage: '',
            listRefreshErrorMessage: ''
          }
        : preserveCurrent
          ? { listRefreshErrorMessage: '' }
          : {})
    })

    try {
      const response = await ReservationService.getMerchantReservations({
        page_id: page,
        page_size: this.data.listPageSize,
        date: this.data.date,
        status: getReservationListStatusFilter(this.data.currentTab)
      })

      const nextReservations = Array.isArray(response.reservations)
        ? response.reservations.map(buildReservationCard)
        : []
      const total = typeof response.total === 'number' ? response.total : nextReservations.length
      const reservations = reset ? nextReservations : [...this.data.reservations, ...nextReservations]

      this.setData({
        reservations,
        listAvailable: true,
        listInitialError: false,
        listInitialErrorMessage: '',
        listRefreshErrorMessage: '',
        listPage: page + 1,
        listHasMore: page * this.data.listPageSize < total,
        listTotal: total,
        listSummaryText: buildListSummaryText(this.data.currentTab, total)
      })
      return true
    } catch (err) {
      logger.error('Load merchant reservation list failed', err)
      const message = getErrorMessage(err, '预订列表加载失败，请稍后重试')

      if (preserveCurrent || hasExistingReservations) {
        this.setData({ listRefreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        this.setData({
          reservations: [],
          listAvailable: false,
          listInitialError: true,
          listInitialErrorMessage: message,
          listPage: 1,
          listHasMore: true,
          listTotal: 0,
          listSummaryText: buildListSummaryText(this.data.currentTab, 0)
        })
      }
      return false
    } finally {
      if (shouldToggleLoading) {
        this.setData({ listLoading: false })
      }
      wx.stopPullDownRefresh()
    }
  },

  onOpenDatePicker() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage || this.data.pageSyncing || this.data.listLoading) {
      return
    }

    this.setData({ datePickerVisible: true })
  },

  onCloseDatePicker() {
    this.setData({ datePickerVisible: false })
  },

  applyDateSelection(nextDate: string) {
    if (!nextDate || nextDate === this.data.date) {
      this.setData({ datePickerVisible: false })
      return
    }

    this.setData({
      date: nextDate,
      datePickerVisible: false
    })

    void this.refreshPage({
      showLoading: false,
      preserveList: false
    })
  },

  onDateConfirm(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.applyDateSelection(e.detail?.value || '')
  },

  async onTabChange(e: WechatMiniprogram.CustomEvent<{ value: ReservationWorkbenchTab }>) {
    const nextTab = e.detail.value
    if (!nextTab || nextTab === this.data.currentTab || this.data.listLoading) return

    const previousTab = this.data.currentTab
    this.setData({ currentTab: nextTab })

    const success = await this.loadReservationList(true, {
      showLoading: false,
      preserveCurrent: this.data.reservations.length > 0
    })

    if (!success) {
      this.setData({
        currentTab: previousTab,
        listSummaryText: buildListSummaryText(previousTab, this.data.listTotal)
      })
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      void this.onRetryAccess()
      return
    }

    void this.refreshPage({
      showLoading: false,
      preserveList: this.data.reservations.length > 0
    })
  },

  onReachBottom() {
    void this.loadReservationList()
  },

  onLoadMore() {
    void this.loadReservationList()
  },

  onRetry() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      void this.onRetryAccess()
      return
    }

    void this.refreshPage({ showLoading: true, preserveList: false })
  },

  onManualRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      void this.onRetryAccess()
      return
    }

    void this.refreshPage({
      showLoading: false,
      preserveList: this.data.reservations.length > 0
    })
  },

  onRetryList() {
    void this.loadReservationList(true, { showLoading: true, preserveCurrent: false })
  },

  async onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      pageSyncing: false,
      lastLoadedAt: 0,
      listAvailable: false,
      listInitialError: false,
      listInitialErrorMessage: '',
      listRefreshErrorMessage: '',
      listLoading: false,
      reservations: [],
      listPage: 1,
      listHasMore: true,
      listTotal: 0,
      datePickerVisible: false,
      listSummaryText: '当前共 0 条预订',
      actionSubmittingKey: '',
      confirmDialogVisible: false,
      confirmDialogTitle: '',
      confirmDialogContent: '',
      confirmDialogConfirmText: '确认',
      confirmDialogConfirmTheme: 'primary',
      confirmDialogSubmitting: false,
      confirmDialogAction: '',
      confirmDialogReservationId: 0,
      confirmDialogContact: '',
      confirmDialogCancelReason: '',
      actionSheetVisible: false,
      actionSheetMode: '',
      actionSheetDescription: '',
      actionSheetItems: [],
      actionSheetReservationId: 0,
      actionSheetReservationContact: ''
    })

    await this.initializePage()
  },

  onActionsCatch() {},

  getReservationCard(id: number) {
    return this.data.reservations.find((item) => item.id === id)
  },

  openReservationEditPage(id?: number) {
    const baseUrl = '/pages/merchant/reservations/edit/index'
    const url = id ? `${baseUrl}?id=${id}` : `${baseUrl}?date=${this.data.date}`
    wx.navigateTo({ url })
  },

  onOpenCreatePage() {
    this.openReservationEditPage()
  },

  onReservationCardTap(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    this.openReservationEditPage(id)
  },

  onOpenEditPage(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    this.openReservationEditPage(id)
  },

  showFeedbackToast(theme: 'success' | 'warning' | 'error', message: string, duration = 2200) {
    Toast({
      context: this,
      selector: '#t-toast',
      theme,
      message,
      placement: 'middle',
      duration,
      direction: 'column'
    })
  },

  showLoadingToast(message: string) {
    Toast({
      context: this,
      selector: '#t-toast',
      theme: 'loading',
      message,
      placement: 'middle',
      duration: 0,
      direction: 'column',
      preventScrollThrough: true
    })
  },

  hideLoadingToast() {
    hideToast({ context: this, selector: '#t-toast' })
  },

  openReservationActionSheet(
    mode: ReservationActionSheetMode,
    reservationId: number,
    contact: string,
    description: string,
    items: ReservationActionSheetItem[]
  ) {
    if (!reservationId || !items.length) return

    this.setData({
      actionSheetVisible: true,
      actionSheetMode: mode,
      actionSheetDescription: description,
      actionSheetItems: items,
      actionSheetReservationId: reservationId,
      actionSheetReservationContact: contact || ''
    })
  },

  resetReservationActionSheet() {
    this.setData({
      actionSheetVisible: false,
      actionSheetMode: '',
      actionSheetDescription: '',
      actionSheetItems: [],
      actionSheetReservationId: 0,
      actionSheetReservationContact: ''
    })
  },

  openCancelReasonSheet(reservationId: number, contact?: string) {
    this.openReservationActionSheet(
      'cancel_reasons',
      reservationId,
      contact || '',
      '请选择取消原因',
      RESERVATION_CANCEL_REASONS.map((reason) => ({
        label: reason,
        key: 'cancel_reason',
        reason,
        color: 'default'
      }))
    )
  },

  onActionSheetCancel() {
    this.resetReservationActionSheet()
  },

  onActionSheetClose() {
    this.resetReservationActionSheet()
  },

  onActionSheetSelected(
    e: WechatMiniprogram.CustomEvent<{ selected?: ReservationActionSheetItem | string, index?: number }>
  ) {
    const selected = e.detail?.selected
    const selectedItem = typeof selected === 'string' ? undefined : selected
    const selectedKey = typeof selected === 'string' ? selected : selected?.key
    const mode = this.data.actionSheetMode
    const reservationId = this.data.actionSheetReservationId
    const contact = this.data.actionSheetReservationContact

    this.resetReservationActionSheet()

    if (!reservationId || !selectedKey) {
      return
    }

    if (mode === 'actions') {
      if (selectedKey === 'edit') {
        this.openReservationEditPage(reservationId)
        return
      }

      if (selectedKey === 'cancel') {
        this.openCancelReasonSheet(reservationId, contact)
        return
      }

      void this.triggerReservationActionByKey(reservationId, selectedKey as ReservationMutationKey, contact)
      return
    }

    if (mode === 'cancel_reasons' && selectedKey === 'cancel_reason' && selectedItem?.reason) {
      this.openConfirmDialog('cancel', reservationId, contact, selectedItem.reason)
    }
  },

  openConfirmDialog(
    actionKey: ReservationMutationKey,
    reservationId: number,
    contact?: string,
    cancelReason?: string
  ) {
    const dialog = getReservationActionDialogConfig(actionKey, contact, cancelReason)

    this.setData({
      confirmDialogVisible: true,
      confirmDialogTitle: dialog.title,
      confirmDialogContent: dialog.content,
      confirmDialogConfirmText: dialog.confirmText,
      confirmDialogConfirmTheme: dialog.confirmTheme,
      confirmDialogSubmitting: false,
      confirmDialogAction: actionKey,
      confirmDialogReservationId: reservationId,
      confirmDialogContact: contact || '',
      confirmDialogCancelReason: cancelReason || ''
    })
  },

  resetConfirmDialogState() {
    this.setData({
      confirmDialogVisible: false,
      confirmDialogTitle: '',
      confirmDialogContent: '',
      confirmDialogConfirmText: '确认',
      confirmDialogConfirmTheme: 'primary',
      confirmDialogSubmitting: false,
      confirmDialogAction: '',
      confirmDialogReservationId: 0,
      confirmDialogContact: '',
      confirmDialogCancelReason: ''
    })
  },

  onCancelConfirmDialog() {
    if (this.data.confirmDialogSubmitting) return
    this.resetConfirmDialogState()
  },

  async executeReservationAction(
    reservationId: number,
    actionKey: ReservationMutationKey,
    cancelReason?: string
  ) {
    switch (actionKey) {
      case 'confirm':
        return confirmReservationByMerchant(reservationId)
      case 'check_in':
        return checkInReservation(reservationId)
      case 'start_cooking':
        return startCookingReservation(reservationId)
      case 'complete':
        return completeReservationByMerchant(reservationId)
      case 'no_show':
        return markReservationNoShow(reservationId)
      case 'cancel':
        return cancelReservation(reservationId, cancelReason)
      default:
        return undefined
    }
  },

  async onConfirmDialogAction() {
    const reservationId = Number(this.data.confirmDialogReservationId || 0)
    const actionKey = this.data.confirmDialogAction as ReservationMutationKey | ''
    const cancelReason = this.data.confirmDialogCancelReason || ''

    if (!reservationId || !actionKey || this.data.confirmDialogSubmitting) {
      this.onCancelConfirmDialog()
      return
    }

    const submittingKey = `${actionKey}:${reservationId}`
    let loadingToastVisible = false

    this.setData({
      confirmDialogSubmitting: true,
      actionSubmittingKey: submittingKey
    })
    this.showLoadingToast(getReservationActionLoadingText(actionKey))
    loadingToastVisible = true

    try {
      await this.executeReservationAction(reservationId, actionKey, cancelReason)
      if (loadingToastVisible) {
        this.hideLoadingToast()
        loadingToastVisible = false
      }

      this.resetConfirmDialogState()
      await this.refreshPage({
        showLoading: false,
        preserveList: this.data.reservations.length > 0
      })
      this.showFeedbackToast('success', getReservationActionSuccessText(actionKey))
    } catch (err) {
      logger.error(`Reservation action failed: ${actionKey}`, err)
      if (loadingToastVisible) {
        this.hideLoadingToast()
        loadingToastVisible = false
      }

      this.resetConfirmDialogState()
      if (actionKey === 'check_in') {
        await this.refreshPage({
          showLoading: false,
          preserveList: this.data.reservations.length > 0
        })
      }

      this.showFeedbackToast('warning', getErrorMessage(err, '操作失败'))
    } finally {
      if (loadingToastVisible) {
        this.hideLoadingToast()
      }
      this.setData({
        confirmDialogSubmitting: false,
        actionSubmittingKey: ''
      })
    }
  },

  async triggerReservationActionByKey(reservationId: number, actionKey: ReservationMutationKey, contact?: string) {
    if (!reservationId) return

    switch (actionKey) {
      case 'confirm':
        this.openConfirmDialog('confirm', reservationId, contact)
        return
      case 'check_in':
        this.openConfirmDialog('check_in', reservationId, contact)
        return
      case 'start_cooking':
        this.openConfirmDialog('start_cooking', reservationId, contact)
        return
      case 'complete':
        this.openConfirmDialog('complete', reservationId, contact)
        return
      case 'no_show':
        this.openConfirmDialog('no_show', reservationId, contact)
        return
      case 'cancel':
        this.openCancelReasonSheet(reservationId, contact)
        return
      default:
        return
    }
  },

  onPrimaryAction(e: WechatMiniprogram.TouchEvent) {
    const { id, action, contact } = e.currentTarget.dataset as {
      id?: number
      action?: ReservationPrimaryActionKey
      contact?: string
    }

    if (!id || !action) return
    void this.triggerReservationActionByKey(id, action, contact)
  },

  onOpenMoreActions(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    const reservation = this.getReservationCard(id)
    if (!reservation) return

    const optionDefs: Array<{ label: string, key: ReservationMutationKey | 'edit' }> = []
    if (reservation.canEdit) {
      optionDefs.push({ label: '编辑预订', key: 'edit' })
    }
    if (reservation.canConfirm && reservation.primaryActionKey !== 'confirm') {
      optionDefs.push({ label: '确认预订', key: 'confirm' })
    }
    if (reservation.canCheckIn && reservation.primaryActionKey !== 'check_in') {
      optionDefs.push({ label: '登记到店', key: 'check_in' })
    }
    if (reservation.canStartCooking && reservation.primaryActionKey !== 'start_cooking') {
      optionDefs.push({ label: '通知后厨起菜', key: 'start_cooking' })
    }
    if (reservation.canComplete && reservation.primaryActionKey !== 'complete') {
      optionDefs.push({ label: '完成预订', key: 'complete' })
    }
    if (reservation.canNoShow) {
      optionDefs.push({ label: '标记未到店', key: 'no_show' })
    }
    if (reservation.canCancel) {
      optionDefs.push({ label: '取消预订', key: 'cancel' })
    }

    if (!optionDefs.length) return

    this.openReservationActionSheet(
      'actions',
      id,
      reservation.contact_name,
      `${reservation.contact_name || '当前顾客'} 的预订操作`,
      optionDefs.map((item) => ({
        label: item.label,
        key: item.key,
        color: item.key === 'cancel' || item.key === 'no_show' ? 'danger' : 'default'
      }))
    )
  }
})