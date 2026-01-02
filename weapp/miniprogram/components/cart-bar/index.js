"use strict";
Component({
    properties: {
        totalPrice: {
            type: Number,
            value: 0
        },
        totalCount: {
            type: Number,
            value: 0
        },
        deliveryFee: {
            type: Number,
            value: 0
        }
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
