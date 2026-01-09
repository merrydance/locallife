
import { deliveryFeeService, DeliveryFeeAdapter } from '../../../api/delivery-fee'
import type { DeliveryFeeConfigResponse, PeakHourConfigResponse, CreateDeliveryFeeConfigRequest, CreatePeakHourConfigRequest } from '../../../api/delivery-fee'

Page({
    data: {
        regionId: 0,
        loading: true,

        // 配送费配置
        feeConfig: null as DeliveryFeeConfigResponse | null,

        // 峰时配置
        peakConfigs: [] as PeakHourConfigResponse[],

        // 表单状态
        baseFeeInput: '', // 元
        baseDistanceInput: '', // 米
        extraFeeInput: '', // 元/km
        minOrderInput: '', // 元
        maxDistanceInput: '', // 米

        // 峰时弹窗
        showPeakModal: false,
        peakForm: {
            startTime: '11:00',
            endTime: '13:00',
            multiplier: '1.5',
            extraFee: '0',
            days: [1, 2, 3, 4, 5] // 默认周一到周五
        },

        daysOptions: [
            { value: 1, label: '一' },
            { value: 2, label: '二' },
            { value: 3, label: '三' },
            { value: 4, label: '四' },
            { value: 5, label: '五' },
            { value: 6, label: '六' },
            { value: 7, label: '日' }
        ]
    },

    onLoad(options: any) {
        if (options.id) {
            this.setData({ regionId: parseInt(options.id) })
            this.loadData()
        } else {
            wx.navigateBack()
        }
    },

    async loadData() {
        this.setData({ loading: true })
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
                loading: false,
                // 初始化表单显示
                baseFeeInput: feeConfig ? (feeConfig.base_fee / 100).toString() : '0',
                baseDistanceInput: feeConfig ? feeConfig.base_distance.toString() : '3000',
                extraFeeInput: feeConfig ? (feeConfig.extra_distance_fee / 100).toString() : '1',
                minOrderInput: feeConfig ? (feeConfig.min_order_amount / 100).toString() : '0',
                maxDistanceInput: feeConfig ? feeConfig.max_delivery_distance.toString() : '10000'
            })

        } catch (err) {
            console.error(err)
            wx.showToast({ title: '加载配置失败', icon: 'error' })
            this.setData({ loading: false })
        }
    },

    // 安全加载配置，如果不存在则返回 null
    async loadFeeConfigSafe(id: number) {
        try {
            return await deliveryFeeService.getRegionConfig(id)
        } catch (e) {
            return null
        }
    },

    onInputChange(e: any) {
        const { field } = e.currentTarget.dataset
        this.setData({ [field]: e.detail.value })
    },

    async onSaveFeeConfig() {
        try {
            const { regionId, baseFeeInput, baseDistanceInput, extraFeeInput, minOrderInput, maxDistanceInput } = this.data

            const data: CreateDeliveryFeeConfigRequest = {
                base_fee: parseFloat(baseFeeInput) * 100,
                base_distance: parseInt(baseDistanceInput),
                extra_distance_fee: parseFloat(extraFeeInput) * 100,
                min_order_amount: parseFloat(minOrderInput) * 100,
                max_delivery_distance: parseInt(maxDistanceInput),
                is_active: true
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

    onPeakFormChange(e: any) {
        const { field } = e.currentTarget.dataset
        this.setData({ [`peakForm.${field}`]: e.detail.value })
    },

    onDayToggle(e: any) {
        const day = e.currentTarget.dataset.day;
        const { days } = this.data.peakForm;
        const newDays = days.includes(day)
            ? days.filter(d => d !== day)
            : [...days, day].sort();

        this.setData({ 'peakForm.days': newDays });
    },

    async onSavePeak() {
        const { regionId, peakForm } = this.data

        const data: CreatePeakHourConfigRequest = {
            start_time: peakForm.startTime,
            end_time: peakForm.endTime,
            multiplier: parseFloat(peakForm.multiplier),
            extra_fee: parseFloat(peakForm.extraFee) * 100,
            days_of_week: peakForm.days,
            is_active: true,
            name: '高峰时段'
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

    async onDeletePeak(e: any) {
        const id = e.currentTarget.dataset.id
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
        const map = ['', '一', '二', '三', '四', '五', '六', '日']
        return days.map(d => map[d]).join('、')
    }
})
