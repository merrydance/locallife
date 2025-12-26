/**
 * 餐厅详情页面
 * 使用真实后端API
 */

import { searchMerchants, MerchantSearchResult } from '../../../api/search-recommendation'
import { searchDishes, DishSummary } from '../../../api/dish'
import { getMerchantReviews, ListReviewsResponse } from '../../../api/personal'
import { DishManagementService, DishCategory } from '../../../api/dish'

Page({
  data: {
    restaurantId: '',
    restaurant: null as any,
    activeTab: 'dishes' as 'dishes' | 'reviews' | 'info',
    activeCategoryId: '',
    categories: [] as DishCategory[],
    dishes: [] as any[],
    filteredDishes: [] as any[],
    reviews: [] as any[],
    cartCount: 0,
    cartPrice: 0,
    navBarHeight: 88,
    loading: true
  },

  onLoad(options: any) {
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

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadRestaurantDetail() {
    this.setData({ loading: true })

    try {
      const merchantId = parseInt(this.data.restaurantId)

      // 并行加载商户信息、菜品和评价
      const [merchantResult, dishesResult, reviewsResult] = await Promise.all([
        this.loadMerchantInfo(merchantId),
        this.loadDishes(merchantId),
        this.loadReviews(merchantId)
      ])

      if (!merchantResult) {
        wx.showToast({ title: '商家不存在', icon: 'error' })
        this.setData({ loading: false })
        return
      }

      // 从菜品中提取分类
      const categories = this.extractCategories(dishesResult)

      this.setData({
        restaurant: merchantResult,
        categories,
        dishes: dishesResult,
        reviews: reviewsResult,
        activeCategoryId: categories[0]?.id?.toString() || '',
        loading: false
      })

      this.filterDishes()
    } catch (error) {
      console.error('加载商户详情失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  async loadMerchantInfo(merchantId: number): Promise<any> {
    try {
      // 使用搜索接口获取商户信息
      const result = await searchMerchants({
        keyword: '',
        page: 1,
        page_size: 100
      })

      const merchant = result.data?.find((m: MerchantSearchResult) => m.id === merchantId)

      if (merchant) {
        return {
          id: merchant.id,
          name: merchant.name,
          cover_image: merchant.cover_image || merchant.logo_url,
          address: merchant.address,
          phone: '', // 搜索结果不包含电话
          rating: merchant.rating ? (merchant.rating / 10).toFixed(1) : '暂无',
          review_count: merchant.review_count || 0,
          tags: merchant.category ? [merchant.category] : [],
          distance_meters: merchant.distance || 0,
          delivery_fee: merchant.delivery_fee || 0,
          delivery_time_minutes: merchant.estimated_delivery_time || 30,
          biz_status: merchant.is_open ? 'OPEN' : 'CLOSED',
          description: merchant.description || ''
        }
      }
      return null
    } catch (error) {
      console.error('加载商户信息失败:', error)
      return null
    }
  },

  async loadDishes(merchantId: number): Promise<any[]> {
    try {
      const result = await searchDishes({
        keyword: '',
        merchant_id: merchantId,
        page_id: 1,
        page_size: 100
      })

      return (result || []).map((dish: DishSummary) => ({
        id: dish.id,
        name: dish.name,
        image_url: dish.image_url,
        price: dish.price,
        original_price: dish.price,
        category_id: '1', // 默认分类，后端暂不返回
        month_sales: dish.monthly_sales || 0,
        rating: '5.0',
        tags: dish.tags || [],
        is_available: dish.is_available
      }))
    } catch (error) {
      console.error('加载菜品失败:', error)
      return []
    }
  },

  async loadReviews(merchantId: number): Promise<any[]> {
    try {
      const result: ListReviewsResponse = await getMerchantReviews(merchantId, {
        page_id: 1,
        page_size: 20
      })

      return (result.reviews || []).map(review => ({
        id: review.id,
        user_name: '用户' + review.user_id,
        user_avatar: '/assets/default-avatar.png',
        content: review.content,
        images: review.images || [],
        created_at: review.created_at,
        reply: review.merchant_reply ? {
          content: review.merchant_reply,
          created_at: review.replied_at
        } : null
      }))
    } catch (error) {
      console.error('加载评价失败:', error)
      return []
    }
  },

  extractCategories(dishes: any[]): DishCategory[] {
    // 由于后端暂不返回分类信息，创建默认分类
    return [
      { id: 1, name: '全部', sort_order: 0 },
      { id: 2, name: '热销', sort_order: 1 },
      { id: 3, name: '推荐', sort_order: 2 }
    ]
  },

  onTabChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ activeTab: e.detail.value })
  },

  onCategoryChange(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    this.setData({ activeCategoryId: id })
    this.filterDishes()
  },

  filterDishes() {
    const { dishes, activeCategoryId } = this.data
    // 由于后端暂不返回分类，显示全部菜品
    this.setData({ filteredDishes: dishes })
  },

  onDishTap(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/takeout/dish-detail/index?id=${id}&merchant_id=${this.data.restaurantId}` })
  },

  async onAddCart(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    const dish = this.data.dishes.find((d: any) => d.id === id)

    if (dish) {
      const CartService = require('../../../services/cart').default
      const success = await CartService.addItem({
        merchantId: this.data.restaurant.id,
        dishId: dish.id,
        dishName: dish.name,
        shopName: this.data.restaurant.name,
        imageUrl: dish.image_url,
        price: dish.price,
        priceDisplay: `¥${(dish.price / 100).toFixed(2)}`,
        quantity: 1
      })

      if (!success) {
        return
      }

      this.updateCartDisplay()
      wx.showToast({ title: '已加入购物车', icon: 'success', duration: 500 })
    }
  },

  updateCartDisplay() {
    const CartService = require('../../../services/cart').default
    const cart = CartService.getCart()
    this.setData({
      cartCount: cart.totalCount,
      cartPrice: cart.totalPrice
    })
  },

  onCheckout() {
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
    const { restaurant } = this.data
    if (restaurant && restaurant.latitude && restaurant.longitude) {
      wx.openLocation({
        latitude: parseFloat(restaurant.latitude),
        longitude: parseFloat(restaurant.longitude),
        name: restaurant.name,
        address: restaurant.address
      })
    } else {
      wx.showToast({ title: '暂无位置信息', icon: 'none' })
    }
  }
})
