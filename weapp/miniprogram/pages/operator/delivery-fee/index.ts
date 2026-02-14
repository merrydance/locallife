import { deliveryFeeService } from '../../../api/delivery-fee'

interface DeliveryFeeConfigView {
    base_fee: number
    base_distance: number
    extra_distance_fee: number
    min_order_amount: number
    max_delivery_distance: number
    is_active: boolean
    night_surcharge: number
    night_start: string
    night_end: string
    extra_distance_unit: number
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
            base_fee: 0,
            base_distance: 0,
            extra_distance_fee: 0,
            min_order_amount: 0,
            max_delivery_distance: 0,
            is_active: true,
            night_surcharge: 0,
            night_start: '22:00',
            night_end: '06:00',
            extra_distance_unit: 1000
        } as DeliveryFeeConfigView,
        regionId: 1, // Default region ID for simple admin config
        initialLoading: true,
        loading: false,
        error: '',
        navBarHeight: 0
    },

    onLoad(options: DeliveryFeePageOptions) {
        if (options.region_id) {
            this.setData({ regionId: parseInt(options.region_id) })
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
        this.setData({ initialLoading: true, error: '' })
        try {
            const config = await deliveryFeeService.getRegionConfig(this.data.regionId)
            const optionalConfig = config as unknown as Record<string, unknown>
            const nightSurcharge = typeof optionalConfig.night_surcharge === 'number' ? optionalConfig.night_surcharge : 0
            const nightStart = typeof optionalConfig.night_start === 'string' ? optionalConfig.night_start : '22:00'
            const nightEnd = typeof optionalConfig.night_end === 'string' ? optionalConfig.night_end : '06:00'
            const extraDistanceUnit = typeof optionalConfig.extra_distance_unit === 'number' ? optionalConfig.extra_distance_unit : 1000
            this.setData({
                config: {
                    ...config,
                    base_fee: config.base_fee / 100,
                    extra_distance_fee: config.extra_distance_fee / 100,
                    min_order_amount: config.min_order_amount / 100,
                    // Mock additional fields if not present in API response yet
                    night_surcharge: nightSurcharge / 100,
                    night_start: nightStart,
                    night_end: nightEnd,
                    extra_distance_unit: extraDistanceUnit
                },
                initialLoading: false
            })
        } catch (error: unknown) {
            console.error(error)
            const errorMsg = error instanceof Error ? error.message : '加载配置失败'
            
            // Mock data for fallback/demo purposes if API fails (or for development)
            // In production, we might want to just show the error.
            // For this refactor, let's show error but allow retry
            this.setData({
                error: errorMsg,
                initialLoading: false
            })
            
             // Fallback for development if needed:
             /*
            this.setData({
                'config.base_fee': 5,
                'config.base_distance': 3000,
                'config.extra_distance_fee': 2,
                'config.min_order_amount': 20,
                'config.max_delivery_distance': 10000,
                initialLoading: false
            });
            */
        }
    },

    onInput(e: WechatMiniprogram.CustomEvent<ConfigInputEventDetail>) {
        const { field } = e.currentTarget.dataset as { field?: string }
        if (!field) return
        this.setData({
            [`config.${field}`]: e.detail.value
        })
    },

    onTimeChange(e: WechatMiniprogram.CustomEvent<ConfigInputEventDetail>) {
        const { field } = e.currentTarget.dataset as { field?: string }
        if (!field) return
        this.setData({
            [`config.${field}`]: e.detail.value
        })
    },

    async onSave() {
        const { config, regionId } = this.data

        try {
            wx.showLoading({ title: '保存中' })

            const submitData = {
                base_fee: Number(config.base_fee) * 100,
                base_distance: Number(config.base_distance),
                extra_distance_fee: Number(config.extra_distance_fee) * 100,
                min_order_amount: Number(config.min_order_amount) * 100,
                max_delivery_distance: Number(config.max_delivery_distance),
                is_active: config.is_active
                // Add unknown fields only if API supports them, otherwise this might just be local state
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

