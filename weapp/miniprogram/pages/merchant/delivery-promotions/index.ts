import { getStableBarHeights } from '../../../utils/responsive'
import {
  deliveryFeeService,
  DeliveryPromotionResponse,
  CreateDeliveryPromotionRequest,
  DeliveryFeeAdapter
} from '../../../api/delivery-fee'
import { getMyMerchantProfile } from '../../../api/merchant'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'

// ==================== 类型定义 ====================

type PromotionStatusTheme = 'success' | 'warning' | 'danger' | 'default'

interface PromotionView extends DeliveryPromotionResponse {
  min_order_yuan: string   // 展示用：元
  discount_yuan: string    // 展示用：元
  valid_from_date: string  // 展示用：YYYY-MM-DD
  valid_until_date: string // 展示用：YYYY-MM-DD
  status_label: string
  status_theme: PromotionStatusTheme
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

const PROMOTIONS_AUTO_REFRESH_WINDOW_MS = 60 * 1000

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
  let status_theme: PromotionStatusTheme = 'default'

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

function upsertPromotionView(promotions: PromotionView[], promotion: DeliveryPromotionResponse) {
  const nextPromotion = buildPromotionView(promotion)
  const index = promotions.findIndex((item) => item.id === nextPromotion.id)

  if (index === -1) {
    return [nextPromotion, ...promotions]
  }

  const nextPromotions = [...promotions]
  nextPromotions[index] = nextPromotion
  return nextPromotions
}

function removePromotionView(promotions: PromotionView[], promotionId: number) {
  return promotions.filter((item) => item.id !== promotionId)
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

// ==================== 页面 ====================

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    actionNoticeMessage: '',
    refreshErrorMessage: '',
    loading: false,
    submitting: false,
    actingPromotionId: 0,
    actingPromotionAction: '',
    promotions: [] as PromotionView[],
    merchantId: 0,
    lastLoadedAt: 0,

    // 表单弹窗
    formVisible: false,
    isEdit: false,
    editId: 0,
    form: defaultFormData()
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

    this.loadPageData(true, true)
  },

  onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: ''
    })
    this.onLoad()
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (
      this.data.merchantId > 0
      && !this.data.initialLoading
      && !this.data.submitting
      && !this.data.actingPromotionId
      && shouldAutoRefresh(this.data.lastLoadedAt, PROMOTIONS_AUTO_REFRESH_WINDOW_MS)
    ) {
      void this.loadPromotions(false)
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadPageData(false, true)
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadPageData(true, true)
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadPageData(false, true)
  },

  // ==================== 初始化 ====================

  async ensureMerchantId() {
    if (this.data.merchantId > 0) {
      return this.data.merchantId
    }

    try {
      const cached = wx.getStorageSync('current_merchant') as { id?: number, merchant_id?: number } | null
      const cachedMerchantId = Number(cached?.id || cached?.merchant_id || 0)
      if (cachedMerchantId > 0) {
        this.setData({ merchantId: cachedMerchantId })
        return cachedMerchantId
      }

      const profile = await getMyMerchantProfile()
      const merchantId = Number(profile.id || 0)
      if (merchantId > 0) {
        this.setData({ merchantId })
        return merchantId
      }

      throw new Error('invalid merchant id')
    } catch (err) {
      logger.error('Failed to get merchant info', err)
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(err, '获取商户信息失败，请重试'),
        refreshErrorMessage: ''
      })
      return 0
    }
  },

  async loadPageData(showLoading = true, force = false) {
    const merchantId = await this.ensureMerchantId()
    if (!merchantId) {
      wx.stopPullDownRefresh()
      return
    }

    await this.loadPromotions(showLoading, force)
  },

  // ==================== 数据加载 ====================

  async loadPromotions(showLoading = true, force = false) {
    if (this.data.loading) return

    const hasConfirmedData = this.data.promotions.length > 0 || this.data.lastLoadedAt > 0
    if (!force && hasConfirmedData && !shouldAutoRefresh(this.data.lastLoadedAt, PROMOTIONS_AUTO_REFRESH_WINDOW_MS)) {
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      loading: true,
      ...(showLoading && !hasConfirmedData
        ? {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          }
        : hasConfirmedData
          ? {
              initialError: false,
              initialErrorMessage: '',
              refreshErrorMessage: ''
            }
          : {})
    })

    try {
      const list = await deliveryFeeService.getMerchantPromotions(this.data.merchantId)
      const promotions = (Array.isArray(list) ? list : []).map(buildPromotionView)
      this.setData({
        promotions,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err) {
      logger.error('Failed to load delivery promotions', err)
      const message = getErrorUserMessage(err, '加载配送优惠失败，请稍后重试')

      if (this.data.initialLoading || !hasConfirmedData) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: ''
        })
      } else {
        this.setData({
          refreshErrorMessage: this.data.actionNoticeMessage
            ? `${message}，当前仍显示本页已更新结果`
            : `${message}，当前已保留上次同步结果`
        })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  // ==================== 表单弹窗 ====================

  onAddPromotion() {
    if (this.data.actingPromotionId) return

    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      formVisible: true,
      isEdit: false,
      editId: 0,
      form: defaultFormData()
    })
  },

  onEditPromotion(e: WechatMiniprogram.TouchEvent) {
    if (this.data.actingPromotionId) return

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
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      formVisible: true,
      isEdit: true,
      editId: id,
      form
    })
  },

  onCloseForm() {
    this.setData({ formVisible: false })
  },

  onFormInput(e: WechatMiniprogram.Input) {
    const field = (e.currentTarget.dataset as { field: string }).field
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      [`form.${field}`]: e.detail.value
    })
  },

  onToggleActive(e: WechatMiniprogram.SwitchChange) {
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      'form.is_active': e.detail.value
    })
  },

  // ==================== 提交表单 ====================

  async onSubmitForm() {
    if (this.data.submitting) return

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
      let savedPromotion: DeliveryPromotionResponse

      if (isEdit && editId) {
        savedPromotion = await deliveryFeeService.updateMerchantPromotion(merchantId, editId, {
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
        savedPromotion = await deliveryFeeService.createMerchantPromotion(merchantId, payload)
      }

      this.setData({
        promotions: upsertPromotionView(this.data.promotions, savedPromotion),
        formVisible: false,
        isEdit: false,
        editId: 0,
        form: defaultFormData(),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        actionNoticeMessage: isEdit ? '配送优惠已更新。' : '配送优惠已创建。',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      void this.loadPromotions(false, true)
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
    if (!id || this.data.actingPromotionId) return

    const targetActive = !active
    const { merchantId } = this.data

    this.setData({ actingPromotionId: id, actingPromotionAction: 'toggle' })
    try {
      const updatedPromotion = await deliveryFeeService.updateMerchantPromotion(merchantId, id, { is_active: targetActive })
      this.setData({
        promotions: upsertPromotionView(this.data.promotions, updatedPromotion),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        actionNoticeMessage: updatedPromotion.is_active ? '配送优惠已启用。' : '配送优惠已停用。',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      void this.loadPromotions(false, true)
    } catch (err) {
      logger.error('Toggle promo status failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '更新状态失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ actingPromotionId: 0, actingPromotionAction: '' })
    }
  },

  // ==================== 删除 ====================

  onDeletePromotion(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as { id: number, name: string }
    if (!id || this.data.actingPromotionId) return

    wx.showModal({
      title: '确认删除',
      content: `确定删除「${name}」吗？删除后不可恢复。`,
      confirmColor: '#e34d59',
      success: async (res) => {
        if (!res.confirm) return
        this.setData({ actingPromotionId: id, actingPromotionAction: 'delete' })
        try {
          await deliveryFeeService.deleteMerchantPromotion(this.data.merchantId, id)
          this.setData({
            promotions: removePromotionView(this.data.promotions, id),
            initialLoading: false,
            initialError: false,
            initialErrorMessage: '',
            actionNoticeMessage: '配送优惠已删除。',
            refreshErrorMessage: '',
            lastLoadedAt: Date.now()
          })
          void this.loadPromotions(false, true)
        } catch (err) {
          logger.error('Delete promotion failed', err)
          wx.showToast({ title: getErrorUserMessage(err, '删除失败，请稍后重试'), icon: 'none' })
        } finally {
          this.setData({ actingPromotionId: 0, actingPromotionAction: '' })
        }
      }
    })
  }
})
