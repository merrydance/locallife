// / <reference path="../../../typings/index.d.ts" />

import { formatImageUrl, ImageSize } from '../../utils/image'

interface FeaturedDish {
  id: number
  name: string
  imageUrl: string
  priceDisplay: string
  price: number
  merchantId: number
  customization_groups?: unknown[]
}

interface MerchantFeedData {
  id: number
  name: string
  imageUrl: string
  isOpen: boolean
  isOrderingSuspended: boolean
  distance: string
  monthlySales: number
  deliveryFeeDisplay: string
  promoText: string
  subsidyText: string
  tags: string[]
  systemLabels: string[]
  displayTags: string[]
  featuredDishes: FeaturedDish[]
  dishesLoading: boolean
  avgPrepMinutes: number
  discountPromoText: string
  voucherText: string
  deliveryPromoText: string
  isNewStore: boolean
  detailLoading: boolean
}

interface ComponentData {
  merchant: MerchantFeedData
  _merchantImageOptimized: boolean
}

const MERCHANT_RESTING_MESSAGE = '商户休息中～'

Component({
  properties: {
    merchant: {
      type: Object,
      value: {},
      observer(newVal: object) {
        const val = newVal as MerchantFeedData
        const data = this.data as unknown as ComponentData
        // 仅首次渲染时优化商户封面图，避免每次菜品加载都重复处理
        if (val && val.imageUrl && !data._merchantImageOptimized) {
          const optimized = formatImageUrl(val.imageUrl, ImageSize.CARD)
          if (optimized !== val.imageUrl) {
            this.setData({
              'merchant.imageUrl': optimized,
              _merchantImageOptimized: true
            })
          } else {
            this.setData({ _merchantImageOptimized: true })
          }
        }
      }
    }
  },

  data: {
    _merchantImageOptimized: false
  },

  methods: {
    isMerchantClosed() {
      const data = this.data as unknown as ComponentData
      return data.merchant.isOpen === false
    },

    showMerchantClosedToast() {
      wx.showToast({ title: MERCHANT_RESTING_MESSAGE, icon: 'none' })
    },

    onMerchantHeaderTap() {
      const data = this.data as unknown as ComponentData
      this.triggerEvent('merchanttap', { id: data.merchant.id })
    },

    onDishTap(e: WechatMiniprogram.TouchEvent) {
      const data = this.data as unknown as ComponentData
      if (data.merchant.isOrderingSuspended) {
        wx.showToast({ title: '当前商户暂停接单', icon: 'none' })
        return
      }
      if (this.isMerchantClosed()) {
        this.showMerchantClosedToast()
        return
      }
      const index = e.currentTarget.dataset.index as number
      const dish = data.merchant.featuredDishes[index]
      if (!dish) return
      const params = [
        `id=${dish.id}`,
        `merchant_id=${dish.merchantId}`,
        `shop_name=${encodeURIComponent(data.merchant.name)}`
      ].join('&')
      wx.navigateTo({ url: `/pages/takeout/dish-detail/index?${params}` })
    },

    onDishAdd(e: WechatMiniprogram.TouchEvent) {
      const data = this.data as unknown as ComponentData
      if (data.merchant.isOrderingSuspended) {
        wx.showToast({ title: '当前商户暂停接单', icon: 'none' })
        return
      }
      if (this.isMerchantClosed()) {
        this.showMerchantClosedToast()
        return
      }
      const index = e.currentTarget.dataset.index as number
      const dish = data.merchant.featuredDishes[index]
      if (!dish) return
      this.triggerEvent('dishadd', { dishId: dish.id, merchantId: dish.merchantId })
    },

    onSelectSpec(e: WechatMiniprogram.TouchEvent) {
      // 有定制项时跳转菜品详情页选规格
      const data = this.data as unknown as ComponentData
      if (data.merchant.isOrderingSuspended) {
        wx.showToast({ title: '当前商户暂停接单', icon: 'none' })
        return
      }
      if (this.isMerchantClosed()) {
        this.showMerchantClosedToast()
        return
      }
      const index = e.currentTarget.dataset.index as number
      const dish = data.merchant.featuredDishes[index]
      if (!dish) return
      const params = [
        `id=${dish.id}`,
        `merchant_id=${dish.merchantId}`,
        `shop_name=${encodeURIComponent(data.merchant.name)}`
      ].join('&')
      wx.navigateTo({ url: `/pages/takeout/dish-detail/index?${params}` })
    }
  }
})
