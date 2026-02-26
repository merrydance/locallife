import { DishAdapter } from '../../adapters/dish'
import { Dish } from '../../models/dish'
import { Category } from '../../models/category'
import { searchDishes, DishSummary, DishSearchParams, ComboSummary, getRecommendedCombos } from '../../api/dish'
import CartService from '../../services/cart'
import { getUserCarts } from '../../api/cart'
import { searchMerchants, MerchantSummary, getPublicMerchantCombos, getPublicMerchantDishes, PublicCombo } from '../../api/merchant'
import { searchCombos, SearchComboItem } from '../../api/combo'
import Navigation from '../../utils/navigation'
import { logger } from '../../utils/logger'
import { ErrorHandler } from '../../utils/error-handler'
import { globalStore } from '../../utils/global-store'
import { requestManager } from '../../utils/request-manager'
import { getStableBarHeights } from '../../utils/responsive'
import { getPublicImageUrl } from '../../utils/image'
import { formatPrice } from '../../utils/util'

const PAGE_CONTEXT = 'takeout_index'
const PAGE_SIZE = 10  // 每页条数，用于无限滚动分页

interface RestaurantViewModel {
  id: number
  name: string
  imageUrl?: string
  cuisineType: string[]
  avgPrice: number
  avgPriceDisplay: string
  distance: string
  address: string
  businessHoursDisplay: string
  isOpen: boolean
  availableRooms: number
  availableRoomsBadge: string
  tags: string[]
  monthlySales: number
  deliveryFee?: number
  deliveryFeeDisplay: string
  promoText: string
  subsidyText: string
}

interface PackageViewModel {
  id: number
  name: string
  merchantId: number
  merchantName: string
  imageUrl: string
  price: number
  priceDisplay: string
  originalPrice: number
  originalPriceDisplay: string
  savingsPercent: number
  monthlySales: number
  salesBadge: string
  distance: string
  deliveryFee?: number
  deliveryFeeDisplay: string
  tags: string[]
  merchantIsOpen: boolean
  estimatedDeliveryTime?: number
  dishImages?: string[]
}

interface UserMessageError {
  userMessage?: string
}

type SearchTimer = ReturnType<typeof setTimeout>
type TakeoutTab = 'dishes' | 'restaurants' | 'packages'

function resolveTakeoutTab(eventDetail: Record<string, unknown> | string): TakeoutTab {
  const tabs: TakeoutTab[] = ['dishes', 'packages', 'restaurants']

  if (typeof eventDetail === 'string' && (tabs as string[]).includes(eventDetail)) {
    return eventDetail as TakeoutTab
  }

  const detail = (eventDetail || {}) as Record<string, unknown>
  const rawValue = detail.value
  if (typeof rawValue === 'string' && (tabs as string[]).includes(rawValue)) {
    return rawValue as TakeoutTab
  }

  const rawIndex = detail.index
  if (typeof rawIndex === 'number' && rawIndex >= 0 && rawIndex < tabs.length) {
    return tabs[rawIndex]
  }

  return 'dishes'
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
    activeTab: 'dishes' as 'dishes' | 'restaurants' | 'packages',
    dishes: [] as Dish[],
    restaurants: [] as RestaurantViewModel[],
    packages: [] as PackageViewModel[],
    categories: [] as Category[],
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
    isPrefetching: false,
    hasServiceProviders: true,
    // 首页 Banner
    banners: [
      {
        id: 1,
        title: '代取市场',
        subtitle: '骑手接受顾客委托到店代取，对顾客负责',
        tag: '权责明确',
        bg: 'linear-gradient(135deg, #FF6B00 0%, #FF9A3C 100%)',
        icon: '/assets/icons/dish.svg',
        url: ''
      },
      {
        id: 2,
        title: '店铺会员',
        subtitle: '成为店铺专属会员，余额也可在线支付',
        tag: '尊享权益',
        bg: 'linear-gradient(135deg, #1A1A2E 0%, #16213E 100%)',
        icon: '/assets/icons/wallet-safe.svg',
        url: '/pages/user_center/wallet/index'
      },
      {
        id: 3,
        title: '包间预订',
        subtitle: '到店前先预订，就餐更从容',
        tag: '极致体验',
        bg: 'linear-gradient(135deg, #0052D9 0%, #2979FF 100%)',
        icon: '/assets/icons/plate.svg',
        url: '/pages/reservation/index'
      }
    ],
    // 4宫格快捷入口
    quickEntries: [
      {
        id: 1,
        label: '外卖',
        icon: '/assets/icons/dish.svg',
        bg: 'rgba(255, 107, 0, 0.1)',
        url: ''
      },
      {
        id: 2,
        label: '预订',
        icon: '/assets/icons/plate.svg',
        bg: 'rgba(0, 82, 217, 0.1)',
        url: '/pages/reservation/index'
      },
      {
        id: 3,
        label: '会员卡',
        icon: '/assets/icons/wallet-safe.svg',
        bg: 'rgba(0, 137, 123, 0.1)',
        url: '/pages/user_center/wallet/index'
      },
      {
        id: 4,
        label: '优惠券',
        icon: '/assets/icons/coupon-ticket.svg',
        bg: 'rgba(255, 152, 0, 0.1)',
        url: '/pages/user_center/coupons/index'
      }
    ]
  },

  // 预加载缓存 (不放在 data 中以免触发渲染)
  _prefetchedDishes: [] as Dish[],
  _prefetchedRestaurants: [] as RestaurantViewModel[],
  _prefetchedPackages: [] as PackageViewModel[],
  _prefetchHasMore: true,
  _unsubscribeLocation: undefined as undefined | (() => void),

  onLoad() {
    wx.showShareMenu({
      withShareTicket: true,
      menus: ['shareAppMessage', 'shareTimeline']
    })

    // 设置导航栏高度和滚动区域高度
    const { navBarHeight } = getStableBarHeights()
    const windowInfo = wx.getWindowInfo()
    // windowHeight 已扣除原生 tabBar，只需扣除自定义导航栏
    const scrollViewHeight = windowInfo.windowHeight - navBarHeight

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

        // 如果还没有数据，位置更新后自动加载
        if (this.data.dishes.length === 0 && !this.data.loading) {
          logger.info('[Takeout] 位置已更新，开始加载数据', undefined, 'Takeout.onLoad')
          this.loadData()
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
        wx.showToast({ title: '已更新位置推荐', icon: 'success', duration: 1500 })
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

  // Gap 7: Banner 点击跳转
  onBannerTap(e: WechatMiniprogram.CustomEvent) {
    const { item } = e.currentTarget.dataset as { item: { url: string } }
    if (!item?.url) return

    const TAB_PAGES = [
      'pages/takeout/index',
      'pages/reservation/index',
      'pages/user_center/index',
      'pages/dining/index'
    ]
    const isTab = TAB_PAGES.some((p) => item.url.includes(p))

    if (isTab) {
      wx.switchTab({ url: item.url })
    } else {
      wx.navigateTo({ url: item.url })
    }
  },

  // Gap 7: 快捷入口点击跳转
  // tabBar 页面（外卖/预订/我的）用 switchTab，普通页用 navigateTo
  onQuickEntryTap(e: WechatMiniprogram.CustomEvent) {
    const { url } = e.currentTarget.dataset as { url: string }
    if (!url) return  // 外卖入口就在当前页，无需跳转

    const TAB_PAGES = [
      'pages/takeout/index',
      'pages/reservation/index',
      'pages/user_center/index',
      'pages/dining/index'
    ]
    const isTab = TAB_PAGES.some((p) => url.includes(p))

    if (isTab) {
      wx.switchTab({ url })
    } else {
      wx.navigateTo({ url })
    }
  },

  onShow() {
    logger.debug('[Takeout.onShow] 页面显示', {
      dishesCount: this.data.dishes.length,
      loading: this.data.loading
    }, 'Takeout.onShow')

    // 从购物车返回时更新购物车数据
    this.updateCartDisplay()

    // 更新位置显示
    const app = getApp<IAppOption>()
    const loc = app.globalData.location
    if (loc && loc.name) {
      this.setData({ address: loc.name })
    }

    // 检查是否需要加载数据
    if (this.data.dishes.length === 0) {
      logger.info('[Takeout.onShow] 开始 tryLoadData', undefined, 'Takeout.onShow')
      this.tryLoadData()
    } else {
      logger.debug('[Takeout.onShow] 跳过 tryLoadData，已有数据')
    }
  },

  // 尝试加载数据，等待 token 准备好，位置未授权则直接引导
  async tryLoadData(retryCount = 0) {
    const MAX_TOKEN_RETRIES = 10 // Token 最多等待 5 秒
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

        wx.showToast({ title: '位置已更新', icon: 'success', duration: 1500 })
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

    // 取消位置订阅
    if (this._unsubscribeLocation) {
      this._unsubscribeLocation()
    }
  },

  onLocationChange() {
    this.setData({ page: 1 })
    this.loadData()
  },

  onTabChange(e: WechatMiniprogram.CustomEvent) {
    const nextTab = resolveTakeoutTab((e.detail || {}) as Record<string, unknown>)
    this.setData(
      {
        activeTab: nextTab,
        page: 1
      },
      () => {
        // 切换 Tab 时重新加载对应类型的标签
        this.loadCategories()
        this.loadData()
      }
    )
  },

  async loadCategories() {
    // 发现型首页不再需要分类，留空或移除
  },

  async loadData() {
    if (this._isLoading) return // 仅检查物理请求锁，不检查 UI loading 标志

    this._isLoading = true
    this.setData({ loading: true, isError: false })

    try {
      const { activeTab, page } = this.data
      const reset = page === 1

      if (reset) {
        await this.refreshServiceProviderState()
      }

      if (activeTab === 'dishes') {
        await this.loadDishes(reset)
      } else if (activeTab === 'packages') {
        await this.loadPackages(reset)
      } else {
        await this.loadRestaurants(reset)
      }
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
    }
  },

  async loadDishes(reset = false) {
    if (reset) {
      // 重置时清空缓存和数据
      this._prefetchedDishes = []
      this._prefetchHasMore = true
      this.setData({
        page: 1,
        dishes: [],
        hasMore: true
      })
    }

    try {
      const app = getApp<IAppOption>()
      const currentPage = reset ? 1 : this.data.page
      let newDishes: Dish[] = []
      let hasMore = true

      // 1. 优先使用预加载的缓存数据（无延迟）
      if (!reset && this._prefetchedDishes.length > 0) {
        newDishes = this._prefetchedDishes
        hasMore = this._prefetchHasMore
        this._prefetchedDishes = [] // 清空缓存
      } else {
        // 2. 没有缓存则请求当前页
        // 获取选中的标签ID用于过滤（空字符串表示"全部"，不传 tag_id）
        const tagId = this.data.activeCategoryId ? parseInt(this.data.activeCategoryId) : null
        const params: DishSearchParams = {
          user_latitude: app.globalData.latitude || undefined,
          user_longitude: app.globalData.longitude || undefined,
          limit: PAGE_SIZE,
          page: currentPage
        }
        // 只有选择了具体标签时才传 tag_id
        if (tagId && !isNaN(tagId)) {
          params.tag_id = tagId
        }
        let result = await searchDishes(params)

        // 生产级兜底1：若带标签过滤导致空列表，自动重试一次无标签过滤
        if (currentPage === 1 && result.dishes.length === 0 && params.tag_id) {
          const { tag_id: _removedTag, ...retryParams } = params
          result = await searchDishes(retryParams)
        }

        newDishes = result.dishes.map((dish: DishSummary) => DishAdapter.fromSummaryDTO(dish))
        hasMore = result.has_more
      }

      // 更新视图
      if (reset) {
        this.setData({
          dishes: newDishes,
          hasMore
        })
      } else {
        this.setData({
          dishes: [...this.data.dishes, ...newDishes],
          hasMore
        })
      }

      // 3. 异步预加载下一页（不阻塞当前渲染）
      if (hasMore) {
        this.prefetchNextDishes(currentPage + 1)
      }

      // 预加载图片
      const { preloadImages } = require('../../utils/image')
      const imageUrls = newDishes.map((dish: Dish) => dish.imageUrl).filter(Boolean)
      setTimeout(() => {
        preloadImages(imageUrls, false)
      }, 100)
    } catch (error) {
      logger.error('加载菜品失败', error, 'Takeout.loadDishes')
      throw error
    }
  },

  async refreshServiceProviderState() {
    try {
      const app = getApp<IAppOption>()
      const merchants = await searchMerchants({
        keyword: '',
        page_id: 1,
        page_size: 1,
        user_latitude: app.globalData.latitude || undefined,
        user_longitude: app.globalData.longitude || undefined
      })

      this.setData({ hasServiceProviders: merchants.length > 0 })
    } catch (error) {
      logger.warn('探测区域商户状态失败，按有服务兜底展示', error, 'Takeout.refreshServiceProviderState')
      this.setData({ hasServiceProviders: true })
    }
  },

  /**
   * 预加载下一页菜品（后台静默执行）
   */
  async prefetchNextDishes(nextPage: number) {
    if (this.data.isPrefetching || this._prefetchedDishes.length > 0) return

    this.setData({ isPrefetching: true })
    try {
      const app = getApp<IAppOption>()
      const tagId = this.data.activeCategoryId ? parseInt(this.data.activeCategoryId) : null
      const params: DishSearchParams = {
        user_latitude: app.globalData.latitude || undefined,
        user_longitude: app.globalData.longitude || undefined,
        limit: PAGE_SIZE,
        page: nextPage
      }

      if (tagId && !isNaN(tagId)) {
        params.tag_id = tagId
      }

      const result = await searchDishes(params)
      this._prefetchedDishes = result.dishes.map((dish: DishSummary) => DishAdapter.fromSummaryDTO(dish))
      this._prefetchHasMore = result.has_more
    } catch (error) {
      logger.debug('预加载下一页失败', error, 'Takeout.prefetchNextDishes')
    } finally {
      this.setData({ isPrefetching: false })
    }
  },

  async loadRestaurants(reset = false) {
    if (reset) {
      this.setData({ page: 1, restaurants: [], hasMore: true })
    }

    try {
      // 使用搜索商户接口
      const app = getApp<IAppOption>()
      const currentPage = reset ? 1 : this.data.page
      const merchants = await searchMerchants({
        keyword: this.data.searchKeyword, // 使用当前搜索词，如果是空字符串则返回默认列表
        page_id: currentPage,
        page_size: PAGE_SIZE,
        user_latitude: app.globalData.latitude || undefined,
        user_longitude: app.globalData.longitude || undefined
      })

      // Backend returns array directly for searchMerchants in some versions, 
      // but wrapper in api/merchant.ts returns MerchantSummary[]
      // We need to handle total/hasMore if possible, but searchMerchants wrapper currently swallows it?
      // Checking api/merchant.ts, it returns response.merchants || []. 
      // Logic for hasMore might be missing in client wrapper if total isn't returned.
      // Assuming hasMore = length === PAGE_SIZE for now or update wrapper later.
      const hasMore = merchants.length === PAGE_SIZE

      const restaurantViewModels: RestaurantViewModel[] = merchants.map((m: MerchantSummary) => ({
        ...(deriveMerchantPromotions(m.tags || [], m.estimated_delivery_fee)),
        id: m.id,
        name: m.name,
        imageUrl: m.logo_url,
        cuisineType: m.tags ? m.tags.slice(0, 2) : [],
        avgPrice: 0,
        avgPriceDisplay: '人均未知',
        distance: DishAdapter.formatDistance(m.distance ?? 0),
        address: m.address || '',
        businessHoursDisplay: m.is_open === false ? '休息中' : '营业中',
        isOpen: m.is_open ?? true, // 商户营业状态（搜索接口未返回时默认营业）
        availableRooms: 0,
        availableRoomsBadge: '',
        tags: m.tags ? m.tags.slice(0, 3) : [],
        // New fields
        monthlySales: m.total_orders ?? m.monthly_sales ?? 0,
        deliveryFee: m.estimated_delivery_fee,
        deliveryFeeDisplay: m.estimated_delivery_fee !== undefined
          ? `配送费¥${(m.estimated_delivery_fee / 100).toFixed(0)}起`
          : ''
      }))


      if (reset) {
        this.setData({
          restaurants: restaurantViewModels,
          hasMore
        })
      } else {
        this.setData({
          restaurants: [...this.data.restaurants, ...restaurantViewModels],
          hasMore
        })
      }

      // 预加载图片
      const { preloadImages } = require('../../utils/image')
      const imageUrls = restaurantViewModels.map((r) => r.imageUrl).filter(Boolean)
      setTimeout(() => {
        preloadImages(imageUrls, false)
      }, 100)

    } catch (error) {
      logger.error('加载商户失败', error, 'Takeout.loadRestaurants')
      throw error
    }
  },

  async loadPackages(reset = false) {
    if (reset) {
      this.setData({ page: 1, packages: [], hasMore: true })
    }

    try {
      // 使用搜索套餐接口
      const app = getApp<IAppOption>()
      const currentPage = reset ? 1 : this.data.page
      // 构造搜索参数
      // 注意：getTags 返回的 id 是数字转字符串，searchCombos 可能需要数字？
      // 当前 backend searchCombos 只接受 keyword。Category 过滤暂不支持？
      // 如果 activeCategoryId 存在，可能需要作为 keyword 或者后续支持 category_id
      // 目前主要支持 keyword 搜索。

      const result = await searchCombos({
        keyword: this.data.searchKeyword,
        page_id: currentPage,
        page_size: PAGE_SIZE,
        user_latitude: app.globalData.latitude || undefined,
        user_longitude: app.globalData.longitude || undefined
      })

      const hasMore = result.combos.length === PAGE_SIZE // 简单分页逻辑，或使用 result.total

      const packageViewModels: PackageViewModel[] = result.combos.map((combo: SearchComboItem) => ({
        id: combo.id,
        name: combo.name,
        merchantId: combo.merchant_id,
        merchantName: combo.merchant_name || '',
        imageUrl: getPublicImageUrl(combo.image_url) || '/assets/placeholder_food.png',
        price: combo.combo_price,

        priceDisplay: formatPrice(combo.combo_price),
        originalPrice: combo.original_price,
        originalPriceDisplay: formatPrice(combo.original_price),
        savingsPercent: combo.savings_percent || 0,
        monthlySales: combo.monthly_sales || 0,
        salesBadge: `月售${combo.monthly_sales || 0}`,
        distance: DishAdapter.formatDistance(combo.distance),
        deliveryFee: combo.estimated_delivery_fee,
        deliveryFeeDisplay: combo.estimated_delivery_fee !== undefined
          ? `配送费¥${(combo.estimated_delivery_fee / 100).toFixed(0)}起`
          : '',
        tags: combo.tags || [],
        merchantIsOpen: combo.merchant_is_open ?? true,
        estimatedDeliveryTime: combo.estimated_delivery_time
      }))


      if (reset) {
        this.setData({
          packages: packageViewModels,
          hasMore
        })
      } else {
        this.setData({
          packages: [...this.data.packages, ...packageViewModels],
          hasMore
        })
      }

      // 异步解析套餐组合图
      this.resolveComboImages()

    } catch (error) {
      logger.error('加载套餐失败', error, 'Takeout.loadPackages')
      throw error
    }
  },

  onTabCategoryChange(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.detail
    this.setData({
      activeCategoryId: id,
      page: 1  // 重置页码
    })
    // 切换标签时重新加载数据（带标签过滤）
    this.loadData()
  },

  async onAddCart(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.detail
    const dish = this.data.dishes.find((d) => d.id === id)
    if (dish) {
      const success = await CartService.addItem({
        merchantId: String(dish.merchantId),
        dishId: String(dish.id)
      })

      if (success) {
        await this.updateCartDisplay()
        wx.showToast({ title: '已加入购物车', icon: 'success', duration: 500 })
      }
    }
  },

  /**
   * 套餐加购物车
   */
  async onAddComboToCart(e: WechatMiniprogram.TouchEvent) {
    const { id, merchantId } = e.currentTarget.dataset
    if (!id || !merchantId) {
      wx.showToast({ title: '套餐信息错误', icon: 'error' })
      return
    }

    const success = await CartService.addItem({
      merchantId: String(merchantId),
      comboId: String(id)
    })

    if (success) {
      await this.updateCartDisplay()
      wx.showToast({ title: '已加入购物车', icon: 'success', duration: 500 })
    }
  },

  async updateCartDisplay() {
    try {
      console.log('[updateCartDisplay] 开始调用API')
      // 直接从后端获取最新购物车汇总，确保数据准确
      const userCarts = await getUserCarts('takeout', { loading: false })

      console.log('[updateCartDisplay] API返回:', JSON.stringify(userCarts))

      const totalCount = userCarts.summary?.total_items || 0
      const totalPrice = userCarts.summary?.total_amount || 0

      console.log('[updateCartDisplay] 设置数据:', { totalCount, totalPrice, activeTab: this.data.activeTab })

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
     * 点击菜品卡片 - 跳转到菜品详情
     */
  onDishClick(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.detail
    // 查找对应菜品
    const dish = this.data.dishes.find((d) => String(d.id) === String(id))

    if (dish) {
      const monthSales = dish.salesBadge
        ? Number(dish.salesBadge.replace(/[^0-9]/g, ''))
        : undefined
      const distanceMeters = dish.distance_meters
      Navigation.toDishDetail(id, {
        shopName: dish.shopName,
        monthSales,
        distance: distanceMeters,
        estimatedDeliveryTime: Math.ceil((dish.estimated_delivery_time || 0) / 60) // 转换为分钟传递
      })
    } else {
      Navigation.toDishDetail(id)
    }
  },

  /**
     * 点击商户名称 - 跳转到商户详情
     */
  onMerchantClick(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.detail
    Navigation.toRestaurantDetail(id)
  },

  /**
     * 点击套餐卡片 - 跳转到套餐详情
     */
  onPackageTap(e: WechatMiniprogram.TouchEvent) {
    const id = e.currentTarget.dataset.id
    const combo = this.data.packages.find((p) => String(p.id) === String(id))

    if (combo) {
      Navigation.toComboDetail(String(id), {
        shopName: combo.merchantName,
        monthSales: combo.monthlySales,
        distance: Number(combo.distance.replace(/[^0-9.]/g, '')) * 1000, // 转换回米
        estimatedDeliveryTime: combo.estimatedDeliveryTime
      })
    } else {
      Navigation.toComboDetail(String(id))
    }
  },

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

  /**
     * 搜索功能 - 内联搜索，带防抖处理
     */
  onSearch(e: WechatMiniprogram.CustomEvent) {
    const keyword = e.detail.value?.trim() || ''

    if (this._searchTimer) {
      clearTimeout(this._searchTimer)
    }

    // 如果关键词为空，立即执行恢复逻辑
    if (!keyword) {
      this.setData({ searchKeyword: '' })
      this.loadData()
      return
    }

    this._searchTimer = setTimeout(async () => {
      this.setData({
        searchKeyword: keyword,
        loading: true
      })

      try {
        await this.searchInline(keyword)
      } catch (error) {
        console.error('搜索失败:', error)
        wx.showToast({ title: '搜索失败', icon: 'error' })
      } finally {
        this.setData({ loading: false })
      }
    }, 500)
  },

  _searchTimer: null as SearchTimer | null,

  /**
   * 内联搜索 - 根据当前 Tab 搜索对应内容
   */
  async searchInline(keyword: string) {
    const { activeTab } = this.data
    const app = getApp<IAppOption>()

    if (activeTab === 'dishes') {
      // 搜索菜品 - 使用推荐接口的 keyword 参数，返回格式与列表完全一致
      const result = await searchDishes({
        keyword,
        page: 1,
        limit: 20,
        user_latitude: app.globalData.latitude ?? undefined,
        user_longitude: app.globalData.longitude ?? undefined
      })

      // 使用 DishAdapter 转换为卡片展示格式（与列表加载保持一致）
      const adaptedDishes = result.dishes.map((dish: DishSummary) => DishAdapter.fromSummaryDTO(dish))

      this.setData({
        dishes: adaptedDishes,
        hasMore: result.has_more,
        page: 1
      })

      if (result.dishes.length === 0) {
        wx.showToast({ title: '未找到相关菜品', icon: 'none' })
      }

    } else if (activeTab === 'restaurants') {
      // 搜索餐厅 - 复用现有 searchMerchants API
      const result = await searchMerchants({
        keyword,
        page_id: 1,
        page_size: 20,
        user_latitude: app.globalData.latitude || undefined,
        user_longitude: app.globalData.longitude || undefined
      })

      // 转换为与列表一致的展示格式（与 loadRestaurants 保持一致）
      const restaurants: RestaurantViewModel[] = (result || []).map((m: MerchantSummary) => ({
        ...(deriveMerchantPromotions(m.tags || [], m.estimated_delivery_fee)),
        id: m.id,
        name: m.name,
        imageUrl: m.logo_url,
        cuisineType: m.tags ? m.tags.slice(0, 2) : [],
        avgPrice: 0,
        avgPriceDisplay: '人均未知',
        distance: DishAdapter.formatDistance(m.distance),
        address: m.address || '',
        businessHoursDisplay: m.is_open === false ? '休息中' : '营业中',
        isOpen: m.is_open ?? true,
        availableRooms: 0,
        availableRoomsBadge: '',
        tags: m.tags ? m.tags.slice(0, 3) : [],
        monthlySales: m.total_orders ?? m.monthly_sales ?? 0,
        deliveryFee: m.estimated_delivery_fee,
        deliveryFeeDisplay: m.estimated_delivery_fee !== undefined
          ? `配送费¥${(m.estimated_delivery_fee / 100).toFixed(0)}起`
          : ''
      }))

      this.setData({
        restaurants,
        hasMore: false,
        page: 1
      })

      if (restaurants.length === 0) {
        wx.showToast({ title: '未找到相关餐厅', icon: 'none' })
      }

    } else if (activeTab === 'packages') {
      // 搜索套餐 - 使用推荐接口的 keyword 参数，返回格式与列表完全一致
      const result = await getRecommendedCombos({
        keyword,
        page: 1,
        limit: 20
      })

      // 转换为与列表一致的展示格式（与 loadPackages 保持一致）
      const combos = result.combos || []
      const packageViewModels: PackageViewModel[] = combos.map((combo: ComboSummary) => ({
        id: combo.id,
        name: combo.name,
        merchantId: combo.merchant_id,
        merchantName: combo.merchant_name || '',
        price: combo.combo_price,
        priceDisplay: formatPrice(combo.combo_price),
        originalPrice: combo.original_price || combo.combo_price,
        originalPriceDisplay: formatPrice(combo.original_price || combo.combo_price),
        imageUrl: getPublicImageUrl(combo.image_url) || '/assets/placeholder_food.png',
        savingsPercent: combo.savings_percent || 0,
        monthlySales: combo.monthly_sales || 0,
        salesBadge: `月售${combo.monthly_sales || 0}`,
        distance: DishAdapter.formatDistance(combo.distance),
        deliveryFee: combo.estimated_delivery_fee,
        deliveryFeeDisplay: combo.estimated_delivery_fee !== undefined
          ? `配送费¥${(combo.estimated_delivery_fee / 100).toFixed(0)}起`
          : '',
        tags: combo.tags || [],
        merchantIsOpen: combo.merchant_is_open ?? true,
        estimatedDeliveryTime: combo.estimated_delivery_time
      }))

      this.setData({
        packages: packageViewModels,
        hasMore: result.has_more || false,
        page: 1
      })

      if (combos.length === 0) {
        wx.showToast({ title: '未找到相关套餐', icon: 'none' })
      }
    }
  },

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
      if (error?.message?.includes('429')) {
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

  /**
   * 异步解析套餐组合图 (针对套餐列表)
   */
  async resolveComboImages() {
    const { packages, activeTab } = this.data
    if (activeTab !== 'packages' || packages.length === 0) return

    const merchantIdsForCombos = new Set<number>()
    const comboIdsToMap = new Set<number>()

    // 1. 收集需要解析的 ID
    packages.forEach((pkg) => {
      if (!pkg.dishImages) {
        merchantIdsForCombos.add(pkg.merchantId)
        comboIdsToMap.add(pkg.id)
      }
    })

    if (merchantIdsForCombos.size === 0) return

    // 2. 并行获取商户详情以提取菜品图
    const comboCache = new Map<number, { dishImages: string[] }>()
    const fetchPromises: Promise<void>[] = []
    
    Array.from(merchantIdsForCombos).forEach((mid) => {
      fetchPromises.push(Promise.all([
        getPublicMerchantCombos(mid),
        getPublicMerchantDishes(mid)
      ]).then(([combosRes, dishesRes]) => {
        const merchantDishes = dishesRes.dishes || []
        if (combosRes.combos) {
          combosRes.combos.forEach((c: PublicCombo) => {
            if (comboIdsToMap.has(c.id)) {
              // 注入菜品图片
              const dishImages = (c.dishes || [])
                .map((cd) => {
                  const dish = merchantDishes.find((d) => d.id === cd.dish_id)
                  return dish?.image_url
                })
                .filter((url): url is string => Boolean(url))
                .map((url: string) => getPublicImageUrl(url))
              
              comboCache.set(c.id, { dishImages })
            }
          })
        }
      }).catch((err: unknown) => {
        logger.warn('Resolve packages failed for merchant', { mid, err })
      }))
    })

    await Promise.all(fetchPromises)

    // 3. 应用结果
    let hasUpdates = false
    const updatedPackages = packages.map((pkg) => {
      // 只有在还没有解析过或者是重新加载时才更新
      if (comboCache.has(pkg.id)) {
        const combo = comboCache.get(pkg.id)
        if (!combo) return pkg
        let resolvedImages = (combo.dishImages || []) as string[]
        
        // 如果解析出来的菜品图太少，或者为了美观，把封面的套餐图也加进去
        if (resolvedImages.length > 0 && resolvedImages.length < 4) {
             // 检查封面图是否有效且不在列表中
             if (pkg.imageUrl && !pkg.imageUrl.includes('placeholder') && !resolvedImages.includes(pkg.imageUrl)) {
                 resolvedImages = [pkg.imageUrl, ...resolvedImages].slice(0, 4)
             }
        }

        if (resolvedImages.length > 0) {
          hasUpdates = true
          return { ...pkg, dishImages: resolvedImages }
        }
      }
      return pkg
    })

    if (hasUpdates) {
      this.setData({ packages: updatedPackages })
    }
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
