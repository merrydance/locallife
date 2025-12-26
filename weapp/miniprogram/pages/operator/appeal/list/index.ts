import { appealManagementService, AppealResponse, operatorAppealReviewService } from '../../../../api/appeals-customer-service';

Page({
    data: {
        appeals: [] as AppealResponse[],
        loading: false,
        status: 'pending' as 'pending' | 'processed'
    },

    onLoad() {
        this.loadAppeals();
    },

    onShow() {
        // Refresh when returning from detail
        this.loadAppeals();
    },

    async loadAppeals() {
        this.setData({ loading: true });
        try {
            // Using operator service for operator page
            const appeals = await operatorAppealReviewService.getPendingAppeals({
                status: this.data.status === 'pending' ? 'pending' : 'approved', // Simplified status mapping
                page_id: 1,
                page_size: 20
            });
            this.setData({
                appeals: appeals.appeals || [],
                loading: false
            });
        } catch (error) {
            console.error(error);
            this.setData({ loading: false });
        }
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
