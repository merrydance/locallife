import { getStableBarHeights } from '../../../utils/responsive'
import {
  buildMerchantDiscountRuleStatusView,
  deleteMerchantDiscountRule,
  listMerchantDiscountRules,
  type MerchantDiscountRuleResponse,
  updateMerchantDiscountRule
} from '../../../api/merchant'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { syncCurrentMerchantContext } from '../../../utils/current-merchant'
import { getErrorUserMessage } from '../../../utils/user-facing'

interface DiscountRuleView extends MerchantDiscountRuleResponse {
  min_order_amount_yuan: string
  discount_amount_yuan: string
  valid_range_text: string
  stacking_text: string
  statusPending: boolean
  deletePending: boolean
}

interface DiscountRulePageChunk {
  rules: DiscountRuleView[]
  pageId: number
  pageSize: number
  total?: number
}

interface DiscountRulePageProbeResult {
  hasMore: boolean
  total: number
  nextPageCache: DiscountRulePageChunk | null
}

const DISCOUNT_RULES_PAGE_SIZE = 50
const DISCOUNT_RULES_AUTO_REFRESH_WINDOW_MS = 60 * 1000

function formatAmount(amount: number) {
  return (amount / 100).toFixed(2)
}

function formatDate(date: string) {
  return String(date || '').slice(0, 10)
}

function buildStackingText(rule: MerchantDiscountRuleResponse) {
  const parts: string[] = []

  if (rule.can_stack_with_voucher) {
    parts.push('可叠加代金券')
  }
  if (rule.can_stack_with_membership) {
    parts.push('可叠加会员权益')
  }
  if (rule.stacking_group) {
    parts.push(`分组 ${rule.stacking_group}`)
  }

  return parts.length ? parts.join(' · ') : '默认不与其他优惠叠加'
}

function buildRuleView(rule: MerchantDiscountRuleResponse): DiscountRuleView {
  const statusView = buildMerchantDiscountRuleStatusView(rule)

  return {
    ...rule,
    status_code: statusView.code,
    status_label: statusView.label,
    status_theme: statusView.theme,
    min_order_amount_yuan: formatAmount(rule.min_order_amount),
    discount_amount_yuan: formatAmount(rule.discount_amount),
    valid_range_text: `${formatDate(rule.valid_from)} 至 ${formatDate(rule.valid_until)}`,
    stacking_text: buildStackingText(rule),
    statusPending: false,
    deletePending: false
  }
}

function buildResultSummaryText(visibleCount: number) {
  return `当前已加载 ${visibleCount} 条满减规则`
}

function buildPresentationUpdate(rules: DiscountRuleView[]) {
  return {
    rules,
    resultSummaryText: buildResultSummaryText(rules.length),
    emptyDescription: '当前还没有满减规则，先新增一个'
  }
}

function appendRuleViews(existingRules: DiscountRuleView[], incomingRules: DiscountRuleView[]) {
  if (!incomingRules.length) {
    return existingRules
  }

  const merged = [...existingRules]
  const seen = new Set(existingRules.map((item) => item.id))

  incomingRules.forEach((rule) => {
    if (seen.has(rule.id)) {
      return
    }

    seen.add(rule.id)
    merged.push(rule)
  })

  return merged
}

function upsertRuleView(rules: DiscountRuleView[], rule: MerchantDiscountRuleResponse) {
  const nextRule = buildRuleView(rule)
  const index = rules.findIndex((item) => item.id === nextRule.id)

  if (index === -1) {
    return [nextRule, ...rules]
  }

  const nextRules = [...rules]
  nextRules[index] = nextRule
  return nextRules
}

function removeRuleView(rules: DiscountRuleView[], ruleId: number) {
  return rules.filter((item) => item.id !== ruleId)
}

function hasReliableTotal(total: number | undefined, loadedCount: number) {
  return typeof total === 'number' && total > loadedCount
}

function normalizeTotal(total: number | undefined, fallback: number) {
  return typeof total === 'number' && total >= 0 ? total : fallback
}

function resolveTargetPageCount(pageId: number, pageSize: number, visibleCount: number) {
  const normalizedPageSize = pageSize > 0 ? pageSize : DISCOUNT_RULES_PAGE_SIZE
  const visiblePageCount = visibleCount > 0 ? Math.ceil(visibleCount / normalizedPageSize) : 1
  return Math.max(pageId || 0, visiblePageCount, 1)
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

function buildEditPageUrl(ruleId?: number) {
  if (ruleId && ruleId > 0) {
    return `/pages/merchant/discount-rules/edit/index?id=${ruleId}`
  }

  return '/pages/merchant/discount-rules/edit/index'
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
    rules: [] as DiscountRuleView[],
    resultSummaryText: '当前已加载 0 条满减规则',
    emptyDescription: '当前还没有满减规则，先新增一个',
    merchantId: 0,
    lastLoadedAt: 0,
    pageId: 0,
    pageSize: DISCOUNT_RULES_PAGE_SIZE,
    total: 0,
    hasMore: false,
    lookaheadRules: [] as DiscountRuleView[],
    lookaheadPageId: 0,
    lookaheadPageSize: DISCOUNT_RULES_PAGE_SIZE,
    lookaheadTotal: 0,
    needsReloadOnShow: false,
    deleteDialogVisible: false,
    deleteDialogSubmitting: false,
    deleteDialogRuleId: 0,
    deleteDialogRuleName: ''
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
      await this.loadRules(true, true)
      return
    }

    if (needsReloadOnShow && this.data.merchantId > 0) {
      await this.loadRules(false, true)
      return
    }

    if (this.data.merchantId > 0 && shouldAutoRefresh(this.data.lastLoadedAt, DISCOUNT_RULES_AUTO_REFRESH_WINDOW_MS)) {
      await this.loadRules(false)
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
          lookaheadRules: [],
          lookaheadPageId: 0,
          lookaheadPageSize: DISCOUNT_RULES_PAGE_SIZE,
          lookaheadTotal: 0,
          initialLoading: true,
          initialError: false,
          initialErrorMessage: '',
          refreshErrorMessage: '',
          needsReloadOnShow: false,
          deleteDialogVisible: false,
          deleteDialogSubmitting: false,
          deleteDialogRuleId: 0,
          deleteDialogRuleName: '',
          ...buildPresentationUpdate([])
        })
        return true
      }

      if (context.merchantId !== this.data.merchantId) {
        this.setData({ merchantId: context.merchantId })
      }

      return false
    } catch (err) {
      logger.error('Sync merchant discount rules context failed', err)
      const message = getErrorUserMessage(err, '获取商户信息失败，请重试')

      if (!this.data.lastLoadedAt && !this.data.rules.length) {
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

    await this.loadRules(showLoading, force || merchantChanged)
  },

  async fetchRulePage(pageNumber: number, requestedPageSize = DISCOUNT_RULES_PAGE_SIZE): Promise<DiscountRulePageChunk> {
    const response = await listMerchantDiscountRules(this.data.merchantId, pageNumber, requestedPageSize)
    return {
      rules: (Array.isArray(response.rules) ? response.rules : []).map(buildRuleView),
      pageId: response.page_id || pageNumber,
      pageSize: response.page_size || requestedPageSize || DISCOUNT_RULES_PAGE_SIZE,
      total: typeof response.total === 'number' && response.total >= 0 ? response.total : undefined
    }
  },

  getCachedRulePage(expectedPage: number): DiscountRulePageChunk | null {
    if (this.data.lookaheadPageId !== expectedPage || !this.data.lookaheadRules.length) {
      return null
    }

    return {
      rules: this.data.lookaheadRules,
      pageId: this.data.lookaheadPageId,
      pageSize: this.data.lookaheadPageSize || this.data.pageSize || DISCOUNT_RULES_PAGE_SIZE,
      total: this.data.lookaheadTotal > 0 ? this.data.lookaheadTotal : undefined
    }
  },

  async resolveRulePageBoundary(page: DiscountRulePageChunk, loadedCount: number): Promise<DiscountRulePageProbeResult> {
    const trustedTotal = hasReliableTotal(page.total, loadedCount) ? page.total : undefined
    if (typeof trustedTotal === 'number') {
      return {
        hasMore: loadedCount < trustedTotal,
        total: Math.max(trustedTotal, loadedCount),
        nextPageCache: null
      }
    }

    if (page.rules.length < page.pageSize) {
      return {
        hasMore: false,
        total: loadedCount,
        nextPageCache: null
      }
    }

    try {
      const nextPage = await this.fetchRulePage((page.pageId || 0) + 1, page.pageSize || DISCOUNT_RULES_PAGE_SIZE)

      if (!nextPage.rules.length) {
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
          normalizeTotal(nextPage.total, loadedCount + nextPage.rules.length),
          loadedCount + nextPage.rules.length
        ),
        nextPageCache: nextPage
      }
    } catch (err) {
      logger.warn('Probe next merchant discount rule page failed, fallback to conservative pagination', {
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

  async fetchRulePages(targetPageCount: number, minimumVisibleCount = 0) {
    let mergedRules: DiscountRuleView[] = []
    let resolvedTotal = 0
    let resolvedPageId = 0
    let resolvedPageSize = this.data.pageSize || DISCOUNT_RULES_PAGE_SIZE
    let hasMore = false
    let nextPageCache: DiscountRulePageChunk | null = null

    for (
      let currentPage = 1;
      currentPage <= targetPageCount || (minimumVisibleCount > 0 && mergedRules.length < minimumVisibleCount && hasMore);
      currentPage += 1
    ) {
      const page = nextPageCache?.pageId === currentPage
        ? nextPageCache
        : await this.fetchRulePage(currentPage, resolvedPageSize || DISCOUNT_RULES_PAGE_SIZE)

      nextPageCache = null
      mergedRules = mergedRules.concat(page.rules)
      resolvedPageId = page.pageId
      resolvedPageSize = page.pageSize

      const pagination = await this.resolveRulePageBoundary(page, mergedRules.length)
      resolvedTotal = pagination.total
      hasMore = pagination.hasMore
      nextPageCache = pagination.nextPageCache

      if (!page.rules.length || !hasMore) {
        break
      }
    }

    return {
      rules: mergedRules,
      pageId: resolvedPageId,
      pageSize: resolvedPageSize,
      total: Math.max(resolvedTotal, mergedRules.length),
      hasMore,
      nextPageCache
    }
  },

  async loadRules(showLoading = true, force = false) {
    if (this.data.loading || !this.data.merchantId) {
      wx.stopPullDownRefresh()
      return
    }

    const hasConfirmedData = this.data.rules.length > 0 || this.data.lastLoadedAt > 0
    if (!force && hasConfirmedData && !shouldAutoRefresh(this.data.lastLoadedAt, DISCOUNT_RULES_AUTO_REFRESH_WINDOW_MS)) {
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
      const minimumVisibleCount = this.data.rules.length
      const result = await this.fetchRulePages(
        resolveTargetPageCount(this.data.pageId, this.data.pageSize || DISCOUNT_RULES_PAGE_SIZE, minimumVisibleCount),
        minimumVisibleCount
      )

      this.setData({
        ...buildPresentationUpdate(result.rules),
        pageId: result.pageId,
        pageSize: result.pageSize,
        total: result.total,
        hasMore: result.hasMore,
        lookaheadRules: result.nextPageCache?.rules || [],
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
      logger.error('Load merchant discount rules failed', err)
      const message = getErrorUserMessage(err, '加载满减规则失败，请稍后重试')

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
      const nextChunk = this.getCachedRulePage(nextPage)
        || await this.fetchRulePage(nextPage, this.data.pageSize || DISCOUNT_RULES_PAGE_SIZE)
      const mergedRules = appendRuleViews(this.data.rules, nextChunk.rules)
      const pagination = await this.resolveRulePageBoundary(nextChunk, mergedRules.length)

      this.setData({
        ...buildPresentationUpdate(mergedRules),
        pageId: nextChunk.pageId,
        pageSize: nextChunk.pageSize,
        total: Math.max(pagination.total, mergedRules.length),
        hasMore: pagination.hasMore,
        lookaheadRules: pagination.nextPageCache?.rules || [],
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
      logger.error('Load more merchant discount rules failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '加载更多失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ loadingMore: false })
    }
  },

  onAddRule() {
    if (this.data.deleteDialogSubmitting || this.data.loading) {
      return
    }

    this.setData({ refreshErrorMessage: '', needsReloadOnShow: true })

    wx.navigateTo({
      url: buildEditPageUrl(),
      fail: (err) => {
        logger.error('Navigate to discount rule create page failed', err)
        this.setData({ needsReloadOnShow: false })
        wx.showToast({ title: '打开新建页失败，请稍后重试', icon: 'none' })
      }
    })
  },

  onEditRule(e: WechatMiniprogram.TouchEvent) {
    if (this.data.deleteDialogSubmitting || this.data.loading) {
      return
    }

    const id = Number((e.currentTarget.dataset as { id?: number | string }).id || 0)
    const rule = this.data.rules.find((item) => item.id === id)
    if (!rule || rule.statusPending || rule.deletePending) {
      return
    }

    this.setData({ refreshErrorMessage: '', needsReloadOnShow: true })

    wx.navigateTo({
      url: buildEditPageUrl(id),
      fail: (err) => {
        logger.error('Navigate to discount rule edit page failed', err)
        this.setData({ needsReloadOnShow: false })
        wx.showToast({ title: '打开编辑页失败，请稍后重试', icon: 'none' })
      }
    })
  },

  onActionsCatch() {},

  async onToggleRuleStatus(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const id = Number((e.currentTarget.dataset as { id?: number | string }).id || 0)
    if (!id) {
      return
    }

    const targetRule = this.data.rules.find((item) => item.id === id)
    if (!targetRule || targetRule.statusPending || targetRule.deletePending) {
      return
    }

    const targetActive = !!e.detail?.value
    if (targetActive === targetRule.is_active) {
      return
    }

    const pendingRules = this.data.rules.map((rule) => (
      rule.id === id ? { ...rule, statusPending: true } : rule
    ))

    this.setData(buildPresentationUpdate(pendingRules))

    try {
      const updatedRule = await updateMerchantDiscountRule(this.data.merchantId, id, {
        is_active: targetActive
      })

      const nextRules = upsertRuleView(pendingRules, updatedRule)
      this.setData({
        ...buildPresentationUpdate(nextRules),
        total: Math.max(this.data.total, nextRules.length),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      wx.showToast({ title: updatedRule.is_active ? '满减规则已启用' : '满减规则已停用', icon: 'none' })
    } catch (err) {
      logger.error('Toggle merchant discount rule status failed', err)
      const restoredRules = pendingRules.map((rule) => (
        rule.id === id ? { ...targetRule, statusPending: false } : rule
      ))
      this.setData(buildPresentationUpdate(restoredRules))
      wx.showToast({ title: getErrorUserMessage(err, '更新状态失败，请稍后重试'), icon: 'none' })
    }
  },

  onRequestDeleteRule(e: WechatMiniprogram.TouchEvent) {
    const dataset = e.currentTarget.dataset as { id?: number | string, name?: string }
    const id = Number(dataset.id || 0)
    if (!id || this.data.deleteDialogSubmitting) {
      return
    }

    const rule = this.data.rules.find((item) => item.id === id)
    if (!rule || rule.statusPending || rule.deletePending) {
      return
    }

    this.setData({
      deleteDialogVisible: true,
      deleteDialogSubmitting: false,
      deleteDialogRuleId: id,
      deleteDialogRuleName: dataset.name || rule.name || ''
    })
  },

  onCancelDeleteDialog() {
    if (this.data.deleteDialogSubmitting) {
      return
    }

    this.setData({
      deleteDialogVisible: false,
      deleteDialogRuleId: 0,
      deleteDialogRuleName: ''
    })
  },

  async onConfirmDeleteRule() {
    if (this.data.deleteDialogSubmitting || !this.data.deleteDialogRuleId) {
      return
    }

    const ruleId = this.data.deleteDialogRuleId
    const pendingRules = this.data.rules.map((rule) => (
      rule.id === ruleId ? { ...rule, deletePending: true } : rule
    ))

    this.setData({
      ...buildPresentationUpdate(pendingRules),
      deleteDialogSubmitting: true
    })

    try {
      await deleteMerchantDiscountRule(this.data.merchantId, ruleId)
      const nextRules = removeRuleView(pendingRules, ruleId)
      this.setData({
        ...buildPresentationUpdate(nextRules),
        total: Math.max(this.data.total - 1, nextRules.length, 0),
        deleteDialogVisible: false,
        deleteDialogSubmitting: false,
        deleteDialogRuleId: 0,
        deleteDialogRuleName: '',
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      wx.showToast({ title: '满减规则已删除', icon: 'none' })
      void this.loadRules(false, true)
    } catch (err) {
      logger.error('Delete merchant discount rule failed', err)
      const restoredRules = pendingRules.map((rule) => (
        rule.id === ruleId ? { ...rule, deletePending: false } : rule
      ))
      this.setData({
        ...buildPresentationUpdate(restoredRules),
        deleteDialogSubmitting: false
      })
      wx.showToast({ title: getErrorUserMessage(err, '删除满减规则失败，请稍后重试'), icon: 'none' })
    }
  }
})