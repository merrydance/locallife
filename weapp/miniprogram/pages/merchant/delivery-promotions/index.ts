import { getStableBarHeights } from '../../../utils/responsive'
import {
  deliveryFeeService,
  DeliveryPromotionResponse,
  CreateDeliveryPromotionRequest,
  DeliveryFeeAdapter
} from '../../../api/delivery-fee'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'

// ==================== 类型定义 ====================

interface PromotionView extends DeliveryPromotionResponse {
  min_order_yuan: string   // 展示用：元
  discount_yuan: string    // 展示用：元
  valid_from_date: string  // 展示用：YYYY-MM-DD
  valid_until_date: string // 展示用：YYYY-MM-DD
  status_label: string
  status_theme: string
}

interface PromotionFormData {
  name: string
  min_order_yuan: string   // 输入框用"元"字符串
  discount_yuan: string
  valid_from: string       // YYYY-MM-DD
  valid_until: string
  is_active: boolean
}

function defaultFormData(): PromotionFormData {
  return {
    name: '',
    min_order_yuan: '',
    discount_yuan: '',
    valid_from: '',
    valid_until: '',
    is_active: true
  }
}

// 将 YYYY-MM-DD 转换为 RFC3339 当天开始/结束时间
function toRFC3339Start(date: string): string {
  return `${date}T00:00:00+08:00`
}
function toRFC3339End(date: string): string {
  return `${date}T23:59:59+08:00`
}

function buildPromotionView(p: DeliveryPromotionResponse): PromotionView {
  const now = new Date()
  const until = new Date(p.valid_until)
  const from = new Date(p.valid_from)
  let status_label = ''
  let status_theme = ''

  if (!p.is_active) {
    status_label = '已停用'
    status_theme = 'default'
  } else if (now > until) {
    status_label = '已过期'
    status_theme = 'danger'
  } else if (now < from) {
    status_label = '未开始'
    status_theme = 'warning'
  } else {
    status_label = '生效中'
    status_theme = 'success'
  }

  return {
    ...p,
    min_order_yuan: DeliveryFeeAdapter.formatAmount(p.min_order_amount),
    discount_yuan: DeliveryFeeAdapter.formatAmount(p.discount_amount),
    valid_from_date: DeliveryFeeAdapter.formatDate(p.valid_from),
    valid_until_date: DeliveryFeeAdapter.formatDate(p.valid_until),
    status_label,
    status_theme
  }
}

// ==================== 页面 ====================

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    submitting: false,
    promotions: [] as PromotionView[],
    merchantId: 0,

    // 表单弹窗
    formVisible: false,
    isEdit: false,
    editId: 0,
    form: defaultFormData()
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.initMerchantId()
  },

  onShow() {
    if (this.data.merchantId > 0) {
      this.loadPromotions()
    }
  },

  onPullDownRefresh() {
    this.loadPromotions()
  },

  // ==================== 初始化 ====================

  async initMerchantId() {
    // 从全局缓存读取当前商户 ID（与其它商户页面保持一致）
    try {
      const { request } = require('../../../utils/request')
      const merchant = await request({ url: '/v1/merchants/me', method: 'GET' })
      const id: number = merchant?.id || 0
      this.setData({ merchantId: id })
      if (id > 0) this.loadPromotions()
    } catch (err) {
      logger.error('Failed to get merchant info', err)
      wx.showToast({ title: '获取商户信息失败', icon: 'none' })
    }
  },

  // ==================== 数据加载 ====================

  async loadPromotions() {
    if (this.data.loading) return
    this.setData({ loading: true })
    try {
      const list = await deliveryFeeService.getMerchantPromotions(this.data.merchantId)
      const promotions = (Array.isArray(list) ? list : []).map(buildPromotionView)
      this.setData({ promotions })
    } catch (err) {
      logger.error('Failed to load delivery promotions', err)
      wx.showToast({ title: '加载失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  // ==================== 表单弹窗 ====================

  onAddPromotion() {
    this.setData({
      formVisible: true,
      isEdit: false,
      editId: 0,
      form: defaultFormData()
    })
  },

  onEditPromotion(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id: number }
    const promo = this.data.promotions.find((p) => p.id === id)
    if (!promo) return

    const form: PromotionFormData = {
      name: promo.name,
      min_order_yuan: DeliveryFeeAdapter.formatAmount(promo.min_order_amount),
      discount_yuan: DeliveryFeeAdapter.formatAmount(promo.discount_amount),
      valid_from: promo.valid_from_date,
      valid_until: promo.valid_until_date,
      is_active: promo.is_active
    }
    this.setData({ formVisible: true, isEdit: true, editId: id, form })
  },

  onCloseForm() {
    this.setData({ formVisible: false })
  },

  onFormInput(e: WechatMiniprogram.Input) {
    const field = (e.currentTarget.dataset as { field: string }).field
    this.setData({ [`form.${field}`]: e.detail.value })
  },

  onToggleActive(e: WechatMiniprogram.SwitchChange) {
    this.setData({ 'form.is_active': e.detail.value })
  },

  // ==================== 提交表单 ====================

  async onSubmitForm() {
    const { form, isEdit, editId, merchantId } = this.data

    // 基础校验
    if (!form.name.trim()) {
      wx.showToast({ title: '请填写优惠名称', icon: 'none' })
      return
    }
    const minOrderFen = Math.round(parseFloat(form.min_order_yuan) * 100)
    const discountFen = Math.round(parseFloat(form.discount_yuan) * 100)
    if (!minOrderFen || minOrderFen <= 0) {
      wx.showToast({ title: '请输入有效的最低订单金额', icon: 'none' })
      return
    }
    if (!discountFen || discountFen <= 0) {
      wx.showToast({ title: '请输入有效的优惠金额', icon: 'none' })
      return
    }
    if (discountFen > minOrderFen) {
      wx.showToast({ title: '优惠金额不能超过最低单价', icon: 'none' })
      return
    }
    if (!form.valid_from) {
      wx.showToast({ title: '请选择开始日期', icon: 'none' })
      return
    }
    if (!form.valid_until) {
      wx.showToast({ title: '请选择结束日期', icon: 'none' })
      return
    }
    if (form.valid_until < form.valid_from) {
      wx.showToast({ title: '结束日期不能早于开始日期', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '保存中...' })
    try {
      if (isEdit && editId) {
        await deliveryFeeService.updateMerchantPromotion(merchantId, editId, {
          name: form.name.trim(),
          min_order_amount: minOrderFen,
          discount_amount: discountFen,
          valid_from: toRFC3339Start(form.valid_from),
          valid_until: toRFC3339End(form.valid_until),
          is_active: form.is_active
        })
      } else {
        const payload: CreateDeliveryPromotionRequest = {
          name: form.name.trim(),
          min_order_amount: minOrderFen,
          discount_amount: discountFen,
          valid_from: toRFC3339Start(form.valid_from),
          valid_until: toRFC3339End(form.valid_until)
        }
        await deliveryFeeService.createMerchantPromotion(merchantId, payload)
      }
      this.setData({ formVisible: false })
      await this.loadPromotions()
    } catch (err: unknown) {
      logger.error('Submit promotion failed', err)
      const msg = getErrorUserMessage(err, isEdit ? '更新失败，请稍后重试' : '创建失败，请稍后重试')
      wx.showToast({ title: msg, icon: 'none' })
    } finally {
      this.setData({ submitting: false })
      wx.hideLoading()
    }
  },

  // ==================== 切换启用状态 ====================

  async onTogglePromoStatus(e: WechatMiniprogram.TouchEvent) {
    const { id, active } = e.currentTarget.dataset as { id: number, active: boolean }
    const targetActive = !active
    const { merchantId } = this.data

    wx.showLoading({ title: '处理中...' })
    try {
      await deliveryFeeService.updateMerchantPromotion(merchantId, id, { is_active: targetActive })
      const idx = this.data.promotions.findIndex((p) => p.id === id)
      if (idx >= 0) {
        const updated = buildPromotionView({ ...this.data.promotions[idx], is_active: targetActive })
        this.setData({ [`promotions[${idx}]`]: updated })
      }
    } catch (err) {
      logger.error('Toggle promo status failed', err)
      wx.showToast({ title: '操作失败', icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  // ==================== 删除 ====================

  onDeletePromotion(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as { id: number, name: string }

    wx.showModal({
      title: '确认删除',
      content: `确定删除「${name}」吗？删除后不可恢复。`,
      confirmColor: '#e34d59',
      success: async (res) => {
        if (!res.confirm) return
        wx.showLoading({ title: '删除中...' })
        try {
          await deliveryFeeService.deleteMerchantPromotion(this.data.merchantId, id)
          await this.loadPromotions()
        } catch (err) {
          logger.error('Delete promotion failed', err)
          wx.showToast({ title: '删除失败', icon: 'none' })
        } finally {
          wx.hideLoading()
        }
      }
    })
  }
})
