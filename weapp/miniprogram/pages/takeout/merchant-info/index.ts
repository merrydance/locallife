import { getPublicMerchantDetail, PublicMerchantDetail } from '../../../api/merchant'
import { getPublicImageUrl } from '../../../utils/image'

type BusinessHoursView = NonNullable<PublicMerchantDetail['business_hours']>[number] & {
  day_name: string
}

type RestaurantInfoViewModel = PublicMerchantDetail & {
  cover_image?: string
  logo_url?: string
  business_license_image_url?: string
  food_permit_url?: string
  business_hours: BusinessHoursView[]
  biz_status: 'OPEN' | 'CLOSED'
  discount_rules: PublicMerchantDetail['discount_rules']
  vouchers: PublicMerchantDetail['vouchers']
  delivery_promotions: PublicMerchantDetail['delivery_promotions']
}

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

      const dayNames = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
      const formattedHours: BusinessHoursView[] = (merchant.business_hours || []).map((h) => ({
        ...h,
        day_name: dayNames[h.day_of_week]
      }))

      this.setData({
        restaurant: {
          ...merchant,
          cover_image: merchant.cover_image || merchant.logo_url || '',
          logo_url: getPublicImageUrl(merchant.logo_url || ''),
          business_license_image_url: merchant.business_license_image_url,
          food_permit_url: merchant.food_permit_url,
          business_hours: formattedHours,
          biz_status: merchant.is_open ? 'OPEN' : 'CLOSED',
          tags: merchant.tags || [],
          discount_rules: merchant.discount_rules || [],
          vouchers: merchant.vouchers || [],
          delivery_promotions: merchant.delivery_promotions || []
        },
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
