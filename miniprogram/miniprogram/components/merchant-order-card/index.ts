import { OrderManagementAdapter } from '../../api/order-management'

Component({
    properties: {
        order: {
            type: Object,
            value: null
        }
    },

    methods: {
        onAccept() {
            const order: any = this.properties.order;
            if (order) {
                this.triggerEvent('metric-action', { action: 'accept', orderId: order.id })
            }
        },

        onReject() {
            const order: any = this.properties.order;
            if (order) {
                this.triggerEvent('metric-action', { action: 'reject', orderId: order.id })
            }
        },

        onMarkReady() {
            const order: any = this.properties.order;
            if (order) {
                this.triggerEvent('metric-action', { action: 'ready', orderId: order.id })
            }
        },

        onComplete() {
            const order: any = this.properties.order;
            if (order) {
                this.triggerEvent('metric-action', { action: 'complete', orderId: order.id })
            }
        },

        // 阻止冒泡
        stopPropagation() { }
    }
})
