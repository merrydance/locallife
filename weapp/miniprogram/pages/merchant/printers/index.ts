import { getStableBarHeights } from '../../../utils/responsive'
import {
  deviceManagementService,
  CreatePrinterRequest,
  PrinterLiveStatusResponse,
  PrinterReconciliationJobResponse,
  PrinterRole,
  PrinterResponse,
  PrinterType,
  UpdatePrinterRequest
} from '../../../api/table-device-management'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'
import dayjs from 'dayjs'

const PRINTERS_AUTO_REFRESH_WINDOW_MS = 60 * 1000

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

type PrinterReconciliationStatusTheme = 'success' | 'warning' | 'default'

interface PrinterReconciliationJobView extends PrinterReconciliationJobResponse {
  title: string
  summary: string
  status_label: string
  status_theme: PrinterReconciliationStatusTheme
  desired_action_label: string
  source_action_label: string
  updated_at_label: string
  retry_hint: string
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

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
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

function getReconciliationDesiredActionLabel(action: string) {
  if (action === 'register') return '补注册云端设备'
  if (action === 'remove') return '补移除云端设备'
  return '补做设备同步'
}

function getReconciliationSourceActionLabel(action: string) {
  if (action === 'create') return '添加打印机'
  if (action === 'delete') return '删除打印机'
  return '设备变更'
}

function getReconciliationStatusLabel(status: string) {
  if (status === 'resolved') return '已恢复'
  if (status === 'pending') return '待恢复'
  return '同步中'
}

function getReconciliationStatusTheme(status: string): PrinterReconciliationStatusTheme {
  if (status === 'resolved') return 'success'
  if (status === 'pending') return 'warning'
  return 'default'
}

function getReconciliationJobSummary(job: PrinterReconciliationJobResponse) {
  const sourceActionLabel = getReconciliationSourceActionLabel(job.source_action)
  const desiredActionLabel = getReconciliationDesiredActionLabel(job.desired_action)

  if (job.status === 'resolved') {
    return `设备恢复已完成，本次${sourceActionLabel}对应的${desiredActionLabel}已经同步到云端。`
  }

  if (job.retry_count > 0) {
    return `此前${sourceActionLabel}后的云端同步未完成，请重试${desiredActionLabel}以恢复设备一致性。`
  }

  return `检测到${sourceActionLabel}后的云端同步未完成，请执行一次${desiredActionLabel}。`
}

function buildPrinterReconciliationJobView(job: PrinterReconciliationJobResponse): PrinterReconciliationJobView {
  const desiredActionLabel = getReconciliationDesiredActionLabel(job.desired_action)
  return {
    ...job,
    title: `${job.printer_name || '打印机'} · ${desiredActionLabel}`,
    summary: getReconciliationJobSummary(job),
    status_label: getReconciliationStatusLabel(job.status),
    status_theme: getReconciliationStatusTheme(job.status),
    desired_action_label: desiredActionLabel,
    source_action_label: getReconciliationSourceActionLabel(job.source_action),
    updated_at_label: formatTimeLabel(job.updated_at),
    retry_hint: job.retry_count > 0 ? `已尝试 ${job.retry_count} 次` : '等待首次恢复'
  }
}

function settlePromises(tasks: Array<Promise<unknown>>) {
  return Promise.all(tasks.map((task) => task.catch(() => undefined)))
}

const getErrorMessage = getErrorUserMessage

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
    loading: false,
    lastLoadedAt: 0,
    printersAvailable: false,
    printersError: false,
    printersErrorMessage: '',
    reconciliationJobsLoading: false,
    reconciliationJobsLoaded: false,
    reconciliationJobsErrorMessage: '',
    reconciliationLastLoadedAt: 0,
    retryingReconciliationJobId: 0,
    reconciliationJobs: [] as PrinterReconciliationJobView[],
    formSubmitting: false,
    deletingPrinterId: 0,
    testingPrinterId: 0,
    statusLoading: false,
    statusPopupVisible: false,
    statusPrinterId: 0,
    statusPrinterName: '',
    liveStatus: null as PrinterLiveStatusView | null,
    printers: [] as PrinterResponse[],
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

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })
    if (accessResult.status !== 'granted') {
      this.setData({ initialLoading: false })
      return
    }

    this.loadPrinters(true, true)
    void this.loadReconciliationJobs(true)
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return

    this.setData({
      printers: ensureArray(this.data.printers)
    })
    if (!this.data.initialLoading && !this.data.loading && shouldAutoRefresh(this.data.lastLoadedAt, PRINTERS_AUTO_REFRESH_WINDOW_MS)) {
      this.loadPrinters(false)
    }
    if (!this.data.reconciliationJobsLoading && shouldAutoRefresh(this.data.reconciliationLastLoadedAt, PRINTERS_AUTO_REFRESH_WINDOW_MS)) {
      void this.loadReconciliationJobs()
    }
  },

  async onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    await settlePromises([
      this.loadPrinters(false, true),
      this.loadReconciliationJobs(true)
    ])
  },

  onRetryAccess() {
    this.setData({ accessReady: false, accessDenied: false, accessErrorMessage: '', initialLoading: true })
    this.onLoad()
  },

  async loadPrinters(showLoading = true, force = false) {
    if (this.data.loading) return

    const hasExistingPrinters = this.data.printersAvailable
    const isSilentRefresh = !showLoading && hasExistingPrinters

    if (!force && hasExistingPrinters && !shouldAutoRefresh(this.data.lastLoadedAt, PRINTERS_AUTO_REFRESH_WINDOW_MS)) {
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const response = await deviceManagementService.listPrinters()
      const list = Array.isArray(response?.printers) ? response.printers : []
      this.setData({
        printers: list,
        printersAvailable: true,
        printersError: false,
        printersErrorMessage: '',
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err) {
      logger.error('Load printers failed', err)
      const message = getErrorMessage(err, '打印机列表加载失败，请稍后重试')

      if (this.data.initialLoading || !hasExistingPrinters) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          printers: [],
          printersAvailable: false,
          printersError: true,
          printersErrorMessage: message,
          refreshErrorMessage: ''
        })
      } else if (isSilentRefresh || hasExistingPrinters) {
        this.setData({
          refreshErrorMessage: buildRefreshErrorMessage([`${message}，当前已保留打印机列表`])
        })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  async loadReconciliationJobs(force = false) {
    if (this.data.reconciliationJobsLoading) return

    const hasLoadedJobs = this.data.reconciliationJobsLoaded
    const hasExistingJobs = this.data.reconciliationJobs.length > 0

    if (!force && hasLoadedJobs && !shouldAutoRefresh(this.data.reconciliationLastLoadedAt, PRINTERS_AUTO_REFRESH_WINDOW_MS)) {
      return
    }

    this.setData({
      reconciliationJobsLoading: true,
      reconciliationJobsErrorMessage: hasExistingJobs ? this.data.reconciliationJobsErrorMessage : ''
    })

    try {
      const response = await deviceManagementService.listPrinterReconciliationJobs('pending')
      const jobs = Array.isArray(response?.jobs)
        ? response.jobs.map(buildPrinterReconciliationJobView)
        : []
      this.setData({
        reconciliationJobs: jobs,
        reconciliationJobsLoaded: true,
        reconciliationJobsErrorMessage: '',
        reconciliationLastLoadedAt: Date.now()
      })
    } catch (err) {
      logger.error('Load printer reconciliation jobs failed', err)
      const message = getErrorMessage(err, '设备同步恢复任务加载失败，请稍后重试')
      this.setData({
        reconciliationJobsLoaded: true,
        reconciliationJobsErrorMessage: hasExistingJobs ? `${message}，当前已保留上次结果` : message
      })
    } finally {
      this.setData({ reconciliationJobsLoading: false })
    }
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadPrinters(true, true)
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadPrinters(false, true)
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
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (this.data.initialLoading || this.data.initialError) return

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
      await this.loadPrinters(false, true)
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
          await this.loadPrinters(false, true)
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

  onRetryReconciliationJobs() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    void this.loadReconciliationJobs(true)
  },

  onRetryReconciliationJob(e: WechatMiniprogram.TouchEvent) {
    const { id, title } = e.currentTarget.dataset as { id?: number, title?: string }
    if (!id || this.data.retryingReconciliationJobId) return

    wx.showModal({
      title: '设备同步恢复',
      content: `确认重试「${title || '恢复任务'}」吗？`,
      confirmText: '重试同步',
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm || this.data.retryingReconciliationJobId) return

        this.setData({ retryingReconciliationJobId: id })
        try {
          const result = await deviceManagementService.retryPrinterReconciliationJob(id)
          wx.showToast({
            title: result.status === 'resolved' ? '设备同步已恢复' : '已发起同步恢复',
            icon: 'success'
          })
          await settlePromises([
            this.loadReconciliationJobs(true),
            this.loadPrinters(false, true)
          ])
        } catch (err) {
          logger.error('Retry printer reconciliation job failed', err)
          wx.showToast({
            title: getErrorMessage(err, '同步恢复失败，请稍后重试'),
            icon: 'none'
          })
        } finally {
          this.setData({ retryingReconciliationJobId: 0 })
        }
      }
    })
  },

  onActionsCatch() {}
})
