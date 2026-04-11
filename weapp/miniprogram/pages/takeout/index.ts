import ConsumerDiscoveryAdapter from '../../adapters/consumer-discovery'
import CartService from '../../services/cart'
import { getUserCarts } from '../../api/cart'
import { searchMerchantsWithMeta, MerchantSummary, getPublicMerchantDishes, getPublicMerchantDetail, getHasUserOrderedFromMerchant, PublicDiscountRule, PublicVoucher, PublicDeliveryPromotion } from '../../api/merchant'
import { getActiveCategories } from '../../api/location'
import Navigation from '../../utils/navigation'
import { logger } from '../../utils/logger'
import { isRateLimitError } from '../../utils/user-facing'
import { ErrorHandler } from '../../utils/error-handler'
import { globalStore } from '../../utils/global-store'
import { requestManager } from '../../utils/request-manager'
import { getStableBarHeights } from '../../utils/responsive'
import { getPublicImageUrl } from '../../utils/image'
import { settleAll } from '../../utils/promise'
import { formatPrice } from '../../utils/util'
import { buildMerchantDisplayTags } from '../../adapters/merchant-labels'
import { buildTakeoutCategoryGridItems, TakeoutCategoryGridItem } from '../../adapters/takeout-categories'

const PAGE_CONTEXT = 'takeout_index'
const PAGE_SIZE = 10  // 每页条数，用于无限滚动分页
const DEFAULT_AVG_PREP_MINUTES = 15
const FIRST_SCREEN_MERCHANT_COUNT = 3
const HYDRATION_BATCH_SIZE = 2
const PRIORITY_META_HYDRATION_DELAY_MS = 260
const BACKGROUND_DISH_HYDRATION_DELAY_MS = 700
const BACKGROUND_META_HYDRATION_DELAY_MS = 1200

interface FeaturedDish {
  id: number
  name: string
  imageUrl: string
  priceDisplay: string
  price: number
  merchantId: number
  customization_groups?: unknown[]
}

interface MerchantFeedViewModel {
  id: number
  name: string
  imageUrl: string
  isOpen: boolean
  isOrderingSuspended: boolean
  distance: string
  monthlySales: number
  deliveryFeeDisplay: string
  promoText: string
  subsidyText: string
  tags: string[]
  systemLabels: string[]
  displayTags: string[]
  featuredDishes: FeaturedDish[]
  dishesLoading: boolean
  // 详情接口补充字段（异步填充）
  avgPrepMinutes: number        // 平均出餐时间（分钟）
  discountPromoText: string     // 满减文案，e.g. "满30减5"
  voucherText: string           // 券文案，e.g. "领券减5元"
  deliveryPromoText: string     // 运费优惠文案，e.g. "满30免运费"
  isNewStore: boolean           // 入驻30天内
  hasOrdered: boolean           // 当前用户曾成功下单
  detailLoading: boolean        // 详情是否仍在加载
  label?: string                // 推荐 / 热销
}

interface UserMessageError {
  userMessage?: string
}

type SearchTimer = ReturnType<typeof setTimeout>

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function chunkArray<T>(items: T[], size: number): T[][] {
  const chunks: T[][] = []
  for (let index = 0; index < items.length; index += size) {
    chunks.push(items.slice(index, index + size))
  }
  return chunks
}

function deriveMerchantPromotions(tags: string[] = [], deliveryFee?: number) {
  const promoTag = tags.find((tag) => /促销|满减|折扣|优惠|券/.test(tag)) || ''
  let subsidyTag = tags.find((tag) => /补贴|免配送|免运费|运费减免|配送补贴/.test(tag)) || ''

  if (!subsidyTag && deliveryFee === 0) {
    subsidyTag = '运费补贴'
  }

  return {
    promoText: promoTag,
    subsidyText: subsidyTag
  }
}

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

  onMerchantRegister() {
    wx.navigateTo({
      url: '/pages/register/merchant/index'
    })
  },

  onOperatorRegister() {
    wx.navigateTo({
      url: '/pages/register/operator/index'
    })
  },

  // Gap 1: 搜索框点击跳转到独立搜索页（不再直接聚焦）
  onSearchTap() {
    wx.navigateTo({ url: '/pages/takeout/search/index' })
  },

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
    const MAX_TOKEN_RETRIES = 20 // Token 最多等待 10 秒
    const RETRY_INTERVAL = 500
    const { getToken } = require('../../utils/auth')
    const app = getApp<IAppOption>()

    const token = getToken()
    const hasLocation = !!(app.globalData.latitude && app.globalData.longitude)

    // 1. 先等待 Token（登录通常很快）
    if (!token) {
      if (retryCount >= MAX_TOKEN_RETRIES) {
        logger.error('❌ 登录超时', { waitedTime: `${(retryCount * RETRY_INTERVAL) / 1000}秒` }, 'Takeout.tryLoadData')
        wx.showModal({
          title: '登录超时',
          content: '请检查网络连接后重试',
          confirmText: '重新加载',
          success: (res) => {
            if (res.confirm) {
              wx.reLaunch({ url: '/pages/takeout/index' })
            }
          }
        })
        return
      }

      if (retryCount === 0) {
        logger.info('等待登录...', undefined, 'Takeout.tryLoadData')
      }

      setTimeout(() => this.tryLoadData(retryCount + 1), RETRY_INTERVAL)
      return
    }

    // 2. Token 已就绪，检查位置
    if (!hasLocation) {
      // 位置未授权，直接显示引导，不再疯狂重试
      logger.info('位置未授权，显示引导界面', undefined, 'Takeout.tryLoadData')
      this.showLocationGuide()
      return
    }

    // 3. Token 和位置都准备好，加载数据
    logger.info('✅ Token 和位置都已准备好，开始加载数据', {
      tokenLength: token.length,
      locationName: app.globalData.location?.name || '未知'
    }, 'Takeout.tryLoadData')

    this.loadData()
  },

  /**
   * 显示位置引导（页面内提示，不弹窗）
   */
  showLocationGuide() {
    // 设置页面状态，显示位置引导提示
    this.setData({
      needLocation: true,
      loading: false,
      address: '请先定位'
    })
    logger.info('显示位置引导提示', undefined, 'Takeout.showLocationGuide')
  },

  /**
   * 用户点击"手动定位"按钮
   */
  onManualLocation() {
    this.openLocationPicker()
  },

  /**
   * 用户点击"重新定位"按钮
   */
  onRetryLocation() {
    const app = getApp<IAppOption>()
    this.setData({ address: '定位中...' })

    // 重新获取位置
    app.getLocationCoordinates()

    // 位置更新后会通过 globalStore 订阅自动触发 loadData
  },

  /**
   * 打开位置选择器
   */
  openLocationPicker() {
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

        // 同步到 globalStore
        const { globalStore } = require('../../utils/global-store')
        globalStore.updateLocation(
          res.latitude,
          res.longitude,
          res.name || res.address,
          res.address
        )

        logger.info('用户手动选择位置', {
          latitude: res.latitude,
          longitude: res.longitude,
          name: res.name
        }, 'Takeout.openLocationPicker')

        // 更新导航栏显示
        this.setData({ address: res.name || res.address })

        // 重新加载数据
        this.loadData()
      },
      fail: (err) => {
        logger.warn('用户取消选择位置', err, 'Takeout.openLocationPicker')

        // 用户取消，再次提示
        wx.showModal({
          title: '需要位置信息',
          content: '本地生活服务必须基于您的位置才能使用',
          confirmText: '重新选择',
          cancelText: '退出',
          success: (res) => {
            if (res.confirm) {
              this.openLocationPicker()
            } else {
              wx.switchTab({ url: '/pages/user_center/index' })
            }
          }
        })
      }
    })
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

  onLocationChange() {
    this.setData({ page: 1 })
    this.loadData()
  },

  async loadCategories() {
    const app = getApp<IAppOption>()
    if (!app.globalData.latitude || !app.globalData.longitude) return

    // 需要 token 才能请求，token 未就绪时跳过，待 loadData 时自然会再次调用
    const { getToken } = require('../../utils/auth')
    if (!getToken()) return

    try {
      const rawList = await getActiveCategories({
        user_latitude: app.globalData.latitude,
        user_longitude: app.globalData.longitude
      })
      const MAX_CATEGORIES = 8
      const availableCategories = rawList.slice(0, MAX_CATEGORIES)
      const hasRealCategories = availableCategories.length > 0
      const availableCategoryIds = new Set(availableCategories.map((category) => String(category.id)))
      const activeCategoryId = this.data.activeCategoryId && availableCategoryIds.has(this.data.activeCategoryId)
        ? this.data.activeCategoryId
        : ''

      this.setData({
        activeCategoryId,
        cuisineCategories: hasRealCategories
          ? buildTakeoutCategoryGridItems(availableCategories, activeCategoryId)
          : []
      })
    } catch (e) {
      // 加载失败不影响主流程，品类网格不显示即可
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
        page_size: PAGE_SIZE,
        user_latitude: app.globalData.latitude || undefined,
        user_longitude: app.globalData.longitude || undefined
      })

      const merchants = result.merchants
      const hasMore = result.hasMore

      const feedItems: MerchantFeedViewModel[] = merchants.map((m: MerchantSummary) => {
        const merchantSummary = ConsumerDiscoveryAdapter.toMerchantSummaryViewModel(m)
        // 计算"新店"：入驻30天内
        const isNewStore = m.created_at
          ? (Date.now() - new Date(m.created_at).getTime()) < 30 * 24 * 60 * 60 * 1000
          : false

        return {
          ...deriveMerchantPromotions(merchantSummary.tags, merchantSummary.deliveryFee),
          id: merchantSummary.id,
          name: merchantSummary.name,
          imageUrl: merchantSummary.imageUrl,
          isOpen: merchantSummary.isOpen,
          isOrderingSuspended: false,
          distance: merchantSummary.distanceDisplay,
          monthlySales: merchantSummary.monthlySales,
          deliveryFeeDisplay: merchantSummary.deliveryFeeDisplay,
          tags: merchantSummary.tags,
          systemLabels: merchantSummary.systemLabels,
          displayTags: merchantSummary.displayTags.slice(0, 3),
          featuredDishes: [],
          dishesLoading: true,
          avgPrepMinutes: DEFAULT_AVG_PREP_MINUTES,
          discountPromoText: '',
          voucherText: '',
          deliveryPromoText: '',
          isNewStore,
          hasOrdered: false,
          detailLoading: true,
          label: merchantSummary.label
        }
      })

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
    const priorityIds = merchantIds.slice(0, FIRST_SCREEN_MERCHANT_COUNT)
    const backgroundIds = merchantIds.slice(FIRST_SCREEN_MERCHANT_COUNT)

    this.queueHydrationPhase(priorityIds, generation, 0, this.hydrateMerchantDishesBatch)
    this.queueHydrationPhase(priorityIds, generation, PRIORITY_META_HYDRATION_DELAY_MS, this.hydrateMerchantMetaBatch)
    this.queueHydrationPhase(backgroundIds, generation, BACKGROUND_DISH_HYDRATION_DELAY_MS, this.hydrateMerchantDishesBatch)
    this.queueHydrationPhase(backgroundIds, generation, BACKGROUND_META_HYDRATION_DELAY_MS, this.hydrateMerchantMetaBatch)
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
    for (const chunk of chunkArray(merchantIds, HYDRATION_BATCH_SIZE)) {
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

      const featuredDishes: FeaturedDish[] = dishesResp.dishes.slice(0, 4).map((dish: import('../../api/merchant').PublicDish) => ({
        id: dish.id,
        name: dish.name,
        imageUrl: getPublicImageUrl(dish.image_url || '') || '/assets/placeholder_food.png',
        priceDisplay: formatPrice(dish.price),
        price: dish.price,
        merchantId,
        customization_groups: dish.customization_groups || []
      }))

      updates[`merchantFeed[${feedIndex}].featuredDishes`] = featuredDishes
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

      updates[`merchantFeed[${feedIndex}].avgPrepMinutes`] =
        (detail.avg_prep_minutes && detail.avg_prep_minutes > 0)
          ? detail.avg_prep_minutes
          : DEFAULT_AVG_PREP_MINUTES
      updates[`merchantFeed[${feedIndex}].tags`] = detail.tags || []
      updates[`merchantFeed[${feedIndex}].systemLabels`] = detail.system_labels || []
      updates[`merchantFeed[${feedIndex}].displayTags`] = buildMerchantDisplayTags(
        detail.system_labels || [],
        detail.tags || [],
        3
      )
      updates[`merchantFeed[${feedIndex}].isOrderingSuspended`] = !!detail.is_ordering_suspended
      updates[`merchantFeed[${feedIndex}].discountPromoText`] = this.buildDiscountPromoText(detail.discount_rules)
      updates[`merchantFeed[${feedIndex}].voucherText`] = this.buildVoucherText(detail.vouchers)
      updates[`merchantFeed[${feedIndex}].deliveryPromoText`] = this.buildDeliveryPromoText(detail.delivery_promotions)
      updates[`merchantFeed[${feedIndex}].hasOrdered`] = hasOrdered
    })

    if (Object.keys(updates).length > 0) {
      this.setData(updates)
    }
  },

  buildDiscountPromoText(discountRules?: PublicDiscountRule[]) {
    if (!discountRules || discountRules.length === 0) {
      return ''
    }

    const best = discountRules.reduce((current, next) => current.min_order_amount <= next.min_order_amount ? current : next)
    return `满${(best.min_order_amount / 100).toFixed(0)}减${(best.discount_amount / 100).toFixed(0)}`
  },

  buildVoucherText(vouchers?: PublicVoucher[]) {
    if (!vouchers || vouchers.length === 0) {
      return ''
    }

    const best = vouchers.reduce((current, next) => current.amount >= next.amount ? current : next)
    return `领券减${(best.amount / 100).toFixed(0)}元`
  },

  buildDeliveryPromoText(deliveryPromotions?: PublicDeliveryPromotion[]) {
    if (!deliveryPromotions || deliveryPromotions.length === 0) {
      return ''
    }

    const best = deliveryPromotions.reduce((current, next) => current.min_order_amount <= next.min_order_amount ? current : next)
    const threshold = best.min_order_amount
    return threshold === 0
      ? '免运费'
      : `满${(threshold / 100).toFixed(0)}免运费`
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

  /**
   * Feed 卡片中的商户点击事件
   */
  onMerchantTapFromFeed(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.detail as { id: number }
    Navigation.toRestaurantDetail(String(id))
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

  // ==================== 导航方法 ====================

  /**
   * 点击购物车 - 跳转到购物车页
   */
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
