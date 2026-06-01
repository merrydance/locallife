import CartService from '../../services/cart'
import type { TakeoutCategoryGridItem } from '../../adapters/takeout-categories'
import { getUserCarts } from '../../api/cart'
import { searchMerchantsWithMeta, getPublicMerchantDishes, getPublicMerchantDetail } from '../../api/merchant'
import Navigation from '../../utils/navigation'
import { logger } from '../../utils/logger'
import { isRateLimitError } from '../../utils/user-facing'
import { ErrorHandler } from '../../utils/error-handler'
import { globalStore } from '../../utils/global-store'
import { requestManager } from '../../utils/request-manager'
import { getStableBarHeights } from '../../utils/responsive'
import { settleAll } from '../../utils/promise'
import { formatPrice } from '../../utils/util'
import {
  buildTakeoutCategoriesState,
  buildTakeoutFeaturedDishes,
  buildTakeoutMerchantFeedItems,
  buildTakeoutMerchantMetaPatch,
  chunkArray,
  openTakeoutLocationPicker,
  retryTakeoutLocation,
  showTakeoutLocationGuide,
  sleep,
  TAKEOUT_BACKGROUND_DISH_HYDRATION_DELAY_MS,
  TAKEOUT_HYDRATION_BATCH_SIZE,
  TAKEOUT_PAGE_SIZE,
  TAKEOUT_PRIORITY_META_HYDRATION_DELAY_MS,
  tryTakeoutLoadData,
  type MerchantFeedViewModel,
  type SearchTimer,
  type UserMessageError
} from '../../utils/takeout-index-support'

const PAGE_CONTEXT = 'takeout_index'
const TAKEOUT_HYDRATION_MERCHANT_LIMIT = 3
const TAKEOUT_CART_REFRESH_INTERVAL_MS = 30000

interface TakeoutActivityBanner {
  id: string
  title: string
  subtitle: string
  badge: string
  cta: string
  icon: string
  url: string
  cardClass: string
}

Page({
  data: {
    activityBanners: [
      {
        id: 'wanted-merchants',
        title: '你想吃谁家外卖？',
        subtitle: '告诉我们，优先邀请他入驻',
        badge: '活动征集',
        cta: '去 +1',
        icon: 'flag',
        url: '/pages/takeout/wanted-merchants/index',
        cardClass: 'activity-card activity-card--wanted'
      }
    ] as TakeoutActivityBanner[],
    merchantFeed: [] as MerchantFeedViewModel[],
    cuisineCategories: [] as TakeoutCategoryGridItem[],
    activeCategoryId: '',
    cartTotalCount: 0,
    cartTotalPrice: 0,
    address: '点此获取位置',
    navBarHeight: 88,
    scrollViewHeight: 600,
    searchKeyword: '',
    page: 1,
    hasMore: true,
    loading: true,
    isError: false,
    errorMsg: '',
    needLocation: false,
    refresherTriggered: false,
    hasServiceProviders: true
  },

  _unsubscribeLocation: undefined as undefined | (() => void),
  _dataLoadedLat: null as number | null,
  _dataLoadedLng: null as number | null,
  _feedHydrationGeneration: 0,
  _feedHydrationTimers: [] as SearchTimer[],
  _pendingReload: false,
  _lastCartRefreshAt: 0,

  onLoad() {
    wx.showShareMenu({
      withShareTicket: true,
      menus: ['shareAppMessage', 'shareTimeline']
    })

    const { navBarHeight } = getStableBarHeights()
    const windowInfo = wx.getWindowInfo()
    const scrollViewHeight = windowInfo.windowHeight

    this.setData({
      navBarHeight,
      scrollViewHeight
    })

    this.loadCategories()

    const app = getApp<IAppOption>()
    const loc = app.globalData.location

    if (loc && loc.name) {
      this.setData({ address: loc.name })
    } else {
      this.setData({ address: '定位中...' })
    }

    this._unsubscribeLocation = globalStore.subscribe('location', (newLocation) => {
      logger.info('[Takeout] 收到位置更新', newLocation, 'Takeout.onLoad')
      if (newLocation.name) {
        this.setData({
          address: newLocation.name,
          needLocation: false
        })

        const _appRef = getApp<IAppOption>()
        const newLat = _appRef.globalData.latitude
        const newLng = _appRef.globalData.longitude

        if (this.data.merchantFeed.length === 0 && !this.data.loading) {
          logger.info('[Takeout] 位置已更新，开始加载数据', undefined, 'Takeout.onLoad')
          this.loadData()
        } else if (newLat && newLng && this._dataLoadedLat !== null && this._dataLoadedLng !== null) {
          const { haversineDistance } = require('../../utils/geo')
          const dist = haversineDistance(this._dataLoadedLat, this._dataLoadedLng, newLat, newLng)
          if (dist > 1.0) {
            logger.info('[Takeout] 位置变化超过 1km，重新加载数据', { dist: `${dist.toFixed(2)}km` }, 'Takeout.onLoad')
            this.setData({ page: 1 })
            this.loadData()
          }
        }
      }
    })
  },

  onLocationTap() {
    wx.chooseLocation({
      success: async (res) => {
        const app = getApp<IAppOption>()

        app.globalData.latitude = res.latitude
        app.globalData.longitude = res.longitude
        app.globalData.location = {
          name: res.name || res.address,
          address: res.address
        }

        this.setData({ address: res.name || res.address })

        this.onLocationChange()
      },
      fail: () => {}
    })
  },

  onMerchantRegister() { wx.navigateTo({ url: '/pages/register/merchant/index' }) },
  onOperatorRegister() { wx.navigateTo({ url: '/pages/register/operator/index' }) },
  onSearchTap() { wx.navigateTo({ url: '/pages/takeout/search/index' }) },
  onActivityBannerTap(e: WechatMiniprogram.CustomEvent) {
    const { url } = e.currentTarget.dataset as { url?: string }
    if (!url) return
    wx.navigateTo({ url })
  },

  onCategoryTap(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset as { id: string }
    const nextCategoryId = String(id || '')
    const shouldResetToAll = nextCategoryId === this.data.activeCategoryId && nextCategoryId !== ''

    this.setData({
      activeCategoryId: shouldResetToAll ? '' : nextCategoryId,
      page: 1
    })

    this.loadData()
  },

  onShow() {
    logger.debug('[Takeout.onShow] 页面显示', {
      feedCount: this.data.merchantFeed.length,
      loading: this.data.loading
    }, 'Takeout.onShow')

    const cachedCart = globalStore.get('cart')
    if (cachedCart) {
      this.setData({ cartTotalCount: cachedCart.totalCount, cartTotalPrice: cachedCart.totalPrice })
    }
    if (this.shouldRefreshCartDisplay()) {
      void this.updateCartDisplay()
    }

    const app = getApp<IAppOption>()
    const loc = app.globalData.location
    if (loc && loc.name) {
      this.setData({ address: loc.name })
    }

    if (this.data.merchantFeed.length === 0) {
      logger.info('[Takeout.onShow] 开始 tryLoadData', undefined, 'Takeout.onShow')
      this.tryLoadData()
    } else {
      logger.debug('[Takeout.onShow] 跳过 tryLoadData，已有数据')
    }
  },

  async tryLoadData(retryCount = 0) {
    await tryTakeoutLoadData(this, retryCount, () => this.showLocationGuide())
  },

  showLocationGuide() {
    showTakeoutLocationGuide(this)
  },

  onManualLocation() { this.openLocationPicker() },

  onRetryLocation() {
    retryTakeoutLocation(this)
  },

  openLocationPicker() {
    openTakeoutLocationPicker(this, () => this.loadData())
  },

  onHide() {
    requestManager.cancelByContext(PAGE_CONTEXT)
    logger.debug('页面隐藏,已取消所有请求', undefined, 'Takeout.onHide')
  },

  onUnload() {
    requestManager.cancelByContext(PAGE_CONTEXT)
    this.resetFeedHydration()

    if (this._unsubscribeLocation) {
      this._unsubscribeLocation()
    }
  },

  onLocationChange() { this.setData({ page: 1 }); this.loadData() },

  async loadCategories() {
    try {
      const categoryState = await buildTakeoutCategoriesState(this.data.activeCategoryId)
      if (categoryState) {
        this.setData(categoryState)
      }
    } catch (e) {
      logger.warn('[Takeout] 品类加载失败', e, 'Takeout.loadCategories')
    }
  },

  async loadData() {
    if (this._isLoading) {
      this._pendingReload = true
      return
    }

    this._pendingReload = false
    this._isLoading = true
    this.setData({ loading: true, isError: false })

    const _app = getApp<IAppOption>()
    this._dataLoadedLat = _app.globalData.latitude
    this._dataLoadedLng = _app.globalData.longitude

    try {
      const { page } = this.data
      const reset = page === 1

      if (reset) {
        await this.loadCategories()
      }

      await this.loadFeed(reset)
    } catch (error: unknown) {
      ErrorHandler.handle(error, 'Takeout.loadData')
      const userMessage = (error as UserMessageError).userMessage
      if (this.data.page === 1) {
        this.setData({
          isError: true,
          errorMsg: (typeof userMessage === 'string' && userMessage) ? userMessage : '数据加载失败',
          hasMore: false
        })
      }
    } finally {
      this._isLoading = false
      this.setData({ loading: false })

      if (this._pendingReload) {
        this._pendingReload = false
        void this.loadData()
      }
    }
  },

  async loadFeed(reset = false) {
    const generation = reset ? this.resetFeedHydration() : this._feedHydrationGeneration

    if (reset) {
      this.setData({ page: 1, merchantFeed: [], hasMore: true })
    }

    try {
      const app = getApp<IAppOption>()
      const currentPage = reset ? 1 : this.data.page

      const result = await searchMerchantsWithMeta({
        keyword: this.data.searchKeyword,
        tag_id: this.data.activeCategoryId ? Number(this.data.activeCategoryId) : undefined,
        sort_by: 'distance',
        page_id: currentPage,
        page_size: TAKEOUT_PAGE_SIZE,
        user_latitude: app.globalData.latitude || undefined,
        user_longitude: app.globalData.longitude || undefined
      })

      const merchants = result.merchants
      const hasMore = result.hasMore

      const feedItems = buildTakeoutMerchantFeedItems(merchants)

      // 记录已有 feed 长度，异步加载菜品时用于按 ID 定位
      if (reset) {
        this.setData({ merchantFeed: feedItems, hasMore, hasServiceProviders: merchants.length > 0 })
      } else {
        this.setData({ merchantFeed: [...this.data.merchantFeed, ...feedItems], hasMore })
      }

      const merchantIds = merchants.map((m) => m.id)
      this.scheduleMerchantHydration(merchantIds, generation)

      const { preloadImages } = require('../../utils/image')
      const imageUrls = feedItems.map((f) => f.imageUrl).filter(Boolean)
      setTimeout(() => preloadImages(imageUrls, false), 100)
    } catch (error) {
      logger.error('加载商家 Feed 失败', error, 'Takeout.loadFeed')
      throw error
    }
  },

  scheduleMerchantHydration(merchantIds: number[], generation: number) {
    const priorityIds = merchantIds.slice(0, TAKEOUT_HYDRATION_MERCHANT_LIMIT)

    this.queueHydrationPhase(priorityIds, generation, TAKEOUT_BACKGROUND_DISH_HYDRATION_DELAY_MS, this.hydrateMerchantDishesBatch)
    this.queueHydrationPhase(priorityIds, generation, TAKEOUT_PRIORITY_META_HYDRATION_DELAY_MS, this.hydrateMerchantMetaBatch)
  },

  queueHydrationPhase(
    merchantIds: number[],
    generation: number,
    initialDelay: number,
    worker: (merchantIds: number[], generation: number) => Promise<void>
  ) {
    if (merchantIds.length === 0) return

    const timer = setTimeout(() => {
      this._feedHydrationTimers = this._feedHydrationTimers.filter((currentTimer) => currentTimer !== timer)
      if (generation !== this._feedHydrationGeneration) return
      void this.runHydrationQueue(merchantIds, generation, worker)
    }, initialDelay)

    this._feedHydrationTimers.push(timer)
  },

  async runHydrationQueue(
    merchantIds: number[],
    generation: number,
    worker: (merchantIds: number[], generation: number) => Promise<void>
  ) {
    for (const chunk of chunkArray(merchantIds, TAKEOUT_HYDRATION_BATCH_SIZE)) {
      if (generation !== this._feedHydrationGeneration) return
      await worker.call(this, chunk, generation)
      if (generation !== this._feedHydrationGeneration) return
      await sleep(120)
    }
  },

  async hydrateMerchantDishesBatch(merchantIds: number[], generation: number) {
    const results = await settleAll(
      merchantIds.map(async (merchantId) => ({
        merchantId,
        dishesResp: await getPublicMerchantDishes(merchantId)
      }))
    )

    if (generation !== this._feedHydrationGeneration) return

    const updates: Record<string, unknown> = {}
    results.forEach((result, index) => {
      const merchantId = result.status === 'fulfilled' ? result.value.merchantId : merchantIds[index]
      const feedIndex = this.data.merchantFeed.findIndex((item) => item.id === merchantId)
      if (feedIndex === -1) return

      updates[`merchantFeed[${feedIndex}].dishesLoading`] = false

      if (result.status !== 'fulfilled') return

      const { dishesResp } = result.value

      updates[`merchantFeed[${feedIndex}].featuredDishes`] = buildTakeoutFeaturedDishes(dishesResp.dishes, merchantId)
    })

    if (Object.keys(updates).length > 0) {
      this.setData(updates)
    }
  },

  async hydrateMerchantMetaBatch(merchantIds: number[], generation: number) {
    const results = await settleAll(
      merchantIds.map(async (merchantId) => {
        const detail = await getPublicMerchantDetail(merchantId, true)
        return { merchantId, detail }
      })
    )

    if (generation !== this._feedHydrationGeneration) return

    const updates: Record<string, unknown> = {}
    results.forEach((result, index) => {
      const merchantId = result.status === 'fulfilled' ? result.value.merchantId : merchantIds[index]
      const feedIndex = this.data.merchantFeed.findIndex((item) => item.id === merchantId)
      if (feedIndex === -1) return

      updates[`merchantFeed[${feedIndex}].detailLoading`] = false

      if (result.status !== 'fulfilled') return

      const { detail } = result.value

      const metaPatch = buildTakeoutMerchantMetaPatch(detail)
      Object.entries(metaPatch).forEach(([key, value]) => {
        updates[`merchantFeed[${feedIndex}].${key}`] = value
      })
    })

    if (Object.keys(updates).length > 0) {
      this.setData(updates)
    }
  },

  clearFeedHydrationTimers() {
    this._feedHydrationTimers.forEach((timer) => clearTimeout(timer))
    this._feedHydrationTimers = []
  },

  resetFeedHydration() {
    this._feedHydrationGeneration += 1
    this.clearFeedHydrationTimers()
    return this._feedHydrationGeneration
  },

  shouldRefreshCartDisplay(force = false) {
    if (force) return true
    return Date.now() - this._lastCartRefreshAt > TAKEOUT_CART_REFRESH_INTERVAL_MS
  },

  async onDishAddFromFeed(e: WechatMiniprogram.CustomEvent) {
    const { dishId, merchantId } = e.detail as { dishId: number, merchantId: number }
    if (!dishId || !merchantId) return

    const merchant = this.data.merchantFeed.find((item) => item.id === merchantId)
    if (merchant?.isOrderingSuspended) {
      wx.showToast({ title: '当前商户暂停接单', icon: 'none' })
      return
    }

    const success = await CartService.addItem({
      merchantId: String(merchantId),
      dishId: String(dishId)
    })

    if (success) {
      await this.updateCartDisplay()
      wx.showToast({ title: '已加入购物车', icon: 'success', duration: 500 })
    }
  },

  onMerchantTapFromFeed(e: WechatMiniprogram.CustomEvent) {
    Navigation.toRestaurantDetail(String((e.detail as { id: number }).id))
  },

  async updateCartDisplay() {
    this._lastCartRefreshAt = Date.now()
    try {
      const userCarts = await getUserCarts('takeout', { loading: false })

      const totalCount = userCarts.summary?.total_items || 0
      const totalPrice = userCarts.summary?.total_amount || 0

      this.setData({
        cartTotalCount: totalCount,
        cartTotalPrice: totalPrice
      })

      globalStore.set('cart', {
        items: [],
        totalCount,
        totalPrice,
        totalPriceDisplay: formatPrice(totalPrice)
      })
    } catch (error) {
      logger.warn('获取购物车汇总失败', error, 'Takeout.updateCartDisplay')
      this.setData({
        cartTotalCount: 0,
        cartTotalPrice: 0
      })
    }
  },

  onCheckout() {
    if (this.data.cartTotalCount === 0) {
      wx.showToast({ title: '购物车是空的', icon: 'none' })
      return
    }
    Navigation.toCart()
  },

  _searchTimer: null as SearchTimer | null,

  onReachBottom() {
    if (!this.data.hasMore || this.data.loading || this._isLoading) {
      return
    }

    const now = Date.now()
    if (this._lastLoadTime && now - this._lastLoadTime < 500) {
      return
    }
    this._lastLoadTime = now

    const previousPage = this.data.page
    const nextPage = previousPage + 1
    
    this.setData({ page: nextPage })

    logger.debug('触发滚动加载', { nextPage }, 'Takeout.onReachBottom')

    this.loadData().catch((error) => {
      if (this.data.page === nextPage) {
        logger.error('加载更多失败，回滚页码', { nextPage, previousPage }, 'Takeout.onReachBottom')
        this.setData({ page: previousPage })
      }
      
      if (isRateLimitError(error)) {
        wx.showToast({ title: '请求太频繁，请稍后再试', icon: 'none', duration: 2000 })
      } else {
        wx.showToast({ title: '加载失败，请重试', icon: 'none' })
      }
    })
  },

  _lastLoadTime: 0,
  _isLoading: false,

  async onRefresh() {
    this.setData({ refresherTriggered: true, page: 1, searchKeyword: '' })

    try {
      await this.loadData()
    } finally {
      setTimeout(() => {
        this.setData({ refresherTriggered: false })
      }, 300)
    }
  },

  onPullDownRefresh() {
    this.setData({ page: 1, searchKeyword: '' })
    this.loadData().then(() => {
      wx.stopPullDownRefresh()
    })
  },

  onShareAppMessage() {
    return {
      title: '本地生活外卖推荐，附近好店马上送达',
      path: '/pages/takeout/index'
    }
  },

  onShareTimeline() {
    return {
      title: '本地生活外卖推荐，附近好店马上送达'
    }
  }
})
