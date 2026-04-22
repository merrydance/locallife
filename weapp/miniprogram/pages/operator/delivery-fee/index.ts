import {
    createEmptyOperatorDeliveryFeeConfigView,
    loadOperatorDeliveryFeeConfigView,
    saveOperatorDeliveryFeeConfig,
    type OperatorDeliveryFeeConfigView
} from '../../../services/operator-region-config'
import { getErrorUserMessage } from '../../../utils/user-facing'

interface DeliveryFeePageOptions {
    region_id?: string
}

interface ConfigInputEventDetail {
    value: string | number
}

Page({
    data: {
        config: createEmptyOperatorDeliveryFeeConfigView() as OperatorDeliveryFeeConfigView,
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
            this.setData({
                config: await loadOperatorDeliveryFeeConfigView(this.data.regionId),
                initialLoading: false
            })
        } catch (error: unknown) {
            console.error(error)
            const errorMsg = getErrorUserMessage(error, '加载配置失败，请稍后重试')
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
            await saveOperatorDeliveryFeeConfig(regionId, config)
        } catch (error: unknown) {
            const message = getErrorUserMessage(error, '保存失败，请稍后重试')
            wx.showToast({ title: message, icon: 'none' })
        } finally {
            wx.hideLoading()
        }
    }
})

