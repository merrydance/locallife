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
            if (this.data.dish) {
                wx.navigateTo({
                    url: `/pages/takeout/dish-detail/index?id=${this.data.dish.id}`
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
