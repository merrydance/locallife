/**
 * 搜索页面
 * 使用真实后端API
 */

import { searchDishes, DishSummary } from '../../../api/dish'
import { searchMerchants, MerchantSummary } from '../../../api/merchant'
import { DishAdapter } from '../../../adapters/dish'
import { responsiveBehavior } from '../../../utils/responsive'

Page({
  behaviors: [responsiveBehavior],
  data: {
    keyword: '',
    activeTab: 'dishes' as 'dishes' | 'restaurants',
    dishes: [] as any[],
    restaurants: [] as any[],
    loading: false
  },

  onLoad(options: any) {
    if (options.keyword) {
      this.setData({ keyword: options.keyword })
      this.onSearch()
    }
    if (options.type) {
      this.setData({ activeTab: options.type })
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onSearchChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ keyword: e.detail.value })
  },

  async onSearch() {
    const { keyword, activeTab } = this.data
    if (!keyword.trim()) return

    this.setData({ loading: true })

    try {
      const app = getApp<IAppOption>()

      if (activeTab === 'dishes') {
        await this.searchDishes(keyword)
      } else {
        await this.searchRestaurants(keyword, app.globalData.latitude || undefined, app.globalData.longitude || undefined)
      }
    } catch (error) {
      console.error('搜索失败:', error)
      wx.showToast({ title: '搜索失败', icon: 'error' })
    } finally {
      this.setData({ loading: false })
    }
  },

  async searchDishes(keyword: string) {
    const result = await searchDishes({
      keyword,
      page_id: 1,
      page_size: 20
    })

    const dishes = (result || []).map((dish: DishSummary) => ({
      id: dish.id,
      name: dish.name,
      shop_name: dish.merchant_name,
      shop_id: dish.merchant_id,
      image_url: dish.image_url,
      price: dish.price,
      month_sales: dish.monthly_sales || 0,
      distance: DishAdapter.formatDistance(dish.distance || 0),
      is_available: dish.is_available
    }))

    this.setData({ dishes })
  },

  async searchRestaurants(keyword: string, latitude?: number, longitude?: number) {
    const result = await searchMerchants({
      keyword,
      page_id: 1,
      page_size: 20,
      user_latitude: latitude,
      user_longitude: longitude
    })

    const restaurants = (result || []).map((merchant: MerchantSummary) => ({
      id: merchant.id,
      name: merchant.name,
      cover_image: merchant.logo_url,
      address: merchant.address,
      distance: DishAdapter.formatDistance(merchant.distance || 0),
      tags: merchant.tags || []
    }))

    this.setData({ restaurants })
  },

  onTabChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ activeTab: e.detail.value })
    // 切换 tab 时重新搜索
    if (this.data.keyword.trim()) {
      this.onSearch()
    }
  },

  onDishTap(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    const dish = this.data.dishes.find(d => d.id === id)
    wx.navigateTo({
      url: `/pages/takeout/dish-detail/index?id=${id}&merchant_id=${dish?.shop_id || ''}`
    })
  },

  onRestaurantTap(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${id}` })
  }
})
