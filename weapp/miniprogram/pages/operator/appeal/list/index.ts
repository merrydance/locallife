import { AppealResponse, operatorAppealReviewService } from '../../../../api/appeals-customer-service'

Page({
    data: {
        appeals: [] as AppealResponse[],
        loading: false,
        initialLoading: true,
        error: null as string | null,
        status: 'pending' as 'pending' | 'approved' | 'rejected',
        navBarHeight: 88,
        regionId: 0
    },

    onLoad(options: { region_id?: string }) {
        const regionId = options.region_id ? parseInt(options.region_id) : 0
        this.setData({ regionId })
        this.loadAppeals()
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    onShow() {
        // Refresh when returning from detail
        if (!this.data.initialLoading) {
            this.loadAppeals(true)
        }
    },

    async loadAppeals(silent = false) {
        if (this.data.loading && !this.data.initialLoading) return
        
        if (!silent) {
            this.setData({ loading: true, error: null })
        }

        try {
            // Using operator service for operator page
            const res = await operatorAppealReviewService.getPendingAppeals({
                status: this.data.status,
                page: 1,
                limit: 20,
                ...(this.data.regionId ? { region_id: this.data.regionId } : {})
            })
            
            this.setData({
                appeals: res.appeals || [],
                loading: false,
                initialLoading: false
            })
        } catch (error) {
            console.error('加载申诉列表失败:', error)
            this.setData({ 
                loading: false,
                initialLoading: false,
                error: '加载申诉列表失败'
            })
        }
    },

    onRetry() {
        this.loadAppeals()
    },

    onTabChange(e: WechatMiniprogram.CustomEvent<{ value: 'pending' | 'approved' | 'rejected' }>) {
        this.setData({ status: e.detail.value })
        this.loadAppeals()
    },

    onDetail(e: WechatMiniprogram.TouchEvent) {
        const { id } = e.currentTarget.dataset as { id?: number }
        if (!id) return
        wx.navigateTo({
            url: `/pages/operator/appeal/detail/index?id=${id}`
        })
    }
})
