import { getStableBarHeights } from '../../../utils/responsive'
import { deviceManagementService, CreatePrinterRequest, PrinterResponse, PrinterType, UpdatePrinterRequest } from '../../../api/table-device-management'
import { logger } from '../../../utils/logger'

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

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    formSubmitting: false,
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
  },

  onPullDownRefresh() {
    this.loadPrinters()
  },

  async loadPrinters() {
    if (this.data.loading) return
    this.setData({ loading: true })
    try {
      const res = await deviceManagementService.listPrinters()
      const list = Array.isArray(res?.printers) ? res.printers : []
      this.setData({ printers: list })
    } catch (err) {
      logger.error('Load printers failed', err)
      wx.showToast({ title: '加载打印机失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  printerTypeLabel(type: PrinterType): string {
    return PRINTER_TYPE_LABELS[type] || type
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
    this.setData({ formVisible: false })
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
        await deviceManagementService.updatePrinter(editingPrinterId, updateParams)
        wx.showToast({ title: '更新成功', icon: 'success' })
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
        await deviceManagementService.createPrinter(createParams)
        wx.showToast({ title: '添加成功', icon: 'success' })
      }
      this.setData({ formVisible: false })
      this.loadPrinters()
    } catch (err) {
      logger.error('Submit printer form failed', err)
      const msg = err instanceof Error ? err.message : '操作失败'
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
        if (!res.confirm) return
        try {
          await deviceManagementService.deletePrinter(id)
          wx.showToast({ title: '已删除', icon: 'success' })
          this.loadPrinters()
        } catch (err) {
          logger.error('Delete printer failed', err)
          wx.showToast({ title: '删除失败', icon: 'none' })
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
        if (!res.confirm) return
        try {
          await deviceManagementService.testPrinter(id)
          wx.showToast({ title: '测试命令已发送', icon: 'success' })
        } catch (err) {
          logger.error('Test printer failed', err)
          wx.showToast({ title: '发送失败', icon: 'none' })
        }
      }
    })
  }
})
