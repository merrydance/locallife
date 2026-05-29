import { getStableBarHeights } from '../../../../utils/responsive'
import {
  createMerchantDiscountRule,
  getMerchantDiscountRule,
  type MerchantDiscountRuleResponse,
  updateMerchantDiscountRule
} from '../../../../api/merchant'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'
import { syncCurrentMerchantContext } from '../../_utils/current-merchant'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface DiscountRuleEditOptions {
  id?: string
}

interface DiscountRuleFormData {
  name: string
  description: string
  min_order_amount_yuan: string
  discount_amount_yuan: string
  can_stack_with_voucher: boolean
  can_stack_with_membership: boolean
  stacking_group: string
  valid_from: string
  valid_until: string
  is_active: boolean
}

interface CurrencyInputState {
  fen: number
  hasValue: boolean
  isValid: boolean
}

function createDefaultFormData(): DiscountRuleFormData {
  return {
    name: '',
    description: '',
    min_order_amount_yuan: '',
    discount_amount_yuan: '',
    can_stack_with_voucher: false,
    can_stack_with_membership: false,
    stacking_group: '',
    valid_from: '',
    valid_until: '',
    is_active: true
  }
}

function formatDate(date: string) {
  return String(date || '').slice(0, 10)
}

function buildFormData(rule: MerchantDiscountRuleResponse): DiscountRuleFormData {
  return {
    name: rule.name || '',
    description: rule.description || '',
    min_order_amount_yuan: (rule.min_order_amount / 100).toFixed(2),
    discount_amount_yuan: (rule.discount_amount / 100).toFixed(2),
    can_stack_with_voucher: !!rule.can_stack_with_voucher,
    can_stack_with_membership: !!rule.can_stack_with_membership,
    stacking_group: rule.stacking_group || '',
    valid_from: formatDate(rule.valid_from),
    valid_until: formatDate(rule.valid_until),
    is_active: !!rule.is_active
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
    ruleId: 0,
    validFromPickerVisible: false,
    validUntilPickerVisible: false,
    formData: createDefaultFormData() as DiscountRuleFormData
  },

  async onLoad(options: DiscountRuleEditOptions) {
    const { navBarHeight } = getStableBarHeights()
    const ruleId = Number(options.id || 0)

    this.setData({
      navBarHeight,
      isEdit: ruleId > 0,
      ruleId
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

      await this.loadRuleDetail(merchantContext.merchantId)
    } catch (err) {
      logger.error('Bootstrap discount rule edit page failed', err)
      this.setData({
        accessReady: true,
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(err, '获取商户信息失败，请稍后重试')
      })
    }
  },

  async loadRuleDetail(merchantId?: number) {
    const targetMerchantId = Number(merchantId || this.data.merchantId)

    this.setData({
      initialLoading: true,
      initialError: false,
      initialErrorMessage: ''
    })

    try {
      const rule = await getMerchantDiscountRule(targetMerchantId, this.data.ruleId)
      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        formData: buildFormData(rule)
      })
    } catch (err) {
      logger.error('Load discount rule detail failed', err)
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(err, '满减规则详情加载失败，请稍后重试')
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
    const { field } = e.currentTarget.dataset as { field?: keyof DiscountRuleFormData }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: String(e.detail.value || '') })
  },

  onDateChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof DiscountRuleFormData }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: String(e.detail.value || '') })
  },

  showValidFromPicker() {
    this.setData({ validFromPickerVisible: true })
  },

  hideValidFromPicker() {
    this.setData({ validFromPickerVisible: false })
  },

  showValidUntilPicker() {
    this.setData({ validUntilPickerVisible: true })
  },

  hideValidUntilPicker() {
    this.setData({ validUntilPickerVisible: false })
  },

  onValidFromConfirm(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({
      'formData.valid_from': String(e.detail.value || ''),
      validFromPickerVisible: false
    })
  },

  onValidUntilConfirm(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({
      'formData.valid_until': String(e.detail.value || ''),
      validUntilPickerVisible: false
    })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof DiscountRuleFormData }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: !!e.detail.value })
  },

  async onSubmit() {
    if (this.data.submitting || this.data.initialLoading || this.data.initialError) {
      return
    }

    const { formData, isEdit, merchantId, ruleId } = this.data
    const name = String(formData.name || '').trim()

    if (!name) {
      wx.showToast({ title: '请填写规则名称', icon: 'none' })
      return
    }

    const minOrderAmount = parseCurrencyInput(formData.min_order_amount_yuan)
    if (!minOrderAmount.hasValue || !minOrderAmount.isValid || minOrderAmount.fen <= 0) {
      wx.showToast({ title: '请输入有效的门槛金额', icon: 'none' })
      return
    }

    const discountAmount = parseCurrencyInput(formData.discount_amount_yuan)
    if (!discountAmount.hasValue || !discountAmount.isValid || discountAmount.fen <= 0) {
      wx.showToast({ title: '请输入有效的优惠金额', icon: 'none' })
      return
    }

    if (discountAmount.fen >= minOrderAmount.fen) {
      wx.showToast({ title: '优惠金额需小于门槛金额', icon: 'none' })
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

    this.setData({ submitting: true })
    wx.showLoading({ title: '保存中...' })

    try {
      if (isEdit && ruleId > 0) {
        await updateMerchantDiscountRule(merchantId, ruleId, {
          name,
          description: String(formData.description || '').trim() || undefined,
          min_order_amount: minOrderAmount.fen,
          discount_amount: discountAmount.fen,
          can_stack_with_voucher: formData.can_stack_with_voucher,
          can_stack_with_membership: formData.can_stack_with_membership,
          stacking_group: String(formData.stacking_group || '').trim() || undefined,
          valid_from: toRFC3339Start(formData.valid_from),
          valid_until: toRFC3339End(formData.valid_until),
          is_active: formData.is_active
        })
      } else {
        await createMerchantDiscountRule(merchantId, {
          name,
          description: String(formData.description || '').trim() || undefined,
          min_order_amount: minOrderAmount.fen,
          discount_amount: discountAmount.fen,
          can_stack_with_voucher: formData.can_stack_with_voucher,
          can_stack_with_membership: formData.can_stack_with_membership,
          stacking_group: String(formData.stacking_group || '').trim() || undefined,
          valid_from: toRFC3339Start(formData.valid_from),
          valid_until: toRFC3339End(formData.valid_until)
        })
      }

      wx.showToast({ title: isEdit ? '满减规则已更新' : '满减规则已创建', icon: 'success' })
      wx.navigateBack()
    } catch (err) {
      logger.error('Submit discount rule failed', err)
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