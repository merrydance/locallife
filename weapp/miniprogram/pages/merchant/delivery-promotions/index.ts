import { getStableBarHeights } from '../../../utils/responsive'
import {
  deliveryFeeService,
  type DeliveryPromotionResponse,
  DeliveryFeeAdapter,
  buildDeliveryPromotionStatusView,
  type DeliveryPromotionStatusTheme
} from '../../../api/delivery-fee'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { syncCurrentMerchantContext } from '../../../utils/current-merchant'
import { getErrorUserMessage } from '../../../utils/user-facing'

interface PromotionView extends DeliveryPromotionResponse {
  min_order_yuan: string
  discount_yuan: string
  valid_from_date: string
  valid_until_date: string
  status_label: string
  status_theme: DeliveryPromotionStatusTheme
  statusPending: boolean
  deletePending: boolean
}

const PROMOTIONS_AUTO_REFRESH_WINDOW_MS = 60 * 1000

function buildResultSummaryText(visibleCount: number) {
  return `当前共 ${visibleCount} 条代取优惠`
}

function buildEmptyDescription() {
  return '当前还没有代取优惠，先新增一个'
}

function buildPresentationUpdate(promotions: PromotionView[]) {
  return {
    promotions,
    resultSummaryText: buildResultSummaryText(promotions.length),
    emptyDescription: buildEmptyDescription()
  }
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

function buildPromotionView(promotion: DeliveryPromotionResponse): PromotionView {
  const statusView = buildDeliveryPromotionStatusView(promotion)

  return {
    ...promotion,
    min_order_yuan: DeliveryFeeAdapter.formatAmount(promotion.min_order_amount),
    discount_yuan: DeliveryFeeAdapter.formatAmount(promotion.discount_amount),
    valid_from_date: DeliveryFeeAdapter.formatDate(promotion.valid_from),
    valid_until_date: DeliveryFeeAdapter.formatDate(promotion.valid_until),
    status_label: statusView.label,
    status_theme: statusView.theme,
    statusPending: false,
    deletePending: false
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

function buildEditPageUrl(promotionId?: number) {
  if (promotionId && promotionId > 0) {
    return `/pages/merchant/delivery-promotions/edit/index?id=${promotionId}`
  }

  return '/pages/merchant/delivery-promotions/edit/index'
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
    refreshErrorMessage: '',
    loading: false,
    promotions: [] as PromotionView[],
    resultSummaryText: '当前共 0 条代取优惠',
    emptyDescription: '当前还没有代取优惠，先新增一个',
    merchantId: 0,
    lastLoadedAt: 0,
    needsReloadOnShow: false,
    deleteDialogVisible: false,
    deleteDialogSubmitting: false,
    deleteDialogPromotionId: 0,
    deleteDialogPromotionName: ''
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

    await this.loadPageData(true, true)
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
    void this.onLoad()
  },

  async onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      return
    }

    if (this.data.initialLoading || this.data.loading || this.data.deleteDialogSubmitting) {
      return
    }

    const needsReloadOnShow = !!this.data.needsReloadOnShow
    if (needsReloadOnShow) {
      this.setData({ needsReloadOnShow: false })
    }

    const merchantChanged = await this.syncMerchantContext()
    if (merchantChanged === null) {
      return
    }

    if (merchantChanged) {
      await this.loadPromotions(true, true)
      return
    }

    if (needsReloadOnShow && this.data.merchantId > 0) {
      await this.loadPromotions(false, true)
      return
    }

    if (this.data.merchantId > 0 && shouldAutoRefresh(this.data.lastLoadedAt, PROMOTIONS_AUTO_REFRESH_WINDOW_MS)) {
      await this.loadPromotions(false)
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }

    void this.loadPageData(false, true)
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) {
      return
    }

    void this.loadPageData(true, true)
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }

    void this.loadPageData(false, true)
  },

  async syncMerchantContext(): Promise<boolean | null> {
    try {
      const context = await syncCurrentMerchantContext({ currentMerchantId: this.data.merchantId })

      if (context.changed) {
        this.setData({
          merchantId: context.merchantId,
          lastLoadedAt: 0,
          initialLoading: true,
          initialError: false,
          initialErrorMessage: '',
          refreshErrorMessage: '',
          ...buildPresentationUpdate([]),
          needsReloadOnShow: false,
          deleteDialogVisible: false,
          deleteDialogSubmitting: false,
          deleteDialogPromotionId: 0,
          deleteDialogPromotionName: ''
        })
        return true
      }

      if (context.merchantId !== this.data.merchantId) {
        this.setData({ merchantId: context.merchantId })
      }

      return false
    } catch (err) {
      logger.error('Sync merchant delivery promotions context failed', err)
      const message = getErrorUserMessage(err, '获取商户信息失败，请重试')

      if (!this.data.lastLoadedAt && !this.data.promotions.length) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: ''
        })
      } else {
        this.setData({
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      }

      return null
    }
  },

  async loadPageData(showLoading = true, force = false) {
    const merchantChanged = await this.syncMerchantContext()
    if (merchantChanged === null) {
      wx.stopPullDownRefresh()
      return
    }

    if (!this.data.merchantId) {
      wx.stopPullDownRefresh()
      return
    }

    await this.loadPromotions(showLoading, force || merchantChanged)
  },

  async loadPromotions(showLoading = true, force = false) {
    if (this.data.loading || !this.data.merchantId) {
      wx.stopPullDownRefresh()
      return
    }

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
        ...buildPresentationUpdate(promotions),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err) {
      logger.error('Failed to load delivery promotions', err)
      const message = getErrorUserMessage(err, '加载代取优惠失败，请稍后重试')

      if (this.data.initialLoading || !hasConfirmedData) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: ''
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

  onAddPromotion() {
    if (this.data.deleteDialogSubmitting || this.data.loading) {
      return
    }

    this.setData({
      refreshErrorMessage: '',
      needsReloadOnShow: true
    })

    wx.navigateTo({
      url: buildEditPageUrl(),
      fail: (err) => {
        logger.error('Navigate to delivery promotion create page failed', err)
        this.setData({ needsReloadOnShow: false })
        wx.showToast({ title: '打开新建页失败，请稍后重试', icon: 'none' })
      }
    })
  },

  onEditPromotion(e: WechatMiniprogram.TouchEvent) {
    if (this.data.deleteDialogSubmitting || this.data.loading) {
      return
    }

    const { id } = e.currentTarget.dataset as { id: number }
    const promotion = this.data.promotions.find((item) => item.id === id)
    if (!promotion || promotion.statusPending || promotion.deletePending) {
      return
    }

    this.setData({
      refreshErrorMessage: '',
      needsReloadOnShow: true
    })

    wx.navigateTo({
      url: buildEditPageUrl(id),
      fail: (err) => {
        logger.error('Navigate to delivery promotion edit page failed', err)
        this.setData({ needsReloadOnShow: false })
        wx.showToast({ title: '打开编辑页失败，请稍后重试', icon: 'none' })
      }
    })
  },

  onActionsCatch() {},

  async onTogglePromoStatus(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) {
      return
    }

    const targetPromotion = this.data.promotions.find((item) => item.id === id)
    if (!targetPromotion || targetPromotion.statusPending || targetPromotion.deletePending) {
      return
    }

    const targetActive = !!e.detail?.value
    if (targetActive === targetPromotion.is_active) {
      return
    }

    const pendingPromotions = this.data.promotions.map((promotion) => (
      promotion.id === id ? { ...promotion, statusPending: true } : promotion
    ))

    this.setData(buildPresentationUpdate(pendingPromotions))
    try {
      const updatedPromotion = await deliveryFeeService.updateMerchantPromotion(this.data.merchantId, id, {
        is_active: targetActive
      })

      const nextPromotions = upsertPromotionView(pendingPromotions, updatedPromotion)
      this.setData({
        ...buildPresentationUpdate(nextPromotions),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      wx.showToast({ title: updatedPromotion.is_active ? '代取优惠已启用' : '代取优惠已停用', icon: 'none' })
    } catch (err) {
      logger.error('Toggle promo status failed', err)
      const restoredPromotions = pendingPromotions.map((promotion) => (
        promotion.id === id ? { ...promotion, statusPending: false } : promotion
      ))

      this.setData(buildPresentationUpdate(restoredPromotions))
      wx.showToast({ title: getErrorUserMessage(err, '更新状态失败，请稍后重试'), icon: 'none' })
    }
  },

  onRequestDeletePromotion(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) {
      return
    }

    const targetPromotion = this.data.promotions.find((item) => item.id === id)
    if (!targetPromotion || targetPromotion.deletePending) {
      return
    }

    this.setData({
      deleteDialogVisible: true,
      deleteDialogSubmitting: false,
      deleteDialogPromotionId: id,
      deleteDialogPromotionName: targetPromotion.name || '该优惠'
    })
  },

  onCancelDeleteDialog() {
    if (this.data.deleteDialogSubmitting) {
      return
    }

    this.setData({
      deleteDialogVisible: false,
      deleteDialogSubmitting: false,
      deleteDialogPromotionId: 0,
      deleteDialogPromotionName: ''
    })
  },

  async onConfirmDeletePromotion() {
    const id = Number(this.data.deleteDialogPromotionId || 0)
    if (!id) {
      this.onCancelDeleteDialog()
      return
    }

    const targetPromotion = this.data.promotions.find((item) => item.id === id)
    if (!targetPromotion || targetPromotion.deletePending) {
      this.onCancelDeleteDialog()
      return
    }

    this.setData({ deleteDialogSubmitting: true })

    const pendingPromotions = this.data.promotions.map((promotion) => (
      promotion.id === id ? { ...promotion, deletePending: true } : promotion
    ))
    this.setData(buildPresentationUpdate(pendingPromotions))

    try {
      await deliveryFeeService.deleteMerchantPromotion(this.data.merchantId, id)
      const nextPromotions = removePromotionView(pendingPromotions, id)

      this.setData({
        ...buildPresentationUpdate(nextPromotions),
        deleteDialogVisible: false,
        deleteDialogSubmitting: false,
        deleteDialogPromotionId: 0,
        deleteDialogPromotionName: '',
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      wx.showToast({ title: '代取优惠已删除', icon: 'none' })
    } catch (err) {
      logger.error('Delete promotion failed', err)
      const restoredPromotions = pendingPromotions.map((promotion) => (
        promotion.id === id ? { ...promotion, deletePending: false } : promotion
      ))

      this.setData({
        ...buildPresentationUpdate(restoredPromotions),
        deleteDialogSubmitting: false
      })
      wx.showToast({ title: getErrorUserMessage(err, '删除失败，请稍后重试'), icon: 'none' })
    }
  }
})