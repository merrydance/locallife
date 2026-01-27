import { appealManagementService, AppealResponse, operatorAppealReviewService } from '../../../../api/appeals-customer-service';

Page({
    data: {
        appeals: [] as AppealResponse[],
        loading: false,
        initialLoading: true,
        error: null as string | null,
        status: 'pending' as 'pending' | 'processed',
        navBarHeight: 88,
    },

    onLoad() {
        this.loadAppeals();
    },

    onNavHeight(e: any) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    onShow() {
        // Refresh when returning from detail
        if (!this.data.initialLoading) {
            this.loadAppeals(true);
        }
    },

    async loadAppeals(silent = false) {
        if (this.data.loading && !this.data.initialLoading) return;
        
        if (!silent) {
            this.setData({ loading: true, error: null });
        }

        try {
            // Using operator service for operator page
            const res = await operatorAppealReviewService.getPendingAppeals({
                status: this.data.status === 'pending' ? 'pending' : 'approved',
                page_id: 1,
                page_size: 20
            });
            
            this.setData({
                appeals: res.appeals || [],
                loading: false,
                initialLoading: false
            });
        } catch (error) {
            console.error('加载申诉列表失败:', error);
            this.setData({ 
                loading: false,
                initialLoading: false,
                error: '加载申诉列表失败'
            });
        }
    },

    onRetry() {
        this.loadAppeals();
    },

    onTabChange(e: any) {
        this.setData({ status: e.detail.value });
        this.loadAppeals();
    },

    onDetail(e: any) {
        const id = e.currentTarget.dataset.id;
        wx.navigateTo({
            url: `/pages/operator/appeal/detail/index?id=${id}`
        });
    }
});
