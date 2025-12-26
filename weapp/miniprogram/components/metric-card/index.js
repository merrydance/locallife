"use strict";
Component({
    properties: {
        label: {
            type: String,
            value: ''
        },
        value: {
            type: String,
            value: '0'
        },
        trend: {
            type: Number,
            value: 0 // 0: none, 1: up, -1: down
        },
        trendValue: {
            type: String,
            value: ''
        }
    }
});
