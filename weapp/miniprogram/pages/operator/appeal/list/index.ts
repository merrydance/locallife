import {
    AppealResponse,
    AppealStatus,
    formatAppealStatus,
    operatorAppealReviewService
} from '../../../../api/appeals-customer-service'

type AppealListStatus = AppealStatus

type AppealListItemView = AppealResponse & {
    status_label: string
    claim_amount_display: string
    summary_text: string
}

function adaptAppealItem(item: AppealResponse): AppealListItemView {
    return {
        ...item,
        status_label: formatAppealStatus(item.status),
        claim_amount_display: item.claim_amount ? `¥${(item.claim_amount / 100).toFixed(2)}` : '-',
        summary_text: item.order_no ? `订单 ${item.order_no}` : `索赔单 ${item.claim_id}`
    }
}

Page({
    data: {
        appeals: [] as AppealListItemView[],
        loading: false,
        loadingMore: false,
        refreshing: false,
        initialLoading: true,
        error: null as string | null,
        status: 'pending' as AppealListStatus,
        navBarHeight: 88,
        regionId: 0,
        page: 1,
        limit: 20,
        total: 0,
        hasMore: true
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
            this.loadAppeals(true, true)
        }
    },

    onPullDownRefresh() {
        this.setData({ refreshing: true })
        this.loadAppeals(true).finally(() => {
            this.setData({ refreshing: false })
            wx.stopPullDownRefresh()
        })
    },

    onReachBottom() {
        if (this.data.hasMore && !this.data.loading && !this.data.loadingMore) {
            this.loadAppeals(false)
        }
    },

    async loadAppeals(reset = true, silent = false) {
        if (this.data.loading || (this.data.loadingMore && !reset)) return

        if (reset) {
            this.setData({
                loading: !silent,
                error: null,
                page: 1,
                hasMore: true
            })
        } else {
            this.setData({ loadingMore: true })
        }

        try {
            const res = await operatorAppealReviewService.getPendingAppeals({
                status: this.data.status,
                page: reset ? 1 : this.data.page,
                limit: this.data.limit,
                ...(this.data.regionId ? { region_id: this.data.regionId } : {})
            })

            const incoming = (res.appeals || []).map(adaptAppealItem)
            const appeals = reset ? incoming : [...this.data.appeals, ...incoming]
            const total = Number(res.total || appeals.length)

            this.setData({
                appeals,
                total,
                page: reset ? 2 : this.data.page + 1,
                hasMore: Boolean(res.has_more) || appeals.length < total,
                loading: false,
                loadingMore: false,
                initialLoading: false
            })
        } catch (error) {
            console.error('加载申诉列表失败:', error)
            this.setData({ 
                loading: false,
                loadingMore: false,
                initialLoading: false,
                error: '加载申诉列表失败'
            })
        }
    },

    onRetry() {
        this.loadAppeals()
    },

    onTabChange(e: WechatMiniprogram.CustomEvent<{ value: AppealListStatus }>) {
        this.setData({ status: e.detail.value })
        this.loadAppeals(true)
    },

    onDetail(e: WechatMiniprogram.TouchEvent) {
        const { id } = e.currentTarget.dataset as { id?: number }
        if (!id) return
        wx.navigateTo({
            url: `/pages/operator/appeal/detail/index?id=${id}`
        })
    }
})
