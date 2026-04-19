/**
 * 运营商工作台
 * 提供区域管理、商户管理、骑手管理、数据统计等功能入口
 */

import {
    loadLegacyOperatorWorkbenchData,
    loadLegacyOperatorWorkbenchRegionData,
    type LegacyAppealSummary,
    type LegacyMerchantSummary,
    type LegacyOperatorFinanceOverview,
    type LegacyOperatorInfo,
    type LegacyRegionStatsItem,
    type LegacyRiderSummary
} from '@/services/operator-console'

interface QuickActionDataset {
    url?: string
}

Page({
    data: {
        loading: false,
        initialLoading: true,
        error: null as string | null,
        navBarHeight: 88,
        refreshing: false,

        // 运营商信息
        operatorInfo: null as LegacyOperatorInfo | null,

        // 财务概览
        financeOverview: null as LegacyOperatorFinanceOverview | null,

        // 区域统计
        regionStats: [] as LegacyRegionStatsItem[],
        regionPickerOptions: [] as Array<{ label: string, value: string }>,
        regionPickerVisible: false,
        selectedRegionIdx: 0,   // picker 用 index
        selectedRegionId: 0,    // 实际 region_id，传给 API
        selectedRegionValue: '',

        // 商户摘要
        merchantSummary: {
            total: 0,
            active: 0,
            suspended: 0,
            pending: 0
        } as LegacyMerchantSummary,

        // 骑手摘要
        riderSummary: {
            total: 0,
            active: 0,
            online: 0,
            suspended: 0,
            pending: 0
        } as LegacyRiderSummary,

        // 申诉摘要
        appealSummary: {
            totalAppeals: 0,
            pendingAppeals: 0,
            avgResolutionTime: 0,
            satisfactionRate: 0
        } as LegacyAppealSummary,

        // 快捷入口
        quickActions: [
            { id: 'merchants', icon: 'shop', label: '商户管理', url: '/pages/operator/merchants/index' },
            { id: 'riders', icon: 'user', label: '骑手管理', url: '/pages/operator/riders/index' },
            { id: 'analytics', icon: 'chart', label: '数据分析', url: '/pages/operator/analytics/index' },
            { id: 'appeals', icon: 'service', label: '客诉处理', url: '/pages/operator/appeal/list/index' }
        ]
    },

    onLoad() {
        this.loadDashboardData()
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    onPullDownRefresh() {
        this.setData({ refreshing: true })
        this.loadDashboardData().finally(() => {
            this.setData({ refreshing: false })
            wx.stopPullDownRefresh()
        })
    },

    /**
     * 加载工作台数据
     */
    async loadDashboardData() {
        if (this.data.loading && !this.data.initialLoading) return
        this.setData({ loading: true, error: null })

        try {
            const nextView = await loadLegacyOperatorWorkbenchData()

            this.setData({
                ...nextView,
                loading: false,
                initialLoading: false
            })
        } catch (error) {
            console.error('加载工作台数据失败:', error)
            this.setData({ 
                loading: false, 
                initialLoading: false,
                error: '加载工作台数据失败'
            })
        }
    },

    onRetry() {
        this.loadDashboardData()
    },

    onOpenRegionPicker() {
        if (this.data.regionStats.length <= 1) {
            return
        }

        this.setData({ regionPickerVisible: true })
    },

    onCloseRegionPicker() {
        this.setData({ regionPickerVisible: false })
    },

    onRegionConfirm(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null }>) {
        const values = Array.isArray(e.detail?.value) ? e.detail.value : []
        const selectedValue = String(values[0] || '')
        const idx = this.data.regionPickerOptions.findIndex((item) => item.value === selectedValue)
        const region = idx >= 0 ? this.data.regionStats[idx] : null

        this.setData({
            regionPickerVisible: false,
            selectedRegionIdx: idx >= 0 ? idx : this.data.selectedRegionIdx,
            selectedRegionId: region?.region_id || this.data.selectedRegionId,
            selectedRegionValue: selectedValue || this.data.selectedRegionValue
        })

        if (region?.region_id) this.loadRegionData(region.region_id)
    },

    /**
     * 切换区域（e.detail.value 是 picker 选中的数组下标）
     */
    onRegionChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
        const idx = parseInt(e.detail.value)
        const regionId = this.data.regionStats[idx]?.region_id || 0
        this.setData({ selectedRegionIdx: idx, selectedRegionId: regionId })
        if (regionId) this.loadRegionData(regionId)
    },

    /**
     * 加载指定区域的数据
     */
    async loadRegionData(regionId: number) {
        try {
            wx.showLoading({ title: '加载中...' })

            const nextView = await loadLegacyOperatorWorkbenchRegionData(regionId)

            this.setData({
                ...nextView
            })
        } catch (error) {
            console.error('加载区域数据失败:', error)
            wx.showToast({
                title: '加载失败',
                icon: 'none'
            })
        } finally {
            wx.hideLoading()
        }
    },

    /**
     * 快捷入口点击
     */
    onQuickActionTap(e: WechatMiniprogram.TouchEvent) {
        const { url } = e.currentTarget.dataset as QuickActionDataset
        if (!url) return
        wx.navigateTo({ url })
    },

    /**
     * 查看财务详情
     */
    onFinanceDetailTap() {
        wx.navigateTo({
            url: '/pages/operator/finance/withdraw/index'
        })
    },

    /**
     * 查看区域详情
     */
    onRegionDetailTap() {
        const { selectedRegionId } = this.data
        if (selectedRegionId) {
            wx.navigateTo({
                url: `/pages/operator/region/config?id=${selectedRegionId}`
            })
        }
    },

    /**
     * 格式化金额
     */
    formatAmount(amount: number): string {
        return `¥${(amount / 100).toFixed(2)}`
    },

    /**
     * 格式化百分比
     */
    formatPercentage(value: number): string {
        return `${value.toFixed(1)}%`
    }
})
