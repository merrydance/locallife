import { operatorBasicManagementService, OperatorBasicManagementAdapter } from '../../../api/operator-basic-management'

Page({
    data: {
        regions: [] as any[],
        initialLoading: true,
        loadingMore: false,
        error: '',
        page: 1,
        pageSize: 20,
        hasMore: true,
        navBarHeight: 0
    },

    onLoad() {
        this.loadRegions(true)
    },

    onPullDownRefresh() {
        this.loadRegions(true)
    },

    onReachBottom() {
        if (this.data.hasMore && !this.data.loadingMore && !this.data.initialLoading) {
            this.loadRegions(false)
        }
    },

    onNavHeight(e: any) {
        this.setData({
            navBarHeight: e.detail.navBarHeight
        })
    },

    onRetry() {
        this.loadRegions(true)
    },

    async loadRegions(reset = false) {
        if (reset) {
            this.setData({ initialLoading: true, error: '', page: 1, regions: [], hasMore: true })
        } else {
            this.setData({ loadingMore: true })
        }

        try {
            const res = await operatorBasicManagementService.getOperatorRegions({
                page: this.data.page,
                limit: this.data.pageSize
            })

            const newRegions = (res.regions || []).map(r => OperatorBasicManagementAdapter.adaptRegionResponse(r))

            this.setData({
                regions: reset ? newRegions : [...this.data.regions, ...newRegions],
                page: this.data.page + 1,
                hasMore: res.has_more,
                initialLoading: false,
                loadingMore: false
            })

        } catch (err: any) {
            console.error(err)
            const errorMsg = err.message || '加载区域列表失败'
            this.setData({
                error: errorMsg,
                initialLoading: false,
                loadingMore: false
            })
            if (!reset) {
                wx.showToast({ title: errorMsg, icon: 'none' })
            }
        } finally {
            if (reset) wx.stopPullDownRefresh()
        }
    },

    // 跳转到详细配置页
    onRegionClick(e: any) {
        const id = e.currentTarget.dataset.id
        wx.navigateTo({
            url: `/pages/operator/region/config?id=${id}`
        })
    },

    onAddRegion() {
        wx.showToast({ title: '添加功能暂未开放', icon: 'none' })
    }
})
