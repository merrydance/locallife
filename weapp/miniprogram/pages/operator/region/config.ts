import {
    createEmptyOperatorPeakConfigSummary,
    createEmptyOperatorRegionConfigSummary,
    loadOperatorRegionConfigOverview
} from '../_services/operator-region-config'
import { getErrorUserMessage } from '../../../utils/user-facing'

interface RegionConfigPageOptions {
    id?: string
    region_name?: string
}

Page({
    data: {
        regionId: 0,
        regionName: '',
        initialLoading: true,
        error: '',
        navBarHeight: 0,
        feeSummary: createEmptyOperatorRegionConfigSummary(),
        peakSummary: createEmptyOperatorPeakConfigSummary()
    },

    onLoad(options: RegionConfigPageOptions) {
        if (options.id) {
            this.setData({
                regionId: parseInt(options.id, 10),
                regionName: options.region_name ? decodeURIComponent(options.region_name) : ''
            })
            this.loadData()
        } else {
            wx.navigateBack()
        }
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
        this.setData({
            navBarHeight: e.detail.navBarHeight
        })
    },

    onRetry() {
        this.loadData()
    },

    async loadData() {
        this.setData({ initialLoading: true, error: '' })
        try {
            const overview = await loadOperatorRegionConfigOverview(this.data.regionId)

            this.setData({
                initialLoading: false,
                feeSummary: overview.feeSummary,
                peakSummary: overview.peakSummary
            })
        } catch (err: unknown) {
            const errorMsg = getErrorUserMessage(err, '加载配置失败，请稍后重试')
            this.setData({
                initialLoading: false,
                error: errorMsg
            })
        }
    },

    onOpenDeliveryFee() {
        const { regionId, regionName } = this.data
        const query = `region_id=${regionId}${regionName ? `&region_name=${encodeURIComponent(regionName)}` : ''}`
        wx.navigateTo({ url: `/pages/operator/delivery-fee/index?${query}` })
    },

    onOpenTimeslot() {
        const { regionId, regionName } = this.data
        const query = `region_id=${regionId}${regionName ? `&region_name=${encodeURIComponent(regionName)}` : ''}`
        wx.navigateTo({ url: `/pages/operator/timeslot/index?${query}` })
    }
})
