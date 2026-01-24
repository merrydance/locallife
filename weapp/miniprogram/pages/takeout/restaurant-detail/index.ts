/**
 * 餐厅详情页面
 * 使用真实后端API
 */

import { getPublicMerchantDetail, getPublicMerchantDishes, getPublicMerchantCombos, PublicMerchantDetail, PublicDishCategory, PublicDish, PublicCombo } from '../../../api/merchant'
import { getPublicMerchantRooms, PublicRoom } from '../../../api/room'
import { getUserCarts } from '../../../api/cart'
import CartService from '../../../services/cart'
import { getPublicImageUrl } from '../../../utils/image'
import { resolveImageURL } from '../../../utils/image-security'
import { formatPriceNoSymbol } from '../../../utils/util'

interface BusinessHoursView {
  day_of_week: number
  open_time: string
  close_time: string
  is_closed: boolean
  day_name: string
}

interface RestaurantViewModel {
  id: number
  name: string
  cover_image?: string
  logo_url: string
  address: string
  phone: string
  latitude: number
  longitude: number
  tags: string[]
  monthly_sales: number
  avg_prep_minutes: number
  biz_status: 'OPEN' | 'CLOSED'
  description: string
  business_license_image_url?: string
  food_permit_url?: string
  business_hours: BusinessHoursView[]
  business_hours_display: string
  discount_rules: PublicMerchantDetail['discount_rules']
  vouchers: PublicMerchantDetail['vouchers']
  delivery_promotions: PublicMerchantDetail['delivery_promotions']
}

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
  dishes: PublicCombo['dishes']
  dish_images: string[] // 新增：包含菜品的图片列表
}

Page({
  data: {
    restaurantId: '',
    restaurant: null as RestaurantViewModel | null,
    activeTab: 'dishes' as 'dishes' | 'combos' | 'rooms',
    activeCategoryId: '' as string | number,
    categories: [] as PublicDishCategory[],
    dishes: [] as DishView[],
    filteredDishes: [] as DishView[],
    combos: [] as ComboView[],
    rooms: [] as PublicRoom[],
    cartCount: 0,
    cartPrice: 0,
    cartPriceDisplay: '0.00',
    navBarHeight: 88,
    loading: true,
    headerCollapsed: false
  },

  onLoad(options: { id?: string }) {
    const restaurantId = options.id
    if (!restaurantId) {
      wx.showToast({ title: '商家ID缺失', icon: 'error' })
      setTimeout(() => wx.navigateBack(), 1500)
      return
    }
    this.setData({ restaurantId })
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
    this.setData({ loading: true })

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
        wx.showToast({ title: '商家不存在', icon: 'error' })
        this.setData({ loading: false })
        return
      }

      // 第二次处理套餐，注入菜品图片
      const combosResult = combosResultFirstPass.map(combo => {
        const dishImages = (combo.dishes || [])
          .map(cd => dishesResult.dishes.find(d => d.id === cd.dish_id)?.image_url)
          .filter(Boolean) as string[]
        return { ...combo, dish_images: dishImages }
      })

      // 从菜品中提取分类
      const categories = this.extractCategories(dishesResult)
      const firstCategoryId = categories[0]?.id || ''

      this.setData({
        restaurant: merchantResult,
        categories,
        dishes: dishesResult.dishes,
        combos: combosResult,
        rooms: roomsResult,
        activeCategoryId: firstCategoryId,
        loading: false
      })

      this.filterDishes()
    } catch (error) {
      console.error('加载商户详情失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  async loadMerchantInfo(merchantId: number): Promise<RestaurantViewModel | null> {
    try {
      const merchant: PublicMerchantDetail = await getPublicMerchantDetail(merchantId)

      if (merchant) {
        // 私有图片需要签名（营业执照、食品许可证）
        const [coverImage, businessLicense, foodPermit] = await Promise.all([
          resolveImageURL(merchant.cover_image || merchant.logo_url || ''),
          resolveImageURL(merchant.business_license_image_url || ''),
          resolveImageURL(merchant.food_permit_url || '')
        ])

        // 格式化营业时间
        let businessHoursDisplay = ''
        if (merchant.business_hours && merchant.business_hours.length > 0) {
          const today = new Date().getDay()
          const todayHours = merchant.business_hours.find(h => h.day_of_week === today)
          if (todayHours) {
            businessHoursDisplay = `${todayHours.open_time} - ${todayHours.close_time}`
          } else if (merchant.business_hours[0]) {
            const first = merchant.business_hours[0]
            businessHoursDisplay = `${first.open_time} - ${first.close_time}`
          }
        }

        // 格式化所有营业时间
        const dayNames = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
        const formattedHours: BusinessHoursView[] = (merchant.business_hours || []).map((h) => ({
          ...h,
          day_name: dayNames[h.day_of_week]
        }));

        return {
          id: merchant.id,
          name: merchant.name,
          cover_image: coverImage,
          logo_url: getPublicImageUrl(merchant.logo_url || ''),
          address: merchant.address,
          phone: merchant.phone,
          latitude: merchant.latitude,
          longitude: merchant.longitude,
          tags: merchant.tags || [],
          monthly_sales: merchant.monthly_sales || 0,
          avg_prep_minutes: merchant.avg_prep_minutes || 15,
          biz_status: merchant.is_open ? 'OPEN' : 'CLOSED',
          description: merchant.description || '',
          business_license_image_url: businessLicense,
          food_permit_url: foodPermit,
          business_hours: formattedHours,
          business_hours_display: businessHoursDisplay,
          discount_rules: merchant.discount_rules || [],
          vouchers: merchant.vouchers || [],
          delivery_promotions: merchant.delivery_promotions || []
        }
      }
      return null
    } catch (error) {
      console.error('加载商户信息失败:', error)
      return null
    }
  },

  async loadDishes(merchantId: number): Promise<{ dishes: DishView[]; categories: PublicDishCategory[] }> {
    try {
      const result = await getPublicMerchantDishes(merchantId)
      const dishes: DishView[] = (result.dishes || []).map((dish: PublicDish) => ({
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
      const result = await getPublicMerchantCombos(merchantId)
      return (result.combos || []).map((combo: PublicCombo) => ({
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
        dish_images: [] // 初始为空，由外层逻辑注入
      }))
    } catch (error) {
      console.error('加载套餐失败:', error)
      return []
    }
  },

  async loadRooms(merchantId: number): Promise<PublicRoom[]> {
    try {
      const result = await getPublicMerchantRooms(merchantId)
      // 包间图片是公共图片，使用getPublicImageUrl处理
      return (result.rooms || []).map((room: PublicRoom) => ({
        ...room,
        primary_image: room.primary_image ? getPublicImageUrl(room.primary_image) : '',
        minimum_spend: room.minimum_spend || 0 // 防御性处理：防止 NaN
      }))
    } catch (error) {
      console.error('加载包间失败:', error)
      return []
    }
  },

  extractCategories(dishesResult: { dishes: DishView[]; categories: PublicDishCategory[] }): PublicDishCategory[] {
    const categoryMap = new Map<number, PublicDishCategory>()
    categoryMap.set(0, { id: 0, name: '全部', sort_order: -1 })

    // 优先使用API返回的分类
    if (dishesResult.categories && dishesResult.categories.length > 0) {
      dishesResult.categories.forEach(cat => {
        if (!categoryMap.has(cat.id)) {
          categoryMap.set(cat.id, cat)
        }
      })
    } else {
      // 回退：从菜品中提取分类
      dishesResult.dishes.forEach(dish => {
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
    const id = e.currentTarget.dataset.id
    wx.navigateTo({ url: `/pages/takeout/dish-detail/index?id=${id}&merchant_id=${this.data.restaurantId}` })
  },

  onComboTap(e: WechatMiniprogram.CustomEvent) {
    const id = e.currentTarget.dataset.id
    wx.showToast({ title: '套餐详情开发中', icon: 'none' })
  },

  async onAddCart(e: WechatMiniprogram.CustomEvent) {
    const id = e.currentTarget.dataset.id
    const { restaurant } = this.data
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
      const userCarts = await getUserCarts('takeout')
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
    wx.navigateTo({ url: '/pages/takeout/cart/index' })
  },

  onCartTap() {
    // 点击购物车栏跳转到购物车页面
    wx.navigateTo({ url: '/pages/takeout/cart/index' })
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
    const roomId = e.currentTarget.dataset.id
    if (roomId) {
      wx.navigateTo({
        url: `/pages/reservation/room-detail/index?id=${roomId}`
      })
    }
  },

  onScroll(e: WechatMiniprogram.CustomEvent) {
    const { scrollTop } = e.detail
    const { headerCollapsed } = this.data
    const threshold = 50 // 滚动阈值

    if (scrollTop > threshold && !headerCollapsed) {
      this.setData({ headerCollapsed: true })
    } else if (scrollTop <= threshold && headerCollapsed) {
      this.setData({ headerCollapsed: false })
    }
  },

  stopPropagation() {
    // 仅用于阻止冒泡
  }
})
