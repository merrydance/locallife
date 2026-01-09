"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
Component({
    properties: {
        order: {
            type: Object,
            value: null
        }
    },
    methods: {
        onAccept() {
            const order = this.properties.order;
            if (order) {
                this.triggerEvent('metric-action', { action: 'accept', orderId: order.id });
            }
        },
        onReject() {
            const order = this.properties.order;
            if (order) {
                this.triggerEvent('metric-action', { action: 'reject', orderId: order.id });
            }
        },
        onMarkReady() {
            const order = this.properties.order;
            if (order) {
                this.triggerEvent('metric-action', { action: 'ready', orderId: order.id });
            }
        },
        onComplete() {
            const order = this.properties.order;
            if (order) {
                this.triggerEvent('metric-action', { action: 'complete', orderId: order.id });
            }
        },
        // 阻止冒泡
        stopPropagation() { }
    }
});
