"use strict";
// / <reference path="../../../typings/index.d.ts" />
Object.defineProperty(exports, "__esModule", { value: true });
const image_1 = require("../../utils/image");
Component({
    properties: {
        dish: {
            type: Object,
            value: {},
            observer(newVal) {
                // 当dish数据更新时，优化图片URL
                if (newVal && newVal.imageUrl && !newVal._imageOptimized) {
                    const optimizedUrl = (0, image_1.formatImageUrl)(newVal.imageUrl, image_1.ImageSize.CARD);
                    this.setData({
                        'dish.imageUrl': optimizedUrl,
                        'dish._imageOptimized': true
                    });
                }
            }
        }
    },
    methods: {
        onTap() {
            const dish = this.data.dish;
            if (dish) {
                // 传递额外信息到详情页（小程序不支持 URLSearchParams）
                const params = [
                    `id=${dish.id}`,
                    `merchant_id=${dish.merchantId || ''}`,
                    `shop_name=${encodeURIComponent(dish.shopName || '')}`,
                    `month_sales=${dish.monthlySales || 0}`,
                    `distance=${dish.distance_meters || 0}`,
                    `delivery_time=${Math.round((dish.deliveryTimeSeconds || 0) / 60) || 0}`
                ].join('&');
                wx.navigateTo({
                    url: `/pages/takeout/dish-detail/index?${params}`
                });
            }
        },
        onAdd(e) {
            if (e.stopPropagation)
                e.stopPropagation();
            if (this.data.dish) {
                this.triggerEvent('add', { id: this.data.dish.id });
            }
        }
    }
});
