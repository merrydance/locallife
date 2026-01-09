import { DeliveryAdapter } from '../../api/rider-delivery';

Component({
    properties: {
        task: {
            type: Object,
            value: null
        },
        type: {
            type: String,
            value: 'active' // available, active, history
        }
    },

    methods: {
        onGrab() {
            this.triggerEvent('grab', { id: this.properties.task.order_id });
        },

        onPickup() {
            this.triggerEvent('pickup', { id: this.properties.task.delivery_id });
        },

        onConfirmPickup() {
            this.triggerEvent('confirmPickup', { id: this.properties.task.delivery_id });
        },

        onDeliver() {
            this.triggerEvent('deliver', { id: this.properties.task.delivery_id });
        },

        onConfirmDeliver() {
            this.triggerEvent('confirmDeliver', { id: this.properties.task.delivery_id });
        },

        onException() {
            this.triggerEvent('exception', { id: this.properties.task.delivery_id });
        },

        onCall(e: WechatMiniprogram.TouchEvent) {
            const phone = e.currentTarget.dataset.phone;
            if (phone) {
                wx.makePhoneCall({ phoneNumber: phone });
            }
        },

        // Formatters exposed to WXML
        formatAmount(amount: number) {
            return DeliveryAdapter.formatAmount(amount);
        },

        formatDistance(distance: number) {
            return DeliveryAdapter.formatDistance(distance);
        },

        formatDeliveryStatus(status: string) {
            return DeliveryAdapter.formatDeliveryStatus(status);
        },

        formatTime(dateStr: string) {
            if (!dateStr) return '';
            const date = new Date(dateStr);
            const hours = ('0' + date.getHours()).slice(-2);
            const minutes = ('0' + date.getMinutes()).slice(-2);
            return `${hours}:${minutes}`;
        },

        calculateEstimatedArrival(createdAt: string, estimatedTime: number) {
            return DeliveryAdapter.calculateEstimatedArrival(createdAt, estimatedTime);
        }
    }
});
