"use strict";
Component({
    properties: {
        orders: {
            type: Array,
            value: []
        }
    },
    methods: {
        onAction(e) {
            const { id, action } = e.currentTarget.dataset;
            this.triggerEvent('action', { id, action });
        }
    }
});
