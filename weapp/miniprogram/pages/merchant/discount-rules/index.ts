import dayjs from 'dayjs'
import {
  createMerchantDiscountRule,
  deleteMerchantDiscountRule,
  getMyMerchantProfile,
  listMerchantDiscountRules,
  MerchantDiscountRuleResponse,
  updateMerchantDiscountRule
} from '../../../api/merchant'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'

type DiscountStatusTheme = 'success' | 'warning' | 'danger' | 'default'

interface DiscountRuleView extends MerchantDiscountRuleResponse {
  min_order_amount_text: string
  discount_amount_text: string
  valid_range_text: string
  stacking_text: string
  status_label: string
  status_theme: DiscountStatusTheme
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
  return `¥${(amount / 100).toFixed(2)}`
}

function toRFC3339Start(date: string) {
  return `${date}T00:00:00+08:00`
}

function toRFC3339End(date: string) {
  return `${date}T23:59:59+08:00`
}

function defaultFormData(): DiscountRuleFormData {
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

function buildStatus(rule: MerchantDiscountRuleResponse) {
  const now = dayjs()
  const validFrom = dayjs(rule.valid_from)
  const validUntil = dayjs(rule.valid_until)

  if (!rule.is_active) {
    return { label: '已停用', theme: 'default' as DiscountStatusTheme }
  }
  if (validUntil.isValid() && now.isAfter(validUntil)) {
    return { label: '已过期', theme: 'danger' as DiscountStatusTheme }
  }
  if (validFrom.isValid() && now.isBefore(validFrom)) {
    return { label: '未开始', theme: 'warning' as DiscountStatusTheme }
  }
  return { label: '生效中', theme: 'success' as DiscountStatusTheme }
}

function buildStackingText(rule: MerchantDiscountRuleResponse) {
  const tags: string[] = []
  if (rule.can_stack_with_voucher) tags.push('可叠加代金券')
  if (rule.can_stack_with_membership) tags.push('可叠加会员权益')
  if (rule.stacking_group) tags.push(`分组 ${rule.stacking_group}`)
  return tags.length ? tags.join(' · ') : '默认不与其他优惠叠加'
}

function buildRuleView(rule: MerchantDiscountRuleResponse): DiscountRuleView {
  const status = buildStatus(rule)
  return {
    ...rule,
    min_order_amount_text: formatAmount(rule.min_order_amount),
    discount_amount_text: formatAmount(rule.discount_amount),
    valid_range_text: `${dayjs(rule.valid_from).format('YYYY-MM-DD')} 至 ${dayjs(rule.valid_until).format('YYYY-MM-DD')}`,
    stacking_text: buildStackingText(rule),
    status_label: status.label,
    status_theme: status.theme
  }
}

function toFormData(rule: MerchantDiscountRuleResponse): DiscountRuleFormData {
  return {
    name: rule.name,
    description: rule.description || '',
    min_order_amount_yuan: (rule.min_order_amount / 100).toFixed(2),
    discount_amount_yuan: (rule.discount_amount / 100).toFixed(2),
    can_stack_with_voucher: rule.can_stack_with_voucher,
    can_stack_with_membership: rule.can_stack_with_membership,
    stacking_group: rule.stacking_group || '',
    valid_from: dayjs(rule.valid_from).format('YYYY-MM-DD'),
    valid_until: dayjs(rule.valid_until).format('YYYY-MM-DD'),
    is_active: rule.is_active
  }
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

function hasReliableTotal(total: number | undefined, loadedCount: number) {
  return typeof total === 'number' && total > loadedCount
}

function normalizeTotal(total: number | undefined, fallback: number) {
  return typeof total === 'number' && total >= 0 ? total : fallback
}

function resolveHasMore(total: number | undefined, loadedCount: number, pageSize: number, lastPageLength: number) {
  // Some adjacent backend list APIs return the current page length in total.
  // Only trust total when it clearly exceeds what is already loaded.
  const trustedTotal = hasReliableTotal(total, loadedCount) ? total : undefined
  if (typeof trustedTotal === 'number') {
    return loadedCount < trustedTotal
  }

  return lastPageLength >= pageSize
}

function resolveTargetPageCount(pageId: number, pageSize: number, visibleCount: number) {
  const normalizedPageSize = pageSize > 0 ? pageSize : DISCOUNT_RULES_PAGE_SIZE
  const visiblePageCount = visibleCount > 0 ? Math.ceil(visibleCount / normalizedPageSize) : 1
  return Math.max(pageId || 0, visiblePageCount, 1)
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

const getErrorMessage = getErrorUserMessage

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
    loadingMore: false,
    submitting: false,
    actingRuleId: 0,
    actingRuleAction: '',
    merchantId: 0,
    lastLoadedAt: 0,
    rules: [] as DiscountRuleView[],
    pageId: 0,
    pageSize: DISCOUNT_RULES_PAGE_SIZE,
    total: 0,
    hasMore: false,
    lookaheadRules: [] as DiscountRuleView[],
    lookaheadPageId: 0,
    lookaheadPageSize: DISCOUNT_RULES_PAGE_SIZE,
    lookaheadTotal: 0,
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
      && !this.data.loadingMore
      && !this.data.actingRuleId
      && shouldAutoRefresh(this.data.lastLoadedAt, DISCOUNT_RULES_AUTO_REFRESH_WINDOW_MS)
    ) {
      void this.loadRules(false)
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

  onReachBottom() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    void this.onLoadMore()
  },

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
      logger.error('Init merchant discount rules context failed', err)
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorMessage(err, '获取商户信息失败，请重试'),
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

    await this.loadRules(showLoading, force)
  },

  async fetchRulePage(pageNumber: number, requestedPageSize = DISCOUNT_RULES_PAGE_SIZE): Promise<DiscountRulePageChunk> {
    const response = await listMerchantDiscountRules(this.data.merchantId, pageNumber, requestedPageSize)
    return {
      rules: (response.rules || []).map(buildRuleView),
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
    if (this.data.loading || !this.data.merchantId) return

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
        rules: result.rules,
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
      const message = getErrorMessage(err, '加载满减规则失败，请稍后重试')

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

  async onLoadMore() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (this.data.loading || this.data.loadingMore || !this.data.merchantId || !this.data.hasMore) {
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
        rules: mergedRules,
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
      wx.showToast({ title: getErrorMessage(err, '加载更多失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ loadingMore: false })
    }
  },

  onAddRule() {
    if (this.data.actingRuleId || this.data.submitting) return

    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      formVisible: true,
      isEdit: false,
      editId: 0,
      form: defaultFormData()
    })
  },

  onEditRule(e: WechatMiniprogram.TouchEvent) {
    if (this.data.actingRuleId || this.data.submitting) return

    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    const rule = this.data.rules.find((item) => item.id === id)
    if (!rule) return
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      formVisible: true,
      isEdit: true,
      editId: id,
      form: toFormData(rule)
    })
  },

  onCloseForm() {
    this.setData({ formVisible: false })
  },

  onTextInput(e: WechatMiniprogram.Input) {
    const { field } = e.currentTarget.dataset as { field?: keyof DiscountRuleFormData }
    if (!field) return
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      [`form.${field}`]: e.detail.value
    })
  },

  onDateChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: 'valid_from' | 'valid_until' }
    if (!field) return
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      [`form.${field}`]: e.detail.value
    })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field?: 'can_stack_with_voucher' | 'can_stack_with_membership' | 'is_active' }
    if (!field) return
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      [`form.${field}`]: Boolean(e.detail.value)
    })
  },

  validateForm() {
    const { form } = this.data
    const minOrderAmount = Number(form.min_order_amount_yuan)
    const discountAmount = Number(form.discount_amount_yuan)

    if (!form.name.trim()) return '请填写规则名称'
    if (!Number.isFinite(minOrderAmount) || minOrderAmount <= 0) return '请输入有效的门槛金额'
    if (!Number.isFinite(discountAmount) || discountAmount <= 0) return '请输入有效的优惠金额'
    if (discountAmount >= minOrderAmount) return '优惠金额需小于门槛金额'
    if (!form.valid_from) return '请选择开始日期'
    if (!form.valid_until) return '请选择结束日期'
    if (form.valid_until < form.valid_from) return '结束日期不能早于开始日期'
    return ''
  },

  async onSubmitForm() {
    if (this.data.submitting) return

    const errorMessage = this.validateForm()
    if (errorMessage) {
      wx.showToast({ title: errorMessage, icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '保存中...' })
    try {
      const payload = {
        name: this.data.form.name.trim(),
        description: this.data.form.description.trim() || undefined,
        min_order_amount: Math.round(Number(this.data.form.min_order_amount_yuan) * 100),
        discount_amount: Math.round(Number(this.data.form.discount_amount_yuan) * 100),
        can_stack_with_voucher: this.data.form.can_stack_with_voucher,
        can_stack_with_membership: this.data.form.can_stack_with_membership,
        stacking_group: this.data.form.stacking_group.trim() || undefined,
        valid_from: toRFC3339Start(this.data.form.valid_from),
        valid_until: toRFC3339End(this.data.form.valid_until)
      }

      const wasEdit = this.data.isEdit
      let savedRule: MerchantDiscountRuleResponse

      if (this.data.isEdit && this.data.editId) {
        savedRule = await updateMerchantDiscountRule(this.data.merchantId, this.data.editId, {
          ...payload,
          is_active: this.data.form.is_active
        })
      } else {
        savedRule = await createMerchantDiscountRule(this.data.merchantId, payload)
      }

      const nextRules = upsertRuleView(this.data.rules, savedRule)
      const nextTotal = Math.max(wasEdit ? this.data.total : this.data.total + 1, nextRules.length)

      this.setData({
        rules: nextRules,
        total: nextTotal,
        hasMore: resolveHasMore(nextTotal, nextRules.length, this.data.pageSize || DISCOUNT_RULES_PAGE_SIZE, 0),
        lookaheadRules: [],
        lookaheadPageId: 0,
        lookaheadPageSize: this.data.pageSize || DISCOUNT_RULES_PAGE_SIZE,
        lookaheadTotal: 0,
        formVisible: false,
        isEdit: false,
        editId: 0,
        form: defaultFormData(),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        actionNoticeMessage: wasEdit ? '满减规则已更新。' : '满减规则已创建。',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      void this.loadRules(false, true)
    } catch (err) {
      logger.error('Submit merchant discount rule failed', err)
      wx.showToast({ title: getErrorMessage(err, this.data.isEdit ? '更新满减规则失败，请稍后重试' : '创建满减规则失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
      wx.hideLoading()
    }
  },

  onToggleRuleStatus(e: WechatMiniprogram.TouchEvent) {
    const { id, active } = e.currentTarget.dataset as { id?: number, active?: boolean }
    if (!id || typeof active !== 'boolean' || this.data.actingRuleId) return

    this.setData({ actingRuleId: id, actingRuleAction: 'toggle' })
    updateMerchantDiscountRule(this.data.merchantId, id, { is_active: !active })
      .then((updatedRule) => {
        const nextRules = upsertRuleView(this.data.rules, updatedRule)
        const nextTotal = Math.max(this.data.total, nextRules.length)

        this.setData({
          rules: nextRules,
          total: nextTotal,
          hasMore: resolveHasMore(nextTotal, nextRules.length, this.data.pageSize || DISCOUNT_RULES_PAGE_SIZE, 0),
          lookaheadRules: [],
          lookaheadPageId: 0,
          lookaheadPageSize: this.data.pageSize || DISCOUNT_RULES_PAGE_SIZE,
          lookaheadTotal: 0,
          initialLoading: false,
          initialError: false,
          initialErrorMessage: '',
          actionNoticeMessage: updatedRule.is_active ? '满减规则已启用。' : '满减规则已停用。',
          refreshErrorMessage: '',
          lastLoadedAt: Date.now()
        })
        void this.loadRules(false, true)
      })
      .catch((err) => {
        logger.error('Toggle merchant discount rule status failed', err)
        wx.showToast({ title: getErrorMessage(err, '更新状态失败，请稍后重试'), icon: 'none' })
      })
      .finally(() => this.setData({ actingRuleId: 0, actingRuleAction: '' }))
  },

  onDeleteRule(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
    if (!id || this.data.actingRuleId) return
    wx.showModal({
      title: '确认删除',
      content: `删除「${name || '该规则'}」后不可恢复，确认继续吗？`,
      confirmColor: '#e34d59',
      success: async (res) => {
        if (!res.confirm) return
        this.setData({ actingRuleId: id, actingRuleAction: 'delete' })
        try {
          await deleteMerchantDiscountRule(this.data.merchantId, id)
          const nextRules = removeRuleView(this.data.rules, id)
          const nextTotal = Math.max(this.data.total - 1, nextRules.length, 0)

          this.setData({
            rules: nextRules,
            total: nextTotal,
            hasMore: resolveHasMore(nextTotal, nextRules.length, this.data.pageSize || DISCOUNT_RULES_PAGE_SIZE, 0),
            lookaheadRules: [],
            lookaheadPageId: 0,
            lookaheadPageSize: this.data.pageSize || DISCOUNT_RULES_PAGE_SIZE,
            lookaheadTotal: 0,
            initialLoading: false,
            initialError: false,
            initialErrorMessage: '',
            actionNoticeMessage: '满减规则已删除。',
            refreshErrorMessage: '',
            lastLoadedAt: Date.now()
          })
          void this.loadRules(false, true)
        } catch (err) {
          logger.error('Delete merchant discount rule failed', err)
          wx.showToast({ title: getErrorMessage(err, '删除满减规则失败，请稍后重试'), icon: 'none' })
        } finally {
          this.setData({ actingRuleId: 0, actingRuleAction: '' })
        }
      }
    })
  }
})