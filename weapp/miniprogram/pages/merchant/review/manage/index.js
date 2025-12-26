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
const review_1 = require("../../../../api/review");
const responsive_1 = require("@/utils/responsive");
Page({
    data: {
        currentTab: 'unreplied',
        tabs: [
            { value: 'unreplied', label: '未回复' },
            { value: 'replied', label: '已回复' },
            { value: 'all', label: '全部' }
        ],
        reviews: [],
        page: 1,
        pageSize: 10,
        hasMore: true,
        loading: false,
        // Reply Dialog
        showReplyDialog: false,
        selectedId: 0,
        replyContent: '',
        isLargeScreen: false
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadReviews(true);
    },
    onTabChange(e) {
        this.setData({
            currentTab: e.detail.value,
            reviews: [],
            page: 1,
            hasMore: true
        });
        this.loadReviews(true);
    },
    loadReviews(reset) {
        return __awaiter(this, void 0, void 0, function* () {
            if (this.data.loading && !reset)
                return;
            this.setData({ loading: true });
            try {
                const page = reset ? 1 : this.data.page;
                // Map tab to API params
                let has_reply;
                if (this.data.currentTab === 'unreplied')
                    has_reply = false;
                if (this.data.currentTab === 'replied')
                    has_reply = true;
                const res = yield review_1.ReviewService.getReviews({
                    page_id: page,
                    page_size: this.data.pageSize,
                    has_reply
                });
                this.setData({
                    reviews: reset ? res.reviews : [...this.data.reviews, ...res.reviews],
                    page: page + 1,
                    hasMore: res.reviews.length === this.data.pageSize,
                    loading: false
                });
            }
            catch (error) {
                console.error(error);
                this.setData({ loading: false });
            }
        });
    },
    onReply(e) {
        const id = e.detail.id; // From component event
        this.setData({
            showReplyDialog: true,
            selectedId: id,
            replyContent: ''
        });
    },
    closeReplyDialog() {
        this.setData({ showReplyDialog: false });
    },
    onReplyInput(e) {
        this.setData({ replyContent: e.detail.value });
    },
    confirmReply() {
        return __awaiter(this, void 0, void 0, function* () {
            if (!this.data.replyContent) {
                wx.showToast({ title: '请输入回复内容', icon: 'none' });
                return;
            }
            try {
                wx.showLoading({ title: '提交中' });
                yield review_1.ReviewService.replyReview(this.data.selectedId, this.data.replyContent);
                wx.showToast({ title: '已回复', icon: 'success' });
                this.closeReplyDialog();
                this.loadReviews(true);
            }
            catch (error) {
                wx.showToast({ title: error.message || '回复失败', icon: 'none' });
            }
            finally {
                wx.hideLoading();
            }
        });
    }
});
