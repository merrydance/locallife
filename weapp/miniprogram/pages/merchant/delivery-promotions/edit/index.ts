import { getStableBarHeights } from '../../../../utils/responsive'
import {
  deliveryFeeService,
  type CreateDeliveryPromotionRequest,
  type DeliveryPromotionResponse,
  DeliveryFeeAdapter
} from '../../../../api/delivery-fee'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'
import { syncCurrentMerchantContext } from '../../../../utils/current-merchant'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface DeliveryPromotionEditOptions {
  id?: string
}

interface PromotionFormData {
  name: string
  min_order_yuan: string
  discount_yuan: string
  valid_from: string
  valid_until: string
  is_active: boolean
}

interface CurrencyInputState {
  fen: number
  hasValue: boolean
  isValid: boolean
}

function createDefaultFormData(): PromotionFormData {
  return {
    name: '',
    min_order_yuan: '',
    discount_yuan: '',
    valid_from: '',
    valid_until: '',
    is_active: true
  }
}

function buildFormData(promotion: DeliveryPromotionResponse): PromotionFormData {
  return {
    name: promotion.name || '',
    min_order_yuan: DeliveryFeeAdapter.formatAmount(promotion.min_order_amount),
    discount_yuan: DeliveryFeeAdapter.formatAmount(promotion.discount_amount),
    valid_from: DeliveryFeeAdapter.formatDate(promotion.valid_from),
    valid_until: DeliveryFeeAdapter.formatDate(promotion.valid_until),
    is_active: !!promotion.is_active
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
    promotionId: 0,
    validFromPickerVisible: false,
    validUntilPickerVisible: false,
    formData: createDefaultFormData() as PromotionFormData
  },

  async onLoad(options: DeliveryPromotionEditOptions) {
    const { navBarHeight } = getStableBarHeights()
    const promotionId = Number(options.id || 0)

    this.setData({
      navBarHeight,
      isEdit: promotionId > 0,
      promotionId
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

      await this.loadPromotionDetail(merchantContext.merchantId)
    } catch (err) {
      logger.error('Bootstrap delivery promotion edit page failed', err)
      this.setData({
        accessReady: true,
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(err, '获取商户信息失败，请稍后重试')
      })
    }
  },

  async loadPromotionDetail(merchantId?: number) {
    const targetMerchantId = Number(merchantId || this.data.merchantId)

    this.setData({
      initialLoading: true,
      initialError: false,
      initialErrorMessage: ''
    })

    try {
      const promotions = await deliveryFeeService.getMerchantPromotions(targetMerchantId)
      const targetPromotion = (Array.isArray(promotions) ? promotions : []).find((item) => item.id === this.data.promotionId)

      if (!targetPromotion) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: '未找到该代取优惠，可能已被删除'
        })
        return
      }

      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        formData: buildFormData(targetPromotion)
      })
    } catch (err) {
      logger.error('Load delivery promotion detail failed', err)
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(err, '代取优惠详情加载失败，请稍后重试')
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
    const { field } = e.currentTarget.dataset as { field?: keyof PromotionFormData }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: String(e.detail.value || '') })
  },

  onDateChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof PromotionFormData }
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
    const { field } = e.currentTarget.dataset as { field?: keyof PromotionFormData }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: !!e.detail.value })
  },

  async onSubmit() {
    if (this.data.submitting || this.data.initialLoading || this.data.initialError) {
      return
    }

    const { formData, isEdit, merchantId, promotionId } = this.data
    const name = String(formData.name || '').trim()

    if (!name) {
      wx.showToast({ title: '请填写优惠名称', icon: 'none' })
      return
    }

    const minOrderAmount = parseCurrencyInput(formData.min_order_yuan)
    if (!minOrderAmount.hasValue || !minOrderAmount.isValid || minOrderAmount.fen <= 0) {
      wx.showToast({ title: '请输入有效的最低订单金额', icon: 'none' })
      return
    }

    const discountAmount = parseCurrencyInput(formData.discount_yuan)
    if (!discountAmount.hasValue || !discountAmount.isValid || discountAmount.fen <= 0) {
      wx.showToast({ title: '请输入有效的优惠金额', icon: 'none' })
      return
    }

    if (discountAmount.fen > minOrderAmount.fen) {
      wx.showToast({ title: '优惠金额不能超过最低订单金额', icon: 'none' })
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
      if (isEdit && promotionId > 0) {
        await deliveryFeeService.updateMerchantPromotion(merchantId, promotionId, {
          name,
          min_order_amount: minOrderAmount.fen,
          discount_amount: discountAmount.fen,
          valid_from: toRFC3339Start(formData.valid_from),
          valid_until: toRFC3339End(formData.valid_until),
          is_active: formData.is_active
        })
      } else {
        const payload: CreateDeliveryPromotionRequest = {
          name,
          min_order_amount: minOrderAmount.fen,
          discount_amount: discountAmount.fen,
          valid_from: toRFC3339Start(formData.valid_from),
          valid_until: toRFC3339End(formData.valid_until)
        }
        await deliveryFeeService.createMerchantPromotion(merchantId, payload)
      }

      wx.showToast({
        title: isEdit ? '代取优惠已更新' : '代取优惠已创建',
        icon: 'success'
      })
      wx.navigateBack()
    } catch (err) {
      logger.error('Submit delivery promotion failed', err)
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