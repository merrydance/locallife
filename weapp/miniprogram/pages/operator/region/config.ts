
import { deliveryFeeService } from '../../../api/delivery-fee'
import type { DeliveryFeeConfigResponse, PeakHourConfigResponse } from '../../../api/delivery-fee'
import { getErrorUserMessage } from '../../../utils/user-facing'

interface RegionConfigPageOptions {
    id?: string
    region_name?: string
}

interface RegionConfigSummary {
    baseFee: string
    baseDistance: string
    extraFeePerKm: string
    valueRatio: string
    minFee: string
    maxFee: string
    statusText: string
}

interface PeakConfigSummary {
    count: number
    daysText: string
    note: string
}

function formatFen(amount?: number): string {
    if (typeof amount !== 'number') {
        return '未配置'
    }
    return `${(amount / 100).toFixed(2)}元`
}

function formatDayList(days: number[]): string {
    const map = ['日', '一', '二', '三', '四', '五', '六']
    return days
        .map((day) => map[day] || '')
        .filter(Boolean)
        .join('、')
}

function buildFeeSummary(config: DeliveryFeeConfigResponse | null): RegionConfigSummary {
    if (!config) {
        return {
            baseFee: '未配置',
            baseDistance: '未配置',
            extraFeePerKm: '未配置',
            valueRatio: '未配置',
            minFee: '未配置',
            maxFee: '不限',
            statusText: '当前区域还没有基础配送费配置'
        }
    }

    return {
        baseFee: formatFen(config.base_fee),
        baseDistance: `${config.base_distance}米`,
        extraFeePerKm: formatFen(config.extra_fee_per_km),
        valueRatio: `${(config.value_ratio * 100).toFixed(2)}%`,
        minFee: formatFen(config.min_fee),
        maxFee: typeof config.max_fee === 'number' ? formatFen(config.max_fee) : '不限',
        statusText: config.is_active ? '当前配置已启用' : '当前配置未启用'
    }
}

function buildPeakSummary(configs: PeakHourConfigResponse[]): PeakConfigSummary {
    if (!configs.length) {
        return {
            count: 0,
            daysText: '暂无峰时时段',
            note: '当前后端仅支持新增和删除；若需调整已有时段，请删除后重建。'
        }
    }

    const uniqueDays = Array.from(
        new Set(
            configs.reduce<number[]>((allDays, item) => allDays.concat(item.days_of_week || []), [])
        )
    ).sort((left, right) => left - right)

    return {
        count: configs.length,
        daysText: uniqueDays.length ? `覆盖周${formatDayList(uniqueDays)}` : '已配置峰时时段',
        note: '当前后端仅支持新增和删除；若需调整已有时段，请删除后重建。'
    }
}

Page({
    data: {
        regionId: 0,
        regionName: '',
        initialLoading: true,
        error: '',
        navBarHeight: 0,
        feeSummary: buildFeeSummary(null),
        peakSummary: buildPeakSummary([])
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
            const regionId = this.data.regionId
            const [feeConfig, peakConfigs] = await Promise.all([
                this.loadFeeConfigSafe(regionId),
                deliveryFeeService.getPeakConfigs(regionId)
            ])

            this.setData({
                initialLoading: false,
                feeSummary: buildFeeSummary(feeConfig),
                peakSummary: buildPeakSummary(peakConfigs)
            })
        } catch (err: unknown) {
            const errorMsg = getErrorUserMessage(err, '加载配置失败，请稍后重试')
            this.setData({
                initialLoading: false,
                error: errorMsg
            })
        }
    },

    async loadFeeConfigSafe(id: number): Promise<DeliveryFeeConfigResponse | null> {
        try {
            return await deliveryFeeService.getRegionConfig(id)
        } catch (_e) {
            return null
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
