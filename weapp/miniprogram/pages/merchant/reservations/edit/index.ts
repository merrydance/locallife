import dayjs from '../../_main_shared/miniprogram_npm/dayjs/index'
import {
  formatReservationStatus,
  isMerchantReservationEditable,
  MerchantCreateReservationRequest,
  ReservationResponse,
  ReservationService,
  ReservationSource,
  UpdateReservationRequest
} from '../../_main_shared/api/reservation'
import { isTableStatusDisabled } from '../../_main_shared/api/table'
import { tableManagementService, TableResponse } from '../../../../api/table-device-management'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface ReservationEditPageOptions {
  id?: string
  date?: string
}

interface ReservationEditFormData {
  table_id: number
  date: string
  time: string
  guest_count: string
  contact_name: string
  contact_phone: string
  source: ReservationSource
  notes: string
}

interface PickerOption {
  label: string
  value: string
}

interface ReservationListPageBridge {
  getReservationCard?: (id: number) => ReservationResponse | undefined
  refreshPage?: (options?: { showLoading?: boolean, preserveList?: boolean }) => void | Promise<void>
}

const DEFAULT_SOURCE: ReservationSource = 'merchant'

function resolveInitialDate(input?: string) {
  return /^\d{4}-\d{2}-\d{2}$/.test(input || '') ? String(input) : dayjs().format('YYYY-MM-DD')
}

function getDefaultReservationTime() {
  return dayjs().add(1, 'hour').minute(0).format('HH:mm')
}

function normalizeReservationTime(value?: string) {
  if (!value) return getDefaultReservationTime()
  const matched = String(value).match(/^(\d{2}:\d{2})/)
  return matched?.[1] || getDefaultReservationTime()
}

function createDefaultForm(date: string): ReservationEditFormData {
  return {
    table_id: 0,
    date,
    time: getDefaultReservationTime(),
    guest_count: '2',
    contact_name: '',
    contact_phone: '',
    source: DEFAULT_SOURCE,
    notes: ''
  }
}

function buildFormFromReservation(reservation: ReservationResponse): ReservationEditFormData {
  return {
    table_id: reservation.table_id,
    date: reservation.reservation_date,
    time: normalizeReservationTime(reservation.reservation_time),
    guest_count: String(reservation.guest_count || ''),
    contact_name: reservation.contact_name || '',
    contact_phone: reservation.contact_phone || '',
    source: reservation.source || DEFAULT_SOURCE,
    notes: reservation.notes || ''
  }
}

function getTableTypeLabel(tableType?: string) {
  return tableType === 'room' ? '包间' : '散台'
}

function buildTableOption(table: Pick<TableResponse, 'id' | 'table_no' | 'capacity' | 'table_type'>): PickerOption {
  const parts = [table.table_no || '未命名桌台']
  if (table.capacity) {
    parts.push(`${table.capacity}人`)
  }
  parts.push(getTableTypeLabel(table.table_type))

  return {
    value: String(table.id),
    label: parts.join(' · ')
  }
}

function findTableLabel(options: PickerOption[], value: string) {
  return options.find((item) => item.value === value)?.label || ''
}

Page({
  data: {
    navBarHeight: 88,
    isEdit: false,
    isEditable: true,
    reservationId: 0,
    statusLabel: '',
    initialDate: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    loadWarningMessage: '',
    submitting: false,
    tablePickerVisible: false,
    datePickerVisible: false,
    timePickerVisible: false,
    tableOptions: [] as PickerOption[],
    selectedTableValue: '',
    selectedTableLabel: '',
    formData: createDefaultForm(dayjs().format('YYYY-MM-DD'))
  },

  async onLoad(options: ReservationEditPageOptions) {
    const { navBarHeight } = getStableBarHeights()
    const reservationId = Number(options?.id || 0)
    const isEdit = Number.isFinite(reservationId) && reservationId > 0
    const initialDate = resolveInitialDate(options?.date)

    this.setData({
      navBarHeight,
      isEdit,
      reservationId: isEdit ? reservationId : 0,
      initialDate,
      formData: createDefaultForm(initialDate)
    })

    await this.loadPageData()
  },

  async resolveReservationForEdit() {
    const reservationId = this.data.reservationId
    if (!reservationId) return null

    const pages = getCurrentPages()
    const previousPage = pages[pages.length - 2] as ReservationListPageBridge | undefined
    const previousReservation = previousPage?.getReservationCard?.(reservationId)
    if (previousReservation) {
      return previousReservation
    }

    return ReservationService.getReservationDetail(reservationId)
  },

  async loadPageData() {
    this.setData({
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      loadWarningMessage: ''
    })

    try {
      const reservation = this.data.isEdit ? await this.resolveReservationForEdit() : null
      if (this.data.isEdit && !reservation) {
        throw new Error('预订信息缺失，请返回列表后重新进入')
      }

      const tableResponse = await tableManagementService.listTables()
      const rawTables = Array.isArray(tableResponse.tables) ? tableResponse.tables : []
      const currentTableId = reservation?.table_id || 0

      const tableOptions = rawTables
        .filter((table) => !isTableStatusDisabled(table.status) || table.id === currentTableId)
        .map(buildTableOption)

      let selectedTableValue = ''
      let selectedTableLabel = ''
      let loadWarningMessage = ''

      if (reservation) {
        const currentValue = String(reservation.table_id)
        const fallbackOption = currentValue && !tableOptions.some((item) => item.value === currentValue)
          ? {
              value: currentValue,
              label: `${reservation.table_no || '当前桌台'} · 当前预订桌台`
            }
          : null

        if (fallbackOption) {
          tableOptions.unshift(fallbackOption)
          loadWarningMessage = '当前预订桌台已不在正常可选列表中，保存前请确认是否需要调整桌台。'
        }

        selectedTableValue = currentValue
        selectedTableLabel = findTableLabel(tableOptions, currentValue)
      } else if (!tableOptions.length) {
        throw new Error('暂无可选桌台，请先在桌台管理中创建并启用桌台')
      }

      const formData = reservation ? buildFormFromReservation(reservation) : createDefaultForm(this.data.initialDate)
      const isEditable = reservation
        ? reservation.merchant_action_state?.can_edit ?? isMerchantReservationEditable(reservation.status)
        : true

      if (reservation && !isEditable) {
        loadWarningMessage = loadWarningMessage || '当前预订状态不可修改，页面仅用于查看已登记信息。'
      }

      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        loadWarningMessage,
        isEditable,
        statusLabel: reservation ? formatReservationStatus(reservation.status) : '',
        tableOptions,
        selectedTableValue,
        selectedTableLabel,
        formData
      })
    } catch (err) {
      logger.error('Load reservation edit page failed', err)
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(err, '预订编辑页加载失败，请稍后重试')
      })
    }
  },

  onRetry() {
    void this.loadPageData()
  },

  showTablePicker() {
    if (!this.data.tableOptions.length) {
      wx.showToast({ title: '暂无可选桌台', icon: 'none' })
      return
    }

    this.setData({ tablePickerVisible: true })
  },

  onTableConfirm(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null, label: string[] | null }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const labels = Array.isArray(e.detail?.label) ? e.detail.label : []
    const selectedTableValue = String(values[0] ?? this.data.selectedTableValue ?? '')
    const selectedTableLabel = String(labels[0] ?? findTableLabel(this.data.tableOptions, selectedTableValue) ?? '')
    const tableId = Number(selectedTableValue)

    if (!Number.isFinite(tableId) || tableId <= 0) {
      this.setData({ tablePickerVisible: false })
      wx.showToast({ title: '请选择桌台', icon: 'none' })
      return
    }

    this.setData({
      tablePickerVisible: false,
      selectedTableValue,
      selectedTableLabel,
      'formData.table_id': tableId
    })
  },

  onTableCancel() {
    this.setData({ tablePickerVisible: false })
  },

  onOpenDatePicker() {
    this.setData({ datePickerVisible: true })
  },

  onCloseDatePicker() {
    this.setData({ datePickerVisible: false })
  },

  onDateConfirm(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const value = String(e.detail?.value || '')
    if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) {
      this.setData({ datePickerVisible: false })
      wx.showToast({ title: '请选择正确日期', icon: 'none' })
      return
    }

    this.setData({
      datePickerVisible: false,
      'formData.date': value
    })
  },

  onOpenTimePicker() {
    this.setData({ timePickerVisible: true })
  },

  onCloseTimePicker() {
    this.setData({ timePickerVisible: false })
  },

  onTimeConfirm(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const value = normalizeReservationTime(e.detail?.value)
    this.setData({
      timePickerVisible: false,
      'formData.time': value
    })
  },

  onInputChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof ReservationEditFormData }
    if (!field) return

    this.setData({ [`formData.${field}`]: String(e.detail?.value || '') })
  },

  onGuestCountChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const value = String(e.detail?.value || '').replace(/\D+/g, '')
    this.setData({ 'formData.guest_count': value })
  },

  onSourceChange(e: WechatMiniprogram.CustomEvent<{ value?: ReservationSource }>) {
    const value = e.detail?.value
    if (!value) return

    this.setData({ 'formData.source': value })
  },

  buildCreatePayload(): MerchantCreateReservationRequest | null {
    const { formData } = this.data
    const guestCount = Number(formData.guest_count)
    const contactName = formData.contact_name.trim()
    const contactPhone = formData.contact_phone.trim()
    const notes = formData.notes.trim()

    if (!formData.table_id) {
      wx.showToast({ title: '请选择桌台', icon: 'none' })
      return null
    }

    if (!/^\d{4}-\d{2}-\d{2}$/.test(formData.date)) {
      wx.showToast({ title: '请选择到店日期', icon: 'none' })
      return null
    }

    if (!/^\d{2}:\d{2}$/.test(formData.time)) {
      wx.showToast({ title: '请选择到店时间', icon: 'none' })
      return null
    }

    if (!Number.isFinite(guestCount) || guestCount <= 0) {
      wx.showToast({ title: '请输入正确人数', icon: 'none' })
      return null
    }

    if (!contactName) {
      wx.showToast({ title: '请输入联系人姓名', icon: 'none' })
      return null
    }

    if (!/^1\d{10}$/.test(contactPhone)) {
      wx.showToast({ title: '请输入正确手机号', icon: 'none' })
      return null
    }

    return {
      table_id: formData.table_id,
      date: formData.date,
      time: formData.time,
      guest_count: guestCount,
      contact_name: contactName,
      contact_phone: contactPhone,
      source: formData.source,
      notes: notes || undefined
    }
  },

  buildUpdatePayload(): UpdateReservationRequest | null {
    const createPayload = this.buildCreatePayload()
    if (!createPayload) return null

    return {
      table_id: createPayload.table_id,
      date: createPayload.date,
      time: createPayload.time,
      guest_count: createPayload.guest_count,
      contact_name: createPayload.contact_name,
      contact_phone: createPayload.contact_phone,
      notes: createPayload.notes
    }
  },

  async onSubmit() {
    if (this.data.submitting || this.data.initialLoading || this.data.initialError) return
    if (this.data.isEdit && !this.data.isEditable) return

    const payload = this.data.isEdit ? this.buildUpdatePayload() : this.buildCreatePayload()
    if (!payload) return

    this.setData({ submitting: true })

    try {
      if (this.data.isEdit) {
        await ReservationService.updateReservation(this.data.reservationId, payload as UpdateReservationRequest)
      } else {
        await ReservationService.merchantCreateReservation(payload as MerchantCreateReservationRequest)
      }

      const pages = getCurrentPages()
      const previousPage = pages[pages.length - 2] as ReservationListPageBridge | undefined
      await previousPage?.refreshPage?.({ showLoading: false, preserveList: false })
      wx.navigateBack()
    } catch (err) {
      logger.error('Submit reservation form failed', err)
      wx.showToast({
        title: getErrorUserMessage(err, this.data.isEdit ? '保存失败，请稍后重试' : '创建失败，请稍后重试'),
        icon: 'none'
      })
    } finally {
      this.setData({ submitting: false })
    }
  }
})