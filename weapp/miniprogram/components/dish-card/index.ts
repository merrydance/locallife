// / <reference path="../../../typings/index.d.ts" />

import { formatImageUrl, ImageSize } from '../../utils/image'

Component({
  properties: {
    dish: {
      type: Object,
      value: {},
      observer(newVal: Record<string, unknown>) {
        // 当dish数据更新时，优化图片URL
        if (newVal && newVal.imageUrl && !newVal._imageOptimized) {
          const optimizedUrl = formatImageUrl(newVal.imageUrl, ImageSize.CARD)
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
        // 传递额外信息到详情页（小程序不支持 URLSearchParams）
        const params = [
          `id=${dish.id}`,
          `merchant_id=${dish.merchantId || ''}`,
          `shop_name=${encodeURIComponent(dish.shopName || '')}`,
          `month_sales=${dish.monthlySales || 0}`,
          `distance=${dish.distance_meters || 0}`,
          `delivery_time=${Math.round((dish.deliveryTimeSeconds || 0) / 60) || 0}`
        ].join('&')
        wx.navigateTo({
          url: `/pages/takeout/dish-detail/index?${params}`
        })
      }
    },

    onAdd(e: WechatMiniprogram.TouchEvent) {
      if ((e as any).stopPropagation) (e as any).stopPropagation()
      if (this.data.dish) {
        this.triggerEvent('add', { id: (this.data.dish as any).id })
      }
    }
  }
})
