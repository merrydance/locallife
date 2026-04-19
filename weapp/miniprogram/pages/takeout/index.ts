import CartService from '../../services/cart'
import type { TakeoutCategoryGridItem } from '../../adapters/takeout-categories'
import { getUserCarts } from '../../api/cart'
import { searchMerchantsWithMeta, getPublicMerchantDishes, getPublicMerchantDetail, getHasUserOrderedFromMerchant } from '../../api/merchant'
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
  TAKEOUT_BACKGROUND_META_HYDRATION_DELAY_MS,
  TAKEOUT_FIRST_SCREEN_MERCHANT_COUNT,
  TAKEOUT_HYDRATION_BATCH_SIZE,
  TAKEOUT_PAGE_SIZE,
  TAKEOUT_PRIORITY_META_HYDRATION_DELAY_MS,
  tryTakeoutLoadData,
  type MerchantFeedViewModel,
  type SearchTimer,
  type UserMessageError
} from '../../utils/takeout-index-support'

const PAGE_CONTEXT = 'takeout_index'

Page({
  data: {
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
  // 当前列表数据是使用哪套坐标加载的（用于跨城检测）
  _dataLoadedLat: null as number | null,
  _dataLoadedLng: null as number | null,
  _feedHydrationGeneration: 0,
  _feedHydrationTimers: [] as SearchTimer[],
  _pendingReload: false,

  onLoad() {
    wx.showShareMenu({
      withShareTicket: true,
      menus: ['shareAppMessage', 'shareTimeline']
    })

    // custom-navbar 是 fixed，scroll-view 用整屏高度，顶部避让交给内层 padding-top。
    const { navBarHeight } = getStableBarHeights()
    const windowInfo = wx.getWindowInfo()
    const scrollViewHeight = windowInfo.windowHeight

    this.setData({
      navBarHeight,
      scrollViewHeight
    })

    // 立即加载分类
    this.loadCategories()

    // 从全局获取位置信息
    const app = getApp<IAppOption>()
    const loc = app.globalData.location

    if (loc && loc.name) {
      // 有缓存位置信息,直接使用
      this.setData({ address: loc.name })
    } else {
      // 没有位置信息，显示提示
      this.setData({ address: '定位中...' })
    }

    // 订阅位置变化
    this._unsubscribeLocation = globalStore.subscribe('location', (newLocation) => {
      logger.info('[Takeout] 收到位置更新', newLocation, 'Takeout.onLoad')
      if (newLocation.name) {
        // 隐藏位置引导提示，更新地址显示
        this.setData({
          address: newLocation.name,
          needLocation: false
        })

        // 用 app.globalData 取新坐标：getLocationCoordinates 成功时最先写入
        // globalData，早于所有 globalStore 操作，此处读值一定是最新的。
        const _appRef = getApp<IAppOption>()
        const newLat = _appRef.globalData.latitude
        const newLng = _appRef.globalData.longitude

        if (this.data.merchantFeed.length === 0 && !this.data.loading) {
          // 还没有数据，直接加载
          logger.info('[Takeout] 位置已更新，开始加载数据', undefined, 'Takeout.onLoad')
          this.loadData()
        } else if (newLat && newLng && this._dataLoadedLat !== null && this._dataLoadedLng !== null) {
          // 已有数据，检查是否跨城（距离 > 1km）
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
    // 导航栏已经处理位置获取，这里只需要响应位置变化事件
    // 如果用户点击页面内的位置，可以打开位置选择器
    wx.chooseLocation({
      success: async (res) => {
        const app = getApp<IAppOption>()

        // 更新全局位置
        app.globalData.latitude = res.latitude
        app.globalData.longitude = res.longitude
        app.globalData.location = {
          name: res.name || res.address,
          address: res.address
        }

        // 更新页面显示
        this.setData({ address: res.name || res.address })

        // 重新加载基于新位置的推荐
        this.onLocationChange()
      },
      fail: () => {
        // 用户取消选择
      }
    })
  },

  onMerchantRegister() { wx.navigateTo({ url: '/pages/register/merchant/index' }) },
  onOperatorRegister() { wx.navigateTo({ url: '/pages/register/operator/index' }) },
  onSearchTap() { wx.navigateTo({ url: '/pages/takeout/search/index' }) },

  // 品类网格点击：页内切换筛选，不再跳转独立页
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

    // 从购物车返回时更新购物车数据
    // 先用 globalStore 缓存立即同步显示（避免每次切 tab 都等待网络），再后台静默刷新
    const cachedCart = globalStore.get('cart')
    if (cachedCart) {
      this.setData({ cartTotalCount: cachedCart.totalCount, cartTotalPrice: cachedCart.totalPrice })
    }
    this.updateCartDisplay()

    // 更新位置显示
    const app = getApp<IAppOption>()
    const loc = app.globalData.location
    if (loc && loc.name) {
      this.setData({ address: loc.name })
    }

    // 检查是否需要加载数据
    if (this.data.merchantFeed.length === 0) {
      logger.info('[Takeout.onShow] 开始 tryLoadData', undefined, 'Takeout.onShow')
      this.tryLoadData()
    } else {
      logger.debug('[Takeout.onShow] 跳过 tryLoadData，已有数据')
    }
  },

  // 尝试加载数据，等待 token 准备好，位置未授权则直接引导
  async tryLoadData(retryCount = 0) {
    await tryTakeoutLoadData(this, retryCount, () => this.showLocationGuide())
  },

  /**
   * 显示位置引导（页面内提示，不弹窗）
   */
  showLocationGuide() {
    showTakeoutLocationGuide(this)
  },

  onManualLocation() { this.openLocationPicker() },

  /**
   * 用户点击"重新定位"按钮
   */
  onRetryLocation() {
    retryTakeoutLocation(this)
  },

  /**
   * 打开位置选择器
   */
  openLocationPicker() {
    openTakeoutLocationPicker(this, () => this.loadData())
  },

  onHide() {
    // 页面隐藏时取消所有pending请求
    requestManager.cancelByContext(PAGE_CONTEXT)
    logger.debug('页面隐藏,已取消所有请求', undefined, 'Takeout.onHide')
  },

  onUnload() {
    // 页面卸载时清理
    requestManager.cancelByContext(PAGE_CONTEXT)
    this.resetFeedHydration()

    // 取消位置订阅
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

    // 记录本次加载时使用的坐标，供跨城检测对比
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
      // 如果是第一页重置加载失败，显示错误状态
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

      // 优先稳定首屏结构，再把详情和更多卡片放到后台渐进水合。
      const merchantIds = merchants.map((m) => m.id)
      this.scheduleMerchantHydration(merchantIds, generation)

      // 预加载商户封面图
      const { preloadImages } = require('../../utils/image')
      const imageUrls = feedItems.map((f) => f.imageUrl).filter(Boolean)
      setTimeout(() => preloadImages(imageUrls, false), 100)
    } catch (error) {
      logger.error('加载商家 Feed 失败', error, 'Takeout.loadFeed')
      throw error
    }
  },

  /**
   * 将卡片水合拆成优先批次和后台批次，避免第一页首屏产生逐店请求风暴。
   */
  scheduleMerchantHydration(merchantIds: number[], generation: number) {
    const priorityIds = merchantIds.slice(0, TAKEOUT_FIRST_SCREEN_MERCHANT_COUNT)
    const backgroundIds = merchantIds.slice(TAKEOUT_FIRST_SCREEN_MERCHANT_COUNT)

    this.queueHydrationPhase(priorityIds, generation, 0, this.hydrateMerchantDishesBatch)
    this.queueHydrationPhase(priorityIds, generation, TAKEOUT_PRIORITY_META_HYDRATION_DELAY_MS, this.hydrateMerchantMetaBatch)
    this.queueHydrationPhase(backgroundIds, generation, TAKEOUT_BACKGROUND_DISH_HYDRATION_DELAY_MS, this.hydrateMerchantDishesBatch)
    this.queueHydrationPhase(backgroundIds, generation, TAKEOUT_BACKGROUND_META_HYDRATION_DELAY_MS, this.hydrateMerchantMetaBatch)
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
        const [detail, hasOrdered] = await Promise.all([
          getPublicMerchantDetail(merchantId, true),
          getHasUserOrderedFromMerchant(merchantId)
        ])
        return { merchantId, detail, hasOrdered }
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

      const { detail, hasOrdered } = result.value

      const metaPatch = buildTakeoutMerchantMetaPatch(detail, hasOrdered)
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

  /**
   * Feed 卡片中的菜品加购事件
   */
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
    try {
      console.log('[updateCartDisplay] 开始调用API')
      // 直接从后端获取最新购物车汇总，确保数据准确
      const userCarts = await getUserCarts('takeout', { loading: false })

      console.log('[updateCartDisplay] API返回:', JSON.stringify(userCarts))

      const totalCount = userCarts.summary?.total_items || 0
      const totalPrice = userCarts.summary?.total_amount || 0

      console.log('[updateCartDisplay] 设置数据:', { totalCount, totalPrice })

      this.setData({
        cartTotalCount: totalCount,
        cartTotalPrice: totalPrice
      })

      // 同时更新 globalStore 供其他组件使用
      globalStore.set('cart', {
        items: [],
        totalCount,
        totalPrice,
        totalPriceDisplay: formatPrice(totalPrice)
      })
    } catch (error) {
      // API 调用失败时重置为 0
      console.error('[updateCartDisplay] 错误:', error)
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
    // 增加防抖和状态检查 (增加 _isLoading 私有变量锁)
    if (!this.data.hasMore || this.data.loading || this._isLoading) {
      return
    }

    // 增加时间戳防抖到 500ms
    const now = Date.now()
    if (this._lastLoadTime && now - this._lastLoadTime < 500) {
      return
    }
    this._lastLoadTime = now

    // 记录上一页页码以便加载失败时恢复
    const previousPage = this.data.page
    const nextPage = previousPage + 1
    
    // 设置新页码并加载
    this.setData({ page: nextPage })

    logger.debug('触发滚动加载', { nextPage }, 'Takeout.onReachBottom')

    this.loadData().catch((error) => {
      // 只有真正的请求错误才回滚（而不是被 loadData 内部 guard 拦截的情况）
      if (this.data.page === nextPage) {
        logger.error('加载更多失败，回滚页码', { nextPage, previousPage }, 'Takeout.onReachBottom')
        this.setData({ page: previousPage })
      }
      
      // 如果是 429 错误，显示更友好的提示
      if (isRateLimitError(error)) {
        wx.showToast({ title: '请求太频繁，请稍后再试', icon: 'none', duration: 2000 })
      } else {
        wx.showToast({ title: '加载失败，请重试', icon: 'none' })
      }
    })
  },

  _lastLoadTime: 0,
  _isLoading: false,

  /**
   * scroll-view 下拉刷新事件处理
   * 在 Skyline 模式下替代 onPullDownRefresh
   */
  async onRefresh() {
    // 刷新时清空搜索框，恢复推荐列表
    this.setData({ refresherTriggered: true, page: 1, searchKeyword: '' })

    try {
      await this.loadData()
    } finally {
      // 延迟关闭刷新动画，给用户视觉反馈
      setTimeout(() => {
        this.setData({ refresherTriggered: false })
      }, 300)
    }
  },

  /**
   * 页面下拉刷新事件（WebView 兼容）
   */
  onPullDownRefresh() {
    // 刷新时清空搜索框，恢复推荐列表
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
