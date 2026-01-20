"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const responsive_1 = require("../../../utils/responsive");
const app = getApp();
Page({
    data: {
        score: 0,
        level: '健康',
        levelDesc: '经营状况良好',
        metrics: [],
        violations: [],
        isLargeScreen: false,
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadHealthInfo();
    },
    onShow() {
        this.loadHealthInfo();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    async loadHealthInfo() {
        this.setData({
            score: 0,
            level: '已下线',
            levelDesc: '信用分功能已下线',
            metrics: [],
            violations: [],
            loading: false
        });
        wx.showToast({ title: '信用分功能已下线', icon: 'none' });
    },
    calculateLevelInfo(score) {
        if (score >= 90) {
            return { level: '优秀', levelDesc: '经营状况优秀' };
        }
        else if (score >= 80) {
            return { level: '健康', levelDesc: '经营状况良好' };
        }
        else if (score >= 60) {
            return { level: '一般', levelDesc: '需要改进' };
        }
        else {
            return { level: '警告', levelDesc: '存在风险' };
        }
    },
    onAppeal(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({ url: `/pages/merchant/appeals/index?id=${id}` });
    }
});
