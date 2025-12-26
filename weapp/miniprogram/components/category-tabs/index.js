"use strict";
Component({
    properties: {
        categories: {
            type: Array,
            value: []
        },
        activeId: {
            type: String,
            value: ''
        }
    },
    methods: {
        onTabClick(e) {
            const { id } = e.currentTarget.dataset;
            this.triggerEvent('change', { id });
        }
    }
});
