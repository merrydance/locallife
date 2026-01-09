import { operatorBasicManagementService, OperatorBasicManagementAdapter } from '../../../api/operator-basic-management'
import type { RegionResponse } from '../../../api/operator-basic-management'

Page({
    data: {
        regions: [] as any[],
        loading: false,
        page: 1,
        pageSize: 20,
        hasMore: true
    },

    onLoad() {
        this.loadRegions(true)
    },

    onPullDownRefresh() {
        this.loadRegions(true)
    },

    onReachBottom() {
        if (this.data.hasMore && !this.data.loading) {
            this.loadRegions(false)
        }
    },

    async loadRegions(reset = false) {
        if (this.data.loading) return

        this.setData({ loading: true })
        if (reset) {
            this.setData({ page: 1, regions: [], hasMore: true })
        }

        try {
            const res = await operatorBasicManagementService.getOperatorRegions({
                page: this.data.page,
                limit: this.data.pageSize
            })

            const newRegions = res.regions.map(r => OperatorBasicManagementAdapter.adaptRegionResponse(r))

            this.setData({
                regions: reset ? newRegions : [...this.data.regions, ...newRegions],
                page: this.data.page + 1,
                hasMore: res.has_more,
                loading: false
            })

        } catch (err) {
            console.error(err)
            wx.showToast({ title: '加载失败', icon: 'error' })
            this.setData({ loading: false })
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
