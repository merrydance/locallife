// / <reference path="../../../typings/index.d.ts" />

import { formatImageUrl, ImageSize } from '../../utils/image'

Component({
  properties: {
    restaurant: {
      type: Object,
      value: {},
      observer(newVal: Record<string, unknown>) {
        // 当restaurant数据更新时，优化图片URL
        if (newVal && newVal.imageUrl && !newVal._imageOptimized) {
          const optimizedUrl = formatImageUrl(newVal.imageUrl, ImageSize.MEDIUM)
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
      if (this.data.restaurant) {
        wx.navigateTo({
          url: `/pages/takeout/restaurant-detail/index?id=${this.data.restaurant.id}`
        })
      }
    }
  }
})
