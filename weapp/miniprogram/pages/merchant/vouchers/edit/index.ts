import { getStableBarHeights } from '../../../../utils/responsive'
import {
  MerchantVoucherService,
  type CreateMerchantVoucherParams,
  type MerchantVoucher,
  type UpdateMerchantVoucherParams
} from '../../../../api/coupon'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'
import { syncCurrentMerchantContext } from '../../../../utils/current-merchant'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type OrderType = 'takeout' | 'dine_in' | 'takeaway' | 'reservation'

interface VoucherEditOptions {
  id?: string
}

interface VoucherFormData {
  code: string
  name: string
  description: string
  amount_yuan: string
  min_order_amount_yuan: string
  total_quantity: string
  valid_from: string
  valid_until: string
  is_active: boolean
  allowed_order_types: OrderType[]
}

interface CurrencyInputState {
  fen: number
  hasValue: boolean
  isValid: boolean
}

const VOUCHER_PAGE_SIZE = 50
const ALL_ORDER_TYPES: OrderType[] = ['takeout', 'dine_in', 'takeaway', 'reservation']
const ORDER_TYPE_OPTIONS: Array<{ value: OrderType, label: string }> = [
  { value: 'takeout', label: '外卖配送' },
  { value: 'dine_in', label: '堂食' },
  { value: 'takeaway', label: '到店自取' },
  { value: 'reservation', label: '预订' }
]

function createDefaultFormData(): VoucherFormData {
  return {
    code: '',
    name: '',
    description: '',
    amount_yuan: '',
    min_order_amount_yuan: '',
    total_quantity: '',
    valid_from: '',
    valid_until: '',
    is_active: true,
    allowed_order_types: [...ALL_ORDER_TYPES]
  }
}

function formatDate(date: string) {
  return String(date || '').slice(0, 10)
}

function buildFormData(voucher: MerchantVoucher): VoucherFormData {
  return {
    code: voucher.code || '',
    name: voucher.name || '',
    description: voucher.description || '',
    amount_yuan: (voucher.amount / 100).toFixed(2),
    min_order_amount_yuan: voucher.min_order_amount > 0 ? (voucher.min_order_amount / 100).toFixed(2) : '',
    total_quantity: String(voucher.total_quantity || ''),
    valid_from: formatDate(voucher.valid_from),
    valid_until: formatDate(voucher.valid_until),
    is_active: !!voucher.is_active,
    allowed_order_types: (voucher.allowed_order_types.length ? voucher.allowed_order_types : ALL_ORDER_TYPES) as OrderType[]
  }
}

function parseCurrencyInput(value: string): CurrencyInputState {
  const trimmedValue = String(value || '').trim()
  if (!trimmedValue) {
    return { fen: 0, hasValue: false, isValid: false }
  }

  if (!/^\d+(\.\d{0,2})?$/.test(trimmedValue)) {
    return { fen: 0, hasValue: true, isValid: false }
  }

  const parsedValue = Number(trimmedValue)
  if (!Number.isFinite(parsedValue)) {
    return { fen: 0, hasValue: true, isValid: false }
  }

  return {
    fen: Math.round(parsedValue * 100),
    hasValue: true,
    isValid: true
  }
}

function toRFC3339Start(date: string): string {
  return `${date}T00:00:00+08:00`
}

function toRFC3339End(date: string): string {
  return `${date}T23:59:59+08:00`
}

function buildCreatePayload(form: VoucherFormData): CreateMerchantVoucherParams {
  return {
    code: String(form.code || '').trim() || undefined,
    name: String(form.name || '').trim(),
    description: String(form.description || '').trim() || undefined,
    amount: Math.round(Number(form.amount_yuan) * 100),
    min_order_amount: form.min_order_amount_yuan ? Math.round(Number(form.min_order_amount_yuan) * 100) : 0,
    total_quantity: Number(form.total_quantity),
    valid_from: toRFC3339Start(form.valid_from),
    valid_until: toRFC3339End(form.valid_until),
    allowed_order_types: form.allowed_order_types
  }
}

function buildUpdatePayload(form: VoucherFormData): UpdateMerchantVoucherParams {
  return {
    name: String(form.name || '').trim(),
    description: String(form.description || '').trim() || undefined,
    amount: Math.round(Number(form.amount_yuan) * 100),
    min_order_amount: form.min_order_amount_yuan ? Math.round(Number(form.min_order_amount_yuan) * 100) : 0,
    total_quantity: Number(form.total_quantity),
    valid_from: toRFC3339Start(form.valid_from),
    valid_until: toRFC3339End(form.valid_until),
    is_active: form.is_active,
    allowed_order_types: form.allowed_order_types
  }
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
    submitting: false,
    isEdit: false,
    merchantId: 0,
    voucherId: 0,
    formData: createDefaultFormData() as VoucherFormData,
    orderTypeOptions: ORDER_TYPE_OPTIONS
  },

  async onLoad(options: VoucherEditOptions) {
    const { navBarHeight } = getStableBarHeights()
    const voucherId = Number(options.id || 0)

    this.setData({
      navBarHeight,
      isEdit: voucherId > 0,
      voucherId
    })

    await this.bootstrapPage()
  },

  async bootstrapPage() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      submitting: false
    })

    const accessResult = await ensureMerchantConsoleAccess()
    if (accessResult.status !== 'granted') {
      this.setData({
        accessReady: true,
        accessDenied: accessResult.status === 'denied',
        accessErrorMessage: accessResult.status === 'error' ? accessResult.message : '',
        initialLoading: false
      })
      return
    }

    try {
      const merchantContext = await syncCurrentMerchantContext()
      if (!merchantContext.merchantId) {
        throw new Error('invalid merchant id')
      }

      this.setData({
        accessReady: true,
        accessDenied: false,
        accessErrorMessage: '',
        merchantId: merchantContext.merchantId
      })

      if (!this.data.isEdit) {
        this.setData({
          initialLoading: false,
          initialError: false,
          initialErrorMessage: '',
          formData: createDefaultFormData()
        })
        return
      }

      await this.loadVoucherDetail(merchantContext.merchantId)
    } catch (err) {
      logger.error('Bootstrap voucher edit page failed', err)
      this.setData({
        accessReady: true,
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(err, '获取商户信息失败，请稍后重试')
      })
    }
  },

  async loadVoucherDetail(merchantId?: number) {
    const targetMerchantId = Number(merchantId || this.data.merchantId)

    this.setData({
      initialLoading: true,
      initialError: false,
      initialErrorMessage: ''
    })

    try {
      let pageId = 1
      let targetVoucher: MerchantVoucher | undefined

      while (pageId <= 20) {
        const response = await MerchantVoucherService.listMerchantVouchers(targetMerchantId, pageId, VOUCHER_PAGE_SIZE)
        targetVoucher = (Array.isArray(response.vouchers) ? response.vouchers : []).find((item) => item.id === this.data.voucherId)

        if (targetVoucher) {
          break
        }

        if (!response.hasMore || !response.vouchers.length || response.vouchers.length < response.pageSize) {
          break
        }

        pageId += 1
      }

      if (!targetVoucher) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: '未找到该代金券，可能已被删除'
        })
        return
      }

      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        formData: buildFormData(targetVoucher)
      })
    } catch (err) {
      logger.error('Load voucher detail failed', err)
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(err, '代金券详情加载失败，请稍后重试')
      })
    }
  },

  onRetryAccess() {
    void this.bootstrapPage()
  },

  onRetry() {
    void this.bootstrapPage()
  },

  onFormInputChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof VoucherFormData }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: String(e.detail.value || '') })
  },

  onDateChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof VoucherFormData }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: String(e.detail.value || '') })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof VoucherFormData }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: !!e.detail.value })
  },

  onToggleOrderType(e: WechatMiniprogram.TouchEvent) {
    const dataset = e.currentTarget.dataset as { value?: OrderType }
    const value = dataset.value
    if (!value) {
      return
    }

    const next = [...this.data.formData.allowed_order_types]
    const index = next.indexOf(value)
    if (index >= 0) {
      next.splice(index, 1)
    } else {
      next.push(value)
    }

    this.setData({ 'formData.allowed_order_types': next })
  },

  async onSubmit() {
    if (this.data.submitting || this.data.initialLoading || this.data.initialError) {
      return
    }

    const { formData, isEdit, merchantId, voucherId } = this.data
    const name = String(formData.name || '').trim()
    if (!name) {
      wx.showToast({ title: '请填写代金券名称', icon: 'none' })
      return
    }

    const amount = parseCurrencyInput(formData.amount_yuan)
    if (!amount.hasValue || !amount.isValid || amount.fen <= 0) {
      wx.showToast({ title: '请输入有效的抵扣金额', icon: 'none' })
      return
    }

    const minOrderAmount = formData.min_order_amount_yuan
      ? parseCurrencyInput(formData.min_order_amount_yuan)
      : { fen: 0, hasValue: true, isValid: true }
    if (!minOrderAmount.isValid || minOrderAmount.fen < 0) {
      wx.showToast({ title: '请输入有效的使用门槛', icon: 'none' })
      return
    }

    const totalQuantity = Number(formData.total_quantity)
    if (!Number.isFinite(totalQuantity) || totalQuantity <= 0) {
      wx.showToast({ title: '请输入有效的发放数量', icon: 'none' })
      return
    }

    if (!formData.valid_from) {
      wx.showToast({ title: '请选择开始日期', icon: 'none' })
      return
    }

    if (!formData.valid_until) {
      wx.showToast({ title: '请选择结束日期', icon: 'none' })
      return
    }

    if (formData.valid_until < formData.valid_from) {
      wx.showToast({ title: '结束日期不能早于开始日期', icon: 'none' })
      return
    }

    if (!formData.allowed_order_types.length) {
      wx.showToast({ title: '请至少选择一个适用订单类型', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '保存中...' })

    try {
      if (isEdit && voucherId > 0) {
        await MerchantVoucherService.updateMerchantVoucher(merchantId, voucherId, buildUpdatePayload(formData))
      } else {
        await MerchantVoucherService.createMerchantVoucher(merchantId, buildCreatePayload(formData))
      }

      wx.showToast({ title: isEdit ? '代金券已更新' : '代金券已创建', icon: 'success' })
      wx.navigateBack()
    } catch (err) {
      logger.error('Submit voucher failed', err)
      wx.showToast({
        title: getErrorUserMessage(err, isEdit ? '更新失败，请稍后重试' : '创建失败，请稍后重试'),
        icon: 'none'
      })
    } finally {
      wx.hideLoading()
      this.setData({ submitting: false })
    }
  }
})