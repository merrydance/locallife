import dayjs from 'dayjs'
import { getStableBarHeights } from '../../../utils/responsive'
import {
  cancelReservation,
  completeReservationByMerchant,
  confirmReservationByMerchant,
  markReservationNoShow,
  MerchantReservationDishSummaryItem,
  merchantCreateReservation,
  ReservationResponse,
  ReservationStatus,
  ReservationService,
  updateReservation
} from '../../../api/reservation'
import { tableManagementService, TableResponse } from '../../../api/table-device-management'
import { logger } from '../../../utils/logger'
import { settleAll } from '../../../utils/promise'
import { getErrorUserMessage } from '../../../utils/user-facing'

type ReservationFilterTab = 'all' | ReservationStatus
type ReservationStatusTheme = 'primary' | 'warning' | 'success' | 'danger' | 'default'
type ReservationCreateSource = 'phone' | 'walkin' | 'merchant'

interface ReservationCardView extends ReservationResponse {
  statusLabel: string
  statusTheme: ReservationStatusTheme
  tableLabel: string
  contactLabel: string
  paymentLabel: string
  itemPreview: string
  canEdit: boolean
  canCancel: boolean
  canConfirm: boolean
  canNoShow: boolean
  canComplete: boolean
}

interface ReservationCreateTableOption {
  id: number
  label: string
}

interface ReservationCreateForm {
  table_id: number
  date: string
  time: string
  guest_count: string
  contact_name: string
  contact_phone: string
  source: ReservationCreateSource
  notes: string
}

function getDefaultReservationTime() {
  return dayjs().add(1, 'hour').startOf('hour').format('HH:mm')
}

function createDefaultReservationForm(date: string): ReservationCreateForm {
  return {
    table_id: 0,
    date,
    time: getDefaultReservationTime(),
    guest_count: '2',
    contact_name: '',
    contact_phone: '',
    source: 'merchant',
    notes: ''
  }
}

function normalizeReservationTime(value?: string) {
  if (!value) return getDefaultReservationTime()
  return value.slice(0, 5)
}

function buildReservationForm(reservation: ReservationResponse): ReservationCreateForm {
  return {
    table_id: reservation.table_id,
    date: reservation.reservation_date,
    time: normalizeReservationTime(reservation.reservation_time),
    guest_count: String(reservation.guest_count || 1),
    contact_name: reservation.contact_name || '',
    contact_phone: reservation.contact_phone || '',
    source: 'merchant',
    notes: reservation.notes || ''
  }
}

function formatTableStatus(status: string) {
  const statusMap: Record<string, string> = {
    available: '空闲',
    occupied: '就餐中',
    reserved: '已预订',
    disabled: '停用'
  }
  return statusMap[status] || status
}

function buildTableOption(table: TableResponse): ReservationCreateTableOption {
  return {
    id: table.id,
    label: `${table.table_no} · ${table.capacity}人 · ${formatTableStatus(table.status)}`
  }
}

function formatReservationStatus(status: ReservationStatus): string {
  const statusMap: Record<ReservationStatus, string> = {
    pending: '待支付',
    paid: '待确认',
    confirmed: '已确认',
    checked_in: '已到店',
    completed: '已完成',
    cancelled: '已取消',
    expired: '已过期',
    no_show: '未到店'
  }
  return statusMap[status] || status
}

function getStatusTheme(status: ReservationStatus): ReservationStatusTheme {
  if (status === 'paid' || status === 'pending') return 'warning'
  if (status === 'confirmed' || status === 'checked_in') return 'primary'
  if (status === 'completed') return 'success'
  if (status === 'cancelled' || status === 'expired' || status === 'no_show') return 'danger'
  return 'default'
}

function formatPaymentMode(mode?: string): string {
  if (mode === 'full') return '全款预订'
  return '定金预订'
}

function formatReservationItems(reservation: ReservationResponse): string {
  if (!reservation.items?.length) return '未预点菜'

  return reservation.items
    .slice(0, 2)
    .map((item) => `${item.name || (item.type === 'combo' ? '套餐' : '菜品')} x${item.quantity}`)
    .join('，')
}

function buildReservationCard(reservation: ReservationResponse): ReservationCardView {
  return {
    ...reservation,
    statusLabel: formatReservationStatus(reservation.status),
    statusTheme: getStatusTheme(reservation.status),
    tableLabel: reservation.table_no || '未分配桌台',
    contactLabel: `${reservation.contact_name} · ${reservation.contact_phone}`,
    paymentLabel: formatPaymentMode(reservation.payment_mode),
    itemPreview: formatReservationItems(reservation),
    canEdit: !['completed', 'cancelled', 'expired'].includes(reservation.status),
    canCancel: ['pending', 'paid', 'confirmed'].includes(reservation.status),
    canConfirm: reservation.status === 'paid',
    canNoShow: reservation.status === 'paid' || reservation.status === 'confirmed',
    canComplete: reservation.status === 'confirmed' || reservation.status === 'checked_in'
  }
}

function filterReservations(
  reservations: ReservationCardView[],
  tab: ReservationFilterTab
) {
  if (tab === 'all') return reservations
  return reservations.filter((reservation) => reservation.status === tab)
}

const getErrorMessage = getErrorUserMessage

async function loadAllMerchantReservations(date: string) {
  const reservations: ReservationResponse[] = []
  const pageSize = 50
  let pageId = 1
  let total = 0
  let hasMorePages = true

  while (hasMorePages) {
    const response = await ReservationService.getMerchantReservations({
      page_id: pageId,
      page_size: pageSize,
      date
    })

    const pageReservations = Array.isArray(response.reservations) ? response.reservations : []
    total = typeof response.total === 'number' ? response.total : Math.max(total, reservations.length + pageReservations.length)
    reservations.push(...pageReservations)

    hasMorePages = !(pageReservations.length < pageSize || reservations.length >= total)
    if (hasMorePages) {
      pageId += 1
    }
  }

  return {
    reservations,
    total: total || reservations.length
  }
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    reservationsAvailable: false,
    reservationsError: false,
    reservationsErrorMessage: '',
    dishSummaryAvailable: false,
    dishSummaryError: false,
    dishSummaryErrorMessage: '',
    loading: false,
    date: '',
    currentTab: 'all' as ReservationFilterTab,
    reservations: [] as ReservationCardView[],
    filteredReservations: [] as ReservationCardView[],
    dishSummary: [] as MerchantReservationDishSummaryItem[],
    createPopupVisible: false,
    createSubmitting: false,
    actionSubmittingKey: '',
    formMode: 'create' as 'create' | 'edit',
    editingReservationId: 0,
    tableOptionsLoading: false,
    tableOptions: [] as ReservationCreateTableOption[],
    selectedTableIndex: 0,
    createForm: createDefaultReservationForm('') as ReservationCreateForm,
    totalReservations: 0,
    hasMoreReservations: false,
    summary: {
      reservationCount: 0,
      pendingCount: 0,
      confirmedCount: 0,
      checkedInCount: 0,
      completedCount: 0,
      tableCount: 0,
      dishKinds: 0,
      dishTotalQuantity: 0
    }
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    const today = dayjs().format('YYYY-MM-DD')
    this.setData({
      navBarHeight,
      date: today,
      createForm: createDefaultReservationForm(today)
    })
    this.loadData(today)
  },

  async loadData(date: string, showLoading = true) {
    if (this.data.loading) return

    const canPreserveReservations = !showLoading && this.data.reservationsAvailable
    const canPreserveDishSummary = !showLoading && this.data.dishSummaryAvailable

    this.setData({
      loading: true,
      ...(showLoading
        ? {
            initialError: false,
            initialErrorMessage: '',
            reservationsError: false,
            reservationsErrorMessage: '',
            dishSummaryError: false,
            dishSummaryErrorMessage: ''
          }
        : {})
    })
    try {
      const [reservationResult, dishSummaryResult] = await settleAll([
        loadAllMerchantReservations(date),
        ReservationService.getMerchantReservationDishes(date)
      ] as const)

      if (reservationResult.status === 'rejected' && dishSummaryResult.status === 'rejected' && !canPreserveReservations && !canPreserveDishSummary) {
        throw reservationResult.reason
      }

      const nextState: Record<string, unknown> = {
        initialLoading: false,
        initialError: false,
        initialErrorMessage: ''
      }

      if (reservationResult.status === 'fulfilled') {
        const reservationRes = reservationResult.value
        const reservations = Array.isArray(reservationRes.reservations)
          ? reservationRes.reservations.map(buildReservationCard)
          : []
        const filteredReservations = filterReservations(reservations, this.data.currentTab)
        const tableCount = new Set(
          reservations
            .map((reservation) => reservation.table_no)
            .filter((tableNo): tableNo is string => !!tableNo)
        ).size

        nextState.reservations = reservations
        nextState.filteredReservations = filteredReservations
        nextState.totalReservations = reservationRes.total || reservations.length
        nextState.hasMoreReservations = false
        nextState.reservationsAvailable = true
        nextState.reservationsError = false
        nextState.reservationsErrorMessage = ''
        nextState.summary = {
          ...this.data.summary,
          reservationCount: reservationRes.total || reservations.length,
          pendingCount: reservations.filter((item) => item.status === 'paid' || item.status === 'pending').length,
          confirmedCount: reservations.filter((item) => item.status === 'confirmed').length,
          checkedInCount: reservations.filter((item) => item.status === 'checked_in').length,
          completedCount: reservations.filter((item) => item.status === 'completed').length,
          tableCount
        }
      } else if (canPreserveReservations) {
        nextState.reservationsError = true
        nextState.reservationsErrorMessage = `${getErrorMessage(reservationResult.reason, '预订列表同步失败')}，当前已保留上次结果`
      } else {
        nextState.reservations = []
        nextState.filteredReservations = []
        nextState.totalReservations = 0
        nextState.hasMoreReservations = false
        nextState.reservationsAvailable = false
        nextState.reservationsError = true
        nextState.reservationsErrorMessage = getErrorMessage(reservationResult.reason, '预订列表加载失败，请稍后重试')
        nextState.summary = {
          ...this.data.summary,
          reservationCount: 0,
          pendingCount: 0,
          confirmedCount: 0,
          checkedInCount: 0,
          completedCount: 0,
          tableCount: 0
        }
      }

      if (dishSummaryResult.status === 'fulfilled') {
        const dishSummary = dishSummaryResult.value.items || []
        const dishTotalQuantity = dishSummary.reduce((sum, item) => sum + (item.total_quantity || 0), 0)
        nextState.dishSummary = dishSummary
        nextState.dishSummaryAvailable = true
        nextState.dishSummaryError = false
        nextState.dishSummaryErrorMessage = ''
        nextState.summary = {
          ...(nextState.summary as typeof this.data.summary || this.data.summary),
          dishKinds: dishSummary.length,
          dishTotalQuantity
        }
      } else if (canPreserveDishSummary) {
        nextState.dishSummaryError = true
        nextState.dishSummaryErrorMessage = `${getErrorMessage(dishSummaryResult.reason, '备菜明细同步失败')}，当前已保留上次结果`
      } else {
        nextState.dishSummary = []
        nextState.dishSummaryAvailable = false
        nextState.dishSummaryError = true
        nextState.dishSummaryErrorMessage = getErrorMessage(dishSummaryResult.reason, '备菜明细加载失败，请稍后重试')
        nextState.summary = {
          ...(nextState.summary as typeof this.data.summary || this.data.summary),
          dishKinds: 0,
          dishTotalQuantity: 0
        }
      }

      this.setData(nextState)
    } catch (err) {
      logger.error('Load reservation summary failed', err)
      const message = getErrorMessage(err, '加载预订数据失败')
      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false, initialLoading: false })
      wx.stopPullDownRefresh()
    }
  },

  onPullDownRefresh() {
    this.loadData(this.data.date, false)
  },

  onDateChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const date = e.detail.value
    this.setData({ date })
    this.loadData(date)
  },

  async loadTableOptions() {
    if (this.data.tableOptionsLoading) return

    this.setData({ tableOptionsLoading: true })
    try {
      const response = await tableManagementService.listTables()
      const tableOptions = Array.isArray(response.tables)
        ? response.tables
            .filter((table) => table.status !== 'disabled')
            .map(buildTableOption)
        : []

      const selectedTableIndex = tableOptions.findIndex((item) => item.id === this.data.createForm.table_id)

      this.setData({
        tableOptions,
        selectedTableIndex: selectedTableIndex >= 0 ? selectedTableIndex : 0,
        'createForm.table_id': selectedTableIndex >= 0 ? tableOptions[selectedTableIndex].id : (tableOptions[0]?.id || 0)
      })
    } catch (err) {
      logger.error('Load reservation table options failed', err)
      wx.showToast({ title: '加载桌台列表失败', icon: 'none' })
    } finally {
      this.setData({ tableOptionsLoading: false })
    }
  },

  async onOpenCreatePopup() {
    const date = this.data.date || dayjs().format('YYYY-MM-DD')
    this.setData({
      createPopupVisible: true,
      formMode: 'create',
      editingReservationId: 0,
      selectedTableIndex: 0,
      createForm: createDefaultReservationForm(date)
    })
    await this.loadTableOptions()
  },

  async onOpenEditPopup(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    const reservation = this.data.reservations.find((item) => item.id === id)
    if (!reservation) return

    this.setData({
      createPopupVisible: true,
      formMode: 'edit',
      editingReservationId: id,
      selectedTableIndex: 0,
      createForm: buildReservationForm(reservation)
    })
    await this.loadTableOptions()
  },

  onCloseCreatePopup() {
    this.setData({ createPopupVisible: false })
  },

  onCreatePopupVisibleChange(e: WechatMiniprogram.CustomEvent<{ visible: boolean }>) {
    if (!e.detail.visible) {
      this.onCloseCreatePopup()
    }
  },

  onCreateTableChange(e: WechatMiniprogram.CustomEvent<{ value: number }>) {
    const selectedTableIndex = Number(e.detail.value || 0)
    const selected = this.data.tableOptions[selectedTableIndex]
    this.setData({
      selectedTableIndex,
      'createForm.table_id': selected?.id || 0
    })
  },

  onCreateDateFieldChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ 'createForm.date': e.detail.value })
  },

  onCreateTimeFieldChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ 'createForm.time': e.detail.value })
  },

  onCreateSourceChange(e: WechatMiniprogram.CustomEvent<{ value: ReservationCreateSource }>) {
    this.setData({ 'createForm.source': e.detail.value })
  },

  onCreateFieldChange(
    e: WechatMiniprogram.CustomEvent<{ value: string }> & {
      currentTarget: { dataset: { field: string } }
    }
  ) {
    const { field } = e.currentTarget.dataset
    this.setData({ [`createForm.${field}`]: e.detail.value || '' })
  },

  async onSubmitCreateReservation() {
    if (this.data.createSubmitting) return

    const form = this.data.createForm
    const isEdit = this.data.formMode === 'edit'
    if (!form.table_id) {
      wx.showToast({ title: '请选择桌台', icon: 'none' })
      return
    }
    if (!form.date) {
      wx.showToast({ title: '请选择预订日期', icon: 'none' })
      return
    }
    if (!form.time) {
      wx.showToast({ title: '请选择预订时间', icon: 'none' })
      return
    }
    if (!form.contact_name.trim()) {
      wx.showToast({ title: '请填写联系人姓名', icon: 'none' })
      return
    }
    if (!form.contact_phone.trim()) {
      wx.showToast({ title: '请填写联系人电话', icon: 'none' })
      return
    }

    const guestCount = Number(form.guest_count)
    if (!Number.isFinite(guestCount) || guestCount < 1 || guestCount > 50) {
      wx.showToast({ title: '预订人数需在 1-50 之间', icon: 'none' })
      return
    }

    this.setData({ createSubmitting: true })
    wx.showLoading({ title: isEdit ? '保存中...' : '创建中...' })

    try {
      if (isEdit) {
        await updateReservation(this.data.editingReservationId, {
          table_id: form.table_id,
          date: form.date,
          time: form.time,
          guest_count: guestCount,
          contact_name: form.contact_name.trim(),
          contact_phone: form.contact_phone.trim(),
          notes: form.notes.trim() || undefined
        })
      } else {
        await merchantCreateReservation({
          table_id: form.table_id,
          date: form.date,
          time: form.time,
          guest_count: guestCount,
          contact_name: form.contact_name.trim(),
          contact_phone: form.contact_phone.trim(),
          source: form.source,
          notes: form.notes.trim() || undefined
        })
      }

      const nextTab: ReservationFilterTab = this.data.currentTab === 'all' || this.data.currentTab === 'confirmed'
        ? this.data.currentTab
        : 'confirmed'

      this.setData({
        createPopupVisible: false,
        currentTab: nextTab,
        date: form.date,
        formMode: 'create',
        editingReservationId: 0,
        createForm: createDefaultReservationForm(form.date)
      })
      await this.loadData(form.date)
    } catch (err) {
      logger.error(isEdit ? 'Update merchant reservation failed' : 'Create merchant reservation failed', err)
      const message = typeof err === 'object' && err !== null && 'userMessage' in err
        ? (err as { userMessage?: string }).userMessage || (isEdit ? '更新预订失败' : '创建预订失败')
        : (isEdit ? '更新预订失败' : '创建预订失败')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ createSubmitting: false })
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: ReservationFilterTab }>) {
    const currentTab = e.detail.value
    this.setData({
      currentTab,
      filteredReservations: filterReservations(this.data.reservations, currentTab)
    })
  },

  onRetry() {
    this.loadData(this.data.date)
  },

  onRetryReservations() {
    this.loadData(this.data.date, false)
  },

  onRetryDishSummary() {
    this.loadData(this.data.date, false)
  },

  async runReservationAction(
    reservationId: number,
    actionKey: 'cancel' | 'confirm' | 'no_show' | 'complete',
    request: () => Promise<unknown>,
    modalTitle: string,
    modalContent: string,
    _successMessage: string
  ) {
    if (this.data.actionSubmittingKey) return

    const confirmed = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: modalTitle,
        content: modalContent,
        confirmText: '确认',
        success: (res) => resolve(Boolean(res.confirm)),
        fail: () => resolve(false)
      })
    })

    if (!confirmed) return

    const submittingKey = `${actionKey}:${reservationId}`
    this.setData({ actionSubmittingKey: submittingKey })
    wx.showLoading({ title: '处理中...' })

    try {
      await request()
      await this.loadData(this.data.date)
    } catch (err) {
      logger.error(`Reservation action failed: ${actionKey}`, err)
      const message = typeof err === 'object' && err !== null && 'userMessage' in err
        ? (err as { userMessage?: string }).userMessage || '操作失败'
        : '操作失败'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ actionSubmittingKey: '' })
    }
  },

  onConfirmReservation(e: WechatMiniprogram.TouchEvent) {
    const { id, contact } = e.currentTarget.dataset as { id?: number, contact?: string }
    if (!id) return

    this.runReservationAction(
      id,
      'confirm',
      () => confirmReservationByMerchant(id),
      '确认预订',
      `确认 ${contact || '该顾客'} 的预订后，状态会进入“已确认”。`,
      '预订已确认'
    )
  },

  onMarkNoShow(e: WechatMiniprogram.TouchEvent) {
    const { id, contact } = e.currentTarget.dataset as { id?: number, contact?: string }
    if (!id) return

    this.runReservationAction(
      id,
      'no_show',
      () => markReservationNoShow(id),
      '标记未到店',
      `确认 ${contact || '该顾客'} 未到店后，预订会进入“未到店”状态且不可直接恢复。`,
      '已标记未到店'
    )
  },

  onCompleteReservation(e: WechatMiniprogram.TouchEvent) {
    const { id, contact } = e.currentTarget.dataset as { id?: number, contact?: string }
    if (!id) return

    this.runReservationAction(
      id,
      'complete',
      () => completeReservationByMerchant(id),
      '完成预订',
      `确认 ${contact || '该顾客'} 已离店并完成就餐后，桌台会释放回空闲状态。`,
      '预订已完成'
    )
  },

  onCancelReservation(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id || this.data.actionSubmittingKey) return

    const reasons = ['顾客临时取消', '桌台冲突需改约', '商户暂停营业', '信息填写有误', '其他原因']
    wx.showActionSheet({
      itemList: reasons,
      success: ({ tapIndex }) => {
        const reason = reasons[tapIndex]
        if (!reason) return

        this.runReservationAction(
          id,
          'cancel',
          () => cancelReservation(id, reason),
          '取消预订',
          `将按“${reason}”取消该预订。若已支付，系统会按当前退款策略处理。`,
          '预订已取消'
        )
      }
    })
  },

  onGoOrderList() {
    wx.navigateTo({ url: '/pages/merchant/orders/list/index?status=paid&order_type=reservation' })
  }
})
