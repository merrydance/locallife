"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const rider_delivery_1 = require("../../api/rider-delivery");
Component({
    properties: {
        task: {
            type: Object,
            value: undefined
        },
        type: {
            type: String,
            value: 'active' // available, active, history
        }
    },
    methods: {
        onGrab() {
            const task = this.properties.task;
            if (!(task === null || task === void 0 ? void 0 : task.order_id))
                return;
            this.triggerEvent('grab', { id: task.order_id });
        },
        onPickup() {
            const task = this.properties.task;
            if (!(task === null || task === void 0 ? void 0 : task.delivery_id))
                return;
            this.triggerEvent('pickup', { id: task.delivery_id });
        },
        onConfirmPickup() {
            const task = this.properties.task;
            if (!(task === null || task === void 0 ? void 0 : task.delivery_id))
                return;
            this.triggerEvent('confirmPickup', { id: task.delivery_id });
        },
        onDeliver() {
            const task = this.properties.task;
            if (!(task === null || task === void 0 ? void 0 : task.delivery_id))
                return;
            this.triggerEvent('deliver', { id: task.delivery_id });
        },
        onConfirmDeliver() {
            const task = this.properties.task;
            if (!(task === null || task === void 0 ? void 0 : task.delivery_id))
                return;
            this.triggerEvent('confirmDeliver', { id: task.delivery_id });
        },
        onException() {
            const task = this.properties.task;
            if (!(task === null || task === void 0 ? void 0 : task.delivery_id))
                return;
            this.triggerEvent('exception', { id: task.delivery_id });
        },
        onCall(e) {
            const phone = e.currentTarget.dataset.phone;
            if (phone) {
                wx.makePhoneCall({ phoneNumber: phone });
            }
        },
        // Formatters exposed to WXML
        formatAmount(amount) {
            return rider_delivery_1.DeliveryAdapter.formatAmount(amount);
        },
        formatDistance(distance) {
            return rider_delivery_1.DeliveryAdapter.formatDistance(distance);
        },
        formatDeliveryStatus(status) {
            return rider_delivery_1.DeliveryAdapter.formatDeliveryStatus(status);
        },
        formatTime(dateStr) {
            if (!dateStr)
                return '';
            const date = new Date(dateStr);
            const hours = ('0' + date.getHours()).slice(-2);
            const minutes = ('0' + date.getMinutes()).slice(-2);
            return `${hours}:${minutes}`;
        },
        calculateEstimatedArrival(createdAt, estimatedTime) {
            return rider_delivery_1.DeliveryAdapter.calculateEstimatedArrival(createdAt, estimatedTime);
        }
    }
});
