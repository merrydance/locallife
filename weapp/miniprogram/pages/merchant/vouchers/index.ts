import { getStableBarHeights } from '../../../utils/responsive'
import {
  MerchantVoucherService
} from '../_main_shared/api/coupon'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { syncCurrentMerchantContext } from '../_utils/current-merchant'
import { getErrorUserMessage } from '../../../utils/user-facing'
import {
  appendVoucherViews,
  buildVoucherEditPageUrl,
  buildVoucherPresentationUpdate,
  buildVoucherView,
  hasReliableTotal,
  normalizeTotal,
  removeVoucherView,
  resolveTargetPageCount,
  shouldAutoRefresh,
  upsertVoucherView,
  VOUCHERS_AUTO_REFRESH_WINDOW_MS,
  VOUCHERS_PAGE_SIZE,
  type VoucherPageChunk,
  type VoucherPageProbeResult,
  type VoucherView
} from '../_utils/merchant-vouchers-view'

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
          ...buildVoucherPresentationUpdate([])
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
        ...buildVoucherPresentationUpdate(result.vouchers),
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
        ...buildVoucherPresentationUpdate(mergedVouchers),
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
      url: buildVoucherEditPageUrl(),
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
      url: buildVoucherEditPageUrl(id),
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

    this.setData(buildVoucherPresentationUpdate(pendingVouchers))

    try {
      const updatedVoucher = await MerchantVoucherService.updateMerchantVoucher(this.data.merchantId, id, {
        is_active: targetActive
      })

      const nextVouchers = upsertVoucherView(pendingVouchers, updatedVoucher)
      this.setData({
        ...buildVoucherPresentationUpdate(nextVouchers),
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
      this.setData(buildVoucherPresentationUpdate(restoredVouchers))
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
      ...buildVoucherPresentationUpdate(pendingVouchers),
      deleteDialogSubmitting: true
    })

    try {
      await MerchantVoucherService.deleteMerchantVoucher(this.data.merchantId, voucherId)
      const nextVouchers = removeVoucherView(pendingVouchers, voucherId)
      this.setData({
        ...buildVoucherPresentationUpdate(nextVouchers),
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
        ...buildVoucherPresentationUpdate(restoredVouchers),
        deleteDialogSubmitting: false
      })
      wx.showToast({ title: getErrorUserMessage(err, '删除代金券失败，请稍后重试'), icon: 'none' })
    }
  }
})