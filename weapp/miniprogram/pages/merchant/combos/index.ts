import { getStableBarHeights } from '../../../utils/responsive'
import { ComboManagementService, ComboSetResponse } from '../_main_shared/api/dish'
import { getPublicImageUrl } from '../../../utils/image'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import {
  ensureMerchantConsoleAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../utils/console-access'

type ComboFilterKey = 'all' | 'online' | 'offline'

interface ComboViewItem extends ComboSetResponse {
  coverImageUrl: string
  imageCount: number
  savingsAmount: number
  hasDiscount: boolean
  statusPending: boolean
  deletePending: boolean
}

interface ComboFilterOption {
  key: ComboFilterKey
  label: string
}

const COMBO_FILTER_OPTIONS: ComboFilterOption[] = [
  { key: 'all', label: '全部' },
  { key: 'online', label: '已上架' },
  { key: 'offline', label: '未上架' }
]

const COMBO_AUTO_REFRESH_WINDOW_MS = 60 * 1000
const getErrorMessage = getErrorUserMessage

function normalizeComboImages(urls?: string[]): string[] {
  if (!Array.isArray(urls)) return []
  return urls.map((url) => getPublicImageUrl(url)).filter((url) => !!url)
}

function buildComboViewItem(combo: ComboSetResponse): ComboViewItem {
  const imageUrls = normalizeComboImages(combo.dish_image_urls)

  return {
    ...combo,
    dish_image_urls: imageUrls,
    coverImageUrl: imageUrls[0] || '',
    imageCount: imageUrls.length,
    savingsAmount: Math.max((combo.original_price || 0) - (combo.combo_price || 0), 0),
    hasDiscount: (combo.original_price || 0) > (combo.combo_price || 0),
    statusPending: false,
    deletePending: false
  }
}

function buildResultSummaryText(params: { visibleCount: number, currentFilter: ComboFilterKey }) {
  if (params.currentFilter === 'all') {
    return `当前共 ${params.visibleCount} 套套餐`
  }

  return `${params.currentFilter === 'online' ? '已上架' : '未上架'}下共 ${params.visibleCount} 套套餐`
}

function buildEmptyDescription(currentFilter: ComboFilterKey) {
  if (currentFilter === 'all') {
    return '还没有套餐，先新增一个'
  }

  return '暂无符合当前筛选条件的套餐'
}

function buildPresentationUpdate(combos: ComboViewItem[], currentFilter: ComboFilterKey) {
  return {
    combos,
    resultSummaryText: buildResultSummaryText({
      visibleCount: combos.length,
      currentFilter
    }),
    emptyDescription: buildEmptyDescription(currentFilter)
  }
}

function resolveHasMore(total: number | undefined, loadedCount: number, pageSize: number, lastPageLength: number) {
  if (typeof total === 'number' && total >= 0) {
    return loadedCount < total
  }

  return lastPageLength >= pageSize
}

interface LoadCombosOptions {
  showLoading?: boolean
  preserveCurrent?: boolean
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
    filterOptions: COMBO_FILTER_OPTIONS,
    currentFilter: 'all' as ComboFilterKey,
    combos: [] as ComboViewItem[],
    resultSummaryText: '当前共 0 套套餐',
    emptyDescription: '还没有套餐，先新增一个',
    pageId: 1,
    pageSize: 20,
    hasMore: true,
    lastLoadedAt: 0,
    needsReloadOnShow: false,
    deleteDialogVisible: false,
    deleteDialogSubmitting: false,
    deleteDialogComboId: 0,
    deleteDialogComboName: ''
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.initializePage()
  },

  onShow() {
    if (
      !this.data.accessReady ||
      this.data.accessDenied ||
      this.data.accessErrorMessage ||
      this.data.initialLoading ||
      this.data.loading
    ) {
      return
    }

    const shouldRefresh =
      this.data.needsReloadOnShow ||
      Date.now() - this.data.lastLoadedAt >= COMBO_AUTO_REFRESH_WINDOW_MS

    if (!shouldRefresh) {
      return
    }

    const preserveCurrent = this.data.combos.length > 0
    this.setData({ needsReloadOnShow: false })
    void this.loadCombos(true, {
      showLoading: !preserveCurrent,
      preserveCurrent
    })
  },

  async initializePage() {
    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: isMerchantConsoleAccessDenied(accessResult),
      accessErrorMessage: getMerchantConsoleAccessErrorMessage(accessResult)
    })

    if (!isMerchantConsoleAccessGranted(accessResult)) {
      this.setData({ initialLoading: false, loading: false })
      wx.stopPullDownRefresh()
      return
    }

    await this.loadCombos(true)
  },

  async loadCombos(reset = false, options?: LoadCombosOptions) {
    if (this.data.loading) return
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (!reset && !this.data.hasMore) return

    const hasExistingCombos = this.data.combos.length > 0
    const preserveCurrent = !!options?.preserveCurrent && reset && hasExistingCombos
    const showLoading = options?.showLoading !== false
    const usePageLoading = reset && showLoading && !hasExistingCombos

    this.setData({
      loading: true,
      ...(usePageLoading
        ? {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          }
        : showLoading
          ? {
              initialError: false,
              initialErrorMessage: '',
              refreshErrorMessage: ''
            }
          : preserveCurrent
            ? { refreshErrorMessage: '' }
            : {})
    })

    try {
      const pageId = reset ? 1 : this.data.pageId
      const isOnline = this.data.currentFilter === 'all'
        ? undefined
        : this.data.currentFilter === 'online'
          ? true
          : false
      const response = await ComboManagementService.listCombos({
        page_id: pageId,
        page_size: this.data.pageSize,
        ...(typeof isOnline === 'boolean' ? { is_online: isOnline } : {})
      })

      const combos = Array.isArray(response?.combo_sets)
        ? response.combo_sets.filter((combo): combo is ComboSetResponse => !!combo).map(buildComboViewItem)
        : []

      const nextCombos = reset ? combos : [...this.data.combos, ...combos]
      const total = typeof response?.total === 'number' ? response.total : undefined

      this.setData({
        ...buildPresentationUpdate(nextCombos, this.data.currentFilter),
        pageId: pageId + 1,
        hasMore: resolveHasMore(total, nextCombos.length, this.data.pageSize, combos.length),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err) {
      logger.error('Load combos failed', err)
      const message = getErrorMessage(err, '加载套餐失败，请稍后重试')

      if (this.data.initialLoading || (!hasExistingCombos && reset)) {
        this.setData({
          ...buildPresentationUpdate([], this.data.currentFilter),
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: ''
        })
      } else if (hasExistingCombos) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      void this.onRetryAccess()
      return
    }

    void this.loadCombos(true, {
      showLoading: false,
      preserveCurrent: this.data.combos.length > 0
    })
  },

  onSelectFilter(e: WechatMiniprogram.TouchEvent) {
    const { key } = e.currentTarget.dataset as { key?: ComboFilterKey }
    if (!key || key === this.data.currentFilter) return

    this.setData(
      {
        currentFilter: key,
        pageId: 1,
        hasMore: true
      },
      () => {
        void this.loadCombos(true, {
          showLoading: this.data.combos.length === 0,
          preserveCurrent: this.data.combos.length > 0
        })
      }
    )
  },

  onReachBottom() {
    this.loadCombos()
  },

  async onToggleOnline(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    const targetCombo = this.data.combos.find((combo) => combo.id === id)
    if (!targetCombo || targetCombo.statusPending || targetCombo.deletePending) return

    const targetStatus = !!e.detail?.value

    if (targetStatus === targetCombo.is_online) return

    const pendingCombos = this.data.combos.map((combo) => (
      combo.id === id ? { ...combo, statusPending: true } : combo
    ))

    this.setData(buildPresentationUpdate(pendingCombos, this.data.currentFilter))

    try {
      const updated = await ComboManagementService.updateComboOnlineStatus(id, {
        is_online: targetStatus
      })
      if (this.data.currentFilter !== 'all') {
        void this.loadCombos(true, {
          showLoading: false,
          preserveCurrent: this.data.combos.length > 0
        })
      } else {
        const nextCombos = pendingCombos.map((combo) => (
          combo.id === id
            ? { ...combo, is_online: updated.is_online, statusPending: false }
            : combo
        ))

        this.setData(buildPresentationUpdate(nextCombos, this.data.currentFilter))
      }
    } catch (err) {
      logger.error('Toggle combo status failed', err)
      const restoredCombos = pendingCombos.map((combo) => (
        combo.id === id ? { ...combo, statusPending: false } : combo
      ))

      this.setData(buildPresentationUpdate(restoredCombos, this.data.currentFilter))
      wx.showToast({ title: getErrorMessage(err, '操作失败，请稍后重试'), icon: 'none' })
    }
  },

  onActionsCatch() {},

  onComboImageError(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    const nextCombos = this.data.combos.map((combo) => {
      if (combo.id !== id || combo.coverImageUrl === '/assets/icons/empty.svg') {
        return combo
      }

      return {
        ...combo,
        coverImageUrl: '/assets/icons/empty.svg'
      }
    })

    this.setData(buildPresentationUpdate(nextCombos, this.data.currentFilter))
  },

  onEditCombo(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    this.setData({ needsReloadOnShow: true })
    wx.navigateTo({ url: `/pages/merchant/combos/edit/index?id=${id}` })
  },

  onRequestDeleteCombo(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    const targetCombo = this.data.combos.find((combo) => combo.id === id)
    if (!targetCombo || targetCombo.deletePending) return

    this.setData({
      deleteDialogVisible: true,
      deleteDialogSubmitting: false,
      deleteDialogComboId: id,
      deleteDialogComboName: targetCombo.name || '该套餐'
    })
  },

  onCancelDeleteDialog() {
    if (this.data.deleteDialogSubmitting) return

    this.setData({
      deleteDialogVisible: false,
      deleteDialogSubmitting: false,
      deleteDialogComboId: 0,
      deleteDialogComboName: ''
    })
  },

  async onConfirmDeleteCombo() {
    const id = Number(this.data.deleteDialogComboId || 0)
    if (!id) {
      this.onCancelDeleteDialog()
      return
    }

    const targetCombo = this.data.combos.find((combo) => combo.id === id)
    if (!targetCombo) {
      this.onCancelDeleteDialog()
      return
    }

    const pendingCombos = this.data.combos.map((combo) => (
      combo.id === id ? { ...combo, deletePending: true } : combo
    ))

    this.setData({
      deleteDialogSubmitting: true,
      ...buildPresentationUpdate(pendingCombos, this.data.currentFilter)
    })

    try {
      await ComboManagementService.deleteCombo(id)
      const nextCombos = pendingCombos.filter((combo) => combo.id !== id)

      this.setData({
        deleteDialogVisible: false,
        deleteDialogSubmitting: false,
        deleteDialogComboId: 0,
        deleteDialogComboName: '',
        ...buildPresentationUpdate(nextCombos, this.data.currentFilter)
      })
    } catch (err) {
      logger.error('Delete combo failed', err)
      const restoredCombos = pendingCombos.map((combo) => (
        combo.id === id ? { ...combo, deletePending: false } : combo
      ))

      this.setData({
        deleteDialogSubmitting: false,
        ...buildPresentationUpdate(restoredCombos, this.data.currentFilter)
      })
      wx.showToast({ title: getErrorMessage(err, '删除失败，请稍后重试'), icon: 'none' })
    }
  },

  onRetry() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      void this.onRetryAccess()
      return
    }

    void this.loadCombos(true)
  },

  async onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: '',
      loading: false,
      ...buildPresentationUpdate([], this.data.currentFilter),
      pageId: 1,
      hasMore: true,
      lastLoadedAt: 0,
      deleteDialogVisible: false,
      deleteDialogSubmitting: false,
      deleteDialogComboId: 0,
      deleteDialogComboName: ''
    })

    await this.initializePage()
  },

  onRetryRefresh() {
    void this.loadCombos(true, {
      showLoading: false,
      preserveCurrent: this.data.combos.length > 0
    })
  },

  onCreateCombo() {
    this.setData({ needsReloadOnShow: true })
    wx.navigateTo({ url: '/pages/merchant/combos/edit/index' })
  }
})
