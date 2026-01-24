// / <reference path="../../../typings/index.d.ts" />

import { formatImageUrl, ImageSize } from '../../utils/image'

type DishCardData = {
  id?: number
  imageUrl?: string
  _imageOptimized?: boolean
  merchantId?: number
  shopName?: string
  month_sales?: number
  salesBadge?: string
  distance_meters?: number
  estimated_delivery_time?: number
}

Component({
  properties: {
    dish: {
      type: Object,
      value: {},
      observer(newVal: Record<string, unknown>) {
        // 当dish数据更新时，优化图片URL
        const dishVal = newVal as DishCardData
        if (dishVal && dishVal.imageUrl && !dishVal._imageOptimized) {
          const optimizedUrl = formatImageUrl(dishVal.imageUrl, ImageSize.CARD)
          this.setData({
            'dish.imageUrl': optimizedUrl,
            'dish._imageOptimized': true
          })
        }
      }
    }
  },

  methods: {
    onTap() {
      const dish = this.data.dish as any
      if (dish) {
        const monthSales = typeof dish.month_sales === 'number'
          ? dish.month_sales
          : typeof dish.salesBadge === 'string'
            ? Number(dish.salesBadge.replace(/[^0-9]/g, ''))
            : 0
        const estimatedMinutes = Math.round((dish.estimated_delivery_time || 0) / 60)
        // 传递额外信息到详情页（小程序不支持 URLSearchParams）
        const params = [
          `id=${dish.id}`,
          `merchant_id=${dish.merchantId || ''}`,
          `shop_name=${encodeURIComponent(dish.shopName || '')}`,
          `month_sales=${monthSales}`,
          `distance=${dish.distance_meters || 0}`,
          `estimated_delivery_time=${estimatedMinutes || 0}`
        ].join('&')
        wx.navigateTo({
          url: `/pages/takeout/dish-detail/index?${params}`
        })
      }
    },

    onAdd(e: WechatMiniprogram.TouchEvent) {
      if ((e as unknown as { stopPropagation?: () => void }).stopPropagation) {
        (e as unknown as { stopPropagation: () => void }).stopPropagation()
      }
      if (this.data.dish) {
        this.triggerEvent('add', { id: (this.data.dish as DishCardData).id })
      }
    },

    /**
     * 选规格 - 跳转到菜品详情页进行规格选择
     */
    onSelectSpec(e: WechatMiniprogram.TouchEvent) {
      if ((e as unknown as { stopPropagation?: () => void }).stopPropagation) {
        (e as unknown as { stopPropagation: () => void }).stopPropagation()
      }
      const dish = this.data.dish as DishCardData
      if (dish) {
        const monthSales = typeof dish.month_sales === 'number'
          ? dish.month_sales
          : typeof dish.salesBadge === 'string'
            ? Number(dish.salesBadge.replace(/[^0-9]/g, ''))
            : 0
        const estimatedMinutes = Math.round((dish.estimated_delivery_time || 0) / 60)
        const params = [
          `id=${dish.id}`,
          `merchant_id=${dish.merchantId || ''}`,
          `shop_name=${encodeURIComponent(dish.shopName || '')}`,
          `month_sales=${monthSales}`,
          `distance=${dish.distance_meters || 0}`,
          `estimated_delivery_time=${estimatedMinutes || 0}`
        ].join('&')
        wx.navigateTo({
          url: `/pages/takeout/dish-detail/index?${params}`
        })
      }
    }
  }
})
