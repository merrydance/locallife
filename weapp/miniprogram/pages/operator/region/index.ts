import { operatorBasicManagementService, OperatorBasicManagementAdapter } from '../../../api/operator-basic-management'

type RegionListItem = ReturnType<typeof OperatorBasicManagementAdapter.adaptRegionResponse>
type RegionPageTarget = 'delivery' | 'rules'

interface RegionPageOptions {
    target?: string
}

Page({
    data: {
        regions: [] as RegionListItem[],
        initialLoading: true,
        loadingMore: false,
        error: '',
        page: 1,
        pageSize: 20,
        hasMore: true,
        navBarHeight: 0,
        target: 'delivery' as RegionPageTarget,
        pageTitle: '区域管理',
        subtitle: '管理您所负责的区域及其配送运费规则'
    },

    onLoad(options: RegionPageOptions) {
        const target: RegionPageTarget = options?.target === 'rules' ? 'rules' : 'delivery'
        this.setData({
            target,
            pageTitle: target === 'rules' ? '选择规则配置区县' : '区域管理',
            subtitle: target === 'rules' ? '请先选择要配置的区县，再进入规则配置页' : '管理您所负责的区域及其配送运费规则'
        })
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

    onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
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

            const newRegions = (res.regions || []).map((r) => OperatorBasicManagementAdapter.adaptRegionResponse(r))

            this.setData({
                regions: reset ? newRegions : [...this.data.regions, ...newRegions],
                page: this.data.page + 1,
                hasMore: Boolean(res.has_more),
                initialLoading: false,
                loadingMore: false
            })

        } catch (err: unknown) {
            console.error(err)
            const errorMsg = err instanceof Error ? err.message : '加载区域列表失败'
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
    onRegionClick(e: WechatMiniprogram.TouchEvent) {
        const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
        if (!id) return

        if (this.data.target === 'rules') {
            const regionName = name ? encodeURIComponent(name) : ''
            wx.navigateTo({
                url: `/pages/operator/rules/index?region_id=${id}&region_name=${regionName}`
            })
            return
        }

        const regionName = name ? encodeURIComponent(name) : ''
        wx.navigateTo({
            url: `/pages/operator/region/config?id=${id}&region_name=${regionName}`
        })
    },

    onAddRegion() {
        wx.navigateTo({ url: '/pages/operator/region-expansion/index' })
    }
})
