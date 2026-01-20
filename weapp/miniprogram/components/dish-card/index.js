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
                const dishVal = newVal;
                if (dishVal && dishVal.imageUrl && !dishVal._imageOptimized) {
                    const optimizedUrl = (0, image_1.formatImageUrl)(dishVal.imageUrl, image_1.ImageSize.CARD);
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
                const monthSales = typeof dish.month_sales === 'number'
                    ? dish.month_sales
                    : typeof dish.salesBadge === 'string'
                        ? Number(dish.salesBadge.replace(/[^0-9]/g, ''))
                        : 0;
                const estimatedMinutes = Math.round((dish.estimated_delivery_time || 0) / 60);
                // 传递额外信息到详情页（小程序不支持 URLSearchParams）
                const params = [
                    `id=${dish.id}`,
                    `merchant_id=${dish.merchantId || ''}`,
                    `shop_name=${encodeURIComponent(dish.shopName || '')}`,
                    `month_sales=${monthSales}`,
                    `distance=${dish.distance_meters || 0}`,
                    `estimated_delivery_time=${estimatedMinutes || 0}`
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
