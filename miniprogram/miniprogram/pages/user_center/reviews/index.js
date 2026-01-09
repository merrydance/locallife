"use strict";
/**
 * 我的评价页面
 * 使用真实后端API
 */
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
        reviews: [],
        loading: false,
        navBarHeight: 88,
        activeTab: 0,
        page: 1,
        pageSize: 10,
        hasMore: true
    },
    onLoad() {
        this.loadReviews(true);
    },
    onShow() {
        // 返回时刷新
        if (this.data.reviews.length > 0) {
            this.loadReviews(true);
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    onTabChange(e) {
        this.setData({ activeTab: e.detail.value });
        this.loadReviews(true);
    },
    onReachBottom() {
        if (this.data.hasMore && !this.data.loading) {
            this.setData({ page: this.data.page + 1 });
            this.loadReviews(false);
        }
    },
    loadReviews() {
        return __awaiter(this, arguments, void 0, function* (reset = false) {
            if (this.data.loading)
                return;
            this.setData({ loading: true });
            if (reset) {
                this.setData({ page: 1, reviews: [], hasMore: true });
            }
            try {
                const { page, pageSize } = this.data;
                const result = yield (0, personal_1.getMyReviews)({
                    page_id: page,
                    page_size: pageSize
                });
                const reviews = (result.reviews || []).map((review) => ({
                    id: review.id,
                    order_id: review.order_id,
                    merchant_id: review.merchant_id,
                    content: review.content,
                    images: review.images || [],
                    created_at: review.created_at,
                    reply: review.merchant_reply,
                    replied_at: review.replied_at,
                    is_visible: review.is_visible
                }));
                const hasMore = reviews.length === pageSize;
                const newReviews = reset ? reviews : [...this.data.reviews, ...reviews];
                this.setData({
                    reviews: newReviews,
                    loading: false,
                    hasMore
                });
            }
            catch (error) {
                console.error('加载评价失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    }
});
