"use strict";
// / <reference path="../../../typings/index.d.ts" />
Object.defineProperty(exports, "__esModule", { value: true });
const image_1 = require("../../utils/image");
Component({
    properties: {
        restaurant: {
            type: Object,
            value: {},
            observer(newVal) {
                // 当restaurant数据更新时，优化图片URL
                if (newVal && newVal.imageUrl && !newVal._imageOptimized) {
                    const optimizedUrl = (0, image_1.formatImageUrl)(newVal.imageUrl, image_1.ImageSize.MEDIUM);
                    this.setData({
                        'restaurant.imageUrl': optimizedUrl,
                        'restaurant._imageOptimized': true
                    });
                }
            }
        }
    },
    methods: {
        onTap() {
            if (this.data.restaurant) {
                wx.navigateTo({
                    url: `/pages/takeout/restaurant-detail/index?id=${this.data.restaurant.id}`
                });
            }
        }
    }
});
