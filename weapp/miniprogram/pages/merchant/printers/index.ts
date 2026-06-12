import dayjs from '../_main_shared/miniprogram_npm/dayjs/index'
import { getStableBarHeights } from '../../../utils/responsive'
import {
  deviceManagementService,
  type PrinterLiveStatusResponse,
  type PrinterRole,
  type PrinterResponse
} from '../../../api/table-device-management'
import {
  ensureMerchantDeviceManagementAccess,
  getMerchantDeviceManagementErrorMessage,
  isMerchantDeviceManagementDenied,
  isMerchantDeviceManagementGranted
} from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { isSettledFulfilled, isSettledRejected, settleAll } from '../../../utils/promise'
import {
  buildPrinterReconciliationJobView,
  buildPrinterTypeLabel,
  buildReconciliationLoadErrorMessage,
  type PrinterReconciliationJobView
} from '../../../utils/printer-reconciliation-view'
import { getErrorUserMessage } from '../../../utils/user-facing'

const PRINTERS_AUTO_REFRESH_WINDOW_MS = 60 * 1000

const PRINTER_ROLE_LABELS: Record<PrinterRole, string> = {
  front: '前台',
  kitchen: '后厨'
}

const DEVICE_MANAGE_ROLE_LABELS: Record<string, string> = {
  owner: '老板',
  manager: '店长',
  chef: '后厨',
  cashier: '收银'
}

type ConfirmActionKind = '' | 'delete' | 'test'
interface PrinterView extends PrinterResponse {
  printer_type_label: string
  printer_role_label: string
  active_label: string
  print_takeout_label: string
  print_dine_in_label: string
  print_reservation_label: string
  created_at_label: string
  updated_at_label: string
}

interface PrinterLiveStatusView extends PrinterLiveStatusResponse {
  checked_at_label: string
  role_label: string
  online_label: string
  working_label: string
}

function ensureArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

function shouldAutoRefresh(lastLoadedAt: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= PRINTERS_AUTO_REFRESH_WINDOW_MS
}

function formatTimeLabel(value?: string) {
  if (!value) return ''
  return dayjs(value).format('MM-DD HH:mm')
}

function printerTypeLabel(type?: string) {
  return buildPrinterTypeLabel(type)
}

function printerRoleLabel(role?: string) {
  if (!role) return '前台'
  return PRINTER_ROLE_LABELS[role as PrinterRole] || role
}

function buildPrinterView(printer: PrinterResponse): PrinterView {
  return {
    ...printer,
    printer_type_label: printerTypeLabel(printer.printer_type),
    printer_role_label: printerRoleLabel(printer.printer_role),
    active_label: printer.is_active ? '启用中' : '已停用',
    print_takeout_label: printer.print_takeout ? '已开启' : '未开启',
    print_dine_in_label: printer.print_dine_in ? '已开启' : '未开启',
    print_reservation_label: printer.print_reservation ? '已开启' : '未开启',
    created_at_label: formatTimeLabel(printer.created_at),
    updated_at_label: formatTimeLabel(printer.updated_at)
  }
}

function deviceManageRoleLabel(role?: string) {
  if (!role) return '--'
  return DEVICE_MANAGE_ROLE_LABELS[role] || role
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

function buildRefreshErrorMessage(messages: string[]) {
  const normalized = messages.filter((message) => typeof message === 'string' && message.trim())
  if (!normalized.length) return ''
  return Array.from(new Set(normalized)).join('；')
}

function buildPrinterResultSummary(count: number) {
  return `当前共 ${count} 台设备`
}

function resolvePopupVisible(detail: unknown) {
  if (typeof detail === 'boolean') {
    return detail
  }

  if (detail && typeof detail === 'object' && 'visible' in (detail as Record<string, unknown>)) {
    return Boolean((detail as { visible?: boolean }).visible)
  }

  return false
}

function resolveConfirmDialog(action: ConfirmActionKind, targetName: string) {
  switch (action) {
    case 'delete':
      return {
        title: '确认删除打印机',
        content: `删除后会停止该设备的后续打印分发，确定删除“${targetName || '该打印机'}”吗？`,
        confirmText: '确认删除',
        confirmTheme: 'danger'
      }
    case 'test':
      return {
        title: '发送测试打印',
        content: `确认向“${targetName || '该打印机'}”发送测试打印命令吗？`,
        confirmText: '发送测试',
        confirmTheme: 'primary'
      }
    default:
      return {
        title: '',
        content: '',
        confirmText: '确认',
        confirmTheme: 'primary'
      }
  }
}

const getErrorMessage = getErrorUserMessage
let printerStatusRequestToken = 0

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    accessBlockTitle: '当前身份暂不支持管理设备',
    accessBlockDescription: '',
    accessRoleLabel: '--',
    merchantName: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    hasLoadedOnce: false,
    loading: false,
    refreshErrorMessage: '',
    commandResultText: '',
    commandResultNote: '',
    commandResultPrinterId: 0,
    resultSummaryText: '当前共 0 台设备',
    lastLoadedAt: 0,
    pageDirty: false,
    needsReloadOnShow: false,
    printers: [] as PrinterView[],
    reconciliationJobs: [] as PrinterReconciliationJobView[],
    reconciliationErrorMessage: '',
    reconciliationLoading: false,
    retryingReconciliationJobId: 0,
    deletingPrinterId: 0,
    testingPrinterId: 0,
    confirmDialogVisible: false,
    confirmDialogTitle: '',
    confirmDialogContent: '',
    confirmDialogConfirmText: '确认',
    confirmDialogConfirmTheme: 'primary',
    confirmDialogSubmitting: false,
    confirmDialogAction: '' as ConfirmActionKind,
    confirmTargetId: 0,
    confirmTargetName: '',
    statusPopupVisible: false,
    statusLoading: false,
    statusErrorMessage: '',
    statusPrinterId: 0,
    statusPrinterName: '',
    liveStatus: null as PrinterLiveStatusView | null
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage(true)
  },

  async onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage || this.data.initialLoading) {
      return
    }

    if (!this.data.pageDirty && !this.data.needsReloadOnShow && !shouldAutoRefresh(this.data.lastLoadedAt)) {
      return
    }

    this.setData({ needsReloadOnShow: false })
    await this.loadPageData(false, true)
  },

  async onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }

    await this.loadPageData(false, true)
  },

  async bootstrapPage(forceAccess = false) {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      accessBlockTitle: '当前身份暂不支持管理设备',
      accessBlockDescription: '',
      accessRoleLabel: '--',
      merchantName: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: forceAccess ? '' : this.data.refreshErrorMessage
    })

    const accessResult = await ensureMerchantDeviceManagementAccess({ force: forceAccess })
    if (!isMerchantDeviceManagementGranted(accessResult)) {
      const deniedResult = isMerchantDeviceManagementDenied(accessResult) ? accessResult : null
      const capability = deniedResult?.capability
      this.setData({
        accessReady: true,
        accessDenied: Boolean(deniedResult),
        accessErrorMessage: getMerchantDeviceManagementErrorMessage(accessResult),
        accessRoleLabel: deviceManageRoleLabel(capability?.staff_role),
        merchantName: capability?.merchant_name || '',
        accessBlockDescription: deniedResult ? deniedResult.message : '',
        initialLoading: false
      })
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      accessReady: true,
      accessDenied: false,
      accessErrorMessage: '',
      accessRoleLabel: deviceManageRoleLabel(accessResult.capability.staff_role),
      merchantName: accessResult.capability.merchant_name || '',
      accessBlockDescription: ''
    })

    await this.loadPageData(true, true)
  },

  async loadPageData(showLoading = true, force = false) {
    if (this.data.loading) return
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }

    const hasTrustedData = this.data.hasLoadedOnce

    if (!force && hasTrustedData && !this.data.pageDirty && !shouldAutoRefresh(this.data.lastLoadedAt)) {
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      loading: true,
      ...(showLoading && !hasTrustedData
        ? {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: '',
            reconciliationLoading: true,
            reconciliationErrorMessage: ''
          }
        : {
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: '',
            reconciliationLoading: true,
            reconciliationErrorMessage: ''
          })
    })

    try {
      const [printersResult, reconciliationResult] = await settleAll([
        deviceManagementService.listPrinters(),
        deviceManagementService.listPrinterReconciliationJobs('pending')
      ] as const)

      const reconciliationState: Record<string, unknown> = {
        reconciliationLoading: false
      }
      if (isSettledFulfilled(reconciliationResult)) {
        reconciliationState.reconciliationJobs = ensureArray(reconciliationResult.value?.jobs)
          .map((job) => buildPrinterReconciliationJobView(job, formatTimeLabel))
        reconciliationState.reconciliationErrorMessage = ''
      } else {
        logger.error('Load printer reconciliation jobs failed', reconciliationResult.reason)
        reconciliationState.reconciliationErrorMessage = buildReconciliationLoadErrorMessage(
          getErrorMessage(reconciliationResult.reason, '设备恢复状态加载失败，请稍后重试'),
          this.data.reconciliationJobs.length > 0
        )
      }

      if (isSettledRejected(printersResult)) {
        this.setData(reconciliationState)
        throw printersResult.reason
      }

      const printers = ensureArray(printersResult.value?.printers).map(buildPrinterView)
      this.setData({
        printers,
        resultSummaryText: buildPrinterResultSummary(printers.length),
        hasLoadedOnce: true,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now(),
        pageDirty: false,
        ...reconciliationState
      })
    } catch (err) {
      logger.error('Load printers failed', err)
      const message = getErrorMessage(err, '设备列表加载失败，请稍后重试')
      if (!hasTrustedData) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else {
        this.setData({
          refreshErrorMessage: buildRefreshErrorMessage([`${message}，当前已保留上次同步结果`])
        })
      }
    } finally {
      this.setData({ loading: false, initialLoading: false, reconciliationLoading: false })
      wx.stopPullDownRefresh()
    }
  },

  async loadReconciliationJobs(showLoading = true) {
    if (this.data.reconciliationLoading) return
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return

    this.setData({
      reconciliationLoading: showLoading,
      reconciliationErrorMessage: ''
    })

    try {
      const response = await deviceManagementService.listPrinterReconciliationJobs('pending')
      this.setData({
        reconciliationJobs: ensureArray(response?.jobs)
          .map((job) => buildPrinterReconciliationJobView(job, formatTimeLabel)),
        reconciliationErrorMessage: ''
      })
    } catch (err) {
      logger.error('Load printer reconciliation jobs failed', err)
      this.setData({
        reconciliationErrorMessage: buildReconciliationLoadErrorMessage(
          getErrorMessage(err, '设备恢复状态加载失败，请稍后重试'),
          this.data.reconciliationJobs.length > 0
        )
      })
    } finally {
      this.setData({ reconciliationLoading: false })
    }
  },

  onRetryAccess() {
    void this.bootstrapPage(true)
  },

  onRetry() {
    void this.loadPageData(true, true)
  },

  onRefreshReconciliationJobs() {
    void this.loadReconciliationJobs(true)
  },

  onCreatePrinter() {
    if (this.data.initialLoading || this.data.initialError) return
    this.setData({ needsReloadOnShow: true })
    wx.navigateTo({ url: '/pages/merchant/printers/edit/index' })
  },

  onOpenEditPrinter(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    this.setData({ needsReloadOnShow: true })
    wx.navigateTo({ url: `/pages/merchant/printers/edit/index?id=${id}` })
  },

  async handleMutationFailure(err: unknown, fallbackMessage: string) {
    await this.loadPageData(false, true)
    wx.showToast({ title: getErrorMessage(err, fallbackMessage), icon: 'none' })
  },

  async onRetryReconciliationJob(e: WechatMiniprogram.TouchEvent | WechatMiniprogram.CustomEvent) {
    const detail = (e as WechatMiniprogram.CustomEvent).detail as { id?: number } | undefined
    const dataset = e.currentTarget.dataset as { id?: number }
    const jobId = Number(detail?.id || dataset.id || 0)
    if (!jobId || this.data.retryingReconciliationJobId) return

    this.setData({
      retryingReconciliationJobId: jobId,
      reconciliationErrorMessage: ''
    })

    try {
      await deviceManagementService.retryPrinterReconciliationJob(jobId)
      await this.loadPageData(false, true)
      const hasRefreshWarning = Boolean(this.data.refreshErrorMessage || this.data.reconciliationErrorMessage)
      this.setData({
        commandResultText: '设备同步恢复已完成',
        commandResultNote: hasRefreshWarning
          ? '恢复已完成，最新状态同步失败，稍后重新进入查看'
          : '设备列表和异常恢复状态已按后端结果回读',
        commandResultPrinterId: 0
      })
    } catch (err) {
      logger.error('Retry printer reconciliation job failed', err)
      const message = getErrorMessage(err, '设备同步恢复失败，请稍后重试')
      this.setData({ reconciliationErrorMessage: message })
    } finally {
      this.setData({ retryingReconciliationJobId: 0 })
    }
  },

  openConfirmDialog(action: ConfirmActionKind, targetId: number, targetName: string) {
    const dialog = resolveConfirmDialog(action, targetName)
    this.setData({
      confirmDialogVisible: true,
      confirmDialogTitle: dialog.title,
      confirmDialogContent: dialog.content,
      confirmDialogConfirmText: dialog.confirmText,
      confirmDialogConfirmTheme: dialog.confirmTheme,
      confirmDialogSubmitting: false,
      confirmDialogAction: action,
      confirmTargetId: targetId,
      confirmTargetName: targetName
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
      confirmTargetId: 0,
      confirmTargetName: ''
    })
  },

  onCancelConfirmDialog() {
    if (this.data.confirmDialogSubmitting) return
    this.resetConfirmDialogState()
  },

  onRequestDeletePrinter(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
    if (!id || this.data.deletingPrinterId) return
    this.openConfirmDialog('delete', id, name || '该打印机')
  },

  onRequestTestPrinter(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
    if (!id || this.data.testingPrinterId) return
    this.openConfirmDialog('test', id, name || '该打印机')
  },

  async onConfirmDialogAction() {
    const targetId = Number(this.data.confirmTargetId || 0)
    if (!targetId || !this.data.confirmDialogAction) {
      this.onCancelConfirmDialog()
      return
    }

    this.setData({ confirmDialogSubmitting: true })

    try {
      if (this.data.confirmDialogAction === 'delete') {
        this.setData({ deletingPrinterId: targetId })
        await deviceManagementService.deletePrinter(targetId)
        this.setData({ pageDirty: true })
        this.resetConfirmDialogState()
        await this.loadPageData(false, true)
        this.setData({
          commandResultText: '打印机删除已提交',
          commandResultNote: this.data.refreshErrorMessage
            ? '删除已提交，设备列表同步失败，稍后重新进入查看'
            : '设备列表已按后端结果回读',
          commandResultPrinterId: 0
        })
      } else if (this.data.confirmDialogAction === 'test') {
        this.setData({ testingPrinterId: targetId })
        await deviceManagementService.testPrinter(targetId)
        this.resetConfirmDialogState()
        this.setData({
          commandResultText: '测试打印命令已提交',
          commandResultNote: '打印是否完成以设备实时状态为准',
          commandResultPrinterId: targetId
        })
      }
    } catch (err) {
      logger.error('Handle printer confirm action failed', err)
      if (this.data.confirmDialogAction === 'delete') {
        await this.handleMutationFailure(
          err,
          '删除失败，请稍后重试'
        )
      } else {
        wx.showToast({ title: getErrorMessage(err, '测试打印失败，请稍后重试'), icon: 'none' })
      }
      this.setData({ confirmDialogSubmitting: false })
    } finally {
      this.setData({
        deletingPrinterId: 0,
        testingPrinterId: 0,
        confirmDialogSubmitting: false
      })
    }
  },

  onStatusPopupVisibleChange(e: WechatMiniprogram.CustomEvent) {
    if (resolvePopupVisible(e.detail) || this.data.statusLoading) {
      return
    }

    this.closeStatusPopup()
  },

  closeStatusPopup() {
    printerStatusRequestToken += 1
    this.setData({
      statusPopupVisible: false,
      statusLoading: false,
      statusErrorMessage: '',
      statusPrinterId: 0,
      statusPrinterName: '',
      liveStatus: null
    })
  },

  async fetchPrinterStatus(printerId: number) {
    const printer = this.data.printers.find((item) => item.id === printerId)
    if (!printer || this.data.statusLoading) return
    const requestToken = ++printerStatusRequestToken

    this.setData({
      statusPopupVisible: true,
      statusLoading: true,
      statusErrorMessage: '',
      statusPrinterId: printerId,
      statusPrinterName: printer.printer_name,
      liveStatus: null
    })

    try {
      const status = await deviceManagementService.getPrinterLiveStatus(printerId)
      if (
        requestToken !== printerStatusRequestToken ||
        !this.data.statusPopupVisible ||
        this.data.statusPrinterId !== printerId
      ) {
        return
      }
      this.setData({
        liveStatus: buildPrinterLiveStatusView(status, printer.printer_role),
        statusErrorMessage: ''
      })
    } catch (err) {
      logger.error('Load printer live status failed', err)
      if (
        requestToken !== printerStatusRequestToken ||
        !this.data.statusPopupVisible ||
        this.data.statusPrinterId !== printerId
      ) {
        return
      }
      this.setData({
        statusErrorMessage: getErrorMessage(err, '实时状态加载失败，请稍后重试'),
        liveStatus: null
      })
    } finally {
      if (requestToken === printerStatusRequestToken && this.data.statusPrinterId === printerId) {
        this.setData({ statusLoading: false })
      }
    }
  },

  onViewPrinterStatus(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    void this.fetchPrinterStatus(id)
  },

  onRefreshPrinterStatus() {
    if (!this.data.statusPrinterId) return
    void this.fetchPrinterStatus(this.data.statusPrinterId)
  },

  onActionsCatch() {}
})
