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
      if (this.data.dish) {
        wx.navigateTo({
          url: `/pages/takeout/dish-detail/index?id=${this.data.dish.id}`
        })
      }
    },

    onAdd(e: WechatMiniprogram.TouchEvent) {
      if (e.stopPropagation) e.stopPropagation()
      if (this.data.dish) {
        this.triggerEvent('add', { id: this.data.dish.id })
      }
    }
  }
})
