import ConsumerDiscoveryAdapter from '../../../adapters/consumer-discovery'
import { searchMerchantsWithMeta, MerchantSummary } from '../../../api/merchant'
import { getUserCarts } from '../../../api/cart'
import { logger } from '../../../utils/logger'
import { isRateLimitError } from '../../../utils/user-facing'
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
  displayTags: string[]
  monthlySales: number
  deliveryFee?: number
  deliveryFeeDisplay: string
  promoText: string
  subsidyText: string
  label?: string  // 推荐 / 热销
}

function deriveMerchantPromotions(tags: string[] = [], deliveryFee?: number) {
  const promoTag = tags.find((tag) => /促销|满减|折扣|优惠|券/.test(tag)) || ''
  let subsidyTag = tags.find((tag) => /补贴|免代取|免运费|运费减免|代取补贴/.test(tag)) || ''
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
    const scrollViewHeight = windowInfo.windowHeight

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
    const scrollViewHeight = windowInfo.windowHeight
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
      const result = await searchMerchantsWithMeta({
        tag_id: this._tagId,
        sort_by: 'distance',
        page_id: currentPage,
        page_size: PAGE_SIZE,
        user_latitude: app.globalData.latitude || undefined,
        user_longitude: app.globalData.longitude || undefined
      })

      const merchants = result.merchants
      const hasMore = result.hasMore

      const viewModels: RestaurantViewModel[] = merchants.map((m: MerchantSummary) => {
        const merchant = ConsumerDiscoveryAdapter.toMerchantSummaryViewModel(m)
        return {
          ...deriveMerchantPromotions(merchant.tags, merchant.deliveryFee),
          id: merchant.id,
          name: merchant.name,
          imageUrl: merchant.imageUrl,
          cuisineType: merchant.tags.slice(0, 2),
          avgPrice: 0,
          avgPriceDisplay: '人均未知',
          distance: merchant.distanceDisplay,
          address: merchant.address,
          businessHoursDisplay: merchant.isOpen ? '营业中' : '休息中',
          isOpen: merchant.isOpen,
          availableRooms: 0,
          availableRoomsBadge: '',
          tags: merchant.tags.slice(0, 3),
          displayTags: merchant.displayTags.slice(0, 3),
          monthlySales: merchant.monthlySales,
          deliveryFee: merchant.deliveryFee,
          deliveryFeeDisplay: merchant.deliveryFeeDisplay,
          label: merchant.label
        }
      })

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
      if (isRateLimitError(error)) {
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
    wx.navigateTo({ url: '/pages/takeout/cart/index' })
  }
})
