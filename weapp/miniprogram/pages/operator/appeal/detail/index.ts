import { operatorAppealReviewService, AppealResponse } from '../../../../api/appeals-customer-service';

Page({
    data: {
        id: 0,
        appeal: null as AppealResponse | null,
        replyContent: '',
        showRejectDialog: false
    },

    onLoad(options: any) {
        if (options.id) {
            this.setData({ id: parseInt(options.id) });
            this.loadDetail(parseInt(options.id));
        }
    },

    async loadDetail(id: number) {
        try {
            const appeal = await operatorAppealReviewService.getAppealDetailForReview(id);
            this.setData({ appeal });
        } catch (error) {
            console.error(error);
            wx.showToast({ title: '加载失败', icon: 'none' });
        }
    },

    onInput(e: any) {
        this.setData({ replyContent: e.detail.value });
    },

    async onApprove() {
        // Assume 'resolved' is the success status or we have a specific approve action
        await this.handleAppeal('approved');
    },

    onReject() {
        this.setData({ showRejectDialog: true });
    },

    async onRejectConfirm() {
        await this.handleAppeal('rejected');
        this.setData({ showRejectDialog: false });
    },

    onRejectCancel() {
        this.setData({ showRejectDialog: false });
    },

    async handleAppeal(status: 'approved' | 'rejected') {
        const { id, replyContent } = this.data;
        try {
            wx.showLoading({ title: '处理中' });
            await operatorAppealReviewService.reviewAppeal(id, {
                status: status,
                review_notes: replyContent // Using replyContent as note
            });
            wx.showToast({ title: '处理成功', icon: 'success' });
            setTimeout(() => wx.navigateBack(), 1500);
        } catch (error: any) {
            wx.showToast({ title: error.message || '处理失败', icon: 'none' });
        } finally {
            wx.hideLoading();
        }
    }
});
