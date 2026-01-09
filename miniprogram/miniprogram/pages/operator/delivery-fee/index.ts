import { deliveryFeeService, DeliveryFeeConfigResponse } from '../../../api/delivery-fee';

Page({
    data: {
        config: {
            base_fee: 0,
            base_distance: 0,
            extra_distance_fee: 0,
            min_order_amount: 0,
            max_delivery_distance: 0,
            is_active: true
        } as Partial<DeliveryFeeConfigResponse>,
        regionId: 1, // Default region ID for simple admin config
        loading: false
    },

    onLoad(options: any) {
        if (options.region_id) {
            this.setData({ regionId: parseInt(options.region_id) });
        }
        this.loadConfig();
    },

    async loadConfig() {
        this.setData({ loading: true });
        try {
            const config = await deliveryFeeService.getRegionConfig(this.data.regionId);
            this.setData({
                config: {
                    ...config,
                    base_fee: config.base_fee / 100,
                    extra_distance_fee: config.extra_distance_fee / 100,
                    min_order_amount: config.min_order_amount / 100
                },
                loading: false
            });
        } catch (error) {
            console.error(error);
            this.setData({
                'config.base_fee': 5,
                'config.base_distance': 3000,
                'config.extra_distance_fee': 2,
                'config.min_order_amount': 20,
                'config.max_delivery_distance': 10000,
                loading: false
            });
        }
    },

    onInput(e: any) {
        const field = e.currentTarget.dataset.field;
        this.setData({
            [`config.${field}`]: e.detail.value
        });
    },

    async onSave() {
        const { config, regionId } = this.data;

        try {
            wx.showLoading({ title: '保存中' });

            const submitData = {
                base_fee: Number(config.base_fee) * 100,
                base_distance: Number(config.base_distance),
                extra_distance_fee: Number(config.extra_distance_fee) * 100,
                min_order_amount: Number(config.min_order_amount) * 100,
                max_delivery_distance: Number(config.max_delivery_distance),
                is_active: config.is_active
            };

            await deliveryFeeService.updateRegionConfig(regionId, submitData);
            wx.showToast({ title: '保存成功', icon: 'success' });
        } catch (error: any) {
            wx.showToast({ title: error.message || '保存失败', icon: 'none' });
        } finally {
            wx.hideLoading();
        }
    }
});
