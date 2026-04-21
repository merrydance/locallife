import { operatorBasicManagementService, OperatorBasicManagementAdapter } from '../../../api/operator-basic-management'
import { getErrorUserMessage } from '../../../utils/user-facing'

type RegionListItem = ReturnType<typeof OperatorBasicManagementAdapter.adaptRegionResponse>
type RegionPageTarget = 'delivery' | 'rules'

interface RegionPageOptions {
    target?: string
}

Page({
    data: {
        regions: [] as RegionListItem[],
        initialLoading: true,
        error: '',
        navBarHeight: 0,
        target: 'delivery' as RegionPageTarget,
        pageTitle: '区域管理'
    },

    onLoad(options: RegionPageOptions) {
        const target: RegionPageTarget = options?.target === 'rules' ? 'rules' : 'delivery'
        this.setData({
            target,
            pageTitle: target === 'rules' ? '选择规则配置区县' : '区域管理'
        })
        this.loadRegions()
    },

    onPullDownRefresh() {
        this.loadRegions()
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
        this.setData({
            navBarHeight: e.detail.navBarHeight
        })
    },

    onRetry() {
        this.loadRegions()
    },

    async loadRegions() {
        this.setData({ initialLoading: true, error: '', regions: [] })
        try {
            const res = await operatorBasicManagementService.getOperatorRegions({
                page: 1,
                limit: 100
            })

            const newRegions = (res.regions || []).map((r) => OperatorBasicManagementAdapter.adaptRegionResponse(r))

            this.setData({
                regions: newRegions,
                initialLoading: false
            })

        } catch (err: unknown) {
            console.error(err)
            const errorMsg = getErrorUserMessage(err, '加载区域列表失败，请稍后重试')
            this.setData({
                error: errorMsg,
                initialLoading: false
            })
        } finally {
            wx.stopPullDownRefresh()
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
