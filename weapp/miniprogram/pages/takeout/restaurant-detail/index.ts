/**
 * 餐厅详情页面
 * 使用真实后端API
 */

import CartService from '../../../services/cart'
import {
  buildCustomerMerchantCouponViews,
  claimCustomerCoupon,
  loadCustomerMerchantCombos,
  loadCustomerMerchantDetail,
  loadCustomerMerchantDishes,
  loadCustomerMerchantRooms,
  loadTakeoutCartSummary,
  markCustomerCouponClaimed,
  type CustomerCombo,
  type CustomerDish,
  type CustomerDishCategory,
  type CustomerMerchantCouponView,
  type CustomerMerchantDetail,
  type CustomerRoom
} from '../../../services/customer-discovery-workflow'
import ConsumerMerchantDetailAdapter, { type ConsumerMerchantDetailViewModel } from '../../../adapters/consumer-merchant-detail'
import { getPublicImageUrl } from '../../../utils/image'
import { formatPriceNoSymbol } from '../../../utils/util'
import Navigation from '../../../utils/navigation'
import {
  resolveRestaurantHeaderCollapsed,
  type RestaurantHeaderScrollDirection
} from '../../../utils/restaurant-detail-header'
import { getErrorUserMessage } from '../../../utils/user-facing'

type RestaurantViewModel = ConsumerMerchantDetailViewModel

interface DishView {
  id: number
  name: string
  image_url: string
  price: number
  priceDisplay: string
  member_price?: number
  memberPriceDisplay: string | null
  original_price?: number
  originalPriceDisplay: string | null
  category_id: number
  category_name: string
  monthly_sales: number
  prepare_time: number
  tags: string[]
  is_available: boolean
  hasCustomizations: boolean
}

interface ComboView {
  id: number
  name: string
  description: string
  image_url: string
  combo_price: number
  comboPriceDisplay: string
  original_price: number
  originalPriceDisplay: string
  savingsDisplay: string
  dishes: CustomerCombo['dishes']
  dish_images: string[] // 新增：包含菜品的图片列表
}

const getErrorMessage = getErrorUserMessage

const ORDERING_SUSPENDED_MESSAGE = '当前商户暂停接单，请稍后再试'
const STORE_CLOSED_MESSAGE = '当前店铺暂未营业，仅支持查看店铺信息'

type RestaurantPageOptions = {
  id?: string
  scene?: string
  activeTab?: 'dishes' | 'combos' | 'rooms'
}

function resolveRestaurantId(options: RestaurantPageOptions): string {
  const directId = (options.id || '').trim()
  if (directId) {
    return directId
  }

  const rawScene = options.scene ? decodeURIComponent(options.scene) : ''
  if (!rawScene) {
    return ''
  }

  const merchantMatch = rawScene.match(/(?:^|-)m_(\d+)(?:-|$)/)
  if (merchantMatch) {
    return merchantMatch[1]
  }

  if (/^\d+$/.test(rawScene)) {
    return rawScene
  }

  return ''
}

Page({
  data: {
    restaurantId: '',
    restaurant: null as RestaurantViewModel | null,
    activeTab: 'dishes' as 'dishes' | 'combos' | 'rooms',
    activeCategoryId: '' as string | number,
    categories: [] as CustomerDishCategory[],
    dishes: [] as DishView[],
    filteredDishes: [] as DishView[],
    combos: [] as ComboView[],
    rooms: [] as CustomerRoom[],
    coupons: [] as CustomerMerchantCouponView[],
    couponLoading: false,
    couponError: '',
    claimingCouponId: 0,
    cartCount: 0,
    cartPrice: 0,
    cartPriceDisplay: '0.00',
    navBarHeight: 88,
    loading: true,
    isError: false,
    errorMsg: '',
    headerCollapsed: false,
    headerTouchY: 0,
    headerScrollDirection: 'none' as RestaurantHeaderScrollDirection
  },

  onLoad(options: RestaurantPageOptions) {
    wx.showShareMenu({
      withShareTicket: true,
      menus: ['shareAppMessage', 'shareTimeline']
    })

    const restaurantId = resolveRestaurantId(options)
    if (!restaurantId) {
      wx.showToast({ title: '商家信息缺失', icon: 'error' })
      setTimeout(() => wx.navigateBack(), 1500)
      return
    }
    this.setData({
      restaurantId,
      activeTab: options.activeTab && ['dishes', 'combos', 'rooms'].includes(options.activeTab)
        ? options.activeTab
        : 'dishes'
    })
    this.loadRestaurantDetail()
  },

  onShow() {
    this.updateCartDisplay()
  },

  onMerchantInfoTap() {
    const { restaurantId } = this.data
    if (!restaurantId) return
    wx.navigateTo({ url: `/pages/takeout/merchant-info/index?id=${restaurantId}` })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadRestaurantDetail() {
    this.setData({ loading: true, isError: false })

    try {
      const merchantId = parseInt(this.data.restaurantId)

      // 并行加载商户信息、菜品、套餐和包间
      const [merchantResult, dishesResult, combosResultFirstPass, roomsResult] = await Promise.all([
        this.loadMerchantInfo(merchantId),
        this.loadDishes(merchantId),
        this.loadCombos(merchantId),
        this.loadRooms(merchantId)
      ])

      if (!merchantResult) {
        this.setData({ 
          loading: false, 
          isError: true, 
          errorMsg: '商家信息不存在或已下架' 
        })
        return
      }

      // 第二次处理套餐，注入菜品图片
      const combosResult = combosResultFirstPass.map((combo) => {
        const dishImages = (combo.dishes || [])
          .map((cd) => dishesResult.dishes.find((d) => d.id === cd.dish_id)?.image_url)
          .filter(Boolean) as string[]
        return { ...combo, dish_images: dishImages }
      })

      // 从菜品中提取分类
      const categories = this.extractCategories(dishesResult)
      const firstCategoryId = categories[0]?.id || ''
      const coupons = buildCustomerMerchantCouponViews(merchantResult.vouchers)

      this.setData({
        restaurant: merchantResult,
        categories,
        dishes: dishesResult.dishes,
        combos: combosResult,
        rooms: roomsResult,
        coupons,
        couponError: '',
        activeCategoryId: firstCategoryId,
        loading: false
      })

      this.filterDishes()
    } catch (error: unknown) {
      console.error('加载商户详情失败:', error)
      this.setData({ 
        loading: false, 
        isError: true, 
        errorMsg: getErrorMessage(error, '数据请求失败，请检查网络')
      })
    }
  },

  async onClaimCoupon(e: WechatMiniprogram.CustomEvent) {
    const couponId = Number(e.detail?.id)
    if (!Number.isFinite(couponId) || this.data.claimingCouponId) return

    this.setData({ claimingCouponId: couponId })
    try {
      await claimCustomerCoupon(couponId)
      this.setData({
        coupons: markCustomerCouponClaimed(this.data.coupons, couponId),
        claimingCouponId: 0,
        couponError: ''
      })
      wx.showToast({ title: '已领取', icon: 'success' })
    } catch (error) {
      this.setData({ claimingCouponId: 0 })
      wx.showToast({ title: getErrorMessage(error, '领取失败，请稍后重试'), icon: 'none' })
    }
  },

  async loadMerchantInfo(merchantId: number): Promise<RestaurantViewModel | null> {
    try {
      const merchant: CustomerMerchantDetail = await loadCustomerMerchantDetail(merchantId)
      return merchant ? ConsumerMerchantDetailAdapter.toViewModel(merchant) : null
    } catch (error) {
      console.error('加载商户信息失败:', error)
      return null
    }
  },

  async loadDishes(merchantId: number): Promise<{ dishes: DishView[], categories: CustomerDishCategory[] }> {
    try {
      const result = await loadCustomerMerchantDishes(merchantId)
      const dishes: DishView[] = (result.dishes || []).map((dish: CustomerDish) => ({
        id: dish.id,
        name: dish.name,
        image_url: getPublicImageUrl(dish.image_url || ''),
        price: dish.price,
        priceDisplay: formatPriceNoSymbol(dish.price || 0),
        member_price: dish.member_price,
        memberPriceDisplay: dish.member_price ? formatPriceNoSymbol(dish.member_price) : null,
        original_price: dish.original_price,
        originalPriceDisplay: dish.original_price ? formatPriceNoSymbol(dish.original_price) : null,
        category_id: dish.category_id || 0,
        category_name: dish.category_name || '未分类',
        monthly_sales: dish.monthly_sales || 0,
        prepare_time: dish.prepare_time || 10,
        tags: dish.tags || [],
        is_available: true,
        hasCustomizations: Array.isArray(dish.customization_groups) && dish.customization_groups.length > 0
      }))
      return { dishes, categories: result.categories || [] }
    } catch (error) {
      console.error('加载菜品失败:', error)
      return { dishes: [], categories: [] }
    }
  },

  async loadCombos(merchantId: number): Promise<ComboView[]> {
    try {
      const result = await loadCustomerMerchantCombos(merchantId)
      return (result.combos || []).map((combo: CustomerCombo) => {
        // 优先使用后端返回的图片列表，如果没有则回退到前端拼接逻辑(兼容旧数据)
        let dishImages: string[] = []
        if (combo.dish_images && combo.dish_images.length > 0) {
           dishImages = combo.dish_images.map((url) => getPublicImageUrl(url))
        }

        return {
          id: combo.id,
          name: combo.name,
          description: combo.description || '',
          image_url: getPublicImageUrl(combo.image_url || ''),
          combo_price: combo.combo_price,
          comboPriceDisplay: formatPriceNoSymbol(combo.combo_price || 0),
          original_price: combo.original_price,
          originalPriceDisplay: formatPriceNoSymbol(combo.original_price || 0),
          savingsDisplay: formatPriceNoSymbol((combo.original_price || 0) - (combo.combo_price || 0)),
          dishes: combo.dishes || [],
          dish_images: dishImages 
        }
      })
    } catch (error) {
      console.error('加载套餐失败:', error)
      return []
    }
  },

  async loadRooms(merchantId: number): Promise<CustomerRoom[]> {
    try {
      const result = await loadCustomerMerchantRooms(merchantId)
      // 包间图片是公共图片，使用getPublicImageUrl处理
      return (result.rooms || []).map((room: CustomerRoom) => ({
        ...room,
        primary_image: room.primary_image ? getPublicImageUrl(room.primary_image) : '',
        minimum_spend: room.minimum_spend || 0,
        minimumSpendDisplay: formatPriceNoSymbol(room.minimum_spend || 0)
      }))
    } catch (error) {
      console.error('加载包间失败:', error)
      return []
    }
  },

  extractCategories(dishesResult: { dishes: DishView[], categories: CustomerDishCategory[] }): CustomerDishCategory[] {
    const categoryMap = new Map<number, CustomerDishCategory>()
    categoryMap.set(0, { id: 0, name: '全部', sort_order: -1 })

    // 优先使用API返回的分类
    if (dishesResult.categories && dishesResult.categories.length > 0) {
      dishesResult.categories.forEach((cat) => {
        if (!categoryMap.has(cat.id)) {
          categoryMap.set(cat.id, cat)
        }
      })
    } else {
      // 回退：从菜品中提取分类
      dishesResult.dishes.forEach((dish) => {
        if (dish.category_id && !categoryMap.has(dish.category_id)) {
          categoryMap.set(dish.category_id, {
            id: dish.category_id,
            name: dish.category_name || `分类${dish.category_id}`,
            sort_order: dish.category_id
          })
        }
      })
    }

    return Array.from(categoryMap.values()).sort((a, b) => a.sort_order - b.sort_order)
  },

  onTabChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ activeTab: e.detail.value })
  },

  onCategoryChange(e: WechatMiniprogram.CustomEvent) {
    const id = e.currentTarget.dataset.id
    this.setData({ activeCategoryId: id })
    this.filterDishes()
  },

  filterDishes() {
    const { dishes, activeCategoryId } = this.data
    if (activeCategoryId === 0 || activeCategoryId === '0' || !activeCategoryId) {
      this.setData({ filteredDishes: dishes })
    } else {
      const filtered = dishes.filter((d) => String(d.category_id) === String(activeCategoryId))
      this.setData({ filteredDishes: filtered })
    }
  },

  onDishTap(e: WechatMiniprogram.CustomEvent) {
    if (!this.canPlaceOrder()) {
      return
    }
    const id = e.currentTarget.dataset.id
    wx.navigateTo({ url: `/pages/takeout/dish-detail/index?id=${id}&merchant_id=${this.data.restaurantId}` })
  },

  onComboTap(e: WechatMiniprogram.CustomEvent) {
    if (!this.canPlaceOrder()) {
      return
    }
    const id = e.currentTarget.dataset.id
    const combo = this.data.combos.find((c) => String(c.id) === String(id))

    if (combo && this.data.restaurant) {
      Navigation.toComboDetail(String(id), {
        shopName: this.data.restaurant.name,
        monthSales: this.data.restaurant.monthly_sales,
        estimatedDeliveryTime: this.data.restaurant.avg_prep_minutes
      })
    } else {
      Navigation.toComboDetail(String(id))
    }
  },

  async onAddCart(e: WechatMiniprogram.CustomEvent) {
    const id = e.currentTarget.dataset.id
    const { restaurant } = this.data
    if (!this.canPlaceOrder()) {
      return
    }
    const dish = this.data.dishes.find((d) => d.id === id)

    if (dish && restaurant) {
      const success = await CartService.addItem({
        merchantId: restaurant.id,
        dishId: dish.id
      })

      if (success) {
        this.updateCartDisplay()
        wx.showToast({ title: '已加入购物车', icon: 'success', duration: 500 })
      }
    }
  },

  async onAddComboCart(e: WechatMiniprogram.CustomEvent) {
    const id = e.currentTarget.dataset.id
    const { restaurant } = this.data
    if (!this.canPlaceOrder()) {
      return
    }
    const combo = this.data.combos.find((c) => c.id === id)

    if (combo && restaurant) {
      const success = await CartService.addItem({
        merchantId: restaurant.id,
        comboId: combo.id
      })

      if (success) {
        this.updateCartDisplay()
        wx.showToast({ title: '已加入购物车', icon: 'success', duration: 500 })
      }
    }
  },

  async updateCartDisplay() {
    try {
      // 使用与外卖首页相同的方式获取购物车状态
      const userCarts = await loadTakeoutCartSummary()
      const totalCount = userCarts.summary?.total_items || 0
      const totalPrice = userCarts.summary?.total_amount || 0

      this.setData({
        cartCount: totalCount,
        cartPrice: totalPrice,
        cartPriceDisplay: formatPriceNoSymbol(totalPrice)
      })
    } catch (error) {
      // 获取失败时重置为0
      this.setData({
        cartCount: 0,
        cartPrice: 0,
        cartPriceDisplay: '0.00'
      })
    }
  },

  onCheckout() {
    if (!this.canPlaceOrder()) {
      return
    }
    Navigation.toCart()
  },

  onCartTap() {
    if (!this.canPlaceOrder()) {
      return
    }
    // 点击购物车栏跳转到购物车页面
    Navigation.toCart()
  },

  onCall() {
    const phone = this.data.restaurant?.phone
    if (phone) {
      wx.makePhoneCall({ phoneNumber: phone })
    } else {
      wx.showToast({ title: '暂无联系电话', icon: 'none' })
    }
  },

  onMapTap() {
    const restaurant = this.data.restaurant
    if (restaurant && restaurant.latitude && restaurant.longitude) {
      wx.openLocation({
        latitude: Number(restaurant.latitude),
        longitude: Number(restaurant.longitude),
        name: restaurant.name,
        address: restaurant.address
      })
    } else {
      wx.showToast({ title: '暂无位置信息', icon: 'none' })
    }
  },

  onPreviewLicense(e: WechatMiniprogram.CustomEvent) {
    const src = e.currentTarget.dataset.src
    if (src) {
      wx.previewImage({
        current: src,
        urls: [src]
      })
    }
  },

  onRoomTap(e: WechatMiniprogram.CustomEvent) {
    if (!this.canPlaceOrder()) {
      return
    }
    const roomId = e.currentTarget.dataset.id
    if (roomId) {
      Navigation.toRoomDetail(String(roomId))
    }
  },

  onScroll(e: WechatMiniprogram.CustomEvent) {
    const { scrollTop } = e.detail
    const { headerCollapsed, headerScrollDirection } = this.data
    const nextHeaderCollapsed = resolveRestaurantHeaderCollapsed({
      scrollTop: Number(scrollTop) || 0,
      headerCollapsed,
      scrollDirection: headerScrollDirection
    })

    if (nextHeaderCollapsed !== headerCollapsed) {
      this.setData({ headerCollapsed: nextHeaderCollapsed })
    }
  },

  onMenuTouchStart(e: WechatMiniprogram.TouchEvent) {
    this.setData({
      headerTouchY: e.touches[0]?.clientY || 0,
      headerScrollDirection: 'none'
    })
  },

  onMenuTouchMove(e: WechatMiniprogram.TouchEvent) {
    const currentY = e.touches[0]?.clientY || 0
    const previousY = this.data.headerTouchY
    const deltaY = currentY - previousY

    if (Math.abs(deltaY) < 2) {
      return
    }

    this.setData({
      headerTouchY: currentY,
      headerScrollDirection: deltaY < 0 ? 'up' : 'down'
    })

    if (deltaY < 0 && !this.data.headerCollapsed) {
      this.setData({ headerCollapsed: true })
    }
  },

  stopPropagation() {
    // 仅用于阻止冒泡
  },

  showOrderingSuspendedToast() {
    wx.showToast({ title: ORDERING_SUSPENDED_MESSAGE, icon: 'none' })
  },

  showStoreClosedToast() {
    wx.showToast({ title: STORE_CLOSED_MESSAGE, icon: 'none' })
  },

  canPlaceOrder() {
    const { restaurant } = this.data
    if (!restaurant) {
      return false
    }
    if (restaurant.is_ordering_suspended) {
      this.showOrderingSuspendedToast()
      return false
    }
    if (restaurant.biz_status !== 'OPEN') {
      this.showStoreClosedToast()
      return false
    }
    return true
  },

  // Gap 8: 分享给朋友
  onShareAppMessage(): WechatMiniprogram.Page.ICustomShareContent {
    const { restaurantId, restaurant } = this.data as {
      restaurantId: number | string
      restaurant?: { name?: string, cover_image?: string }
    }
    return {
      title: restaurant?.name ? `${restaurant.name} — 一起来吃！` : '发现一家好店，快来看看！',
      path: `/pages/takeout/restaurant-detail/index?id=${restaurantId}`,
      imageUrl: restaurant?.cover_image || ''
    }
  },

  // Gap 8: 分享到朋友圈
  onShareTimeline(): WechatMiniprogram.Page.ICustomTimelineContent {
    const { restaurant } = this.data as {
      restaurant?: { name?: string, cover_image?: string }
    }
    return {
      title: restaurant?.name ? `${restaurant.name} — 美食推荐` : '发现一家好店',
      imageUrl: restaurant?.cover_image || ''
    }
  }
})
