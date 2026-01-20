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
import { formatPriceNoSymbol } from '../../../utils/util'

type FavoriteDishView = {
  id: number
  type: 'DISH'
  name: string
  image: string
  price: number
  priceDisplay: string
  merchantId: number
  merchantName: string
  desc: string
  isAvailable: boolean
}

type FavoriteMerchantView = {
  id: number
  type: 'MERCHANT'
  name: string
  image: string
  address: string
  status: string
  desc: string
}

type FavoriteView = FavoriteDishView | FavoriteMerchantView

Page({
  data: {
    favorites: [] as FavoriteView[],
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
    const result = await getFavoriteDishes({ page: 1, page_size: 50 })

    const favorites: FavoriteDishView[] = (result.dishes || []).map((item: FavoriteDishResponse) => ({
      id: item.dish_id,
      type: 'DISH',
      name: item.dish_name,
      image: item.image_url || '/assets/default-dish.png',
      price: item.price,
      priceDisplay: formatPriceNoSymbol(item.price || 0),
      merchantId: item.merchant_id,
      merchantName: item.merchant_name,
      desc: item.description || item.merchant_name,
      isAvailable: item.is_available
    }))

    this.setData({ favorites })
  },

  async loadFavoriteMerchants() {
    const result = await getFavoriteMerchants({ page: 1, page_size: 50 })

    const favorites: FavoriteMerchantView[] = (result.merchants || []).map((item: FavoriteMerchantResponse) => ({
      id: item.merchant_id,
      type: 'MERCHANT',
      name: item.merchant_name,
      image: item.merchant_logo || '/assets/default-merchant.png',
      address: item.address,
      status: item.status,
      desc: item.address
    }))

    this.setData({ favorites })
  },

  onItemClick(e: WechatMiniprogram.CustomEvent) {
    const { id, type } = e.currentTarget.dataset as { id?: number; type?: 'DISH' | 'MERCHANT' }
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
    const { id, type } = e.currentTarget.dataset as { id?: number; type?: 'DISH' | 'MERCHANT' }
    if (!id || !type) return

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
