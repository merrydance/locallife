import dayjs from 'dayjs'
import { CreateMerchantVoucherParams, MerchantVoucher, MerchantVoucherService, UpdateMerchantVoucherParams } from '../../../api/coupon'
import { getMyMerchantProfile } from '../../../api/merchant'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'

type VoucherStatusTheme = 'success' | 'warning' | 'danger' | 'default'
type OrderType = 'takeout' | 'dine_in' | 'takeaway' | 'reservation'

interface VoucherView extends MerchantVoucher {
  amount_text: string
  min_order_amount_text: string
  usage_condition_text: string
  valid_range_text: string
  remaining_quantity: number
  remaining_quantity_text: string
  order_type_labels: string[]
  status_label: string
  status_theme: VoucherStatusTheme
}

interface VoucherPageChunk {
  vouchers: VoucherView[]
  pageId: number
  pageSize: number
  total?: number
}

interface VoucherPageProbeResult {
  hasMore: boolean
  total: number
  nextPageCache: VoucherPageChunk | null
}

interface VoucherFormData {
  code: string
  name: string
  description: string
  amount_yuan: string
  min_order_amount_yuan: string
  total_quantity: string
  valid_from: string
  valid_until: string
  is_active: boolean
  allowed_order_types: OrderType[]
}

const VOUCHERS_PAGE_SIZE = 50
const VOUCHERS_AUTO_REFRESH_WINDOW_MS = 60 * 1000

const ALL_ORDER_TYPES: OrderType[] = ['takeout', 'dine_in', 'takeaway', 'reservation']

const ORDER_TYPE_OPTIONS: Array<{ value: OrderType, label: string }> = [
  { value: 'takeout', label: '外卖配送' },
  { value: 'dine_in', label: '堂食' },
  { value: 'takeaway', label: '到店自取' },
  { value: 'reservation', label: '预订' }
]

function formatMoney(amount: number) {
  return `¥${(amount / 100).toFixed(2)}`
}

function toRFC3339Start(date: string) {
  return `${date}T00:00:00+08:00`
}

function toRFC3339End(date: string) {
  return `${date}T23:59:59+08:00`
}

function defaultFormData(): VoucherFormData {
  return {
    code: '',
    name: '',
    description: '',
    amount_yuan: '',
    min_order_amount_yuan: '',
    total_quantity: '',
    valid_from: '',
    valid_until: '',
    is_active: true,
    allowed_order_types: [...ALL_ORDER_TYPES]
  }
}

function getOrderTypeLabel(type: OrderType) {
  const matched = ORDER_TYPE_OPTIONS.find((item) => item.value === type)
  return matched?.label || type
}

function buildStatus(view: MerchantVoucher) {
  const now = dayjs()
  const validFrom = dayjs(view.valid_from)
  const validUntil = dayjs(view.valid_until)
  const remainingQuantity = Math.max(view.total_quantity - view.claimed_quantity, 0)

  if (!view.is_active) {
    return { label: '已停用', theme: 'default' as VoucherStatusTheme }
  }
  if (validUntil.isValid() && now.isAfter(validUntil)) {
    return { label: '已过期', theme: 'danger' as VoucherStatusTheme }
  }
  if (validFrom.isValid() && now.isBefore(validFrom)) {
    return { label: '未开始', theme: 'warning' as VoucherStatusTheme }
  }
  if (remainingQuantity <= 0) {
    return { label: '已领完', theme: 'warning' as VoucherStatusTheme }
  }
  return { label: '发放中', theme: 'success' as VoucherStatusTheme }
}

function buildVoucherView(voucher: MerchantVoucher): VoucherView {
  const status = buildStatus(voucher)
  const remainingQuantity = Math.max(voucher.total_quantity - voucher.claimed_quantity, 0)
  const minOrderAmountText = voucher.min_order_amount > 0 ? formatMoney(voucher.min_order_amount) : '不限门槛'
  return {
    ...voucher,
    amount_text: formatMoney(voucher.amount),
    min_order_amount_text: minOrderAmountText,
    usage_condition_text: voucher.min_order_amount > 0 ? `满 ${minOrderAmountText} 可用` : '不限门槛可用',
    valid_range_text: `${dayjs(voucher.valid_from).format('YYYY-MM-DD')} 至 ${dayjs(voucher.valid_until).format('YYYY-MM-DD')}`,
    remaining_quantity: remainingQuantity,
    remaining_quantity_text: `${remainingQuantity} / ${voucher.total_quantity}`,
    order_type_labels: (voucher.allowed_order_types.length ? voucher.allowed_order_types : ALL_ORDER_TYPES).map((item) => getOrderTypeLabel(item as OrderType)),
    status_label: status.label,
    status_theme: status.theme
  }
}

function toFormData(voucher: MerchantVoucher): VoucherFormData {
  return {
    code: voucher.code,
    name: voucher.name,
    description: voucher.description,
    amount_yuan: (voucher.amount / 100).toFixed(2),
    min_order_amount_yuan: voucher.min_order_amount > 0 ? (voucher.min_order_amount / 100).toFixed(2) : '',
    total_quantity: String(voucher.total_quantity),
    valid_from: dayjs(voucher.valid_from).format('YYYY-MM-DD'),
    valid_until: dayjs(voucher.valid_until).format('YYYY-MM-DD'),
    is_active: voucher.is_active,
    allowed_order_types: (voucher.allowed_order_types.length ? voucher.allowed_order_types : ALL_ORDER_TYPES) as OrderType[]
  }
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
  const normalizedPageSize = pageSize > 0 ? pageSize : VOUCHERS_PAGE_SIZE
  const visiblePageCount = visibleCount > 0 ? Math.ceil(visibleCount / normalizedPageSize) : 1
  return Math.max(pageId || 0, visiblePageCount, 1)
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

function getCreatePayload(form: VoucherFormData): CreateMerchantVoucherParams {
  return {
    code: form.code.trim() || undefined,
    name: form.name.trim(),
    description: form.description.trim() || undefined,
    amount: Math.round(Number(form.amount_yuan) * 100),
    min_order_amount: form.min_order_amount_yuan ? Math.round(Number(form.min_order_amount_yuan) * 100) : 0,
    total_quantity: Number(form.total_quantity),
    valid_from: toRFC3339Start(form.valid_from),
    valid_until: toRFC3339End(form.valid_until),
    allowed_order_types: form.allowed_order_types
  }
}

function getUpdatePayload(form: VoucherFormData): UpdateMerchantVoucherParams {
  return {
    name: form.name.trim(),
    description: form.description.trim() || undefined,
    amount: Math.round(Number(form.amount_yuan) * 100),
    min_order_amount: form.min_order_amount_yuan ? Math.round(Number(form.min_order_amount_yuan) * 100) : 0,
    total_quantity: Number(form.total_quantity),
    valid_from: toRFC3339Start(form.valid_from),
    valid_until: toRFC3339End(form.valid_until),
    is_active: form.is_active,
    allowed_order_types: form.allowed_order_types
  }
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
    actingVoucherId: 0,
    actingVoucherAction: '',
    merchantId: 0,
    lastLoadedAt: 0,
    vouchers: [] as VoucherView[],
    pageId: 0,
    pageSize: VOUCHERS_PAGE_SIZE,
    total: 0,
    hasMore: false,
    lookaheadVouchers: [] as VoucherView[],
    lookaheadPageId: 0,
    lookaheadPageSize: VOUCHERS_PAGE_SIZE,
    lookaheadTotal: 0,
    formVisible: false,
    isEdit: false,
    editId: 0,
    form: defaultFormData(),
    orderTypeOptions: ORDER_TYPE_OPTIONS
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
      && !this.data.actingVoucherId
      && shouldAutoRefresh(this.data.lastLoadedAt, VOUCHERS_AUTO_REFRESH_WINDOW_MS)
    ) {
      void this.loadVouchers(false)
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
      logger.error('Init merchant voucher context failed', err)
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

    await this.loadVouchers(showLoading, force)
  },

  async fetchVoucherPage(pageNumber: number, requestedPageSize = VOUCHERS_PAGE_SIZE): Promise<VoucherPageChunk> {
    const response = await MerchantVoucherService.listMerchantVouchers(this.data.merchantId, pageNumber, requestedPageSize)
    return {
      vouchers: (response.vouchers || []).map(buildVoucherView),
      pageId: response.page || pageNumber,
      pageSize: response.pageSize || requestedPageSize || VOUCHERS_PAGE_SIZE,
      total: typeof response.total === 'number' && response.total >= 0 ? response.total : undefined
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
    if (this.data.loading || !this.data.merchantId) return

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
        vouchers: result.vouchers,
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
      const message = getErrorMessage(err, '加载代金券失败，请稍后重试')

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
      const nextChunk = this.getCachedVoucherPage(nextPage)
        || await this.fetchVoucherPage(nextPage, this.data.pageSize || VOUCHERS_PAGE_SIZE)
      const mergedVouchers = appendVoucherViews(this.data.vouchers, nextChunk.vouchers)
      const pagination = await this.resolveVoucherPageBoundary(nextChunk, mergedVouchers.length)

      this.setData({
        vouchers: mergedVouchers,
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
      wx.showToast({ title: getErrorMessage(err, '加载更多失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ loadingMore: false })
    }
  },

  onAddVoucher() {
    if (this.data.actingVoucherId) return

    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      formVisible: true,
      isEdit: false,
      editId: 0,
      form: defaultFormData()
    })
  },

  onEditVoucher(e: WechatMiniprogram.TouchEvent) {
    if (this.data.actingVoucherId) return

    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    const target = this.data.vouchers.find((item) => item.id === id)
    if (!target) return

    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      formVisible: true,
      isEdit: true,
      editId: id,
      form: toFormData(target)
    })
  },

  onCloseForm() {
    this.setData({ formVisible: false })
  },

  onTextInput(e: WechatMiniprogram.Input) {
    const { field } = e.currentTarget.dataset as { field?: keyof VoucherFormData }
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
    const { field } = e.currentTarget.dataset as { field?: 'is_active' }
    if (!field) return
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      [`form.${field}`]: Boolean(e.detail.value)
    })
  },

  onToggleOrderType(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: OrderType }
    if (!value) return

    const next = [...this.data.form.allowed_order_types]
    const index = next.indexOf(value)
    if (index >= 0) {
      next.splice(index, 1)
    } else {
      next.push(value)
    }

    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      'form.allowed_order_types': next
    })
  },

  validateForm() {
    const { form } = this.data
    if (!form.name.trim()) return '请填写代金券名称'
    if (!form.amount_yuan || Number(form.amount_yuan) <= 0) return '请输入有效的抵扣金额'
    if (form.min_order_amount_yuan && Number(form.min_order_amount_yuan) < 0) return '使用门槛不能小于 0'
    if (!form.total_quantity || Number(form.total_quantity) <= 0) return '请输入有效的发放数量'
    if (!form.valid_from) return '请选择生效开始日期'
    if (!form.valid_until) return '请选择生效结束日期'
    if (form.valid_until < form.valid_from) return '结束日期不能早于开始日期'
    if (!form.allowed_order_types.length) return '请至少选择一个可用订单类型'
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
      const wasEdit = this.data.isEdit
      let savedVoucher: MerchantVoucher

      if (wasEdit && this.data.editId) {
        savedVoucher = await MerchantVoucherService.updateMerchantVoucher(this.data.merchantId, this.data.editId, getUpdatePayload(this.data.form))
      } else {
        savedVoucher = await MerchantVoucherService.createMerchantVoucher(this.data.merchantId, getCreatePayload(this.data.form))
      }

      const nextVouchers = upsertVoucherView(this.data.vouchers, savedVoucher)
      const nextTotal = Math.max(wasEdit ? this.data.total : this.data.total + 1, nextVouchers.length)

      this.setData({
        vouchers: nextVouchers,
        total: nextTotal,
        hasMore: resolveHasMore(nextTotal, nextVouchers.length, this.data.pageSize || VOUCHERS_PAGE_SIZE, 0),
        lookaheadVouchers: [],
        lookaheadPageId: 0,
        lookaheadPageSize: this.data.pageSize || VOUCHERS_PAGE_SIZE,
        lookaheadTotal: 0,
        formVisible: false,
        isEdit: false,
        editId: 0,
        form: defaultFormData(),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        actionNoticeMessage: wasEdit ? '代金券已更新。' : '代金券已创建。',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      void this.loadVouchers(false, true)
    } catch (err) {
      logger.error('Submit merchant voucher failed', err)
      wx.showToast({ title: getErrorMessage(err, this.data.isEdit ? '更新代金券失败，请稍后重试' : '创建代金券失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
      wx.hideLoading()
    }
  },

  onToggleVoucherStatus(e: WechatMiniprogram.TouchEvent) {
    const { id, active, name } = e.currentTarget.dataset as { id?: number, active?: boolean, name?: string }
    if (!id || typeof active !== 'boolean' || this.data.actingVoucherId) return

    wx.showModal({
      title: active ? '停用代金券' : '启用代金券',
      content: `${active ? '停用' : '启用'}「${name || '该代金券'}」后，顾客${active ? '将不能继续领取和使用' : '可以继续领取并在有效期内使用'}。`,
      success: async (res) => {
        if (!res.confirm) return
        this.setData({ actingVoucherId: id, actingVoucherAction: 'toggle' })
        try {
          const updatedVoucher = await MerchantVoucherService.updateMerchantVoucher(this.data.merchantId, id, { is_active: !active })
          const nextVouchers = upsertVoucherView(this.data.vouchers, updatedVoucher)
          const nextTotal = Math.max(this.data.total, nextVouchers.length)

          this.setData({
            vouchers: nextVouchers,
            total: nextTotal,
            hasMore: resolveHasMore(nextTotal, nextVouchers.length, this.data.pageSize || VOUCHERS_PAGE_SIZE, 0),
            lookaheadVouchers: [],
            lookaheadPageId: 0,
            lookaheadPageSize: this.data.pageSize || VOUCHERS_PAGE_SIZE,
            lookaheadTotal: 0,
            initialLoading: false,
            initialError: false,
            initialErrorMessage: '',
            actionNoticeMessage: updatedVoucher.is_active ? '代金券已启用。' : '代金券已停用。',
            refreshErrorMessage: '',
            lastLoadedAt: Date.now()
          })
          void this.loadVouchers(false, true)
        } catch (err) {
          logger.error('Toggle merchant voucher status failed', err)
          wx.showToast({ title: getErrorMessage(err, '更新状态失败，请稍后重试'), icon: 'none' })
        } finally {
          this.setData({ actingVoucherId: 0, actingVoucherAction: '' })
        }
      }
    })
  },

  onDeleteVoucher(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
    if (!id || this.data.actingVoucherId) return

    wx.showModal({
      title: '确认删除',
      content: `删除「${name || '该代金券'}」后不可恢复；若仍有未使用券，后端会拒绝删除。`,
      confirmColor: '#e34d59',
      success: async (res) => {
        if (!res.confirm) return

        this.setData({ actingVoucherId: id, actingVoucherAction: 'delete' })
        try {
          await MerchantVoucherService.deleteMerchantVoucher(this.data.merchantId, id)
          const nextVouchers = removeVoucherView(this.data.vouchers, id)
          const nextTotal = Math.max(this.data.total - 1, nextVouchers.length, 0)

          this.setData({
            vouchers: nextVouchers,
            total: nextTotal,
            hasMore: resolveHasMore(nextTotal, nextVouchers.length, this.data.pageSize || VOUCHERS_PAGE_SIZE, 0),
            lookaheadVouchers: [],
            lookaheadPageId: 0,
            lookaheadPageSize: this.data.pageSize || VOUCHERS_PAGE_SIZE,
            lookaheadTotal: 0,
            initialLoading: false,
            initialError: false,
            initialErrorMessage: '',
            actionNoticeMessage: '代金券已删除。',
            refreshErrorMessage: '',
            lastLoadedAt: Date.now()
          })
          void this.loadVouchers(false, true)
        } catch (err) {
          logger.error('Delete merchant voucher failed', err)
          wx.showToast({ title: getErrorMessage(err, '删除代金券失败，请稍后重试'), icon: 'none' })
        } finally {
          this.setData({ actingVoucherId: 0, actingVoucherAction: '' })
        }
      }
    })
  }
})