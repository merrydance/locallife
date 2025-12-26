"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
const personal_1 = require("../../../api/personal");
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
    loadMemberships() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const response = yield (0, personal_1.getMyMemberships)();
                const memberships = response.memberships.map((m) => ({
                    id: m.id,
                    merchantName: m.merchant_name || '商户',
                    merchantLogo: m.merchant_logo_url || '',
                    level: m.level,
                    levelName: this.getLevelName(m.level),
                    balance: m.balance,
                    points: m.points,
                    discount: this.getLevelDiscount(m.level)
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
        });
    },
    getLevelName(level) {
        const levelMap = {
            'NORMAL': '普通会员',
            'SILVER': '银牌会员',
            'GOLD': '金牌会员',
            'PLATINUM': '铂金会员',
            'DIAMOND': '钻石会员'
        };
        return levelMap[level] || '普通会员';
    },
    getLevelDiscount(level) {
        const discountMap = {
            'NORMAL': 100,
            'SILVER': 98,
            'GOLD': 95,
            'PLATINUM': 92,
            'DIAMOND': 88
        };
        return discountMap[level] || 100;
    },
    onMembershipTap(e) {
        const { id } = e.currentTarget.dataset;
        if (id) {
            wx.navigateTo({
                url: `/pages/user_center/membership/detail/index?id=${id}`
            });
        }
    }
});
