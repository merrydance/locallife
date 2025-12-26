/**
 * 收藏页面
 * 使用真实后端API
 */

import {
  getFavoriteDishes,
  getFavoriteMerchants,
  removeDishFromFavorites,
  removeMerchantFromFavorites,
  FavoriteDishResponse,
  FavoriteMerchantResponse
} from '../../../api/personal'
import { logger } from '../../../utils/logger'

Page({
  data: {
    favorites: [] as any[],
    activeTab: 'dishes' as 'dishes' | 'merchants',
    loading: false,
    navBarHeight: 88
  },

  onLoad() {
    this.loadFavorites()
  },

  onShow() {
    // 返回时刷新数据
    if (this.data.favorites.length > 0) {
      this.loadFavorites()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onTabChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ activeTab: e.detail.value })
    this.loadFavorites()
  },

  async loadFavorites() {
    this.setData({ loading: true })

    try {
      const { activeTab } = this.data

      if (activeTab === 'dishes') {
        await this.loadFavoriteDishes()
      } else {
        await this.loadFavoriteMerchants()
      }
    } catch (error) {
      console.error('加载收藏失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
    } finally {
      this.setData({ loading: false })
    }
  },

  async loadFavoriteDishes() {
    const result = await getFavoriteDishes({ page_id: 1, page_size: 50 })

    const favorites = (result.dishes || []).map((item: FavoriteDishResponse) => ({
      id: item.dish_id,
      type: 'DISH',
      name: item.dish_name,
      image: item.dish_image_url || '/assets/default-dish.png',
      price: item.price,
      merchantId: item.merchant_id,
      merchantName: item.merchant_name,
      desc: item.merchant_name
    }))

    this.setData({ favorites })
  },

  async loadFavoriteMerchants() {
    const result = await getFavoriteMerchants({ page_id: 1, page_size: 50 })

    const favorites = (result.merchants || []).map((item: FavoriteMerchantResponse) => ({
      id: item.merchant_id,
      type: 'MERCHANT',
      name: item.merchant_name,
      image: item.merchant_logo_url || '/assets/default-merchant.png',
      monthlySales: item.monthly_sales,
      deliveryFee: item.estimated_delivery_fee,
      tags: item.tags || [],
      desc: item.tags?.slice(0, 2).join(' · ') || ''
    }))

    this.setData({ favorites })
  },

  onItemClick(e: WechatMiniprogram.CustomEvent) {
    const { id, type } = e.currentTarget.dataset
    if (type === 'DISH') {
      const item = this.data.favorites.find(f => f.id === id)
      wx.navigateTo({
        url: `/pages/takeout/dish-detail/index?id=${id}&merchant_id=${item?.merchantId || ''}`
      })
    } else {
      wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${id}` })
    }
  },

  onRemoveFavorite(e: WechatMiniprogram.CustomEvent) {
    const { id, type } = e.currentTarget.dataset

    wx.showModal({
      title: '取消收藏',
      content: '确定要取消收藏吗？',
      success: async (res) => {
        if (res.confirm) {
          await this.doRemoveFavorite(id, type)
        }
      }
    })
  },

  async doRemoveFavorite(id: number, type: string) {
    wx.showLoading({ title: '处理中...' })
    try {
      if (type === 'DISH') {
        await removeDishFromFavorites(id)
      } else {
        await removeMerchantFromFavorites(id)
      }
      wx.hideLoading()
      wx.showToast({ title: '已取消收藏', icon: 'success' })

      // 从列表中移除
      const favorites = this.data.favorites.filter(f => !(f.id === id && f.type === type))
      this.setData({ favorites })
    } catch (error) {
      wx.hideLoading()
      logger.error('取消收藏失败', error, 'favorites.doRemoveFavorite')
      wx.showToast({ title: '操作失败', icon: 'error' })
    }
  }
})
