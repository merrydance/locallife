"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const util_1 = require("../../utils/util");
Component({
    properties: {
        totalPrice: {
            type: Number,
            value: 0
        },
        totalPriceDisplay: {
            type: String,
            value: ''
        },
        totalCount: {
            type: Number,
            value: 0
        },
        deliveryFee: {
            type: Number,
            value: 0
        },
        deliveryFeeDisplay: {
            type: String,
            value: ''
        },
        alwaysShow: {
            type: Boolean,
            value: false
        },
        dockBottom: {
            type: Boolean,
            value: false // 贴底模式，用于餐厅详情页
        }
    },
    observers: {
        'totalPrice, totalPriceDisplay': function (price, display) {
            // 如果传入了格式化好的价格则使用，否则自己格式化
            if (!display && price >= 0) {
                this.setData({ computedPriceDisplay: (0, util_1.formatPriceNoSymbol)(price) });
            }
            else {
                this.setData({ computedPriceDisplay: display });
            }
        },
        'deliveryFee, deliveryFeeDisplay': function (fee, display) {
            if (!display && fee > 0) {
                this.setData({ computedDeliveryDisplay: (0, util_1.formatPriceNoSymbol)(fee) });
            }
            else {
                this.setData({ computedDeliveryDisplay: display });
            }
        }
    },
    data: {
        computedPriceDisplay: '0.00',
        computedDeliveryDisplay: ''
    },
    methods: {
        onCheckout() {
            if (this.properties.totalCount > 0) {
                this.triggerEvent('checkout');
            }
        },
        onCartClick() {
            this.triggerEvent('toggle');
        }
    }
});
