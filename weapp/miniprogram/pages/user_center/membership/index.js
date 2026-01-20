"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const personal_1 = require("../../../api/personal");
const util_1 = require("../../../utils/util");
Page({
    data: {
        memberships: [],
        benefits: [
            { icon: 'discount', name: '会员折扣', desc: '专属会员价' },
            { icon: 'gift', name: '积分奖励', desc: '消费得积分' },
            { icon: 'service', name: '专属客服', desc: '优先接入' },
            { icon: 'shop', name: '余额充值', desc: '充值有优惠' }
        ],
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.loadMemberships();
    },
    onShow() {
        this.loadMemberships();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    async loadMemberships() {
        this.setData({ loading: true });
        try {
            const response = await (0, personal_1.getMyMemberships)();
            const memberships = response.memberships.map((m) => ({
                id: m.id,
                merchantName: m.merchant_name || '商户',
                balance: m.balance,
                balanceDisplay: (0, util_1.formatPriceNoSymbol)(m.balance || 0),
                totalRechargedDisplay: (0, util_1.formatPriceNoSymbol)(m.total_recharged || 0),
                totalConsumedDisplay: (0, util_1.formatPriceNoSymbol)(m.total_consumed || 0)
            }));
            this.setData({
                memberships,
                loading: false
            });
        }
        catch (error) {
            console.error('加载会员卡失败:', error);
            wx.showToast({ title: '加载失败', icon: 'error' });
            this.setData({ loading: false, memberships: [] });
        }
    },
});
