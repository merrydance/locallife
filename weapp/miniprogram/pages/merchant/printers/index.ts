import { getStableBarHeights } from '../../../utils/responsive'
import { deviceManagementService, CreatePrinterRequest, PrinterResponse, PrinterType, UpdatePrinterRequest } from '../../../api/table-device-management'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'

const PRINTER_TYPE_LABELS: Record<PrinterType, string> = {
  feieyun: '飞鹅云',
  yilianyun: '易联云',
  other: '其他'
}

interface PrinterFormData {
  printer_name: string
  printer_sn: string
  printer_key: string
  printer_type: PrinterType
  print_takeout: boolean
  print_dine_in: boolean
  print_reservation: boolean
  is_active: boolean
}

function createDefaultFormData(): PrinterFormData {
  return {
    printer_name: '',
    printer_sn: '',
    printer_key: '',
    printer_type: 'feieyun',
    print_takeout: true,
    print_dine_in: true,
    print_reservation: true,
    is_active: true
  }
}

function ensureArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
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
    formSubmitting: false,
    deletingPrinterId: 0,
    testingPrinterId: 0,
    printers: [] as PrinterResponse[],
    formVisible: false,
    isEdit: false,
    editingPrinterId: 0,
    formData: createDefaultFormData(),
    printerTypeOptions: [
      { label: '飞鹅云', value: 'feieyun' },
      { label: '易联云', value: 'yilianyun' },
      { label: '其他', value: 'other' }
    ]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadPrinters()
  },

  onShow() {
    this.setData({ printers: ensureArray(this.data.printers) })
    if (!this.data.initialLoading && !this.data.loading && this.data.printers.length > 0) {
      this.loadPrinters(false)
    }
  },

  onPullDownRefresh() {
    this.loadPrinters(false)
  },

  async loadPrinters(showLoading = true) {
    if (this.data.loading) return

    const hasExistingPrinters = this.data.printers.length > 0
    const isSilentRefresh = !showLoading && hasExistingPrinters

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const res = await deviceManagementService.listPrinters()
      const list = Array.isArray(res?.printers) ? res.printers : []
      this.setData({
        printers: list,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
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

  printerTypeLabel(type: PrinterType): string {
    return PRINTER_TYPE_LABELS[type] || type
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

  onActionsCatch() {}
})
