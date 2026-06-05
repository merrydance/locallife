import { getStableBarHeights } from '../../../../utils/responsive'
import {
  deviceManagementService,
  type AuthorizeScannedYilianyunPrinterRequest,
  type CreatePrinterRequest,
  type PrinterResponse,
  type PrinterRole,
  type PrinterType,
  type UpdatePrinterRequest
} from '../../../../api/table-device-management'
import {
  ensureMerchantDeviceManagementAccess,
  getMerchantDeviceManagementErrorMessage,
  isMerchantDeviceManagementDenied,
  isMerchantDeviceManagementGranted
} from '../../../../utils/console-access'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface PrinterEditOptions {
  id?: string
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

interface PrinterOption<T extends string> {
  label: string
  value: T
  desc: string
}

type DirectCreatePrinterType = 'feieyun' | 'shangpeng'

const PRINTER_TYPE_OPTIONS: Array<PrinterOption<PrinterType>> = [
  {
    label: '飞鹅云',
    value: 'feieyun',
    desc: '输入设备序列号和密钥，系统会远程添加到飞鹅云应用。'
  },
  {
    label: '商鹏云',
    value: 'shangpeng',
    desc: '输入设备序列号和密钥，系统会远程添加到商鹏云应用。'
  },
  {
    label: '易联云',
    value: 'yilianyun',
    desc: '输入机器码和终端密钥，授权成功后自动创建打印机。'
  }
]

const PRINTER_ROLE_OPTIONS: Array<PrinterOption<PrinterRole>> = [
  {
    label: '前台打印机',
    value: 'front',
    desc: '适合收银、外卖打单和前台统一出单。'
  },
  {
    label: '后厨打印机',
    value: 'kitchen',
    desc: '适合后厨备餐、分菜和出餐协同。'
  }
]

const getErrorMessage = getErrorUserMessage

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

function buildFormData(printer: PrinterResponse): PrinterFormData {
  return {
    printer_name: printer.printer_name,
    printer_sn: printer.printer_sn,
    printer_key: '',
    printer_type: printer.printer_type,
    printer_role: printer.printer_role || 'front',
    print_takeout: printer.print_takeout,
    print_dine_in: printer.print_dine_in,
    print_reservation: printer.print_reservation,
    is_active: printer.is_active
  }
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    accessDeniedMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    submitting: false,
    isEdit: false,
    printerId: 0,
    formData: createDefaultFormData() as PrinterFormData,
    printerTypeOptions: PRINTER_TYPE_OPTIONS,
    printerRoleOptions: PRINTER_ROLE_OPTIONS
  },

  async onLoad(options: PrinterEditOptions) {
    const { navBarHeight } = getStableBarHeights()
    const printerId = Number(options.id || 0)
    this.setData({
      navBarHeight,
      isEdit: printerId > 0,
      printerId
    })

    await this.bootstrapPage()
  },

  async bootstrapPage() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      accessDeniedMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: ''
    })

    const accessResult = await ensureMerchantDeviceManagementAccess({ force: true })
    if (!isMerchantDeviceManagementGranted(accessResult)) {
      this.setData({
        accessReady: true,
        accessDenied: isMerchantDeviceManagementDenied(accessResult),
        accessDeniedMessage: isMerchantDeviceManagementDenied(accessResult) ? accessResult.message : '',
        accessErrorMessage: getMerchantDeviceManagementErrorMessage(accessResult),
        initialLoading: false
      })
      return
    }

    this.setData({
      accessReady: true,
      accessDenied: false,
      accessDeniedMessage: '',
      accessErrorMessage: ''
    })

    if (this.data.isEdit) {
      await this.loadPrinterDetail()
      return
    }

    this.setData({
      initialLoading: false,
      initialError: false,
      initialErrorMessage: '',
      formData: createDefaultFormData()
    })
  },

  async loadPrinterDetail() {
    this.setData({
      initialLoading: true,
      initialError: false,
      initialErrorMessage: ''
    })

    try {
      const printer = await deviceManagementService.getPrinterDetail(this.data.printerId)
      this.setData({
        formData: buildFormData(printer),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: ''
      })
    } catch (err) {
      logger.error('Load printer detail failed', err)
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorMessage(err, '打印机详情加载失败，请稍后重试')
      })
    }
  },

  onRetryAccess() {
    void this.bootstrapPage()
  },

  onRetry() {
    if (this.data.isEdit) {
      void this.loadPrinterDetail()
      return
    }

    void this.bootstrapPage()
  },

  onFormInputChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof PrinterFormData }
    if (!field) return
    this.setData({ [`formData.${field}`]: e.detail.value || '' })
  },

  onPrinterTypeChange(e: WechatMiniprogram.CustomEvent<{ value: PrinterType }>) {
    const printerType = e.detail.value
    this.setData({
      'formData.printer_type': printerType,
      'formData.printer_sn': '',
      'formData.printer_key': ''
    })
  },

  onPrinterRoleChange(e: WechatMiniprogram.CustomEvent<{ value: PrinterRole }>) {
    this.setData({ 'formData.printer_role': e.detail.value })
  },

  onFormSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof PrinterFormData }
    if (!field) return
    this.setData({ [`formData.${field}`]: Boolean(e.detail.value) })
  },

  async onSubmit() {
    if (this.data.submitting || this.data.initialLoading || this.data.initialError) return

    const { formData, isEdit, printerId } = this.data
    if (!formData.printer_name.trim()) {
      wx.showToast({ title: '请填写打印机名称', icon: 'none' })
      return
    }
    const isYilianyunAuthorization = !isEdit && formData.printer_type === 'yilianyun'
    if (!isEdit && !formData.printer_sn.trim()) {
      wx.showToast({ title: isYilianyunAuthorization ? '请填写机器码' : '请填写打印机序列号', icon: 'none' })
      return
    }
    if (!isEdit && !formData.printer_key.trim()) {
      wx.showToast({ title: isYilianyunAuthorization ? '请填写终端密钥' : '请填写打印机密钥', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: isYilianyunAuthorization ? '授权中...' : '保存中...' })

    try {
      if (isEdit) {
        const updateParams: UpdatePrinterRequest = {
          printer_name: formData.printer_name.trim(),
          printer_role: formData.printer_role,
          print_takeout: formData.print_takeout,
          print_dine_in: formData.print_dine_in,
          print_reservation: formData.print_reservation,
          is_active: formData.is_active
        }
        if (formData.printer_key.trim()) {
          updateParams.printer_key = formData.printer_key.trim()
        }
        await deviceManagementService.updatePrinter(printerId, updateParams)
      } else if (isYilianyunAuthorization) {
        const authorizeParams: AuthorizeScannedYilianyunPrinterRequest = {
          machine_code: formData.printer_sn.trim(),
          printer_name: formData.printer_name.trim(),
          printer_role: formData.printer_role,
          print_takeout: formData.print_takeout,
          print_dine_in: formData.print_dine_in,
          print_reservation: formData.print_reservation,
          msign: formData.printer_key.trim()
        }
        await deviceManagementService.authorizeScannedYilianyunPrinter(authorizeParams)
      } else {
        if (formData.printer_type !== 'feieyun' && formData.printer_type !== 'shangpeng') {
          wx.showToast({ title: '当前设备类型需要通过授权绑定', icon: 'none' })
          return
        }
        const createParams: CreatePrinterRequest = {
          printer_name: formData.printer_name.trim(),
          printer_sn: formData.printer_sn.trim(),
          printer_key: formData.printer_key.trim(),
          printer_type: formData.printer_type as DirectCreatePrinterType,
          printer_role: formData.printer_role,
          print_takeout: formData.print_takeout,
          print_dine_in: formData.print_dine_in,
          print_reservation: formData.print_reservation
        }
        await deviceManagementService.createPrinter(createParams)
      }

      wx.showToast({
        title: isEdit ? '打印机已更新' : (isYilianyunAuthorization ? '授权已完成' : '打印机已添加'),
        icon: 'success'
      })
      wx.navigateBack()
    } catch (err) {
      logger.error('Submit printer edit failed', err)
      wx.showToast({ title: getErrorMessage(err, isEdit ? '更新失败，请稍后重试' : '添加失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ submitting: false })
    }
  }
})
