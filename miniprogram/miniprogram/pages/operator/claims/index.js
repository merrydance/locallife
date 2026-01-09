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
const responsive_1 = require("@/utils/responsive");
const operator_analytics_1 = require("../../../api/operator-analytics");
Page({
    data: {
        claims: [],
        isLargeScreen: false,
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadClaims();
    },
    onShow() {
        this.loadClaims();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadClaims() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const response = yield operator_analytics_1.operatorAppealService.getAppealList({
                    page: 1,
                    limit: 20
                });
                const claims = response.appeals.map((a) => ({
                    id: a.id,
                    type: a.appeal_type,
                    reporter: a.user_name || `用户${a.user_id}`,
                    target: a.merchant_id ? `商家${a.merchant_id}` : (a.rider_id ? `骑手${a.rider_id}` : '未知'),
                    desc: a.description,
                    status: a.status,
                    result: a.resolution_time ? `处理时间: ${a.resolution_time}分钟` : undefined,
                    created_at: a.created_at
                }));
                this.setData({
                    claims,
                    loading: false
                });
            }
            catch (error) {
                console.error('加载申诉列表失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false, claims: [] });
            }
        });
    },
    onResolve(e) {
        const { id } = e.currentTarget.dataset;
        if (!id)
            return;
        wx.showActionSheet({
            itemList: ['批准申诉', '驳回申诉'],
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                const statuses = ['approved', 'rejected'];
                const status = statuses[res.tapIndex];
                try {
                    const reviewData = {
                        status,
                        review_notes: status === 'approved' ? '申诉通过' : '申诉驳回'
                    };
                    yield operator_analytics_1.operatorAppealService.reviewAppeal(Number(id), reviewData);
                    wx.showToast({ title: '仲裁完成', icon: 'success' });
                    this.loadClaims();
                }
                catch (error) {
                    console.error('仲裁失败:', error);
                    wx.showToast({ title: '仲裁失败', icon: 'error' });
                }
            })
        });
    }
});
