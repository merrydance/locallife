import { searchMerchants, MerchantSummary } from '../../../api/merchant'
import { getUserCarts } from '../../../api/cart'
import { DishAdapter } from '../../../adapters/dish'
import { logger } from '../../../utils/logger'
import { globalStore } from '../../../utils/global-store'
import { getStableBarHeights } from '../../../utils/responsive'
import { formatPrice } from '../../../utils/util'

const PAGE_SIZE = 10

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

function deriveMerchantPromotions(tags: string[] = [], deliveryFee?: number) {
  const promoTag = tags.find((tag) => /促销|满减|折扣|优惠|券/.test(tag)) || ''
  let subsidyTag = tags.find((tag) => /补贴|免配送|免运费|运费减免|配送补贴/.test(tag)) || ''
  if (!subsidyTag && deliveryFee === 0) {
    subsidyTag = '运费补贴'
  }
  return { promoText: promoTag, subsidyText: subsidyTag }
}

Page({
  data: {
    categoryName: '',
    tagId: 0,
    merchants: [] as RestaurantViewModel[],
    loading: true,
    hasMore: true,
    page: 1,
    navBarHeight: 88,
    scrollViewHeight: 600,
    refresherTriggered: false,
    cartTotalCount: 0,
    cartTotalPrice: 0
  },

  _tagId: 0,
  _isLoading: false,
  _lastLoadTime: 0,
  _unsubscribeCart: null as (() => void) | null,

  onLoad(options: Record<string, string>) {
    const tagId = parseInt(options.tag_id || '0', 10)
    const name = decodeURIComponent(options.name || '')

    this._tagId = tagId

    const { navBarHeight } = getStableBarHeights()
    const windowInfo = wx.getWindowInfo()
    const scrollViewHeight = windowInfo.windowHeight - navBarHeight

    this.setData({ tagId, categoryName: name, navBarHeight, scrollViewHeight })

    this.loadMerchants(true)
    this.updateCartDisplay()

    // 订阅购物车变化
    this._unsubscribeCart = globalStore.subscribe('cart', (cart) => {
      this.setData({
        cartTotalCount: cart.totalCount || 0,
        cartTotalPrice: cart.totalPrice || 0
      })
    })
  },

  onUnload() {
    if (this._unsubscribeCart) {
      this._unsubscribeCart()
      this._unsubscribeCart = null
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    const navBarHeight: number = e.detail.navBarHeight
    const windowInfo = wx.getWindowInfo()
    const scrollViewHeight = windowInfo.windowHeight - navBarHeight
    this.setData({ navBarHeight, scrollViewHeight })
  },

  async loadMerchants(reset = false) {
    if (this._isLoading) return
    if (!reset && !this.data.hasMore) return

    this._isLoading = true
    const app = getApp<IAppOption>()
    const currentPage = reset ? 1 : this.data.page

    if (reset) {
      this.setData({ loading: true, page: 1, merchants: [], hasMore: true })
    } else {
      this.setData({ loading: true })
    }

    try {
      const merchants = await searchMerchants({
        tag_id: this._tagId,
        page_id: currentPage,
        page_size: PAGE_SIZE,
        user_latitude: app.globalData.latitude || undefined,
        user_longitude: app.globalData.longitude || undefined
      })

      const hasMore = merchants.length === PAGE_SIZE

      const viewModels: RestaurantViewModel[] = merchants.map((m: MerchantSummary) => ({
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

      if (reset) {
        this.setData({ merchants: viewModels, hasMore, page: 1 })
      } else {
        this.setData({
          merchants: [...this.data.merchants, ...viewModels],
          hasMore,
          page: currentPage
        })
      }
    } catch (error) {
      logger.error('加载品类商户失败', error, 'CategoryPage.loadMerchants')
    } finally {
      this.setData({ loading: false })
      this._isLoading = false
    }
  },

  onReachBottom() {
    if (!this.data.hasMore || this.data.loading || this._isLoading) return

    const now = Date.now()
    if (now - this._lastLoadTime < 500) return
    this._lastLoadTime = now

    const nextPage = this.data.page + 1
    this.setData({ page: nextPage })

    this.loadMerchants(false).catch((error) => {
      this.setData({ page: nextPage - 1 })
      if (error?.message?.includes('429')) {
        wx.showToast({ title: '请求太频繁，请稍后再试', icon: 'none', duration: 2000 })
      } else {
        wx.showToast({ title: '加载失败，请重试', icon: 'none' })
      }
    })
  },

  async onRefresh() {
    this.setData({ refresherTriggered: true })
    try {
      await this.loadMerchants(true)
    } finally {
      setTimeout(() => {
        this.setData({ refresherTriggered: false })
      }, 300)
    }
  },

  async updateCartDisplay() {
    try {
      const userCarts = await getUserCarts('takeout', { loading: false })
      const totalCount = userCarts.summary?.total_items || 0
      const totalPrice = userCarts.summary?.total_amount || 0
      this.setData({ cartTotalCount: totalCount, cartTotalPrice: totalPrice })
      globalStore.set('cart', {
        items: [],
        totalCount,
        totalPrice,
        totalPriceDisplay: formatPrice(totalPrice)
      })
    } catch {
      // 静默失败
    }
  },

  onCartTap() {
    wx.switchTab({ url: '/pages/cart/index' })
  }
})
