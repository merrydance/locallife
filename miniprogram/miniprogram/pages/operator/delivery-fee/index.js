"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
const delivery_fee_1 = require("../../../api/delivery-fee");
Page({
    data: {
        config: {
            base_fee: 0,
            base_distance: 0,
            extra_distance_fee: 0,
            min_order_amount: 0,
            max_delivery_distance: 0,
            is_active: true
        },
        regionId: 1, // Default region ID for simple admin config
        loading: false
    },
    onLoad(options) {
        if (options.region_id) {
            this.setData({ regionId: parseInt(options.region_id) });
        }
        this.loadConfig();
    },
    loadConfig() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const config = yield delivery_fee_1.deliveryFeeService.getRegionConfig(this.data.regionId);
                this.setData({
                    config: Object.assign(Object.assign({}, config), { base_fee: config.base_fee / 100, extra_distance_fee: config.extra_distance_fee / 100, min_order_amount: config.min_order_amount / 100 }),
                    loading: false
                });
            }
            catch (error) {
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
        });
    },
    onInput(e) {
        const field = e.currentTarget.dataset.field;
        this.setData({
            [`config.${field}`]: e.detail.value
        });
    },
    onSave() {
        return __awaiter(this, void 0, void 0, function* () {
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
                yield delivery_fee_1.deliveryFeeService.updateRegionConfig(regionId, submitData);
                wx.showToast({ title: '保存成功', icon: 'success' });
            }
            catch (error) {
                wx.showToast({ title: error.message || '保存失败', icon: 'none' });
            }
            finally {
                wx.hideLoading();
            }
        });
    }
});
