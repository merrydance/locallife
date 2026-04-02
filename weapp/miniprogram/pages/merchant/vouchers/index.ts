import dayjs from 'dayjs'
import { CreateMerchantVoucherParams, MerchantVoucher, MerchantVoucherService, UpdateMerchantVoucherParams } from '../../../api/coupon'
import { getMyMerchantProfile } from '../../../api/merchant'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

type VoucherStatusTheme = 'success' | 'warning' | 'danger' | 'default'
type OrderType = 'takeout' | 'dine_in' | 'takeaway' | 'reservation'

interface VoucherView extends MerchantVoucher {
  amount_text: string
  min_order_amount_text: string
  valid_range_text: string
  remaining_quantity: number
  remaining_quantity_text: string
  order_type_labels: string[]
  status_label: string
  status_theme: VoucherStatusTheme
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

const ALL_ORDER_TYPES: OrderType[] = ['takeout', 'dine_in', 'takeaway', 'reservation']

const ORDER_TYPE_OPTIONS: Array<{ value: OrderType, label: string }> = [
  { value: 'takeout', label: '外卖配送' },
  { value: 'dine_in', label: '堂食' },
  { value: 'takeaway', label: '到店自取' },
  { value: 'reservation', label: '预订' }
]

function formatMoney(amount: number) {
  return `¥${(amount / 100).toFixed(2)}`
}

function toRFC3339Start(date: string) {
  return `${date}T00:00:00+08:00`
}

function toRFC3339End(date: string) {
  return `${date}T23:59:59+08:00`
}

function defaultFormData(): VoucherFormData {
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

function getOrderTypeLabel(type: OrderType) {
  const matched = ORDER_TYPE_OPTIONS.find((item) => item.value === type)
  return matched?.label || type
}

function buildStatus(view: MerchantVoucher) {
  const now = dayjs()
  const validFrom = dayjs(view.valid_from)
  const validUntil = dayjs(view.valid_until)
  const remainingQuantity = Math.max(view.total_quantity - view.claimed_quantity, 0)

  if (!view.is_active) {
    return { label: '已停用', theme: 'default' as VoucherStatusTheme }
  }
  if (validUntil.isValid() && now.isAfter(validUntil)) {
    return { label: '已过期', theme: 'danger' as VoucherStatusTheme }
  }
  if (validFrom.isValid() && now.isBefore(validFrom)) {
    return { label: '未开始', theme: 'warning' as VoucherStatusTheme }
  }
  if (remainingQuantity <= 0) {
    return { label: '已领完', theme: 'warning' as VoucherStatusTheme }
  }
  return { label: '发放中', theme: 'success' as VoucherStatusTheme }
}

function buildVoucherView(voucher: MerchantVoucher): VoucherView {
  const status = buildStatus(voucher)
  const remainingQuantity = Math.max(voucher.total_quantity - voucher.claimed_quantity, 0)
  return {
    ...voucher,
    amount_text: formatMoney(voucher.amount),
    min_order_amount_text: voucher.min_order_amount > 0 ? formatMoney(voucher.min_order_amount) : '不限门槛',
    valid_range_text: `${dayjs(voucher.valid_from).format('YYYY-MM-DD')} 至 ${dayjs(voucher.valid_until).format('YYYY-MM-DD')}`,
    remaining_quantity: remainingQuantity,
    remaining_quantity_text: `${remainingQuantity} / ${voucher.total_quantity}`,
    order_type_labels: (voucher.allowed_order_types.length ? voucher.allowed_order_types : ALL_ORDER_TYPES).map((item) => getOrderTypeLabel(item as OrderType)),
    status_label: status.label,
    status_theme: status.theme
  }
}

function toFormData(voucher: MerchantVoucher): VoucherFormData {
  return {
    code: voucher.code,
    name: voucher.name,
    description: voucher.description,
    amount_yuan: (voucher.amount / 100).toFixed(2),
    min_order_amount_yuan: voucher.min_order_amount > 0 ? (voucher.min_order_amount / 100).toFixed(2) : '',
    total_quantity: String(voucher.total_quantity),
    valid_from: dayjs(voucher.valid_from).format('YYYY-MM-DD'),
    valid_until: dayjs(voucher.valid_until).format('YYYY-MM-DD'),
    is_active: voucher.is_active,
    allowed_order_types: (voucher.allowed_order_types.length ? voucher.allowed_order_types : ALL_ORDER_TYPES) as OrderType[]
  }
}

function getCreatePayload(form: VoucherFormData): CreateMerchantVoucherParams {
  return {
    code: form.code.trim() || undefined,
    name: form.name.trim(),
    description: form.description.trim() || undefined,
    amount: Math.round(Number(form.amount_yuan) * 100),
    min_order_amount: form.min_order_amount_yuan ? Math.round(Number(form.min_order_amount_yuan) * 100) : 0,
    total_quantity: Number(form.total_quantity),
    valid_from: toRFC3339Start(form.valid_from),
    valid_until: toRFC3339End(form.valid_until),
    allowed_order_types: form.allowed_order_types
  }
}

function getUpdatePayload(form: VoucherFormData): UpdateMerchantVoucherParams {
  return {
    name: form.name.trim(),
    description: form.description.trim() || undefined,
    amount: Math.round(Number(form.amount_yuan) * 100),
    min_order_amount: form.min_order_amount_yuan ? Math.round(Number(form.min_order_amount_yuan) * 100) : 0,
    total_quantity: Number(form.total_quantity),
    valid_from: toRFC3339Start(form.valid_from),
    valid_until: toRFC3339End(form.valid_until),
    is_active: form.is_active,
    allowed_order_types: form.allowed_order_types
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    submitting: false,
    merchantId: 0,
    vouchers: [] as VoucherView[],
    formVisible: false,
    isEdit: false,
    editId: 0,
    form: defaultFormData(),
    orderTypeOptions: ORDER_TYPE_OPTIONS
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.initMerchantId()
  },

  onShow() {
    if (this.data.merchantId > 0) {
      this.loadVouchers()
    }
  },

  onPullDownRefresh() {
    this.loadVouchers()
  },

  async initMerchantId() {
    try {
      const cached = wx.getStorageSync('current_merchant') as { id?: number, merchant_id?: number } | null
      const cachedMerchantId = Number(cached?.id || cached?.merchant_id || 0)
      if (cachedMerchantId > 0) {
        this.setData({ merchantId: cachedMerchantId })
        await this.loadVouchers()
        return
      }

      const profile = await getMyMerchantProfile()
      this.setData({ merchantId: profile.id })
      await this.loadVouchers()
    } catch (err) {
      logger.error('Init merchant voucher context failed', err)
      wx.showToast({ title: '获取商户信息失败', icon: 'none' })
    }
  },

  async loadVouchers() {
    if (this.data.loading || !this.data.merchantId) return
    this.setData({ loading: true })
    try {
      const result = await MerchantVoucherService.listMerchantVouchers(this.data.merchantId, 1, 50)
      this.setData({ vouchers: result.vouchers.map(buildVoucherView) })
    } catch (err) {
      logger.error('Load merchant vouchers failed', err)
      wx.showToast({ title: getErrorMessage(err, '加载代金券失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onAddVoucher() {
    this.setData({
      formVisible: true,
      isEdit: false,
      editId: 0,
      form: defaultFormData()
    })
  },

  onEditVoucher(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    const target = this.data.vouchers.find((item) => item.id === id)
    if (!target) return

    this.setData({
      formVisible: true,
      isEdit: true,
      editId: id,
      form: toFormData(target)
    })
  },

  onCloseForm() {
    this.setData({ formVisible: false })
  },

  onTextInput(e: WechatMiniprogram.Input) {
    const { field } = e.currentTarget.dataset as { field?: keyof VoucherFormData }
    if (!field) return
    this.setData({ [`form.${field}`]: e.detail.value })
  },

  onDateChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: 'valid_from' | 'valid_until' }
    if (!field) return
    this.setData({ [`form.${field}`]: e.detail.value })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field?: 'is_active' }
    if (!field) return
    this.setData({ [`form.${field}`]: Boolean(e.detail.value) })
  },

  onToggleOrderType(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: OrderType }
    if (!value) return

    const next = [...this.data.form.allowed_order_types]
    const index = next.indexOf(value)
    if (index >= 0) {
      next.splice(index, 1)
    } else {
      next.push(value)
    }

    this.setData({ 'form.allowed_order_types': next })
  },

  validateForm() {
    const { form } = this.data
    if (!form.name.trim()) return '请填写代金券名称'
    if (!form.amount_yuan || Number(form.amount_yuan) <= 0) return '请输入有效的抵扣金额'
    if (form.min_order_amount_yuan && Number(form.min_order_amount_yuan) < 0) return '使用门槛不能小于 0'
    if (!form.total_quantity || Number(form.total_quantity) <= 0) return '请输入有效的发放数量'
    if (!form.valid_from) return '请选择生效开始日期'
    if (!form.valid_until) return '请选择生效结束日期'
    if (form.valid_until < form.valid_from) return '结束日期不能早于开始日期'
    if (!form.allowed_order_types.length) return '请至少选择一个可用订单类型'
    return ''
  },

  async onSubmitForm() {
    const errorMessage = this.validateForm()
    if (errorMessage) {
      wx.showToast({ title: errorMessage, icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '保存中...' })
    try {
      if (this.data.isEdit && this.data.editId) {
        await MerchantVoucherService.updateMerchantVoucher(this.data.merchantId, this.data.editId, getUpdatePayload(this.data.form))
      } else {
        await MerchantVoucherService.createMerchantVoucher(this.data.merchantId, getCreatePayload(this.data.form))
      }

      this.setData({ formVisible: false })
      await this.loadVouchers()
    } catch (err) {
      logger.error('Submit merchant voucher failed', err)
      wx.showToast({ title: getErrorMessage(err, this.data.isEdit ? '更新代金券失败，请稍后重试' : '创建代金券失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
      wx.hideLoading()
    }
  },

  onToggleVoucherStatus(e: WechatMiniprogram.TouchEvent) {
    const { id, active, name } = e.currentTarget.dataset as { id?: number, active?: boolean, name?: string }
    if (!id || typeof active !== 'boolean') return

    wx.showModal({
      title: active ? '停用代金券' : '启用代金券',
      content: `${active ? '停用' : '启用'}「${name || '该代金券'}」后，顾客${active ? '将不能继续领取和使用' : '可以继续领取并在有效期内使用'}。`,
      success: async (res) => {
        if (!res.confirm) return
        wx.showLoading({ title: '处理中...' })
        try {
          await MerchantVoucherService.updateMerchantVoucher(this.data.merchantId, id, { is_active: !active })
          await this.loadVouchers()
        } catch (err) {
          logger.error('Toggle merchant voucher status failed', err)
          wx.showToast({ title: getErrorMessage(err, '更新状态失败，请稍后重试'), icon: 'none' })
        } finally {
          wx.hideLoading()
        }
      }
    })
  },

  onDeleteVoucher(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
    if (!id) return

    wx.showModal({
      title: '确认删除',
      content: `删除「${name || '该代金券'}」后不可恢复；若仍有未使用券，后端会拒绝删除。`,
      confirmColor: '#e34d59',
      success: async (res) => {
        if (!res.confirm) return

        wx.showLoading({ title: '删除中...' })
        try {
          await MerchantVoucherService.deleteMerchantVoucher(this.data.merchantId, id)
          await this.loadVouchers()
        } catch (err) {
          logger.error('Delete merchant voucher failed', err)
          wx.showToast({ title: getErrorMessage(err, '删除代金券失败，请稍后重试'), icon: 'none' })
        } finally {
          wx.hideLoading()
        }
      }
    })
  }
})