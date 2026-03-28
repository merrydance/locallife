import { deliveryFeeService } from '../../../api/delivery-fee'

interface DeliveryFeeConfigView {
    base_fee: string
    base_distance: string
    extra_fee_per_km: string
    value_ratio: string
    min_fee: string
    max_fee: string
    is_active: boolean
}

interface DeliveryFeePageOptions {
    region_id?: string
}

interface ConfigInputEventDetail {
    value: string | number
}

Page({
    data: {
        config: {
            base_fee: '',
            base_distance: '',
            extra_fee_per_km: '',
            value_ratio: '0',
            min_fee: '',
            max_fee: '',
            is_active: true
        } as DeliveryFeeConfigView,
        regionId: 0,
        initialLoading: true,
        loading: false,
        error: '',
        navBarHeight: 0
    },

    onLoad(options: DeliveryFeePageOptions) {
        if (options.region_id) {
            this.setData({ regionId: parseInt(options.region_id) })
        }
        if (!this.data.regionId) {
            this.setData({ initialLoading: false, error: '缺少区域ID，无法加载配送费配置' })
            return
        }
        this.loadConfig()
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
        this.setData({
            navBarHeight: e.detail.navBarHeight
        })
    },

    onRetry() {
        this.loadConfig()
    },

    async loadConfig() {
        if (!this.data.regionId) {
            this.setData({ initialLoading: false, error: '缺少区域ID，无法加载配送费配置' })
            return
        }

        this.setData({ initialLoading: true, error: '' })
        try {
            const config = await deliveryFeeService.getRegionConfig(this.data.regionId)
            this.setData({
                config: {
                    base_fee: (config.base_fee / 100).toFixed(2),
                    base_distance: String(config.base_distance),
                    extra_fee_per_km: (config.extra_fee_per_km / 100).toFixed(2),
                    value_ratio: String(config.value_ratio),
                    min_fee: (config.min_fee / 100).toFixed(2),
                    max_fee: typeof config.max_fee === 'number' ? (config.max_fee / 100).toFixed(2) : '',
                    is_active: config.is_active
                },
                initialLoading: false
            })
        } catch (error: unknown) {
            console.error(error)
            const errorMsg = error instanceof Error ? error.message : '加载配置失败'
            this.setData({
                error: errorMsg,
                initialLoading: false
            })
        }
    },

    onInput(e: WechatMiniprogram.CustomEvent<ConfigInputEventDetail>) {
        const { field } = e.currentTarget.dataset as { field?: string }
        if (!field) return
        this.setData({
            [`config.${field}`]: e.detail.value
        })
    },

    onActiveChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
        this.setData({
            'config.is_active': e.detail.value
        })
    },

    async onSave() {
        const { config, regionId } = this.data

        if (!regionId) {
            wx.showToast({ title: '缺少区域ID', icon: 'none' })
            return
        }

        try {
            wx.showLoading({ title: '保存中' })

            const submitData = {
                region_id: regionId,
                base_fee: Math.round(Number(config.base_fee) * 100),
                base_distance: Number(config.base_distance),
                extra_fee_per_km: Math.round(Number(config.extra_fee_per_km) * 100),
                value_ratio: Number(config.value_ratio),
                min_fee: Math.round(Number(config.min_fee) * 100),
                max_fee: config.max_fee ? Math.round(Number(config.max_fee) * 100) : undefined
            }

            await deliveryFeeService.updateRegionConfig(regionId, submitData)
            wx.showToast({ title: '保存成功', icon: 'success' })
        } catch (error: unknown) {
            const message = error instanceof Error ? error.message : '保存失败'
            wx.showToast({ title: message, icon: 'none' })
        } finally {
            wx.hideLoading()
        }
    }
})

