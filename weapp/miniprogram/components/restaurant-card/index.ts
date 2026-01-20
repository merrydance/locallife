// / <reference path="../../../typings/index.d.ts" />

import { formatImageUrl, ImageSize } from '../../utils/image'

type RestaurantCardData = {
  id?: number
  imageUrl?: string
  _imageOptimized?: boolean
}

Component({
  properties: {
    restaurant: {
      type: Object,
      value: {},
      observer(newVal: Record<string, unknown>) {
        // 当restaurant数据更新时，优化图片URL
        const restaurantVal = newVal as RestaurantCardData
        if (restaurantVal && restaurantVal.imageUrl && !restaurantVal._imageOptimized) {
          const optimizedUrl = formatImageUrl(restaurantVal.imageUrl, ImageSize.MEDIUM)
          this.setData({
            'restaurant.imageUrl': optimizedUrl,
            'restaurant._imageOptimized': true
          })
        }
      }
    }
  },

  methods: {
    onTap() {
      const restaurant = this.data.restaurant as RestaurantCardData
      if (restaurant?.id) {
        wx.navigateTo({
          url: `/pages/takeout/restaurant-detail/index?id=${restaurant.id}`
        })
      }
    }
  }
})
