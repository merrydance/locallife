import { getStableBarHeights } from '../../../utils/responsive'
import {
  buildMerchantVoucherStatusView,
  MerchantVoucherService,
  type MerchantVoucher
} from '../../../api/coupon'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { syncCurrentMerchantContext } from '../../../utils/current-merchant'
import { getErrorUserMessage } from '../../../utils/user-facing'

type OrderType = 'takeout' | 'dine_in' | 'takeaway' | 'reservation'

interface VoucherView extends MerchantVoucher {
  amount_yuan: string
  min_order_amount_yuan: string
  usage_condition_text: string
  valid_range_text: string
  remaining_quantity_text: string
  order_type_labels: string[]
  statusPending: boolean
  deletePending: boolean
}

interface VoucherPageChunk {
  vouchers: VoucherView[]
  pageId: number
  pageSize: number
  total?: number
  hasMore?: boolean
}

interface VoucherPageProbeResult {
  hasMore: boolean
  total: number
  nextPageCache: VoucherPageChunk | null
}

const VOUCHERS_PAGE_SIZE = 50
const VOUCHERS_AUTO_REFRESH_WINDOW_MS = 60 * 1000

const ALL_ORDER_TYPES: OrderType[] = ['takeout', 'dine_in', 'takeaway', 'reservation']

const ORDER_TYPE_LABEL_MAP: Record<OrderType, string> = {
  takeout: '外卖配送',
  dine_in: '堂食',
  takeaway: '到店自取',
  reservation: '预订'
}

function formatAmount(amount: number) {
  return (amount / 100).toFixed(2)
}

function formatDate(date: string) {
  return String(date || '').slice(0, 10)
}

function getOrderTypeLabels(orderTypes: string[]) {
  const normalized = (orderTypes.length ? orderTypes : ALL_ORDER_TYPES) as OrderType[]
  return normalized.map((item) => ORDER_TYPE_LABEL_MAP[item] || item)
}

function buildVoucherView(voucher: MerchantVoucher): VoucherView {
  const statusView = buildMerchantVoucherStatusView(voucher)
  const minOrderAmountYuan = formatAmount(voucher.min_order_amount)
  const remainingQuantity = Math.max(voucher.total_quantity - voucher.claimed_quantity, 0)

  return {
    ...voucher,
    status_code: statusView.code,
    status_label: statusView.label,
    status_theme: statusView.theme,
    amount_yuan: formatAmount(voucher.amount),
    min_order_amount_yuan: minOrderAmountYuan,
    usage_condition_text: voucher.min_order_amount > 0 ? `满 ¥${minOrderAmountYuan} 可用` : '不限门槛可用',
    valid_range_text: `${formatDate(voucher.valid_from)} 至 ${formatDate(voucher.valid_until)}`,
    remaining_quantity_text: `${remainingQuantity} / ${voucher.total_quantity}`,
    order_type_labels: getOrderTypeLabels(voucher.allowed_order_types),
    statusPending: false,
    deletePending: false
  }
}

function buildResultSummaryText(visibleCount: number) {
  return `当前已加载 ${visibleCount} 张代金券`
}

function buildPresentationUpdate(vouchers: VoucherView[]) {
  return {
    vouchers,
    resultSummaryText: buildResultSummaryText(vouchers.length),
    emptyDescription: '当前还没有代金券，先新增一个'
  }
}

function appendVoucherViews(existingVouchers: VoucherView[], incomingVouchers: VoucherView[]) {
  if (!incomingVouchers.length) {
    return existingVouchers
  }

  const merged = [...existingVouchers]
  const seen = new Set(existingVouchers.map((item) => item.id))

  incomingVouchers.forEach((voucher) => {
    if (seen.has(voucher.id)) {
      return
    }

    seen.add(voucher.id)
    merged.push(voucher)
  })

  return merged
}

function upsertVoucherView(vouchers: VoucherView[], voucher: MerchantVoucher) {
  const nextVoucher = buildVoucherView(voucher)
  const index = vouchers.findIndex((item) => item.id === nextVoucher.id)

  if (index === -1) {
    return [nextVoucher, ...vouchers]
  }

  const nextVouchers = [...vouchers]
  nextVouchers[index] = nextVoucher
  return nextVouchers
}

function removeVoucherView(vouchers: VoucherView[], voucherId: number) {
  return vouchers.filter((item) => item.id !== voucherId)
}

function hasReliableTotal(total: number | undefined, loadedCount: number) {
  return typeof total === 'number' && total > loadedCount
}

function normalizeTotal(total: number | undefined, fallback: number) {
  return typeof total === 'number' && total >= 0 ? total : fallback
}

function resolveTargetPageCount(pageId: number, pageSize: number, visibleCount: number) {
  const normalizedPageSize = pageSize > 0 ? pageSize : VOUCHERS_PAGE_SIZE
  const visiblePageCount = visibleCount > 0 ? Math.ceil(visibleCount / normalizedPageSize) : 1
  return Math.max(pageId || 0, visiblePageCount, 1)
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

function buildEditPageUrl(voucherId?: number) {
  if (voucherId && voucherId > 0) {
    return `/pages/merchant/vouchers/edit/index?id=${voucherId}`
  }

  return '/pages/merchant/vouchers/edit/index'
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
    loadingMore: false,
    vouchers: [] as VoucherView[],
    resultSummaryText: '当前已加载 0 张代金券',
    emptyDescription: '当前还没有代金券，先新增一个',
    merchantId: 0,
    lastLoadedAt: 0,
    pageId: 0,
    pageSize: VOUCHERS_PAGE_SIZE,
    total: 0,
    hasMore: false,
    lookaheadVouchers: [] as VoucherView[],
    lookaheadPageId: 0,
    lookaheadPageSize: VOUCHERS_PAGE_SIZE,
    lookaheadTotal: 0,
    needsReloadOnShow: false,
    deleteDialogVisible: false,
    deleteDialogSubmitting: false,
    deleteDialogVoucherId: 0,
    deleteDialogVoucherName: ''
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

    if (this.data.initialLoading || this.data.loading || this.data.loadingMore || this.data.deleteDialogSubmitting) {
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
      await this.loadVouchers(true, true)
      return
    }

    if (needsReloadOnShow && this.data.merchantId > 0) {
      await this.loadVouchers(false, true)
      return
    }

    if (this.data.merchantId > 0 && shouldAutoRefresh(this.data.lastLoadedAt, VOUCHERS_AUTO_REFRESH_WINDOW_MS)) {
      await this.loadVouchers(false)
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

  onReachBottom() {
    void this.onLoadMore()
  },

  async syncMerchantContext(): Promise<boolean | null> {
    try {
      const context = await syncCurrentMerchantContext({ currentMerchantId: this.data.merchantId })

      if (context.changed) {
        this.setData({
          merchantId: context.merchantId,
          lastLoadedAt: 0,
          pageId: 0,
          total: 0,
          hasMore: false,
          lookaheadVouchers: [],
          lookaheadPageId: 0,
          lookaheadPageSize: VOUCHERS_PAGE_SIZE,
          lookaheadTotal: 0,
          initialLoading: true,
          initialError: false,
          initialErrorMessage: '',
          refreshErrorMessage: '',
          needsReloadOnShow: false,
          deleteDialogVisible: false,
          deleteDialogSubmitting: false,
          deleteDialogVoucherId: 0,
          deleteDialogVoucherName: '',
          ...buildPresentationUpdate([])
        })
        return true
      }

      if (context.merchantId !== this.data.merchantId) {
        this.setData({ merchantId: context.merchantId })
      }

      return false
    } catch (err) {
      logger.error('Sync merchant vouchers context failed', err)
      const message = getErrorUserMessage(err, '获取商户信息失败，请重试')

      if (!this.data.lastLoadedAt && !this.data.vouchers.length) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: ''
        })
      } else {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
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

    await this.loadVouchers(showLoading, force || merchantChanged)
  },

  async fetchVoucherPage(pageNumber: number, requestedPageSize = VOUCHERS_PAGE_SIZE): Promise<VoucherPageChunk> {
    const response = await MerchantVoucherService.listMerchantVouchers(this.data.merchantId, pageNumber, requestedPageSize)
    return {
      vouchers: (Array.isArray(response.vouchers) ? response.vouchers : []).map(buildVoucherView),
      pageId: response.page || pageNumber,
      pageSize: response.pageSize || requestedPageSize || VOUCHERS_PAGE_SIZE,
      total: typeof response.total === 'number' && response.total >= 0 ? response.total : undefined,
      hasMore: response.hasMore
    }
  },

  getCachedVoucherPage(expectedPage: number): VoucherPageChunk | null {
    if (this.data.lookaheadPageId !== expectedPage || !this.data.lookaheadVouchers.length) {
      return null
    }

    return {
      vouchers: this.data.lookaheadVouchers,
      pageId: this.data.lookaheadPageId,
      pageSize: this.data.lookaheadPageSize || this.data.pageSize || VOUCHERS_PAGE_SIZE,
      total: this.data.lookaheadTotal > 0 ? this.data.lookaheadTotal : undefined
    }
  },

  async resolveVoucherPageBoundary(page: VoucherPageChunk, loadedCount: number): Promise<VoucherPageProbeResult> {
    const trustedTotal = hasReliableTotal(page.total, loadedCount) ? page.total : undefined
    if (typeof trustedTotal === 'number') {
      return {
        hasMore: loadedCount < trustedTotal,
        total: Math.max(trustedTotal, loadedCount),
        nextPageCache: null
      }
    }

    if (page.vouchers.length < page.pageSize) {
      return {
        hasMore: false,
        total: loadedCount,
        nextPageCache: null
      }
    }

    try {
      const nextPage = await this.fetchVoucherPage((page.pageId || 0) + 1, page.pageSize || VOUCHERS_PAGE_SIZE)

      if (!nextPage.vouchers.length) {
        return {
          hasMore: false,
          total: loadedCount,
          nextPageCache: null
        }
      }

      return {
        hasMore: true,
        total: Math.max(
          normalizeTotal(page.total, loadedCount),
          normalizeTotal(nextPage.total, loadedCount + nextPage.vouchers.length),
          loadedCount + nextPage.vouchers.length
        ),
        nextPageCache: nextPage
      }
    } catch (err) {
      logger.warn('Probe next merchant voucher page failed, fallback to conservative pagination', {
        err,
        currentPageId: page.pageId,
        loadedCount
      })

      return {
        hasMore: true,
        total: Math.max(normalizeTotal(page.total, loadedCount + 1), loadedCount + 1),
        nextPageCache: null
      }
    }
  },

  async fetchVoucherPages(targetPageCount: number, minimumVisibleCount = 0) {
    let mergedVouchers: VoucherView[] = []
    let resolvedTotal = 0
    let resolvedPageId = 0
    let resolvedPageSize = this.data.pageSize || VOUCHERS_PAGE_SIZE
    let hasMore = false
    let nextPageCache: VoucherPageChunk | null = null

    for (
      let currentPage = 1;
      currentPage <= targetPageCount || (minimumVisibleCount > 0 && mergedVouchers.length < minimumVisibleCount && hasMore);
      currentPage += 1
    ) {
      const page = nextPageCache?.pageId === currentPage
        ? nextPageCache
        : await this.fetchVoucherPage(currentPage, resolvedPageSize || VOUCHERS_PAGE_SIZE)

      nextPageCache = null
      mergedVouchers = mergedVouchers.concat(page.vouchers)
      resolvedPageId = page.pageId
      resolvedPageSize = page.pageSize

      const pagination = await this.resolveVoucherPageBoundary(page, mergedVouchers.length)
      resolvedTotal = pagination.total
      hasMore = pagination.hasMore
      nextPageCache = pagination.nextPageCache

      if (!page.vouchers.length || !hasMore) {
        break
      }
    }

    return {
      vouchers: mergedVouchers,
      pageId: resolvedPageId,
      pageSize: resolvedPageSize,
      total: Math.max(resolvedTotal, mergedVouchers.length),
      hasMore,
      nextPageCache
    }
  },

  async loadVouchers(showLoading = true, force = false) {
    if (this.data.loading || !this.data.merchantId) {
      wx.stopPullDownRefresh()
      return
    }

    const hasConfirmedData = this.data.vouchers.length > 0 || this.data.lastLoadedAt > 0
    if (!force && hasConfirmedData && !shouldAutoRefresh(this.data.lastLoadedAt, VOUCHERS_AUTO_REFRESH_WINDOW_MS)) {
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
      const minimumVisibleCount = this.data.vouchers.length
      const result = await this.fetchVoucherPages(
        resolveTargetPageCount(this.data.pageId, this.data.pageSize || VOUCHERS_PAGE_SIZE, minimumVisibleCount),
        minimumVisibleCount
      )

      this.setData({
        ...buildPresentationUpdate(result.vouchers),
        pageId: result.pageId,
        pageSize: result.pageSize,
        total: result.total,
        hasMore: result.hasMore,
        lookaheadVouchers: result.nextPageCache?.vouchers || [],
        lookaheadPageId: result.nextPageCache?.pageId || 0,
        lookaheadPageSize: result.nextPageCache?.pageSize || result.pageSize,
        lookaheadTotal: result.nextPageCache?.total || 0,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err) {
      logger.error('Load merchant vouchers failed', err)
      const message = getErrorUserMessage(err, '加载代金券失败，请稍后重试')

      if (this.data.initialLoading || !hasConfirmedData) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: ''
        })
      } else {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  async onLoadMore() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      return
    }

    if (this.data.loading || this.data.loadingMore || !this.data.merchantId || !this.data.hasMore || this.data.deleteDialogSubmitting) {
      return
    }

    this.setData({ loadingMore: true })

    try {
      const nextPage = Math.max(this.data.pageId, 0) + 1
      const nextChunk = this.getCachedVoucherPage(nextPage)
        || await this.fetchVoucherPage(nextPage, this.data.pageSize || VOUCHERS_PAGE_SIZE)
      const mergedVouchers = appendVoucherViews(this.data.vouchers, nextChunk.vouchers)
      const pagination = await this.resolveVoucherPageBoundary(nextChunk, mergedVouchers.length)

      this.setData({
        ...buildPresentationUpdate(mergedVouchers),
        pageId: nextChunk.pageId,
        pageSize: nextChunk.pageSize,
        total: Math.max(pagination.total, mergedVouchers.length),
        hasMore: pagination.hasMore,
        lookaheadVouchers: pagination.nextPageCache?.vouchers || [],
        lookaheadPageId: pagination.nextPageCache?.pageId || 0,
        lookaheadPageSize: pagination.nextPageCache?.pageSize || nextChunk.pageSize,
        lookaheadTotal: pagination.nextPageCache?.total || 0,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err) {
      logger.error('Load more merchant vouchers failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '加载更多失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ loadingMore: false })
    }
  },

  onAddVoucher() {
    if (this.data.deleteDialogSubmitting || this.data.loading) {
      return
    }

    this.setData({ refreshErrorMessage: '', needsReloadOnShow: true })

    wx.navigateTo({
      url: buildEditPageUrl(),
      fail: (err) => {
        logger.error('Navigate to voucher create page failed', err)
        this.setData({ needsReloadOnShow: false })
        wx.showToast({ title: '打开新建页失败，请稍后重试', icon: 'none' })
      }
    })
  },

  onEditVoucher(e: WechatMiniprogram.TouchEvent) {
    if (this.data.deleteDialogSubmitting || this.data.loading) {
      return
    }

    const id = Number((e.currentTarget.dataset as { id?: number | string }).id || 0)
    const voucher = this.data.vouchers.find((item) => item.id === id)
    if (!voucher || voucher.statusPending || voucher.deletePending) {
      return
    }

    this.setData({ refreshErrorMessage: '', needsReloadOnShow: true })

    wx.navigateTo({
      url: buildEditPageUrl(id),
      fail: (err) => {
        logger.error('Navigate to voucher edit page failed', err)
        this.setData({ needsReloadOnShow: false })
        wx.showToast({ title: '打开编辑页失败，请稍后重试', icon: 'none' })
      }
    })
  },

  onActionsCatch() {},

  async onToggleVoucherStatus(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const id = Number((e.currentTarget.dataset as { id?: number | string }).id || 0)
    if (!id) {
      return
    }

    const targetVoucher = this.data.vouchers.find((item) => item.id === id)
    if (!targetVoucher || targetVoucher.statusPending || targetVoucher.deletePending) {
      return
    }

    const targetActive = !!e.detail?.value
    if (targetActive === targetVoucher.is_active) {
      return
    }

    const pendingVouchers = this.data.vouchers.map((voucher) => (
      voucher.id === id ? { ...voucher, statusPending: true } : voucher
    ))

    this.setData(buildPresentationUpdate(pendingVouchers))

    try {
      const updatedVoucher = await MerchantVoucherService.updateMerchantVoucher(this.data.merchantId, id, {
        is_active: targetActive
      })

      const nextVouchers = upsertVoucherView(pendingVouchers, updatedVoucher)
      this.setData({
        ...buildPresentationUpdate(nextVouchers),
        total: Math.max(this.data.total, nextVouchers.length),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      wx.showToast({ title: updatedVoucher.is_active ? '代金券已启用' : '代金券已停用', icon: 'none' })
    } catch (err) {
      logger.error('Toggle merchant voucher status failed', err)
      const restoredVouchers = pendingVouchers.map((voucher) => (
        voucher.id === id ? { ...targetVoucher, statusPending: false } : voucher
      ))
      this.setData(buildPresentationUpdate(restoredVouchers))
      wx.showToast({ title: getErrorUserMessage(err, '更新状态失败，请稍后重试'), icon: 'none' })
    }
  },

  onRequestDeleteVoucher(e: WechatMiniprogram.TouchEvent) {
    const dataset = e.currentTarget.dataset as { id?: number | string, name?: string }
    const id = Number(dataset.id || 0)
    if (!id || this.data.deleteDialogSubmitting) {
      return
    }

    const voucher = this.data.vouchers.find((item) => item.id === id)
    if (!voucher || voucher.statusPending || voucher.deletePending) {
      return
    }

    this.setData({
      deleteDialogVisible: true,
      deleteDialogSubmitting: false,
      deleteDialogVoucherId: id,
      deleteDialogVoucherName: dataset.name || voucher.name || ''
    })
  },

  onCancelDeleteDialog() {
    if (this.data.deleteDialogSubmitting) {
      return
    }

    this.setData({
      deleteDialogVisible: false,
      deleteDialogVoucherId: 0,
      deleteDialogVoucherName: ''
    })
  },

  async onConfirmDeleteVoucher() {
    if (this.data.deleteDialogSubmitting || !this.data.deleteDialogVoucherId) {
      return
    }

    const voucherId = this.data.deleteDialogVoucherId
    const pendingVouchers = this.data.vouchers.map((voucher) => (
      voucher.id === voucherId ? { ...voucher, deletePending: true } : voucher
    ))

    this.setData({
      ...buildPresentationUpdate(pendingVouchers),
      deleteDialogSubmitting: true
    })

    try {
      await MerchantVoucherService.deleteMerchantVoucher(this.data.merchantId, voucherId)
      const nextVouchers = removeVoucherView(pendingVouchers, voucherId)
      this.setData({
        ...buildPresentationUpdate(nextVouchers),
        total: Math.max(this.data.total - 1, nextVouchers.length, 0),
        deleteDialogVisible: false,
        deleteDialogSubmitting: false,
        deleteDialogVoucherId: 0,
        deleteDialogVoucherName: '',
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      wx.showToast({ title: '代金券已删除', icon: 'none' })
      void this.loadVouchers(false, true)
    } catch (err) {
      logger.error('Delete merchant voucher failed', err)
      const restoredVouchers = pendingVouchers.map((voucher) => (
        voucher.id === voucherId ? { ...voucher, deletePending: false } : voucher
      ))
      this.setData({
        ...buildPresentationUpdate(restoredVouchers),
        deleteDialogSubmitting: false
      })
      wx.showToast({ title: getErrorUserMessage(err, '删除代金券失败，请稍后重试'), icon: 'none' })
    }
  }
})