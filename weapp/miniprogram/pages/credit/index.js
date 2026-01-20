"use strict";
Page({
    data: {
        score: null,
        history: [],
        loading: false,
        chartValue: 0,
        gradientColor: {
            '0%': '#E34D59',
            '100%': '#FFB000',
        }
    },
    onLoad() {
        this.showDeprecatedNotice();
    },
    showDeprecatedNotice() {
        this.setData({
            score: null,
            history: [],
            loading: false,
            chartValue: 0
        });
        wx.showToast({ title: '信用分功能已下线', icon: 'none' });
    }
});
