import { operatorAppealReviewService, AppealResponse } from '../../../../api/appeals-customer-service';

Page({
    data: {
        id: 0,
        appeal: null as AppealResponse | null,
        replyContent: '',
        showRejectDialog: false,
        initialLoading: true,
        loading: false,
        submitting: false,
        error: null as string | null,
        navBarHeight: 88,
    },

    onLoad(options: any) {
        if (options.id) {
            this.setData({ id: parseInt(options.id) });
            this.loadDetail(parseInt(options.id));
        } else {
            this.setData({ 
                initialLoading: false,
                error: '未提供申诉ID'
            });
        }
    },

    onNavHeight(e: any) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    async loadDetail(id: number) {
        this.setData({ loading: true, error: null });
        try {
            const appeal = await operatorAppealReviewService.getAppealDetailForReview(id);
            this.setData({ 
                appeal,
                loading: false,
                initialLoading: false
            });
        } catch (error) {
            console.error('加载详情失败:', error);
            this.setData({ 
                loading: false,
                initialLoading: false,
                error: '加载详情失败'
            });
        }
    },

    onRetry() {
        if (this.data.id) {
            this.loadDetail(this.data.id);
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
        if (status === 'rejected' && (!replyContent || replyContent.trim().length < 5)) {
            wx.showToast({ title: '驳回说明至少5个字符', icon: 'none' });
            return;
        }

        try {
            this.setData({ submitting: true });
            wx.showLoading({ title: '处理中...', mask: true });
            await operatorAppealReviewService.reviewAppeal(id, {
                status: status,
                review_notes: replyContent
            });
            wx.showToast({ title: '处理成功', icon: 'success' });
            setTimeout(() => wx.navigateBack(), 1500);
        } catch (error: any) {
            wx.showToast({ title: error.message || '处理失败', icon: 'none' });
        } finally {
            this.setData({ submitting: false });
            wx.hideLoading();
        }
    }
});
