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
const responsive_1 = require("@/utils/responsive");
Page({
    data: {
        rules: [],
        isLargeScreen: false,
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadRules();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadRules() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                // Mock data - GET /api/v1/operator/rules
                const mockRules = [
                    {
                        id: 'rule_1',
                        name: '商户入驻保证金',
                        key: 'MERCHANT_DEPOSIT',
                        value: '5000',
                        unit: '元',
                        desc: '商户入驻需缴纳的保证金金额'
                    },
                    {
                        id: 'rule_2',
                        name: '骑手入驻押金',
                        key: 'RIDER_DEPOSIT',
                        value: '500',
                        unit: '元',
                        desc: '骑手接单前需缴纳的押金金额'
                    },
                    {
                        id: 'rule_3',
                        name: '平台抽成比例',
                        key: 'PLATFORM_COMMISSION',
                        value: '15',
                        unit: '%',
                        desc: '每笔订单平台收取的服务费比例'
                    }
                ];
                this.setData({
                    rules: mockRules,
                    loading: false
                });
            }
            catch (error) {
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    onEditRule(e) {
        const { id } = e.currentTarget.dataset;
        const rule = this.data.rules.find((r) => r.id === id);
        if (!rule)
            return;
        wx.showModal({
            title: `修改${rule.name}`,
            content: rule.value,
            editable: true,
            placeholderText: '请输入新值',
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm && res.content) {
                    // PATCH /api/v1/operator/rules/{id}
                    wx.showToast({ title: '修改成功', icon: 'success' });
                    this.loadRules();
                }
            })
        });
    }
});
