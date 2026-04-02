import { getStableBarHeights } from '../../../utils/responsive'
import {
  deviceManagementService,
  CreatePrinterRequest,
  PrinterLiveStatusResponse,
  PrinterReconciliationJobStatus,
  PrinterReconciliationJobResponse,
  PrinterRole,
  PrinterResponse,
  PrinterType,
  UpdatePrinterRequest
} from '../../../api/table-device-management'
import { logger } from '../../../utils/logger'
import { settleAll } from '../../../utils/promise'
import { getErrorUserMessage } from '../../../utils/user-facing'
import dayjs from 'dayjs'

const PRINTER_TYPE_LABELS: Record<PrinterType, string> = {
  feieyun: '飞鹅云',
  yilianyun: '易联云',
  other: '其他'
}

const PRINTER_ROLE_LABELS: Record<PrinterRole, string> = {
  front: '前台',
  kitchen: '后厨'
}

interface PrinterFormData {
  printer_name: string
  printer_sn: string
  printer_key: string
  printer_type: PrinterType
  printer_role: PrinterRole
  print_takeout: boolean
  print_dine_in: boolean
  print_reservation: boolean
  is_active: boolean
}

interface PrinterLiveStatusView extends PrinterLiveStatusResponse {
  checked_at_label: string
  role_label: string
  online_label: string
  working_label: string
}

interface PrinterReconciliationJobView extends PrinterReconciliationJobResponse {
  title: string
  summary: string
  status_label: string
  status_theme: 'warning' | 'success' | 'default'
  action_label: string
  source_action_label: string
  created_at_label: string
  updated_at_label: string
  resolved_at_label: string
  failure_reason_label: string
  last_error_label: string
}

function createDefaultFormData(): PrinterFormData {
  return {
    printer_name: '',
    printer_sn: '',
    printer_key: '',
    printer_type: 'feieyun',
    printer_role: 'front',
    print_takeout: true,
    print_dine_in: true,
    print_reservation: true,
    is_active: true
  }
}

function ensureArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

function buildRefreshErrorMessage(messages: string[]) {
  const normalized = messages.filter((message) => typeof message === 'string' && message.trim())
  if (!normalized.length) return ''
  return Array.from(new Set(normalized)).join('；')
}

function buildReconciliationTitle(job: PrinterReconciliationJobResponse) {
  if (job.desired_action === 'remove') {
    return '云端删除待补偿'
  }
  if (job.desired_action === 'register') {
    return '云端注册待补偿'
  }
  return '打印机状态待对齐'
}

function buildReconciliationSummary(job: PrinterReconciliationJobResponse) {
  const printerName = job.printer_name || '该打印机'
  if (job.status === 'resolved') {
    if (job.desired_action === 'remove') {
      return `${printerName} 的云端删除状态已重新对齐。`
    }
    if (job.desired_action === 'register') {
      return `${printerName} 的云端注册状态已重新对齐。`
    }
    return `${printerName} 的云端状态已经恢复一致。`
  }
  if (job.desired_action === 'remove') {
    return `${printerName} 的云端删除未完成，建议重试以清理残留配置。`
  }
  if (job.desired_action === 'register') {
    return `${printerName} 的云端注册未完成，建议重试以恢复打印同步。`
  }
  return `${printerName} 的云端状态暂未和本地配置对齐，请稍后重试。`
}

function buildReconciliationStatusLabel(status: string) {
  if (status === 'resolved') return '已解决'
  if (status === 'pending') return '待处理'
  return '待同步'
}

function buildReconciliationStatusTheme(status: string): 'warning' | 'success' | 'default' {
  if (status === 'resolved') return 'success'
  if (status === 'pending') return 'warning'
  return 'default'
}

function buildReconciliationActionLabel(job: PrinterReconciliationJobResponse) {
  if (job.status === 'resolved') {
    return '已完成同步'
  }
  if (job.desired_action === 'remove') {
    return '重试云端删除'
  }
  if (job.desired_action === 'register') {
    return '重试云端注册'
  }
  return '重试同步'
}

function buildSourceActionLabel(sourceAction: string) {
  if (sourceAction === 'create') return '新增打印机'
  if (sourceAction === 'delete') return '删除打印机'
  return sourceAction || '未知操作'
}

function formatTimeLabel(value?: string) {
  if (!value) return ''
  return dayjs(value).format('MM-DD HH:mm')
}

function printerRoleLabel(role?: string) {
  if (!role) return '前台'
  return PRINTER_ROLE_LABELS[role as PrinterRole] || role
}

function buildPrinterLiveStatusView(status: PrinterLiveStatusResponse, role?: string): PrinterLiveStatusView {
  return {
    ...status,
    checked_at_label: formatTimeLabel(status.checked_at),
    role_label: printerRoleLabel(role),
    online_label: status.online ? '在线' : '离线',
    working_label: status.working ? '工作正常' : '待处理'
  }
}

function buildReconciliationJob(job: PrinterReconciliationJobResponse): PrinterReconciliationJobView {
  return {
    ...job,
    title: buildReconciliationTitle(job),
    summary: buildReconciliationSummary(job),
    status_label: buildReconciliationStatusLabel(job.status),
    status_theme: buildReconciliationStatusTheme(job.status),
    action_label: buildReconciliationActionLabel(job),
    source_action_label: buildSourceActionLabel(job.source_action),
    created_at_label: formatTimeLabel(job.created_at),
    updated_at_label: formatTimeLabel(job.updated_at),
    resolved_at_label: formatTimeLabel(job.resolved_at),
    failure_reason_label: job.failure_reason || '',
    last_error_label: job.last_error || ''
  }
}

function buildReconciliationEmptyMessage(status: PrinterReconciliationJobStatus) {
  return status === 'resolved' ? '暂无已解决对账任务' : '暂无待处理对账任务'
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    printersAvailable: false,
    printersError: false,
    printersErrorMessage: '',
    reconciliationAvailable: false,
    reconciliationError: false,
    reconciliationErrorMessage: '',
    currentReconciliationStatus: 'pending' as PrinterReconciliationJobStatus,
    loadedReconciliationStatus: 'pending' as PrinterReconciliationJobStatus,
    reconciliationEmptyMessage: buildReconciliationEmptyMessage('pending'),
    formSubmitting: false,
    deletingPrinterId: 0,
    testingPrinterId: 0,
    retryingReconciliationId: 0,
    statusLoading: false,
    statusPopupVisible: false,
    statusPrinterId: 0,
    statusPrinterName: '',
    liveStatus: null as PrinterLiveStatusView | null,
    printers: [] as PrinterResponse[],
    reconciliationJobs: [] as PrinterReconciliationJobView[],
    formVisible: false,
    isEdit: false,
    editingPrinterId: 0,
    formData: createDefaultFormData(),
    printerTypeOptions: [
      { label: '飞鹅云', value: 'feieyun' }
    ],
    printerRoleOptions: [
      { label: '前台打印机', value: 'front' },
      { label: '后厨打印机', value: 'kitchen' }
    ]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadPrinters()
  },

  onShow() {
    this.setData({
      printers: ensureArray(this.data.printers),
      reconciliationJobs: ensureArray(this.data.reconciliationJobs)
    })
    if (!this.data.initialLoading && !this.data.loading) {
      this.loadPrinters(false)
    }
  },

  onReconciliationTabChange(e: WechatMiniprogram.CustomEvent<{ value: PrinterReconciliationJobStatus }>) {
    const nextStatus = e.detail.value
    if (!nextStatus || nextStatus === this.data.currentReconciliationStatus) return

    this.setData({
      currentReconciliationStatus: nextStatus,
      reconciliationError: false,
      reconciliationErrorMessage: '',
      refreshErrorMessage: '',
      reconciliationEmptyMessage: buildReconciliationEmptyMessage(nextStatus)
    }, () => {
      this.loadPrinters(false)
    })
  },

  onPullDownRefresh() {
    this.loadPrinters(false)
  },

  async loadPrinters(showLoading = true) {
    if (this.data.loading) return

    const hasExistingPrinters = this.data.printersAvailable
    const hasExistingReconciliationJobs = this.data.reconciliationAvailable
      && this.data.loadedReconciliationStatus === this.data.currentReconciliationStatus
    const isSilentRefresh = !showLoading && (hasExistingPrinters || hasExistingReconciliationJobs)
    const currentReconciliationStatus = this.data.currentReconciliationStatus

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const [printersResult, reconciliationResult] = await settleAll([
        deviceManagementService.listPrinters(),
        deviceManagementService.listPrinterReconciliationJobs(currentReconciliationStatus)
      ] as const)

      const refreshMessages: string[] = []
      let hasRenderableSection = false
      let firstErrorMessage = ''
      const nextState: Record<string, unknown> = {
        initialLoading: false,
        initialError: false,
        initialErrorMessage: ''
      }

      if (printersResult.status === 'fulfilled') {
        const list = Array.isArray(printersResult.value?.printers) ? printersResult.value.printers : []
        nextState.printers = list
        nextState.printersAvailable = true
        nextState.printersError = false
        nextState.printersErrorMessage = ''
        hasRenderableSection = true
      } else {
        const message = getErrorMessage(printersResult.reason, '打印机列表加载失败，请稍后重试')
        if (hasExistingPrinters) {
          refreshMessages.push(`${message}，当前已保留打印机列表`)
          nextState.printersError = false
          nextState.printersErrorMessage = ''
          hasRenderableSection = true
        } else {
          nextState.printers = []
          nextState.printersAvailable = false
          nextState.printersError = true
          nextState.printersErrorMessage = message
          firstErrorMessage = firstErrorMessage || message
        }
      }

      if (reconciliationResult.status === 'fulfilled') {
        const jobs = Array.isArray(reconciliationResult.value?.jobs)
          ? reconciliationResult.value.jobs.map(buildReconciliationJob)
          : []
        nextState.reconciliationJobs = jobs
        nextState.reconciliationAvailable = true
        nextState.reconciliationError = false
        nextState.reconciliationErrorMessage = ''
        nextState.loadedReconciliationStatus = currentReconciliationStatus
        nextState.reconciliationEmptyMessage = buildReconciliationEmptyMessage(currentReconciliationStatus)
        hasRenderableSection = true
      } else {
        const message = getErrorMessage(reconciliationResult.reason, '对账任务加载失败，请稍后重试')
        if (hasExistingReconciliationJobs) {
          const currentLabel = currentReconciliationStatus === 'resolved' ? '已解决任务' : '待处理任务'
          refreshMessages.push(`${message}，当前已保留${currentLabel}`)
          nextState.reconciliationError = false
          nextState.reconciliationErrorMessage = ''
          hasRenderableSection = true
        } else {
          nextState.reconciliationJobs = []
          nextState.reconciliationAvailable = false
          nextState.reconciliationError = true
          nextState.reconciliationErrorMessage = message
          nextState.reconciliationEmptyMessage = buildReconciliationEmptyMessage(currentReconciliationStatus)
          firstErrorMessage = firstErrorMessage || message
        }
      }

      if (!hasRenderableSection) {
        nextState.initialError = true
        nextState.initialErrorMessage = firstErrorMessage || '打印机数据加载失败，请稍后重试'
        nextState.refreshErrorMessage = ''
      } else {
        nextState.refreshErrorMessage = buildRefreshErrorMessage(refreshMessages)
      }

      this.setData(nextState)
    } catch (err) {
      logger.error('Load printers failed', err)
      const message = getErrorMessage(err, '加载打印机失败，请稍后重试')

      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (isSilentRefresh) {
        this.setData({
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      } else {
        this.setData({
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    this.loadPrinters()
  },

  onRetryRefresh() {
    this.loadPrinters(false)
  },

  onGoPrintAnomalies() {
    wx.navigateTo({ url: '/pages/merchant/orders/print-anomalies/index' })
  },

  async onRetryReconciliationJob(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
    if (!id || this.data.retryingReconciliationId) return

    wx.showModal({
      title: '重试对账任务',
      content: `重新同步打印机「${name || '未命名打印机'}」的云端状态？`,
      confirmText: '立即重试',
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm || this.data.retryingReconciliationId) return

        this.setData({ retryingReconciliationId: id })
        try {
          await deviceManagementService.retryPrinterReconciliationJob(id)
          await this.loadPrinters(false)
          wx.showToast({
            title: this.data.currentReconciliationStatus === 'pending' ? '已重试，任务状态已刷新' : '已同步最新任务状态',
            icon: 'success'
          })
        } catch (err) {
          logger.error('Retry printer reconciliation job failed', err)
          wx.showToast({
            title: getErrorMessage(err, '重试失败，请稍后重试'),
            icon: 'none'
          })
        } finally {
          this.setData({ retryingReconciliationId: 0 })
        }
      }
    })
  },

  printerTypeLabel(type: PrinterType): string {
    return PRINTER_TYPE_LABELS[type] || type
  },

  printerRoleLabel(role?: string): string {
    return printerRoleLabel(role)
  },

  applyPrinters(printers: PrinterResponse[]) {
    this.setData({ printers: ensureArray(printers) })
  },

  patchPrinter(printer: PrinterResponse) {
    const exists = this.data.printers.some((item) => item.id === printer.id)
    this.applyPrinters(
      exists
        ? this.data.printers.map((item) => item.id === printer.id ? printer : item)
        : [printer, ...this.data.printers]
    )
  },

  removePrinter(printerId: number) {
    this.applyPrinters(this.data.printers.filter((item) => item.id !== printerId))
  },

  resetFormState() {
    this.setData({
      formVisible: false,
      isEdit: false,
      editingPrinterId: 0,
      formData: createDefaultFormData()
    })
  },

  onAddPrinter() {
    this.setData({
      formVisible: true,
      isEdit: false,
      editingPrinterId: 0,
      formData: createDefaultFormData()
    })
  },

  onPrinterClick(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    const printer = this.data.printers.find((p) => p.id === id)
    if (!printer) return
    this.setData({
      formVisible: true,
      isEdit: true,
      editingPrinterId: id,
      formData: {
        printer_name: printer.printer_name,
        printer_sn: printer.printer_sn,
        printer_key: '',
        printer_type: printer.printer_type,
        printer_role: (printer.printer_role || 'front') as PrinterRole,
        print_takeout: printer.print_takeout,
        print_dine_in: printer.print_dine_in,
        print_reservation: printer.print_reservation,
        is_active: printer.is_active
      }
    })
  },

  onCloseForm() {
    if (this.data.formSubmitting) return
    this.resetFormState()
  },

  onTextInput(e: WechatMiniprogram.Input) {
    const { field } = e.currentTarget.dataset as { field: string }
    this.setData({ [`formData.${field}`]: e.detail.value })
  },

  onTypeChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ 'formData.printer_type': e.detail.value as PrinterType })
  },

  onRoleChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ 'formData.printer_role': e.detail.value as PrinterRole })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent) {
    const { field } = e.currentTarget.dataset as { field: string }
    this.setData({ [`formData.${field}`]: e.detail.value })
  },

  async onSubmitForm() {
    const { formData, isEdit, editingPrinterId, formSubmitting } = this.data
    if (formSubmitting) return

    if (!formData.printer_name.trim()) {
      return wx.showToast({ title: '请填写打印机名称', icon: 'none' })
    }
    if (!isEdit && !formData.printer_sn.trim()) {
      return wx.showToast({ title: '请填写打印机序列号', icon: 'none' })
    }
    if (!isEdit && !formData.printer_key.trim()) {
      return wx.showToast({ title: '请填写打印机密钥', icon: 'none' })
    }

    this.setData({ formSubmitting: true })
    try {
      let savedPrinter: PrinterResponse
      if (isEdit) {
        const updateParams: UpdatePrinterRequest = {
          printer_name: formData.printer_name,
          printer_role: formData.printer_role,
          print_takeout: formData.print_takeout,
          print_dine_in: formData.print_dine_in,
          print_reservation: formData.print_reservation,
          is_active: formData.is_active
        }
        if (formData.printer_key.trim()) {
          updateParams.printer_key = formData.printer_key
        }
        savedPrinter = await deviceManagementService.updatePrinter(editingPrinterId, updateParams)
      } else {
        const createParams: CreatePrinterRequest = {
          printer_name: formData.printer_name,
          printer_sn: formData.printer_sn,
          printer_key: formData.printer_key,
          printer_type: formData.printer_type,
          printer_role: formData.printer_role,
          print_takeout: formData.print_takeout,
          print_dine_in: formData.print_dine_in,
          print_reservation: formData.print_reservation
        }
        savedPrinter = await deviceManagementService.createPrinter(createParams)
      }
      this.patchPrinter(savedPrinter)
      this.setData({ refreshErrorMessage: '' })
      this.resetFormState()
      await this.loadPrinters(false)
    } catch (err) {
      logger.error('Submit printer form failed', err)
      const msg = getErrorMessage(err, '操作失败，请稍后重试')
      wx.showToast({ title: msg, icon: 'none' })
    } finally {
      this.setData({ formSubmitting: false })
    }
  },

  onDeletePrinter(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
    if (!id) return
    wx.showModal({
      title: '确认删除',
      content: `确认删除打印机「${name || id}」吗？`,
      confirmText: '删除',
      confirmColor: '#e34d59',
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm || this.data.deletingPrinterId) return
        this.setData({ deletingPrinterId: id })
        try {
          await deviceManagementService.deletePrinter(id)
          this.removePrinter(id)
          if (this.data.editingPrinterId === id) {
            this.resetFormState()
          }
          await this.loadPrinters(false)
        } catch (err) {
          logger.error('Delete printer failed', err)
          const message = getErrorMessage(err, '删除失败，请稍后重试')
          wx.showToast({ title: message, icon: 'none' })
        } finally {
          this.setData({ deletingPrinterId: 0 })
        }
      }
    })
  },

  onTestPrinter(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
    if (!id) return
    wx.showModal({
      title: '测试打印',
      content: `向「${name || '打印机'}」发送测试打印命令？`,
      confirmText: '发送',
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm || this.data.testingPrinterId) return
        this.setData({ testingPrinterId: id })
        try {
          await deviceManagementService.testPrinter(id)
          wx.showToast({ title: '测试命令已发送', icon: 'success' })
        } catch (err) {
          logger.error('Test printer failed', err)
          const message = getErrorMessage(err, '发送失败，请稍后重试')
          wx.showToast({ title: message, icon: 'none' })
        } finally {
          this.setData({ testingPrinterId: 0 })
        }
      }
    })
  },

  onCloseStatusPopup() {
    if (this.data.statusLoading) return
    this.setData({
      statusPopupVisible: false,
      statusPrinterId: 0,
      statusPrinterName: '',
      liveStatus: null
    })
  },

  async fetchPrinterStatus(printerId: number) {
    const printer = this.data.printers.find((item) => item.id === printerId)
    if (!printer || this.data.statusLoading) return

    if (printer.printer_type !== 'feieyun') {
      wx.showToast({ title: '当前仅支持飞鹅云打印机查询实时状态', icon: 'none' })
      return
    }

    this.setData({
      statusLoading: true,
      statusPopupVisible: true,
      statusPrinterId: printerId,
      statusPrinterName: printer.printer_name,
      liveStatus: null
    })

    try {
      const status = await deviceManagementService.getPrinterLiveStatus(printerId)
      this.setData({
        liveStatus: buildPrinterLiveStatusView(status, printer.printer_role)
      })
    } catch (err) {
      logger.error('Load printer live status failed', err)
      wx.showToast({
        title: getErrorMessage(err, '实时状态加载失败，请稍后重试'),
        icon: 'none'
      })
      this.setData({ statusPopupVisible: false, statusPrinterId: 0, statusPrinterName: '' })
    } finally {
      this.setData({ statusLoading: false })
    }
  },

  async onViewPrinterStatus(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    await this.fetchPrinterStatus(id)
  },

  onRefreshPrinterStatus() {
    if (!this.data.statusPrinterId) return
    this.fetchPrinterStatus(this.data.statusPrinterId)
  },

  onActionsCatch() {}
})
