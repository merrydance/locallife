import { getPublicMerchantDetail, PublicMerchantDetail } from '../../../api/merchant'
import ConsumerMerchantDetailAdapter, { type ConsumerMerchantDetailViewModel } from '../../../adapters/consumer-merchant-detail'

type RestaurantInfoViewModel = ConsumerMerchantDetailViewModel

const getErrorMessage = (error: unknown, fallback: string): string => {
  if (typeof error === 'object' && error !== null && 'userMessage' in error) {
    const { userMessage } = error as { userMessage?: unknown }
    if (typeof userMessage === 'string' && userMessage.trim()) {
      return userMessage
    }
  }
  if (error && typeof error === 'object' && 'message' in error) {
    const { message } = error as { message?: unknown }
    if (typeof message === 'string' && message.trim()) {
      return message
    }
  }
  return fallback
}

Page({
  data: {
    restaurantId: '',
    restaurant: null as RestaurantInfoViewModel | null,
    navBarHeight: 88,
    loading: true,
    isError: false,
    errorMsg: ''
  },

  onLoad(options: { id?: string }) {
    const restaurantId = options.id
    if (!restaurantId) {
      wx.showToast({ title: '商家ID缺失', icon: 'error' })
      setTimeout(() => wx.navigateBack(), 1500)
      return
    }
    this.setData({ restaurantId })
    this.loadMerchantInfo()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadMerchantInfo() {
    this.setData({ loading: true, isError: false })
    try {
      const merchantId = parseInt(this.data.restaurantId)
      const merchant: PublicMerchantDetail = await getPublicMerchantDetail(merchantId)

      if (!merchant) {
        this.setData({ 
          restaurant: null, 
          loading: false, 
          isError: true, 
          errorMsg: '商家信息不存在' 
        })
        return
      }

      this.setData({
        restaurant: ConsumerMerchantDetailAdapter.toViewModel(merchant),
        loading: false
      })
    } catch (error: unknown) {
      console.error('加载商户信息失败:', error)
      this.setData({ 
        loading: false, 
        isError: true, 
        errorMsg: getErrorMessage(error, '加载商家详情失败') 
      })
    }
  },

  onCall() {
    const phone = this.data.restaurant?.phone
    if (!phone) return
    wx.makePhoneCall({ phoneNumber: phone })
  },

  onMapTap() {
    const restaurant = this.data.restaurant
    if (!restaurant || !restaurant.latitude || !restaurant.longitude) return
    wx.openLocation({
      latitude: restaurant.latitude,
      longitude: restaurant.longitude,
      name: restaurant.name,
      address: restaurant.address
    })
  },

  onPreviewLicense(e: WechatMiniprogram.CustomEvent) {
    const { src } = e.currentTarget.dataset as { src?: string }
    if (!src) return
    wx.previewImage({ urls: [src] })
  }
})
