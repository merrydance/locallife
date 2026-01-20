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
                const restaurantVal = newVal;
                if (restaurantVal && restaurantVal.imageUrl && !restaurantVal._imageOptimized) {
                    const optimizedUrl = (0, image_1.formatImageUrl)(restaurantVal.imageUrl, image_1.ImageSize.MEDIUM);
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
            const restaurant = this.data.restaurant;
            if (restaurant === null || restaurant === void 0 ? void 0 : restaurant.id) {
                wx.navigateTo({
                    url: `/pages/takeout/restaurant-detail/index?id=${restaurant.id}`
                });
            }
        }
    }
});
