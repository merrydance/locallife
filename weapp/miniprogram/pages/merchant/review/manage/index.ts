import { ReviewService, ReviewResponse } from '../../../../api/review';
import { isLargeScreen } from '@/utils/responsive';

Page({
    data: {
        currentTab: 'unreplied',
        tabs: [
            { value: 'unreplied', label: '未回复' },
            { value: 'replied', label: '已回复' },
            { value: 'all', label: '全部' }
        ],
        reviews: [] as ReviewResponse[],
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
        this.setData({ isLargeScreen: isLargeScreen() });
        this.loadReviews(true);
    },

    onTabChange(e: any) {
        this.setData({
            currentTab: e.detail.value,
            reviews: [],
            page: 1,
            hasMore: true
        });
        this.loadReviews(true);
    },

    async loadReviews(reset: boolean) {
        if (this.data.loading && !reset) return;

        this.setData({ loading: true });

        try {
            const page = reset ? 1 : this.data.page;

            // Map tab to API params
            let has_reply: boolean | undefined;
            if (this.data.currentTab === 'unreplied') has_reply = false;
            if (this.data.currentTab === 'replied') has_reply = true;

            const res = await ReviewService.getReviews({
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

        } catch (error) {
            console.error(error);
            this.setData({ loading: false });
        }
    },

    onReply(e: any) {
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

    onReplyInput(e: any) {
        this.setData({ replyContent: e.detail.value });
    },

    async confirmReply() {
        if (!this.data.replyContent) {
            wx.showToast({ title: '请输入回复内容', icon: 'none' });
            return;
        }

        try {
            wx.showLoading({ title: '提交中' });
            await ReviewService.replyReview(this.data.selectedId, this.data.replyContent);
            wx.showToast({ title: '已回复', icon: 'success' });
            this.closeReplyDialog();
            this.loadReviews(true);
        } catch (error: any) {
            wx.showToast({ title: error.message || '回复失败', icon: 'none' });
        } finally {
            wx.hideLoading();
        }
    }
});
