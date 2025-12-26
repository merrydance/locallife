"use strict";
Component({
    properties: {
        task: {
            type: Object,
            value: {}
        }
    },
    methods: {
        onAction() {
            const { task } = this.properties;
            if (!task || !task.id)
                return;
            let nextAction = '';
            const s = task.status;
            // Handle both string and numeric status
            if (s === 'PENDING' || s === 'CONFIRMED' || s === 'CREATED' || s === 0)
                nextAction = 'accept';
            else if (s === 'ACCEPTED' || s === 1)
                nextAction = 'pickup';
            else if (s === 'PICKED_UP' || s === 'DELIVERING' || s === 2)
                nextAction = 'deliver';
            if (nextAction) {
                this.triggerEvent('action', { id: task.id, action: nextAction });
            }
        },
        onCall(e) {
            const { type } = e.currentTarget.dataset;
            const { task } = this.properties;
            // Use task phone if available, else fallback (per ISSUES.md)
            const phoneNumber = type === 'shop' ? (task.merchant_phone || '13800138000') : (task.customer_phone || '13900139000');
            wx.makePhoneCall({ phoneNumber });
        }
    }
});
