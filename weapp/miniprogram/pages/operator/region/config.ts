
import { deliveryFeeService } from '../../../api/delivery-fee'
import type { DeliveryFeeConfigResponse, PeakHourConfigResponse, CreateDeliveryFeeConfigRequest, CreatePeakHourConfigRequest } from '../../../api/delivery-fee'

interface RegionConfigPageOptions {
    id?: string
}

interface FieldInputEvent {
    detail: { value: string }
    currentTarget: { dataset: { field?: string } }
}

interface DayToggleEvent {
    currentTarget: { dataset: { day?: number } }
}

Page({
    data: {
        regionId: 0,
        initialLoading: true,
        error: '',
        navBarHeight: 0,

        // 配送费配置
        feeConfig: null as DeliveryFeeConfigResponse | null,

        // 峰时配置
        peakConfigs: [] as PeakHourConfigResponse[],

        // 表单状态
        baseFeeInput: '', // 元
        baseDistanceInput: '', // 米
        extraFeeInput: '', // 元/km
        valueRatioInput: '', // %
        minFeeInput: '', // 元
        maxFeeInput: '', // 元，空表示不限

        // 峰时弹窗
        showPeakModal: false,
        peakForm: {
            startTime: '11:00',
            endTime: '13:00',
            coefficient: '1.50',
            days: [1, 2, 3, 4, 5] // 0=周日, 1-5=周一到周五
        },

        daysOptions: [
            { value: 0, label: '日' },
            { value: 1, label: '一' },
            { value: 2, label: '二' },
            { value: 3, label: '三' },
            { value: 4, label: '四' },
            { value: 5, label: '五' },
            { value: 6, label: '六' }
        ]
    },

    onLoad(options: RegionConfigPageOptions) {
        if (options.id) {
            this.setData({ regionId: parseInt(options.id) })
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

            // 并行加载配置
            const [feeConfig, peakConfigs] = await Promise.all([
                // 容错处理：如果配置不存在(404)，后端可能报错，这里应该在Service层处理或这里catch
                // 假设 Service 会抛出异常如果 404
                this.loadFeeConfigSafe(regionId),
                deliveryFeeService.getPeakConfigs(regionId)
            ])

            this.setData({
                feeConfig,
                peakConfigs,
                initialLoading: false,
                // 初始化表单显示
                baseFeeInput: feeConfig ? (feeConfig.base_fee / 100).toString() : '0',
                baseDistanceInput: feeConfig ? feeConfig.base_distance.toString() : '3000',
                extraFeeInput: feeConfig ? (feeConfig.extra_fee_per_km / 100).toString() : '1',
                valueRatioInput: feeConfig ? (feeConfig.value_ratio * 100).toString() : '1',
                minFeeInput: feeConfig ? (feeConfig.min_fee / 100).toString() : '0',
                maxFeeInput: feeConfig && typeof feeConfig.max_fee === 'number' ? (feeConfig.max_fee / 100).toString() : ''
            })

        } catch (err: unknown) {
            console.error(err)
            const errorMsg = err instanceof Error ? err.message : '加载配置失败'
            this.setData({
                initialLoading: false,
                error: errorMsg
            })
            // wx.showToast({ title: '加载配置失败', icon: 'error' }) // Reduce noise if displaying error UI
        }
    },

    // 安全加载配置，如果不存在则返回 null
    async loadFeeConfigSafe(id: number): Promise<DeliveryFeeConfigResponse | null> {
        try {
            return await deliveryFeeService.getRegionConfig(id)
        } catch (_e) {
            return null
        }
    },

    onInputChange(e: FieldInputEvent) {
        const { field } = e.currentTarget.dataset
        if (!field) return
        this.setData({ [field]: e.detail.value })
    },

    async onSaveFeeConfig() {
        try {
            const { regionId, baseFeeInput, baseDistanceInput, extraFeeInput, valueRatioInput, minFeeInput, maxFeeInput } = this.data

            const maxFee = maxFeeInput === '' ? undefined : Math.round(parseFloat(maxFeeInput) * 100)

            const data: CreateDeliveryFeeConfigRequest = {
                region_id: regionId,
                base_fee: parseFloat(baseFeeInput) * 100,
                base_distance: parseInt(baseDistanceInput),
                extra_fee_per_km: parseFloat(extraFeeInput) * 100,
                value_ratio: parseFloat(valueRatioInput) / 100,
                min_fee: parseFloat(minFeeInput) * 100,
                max_fee: maxFee
            }

            wx.showLoading({ title: '保存中' })
            const res = await deliveryFeeService.updateRegionConfig(regionId, data)
            this.setData({ feeConfig: res })
            wx.showToast({ title: '保存成功' })

        } catch (err) {
            wx.showToast({ title: '保存失败', icon: 'error' })
        }
    },

    // 峰时管理
    onAddPeak() {
        this.setData({ showPeakModal: true })
    },

    onClosePeakModal() {
        this.setData({ showPeakModal: false })
    },

    onPeakFormChange(e: FieldInputEvent) {
        const { field } = e.currentTarget.dataset
        if (!field) return
        this.setData({ [`peakForm.${field}`]: e.detail.value })
    },

    onDayToggle(e: DayToggleEvent) {
        const day = e.currentTarget.dataset.day
        if (day === undefined) return
        const { days } = this.data.peakForm
        const newDays = days.includes(day)
            ? days.filter((d) => d !== day)
            : [...days, day].sort()

        this.setData({ 'peakForm.days': newDays })
    },

    async onSavePeak() {
        const { regionId, peakForm } = this.data

        const data: CreatePeakHourConfigRequest = {
            region_id: regionId,
            start_time: peakForm.startTime,
            end_time: peakForm.endTime,
            coefficient: parseFloat(peakForm.coefficient),
            days_of_week: peakForm.days
        }

        try {
            wx.showLoading({ title: '添加中' })
            await deliveryFeeService.createPeakConfig(regionId, data)

            this.setData({ showPeakModal: false })
            const peaks = await deliveryFeeService.getPeakConfigs(regionId)
            this.setData({ peakConfigs: peaks })

            wx.showToast({ title: '添加成功' })
        } catch (err) {
            wx.showToast({ title: '添加失败', icon: 'error' })
        }
    },

    async onDeletePeak(e: WechatMiniprogram.TouchEvent) {
        const { id } = e.currentTarget.dataset as { id?: number }
        if (!id) return
        wx.showModal({
            title: '删除确认',
            content: '确定删除该峰时配置吗？',
            success: async (res) => {
                if (res.confirm) {
                    await deliveryFeeService.deletePeakConfig(id)
                    const peaks = await deliveryFeeService.getPeakConfigs(this.data.regionId)
                    this.setData({ peakConfigs: peaks })
                }
            }
        })
    },

    formatDays(days: number[]) {
        const map = ['日', '一', '二', '三', '四', '五', '六']
        return days.map((d) => map[d]).join('、')
    }
})
